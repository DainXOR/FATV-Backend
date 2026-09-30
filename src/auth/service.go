package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"dainxor/atv/configs"
	"dainxor/atv/emailer"
	"dainxor/atv/logger"
	"dainxor/atv/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidCode        = errors.New("invalid or expired code")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
)

type Service struct{}

var Default Service

type LoginResult struct {
	User         models.AuthUserDB
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
}

type PublicUser struct {
	ID     string               `json:"id"`
	Email  string               `json:"email"`
	Role   models.UserRole      `json:"role"`
	Status models.AccountStatus `json:"status,omitempty"`
}

func PublicUserFrom(u models.AuthUserDB) PublicUser {
	return PublicUser{ID: u.ID.Hex(), Email: u.Email, Role: u.Role, Status: u.Status}
}

func (Service) Initialize() error {
	if err := configs.DB.EnsureAuthIndexes(); err != nil {
		return err
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: "global"}}, models.AuthSettingsDB{})
	if result.IsOk() {
		return nil
	}
	if !errors.Is(result.Error(), configs.DBErr.NotFound()) {
		return result.Error()
	}
	settings := models.AuthSettingsDB{
		ID: "global", IdleTimeoutMinutes: 480, AbsoluteLifetimeMinutes: 10080,
		SelfServiceRecoveryEnabled: true, SetupCodeLifetimeMinutes: 30,
		RecoveryCodeLifetimeMinutes: 30, UpdatedAt: time.Now().UTC(),
	}
	if _, err := insertModel(&settings); err != nil {
		return err
	}
	logger.Info("Initialized default authentication settings")
	return nil
}

func (Service) Settings() (models.AuthSettingsDB, error) {
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: "global"}}, models.AuthSettingsDB{})
	if result.IsErr() {
		return models.AuthSettingsDB{}, result.Error()
	}
	return models.InterfaceTo[models.AuthSettingsDB](result.Value()), nil
}

func (s Service) Login(email, password string) (LoginResult, error) {
	address, err := emailer.ParseAddress(email)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	result := configs.DB.FindOne(bson.D{{Key: "email", Value: string(address)}}, models.AuthUserDB{})
	if result.IsErr() {
		return LoginResult{}, ErrInvalidCredentials
	}
	user := models.InterfaceTo[models.AuthUserDB](result.Value())
	if user.Status != models.AccountActive || !user.Role.Valid() || user.PasswordHash == "" {
		s.audit(user.ID, "login", user.ID.Hex(), "denied")
		return LoginResult{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		s.audit(user.ID, "login", user.ID.Hex(), "denied")
		return LoginResult{}, ErrInvalidCredentials
	}
	settings, err := s.Settings()
	if err != nil {
		return LoginResult{}, err
	}
	token, err := randomToken(32)
	if err != nil {
		return LoginResult{}, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return LoginResult{}, err
	}
	now := time.Now().UTC()
	expires := now.Add(time.Duration(settings.AbsoluteLifetimeMinutes) * time.Minute)
	session := models.AuthSessionDB{
		UserID: user.ID, TokenHash: hash(token), CreatedAt: now,
		LastActivityAt: now, AbsoluteExpiresAt: expires,
	}
	if _, err := configs.DB.InsertOne(&session).GetRaw(); err != nil {
		logger.Error("Could not create authentication session:", err)
		return LoginResult{}, err
	}
	_ = configs.DB.PatchOne(bson.D{{Key: "_id", Value: user.ID}},
		models.AuthUserPatch{LastLoginAt: now, UpdatedAt: now})
	s.audit(user.ID, "login", user.ID.Hex(), "success")
	return LoginResult{User: user, SessionToken: token, CSRFToken: csrf, ExpiresAt: expires}, nil
}

func (Service) Authenticate(token string) (models.AuthUserDB, models.AuthSessionDB, error) {
	if token == "" {
		return models.AuthUserDB{}, models.AuthSessionDB{}, ErrUnauthorized
	}
	result := configs.DB.FindOne(bson.D{{Key: "token_hash", Value: hash(token)}}, models.AuthSessionDB{})
	if result.IsErr() {
		return models.AuthUserDB{}, models.AuthSessionDB{}, ErrUnauthorized
	}
	session := models.InterfaceTo[models.AuthSessionDB](result.Value())
	now := time.Now().UTC()
	if !session.RevokedAt.IsZero() {
		return models.AuthUserDB{}, session, ErrUnauthorized
	}
	settings, err := (Service{}).Settings()
	if err != nil {
		return models.AuthUserDB{}, session, err
	}
	if now.After(session.CreatedAt.Add(time.Duration(settings.AbsoluteLifetimeMinutes)*time.Minute)) ||
		now.Sub(session.LastActivityAt) > time.Duration(settings.IdleTimeoutMinutes)*time.Minute {
		_ = configs.DB.PatchOne(bson.D{{Key: "_id", Value: session.ID}},
			models.AuthSessionPatch{RevokedAt: now, RevocationReason: "expired"})
		return models.AuthUserDB{}, session, ErrUnauthorized
	}
	userResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: session.UserID}}, models.AuthUserDB{})
	if userResult.IsErr() {
		return models.AuthUserDB{}, session, ErrUnauthorized
	}
	user := models.InterfaceTo[models.AuthUserDB](userResult.Value())
	if user.Status != models.AccountActive || !user.Role.Valid() {
		_ = configs.DB.PatchOne(bson.D{{Key: "_id", Value: session.ID}},
			models.AuthSessionPatch{RevokedAt: now, RevocationReason: "account_inactive"})
		return models.AuthUserDB{}, session, ErrUnauthorized
	}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: session.ID}, {Key: "revoked_at", Value: nil}},
		models.AuthSessionPatch{LastActivityAt: now}); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
		return models.AuthUserDB{}, session, ErrUnauthorized
	}
	return user, session, nil
}

func (Service) Logout(token string) {
	if token == "" {
		return
	}
	result := configs.DB.FindOne(bson.D{{Key: "token_hash", Value: hash(token)}}, models.AuthSessionDB{})
	if result.IsErr() {
		return
	}
	session := models.InterfaceTo[models.AuthSessionDB](result.Value())
	now := time.Now().UTC()
	_ = configs.DB.PatchOne(bson.D{{Key: "_id", Value: session.ID}, {Key: "revoked_at", Value: nil}},
		models.AuthSessionPatch{RevokedAt: now, RevocationReason: "logout"})
	Default.audit(session.UserID, "logout", session.ID.Hex(), "success")
}

func (Service) CreateAccount(email string, role models.UserRole, actor models.DBID) error {
	address, err := emailer.ParseAddress(email)
	if err != nil || !role.Valid() {
		return fmt.Errorf("invalid email or role")
	}
	if _, err := emailer.NewFromEnv(); err != nil {
		return err
	}
	existing := configs.DB.FindOne(bson.D{{Key: "email", Value: string(address)}}, models.AuthUserDB{})
	if existing.IsOk() {
		return ErrConflict
	}
	if existing.Error() != nil && !errors.Is(existing.Error(), configs.DBErr.NotFound()) {
		return existing.Error()
	}
	now := time.Now().UTC()
	user := models.AuthUserDB{Email: string(address), Role: role, Status: models.AccountPending,
		CreatedAt: now, UpdatedAt: now, CreatedBy: actor}
	inserted, err := insertModel(&user)
	if err != nil {
		return err
	}
	user.ID = inserted
	if err := Default.issueCode(user, models.CodeSetup); err != nil {
		return err
	}
	Default.audit(actor, "account_created", user.ID.Hex(), "success")
	return nil
}

func (Service) ListAccounts() ([]models.AuthUserDB, error) {
	result := configs.DB.FindAll(bson.D{}, models.AuthUserDB{})
	if result.IsErr() {
		return nil, result.Error()
	}
	users := make([]models.AuthUserDB, 0)
	for _, item := range result.Value() {
		user := models.InterfaceTo[models.AuthUserDB](item)
		if user.Status != models.AccountDeleted {
			users = append(users, user)
		}
	}
	return users, nil
}

func (Service) UpdateAccountRole(actor, target models.DBID, role models.UserRole) error {
	if !role.Valid() {
		return fmt.Errorf("invalid account role")
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.AuthUserDB{})
	if result.IsErr() {
		return ErrNotFound
	}
	user := models.InterfaceTo[models.AuthUserDB](result.Value())
	if user.Role == models.RoleAdmin && role != models.RoleAdmin && user.Status == models.AccountActive {
		admins := configs.DB.FindAll(bson.D{{Key: "role", Value: models.RoleAdmin}, {Key: "status", Value: models.AccountActive}}, models.AuthUserDB{})
		if admins.IsErr() || len(admins.Value()) <= 1 {
			return fmt.Errorf("cannot demote the last active administrator")
		}
	}
	now := time.Now().UTC()
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: target}},
		models.AuthUserPatch{Role: role, UpdatedAt: now}); err != nil {
		return err
	}
	Default.audit(actor, "account_role_changed", target.Hex(), "success")
	return nil
}

func (Service) AdminResendSetup(actor, target models.DBID) error {
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.AuthUserDB{})
	if result.IsErr() {
		return ErrNotFound
	}
	user := models.InterfaceTo[models.AuthUserDB](result.Value())
	if user.Status != models.AccountPending {
		return ErrNotFound
	}
	if err := Default.issueCode(user, models.CodeSetup); err != nil {
		return err
	}
	Default.audit(actor, "admin_setup_resent", target.Hex(), "success")
	return nil
}

func (Service) UpdateAccountStatus(actor, target models.DBID, status models.AccountStatus) error {
	if status != models.AccountActive && status != models.AccountDisabled && status != models.AccountDeleted {
		return fmt.Errorf("invalid account status")
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.AuthUserDB{})
	if result.IsErr() {
		return ErrNotFound
	}
	user := models.InterfaceTo[models.AuthUserDB](result.Value())
	if user.Status == models.AccountDeleted && status != models.AccountDeleted {
		return fmt.Errorf("soft-deleted accounts cannot be reactivated")
	}
	if user.Role == models.RoleAdmin && user.Status == models.AccountActive && status != models.AccountActive {
		admins := configs.DB.FindAll(bson.D{{Key: "role", Value: models.RoleAdmin}, {Key: "status", Value: models.AccountActive}}, models.AuthUserDB{})
		if admins.IsErr() || len(admins.Value()) <= 1 {
			return fmt.Errorf("cannot disable or delete the last active administrator")
		}
	}
	now := time.Now().UTC()
	patch := models.AuthUserPatch{Status: status, UpdatedAt: now}
	if status == models.AccountDeleted {
		patch.DeletedAt = now
	}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: target}}, patch); err != nil {
		return err
	}
	if status != models.AccountActive {
		_ = configs.DB.UpdateMany(bson.D{{Key: "user_id", Value: target}, {Key: "revoked_at", Value: nil}},
			models.AuthSessionPatch{RevokedAt: now, RevocationReason: "account_status_changed"})
	}
	Default.audit(actor, "account_status_changed", target.Hex(), "success")
	return nil
}

func (Service) UpdateSettings(actor models.DBID, next models.AuthSettingsDB) error {
	if next.IdleTimeoutMinutes < 1 || next.IdleTimeoutMinutes > 10080 ||
		next.AbsoluteLifetimeMinutes < 1 || next.AbsoluteLifetimeMinutes > 43200 ||
		next.SetupCodeLifetimeMinutes < 5 || next.SetupCodeLifetimeMinutes > 1440 ||
		next.RecoveryCodeLifetimeMinutes < 5 || next.RecoveryCodeLifetimeMinutes > 1440 {
		return fmt.Errorf("security settings are outside permitted ranges")
	}
	now := time.Now().UTC()
	patch := models.AuthSettingsPatch{
		IdleTimeoutMinutes: next.IdleTimeoutMinutes, AbsoluteLifetimeMinutes: next.AbsoluteLifetimeMinutes,
		SelfServiceRecoveryEnabled:  next.SelfServiceRecoveryEnabled,
		SetupCodeLifetimeMinutes:    next.SetupCodeLifetimeMinutes,
		RecoveryCodeLifetimeMinutes: next.RecoveryCodeLifetimeMinutes,
		UpdatedAt:                   now, UpdatedBy: actor,
	}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: "global"}}, patch); err != nil {
		return err
	}
	Default.audit(actor, "security_settings_changed", "global", "success")
	return nil
}

func (Service) CompleteSetup(email, code, password string) error {
	return Default.completeCode(email, code, models.CodeSetup, password)
}

func (Service) RequestRecovery(email string) error {
	address, err := emailer.ParseAddress(email)
	if err != nil {
		return nil
	}
	settings, err := Default.Settings()
	if err != nil {
		return err
	}
	if !settings.SelfServiceRecoveryEnabled {
		return nil
	}
	result := configs.DB.FindOne(bson.D{{Key: "email", Value: string(address)}}, models.AuthUserDB{})
	if result.IsErr() {
		return nil
	}
	user := models.InterfaceTo[models.AuthUserDB](result.Value())
	if user.Status != models.AccountActive {
		return nil
	}
	return Default.issueCode(user, models.CodeRecovery)
}

func (Service) CompleteRecovery(email, code, password string) error {
	settings, err := Default.Settings()
	if err != nil {
		return err
	}
	if !settings.SelfServiceRecoveryEnabled {
		return ErrForbidden
	}
	return Default.completeCode(email, code, models.CodeRecovery, password)
}

func (Service) AdminIssueRecovery(actor, target models.DBID) error {
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.AuthUserDB{})
	if result.IsErr() {
		return ErrNotFound
	}
	user := models.InterfaceTo[models.AuthUserDB](result.Value())
	if user.Status != models.AccountActive {
		return ErrNotFound
	}
	if err := Default.issueCode(user, models.CodeRecovery); err != nil {
		return err
	}
	Default.audit(actor, "admin_recovery_issued", target.Hex(), "success")
	return nil
}

func (Service) issueCode(user models.AuthUserDB, purpose models.ActionCodePurpose) error {
	settings, err := Default.Settings()
	if err != nil {
		return err
	}
	lifetime := settings.SetupCodeLifetimeMinutes
	if purpose == models.CodeRecovery {
		lifetime = settings.RecoveryCodeLifetimeMinutes
	}
	mailer, err := emailer.NewFromEnv()
	if err != nil {
		return err
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_ = configs.DB.UpdateMany(bson.D{{Key: "user_id", Value: user.ID}, {Key: "purpose", Value: purpose},
		{Key: "used_at", Value: nil}, {Key: "invalidated_at", Value: nil}},
		models.AuthActionCodePatch{InvalidatedAt: now})
	record := models.AuthActionCodeDB{UserID: user.ID, Purpose: purpose, CodeHash: hash(code),
		CreatedAt: now, ExpiresAt: now.Add(time.Duration(lifetime) * time.Minute)}
	if _, err := insertModel(&record); err != nil {
		return err
	}
	var sendErr error
	if purpose == models.CodeSetup {
		sendErr = mailer.SendSetupCode(emailer.Address(user.Email), code, lifetime)
	} else {
		sendErr = mailer.SendRecoveryCode(emailer.Address(user.Email), code, lifetime)
	}
	if sendErr != nil {
		_ = configs.DB.PatchOne(bson.D{{Key: "token_hash", Value: hash(code)}},
			models.AuthActionCodePatch{InvalidatedAt: time.Now().UTC()})
		logger.Error("Email delivery failed for account action code; code was not logged:", sendErr)
		return sendErr
	}
	logger.Info("Account action code email sent", "user_id", user.ID.Hex(), "purpose", purpose)
	return nil
}

func (Service) completeCode(email, code string, purpose models.ActionCodePurpose, password string) error {
	if len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("password must be between 12 and 72 bytes")
	}
	address, err := emailer.ParseAddress(email)
	if err != nil {
		return ErrInvalidCode
	}
	userResult := configs.DB.FindOne(bson.D{{Key: "email", Value: string(address)}}, models.AuthUserDB{})
	if userResult.IsErr() {
		return ErrInvalidCode
	}
	user := models.InterfaceTo[models.AuthUserDB](userResult.Value())
	if purpose == models.CodeSetup && user.Status != models.AccountPending {
		return ErrInvalidCode
	}
	if purpose == models.CodeRecovery && user.Status != models.AccountActive {
		return ErrInvalidCode
	}
	codeResult := configs.DB.FindOne(bson.D{{Key: "user_id", Value: user.ID}, {Key: "purpose", Value: purpose},
		{Key: "code_hash", Value: hash(strings.TrimSpace(code))}, {Key: "used_at", Value: nil},
		{Key: "invalidated_at", Value: nil}, {Key: "expires_at", Value: bson.D{{Key: "$gt", Value: time.Now().UTC()}}}},
		models.AuthActionCodeDB{})
	if codeResult.IsErr() {
		Default.recordCodeFailure(user.ID, purpose)
		return ErrInvalidCode
	}
	action := models.InterfaceTo[models.AuthActionCodeDB](codeResult.Value())
	now := time.Now().UTC()
	// Conditional update makes code consumption single-use under concurrent requests.
	if err := configs.DB.PatchOne(bson.D{
		{Key: "_id", Value: action.ID},
		{Key: "used_at", Value: nil},
		{Key: "invalidated_at", Value: nil},
		{Key: "expires_at", Value: bson.D{{Key: "$gt", Value: now}}},
	}, models.AuthActionCodePatch{UsedAt: now}); err != nil {
		return ErrInvalidCode
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	status := user.Status
	if purpose == models.CodeSetup {
		status = models.AccountActive
	}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: user.ID}},
		models.AuthUserPatch{PasswordHash: string(passwordHash), Status: status, EmailVerifiedAt: now, UpdatedAt: now}); err != nil {
		return err
	}
	_ = configs.DB.UpdateMany(bson.D{{Key: "user_id", Value: user.ID}, {Key: "revoked_at", Value: nil}},
		models.AuthSessionPatch{RevokedAt: now, RevocationReason: "password_changed"})
	Default.audit(user.ID, string(purpose)+"_completed", user.ID.Hex(), "success")
	return nil
}

func (Service) audit(actor models.DBID, event, target, outcome string) {
	record := models.AuthAuditDB{At: time.Now().UTC(), ActorID: actor, Event: event, TargetID: target, Outcome: outcome}
	if _, err := configs.DB.InsertOne(&record).GetRaw(); err != nil {
		logger.Warning("Could not persist auth audit event:", event, err)
	}
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func randomToken(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08d", n.Int64()), nil
}

func SessionCookieSecure() bool {
	return strings.EqualFold(os.Getenv("SESSION_COOKIE_SECURE"), "true") ||
		strings.EqualFold(os.Getenv("APP_ENV"), "prod")
}

func (Service) BootstrapAdminFromEnv() error {
	email := strings.TrimSpace(os.Getenv("FATV_BOOTSTRAP_ADMIN_EMAIL"))
	password := os.Getenv("FATV_BOOTSTRAP_ADMIN_PASSWORD")
	if email == "" {
		logger.Warning("No FATV_BOOTSTRAP_ADMIN_EMAIL configured; no bootstrap administrator was created")
		return nil
	}
	existing := configs.DB.FindAll(bson.D{}, models.AuthUserDB{})
	if existing.IsErr() {
		return existing.Error()
	}
	if len(existing.Value()) > 0 {
		logger.Info("Bootstrap admin skipped because platform accounts already exist")
		return nil
	}

	// A configured bootstrap password allows first-run setup without SMTP.
	// This is deliberately one-time: it only runs while the platform has no accounts.
	if password != "" {
		if len(password) < 16 {
			return fmt.Errorf("FATV_BOOTSTRAP_ADMIN_PASSWORD must be at least 16 characters")
		}
		address, err := emailer.ParseAddress(email)
		if err != nil {
			return fmt.Errorf("invalid FATV_BOOTSTRAP_ADMIN_EMAIL: %w", err)
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		user := models.AuthUserDB{
			Email: string(address), PasswordHash: string(passwordHash), Role: models.RoleAdmin,
			Status: models.AccountActive, EmailVerifiedAt: now, CreatedAt: now, UpdatedAt: now,
		}
		if _, err := insertModel(&user); err != nil {
			return err
		}
		Default.audit(user.ID, "bootstrap_admin_created", user.ID.Hex(), "success")
		logger.Info("Initial FATV administrator created and activated; SMTP was not required")
		return nil
	}

	logger.Warning("Creating the initial FATV administrator using email setup; SMTP must be configured")
	return Default.CreateAccount(email, models.RoleAdmin, models.DBID{})
}

func insertModel(model models.DBModelInterface) (models.DBID, error) {
	result := configs.DB.InsertOne(model)
	if result.IsErr() {
		return models.DBID{}, result.Error()
	}
	return result.Value(), nil
}

func (Service) recordCodeFailure(userID models.DBID, purpose models.ActionCodePurpose) {
	result := configs.DB.FindAll(bson.D{{Key: "user_id", Value: userID}, {Key: "purpose", Value: purpose},
		{Key: "used_at", Value: nil}, {Key: "invalidated_at", Value: nil},
		{Key: "expires_at", Value: bson.D{{Key: "$gt", Value: time.Now().UTC()}}}},
		models.AuthActionCodeDB{})
	if result.IsErr() {
		return
	}
	var latest models.AuthActionCodeDB
	for _, item := range result.Value() {
		candidate := models.InterfaceTo[models.AuthActionCodeDB](item)
		if candidate.CreatedAt.After(latest.CreatedAt) {
			latest = candidate
		}
	}
	if latest.ID.IsZero() {
		return
	}
	attempts := latest.AttemptCount + 1
	patch := models.AuthActionCodePatch{AttemptCount: attempts}
	if attempts >= 5 {
		patch.InvalidatedAt = time.Now().UTC()
	}
	_ = configs.DB.PatchOne(bson.D{{Key: "_id", Value: latest.ID}, {Key: "used_at", Value: nil},
		{Key: "invalidated_at", Value: nil}}, patch)
}

func (s Service) RecordAudit(actor models.DBID, event, target, outcome string) {
	s.audit(actor, event, target, outcome)
}

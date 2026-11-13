package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strings"
	"time"

	"dainxor/atv/configs"
	"dainxor/atv/emailer"
	"dainxor/atv/internal/secretscope"
	"dainxor/atv/logger"
	"dainxor/atv/models"
	"dainxor/atv/utils"

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
	User         models.UserDB
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
}

type ActionInstructions struct {
	Code string
	Link string
}

type PublicUser struct {
	ID                  string               `json:"id"`
	Name                string               `json:"name,omitempty"`
	PhoneNumber         string               `json:"phone_number,omitempty"`
	CompanionID         string               `json:"companion_id,omitempty"`
	InstitutionEmail    string               `json:"institution_email,omitempty"`
	ActiveActionCode    string               `json:"active_action_code,omitempty"`
	ActionCodeExpiresAt time.Time            `json:"action_code_expires_at,omitempty"`
	SpecialityID        string               `json:"speciality_id,omitempty"`
	Email               string               `json:"email"`
	Role                models.UserRole      `json:"role"`
	IDProcess           []string             `json:"id_process,omitempty"`
	Status              models.AccountStatus `json:"status,omitempty"`
}

func PublicUserFrom(u models.UserDB) PublicUser {
	public := PublicUser{ID: u.ID.Hex(), Name: u.Name, PhoneNumber: u.PhoneNumber, Email: u.Email, Role: u.Role, Status: u.Status}
	if u.Role == models.RoleStaff {
		result := configs.DB.FindOne(bson.D{{Key: "user_id", Value: u.ID}}, models.CompanionDB{})
		if result.IsOk() {
			companion := models.InterfaceTo[models.CompanionDB](result.Value())
			public.CompanionID = companion.ID.Hex()
			public.InstitutionEmail = companion.InstitutionEmail
			public.SpecialityID = companion.IDSpeciality.Hex()
			for _, id := range companion.IDProcess {
				public.IDProcess = append(public.IDProcess, id.Hex())
			}
		}
	}
	return public
}

func (Service) Initialize() error {
	if err := migrateLegacyAccounts(); err != nil {
		return err
	}
	if err := configs.DB.EnsureAuthIndexes(); err != nil {
		return err
	}
	// TTL indexes remove expired records asynchronously; this startup sweep also
	// removes records that were already consumed before immediate deletion existed.
	now := time.Now().UTC()
	_ = configs.DB.PermanentDeleteMany(
		bson.D{{Key: "$or", Value: bson.A{
			bson.D{{Key: "expires_at", Value: bson.D{{Key: "$lte", Value: now}}}},
			bson.D{{Key: "used_at", Value: bson.D{{Key: "$ne", Value: nil}}}},
		}}},
		models.AuthActionCodeDB{},
	)
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
	user, err := findUserByEmail(email)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	if user.Status != models.AccountActive || !user.Role.Valid() || user.PasswordHash == "" {
		s.audit(user.ID, "login", user.ID.Hex(), "denied")
		return LoginResult{}, ErrInvalidCredentials
	}
	derived, deriveErr := passwordInput(password)
	if deriveErr != nil {
		return LoginResult{}, deriveErr
	}
	var compareErr error
	secretscope.Do(func() { compareErr = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), derived) })
	clearSecret(derived)
	if compareErr != nil {
		// One-time compatibility for accounts created before peppered bcrypt.
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			s.audit(user.ID, "login", user.ID.Hex(), "denied")
			return LoginResult{}, ErrInvalidCredentials
		}
		upgraded, err := hashPassword(password)
		if err != nil {
			return LoginResult{}, err
		}
		if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: user.ID}}, models.UserPatch{PasswordHash: upgraded, UpdatedAt: time.Now().UTC()}); err != nil {
			return LoginResult{}, err
		}
		user.PasswordHash = upgraded
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
		models.UserPatch{LastLoginAt: now, UpdatedAt: now})
	s.audit(user.ID, "login", user.ID.Hex(), "success")
	return LoginResult{User: user, SessionToken: token, CSRFToken: csrf, ExpiresAt: expires}, nil
}

func (Service) Authenticate(token string) (models.UserDB, models.AuthSessionDB, error) {
	if token == "" {
		return models.UserDB{}, models.AuthSessionDB{}, ErrUnauthorized
	}
	result := configs.DB.FindOne(bson.D{{Key: "token_hash", Value: hash(token)}}, models.AuthSessionDB{})
	if result.IsErr() {
		return models.UserDB{}, models.AuthSessionDB{}, ErrUnauthorized
	}
	session := models.InterfaceTo[models.AuthSessionDB](result.Value())
	now := time.Now().UTC()
	if !session.RevokedAt.IsZero() {
		return models.UserDB{}, session, ErrUnauthorized
	}
	settings, err := (Service{}).Settings()
	if err != nil {
		return models.UserDB{}, session, err
	}
	if now.After(session.CreatedAt.Add(time.Duration(settings.AbsoluteLifetimeMinutes)*time.Minute)) ||
		now.Sub(session.LastActivityAt) > time.Duration(settings.IdleTimeoutMinutes)*time.Minute {
		_ = configs.DB.PatchOne(bson.D{{Key: "_id", Value: session.ID}},
			models.AuthSessionPatch{RevokedAt: now, RevocationReason: "expired"})
		return models.UserDB{}, session, ErrUnauthorized
	}
	userResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: session.UserID}}, models.UserDB{})
	if userResult.IsErr() {
		return models.UserDB{}, session, ErrUnauthorized
	}
	user := models.InterfaceTo[models.UserDB](userResult.Value())
	if user.Status != models.AccountActive || !user.Role.Valid() {
		_ = configs.DB.PatchOne(bson.D{{Key: "_id", Value: session.ID}},
			models.AuthSessionPatch{RevokedAt: now, RevocationReason: "account_inactive"})
		return models.UserDB{}, session, ErrUnauthorized
	}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: session.ID}, {Key: "revoked_at", Value: nil}},
		models.AuthSessionPatch{LastActivityAt: now}); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
		return models.UserDB{}, session, ErrUnauthorized
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

func (s Service) CreateAccount(email string, role models.UserRole, actor models.DBID) error {
	_, err := s.CreateAccountWithProfile(email, "", "", role, actor)
	return err
}

func (Service) CreateAccountWithProfile(email, name, phone string, role models.UserRole, actor models.DBID) (ActionInstructions, error) {
	address, err := emailer.ParseAddress(email)
	if err != nil || !role.Valid() {
		return ActionInstructions{}, fmt.Errorf("invalid email or role")
	}
	existing := configs.DB.FindOne(bson.D{{Key: "email", Value: string(address)}}, models.UserDB{})
	if existing.IsOk() {
		return ActionInstructions{}, ErrConflict
	}
	if existing.Error() != nil && !errors.Is(existing.Error(), configs.DBErr.NotFound()) {
		return ActionInstructions{}, existing.Error()
	}
	now := time.Now().UTC()
	user := models.UserDB{Name: strings.TrimSpace(name), PhoneNumber: strings.TrimSpace(phone),
		Email: string(address), Role: role, Status: models.AccountPending,
		CreatedAt: now, UpdatedAt: now, CreatedBy: actor}
	inserted, err := insertModel(&user)
	if err != nil {
		return ActionInstructions{}, err
	}
	user.ID = inserted
	instructions, err := Default.issueCode(user, models.CodeSetup)
	if err != nil {
		return ActionInstructions{}, err
	}
	Default.audit(actor, "account_created", user.ID.Hex(), "success")
	return instructions, nil
}

func (Service) ListAccounts() ([]models.UserDB, error) {
	result := configs.DB.FindAll(bson.D{}, models.UserDB{})
	if result.IsErr() {
		return nil, result.Error()
	}
	users := make([]models.UserDB, 0)
	for _, item := range result.Value() {
		user := models.InterfaceTo[models.UserDB](item)
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
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.UserDB{})
	if result.IsErr() {
		return ErrNotFound
	}
	user := models.InterfaceTo[models.UserDB](result.Value())
	if user.Role == models.RoleAdmin && role != models.RoleAdmin && user.Status == models.AccountActive {
		admins := configs.DB.FindAll(bson.D{{Key: "role", Value: models.RoleAdmin}, {Key: "status", Value: models.AccountActive}}, models.UserDB{})
		if admins.IsErr() || len(admins.Value()) <= 1 {
			return fmt.Errorf("cannot demote the last active administrator")
		}
	}
	now := time.Now().UTC()
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: target}},
		models.UserPatch{Role: role, UpdatedAt: now}); err != nil {
		return err
	}
	Default.audit(actor, "account_role_changed", target.Hex(), "success")
	return nil
}

func (Service) AdminResendSetup(actor, target models.DBID) (ActionInstructions, error) {
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.UserDB{})
	if result.IsErr() {
		return ActionInstructions{}, ErrNotFound
	}
	user := models.InterfaceTo[models.UserDB](result.Value())
	if user.Status != models.AccountPending {
		return ActionInstructions{}, ErrNotFound
	}
	instructions, err := Default.issueCode(user, models.CodeSetup)
	if err != nil {
		return ActionInstructions{}, err
	}
	Default.audit(actor, "admin_setup_resent", target.Hex(), "success")
	return instructions, nil
}

func (Service) UpdateAccountStatus(actor, target models.DBID, status models.AccountStatus) error {
	if status != models.AccountActive && status != models.AccountDisabled && status != models.AccountDeleted {
		return fmt.Errorf("invalid account status")
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.UserDB{})
	if result.IsErr() {
		return ErrNotFound
	}
	user := models.InterfaceTo[models.UserDB](result.Value())
	if user.Status == models.AccountDeleted && status != models.AccountDeleted {
		return fmt.Errorf("soft-deleted accounts cannot be reactivated")
	}
	if user.Role == models.RoleAdmin && user.Status == models.AccountActive && status != models.AccountActive {
		admins := configs.DB.FindAll(bson.D{{Key: "role", Value: models.RoleAdmin}, {Key: "status", Value: models.AccountActive}}, models.UserDB{})
		if admins.IsErr() || len(admins.Value()) <= 1 {
			return fmt.Errorf("cannot disable or delete the last active administrator")
		}
	}
	now := time.Now().UTC()
	patch := models.UserPatch{Status: status, UpdatedAt: now}
	if status == models.AccountDeleted {
		patch.DeletedAt = now
	}
	if status == models.AccountActive && user.Status != models.AccountActive {
		return fmt.Errorf("inactive accounts must complete account setup before activation")
	}
	filter := bson.D{{Key: "_id", Value: target}}
	var updateErr error
	if status == models.AccountDisabled || status == models.AccountDeleted {
		updateErr = configs.DB.PatchOneAndUnset(filter, patch, "password_hash")
	} else {
		updateErr = configs.DB.PatchOne(filter, patch)
	}
	if updateErr != nil {
		return updateErr
	}
	if status != models.AccountActive {
		_ = configs.DB.UpdateMany(bson.D{{Key: "user_id", Value: target}, {Key: "revoked_at", Value: nil}},
			models.AuthSessionPatch{RevokedAt: now, RevocationReason: "account_status_changed"})
		_ = configs.DB.PermanentDeleteMany(bson.D{{Key: "user_id", Value: target}, {Key: "used_at", Value: nil},
			{Key: "invalidated_at", Value: nil}}, models.AuthActionCodeDB{})
	}
	Default.audit(actor, "account_status_changed", target.Hex(), "success")
	return nil
}

func (Service) ReenableAccount(actor, target models.DBID) (ActionInstructions, error) {
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.UserDB{})
	if result.IsErr() {
		return ActionInstructions{}, ErrNotFound
	}
	user := models.InterfaceTo[models.UserDB](result.Value())
	if user.Status != models.AccountDisabled {
		return ActionInstructions{}, fmt.Errorf("only disabled accounts can be re-enabled")
	}
	now := time.Now().UTC()
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: target}}, models.UserPatch{Status: models.AccountPending, UpdatedAt: now}); err != nil {
		return ActionInstructions{}, err
	}
	user.Status = models.AccountPending
	instructions, err := Default.issueCode(user, models.CodeSetup)
	if err != nil {
		_ = configs.DB.PatchOne(bson.D{{Key: "_id", Value: target}}, models.UserPatch{Status: models.AccountDisabled, UpdatedAt: time.Now().UTC()})
		return ActionInstructions{}, err
	}
	Default.audit(actor, "account_reenabled_setup_required", target.Hex(), "success")
	return instructions, nil
}

func (Service) UpdateAccountEmails(actor, target models.DBID, personalEmail, institutionEmail string) error {
	address, err := emailer.ParseAddress(personalEmail)
	if err != nil {
		return fmt.Errorf("invalid personal email")
	}
	userResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.UserDB{})
	if userResult.IsErr() {
		return ErrNotFound
	}
	user := models.InterfaceTo[models.UserDB](userResult.Value())
	duplicate := configs.DB.FindOne(bson.D{{Key: "email", Value: string(address)}, {Key: "_id", Value: bson.D{{Key: "$ne", Value: target}}}}, models.UserDB{})
	if duplicate.IsOk() {
		return ErrConflict
	}
	if duplicate.IsErr() && !errors.Is(duplicate.Error(), configs.DBErr.NotFound()) {
		return duplicate.Error()
	}

	var companion models.CompanionDB
	var parsedInstitution emailer.Address
	if user.Role == models.RoleStaff {
		parsedInstitution, err = emailer.ParseAddress(strings.TrimSpace(institutionEmail))
		if err != nil {
			return fmt.Errorf("a valid institutional email is required for staff accounts")
		}
		companionResult := configs.DB.FindOne(bson.D{{Key: "user_id", Value: target}}, models.CompanionDB{})
		if companionResult.IsErr() {
			return ErrNotFound
		}
		companion = models.InterfaceTo[models.CompanionDB](companionResult.Value())
		conflicts := configs.DB.FindAll(bson.D{
			{Key: "institution_email", Value: bson.Regex{Pattern: "^" + regexp.QuoteMeta(string(parsedInstitution)) + "$", Options: "i"}},
			{Key: "user_id", Value: bson.D{{Key: "$ne", Value: target}}},
		}, models.CompanionDB{})
		if conflicts.IsOk() && len(conflicts.Value()) > 0 {
			return ErrConflict
		}
		if conflicts.IsErr() && !errors.Is(conflicts.Error(), configs.DBErr.NotFound()) {
			return conflicts.Error()
		}

		personalAsInstitutional := configs.DB.FindOne(bson.D{
			{Key: "institution_email", Value: bson.Regex{Pattern: "^" + regexp.QuoteMeta(string(address)) + "$", Options: "i"}},
			{Key: "user_id", Value: bson.D{{Key: "$ne", Value: target}}},
		}, models.CompanionDB{})
		if personalAsInstitutional.IsOk() {
			return ErrConflict
		}
		if personalAsInstitutional.IsErr() && !errors.Is(personalAsInstitutional.Error(), configs.DBErr.NotFound()) {
			return personalAsInstitutional.Error()
		}

		institutionalAsPersonal := configs.DB.FindOne(bson.D{
			{Key: "email", Value: string(parsedInstitution)},
			{Key: "_id", Value: bson.D{{Key: "$ne", Value: target}}},
		}, models.UserDB{})
		if institutionalAsPersonal.IsOk() {
			return ErrConflict
		}
		if institutionalAsPersonal.IsErr() && !errors.Is(institutionalAsPersonal.Error(), configs.DBErr.NotFound()) {
			return institutionalAsPersonal.Error()
		}
	}

	now := time.Now().UTC()
	if user.Email != string(address) {
		if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: target}}, models.UserPatch{Email: string(address), UpdatedAt: now}); err != nil {
			return err
		}
	}
	if user.Role == models.RoleStaff {
		if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: companion.ID}},
			models.CompanionInstitutionEmailPatch{InstitutionEmail: string(parsedInstitution), UpdatedAt: now}); err != nil {
			return err
		}
	}
	Default.audit(actor, "account_emails_updated", target.Hex(), "success")
	return nil
}

func (Service) AdminRemoveActionCode(actor, target models.DBID) error {
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.UserDB{})
	if result.IsErr() {
		return ErrNotFound
	}
	now := time.Now().UTC()
	if err := configs.DB.PermanentDeleteMany(bson.D{
		{Key: "user_id", Value: target}, {Key: "used_at", Value: nil},
		{Key: "invalidated_at", Value: nil}, {Key: "expires_at", Value: bson.D{{Key: "$gt", Value: now}}},
	}, models.AuthActionCodeDB{}); err != nil && !errors.Is(err, configs.DBErr.NotFound()) {
		return err
	}
	Default.audit(actor, "admin_action_code_removed", target.Hex(), "success")
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
	result := configs.DB.FindOne(bson.D{{Key: "email", Value: string(address)}}, models.UserDB{})
	if result.IsErr() {
		return nil
	}
	user := models.InterfaceTo[models.UserDB](result.Value())
	if user.Status != models.AccountActive {
		return nil
	}
	_, err = Default.issueCode(user, models.CodeRecovery)
	return err
}

func (Service) CompleteRecovery(email, code, password string) error {
	// The self-service setting controls code issuance, not completion of a
	// valid code already issued by an administrator.
	return Default.completeCode(email, code, models.CodeRecovery, password)
}

func findUserByEmail(email string) (models.UserDB, error) {
	address, err := emailer.ParseAddress(email)
	if err != nil {
		return models.UserDB{}, ErrInvalidCredentials
	}
	primary := configs.DB.FindOne(bson.D{{Key: "email", Value: string(address)}}, models.UserDB{})
	var matched models.UserDB
	if primary.IsOk() {
		matched = models.InterfaceTo[models.UserDB](primary.Value())
	} else if primary.Error() != nil && !errors.Is(primary.Error(), configs.DBErr.NotFound()) {
		return models.UserDB{}, primary.Error()
	}
	companions := configs.DB.FindAll(bson.D{{Key: "institution_email", Value: bson.Regex{Pattern: "^" + regexp.QuoteMeta(string(address)) + "$", Options: "i"}}}, models.CompanionDB{})
	if companions.IsErr() && !errors.Is(companions.Error(), configs.DBErr.NotFound()) {
		return models.UserDB{}, companions.Error()
	}
	if companions.IsOk() {
		for _, raw := range companions.Value() {
			companion := models.InterfaceTo[models.CompanionDB](raw)
			result := configs.DB.FindOne(bson.D{{Key: "_id", Value: companion.UserID}}, models.UserDB{})
			if result.IsErr() {
				continue
			}
			user := models.InterfaceTo[models.UserDB](result.Value())
			if user.Status == models.AccountDeleted {
				continue
			}
			if !matched.ID.IsZero() && matched.ID != user.ID {
				return models.UserDB{}, ErrInvalidCredentials
			}
			matched = user
		}
	}
	if matched.ID.IsZero() {
		return models.UserDB{}, ErrInvalidCredentials
	}
	return matched, nil
}

func canRecoverAccount(status models.AccountStatus) bool {
	return status == models.AccountActive || status == models.AccountPending
}

func (Service) AdminIssueRecovery(actor, target models.DBID) (ActionInstructions, error) {
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: target}}, models.UserDB{})
	if result.IsErr() {
		return ActionInstructions{}, ErrNotFound
	}
	user := models.InterfaceTo[models.UserDB](result.Value())
	if !canRecoverAccount(user.Status) {
		return ActionInstructions{}, ErrNotFound
	}
	instructions, err := Default.issueCode(user, models.CodeRecovery)
	if err != nil {
		return ActionInstructions{}, err
	}
	Default.audit(actor, "admin_recovery_issued", target.Hex(), "success")
	return instructions, nil
}

func (Service) issueCode(user models.UserDB, purpose models.ActionCodePurpose) (ActionInstructions, error) {
	settings, err := Default.Settings()
	if err != nil {
		return ActionInstructions{}, err
	}
	lifetime := settings.SetupCodeLifetimeMinutes
	if purpose == models.CodeRecovery {
		lifetime = settings.RecoveryCodeLifetimeMinutes
	}

	devCodes := utils.GetEnvBool("FATV_DEV_SHOW_ACTION_CODES", false)
	if devCodes && !developmentEnvironment() {
		return ActionInstructions{}, errors.New("development action-code display is disabled outside development")
	}
	var mailer *emailer.Mailer
	if !devCodes {
		mailer, err = emailer.NewFromEnv()
		if err != nil {
			return ActionInstructions{}, err
		}
	}

	code, err := randomCode()
	if err != nil {
		return ActionInstructions{}, err
	}
	now := time.Now().UTC()
	// A user may have only one outstanding account-action instruction at a time.
	if err := configs.DB.PermanentDeleteMany(bson.D{
		{Key: "user_id", Value: user.ID}, {Key: "used_at", Value: nil},
		{Key: "invalidated_at", Value: nil}, {Key: "expires_at", Value: bson.D{{Key: "$gt", Value: now}}},
	}, models.AuthActionCodeDB{}); err != nil && !errors.Is(err, configs.DBErr.NotFound()) {
		return ActionInstructions{}, err
	}
	if purpose == models.CodeSetup {
		_ = configs.DB.UpdateMany(bson.D{{Key: "user_id", Value: user.ID}, {Key: "purpose", Value: models.CodeSetupLink},
			{Key: "used_at", Value: nil}, {Key: "invalidated_at", Value: nil}},
			models.AuthActionCodePatch{InvalidatedAt: now})
	}
	if purpose == models.CodeRecovery && user.Status == models.AccountPending {
		for _, pendingPurpose := range []models.ActionCodePurpose{models.CodeSetup, models.CodeSetupLink} {
			_ = configs.DB.UpdateMany(bson.D{{Key: "user_id", Value: user.ID}, {Key: "purpose", Value: pendingPurpose},
				{Key: "used_at", Value: nil}, {Key: "invalidated_at", Value: nil}},
				models.AuthActionCodePatch{InvalidatedAt: now})
		}
	}

	record := models.AuthActionCodeDB{UserID: user.ID, Purpose: purpose, CodeHash: hash(code),
		CreatedAt: now, ExpiresAt: now.Add(time.Duration(lifetime) * time.Minute)}
	if devCodes {
		record.CodePlaintext = code
	}
	if _, err := insertModel(&record); err != nil {
		return ActionInstructions{}, err
	}

	instructions := ActionInstructions{}
	if devCodes {
		instructions.Code = code
	}
	if purpose == models.CodeSetup {
		token, tokenErr := randomToken(32)
		if tokenErr != nil {
			return ActionInstructions{}, tokenErr
		}
		linkRecord := models.AuthActionCodeDB{
			UserID: user.ID, Purpose: models.CodeSetupLink, CodeHash: hash(token),
			CreatedAt: now, ExpiresAt: now.Add(time.Duration(lifetime) * time.Minute),
		}
		if _, err := insertModel(&linkRecord); err != nil {
			return ActionInstructions{}, err
		}
		setupURL, urlErr := configs.Frontend.URL("/setup-account#token=" + token)
		if urlErr != nil {
			return ActionInstructions{}, urlErr
		}
		instructions.Link = setupURL
	}

	if !devCodes {
		var sendErr error
		if purpose == models.CodeSetup {
			sendErr = mailer.SendSetupCode(emailer.Address(user.Email), code, instructions.Link, lifetime)
		} else {
			sendErr = mailer.SendRecoveryCode(emailer.Address(user.Email), code, lifetime)
		}
		if sendErr != nil {
			_ = configs.DB.PatchOne(bson.D{{Key: "user_id", Value: user.ID}, {Key: "purpose", Value: purpose},
				{Key: "code_hash", Value: hash(code)}}, models.AuthActionCodePatch{InvalidatedAt: time.Now().UTC()})
			_ = configs.DB.UpdateMany(bson.D{{Key: "user_id", Value: user.ID}, {Key: "purpose", Value: models.CodeSetupLink},
				{Key: "used_at", Value: nil}, {Key: "invalidated_at", Value: nil}},
				models.AuthActionCodePatch{InvalidatedAt: time.Now().UTC()})
			logger.Error("Email delivery failed for account action code; code was not logged:", sendErr)
			return ActionInstructions{}, sendErr
		}
		logger.Info("Account action code email sent", "user_id", user.ID.Hex(), "purpose", purpose)
	}
	return instructions, nil
}

func (Service) DevelopmentSetupCode(email string) (string, error) {
	if !utils.GetEnvBool("FATV_DEV_SHOW_ACTION_CODES", false) || !developmentEnvironment() {
		return "", ErrForbidden
	}
	user, err := findUserByEmail(email)
	if err != nil {
		return "", ErrInvalidCode
	}
	if user.Status != models.AccountPending {
		return "", ErrInvalidCode
	}
	now := time.Now().UTC()
	result := configs.DB.FindOne(bson.D{
		{Key: "user_id", Value: user.ID}, {Key: "purpose", Value: models.CodeSetup},
		{Key: "used_at", Value: nil}, {Key: "invalidated_at", Value: nil},
		{Key: "expires_at", Value: bson.D{{Key: "$gt", Value: now}}},
	}, models.AuthActionCodeDB{})
	if result.IsErr() {
		return "", ErrInvalidCode
	}
	action := models.InterfaceTo[models.AuthActionCodeDB](result.Value())
	if action.CodePlaintext == "" {
		return "", ErrInvalidCode
	}
	return action.CodePlaintext, nil
}

func (Service) CompleteSetupLink(token, password string) (LoginResult, error) {
	if len(password) < 12 || len(password) > 72 || len(token) < 32 {
		return LoginResult{}, ErrInvalidCode
	}
	result := configs.DB.FindOne(bson.D{
		{Key: "purpose", Value: models.CodeSetupLink},
		{Key: "code_hash", Value: hash(strings.TrimSpace(token))},
		{Key: "used_at", Value: nil}, {Key: "invalidated_at", Value: nil},
		{Key: "expires_at", Value: bson.D{{Key: "$gt", Value: time.Now().UTC()}}},
	}, models.AuthActionCodeDB{})
	if result.IsErr() {
		return LoginResult{}, ErrInvalidCode
	}
	action := models.InterfaceTo[models.AuthActionCodeDB](result.Value())
	userResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: action.UserID}}, models.UserDB{})
	if userResult.IsErr() {
		return LoginResult{}, ErrInvalidCode
	}
	user := models.InterfaceTo[models.UserDB](userResult.Value())
	if user.Status != models.AccountPending {
		return LoginResult{}, ErrInvalidCode
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return LoginResult{}, err
	}
	now := time.Now().UTC()
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: action.ID}, {Key: "used_at", Value: nil},
		{Key: "invalidated_at", Value: nil}, {Key: "expires_at", Value: bson.D{{Key: "$gt", Value: now}}}},
		models.AuthActionCodePatch{UsedAt: now}); err != nil {
		return LoginResult{}, ErrInvalidCode
	}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: user.ID}, {Key: "status", Value: models.AccountPending}},
		models.UserPatch{PasswordHash: passwordHash, Status: models.AccountActive, EmailVerifiedAt: now, UpdatedAt: now}); err != nil {
		return LoginResult{}, err
	}
	_ = configs.DB.UpdateMany(bson.D{{Key: "user_id", Value: user.ID}, {Key: "used_at", Value: nil},
		{Key: "invalidated_at", Value: nil}}, models.AuthActionCodePatch{InvalidatedAt: now})
	if err := configs.DB.PermanentDeleteOne(bson.D{{Key: "_id", Value: action.ID}}, models.AuthActionCodeDB{}); err != nil {
		logger.Warning("Could not delete consumed setup-link code:", err)
	}
	Default.audit(user.ID, "setup_link_completed", user.ID.Hex(), "success")
	login, err := Default.Login(user.Email, password)
	if err != nil {
		return LoginResult{}, err
	}
	return login, nil
}

func developmentEnvironment() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "dev", "debug", "test":
		return true
	default:
		return false
	}
}

func (Service) completeCode(email, code string, purpose models.ActionCodePurpose, password string) error {
	if len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("password must be between 12 and 72 bytes")
	}
	user, err := findUserByEmail(email)
	if err != nil {
		return ErrInvalidCode
	}
	if purpose == models.CodeSetup && user.Status != models.AccountPending {
		return ErrInvalidCode
	}
	if purpose == models.CodeRecovery && !canRecoverAccount(user.Status) {
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
	passwordHash, err := hashPassword(password)
	if err != nil {
		return err
	}
	status := user.Status
	if purpose == models.CodeSetup || (purpose == models.CodeRecovery && user.Status == models.AccountPending) {
		status = models.AccountActive
	}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: user.ID}},
		models.UserPatch{PasswordHash: passwordHash, Status: status, EmailVerifiedAt: now, UpdatedAt: now}); err != nil {
		return err
	}
	_ = configs.DB.UpdateMany(bson.D{{Key: "user_id", Value: user.ID}, {Key: "revoked_at", Value: nil}},
		models.AuthSessionPatch{RevokedAt: now, RevocationReason: "password_changed"})
	if err := configs.DB.PermanentDeleteOne(bson.D{{Key: "_id", Value: action.ID}}, models.AuthActionCodeDB{}); err != nil {
		logger.Warning("Could not delete consumed account action code:", err)
	}
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
	pepper := strings.TrimSpace(os.Getenv("FATV_AUTH_PEPPER"))
	if pepper == "" {
		// Compatibility for existing dev data; production validation rejects missing pepper.
		sum := sha256.Sum256([]byte(value))
		return hex.EncodeToString(sum[:])
	}
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// passwordInput derives a fixed-size bcrypt input using the server-side pepper.
// The mutable input buffer is cleared by callers immediately after bcrypt returns.
func hashPassword(password string) (string, error) {
	input, err := passwordInput(password)
	if err != nil {
		return "", err
	}
	var hashed []byte
	secretscope.Do(func() { hashed, err = bcrypt.GenerateFromPassword(input, bcrypt.DefaultCost) })
	clearSecret(input)
	return string(hashed), err
}

func passwordInput(password string) ([]byte, error) {
	pepper := strings.TrimSpace(os.Getenv("FATV_AUTH_PEPPER"))
	if pepper == "" {
		return nil, errors.New("FATV_AUTH_PEPPER is required")
	}
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(password))
	return mac.Sum(nil), nil
}

func clearSecret(value []byte) {
	clear(value)
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

func (Service) ValidateDevelopmentOptions() error {
	bypass := utils.GetEnvBool("FATV_DEV_AUTH_BYPASS", false)
	showCodes := utils.GetEnvBool("FATV_DEV_SHOW_ACTION_CODES", false)
	if (bypass || showCodes) && !developmentEnvironment() {
		return errors.New("FATV_DEV_AUTH_BYPASS and FATV_DEV_SHOW_ACTION_CODES are only allowed in APP_ENV=dev, debug, or test")
	}
	return nil
}

func (Service) BootstrapAdminFromEnv() error {
	email := strings.TrimSpace(os.Getenv("FATV_BOOTSTRAP_ADMIN_EMAIL"))
	password := os.Getenv("FATV_BOOTSTRAP_ADMIN_PASSWORD")
	if email == "" {
		logger.Warning("No FATV_BOOTSTRAP_ADMIN_EMAIL configured; no bootstrap administrator was created")
		return nil
	}
	existing := configs.DB.FindAll(bson.D{}, models.UserDB{})
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
		passwordHash, err := hashPassword(password)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		user := models.UserDB{
			Email: string(address), PasswordHash: passwordHash, Role: models.RoleAdmin,
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

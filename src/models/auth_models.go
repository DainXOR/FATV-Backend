package models

import "time"

type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleStaff UserRole = "staff"
)

func (r UserRole) Valid() bool { return r == RoleAdmin || r == RoleStaff }

type AccountStatus string

const (
	AccountPending  AccountStatus = "pending"
	AccountActive   AccountStatus = "active"
	AccountDisabled AccountStatus = "disabled"
	AccountDeleted  AccountStatus = "deleted"
)

func (s AccountStatus) Valid() bool {
	return s == AccountPending || s == AccountActive || s == AccountDisabled || s == AccountDeleted
}

type ActionCodePurpose string

const (
	CodeSetup    ActionCodePurpose = "setup"
	CodeRecovery ActionCodePurpose = "password_recovery"
)

func (p ActionCodePurpose) Valid() bool { return p == CodeSetup || p == CodeRecovery }

type AuthUserDB struct {
	ID              DBID          `json:"id" bson:"_id,omitempty"`
	Email           string        `json:"email" bson:"email"`
	PasswordHash    string        `json:"-" bson:"password_hash,omitempty"`
	Role            UserRole      `json:"role" bson:"role"`
	Status          AccountStatus `json:"status" bson:"status"`
	EmailVerifiedAt time.Time     `json:"email_verified_at,omitempty" bson:"email_verified_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at" bson:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at" bson:"updated_at"`
	DeletedAt       time.Time     `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
	CreatedBy       DBID          `json:"created_by,omitempty" bson:"created_by,omitempty"`
	LastLoginAt     time.Time     `json:"last_login_at,omitempty" bson:"last_login_at,omitempty"`
}

func (AuthUserDB) TableName() string { return "auth_users" }
func (u AuthUserDB) IsZero() bool    { return u.ID.IsZero() && u.Email == "" }

type AuthSessionDB struct {
	ID                DBID      `json:"id" bson:"_id,omitempty"`
	UserID            DBID      `json:"user_id" bson:"user_id"`
	TokenHash         string    `json:"-" bson:"token_hash"`
	CreatedAt         time.Time `json:"created_at" bson:"created_at"`
	LastActivityAt    time.Time `json:"last_activity_at" bson:"last_activity_at"`
	AbsoluteExpiresAt time.Time `json:"absolute_expires_at" bson:"absolute_expires_at"`
	RevokedAt         time.Time `json:"revoked_at,omitempty" bson:"revoked_at,omitempty"`
	RevocationReason  string    `json:"-" bson:"revocation_reason,omitempty"`
}

func (AuthSessionDB) TableName() string { return "auth_sessions" }
func (s AuthSessionDB) IsZero() bool    { return s.ID.IsZero() && s.TokenHash == "" }

type AuthSessionPatch struct {
	LastActivityAt   time.Time `bson:"last_activity_at,omitempty"`
	RevokedAt        time.Time `bson:"revoked_at,omitempty"`
	RevocationReason string    `bson:"revocation_reason,omitempty"`
}

func (AuthSessionPatch) TableName() string { return "auth_sessions" }
func (AuthSessionPatch) IsZero() bool      { return false }

type AuthSettingsDB struct {
	ID                          string    `json:"id" bson:"_id"`
	IdleTimeoutMinutes          int       `json:"idle_timeout_minutes" bson:"idle_timeout_minutes"`
	AbsoluteLifetimeMinutes     int       `json:"absolute_session_lifetime_minutes" bson:"absolute_session_lifetime_minutes"`
	SelfServiceRecoveryEnabled  bool      `json:"self_service_recovery_enabled" bson:"self_service_recovery_enabled"`
	SetupCodeLifetimeMinutes    int       `json:"setup_code_lifetime_minutes" bson:"setup_code_lifetime_minutes"`
	RecoveryCodeLifetimeMinutes int       `json:"recovery_code_lifetime_minutes" bson:"recovery_code_lifetime_minutes"`
	UpdatedAt                   time.Time `json:"updated_at" bson:"updated_at"`
	UpdatedBy                   DBID      `json:"updated_by,omitempty" bson:"updated_by,omitempty"`
}

func (AuthSettingsDB) TableName() string { return "auth_settings" }
func (s AuthSettingsDB) IsZero() bool    { return s.ID == "" }

type AuthActionCodeDB struct {
	ID            DBID              `json:"id" bson:"_id,omitempty"`
	UserID        DBID              `json:"user_id" bson:"user_id"`
	Purpose       ActionCodePurpose `json:"purpose" bson:"purpose"`
	CodeHash      string            `json:"-" bson:"code_hash"`
	CreatedAt     time.Time         `json:"created_at" bson:"created_at"`
	ExpiresAt     time.Time         `json:"expires_at" bson:"expires_at"`
	UsedAt        time.Time         `json:"used_at,omitempty" bson:"used_at,omitempty"`
	InvalidatedAt time.Time         `json:"invalidated_at,omitempty" bson:"invalidated_at,omitempty"`
	AttemptCount  int               `json:"-" bson:"attempt_count"`
}

func (AuthActionCodeDB) TableName() string { return "auth_action_codes" }
func (c AuthActionCodeDB) IsZero() bool    { return c.ID.IsZero() && c.CodeHash == "" }

type FormInvitationState string

const (
	InvitationStateOpen      FormInvitationState = "open"
	InvitationStateSubmitted FormInvitationState = "submitted"
)

func (s FormInvitationState) Valid() bool {
	return s == InvitationStateOpen || s == InvitationStateSubmitted
}

type FormInvitationDB struct {
	ID              DBID                `json:"id" bson:"_id,omitempty"`
	FormID          DBID                `json:"form_id" bson:"form_id"`
	StudentID       DBID                `json:"student_id" bson:"student_id"`
	TokenHash       string              `json:"-" bson:"token_hash"`
	ExpiresAt       time.Time           `json:"expires_at" bson:"expires_at"`
	CreatedBy       DBID                `json:"created_by" bson:"created_by"`
	CreatedAt       time.Time           `json:"created_at" bson:"created_at"`
	SubmissionState FormInvitationState `json:"submission_state" bson:"submission_state,omitempty"`
	RevokedAt       time.Time           `json:"revoked_at,omitempty" bson:"revoked_at,omitempty"`
	SubmittedAt     time.Time           `json:"submitted_at,omitempty" bson:"submitted_at,omitempty"`
}

func (FormInvitationDB) TableName() string { return "form_invitations" }
func (i FormInvitationDB) IsZero() bool    { return i.ID.IsZero() && i.TokenHash == "" }

type FormInvitationPatch struct {
	RevokedAt       time.Time           `bson:"revoked_at,omitempty"`
	SubmittedAt     time.Time           `bson:"submitted_at,omitempty"`
	SubmissionState FormInvitationState `bson:"submission_state,omitempty"`
}

func (FormInvitationPatch) TableName() string { return "form_invitations" }
func (FormInvitationPatch) IsZero() bool      { return false }

type FormInvitationRollbackPatch struct {
	SubmittedAt     time.Time           `bson:"submitted_at"`
	SubmissionState FormInvitationState `bson:"submission_state"`
}

func (FormInvitationRollbackPatch) TableName() string { return "form_invitations" }
func (FormInvitationRollbackPatch) IsZero() bool      { return false }

type AuthAuditDB struct {
	ID       DBID      `json:"id" bson:"_id,omitempty"`
	At       time.Time `json:"at" bson:"at"`
	ActorID  DBID      `json:"actor_id,omitempty" bson:"actor_id,omitempty"`
	Event    string    `json:"event" bson:"event"`
	TargetID string    `json:"target_id,omitempty" bson:"target_id,omitempty"`
	Outcome  string    `json:"outcome" bson:"outcome"`
}

func (AuthAuditDB) TableName() string { return "auth_audit_logs" }
func (a AuthAuditDB) IsZero() bool    { return a.ID.IsZero() }

type AuthUserPatch struct {
	PasswordHash    string        `bson:"password_hash,omitempty"`
	Role            UserRole      `bson:"role,omitempty"`
	Status          AccountStatus `bson:"status,omitempty"`
	EmailVerifiedAt time.Time     `bson:"email_verified_at,omitempty"`
	UpdatedAt       time.Time     `bson:"updated_at,omitempty"`
	DeletedAt       time.Time     `bson:"deleted_at,omitempty"`
	LastLoginAt     time.Time     `bson:"last_login_at,omitempty"`
}

func (AuthUserPatch) TableName() string { return "auth_users" }
func (AuthUserPatch) IsZero() bool      { return false }

type AuthActionCodePatch struct {
	UsedAt        time.Time `bson:"used_at,omitempty"`
	InvalidatedAt time.Time `bson:"invalidated_at,omitempty"`
	AttemptCount  int       `bson:"attempt_count"`
}

func (AuthActionCodePatch) TableName() string { return "auth_action_codes" }
func (AuthActionCodePatch) IsZero() bool      { return false }

type AuthSettingsPatch struct {
	IdleTimeoutMinutes          int       `bson:"idle_timeout_minutes"`
	AbsoluteLifetimeMinutes     int       `bson:"absolute_session_lifetime_minutes"`
	SelfServiceRecoveryEnabled  bool      `bson:"self_service_recovery_enabled"`
	SetupCodeLifetimeMinutes    int       `bson:"setup_code_lifetime_minutes"`
	RecoveryCodeLifetimeMinutes int       `bson:"recovery_code_lifetime_minutes"`
	UpdatedAt                   time.Time `bson:"updated_at"`
	UpdatedBy                   DBID      `bson:"updated_by,omitempty"`
}

func (AuthSettingsPatch) TableName() string { return "auth_settings" }
func (AuthSettingsPatch) IsZero() bool      { return false }

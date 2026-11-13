package models

type ActionCodePurpose string

const (
	CodeSetup     ActionCodePurpose = "setup"
	CodeSetupLink ActionCodePurpose = "setup_link"
	CodeRecovery  ActionCodePurpose = "password_recovery"
)

func (p ActionCodePurpose) Valid() bool {
	return p == CodeSetup || p == CodeSetupLink || p == CodeRecovery
}

type AuthSessionDB struct {
	ID                DBID       `json:"id" bson:"_id,omitempty"`
	UserID            DBID       `json:"user_id" bson:"user_id"`
	TokenHash         string     `json:"-" bson:"token_hash"`
	CreatedAt         DBDateTime `json:"created_at" bson:"created_at"`
	LastActivityAt    DBDateTime `json:"last_activity_at" bson:"last_activity_at"`
	AbsoluteExpiresAt DBDateTime `json:"absolute_expires_at" bson:"absolute_expires_at"`
	RevokedAt         DBDateTime `json:"revoked_at,omitempty" bson:"revoked_at,omitempty"`
	RevocationReason  string     `json:"-" bson:"revocation_reason,omitempty"`
}

func (AuthSessionDB) TableName() string { return "auth_sessions" }
func (s AuthSessionDB) IsZero() bool    { return s.ID.IsZero() && s.TokenHash == "" }

type AuthSessionPatch struct {
	LastActivityAt   DBDateTime `bson:"last_activity_at,omitempty"`
	RevokedAt        DBDateTime `bson:"revoked_at,omitempty"`
	RevocationReason string     `bson:"revocation_reason,omitempty"`
}

func (AuthSessionPatch) TableName() string { return "auth_sessions" }
func (AuthSessionPatch) IsZero() bool      { return false }

type AuthSettingsDB struct {
	ID                          string     `json:"id" bson:"_id"`
	IdleTimeoutMinutes          int        `json:"idle_timeout_minutes" bson:"idle_timeout_minutes"`
	AbsoluteLifetimeMinutes     int        `json:"absolute_session_lifetime_minutes" bson:"absolute_session_lifetime_minutes"`
	SelfServiceRecoveryEnabled  bool       `json:"self_service_recovery_enabled" bson:"self_service_recovery_enabled"`
	SetupCodeLifetimeMinutes    int        `json:"setup_code_lifetime_minutes" bson:"setup_code_lifetime_minutes"`
	RecoveryCodeLifetimeMinutes int        `json:"recovery_code_lifetime_minutes" bson:"recovery_code_lifetime_minutes"`
	UpdatedAt                   DBDateTime `json:"updated_at" bson:"updated_at"`
	UpdatedBy                   DBID       `json:"updated_by,omitempty" bson:"updated_by,omitempty"`
}

func (AuthSettingsDB) TableName() string { return "auth_settings" }
func (s AuthSettingsDB) IsZero() bool    { return s.ID == "" }

type AuthActionCodeDB struct {
	ID             DBID              `json:"id" bson:"_id,omitempty"`
	UserID         DBID              `json:"user_id" bson:"user_id"`
	Purpose        ActionCodePurpose `json:"purpose" bson:"purpose"`
	CodeHash       string            `json:"-" bson:"code_hash"`
	CodeCiphertext string            `json:"-" bson:"code_ciphertext,omitempty"` // retained only for legacy rows
	CodePlaintext  string            `json:"-" bson:"code_plaintext,omitempty"`
	CreatedAt      DBDateTime        `json:"created_at" bson:"created_at"`
	ExpiresAt      DBDateTime        `json:"expires_at" bson:"expires_at"`
	UsedAt         DBDateTime        `json:"used_at,omitempty" bson:"used_at,omitempty"`
	InvalidatedAt  DBDateTime        `json:"invalidated_at,omitempty" bson:"invalidated_at,omitempty"`
	AttemptCount   int               `json:"-" bson:"attempt_count"`
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
	ExpiresAt       DBDateTime          `json:"expires_at" bson:"expires_at"`
	CreatedBy       DBID                `json:"created_by" bson:"created_by"`
	CreatedAt       DBDateTime          `json:"created_at" bson:"created_at"`
	SubmissionState FormInvitationState `json:"submission_state" bson:"submission_state,omitempty"`
	RevokedAt       DBDateTime          `json:"revoked_at,omitempty" bson:"revoked_at,omitempty"`
	SubmittedAt     DBDateTime          `json:"submitted_at,omitempty" bson:"submitted_at,omitempty"`
}

func (FormInvitationDB) TableName() string { return "form_invitations" }
func (i FormInvitationDB) IsZero() bool    { return i.ID.IsZero() && i.TokenHash == "" }

type FormInvitationPatch struct {
	RevokedAt       DBDateTime          `bson:"revoked_at,omitempty"`
	SubmittedAt     DBDateTime          `bson:"submitted_at,omitempty"`
	SubmissionState FormInvitationState `bson:"submission_state,omitempty"`
}

func (FormInvitationPatch) TableName() string { return "form_invitations" }
func (FormInvitationPatch) IsZero() bool      { return false }

type FormInvitationRollbackPatch struct {
	SubmittedAt     DBDateTime          `bson:"submitted_at"`
	SubmissionState FormInvitationState `bson:"submission_state"`
}

func (FormInvitationRollbackPatch) TableName() string { return "form_invitations" }
func (FormInvitationRollbackPatch) IsZero() bool      { return false }

type AuthAuditDB struct {
	ID       DBID       `json:"id" bson:"_id,omitempty"`
	At       DBDateTime `json:"at" bson:"at"`
	ActorID  DBID       `json:"actor_id,omitempty" bson:"actor_id,omitempty"`
	Event    string     `json:"event" bson:"event"`
	TargetID string     `json:"target_id,omitempty" bson:"target_id,omitempty"`
	Outcome  string     `json:"outcome" bson:"outcome"`
}

func (AuthAuditDB) TableName() string { return "auth_audit_logs" }
func (a AuthAuditDB) IsZero() bool    { return a.ID.IsZero() }

type AuthActionCodePatch struct {
	UsedAt         DBDateTime `bson:"used_at,omitempty"`
	InvalidatedAt  DBDateTime `bson:"invalidated_at,omitempty"`
	AttemptCount   int        `bson:"attempt_count"`
	CodeCiphertext string     `bson:"code_ciphertext,omitempty"`
}

func (AuthActionCodePatch) TableName() string { return "auth_action_codes" }
func (AuthActionCodePatch) IsZero() bool      { return false }

type AuthSettingsPatch struct {
	IdleTimeoutMinutes          int        `bson:"idle_timeout_minutes"`
	AbsoluteLifetimeMinutes     int        `bson:"absolute_session_lifetime_minutes"`
	SelfServiceRecoveryEnabled  bool       `bson:"self_service_recovery_enabled"`
	SetupCodeLifetimeMinutes    int        `bson:"setup_code_lifetime_minutes"`
	RecoveryCodeLifetimeMinutes int        `bson:"recovery_code_lifetime_minutes"`
	UpdatedAt                   DBDateTime `bson:"updated_at"`
	UpdatedBy                   DBID       `bson:"updated_by,omitempty"`
}

func (AuthSettingsPatch) TableName() string { return "auth_settings" }
func (AuthSettingsPatch) IsZero() bool      { return false }

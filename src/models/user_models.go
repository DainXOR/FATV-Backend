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

type UserDB struct {
	ID              DBID          `json:"id" bson:"_id,omitempty"`
	Name            string        `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber     string        `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
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

func (UserDB) TableName() string { return "users" }
func (u UserDB) IsZero() bool    { return u.ID.IsZero() && u.Email == "" }

type UserPatch struct {
	Name            string        `bson:"name,omitempty"`
	PhoneNumber     string        `bson:"phone_number,omitempty"`
	PasswordHash    string        `bson:"password_hash,omitempty"`
	Role            UserRole      `bson:"role,omitempty"`
	Status          AccountStatus `bson:"status,omitempty"`
	EmailVerifiedAt time.Time     `bson:"email_verified_at,omitempty"`
	UpdatedAt       time.Time     `bson:"updated_at,omitempty"`
	DeletedAt       time.Time     `bson:"deleted_at,omitempty"`
	LastLoginAt     time.Time     `bson:"last_login_at,omitempty"`
}

func (UserPatch) TableName() string { return "users" }
func (UserPatch) IsZero() bool      { return false }

// LegacyAuthUserDB is only used to migrate the former auth_users collection.
type LegacyAuthUserDB struct {
	ID              DBID          `json:"id" bson:"_id,omitempty"`
	Name            string        `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumber     string        `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
	Email           string        `json:"email" bson:"email"`
	PasswordHash    string        `json:"-" bson:"password_hash,omitempty"`
	Role            UserRole      `json:"role" bson:"role"`
	ProcessIDs      []DBID        `json:"process_ids,omitempty" bson:"process_ids,omitempty"`
	Status          AccountStatus `json:"status" bson:"status"`
	EmailVerifiedAt time.Time     `json:"email_verified_at,omitempty" bson:"email_verified_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at" bson:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at" bson:"updated_at"`
	DeletedAt       time.Time     `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
	CreatedBy       DBID          `json:"created_by,omitempty" bson:"created_by,omitempty"`
	LastLoginAt     time.Time     `json:"last_login_at,omitempty" bson:"last_login_at,omitempty"`
}

func (LegacyAuthUserDB) TableName() string { return "auth_users" }
func (u LegacyAuthUserDB) IsZero() bool    { return u.ID.IsZero() && u.Email == "" }

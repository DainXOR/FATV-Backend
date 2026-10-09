package models

// LegacyCompanionDB is used only while migrating old companion documents.
type LegacyCompanionDB struct {
	ID               DBID       `bson:"_id,omitempty"`
	UserID           DBID       `bson:"user_id,omitempty"`
	NumberID         string     `bson:"number_id,omitempty"`
	FirstName        string     `bson:"first_name,omitempty"`
	LastName         string     `bson:"last_name,omitempty"`
	Email            string     `bson:"email,omitempty"`
	InstitutionEmail string     `bson:"institution_email,omitempty"`
	PhoneNumber      string     `bson:"phone_number,omitempty"`
	IDSpeciality     DBID       `bson:"id_speciality,omitempty"`
	ProcessID        DBID       `bson:"process_id,omitempty"`
	IDProcess        []DBID     `bson:"id_process,omitempty"`
	CreatedAt        DBDateTime `bson:"created_at,omitempty"`
	UpdatedAt        DBDateTime `bson:"updated_at,omitempty"`
	DeletedAt        DBDateTime `bson:"deleted_at,omitempty"`
}

func (LegacyCompanionDB) TableName() string { return "companions" }
func (c LegacyCompanionDB) IsZero() bool    { return c.ID.IsZero() }

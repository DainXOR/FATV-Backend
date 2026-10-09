package models

type CompanionAssignmentPatch struct {
	UserID           DBID       `bson:"user_id,omitempty"`
	InstitutionEmail string     `bson:"institution_email,omitempty"`
	IDSpeciality     DBID       `bson:"id_speciality,omitempty"`
	IDProcess        []DBID     `bson:"id_process,omitempty"`
	UpdatedAt        DBDateTime `bson:"updated_at,omitempty"`
}

func (CompanionAssignmentPatch) TableName() string { return "companions" }
func (CompanionAssignmentPatch) IsZero() bool      { return false }

type CompanionProcessIDsPatch struct {
	IDProcess []DBID     `bson:"id_process"`
	UpdatedAt DBDateTime `bson:"updated_at"`
}

func (CompanionProcessIDsPatch) TableName() string { return "companions" }
func (CompanionProcessIDsPatch) IsZero() bool      { return false }

package models

import (
	"errors"

	"dainxor/atv/logger"
	"dainxor/atv/types"
)

type CompanionDB struct {
	ID               DBID       `json:"id,omitempty" bson:"_id,omitempty"`
	UserID           DBID       `json:"user_id" bson:"user_id"`
	InstitutionEmail string     `json:"institution_email" bson:"institution_email"`
	IDSpeciality     DBID       `json:"id_speciality" bson:"id_speciality"`
	IDProcess        []DBID     `json:"id_process" bson:"id_process"`
	CreatedAt        DBDateTime `json:"created_at,omitzero" bson:"created_at,omitempty"`
	UpdatedAt        DBDateTime `json:"updated_at,omitzero" bson:"updated_at,omitempty"`
	DeletedAt        DBDateTime `json:"deleted_at" bson:"deleted_at"`
}

type CompanionCreate struct {
	UserID           string   `json:"user_id"`
	InstitutionEmail string   `json:"institution_email"`
	IDSpeciality     string   `json:"id_speciality"`
	IDProcess        []string `json:"id_process"`
}

type CompanionResponse struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	InstitutionEmail string     `json:"institution_email"`
	IDSpeciality     string     `json:"id_speciality"`
	IDProcess        []string   `json:"id_process"`
	CreatedAt        DBDateTime `json:"created_at"`
	UpdatedAt        DBDateTime `json:"updated_at"`
}

func (c CompanionCreate) ToInsert() types.Result[CompanionDB] {
	obj := CompanionDB{
		InstitutionEmail: c.InstitutionEmail,
		CreatedAt:        Time.Now(),
		UpdatedAt:        Time.Now(),
		DeletedAt:        Time.Zero(),
	}
	if !ID.Ensure(c.UserID, &obj.UserID, "UserID") {
		return types.ResultErr[CompanionDB](errors.New("Invalid UserID"))
	}
	if !ID.Ensure(c.IDSpeciality, &obj.IDSpeciality, "IDSpeciality") {
		logger.Lava(types.V("0.2.0"), "Using not standarized error")
		return types.ResultErr[CompanionDB](errors.New("Invalid IDSpeciality"))
	}
	for _, raw := range c.IDProcess {
		var id DBID
		if !ID.Ensure(raw, &id, "IDProcess") {
			return types.ResultErr[CompanionDB](errors.New("Invalid IDProcess"))
		}
		obj.IDProcess = append(obj.IDProcess, id)
	}
	return types.ResultOk(obj)
}

func (c CompanionCreate) ToUpdate() types.Result[CompanionDB] {
	obj := CompanionDB{InstitutionEmail: c.InstitutionEmail, UpdatedAt: Time.Now()}
	if !ID.OmitEmpty(c.UserID, &obj.UserID, "UserID") {
		return types.ResultErr[CompanionDB](errors.New("Invalid UserID"))
	}
	if !ID.OmitEmpty(c.IDSpeciality, &obj.IDSpeciality, "IDSpeciality") {
		return types.ResultErr[CompanionDB](errors.New("Invalid IDSpeciality"))
	}
	for _, raw := range c.IDProcess {
		var id DBID
		if !ID.Ensure(raw, &id, "IDProcess") {
			return types.ResultErr[CompanionDB](errors.New("Invalid IDProcess"))
		}
		obj.IDProcess = append(obj.IDProcess, id)
	}
	return types.ResultOk(obj)
}

func (c CompanionDB) ToResponse() CompanionResponse {
	processIDs := make([]string, 0, len(c.IDProcess))
	for _, id := range c.IDProcess {
		processIDs = append(processIDs, id.Hex())
	}
	return CompanionResponse{
		ID: c.ID.Hex(), UserID: c.UserID.Hex(), InstitutionEmail: c.InstitutionEmail,
		IDSpeciality: c.IDSpeciality.Hex(), IDProcess: processIDs,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func (c CompanionDB) IsZero() bool    { return c.ID.IsZero() }
func (CompanionDB) TableName() string { return "companions" }

var _ DBModelInterface = (*CompanionDB)(nil)

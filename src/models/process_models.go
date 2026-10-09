package models

import "time"

type ProcessDB struct {
	ID        DBID      `json:"id" bson:"_id,omitempty"`
	Name      string    `json:"name" bson:"name"`
	Active    bool      `json:"active" bson:"active"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

func (ProcessDB) TableName() string { return "processes" }
func (p ProcessDB) IsZero() bool    { return p.ID.IsZero() && p.Name == "" }

type ProcessCreate struct {
	Name   string `json:"name" binding:"required"`
	Active *bool  `json:"active"`
}

type ProcessUpdate struct {
	Name   string `json:"name" binding:"required"`
	Active bool   `json:"active"`
}

type ProcessPatch struct {
	Name      string    `json:"name,omitempty" bson:"name,omitempty"`
	Active    *bool     `json:"active,omitempty" bson:"active,omitempty"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

func (ProcessPatch) TableName() string { return "processes" }
func (ProcessPatch) IsZero() bool      { return false }

type StudentProcessPatch struct {
	ProcessID DBID      `json:"process_id" bson:"process_id"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

func (StudentProcessPatch) TableName() string { return "students" }
func (StudentProcessPatch) IsZero() bool      { return false }

type StaffProcessAssignment struct {
	IDProcess []string `json:"id_process" binding:"required"`
}
type StudentProcessAssignment struct {
	ProcessID string `json:"process_id" binding:"required"`
}

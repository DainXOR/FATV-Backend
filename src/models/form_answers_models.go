package models

import (
	"dainxor/atv/logger"
	"dainxor/atv/types"
	"dainxor/atv/utils"
	"errors"
)

type Answers[ID comparable] map[ID][]string

//	type Answer[ID any] struct {
//		IDQuestion      ID       `json:"id_question" bson:"id_question,omitempty"`
//		ProvidedAnswers []string `json:"answers" bson:"answers,omitempty"`
//	}
type FormAnswerDB struct {
	ID           DBID          `json:"id" bson:"_id,omitempty"`
	IDForm       DBID          `json:"id_form" bson:"id_form,omitempty"`
	InvitationID DBID          `json:"-" bson:"invitation_id,omitempty"`
	Answers      Answers[DBID] `json:"answers" bson:"answers,omitempty"`
	CreatedAt    DBDateTime    `json:"created_at,omitzero" bson:"created_at,omitempty"`
	UpdatedAt    DBDateTime    `json:"updated_at,omitzero" bson:"updated_at,omitempty"`
	DeletedAt    DBDateTime    `json:"deleted_at" bson:"deleted_at"`
}
type FormAnswerCreate struct {
	IDForm       string          `json:"id_form"`
	Answers      Answers[string] `json:"answers"`
	InvitationID DBID            `json:"-" bson:"-"`
}
type FormAnswerResponse struct {
	ID        string          `json:"id"`
	IDForm    string          `json:"id_form"`
	Answers   Answers[string] `json:"answers"`
	CreatedAt DBDateTime      `json:"created_at,omitzero"`
	UpdatedAt DBDateTime      `json:"updated_at,omitzero"`
}

func (o FormAnswerCreate) ToInsert() types.Result[FormAnswerDB] {
	obj := FormAnswerDB{
		InvitationID: o.InvitationID,
		CreatedAt:    Time.Now(),
		UpdatedAt:    Time.Now(),
		DeletedAt:    Time.Zero(),
	}

	if !ID.Ensure(o.IDForm, &obj.IDForm, "IDForm") {
		logger.Lava(types.V("0.2.1"), "Using not standarized error")
		return types.ResultErr[FormAnswerDB](errors.New("Invalid IDForm"))
	}

	obj.Answers = make(Answers[DBID], len(o.Answers))
	var oid DBID
	for id, answer := range o.Answers {
		if !ID.Ensure(id, &oid, "IDQuestionType") {
			logger.Lava(types.V("0.2.1"), "Using not standarized error")
			return types.ResultErr[FormAnswerDB](errors.New("Invalid IDQuestionType"))
		}

		obj.Answers[oid] = answer
	}

	return types.ResultOk(obj)
}
func (o FormAnswerCreate) ToUpdate() types.Result[FormAnswerDB] {
	obj := FormAnswerDB{
		UpdatedAt: Time.Now(),
	}

	if !ID.OmitEmpty(o.IDForm, &obj.IDForm, "IDForm") {
		logger.Lava(types.V("0.2.1"), "Using not standarized error")
		return types.ResultErr[FormAnswerDB](errors.New("Invalid IDForm"))
	}

	obj.Answers = make(Answers[DBID], len(o.Answers))

	for id, answer := range o.Answers {
		var oid DBID

		if !ID.OmitEmpty(id, &oid, "IDQuestionType") {
			logger.Lava(types.V("0.2.1"), "Using not standarized error")
			return types.ResultErr[FormAnswerDB](errors.New("Invalid IDQuestionType"))
		}

		obj.Answers[oid] = answer
	}

	return types.ResultOk(obj)
}
func (o FormAnswerDB) ToResponse() FormAnswerResponse {
	return FormAnswerResponse{
		ID:        o.ID.Hex(),
		IDForm:    o.IDForm.Hex(),
		Answers:   utils.DMap(o.Answers, func(id DBID, vals []string) (string, []string) { return id.Hex(), vals }),
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
}

func (o FormAnswerDB) IsZero() bool {
	zeroObj := FormAnswerDB{}

	comp := o.ID == zeroObj.ID &&
		o.IDForm == zeroObj.IDForm &&
		o.CreatedAt.Equal(zeroObj.CreatedAt) &&
		o.UpdatedAt.Equal(zeroObj.UpdatedAt) &&
		o.DeletedAt.Equal(zeroObj.DeletedAt) &&
		len(o.Answers) == 0

	//if comp {
	//	for _, o := range o.Answers {
	//		comp = comp && o.IDQuestion.IsZero()
	//		comp = comp && (len(o.Answers) == 0)
	//	}
	//}

	return comp
}

func (FormAnswerDB) TableName() string {
	return "form_answers"
}

var _ DBModelInterface = (*FormAnswerDB)(nil)

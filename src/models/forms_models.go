package models

import (
	"dainxor/atv/logger"
	"dainxor/atv/types"
	"dainxor/atv/utils"
	"errors"
)

type formSectionNS struct{}

var FormSection formSectionNS

func (formSectionNS) Academic() string {
	return "academic"
}
func (formSectionNS) Socioeconomic() string {
	return "socioeconomic"
}
func (formSectionNS) Relacional() string {
	return "relacional"
}
func (formSectionNS) Socioemotional() string {
	return "socioemotional"
}

type parentQuestion[ID any] struct {
	IDQuestion    ID       `json:"id_question" bson:"id_question"`
	NeededAnswers []string `json:"needed_answers" bson:"needed_answers"`
}

type questionInfo[ID any] struct {
	IDQuestion ID                 `json:"id_question" bson:"id_question,omitempty"`
	Position   uint8              `json:"position" bson:"position"`
	Weight     uint8              `json:"weight" bson:"weight"`
	Optional   bool               `json:"optional" bson:"optional"`
	Parent     parentQuestion[ID] `json:"parent" bson:"parent"`
}

func questionFrom[IID, OID any](other questionInfo[IID], mapper func(IID) OID) questionInfo[OID] {
	return questionInfo[OID]{
		IDQuestion: mapper(other.IDQuestion),
		Position:   other.Position,
		Weight:     other.Weight,
		Optional:   other.Optional,
		Parent:     parentQuestion[OID]{IDQuestion: mapper(other.Parent.IDQuestion), NeededAnswers: other.Parent.NeededAnswers},
	}
}
func questionEmpty[ID any]() questionInfo[ID] {
	return questionInfo[ID]{}
}

type sectionInfo struct {
	Name   string `json:"name" bson:"name,omitempty"`
	Weight uint8  `json:"weight" bson:"weight"`
}

type sectionsQuestions[ID any] = map[uint8][]questionInfo[ID] // Section pos -> []questionInfo
type sectionsInfo = map[uint8]sectionInfo                     // Section pos -> name

type FormDB struct {
	ID                 DBID                    `json:"id" bson:"_id,omitempty"`
	Name               string                  `json:"name" bson:"name,omitempty"`
	Description        string                  `json:"description" bson:"description,omitempty"`
	Date               string                  `json:"date" bson:"date,omitempty"`
	QuestionsInfo      []questionInfo[DBID]    `json:"questions_info" bson:"questions_info,omitempty"`
	Sections           sectionsQuestions[DBID] `json:"sections" bson:"sections,omitempty"`
	SectionsInfo       sectionsInfo            `json:"sections_info" bson:"sections_info,omitempty"`
	PositionsAreGlobal bool                    `json:"positions_are_global" bson:"positions_are_global,omitempty"`
	CreatedAt          DBDateTime              `json:"created_at,omitzero" bson:"created_at,omitempty"`
	UpdatedAt          DBDateTime              `json:"updated_at,omitzero" bson:"updated_at,omitempty"`
	DeletedAt          DBDateTime              `json:"deleted_at" bson:"deleted_at"`
}
type FormCreate struct {
	Name          string                    `json:"name"`
	Description   string                    `json:"description"`
	Date          string                    `json:"date"`
	QuestionsInfo []questionInfo[string]    `json:"questions_info"`
	Sections      sectionsQuestions[string] `json:"sections"`
	SectionsInfo  sectionsInfo              `json:"sections_info" bson:"sections_info,omitempty"`

	PositionsAreGlobal bool `json:"positions_are_global"`
}
type FormResponse struct {
	ID                 string                    `json:"id"`
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	Date               string                    `json:"date"`
	QuestionsInfo      []questionInfo[string]    `json:"questions_info"`
	Sections           sectionsQuestions[string] `json:"sections"`
	SectionsInfo       sectionsInfo              `json:"sections_info" bson:"sections_info,omitempty"`
	PositionsAreGlobal bool                      `json:"positions_are_global"`
	CreatedAt          DBDateTime                `json:"created_at"`
	UpdatedAt          DBDateTime                `json:"updated_at"`
}

func (q FormCreate) ToInsert() types.Result[FormDB] {
	obj := FormDB{
		Name:          q.Name,
		Description:   q.Description,
		Date:          q.Date,
		QuestionsInfo: nil,
		Sections:      nil,
		SectionsInfo:  q.SectionsInfo,
		CreatedAt:     Time.Now(),
		UpdatedAt:     Time.Now(),
		DeletedAt:     Time.Zero(),
	}

	logger.Lava(types.V("0.2.0"), "Using not standarized error")
	questions, err := utils.MapE(q.QuestionsInfo, func(v questionInfo[string]) (questionInfo[DBID], error) {
		var oid DBID
		var oidParent DBID

		if !ID.Ensure(v.IDQuestion, &oid, "IDQuestion") {
			return questionEmpty[DBID](), errors.New("Invalid IDQuestion: " + v.IDQuestion)
		}
		if !ID.OmitEmpty(v.Parent.IDQuestion, &oidParent, "IDParentQuestion") {
			return questionEmpty[DBID](), errors.New("Invalid IDParentQuestion: " + v.Parent.IDQuestion)
		}

		return questionFrom(v, func(id string) DBID {
			oid, _ := ID.ToDB(id)
			return oid
		}), nil
	}, true) // Keeps mapping even if any element errors

	if err != nil {
		logger.Error("Failed mapping the question info values:", err)
		obj.QuestionsInfo = make([]questionInfo[DBID], 0)
		return types.ResultErr[FormDB](err)
	}

	obj.QuestionsInfo = questions

	sections, err := utils.DMapE(q.Sections, func(key uint8, val []questionInfo[string]) (uint8, []questionInfo[DBID], error) {
		newValue, errMap := utils.MapE(val, func(v questionInfo[string]) (questionInfo[DBID], error) {
			var oid DBID
			var oidParent DBID

			if !ID.Ensure(v.IDQuestion, &oid, "IDQuestion") {
				return questionEmpty[DBID](), errors.New("Invalid IDQuestion: " + v.IDQuestion)
			}
			if !ID.OmitEmpty(v.Parent.IDQuestion, &oidParent, "IDParentQuestion") {
				return questionEmpty[DBID](), errors.New("Invalid IDParentQuestion: " + v.Parent.IDQuestion)
			}

			return questionFrom(v, func(id string) DBID {
				oid, _ := ID.ToDB(id)
				return oid
			}), nil
		}, true)
		return key, newValue, errMap
	}, true)

	if err != nil {
		logger.Warning("Partial success mapping the question info values:", err)
		obj.Sections = make(sectionsQuestions[DBID], 0)
		return types.ResultErr[FormDB](err)
	}

	obj.Sections = sections
	return types.ResultOk(obj)
}
func (q FormCreate) ToUpdate() types.Result[FormDB] {
	obj := FormDB{
		Name:          q.Name,
		Description:   q.Description,
		Date:          q.Date,
		QuestionsInfo: nil,
		Sections:      nil,
		SectionsInfo:  q.SectionsInfo,
		UpdatedAt:     Time.Now(),
	}

	logger.Lava(types.V("0.2.1"), "Using not standarized error")
	questions, err := utils.MapE(q.QuestionsInfo, func(v questionInfo[string]) (questionInfo[DBID], error) {
		var oid DBID
		var oidParent DBID

		if !ID.OmitEmpty(v.IDQuestion, &oid, "IDQuestion") {
			return questionEmpty[DBID](), errors.New("Invalid IDQuestion: " + v.IDQuestion)
		}
		if !ID.OmitEmpty(v.Parent.IDQuestion, &oidParent, "IDParentQuestion") {
			return questionEmpty[DBID](), errors.New("Invalid IDParentQuestion: " + v.Parent.IDQuestion)
		}

		return questionFrom(v, func(id string) DBID {
			oid, _ := ID.ToDB(id)
			return oid
		}), nil
	}, true)

	if err != nil {
		logger.Warning("Partial success mapping the question info values:", err)
		obj.QuestionsInfo = make([]questionInfo[DBID], 0)
		return types.ResultErr[FormDB](err)
	}

	obj.QuestionsInfo = questions

	sections, err := utils.DMapE(q.Sections, func(key uint8, val []questionInfo[string]) (uint8, []questionInfo[DBID], error) {
		newValue, errMap := utils.MapE(val, func(v questionInfo[string]) (questionInfo[DBID], error) {
			var oid DBID
			var oidParent DBID

			if !ID.OmitEmpty(v.IDQuestion, &oid, "IDQuestion") {
				return questionEmpty[DBID](), errors.New("Invalid IDQuestion: " + v.IDQuestion)
			}
			if !ID.OmitEmpty(v.Parent.IDQuestion, &oidParent, "IDParentQuestion") {
				return questionEmpty[DBID](), errors.New("Invalid IDParentQuestion: " + v.Parent.IDQuestion)
			}

			return questionFrom(v, func(id string) DBID {
				oid, _ := ID.ToDB(id)
				return oid
			}), nil
		}, true)

		return key, newValue, errMap
	}, true)

	if err != nil {
		logger.Warning("Partial success mapping the question info values:", err)
		obj.Sections = make(sectionsQuestions[DBID], 0)
		return types.ResultErr[FormDB](err)
	}

	obj.Sections = sections
	return types.ResultOk(obj)
}
func (q FormDB) ToResponse() FormResponse {
	return FormResponse{
		ID:          q.ID.Hex(),
		Name:        q.Name,
		Description: q.Description,
		Date:        q.Date,
		QuestionsInfo: utils.Map(q.QuestionsInfo, func(v questionInfo[DBID]) questionInfo[string] {
			return questionFrom(v, DBID.Hex)
		}),
		Sections: utils.DMap(q.Sections, func(k uint8, v []questionInfo[DBID]) (uint8, []questionInfo[string]) {
			return k, utils.Map(v, func(v questionInfo[DBID]) questionInfo[string] { return questionFrom(v, DBID.Hex) })
		}),
		SectionsInfo: q.SectionsInfo,
		UpdatedAt:    q.UpdatedAt,
		CreatedAt:    q.CreatedAt,
	}
}

func (q FormDB) IsZero() bool {
	zeroObj := FormDB{}

	return q.ID == zeroObj.ID &&
		q.Name == zeroObj.Name &&
		q.Description == zeroObj.Description &&
		q.Date == zeroObj.Date &&
		q.CreatedAt.Equal(zeroObj.CreatedAt) &&
		q.UpdatedAt.Equal(zeroObj.UpdatedAt) &&
		q.DeletedAt.Equal(zeroObj.DeletedAt) &&
		len(q.QuestionsInfo) == 0 &&
		len(q.Sections) == 0
}

func (FormDB) TableName() string {
	return "forms"
}

var _ DBModelInterface = (*FormDB)(nil)

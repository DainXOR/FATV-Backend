package models

import (
	"dainxor/atv/types"
	"dainxor/atv/utils"
	"errors"
)

type formSectionNS struct{}

var FormSection formSectionNS

func (formSectionNS) Academic() string       { return "academic" }
func (formSectionNS) Socioeconomic() string  { return "socioeconomic" }
func (formSectionNS) Relacional() string     { return "relacional" }
func (formSectionNS) Socioemotional() string { return "socioemotional" }

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

func questionEmpty[ID any]() questionInfo[ID] { return questionInfo[ID]{} }

type sectionInfo struct {
	Name   string `json:"name" bson:"name,omitempty"`
	Weight uint8  `json:"weight" bson:"weight"`
}

type sectionsQuestions[ID any] = map[uint8][]questionInfo[ID]
type sectionsInfo = map[uint8]sectionInfo

type FormDB struct {
	ID                 DBID                    `json:"id" bson:"_id,omitempty"`
	Name               string                  `json:"name" bson:"name,omitempty"`
	Description        string                  `json:"description" bson:"description,omitempty"`
	ProcessTags        []string                `json:"process_tags" bson:"process_tags"`
	Date               string                  `json:"date" bson:"date,omitempty"`
	Sections           sectionsQuestions[DBID] `json:"sections" bson:"sections"`
	SectionsInfo       sectionsInfo            `json:"sections_info" bson:"sections_info"`
	PositionsAreGlobal bool                    `json:"positions_are_global" bson:"positions_are_global"`
	CreatedAt          DBDateTime              `json:"created_at,omitzero" bson:"created_at,omitempty"`
	UpdatedAt          DBDateTime              `json:"updated_at,omitzero" bson:"updated_at,omitempty"`
	DeletedAt          DBDateTime              `json:"deleted_at" bson:"deleted_at"`
}

type FormCreate struct {
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	ProcessTags        []string                  `json:"process_tags"`
	Date               string                    `json:"date"`
	Sections           sectionsQuestions[string] `json:"sections"`
	SectionsInfo       sectionsInfo              `json:"sections_info"`
	PositionsAreGlobal bool                      `json:"positions_are_global"`
}

type FormResponse struct {
	ID                 string                    `json:"id"`
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	ProcessTags        []string                  `json:"process_tags"`
	Date               string                    `json:"date"`
	Sections           sectionsQuestions[string] `json:"sections"`
	SectionsInfo       sectionsInfo              `json:"sections_info"`
	PositionsAreGlobal bool                      `json:"positions_are_global"`
	CreatedAt          DBDateTime                `json:"created_at"`
	UpdatedAt          DBDateTime                `json:"updated_at"`
}

func mapFormSections(sections sectionsQuestions[string]) (sectionsQuestions[DBID], error) {
	mapped := make(sectionsQuestions[DBID], len(sections))
	for sectionPosition, questions := range sections {
		mappedQuestions := make([]questionInfo[DBID], 0, len(questions))
		for _, question := range questions {
			var questionID, parentID DBID
			if !ID.Ensure(question.IDQuestion, &questionID, "IDQuestion") {
				return nil, errors.New("Invalid IDQuestion: " + question.IDQuestion)
			}
			if !ID.OmitEmpty(question.Parent.IDQuestion, &parentID, "IDParentQuestion") {
				return nil, errors.New("Invalid IDParentQuestion: " + question.Parent.IDQuestion)
			}
			mappedQuestions = append(mappedQuestions, questionFrom(question, func(id string) DBID {
				mappedID, _ := ID.ToDB(id)
				return mappedID
			}))
		}
		mapped[sectionPosition] = mappedQuestions
	}
	return mapped, nil
}

func (q FormCreate) ToInsert() types.Result[FormDB] {
	sections, err := mapFormSections(q.Sections)
	if err != nil {
		return types.ResultErr[FormDB](err)
	}
	now := Time.Now().UTC()
	return types.ResultOk(FormDB{
		Name:               q.Name,
		Description:        q.Description,
		ProcessTags:        q.ProcessTags,
		Date:               q.Date,
		Sections:           sections,
		SectionsInfo:       q.SectionsInfo,
		PositionsAreGlobal: q.PositionsAreGlobal,
		CreatedAt:          now,
		UpdatedAt:          now,
		DeletedAt:          Time.Zero(),
	})
}

func (q FormCreate) ToUpdate() types.Result[FormDB] {
	sections, err := mapFormSections(q.Sections)
	if err != nil {
		return types.ResultErr[FormDB](err)
	}
	return types.ResultOk(FormDB{
		Name:               q.Name,
		Description:        q.Description,
		ProcessTags:        q.ProcessTags,
		Date:               q.Date,
		Sections:           sections,
		SectionsInfo:       q.SectionsInfo,
		PositionsAreGlobal: q.PositionsAreGlobal,
		UpdatedAt:          Time.Now().UTC(),
	})
}

func (q FormDB) ToResponse() FormResponse {
	return FormResponse{
		ID:          q.ID.Hex(),
		Name:        q.Name,
		Description: q.Description,
		ProcessTags: q.ProcessTags,
		Date:        q.Date,
		Sections: utils.DMap(q.Sections, func(k uint8, values []questionInfo[DBID]) (uint8, []questionInfo[string]) {
			return k, utils.Map(values, func(value questionInfo[DBID]) questionInfo[string] {
				return questionFrom(value, func(id DBID) string {
					if id.IsZero() {
						return ""
					}
					return id.Hex()
				})
			})
		}),
		SectionsInfo:       q.SectionsInfo,
		PositionsAreGlobal: q.PositionsAreGlobal,
		UpdatedAt:          q.UpdatedAt,
		CreatedAt:          q.CreatedAt,
	}
}

func (q FormDB) IsZero() bool {
	zero := FormDB{}
	return q.ID == zero.ID && q.Name == zero.Name && q.Description == zero.Description &&
		q.Date == zero.Date && q.CreatedAt.Equal(zero.CreatedAt) && q.UpdatedAt.Equal(zero.UpdatedAt) &&
		q.DeletedAt.Equal(zero.DeletedAt) && len(q.Sections) == 0 && len(q.SectionsInfo) == 0
}

func (FormDB) TableName() string { return "forms" }

var _ DBModelInterface = (*FormDB)(nil)

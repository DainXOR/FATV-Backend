package models

import (
	"encoding/json"
	"testing"
)

func TestFormCreateUsesSectionStructureAndGlobalPositionFlag(t *testing.T) {
	questionID := "693303d08d375622ce1f0482"
	form := FormCreate{
		Name:               "Section-based form",
		PositionsAreGlobal: true,
		Sections: sectionsQuestions[string]{
			1: {{IDQuestion: questionID, Position: 1, Weight: 5}},
			2: {{IDQuestion: "693310ec8d375622ce1f0494", Position: 2, Weight: 10}},
		},
		SectionsInfo: sectionsInfo{
			1: {Name: "Academic", Weight: 60},
			2: {Name: "Social", Weight: 40},
		},
	}

	created := form.ToInsert()
	if created.IsErr() {
		t.Fatalf("ToInsert() returned error: %v", created.Error())
	}
	stored := created.Value()
	if stored.PositionsAreGlobal != form.PositionsAreGlobal {
		t.Fatalf("PositionsAreGlobal = %v, want %v", stored.PositionsAreGlobal, form.PositionsAreGlobal)
	}
	if len(stored.Sections) != 2 || len(stored.Sections[1]) != 1 || len(stored.Sections[2]) != 1 {
		t.Fatalf("section membership was not preserved: %#v", stored.Sections)
	}
	if stored.Sections[1][0].IDQuestion.Hex() != questionID {
		t.Fatalf("question ID was not mapped into the section")
	}
	if stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() {
		t.Fatalf("creation/update timestamps must be populated")
	}

	response, err := json.Marshal(stored.ToResponse())
	if err != nil {
		t.Fatalf("failed to marshal form response: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(response, &fields); err != nil {
		t.Fatalf("failed to inspect form response: %v", err)
	}
	if _, exists := fields["questions_info"]; exists {
		t.Fatalf("form response still exposes deprecated questions_info field")
	}
	if _, exists := fields["sections"]; !exists {
		t.Fatalf("form response is missing sections")
	}
	if _, exists := fields["sections_info"]; !exists {
		t.Fatalf("form response is missing sections_info")
	}
	if _, exists := fields["positions_are_global"]; !exists {
		t.Fatalf("form response is missing positions_are_global")
	}
}

func TestFormCreateRejectsInvalidQuestionIDInsideSection(t *testing.T) {
	form := FormCreate{Sections: sectionsQuestions[string]{1: {{IDQuestion: "not-an-object-id"}}}}
	if result := form.ToInsert(); result.IsOk() {
		t.Fatal("ToInsert() succeeded with an invalid section question ID")
	}
}

func TestFormResponseNormalizesEmptyParentID(t *testing.T) {
	questionID, err := ID.ToDB("693303d08d375622ce1f0482")
	if err != nil {
		t.Fatal(err)
	}
	form := FormDB{
		Sections: sectionsQuestions[DBID]{
			1: {{
				IDQuestion: questionID,
				Parent:     parentQuestion[DBID]{NeededAnswers: []string{}},
			}},
		},
	}
	response := form.ToResponse()
	if got := response.Sections[1][0].Parent.IDQuestion; got != "" {
		t.Fatalf("empty parent ID = %q, want empty string", got)
	}
}

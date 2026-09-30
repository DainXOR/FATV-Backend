package formaccess

import (
	"testing"
	"time"

	"dainxor/atv/models"
)

func TestInvitationAvailability(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	base := models.FormInvitationDB{
		ExpiresAt:       now.Add(time.Hour),
		SubmissionState: models.InvitationStateOpen,
	}
	tests := []struct {
		name            string
		inv             models.FormInvitationDB
		wantUnavailable bool
	}{
		{name: "open invitation", inv: base, wantUnavailable: false},
		{name: "expired invitation", inv: func() models.FormInvitationDB { x := base; x.ExpiresAt = now; return x }(), wantUnavailable: true},
		{name: "revoked invitation", inv: func() models.FormInvitationDB { x := base; x.RevokedAt = now; return x }(), wantUnavailable: true},
		{name: "submitted invitation", inv: func() models.FormInvitationDB {
			x := base
			x.SubmissionState = models.InvitationStateSubmitted
			return x
		}(), wantUnavailable: true},
		{name: "unknown state", inv: func() models.FormInvitationDB { x := base; x.SubmissionState = "unexpected"; return x }(), wantUnavailable: true},
		{name: "legacy submitted invitation", inv: func() models.FormInvitationDB { x := base; x.SubmissionState = ""; x.SubmittedAt = now; return x }(), wantUnavailable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := invitationUnavailable(test.inv, now); got != test.wantUnavailable {
				t.Fatalf("invitationUnavailable() = %v, want %v", got, test.wantUnavailable)
			}
		})
	}
}

func TestTokenHashDoesNotExposeToken(t *testing.T) {
	token := "sample-high-entropy-token"
	if tokenHash(token) == token {
		t.Fatal("token hash unexpectedly equals raw token")
	}
	if tokenHash(token) != tokenHash(token) {
		t.Fatal("same token must produce stable hash for lookup")
	}
	if tokenHash(token) == tokenHash("different-token") {
		t.Fatal("different tokens must not share the same hash")
	}
}

func TestQuestionTypeNormalization(t *testing.T) {
	if got := normalizeQuestionType(" Opción única "); got != "opcion unica" {
		t.Fatalf("normalizeQuestionType() = %q", got)
	}
	for _, value := range []string{"Verdadero / Falso", "Verdadero o Falso"} {
		if !isTrueFalseQuestionType(normalizeQuestionType(value)) {
			t.Errorf("expected true/false type to be recognized: %q", value)
		}
	}
	if isTrueFalseQuestionType(normalizeQuestionType("Múltiple respuesta")) {
		t.Fatal("multiple choice must not be treated as true/false")
	}
}

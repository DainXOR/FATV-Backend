package controller

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCreateFormLinkBodySupportsOptionalExpiry(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantExpiry bool
	}{
		{
			name:       "never expires",
			payload:    `{"form_id":"6a409b9f491deeeda42e99b0","expires_at":null}`,
			wantExpiry: false,
		},
		{
			name:       "expires at configured time",
			payload:    `{"form_id":"6a409b9f491deeeda42e99b0","expires_at":"2030-01-02T03:04:05Z"}`,
			wantExpiry: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var body createFormLinkBody
			if err := json.Unmarshal([]byte(test.payload), &body); err != nil {
				t.Fatalf("payload should decode: %v", err)
			}
			if body.FormID != "6a409b9f491deeeda42e99b0" {
				t.Fatalf("unexpected form id: %q", body.FormID)
			}
			if got := body.ExpiresAt != nil; got != test.wantExpiry {
				t.Fatalf("expiry presence = %v, want %v", got, test.wantExpiry)
			}
			if test.wantExpiry && !body.ExpiresAt.Equal(time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)) {
				t.Fatalf("unexpected expiry: %v", body.ExpiresAt)
			}
		})
	}
}

func TestIssueInvitationsBodySupportsNullExpiry(t *testing.T) {
	var body issueInvitationsBody
	payload := `{"form_id":"6a409b9f491deeeda42e99b0","student_ids":["6a409b9f491deeeda42e99b1"],"expires_at":null}`
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		t.Fatalf("payload should decode: %v", err)
	}
	if body.ExpiresAt != nil {
		t.Fatalf("expiry should be nil for a never-expiring invitation")
	}
	if !expiryValue(body.ExpiresAt).IsZero() {
		t.Fatalf("nil expiry should be converted to zero time")
	}
}

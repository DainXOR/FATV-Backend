package models

import (
	"testing"
	"time"
)

func TestTimeParseSupportsFrontendAndLegacyFormats(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "frontend ISO with seconds", value: "2025-12-05T11:40:00"},
		{name: "frontend ISO without seconds", value: "2025-12-05T11:40"},
		{name: "legacy day-first", value: "05-12-2025T11:40"},
		{name: "slash day-first", value: "23/08/2025 5:00:17"},
		{name: "RFC3339", value: "2025-12-05T11:40:00Z"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := Time.Parse(test.value)
			if err != nil {
				t.Fatalf("Time.Parse(%q) error = %v", test.value, err)
			}
			if parsed.IsZero() {
				t.Fatalf("Time.Parse(%q) returned zero time", test.value)
			}
		})
	}
}

func TestTimeParseRejectsInvalidInput(t *testing.T) {
	if parsed, err := Time.Parse("not-a-date"); err == nil || !parsed.Equal(time.Time{}) {
		t.Fatalf("invalid date should return zero time and an error, got %v, %v", parsed, err)
	}
}

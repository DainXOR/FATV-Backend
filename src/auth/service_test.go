package auth

import (
	"testing"
	"unicode/utf8"
)

func TestRandomTokenIsHighEntropyAndUnique(t *testing.T) {
	first, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 || first == second || !utf8.ValidString(first) {
		t.Fatal("random token did not meet expected shape/uniqueness")
	}
	if hash(first) == first {
		t.Fatal("token hash must not equal the raw token")
	}
}

func TestRandomCodeHasEightDigits(t *testing.T) {
	code, err := randomCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 8 {
		t.Fatalf("expected 8-digit code, got %q", code)
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			t.Fatalf("non-numeric code generated: %q", code)
		}
	}
}

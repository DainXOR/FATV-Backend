package auth

import (
	"testing"
	"unicode/utf8"

	"dainxor/atv/models"
	"golang.org/x/crypto/bcrypt"
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

func TestPasswordHashUsesPepperAndVerifies(t *testing.T) {
	t.Setenv("FATV_AUTH_PEPPER", "test-pepper")
	hashed, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	input, err := passwordInput("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	defer clearSecret(input)
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), input); err != nil {
		t.Fatal("peppered password did not verify")
	}
	wrong, _ := passwordInput("wrong password")
	defer clearSecret(wrong)
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), wrong); err == nil {
		t.Fatal("wrong password verified")
	}
}

func TestDevelopmentOptionsAreRejectedOutsideDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("FATV_DEV_AUTH_BYPASS", "true")
	t.Setenv("FATV_DEV_SHOW_ACTION_CODES", "false")
	if err := (Service{}).ValidateDevelopmentOptions(); err == nil {
		t.Fatal("expected production development-option rejection")
	}
}

func TestDevelopmentOptionsAreAllowedInDebug(t *testing.T) {
	t.Setenv("APP_ENV", "debug")
	t.Setenv("FATV_DEV_AUTH_BYPASS", "true")
	t.Setenv("FATV_DEV_SHOW_ACTION_CODES", "true")
	if err := (Service{}).ValidateDevelopmentOptions(); err != nil {
		t.Fatalf("unexpected development-option error: %v", err)
	}
}

func TestCanRecoverAccount(t *testing.T) {
	tests := []struct {
		status models.AccountStatus
		want   bool
	}{
		{models.AccountPending, true},
		{models.AccountActive, true},
		{models.AccountDisabled, false},
		{models.AccountDeleted, false},
	}

	for _, test := range tests {
		if got := canRecoverAccount(test.status); got != test.want {
			t.Errorf("canRecoverAccount(%q) = %t, want %t", test.status, got, test.want)
		}
	}
}

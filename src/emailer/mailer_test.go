package emailer

import "testing"

func TestParseAddress(t *testing.T) {
	valid, err := ParseAddress("Staff@example.org")
	if err != nil || valid != Address("staff@example.org") {
		t.Fatalf("expected normalized address, got %q, err=%v", valid, err)
	}
	for _, value := range []string{"", "not-an-email", "Name <staff@example.org>", "a@b", "staff@example.org\r\nBcc:evil@example.org"} {
		if _, err := ParseAddress(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}

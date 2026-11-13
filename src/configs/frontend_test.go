package configs

import "testing"

func TestFrontendBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "missing", wantErr: true},
		{name: "valid localhost", value: "http://localhost:3000/", want: "http://localhost:3000"},
		{name: "valid https", value: "https://forms.example.org", want: "https://forms.example.org"},
		{name: "relative URL", value: "/student-form", wantErr: true},
		{name: "unsupported scheme", value: "ftp://forms.example.org", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("FRONTEND_BASE_URL", test.value)
			got, err := Frontend.BaseURL()
			if (err != nil) != test.wantErr {
				t.Fatalf("BaseURL() error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("BaseURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFrontendURL(t *testing.T) {
	t.Setenv("FRONTEND_BASE_URL", "http://localhost:3000/")
	got, err := Frontend.URL("/student-form/example")
	if err != nil || got != "http://localhost:3000/student-form/example" {
		t.Fatalf("URL() = %q, err = %v", got, err)
	}
	if _, err := Frontend.URL("student-form/example"); err == nil {
		t.Fatal("URL() should reject a path without a leading slash")
	}
}

package middleware

import "testing"

func TestPublicAndProtectedRouteClassification(t *testing.T) {
	public := []string{
		"/", "/api/info/ping", "/api/v1/auth/login",
		"/api/v1/auth/setup", "/api/v1/auth/recovery/request",
		"/api/v1/public/forms/opaque-token",
	}
	for _, path := range public {
		if !isPublicPath(path) {
			t.Errorf("expected public path: %s", path)
		}
	}
	protected := []string{
		"/api/v1/auth/me", "/api/v1/auth/accounts",
		"/api/v1/students", "/api/v1/forms/answers",
		"/api/test/get",
	}
	for _, path := range protected {
		if isPublicPath(path) {
			t.Errorf("expected protected path: %s", path)
		}
	}
}

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthVerification(t *testing.T) {
	t.Parallel()

	a := NewAuthenticator("secret-token-123", false)

	req1 := httptest.NewRequest("GET", "/api/containers", nil)
	req1.Header.Set("Authorization", "Bearer secret-token-123")
	if !a.AuthenticateRequest(req1) {
		t.Errorf("expected Bearer token authentication to succeed")
	}

	req2 := httptest.NewRequest("GET", "/api/containers", nil)
	req2.Header.Set("Authorization", "Bearer invalid-token")
	if a.AuthenticateRequest(req2) {
		t.Errorf("expected invalid Bearer token to fail authentication")
	}

	req3 := httptest.NewRequest("GET", "/api/containers", nil)
	req3.AddCookie(&http.Cookie{Name: CookieName, Value: "secret-token-123"})
	if !a.AuthenticateRequest(req3) {
		t.Errorf("expected cookie authentication to succeed")
	}
}

func TestAuthNotRequiredWhenTokenEmpty(t *testing.T) {
	t.Parallel()

	a := NewAuthenticator("", false)
	req := httptest.NewRequest("GET", "/api/containers", nil)
	if !a.AuthenticateRequest(req) {
		t.Errorf("expected authentication to pass when token is empty")
	}
}

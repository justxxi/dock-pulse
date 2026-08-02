package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

const CookieName = "dock_pulse_token"

type Authenticator struct {
	token string
	isTLS bool
}

func NewAuthenticator(token string, isTLS bool) *Authenticator {
	return &Authenticator{
		token: token,
		isTLS: isTLS,
	}
}

func (a *Authenticator) IsRequired() bool {
	return a.token != ""
}

func (a *Authenticator) VerifyToken(token string) bool {
	if a.token == "" {
		return true
	}
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(a.token)) == 1
}

func (a *Authenticator) AuthenticateRequest(r *http.Request) bool {
	if a.token == "" {
		return true
	}

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if a.VerifyToken(token) {
			return true
		}
	}

	if cookie, err := r.Cookie(CookieName); err == nil {
		if a.VerifyToken(cookie.Value) {
			return true
		}
	}

	if queryToken := r.URL.Query().Get("token"); queryToken != "" {
		if a.VerifyToken(queryToken) {
			return true
		}
	}

	return false
}

func (a *Authenticator) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HttpOnly: true,
		Secure:   a.isTLS,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *Authenticator) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   a.isTLS,
		SameSite: http.SameSiteStrictMode,
	})
}

package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testService(t *testing.T) *Service {
	t.Helper()
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	key, err := GenerateSigningKey()
	if err != nil {
		t.Fatal(err)
	}
	return &Service{
		PasswordHash: hash,
		SigningKey:   key,
		Sessions:     NewSessionManager(),
		Limiter:      NewRateLimiter(5, time.Minute),
		LoginPage:    []byte("<html>login</html>"),
	}
}

func TestLoginSuccess(t *testing.T) {
	svc := testService(t)
	body, _ := json.Marshal(map[string]string{"password": "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	svc.Login().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "dg_session" {
			found = true
			if !c.HttpOnly {
				t.Error("cookie should be HttpOnly")
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Error("cookie should be SameSite=Lax")
			}
		}
	}
	if !found {
		t.Error("response should set dg_session cookie")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("response should have Cache-Control: no-store")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc := testService(t)
	body, _ := json.Marshal(map[string]string{"password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	svc.Login().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestLoginWrongContentType(t *testing.T) {
	svc := testService(t)
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader([]byte("password=x")))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	svc.Login().ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", w.Code)
	}
}

func TestLoginRateLimited(t *testing.T) {
	svc := testService(t)
	svc.Limiter = NewRateLimiter(1, time.Minute)

	body, _ := json.Marshal(map[string]string{"password": "wrong"})

	// First attempt
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	svc.Login().ServeHTTP(w, req)

	// Second attempt (rate limited)
	req = httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "1.2.3.4:1234"
	w = httptest.NewRecorder()
	svc.Login().ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 response should include Retry-After header")
	}
}

func TestLoginBodyTooLarge(t *testing.T) {
	svc := testService(t)
	bigBody := bytes.Repeat([]byte("a"), 2048)
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(bigBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	svc.Login().ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Error("oversized body should not succeed")
	}
}

func TestLogout(t *testing.T) {
	svc := testService(t)
	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	w := httptest.NewRecorder()

	svc.Logout().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "dg_session" && c.MaxAge >= 0 {
			t.Error("logout should clear cookie (MaxAge < 0)")
		}
	}
	if svc.Sessions.InvalidatedAt().IsZero() {
		t.Error("logout should invalidate sessions")
	}
}

func TestCheck(t *testing.T) {
	svc := testService(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	w := httptest.NewRecorder()

	svc.Check().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

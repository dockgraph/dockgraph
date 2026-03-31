package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testServiceWithKey(t *testing.T) (*Service, []byte) {
	t.Helper()
	hash, _ := HashPassword("secret")
	key, _ := GenerateSigningKey()
	svc := &Service{
		PasswordHash: hash,
		SigningKey:   key,
		Sessions:     NewSessionManager(),
		Limiter:      NewRateLimiter(5, time.Minute),
		LoginPage:    []byte("<html>login</html>"),
	}
	return svc, key
}

func validCookie(t *testing.T, key []byte) *http.Cookie {
	t.Helper()
	jti, _ := GenerateJTI()
	token, _ := SignToken(Claims{
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		JTI:       jti,
	}, key)
	return &http.Cookie{Name: "dg_session", Value: token}
}

func TestMiddlewareAllowsPublicPaths(t *testing.T) {
	svc, _ := testServiceWithKey(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := svc.Middleware(inner)

	for _, path := range []string{"/healthz", "/api/login"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if path == "/api/login" {
			req.Method = http.MethodPost
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("path %s: status = %d, want 200", path, w.Code)
		}
	}
}

func TestMiddlewareServesLoginWithoutCookie(t *testing.T) {
	svc, _ := testServiceWithKey(t)
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	})
	handler := svc.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "<html>login</html>" {
		t.Error("should serve login page")
	}
}

func TestMiddlewarePassesWithValidCookie(t *testing.T) {
	svc, key := testServiceWithKey(t)
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := svc.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(validCookie(t, key))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("inner handler should be called with valid cookie")
	}
}

func TestMiddlewareRejectsExpiredCookie(t *testing.T) {
	svc, key := testServiceWithKey(t)
	past := time.Now().Add(-2 * time.Hour)
	token, _ := SignToken(Claims{IssuedAt: past, ExpiresAt: past.Add(time.Hour), JTI: "x"}, key)
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	})
	handler := svc.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "dg_session", Value: token})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Body.String() != "<html>login</html>" {
		t.Error("should serve login page for expired cookie")
	}
}

func TestMiddlewareRejectsInvalidatedSession(t *testing.T) {
	svc, key := testServiceWithKey(t)
	cookie := validCookie(t, key)
	svc.Sessions.Invalidate()
	time.Sleep(10 * time.Millisecond)

	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	})
	handler := svc.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Body.String() != "<html>login</html>" {
		t.Error("should serve login page for invalidated session")
	}
}

func TestMiddlewareReturns401ForAPIWithoutAuth(t *testing.T) {
	svc, _ := testServiceWithKey(t)
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	})
	handler := svc.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("API without auth: status = %d, want 401", w.Code)
	}
}

func TestMiddlewareSetsClaimsInContext(t *testing.T) {
	svc, key := testServiceWithKey(t)
	var gotClaims bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		gotClaims = ok && claims.JTI != ""
		w.WriteHeader(http.StatusOK)
	})
	handler := svc.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(validCookie(t, key))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !gotClaims {
		t.Error("middleware should set claims in request context")
	}
}

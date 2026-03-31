package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const claimsContextKey contextKey = "auth_claims"

// Middleware returns HTTP middleware that validates the JWT session cookie.
// Public paths (healthz, login) bypass authentication.
// Unauthenticated page requests get the login page.
// Unauthenticated API/WebSocket requests get 401.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		claims, ok := s.validateRequest(r)
		if !ok {
			s.serveUnauthorized(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext extracts JWT claims from the request context.
// Returns zero Claims and false when auth is disabled or not yet validated.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(Claims)
	return claims, ok
}

// InjectClaims adds claims to a context. Exported for testing only.
func InjectClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

func (s *Service) validateRequest(r *http.Request) (Claims, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return Claims{}, false
	}
	claims, err := VerifyToken(cookie.Value, s.SigningKey)
	if err != nil {
		return Claims{}, false
	}
	if !s.Sessions.IsValid(claims.IssuedAt) {
		return Claims{}, false
	}
	return claims, true
}

func (s *Service) serveUnauthorized(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("Upgrade") == "websocket" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(s.LoginPage)
}

func isPublicPath(method, path string) bool {
	if path == "/healthz" {
		return true
	}
	if method == http.MethodPost && path == "/api/login" {
		return true
	}
	return false
}

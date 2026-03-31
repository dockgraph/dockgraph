package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	cookieName     = "dg_session"
	sessionMaxAge  = 7 * 24 * time.Hour
	maxRequestBody = 1024
)

// Service holds auth dependencies shared by handlers and middleware.
type Service struct {
	PasswordHash string
	SigningKey   []byte
	Sessions     *SessionManager
	Limiter      *RateLimiter
	LoginPage    []byte
}

// Login returns a handler for POST /api/login.
func (s *Service) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

		ip := clientIP(r)
		if !s.Limiter.Allow(ip) {
			writeRateLimited(w, s.Limiter.RetryAfter(ip))
			return
		}

		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		ok, err := CheckPassword(body.Password, s.PasswordHash)
		if err != nil || !ok {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid password"})
			return
		}

		if err := s.setSessionCookie(w, r); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
	}
}

// Logout returns a handler for POST /api/logout.
func (s *Service) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.Sessions.Invalidate()
		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
		})
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
	}
}

// Check returns a handler for GET /api/auth/check.
// Returns 200 with JSON if the request reached this handler (passed middleware).
func (s *Service) Check() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authenticated":true}`))
	}
}

func (s *Service) setSessionCookie(w http.ResponseWriter, r *http.Request) error {
	now := time.Now()
	jti, err := GenerateJTI()
	if err != nil {
		return err
	}
	token, err := SignToken(Claims{
		IssuedAt:  now,
		ExpiresAt: now.Add(sessionMaxAge),
		JTI:       jti,
	}, s.SigningKey)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeRateLimited(w http.ResponseWriter, retryAfter int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":      "too many attempts",
		"retryAfter": retryAfter,
	})
}

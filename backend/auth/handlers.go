package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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

// loginQueueTimeout bounds how long a login waits for a free password
// verification slot (see maxConcurrentChecks) before it is turned away. A
// variable so tests can lower it.
var loginQueueTimeout = 5 * time.Second

// Service holds auth dependencies shared by handlers and middleware.
type Service struct {
	PasswordHash string
	SigningKey   []byte
	Sessions     *SessionManager
	Limiter      *RateLimiter
	LoginPage    []byte
	// LoginScript is the login page's script, served at LoginScriptPath so
	// the page needs no inline script under the Content-Security-Policy.
	LoginScript []byte
	// Proxies are the reverse proxies trusted to report the client's address
	// and scheme; empty trusts none.
	Proxies TrustedProxies
}

// LoginScriptPath is where the login page loads its script from. Like the
// page itself it is served without a session.
const LoginScriptPath = "/login.js"

// Login returns a handler for POST /api/login.
func (s *Service) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

		client := r.RemoteAddr
		if addr, ok := s.Proxies.ClientAddr(r); ok {
			client = addr.String()
		}
		key := s.Proxies.ClientKey(r)
		if allowed, firstRefusal := s.Limiter.attempt(key); !allowed {
			retryAfter := max(s.Limiter.RetryAfter(key), 1)
			if firstRefusal {
				log.Printf("WARN  too many login attempts from %s: refusing them for %ds", client, retryAfter)
			}
			writeRateLimited(w, retryAfter)
			return
		}

		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), loginQueueTimeout)
		ok, err := CheckPasswordBounded(ctx, body.Password, s.PasswordHash)
		cancel()
		if errors.Is(err, ErrBusy) {
			// The password was never checked: don't count the attempt.
			s.Limiter.Forget(key)
			if r.Context().Err() == nil {
				writeBusy(w)
			}
			return
		}
		if err != nil || !ok {
			log.Printf("WARN  failed login from %s", client)
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
			Secure:   s.Proxies.IsHTTPS(r),
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
		Secure:   s.Proxies.IsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// LoginScriptHandler returns a handler serving LoginScript.
func (s *Service) LoginScriptHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(s.LoginScript)
	}
}

// writeBusy answers a login that found every verification slot taken.
func writeBusy(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "server busy"})
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

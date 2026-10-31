package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
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

func loginRequest(password, remote string) *http.Request {
	body, _ := json.Marshal(map[string]string{"password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remote
	return req
}

// Every address of one IPv6 /64 shares a single budget.
func TestLoginRateLimitedPerIPv6Prefix(t *testing.T) {
	svc := testService(t)
	svc.Limiter = NewRateLimiter(1, time.Minute)

	w := httptest.NewRecorder()
	svc.Login().ServeHTTP(w, loginRequest("wrong", "[2001:db8:1:2::10]:4000"))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("first attempt status = %d, want 401", w.Code)
	}
	w = httptest.NewRecorder()
	svc.Login().ServeHTTP(w, loginRequest("wrong", "[2001:db8:1:2::11]:4000"))
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("same /64 status = %d, want 429", w.Code)
	}
	w = httptest.NewRecorder()
	svc.Login().ServeHTTP(w, loginRequest("wrong", "[2001:db8:1:3::10]:4000"))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("other /64 status = %d, want 401", w.Code)
	}
}

// Behind a trusted proxy, clients are told apart by X-Forwarded-For, so one
// client's failures don't lock out every other.
func TestLoginRateLimitedPerClientBehindTrustedProxy(t *testing.T) {
	svc := testService(t)
	svc.Limiter = NewRateLimiter(1, time.Minute)
	svc.Proxies = mustProxies(t, "10.0.0.2")

	for i, tc := range []struct {
		xff  string
		want int
	}{
		{"198.51.100.1", http.StatusUnauthorized},
		{"198.51.100.1", http.StatusTooManyRequests},
		{"198.51.100.2", http.StatusUnauthorized},
	} {
		req := loginRequest("wrong", "10.0.0.2:4000")
		req.Header.Set("X-Forwarded-For", tc.xff)
		w := httptest.NewRecorder()
		svc.Login().ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("attempt %d from %s: status = %d, want %d", i+1, tc.xff, w.Code, tc.want)
		}
	}
}

// captureLog sends the standard logger's output to a buffer until the test
// ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

// A rate-limited client is logged once, when it is first refused, however
// often it keeps trying; and only its parsed address reaches the log.
func TestLoginLogsRateLimitedClientOnce(t *testing.T) {
	logs := captureLog(t)
	svc := testService(t)
	svc.Limiter = NewRateLimiter(1, time.Minute)
	svc.Proxies = mustProxies(t, "10.0.0.2")

	for range 4 {
		svc.Login().ServeHTTP(httptest.NewRecorder(), loginRequest("wrong", "203.0.113.50:4000"))
	}
	if n := strings.Count(logs.String(), "too many login attempts from 203.0.113.50"); n != 1 {
		t.Errorf("logged %d refusals, want 1:\n%s", n, logs.String())
	}

	for range 2 {
		req := loginRequest("wrong", "10.0.0.2:4000")
		req.Header.Set("X-Forwarded-For", "198.51.100.7\nINFO  forged line")
		svc.Login().ServeHTTP(httptest.NewRecorder(), req)
	}
	if out := logs.String(); strings.Contains(out, "forged") || !strings.Contains(out, "too many login attempts from 10.0.0.2") {
		t.Errorf("refusal behind the proxy should log the last trusted hop only:\n%s", out)
	}
}

// holdCheckSlots takes every password verification slot until the test ends.
func holdCheckSlots(t *testing.T) {
	t.Helper()
	for range maxConcurrentChecks {
		checkSlots <- struct{}{}
	}
	t.Cleanup(func() {
		for range maxConcurrentChecks {
			<-checkSlots
		}
	})
}

func TestLoginBusyWhenNoVerificationSlot(t *testing.T) {
	svc := testService(t)
	holdCheckSlots(t)
	prev := loginQueueTimeout
	loginQueueTimeout = 10 * time.Millisecond
	t.Cleanup(func() { loginQueueTimeout = prev })

	svc.Limiter = NewRateLimiter(1, time.Minute)
	for i := range 3 {
		w := httptest.NewRecorder()
		svc.Login().ServeHTTP(w, loginRequest("secret", "192.0.2.1:4000"))
		// Busy attempts never checked the password, so they don't use up
		// the client's budget: each one is 503, never 429.
		if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
			t.Errorf("attempt %d: status %d Retry-After %q, want 503 with Retry-After", i+1, w.Code, w.Header().Get("Retry-After"))
		}
	}
}

func TestLoginWritesNothingForGoneClientWhileBusy(t *testing.T) {
	svc := testService(t)
	holdCheckSlots(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	svc.Login().ServeHTTP(w, loginRequest("secret", "192.0.2.1:4000").WithContext(ctx))
	if w.Body.Len() != 0 {
		t.Errorf("gone client got body %q, want nothing written", w.Body.String())
	}
}

func TestSessionCookieSecureBehindTLSProxy(t *testing.T) {
	svc := testService(t)
	svc.Proxies = mustProxies(t, "10.0.0.2")

	for _, tc := range []struct {
		name   string
		remote string
		want   bool
	}{
		{"trusted proxy terminating TLS", "10.0.0.2:4000", true},
		{"untrusted peer claiming https", "203.0.113.7:4000", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := loginRequest("secret", tc.remote)
			req.Header.Set("X-Forwarded-Proto", "https")
			w := httptest.NewRecorder()
			svc.Login().ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			for _, c := range w.Result().Cookies() {
				if c.Name == cookieName && c.Secure != tc.want {
					t.Errorf("Secure = %v, want %v", c.Secure, tc.want)
				}
			}
		})
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

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dockgraph/dockgraph/auth"
)

// stubHealth implements HealthChecker for tests.
type stubHealth struct {
	err error
}

func (s *stubHealth) HealthCheck(_ context.Context) error {
	return s.err
}

func TestHealthzOK(t *testing.T) {
	hub := NewHub()
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, nil, nil, nil, nil, "")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %q", string(body))
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("expected Content-Type text/plain, got %q", ct)
	}
}

func TestHealthzDockerUnreachable(t *testing.T) {
	hub := NewHub()
	pinger := &stubHealth{err: fmt.Errorf("connection refused")}
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, pinger, nil, nil, nil, nil, nil, "")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected error body")
	}
}

func TestSPAHandlerServesStaticFiles(t *testing.T) {
	fs := fstest.MapFS{
		"index.html":       {Data: []byte("<html>app</html>")},
		"assets/style.css": {Data: []byte("body{}")},
		"assets/app.js":    {Data: []byte("console.log()")},
	}

	hub := NewHub()
	handler := NewServer(hub, fs, &stubHealth{}, nil, nil, nil, nil, nil, "")
	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		path string
		want string
	}{
		{"/", "<html>app</html>"},
		{"/assets/style.css", "body{}"},
		{"/assets/app.js", "console.log()"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := http.Get(server.URL + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if string(body) != tc.want {
				t.Errorf("expected %q, got %q", tc.want, string(body))
			}
		})
	}
}

func TestSPAHandlerFallbackToIndex(t *testing.T) {
	fs := fstest.MapFS{
		"index.html": {Data: []byte("<html>spa</html>")},
	}

	hub := NewHub()
	handler := NewServer(hub, fs, &stubHealth{}, nil, nil, nil, nil, nil, "")
	server := httptest.NewServer(handler)
	defer server.Close()

	// Request a non-existent path — should fall back to index.html
	resp, err := http.Get(server.URL + "/some/deep/route")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "<html>spa</html>" {
		t.Errorf("expected SPA fallback, got %q", string(body))
	}
}

func TestCSPHeader(t *testing.T) {
	hub := NewHub()
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, nil, nil, nil, nil, "")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected Content-Security-Policy header")
	}
}

func TestSecurityHeadersComplete(t *testing.T) {
	hub := NewHub()
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, nil, nil, nil, nil, "")
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	headers := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": contentSecurityPolicy,
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
		"X-XSS-Protection":        "0",
	}
	for header, want := range headers {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

// The app connects only to its own origin, its WebSocket included, which
// 'self' covers; scheme sources such as "wss:" would allow any host.
func TestContentSecurityPolicyConnectsToOwnOriginOnly(t *testing.T) {
	directives := map[string]string{}
	for _, d := range strings.Split(contentSecurityPolicy, ";") {
		name, value, _ := strings.Cut(strings.TrimSpace(d), " ")
		directives[name] = value
	}
	if got := directives["connect-src"]; got != "'self'" {
		t.Errorf("connect-src = %q, want 'self'", got)
	}
	if got := directives["default-src"]; got != "'self'" {
		t.Errorf("default-src = %q, want 'self'", got)
	}
}

func TestNewServerRegistersAuthRoutes(t *testing.T) {
	hub := NewHub()
	fs := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	hash, _ := auth.HashPassword("secret")
	key, _ := auth.GenerateSigningKey()
	svc := &auth.Service{
		PasswordHash: hash,
		SigningKey:   key,
		Sessions:     auth.NewSessionManager(),
		Limiter:      auth.NewRateLimiter(5, time.Minute),
		LoginPage:    []byte("<html>login</html>"),
	}
	handler := NewServer(hub, fs, &stubHealth{}, svc, nil, nil, nil, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Should get 400 (bad body) not 404 — the route exists
	if w.Code == http.StatusNotFound {
		t.Error("/api/login should be registered when auth enabled")
	}
}

func newAuthServer(t *testing.T) http.Handler {
	t.Helper()
	hash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	key, err := auth.GenerateSigningKey()
	if err != nil {
		t.Fatal(err)
	}
	svc := &auth.Service{
		PasswordHash: hash,
		SigningKey:   key,
		Sessions:     auth.NewSessionManager(),
		Limiter:      auth.NewRateLimiter(5, time.Minute),
		LoginPage:    []byte("<html>login</html>"),
		LoginScript:  []byte("/* login */"),
	}
	fs := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	return NewServer(NewHub(), fs, &stubHealth{}, svc, nil, nil, nil, nil, "")
}

// The login page, its script and 401s are written by the auth layer, outside
// the routes: they must still carry the security headers.
func TestSecurityHeadersOnUnauthenticatedResponses(t *testing.T) {
	handler := newAuthServer(t)
	for _, path := range []string{"/", "/api/containers/web", auth.LoginScriptPath} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if got := w.Header().Get("Content-Security-Policy"); got != contentSecurityPolicy {
			t.Errorf("%s: Content-Security-Policy = %q", path, got)
		}
		if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q", path, got)
		}
	}
}

func TestLoginScriptServedWithoutSession(t *testing.T) {
	handler := newAuthServer(t)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, auth.LoginScriptPath, nil))
	if w.Code != http.StatusOK || w.Body.String() != "/* login */" {
		t.Fatalf("status %d body %q, want 200 and the script", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", ct)
	}
}

func TestCrossOriginPostRejected(t *testing.T) {
	handler := newAuthServer(t)
	for _, tc := range []struct {
		name   string
		header string
		value  string
		want   int
	}{
		{"cross-site fetch", "Sec-Fetch-Site", "cross-site", http.StatusForbidden},
		{"sibling subdomain", "Sec-Fetch-Site", "same-site", http.StatusForbidden},
		{"same origin", "Sec-Fetch-Site", "same-origin", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
			req.Header.Set(tc.header, tc.value)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			// Logout needs a session, so an allowed request reaches the auth
			// layer and gets 401; a rejected one stops at 403 before it.
			got := w.Code
			if got == http.StatusUnauthorized {
				got = http.StatusOK
			}
			if got != tc.want {
				t.Errorf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestNewServerNoAuthRoutes(t *testing.T) {
	hub := NewHub()
	fs := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	handler := NewServer(hub, fs, &stubHealth{}, nil, nil, nil, nil, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Without auth, /api/login falls through to SPA handler
	body, _ := io.ReadAll(w.Body)
	if string(body) != "<html>app</html>" {
		t.Error("without auth, /api/login should fall through to SPA")
	}
}

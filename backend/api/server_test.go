package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, nil)
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
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, pinger, nil, nil)
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
	handler := NewServer(hub, fs, &stubHealth{}, nil, nil)
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
	handler := NewServer(hub, fs, &stubHealth{}, nil, nil)
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
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, nil)
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
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, nil)
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
		"Content-Security-Policy": "default-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:",
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
	handler := NewServer(hub, fs, &stubHealth{}, svc, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Should get 400 (bad body) not 404 — the route exists
	if w.Code == http.StatusNotFound {
		t.Error("/api/login should be registered when auth enabled")
	}
}

func TestNewServerNoAuthRoutes(t *testing.T) {
	hub := NewHub()
	fs := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	handler := NewServer(hub, fs, &stubHealth{}, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Without auth, /api/login falls through to SPA handler
	body, _ := io.ReadAll(w.Body)
	if string(body) != "<html>app</html>" {
		t.Error("without auth, /api/login should fall through to SPA")
	}
}

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// stubPinger implements just the Ping method for health check tests.
type stubPinger struct {
	client.APIClient
	err error
}

func (s *stubPinger) Ping(_ context.Context) (types.Ping, error) {
	return types.Ping{}, s.err
}

func TestHealthzOK(t *testing.T) {
	hub := NewHub()
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubPinger{})
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
	pinger := &stubPinger{err: fmt.Errorf("connection refused")}
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, pinger)
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
		"index.html":        {Data: []byte("<html>app</html>")},
		"assets/style.css":  {Data: []byte("body{}")},
		"assets/app.js":     {Data: []byte("console.log()")},
	}

	hub := NewHub()
	handler := NewServer(hub, fs, &stubPinger{})
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
	handler := NewServer(hub, fs, &stubPinger{})
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
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubPinger{})
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
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubPinger{})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	headers := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":        "DENY",
		"Content-Security-Policy": "default-src 'self'; style-src 'self' 'unsafe-inline'",
	}
	for header, want := range headers {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

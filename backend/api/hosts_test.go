package api

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
	"unicode/utf8"
)

func TestHostAllowlistDefaults(t *testing.T) {
	var allow HostAllowlist // nothing listed
	for _, tc := range []struct {
		host string
		want bool
	}{
		{"127.0.0.1:7800", true},
		{"192.168.1.20:7800", true},
		{"192.168.1.20", true},
		{"[::1]:7800", true},
		{"[::1]", true},
		{"[fe80::1%25eth0]:7800", true}, // an address with a zone is still a literal
		{"localhost:7800", true},
		{"LOCALHOST", true},
		{"localhost.:7800", true},
		{"app.localhost:7800", true},
		{"attacker.example:7800", false},
		{"localhost.attacker.example", false},
		{"nas.local:7800", false},
		{"dockgraph:7800", false},
		{"", false},
	} {
		if got := allow.Allows(tc.host); got != tc.want {
			t.Errorf("Allows(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestParseAllowedHosts(t *testing.T) {
	allow, err := ParseAllowedHosts(" dockgraph.example.com, .lab.example, *.home.example ,nas.local., DOCKGRAPH ")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		host string
		want bool
	}{
		{"dockgraph.example.com", true},
		{"dockgraph.example.com:443", true},
		{"other.example.com", false},
		{"lab.example", true},
		{"a.b.lab.example:7800", true},
		{"evillab.example", false},
		{"home.example", true},
		{"x.home.example", true},
		{"nas.local:7800", true},
		{"dockgraph:7800", true},
		{"10.0.0.1", true}, // defaults still apply
	} {
		if got := allow.Allows(tc.host); got != tc.want {
			t.Errorf("Allows(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}

	anyHost, err := ParseAllowedHosts("*")
	if err != nil || !anyHost.Allows("anything.example:1") {
		t.Errorf(`"*" should allow every host (err %v)`, err)
	}

	for _, bad := range []string{"example.com:7800", "http://example.com", "exa*mple.com", "user@example.com"} {
		if _, err := ParseAllowedHosts(bad); err == nil {
			t.Errorf("ParseAllowedHosts(%q) succeeded, want an error", bad)
		}
	}
}

func TestServerHostCheck(t *testing.T) {
	fs := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	allow, _ := ParseAllowedHosts("dockgraph.example.com")
	handler := NewServer(NewHub(), fs, &stubHealth{}, nil, nil, nil, nil, nil, "", WithHostAllowlist(allow))

	for _, tc := range []struct {
		host, path string
		want       int
	}{
		{"192.168.1.20:7800", "/", http.StatusOK},
		{"dockgraph.example.com", "/", http.StatusOK},
		{"attacker.example:7800", "/", http.StatusForbidden},
		{"attacker.example:7800", "/ws", http.StatusForbidden},
		{"attacker.example:7800", "/api/containers/web", http.StatusForbidden},
		{"attacker.example:7800", "/healthz", http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Host = tc.host
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("GET %s with Host %s: status = %d, want %d", tc.path, tc.host, w.Code, tc.want)
		}
		if w.Code == http.StatusForbidden {
			if !strings.Contains(w.Body.String(), "DG_ALLOWED_HOSTS") {
				t.Errorf("rejection should name DG_ALLOWED_HOSTS, got %q", w.Body.String())
			}
			if w.Header().Get("Content-Security-Policy") == "" {
				t.Error("rejection should carry the security headers")
			}
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

// Rejected hosts are logged a few per minute and the rest counted: made-up
// names can neither flood the log nor keep a name logged later out of it.
func TestRejectedHostLogIsBoundedPerInterval(t *testing.T) {
	logs := captureLog(t)
	rejected := newRejectedHostLog()
	start := time.Now()
	for i := range 2 * hostLogBurst {
		rejected.note(fmt.Sprintf("junk%d.example", i), start)
	}
	rejected.note("junk0.example", start) // already logged this interval
	rejected.note("dockgraph.lan", start.Add(time.Second))
	if n := strings.Count(logs.String(), "rejected request for host"); n != hostLogBurst {
		t.Errorf("logged %d hosts in one interval, want %d:\n%s", n, hostLogBurst, logs.String())
	}
	if strings.Contains(logs.String(), "dockgraph.lan") {
		t.Errorf("a host past the interval's lines was logged:\n%s", logs.String())
	}

	rejected.note("dockgraph.lan", start.Add(hostLogInterval))
	out := logs.String()
	if !strings.Contains(out, `rejected request for host "dockgraph.lan"`) {
		t.Errorf("the next interval should log the host again:\n%s", out)
	}
	if want := fmt.Sprintf("%d more rejected requests", hostLogBurst+1); !strings.Contains(out, want) {
		t.Errorf("the requests left unlogged should be counted (%q):\n%s", want, out)
	}
}

// Host names reach the log quoted, so they cannot forge log lines, and cut
// to the longest valid DNS name.
func TestHostCheckLogsHostQuotedAndShort(t *testing.T) {
	logs := captureLog(t)
	handler := hostCheck(&HostAllowlist{}, http.NotFoundHandler())
	for _, host := range []string{"evil.example\nINFO  forged line\x1b[2J", strings.Repeat("a", 10000) + ".example"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	out := logs.String()
	if strings.Contains(out, "\nINFO  forged") || strings.ContainsRune(out, 0x1b) {
		t.Errorf("a Host header reached the log unquoted:\n%s", out)
	}
	if strings.Count(out, "rejected request for host") != 2 || len(out) > 1000 {
		t.Errorf("want two short lines, got %d bytes:\n%.400s", len(out), out)
	}
}

// A long Host is cut between characters, and the 403 answer names it no
// longer than the log does.
func TestHostCheckCutsLongHostWhole(t *testing.T) {
	logs := captureLog(t)
	handler := hostCheck(&HostAllowlist{}, http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = strings.Repeat("a", maxLoggedHostLen-1) + strings.Repeat("é", 2000)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !utf8.ValidString(logs.String()) || strings.Contains(logs.String(), `\x`) {
		t.Errorf("log cut inside a character: %.80q", logs.String()[len(logs.String())-80:])
	}
	if w.Code != http.StatusForbidden || w.Body.Len() > 2*maxLoggedHostLen {
		t.Errorf("status %d, %d-byte body; want 403 with the host cut", w.Code, w.Body.Len())
	}
}

// Only a host that is cut gets the ellipsis.
func TestShortHost(t *testing.T) {
	limit := strings.Repeat("a", maxLoggedHostLen)
	tests := []struct {
		name, host, want string
	}{
		{"short", "example.com", "example.com"},
		{"at the limit", limit, limit},
		{"one byte over", limit + "b", limit + "..."},
		{"cut before a character", strings.Repeat("a", maxLoggedHostLen-1) + "éé", strings.Repeat("a", maxLoggedHostLen-1) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortHost(tt.host); got != tt.want {
				t.Errorf("shortHost = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServerWithoutHostCheckServesAnyHost(t *testing.T) {
	fs := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	handler := NewServer(NewHub(), fs, &stubHealth{}, nil, nil, nil, nil, nil, "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "anything.example"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

package api

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/dockgraph/dockgraph/collector"
)

// HostAllowlist decides which Host headers the server answers, so that a
// page served from a foreign DNS name that later resolves to this server
// (DNS rebinding) cannot read it as its own origin: the browser keeps sending
// the foreign name as Host.
//
// IP literals, localhost and *.localhost are always allowed: a browser
// resolves them without asking DNS, so no foreign page can take them over.
// Any other name must be listed in DG_ALLOWED_HOSTS: an exact name, or a
// leading dot (".example.com") for the name and its subdomains. "*" allows
// every host.
type HostAllowlist struct {
	any      bool
	exact    map[string]bool
	suffixes []string // ".example.com"
}

// ParseAllowedHosts parses a comma-separated DG_ALLOWED_HOSTS value.
func ParseAllowedHosts(s string) (*HostAllowlist, error) {
	h := &HostAllowlist{exact: make(map[string]bool)}
	for _, field := range strings.Split(s, ",") {
		name := normalizeHost(strings.TrimSpace(field))
		if strings.HasPrefix(name, "*.") {
			name = name[1:] // "*.example.com" reads as ".example.com"
		}
		switch {
		case name == "":
			continue
		case name == "*":
			h.any = true
		case strings.ContainsAny(name, "*/:@ "):
			return nil, fmt.Errorf("invalid host %q: use a host name without port, or a leading dot for subdomains", field)
		case strings.HasPrefix(name, "."):
			h.suffixes = append(h.suffixes, name)
		default:
			h.exact[name] = true
		}
	}
	return h, nil
}

// Allows reports whether hostport (a Host header, with or without port) is
// served.
func (h *HostAllowlist) Allows(hostport string) bool {
	if h.any {
		return true
	}
	host := hostOnly(hostport)
	if host == "" {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if h.exact[host] {
		return true
	}
	for _, suffix := range h.suffixes {
		if host == suffix[1:] || strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// hostOnly strips the port and IPv6 brackets from a Host header and
// normalises the name.
func hostOnly(hostport string) string {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	} else if strings.HasPrefix(hostport, "[") && strings.HasSuffix(hostport, "]") {
		host = hostport[1 : len(hostport)-1]
	}
	return normalizeHost(host)
}

// normalizeHost lower-cases a host name and drops a trailing root dot
// ("example.com." is "example.com").
func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// At most hostLogBurst rejected hosts are logged per hostLogInterval, each
// once; further rejections are only counted. Host headers are chosen by the
// client: made-up names must not flood the log, nor, once a burst of them
// has passed, keep out the name an operator forgot to list.
const (
	hostLogInterval = time.Minute
	hostLogBurst    = 5
	// maxLoggedHostLen is the longest valid DNS name; a longer Host is cut
	// to it in the log and the 403 answer.
	maxLoggedHostLen = 253
)

// rejectedHostLog logs the hosts hostCheck refuses, within the bounds above.
type rejectedHostLog struct {
	mu         sync.Mutex
	since      time.Time       // start of the current interval
	logged     map[string]bool // hosts logged in the current interval
	suppressed int             // requests left unlogged in the current interval
}

func newRejectedHostLog() *rejectedHostLog {
	return &rejectedHostLog{logged: make(map[string]bool, hostLogBurst)}
}

// note logs a request for host rejected at now, unless host was already
// logged in this interval or the interval's lines are used up. The first
// request of a new interval also reports how many went unlogged before it.
func (l *rejectedHostLog) note(host string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.since) >= hostLogInterval {
		if l.suppressed > 0 {
			log.Printf("WARN  %d more rejected requests for hosts not in DG_ALLOWED_HOSTS were not logged (at most %d hosts are logged per minute)", l.suppressed, hostLogBurst)
		}
		l.since, l.suppressed = now, 0
		clear(l.logged)
	}
	switch {
	case l.logged[host]:
	case len(l.logged) >= hostLogBurst:
		l.suppressed++
	default:
		l.logged[host] = true
		log.Printf("WARN  rejected request for host %q: add it to DG_ALLOWED_HOSTS to serve it", host)
	}
}

// shortHost cuts host, for the log and the 403 answer, to maxLoggedHostLen
// bytes, between characters.
func shortHost(host string) string {
	if cut := collector.CapText(host, maxLoggedHostLen); len(cut) < len(host) {
		return cut + "..."
	}
	return host
}

// hostCheck answers 403 to requests whose Host the allowlist does not serve.
// /healthz is exempt: it reveals nothing, and monitors often reach the
// server by a name nobody thought to list.
func hostCheck(allow *HostAllowlist, next http.Handler) http.Handler {
	rejected := newRejectedHostLog()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && !allow.Allows(r.Host) {
			host := shortHost(hostOnly(r.Host))
			rejected.note(host, time.Now())
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "Host %q is not allowed. To serve it, add it to DG_ALLOWED_HOSTS.\n", host)
			return
		}
		next.ServeHTTP(w, r)
	})
}

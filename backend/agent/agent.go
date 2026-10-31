// Package agent implements the per-node dockgraph agent (DG_MODE=agent).
// Deployed as a global swarm service, it exposes the local daemon's
// container stats, inspect data and logs, and its volumes, over an
// authenticated HTTP API so the swarm server can cover every node.
package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dockgraph/dockgraph/agentauth"
	"github.com/dockgraph/dockgraph/api"
	"github.com/dockgraph/dockgraph/collector"
)

// infoTimeout bounds the Docker Info call behind /agent/v1/info.
const infoTimeout = 5 * time.Second

// rejectLogInterval is how often, at most, the agent logs rejected requests
// for a given reason: a misconfigured server is rejected on every poll.
const rejectLogInterval = 5 * time.Minute

// ErrNoCredentials is returned by New when neither the server's public key
// nor a token is configured.
var ErrNoCredentials = errors.New("agent requires the server's public key: set DG_AGENT_PUBLIC_KEY_FILE (or the older DG_AGENT_TOKEN)")

// Config configures an agent Server.
type Config struct {
	// PublicKeys are the server's public keys. When set, every /agent/v1
	// request must be signed by one of them, and Token is ignored.
	PublicKeys []ed25519.PublicKey
	// Token is the older shared bearer secret, required of every /agent/v1
	// request when no public key is configured.
	Token string
	// StatsInterval is how often local container stats are sampled.
	StatsInterval time.Duration
	// StatsWorkers bounds concurrent Docker stats calls.
	StatsWorkers int
	// Health backs GET /healthz; nil reports healthy.
	Health api.HealthChecker
}

// Server is the agent HTTP API plus its background stats sampler.
type Server struct {
	cli       collector.DockerClient
	cfg       Config
	verifier  *agentauth.Verifier // nil when the token is checked instead
	tokenHash [sha256.Size]byte

	mu      sync.RWMutex
	samples []collector.ContainerSample
	info    collector.AgentInfo // cached once a node ID is known

	rejectMu     sync.Mutex
	rejectLogged map[error]time.Time
}

// New creates an agent server. It refuses to start without credentials:
// the server's public keys or, failing those, a token.
func New(cli collector.DockerClient, cfg Config) (*Server, error) {
	s := &Server{
		cli:          cli,
		samples:      []collector.ContainerSample{},
		rejectLogged: make(map[error]time.Time),
	}
	switch {
	case len(cfg.PublicKeys) > 0:
		s.verifier = agentauth.NewVerifier(cfg.PublicKeys)
	case strings.TrimSpace(cfg.Token) != "":
		s.tokenHash = sha256.Sum256([]byte(cfg.Token))
	default:
		return nil, ErrNoCredentials
	}
	if cfg.StatsInterval <= 0 {
		cfg.StatsInterval = 3 * time.Second
	}
	if cfg.StatsWorkers < 1 {
		cfg.StatsWorkers = 1
	}
	s.cfg = cfg
	return s, nil
}

// Run samples local container stats every StatsInterval until ctx is
// cancelled. The latest round is served by /agent/v1/stats.
func (s *Server) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.StatsInterval)
	defer ticker.Stop()
	for {
		samples := collector.PollSamples(ctx, s.cli, s.cfg.StatsWorkers)
		if ctx.Err() != nil {
			return
		}
		s.mu.Lock()
		s.samples = samples
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Handler returns the agent's routes. Everything under /agent/v1 requires
// the server's credentials; /healthz is open for the container healthcheck.
func (s *Server) Handler() http.Handler {
	v1 := http.NewServeMux()
	p := collector.AgentAPIPrefix
	v1.HandleFunc("GET "+p+"/info", s.handleInfo)
	v1.HandleFunc("GET "+p+"/stats", s.handleStats)
	// The api handlers validate {id} and mask sensitive env values, so the
	// agent answers exactly like the server does for local containers.
	v1.HandleFunc("GET "+p+"/containers/{id}", api.HandleContainerInspect(s.cli))
	v1.HandleFunc("GET "+p+"/containers/{id}/logs", api.HandleContainerLogs(s.cli))
	v1.HandleFunc("GET "+p+"/containers/{id}/logs/history", api.HandleContainerLogsHistory(s.cli))
	// Swarm volumes are node-local; the server asks every agent for one.
	v1.HandleFunc("GET "+p+"/volumes/{name}", api.HandleVolumeInspect(s.cli))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	if s.verifier != nil {
		mux.Handle(p+"/", s.requireSignature(v1))
	} else {
		mux.Handle(p+"/", s.requireToken(v1))
	}
	return mux
}

// Challenges sent with a 401. Bearer ones follow RFC 6750 section 3:
// without credentials the challenge names only the scheme and realm, and a
// rejected token adds error="invalid_token".
const (
	challengeNoToken      = "Bearer realm=\"dockgraph-agent\""
	challengeInvalidToken = "Bearer realm=\"dockgraph-agent\", error=\"invalid_token\""
	challengeSignature    = agentauth.SignatureHeader + " realm=\"dockgraph-agent\""
)

// requireSignature rejects requests not signed by the server (see
// agentauth.Verifier). A bearer token, if any, is ignored.
func (s *Server) requireSignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.verifier.Verify(r); err != nil {
			s.logRejected(r, err)
			w.Header().Set("WWW-Authenticate", challengeSignature)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// logRejected logs a rejected request, at most once per rejectLogInterval
// for each reason.
func (s *Server) logRejected(r *http.Request, err error) {
	s.rejectMu.Lock()
	defer s.rejectMu.Unlock()
	if last, ok := s.rejectLogged[err]; ok && time.Since(last) < rejectLogInterval {
		return
	}
	s.rejectLogged[err] = time.Now()
	log.Printf("WARN  rejected a request from %s: %v (check that the server's DG_AGENT_KEY matches this agent's DG_AGENT_PUBLIC_KEY, and the node clocks)", r.RemoteAddr, err)
}

// requireToken rejects requests without the exact bearer token. Both sides
// are hashed first so the comparison is constant-time regardless of length.
func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		got := sha256.Sum256([]byte(token))
		if !ok || subtle.ConstantTimeCompare(got[:], s.tokenHash[:]) != 1 {
			challenge := challengeInvalidToken
			if !ok {
				challenge = challengeNoToken
			}
			w.Header().Set("WWW-Authenticate", challenge)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if s.cfg.Health != nil {
		if err := s.cfg.Health.HealthCheck(r.Context()); err != nil {
			log.Printf("healthcheck failed: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "docker unreachable")
			return
		}
	}
	fmt.Fprint(w, "ok")
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	info, err := s.nodeInfo(r.Context())
	if err != nil {
		log.Printf("agent info: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "docker unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	// A node ID failure must not hide stats; the pool maps nodes via /info.
	info, _ := s.nodeInfo(r.Context())
	s.mu.RLock()
	samples := s.samples
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, collector.AgentStats{NodeID: info.NodeID, Samples: samples})
}

// nodeInfo returns the swarm node ID and hostname of the local daemon,
// cached once a node ID has been seen (it never changes for a joined node).
func (s *Server) nodeInfo(ctx context.Context) (collector.AgentInfo, error) {
	s.mu.RLock()
	cached := s.info
	s.mu.RUnlock()
	if cached.NodeID != "" {
		return cached, nil
	}

	ctx, cancel := context.WithTimeout(ctx, infoTimeout)
	defer cancel()
	raw, err := s.cli.Info(ctx)
	if err != nil {
		return collector.AgentInfo{}, err
	}
	info := collector.AgentInfo{NodeID: raw.Swarm.NodeID, Hostname: raw.Name}
	if info.NodeID != "" {
		s.mu.Lock()
		s.info = info
		s.mu.Unlock()
	}
	return info, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

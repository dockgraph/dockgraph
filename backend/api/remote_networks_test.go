package api

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dockgraph/dockgraph/collector"
)

// networkRoute serves GET /api/networks/{name} through the proxy, with a
// local handler that records whether it ran.
func networkRoute(p *AgentProxy, localHit *atomic.Bool) *http.ServeMux {
	local := func(w http.ResponseWriter, _ *http.Request) {
		localHit.Store(true)
		_, _ = w.Write([]byte(`{"name":"local"}`))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/networks/{name}", p.inspectNetwork(local))
	return mux
}

func getNetwork(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestInspectNetworkForwardsToOwner(t *testing.T) {
	rec := &agentRecorder{body: `{"name":"remote"}`}
	agent := volumeAgent(t, "mgr-b", rec)
	p := &AgentProxy{Agents: stubAgents{agent}, Token: "tok", LocalHostname: "mgr-a"}
	var hit atomic.Bool

	w := getNetwork(networkRoute(p, &hit), "/api/networks/lan?node=mgr-b")

	if hit.Load() {
		t.Error("local daemon answered for another node's network")
	}
	if w.Code != http.StatusOK || w.Body.String() != `{"name":"remote"}` {
		t.Errorf("got %d %s", w.Code, w.Body)
	}
	if rec.path != collector.AgentAPIPrefix+"/networks/lan" || rec.auth != "Bearer tok" {
		t.Errorf("agent saw path %q auth %q", rec.path, rec.auth)
	}
}

func TestInspectNetworkServedLocally(t *testing.T) {
	p := &AgentProxy{Agents: stubAgents{}, LocalHostname: "mgr-a"}
	for name, path := range map[string]string{
		"own node":     "/api/networks/lan?node=mgr-a",
		"no owner":     "/api/networks/app_net",
		"swarm-scoped": "/api/networks/app_net?node=",
	} {
		var hit atomic.Bool
		w := getNetwork(networkRoute(p, &hit), path)
		if !hit.Load() || w.Code != http.StatusOK {
			t.Errorf("%s: local = %v, status %d", name, hit.Load(), w.Code)
		}
	}
}

// An owner whose agent is unknown is an error, never a same-named local network.
func TestInspectNetworkOwnerUnreachable(t *testing.T) {
	p := &AgentProxy{Agents: stubAgents{}, LocalHostname: "mgr-a"}
	var hit atomic.Bool
	w := getNetwork(networkRoute(p, &hit), "/api/networks/lan?node=mgr-b")
	if hit.Load() || w.Code != http.StatusBadGateway {
		t.Errorf("local = %v, status %d, want 502 without local answer", hit.Load(), w.Code)
	}
}

func TestInspectNetworkRejectsBadOwner(t *testing.T) {
	p := &AgentProxy{Agents: stubAgents{}, LocalHostname: "mgr-a"}
	var hit atomic.Bool
	w := getNetwork(networkRoute(p, &hit), "/api/networks/lan?node=../x")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/dockgraph/dockgraph/agentauth"
	"github.com/dockgraph/dockgraph/collector"
)

// stubAgents lists a fixed set of remote agents.
type stubAgents []collector.RemoteAgent

func (s stubAgents) RemoteAgents() []collector.RemoteAgent { return s }

// volumeAgent starts a fake agent answering every request with status and
// body, and returns it as a remote agent on hostname.
func volumeAgent(t *testing.T, hostname string, rec *agentRecorder) collector.RemoteAgent {
	t.Helper()
	rec.ctype = "application/json"
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	return collector.RemoteAgent{Hostname: hostname, BaseURL: srv.URL}
}

// localVolumeMissing is a local daemon without the volume.
var localVolumeMissing = &stubVolumeInspector{err: cerrdefs.ErrNotFound.WithMessage("no such volume")}

// getVolume serves GET /api/volumes/{name} through the proxy and decodes
// the JSON body.
func getVolume(t *testing.T, p *AgentProxy, local VolumeInspector, name string) (int, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/volumes/{name}", p.inspectVolume(local))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/volumes/"+name, nil))
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return w.Code, body
}

// nodesOf returns the "nodes" field of a volume response.
func nodesOf(body map[string]any) []string {
	raw, _ := body["nodes"].([]any)
	nodes := make([]string, 0, len(raw))
	for _, n := range raw {
		s, _ := n.(string)
		nodes = append(nodes, s)
	}
	return nodes
}

func TestVolumeInspectFoundOnRemoteNode(t *testing.T) {
	holder := &agentRecorder{body: `{"name":"shop_data","driver":"local"}`}
	p := &AgentProxy{
		Auth:          agentauth.Token("tok"),
		LocalHostname: "manager-1",
		Agents: stubAgents{
			volumeAgent(t, "worker-1", &agentRecorder{status: http.StatusNotFound, body: `{"error":"volume not found"}`}),
			volumeAgent(t, "worker-2", holder),
		},
	}

	code, body := getVolume(t, p, localVolumeMissing, "shop_data")

	if code != http.StatusOK || body["name"] != "shop_data" || body["driver"] != "local" {
		t.Fatalf("status %d body %v", code, body)
	}
	if body["node"] != "worker-2" {
		t.Errorf("node = %v, want worker-2", body["node"])
	}
	if got := nodesOf(body); !slices.Equal(got, []string{"worker-2"}) {
		t.Errorf("nodes = %v, want [worker-2]", got)
	}
	if holder.path != "/agent/v1/volumes/shop_data" || holder.auth != "Bearer tok" {
		t.Errorf("agent got %s with auth %q", holder.path, holder.auth)
	}
}

// The server masks an agent's volume answer as it masks its own.
func TestVolumeInspectMasksAgentAnswer(t *testing.T) {
	holder := &agentRecorder{body: `{"name":"backup","driver":"cifs","scope":"local",` +
		`"labels":{"backup.password":"pw-in-label","tier":"db"},` +
		`"options":{"o":"addr=nas,password=pw-in-option","type":"cifs"},` +
		`"status":{"token":"pw-in-status","mounted":true}}`}
	p := &AgentProxy{
		Auth:          agentauth.Token("tok"),
		LocalHostname: "manager-1",
		Agents:        stubAgents{volumeAgent(t, "worker-1", holder)},
	}

	code, body := getVolume(t, p, localVolumeMissing, "backup")
	encoded, _ := json.Marshal(body)
	if code != http.StatusOK {
		t.Fatalf("status %d body %s", code, encoded)
	}
	if strings.Contains(string(encoded), "pw-in-") {
		t.Errorf("agent credentials reached the client: %s", encoded)
	}
	for _, kept := range []string{`"cifs"`, `"db"`, `"addr=nas`, `"mounted":true`} {
		if !strings.Contains(string(encoded), kept) {
			t.Errorf("%s missing from %s", kept, encoded)
		}
	}
}

// A node whose agent already had its share of client requests is skipped
// like an unreachable one.
func TestVolumeInspectBoundsRequestsPerAgent(t *testing.T) {
	oldRate, oldBurst := agentRequestRate, agentRequestBurst
	agentRequestRate, agentRequestBurst = 0.001, 1
	t.Cleanup(func() { agentRequestRate, agentRequestBurst = oldRate, oldBurst })

	holder := &agentRecorder{body: `{"name":"backup","driver":"local"}`}
	p := &AgentProxy{
		Auth:          agentauth.Token("tok"),
		LocalHostname: "manager-1",
		Agents:        stubAgents{volumeAgent(t, "worker-1", holder)},
	}

	if code, body := getVolume(t, p, localVolumeMissing, "backup"); code != http.StatusOK {
		t.Fatalf("first: status %d body %v", code, body)
	}
	if code, body := getVolume(t, p, localVolumeMissing, "backup"); code != http.StatusBadGateway {
		t.Errorf("second: status %d body %v, want 502", code, body)
	}
}

// A volume answer far larger than any volume's inspect is refused.
func TestVolumeInspectOversizedAnswerIsBadGateway(t *testing.T) {
	holder := &agentRecorder{body: `{"name":"backup","labels":{"x":"` + strings.Repeat("y", int(maxRemoteVolumeBytes)) + `"}}`}
	p := &AgentProxy{
		Auth:          agentauth.Token("tok"),
		LocalHostname: "manager-1",
		Agents:        stubAgents{volumeAgent(t, "worker-1", holder)},
	}

	code, body := getVolume(t, p, localVolumeMissing, "backup")
	if code != http.StatusBadGateway {
		t.Errorf("status %d body %v, want 502", code, body)
	}
}

func TestVolumeInspectPrefersLocalNode(t *testing.T) {
	p := &AgentProxy{
		Auth:          agentauth.Token("tok"),
		LocalHostname: "manager-1",
		Agents:        stubAgents{volumeAgent(t, "alpha", &agentRecorder{body: `{"name":"my-volume","driver":"remote-copy"}`})},
	}

	code, body := getVolume(t, p, &stubVolumeInspector{result: validVolume()}, "my-volume")

	if code != http.StatusOK || body["driver"] != "local" || body["node"] != "manager-1" {
		t.Fatalf("status %d body %v, want the local volume", code, body)
	}
	if got := nodesOf(body); !slices.Equal(got, []string{"alpha", "manager-1"}) {
		t.Errorf("nodes = %v, want every holder sorted", got)
	}
}

func TestVolumeInspectNotFoundAnywhere(t *testing.T) {
	p := &AgentProxy{
		Auth:   agentauth.Token("tok"),
		Agents: stubAgents{volumeAgent(t, "worker-1", &agentRecorder{status: http.StatusNotFound, body: `{}`})},
	}

	if code, body := getVolume(t, p, localVolumeMissing, "shop_data"); code != http.StatusNotFound {
		t.Errorf("status %d body %v, want 404", code, body)
	}
}

func TestVolumeInspectUnreachableNodeIsBadGateway(t *testing.T) {
	gone := httptest.NewServer(http.NotFoundHandler())
	gone.Close()
	p := &AgentProxy{
		Auth:   agentauth.Token("tok"),
		Agents: stubAgents{{Hostname: "worker-1", BaseURL: gone.URL}},
	}

	// The volume may be on the node that did not answer: not a plain 404.
	if code, body := getVolume(t, p, localVolumeMissing, "shop_data"); code != http.StatusBadGateway {
		t.Errorf("status %d body %v, want 502", code, body)
	}
}

func TestVolumeInspectTokenRejectedIsBadGateway(t *testing.T) {
	p := &AgentProxy{Locator: &stubLocator{rejected: true}, Agents: stubAgents{}, Auth: agentauth.Token("tok")}

	code, body := getVolume(t, p, localVolumeMissing, "shop_data")
	if msg, _ := body["error"].(string); code != http.StatusBadGateway || !strings.Contains(msg, "DG_AGENT_TOKEN") {
		t.Errorf("status %d body %v, want 502 naming the token", code, body)
	}
}

func TestVolumeInspectSlowNodeIsBounded(t *testing.T) {
	old := volumeLookupTimeout
	volumeLookupTimeout = 100 * time.Millisecond
	t.Cleanup(func() { volumeLookupTimeout = old })

	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	p := &AgentProxy{
		Auth: agentauth.Token("tok"),
		Agents: stubAgents{
			volumeAgent(t, "frozen", &agentRecorder{blockCh: block}),
			volumeAgent(t, "worker-2", &agentRecorder{body: `{"name":"shop_data"}`}),
		},
	}

	start := time.Now()
	code, body := getVolume(t, p, localVolumeMissing, "shop_data")
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("lookup took %s with a frozen node", elapsed)
	}
	if code != http.StatusOK || body["node"] != "worker-2" {
		t.Errorf("status %d body %v, want the answering node's volume", code, body)
	}
}

func TestVolumeInspectWithoutAgentsIsLocalOnly(t *testing.T) {
	var p *AgentProxy
	code, body := getVolume(t, p, &stubVolumeInspector{result: validVolume()}, "my-volume")
	if code != http.StatusOK || body["name"] != "my-volume" {
		t.Fatalf("status %d body %v", code, body)
	}
	if _, ok := body["node"]; ok {
		t.Errorf("standalone response gained a node field: %v", body)
	}
}

func TestServerRoutesVolumeInspectToAgents(t *testing.T) {
	agent := volumeAgent(t, "worker-2", &agentRecorder{body: `{"name":"shop_data"}`})
	docker := &stubDockerAPI{logger: &stubContainerLogger{}}
	handler := NewServer(NewHub(), fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, docker, nil, nil, nil, "swarm",
		WithAgentProxy(&AgentProxy{Locator: &stubLocator{}, Agents: stubAgents{agent}, Auth: agentauth.Token("tok")}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/volumes/shop_data", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status %d body %s, want the agent's volume", w.Code, w.Body)
	}
}

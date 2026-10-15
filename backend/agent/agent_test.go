package agent

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	systemtypes "github.com/docker/docker/api/types/system"
	"github.com/dockgraph/dockgraph/collector"
)

const token = "agent-secret"

// stubDocker implements the Docker calls the agent makes; any other method
// panics through the nil embedded interface.
type stubDocker struct {
	collector.DockerClient

	mu        sync.Mutex
	infoCalls int
	infoErr   error
}

func (s *stubDocker) Info(context.Context) (systemtypes.Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.infoCalls++
	if s.infoErr != nil {
		return systemtypes.Info{}, s.infoErr
	}
	return systemtypes.Info{Name: "worker-1", Swarm: swarm.Info{NodeID: "node-abc"}}, nil
}

func (s *stubDocker) ContainerList(context.Context, containertypes.ListOptions) ([]containertypes.Summary, error) {
	return []containertypes.Summary{{
		ID:     "aaaaaaaaaaaa",
		Names:  []string{"/shop_web.1.t1"},
		State:  collector.StateRunning,
		Labels: map[string]string{"com.docker.swarm.task.id": "t1", "com.docker.swarm.service.name": "shop_web"},
	}}, nil
}

func (s *stubDocker) ContainerStats(context.Context, string, bool) (containertypes.StatsResponseReader, error) {
	var raw containertypes.StatsResponse
	raw.MemoryStats.Usage = 4096
	buf, _ := json.Marshal(raw)
	return containertypes.StatsResponseReader{Body: io.NopCloser(bytes.NewReader(buf))}, nil
}

func (s *stubDocker) ContainerInspect(_ context.Context, id string) (containertypes.InspectResponse, error) {
	if id != "shop_web.1.t1" {
		return containertypes.InspectResponse{}, errors.New("not found")
	}
	return containertypes.InspectResponse{
		ContainerJSONBase: &containertypes.ContainerJSONBase{
			Name:       "/shop_web.1.t1",
			State:      &containertypes.State{Status: "running", Running: true},
			HostConfig: &containertypes.HostConfig{},
		},
		Config:          &containertypes.Config{Image: "nginx", Env: []string{"DB_PASSWORD=hunter2", "PORT=80"}},
		NetworkSettings: &containertypes.NetworkSettings{},
	}, nil
}

func (s *stubDocker) ContainerLogs(context.Context, string, containertypes.LogsOptions) (io.ReadCloser, error) {
	var buf bytes.Buffer
	for _, line := range []string{"2026-01-01T00:00:00.000000000Z hello\n", "2026-01-01T00:00:01.000000000Z world\n"} {
		hdr := [8]byte{1}
		binary.BigEndian.PutUint32(hdr[4:], uint32(len(line)))
		buf.Write(hdr[:])
		buf.WriteString(line)
	}
	return io.NopCloser(&buf), nil
}

func newTestServer(t *testing.T, cli *stubDocker) (*Server, *httptest.Server) {
	t.Helper()
	srv, err := New(cli, Config{Token: token, StatsInterval: time.Hour, StatsWorkers: 2})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts
}

func get(t *testing.T, url, bearer string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestNewRequiresToken(t *testing.T) {
	for _, tok := range []string{"", "   "} {
		if _, err := New(&stubDocker{}, Config{Token: tok}); !errors.Is(err, ErrNoToken) {
			t.Errorf("token %q: got %v, want ErrNoToken", tok, err)
		}
	}
}

func TestAuthRequired(t *testing.T) {
	_, ts := newTestServer(t, &stubDocker{})
	for _, path := range []string{"/info", "/stats", "/containers/shop_web.1.t1", "/containers/shop_web.1.t1/logs"} {
		for _, bearer := range []string{"", "wrong", token + "x"} {
			resp := get(t, ts.URL+collector.AgentAPIPrefix+path, bearer)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s with %q: status %d, want 401", path, bearer, resp.StatusCode)
			}
		}
	}
	// The healthcheck stays open.
	if resp := get(t, ts.URL+"/healthz", ""); resp.StatusCode != http.StatusOK {
		t.Errorf("healthz: %d", resp.StatusCode)
	}
}

func TestInfoCachesNodeID(t *testing.T) {
	cli := &stubDocker{}
	_, ts := newTestServer(t, cli)
	for range 3 {
		resp := get(t, ts.URL+collector.AgentAPIPrefix+"/info", token)
		var info collector.AgentInfo
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			t.Fatal(err)
		}
		if info.NodeID != "node-abc" || info.Hostname != "worker-1" {
			t.Fatalf("unexpected info %+v", info)
		}
	}
	if cli.infoCalls != 1 {
		t.Errorf("Info called %d times, want 1 (cached)", cli.infoCalls)
	}
}

func TestInfoDockerDown(t *testing.T) {
	_, ts := newTestServer(t, &stubDocker{infoErr: errors.New("down")})
	if resp := get(t, ts.URL+collector.AgentAPIPrefix+"/info", token); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", resp.StatusCode)
	}
}

func TestStatsServesLatestSamples(t *testing.T) {
	srv, ts := newTestServer(t, &stubDocker{})

	// Before the first sampling round: empty list, not null.
	resp := get(t, ts.URL+collector.AgentAPIPrefix+"/stats", token)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"samples":[]`) {
		t.Errorf("want empty samples, got %s", body)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); srv.Run(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	var stats collector.AgentStats
	for {
		resp := get(t, ts.URL+collector.AgentAPIPrefix+"/stats", token)
		stats = collector.AgentStats{}
		_ = json.NewDecoder(resp.Body).Decode(&stats)
		if len(stats.Samples) > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	if stats.NodeID != "node-abc" || len(stats.Samples) != 1 {
		t.Fatalf("unexpected stats %+v", stats)
	}
	s := stats.Samples[0]
	if s.TaskID() != "t1" || s.ServiceName() != "shop_web" || s.Stats.MemUsage != 4096 {
		t.Errorf("unexpected sample %+v", s)
	}
}

func TestContainerInspectMasksAndValidates(t *testing.T) {
	_, ts := newTestServer(t, &stubDocker{})
	resp := get(t, ts.URL+collector.AgentAPIPrefix+"/containers/shop_web.1.t1", token)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "hunter2") {
		t.Error("sensitive env value leaked")
	}

	if resp := get(t, ts.URL+collector.AgentAPIPrefix+"/containers/-bad", token); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid id: status %d, want 400", resp.StatusCode)
	}
	if resp := get(t, ts.URL+collector.AgentAPIPrefix+"/containers/missing", token); resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing: status %d, want 404", resp.StatusCode)
	}
}

func TestContainerLogsSSE(t *testing.T) {
	_, ts := newTestServer(t, &stubDocker{})
	resp := get(t, ts.URL+collector.AgentAPIPrefix+"/containers/shop_web.1.t1/logs", token)
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"line":"hello"`) || !strings.Contains(string(body), `"line":"world"`) {
		t.Errorf("unexpected SSE body %s", body)
	}

	resp = get(t, ts.URL+collector.AgentAPIPrefix+"/containers/shop_web.1.t1/logs/history?limit=1", token)
	body, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"line":"hello"`) || strings.Contains(string(body), "world") {
		t.Errorf("unexpected history body %s", body)
	}
}

func TestAuthChallenge(t *testing.T) {
	_, ts := newTestServer(t, &stubDocker{})
	url := ts.URL + collector.AgentAPIPrefix + "/info"
	for _, tc := range []struct{ bearer, want string }{
		{"", `Bearer realm="dockgraph-agent"`},
		{"wrong", `Bearer realm="dockgraph-agent", error="invalid_token"`},
	} {
		resp := get(t, url, tc.bearer)
		if got := resp.Header.Get("WWW-Authenticate"); got != tc.want {
			t.Errorf("bearer %q: WWW-Authenticate %q, want %q", tc.bearer, got, tc.want)
		}
	}
}

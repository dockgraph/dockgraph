package collector

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
)

const testAgentToken = "s3cret"

// stubResolver returns fixed addresses (host:port of httptest agents).
type stubResolver struct {
	mu    sync.Mutex
	addrs []string
	err   error
	hosts []string
}

func (r *stubResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hosts = append(r.hosts, host)
	return r.addrs, r.err
}

func (r *stubResolver) set(addrs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addrs = addrs
}

// fakeAgent serves /info and /stats like a real agent.
type fakeAgent struct {
	srv        *httptest.Server
	nodeID     string
	samples    []ContainerSample
	statsCalls atomic.Int32
}

func newFakeAgent(t *testing.T, nodeID string, samples ...ContainerSample) *fakeAgent {
	t.Helper()
	a := &fakeAgent{nodeID: nodeID, samples: samples}
	a.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testAgentToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case AgentAPIPrefix + "/info":
			_ = json.NewEncoder(w).Encode(AgentInfo{NodeID: a.nodeID, Hostname: "host-" + a.nodeID})
		case AgentAPIPrefix + "/stats":
			a.statsCalls.Add(1)
			_ = json.NewEncoder(w).Encode(AgentStats{NodeID: a.nodeID, Samples: a.samples})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(a.srv.Close)
	return a
}

func (a *fakeAgent) addr() string { return strings.TrimPrefix(a.srv.URL, "http://") }

func taskSample(id, name, service, taskID string, cpu float64, mem uint64) ContainerSample {
	return ContainerSample{
		ID:   id,
		Name: name,
		Labels: map[string]string{
			swarmTaskIDLabel:      taskID,
			swarmServiceNameLabel: service,
		},
		Stats: ContainerStats{CPUPercent: cpu, MemUsage: mem, MemLimit: 1000},
	}
}

func newTestPool(res *stubResolver, localNode string, tasks TaskLister) *AgentPool {
	return NewAgentPool(AgentPoolConfig{
		Addr:           "tasks.agent",
		Port:           "7801",
		Token:          testAgentToken,
		LocalNodeID:    localNode,
		Resolver:       res,
		Tasks:          tasks,
		RequestTimeout: 2 * time.Second,
	})
}

func TestAgentPoolSamplesSkipsLocalNode(t *testing.T) {
	local := newFakeAgent(t, "node-local", taskSample("aaaaaaaaaaaa1", "shop_web.1.t1", "shop_web", "t1", 10, 100))
	remote := newFakeAgent(t, "node-remote", taskSample("bbbbbbbbbbbb2", "shop_web.2.t2", "shop_web", "t2", 20, 200))
	res := &stubResolver{addrs: []string{local.addr(), remote.addr()}}

	p := newTestPool(res, "node-local", nil)
	p.Refresh(context.Background())

	samples := p.Samples(context.Background())
	if len(samples) != 1 || samples[0].Name != "shop_web.2.t2" {
		t.Fatalf("expected only the remote sample, got %+v", samples)
	}
	if local.statsCalls.Load() != 0 {
		t.Errorf("local agent must not be polled for stats")
	}
	if res.hosts[0] != "tasks.agent" {
		t.Errorf("resolved %q, want tasks.agent", res.hosts[0])
	}
}

func TestAgentPoolRejectsBadToken(t *testing.T) {
	a := newFakeAgent(t, "node-1")
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.agent", Port: "7801", Token: "wrong",
		Resolver: &stubResolver{addrs: []string{a.addr()}},
	})
	p.Refresh(context.Background())
	if n := len(p.remoteAgents()); n != 0 {
		t.Fatalf("agent with rejected token must be dropped, got %d", n)
	}
}

func TestAgentPoolNoAgentsFallsBack(t *testing.T) {
	res := &stubResolver{err: errors.New("no such host")}
	p := newTestPool(res, "node-local", nil)
	p.Refresh(context.Background())
	if s := p.Samples(context.Background()); len(s) != 0 {
		t.Fatalf("expected no samples, got %v", s)
	}
	if _, ok := p.LocateContainer(context.Background(), "whatever"); ok {
		t.Error("nothing should be located without agents")
	}
}

func TestAgentPoolKeepsAgentsOnTransientDNSError(t *testing.T) {
	a1 := newFakeAgent(t, "node-1")
	res := &stubResolver{addrs: []string{a1.addr()}}
	p := newTestPool(res, "", nil)
	p.Refresh(context.Background())
	if n := len(p.remoteAgents()); n != 1 {
		t.Fatalf("want 1 agent, got %d", n)
	}

	res.err = &net.DNSError{Err: "i/o timeout", Name: "tasks.agent", IsTimeout: true}
	p.Refresh(context.Background())
	if n := len(p.remoteAgents()); n != 1 {
		t.Fatalf("transient DNS error must keep known agents, got %d", n)
	}

	res.err = &net.DNSError{Err: "no such host", Name: "tasks.agent", IsNotFound: true}
	res.addrs = nil
	p.Refresh(context.Background())
	if n := len(p.remoteAgents()); n != 0 {
		t.Fatalf("NXDOMAIN must drop agents, got %d", n)
	}
}

func TestAgentPoolReresolveDropsGoneAgents(t *testing.T) {
	a1 := newFakeAgent(t, "node-1")
	a2 := newFakeAgent(t, "node-2")
	res := &stubResolver{addrs: []string{a1.addr(), a2.addr()}}
	p := newTestPool(res, "", nil)
	p.Refresh(context.Background())
	if n := len(p.remoteAgents()); n != 2 {
		t.Fatalf("want 2 agents, got %d", n)
	}
	res.set([]string{a2.addr()})
	p.Refresh(context.Background())
	agents := p.remoteAgents()
	if _, ok := agents["node-2"]; len(agents) != 1 || !ok {
		t.Fatalf("want only node-2, got %v", agents)
	}
}

func TestAgentPoolPortFromAddr(t *testing.T) {
	res := &stubResolver{addrs: []string{"10.0.0.5"}}
	var gotHost string
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.dg_agent:9000", Port: "7801", Token: testAgentToken, Resolver: res,
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotHost = r.URL.Host
			return nil, errors.New("offline")
		})},
	})
	p.Refresh(context.Background())
	if res.hosts[0] != "tasks.dg_agent" || gotHost != "10.0.0.5:9000" {
		t.Errorf("resolved %q dialled %q", res.hosts[0], gotHost)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAgentPoolLocateContainer(t *testing.T) {
	remote := newFakeAgent(t, "node-remote", taskSample("bbbbbbbbbbbbbbbb", "shop_web.2.t2", "shop_web", "t2", 1, 1))
	local := newFakeAgent(t, "node-local")
	tasks := &stubDockerClient{tasks: []swarm.Task{
		{ID: "t9", NodeID: "node-remote", Status: swarm.TaskStatus{ContainerStatus: &swarm.ContainerStatus{ContainerID: "cccccccccccccccc"}}},
		{ID: "t8", NodeID: "node-local"},
		{ID: "t7", NodeID: "node-gone"},
	}}
	p := newTestPool(&stubResolver{addrs: []string{remote.addr(), local.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	want := "http://" + remote.addr()
	cases := []struct {
		id string
		ok bool
	}{
		{"bbbbbbbbbbbbbbbb", true},   // sampled ID
		{"bbbbbbbbbbbb", true},       // ID prefix
		{"shop_web.2.t2", true},      // sampled name
		{"t2", true},                 // sampled task ID
		{"cccccccccccc", true},       // stopped container via task list
		{"shop_api.1.t9", true},      // task container name via task list
		{"shop_api.1.t8", false},     // local node
		{"t7", false},                // node without agent
		{"unknown-container", false}, // not a task
	}
	for _, c := range cases {
		got, ok := p.LocateContainer(context.Background(), c.id)
		if ok != c.ok || (ok && got != want) {
			t.Errorf("LocateContainer(%q) = %q,%v want ok=%v", c.id, got, ok, c.ok)
		}
	}
}

// Standalone container names are only unique per node: a name sampled on
// several nodes must not be routed to whichever node reported last.
func TestAgentPoolLocateContainerSameNameOnTwoNodes(t *testing.T) {
	a := newFakeAgent(t, "node-a",
		ContainerSample{ID: "aaaaaaaaaaaaaaaa", Name: "redis"},
		ContainerSample{ID: "a1a1a1a1a1a1a1a1", Name: "redis-cache-primary"})
	b := newFakeAgent(t, "node-b",
		ContainerSample{ID: "bbbbbbbbbbbbbbbb", Name: "redis"},
		ContainerSample{ID: "b1b1b1b1b1b1b1b1", Name: "worker"})
	tasks := &stubDockerClient{}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())

	urlA, urlB := "http://"+a.addr(), "http://"+b.addr()
	cases := []struct {
		id   string
		want string // "" when not located
	}{
		{"redis", ""},              // same name on both nodes: ambiguous
		{"aaaaaaaaaaaaaaaa", urlA}, // IDs stay exact
		{"bbbbbbbbbbbbbbbb", urlB},
		{"bbbbbbbbbbbb", urlB}, // ID prefix
		{"worker", urlB},       // name on a single node
		{"redis-cache-primary", urlA},
		{"redis-cache-", ""}, // prefixes match IDs, never names
	}
	// Agents answer in random order; every poll must give the same answer.
	for round := range 5 {
		p.Samples(context.Background())
		for _, c := range cases {
			got, ok := p.LocateContainer(context.Background(), c.id)
			if ok != (c.want != "") || got != c.want {
				t.Fatalf("round %d: LocateContainer(%q) = %q,%v want %q", round, c.id, got, ok, c.want)
			}
		}
	}
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if tasks.taskListCalls != 5 {
		t.Errorf("task list calls %d, want 5 (only the unmatched prefix falls back to it)", tasks.taskListCalls)
	}
}

func TestAgentPoolLocateCachedOnlyUniqueKeys(t *testing.T) {
	remote := newFakeAgent(t, "node-remote",
		taskSample("bbbbbbbbbbbbbbbb", "shop_web.2.t2", "shop_web", "t2", 1, 1),
		ContainerSample{ID: "dddddddddddddddd", Name: "redis"})
	tasks := &stubDockerClient{}
	p := newTestPool(&stubResolver{addrs: []string{remote.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	want := "http://" + remote.addr()
	for _, id := range []string{"bbbbbbbbbbbbbbbb", "bbbbbbbbbbbb", "t2", "shop_web.2.t2", "dddddddddddd"} {
		if got, ok := p.LocateCached(id); !ok || got != want {
			t.Errorf("LocateCached(%q) = %q,%v want %q", id, got, ok, want)
		}
	}
	// A standalone name may also exist locally: only the full lookup,
	// run after the local daemon was asked, resolves it.
	if _, ok := p.LocateCached("redis"); ok {
		t.Error("LocateCached must not resolve a standalone container name")
	}
	if got, ok := p.LocateContainer(context.Background(), "redis"); !ok || got != want {
		t.Errorf("LocateContainer(redis) = %q,%v want %q", got, ok, want)
	}
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if tasks.taskListCalls != 0 {
		t.Errorf("task list calls %d, want 0 for sampled containers", tasks.taskListCalls)
	}
}

// recordingTasks records the options of every TaskList call.
type recordingTasks struct {
	mu   sync.Mutex
	opts []swarm.TaskListOptions
}

func (r *recordingTasks) TaskList(_ context.Context, opts swarm.TaskListOptions) ([]swarm.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.opts = append(r.opts, opts)
	return nil, nil
}

func TestAgentPoolTaskLookupFiltersByTaskID(t *testing.T) {
	cases := []struct {
		id         string
		wantFilter string // "" for the whole task list
	}{
		{"shop_api.1.t9", "t9"}, // task container name
		{"t9", "t9"},            // task ID
		{"mistyped-name", "mistyped-name"},
		{"cccccccccccc", ""}, // container ID prefix: needs every task
		{"cccccccccccccccccccc", ""},
	}
	for _, c := range cases {
		tasks := &recordingTasks{}
		p := newTestPool(&stubResolver{}, "node-local", tasks)
		if _, ok := p.LocateContainer(context.Background(), c.id); ok {
			t.Errorf("%s: located with no tasks", c.id)
		}
		if len(tasks.opts) != 1 {
			t.Fatalf("%s: %d task list calls, want 1", c.id, len(tasks.opts))
		}
		got := tasks.opts[0].Filters.Get("id")
		if c.wantFilter == "" && len(got) != 0 || c.wantFilter != "" && !slices.Equal(got, []string{c.wantFilter}) {
			t.Errorf("%s: id filter %v, want %q", c.id, got, c.wantFilter)
		}
	}
}

func TestAgentPoolStartStop(t *testing.T) {
	a := newFakeAgent(t, "node-1")
	res := &stubResolver{addrs: []string{a.addr()}}
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.agent", Port: "7801", Token: testAgentToken, Resolver: res,
		ResolveInterval: 10 * time.Millisecond,
	})
	p.Start(context.Background())
	deadline := time.Now().Add(2 * time.Second)
	for {
		res.mu.Lock()
		n := len(res.hosts)
		res.mu.Unlock()
		if n >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("pool did not re-resolve")
		}
		time.Sleep(5 * time.Millisecond)
	}
	p.Stop()
}

// While no agent is known, the pool retries well before ResolveInterval, and
// picks agents up as soon as they register.
func TestAgentPoolRetriesSoonWithoutAgents(t *testing.T) {
	a := newFakeAgent(t, "node-1")
	res := &stubResolver{err: &net.DNSError{Err: "no such host", Name: "tasks.agent", IsNotFound: true}}
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.agent", Port: "7801", Token: testAgentToken, Resolver: res,
		ResolveInterval: time.Hour,
	})
	p.retryMin = 5 * time.Millisecond
	p.Start(context.Background())
	defer p.Stop()

	time.Sleep(20 * time.Millisecond)
	res.mu.Lock()
	res.err = nil
	res.addrs = []string{a.addr()}
	res.mu.Unlock()

	deadline := time.Now().Add(2 * time.Second)
	for p.agentCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("pool did not retry before the hour-long resolve interval")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Once agents are known, the pool waits the full ResolveInterval.
func TestAgentPoolKeepsIntervalWithAgents(t *testing.T) {
	a := newFakeAgent(t, "node-1")
	res := &stubResolver{addrs: []string{a.addr()}}
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.agent", Port: "7801", Token: testAgentToken, Resolver: res,
		ResolveInterval: time.Hour,
	})
	p.retryMin = time.Millisecond
	p.Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	p.Stop()
	res.mu.Lock()
	defer res.mu.Unlock()
	if len(res.hosts) != 1 {
		t.Errorf("resolved %d times, want 1", len(res.hosts))
	}
}

// remoteStub is a RemoteSampler returning fixed samples and reporting nodes.
type remoteStub struct {
	samples []ContainerSample
	nodes   []string
}

func (r remoteStub) Samples(context.Context) []ContainerSample { return r.samples }

func (r remoteStub) ReportingNodes() []string { return r.nodes }

func TestStatsCollectorEmitsIdleReportingNodes(t *testing.T) {
	sc := NewStatsCollector(&stubDockerClient{}, time.Second, 2)
	sc.SetLocalNode("n-local", "mgr")
	sc.SetRemote(remoteStub{nodes: []string{"idle-wrk"}})

	snap := sc.poll(context.Background())
	for _, key := range []string{"node:mgr", "node:idle-wrk"} {
		if got, ok := snap.Stats[key]; !ok || got != (ContainerStats{}) {
			t.Errorf("%s = %+v (present %v), want zero entry", key, got, ok)
		}
	}
}

func TestStatsCollectorMergesRemoteWithoutDoubleCounting(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: containerSummaries("aaaaaaaaaaaa", "shop_web.1.t1", map[string]string{
			swarmTaskIDLabel: "t1", swarmServiceNameLabel: "shop_web",
		}),
		statsFn: func(string) (containerStatsReader, error) { return fakeStatsBody(stats), nil },
	}
	sc := NewStatsCollector(cli, time.Hour, 2)
	sc.SetRemote(remoteStub{samples: []ContainerSample{
		// Same container reported by the local agent: must be ignored.
		taskSample("aaaaaaaaaaaa", "shop_web.1.t1", "shop_web", "t1", 999, 999),
		taskSample("bbbbbbbbbbbb", "shop_web.2.t2", "shop_web", "t2", 5, 300),
	}})

	snap := sc.poll(context.Background())
	local := snap.Stats["shop_web.1.t1"]
	if local.CPUPercent == 999 {
		t.Fatal("remote duplicate overrode local sample")
	}
	agg, ok := snap.Stats["shop_web"]
	if !ok {
		t.Fatalf("missing service aggregate: %v", snap.Stats)
	}
	if agg.CPUPercent != local.CPUPercent+5 || agg.MemUsage != local.MemUsage+300 {
		t.Errorf("aggregate %+v, local %+v", agg, local)
	}
	if _, ok := snap.Stats["shop_web.2.t2"]; !ok {
		t.Error("missing remote task stats")
	}
}

func TestBuildStatsSnapshotAggregates(t *testing.T) {
	snap := BuildStatsSnapshot([]ContainerSample{
		taskSample("a", "svc.1.x", "svc", "x", 10, 100),
		{
			ID: "b", Name: "svc.2.y", Labels: map[string]string{swarmServiceNameLabel: "svc"},
			Stats: ContainerStats{CPUPercent: 5, CPUThrottled: 40, MemUsage: 50, MemLimit: 1000, NetRx: 7, PIDs: 3},
		},
		{ID: "c", Name: "standalone", Stats: ContainerStats{CPUPercent: 1}},
		// A container named like a service keeps its own entry.
		{ID: "d", Name: "other", Stats: ContainerStats{CPUPercent: 2}},
		taskSample("e", "other.1.z", "other", "z", 30, 30),
	})
	agg := snap.Stats["svc"]
	want := ContainerStats{CPUPercent: 15, CPUThrottled: 40, MemUsage: 150, MemLimit: 2000, NetRx: 7, PIDs: 3}
	if agg != want {
		t.Errorf("aggregate = %+v, want %+v", agg, want)
	}
	if snap.Stats["other"].CPUPercent != 2 {
		t.Errorf("container entry clobbered by aggregate: %+v", snap.Stats["other"])
	}
	if len(snap.Stats) != 6 {
		t.Errorf("want 6 entries, got %d: %v", len(snap.Stats), snap.Stats)
	}
}

func TestPollSamplesCarriesSwarmLabels(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: containerSummaries("aaaaaaaaaaaa", "shop_web.1.t1", map[string]string{
			swarmTaskIDLabel: "t1", swarmServiceNameLabel: "shop_web", "unrelated": "x",
		}),
		statsFn: func(string) (containerStatsReader, error) { return fakeStatsBody(stats), nil },
	}
	samples := PollSamples(context.Background(), cli, 2)
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	s := samples[0]
	if s.ID != "aaaaaaaaaaaa" || s.TaskID() != "t1" || s.ServiceName() != "shop_web" {
		t.Errorf("unexpected sample %+v", s)
	}
	if _, leaked := s.Labels["unrelated"]; leaked {
		t.Error("unrelated labels must not be copied")
	}
}

// containerStatsReader shortens the stats stub signature.
type containerStatsReader = containertypes.StatsResponseReader

// containerSummaries returns one running container with the given labels.
func containerSummaries(id, name string, labels map[string]string) []containertypes.Summary {
	return []containertypes.Summary{{ID: id, Names: []string{"/" + name}, State: StateRunning, Labels: labels}}
}

func TestAgentPoolSamplesStampsNode(t *testing.T) {
	spoofed := taskSample("bbbbbbbbbbbb2", "shop_web.2.t2", "shop_web", "t2", 20, 200)
	spoofed.NodeID, spoofed.NodeHostname = "bogus", "bogus-host"
	remote := newFakeAgent(t, "node-remote", spoofed)
	p := newTestPool(&stubResolver{addrs: []string{remote.addr()}}, "node-local", nil)
	p.Refresh(context.Background())

	samples := p.Samples(context.Background())
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %+v", samples)
	}
	if samples[0].NodeID != "node-remote" || samples[0].NodeHostname != "host-node-remote" {
		t.Errorf("sample not stamped with its agent's node: %+v", samples[0])
	}
}

func TestStatsCollectorStampsLocalSamples(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: containerSummaries("aaaaaaaaaaaa", "shop_web.1.t1", map[string]string{
			swarmTaskIDLabel: "t1", swarmServiceNameLabel: "shop_web",
		}),
		statsFn: func(string) (containerStatsReader, error) { return fakeStatsBody(stats), nil },
	}

	// Without a local node (standalone), no node aggregate appears.
	sc := NewStatsCollector(cli, time.Hour, 2)
	for key := range sc.poll(context.Background()).Stats {
		if IsNodeSeries(key) {
			t.Errorf("unexpected node series %q without a local node", key)
		}
	}

	// Local node only, no remote sampler.
	sc = NewStatsCollector(cli, time.Hour, 2)
	sc.SetLocalNode("n-local", "mgr")
	snap := sc.poll(context.Background())
	local := snap.Stats["shop_web.1.t1"]
	if got, ok := snap.Stats["node:mgr"]; !ok || got != local {
		t.Errorf("node:mgr = %+v (present %v), want %+v", got, ok, local)
	}

	// Local and remote: each node gets its own aggregate.
	sc = NewStatsCollector(cli, time.Hour, 2)
	sc.SetLocalNode("n-local", "mgr")
	remote := taskSample("bbbbbbbbbbbb", "shop_web.2.t2", "shop_web", "t2", 5, 300)
	remote.NodeID, remote.NodeHostname = "n-remote", "wrk"
	sc.SetRemote(remoteStub{samples: []ContainerSample{remote}})
	snap = sc.poll(context.Background())
	if got := snap.Stats["node:mgr"]; got != local {
		t.Errorf("node:mgr = %+v, want %+v", got, local)
	}
	if got := snap.Stats["node:wrk"]; got.CPUPercent != 5 || got.MemUsage != 300 {
		t.Errorf("node:wrk = %+v", got)
	}
	if agg := snap.Stats["shop_web"]; agg.MemUsage != local.MemUsage+300 {
		t.Errorf("service aggregate polluted or wrong: %+v", agg)
	}
}

func TestBuildStatsSnapshotNodeAggregates(t *testing.T) {
	onNode := func(s ContainerSample, id, host string) ContainerSample {
		s.NodeID, s.NodeHostname = id, host
		return s
	}
	snap := BuildStatsSnapshot([]ContainerSample{
		onNode(taskSample("a", "svc.1.x", "svc", "x", 10, 100), "n1", "mgr"),
		onNode(taskSample("b", "svc.2.y", "svc", "y", 20, 200), "n2", "wrk"),
		onNode(ContainerSample{
			ID: "c", Name: "standalone",
			Stats: ContainerStats{CPUPercent: 1, CPUThrottled: 60, MemUsage: 10, MemLimit: 500, NetTx: 4, BlockRead: 9, PIDs: 2},
		}, "n1", "mgr"),
		// Unattributed sample: counted in no node.
		taskSample("d", "svc.3.z", "svc", "z", 100, 1000),
	})

	wantMgr := ContainerStats{CPUPercent: 11, CPUThrottled: 60, MemUsage: 110, MemLimit: 1500, NetTx: 4, BlockRead: 9, PIDs: 2}
	if got := snap.Stats["node:mgr"]; got != wantMgr {
		t.Errorf("node:mgr = %+v, want %+v", got, wantMgr)
	}
	wantWrk := ContainerStats{CPUPercent: 20, MemUsage: 200, MemLimit: 1000}
	if got := snap.Stats["node:wrk"]; got != wantWrk {
		t.Errorf("node:wrk = %+v, want %+v", got, wantWrk)
	}
	// The service aggregate is unaffected by node attribution.
	if got := snap.Stats["svc"]; got.CPUPercent != 130 || got.MemUsage != 1300 {
		t.Errorf("svc aggregate = %+v", got)
	}
	// 4 containers + 1 service + 2 nodes.
	if len(snap.Stats) != 7 {
		t.Errorf("want 7 entries, got %d: %v", len(snap.Stats), snap.Stats)
	}
}

func TestIsNodeSeries(t *testing.T) {
	for key, want := range map[string]bool{"node:mgr": true, "node:": true, "node": false, "web": false, "shop_web.1.x": false} {
		if got := IsNodeSeries(key); got != want {
			t.Errorf("IsNodeSeries(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestAgentPoolReportsAgentOnlyNodes(t *testing.T) {
	// An agent on a node with no other container reports no samples, yet
	// its node must still count as reporting.
	idle := newFakeAgent(t, "node-idle")
	busy := newFakeAgent(t, "node-busy", taskSample("bbbbbbbbbbbb2", "shop_web.2.t2", "shop_web", "t2", 20, 200))
	p := newTestPool(&stubResolver{addrs: []string{idle.addr(), busy.addr()}}, "node-local", nil)
	p.Refresh(context.Background())

	p.Samples(context.Background())
	if got, want := p.ReportingNodes(), []string{"host-node-busy", "host-node-idle"}; !slices.Equal(got, want) {
		t.Fatalf("ReportingNodes = %v, want %v", got, want)
	}

	snap := BuildStatsSnapshot(p.Samples(context.Background()), p.ReportingNodes()...)
	if got, ok := snap.Stats["node:host-node-idle"]; !ok || got != (ContainerStats{}) {
		t.Errorf("idle node aggregate = %+v (present %v), want zero entry", got, ok)
	}
	if got := snap.Stats["node:host-node-busy"]; got.CPUPercent != 20 || got.MemUsage != 200 {
		t.Errorf("busy node aggregate = %+v", got)
	}
}

func TestAgentPoolReportingClearedWithoutAgents(t *testing.T) {
	a := newFakeAgent(t, "node-1")
	res := &stubResolver{addrs: []string{a.addr()}}
	p := newTestPool(res, "node-local", nil)
	p.Refresh(context.Background())
	p.Samples(context.Background())
	if len(p.ReportingNodes()) != 1 {
		t.Fatalf("expected one reporting node, got %v", p.ReportingNodes())
	}

	res.set(nil)
	res.mu.Lock()
	res.err = &net.DNSError{Err: "no such host", Name: "tasks.agent", IsNotFound: true}
	res.mu.Unlock()
	p.Refresh(context.Background())
	p.Samples(context.Background())
	if got := p.ReportingNodes(); len(got) != 0 {
		t.Errorf("ReportingNodes after agents vanished = %v, want none", got)
	}
}

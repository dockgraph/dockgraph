package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/dockgraph/dockgraph/agentauth"
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
	infoCalls  atomic.Int32
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
			a.infoCalls.Add(1)
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
		Auth:           agentauth.Token(testAgentToken),
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

func TestAgentPoolRemoteAgentsSortedWithoutLocal(t *testing.T) {
	local := newFakeAgent(t, "node-a")
	c := newFakeAgent(t, "node-c")
	b := newFakeAgent(t, "node-b")
	p := newTestPool(&stubResolver{addrs: []string{c.addr(), local.addr(), b.addr()}}, "node-a", nil)
	p.Refresh(context.Background())

	got := p.RemoteAgents()
	want := []RemoteAgent{
		{Hostname: "host-node-b", BaseURL: b.srv.URL},
		{Hostname: "host-node-c", BaseURL: c.srv.URL},
	}
	if !slices.Equal(got, want) {
		t.Errorf("RemoteAgents() = %v, want %v", got, want)
	}
}

// With a key, the pool signs its requests and an agent that checks
// signatures accepts them.
func TestAgentPoolSignsRequests(t *testing.T) {
	keyText, err := agentauth.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	key, _ := agentauth.ParsePrivateKey(keyText)
	publicKeys, _ := agentauth.ParsePublicKeys(agentauth.PublicKeyText(key))
	verifier := agentauth.NewVerifier(publicKeys)

	var rejected atomic.Int32
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := verifier.Verify(r); err != nil {
			rejected.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case AgentAPIPrefix + "/info":
			_ = json.NewEncoder(w).Encode(AgentInfo{NodeID: "node-1", Hostname: "host-node-1"})
		case AgentAPIPrefix + "/stats":
			_ = json.NewEncoder(w).Encode(AgentStats{NodeID: "node-1", Samples: []ContainerSample{
				taskSample("aaaaaaaaaaaaaaaa", "shop_web.1.t1", "shop_web", "t1", 1, 1),
			}})
		}
	}))
	t.Cleanup(agent.Close)

	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.agent", Port: "7801", Auth: agentauth.NewSigner(key),
		Resolver: &stubResolver{addrs: []string{strings.TrimPrefix(agent.URL, "http://")}},
	})
	p.Refresh(context.Background())
	if s := p.Samples(context.Background()); len(s) != 1 {
		t.Fatalf("got %d samples, want 1", len(s))
	}
	if n := rejected.Load(); n != 0 {
		t.Errorf("agent rejected %d signed requests", n)
	}
	if p.CredentialsRejected() {
		t.Error("CredentialsRejected() = true for signed requests")
	}
}

func TestAgentPoolRejectsBadToken(t *testing.T) {
	a := newFakeAgent(t, "node-1")
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.agent", Port: "7801", Auth: agentauth.Token("wrong"),
		Resolver: &stubResolver{addrs: []string{a.addr()}},
	})
	p.Refresh(context.Background())
	if n := len(p.remoteEndpoints()); n != 0 {
		t.Fatalf("agent with rejected token must be dropped, got %d", n)
	}
}

func TestAgentPoolLogsTokenRejectionOnce(t *testing.T) {
	buf := captureLog(t)
	a, b := newFakeAgent(t, "node-1"), newFakeAgent(t, "node-2")
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.agent", Port: "7801", Auth: agentauth.Token("wrong"),
		Resolver: &stubResolver{addrs: []string{a.addr(), b.addr()}},
	})
	for range 3 {
		p.Refresh(context.Background())
	}

	if !p.CredentialsRejected() {
		t.Error("CredentialsRejected() = false after every agent answered 401")
	}
	if n := strings.Count(buf.String(), "rejected the server's credentials"); n != 1 {
		t.Errorf("rejection logged %d times, want once; log:\n%s", n, buf.String())
	}
	if strings.Contains(buf.String(), "status 401") {
		t.Errorf("raw per-agent 401s logged:\n%s", buf.String())
	}
}

func TestAgentPoolTokenRejectionClears(t *testing.T) {
	buf := captureLog(t)
	rejecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(rejecting.Close)
	res := &stubResolver{addrs: []string{strings.TrimPrefix(rejecting.URL, "http://")}}
	p := newTestPool(res, "node-local", nil)

	p.Refresh(context.Background())
	if !p.CredentialsRejected() {
		t.Fatal("CredentialsRejected() = false with a rejecting agent")
	}

	// The agent is redeployed with the right credentials.
	res.set([]string{newFakeAgent(t, "node-1").addr()})
	p.Refresh(context.Background())
	if p.CredentialsRejected() {
		t.Error("CredentialsRejected() = true after the agents accepted the credentials")
	}
	if !strings.Contains(buf.String(), "accept the server's credentials again") {
		t.Errorf("recovery not logged:\n%s", buf.String())
	}
}

// Rejecting agents that vanish (stack removed) did not accept anything.
func TestAgentPoolTokenRejectionGoneWithAgentsIsNotRecovery(t *testing.T) {
	buf := captureLog(t)
	rejecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(rejecting.Close)
	res := &stubResolver{addrs: []string{strings.TrimPrefix(rejecting.URL, "http://")}}
	p := newTestPool(res, "node-local", nil)
	p.Refresh(context.Background())

	res.set(nil)
	p.Refresh(context.Background())
	if p.CredentialsRejected() {
		t.Error("CredentialsRejected() = true with no agent left")
	}
	if strings.Contains(buf.String(), "accept the server's credentials again") {
		t.Errorf("recovery logged although no agent accepted the credentials:\n%s", buf.String())
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
	if n := len(p.remoteEndpoints()); n != 1 {
		t.Fatalf("want 1 agent, got %d", n)
	}

	res.err = &net.DNSError{Err: "i/o timeout", Name: "tasks.agent", IsTimeout: true}
	p.Refresh(context.Background())
	if n := len(p.remoteEndpoints()); n != 1 {
		t.Fatalf("transient DNS error must keep known agents, got %d", n)
	}

	res.err = &net.DNSError{Err: "no such host", Name: "tasks.agent", IsNotFound: true}
	res.addrs = nil
	p.Refresh(context.Background())
	if n := len(p.remoteEndpoints()); n != 0 {
		t.Fatalf("NXDOMAIN must drop agents, got %d", n)
	}
}

func TestAgentPoolReresolveDropsGoneAgents(t *testing.T) {
	a1 := newFakeAgent(t, "node-1")
	a2 := newFakeAgent(t, "node-2")
	res := &stubResolver{addrs: []string{a1.addr(), a2.addr()}}
	p := newTestPool(res, "", nil)
	p.Refresh(context.Background())
	if n := len(p.remoteEndpoints()); n != 2 {
		t.Fatalf("want 2 agents, got %d", n)
	}
	res.set([]string{a2.addr()})
	p.Refresh(context.Background())
	agents := p.remoteEndpoints()
	if _, ok := agents["node-2"]; len(agents) != 1 || !ok {
		t.Fatalf("want only node-2, got %v", agents)
	}
}

func TestAgentPoolPortFromAddr(t *testing.T) {
	res := &stubResolver{addrs: []string{"10.0.0.5"}}
	var gotHost string
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.dg_agent:9000", Port: "7801", Auth: agentauth.Token(testAgentToken), Resolver: res,
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

// agentTask is the running task of a fake agent on nodeID, as the swarm
// lists it: its network attachment holds the agent's address. Fake agents
// all listen on 127.0.0.1.
func agentTask(nodeID string) swarm.Task {
	return swarm.Task{
		ID:     "agent-" + nodeID,
		NodeID: nodeID,
		NetworksAttachments: []swarm.NetworkAttachment{
			{Addresses: []string{"127.0.0.1/8"}},
		},
	}
}

// runningTask is a task of the swarm with its container.
func runningTask(id, nodeID, containerID string) swarm.Task {
	return swarm.Task{ID: id, NodeID: nodeID, Status: swarm.TaskStatus{ContainerStatus: &swarm.ContainerStatus{ContainerID: containerID}}}
}

func TestAgentPoolLocateContainer(t *testing.T) {
	remote := newFakeAgent(t, "node-remote", taskSample("bbbbbbbbbbbbbbbb", "shop_web.2.t2", "shop_web", "t2", 1, 1))
	local := newFakeAgent(t, "node-local")
	tasks := &stubDockerClient{tasks: []swarm.Task{
		agentTask("node-remote"),
		agentTask("node-local"),
		runningTask("t2", "node-remote", "bbbbbbbbbbbbbbbb"),
		runningTask("t9", "node-remote", "cccccccccccccccc"),
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
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-a"), agentTask("node-b")}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	tasks.mu.Lock()
	tasks.taskListCalls = 0 // the refresh verified the agents' nodes
	tasks.mu.Unlock()

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
	// None of these is a sampled task, so each lookup asks the swarm once
	// whether it runs the container before an agent's report places it.
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if want := 5 * len(cases); tasks.taskListCalls != want {
		t.Errorf("task list calls %d, want %d (one per lookup)", tasks.taskListCalls, want)
	}
}

// A task container is placed where the swarm runs its task, whichever agent
// reports it; standalone containers are placed by their agent's report.
func TestAgentPoolLocatesTaskContainersThroughSwarm(t *testing.T) {
	a := newFakeAgent(t, "node-a",
		// Reported by node-a, but the swarm runs task t2 on node-b.
		taskSample("bbbbbbbbbbbbbbbb", "shop_web.2.t2", "shop_web", "t2", 1, 1),
		ContainerSample{ID: "dddddddddddddddd", Name: "redis"})
	b := newFakeAgent(t, "node-b")
	tasks := &stubDockerClient{tasks: []swarm.Task{
		agentTask("node-a"),
		agentTask("node-b"),
		runningTask("t2", "node-b", "bbbbbbbbbbbbbbbb"),
	}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	urlA, urlB := "http://"+a.addr(), "http://"+b.addr()
	for _, id := range []string{"bbbbbbbbbbbbbbbb", "bbbbbbbbbbbb", "t2", "shop_web.2.t2"} {
		if got, ok := p.LocateContainer(context.Background(), id); !ok || got != urlB {
			t.Errorf("LocateContainer(%q) = %q,%v want node-b %q", id, got, ok, urlB)
		}
	}
	for _, id := range []string{"redis", "dddddddddddd"} {
		if got, ok := p.LocateContainer(context.Background(), id); !ok || got != urlA {
			t.Errorf("LocateContainer(%q) = %q,%v want node-a %q", id, got, ok, urlA)
		}
	}
}

// A task never changes node, so a sampled task container's placement is
// read from the swarm once, not on every request (log pages, inspect).
func TestAgentPoolCachesTaskPlacement(t *testing.T) {
	a := newFakeAgent(t, "node-a", taskSample("bbbbbbbbbbbbbbbb", "shop_web.2.t2", "shop_web", "t2", 1, 1))
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-a"), runningTask("t2", "node-a", "bbbbbbbbbbbbbbbb")}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())
	tasks.mu.Lock()
	tasks.taskListCalls = 0
	tasks.mu.Unlock()

	for range 3 {
		if _, ok := p.LocateContainer(context.Background(), "shop_web.2.t2"); !ok {
			t.Fatal("not located")
		}
	}
	tasks.mu.Lock()
	calls := tasks.taskListCalls
	tasks.mu.Unlock()
	if calls != 1 {
		t.Errorf("task list calls %d, want 1", calls)
	}

	// A task gone from the samples is dropped from the cache.
	a.samples = nil
	p.Samples(context.Background())
	p.mu.RLock()
	n := len(p.placements)
	p.mu.RUnlock()
	if n != 0 {
		t.Errorf("%d cached placements after the task left, want 0", n)
	}
}

// A reported task whose swarm task holds another container is not located.
func TestAgentPoolTaskContainerMismatchNotLocated(t *testing.T) {
	a := newFakeAgent(t, "node-a", taskSample("eeeeeeeeeeeeeeee", "shop_web.2.t2", "shop_web", "t2", 1, 1))
	tasks := &stubDockerClient{tasks: []swarm.Task{
		agentTask("node-a"),
		runningTask("t2", "node-a", "ffffffffffffffff"),
	}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())
	if got, ok := p.LocateContainer(context.Background(), "eeeeeeeeeeeeeeee"); ok {
		t.Errorf("LocateContainer = %q, want not located", got)
	}
}

// A task container's name ends in its task ID ({service}.{slot}.{taskID}),
// so a report pairing the name with another task does not place it.
func TestAgentPoolTaskNameMustEndInItsTaskID(t *testing.T) {
	a := newFakeAgent(t, "node-a", taskSample("aaaaaaaaaaaaaaaa", "shop_web.2.t2", "shop_web", "tA", 1, 1))
	b := newFakeAgent(t, "node-b") // shop_web.2.t2 is not sampled there (stopped, or not yet)
	tasks := &stubDockerClient{tasks: []swarm.Task{
		agentTask("node-a"),
		agentTask("node-b"),
		runningTask("tA", "node-a", "aaaaaaaaaaaaaaaa"),
		runningTask("t2", "node-b", "bbbbbbbbbbbbbbbb"),
	}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	if got, ok := p.LocateContainer(context.Background(), "shop_web.2.t2"); !ok || got != "http://"+b.addr() {
		t.Errorf("LocateContainer = %q,%v want node-b, where the swarm runs t2", got, ok)
	}
}

// A name or task ID reported in a sample's ID field is held to the same rule:
// it must name the sample's task.
func TestAgentPoolTaskNameInIDFieldMustEndInItsTaskID(t *testing.T) {
	for _, id := range []string{"shop_web.2.t2", "t2"} {
		t.Run(id, func(t *testing.T) {
			a := newFakeAgent(t, "node-a", taskSample(id, "shop_web.1.tA", "shop_web", "tA", 1, 1))
			b := newFakeAgent(t, "node-b")
			tasks := &stubDockerClient{tasks: []swarm.Task{
				agentTask("node-a"),
				agentTask("node-b"),
				runningTask("tA", "node-a", "aaaaaaaaaaaaaaaa"),
				runningTask("t2", "node-b", "bbbbbbbbbbbbbbbb"),
			}}
			p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
			p.Refresh(context.Background())
			p.Samples(context.Background())

			if got, ok := p.LocateContainer(context.Background(), id); !ok || got != "http://"+b.addr() {
				t.Errorf("LocateContainer(%q) = %q,%v want node-b, where the swarm runs t2", id, got, ok)
			}
		})
	}
}

// A task list that can't be read is asked once per lookup, not again for
// the fallback.
func TestAgentPoolTaskListErrorAskedOnce(t *testing.T) {
	a := newFakeAgent(t, "node-a", taskSample("aaaaaaaaaaaaaaaa", "shop_web.1.tA", "shop_web", "tA", 1, 1))
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-a")}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	tasks.mu.Lock()
	tasks.taskErr = errors.New("manager unavailable")
	tasks.taskListCalls = 0
	tasks.mu.Unlock()
	if got, ok := p.LocateContainer(context.Background(), "shop_web.1.tA"); ok {
		t.Errorf("LocateContainer = %q, want not located while the task list fails", got)
	}
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	if tasks.taskListCalls != 1 {
		t.Errorf("task list calls %d, want 1", tasks.taskListCalls)
	}
}

// A container ID reported for a task the swarm runs with another container
// is placed by the swarm.
func TestAgentPoolContainerOfAnotherTaskPlacedBySwarm(t *testing.T) {
	a := newFakeAgent(t, "node-a", taskSample("bbbbbbbbbbbbbbbb", "shop_web.1.tA", "shop_web", "tA", 1, 1))
	b := newFakeAgent(t, "node-b")
	tasks := &stubDockerClient{tasks: []swarm.Task{
		agentTask("node-a"),
		agentTask("node-b"),
		runningTask("tA", "node-a", "aaaaaaaaaaaaaaaa"),
		runningTask("t2", "node-b", "bbbbbbbbbbbbbbbb"),
	}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	if got, ok := p.LocateContainer(context.Background(), "bbbbbbbbbbbbbbbb"); !ok || got != "http://"+b.addr() {
		t.Errorf("LocateContainer = %q,%v want node-b, where the swarm runs this container", got, ok)
	}
}

// A standalone container is located only when its ID, name and ID prefix
// all point at one node: one node reporting another's container name as
// an ID, or its ID prefix as an ID or a name, makes it ambiguous rather
// than routing it there.
func TestAgentPoolStandaloneClaimsMustAgree(t *testing.T) {
	const webID = "5b5b5b5b5b5b0000000000000000000000000000000000000000000000000000"
	const dbID = "5c5c5c5c5c5c0000000000000000000000000000000000000000000000000000"
	a := newFakeAgent(t, "node-a",
		ContainerSample{ID: "web", Name: "decoy-1"},
		ContainerSample{ID: "5b5b5b5b5b5b", Name: "decoy-2"},
		ContainerSample{ID: "a0a0a0a0a0a0a0a0", Name: "5c5c5c5c5c5c"})
	b := newFakeAgent(t, "node-b",
		ContainerSample{ID: webID, Name: "web"},
		ContainerSample{ID: dbID, Name: "db"})
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-a"), agentTask("node-b")}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	urlB := "http://" + b.addr()
	for _, c := range []struct{ id, want string }{
		{"web", ""},          // a name on node-b, an ID on node-a
		{"5b5b5b5b5b5b", ""}, // a prefix on node-b, an ID on node-a
		{"5c5c5c5c5c5c", ""}, // a prefix on node-b, a name on node-a
		{webID, urlB},        // full IDs stay exact
		{"db", urlB},
	} {
		if got, ok := p.LocateContainer(context.Background(), c.id); ok != (c.want != "") || got != c.want {
			t.Errorf("LocateContainer(%q) = %q,%v want %q", c.id, got, ok, c.want)
		}
	}
}

// Two tasks on different nodes claiming one container leave it unplaced:
// a task's container is what its node reports to the swarm.
func TestAgentPoolTaskContainerClaimedByTwoNodesNotLocated(t *testing.T) {
	a := newFakeAgent(t, "node-a")
	b := newFakeAgent(t, "node-b")
	tasks := &stubDockerClient{tasks: []swarm.Task{
		agentTask("node-a"),
		agentTask("node-b"),
		runningTask("tA", "node-a", "b0b0b0b0b0b0b1"),
		runningTask("tB", "node-b", "b0b0b0b0b0b0b1"),
		runningTask("tC", "node-b", "c0c0c0c0c0c0c1"),
	}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	if got, ok := p.LocateContainer(context.Background(), "b0b0b0b0b0b0b1"); ok {
		t.Errorf("container claimed by two nodes located at %q", got)
	}
	if got, ok := p.LocateContainer(context.Background(), "c0c0c0c0c0c0"); !ok || got != "http://"+b.addr() {
		t.Errorf("LocateContainer = %q,%v want node-b", got, ok)
	}
}

// A task's container is what its node tells the swarm. When another node
// reports that container as its own standalone one, nothing tells which is
// right, so it isn't located.
func TestAgentPoolTaskClaimContradictedByStandaloneReport(t *testing.T) {
	const webID = "5b5b5b5b5b5b0000000000000000000000000000000000000000000000000000"
	a := newFakeAgent(t, "node-a")
	b := newFakeAgent(t, "node-b", ContainerSample{ID: webID, Name: "web"})
	tasks := &stubDockerClient{tasks: []swarm.Task{
		agentTask("node-a"),
		agentTask("node-b"),
		runningTask("tA", "node-a", webID),
	}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	for _, id := range []string{webID, webID[:12]} {
		if got, ok := p.LocateContainer(context.Background(), id); ok {
			t.Errorf("LocateContainer(%q) = %q, want not located", id, got)
		}
	}
	if got, ok := p.LocateContainer(context.Background(), "web"); !ok || got != "http://"+b.addr() {
		t.Errorf(`LocateContainer("web") = %q,%v want node-b`, got, ok)
	}
}

// Stats for a container or task reported by several nodes are dropped: a
// container runs on one node, so all but one report is wrong, and which one
// is unknown.
func TestAgentPoolSamplesDropsReportsClaimedByTwoNodes(t *testing.T) {
	a := newFakeAgent(t, "node-a",
		ContainerSample{ID: "aaaaaaaaaaaaaaaa", Name: "web", Stats: ContainerStats{CPUPercent: 99}},
		taskSample("a1a1a1a1a1a1a1a1", "shop_db.1.t1", "shop_db", "t1", 99, 1),
		ContainerSample{ID: "a2a2a2a2a2a2a2a2", Name: "only-on-a"})
	b := newFakeAgent(t, "node-b",
		ContainerSample{ID: "aaaaaaaaaaaaaaaa", Name: "web", Stats: ContainerStats{CPUPercent: 1}},
		taskSample("b1b1b1b1b1b1b1b1", "shop_db.1.t1", "shop_db", "t1", 1, 1),
		ContainerSample{ID: "b2b2b2b2b2b2b2b2", Name: "only-on-b"})
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-a"), agentTask("node-b")}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())

	var names []string
	for _, s := range p.Samples(context.Background()) {
		names = append(names, s.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"only-on-a", "only-on-b"}) {
		t.Errorf("samples %v, want only those a single node reports", names)
	}
}

// An agent reporting more containers than a node runs is cut to a bound.
func TestAgentPoolCapsSamplesPerAgent(t *testing.T) {
	var many []ContainerSample
	for i := range maxAgentSamples + 10 {
		many = append(many, ContainerSample{ID: fmt.Sprintf("%016x", i), Name: fmt.Sprint("c", i)})
	}
	a := newFakeAgent(t, "node-a", many...)
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-a")}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())

	if got := len(p.Samples(context.Background())); got != maxAgentSamples {
		t.Errorf("%d samples, want %d", got, maxAgentSamples)
	}
}

// A stats answer over the size bound is refused whole.
func TestAgentPoolRefusesOversizedStats(t *testing.T) {
	old := maxAgentStatsBytes
	maxAgentStatsBytes = 256
	t.Cleanup(func() { maxAgentStatsBytes = old })

	var many []ContainerSample
	for i := range 20 {
		many = append(many, ContainerSample{ID: fmt.Sprintf("%016x", i), Name: fmt.Sprint("c", i)})
	}
	a := newFakeAgent(t, "node-a", many...)
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-a")}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())

	if got := p.Samples(context.Background()); len(got) != 0 {
		t.Errorf("%d samples from an oversized answer, want none", len(got))
	}
}

// The pool never follows an agent's redirect: an agent can't make the
// server request another address.
func TestAgentPoolDoesNotFollowRedirects(t *testing.T) {
	var elsewhere atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere.Add(1) }))
	t.Cleanup(target.Close)
	a := newFakeAgent(t, "node-a")
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == AgentAPIPrefix+"/stats" {
			http.Redirect(w, r, target.URL+"/internal", http.StatusFound)
			return
		}
		a.srv.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(redirecting.Close)
	addr := strings.TrimPrefix(redirecting.URL, "http://")
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-a")}}
	p := newTestPool(&stubResolver{addrs: []string{addr}}, "node-local", tasks)
	p.Refresh(context.Background())

	p.Samples(context.Background())
	if n := elsewhere.Load(); n != 0 {
		t.Errorf("redirect followed %d times", n)
	}
}

// Agents report only running containers, so a stopped task container is
// placed by the swarm, even when another node reports it as standalone: by
// name, which the manager places. Its container ID is what the task's node
// told the swarm, which the other node's report contradicts, so by ID it is
// not located.
func TestAgentPoolStandaloneReportDoesNotPlaceTaskContainer(t *testing.T) {
	a := newFakeAgent(t, "node-a", ContainerSample{ID: "bbbbbbbbbbbbbbbb", Name: "shop_web.2.t2"})
	b := newFakeAgent(t, "node-b")
	tasks := &stubDockerClient{tasks: []swarm.Task{
		agentTask("node-a"),
		agentTask("node-b"),
		runningTask("t2", "node-b", "bbbbbbbbbbbbbbbb"),
	}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr(), b.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())

	if got, ok := p.LocateContainer(context.Background(), "shop_web.2.t2"); !ok || got != "http://"+b.addr() {
		t.Errorf("LocateContainer by name = %q,%v want node-b", got, ok)
	}
	for _, id := range []string{"bbbbbbbbbbbbbbbb", "bbbbbbbbbbbb"} {
		if got, ok := p.LocateContainer(context.Background(), id); ok {
			t.Errorf("LocateContainer(%q) = %q, want not located", id, got)
		}
	}
}

// Without the task list, a standalone report can't be told from a task
// container's, so it does not place it.
func TestAgentPoolStandaloneNotLocatedWhileTaskListFails(t *testing.T) {
	a := newFakeAgent(t, "node-a", ContainerSample{ID: "aaaaaaaaaaaaaaaa", Name: "redis"})
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-a")}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	p.Samples(context.Background())
	if _, ok := p.LocateContainer(context.Background(), "redis"); !ok {
		t.Fatal("standalone container not located while the task list works")
	}

	tasks.mu.Lock()
	tasks.taskErr = errors.New("manager unavailable")
	tasks.mu.Unlock()
	if got, ok := p.LocateContainer(context.Background(), "redis"); ok {
		t.Errorf("LocateContainer = %q, want not located while the task list fails", got)
	}
}

// An agent is only used for the node it names when the swarm runs a task
// with the agent's address on that node.
func TestAgentPoolIgnoresAgentOnUnconfirmedNode(t *testing.T) {
	a := newFakeAgent(t, "node-a")
	tasks := &stubDockerClient{tasks: []swarm.Task{agentTask("node-b")}}
	p := newTestPool(&stubResolver{addrs: []string{a.addr()}}, "node-local", tasks)
	p.Refresh(context.Background())
	if n := len(p.remoteEndpoints()); n != 0 {
		t.Fatalf("agents = %d, want the unconfirmed one ignored", n)
	}

	// Confirmed later, and kept while the swarm can't be asked.
	tasks.mu.Lock()
	tasks.tasks = []swarm.Task{agentTask("node-a")}
	tasks.mu.Unlock()
	p.Refresh(context.Background())
	if _, ok := p.remoteEndpoints()["node-a"]; !ok {
		t.Fatal("confirmed agent not used")
	}
	tasks.mu.Lock()
	tasks.taskErr = errors.New("manager unavailable")
	tasks.mu.Unlock()
	p.Refresh(context.Background())
	if _, ok := p.remoteEndpoints()["node-a"]; !ok {
		t.Error("a confirmed agent must survive a failed task list")
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
		Addr: "tasks.agent", Port: "7801", Auth: agentauth.Token(testAgentToken), Resolver: res,
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
		Addr: "tasks.agent", Port: "7801", Auth: agentauth.Token(testAgentToken), Resolver: res,
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
		Addr: "tasks.agent", Port: "7801", Auth: agentauth.Token(testAgentToken), Resolver: res,
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

// An agent that registers in DNS between full refreshes (a node joined) is
// picked up on the next change check, not a whole ResolveInterval later.
func TestAgentPoolPicksUpNewAgentBeforeInterval(t *testing.T) {
	a, b := newFakeAgent(t, "node-1"), newFakeAgent(t, "node-2")
	res := &stubResolver{addrs: []string{a.addr()}}
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.agent", Port: "7801", Auth: agentauth.Token(testAgentToken), Resolver: res,
		ResolveInterval: time.Hour,
		ChangeInterval:  5 * time.Millisecond,
	})
	p.Start(context.Background())
	defer p.Stop()

	res.set([]string{a.addr(), b.addr()})
	deadline := time.Now().Add(2 * time.Second)
	for p.agentCount() != 2 {
		if time.Now().After(deadline) {
			t.Fatal("new agent not picked up before the hour-long resolve interval")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Change checks only resolve DNS: unchanged agents are not queried again.
func TestAgentPoolChangeCheckSkipsUnchangedAgents(t *testing.T) {
	a := newFakeAgent(t, "node-1")
	p := NewAgentPool(AgentPoolConfig{
		Addr: "tasks.agent", Port: "7801", Auth: agentauth.Token(testAgentToken),
		Resolver:        &stubResolver{addrs: []string{a.addr()}},
		ResolveInterval: time.Hour,
		ChangeInterval:  time.Millisecond,
	})
	p.Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	p.Stop()
	if n := a.infoCalls.Load(); n != 1 {
		t.Errorf("/info called %d times, want 1", n)
	}
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

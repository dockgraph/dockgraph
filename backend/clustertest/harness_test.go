//go:build clustertest

// Package clustertest drives the multi-node swarm cluster started by
// test/swarm_cluster.sh: it breaks nodes, agents and the swarm itself and
// checks what DockGraph reports through its HTTP API and WebSocket.
//
// The tests mutate a shared cluster and run in source order; each one
// restores the state it changed. They adapt to the cluster's size (the
// MANAGERS and WORKERS it was started with). Run them with:
//
//	test/swarm_cluster.sh up
//	cd backend && go test -tags clustertest -v -timeout 60m ./clustertest/
//
// Set DG_CLUSTER_SOAK (e.g. 15m) to add a churn soak at the end.
package clustertest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dockgraph/dockgraph/collector"
	"github.com/gorilla/websocket"
)

// Cluster layout created by test/swarm_cluster.sh.
const (
	leader        = "dgc-m1"
	clusterLabel  = "dockgraph.cluster=dgc"
	clusterNet    = "dgc-net"
	clusterScript = "../../test/swarm_cluster.sh"
	serverService = "dg_server"
	agentService  = "dg_agent"
	// The agents' public key config, mounted at /<name> (stack.yml).
	agentPubkeyConfig = "dg_agent_pubkey"
	workStack         = "work"
)

// The cluster as found by TestMain: the numbered managers (dgc-mN) and
// workers (dgc-wN) the cluster script started, in order, and the host port
// each manager publishes the UI on. Nodes a scenario adds itself are not
// part of it. The scenarios need dgc-m1 and at least dgc-w1 and dgc-w2;
// the ones that need several managers skip without them.
var (
	managers    []string
	workers     []string
	uiPorts     = map[string]int{}
	firstUIPort int // the leader's UI port
)

// clusterNodeName matches the nodes the cluster script numbers.
var clusterNodeName = regexp.MustCompile(`^dgc-([mw])(\d+)$`)

func allNodes() []string { return append(append([]string{}, managers...), workers...) }

func TestMain(m *testing.M) {
	if err := discoverCluster(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\nstart the cluster with test/swarm_cluster.sh up "+
			"(prefix DOCKER_CONTEXT=default when the active context is rootless)\n", err)
		os.Exit(1)
	}
	fmt.Printf("cluster: %d managers, %d workers\n", len(managers), len(workers))
	os.Exit(m.Run())
}

// discoverCluster fills managers, workers and the UI ports from the running
// cluster, so the scenarios follow whatever MANAGERS and WORKERS it was
// started with.
func discoverCluster() error {
	out, err := exec.Command("docker", "ps", "--filter", "label="+clusterLabel, "--format", "{{.Names}}").Output()
	if err != nil {
		return fmt.Errorf("listing cluster nodes: %w", err)
	}
	number := map[string]int{}
	for _, name := range strings.Fields(string(out)) {
		match := clusterNodeName.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		number[name], _ = strconv.Atoi(match[2])
		if match[1] == "m" {
			managers = append(managers, name)
		} else {
			workers = append(workers, name)
		}
	}
	byNumber := func(a, b string) int { return number[a] - number[b] }
	slices.SortFunc(managers, byNumber)
	slices.SortFunc(workers, byNumber)

	if len(managers) == 0 || managers[0] != leader {
		return fmt.Errorf("no %s node found", leader)
	}
	if len(workers) < 2 || workers[0] != "dgc-w1" || workers[1] != "dgc-w2" {
		return fmt.Errorf("the scenarios need workers dgc-w1 and dgc-w2, found %v", workers)
	}
	for _, m := range managers {
		addr, err := exec.Command("docker", "port", m, "7800").Output()
		if err != nil {
			return fmt.Errorf("UI port of %s: %w", m, err)
		}
		// "127.0.0.1:17901", possibly followed by an IPv6 binding.
		first, _, _ := strings.Cut(strings.TrimSpace(string(addr)), "\n")
		port, err := strconv.Atoi(first[strings.LastIndexByte(first, ':')+1:])
		if err != nil {
			return fmt.Errorf("UI port of %s: %q", m, first)
		}
		uiPorts[m] = port
	}
	firstUIPort = uiPorts[leader]

	// The swarm must answer, not only the node containers exist.
	if out, err := exec.Command("docker", "exec", leader, "docker", "node", "ls").CombinedOutput(); err != nil {
		return fmt.Errorf("swarm not reachable on %s: %v: %s", leader, err, out)
	}
	return nil
}

// requireManagers skips a scenario that needs more managers than the
// cluster has.
func requireManagers(t *testing.T, n int) {
	t.Helper()
	if len(managers) < n {
		t.Skipf("needs %d managers, the cluster has %d", n, len(managers))
	}
}

// ── Docker ────────────────────────────────────────────────────

// host runs a docker command against the daemon hosting the cluster nodes.
func host(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// on runs a docker command on a cluster node.
func on(t *testing.T, node string, args ...string) string {
	t.Helper()
	return host(t, append([]string{"exec", node, "docker"}, args...)...)
}

// tryOn is on without failing the test, for commands expected to fail.
func tryOn(node string, args ...string) (string, error) {
	out, err := exec.Command("docker", append([]string{"exec", node, "docker"}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// script runs a test/swarm_cluster.sh command.
func script(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command(clusterScript, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("swarm_cluster.sh %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// containerOn returns the ID of a running task container of service on node.
func containerOn(t *testing.T, node, service string) string {
	t.Helper()
	var id string
	eventually(t, 60*time.Second, fmt.Sprintf("%s container on %s", service, node), func() error {
		id = firstLine(on(t, node, "ps", "-q", "--no-trunc", "--filter", "label=com.docker.swarm.service.name="+service))
		if id == "" {
			return fmt.Errorf("none running")
		}
		return nil
	})
	return id
}

// serverNode returns the hostname of the node running the DockGraph server.
func serverNode(t *testing.T) string {
	t.Helper()
	var node string
	eventually(t, 90*time.Second, "running server task", func() error {
		out, err := tryOn(leader, "service", "ps", serverService, "--filter", "desired-state=running",
			"--format", "{{.Node}} {{.CurrentState}}")
		if err != nil {
			return fmt.Errorf("%v: %s", err, out)
		}
		for _, line := range strings.Split(out, "\n") {
			n, state, _ := strings.Cut(line, " ")
			if strings.HasPrefix(state, "Running") {
				node = n
				return nil
			}
		}
		return fmt.Errorf("no running task: %q", out)
	})
	return node
}

// serverTaskID returns the ID of the running server task, to detect restarts.
func serverTaskID(t *testing.T) string {
	t.Helper()
	return firstLine(on(t, leader, "service", "ps", "-q", serverService, "--filter", "desired-state=running"))
}

// serverLogs returns the server's log output since the given time.
func serverLogs(t *testing.T, since time.Time) string {
	t.Helper()
	return on(t, leader, "service", "logs", "--raw", "--since", since.UTC().Format(time.RFC3339), serverService)
}

// waitConverged waits until every service of stack runs all its replicas.
func waitConverged(t *testing.T, stack string) {
	t.Helper()
	eventually(t, 3*time.Minute, "stack "+stack+" converged", func() error {
		out, err := tryOn(leader, "service", "ls", "--filter", "label=com.docker.stack.namespace="+stack,
			"--format", "{{.Name}} {{.Replicas}}")
		if err != nil {
			return fmt.Errorf("%v: %s", err, out)
		}
		for _, line := range strings.Split(out, "\n") {
			name, replicas, _ := strings.Cut(line, " ")
			running, desired, _ := strings.Cut(strings.Fields(replicas)[0], "/")
			if running != desired {
				return fmt.Errorf("%s at %s", name, replicas)
			}
		}
		return nil
	})
}

// setAvailability sets a node's availability (active, pause, drain).
func setAvailability(t *testing.T, node, availability string) {
	t.Helper()
	on(t, leader, "node", "update", "--availability", availability, node)
}

// nodeID returns the swarm node ID of a cluster node container.
func nodeID(t *testing.T, name string) string {
	t.Helper()
	return on(t, name, "info", "--format", "{{.Swarm.NodeID}}")
}

// removeNode deletes a node container and then its swarm membership. The
// node is looked up by ID: several nodes may share a hostname.
func removeNode(t *testing.T, name, id string) {
	t.Helper()
	host(t, "rm", "-fv", name)
	eventually(t, 2*time.Minute, name+" removed from the swarm", func() error {
		out, err := tryOn(leader, "node", "rm", "--force", id)
		if err != nil && !strings.Contains(out, "not found") {
			return fmt.Errorf("%v: %s", err, out)
		}
		return nil
	})
}

// ── HTTP ──────────────────────────────────────────────────────

var client = &http.Client{Timeout: 45 * time.Second}

func baseURL(port int) string { return fmt.Sprintf("http://127.0.0.1:%d", port) }

// uiPort returns the UI port published by a manager node. The routing mesh
// serves the UI on every node, so any live manager works.
func uiPort(manager string) int {
	if port, ok := uiPorts[manager]; ok {
		return port
	}
	return firstUIPort
}

type response struct {
	code    int
	body    string
	elapsed time.Duration
}

// getVia issues a GET through the given UI port.
func getVia(t *testing.T, port int, path string) response {
	t.Helper()
	start := time.Now()
	resp, err := client.Get(baseURL(port) + path)
	if err != nil {
		return response{code: 0, body: err.Error(), elapsed: time.Since(start)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return response{code: resp.StatusCode, body: string(body), elapsed: time.Since(start)}
}

func get(t *testing.T, path string) response {
	t.Helper()
	return getVia(t, firstUIPort, path)
}

// readSSE reads a server-sent event stream for d and returns the data lines.
func readSSE(t *testing.T, path string, d time.Duration) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL(firstUIPort)+path, nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: %d %s", path, resp.StatusCode, body)
	}
	var lines []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if data, ok := strings.CutPrefix(sc.Text(), "data:"); ok {
			lines = append(lines, strings.TrimSpace(data))
		}
	}
	return lines
}

// ── WebSocket ─────────────────────────────────────────────────

// snapshotVia returns the initial graph snapshot the server sends a new
// WebSocket client.
func snapshotVia(port int) (collector.GraphSnapshot, error) {
	var snap collector.GraphSnapshot
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d/ws", port), nil)
	if err != nil {
		return snap, err
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	for {
		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return snap, err
		}
		if msg.Type == collector.MsgTypeSnapshot {
			return snap, json.Unmarshal(msg.Data, &snap)
		}
	}
}

func snapshot(t *testing.T) collector.GraphSnapshot {
	t.Helper()
	snap, err := snapshotVia(firstUIPort)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return snap
}

func findNode(snap collector.GraphSnapshot, id string) *collector.Node {
	for i := range snap.Nodes {
		if snap.Nodes[i].ID == id {
			return &snap.Nodes[i]
		}
	}
	return nil
}

// serviceNode returns the graph node of a swarm service or an error naming
// what is missing, for use inside eventually.
func serviceNode(snap collector.GraphSnapshot, name string) (*collector.Node, error) {
	n := findNode(snap, "service:"+name)
	if n == nil {
		return nil, fmt.Errorf("service:%s not in snapshot", name)
	}
	if n.Service == nil {
		return nil, fmt.Errorf("service:%s has no service info", name)
	}
	return n, nil
}

// expectService waits until the snapshot shows service with the given
// status and replica counts.
func expectService(t *testing.T, name, status string, running, desired int, timeout time.Duration) {
	t.Helper()
	eventually(t, timeout, fmt.Sprintf("%s %s %d/%d", name, status, running, desired), func() error {
		n, err := serviceNode(snapshot(t), name)
		if err != nil {
			return err
		}
		r := n.Service.Replicas
		if n.Status != status || r.Running != running || r.Desired != desired {
			return fmt.Errorf("got %s %d/%d", n.Status, r.Running, r.Desired)
		}
		return nil
	})
}

// ── Stats ─────────────────────────────────────────────────────

type statsHistory struct {
	Timestamps []int64                         `json:"timestamps"`
	Containers map[string]map[string][]float64 `json:"containers"`
}

func nodeStats(t *testing.T) statsHistory {
	t.Helper()
	r := get(t, "/api/stats/history?range=5m&scope=nodes")
	if r.code != http.StatusOK {
		t.Fatalf("node stats: %d %s", r.code, r.body)
	}
	var h statsHistory
	if err := json.Unmarshal([]byte(r.body), &h); err != nil {
		t.Fatalf("node stats: %v", err)
	}
	return h
}

// latestMem returns the most recent memory sample of a node series, or -1
// when the series is missing.
func latestMem(h statsHistory, hostname string) float64 {
	s, ok := h.Containers["node:"+hostname]
	if !ok || len(s["mem"]) == 0 {
		return -1
	}
	return s["mem"][len(s["mem"])-1]
}

// ── Polling ───────────────────────────────────────────────────

// eventually polls check every 2s until it returns nil or timeout passes.
func eventually(t *testing.T, timeout time.Duration, what string, check func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var err error
	for {
		if err = check(); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s: %v", timeout, what, err)
		}
		time.Sleep(2 * time.Second)
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

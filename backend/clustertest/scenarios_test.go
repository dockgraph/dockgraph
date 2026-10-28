//go:build clustertest

package clustertest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

// ── Steady state ──────────────────────────────────────────────

func TestBaseline(t *testing.T) {
	snap := snapshot(t)

	leaders := 0
	for _, name := range allNodes() {
		n := findNode(snap, "swarmnode:"+name)
		if n == nil {
			t.Fatalf("swarmnode:%s missing", name)
		}
		if n.Status != "ready" || n.SwarmNode.Availability != "active" {
			t.Errorf("%s: status %s availability %s", name, n.Status, n.SwarmNode.Availability)
		}
		wantRole := "worker"
		if slices.Contains(managers, name) {
			wantRole = "manager"
		}
		if n.SwarmNode.Role != wantRole {
			t.Errorf("%s: role %s, want %s", name, n.SwarmNode.Role, wantRole)
		}
		if n.SwarmNode.Leader {
			leaders++
		}
	}
	if leaders != 1 {
		t.Errorf("%d leaders flagged, want 1", leaders)
	}

	nodes := len(allNodes()) // shipper is global: one task per node
	for name, want := range map[string][2]int{
		"work_web": {4, 4}, "work_api": {2, 2}, "work_db": {1, 1}, "work_chatty": {1, 1}, "work_shipper": {nodes, nodes},
	} {
		n, err := serviceNode(snap, name)
		if err != nil {
			t.Error(err)
			continue
		}
		r := n.Service.Replicas
		if n.Status != "running" || r.Running != want[0] || r.Desired != want[1] {
			t.Errorf("%s: %s %d/%d, want running %d/%d", name, n.Status, r.Running, r.Desired, want[0], want[1])
		}
	}

	chatty, _ := serviceNode(snap, "work_chatty")
	if chatty != nil && (len(chatty.Service.Tasks) == 0 || chatty.Service.Tasks[0].NodeHostname != "dgc-w1") {
		t.Errorf("chatty tasks not placed on dgc-w1: %+v", chatty.Service.Tasks)
	}

	for _, n := range snap.Nodes {
		if n.Stack == "dg" || strings.HasPrefix(n.ID, "service:dg_") {
			t.Errorf("DockGraph's own resource %s is shown", n.ID)
		}
	}

	eventually(t, 60*time.Second, "node stats from every node", func() error {
		h := nodeStats(t)
		for _, name := range allNodes() {
			if latestMem(h, name) <= 0 {
				return fmt.Errorf("node:%s has no recent sample", name)
			}
		}
		return nil
	})
}

// ── Server <-> agent proxy ────────────────────────────────────

func TestRemoteContainerProxy(t *testing.T) {
	id := containerOn(t, "dgc-w1", "work_chatty")
	name := containerName(t, "dgc-w1", id)

	for _, key := range []string{id, id[:12], name} {
		r := get(t, "/api/containers/"+key)
		if r.code != http.StatusOK || !inspected(r, name) {
			t.Errorf("inspect by %q: %d (%s) %.200s", key, r.code, r.elapsed, r.body)
		}
	}

	// Through another manager: the routing mesh must reach the same server.
	if other := managers[len(managers)-1]; other != leader {
		if r := getVia(t, uiPort(other), "/api/containers/"+id); !inspected(r, name) {
			t.Errorf("inspect via %s: %d %.200s", other, r.code, r.body)
		}
	}

	r := get(t, "/api/containers/"+id+"/logs/history?limit=5")
	if r.code != http.StatusOK || !strings.Contains(r.body, "chatty line") {
		t.Errorf("remote log history: %d %.300s", r.code, r.body)
	}

	lines := readSSE(t, "/api/containers/"+id+"/logs?tail=1", 6*time.Second)
	if len(lines) < 4 {
		t.Errorf("remote log stream: %d lines in 6s, want >= 4: %v", len(lines), lines)
	}

	// A container on a manager that does not run the server.
	srv := serverNode(t)
	for _, m := range managers {
		if m == srv {
			continue
		}
		sid := containerOn(t, m, "work_shipper")
		if r := get(t, "/api/containers/"+sid); r.code != http.StatusOK {
			t.Errorf("inspect shipper on %s: %d %.200s", m, r.code, r.body)
		}
		break
	}

	// Unknown containers must fail fast, not wait on every agent.
	r = get(t, "/api/containers/0123456789ab")
	if r.code != http.StatusNotFound || r.elapsed > 10*time.Second {
		t.Errorf("unknown container: %d after %s, want 404 quickly: %.200s", r.code, r.elapsed, r.body)
	}
}

// Swarm volumes are node-local: the graph shows a service's named volume
// even when it only exists on another node, so its inspect must reach it.
func TestRemoteVolumeInspect(t *testing.T) {
	dbNode := firstLine(on(t, leader, "service", "ps", "work_db", "--filter", "desired-state=running", "--format", "{{.Node}}"))
	if dbNode == serverNode(t) {
		t.Skip("work_db runs next to the server; the volume is local")
	}
	if n := findNode(snapshot(t), "volume:work_data"); n == nil {
		t.Fatalf("volume:work_data not in the graph")
	}
	r := get(t, "/api/volumes/work_data")
	if r.code != http.StatusOK || !strings.Contains(r.body, `"work_data"`) {
		t.Errorf("inspect volume on %s: %d %.200s", dbNode, r.code, r.body)
	}
}

func TestRemoteLogStreamOutlivesWriteTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("streams for 75s")
	}
	id := containerOn(t, "dgc-w1", "work_chatty")
	// The server's WriteTimeout is 60s; the stream must keep flowing past it.
	lines := readSSE(t, "/api/containers/"+id+"/logs?tail=1", 75*time.Second)
	if len(lines) < 65 {
		t.Errorf("got %d lines in 75s, want >= 65 (stream cut early?)", len(lines))
	}
}

// ── Service states ────────────────────────────────────────────

func TestServiceStates(t *testing.T) {
	names := []string{"t_zero", "t_pending", "t_job", "t_roll"}
	t.Cleanup(func() {
		for _, n := range names {
			_, _ = tryOn(leader, "service", "rm", n)
		}
	})

	on(t, leader, "service", "create", "-d", "--name", "t_zero", "--replicas", "0", "busybox:1.37", "sleep", "infinity")
	on(t, leader, "service", "create", "-d", "--name", "t_pending", "--replicas", "2",
		"--constraint", "node.labels.never==true", "busybox:1.37", "sleep", "infinity")
	on(t, leader, "service", "create", "-d", "--name", "t_job", "--mode", "replicated-job", "--replicas", "3",
		"busybox:1.37", "true")
	on(t, leader, "service", "create", "-d", "--name", "t_roll", "--replicas", "2", "busybox:1.37", "sleep", "infinity")

	expectService(t, "t_zero", "stopped", 0, 0, 60*time.Second)
	expectService(t, "t_pending", "degraded", 0, 2, 60*time.Second)
	expectService(t, "t_roll", "running", 2, 2, 60*time.Second)

	eventually(t, 90*time.Second, "t_job completed", func() error {
		n, err := serviceNode(snapshot(t), "t_job")
		if err != nil {
			return err
		}
		complete := 0
		for _, task := range n.Service.Tasks {
			if task.State == "complete" {
				complete++
			}
		}
		if n.Service.Mode != "replicated-job" || complete != 3 {
			return fmt.Errorf("mode %s, %d complete tasks", n.Service.Mode, complete)
		}
		// A job that finished successfully is not a failing service.
		if r := n.Service.Replicas; n.Status != "completed" || r.Completed != 3 {
			return fmt.Errorf("status %s, replicas %d/%d, %d completed", n.Status, r.Running, r.Desired, r.Completed)
		}
		return nil
	})

	// A failing update rolls back; the failed tasks stay visible with errors.
	on(t, leader, "service", "update", "-d", "--image", "busybox:0.0-does-not-exist",
		"--update-failure-action", "rollback", "--update-monitor", "5s", "--update-max-failure-ratio", "0", "t_roll")
	eventually(t, 3*time.Minute, "t_roll rolled back", func() error {
		n, err := serviceNode(snapshot(t), "t_roll")
		if err != nil {
			return err
		}
		if n.Service.UpdateStatus != "rollback_completed" || n.Status != "running" {
			return fmt.Errorf("update %q status %s", n.Service.UpdateStatus, n.Status)
		}
		for _, task := range n.Service.Tasks {
			if task.Error != "" {
				return nil
			}
		}
		return fmt.Errorf("no failed task with an error kept")
	})
}

// ── Stacks ────────────────────────────────────────────────────

func TestStackFileGhosts(t *testing.T) {
	for _, m := range managers {
		on(t, m, "run", "--rm", "-v", "/opt:/opt", "busybox:1.37", "mkdir", "-p", "/opt/stacks")
		host(t, "cp", "../../test/cluster/workload.yml", m+":/opt/stacks/work.yml")
	}
	t.Cleanup(func() {
		on(t, leader, "stack", "deploy", "--detach=true", "--resolve-image", "never", "-c", "/workload.yml", workStack)
		on(t, leader, "service", "update", "--detach=false", "--quiet",
			"--mount-rm", "/stacks/work.yml", "--env-rm", "DG_COMPOSE_PATH", serverService)
		waitConverged(t, workStack)
		waitConverged(t, "dg")
	})

	on(t, leader, "service", "update", "--detach=false", "--quiet",
		"--mount-add", "type=bind,src=/opt/stacks/work.yml,dst=/stacks/work.yml,readonly",
		"--env-add", "DG_COMPOSE_PATH=work=/stacks/work.yml", serverService)
	waitConverged(t, "dg")

	on(t, leader, "service", "rm", "work_api")
	eventually(t, 60*time.Second, "work_api shown as a ghost", func() error {
		snap := snapshot(t)
		n := findNode(snap, "service:work_api")
		if n == nil || n.Status != "not_running" {
			return fmt.Errorf("work_api: %+v", n)
		}
		count := 0
		for _, node := range snap.Nodes {
			if node.ID == "service:work_web" {
				count++
				if node.Status != "running" {
					return fmt.Errorf("work_web deployed but %s", node.Status)
				}
			}
		}
		if count != 1 {
			return fmt.Errorf("work_web appears %d times", count)
		}
		return nil
	})
}

func TestStackRemoveAndRedeploy(t *testing.T) {
	on(t, leader, "stack", "rm", workStack)
	t.Cleanup(func() {
		on(t, leader, "stack", "deploy", "--detach=true", "--resolve-image", "never", "-c", "/workload.yml", workStack)
		waitConverged(t, workStack)
	})

	eventually(t, 90*time.Second, "work stack gone from the graph", func() error {
		for _, n := range snapshot(t).Nodes {
			if n.Stack == workStack && n.Type != "volume" {
				return fmt.Errorf("%s still shown", n.ID)
			}
		}
		return nil
	})

	on(t, leader, "stack", "deploy", "--detach=true", "--resolve-image", "never", "-c", "/workload.yml", workStack)
	waitConverged(t, workStack)
	expectService(t, "work_web", "running", 4, 4, 60*time.Second)
}

// ── Node changes ──────────────────────────────────────────────

func TestNodeDrain(t *testing.T) {
	oldChatty := containerOn(t, "dgc-w1", "work_chatty")
	setAvailability(t, "dgc-w1", "drain")
	t.Cleanup(func() {
		setAvailability(t, "dgc-w1", "active")
		waitConverged(t, workStack)
		waitConverged(t, "dg")
	})

	// chatty can only run on dgc-w1, so it waits pending.
	expectService(t, "work_chatty", "degraded", 0, 1, 90*time.Second)
	nodes := len(allNodes())
	expectService(t, "work_shipper", "running", nodes-1, nodes-1, 90*time.Second)

	n := findNode(snapshot(t), "swarmnode:dgc-w1")
	if n == nil || n.SwarmNode.Availability != "drain" {
		t.Errorf("swarmnode:dgc-w1 availability: %+v", n)
	}

	// The old task container still exists (stopped) on dgc-w1, but its agent
	// is drained too. Before and after the next agent refresh the request
	// must answer quickly, whatever it can still find.
	for _, wait := range []time.Duration{0, 35 * time.Second} {
		time.Sleep(wait)
		r := get(t, "/api/containers/"+oldChatty)
		t.Logf("drained container after %s: %d in %s", wait, r.code, r.elapsed)
		if r.elapsed > 10*time.Second {
			t.Errorf("drained container took %s", r.elapsed)
		}
	}
}

func TestServerMoves(t *testing.T) {
	requireManagers(t, 2) // the server only runs on managers
	from := serverNode(t)
	before := serverTaskID(t)
	on(t, leader, "service", "update", "--detach=false", "--quiet",
		"--constraint-add", "node.hostname!="+from, serverService)
	t.Cleanup(func() {
		on(t, leader, "service", "update", "--detach=false", "--quiet",
			"--constraint-rm", "node.hostname!="+from, serverService)
	})

	to := serverNode(t)
	if to == from || serverTaskID(t) == before {
		t.Fatalf("server did not move from %s", from)
	}
	t.Logf("server moved %s -> %s", from, to)
	eventually(t, 60*time.Second, "server healthy after the move", func() error {
		if r := get(t, "/healthz"); r.code != http.StatusOK {
			return fmt.Errorf("healthz %d", r.code)
		}
		return nil
	})

	// The old server node is now remote: its containers go through its agent.
	sid := containerOn(t, from, "work_shipper")
	eventually(t, 60*time.Second, "shipper on the old server node via its agent", func() error {
		if r := get(t, "/api/containers/"+sid); r.code != http.StatusOK {
			return fmt.Errorf("%d %.200s", r.code, r.body)
		}
		return nil
	})
	// And the new one is local.
	lid := containerOn(t, to, "work_shipper")
	if r := get(t, "/api/containers/"+lid); r.code != http.StatusOK {
		t.Errorf("shipper on the new server node: %d %.200s", r.code, r.body)
	}

	eventually(t, 90*time.Second, "node stats from every node after the move", func() error {
		h := nodeStats(t)
		for _, name := range allNodes() {
			if latestMem(h, name) <= 0 {
				return fmt.Errorf("node:%s has no recent sample", name)
			}
		}
		return nil
	})
}

func TestAgentTokenMismatch(t *testing.T) {
	before := serverTaskID(t)
	createSecret(t, "dg_agent_token_bad", randomHex(t))
	// The rejection is logged once, as the agents roll over: watch from here.
	since := time.Now()
	on(t, leader, "service", "update", "--detach=false", "--quiet",
		"--secret-rm", agentSecret, "--secret-add", "source=dg_agent_token_bad,target="+agentSecret, agentService)
	t.Cleanup(func() {
		on(t, leader, "service", "update", "--detach=false", "--quiet",
			"--secret-rm", "dg_agent_token_bad", "--secret-add", "source="+agentSecret+",target="+agentSecret, agentService)
		_, _ = tryOn(leader, "secret", "rm", "dg_agent_token_bad")
		waitConverged(t, "dg")
		id := containerOn(t, "dgc-w1", "work_chatty")
		eventually(t, 60*time.Second, "proxy recovers with the right token", func() error {
			if r := get(t, "/api/containers/"+id); r.code != http.StatusOK {
				return fmt.Errorf("%d", r.code)
			}
			return nil
		})
	})
	waitConverged(t, "dg")

	// Let at least one full agent refresh (30s) run with the bad token.
	time.Sleep(35 * time.Second)
	logs := serverLogs(t, since)
	t.Logf("server log during the mismatch:\n%s", tail(logs, 10))
	if !strings.Contains(strings.ToLower(logs), "token") {
		t.Errorf("server log never names the token as the problem")
	}
	if total := strings.Count(logs, "\n"); total > 200 {
		t.Errorf("server logged %d lines in ~65s (log flood)", total)
	}

	// The UI must not see a 401 (it would log the user out) nor a plain
	// "not found" that hides the misconfiguration.
	id := containerOn(t, "dgc-w1", "work_chatty")
	r := get(t, "/api/containers/"+id)
	t.Logf("remote inspect with a bad token: %d %.200s", r.code, r.body)
	if r.code != http.StatusBadGateway {
		t.Errorf("remote inspect with a bad token: %d, want 502", r.code)
	}
	if r := get(t, "/api/containers/"+id+"/logs/history?limit=5"); r.code == http.StatusUnauthorized {
		t.Errorf("agent auth failure surfaced as 401")
	}
	if serverTaskID(t) != before {
		t.Errorf("server restarted")
	}
}

// Nodes the scenarios add and remove themselves. Unnumbered, so they never
// clash with the dgc-wN workers of a larger cluster.
const (
	extraNode = "dgc-extra"
	twinNode  = "dgc-twin"
)

func TestNodeJoinAndLeave(t *testing.T) {
	nodes := len(allNodes())
	script(t, "add-node", extraNode, "worker")
	id := nodeID(t, extraNode)
	removed := false
	t.Cleanup(func() {
		if !removed {
			removeNode(t, extraNode, id)
		}
	})

	eventually(t, 90*time.Second, extraNode+" in the graph", func() error {
		n := findNode(snapshot(t), "swarmnode:"+extraNode)
		if n == nil || n.Status != "ready" {
			return fmt.Errorf("swarmnode:%s: %+v", extraNode, n)
		}
		return nil
	})
	expectService(t, "work_shipper", "running", nodes+1, nodes+1, 2*time.Minute)

	sid := containerOn(t, extraNode, "work_shipper")
	eventually(t, 90*time.Second, "new node's agent discovered", func() error {
		if latestMem(nodeStats(t), extraNode) <= 0 {
			return fmt.Errorf("no node:%s stats", extraNode)
		}
		if r := get(t, "/api/containers/"+sid); r.code != http.StatusOK {
			return fmt.Errorf("inspect %d", r.code)
		}
		return nil
	})

	// Kill the node without leaving the swarm.
	host(t, "rm", "-fv", extraNode)
	eventually(t, 2*time.Minute, extraNode+" down", func() error {
		n := findNode(snapshot(t), "swarmnode:"+extraNode)
		if n == nil || n.Status != "down" {
			return fmt.Errorf("swarmnode:%s: %+v", extraNode, n)
		}
		return nil
	})
	r := get(t, "/api/containers/"+sid)
	t.Logf("container on the dead node: %d after %s", r.code, r.elapsed)
	if r.elapsed > 30*time.Second {
		t.Errorf("request for a container on a dead node took %s", r.elapsed)
	}
	expectService(t, "work_shipper", "running", nodes, nodes, 2*time.Minute)

	on(t, leader, "node", "rm", "--force", id)
	removed = true
	eventually(t, 60*time.Second, extraNode+" gone from the graph", func() error {
		if n := findNode(snapshot(t), "swarmnode:"+extraNode); n != nil {
			return fmt.Errorf("still shown: %s", n.Status)
		}
		return nil
	})
}

func TestDuplicateHostname(t *testing.T) {
	since := time.Now()
	nodes := len(allNodes())
	script(t, "add-node", twinNode, "worker", "dgc-w1")
	id := nodeID(t, twinNode)
	t.Cleanup(func() { removeNode(t, twinNode, id) })

	expectService(t, "work_shipper", "running", nodes+1, nodes+1, 2*time.Minute)
	snap := snapshot(t)
	count := 0
	for _, n := range snap.Nodes {
		if n.Type == "swarmnode" && n.Name == "dgc-w1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("%d swarmnode entries for dgc-w1, want 1", count)
	}

	eventually(t, 60*time.Second, "duplicate hostname warning", func() error {
		if !strings.Contains(serverLogs(t, since), "both ready with hostname") {
			return fmt.Errorf("not logged yet")
		}
		return nil
	})

	// Each twin's containers must still resolve to the right agent, once
	// the new agent has been discovered (agent refresh runs every 30s).
	chatty := containerOn(t, "dgc-w1", "work_chatty")
	chattyName := containerName(t, "dgc-w1", chatty)
	twin := containerOn(t, twinNode, "work_shipper")
	twinName := containerName(t, twinNode, twin)
	eventually(t, 90*time.Second, "twin's container reachable", func() error {
		if r := get(t, "/api/containers/"+twin); !inspected(r, twinName) {
			return fmt.Errorf("%d %.200s", r.code, r.body)
		}
		return nil
	})
	for range 5 {
		for cid, name := range map[string]string{chatty: chattyName, twin: twinName} {
			if r := get(t, "/api/containers/"+cid); !inspected(r, name) {
				t.Errorf("inspect %s: %d %.200s", cid[:12], r.code, r.body)
			}
		}
	}
}

// ── Failures ──────────────────────────────────────────────────

// checkUnreachableNode breaks dgc-w2 with breakNode and checks that the rest
// of DockGraph keeps working, then heals it and checks recovery.
func checkUnreachableNode(t *testing.T, breakNode, heal func()) {
	const node = "dgc-w2"
	before := serverTaskID(t)
	sid := containerOn(t, node, "work_shipper")
	if r := get(t, "/api/containers/"+sid); r.code != http.StatusOK {
		t.Fatalf("precondition: inspect %d", r.code)
	}

	breakNode()
	healed := false
	t.Cleanup(func() {
		if !healed {
			heal()
		}
	})
	start := time.Now()

	r := get(t, "/api/containers/"+sid)
	t.Logf("inspect via the unreachable agent: %d after %s", r.code, r.elapsed)
	if r.code != http.StatusBadGateway && r.code != http.StatusNotFound {
		t.Errorf("inspect: %d %.200s", r.code, r.body)
	}
	if r.elapsed > 25*time.Second {
		t.Errorf("inspect took %s", r.elapsed)
	}

	r = get(t, "/api/logs/history?stack=work&limit=20")
	t.Logf("stack log history with a node unreachable: %d after %s", r.code, r.elapsed)
	if r.code != http.StatusOK || r.elapsed > 30*time.Second {
		t.Errorf("stack log history: %d after %s %.200s", r.code, r.elapsed, r.body)
	}
	// chatty runs on a reachable node and logs every second: one unreachable
	// node must not blank the other services' logs.
	if !strings.Contains(r.body, "chatty line") {
		t.Errorf("stack log history lost reachable services' lines: %.300s", r.body)
	}

	for range 5 {
		if r := get(t, "/healthz"); r.code != http.StatusOK || r.elapsed > 2*time.Second {
			t.Errorf("healthz: %d after %s", r.code, r.elapsed)
		}
		snapStart := time.Now()
		if _, err := snapshotVia(firstUIPort); err != nil || time.Since(snapStart) > 5*time.Second {
			t.Errorf("snapshot: %v after %s", err, time.Since(snapStart))
		}
		time.Sleep(time.Second)
	}

	// Other nodes' stats keep flowing.
	h1 := nodeStats(t)
	time.Sleep(12 * time.Second)
	h2 := nodeStats(t)
	if len(h2.Timestamps) == 0 || len(h1.Timestamps) == 0 || h2.Timestamps[len(h2.Timestamps)-1] <= h1.Timestamps[len(h1.Timestamps)-1] {
		t.Errorf("stats history stopped advancing")
	}
	if latestMem(h2, "dgc-w1") <= 0 {
		t.Errorf("node:dgc-w1 stats stalled while %s is unreachable", node)
	}

	eventually(t, 2*time.Minute, node+" down", func() error {
		n := findNode(snapshot(t), "swarmnode:"+node)
		if n == nil || n.Status != "down" {
			return fmt.Errorf("swarmnode:%s: %+v", node, n)
		}
		return nil
	})
	t.Logf("%s reported down after %s", node, time.Since(start).Round(time.Second))

	heal()
	healed = true
	eventually(t, 2*time.Minute, node+" ready again", func() error {
		n := findNode(snapshot(t), "swarmnode:"+node)
		if n == nil || n.Status != "ready" {
			return fmt.Errorf("swarmnode:%s: %+v", node, n)
		}
		return nil
	})
	waitConverged(t, "dg")
	waitConverged(t, workStack)
	sid = containerOn(t, node, "work_shipper")
	eventually(t, 90*time.Second, "proxy to "+node+" recovered", func() error {
		if r := get(t, "/api/containers/"+sid); r.code != http.StatusOK {
			return fmt.Errorf("%d %.200s", r.code, r.body)
		}
		return nil
	})
	if serverTaskID(t) != before {
		t.Errorf("server restarted")
	}
}

func TestAgentNodePaused(t *testing.T) {
	checkUnreachableNode(t,
		func() { host(t, "pause", "dgc-w2") },
		func() { host(t, "unpause", "dgc-w2") })
}

func TestAgentNodePartitioned(t *testing.T) {
	ip := host(t, "inspect", "-f", "{{(index .NetworkSettings.Networks \""+clusterNet+"\").IPAddress}}", "dgc-w2")
	checkUnreachableNode(t,
		func() { host(t, "network", "disconnect", clusterNet, "dgc-w2") },
		func() { host(t, "network", "connect", "--ip", ip, clusterNet, "dgc-w2") })
}

func TestManagerQuorumLoss(t *testing.T) {
	requireManagers(t, 3) // an HA control plane that loses its majority
	srv := serverNode(t)
	port := uiPort(srv)
	var paused []string
	for _, m := range managers {
		if m != srv {
			paused = append(paused, m)
		}
	}
	srvContainer := firstLine(on(t, srv, "ps", "-q", "--filter", "label=com.docker.swarm.service.name="+serverService))

	for _, m := range paused {
		host(t, "pause", m)
	}
	healed := false
	heal := func() {
		for _, m := range paused {
			_, _ = exec.Command("docker", "unpause", m).CombinedOutput()
		}
	}
	t.Cleanup(func() {
		if !healed {
			heal()
		}
	})

	eventually(t, 2*time.Minute, "quorum lost", func() error {
		out, err := tryOn(srv, "node", "ls")
		if err == nil {
			return fmt.Errorf("swarm API still answers")
		}
		t.Logf("swarm API: %s", firstLine(out))
		return nil
	})

	for range 6 {
		if r := getVia(t, port, "/healthz"); r.code != http.StatusOK {
			t.Errorf("healthz without quorum: %d %s", r.code, r.body)
		}
		start := time.Now()
		snap, err := snapshotVia(port)
		if err != nil || time.Since(start) > 5*time.Second {
			t.Errorf("snapshot without quorum: %v after %s", err, time.Since(start))
		} else if findNode(snap, "service:work_web") == nil {
			t.Errorf("snapshot without quorum lost the services")
		}
		time.Sleep(5 * time.Second)
	}
	if r := getVia(t, port, "/api/system/info"); r.code != http.StatusOK {
		t.Logf("system info without quorum: %d %.200s", r.code, r.body)
	}

	now := firstLine(on(t, srv, "ps", "-q", "--filter", "label=com.docker.swarm.service.name="+serverService))
	if now != srvContainer {
		t.Errorf("server container restarted during quorum loss")
	}

	heal()
	healed = true
	// Leadership can still move right after quorum returns ("node lost
	// leader status"), so the first write is retried until it sticks.
	t.Cleanup(func() { _, _ = tryOn(leader, "service", "rm", "t_after_quorum") })
	eventually(t, 2*time.Minute, "swarm accepts writes again", func() error {
		out, err := tryOn(leader, "service", "create", "-d", "--name", "t_after_quorum", "busybox:1.37", "sleep", "infinity")
		if err != nil && !strings.Contains(out, "already exists") {
			return fmt.Errorf("%v: %s", err, out)
		}
		return nil
	})
	expectService(t, "t_after_quorum", "running", 1, 1, 90*time.Second)
}

// ── Soak ──────────────────────────────────────────────────────

// TestChurnSoak scales and updates services at random for DG_CLUSTER_SOAK
// (e.g. 15m), then checks the graph matches the swarm and the server stayed
// up within its memory limit.
func TestChurnSoak(t *testing.T) {
	d, err := time.ParseDuration(os.Getenv("DG_CLUSTER_SOAK"))
	if err != nil || d <= 0 {
		t.Skip("set DG_CLUSTER_SOAK (e.g. 15m) to run")
	}
	before := serverTaskID(t)
	srv := serverNode(t)
	t.Logf("server memory at start: %s", serverMemory(t, srv))
	t.Cleanup(func() {
		for i := range 5 {
			_, _ = tryOn(leader, "service", "rm", fmt.Sprintf("t_churn_%d", i))
		}
		on(t, leader, "service", "scale", "-d", "work_web=4", "work_api=2")
		waitConverged(t, workStack)
	})

	deadline := time.Now().Add(d)
	for i := 0; time.Now().Before(deadline); i++ {
		switch mathrand.IntN(4) {
		case 0:
			on(t, leader, "service", "scale", "-d", fmt.Sprintf("work_web=%d", mathrand.IntN(9)))
		case 1:
			on(t, leader, "service", "scale", "-d", fmt.Sprintf("work_api=%d", 1+mathrand.IntN(4)))
		case 2:
			on(t, leader, "service", "update", "-d", "--force", "work_shipper")
		case 3:
			name := fmt.Sprintf("t_churn_%d", mathrand.IntN(5))
			if _, err := tryOn(leader, "service", "inspect", name); err == nil {
				on(t, leader, "service", "rm", name)
			} else {
				on(t, leader, "service", "create", "-d", "--name", name, "--replicas", fmt.Sprint(1+mathrand.IntN(3)),
					"busybox:1.37", "sleep", "infinity")
			}
		}
		time.Sleep(time.Duration(2+mathrand.IntN(6)) * time.Second)
		if i%20 == 19 {
			t.Logf("%s left, server memory %s", time.Until(deadline).Round(time.Second), serverMemory(t, srv))
		}
	}

	waitConverged(t, workStack)
	eventually(t, 60*time.Second, "graph matches the swarm", func() error {
		out := on(t, leader, "service", "ls", "--format", "{{.Name}} {{.Replicas}}")
		snap := snapshot(t)
		for _, line := range strings.Split(out, "\n") {
			name, replicas, _ := strings.Cut(line, " ")
			if strings.HasPrefix(name, "dg_") {
				continue
			}
			n, err := serviceNode(snap, name)
			if err != nil {
				return err
			}
			got := fmt.Sprintf("%d/%d", n.Service.Replicas.Running, n.Service.Replicas.Desired)
			if got != strings.Fields(replicas)[0] {
				return fmt.Errorf("%s: graph %s, swarm %s", name, got, replicas)
			}
		}
		return nil
	})

	t.Logf("server memory at end: %s", serverMemory(t, srv))
	if serverTaskID(t) != before {
		t.Errorf("server restarted during the soak")
	}
}

// ── Helpers ───────────────────────────────────────────────────

func createSecret(t *testing.T, name, value string) {
	t.Helper()
	cmd := exec.Command("docker", "exec", "-i", leader, "docker", "secret", "create", name, "-")
	cmd.Stdin = strings.NewReader(value)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("secret create %s: %v\n%s", name, err, out)
	}
}

func randomHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// containerName returns a container's name on node, as the inspect API
// reports it.
func containerName(t *testing.T, node, id string) string {
	t.Helper()
	return strings.TrimPrefix(on(t, node, "inspect", "-f", "{{.Name}}", id), "/")
}

// inspected reports whether r is a successful inspect of the named container.
func inspected(r response, name string) bool {
	return r.code == http.StatusOK && strings.Contains(r.body, `"name":"`+name+`"`)
}

// serverMemory reports the server container's memory usage on its node.
func serverMemory(t *testing.T, node string) string {
	t.Helper()
	id := firstLine(on(t, node, "ps", "-q", "--filter", "label=com.docker.swarm.service.name="+serverService))
	if id == "" {
		return "unknown"
	}
	return on(t, node, "stats", "--no-stream", "--format", "{{.MemUsage}}", id)
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

package state

import (
	"testing"
	"time"

	"github.com/dockgraph/dockgraph/collector"
)

func TestMergeSnapshots(t *testing.T) {
	m := NewManager()

	composeSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:api", Type: "container", Name: "api", Status: "not_running", Source: "main.yaml"},
			{ID: "network:backend", Type: "network", Name: "backend", Source: "main.yaml"},
		},
	}

	dockerSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:api", Type: "container", Name: "api", Status: "running", Image: "python:3.12"},
			{ID: "network:abc123", Type: "network", Name: "backend", Driver: "bridge"},
		},
	}

	m.HandleUpdate("compose", false, collector.StateUpdate{Snapshot: &composeSnap})
	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: &dockerSnap})

	merged := m.Current()

	var apiNode *collector.Node
	for _, n := range merged.Nodes {
		if n.Name == "api" && n.Type == "container" {
			apiNode = &n
			break
		}
	}

	if apiNode == nil {
		t.Fatal("api container node not found in merged snapshot")
	}
	if apiNode.Status != "running" {
		t.Errorf("expected running status from docker, got %s", apiNode.Status)
	}
}

func TestMergeVolumeStatus(t *testing.T) {
	m := NewManager()

	composeSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "volume:myapp_pgdata", Type: "volume", Name: "myapp_pgdata", Status: "not_running", Source: "compose.yaml"},
		},
	}

	dockerSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "volume:myapp_pgdata", Type: "volume", Name: "myapp_pgdata", Status: "created", Driver: "local"},
		},
	}

	m.HandleUpdate("compose", false, collector.StateUpdate{Snapshot: &composeSnap})
	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: &dockerSnap})

	merged := m.Current()

	var volNode *collector.Node
	for _, n := range merged.Nodes {
		if n.Name == "myapp_pgdata" && n.Type == "volume" {
			volNode = &n
			break
		}
	}

	if volNode == nil {
		t.Fatal("volume node not found in merged snapshot")
	}
	if volNode.Status != "created" {
		t.Errorf("expected docker status 'created' to win, got %s", volNode.Status)
	}
	if volNode.Source != "compose.yaml" {
		t.Errorf("expected compose source to be preserved, got %s", volNode.Source)
	}
}

func TestSubscribe(t *testing.T) {
	m := NewManager()
	ch, unsub := m.Subscribe()
	defer unsub()

	go func() {
		m.HandleUpdate("test", true, collector.StateUpdate{
			Snapshot: &collector.GraphSnapshot{
				Nodes: []collector.Node{
					{ID: "container:x", Type: "container", Name: "x"},
				},
			},
		})
	}()

	select {
	case msg := <-ch:
		if msg.Type != "snapshot" {
			t.Errorf("expected snapshot, got %s", msg.Type)
		}
		if len(msg.Snapshot.Nodes) == 0 {
			t.Error("expected nodes in snapshot")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for subscription message")
	}
}

func TestUnsubscribe(t *testing.T) {
	m := NewManager()
	_, unsub := m.Subscribe()
	_, unsub2 := m.Subscribe()

	unsub()

	m.mu.RLock()
	count := len(m.subscribers)
	m.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", count)
	}

	unsub2()
	m.mu.RLock()
	count = len(m.subscribers)
	m.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 subscribers after both unsubscribe, got %d", count)
	}
}

func TestDoubleUnsubscribe(t *testing.T) {
	m := NewManager()
	_, unsub := m.Subscribe()

	unsub()
	unsub() // second call must not panic

	m.mu.RLock()
	count := len(m.subscribers)
	m.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 subscribers, got %d", count)
	}
}

func TestSubscribeReceivesExistingState(t *testing.T) {
	m := NewManager()

	m.HandleUpdate("docker", true, collector.StateUpdate{
		Snapshot: &collector.GraphSnapshot{
			Nodes: []collector.Node{
				{ID: "container:web", Type: "container", Name: "web"},
			},
		},
	})

	// Subscribing after state exists should immediately get the current snapshot
	ch, unsub := m.Subscribe()
	defer unsub()

	select {
	case msg := <-ch:
		if msg.Type != "snapshot" {
			t.Errorf("expected snapshot, got %s", msg.Type)
		}
		if len(msg.Snapshot.Nodes) == 0 {
			t.Error("expected nodes in immediate snapshot")
		}
	case <-time.After(time.Second):
		t.Fatal("expected immediate snapshot on subscribe")
	}
}

func TestHandleUpdateNilSnapshot(t *testing.T) {
	m := NewManager()
	// Should not panic
	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: nil})

	merged := m.Current()
	if len(merged.Nodes) != 0 {
		t.Errorf("expected 0 nodes after nil snapshot update, got %d", len(merged.Nodes))
	}
}

func TestCurrent(t *testing.T) {
	m := NewManager()

	empty := m.Current()
	if len(empty.Nodes) != 0 {
		t.Errorf("expected empty current, got %d nodes", len(empty.Nodes))
	}

	m.HandleUpdate("docker", true, collector.StateUpdate{
		Snapshot: &collector.GraphSnapshot{
			Nodes: []collector.Node{
				{ID: "container:x", Type: "container", Name: "x"},
			},
		},
	})

	current := m.Current()
	if len(current.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(current.Nodes))
	}
}

func TestMergeEdgesDeduplicate(t *testing.T) {
	m := NewManager()

	snap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:a", Type: "container", Name: "a"},
			{ID: "container:b", Type: "container", Name: "b"},
		},
		Edges: []collector.Edge{
			{ID: "e:dep:a:b", Type: "depends_on", Source: "container:a", Target: "container:b"},
			{ID: "e:dep:a:b", Type: "depends_on", Source: "container:a", Target: "container:b"}, // duplicate
		},
	}

	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: &snap})
	merged := m.Current()

	if len(merged.Edges) != 1 {
		t.Errorf("expected 1 edge (deduplicated), got %d", len(merged.Edges))
	}
}

func TestMergeEdgesDanglingFiltered(t *testing.T) {
	m := NewManager()

	snap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:a", Type: "container", Name: "a"},
		},
		Edges: []collector.Edge{
			{ID: "e:dep:a:missing", Type: "depends_on", Source: "container:a", Target: "container:missing"},
		},
	}

	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: &snap})
	merged := m.Current()

	if len(merged.Edges) != 0 {
		t.Errorf("expected 0 edges (target missing), got %d", len(merged.Edges))
	}
}

func TestMergeEdgesDockerPrecedence(t *testing.T) {
	m := NewManager()

	composeSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web"},
			{ID: "container:db", Type: "container", Name: "db"},
		},
		Edges: []collector.Edge{
			{ID: "e:dep:web:db", Type: "depends_on", Source: "container:web", Target: "container:db"},
		},
	}

	dockerSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "running"},
			{ID: "container:db", Type: "container", Name: "db", Status: "running"},
		},
		Edges: []collector.Edge{
			{ID: "e:dep:web:db", Type: "depends_on", Source: "container:web", Target: "container:db"},
		},
	}

	m.HandleUpdate("compose", false, collector.StateUpdate{Snapshot: &composeSnap})
	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: &dockerSnap})

	merged := m.Current()
	if len(merged.Edges) != 1 {
		t.Errorf("expected 1 edge (docker wins over duplicate compose), got %d", len(merged.Edges))
	}
}

func TestMergeNodesComposeOnlyPreserved(t *testing.T) {
	m := NewManager()

	composeSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:worker", Type: "container", Name: "worker", Status: "not_running", Source: "compose.yml"},
		},
	}

	dockerSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "running"},
		},
	}

	m.HandleUpdate("compose", false, collector.StateUpdate{Snapshot: &composeSnap})
	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: &dockerSnap})

	merged := m.Current()
	if len(merged.Nodes) != 2 {
		t.Fatalf("expected 2 nodes (docker + compose-only), got %d", len(merged.Nodes))
	}

	var worker *collector.Node
	for _, n := range merged.Nodes {
		if n.Name == "worker" {
			worker = &n
		}
	}
	if worker == nil {
		t.Fatal("compose-only worker node not in merged result")
	}
	if worker.Status != "not_running" {
		t.Errorf("expected not_running status, got %s", worker.Status)
	}
}

func TestMergeNodesDockerBackfillsNetworkID(t *testing.T) {
	m := NewManager()

	composeSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:api", Type: "container", Name: "api", NetworkID: "network:backend", Source: "compose.yml"},
		},
	}

	dockerSnap := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:api", Type: "container", Name: "api", Status: "running"},
		},
	}

	m.HandleUpdate("compose", false, collector.StateUpdate{Snapshot: &composeSnap})
	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: &dockerSnap})

	merged := m.Current()
	var api *collector.Node
	for _, n := range merged.Nodes {
		if n.Name == "api" {
			api = &n
		}
	}
	if api == nil {
		t.Fatal("api node not found")
	}
	if api.NetworkID != "network:backend" {
		t.Errorf("expected compose NetworkID to be backfilled, got %q", api.NetworkID)
	}
	if api.Source != "compose.yml" {
		t.Errorf("expected compose Source to be preserved, got %q", api.Source)
	}
	if api.Status != "running" {
		t.Errorf("expected docker Status to win, got %q", api.Status)
	}
}

func TestMergeEmptySnapshots(t *testing.T) {
	m := NewManager()
	m.HandleUpdate("docker", true, collector.StateUpdate{
		Snapshot: &collector.GraphSnapshot{},
	})
	merged := m.Current()
	if merged.Nodes == nil {
		t.Error("expected non-nil nodes slice")
	}
	if merged.Edges == nil {
		t.Error("expected non-nil edges slice")
	}
}

func TestMergeSnapshotsDeterministicOrder(t *testing.T) {
	m := NewManager()

	m.HandleUpdate("docker", true, collector.StateUpdate{
		Snapshot: &collector.GraphSnapshot{
			Nodes: []collector.Node{
				{ID: "container:z", Type: "container", Name: "z"},
				{ID: "container:a", Type: "container", Name: "a"},
			},
		},
	})

	merged := m.Current()
	if len(merged.Nodes) < 2 {
		t.Fatalf("expected 2 nodes, got %d", len(merged.Nodes))
	}
	if merged.Nodes[0].ID >= merged.Nodes[1].ID {
		t.Errorf("expected sorted nodes, got %s >= %s", merged.Nodes[0].ID, merged.Nodes[1].ID)
	}
}

func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	m := NewManager()
	ch, unsub := m.Subscribe()
	defer unsub()

	// Fill the subscriber channel (capacity 16)
	for i := 0; i < 20; i++ {
		m.HandleUpdate("docker", true, collector.StateUpdate{
			Snapshot: &collector.GraphSnapshot{
				Nodes: []collector.Node{
					{ID: "container:x", Type: "container", Name: "x"},
				},
			},
		})
	}

	// Should not deadlock — drain what we can
	drained := 0
	for {
		select {
		case <-ch:
			drained++
		default:
			goto done
		}
	}
done:
	if drained == 0 {
		t.Error("expected at least some messages delivered")
	}
}

func TestHandleUpdateEmitsDelta(t *testing.T) {
	m := NewManager()

	// First update: always a snapshot
	m.HandleUpdate("docker", true, collector.StateUpdate{
		Snapshot: &collector.GraphSnapshot{
			Nodes: []collector.Node{
				{ID: "container:web", Type: "container", Name: "web", Status: "running"},
			},
		},
	})

	ch, unsub := m.Subscribe()
	defer unsub()

	// Drain the initial snapshot from Subscribe
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial snapshot")
	}

	// Second update: status change should produce a delta
	m.HandleUpdate("docker", true, collector.StateUpdate{
		Snapshot: &collector.GraphSnapshot{
			Nodes: []collector.Node{
				{ID: "container:web", Type: "container", Name: "web", Status: "exited"},
			},
		},
	})

	select {
	case msg := <-ch:
		if msg.Type != "delta" {
			t.Errorf("expected delta, got %s", msg.Type)
		}
		if msg.Delta == nil {
			t.Fatal("expected delta payload")
		}
		if len(msg.Delta.NodesUpdated) != 1 {
			t.Errorf("expected 1 updated node, got %d", len(msg.Delta.NodesUpdated))
		}
		if msg.Snapshot == nil {
			t.Error("expected snapshot attached for hub bookkeeping")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for delta message")
	}
}

func TestHandleUpdateSkipsWhenUnchanged(t *testing.T) {
	m := NewManager()

	snap := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "running"},
		},
	}

	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: snap})

	ch, unsub := m.Subscribe()
	defer unsub()

	// Drain initial snapshot
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial snapshot")
	}

	// Same data again — should not produce any message
	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: snap})

	select {
	case msg := <-ch:
		t.Errorf("expected no message for unchanged state, got type=%s", msg.Type)
	case <-time.After(200 * time.Millisecond):
		// success — no message
	}
}

func TestFirstUpdateIsSnapshot(t *testing.T) {
	m := NewManager()
	ch, unsub := m.Subscribe()
	defer unsub()

	m.HandleUpdate("docker", true, collector.StateUpdate{
		Snapshot: &collector.GraphSnapshot{
			Nodes: []collector.Node{
				{ID: "container:web", Type: "container", Name: "web"},
			},
		},
	})

	select {
	case msg := <-ch:
		if msg.Type != "snapshot" {
			t.Errorf("first update should be snapshot, got %s", msg.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first snapshot")
	}
}

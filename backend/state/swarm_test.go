package state

import (
	"testing"

	"github.com/dockgraph/dockgraph/collector"
)

// A stack-file ghost service and the live swarm service share (Name, Type),
// so they merge into one node: live state wins, compose metadata is backfilled.
func TestMergeStackGhostWithLiveService(t *testing.T) {
	m := NewManager()

	ghost := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{
				ID: "service:shop_web", Type: "service", Name: "shop_web", Status: "not_running", Source: "shop.yml", Stack: "shop",
				NetworkID: "network:shop_back",
				Service:   &collector.ServiceInfo{Mode: "replicated", Replicas: collector.ReplicaCount{Desired: 3}},
			},
			{ID: "service:shop_worker", Type: "service", Name: "shop_worker", Status: "not_running", Source: "shop.yml", Stack: "shop"},
			{ID: "network:shop_back", Type: "network", Name: "shop_back", Status: "not_running", Source: "shop.yml", Stack: "shop"},
		},
	}
	live := collector.GraphSnapshot{
		Nodes: []collector.Node{
			{
				ID: "service:shop_web", Type: "service", Name: "shop_web", Status: "running", Stack: "shop",
				Service: &collector.ServiceInfo{Mode: "replicated", Replicas: collector.ReplicaCount{Running: 3, Desired: 3}},
			},
			{ID: "network:shop_back", Type: "network", Name: "shop_back", Driver: "overlay", Stack: "shop"},
		},
	}

	m.HandleUpdate("compose", false, collector.StateUpdate{Snapshot: &ghost})
	m.HandleUpdate("docker", true, collector.StateUpdate{Snapshot: &live})
	merged := m.Current()

	var web, worker *collector.Node
	count := 0
	for i := range merged.Nodes {
		n := &merged.Nodes[i]
		if n.Name == "shop_web" {
			web = n
			count++
		}
		if n.Name == "shop_worker" {
			worker = n
		}
	}
	if count != 1 || web == nil {
		t.Fatalf("expected exactly one merged shop_web node, got %d", count)
	}
	if web.Status != "running" || web.Service.Replicas.Running != 3 {
		t.Errorf("live state should win: %+v", web)
	}
	if web.Source != "shop.yml" {
		t.Errorf("source not backfilled: %q", web.Source)
	}
	if worker == nil || worker.Status != "not_running" {
		t.Errorf("undeployed ghost should remain: %+v", worker)
	}
}

func TestNodeEqualSwarmFields(t *testing.T) {
	base := collector.Node{
		ID: "service:a", Type: "service", Name: "a", Stack: "s",
		Service: &collector.ServiceInfo{
			Mode: "replicated", Replicas: collector.ReplicaCount{Running: 1, Desired: 2},
			Tasks: []collector.TaskInfo{{ID: "t1", State: "running"}},
		},
	}

	same := base
	same.Service = &collector.ServiceInfo{
		Mode: "replicated", Replicas: collector.ReplicaCount{Running: 1, Desired: 2},
		Tasks: []collector.TaskInfo{{ID: "t1", State: "running"}},
	}
	if !nodeEqual(base, same) {
		t.Error("equal service info should compare equal")
	}

	tests := []struct {
		name   string
		mutate func(n *collector.Node)
	}{
		{"stack", func(n *collector.Node) { n.Stack = "other" }},
		{"replicas", func(n *collector.Node) {
			n.Service = &collector.ServiceInfo{
				Mode: "replicated", Replicas: collector.ReplicaCount{Running: 2, Desired: 2},
				Tasks: []collector.TaskInfo{{ID: "t1", State: "running"}},
			}
		}},
		{"task state", func(n *collector.Node) {
			n.Service = &collector.ServiceInfo{
				Mode: "replicated", Replicas: collector.ReplicaCount{Running: 1, Desired: 2},
				Tasks: []collector.TaskInfo{{ID: "t1", State: "failed"}},
			}
		}},
		{"service removed", func(n *collector.Node) { n.Service = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := base
			tt.mutate(&changed)
			if nodeEqual(base, changed) {
				t.Error("change not detected")
			}
		})
	}
}

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

	m.HandleUpdate("compose", collector.StateUpdate{Snapshot: &composeSnap})
	m.HandleUpdate("docker", collector.StateUpdate{Snapshot: &dockerSnap})

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

func TestSubscribe(t *testing.T) {
	m := NewManager()
	ch := m.Subscribe()

	go func() {
		m.HandleUpdate("test", collector.StateUpdate{
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

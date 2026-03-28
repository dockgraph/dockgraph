package state

import (
	"reflect"
	"testing"

	"github.com/dockgraph/dockgraph/collector"
)

func TestDiffNoChange(t *testing.T) {
	snap := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "running"},
		},
		Edges: []collector.Edge{
			{ID: "e:dep:web:db", Type: "depends_on", Source: "container:web", Target: "container:db"},
		},
	}

	delta, changed := diffSnapshots(snap, snap)
	if changed {
		t.Error("expected no change for identical snapshots")
	}
	if len(delta.NodesAdded) != 0 || len(delta.NodesRemoved) != 0 || len(delta.NodesUpdated) != 0 {
		t.Errorf("expected empty delta, got %+v", delta)
	}
}

func TestDiffNodeAdded(t *testing.T) {
	old := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web"},
		},
	}
	curr := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web"},
			{ID: "container:db", Type: "container", Name: "db"},
		},
	}

	delta, changed := diffSnapshots(old, curr)
	if !changed {
		t.Fatal("expected change")
	}
	if len(delta.NodesAdded) != 1 || delta.NodesAdded[0].ID != "container:db" {
		t.Errorf("expected db added, got %+v", delta.NodesAdded)
	}
}

func TestDiffNodeRemoved(t *testing.T) {
	old := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web"},
			{ID: "container:db", Type: "container", Name: "db"},
		},
	}
	curr := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web"},
		},
	}

	delta, changed := diffSnapshots(old, curr)
	if !changed {
		t.Fatal("expected change")
	}
	if len(delta.NodesRemoved) != 1 || delta.NodesRemoved[0] != "container:db" {
		t.Errorf("expected db removed, got %+v", delta.NodesRemoved)
	}
}

func TestDiffNodeUpdated(t *testing.T) {
	old := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "running"},
		},
	}
	curr := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "exited"},
		},
	}

	delta, changed := diffSnapshots(old, curr)
	if !changed {
		t.Fatal("expected change")
	}
	if len(delta.NodesUpdated) != 1 || delta.NodesUpdated[0].Status != "exited" {
		t.Errorf("expected web updated with exited status, got %+v", delta.NodesUpdated)
	}
}

func TestDiffEdgeAdded(t *testing.T) {
	old := &collector.GraphSnapshot{
		Edges: []collector.Edge{},
	}
	curr := &collector.GraphSnapshot{
		Edges: []collector.Edge{
			{ID: "e:dep:web:db", Type: "depends_on", Source: "container:web", Target: "container:db"},
		},
	}

	delta, changed := diffSnapshots(old, curr)
	if !changed {
		t.Fatal("expected change")
	}
	if len(delta.EdgesAdded) != 1 || delta.EdgesAdded[0].ID != "e:dep:web:db" {
		t.Errorf("expected edge added, got %+v", delta.EdgesAdded)
	}
}

func TestDiffEdgeRemoved(t *testing.T) {
	old := &collector.GraphSnapshot{
		Edges: []collector.Edge{
			{ID: "e:dep:web:db", Type: "depends_on", Source: "container:web", Target: "container:db"},
		},
	}
	curr := &collector.GraphSnapshot{
		Edges: []collector.Edge{},
	}

	delta, changed := diffSnapshots(old, curr)
	if !changed {
		t.Fatal("expected change")
	}
	if len(delta.EdgesRemoved) != 1 || delta.EdgesRemoved[0] != "e:dep:web:db" {
		t.Errorf("expected edge removed, got %+v", delta.EdgesRemoved)
	}
}

func TestDiffMixed(t *testing.T) {
	old := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "running"},
			{ID: "container:old", Type: "container", Name: "old"},
		},
		Edges: []collector.Edge{
			{ID: "e:dep:web:old", Type: "depends_on", Source: "container:web", Target: "container:old"},
		},
	}
	curr := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "exited"},
			{ID: "container:new", Type: "container", Name: "new"},
		},
		Edges: []collector.Edge{
			{ID: "e:dep:web:new", Type: "depends_on", Source: "container:web", Target: "container:new"},
		},
	}

	delta, changed := diffSnapshots(old, curr)
	if !changed {
		t.Fatal("expected change")
	}
	if len(delta.NodesAdded) != 1 {
		t.Errorf("expected 1 node added, got %d", len(delta.NodesAdded))
	}
	if len(delta.NodesRemoved) != 1 {
		t.Errorf("expected 1 node removed, got %d", len(delta.NodesRemoved))
	}
	if len(delta.NodesUpdated) != 1 {
		t.Errorf("expected 1 node updated, got %d", len(delta.NodesUpdated))
	}
	if len(delta.EdgesAdded) != 1 {
		t.Errorf("expected 1 edge added, got %d", len(delta.EdgesAdded))
	}
	if len(delta.EdgesRemoved) != 1 {
		t.Errorf("expected 1 edge removed, got %d", len(delta.EdgesRemoved))
	}
}

func TestDiffNilSnapshots(t *testing.T) {
	node := collector.Node{ID: "container:web", Type: "container", Name: "web"}
	snap := &collector.GraphSnapshot{Nodes: []collector.Node{node}}

	// nil old = everything is new
	delta, changed := diffSnapshots(nil, snap)
	if !changed {
		t.Fatal("expected change")
	}
	if len(delta.NodesAdded) != 1 {
		t.Errorf("expected 1 node added, got %d", len(delta.NodesAdded))
	}

	// nil new = everything removed
	delta, changed = diffSnapshots(snap, nil)
	if !changed {
		t.Fatal("expected change")
	}
	if len(delta.NodesRemoved) != 1 {
		t.Errorf("expected 1 node removed, got %d", len(delta.NodesRemoved))
	}

	// both nil = no change
	delta, changed = diffSnapshots(nil, nil)
	if changed {
		t.Error("expected no change for two nil snapshots")
	}
	_ = delta
}

func TestDiffNodeWithLabelsChanged(t *testing.T) {
	old := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Labels: map[string]string{"env": "dev"}},
		},
	}
	curr := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Labels: map[string]string{"env": "prod"}},
		},
	}

	delta, changed := diffSnapshots(old, curr)
	if !changed {
		t.Fatal("expected change when labels differ")
	}
	if len(delta.NodesUpdated) != 1 {
		t.Errorf("expected 1 node updated, got %d", len(delta.NodesUpdated))
	}
	if !reflect.DeepEqual(delta.NodesUpdated[0].Labels, map[string]string{"env": "prod"}) {
		t.Errorf("expected updated labels, got %+v", delta.NodesUpdated[0].Labels)
	}
}

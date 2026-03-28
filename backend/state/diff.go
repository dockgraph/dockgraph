package state

import (
	"reflect"

	"github.com/dockgraph/dockgraph/collector"
)

// diffSnapshots compares two graph snapshots and returns a DeltaUpdate
// describing the differences. The changed return value is false when the
// snapshots are identical, allowing callers to skip broadcasting entirely.
func diffSnapshots(old, new *collector.GraphSnapshot) (collector.DeltaUpdate, bool) {
	var delta collector.DeltaUpdate

	oldNodes := indexNodes(old)
	newNodes := indexNodes(new)

	for id, newNode := range newNodes {
		oldNode, exists := oldNodes[id]
		if !exists {
			delta.NodesAdded = append(delta.NodesAdded, newNode)
		} else if !reflect.DeepEqual(oldNode, newNode) {
			delta.NodesUpdated = append(delta.NodesUpdated, newNode)
		}
	}
	for id := range oldNodes {
		if _, exists := newNodes[id]; !exists {
			delta.NodesRemoved = append(delta.NodesRemoved, id)
		}
	}

	oldEdges := indexEdges(old)
	newEdges := indexEdges(new)

	for id, newEdge := range newEdges {
		if _, exists := oldEdges[id]; !exists {
			delta.EdgesAdded = append(delta.EdgesAdded, newEdge)
		}
	}
	for id := range oldEdges {
		if _, exists := newEdges[id]; !exists {
			delta.EdgesRemoved = append(delta.EdgesRemoved, id)
		}
	}

	changed := len(delta.NodesAdded) > 0 ||
		len(delta.NodesRemoved) > 0 ||
		len(delta.NodesUpdated) > 0 ||
		len(delta.EdgesAdded) > 0 ||
		len(delta.EdgesRemoved) > 0

	return delta, changed
}

func indexNodes(snap *collector.GraphSnapshot) map[string]collector.Node {
	if snap == nil {
		return nil
	}
	m := make(map[string]collector.Node, len(snap.Nodes))
	for _, n := range snap.Nodes {
		m[n.ID] = n
	}
	return m
}

func indexEdges(snap *collector.GraphSnapshot) map[string]collector.Edge {
	if snap == nil {
		return nil
	}
	m := make(map[string]collector.Edge, len(snap.Edges))
	for _, e := range snap.Edges {
		m[e.ID] = e
	}
	return m
}

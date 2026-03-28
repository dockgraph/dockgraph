package state

import (
	"reflect"

	"github.com/dockgraph/dockgraph/collector"
)

// diffSnapshots compares two graph snapshots and returns a DeltaUpdate
// describing the differences. The changed return value is false when the
// snapshots are identical, allowing callers to skip broadcasting entirely.
func diffSnapshots(prev, curr *collector.GraphSnapshot) (collector.DeltaUpdate, bool) {
	var delta collector.DeltaUpdate

	prevNodes := indexNodes(prev)
	currNodes := indexNodes(curr)

	for id, currNode := range currNodes {
		prevNode, exists := prevNodes[id]
		if !exists {
			delta.NodesAdded = append(delta.NodesAdded, currNode)
		} else if !reflect.DeepEqual(prevNode, currNode) {
			delta.NodesUpdated = append(delta.NodesUpdated, currNode)
		}
	}
	for id := range prevNodes {
		if _, exists := currNodes[id]; !exists {
			delta.NodesRemoved = append(delta.NodesRemoved, id)
		}
	}

	prevEdges := indexEdges(prev)
	currEdges := indexEdges(curr)

	for id, currEdge := range currEdges {
		if _, exists := prevEdges[id]; !exists {
			delta.EdgesAdded = append(delta.EdgesAdded, currEdge)
		}
	}
	for id := range prevEdges {
		if _, exists := currEdges[id]; !exists {
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

// Package state merges snapshots from multiple collectors into a single
// unified graph and notifies subscribers of changes.
package state

import (
	"sort"
	"sync"

	"github.com/dockgraph/dockgraph/collector"
)

type subID int

// Manager receives graph snapshots from Docker and Compose collectors,
// merges them into a single topology, and broadcasts updates to WebSocket subscribers.
type Manager struct {
	mu          sync.RWMutex
	snapshots   map[string]*collector.GraphSnapshot
	merged      collector.GraphSnapshot
	hasState    bool
	subscribers map[subID]chan collector.StateMessage
	nextID      subID
}

// NewManager creates an empty state manager ready to accept collector updates.
func NewManager() *Manager {
	return &Manager{
		snapshots:   make(map[string]*collector.GraphSnapshot),
		subscribers: make(map[subID]chan collector.StateMessage),
	}
}

// Subscribe returns a channel that receives state messages whenever the merged
// graph changes, along with a cleanup function to unsubscribe. If a merged
// graph already exists, the current state is sent immediately so new
// subscribers start with a complete view.
func (m *Manager) Subscribe() (<-chan collector.StateMessage, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan collector.StateMessage, 16)
	m.nextID++
	id := m.nextID
	m.subscribers[id] = ch

	if len(m.merged.Nodes) > 0 {
		ch <- collector.StateMessage{
			Type:     "snapshot",
			Snapshot: &m.merged,
		}
	}

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			m.mu.Lock()
			delete(m.subscribers, id)
			close(ch)
			m.mu.Unlock()
		})
	}

	return ch, unsub
}

// Current returns the latest merged graph snapshot.
func (m *Manager) Current() collector.GraphSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.merged
}

// HandleUpdate stores a new snapshot from a named collector, re-merges all
// snapshots, and broadcasts the result to subscribers. The first update always
// produces a full snapshot. Subsequent updates emit a delta when the graph
// changed, or skip the broadcast entirely when nothing changed.
func (m *Manager) HandleUpdate(name string, update collector.StateUpdate) {
	m.mu.Lock()

	if update.Snapshot != nil {
		m.snapshots[name] = update.Snapshot
	}

	// Separate by source so docker snapshots get merge precedence for runtime state
	// (actual container status, ports, etc.) while compose-only nodes are preserved.
	composeSnaps := make(map[string]*collector.GraphSnapshot)
	dockerSnaps := make(map[string]*collector.GraphSnapshot)
	for k, v := range m.snapshots {
		if k == "docker" {
			dockerSnaps[k] = v
		} else {
			composeSnaps[k] = v
		}
	}

	prev := m.merged
	hadState := m.hasState
	m.merged = mergeSnapshots(composeSnaps, dockerSnaps)
	m.hasState = true
	snapshot := m.merged

	m.mu.Unlock()

	var msg collector.StateMessage
	if !hadState {
		msg = collector.StateMessage{
			Type:     "snapshot",
			Snapshot: &snapshot,
		}
	} else {
		delta, changed := diffSnapshots(&prev, &snapshot)
		if !changed {
			return
		}
		msg = collector.StateMessage{
			Type:     "delta",
			Delta:    &delta,
			Snapshot: &snapshot,
		}
	}

	m.mu.RLock()
	for _, ch := range m.subscribers {
		select {
		case ch <- msg:
		default:
		}
	}
	m.mu.RUnlock()
}

// mergeSnapshots combines Docker and Compose snapshots into a single graph.
//
// Merge strategy: Docker data takes precedence for nodes that exist in both
// sources (since Docker has the actual runtime state), but compose-only metadata
// like Source and NetworkID is preserved when Docker doesn't provide it.
// Nodes that only exist in compose files (not yet running) are included as-is.
func mergeSnapshots(composeSnaps, dockerSnaps map[string]*collector.GraphSnapshot) collector.GraphSnapshot {
	composeNodes, composeEdges := flattenSnapshots(composeSnaps)
	dockerNodes, dockerEdges := flattenSnapshots(dockerSnaps)

	nodes := mergeNodes(dockerNodes, composeNodes)
	edges := mergeEdges(dockerEdges, composeEdges, nodes)

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })

	if nodes == nil {
		nodes = []collector.Node{}
	}
	if edges == nil {
		edges = []collector.Edge{}
	}
	return collector.GraphSnapshot{Nodes: nodes, Edges: edges}
}

type nodeKey struct {
	name     string
	nodeType string
}

// flattenSnapshots combines multiple snapshots into a single node map and edge list.
func flattenSnapshots(snaps map[string]*collector.GraphSnapshot) (map[nodeKey]collector.Node, []collector.Edge) {
	nodes := make(map[nodeKey]collector.Node)
	var edges []collector.Edge
	for _, snap := range snaps {
		for _, n := range snap.Nodes {
			nodes[nodeKey{n.Name, n.Type}] = n
		}
		edges = append(edges, snap.Edges...)
	}
	return nodes, edges
}

// mergeNodes combines docker and compose nodes, giving docker precedence.
// Compose-only metadata (Source, NetworkID) is backfilled when docker doesn't provide it.
func mergeNodes(dockerNodes, composeNodes map[nodeKey]collector.Node) []collector.Node {
	var result []collector.Node
	seen := make(map[nodeKey]bool)

	for key, dockerNode := range dockerNodes {
		seen[key] = true
		if composeNode, ok := composeNodes[key]; ok {
			merged := dockerNode
			if composeNode.Source != "" {
				merged.Source = composeNode.Source
			}
			if merged.NetworkID == "" && composeNode.NetworkID != "" {
				merged.NetworkID = composeNode.NetworkID
			}
			result = append(result, merged)
		} else {
			result = append(result, dockerNode)
		}
	}

	for key, composeNode := range composeNodes {
		if !seen[key] {
			result = append(result, composeNode)
		}
	}

	return result
}

// mergeEdges combines edges from both sources, giving docker precedence. Skips
// duplicate IDs and edges whose source or target node is absent from the merged set.
func mergeEdges(dockerEdges, composeEdges []collector.Edge, nodes []collector.Node) []collector.Edge {
	nodeIDs := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}

	var result []collector.Edge
	seen := make(map[string]bool)

	add := func(e collector.Edge) {
		if seen[e.ID] || !nodeIDs[e.Source] || !nodeIDs[e.Target] {
			return
		}
		seen[e.ID] = true
		result = append(result, e)
	}

	// Docker edges first (they reflect actual runtime state)
	for _, e := range dockerEdges {
		add(e)
	}
	for _, e := range composeEdges {
		add(e)
	}

	return result
}

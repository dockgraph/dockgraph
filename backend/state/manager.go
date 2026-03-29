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
	mu             sync.RWMutex
	snapshots      map[string]*collector.GraphSnapshot
	runtimeSources map[string]bool // true if the source provides runtime state
	merged         collector.GraphSnapshot
	hasState       bool
	subscribers    map[subID]chan collector.StateMessage
	nextID         subID
}

// NewManager creates an empty state manager ready to accept collector updates.
func NewManager() *Manager {
	return &Manager{
		snapshots:      make(map[string]*collector.GraphSnapshot),
		runtimeSources: make(map[string]bool),
		subscribers:    make(map[subID]chan collector.StateMessage),
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
		snap := m.merged
		ch <- collector.StateMessage{
			Type:     "snapshot",
			Snapshot: &snap,
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
// snapshots, and broadcasts the result to subscribers. The runtime flag
// indicates whether this source provides live state (e.g. Docker daemon) or
// declarative definitions (e.g. compose files). Runtime sources take precedence
// during merges. The first update always produces a full snapshot. Subsequent
// updates emit a delta when the graph changed, or skip the broadcast entirely
// when nothing changed.
func (m *Manager) HandleUpdate(name string, runtime bool, update collector.StateUpdate) {
	m.mu.Lock()

	if update.Snapshot != nil {
		m.snapshots[name] = update.Snapshot
		m.runtimeSources[name] = runtime
	}

	// Separate by source type so runtime snapshots get merge precedence for
	// actual state (container status, ports, etc.) while declarative-only
	// nodes are preserved.
	declarativeSnaps := make(map[string]*collector.GraphSnapshot)
	runtimeSnaps := make(map[string]*collector.GraphSnapshot)
	for k, v := range m.snapshots {
		if m.runtimeSources[k] {
			runtimeSnaps[k] = v
		} else {
			declarativeSnaps[k] = v
		}
	}

	prev := m.merged
	hadState := m.hasState
	m.merged = mergeSnapshots(declarativeSnaps, runtimeSnaps)
	m.hasState = true
	snapshot := m.merged

	// Build and broadcast the message under the same lock to prevent a new
	// subscriber (via Subscribe) from receiving a stale snapshot and then
	// also catching the broadcast — which would duplicate the initial state.
	var msg collector.StateMessage
	if !hadState {
		msg = collector.StateMessage{
			Type:     "snapshot",
			Snapshot: &snapshot,
		}
	} else {
		delta, changed := diffSnapshots(&prev, &snapshot)
		if !changed {
			m.mu.Unlock()
			return
		}
		msg = collector.StateMessage{
			Type:     "delta",
			Delta:    &delta,
			Snapshot: &snapshot,
		}
	}

	for _, ch := range m.subscribers {
		select {
		case ch <- msg:
		default:
		}
	}
	m.mu.Unlock()
}

// mergeSnapshots combines runtime and declarative snapshots into a single graph.
//
// Merge strategy: runtime data takes precedence for nodes that exist in both
// source types (since it reflects the actual state), but declarative-only
// metadata like Source and NetworkID is preserved when the runtime source
// doesn't provide it. Nodes that only exist in declarative sources are
// included as-is.
func mergeSnapshots(declarativeSnaps, runtimeSnaps map[string]*collector.GraphSnapshot) collector.GraphSnapshot {
	composeNodes, composeEdges := flattenSnapshots(declarativeSnaps)
	dockerNodes, dockerEdges := flattenSnapshots(runtimeSnaps)

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

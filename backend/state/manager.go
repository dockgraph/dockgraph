package state

import (
	"sort"
	"sync"

	"github.com/dockgraph/docker-flow/collector"
)

type subID int

type Manager struct {
	mu          sync.RWMutex
	snapshots   map[string]*collector.GraphSnapshot
	merged      collector.GraphSnapshot
	subscribers map[subID]chan collector.StateMessage
	nextID      subID
}

func NewManager() *Manager {
	return &Manager{
		snapshots:   make(map[string]*collector.GraphSnapshot),
		subscribers: make(map[subID]chan collector.StateMessage),
	}
}

func (m *Manager) Subscribe() <-chan collector.StateMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan collector.StateMessage, 16)
	m.nextID++
	m.subscribers[m.nextID] = ch

	if len(m.merged.Nodes) > 0 {
		ch <- collector.StateMessage{
			Type:     "snapshot",
			Snapshot: &m.merged,
		}
	}

	return ch
}

func (m *Manager) Current() collector.GraphSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.merged
}

func (m *Manager) HandleUpdate(name string, update collector.StateUpdate) {
	m.mu.Lock()

	if update.Snapshot != nil {
		m.snapshots[name] = update.Snapshot
	}

	composeSnaps := make(map[string]*collector.GraphSnapshot)
	dockerSnaps := make(map[string]*collector.GraphSnapshot)
	for k, v := range m.snapshots {
		if k == "docker" {
			dockerSnaps[k] = v
		} else {
			composeSnaps[k] = v
		}
	}

	m.merged = mergeSnapshots(composeSnaps, dockerSnaps)
	snapshot := m.merged
	subs := make([]chan collector.StateMessage, 0, len(m.subscribers))
	for _, ch := range m.subscribers {
		subs = append(subs, ch)
	}

	m.mu.Unlock()

	msg := collector.StateMessage{
		Type:     "snapshot",
		Snapshot: &snapshot,
	}
	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

func mergeSnapshots(composeSnaps, dockerSnaps map[string]*collector.GraphSnapshot) collector.GraphSnapshot {
	var result collector.GraphSnapshot

	type nodeKey struct {
		name     string
		nodeType string
	}
	composeNodes := make(map[nodeKey]collector.Node)
	var composeEdges []collector.Edge

	for _, snap := range composeSnaps {
		for _, n := range snap.Nodes {
			composeNodes[nodeKey{n.Name, n.Type}] = n
		}
		composeEdges = append(composeEdges, snap.Edges...)
	}

	dockerNodes := make(map[nodeKey]collector.Node)
	var dockerEdges []collector.Edge

	for _, snap := range dockerSnaps {
		for _, n := range snap.Nodes {
			dockerNodes[nodeKey{n.Name, n.Type}] = n
		}
		dockerEdges = append(dockerEdges, snap.Edges...)
	}

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
			result.Nodes = append(result.Nodes, merged)
		} else {
			result.Nodes = append(result.Nodes, dockerNode)
		}
	}

	for key, composeNode := range composeNodes {
		if !seen[key] {
			result.Nodes = append(result.Nodes, composeNode)
		}
	}

	edgeIDs := make(map[string]bool)
	for _, e := range dockerEdges {
		if !edgeIDs[e.ID] {
			edgeIDs[e.ID] = true
			result.Edges = append(result.Edges, e)
		}
	}
	for _, e := range composeEdges {
		if !edgeIDs[e.ID] {
			edgeIDs[e.ID] = true
			result.Edges = append(result.Edges, e)
		}
	}

	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	sort.Slice(result.Edges, func(i, j int) bool { return result.Edges[i].ID < result.Edges[j].ID })

	return result
}

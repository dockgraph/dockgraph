package collector

import "context"

type PortMapping struct {
	Host      int `json:"host"`
	Container int `json:"container"`
}

type Node struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Image     string            `json:"image,omitempty"`
	Status    string            `json:"status,omitempty"`
	Ports     []PortMapping     `json:"ports,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	NetworkID string            `json:"networkId,omitempty"`
	Driver    string            `json:"driver,omitempty"`
	Source    string            `json:"source,omitempty"`
}

type Edge struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	MountPath string `json:"mountPath,omitempty"`
}

type GraphSnapshot struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type DeltaUpdate struct {
	NodesAdded   []Node   `json:"nodesAdded,omitempty"`
	NodesRemoved []string `json:"nodesRemoved,omitempty"`
	NodesUpdated []Node   `json:"nodesUpdated,omitempty"`
	EdgesAdded   []Edge   `json:"edgesAdded,omitempty"`
	EdgesRemoved []string `json:"edgesRemoved,omitempty"`
}

type WireMessage struct {
	Type    string      `json:"type"`
	Version int         `json:"version"`
	Data    any `json:"data"`
}

func NewSnapshotMessage(s GraphSnapshot) WireMessage {
	return WireMessage{Type: "snapshot", Version: 1, Data: s}
}

func NewDeltaMessage(d DeltaUpdate) WireMessage {
	return WireMessage{Type: "delta", Version: 1, Data: d}
}

type StateMessage struct {
	Type     string
	Snapshot *GraphSnapshot
	Delta    *DeltaUpdate
}

type StateUpdate struct {
	Snapshot *GraphSnapshot
}

type Collector interface {
	Start(ctx context.Context) error
	Updates() <-chan StateUpdate
	Stop() error
}

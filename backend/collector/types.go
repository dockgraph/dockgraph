// Package collector provides data collectors that monitor Docker resources
// (containers, networks, volumes) and Docker Compose definitions, producing
// graph snapshots that represent the current infrastructure topology.
//
// Node and edge IDs follow a namespaced format to prevent collisions:
//
//	Nodes: "container:{name}", "network:{name}", "volume:{name}"
//	Edges: "e:dep:{source}:{target}", "e:net:{source}:{target}", "e:vol:{source}:{target}"
package collector

import (
	"context"
	"io"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
)

// PortMapping represents a host-to-container port binding.
type PortMapping struct {
	Host      int `json:"host"`
	Container int `json:"container"`
}

// Node represents a single element in the infrastructure graph.
// The Type field determines which optional fields are populated:
//
//	"container" — Image, Status, Ports, Labels, NetworkID
//	"network"   — Driver
//	"volume"    — Driver, Status
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

// Edge represents a directed relationship between two nodes.
// Edge types: "depends_on", "volume_mount", "secondary_network".
type Edge struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	MountPath string `json:"mountPath,omitempty"`
}

// GraphSnapshot is a complete point-in-time view of the infrastructure graph.
type GraphSnapshot struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// DeltaUpdate describes incremental changes to the graph since the last snapshot.
type DeltaUpdate struct {
	NodesAdded   []Node   `json:"nodesAdded,omitempty"`
	NodesRemoved []string `json:"nodesRemoved,omitempty"`
	NodesUpdated []Node   `json:"nodesUpdated,omitempty"`
	EdgesAdded   []Edge   `json:"edgesAdded,omitempty"`
	EdgesRemoved []string `json:"edgesRemoved,omitempty"`
}

// WireMessage is the envelope sent over the WebSocket connection.
// Type is either "snapshot" or "delta", and Data contains the corresponding payload.
type WireMessage struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	Data    any    `json:"data,omitempty"`
}

// NewSnapshotMessage wraps a full graph snapshot for WebSocket transmission.
func NewSnapshotMessage(s GraphSnapshot) WireMessage {
	return WireMessage{Type: "snapshot", Version: 1, Data: s}
}

// NewDeltaMessage wraps an incremental update for WebSocket transmission.
func NewDeltaMessage(d DeltaUpdate) WireMessage {
	return WireMessage{Type: "delta", Version: 1, Data: d}
}

// StateMessage is an internal message passed from the state manager to the
// WebSocket hub, carrying either a full snapshot or a delta update.
type StateMessage struct {
	Type     string
	Snapshot *GraphSnapshot
	Delta    *DeltaUpdate
	Stats    *StatsSnapshot
}

// StateUpdate is emitted by collectors whenever they detect a topology change.
type StateUpdate struct {
	Snapshot *GraphSnapshot
}

// DockerClient is the subset of the Docker API used by the collector package.
// Defined here (at the consumer) rather than depending on the full client.APIClient
// so the dependency surface is explicit and test stubs are minimal.
type DockerClient interface {
	ContainerList(ctx context.Context, options containertypes.ListOptions) ([]containertypes.Summary, error)
	NetworkList(ctx context.Context, options networktypes.ListOptions) ([]networktypes.Summary, error)
	VolumeList(ctx context.Context, options volumetypes.ListOptions) (volumetypes.ListResponse, error)
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (containertypes.StatsResponseReader, error)
	ContainerInspect(ctx context.Context, containerID string) (containertypes.InspectResponse, error)
	ContainerLogs(ctx context.Context, containerID string, options containertypes.LogsOptions) (io.ReadCloser, error)
	Close() error
}

// Collector defines the interface for infrastructure data sources.
// Implementations produce StateUpdate values on a channel that the
// state manager consumes and merges.
type Collector interface {
	Start(ctx context.Context) error
	Updates() <-chan StateUpdate
	Stop() error
}

package collector

import (
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	networktypes "github.com/docker/docker/api/types/network"
)

// --- Node builders ---

func TestBuildNodeFromContainer(t *testing.T) {
	node := buildContainerNode(
		"my-api",
		"python:3.12",
		"running",
		nil,
		[]PortMapping{{Host: 8000, Container: 8000}},
	)

	if node.ID != "container:my-api" {
		t.Errorf("expected ID container:my-api, got %s", node.ID)
	}
	if node.Type != "container" {
		t.Errorf("expected type container, got %s", node.Type)
	}
	if node.Name != "my-api" {
		t.Errorf("expected name my-api, got %s", node.Name)
	}
	if node.Status != "running" {
		t.Errorf("expected status running, got %s", node.Status)
	}
	if len(node.Ports) != 1 || node.Ports[0].Host != 8000 {
		t.Errorf("expected port 8000, got %v", node.Ports)
	}
	if node.Labels != nil {
		t.Error("expected nil labels — labels must not be forwarded to clients")
	}
}

func TestBuildNetworkNode(t *testing.T) {
	node := buildNetworkNode("backend-net", "bridge")

	if node.ID != "network:backend-net" {
		t.Errorf("expected ID network:backend-net, got %s", node.ID)
	}
	if node.Type != "network" {
		t.Errorf("expected type network, got %s", node.Type)
	}
	if node.Driver != "bridge" {
		t.Errorf("expected driver bridge, got %s", node.Driver)
	}
}

func TestBuildVolumeNode(t *testing.T) {
	node := buildVolumeNode("pg-data", "local")

	if node.ID != "volume:pg-data" {
		t.Errorf("expected ID volume:pg-data, got %s", node.ID)
	}
	if node.Type != "volume" {
		t.Errorf("expected type volume, got %s", node.Type)
	}
}

// --- Helpers ---

func TestSelfExclusionByLabel(t *testing.T) {
	containers := []containertypes.Summary{
		{
			Names: []string{"/app"},
			Image: "dockgraph:latest",
			State: "running",
		},
		{
			Names:  []string{"/self"},
			Image:  "myapp:1.0",
			Labels: map[string]string{SelfExcludeLabel: "true"},
			State:  "running",
		},
	}

	var included []string
	for _, c := range containers {
		if c.Labels[SelfExcludeLabel] == "true" {
			continue
		}
		included = append(included, strings.TrimPrefix(c.Names[0], "/"))
	}

	if len(included) != 1 || included[0] != "app" {
		t.Errorf("expected only 'app' to be included, got %v", included)
	}
}

func TestIsTopologyEvent(t *testing.T) {
	topology := []string{
		"start", "stop", "die", "kill", "create", "destroy", "rename",
		"pause", "unpause", "health_status", "connect", "disconnect",
	}
	for _, action := range topology {
		if !isTopologyEvent(events.Action(action)) {
			t.Errorf("expected %q to be a topology event", action)
		}
	}

	nonTopology := []string{"pull", "attach", "exec_start", "push", ""}
	for _, action := range nonTopology {
		if isTopologyEvent(events.Action(action)) {
			t.Errorf("expected %q to NOT be a topology event", action)
		}
	}
}

func TestContainerStatus(t *testing.T) {
	tests := []struct {
		state, statusStr, want string
	}{
		{"running", "Up 5 minutes", "running"},
		{"running", "Up 5 minutes (unhealthy)", "unhealthy"},
		{"exited", "Exited (1) 2 hours ago", "exited"},
		{"created", "", "created"},
	}
	for _, tc := range tests {
		got := containerStatus(tc.state, tc.statusStr)
		if got != tc.want {
			t.Errorf("containerStatus(%q, %q) = %q, want %q", tc.state, tc.statusStr, got, tc.want)
		}
	}
}

func TestExtractPorts(t *testing.T) {
	ports := extractPorts([]containertypes.Port{
		{PublicPort: 8080, PrivatePort: 80},
		{PublicPort: 0, PrivatePort: 3306}, // unexposed, should be filtered
		{PublicPort: 443, PrivatePort: 443},
	})

	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
	if ports[0].Host != 8080 || ports[0].Container != 80 {
		t.Errorf("first port: got %+v", ports[0])
	}
	if ports[1].Host != 443 || ports[1].Container != 443 {
		t.Errorf("second port: got %+v", ports[1])
	}
}

func TestExtractPortsEmpty(t *testing.T) {
	ports := extractPorts(nil)
	if ports != nil {
		t.Errorf("expected nil for empty input, got %v", ports)
	}
}

// --- Network resolution ---

func TestResolveNetworkNames(t *testing.T) {
	networks := []networktypes.Summary{
		{ID: "abc", Name: "my-app-net"},
		{ID: "def", Name: "bridge"}, // built-in, excluded
		{ID: "ghi", Name: "host"},   // built-in, excluded
		{ID: "jkl", Name: "none"},   // built-in, excluded
		{ID: "mno", Name: "backend"},
	}

	result := resolveNetworkNames(networks)

	if len(result) != 2 {
		t.Fatalf("expected 2 networks, got %d", len(result))
	}
	if result["abc"] != "my-app-net" {
		t.Errorf("expected my-app-net, got %s", result["abc"])
	}
	if result["mno"] != "backend" {
		t.Errorf("expected backend, got %s", result["mno"])
	}
}

func TestResolveNetworkNamesEmpty(t *testing.T) {
	result := resolveNetworkNames(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestResolveServiceNames(t *testing.T) {
	containers := []containertypes.Summary{
		{
			Names: []string{"/myapp-web-1"},
			Labels: map[string]string{
				"com.docker.compose.project": "myapp",
				"com.docker.compose.service": "web",
			},
		},
		{
			Names: []string{"/myapp-db-1"},
			Labels: map[string]string{
				"com.docker.compose.project": "myapp",
				"com.docker.compose.service": "db",
			},
		},
		{
			Names:  []string{"/standalone"},
			Labels: map[string]string{}, // no compose labels, should be skipped
		},
	}

	result := resolveServiceNames(containers)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[serviceKey{"myapp", "web"}] != "myapp-web-1" {
		t.Errorf("web: got %s", result[serviceKey{"myapp", "web"}])
	}
	if result[serviceKey{"myapp", "db"}] != "myapp-db-1" {
		t.Errorf("db: got %s", result[serviceKey{"myapp", "db"}])
	}
}

// --- Network classification ---

func TestClassifyContainerNetworks(t *testing.T) {
	container := containertypes.Summary{
		NetworkSettings: &containertypes.NetworkSettingsSummary{
			Networks: map[string]*networktypes.EndpointSettings{
				"backend": {NetworkID: "net-abc"},
				"shared":  {NetworkID: "net-def"},
			},
		},
	}
	idToName := map[string]string{
		"net-abc": "backend",
		"net-def": "shared",
	}

	primary, secondary := classifyContainerNetworks(container, idToName)

	if primary != "backend" {
		t.Errorf("expected primary=backend, got %s", primary)
	}
	if len(secondary) != 1 || secondary[0] != "shared" {
		t.Errorf("expected secondary=[shared], got %v", secondary)
	}
}

func TestClassifyContainerNetworksNilSettings(t *testing.T) {
	container := containertypes.Summary{NetworkSettings: nil}
	primary, secondary := classifyContainerNetworks(container, map[string]string{})

	if primary != "" {
		t.Errorf("expected empty primary, got %s", primary)
	}
	if secondary != nil {
		t.Errorf("expected nil secondary, got %v", secondary)
	}
}

func TestClassifyContainerNetworksUnknownID(t *testing.T) {
	container := containertypes.Summary{
		NetworkSettings: &containertypes.NetworkSettingsSummary{
			Networks: map[string]*networktypes.EndpointSettings{
				"bridge": {NetworkID: "unknown-id"},
			},
		},
	}

	primary, secondary := classifyContainerNetworks(container, map[string]string{})

	if primary != "" {
		t.Errorf("expected empty primary for unknown network, got %s", primary)
	}
	if secondary != nil {
		t.Errorf("expected nil secondary, got %v", secondary)
	}
}

// --- Edge builders ---

func TestBuildContainerEdges(t *testing.T) {
	container := containertypes.Summary{
		NetworkSettings: &containertypes.NetworkSettingsSummary{
			Networks: map[string]*networktypes.EndpointSettings{
				"primary": {NetworkID: "net-1"},
				"shared":  {NetworkID: "net-2"},
			},
		},
		Mounts: []containertypes.MountPoint{
			{Type: "volume", Name: "pg-data", Destination: "/var/lib/postgresql/data"},
			{Type: "bind", Name: "", Destination: "/app"}, // bind mount, should be skipped
		},
		Labels: map[string]string{
			"com.docker.compose.project":    "myapp",
			"com.docker.compose.depends_on": "redis:service_started:false,db:service_healthy:false",
		},
	}

	networkIDToName := map[string]string{
		"net-1": "primary",
		"net-2": "shared",
	}
	serviceNames := map[serviceKey]string{
		{project: "myapp", service: "redis"}: "myapp-redis-1",
		{project: "myapp", service: "db"}:    "myapp-db-1",
	}

	edges := buildContainerEdges("myapp-web-1", container, networkIDToName, serviceNames)

	// Count by type
	var netEdges, volEdges, depEdges []Edge
	for _, e := range edges {
		switch e.Type {
		case "secondary_network":
			netEdges = append(netEdges, e)
		case "volume_mount":
			volEdges = append(volEdges, e)
		case "depends_on":
			depEdges = append(depEdges, e)
		}
	}

	if len(netEdges) != 1 {
		t.Errorf("expected 1 secondary network edge, got %d", len(netEdges))
	}
	if len(volEdges) != 1 {
		t.Errorf("expected 1 volume mount edge, got %d", len(volEdges))
	}
	if volEdges[0].MountPath != "/var/lib/postgresql/data" {
		t.Errorf("expected mount path /var/lib/postgresql/data, got %s", volEdges[0].MountPath)
	}
	if len(depEdges) != 2 {
		t.Errorf("expected 2 depends_on edges, got %d", len(depEdges))
	}
}

func TestBuildContainerEdgesNoLabels(t *testing.T) {
	container := containertypes.Summary{
		NetworkSettings: &containertypes.NetworkSettingsSummary{
			Networks: map[string]*networktypes.EndpointSettings{
				"only": {NetworkID: "net-1"},
			},
		},
		Labels: map[string]string{},
	}

	edges := buildContainerEdges("test", container, map[string]string{"net-1": "only"}, nil)

	// Single network = primary, no secondary edges. No mounts, no depends_on.
	if len(edges) != 0 {
		t.Errorf("expected 0 edges for container with no secondary networks, mounts, or deps; got %d", len(edges))
	}
}

// --- Wire message constructors ---

func TestNewSnapshotMessage(t *testing.T) {
	snap := GraphSnapshot{
		Nodes: []Node{{ID: "container:test", Type: "container", Name: "test"}},
	}
	msg := NewSnapshotMessage(snap)

	if msg.Type != "snapshot" {
		t.Errorf("expected type snapshot, got %s", msg.Type)
	}
	if msg.Version != 1 {
		t.Errorf("expected version 1, got %d", msg.Version)
	}
}

func TestNewDeltaMessage(t *testing.T) {
	delta := DeltaUpdate{
		NodesRemoved: []string{"container:old"},
	}
	msg := NewDeltaMessage(delta)

	if msg.Type != "delta" {
		t.Errorf("expected type delta, got %s", msg.Type)
	}
	if msg.Version != 1 {
		t.Errorf("expected version 1, got %d", msg.Version)
	}
}

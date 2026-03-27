package collector

import (
	"context"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
)

// --- fetchResources ---

func TestFetchResources(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{{Names: []string{"/a"}}},
		networks:   []networktypes.Summary{{ID: "n1", Name: "net"}},
		volumes:    []*volumetypes.Volume{{Name: "vol1"}},
	}

	res, err := fetchResources(context.Background(), cli)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(res.containers))
	}
	if len(res.networks) != 1 {
		t.Errorf("expected 1 network, got %d", len(res.networks))
	}
	if len(res.volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(res.volumes))
	}
}

func TestFetchResourcesContainerError(t *testing.T) {
	_, err := fetchResources(context.Background(), errClient("containers"))
	if err == nil {
		t.Fatal("expected error for container list failure")
	}
}

func TestFetchResourcesNetworkError(t *testing.T) {
	_, err := fetchResources(context.Background(), errClient("networks"))
	if err == nil {
		t.Fatal("expected error for network list failure")
	}
}

func TestFetchResourcesVolumeError(t *testing.T) {
	_, err := fetchResources(context.Background(), errClient("volumes"))
	if err == nil {
		t.Fatal("expected error for volume list failure")
	}
}

// --- buildSnapshot ---

func TestBuildSnapshot(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{
				Names:  []string{"/web"},
				Image:  "nginx:latest",
				State:  "running",
				Status: "Up 5 minutes",
				NetworkSettings: &containertypes.NetworkSettingsSummary{
					Networks: map[string]*networktypes.EndpointSettings{
						"frontend": {NetworkID: "net-1"},
					},
				},
				Mounts: []containertypes.MountPoint{
					{Type: "volume", Name: "data", Destination: "/data"},
				},
			},
		},
		networks: []networktypes.Summary{
			{ID: "net-1", Name: "frontend"},
			{ID: "br0", Name: "bridge"}, // built-in, should be excluded
		},
		volumes: []*volumetypes.Volume{
			{Name: "data", Driver: "local"},
		},
	}

	dc := NewDockerCollector(cli, 0)
	snap, err := dc.buildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	containers := filterNodes(snap.Nodes, "container")
	networks := filterNodes(snap.Nodes, "network")
	volumes := filterNodes(snap.Nodes, "volume")

	if len(containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(containers))
	}
	if len(networks) != 1 {
		t.Errorf("expected 1 network (bridge excluded), got %d", len(networks))
	}
	if len(volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(volumes))
	}

	if containers[0].Name != "web" {
		t.Errorf("expected container name 'web', got %s", containers[0].Name)
	}
	if containers[0].Status != "running" {
		t.Errorf("expected status running, got %s", containers[0].Status)
	}
	if containers[0].NetworkID != "network:frontend" {
		t.Errorf("expected networkID 'network:frontend', got %s", containers[0].NetworkID)
	}
}

func TestBuildSnapshotSelfExclusion(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{
				Names:  []string{"/app"},
				Image:  "app:1",
				State:  "running",
				Labels: map[string]string{},
			},
			{
				Names:  []string{"/dockgraph"},
				Image:  "dockgraph:latest",
				State:  "running",
				Labels: map[string]string{SelfExcludeLabel: "true"},
			},
		},
	}

	dc := NewDockerCollector(cli, 0)
	snap, err := dc.buildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	containers := filterNodes(snap.Nodes, "container")
	if len(containers) != 1 {
		t.Fatalf("expected 1 container (self excluded), got %d", len(containers))
	}
	if containers[0].Name != "app" {
		t.Errorf("expected 'app', got %s", containers[0].Name)
	}
}

func TestBuildSnapshotEmptyNames(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/valid"}, Image: "img", State: "running"},
			{Names: []string{}, Image: "broken", State: "running"}, // no names, should be skipped
		},
	}

	dc := NewDockerCollector(cli, 0)
	snap, err := dc.buildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	containers := filterNodes(snap.Nodes, "container")
	if len(containers) != 1 {
		t.Errorf("expected 1 container (nameless skipped), got %d", len(containers))
	}
}

func TestBuildSnapshotEmpty(t *testing.T) {
	cli := &stubDockerClient{}
	dc := NewDockerCollector(cli, 0)
	snap, err := dc.buildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(snap.Nodes))
	}
	if len(snap.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(snap.Edges))
	}
}

func TestBuildSnapshotDeterministicOrder(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/z-svc"}, Image: "z", State: "running"},
			{Names: []string{"/a-svc"}, Image: "a", State: "running"},
		},
	}
	dc := NewDockerCollector(cli, 0)
	snap, err := dc.buildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(snap.Nodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(snap.Nodes))
	}
	if snap.Nodes[0].ID >= snap.Nodes[1].ID {
		t.Errorf("nodes not sorted: %s >= %s", snap.Nodes[0].ID, snap.Nodes[1].ID)
	}
}

func TestBuildSnapshotDependsOnEdges(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{
				Names: []string{"/myapp-web-1"},
				Image: "web:1",
				State: "running",
				Labels: map[string]string{
					"com.docker.compose.project":    "myapp",
					"com.docker.compose.service":    "web",
					"com.docker.compose.depends_on": "db:service_healthy:false",
				},
			},
			{
				Names: []string{"/myapp-db-1"},
				Image: "postgres:16",
				State: "running",
				Labels: map[string]string{
					"com.docker.compose.project": "myapp",
					"com.docker.compose.service": "db",
				},
			},
		},
	}
	dc := NewDockerCollector(cli, 0)
	snap, err := dc.buildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	depEdges := filterEdges(snap.Edges, "depends_on")
	if len(depEdges) != 1 {
		t.Fatalf("expected 1 depends_on edge, got %d", len(depEdges))
	}
	if depEdges[0].Source != "container:myapp-web-1" {
		t.Errorf("expected source container:myapp-web-1, got %s", depEdges[0].Source)
	}
	if depEdges[0].Target != "container:myapp-db-1" {
		t.Errorf("expected target container:myapp-db-1, got %s", depEdges[0].Target)
	}
}

// --- resolveServiceNames edge cases ---

func TestResolveServiceNamesEmptyNames(t *testing.T) {
	containers := []containertypes.Summary{
		{
			Names: []string{},
			Labels: map[string]string{
				"com.docker.compose.project": "proj",
				"com.docker.compose.service": "svc",
			},
		},
	}
	result := resolveServiceNames(containers)
	if len(result) != 0 {
		t.Errorf("expected 0 entries for nameless container, got %d", len(result))
	}
}

func TestResolveServiceNamesPartialLabels(t *testing.T) {
	containers := []containertypes.Summary{
		{
			Names:  []string{"/c1"},
			Labels: map[string]string{"com.docker.compose.project": "proj"}, // no service label
		},
		{
			Names:  []string{"/c2"},
			Labels: map[string]string{"com.docker.compose.service": "svc"}, // no project label
		},
	}
	result := resolveServiceNames(containers)
	if len(result) != 0 {
		t.Errorf("expected 0 entries for incomplete labels, got %d", len(result))
	}
}

// --- buildContainerEdges edge cases ---

func TestBuildContainerEdgesEmptyDependsOnEntry(t *testing.T) {
	container := containertypes.Summary{
		Labels: map[string]string{
			"com.docker.compose.project":    "proj",
			"com.docker.compose.depends_on": ",,valid_svc:cond:restart",
		},
	}
	edges := buildContainerEdges("test", container, map[string]string{}, map[serviceKey]string{
		{project: "proj", service: "valid_svc"}: "proj-valid_svc-1",
	})

	depEdges := filterEdges(edges, "depends_on")
	if len(depEdges) != 1 {
		t.Errorf("expected 1 depends_on edge (empty entries skipped), got %d", len(depEdges))
	}
}

func TestBuildContainerEdgesUnresolvedService(t *testing.T) {
	container := containertypes.Summary{
		Labels: map[string]string{
			"com.docker.compose.project":    "proj",
			"com.docker.compose.depends_on": "nonexistent:service_started:false",
		},
	}
	edges := buildContainerEdges("test", container, map[string]string{}, map[serviceKey]string{})

	if len(edges) != 0 {
		t.Errorf("expected 0 edges for unresolved service, got %d", len(edges))
	}
}

func TestBuildContainerEdgesNoDepsLabel(t *testing.T) {
	container := containertypes.Summary{
		Labels: map[string]string{
			"com.docker.compose.project": "proj",
		},
	}
	edges := buildContainerEdges("test", container, map[string]string{}, map[serviceKey]string{})
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

package collector

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	composetypes "github.com/compose-spec/compose-go/v2/types"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestParseComposeFile(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	containers := filterNodes(snap.Nodes, "container")
	networks := filterNodes(snap.Nodes, "network")
	volumes := filterNodes(snap.Nodes, "volume")

	if len(containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(containers))
	}
	if len(networks) != 2 {
		t.Errorf("expected 2 networks, got %d", len(networks))
	}
	if len(volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(volumes))
	}

	for _, n := range snap.Nodes {
		if n.Source != "simple.yaml" {
			t.Errorf("expected source simple.yaml, got %s for node %s", n.Source, n.ID)
		}
	}
}

func TestParseDependsOnEdges(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	dependsOn := filterEdges(snap.Edges, "depends_on")
	if len(dependsOn) != 2 {
		t.Errorf("expected 2 depends_on edges, got %d", len(dependsOn))
	}
}

// The side panel renders ComposeConfig.DependsOn as clickable links that
// navigate to "container:<value>". Those values must equal the full container
// node names (project-prefixed, replica-suffixed) so the link resolves to an
// actual node — bare service names like "postgres" match nothing.
func TestComposeConfigDependsOnUsesFullContainerNames(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	nodeIDs := make(map[string]bool)
	for _, n := range snap.Nodes {
		nodeIDs[n.ID] = true
	}

	var checked int
	for _, n := range snap.Nodes {
		if n.Compose == nil {
			continue
		}
		for _, dep := range n.Compose.DependsOn {
			checked++
			if !nodeIDs["container:"+dep] {
				t.Errorf("node %s depends_on %q resolves to missing node container:%s", n.ID, dep, dep)
			}
		}
	}
	if checked == 0 {
		t.Fatal("expected at least one depends_on entry to verify")
	}
}

// The side panel renders ComposeConfig.Volumes as mount rows, with named
// volumes linking to "volume:<name>". The Name field must hold the full,
// project-prefixed volume node name so the link resolves to an actual node.
func TestComposeConfigVolumesUseFullVolumeNodeNames(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	nodeIDs := make(map[string]bool)
	for _, n := range snap.Nodes {
		nodeIDs[n.ID] = true
	}

	var checked int
	for _, n := range snap.Nodes {
		if n.Compose == nil {
			continue
		}
		for _, m := range n.Compose.Volumes {
			if m.Type != mountTypeVolume {
				continue
			}
			checked++
			if m.Name == "" {
				t.Errorf("node %s named volume mount to %q has empty Name", n.ID, m.Destination)
				continue
			}
			if !nodeIDs["volume:"+m.Name] {
				t.Errorf("node %s volume mount %q resolves to missing node volume:%s", n.ID, m.Name, m.Name)
			}
		}
	}
	if checked == 0 {
		t.Fatal("expected at least one named volume mount to verify")
	}
}

// Exercises the full mount mapping that the parse-based tests can't reach with
// the shared fixture: a read-only named volume and a bind mount.
func TestBuildComposeConfigVolumeMounts(t *testing.T) {
	naming := composeNaming{project: "proj"}
	svc := composetypes.ServiceConfig{
		Name: "api",
		Volumes: []composetypes.ServiceVolumeConfig{
			{Type: mountTypeVolume, Source: "db_data", Target: "/data", ReadOnly: true},
			{Type: mountTypeBind, Source: "/etc/app", Target: "/config"},
		},
	}

	cfg := buildComposeConfig(svc, naming)
	if len(cfg.Volumes) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(cfg.Volumes))
	}

	want := []ComposeMount{
		{Type: mountTypeVolume, Source: "db_data", Destination: "/data", RW: false, Name: "proj_db_data"},
		{Type: mountTypeBind, Source: "/etc/app", Destination: "/config", RW: true, Name: ""},
	}
	for i, w := range want {
		if cfg.Volumes[i] != w {
			t.Errorf("mount[%d] = %+v, want %+v", i, cfg.Volumes[i], w)
		}
	}
}

func TestBuildComposeConfigMasksSecretsAndCapturesLabels(t *testing.T) {
	secret := "s3cr3t"
	level := "info"
	svc := composetypes.ServiceConfig{
		Name: "api",
		Environment: composetypes.MappingWithEquals{
			"DB_PASSWORD": &secret,
			"LOG_LEVEL":   &level,
		},
		Labels: composetypes.Labels{
			"com.example.team": "platform",
		},
	}

	cfg := buildComposeConfig(svc, composeNaming{project: "proj"})

	if got := cfg.Environment["DB_PASSWORD"]; got != "********" {
		t.Errorf("expected masked password, got %q", got)
	}
	if got := cfg.Environment["LOG_LEVEL"]; got != "info" {
		t.Errorf("expected LOG_LEVEL=info, got %q", got)
	}
	if got := cfg.Labels["com.example.team"]; got != "platform" {
		t.Errorf("expected label captured, got %q", got)
	}
}

func TestParseComposePortsProtocol(t *testing.T) {
	ports := []composetypes.ServicePortConfig{
		{Published: "8070", Target: 80}, // unset → defaults to tcp
		{Published: "5353", Target: 53, Protocol: "udp"},
	}

	got := parseComposePorts(ports)
	if len(got) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(got))
	}
	if got[0].Protocol != "tcp" {
		t.Errorf("expected default protocol tcp, got %q", got[0].Protocol)
	}
	if got[1].Protocol != "udp" {
		t.Errorf("expected protocol udp, got %q", got[1].Protocol)
	}
}

func TestParseVolumeMountEdges(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	mounts := filterEdges(snap.Edges, "volume_mount")
	if len(mounts) != 1 {
		t.Errorf("expected 1 volume_mount edge, got %d", len(mounts))
	}
	if len(mounts) > 0 && mounts[0].MountPath != "/var/lib/postgresql/data" {
		t.Errorf("expected mount path /var/lib/postgresql/data, got %s", mounts[0].MountPath)
	}
}

func TestComposeVolumeStatus(t *testing.T) {
	snap, err := parseComposeFile(context.Background(), filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	volumes := filterNodes(snap.Nodes, "volume")
	if len(volumes) == 0 {
		t.Fatal("expected at least one volume")
	}
	for _, v := range volumes {
		if v.Status != "not_running" {
			t.Errorf("expected compose volume %s to have status not_running, got %s", v.Name, v.Status)
		}
	}
}

func TestParseComposePortsSingle(t *testing.T) {
	ports := parseComposePorts([]composetypes.ServicePortConfig{
		{Published: "8080", Target: 80},
	})
	if len(ports) != 1 || ports[0].Host != 8080 || ports[0].Container != 80 {
		t.Errorf("expected [{8080 80}], got %+v", ports)
	}
}

func TestParseComposePortsRange(t *testing.T) {
	ports := parseComposePorts([]composetypes.ServicePortConfig{
		{Published: "9000-9002", Target: 9000},
	})
	if len(ports) != 3 {
		t.Fatalf("expected 3 mappings, got %d", len(ports))
	}
	for i, want := range []int{9000, 9001, 9002} {
		if ports[i].Host != want || ports[i].Container != want {
			t.Errorf("mapping[%d] = %+v, want {%d %d}", i, ports[i], want, want)
		}
	}
}

func TestParseComposePortsUnpublishedSkipped(t *testing.T) {
	ports := parseComposePorts([]composetypes.ServicePortConfig{
		{Published: "", Target: 3000},
	})
	if len(ports) != 0 {
		t.Errorf("expected no mappings for unpublished port, got %+v", ports)
	}
}

func TestParseComposePortsInvalidSkipped(t *testing.T) {
	ports := parseComposePorts([]composetypes.ServicePortConfig{
		{Published: "notanumber", Target: 80},
	})
	if len(ports) != 0 {
		t.Errorf("expected no mappings for unparseable port, got %+v", ports)
	}
}

func filterNodes(nodes []Node, nodeType string) []Node {
	var filtered []Node
	for _, n := range nodes {
		if n.Type == nodeType {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

func filterEdges(edges []Edge, edgeType string) []Edge {
	var filtered []Edge
	for _, e := range edges {
		if e.Type == edgeType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

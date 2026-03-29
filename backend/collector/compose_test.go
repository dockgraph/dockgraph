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

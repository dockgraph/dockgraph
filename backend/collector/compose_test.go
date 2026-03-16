package collector

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestParseComposeFile(t *testing.T) {
	snap, err := parseComposeFile(filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
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
	snap, err := parseComposeFile(filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	dependsOn := filterEdges(snap.Edges, "depends_on")
	if len(dependsOn) != 2 {
		t.Errorf("expected 2 depends_on edges, got %d", len(dependsOn))
	}
}

func TestParseVolumeMountEdges(t *testing.T) {
	snap, err := parseComposeFile(filepath.Join(testdataDir(), "simple.yaml"), "simple.yaml")
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

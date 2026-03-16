package collector

import (
	"testing"
)

func TestBuildNodeFromContainer(t *testing.T) {
	node := buildContainerNode(
		"abc123",
		"my-api",
		"python:3.12",
		"running",
		map[string]string{
			"com.docker.compose.service": "api",
			"com.docker.compose.project": "myapp",
		},
		[]PortMapping{{Host: 8000, Container: 8000}},
		"network:backend",
	)

	if node.ID != "container:abc123" {
		t.Errorf("expected ID container:abc123, got %s", node.ID)
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
	if node.NetworkID != "network:backend" {
		t.Errorf("expected networkId network:backend, got %s", node.NetworkID)
	}
	if len(node.Ports) != 1 || node.Ports[0].Host != 8000 {
		t.Errorf("expected port 8000, got %v", node.Ports)
	}
}

func TestBuildNetworkNode(t *testing.T) {
	node := buildNetworkNode("net123", "backend-net", "bridge")

	if node.ID != "network:net123" {
		t.Errorf("expected ID network:net123, got %s", node.ID)
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

func TestSelfExclusion(t *testing.T) {
	if !isSelfContainer("docker-flow:latest") {
		t.Error("should detect docker-flow image")
	}
	if !isSelfContainer("ghcr.io/user/docker-flow:v1.0") {
		t.Error("should detect docker-flow in registry path")
	}
	if isSelfContainer("postgres:16") {
		t.Error("should not exclude postgres")
	}
}

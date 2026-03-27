package collector

import (
	"context"
	"fmt"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
)

func TestDetectComposePaths(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{
				Names:  []string{"/dockgraph"},
				Labels: map[string]string{SelfExcludeLabel: "true"},
				Mounts: []containertypes.MountPoint{
					{Type: "bind", Destination: "/var/run/docker.sock"},
					{Type: "bind", Destination: "/compose/stacks"},
					{Type: "volume", Name: "data", Destination: "/data"},
				},
			},
		},
	}

	paths, err := DetectComposePaths(context.Background(), cli)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path (sock and volume excluded), got %d: %v", len(paths), paths)
	}
	if paths[0] != "/compose/stacks" {
		t.Errorf("expected /compose/stacks, got %s", paths[0])
	}
}

func TestDetectComposePathsNoSelfContainer(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{}, // no dockgraph container found
	}

	paths, err := DetectComposePaths(context.Background(), cli)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil paths when no self container found, got %v", paths)
	}
}

func TestDetectComposePathsDockerSockVariants(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{
				Names:  []string{"/dg"},
				Labels: map[string]string{SelfExcludeLabel: "true"},
				Mounts: []containertypes.MountPoint{
					{Type: "bind", Destination: "/var/run/docker.sock"},
					{Type: "bind", Destination: "/run/docker.sock"},
					{Type: "bind", Destination: "/mnt/wsl/docker.sock"},
				},
			},
		},
	}

	paths, err := DetectComposePaths(context.Background(), cli)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths (all docker.sock variants), got %d: %v", len(paths), paths)
	}
}

func TestDetectComposePathsAPIError(t *testing.T) {
	cli := &stubDockerClient{containerErr: fmt.Errorf("daemon unavailable")}

	_, err := DetectComposePaths(context.Background(), cli)
	if err == nil {
		t.Fatal("expected error when Docker API fails")
	}
}

func TestDetectComposePathsMultipleMounts(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{
				Names:  []string{"/dg"},
				Labels: map[string]string{SelfExcludeLabel: "true"},
				Mounts: []containertypes.MountPoint{
					{Type: "bind", Destination: "/var/run/docker.sock"},
					{Type: "bind", Destination: "/compose/a"},
					{Type: "bind", Destination: "/compose/b"},
				},
			},
		},
	}

	paths, err := DetectComposePaths(context.Background(), cli)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}

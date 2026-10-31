package collector

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
)

// withOwnID makes ownContainerID report id for the rest of the test.
func withOwnID(t *testing.T, id string) {
	t.Helper()
	prev := ownContainerID
	ownContainerID = func() string { return id }
	t.Cleanup(func() { ownContainerID = prev })
}

func TestDetectComposePathsPicksOwnContainer(t *testing.T) {
	own := strings.Repeat("a", 64)
	withOwnID(t, own)
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			// Newer and labelled, but not DockGraph: listed first, as Docker
			// lists the newest first.
			{
				ID: strings.Repeat("b", 64), Names: []string{"/squatter"}, Labels: map[string]string{SelfExcludeLabel: "true"},
				Mounts: []containertypes.MountPoint{{Type: "bind", Destination: "/sys/fs"}},
			},
			{
				ID: own, Names: []string{"/dockgraph"}, Labels: map[string]string{SelfExcludeLabel: "true"},
				Mounts: []containertypes.MountPoint{{Type: "bind", Destination: "/stacks"}},
			},
		},
	}
	paths, err := DetectComposePaths(context.Background(), cli)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "/stacks" {
		t.Errorf("paths = %v, want [/stacks] from the own container", paths)
	}

	// A short ID (the default hostname) works as a prefix.
	withOwnID(t, own[:12])
	if paths, _ := DetectComposePaths(context.Background(), cli); len(paths) != 1 || paths[0] != "/stacks" {
		t.Errorf("short ID: paths = %v, want [/stacks]", paths)
	}
}

func TestDetectComposePathsOwnContainerNotLabelled(t *testing.T) {
	withOwnID(t, strings.Repeat("a", 64))
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{
				ID: strings.Repeat("b", 64), Labels: map[string]string{SelfExcludeLabel: "true"},
				Mounts: []containertypes.MountPoint{{Type: "bind", Destination: "/sys/fs"}},
			},
		},
	}
	if paths, _ := DetectComposePaths(context.Background(), cli); paths != nil {
		t.Errorf("paths = %v, want none: another container's mounts must not be used", paths)
	}
}

func TestDetectComposePathsUnknownIDSeveralLabelled(t *testing.T) {
	withOwnID(t, "")
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{ID: "1", Labels: map[string]string{SelfExcludeLabel: "true"}, Mounts: []containertypes.MountPoint{{Type: "bind", Destination: "/a"}}},
			{ID: "2", Labels: map[string]string{SelfExcludeLabel: "true"}, Mounts: []containertypes.MountPoint{{Type: "bind", Destination: "/b"}}},
		},
	}
	if paths, _ := DetectComposePaths(context.Background(), cli); paths != nil {
		t.Errorf("paths = %v, want none when the own container can't be told apart", paths)
	}
}

func TestContainerIDFromMountinfo(t *testing.T) {
	id := strings.Repeat("0123456789abcdef", 4)
	docker := "1519 1508 0:30 /docker/containers/" + id + "/hostname /etc/hostname rw,relatime - ext4 /dev/sda1 rw\n"
	podman := "740 731 0:44 /containers/storage/overlay-containers/" + id + "/userdata/hostname /etc/hostname rw - tmpfs tmpfs rw\n"
	host := "22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw\n"
	for name, info := range map[string]string{"docker": host + docker, "podman": podman} {
		if got := containerIDFromMountinfo(bufio.NewScanner(strings.NewReader(info))); got != id {
			t.Errorf("%s: got %q, want %q", name, got, id)
		}
	}
	if got := containerIDFromMountinfo(bufio.NewScanner(strings.NewReader(host))); got != "" {
		t.Errorf("host mountinfo: got %q, want none", got)
	}
}

func TestDetectComposePaths(t *testing.T) {
	withOwnID(t, "")
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
	withOwnID(t, "")
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
	withOwnID(t, "")
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

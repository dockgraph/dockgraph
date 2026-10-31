package collector

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// DetectComposePaths inspects the dockgraph container's own bind mounts
// to discover directories and files the user has mounted for compose scanning.
// The Docker socket mount is excluded automatically. Uses All:true because
// at startup the container may not yet report as "running".
//
// Any container can carry the self-exclusion label, so the label alone does
// not say which one is DockGraph: the container is picked by its own ID (see
// ownContainerID). Only when that ID is unknown is a single labelled
// container used; with several, none is, since the paths to scan would
// depend on which other containers happen to exist.
func DetectComposePaths(ctx context.Context, cli DockerClient) ([]string, error) {
	containers, err := cli.ContainerList(ctx, containertypes.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", SelfExcludeLabel+"=true")),
	})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	self, ok := pickSelf(containers, ownContainerID())
	if !ok {
		return nil, nil
	}

	var paths []string
	for _, m := range self.Mounts {
		if m.Type != mountTypeBind {
			continue
		}
		if strings.HasSuffix(m.Destination, "docker.sock") {
			continue
		}
		paths = append(paths, m.Destination)
	}
	return paths, nil
}

// pickSelf returns DockGraph's own container among the labelled ones: the
// one whose ID starts with ownID or, when ownID is empty, the only one.
func pickSelf(labelled []containertypes.Summary, ownID string) (containertypes.Summary, bool) {
	if ownID != "" {
		for _, c := range labelled {
			if strings.HasPrefix(c.ID, ownID) {
				return c, true
			}
		}
		if len(labelled) > 0 {
			log.Printf("compose auto-detect: this container (%.12s) has no %s=true label; set the label or DG_COMPOSE_PATH", ownID, SelfExcludeLabel)
		}
		return containertypes.Summary{}, false
	}
	switch len(labelled) {
	case 0:
		return containertypes.Summary{}, false
	case 1:
		return labelled[0], true
	default:
		log.Printf("compose auto-detect: %d containers carry %s=true and this container's ID is unknown; set DG_COMPOSE_PATH", len(labelled), SelfExcludeLabel)
		return containertypes.Summary{}, false
	}
}

// ownContainerID returns the ID, or ID prefix, of the container DockGraph
// runs in, or "" when it can't tell. A variable so tests can replace it.
var ownContainerID = func() string {
	if f, err := os.Open("/proc/self/mountinfo"); err == nil {
		defer f.Close()
		if id := containerIDFromMountinfo(bufio.NewScanner(f)); id != "" {
			return id
		}
	}
	// Docker names a container's host after its short ID unless told otherwise.
	if h, err := os.Hostname(); err == nil && shortContainerID.MatchString(h) {
		return h
	}
	return ""
}

var (
	// mountinfoContainerID finds the container directory the runtime
	// bind-mounts /etc/hostname, /etc/hosts and /etc/resolv.conf from
	// (.../docker/containers/<id>/hostname, or Podman's
	// .../overlay-containers/<id>/userdata/hostname).
	mountinfoContainerID = regexp.MustCompile(`containers/([0-9a-f]{64})/`)
	shortContainerID     = regexp.MustCompile(`^[0-9a-f]{12}$`)
)

// containerIDFromMountinfo returns the first container ID named in a
// /proc/self/mountinfo listing.
func containerIDFromMountinfo(lines *bufio.Scanner) string {
	for lines.Scan() {
		if m := mountinfoContainerID.FindStringSubmatch(lines.Text()); m != nil {
			return m[1]
		}
	}
	return ""
}

package collector

import (
	"context"
	"fmt"
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// DetectComposePaths inspects the dockgraph container's own bind mounts
// to discover directories and files the user has mounted for compose scanning.
// The Docker socket mount is excluded automatically. Uses All:true because
// at startup the container may not yet report as "running".
func DetectComposePaths(ctx context.Context, cli client.APIClient) ([]string, error) {
	containers, err := cli.ContainerList(ctx, containertypes.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", SelfExcludeLabel+"=true")),
	})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	if len(containers) == 0 {
		return nil, nil
	}

	var paths []string
	for _, m := range containers[0].Mounts {
		if m.Type != "bind" {
			continue
		}
		if strings.HasSuffix(m.Destination, "docker.sock") {
			continue
		}
		paths = append(paths, m.Destination)
	}
	return paths, nil
}

package collector

import (
	"context"
	"sort"
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// dockerResources holds the raw API responses from Docker,
// grouping them for downstream processing.
type dockerResources struct {
	containers []containertypes.Summary
	networks   []networktypes.Summary
	volumes    []*volumetypes.Volume
}

// fetchResources queries the Docker daemon for all containers, networks, and volumes.
func fetchResources(ctx context.Context, cli client.APIClient) (dockerResources, error) {
	containers, err := cli.ContainerList(ctx, containertypes.ListOptions{All: true})
	if err != nil {
		return dockerResources{}, err
	}

	networks, err := cli.NetworkList(ctx, networktypes.ListOptions{})
	if err != nil {
		return dockerResources{}, err
	}

	volResp, err := cli.VolumeList(ctx, volumetypes.ListOptions{})
	if err != nil {
		return dockerResources{}, err
	}

	return dockerResources{
		containers: containers,
		networks:   networks,
		volumes:    volResp.Volumes,
	}, nil
}

// resolveNetworkNames maps Docker's internal network hex IDs to human-readable names,
// skipping the built-in bridge/host/none networks that aren't part of custom topologies.
func resolveNetworkNames(networks []networktypes.Summary) map[string]string {
	idToName := make(map[string]string)
	for _, n := range networks {
		if n.Name == "bridge" || n.Name == "host" || n.Name == "none" {
			continue
		}
		idToName[n.ID] = n.Name
	}
	return idToName
}

// serviceKey identifies a Docker Compose service within a specific project.
type serviceKey struct {
	project string
	service string
}

// resolveServiceNames builds a lookup from compose project+service to the actual
// container name, used to resolve depends_on edges.
func resolveServiceNames(containers []containertypes.Summary) map[serviceKey]string {
	m := make(map[serviceKey]string)
	for _, c := range containers {
		project := c.Labels["com.docker.compose.project"]
		service := c.Labels["com.docker.compose.service"]
		if project != "" && service != "" {
			m[serviceKey{project, service}] = strings.TrimPrefix(c.Names[0], "/")
		}
	}
	return m
}

// buildContainerEdges creates secondary-network, volume-mount, and depends_on edges
// for a single container.
func buildContainerEdges(name string, c containertypes.Summary, networkIDToName map[string]string, serviceNames map[serviceKey]string) []Edge {
	var edges []Edge

	// Secondary network connections
	_, secondaryNets := classifyContainerNetworks(c, networkIDToName)
	for _, netName := range secondaryNets {
		edges = append(edges, Edge{
			ID:     "e:net:" + name + ":" + netName,
			Type:   "secondary_network",
			Source: "container:" + name,
			Target: "network:" + netName,
		})
	}

	// Volume mounts
	for _, m := range c.Mounts {
		if m.Type == "volume" {
			edges = append(edges, Edge{
				ID:        "e:vol:" + m.Name + ":" + name,
				Type:      "volume_mount",
				Source:    "volume:" + m.Name,
				Target:    "container:" + name,
				MountPath: m.Destination,
			})
		}
	}

	// Depends-on edges derived from compose labels.
	// The label format is comma-separated entries of "service:condition:restart",
	// e.g. "db:service_healthy:false,redis:service_started:false".
	project := c.Labels["com.docker.compose.project"]
	depsLabel := c.Labels["com.docker.compose.depends_on"]
	if project != "" && depsLabel != "" {
		for _, entry := range strings.Split(depsLabel, ",") {
			parts := strings.SplitN(entry, ":", 3)
			if len(parts) == 0 || parts[0] == "" {
				continue
			}
			depName, ok := serviceNames[serviceKey{project, parts[0]}]
			if !ok {
				continue
			}
			edges = append(edges, Edge{
				ID:     "e:dep:" + name + ":" + depName,
				Type:   "depends_on",
				Source: "container:" + name,
				Target: "container:" + depName,
			})
		}
	}

	return edges
}

// classifyContainerNetworks resolves a container's Docker network IDs to names
// and splits them into primary and secondary.
func classifyContainerNetworks(c containertypes.Summary, networkIDToName map[string]string) (primary string, secondary []string) {
	var tracked []string
	if c.NetworkSettings != nil {
		for _, ns := range c.NetworkSettings.Networks {
			if name, ok := networkIDToName[ns.NetworkID]; ok {
				tracked = append(tracked, name)
			}
		}
	}
	return classifyNetworks(tracked)
}

// buildSnapshot queries the Docker daemon and assembles a complete graph of
// containers, networks, volumes, and their relationships.
func (d *DockerCollector) buildSnapshot(ctx context.Context) (GraphSnapshot, error) {
	res, err := fetchResources(ctx, d.client)
	if err != nil {
		return GraphSnapshot{}, err
	}

	var snap GraphSnapshot

	networkIDToName := resolveNetworkNames(res.networks)
	for _, n := range res.networks {
		if networkIDToName[n.ID] != "" {
			snap.Nodes = append(snap.Nodes, buildNetworkNode(n.Name, n.Driver))
		}
	}

	serviceNames := resolveServiceNames(res.containers)

	for _, c := range res.containers {
		if c.Labels[SelfExcludeLabel] == "true" {
			continue
		}

		name := strings.TrimPrefix(c.Names[0], "/")
		status := containerStatus(c.State, c.Status)
		ports := extractPorts(c.Ports)

		primary, _ := classifyContainerNetworks(c, networkIDToName)
		node := buildContainerNode(name, c.Image, status, nil, ports)
		if primary != "" {
			node.NetworkID = "network:" + primary
		}
		snap.Nodes = append(snap.Nodes, node)

		snap.Edges = append(snap.Edges, buildContainerEdges(name, c, networkIDToName, serviceNames)...)
	}

	for _, v := range res.volumes {
		snap.Nodes = append(snap.Nodes, buildVolumeNode(v.Name, v.Driver))
	}

	// Deterministic ordering prevents false-positive diffs in the state manager's merge.
	sort.Slice(snap.Nodes, func(i, j int) bool { return snap.Nodes[i].ID < snap.Nodes[j].ID })
	sort.Slice(snap.Edges, func(i, j int) bool { return snap.Edges[i].ID < snap.Edges[j].ID })

	return snap, nil
}

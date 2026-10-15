package collector

import (
	"context"
	"sort"
	"strings"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
	"golang.org/x/sync/errgroup"
)

// dockerResources holds the raw API responses from Docker,
// grouping them for downstream processing.
type dockerResources struct {
	containers []containertypes.Summary
	networks   []networktypes.Summary
	volumes    []*volumetypes.Volume
}

// fetchResources queries the Docker daemon for all containers, networks, and
// volumes concurrently. If any call fails, the context is cancelled and the
// first error is returned.
func fetchResources(ctx context.Context, cli DockerClient) (dockerResources, error) {
	var res dockerResources
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		res.containers, err = cli.ContainerList(ctx, containertypes.ListOptions{All: true})
		return err
	})
	g.Go(func() error {
		var err error
		res.networks, err = cli.NetworkList(ctx, networktypes.ListOptions{})
		return err
	})
	g.Go(func() error {
		volResp, err := cli.VolumeList(ctx, volumetypes.ListOptions{})
		if err != nil {
			return err
		}
		res.volumes = volResp.Volumes
		return nil
	})

	if err := g.Wait(); err != nil {
		return dockerResources{}, err
	}
	return res, nil
}

// resolveNetworkNames maps Docker's internal network hex IDs to human-readable names,
// skipping the built-in bridge/host/none networks and the swarm routing-mesh
// networks (ingress, docker_gwbridge) that aren't part of custom topologies.
func resolveNetworkNames(networks []networktypes.Summary) map[string]string {
	idToName := make(map[string]string)
	for _, n := range networks {
		if isBuiltinNetwork(n) {
			continue
		}
		idToName[n.ID] = n.Name
	}
	return idToName
}

// isBuiltinNetwork reports whether a network is Docker plumbing rather than
// part of a user-defined topology. The Ingress flag catches a custom-named
// routing-mesh network as well as the default "ingress".
func isBuiltinNetwork(n networktypes.Summary) bool {
	switch n.Name {
	case networkBridge, "host", "none", networkIngress, networkGwBridge:
		return true
	}
	return n.Ingress
}

// networkProjectMap maps each network name to the project that owns it, taken
// from the Docker Compose project label or the swarm stack namespace. Used to home a container in its
// own project's network rather than a shared or external one it merely joins.
func networkProjectMap(networks []networktypes.Summary) map[string]string {
	projects := make(map[string]string)
	for _, n := range networks {
		if p := ProjectOf(n.Labels); p != "" {
			projects[n.Name] = p
		}
	}
	return projects
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
		if len(c.Names) == 0 {
			continue
		}
		project := ProjectOf(c.Labels)
		service := c.Labels["com.docker.compose.service"]
		if project != "" && service != "" {
			m[serviceKey{project, service}] = strings.TrimPrefix(c.Names[0], "/")
		}
	}
	return m
}

// buildContainerEdges creates secondary-network, volume-mount, and depends_on edges
// for a single container.
func buildContainerEdges(name string, c containertypes.Summary, networkIDToName, networkProjects map[string]string, serviceNames map[serviceKey]string) []Edge {
	var edges []Edge

	// Secondary network connections
	_, secondaryNets := classifyContainerNetworks(c, networkIDToName, networkProjects)
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
		if m.Type == mountTypeVolume {
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
	project := ProjectOf(c.Labels)
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
// and splits them into primary and secondary, preferring the container's own
// compose-project network as primary.
func classifyContainerNetworks(c containertypes.Summary, networkIDToName, networkProjects map[string]string) (primary string, secondary []string) {
	var tracked []string
	if c.NetworkSettings != nil {
		for _, ns := range c.NetworkSettings.Networks {
			if name, ok := networkIDToName[ns.NetworkID]; ok {
				tracked = append(tracked, name)
			}
		}
	}
	return classifyNetworks(tracked, ProjectOf(c.Labels), networkProjects)
}

// projectMember is one workload (container or swarm service) of a project,
// flagged when it is DockGraph itself.
type projectMember struct {
	project string
	self    bool
}

// containerMembers lists the project membership of each container.
func containerMembers(containers []containertypes.Summary) []projectMember {
	members := make([]projectMember, 0, len(containers))
	for _, c := range containers {
		members = append(members, projectMember{ProjectOf(c.Labels), isSelfExcluded(c.Labels)})
	}
	return members
}

// selfOnlyProjects returns the set of projects (compose projects or swarm
// stacks) whose only workload is DockGraph itself. A project's auto-created
// networks and volumes are hidden only for these — if a project also runs real
// services, its resources are part of the visible topology.
func selfOnlyProjects(members []projectMember) map[string]bool {
	hasSelf := make(map[string]bool)
	hasOther := make(map[string]bool)
	for _, m := range members {
		if m.project == "" {
			continue
		}
		if m.self {
			hasSelf[m.project] = true
		} else {
			hasOther[m.project] = true
		}
	}

	result := make(map[string]bool)
	for project := range hasSelf {
		if !hasOther[project] {
			result[project] = true
		}
	}
	return result
}

// buildSnapshot queries the Docker daemon and assembles a complete graph of
// containers, networks, volumes, and their relationships.
func (d *DockerCollector) buildSnapshot(ctx context.Context) (GraphSnapshot, error) {
	res, err := fetchResources(ctx, d.client)
	if err != nil {
		return GraphSnapshot{}, err
	}

	// Hide auto-created networks and volumes only for projects whose sole
	// container is DockGraph itself. When DockGraph shares a project with real
	// services, those resources belong to the visible topology and must stay —
	// only the DockGraph container is excluded.
	snap := assembleSnapshot(res, selfOnlyProjects(containerMembers(res.containers)))
	sortSnapshot(&snap)
	return snap, nil
}

// assembleSnapshot converts raw Docker resources into graph nodes and edges.
// Networks and volumes owned by a project in selfProjects are omitted.
func assembleSnapshot(res dockerResources, selfProjects map[string]bool) GraphSnapshot {
	snap := GraphSnapshot{
		Nodes: []Node{},
		Edges: []Edge{},
	}

	networkIDToName := resolveNetworkNames(res.networks)
	networkProjects := networkProjectMap(res.networks)
	for _, n := range res.networks {
		if networkIDToName[n.ID] != "" {
			project := ProjectOf(n.Labels)
			if project != "" && selfProjects[project] {
				continue
			}
			node := buildNetworkNode(n.Name, n.Driver)
			if len(n.IPAM.Config) > 0 {
				node.Subnet = n.IPAM.Config[0].Subnet
				node.Gateway = n.IPAM.Config[0].Gateway
			}
			node.Labels = projectLabels(n.Labels)
			node.Stack = project
			snap.Nodes = append(snap.Nodes, node)
		}
	}

	serviceNames := resolveServiceNames(res.containers)

	for _, c := range res.containers {
		if isSelfExcluded(c.Labels) {
			continue
		}
		if len(c.Names) == 0 {
			continue
		}

		name := strings.TrimPrefix(c.Names[0], "/")
		status := containerStatus(c.State, c.Status)
		ports := extractPorts(c.Ports)

		primary, _ := classifyContainerNetworks(c, networkIDToName, networkProjects)
		node := buildContainerNode(name, c.Image, status, ports)
		if c.Created > 0 {
			node.CreatedAt = time.Unix(c.Created, 0).UTC().Format(time.RFC3339)
		}
		if primary != "" {
			node.NetworkID = "network:" + primary
		}
		node.Labels = projectLabels(c.Labels)
		node.Stack = ProjectOf(c.Labels)
		snap.Nodes = append(snap.Nodes, node)

		snap.Edges = append(snap.Edges, buildContainerEdges(name, c, networkIDToName, networkProjects, serviceNames)...)
	}

	for _, v := range res.volumes {
		project := ProjectOf(v.Labels)
		if project != "" && selfProjects[project] {
			continue
		}
		node := buildVolumeNode(v.Name, v.Driver, "created")
		node.Labels = projectLabels(v.Labels)
		node.Stack = project
		snap.Nodes = append(snap.Nodes, node)
	}

	return snap
}

// sortSnapshot orders nodes and edges by ID. Deterministic ordering prevents
// false-positive diffs in the state manager's merge.
func sortSnapshot(snap *GraphSnapshot) {
	sort.Slice(snap.Nodes, func(i, j int) bool { return snap.Nodes[i].ID < snap.Nodes[j].ID })
	sort.Slice(snap.Edges, func(i, j int) bool { return snap.Edges[i].ID < snap.Edges[j].ID })
}

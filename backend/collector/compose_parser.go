package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/loader"
	composetypes "github.com/compose-spec/compose-go/v2/types"

	"github.com/dockgraph/dockgraph/secrets"
)

// composeNaming provides Docker Compose naming conventions for a given project.
// Docker Compose uses different separators for different resource types:
//
//	networks/volumes: {project}_{name}   (underscore)
//	containers:       {project}-{service}-1 (hyphens)
//
// When the file is deployed as a swarm stack, services are named
// {stack}_{service} and become "service" nodes instead of containers.
type composeNaming struct {
	project string
	swarm   bool
	// networks/volumes map a top-level key to its runtime name when it
	// differs from {project}_{key}: `external: true` resources keep their own
	// name, and an explicit `name:` overrides the prefix.
	networks map[string]string
	volumes  map[string]string
}

func (n composeNaming) network(name string) string {
	if resolved := n.networks[name]; resolved != "" {
		return resolved
	}
	return n.project + "_" + name
}

func (n composeNaming) volume(name string) string {
	if resolved := n.volumes[name]; resolved != "" {
		return resolved
	}
	return n.project + "_" + name
}

// newComposeNaming builds the naming scheme for a loaded project, recording
// the resolved runtime name of every top-level network and volume.
func newComposeNaming(project *composetypes.Project, swarm bool) composeNaming {
	n := composeNaming{
		project:  project.Name,
		swarm:    swarm,
		networks: make(map[string]string, len(project.Networks)),
		volumes:  make(map[string]string, len(project.Volumes)),
	}
	for key, net := range project.Networks {
		if net.Name != "" {
			n.networks[key] = net.Name
		}
	}
	for key, vol := range project.Volumes {
		if vol.Name != "" {
			n.volumes[key] = vol.Name
		}
	}
	return n
}

// container returns the runtime name of a service's workload: the first
// compose container, or the swarm service.
func (n composeNaming) container(name string) string {
	if n.swarm {
		return n.project + "_" + name
	}
	return n.project + "-" + name + "-1"
}

// nodeID returns the graph node ID of a service's workload.
func (n composeNaming) nodeID(name string) string {
	if n.swarm {
		return "service:" + n.container(name)
	}
	return "container:" + n.container(name)
}

// buildComposeNetworkNodes creates graph nodes for each non-default network
// defined in the compose file.
func buildComposeNetworkNodes(project *composetypes.Project, naming composeNaming, sourceName string) ([]Node, map[string]bool) {
	tracked := make(map[string]bool)
	var nodes []Node
	for name, net := range project.Networks {
		if name == "default" {
			continue
		}
		tracked[name] = true
		fullName := naming.network(name)
		node := buildNetworkNode(fullName, "")
		node.Status = "not_running"
		node.Source = sourceName
		// External networks exist independently of the project.
		if !bool(net.External) {
			node.Stack = naming.project
		}
		nodes = append(nodes, node)
	}
	return nodes, tracked
}

// buildComposeVolumeNodes creates graph nodes for each named volume
// defined in the compose file.
func buildComposeVolumeNodes(project *composetypes.Project, naming composeNaming, sourceName string) []Node {
	var nodes []Node
	for name, vol := range project.Volumes {
		fullName := naming.volume(name)
		node := buildVolumeNode(fullName, "", "not_running")
		node.Source = sourceName
		if !bool(vol.External) {
			node.Stack = naming.project
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// parseComposePorts converts compose port configs into the common PortMapping format.
// Port ranges (e.g. "8080-8090") are expanded into individual mappings.
func parseComposePorts(ports []composetypes.ServicePortConfig) []PortMapping {
	var result []PortMapping
	for _, p := range ports {
		if p.Published == "" {
			continue
		}

		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}

		var startHost, endHost int
		if n, _ := fmt.Sscanf(p.Published, "%d-%d", &startHost, &endHost); n == 2 && endHost >= startHost {
			containerPort := int(p.Target)
			for hp := startHost; hp <= endHost; hp++ {
				result = append(result, PortMapping{
					Host:      hp,
					Container: containerPort + (hp - startHost),
					Protocol:  proto,
				})
			}
		} else {
			var hostPort int
			if n, _ := fmt.Sscanf(p.Published, "%d", &hostPort); n < 1 {
				continue
			}
			result = append(result, PortMapping{
				Host:      hostPort,
				Container: int(p.Target),
				Protocol:  proto,
			})
		}
	}
	return result
}

// buildServiceNode creates a container node (or, for a swarm stack, a service
// node) for a single compose service, classifies its networks, and delegates
// edge creation to buildServiceEdges.
func buildServiceNode(svc composetypes.ServiceConfig, naming composeNaming, trackedNets map[string]bool, sourceName string) (Node, []Edge) {
	svcName := naming.container(svc.Name)

	var trackedNetNames []string
	for netName := range svc.Networks {
		if trackedNets[netName] {
			trackedNetNames = append(trackedNetNames, netName)
		}
	}
	primary, secondaryNets := classifyNetworks(trackedNetNames, "", nil)

	node := buildContainerNode(svcName, svc.Image, "not_running", parseComposePorts(svc.Ports))
	node.Source = sourceName
	node.Stack = naming.project
	node.Compose = buildComposeConfig(svc, naming)
	if naming.swarm {
		node.ID = naming.nodeID(svc.Name)
		node.Type = nodeTypeService
		node.Service = stackServiceInfo(svc)
	}
	if primary != "" {
		node.NetworkID = "network:" + naming.network(primary)
	}

	edges := buildServiceEdges(svc, naming, svcName, secondaryNets)
	return node, edges
}

// buildServiceEdges creates secondary-network, depends_on, and volume-mount
// edges for a compose service.
func buildServiceEdges(svc composetypes.ServiceConfig, naming composeNaming, svcName string, secondaryNets []string) []Edge {
	containerID := naming.nodeID(svc.Name)
	var edges []Edge

	for _, netName := range secondaryNets {
		fullNetName := naming.network(netName)
		edges = append(edges, Edge{
			ID:     "e:net:" + svcName + ":" + fullNetName,
			Type:   "secondary_network",
			Source: containerID,
			Target: "network:" + fullNetName,
		})
	}

	for depName := range svc.DependsOn {
		depFullName := naming.container(depName)
		edges = append(edges, Edge{
			ID:     "e:dep:" + svcName + ":" + depFullName,
			Type:   "depends_on",
			Source: containerID,
			Target: naming.nodeID(depName),
		})
	}

	for _, v := range svc.Volumes {
		if v.Type == mountTypeVolume {
			fullVolName := naming.volume(v.Source)
			edges = append(edges, Edge{
				ID:        "e:vol:" + fullVolName + ":" + svcName,
				Type:      "volume_mount",
				Source:    "volume:" + fullVolName,
				Target:    containerID,
				MountPath: v.Target,
			})
		}
	}

	return edges
}

// stackServiceInfo reads a stack service's deploy mode and replica count into
// the ghost node's ServiceInfo. Replicated services and jobs default to one
// replica; global services and global jobs have no fixed count until the
// manager schedules them on the eligible nodes.
func stackServiceInfo(svc composetypes.ServiceConfig) *ServiceInfo {
	info := &ServiceInfo{Mode: serviceModeReplicated, Replicas: ReplicaCount{Desired: 1}}
	if svc.Deploy == nil {
		return info
	}
	switch svc.Deploy.Mode {
	case serviceModeGlobal, serviceModeGlobalJob:
		info.Mode = svc.Deploy.Mode
		info.Replicas.Desired = 0
		return info
	case serviceModeReplicatedJob:
		info.Mode = svc.Deploy.Mode
	}
	if svc.Deploy.Replicas != nil {
		info.Replicas.Desired = *svc.Deploy.Replicas
	}
	return info
}

// isComposeServiceSelfExcluded checks a compose service's container labels
// and, for stacks, its deploy labels (which become the swarm service labels).
func isComposeServiceSelfExcluded(svc composetypes.ServiceConfig) bool {
	if isSelfExcluded(svc.Labels) {
		return true
	}
	return svc.Deploy != nil && isSelfExcluded(svc.Deploy.Labels)
}

// buildComposeConfig extracts service configuration from a compose service
// for display in the detail panel when the container isn't running.
func buildComposeConfig(svc composetypes.ServiceConfig, naming composeNaming) *ComposeConfig {
	cfg := &ComposeConfig{
		Service:    svc.Name,
		Command:    svc.Command,
		Entrypoint: svc.Entrypoint,
		Restart:    svc.Restart,
		User:       svc.User,
		WorkingDir: svc.WorkingDir,
		Privileged: svc.Privileged,
		ReadOnly:   svc.ReadOnly,
		CapAdd:     svc.CapAdd,
		CapDrop:    svc.CapDrop,
	}

	// Mask credential-looking values, matching how running containers are
	// served, so secrets declared in a compose file aren't shown in clear.
	if len(svc.Environment) > 0 {
		cfg.Environment = make(map[string]string, len(svc.Environment))
		for k, v := range svc.Environment {
			if v == nil {
				continue
			}
			if secrets.IsSensitiveKey(k) {
				cfg.Environment[k] = secrets.Masked
			} else {
				cfg.Environment[k] = *v
			}
		}
	}

	// Labels get the same masking as live service labels: a stack-file ghost
	// shows them in the service panel, and they may carry credentials.
	if len(svc.Labels) > 0 {
		cfg.Labels = secrets.MaskLabels(svc.Labels)
	}

	// Store the full container node name (project-prefixed, replica-suffixed)
	// so the side panel's dependency links resolve to real graph nodes, the
	// same way Networks below uses the fully-qualified name.
	for depName := range svc.DependsOn {
		cfg.DependsOn = append(cfg.DependsOn, naming.container(depName))
	}

	// Named volumes carry their fully-qualified node name so the panel link
	// resolves to the matching volume node; bind mounts keep only the host path.
	for _, v := range svc.Volumes {
		mount := ComposeMount{
			Type:        v.Type,
			Source:      v.Source,
			Destination: v.Target,
			RW:          !v.ReadOnly,
		}
		if v.Type == mountTypeVolume {
			mount.Name = naming.volume(v.Source)
		}
		cfg.Volumes = append(cfg.Volumes, mount)
	}

	for netName := range svc.Networks {
		cfg.Networks = append(cfg.Networks, naming.network(netName))
	}

	return cfg
}

// composeParseOptions controls how a compose file is interpreted.
// ProjectName, when set, overrides the file's top-level `name` (as
// `docker stack deploy -c file NAME` does). Swarm treats the file as a stack:
// services become service nodes and, without a name, the project falls back
// to the file's basename.
type composeParseOptions struct {
	ProjectName string
	Swarm       bool
}

// parseComposeFile loads a Docker Compose file and converts it into a graph
// snapshot containing all services, networks, volumes, and their relationships.
func parseComposeFile(ctx context.Context, path, sourceName string) (GraphSnapshot, error) {
	return parseComposeFileWith(ctx, path, sourceName, composeParseOptions{})
}

// parseComposeFileWith is parseComposeFile with explicit naming/mode options.
func parseComposeFileWith(ctx context.Context, path, sourceName string, opts composeParseOptions) (GraphSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GraphSnapshot{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	project, err := loader.LoadWithContext(ctx, composetypes.ConfigDetails{
		ConfigFiles: []composetypes.ConfigFile{
			{Filename: path, Content: data},
		},
	}, func(o *loader.Options) {
		switch {
		case opts.ProjectName != "":
			o.SetProjectName(loader.NormalizeProjectName(opts.ProjectName), true)
		case opts.Swarm:
			// Stack files rarely declare `name`; default to the file basename.
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			o.SetProjectName(loader.NormalizeProjectName(base), false)
		}
	})
	if err != nil {
		return GraphSnapshot{}, fmt.Errorf("%w (does the file have a top-level 'name' field?)", err)
	}

	naming := newComposeNaming(project, opts.Swarm)
	var snap GraphSnapshot

	networkNodes, trackedNets := buildComposeNetworkNodes(project, naming, sourceName)
	snap.Nodes = append(snap.Nodes, networkNodes...)
	snap.Nodes = append(snap.Nodes, buildComposeVolumeNodes(project, naming, sourceName)...)

	for _, svc := range project.AllServices() {
		if isComposeServiceSelfExcluded(svc) {
			continue
		}
		node, edges := buildServiceNode(svc, naming, trackedNets, sourceName)
		snap.Nodes = append(snap.Nodes, node)
		snap.Edges = append(snap.Edges, edges...)
	}

	return snap, nil
}

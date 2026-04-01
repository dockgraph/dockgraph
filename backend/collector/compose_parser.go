package collector

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/compose-spec/compose-go/v2/loader"
	composetypes "github.com/compose-spec/compose-go/v2/types"
)

// composeNaming provides Docker Compose naming conventions for a given project.
// Docker Compose uses different separators for different resource types:
//
//	networks/volumes: {project}_{name}   (underscore)
//	containers:       {project}-{service}-1 (hyphens)
type composeNaming struct {
	project string
}

func (n composeNaming) network(name string) string   { return n.project + "_" + name }
func (n composeNaming) volume(name string) string    { return n.project + "_" + name }
func (n composeNaming) container(name string) string { return n.project + "-" + name + "-1" }

// buildComposeNetworkNodes creates graph nodes for each non-default network
// defined in the compose file.
func buildComposeNetworkNodes(project *composetypes.Project, naming composeNaming, sourceName string) ([]Node, map[string]bool) {
	tracked := make(map[string]bool)
	var nodes []Node
	for name := range project.Networks {
		if name == "default" {
			continue
		}
		tracked[name] = true
		fullName := naming.network(name)
		node := buildNetworkNode(fullName, "")
		node.Status = "not_running"
		node.Source = sourceName
		nodes = append(nodes, node)
	}
	return nodes, tracked
}

// buildComposeVolumeNodes creates graph nodes for each named volume
// defined in the compose file.
func buildComposeVolumeNodes(project *composetypes.Project, naming composeNaming, sourceName string) []Node {
	var nodes []Node
	for name := range project.Volumes {
		fullName := naming.volume(name)
		node := buildVolumeNode(fullName, "", "not_running")
		node.Source = sourceName
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

		var startHost, endHost int
		if n, _ := fmt.Sscanf(p.Published, "%d-%d", &startHost, &endHost); n == 2 && endHost >= startHost {
			containerPort := int(p.Target)
			for hp := startHost; hp <= endHost; hp++ {
				result = append(result, PortMapping{
					Host:      hp,
					Container: containerPort + (hp - startHost),
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
			})
		}
	}
	return result
}

// buildServiceNode creates a container node for a single compose service,
// classifies its networks, and delegates edge creation to buildServiceEdges.
func buildServiceNode(svc composetypes.ServiceConfig, naming composeNaming, trackedNets map[string]bool, sourceName string) (Node, []Edge) {
	svcName := naming.container(svc.Name)

	var trackedNetNames []string
	for netName := range svc.Networks {
		if trackedNets[netName] {
			trackedNetNames = append(trackedNetNames, netName)
		}
	}
	primary, secondaryNets := classifyNetworks(trackedNetNames)

	node := buildContainerNode(svcName, svc.Image, "not_running", parseComposePorts(svc.Ports))
	node.Source = sourceName
	node.Compose = buildComposeConfig(svc, naming)
	if primary != "" {
		node.NetworkID = "network:" + naming.network(primary)
	}

	edges := buildServiceEdges(svc, naming, svcName, secondaryNets)
	return node, edges
}

// buildServiceEdges creates secondary-network, depends_on, and volume-mount
// edges for a compose service.
func buildServiceEdges(svc composetypes.ServiceConfig, naming composeNaming, svcName string, secondaryNets []string) []Edge {
	containerID := "container:" + svcName
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
			Target: "container:" + depFullName,
		})
	}

	for _, v := range svc.Volumes {
		if v.Type == "volume" {
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

	if len(svc.Environment) > 0 {
		cfg.Environment = make(map[string]string, len(svc.Environment))
		for k, v := range svc.Environment {
			if v != nil {
				cfg.Environment[k] = *v
			}
		}
	}

	for depName := range svc.DependsOn {
		cfg.DependsOn = append(cfg.DependsOn, depName)
	}

	for _, v := range svc.Volumes {
		summary := v.Source + ":" + v.Target
		if v.ReadOnly {
			summary += ":ro"
		}
		cfg.Volumes = append(cfg.Volumes, summary)
	}

	for netName := range svc.Networks {
		cfg.Networks = append(cfg.Networks, naming.network(netName))
	}

	return cfg
}

// parseComposeFile loads a Docker Compose file and converts it into a graph
// snapshot containing all services, networks, volumes, and their relationships.
func parseComposeFile(ctx context.Context, path, sourceName string) (GraphSnapshot, error) {
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
	})
	if err != nil {
		return GraphSnapshot{}, fmt.Errorf("%w (does the file have a top-level 'name' field?)", err)
	}

	naming := composeNaming{project: project.Name}
	var snap GraphSnapshot

	networkNodes, trackedNets := buildComposeNetworkNodes(project, naming, sourceName)
	snap.Nodes = append(snap.Nodes, networkNodes...)
	snap.Nodes = append(snap.Nodes, buildComposeVolumeNodes(project, naming, sourceName)...)

	for _, svc := range project.AllServices() {
		if isSelfExcluded(svc.Labels) {
			continue
		}
		node, edges := buildServiceNode(svc, naming, trackedNets, sourceName)
		snap.Nodes = append(snap.Nodes, node)
		snap.Edges = append(snap.Edges, edges...)
	}

	return snap, nil
}

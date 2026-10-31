package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/filters"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/dockgraph/dockgraph/collector"
	"github.com/dockgraph/dockgraph/secrets"
)

// NetworkInspector is the subset of the Docker API needed for network inspection.
type NetworkInspector interface {
	NetworkInspect(ctx context.Context, networkID string, options networktypes.InspectOptions) (networktypes.Inspect, error)
}

// NetworkMemberLister is the subset of the swarm API needed to list the
// services and tasks attached to a swarm network across the cluster.
type NetworkMemberLister interface {
	ServiceList(ctx context.Context, options swarm.ServiceListOptions) ([]swarm.Service, error)
	TaskList(ctx context.Context, options swarm.TaskListOptions) ([]swarm.Task, error)
	NodeList(ctx context.Context, options swarm.NodeListOptions) ([]swarm.Node, error)
}

// networkService is a swarm service attached to the inspected network.
type networkService struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
	// VIP is the service's virtual IP on the network, empty in DNS
	// round-robin endpoint mode.
	VIP   string        `json:"vip,omitempty"`
	Tasks []networkTask `json:"tasks"`
}

// networkTask is a current task of a networkService and its addresses on the network.
type networkTask struct {
	collector.TaskInfo
	Addresses []string `json:"addresses,omitempty"`
}

// HandleNetworkInspect returns a handler for GET /api/networks/{name}. With
// members set (swarm mode), a swarm network also lists its services and their
// tasks cluster-wide: Docker's own inspect only sees the local node.
func HandleNetworkInspect(inspector NetworkInspector, members NetworkMemberLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !validResourceName.MatchString(name) {
			jsonError(w, "invalid network name", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		network, err := inspector.NetworkInspect(ctx, name, networktypes.InspectOptions{})
		if err != nil {
			log.Printf("inspect network %s: %v", name, err)
			inspectError(w, "network", err)
			return
		}

		var resp map[string]any
		if members != nil && network.Scope == swarmScope {
			services, taskContainers, err := listNetworkServices(ctx, members, network)
			if err != nil {
				// The local view is all there is: show it, flagged as partial.
				log.Printf("network %s members: %v", name, err)
				resp = buildNetworkInspectResponse(network, nil)
				resp["servicesUnavailable"] = true
			} else {
				resp = buildNetworkInspectResponse(network, taskContainers)
				resp["services"] = services
			}
		} else {
			resp = buildNetworkInspectResponse(network, nil)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// swarmScope is the scope of a network that spans the swarm (an overlay).
const swarmScope = "swarm"

// listNetworkServices lists the swarm services attached to a network (by the
// graph's rules, so DockGraph's own are left out) with their current tasks
// and each task's addresses on it. It also returns the container IDs of all
// current tasks, which the containers list leaves to their services. Nodes
// only add hostnames, so a failed node list is not an error.
func listNetworkServices(ctx context.Context, members NetworkMemberLister, network networktypes.Inspect) ([]networkService, map[string]bool, error) {
	services, err := members.ServiceList(ctx, swarm.ServiceListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("list services: %w", err)
	}
	// Current tasks only: replaced and finished ones have left the network.
	tasks, err := members.TaskList(ctx, swarm.TaskListOptions{
		Filters: filters.NewArgs(filters.Arg("desired-state", string(swarm.TaskStateRunning))),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list tasks: %w", err)
	}
	nodes, err := members.NodeList(ctx, swarm.NodeListOptions{})
	if err != nil {
		log.Printf("network %s nodes: %v", network.Name, err)
	}

	taskContainers := make(map[string]bool)
	tasksByService := make(map[string][]swarm.Task)
	addresses := make(map[string][]string)
	for _, t := range tasks {
		if cs := t.Status.ContainerStatus; cs != nil && cs.ContainerID != "" {
			taskContainers[cs.ContainerID] = true
		}
		tasksByService[t.ServiceID] = append(tasksByService[t.ServiceID], t)
		for _, a := range t.NetworksAttachments {
			if a.Network.ID == network.ID {
				addresses[t.ID] = a.Addresses
			}
		}
	}

	result := make([]networkService, 0)
	for _, svc := range services {
		if collector.IsServiceSelfExcluded(svc) || !collector.ServiceOnNetwork(svc, network.ID, network.Name) {
			continue
		}
		// The task list the service panel shows, sorted by slot and node.
		info, _ := collector.SummarizeService(svc, tasksByService[svc.ID], nodes)
		entry := networkService{
			Name:  svc.Spec.Name,
			Mode:  info.Mode,
			VIP:   serviceVIP(svc, network.ID),
			Tasks: make([]networkTask, 0, len(info.Tasks)),
		}
		for _, ti := range info.Tasks {
			entry.Tasks = append(entry.Tasks, networkTask{TaskInfo: ti, Addresses: addresses[ti.ID]})
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, taskContainers, nil
}

// serviceVIP returns a service's virtual IP on a network, or "" if it has none.
func serviceVIP(svc swarm.Service, networkID string) string {
	for _, v := range svc.Endpoint.VirtualIPs {
		if v.NetworkID == networkID {
			return v.Addr
		}
	}
	return ""
}

// buildNetworkInspectResponse renders a network for the detail panel. Endpoints
// of taskContainers are left out (their services list them) and so are load
// balancers, which no container backs.
func buildNetworkInspectResponse(network networktypes.Inspect, taskContainers map[string]bool) map[string]any {
	resp := map[string]any{
		fieldName:    network.Name,
		"id":         network.ID,
		"driver":     network.Driver,
		"scope":      network.Scope,
		"internal":   network.Internal,
		"enableIPv6": network.EnableIPv6,
		"created":    network.Created,
		"options":    secrets.MaskLabels(network.Options),
		fieldLabels:  secrets.MaskLabels(network.Labels),
	}

	if network.IPAM.Config != nil {
		ipamConfigs := make([]map[string]any, 0, len(network.IPAM.Config))
		for _, cfg := range network.IPAM.Config {
			entry := map[string]any{
				"subnet":       cfg.Subnet,
				"gateway":      cfg.Gateway,
				"ipRange":      cfg.IPRange,
				"auxAddresses": cfg.AuxAddress,
			}
			ipamConfigs = append(ipamConfigs, entry)
		}
		resp["ipam"] = map[string]any{
			"driver": network.IPAM.Driver,
			"config": ipamConfigs,
		}
	}

	if network.Containers != nil {
		containers := make([]map[string]string, 0, len(network.Containers))
		for id, ep := range network.Containers {
			if isLoadBalancerEndpoint(id) || taskContainers[id] {
				continue
			}
			containers = append(containers, map[string]string{
				"id":          id,
				fieldName:     ep.Name,
				"ipv4Address": ep.IPv4Address,
				"ipv6Address": ep.IPv6Address,
				"macAddress":  ep.MacAddress,
			})
		}
		resp["containers"] = containers
	}

	return resp
}

// isLoadBalancerEndpoint reports whether a network endpoint is a swarm load
// balancer (lb-<network> on an overlay, ingress-sbox on the ingress network).
// Docker lists them among the network's containers, but no container backs them.
func isLoadBalancerEndpoint(id string) bool {
	return strings.HasPrefix(id, "lb-") || id == "ingress-sbox"
}

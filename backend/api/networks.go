package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	networktypes "github.com/docker/docker/api/types/network"
)

// NetworkInspector is the subset of the Docker API needed for network inspection.
type NetworkInspector interface {
	NetworkInspect(ctx context.Context, networkID string, options networktypes.InspectOptions) (networktypes.Inspect, error)
}

// HandleNetworkInspect returns a handler for GET /api/networks/{name}.
func HandleNetworkInspect(inspector NetworkInspector) http.HandlerFunc {
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
			jsonError(w, "network not found", http.StatusNotFound)
			return
		}

		resp := buildNetworkInspectResponse(network)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func buildNetworkInspectResponse(network networktypes.Inspect) map[string]any {
	resp := map[string]any{
		"name":       network.Name,
		"id":         network.ID,
		"driver":     network.Driver,
		"scope":      network.Scope,
		"internal":   network.Internal,
		"enableIPv6": network.EnableIPv6,
		"created":    network.Created,
		"options":    network.Options,
		"labels":     network.Labels,
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
			containers = append(containers, map[string]string{
				"id":          id,
				"name":        ep.Name,
				"ipv4Address": ep.IPv4Address,
				"ipv6Address": ep.IPv6Address,
				"macAddress":  ep.MacAddress,
			})
		}
		resp["containers"] = containers
	}

	return resp
}

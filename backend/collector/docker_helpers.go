package collector

import (
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
)

// SelfExcludeLabel marks a container as belonging to dockgraph itself,
// causing it to be excluded from the topology graph.
const SelfExcludeLabel = "dockgraph.self"

// selfExcludeValue is the expected label value for self-exclusion.
const selfExcludeValue = "true"

// isSelfExcluded checks whether a container's labels mark it as belonging to dockgraph.
func isSelfExcluded(labels map[string]string) bool {
	return labels[SelfExcludeLabel] == selfExcludeValue
}

// isTopologyEvent returns true for Docker events that indicate a change
// in the container/network/volume topology.
func isTopologyEvent(action events.Action) bool {
	switch string(action) {
	case "start", "stop", "die", "kill", "create", "destroy", "rename",
		"pause", "unpause", "health_status",
		"connect", "disconnect":
		return true
	}
	return false
}

// containerStatus derives a display status from Docker's state and status strings.
// The unhealthy status is surfaced explicitly because Docker reports it only inside
// the status text (e.g. "Up 5 minutes (unhealthy)"), not as a standalone state.
func containerStatus(state, statusStr string) string {
	if strings.Contains(statusStr, "(unhealthy)") {
		return "unhealthy"
	}
	return state
}

// extractPorts converts Docker's port list into the wire-format representation,
// keeping only ports that have a published (host) mapping. Docker returns
// separate entries for IPv4 and IPv6 bindings, so we deduplicate by
// host:container pair.
func extractPorts(apiPorts []containertypes.Port) []PortMapping {
	type portKey struct{ host, container int }
	seen := make(map[portKey]bool)
	var ports []PortMapping
	for _, p := range apiPorts {
		if p.PublicPort != 0 {
			k := portKey{int(p.PublicPort), int(p.PrivatePort)}
			if !seen[k] {
				seen[k] = true
				ports = append(ports, PortMapping{
					Host:      k.host,
					Container: k.container,
				})
			}
		}
	}
	return ports
}

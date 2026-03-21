package collector

import (
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
)

// SelfExcludeLabel marks a container as belonging to docker-flow itself,
// causing it to be excluded from the topology graph.
const SelfExcludeLabel = "dev.dockerflow.self"

func isSelfContainer(image string) bool {
	return strings.Contains(image, "docker-flow")
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
// keeping only ports that have a published (host) mapping.
func extractPorts(apiPorts []containertypes.Port) []PortMapping {
	var ports []PortMapping
	for _, p := range apiPorts {
		if p.PublicPort != 0 {
			ports = append(ports, PortMapping{
				Host:      int(p.PublicPort),
				Container: int(p.PrivatePort),
			})
		}
	}
	return ports
}

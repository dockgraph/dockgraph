package collector

import "sort"

// buildContainerNode creates a container-type graph node with the given attributes.
func buildContainerNode(name, image, status string, ports []PortMapping) Node {
	return Node{
		ID:     "container:" + name,
		Type:   nodeTypeContainer,
		Name:   name,
		Image:  image,
		Status: status,
		Ports:  ports,
	}
}

// buildNetworkNode creates a network-type graph node.
func buildNetworkNode(name, driver string) Node {
	return Node{
		ID:     "network:" + name,
		Type:   "network",
		Name:   name,
		Driver: driver,
	}
}

// buildVolumeNode creates a volume-type graph node.
func buildVolumeNode(name, driver, status string) Node {
	return Node{
		ID:     "volume:" + name,
		Type:   nodeTypeVolume,
		Name:   name,
		Driver: driver,
		Status: status,
	}
}

// classifyNetworks designates one network as primary (used for hierarchy
// grouping) and the rest as secondary. It prefers the network owned by the
// container's own compose project, so a container is homed in its own stack
// rather than a shared or external network it merely joins; without such a
// match it falls back to the first network alphabetically. Names are sorted
// first so assignment stays stable across polls. ownProject is the container's
// compose project; networkProjects maps network name to its owning project.
func classifyNetworks(netNames []string, ownProject string, networkProjects map[string]string) (primary string, secondary []string) {
	sort.Strings(netNames)

	if ownProject != "" {
		for _, name := range netNames {
			if networkProjects[name] == ownProject {
				primary = name
				break
			}
		}
	}
	if primary == "" && len(netNames) > 0 {
		primary = netNames[0]
	}

	for _, name := range netNames {
		if name != primary {
			secondary = append(secondary, name)
		}
	}
	return
}

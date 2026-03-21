package collector

import "sort"

// buildContainerNode creates a container-type graph node with the given attributes.
func buildContainerNode(name, image, status string, labels map[string]string, ports []PortMapping) Node {
	return Node{
		ID:     "container:" + name,
		Type:   "container",
		Name:   name,
		Image:  image,
		Status: status,
		Labels: labels,
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
func buildVolumeNode(name, driver string) Node {
	return Node{
		ID:     "volume:" + name,
		Type:   "volume",
		Name:   name,
		Driver: driver,
	}
}

// classifyNetworks sorts network names and designates the first alphabetically
// as the primary (used for hierarchy grouping), rest as secondary. Sorting
// ensures stable assignment across polls so containers don't jump between groups.
func classifyNetworks(netNames []string) (primary string, secondary []string) {
	sort.Strings(netNames)
	for _, name := range netNames {
		if primary == "" {
			primary = name
		} else {
			secondary = append(secondary, name)
		}
	}
	return
}

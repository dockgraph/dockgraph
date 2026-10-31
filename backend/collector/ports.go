package collector

import (
	"cmp"
	"slices"
	"strings"
)

// PortRanges returns ports with each run of mappings that are one port apart
// on both sides, with one protocol, as one mapping of the run's range, the
// way `docker ps` lists them. Docker, swarm and the compose loader list a
// published range port by port, so a service publishing thousands of ports
// would otherwise be thousands of mappings in the graph. The result is
// sorted by host port.
func PortRanges(ports []PortMapping) []PortMapping {
	if len(ports) < 2 {
		return ports
	}
	sorted := slices.Clone(ports)
	slices.SortFunc(sorted, func(a, b PortMapping) int {
		return cmp.Or(strings.Compare(a.Protocol, b.Protocol), cmp.Compare(a.Host, b.Host), cmp.Compare(a.Container, b.Container))
	})
	var ranges []PortMapping
	for _, p := range sorted {
		if n := len(ranges); n > 0 && ranges[n-1].continuedBy(p) {
			ranges[n-1].HostEnd, ranges[n-1].ContainerEnd = p.Host, p.Container
			continue
		}
		ranges = append(ranges, p)
	}
	slices.SortStableFunc(ranges, func(a, b PortMapping) int {
		return cmp.Or(cmp.Compare(a.Host, b.Host), cmp.Compare(a.Container, b.Container))
	})
	return ranges
}

// continuedBy reports whether p, a mapping of one port, comes next in the
// run m is or starts: one port past it on both sides, with its protocol. A
// host range for one container port, and an unpublished port, start no run.
func (m PortMapping) continuedBy(p PortMapping) bool {
	single := p.HostEnd == 0 && p.ContainerEnd == 0
	hostRange := m.HostEnd != 0 && m.ContainerEnd == 0
	return single && !hostRange && m.Host != 0 && p.Protocol == m.Protocol &&
		p.Host == max(m.Host, m.HostEnd)+1 && p.Container == max(m.Container, m.ContainerEnd)+1
}

package api

import (
	"strconv"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/dockgraph/dockgraph/collector"
	"github.com/dockgraph/dockgraph/secrets"
)

// buildPorts lists the container's published ports, ranges as ranges
// (collector.PortRanges), each side written as Docker writes it: "8000" or
// "8000-8010". A port bound for both IPv4 and IPv6 is listed once.
func buildPorts(info containertypes.InspectResponse) []map[string]any {
	ports := make([]map[string]any, 0)
	if info.NetworkSettings == nil {
		return ports
	}
	var mappings []collector.PortMapping
	seen := make(map[collector.PortMapping]bool)
	for port, bindings := range info.NetworkSettings.Ports {
		for _, b := range bindings {
			host, err := strconv.Atoi(b.HostPort)
			if err != nil {
				// Not a port number: listed as Docker gives it.
				ports = append(ports, map[string]any{
					"hostPort":      b.HostPort,
					"containerPort": port.Port(),
					"protocol":      port.Proto(),
				})
				continue
			}
			m := collector.PortMapping{Host: host, Container: port.Int(), Protocol: port.Proto()}
			if !seen[m] {
				seen[m] = true
				mappings = append(mappings, m)
			}
		}
	}
	for _, m := range collector.PortRanges(mappings) {
		ports = append(ports, map[string]any{
			"hostPort":      portSpan(m.Host, m.HostEnd),
			"containerPort": portSpan(m.Container, m.ContainerEnd),
			"protocol":      m.Protocol,
		})
	}
	return ports
}

// portSpan writes a port, or the range of ports from start to end when end
// is set, as Docker does.
func portSpan(start, end int) string {
	if end == 0 {
		return strconv.Itoa(start)
	}
	return strconv.Itoa(start) + "-" + strconv.Itoa(end)
}

func buildMounts(info containertypes.InspectResponse) []map[string]any {
	mounts := make([]map[string]any, 0, len(info.Mounts))
	for _, m := range info.Mounts {
		entry := map[string]any{
			"type":        string(m.Type),
			"source":      m.Source,
			"destination": m.Destination,
			"rw":          m.RW,
			"propagation": string(m.Propagation),
		}
		if m.Name != "" {
			entry[fieldName] = m.Name
		}
		mounts = append(mounts, entry)
	}
	return mounts
}

func buildNetworks(info containertypes.InspectResponse) []map[string]any {
	nets := make([]map[string]any, 0)
	if info.NetworkSettings == nil {
		return nets
	}
	for name, ns := range info.NetworkSettings.Networks {
		nets = append(nets, map[string]any{
			fieldName:     name,
			"ipAddress":   ns.IPAddress,
			"gateway":     ns.Gateway,
			"macAddress":  ns.MacAddress,
			"ipPrefixLen": ns.IPPrefixLen,
		})
	}
	return nets
}

func buildSecurity(info containertypes.InspectResponse) map[string]any {
	return map[string]any{
		"privileged":     info.HostConfig.Privileged,
		"readonlyRootfs": info.HostConfig.ReadonlyRootfs,
		"capAdd":         info.HostConfig.CapAdd,
		"capDrop":        info.HostConfig.CapDrop,
	}
}

func buildResources(info containertypes.InspectResponse) map[string]any {
	return map[string]any{
		"cpuQuota":          info.HostConfig.CPUQuota,
		"cpuPeriod":         info.HostConfig.CPUPeriod,
		"nanoCpus":          info.HostConfig.NanoCPUs,
		"memoryLimit":       info.HostConfig.Memory,
		"memoryReservation": info.HostConfig.MemoryReservation,
	}
}

func buildHealth(info containertypes.InspectResponse) map[string]any {
	h := info.State.Health
	resp := map[string]any{
		fieldStatus:     h.Status,
		"failingStreak": h.FailingStreak,
	}
	logs := make([]map[string]any, 0, len(h.Log))
	for _, entry := range h.Log {
		logs = append(logs, map[string]any{
			"start":    entry.Start,
			"end":      entry.End,
			"exitCode": entry.ExitCode,
			// Probe output often echoes responses from internal endpoints.
			"output": secrets.MaskValue(entry.Output),
		})
	}
	resp["log"] = logs
	return resp
}

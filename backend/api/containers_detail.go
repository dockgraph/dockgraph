package api

import (
	containertypes "github.com/docker/docker/api/types/container"
)

func buildPorts(info containertypes.InspectResponse) []map[string]any {
	var ports []map[string]any
	for port, bindings := range info.NetworkSettings.Ports {
		for _, b := range bindings {
			ports = append(ports, map[string]any{
				"hostPort":      b.HostPort,
				"containerPort": port.Port(),
				"protocol":      port.Proto(),
			})
		}
	}
	return ports
}

func buildMounts(info containertypes.InspectResponse) []map[string]any {
	mounts := make([]map[string]any, 0, len(info.Mounts))
	for _, m := range info.Mounts {
		mounts = append(mounts, map[string]any{
			"type":        string(m.Type),
			"source":      m.Source,
			"destination": m.Destination,
			"rw":          m.RW,
			"propagation": string(m.Propagation),
		})
	}
	return mounts
}

func buildNetworks(info containertypes.InspectResponse) []map[string]any {
	var nets []map[string]any
	if info.NetworkSettings == nil {
		return nets
	}
	for name, ns := range info.NetworkSettings.Networks {
		nets = append(nets, map[string]any{
			"name":        name,
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
		"status":        h.Status,
		"failingStreak": h.FailingStreak,
	}
	logs := make([]map[string]any, 0, len(h.Log))
	for _, entry := range h.Log {
		logs = append(logs, map[string]any{
			"start":    entry.Start,
			"end":      entry.End,
			"exitCode": entry.ExitCode,
			"output":   entry.Output,
		})
	}
	resp["log"] = logs
	return resp
}

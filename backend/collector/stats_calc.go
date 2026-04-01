package collector

import (
	containertypes "github.com/docker/docker/api/types/container"
)

// calcCPUPercent computes CPU usage percentage from Docker stats.
// Uses the delta between current and previous readings.
func calcCPUPercent(stats *containertypes.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta <= 0 || cpuDelta < 0 {
		return 0
	}
	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}
	return (cpuDelta / systemDelta) * onlineCPUs * 100
}

// calcCPUThrottle computes the percentage of periods where the container
// was throttled. Zero when no throttling is configured.
func calcCPUThrottle(stats *containertypes.StatsResponse) float64 {
	total := stats.CPUStats.ThrottlingData.Periods
	if total == 0 {
		return 0
	}
	return float64(stats.CPUStats.ThrottlingData.ThrottledPeriods) / float64(total) * 100
}

// calcMemUsage returns the actual memory used by the container, handling
// the difference between cgroup v1 and v2 reporting. On cgroup v1,
// Usage includes page cache, so we subtract InactiveFile. On cgroup v2,
// the Stats map may be empty but Usage is already the correct value.
func calcMemUsage(mem containertypes.MemoryStats) uint64 {
	if mem.Usage == 0 {
		return 0
	}
	// cgroup v1: subtract inactive file cache from usage for accurate RSS.
	if inactiveFile, ok := mem.Stats["inactive_file"]; ok && inactiveFile < mem.Usage {
		return mem.Usage - inactiveFile
	}
	// cgroup v2 or no Stats map: Usage is already correct.
	return mem.Usage
}

// sumNetworkIO sums rx/tx bytes and errors across all network interfaces.
func sumNetworkIO(networks map[string]containertypes.NetworkStats) (rx, tx, rxErr, txErr uint64) {
	for _, n := range networks {
		rx += n.RxBytes
		tx += n.TxBytes
		rxErr += n.RxErrors
		txErr += n.TxErrors
	}
	return
}

// sumBlockIO sums read and write bytes across all block devices.
func sumBlockIO(blkio containertypes.BlkioStats) (read, write uint64) {
	for _, entry := range blkio.IoServiceBytesRecursive {
		switch entry.Op {
		case "read", "Read":
			read += entry.Value
		case "write", "Write":
			write += entry.Value
		}
	}
	return
}

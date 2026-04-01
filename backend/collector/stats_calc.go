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

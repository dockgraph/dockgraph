package collector

// ContainerStats holds resource usage metrics for a single container.
type ContainerStats struct {
	CPUPercent   float64 `json:"cpuPercent"`
	CPUThrottled float64 `json:"cpuThrottled"`
	MemUsage     uint64  `json:"memUsage"`
	MemLimit     uint64  `json:"memLimit"`
	NetRx        uint64  `json:"netRx"`
	NetTx        uint64  `json:"netTx"`
	NetRxErrors  uint64  `json:"netRxErrors"`
	NetTxErrors  uint64  `json:"netTxErrors"`
	BlockRead    uint64  `json:"blockRead"`
	BlockWrite   uint64  `json:"blockWrite"`
	PIDs         uint64  `json:"pids"`
}

// StatsSnapshot maps container names to their current resource usage.
type StatsSnapshot struct {
	Stats map[string]ContainerStats `json:"stats"`
}

// NewStatsMessage wraps a stats snapshot for WebSocket transmission.
func NewStatsMessage(s StatsSnapshot) WireMessage {
	return WireMessage{Type: "stats", Version: 1, Data: s}
}

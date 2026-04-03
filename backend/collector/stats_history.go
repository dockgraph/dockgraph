package collector

import (
	"sync"
	"time"
)

// HistoryPoint is a single timestamped stats snapshot stored in the ring buffer.
type HistoryPoint struct {
	Time  time.Time
	Stats map[string]ContainerStats
}

// ContainerTimeSeries holds parallel arrays of metric values for one container.
type ContainerTimeSeries struct {
	CPU        []float64 `json:"cpu"`
	Mem        []uint64  `json:"mem"`
	NetRx      []uint64  `json:"netRx"`
	NetTx      []uint64  `json:"netTx"`
	BlockRead  []uint64  `json:"blockRead"`
	BlockWrite []uint64  `json:"blockWrite"`
}

// HistoryResult is the query response: shared timestamps + per-container series.
type HistoryResult struct {
	Timestamps []int64                        `json:"timestamps"`
	Containers map[string]ContainerTimeSeries `json:"containers"`
}

// StatsHistory stores timestamped stats snapshots in memory with a maximum retention period.
type StatsHistory struct {
	mu        sync.RWMutex
	points    []HistoryPoint
	retention time.Duration
}

// NewStatsHistory creates a ring buffer that retains data for the given duration.
func NewStatsHistory(retention time.Duration) *StatsHistory {
	return &StatsHistory{
		retention: retention,
		points:    make([]HistoryPoint, 0, 4096),
	}
}

// Record appends a timestamped stats snapshot.
func (h *StatsHistory) Record(ts time.Time, snap StatsSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.points = append(h.points, HistoryPoint{Time: ts, Stats: snap.Stats})
}

// Evict removes points older than the retention window.
func (h *StatsHistory) Evict() {
	h.mu.Lock()
	defer h.mu.Unlock()
	cutoff := time.Now().Add(-h.retention)
	i := 0
	for i < len(h.points) && h.points[i].Time.Before(cutoff) {
		i++
	}
	if i > 0 {
		h.points = append(h.points[:0], h.points[i:]...)
	}
}

// Downsample merges points older than 1 hour into buckets of the given width.
// Points within the last hour are kept at full resolution.
func (h *StatsHistory) Downsample(bucketWidth time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	oneHourAgo := time.Now().Add(-time.Hour)
	splitIdx := 0
	for splitIdx < len(h.points) && h.points[splitIdx].Time.Before(oneHourAgo) {
		splitIdx++
	}
	if splitIdx < 2 {
		return
	}

	old := h.points[:splitIdx]
	recent := h.points[splitIdx:]

	var downsampled []HistoryPoint
	bucketStart := old[0].Time.Truncate(bucketWidth)
	var bucket []HistoryPoint

	for _, p := range old {
		if p.Time.Before(bucketStart.Add(bucketWidth)) {
			bucket = append(bucket, p)
		} else {
			if len(bucket) > 0 {
				downsampled = append(downsampled, downsampleBucket(bucketStart, bucket))
			}
			bucketStart = p.Time.Truncate(bucketWidth)
			bucket = []HistoryPoint{p}
		}
	}
	if len(bucket) > 0 {
		downsampled = append(downsampled, downsampleBucket(bucketStart, bucket))
	}

	result := make([]HistoryPoint, 0, len(downsampled)+len(recent))
	result = append(result, downsampled...)
	result = append(result, recent...)
	h.points = result
}

// downsampleBucket reduces a bucket of points to a single representative point.
// Rates (CPU) are averaged. Cumulative counters and gauges (memory, network, block I/O)
// use the last observed value since they represent point-in-time measurements.
func downsampleBucket(ts time.Time, points []HistoryPoint) HistoryPoint {
	merged := make(map[string]ContainerStats)
	counts := make(map[string]float64)

	for _, p := range points {
		for name, s := range p.Stats {
			existing := merged[name]
			// Rates: accumulate for averaging.
			existing.CPUPercent += s.CPUPercent
			existing.CPUThrottled += s.CPUThrottled
			// Gauges and counters: last value wins.
			existing.MemUsage = s.MemUsage
			existing.MemLimit = s.MemLimit
			existing.NetRx = s.NetRx
			existing.NetTx = s.NetTx
			existing.NetRxErrors = s.NetRxErrors
			existing.NetTxErrors = s.NetTxErrors
			existing.BlockRead = s.BlockRead
			existing.BlockWrite = s.BlockWrite
			existing.PIDs = s.PIDs
			merged[name] = existing
			counts[name]++
		}
	}

	// Average the rate-based fields only.
	for name, s := range merged {
		n := counts[name]
		s.CPUPercent /= n
		s.CPUThrottled /= n
		merged[name] = s
	}

	return HistoryPoint{Time: ts, Stats: merged}
}

// Query returns all points within [from, to] as a structured result.
func (h *StatsHistory) Query(from, to time.Time) HistoryResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := HistoryResult{
		Timestamps: make([]int64, 0),
		Containers: make(map[string]ContainerTimeSeries),
	}

	names := make(map[string]bool)
	var filtered []HistoryPoint
	for _, p := range h.points {
		if (p.Time.Equal(from) || p.Time.After(from)) && (p.Time.Equal(to) || p.Time.Before(to)) {
			filtered = append(filtered, p)
			for name := range p.Stats {
				names[name] = true
			}
		}
	}

	for _, p := range filtered {
		result.Timestamps = append(result.Timestamps, p.Time.Unix())
		for name := range names {
			ts := result.Containers[name]
			s := p.Stats[name]
			ts.CPU = append(ts.CPU, s.CPUPercent)
			ts.Mem = append(ts.Mem, s.MemUsage)
			ts.NetRx = append(ts.NetRx, s.NetRx)
			ts.NetTx = append(ts.NetTx, s.NetTx)
			ts.BlockRead = append(ts.BlockRead, s.BlockRead)
			ts.BlockWrite = append(ts.BlockWrite, s.BlockWrite)
			result.Containers[name] = ts
		}
	}

	return result
}

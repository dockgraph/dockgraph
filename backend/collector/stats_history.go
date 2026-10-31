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

// maxHistorySamples bounds the container samples the history holds across
// all its points, about 50 MB. Memory grows with containers × points, so
// with many containers (or nodes reporting many) the oldest points go
// before the retention period ends. A variable so tests can lower it.
var maxHistorySamples = 300_000

// StatsHistory stores timestamped stats snapshots in memory with a maximum
// retention period, within maxHistorySamples.
type StatsHistory struct {
	mu        sync.RWMutex
	points    []HistoryPoint
	samples   int // container samples across points
	retention time.Duration
}

// NewStatsHistory creates a ring buffer that retains data for the given duration.
func NewStatsHistory(retention time.Duration) *StatsHistory {
	return &StatsHistory{
		retention: retention,
		points:    make([]HistoryPoint, 0, 4096),
	}
}

// Record appends a timestamped stats snapshot, dropping the oldest points
// while the history holds more than maxHistorySamples samples.
func (h *StatsHistory) Record(ts time.Time, snap StatsSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.points = append(h.points, HistoryPoint{Time: ts, Stats: snap.Stats})
	h.samples += len(snap.Stats)
	i := 0
	for h.samples > maxHistorySamples && i < len(h.points)-1 {
		h.samples -= len(h.points[i].Stats)
		i++
	}
	h.dropOldest(i)
}

// Evict removes points older than the retention window.
func (h *StatsHistory) Evict() {
	h.mu.Lock()
	defer h.mu.Unlock()
	cutoff := time.Now().Add(-h.retention)
	i := 0
	for i < len(h.points) && h.points[i].Time.Before(cutoff) {
		h.samples -= len(h.points[i].Stats)
		i++
	}
	h.dropOldest(i)
}

// dropOldest removes the first n points. The caller holds h.mu and has
// already taken their samples off h.samples.
func (h *StatsHistory) dropOldest(n int) {
	if n > 0 {
		h.points = append(h.points[:0], h.points[n:]...)
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
	h.samples = 0
	for _, p := range h.points {
		h.samples += len(p.Stats)
	}
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

// maxQueryCells bounds the values a query returns per metric: a series for
// every container across every point returned. Containers that come and go
// (tasks restarting) add series without adding samples to any one point, so
// the product can far exceed maxHistorySamples. A variable so tests can
// lower it.
var maxQueryCells = 120_000

// Query returns the points within [from, to] as a structured result, with
// the series whose names keep accepts (all of them when keep is nil); only
// those count toward maxQueryCells. Past maxQueryCells it returns every n-th
// point instead, the newest included.
func (h *StatsHistory) Query(from, to time.Time, keep func(name string) bool) HistoryResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := HistoryResult{
		Timestamps: make([]int64, 0),
		Containers: make(map[string]ContainerTimeSeries),
	}

	var inRange []HistoryPoint
	for _, p := range h.points {
		if (p.Time.Equal(from) || p.Time.After(from)) && (p.Time.Equal(to) || p.Time.Before(to)) {
			inRange = append(inRange, p)
		}
	}
	var filtered []HistoryPoint
	var names map[string]bool
	for step := 1; ; step *= 2 {
		filtered, names = everyNth(inRange, step, keep)
		if len(names)*len(filtered) <= maxQueryCells || len(filtered) <= 1 {
			break
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

// everyNth returns every step-th point counting back from the newest, in
// order, and the names of the containers they hold that keep accepts (all
// of them when keep is nil).
func everyNth(points []HistoryPoint, step int, keep func(string) bool) ([]HistoryPoint, map[string]bool) {
	var kept []HistoryPoint
	names := make(map[string]bool)
	if len(points) == 0 {
		return kept, names
	}
	for i := (len(points) - 1) % step; i < len(points); i += step {
		kept = append(kept, points[i])
		for name := range points[i].Stats {
			if keep == nil || keep(name) {
				names[name] = true
			}
		}
	}
	return kept, names
}

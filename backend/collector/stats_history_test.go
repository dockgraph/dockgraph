package collector

import (
	"fmt"
	"testing"
	"time"
)

func TestStatsHistory_RecordAndQuery(t *testing.T) {
	h := NewStatsHistory(24 * time.Hour)

	now := time.Now()
	snap := StatsSnapshot{Stats: map[string]ContainerStats{
		"web": {CPUPercent: 25.0, MemUsage: 1000},
	}}
	h.Record(now, snap)

	result := h.Query(now.Add(-time.Minute), now.Add(time.Minute), nil)
	if len(result.Timestamps) != 1 {
		t.Fatalf("expected 1 timestamp, got %d", len(result.Timestamps))
	}
	web, ok := result.Containers["web"]
	if !ok {
		t.Fatal("missing container 'web'")
	}
	if web.CPU[0] != 25.0 {
		t.Errorf("cpu = %f, want 25.0", web.CPU[0])
	}
	if web.Mem[0] != 1000 {
		t.Errorf("mem = %d, want 1000", web.Mem[0])
	}
}

func TestStatsHistory_Expiry(t *testing.T) {
	h := NewStatsHistory(time.Minute)

	old := time.Now().Add(-2 * time.Minute)
	h.Record(old, StatsSnapshot{Stats: map[string]ContainerStats{
		"old": {CPUPercent: 10},
	}})

	recent := time.Now()
	h.Record(recent, StatsSnapshot{Stats: map[string]ContainerStats{
		"new": {CPUPercent: 20},
	}})

	h.Evict()

	result := h.Query(old.Add(-time.Second), recent.Add(time.Second), nil)
	if len(result.Timestamps) != 1 {
		t.Fatalf("expected 1 timestamp after eviction, got %d", len(result.Timestamps))
	}
	if _, ok := result.Containers["old"]; ok {
		t.Error("expected 'old' container to be evicted")
	}
}

// The history holds at most maxHistorySamples container samples overall,
// dropping the oldest points first: with many containers it covers a
// shorter span instead of growing without bound.
func TestStatsHistory_BoundsSamples(t *testing.T) {
	old := maxHistorySamples
	maxHistorySamples = 100
	t.Cleanup(func() { maxHistorySamples = old })

	h := NewStatsHistory(24 * time.Hour)
	base := time.Now().Add(-time.Minute)
	for i := range 30 {
		stats := make(map[string]ContainerStats, 10)
		for c := range 10 {
			stats[string(rune('a'+c))] = ContainerStats{CPUPercent: float64(i)}
		}
		h.Record(base.Add(time.Duration(i)*time.Second), StatsSnapshot{Stats: stats})
	}

	result := h.Query(base.Add(-time.Second), base.Add(time.Minute), nil)
	if n := len(result.Timestamps); n != 10 {
		t.Fatalf("%d points kept, want the newest 10 (100 samples)", n)
	}
	if got := result.Containers["a"].CPU[9]; got != 29 {
		t.Errorf("newest point CPU %v, want 29", got)
	}
	// A single point larger than the bound is still kept.
	huge := make(map[string]ContainerStats, 150)
	for c := range 150 {
		huge[string(rune(0x100+c))] = ContainerStats{}
	}
	h.Record(base.Add(31*time.Second), StatsSnapshot{Stats: huge})
	if n := len(h.Query(base.Add(-time.Second), base.Add(time.Minute), nil).Timestamps); n != 1 {
		t.Errorf("%d points after an oversized one, want 1", n)
	}
}

// A query answers a series per container across every point it returns, so
// its size is containers × points; past maxQueryCells it returns every
// n-th point instead, keeping the newest.
func TestStatsHistory_QueryBoundsCells(t *testing.T) {
	old := maxQueryCells
	maxQueryCells = 1000
	t.Cleanup(func() { maxQueryCells = old })

	h := NewStatsHistory(24 * time.Hour)
	base := time.Now().Add(-time.Hour)
	for i := range 100 {
		// Fresh names on every point, as with tasks that keep restarting.
		stats := map[string]ContainerStats{fmt.Sprint("a", i): {CPUPercent: float64(i)}, fmt.Sprint("b", i): {}}
		h.Record(base.Add(time.Duration(i)*time.Second), StatsSnapshot{Stats: stats})
	}

	result := h.Query(base.Add(-time.Second), base.Add(time.Hour), nil)
	cells := len(result.Containers) * len(result.Timestamps)
	if cells > maxQueryCells {
		t.Errorf("%d series × %d points = %d cells, want at most %d", len(result.Containers), len(result.Timestamps), cells, maxQueryCells)
	}
	if n := len(result.Timestamps); n == 0 || result.Timestamps[n-1] != base.Add(99*time.Second).Unix() {
		t.Errorf("the newest point must be kept")
	}
	for name, s := range result.Containers {
		if len(s.CPU) != len(result.Timestamps) {
			t.Fatalf("series %s has %d values for %d points", name, len(s.CPU), len(result.Timestamps))
		}
	}
}

// A filtered query is thinned by the series it returns, not by those it
// leaves out.
func TestStatsHistory_QueryThinsByKeptSeries(t *testing.T) {
	old := maxQueryCells
	maxQueryCells = 1000
	t.Cleanup(func() { maxQueryCells = old })

	h := NewStatsHistory(24 * time.Hour)
	base := time.Now().Add(-time.Hour)
	for i := range 100 {
		stats := map[string]ContainerStats{"node:a": {}, "node:b": {}}
		for c := range 50 {
			stats[fmt.Sprint("c", c)] = ContainerStats{}
		}
		h.Record(base.Add(time.Duration(i)*time.Second), StatsSnapshot{Stats: stats})
	}

	result := h.Query(base.Add(-time.Second), base.Add(time.Hour), IsNodeSeries)
	if len(result.Containers) != 2 || len(result.Timestamps) != 100 {
		t.Errorf("%d series × %d points, want the 2 node series at all 100 points", len(result.Containers), len(result.Timestamps))
	}
}

func TestStatsHistory_Downsample(t *testing.T) {
	h := NewStatsHistory(24 * time.Hour)

	base := time.Now().Add(-2 * time.Hour)
	for i := 0; i < 60; i++ {
		ts := base.Add(time.Duration(i) * 3 * time.Second)
		h.Record(ts, StatsSnapshot{Stats: map[string]ContainerStats{
			"app": {CPUPercent: float64(i), MemUsage: uint64(i * 1000)},
		}})
	}

	h.Downsample(30 * time.Second)

	result := h.Query(base.Add(-time.Second), base.Add(3*time.Minute+time.Second), nil)
	if len(result.Timestamps) > 10 {
		t.Errorf("expected downsampled to ~6 points, got %d", len(result.Timestamps))
	}
}

func TestStatsHistory_QueryEmptyRange(t *testing.T) {
	h := NewStatsHistory(time.Hour)
	result := h.Query(time.Now(), time.Now().Add(time.Minute), nil)
	if len(result.Timestamps) != 0 {
		t.Errorf("expected 0 timestamps for empty history, got %d", len(result.Timestamps))
	}
}

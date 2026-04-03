package collector

import (
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

	result := h.Query(now.Add(-time.Minute), now.Add(time.Minute))
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

	result := h.Query(old.Add(-time.Second), recent.Add(time.Second))
	if len(result.Timestamps) != 1 {
		t.Fatalf("expected 1 timestamp after eviction, got %d", len(result.Timestamps))
	}
	if _, ok := result.Containers["old"]; ok {
		t.Error("expected 'old' container to be evicted")
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

	result := h.Query(base.Add(-time.Second), base.Add(3*time.Minute+time.Second))
	if len(result.Timestamps) > 10 {
		t.Errorf("expected downsampled to ~6 points, got %d", len(result.Timestamps))
	}
}

func TestStatsHistory_QueryEmptyRange(t *testing.T) {
	h := NewStatsHistory(time.Hour)
	result := h.Query(time.Now(), time.Now().Add(time.Minute))
	if len(result.Timestamps) != 0 {
		t.Errorf("expected 0 timestamps for empty history, got %d", len(result.Timestamps))
	}
}

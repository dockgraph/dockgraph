package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dockgraph/dockgraph/collector"
)

func TestHandleStatsHistory_ValidRange(t *testing.T) {
	h := collector.NewStatsHistory(24 * time.Hour)
	now := time.Now()
	for i := 0; i < 10; i++ {
		h.Record(now.Add(-time.Duration(10-i)*time.Minute), collector.StatsSnapshot{
			Stats: map[string]collector.ContainerStats{
				"web": {CPUPercent: float64(i * 10), MemUsage: uint64(i * 1000)},
			},
		})
	}

	handler := HandleStatsHistory(h)
	req := httptest.NewRequest(http.MethodGet, "/api/stats/history?range=1h", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["range"] != "1h" {
		t.Errorf("range = %v, want 1h", body["range"])
	}
	if body["resolution"] != float64(3) {
		t.Errorf("resolution = %v, want 3", body["resolution"])
	}
}

func TestHandleStatsHistory_LargeRange(t *testing.T) {
	h := collector.NewStatsHistory(24 * time.Hour)
	handler := HandleStatsHistory(h)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/history?range=6h", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	_ = json.NewDecoder(w.Body).Decode(&body)
	if body["resolution"] != float64(30) {
		t.Errorf("resolution = %v, want 30 for 6h range", body["resolution"])
	}
}

func TestHandleStatsHistory_InvalidRange(t *testing.T) {
	h := collector.NewStatsHistory(time.Hour)
	handler := HandleStatsHistory(h)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/history?range=invalid", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleStatsHistory_DefaultRange(t *testing.T) {
	h := collector.NewStatsHistory(time.Hour)
	handler := HandleStatsHistory(h)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/history", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	_ = json.NewDecoder(w.Body).Decode(&body)
	if body["range"] != "1h" {
		t.Errorf("range = %v, want default 1h", body["range"])
	}
}

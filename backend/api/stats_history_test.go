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

	handler := HandleStatsHistory(h, nil, nil)
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
	handler := HandleStatsHistory(h, nil, nil)

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
	handler := HandleStatsHistory(h, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/history?range=invalid", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleStatsHistory_DefaultRange(t *testing.T) {
	h := collector.NewStatsHistory(time.Hour)
	handler := HandleStatsHistory(h, nil, nil)

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

func nodeScopeHistory() *collector.StatsHistory {
	h := collector.NewStatsHistory(24 * time.Hour)
	h.Record(time.Now().Add(-time.Minute), collector.StatsSnapshot{Stats: map[string]collector.ContainerStats{
		"web":         {CPUPercent: 1},
		"shop_api":    {CPUPercent: 2},
		"node:mgr":    {CPUPercent: 3},
		"node:worker": {CPUPercent: 4},
	}})
	return h
}

func historyKeys(t *testing.T, w *httptest.ResponseRecorder) map[string]bool {
	t.Helper()
	var body struct {
		Containers map[string]any `json:"containers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	keys := make(map[string]bool, len(body.Containers))
	for k := range body.Containers {
		keys[k] = true
	}
	return keys
}

func TestHandleStatsHistory_DefaultExcludesNodeSeries(t *testing.T) {
	handler := HandleStatsHistory(nodeScopeHistory(), nil, nil)
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodGet, "/api/stats/history?range=5m", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	keys := historyKeys(t, w)
	if len(keys) != 2 || !keys["web"] || !keys["shop_api"] {
		t.Errorf("default keys = %v, want web and shop_api", keys)
	}
}

func TestHandleStatsHistory_ScopeNodes(t *testing.T) {
	handler := HandleStatsHistory(nodeScopeHistory(), nil, nil)
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodGet, "/api/stats/history?range=5m&scope=nodes", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	keys := historyKeys(t, w)
	if len(keys) != 2 || !keys["node:mgr"] || !keys["node:worker"] {
		t.Errorf("scope=nodes keys = %v, want node:mgr and node:worker", keys)
	}
}

func TestHandleStatsHistory_ScopeErrors(t *testing.T) {
	handler := HandleStatsHistory(nodeScopeHistory(), nil, nil)
	for _, q := range []string{"scope=nodes&stack=shop", "scope=bogus", "scope=containers"} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodGet, "/api/stats/history?"+q, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, w.Code)
		}
	}
}

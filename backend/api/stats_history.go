package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dockgraph/dockgraph/collector"
)

var validRanges = map[string]time.Duration{
	"5m":  5 * time.Minute,
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
}

// HandleStatsHistory returns a handler for GET /api/stats/history?range={5m|1h|6h|24h}.
func HandleStatsHistory(history *collector.StatsHistory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rangeStr := r.URL.Query().Get("range")
		if rangeStr == "" {
			rangeStr = "1h"
		}

		dur, ok := validRanges[rangeStr]
		if !ok {
			jsonError(w, "invalid range: use 5m, 1h, 6h, or 24h", http.StatusBadRequest)
			return
		}

		now := time.Now()
		result := history.Query(now.Add(-dur), now)

		resolution := 3
		if dur > time.Hour {
			resolution = 30
		}

		resp := map[string]any{
			"range":      rangeStr,
			"resolution": resolution,
			"timestamps": result.Timestamps,
			"containers": result.Containers,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

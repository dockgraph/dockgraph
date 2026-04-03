package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/dockgraph/dockgraph/collector"
)

// HandleRecentEvents returns a handler for GET /api/events/recent?limit={n}.
func HandleRecentEvents(history *collector.EventHistory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if q := r.URL.Query().Get("limit"); q != "" {
			if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}

		events := history.Recent(limit)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"events": events,
		})
	}
}

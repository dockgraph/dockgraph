package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dockgraph/dockgraph/collector"
)

func TestHandleRecentEvents(t *testing.T) {
	h := collector.NewEventHistory(100)
	h.Add(collector.DockerEvent{
		Timestamp: time.Now(),
		Action:    "start",
		Type:      "container",
		Name:      "web",
	})
	h.Add(collector.DockerEvent{
		Timestamp: time.Now(),
		Action:    "stop",
		Type:      "container",
		Name:      "db",
	})

	handler := HandleRecentEvents(h)
	req := httptest.NewRequest(http.MethodGet, "/api/events/recent?limit=10", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	events, ok := body["events"].([]any)
	if !ok {
		t.Fatal("missing events array")
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestHandleRecentEvents_DefaultLimit(t *testing.T) {
	h := collector.NewEventHistory(100)
	handler := HandleRecentEvents(h)

	req := httptest.NewRequest(http.MethodGet, "/api/events/recent", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleRecentEvents_ReverseChrono(t *testing.T) {
	h := collector.NewEventHistory(100)
	h.Add(collector.DockerEvent{Timestamp: time.Now(), Action: "first", Type: "container", Name: "a"})
	h.Add(collector.DockerEvent{Timestamp: time.Now(), Action: "second", Type: "container", Name: "b"})

	handler := HandleRecentEvents(h)
	req := httptest.NewRequest(http.MethodGet, "/api/events/recent", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	var body struct {
		Events []collector.DockerEvent `json:"events"`
	}
	_ = json.NewDecoder(w.Body).Decode(&body)

	if len(body.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(body.Events))
	}
	if body.Events[0].Action != "second" {
		t.Errorf("first event action = %q, want 'second' (most recent)", body.Events[0].Action)
	}
}

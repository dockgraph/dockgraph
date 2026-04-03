package collector

import (
	"testing"
	"time"
)

func TestEventHistory_AddAndRecent(t *testing.T) {
	h := NewEventHistory(100)

	h.Add(DockerEvent{
		Timestamp: time.Now(),
		Action:    "start",
		Type:      "container",
		Name:      "web",
	})
	h.Add(DockerEvent{
		Timestamp: time.Now(),
		Action:    "connect",
		Type:      "network",
		Name:      "app-net",
	})

	events := h.Recent(10)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "app-net" {
		t.Errorf("first event name = %q, want %q", events[0].Name, "app-net")
	}
}

func TestEventHistory_LimitEnforced(t *testing.T) {
	h := NewEventHistory(5)

	for i := 0; i < 10; i++ {
		h.Add(DockerEvent{
			Timestamp: time.Now(),
			Action:    "start",
			Type:      "container",
			Name:      "c",
		})
	}

	events := h.Recent(100)
	if len(events) != 5 {
		t.Fatalf("expected 5 events (capacity), got %d", len(events))
	}
}

func TestEventHistory_RecentLimit(t *testing.T) {
	h := NewEventHistory(100)

	for i := 0; i < 20; i++ {
		h.Add(DockerEvent{
			Timestamp: time.Now(),
			Action:    "start",
			Type:      "container",
			Name:      "c",
		})
	}

	events := h.Recent(5)
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

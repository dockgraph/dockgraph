package collector

import (
	"sync"
	"time"
)

// DockerEvent is a simplified Docker event stored in the history buffer.
type DockerEvent struct {
	Timestamp  time.Time         `json:"timestamp"`
	Action     string            `json:"action"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// EventHistory is a fixed-capacity circular buffer of recent Docker events.
// Add is O(1). Recent returns events in reverse chronological order.
type EventHistory struct {
	mu       sync.RWMutex
	buf      []DockerEvent
	capacity int
	head     int // next write position
	count    int // number of elements stored
}

// NewEventHistory creates a buffer that holds at most capacity events.
func NewEventHistory(capacity int) *EventHistory {
	return &EventHistory{
		buf:      make([]DockerEvent, capacity),
		capacity: capacity,
	}
}

// Add appends an event in O(1), overwriting the oldest if at capacity.
func (h *EventHistory) Add(e DockerEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buf[h.head] = e
	h.head = (h.head + 1) % h.capacity
	if h.count < h.capacity {
		h.count++
	}
}

// Recent returns the last n events in reverse chronological order.
func (h *EventHistory) Recent(n int) []DockerEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n > h.count {
		n = h.count
	}

	result := make([]DockerEvent, n)
	for i := 0; i < n; i++ {
		// Walk backwards from the most recent entry.
		idx := (h.head - 1 - i + h.capacity) % h.capacity
		result[i] = h.buf[idx]
	}
	return result
}

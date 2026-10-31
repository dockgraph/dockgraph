package collector

import (
	"sync"
	"time"

	"github.com/dockgraph/dockgraph/secrets"
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

// eventAttributes are the event attributes the history keeps. Docker also
// attaches every label of a container to its events; those are dropped, as
// whoever runs a container controls their number and size, and they are
// where credentials turn up.
var eventAttributes = []string{"name", "image", "exitCode", "signal", "driver", "type", "container"}

// Add appends an event in O(1), overwriting the oldest if at capacity. It
// keeps only eventAttributes, each cut to MaxNameBytes like the name, so an
// event costs a bounded amount whatever its container carries. The values
// kept are masked here, once, and never held in clear.
func (h *EventHistory) Add(e DockerEvent) {
	e.Name = CapText(e.Name, MaxNameBytes)
	e.Attributes = keptAttributes(e.Attributes)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buf[h.head] = e
	h.head = (h.head + 1) % h.capacity
	if h.count < h.capacity {
		h.count++
	}
}

// keptAttributes returns the eventAttributes of attrs, values cut to
// MaxNameBytes and masked, or nil when attrs has none of them.
func keptAttributes(attrs map[string]string) map[string]string {
	var kept map[string]string
	for _, k := range eventAttributes {
		v, ok := attrs[k]
		if !ok {
			continue
		}
		if kept == nil {
			kept = make(map[string]string, len(eventAttributes))
		}
		kept[k] = secrets.MaskEnvValue(k, CapText(v, MaxNameBytes))
	}
	return kept
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

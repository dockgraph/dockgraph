package collector

import "time"

// debouncer coalesces rapid triggers into a single notification after a quiet period.
// Designed for event-driven polling where bursts of events (e.g. docker-compose up)
// should result in a single poll rather than one per event.
type debouncer struct {
	delay  time.Duration
	timer  *time.Timer
	notify chan struct{}
}

// newDebouncer creates a debouncer with the given quiet period.
func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{
		delay:  delay,
		notify: make(chan struct{}, 1),
	}
}

// trigger resets the debounce timer. When the timer fires, a single
// notification is sent on the notify channel (non-blocking).
func (d *debouncer) trigger() {
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, func() {
		select {
		case d.notify <- struct{}{}:
		default:
		}
	})
}

// stop cancels any pending debounce timer.
func (d *debouncer) stop() {
	if d.timer != nil {
		d.timer.Stop()
	}
}

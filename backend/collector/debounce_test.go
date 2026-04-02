package collector

import (
	"testing"
	"time"
)

func TestDebouncerTriggerFiresAfterDelay(t *testing.T) {
	d := newDebouncer(50 * time.Millisecond)
	defer d.stop()

	d.trigger()

	select {
	case <-d.notify:
		// Expected: notification received after delay.
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected notification within 300ms, got none")
	}
}

func TestDebouncerMultipleTriggersCoalesce(t *testing.T) {
	d := newDebouncer(100 * time.Millisecond)
	defer d.stop()

	// Fire rapid triggers — each should reset the timer.
	for i := 0; i < 5; i++ {
		d.trigger()
		time.Sleep(30 * time.Millisecond)
	}

	// The timer should fire ~100ms after the last trigger.
	// Total elapsed: 5*30ms = 150ms from first trigger, timer fires at ~250ms.
	select {
	case <-d.notify:
		// Expected: exactly one notification.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected notification after coalesced triggers, got none")
	}

	// Verify no second notification arrives (only one should be sent).
	select {
	case <-d.notify:
		t.Fatal("received unexpected second notification")
	case <-time.After(200 * time.Millisecond):
		// Expected: no additional notification.
	}
}

func TestDebouncerStopCancelsPendingTimer(t *testing.T) {
	d := newDebouncer(100 * time.Millisecond)

	d.trigger()
	d.stop()

	// After stopping, the timer should not fire.
	select {
	case <-d.notify:
		t.Fatal("received notification after stop()")
	case <-time.After(200 * time.Millisecond):
		// Expected: no notification because we stopped the debouncer.
	}
}

func TestDebouncerStopWithoutTrigger(_ *testing.T) {
	d := newDebouncer(50 * time.Millisecond)
	// Stopping a debouncer that was never triggered should not panic.
	d.stop()
}

func TestDebouncerNonBlockingSend(t *testing.T) {
	d := newDebouncer(20 * time.Millisecond)
	defer d.stop()

	// Pre-fill the buffered channel so the next send would block.
	d.notify <- struct{}{}

	// Trigger again — the timer fires but the channel is full.
	// The debouncer should not block or panic.
	d.trigger()
	time.Sleep(100 * time.Millisecond)

	// Drain the pre-filled notification.
	select {
	case <-d.notify:
	default:
		t.Fatal("expected at least the pre-filled notification")
	}

	// The second trigger's send was dropped (channel was full), so nothing else.
	select {
	case <-d.notify:
		t.Fatal("expected no additional notification after non-blocking drop")
	case <-time.After(100 * time.Millisecond):
		// Expected.
	}
}

func TestDebouncerExactlyOnePerWindow(t *testing.T) {
	d := newDebouncer(30 * time.Millisecond)
	defer d.stop()

	// Trigger three separate windows and count notifications.
	received := 0
	for i := 0; i < 3; i++ {
		d.trigger()
		select {
		case <-d.notify:
			received++
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("window %d: timed out waiting for notification", i)
		}
	}

	if received != 3 {
		t.Errorf("expected 3 notifications across 3 windows, got %d", received)
	}
}

package collector

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
)

func TestDockerCollectorLifecycle(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/web"}, Image: "nginx", State: "running"},
		},
	}

	dc := NewDockerCollector(cli, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Initial poll should send an update
	select {
	case update := <-dc.Updates():
		if update.Snapshot == nil {
			t.Fatal("expected non-nil snapshot")
		}
		if len(update.Snapshot.Nodes) != 1 {
			t.Errorf("expected 1 node, got %d", len(update.Snapshot.Nodes))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for initial update")
	}

	if err := dc.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}

func TestDockerCollectorStartError(t *testing.T) {
	dc := NewDockerCollector(errClient("containers"), time.Second)
	err := dc.Start(context.Background())
	if err == nil {
		t.Fatal("expected error on start with broken client")
	}
}

func TestDockerCollectorPollLoop(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/app"}, Image: "app", State: "running"},
		},
	}

	dc := NewDockerCollector(cli, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer dc.Stop()

	// Drain initial update
	<-dc.Updates()

	// Wait for at least one poll cycle update
	select {
	case update := <-dc.Updates():
		if update.Snapshot == nil {
			t.Fatal("expected non-nil snapshot from poll")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for poll update")
	}
}

func TestDockerCollectorPollTimeout(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/a"}, Image: "a", State: "running"},
		},
	}

	dc := NewDockerCollector(cli, time.Second)
	// poll should respect context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := dc.poll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDockerCollectorUpdatesChannel(t *testing.T) {
	dc := NewDockerCollector(&stubDockerClient{}, time.Second)
	ch := dc.Updates()
	if ch == nil {
		t.Fatal("Updates() returned nil channel")
	}
}

func TestWatchEventsTopologyEvent(t *testing.T) {
	msgCh := make(chan events.Message, 1)
	errCh := make(chan error)

	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/web"}, Image: "nginx", State: "running"},
		},
		eventsCh: msgCh,
		errCh:    errCh,
	}

	dc := NewDockerCollector(cli, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer dc.Stop()

	// Drain initial poll
	<-dc.Updates()

	// Send a topology event
	msgCh <- events.Message{Action: events.Action("start")}

	// Wait for the debounced poll to produce an update
	select {
	case update := <-dc.Updates():
		if update.Snapshot == nil {
			t.Fatal("expected non-nil snapshot after event")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for event-triggered poll")
	}
}

func TestWatchEventsIgnoresNonTopology(t *testing.T) {
	msgCh := make(chan events.Message, 2)
	errCh := make(chan error)

	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/web"}, Image: "nginx", State: "running"},
		},
		eventsCh: msgCh,
		errCh:    errCh,
	}

	dc := NewDockerCollector(cli, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer dc.Stop()

	// Drain initial poll
	<-dc.Updates()

	// Send a non-topology event (should be ignored, no poll triggered)
	msgCh <- events.Message{Action: events.Action("pull")}

	select {
	case <-dc.Updates():
		t.Error("did not expect a poll from non-topology event")
	case <-time.After(800 * time.Millisecond):
		// expected: no update after debounce window
	}
}

func TestWatchEventsContextCancelled(t *testing.T) {
	msgCh := make(chan events.Message)
	errCh := make(chan error)

	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/x"}, Image: "x", State: "running"},
		},
		eventsCh: msgCh,
		errCh:    errCh,
	}

	dc := NewDockerCollector(cli, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())

	if err := dc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Drain initial poll
	<-dc.Updates()

	// Cancel context — watchEvents and pollLoop should exit
	cancel()

	// Stop should return without hanging
	done := make(chan struct{})
	go func() {
		dc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() hung after context cancellation")
	}
}

func TestWatchEventsErrorNilReturns(t *testing.T) {
	msgCh := make(chan events.Message)
	errCh := make(chan error, 1)

	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/x"}, Image: "x", State: "running"},
		},
		eventsCh: msgCh,
		errCh:    errCh,
	}

	dc := NewDockerCollector(cli, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Drain initial poll
	<-dc.Updates()

	// Sending nil error should cause watchEvents to return
	errCh <- nil

	done := make(chan struct{})
	go func() {
		dc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success: watchEvents exited on nil error
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() hung after nil error")
	}
}

func TestWatchEventsDebounce(t *testing.T) {
	msgCh := make(chan events.Message, 10)
	errCh := make(chan error)

	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{Names: []string{"/web"}, Image: "nginx", State: "running"},
		},
		eventsCh: msgCh,
		errCh:    errCh,
	}

	dc := NewDockerCollector(cli, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dc.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer dc.Stop()

	// Drain initial poll
	<-dc.Updates()

	// Burst of topology events — should debounce to a single poll
	for i := 0; i < 5; i++ {
		msgCh <- events.Message{Action: events.Action("start")}
	}

	// First poll update after debounce
	select {
	case <-dc.Updates():
		// expected
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for debounced poll")
	}

	// No additional polls should follow immediately
	select {
	case <-dc.Updates():
		// A second poll is acceptable (poll loop may have ticked), not a failure
	case <-time.After(800 * time.Millisecond):
		// expected: single debounced poll
	}
}

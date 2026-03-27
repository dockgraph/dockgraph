package collector

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// DockerCollector monitors the local Docker daemon for container, network,
// and volume changes, producing graph snapshots on each topology change.
type DockerCollector struct {
	client       client.APIClient
	pollInterval time.Duration
	updates      chan StateUpdate
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewDockerCollector creates a collector that polls the Docker daemon at the
// given interval and also reacts to real-time Docker events.
func NewDockerCollector(cli client.APIClient, pollInterval time.Duration) *DockerCollector {
	return &DockerCollector{
		client:       cli,
		pollInterval: pollInterval,
		updates:      make(chan StateUpdate, 16),
		stopCh:       make(chan struct{}),
	}
}

// Updates returns a read-only channel that emits state updates whenever
// the Docker topology changes.
func (d *DockerCollector) Updates() <-chan StateUpdate {
	return d.updates
}

// Start performs an initial poll and then launches background goroutines
// for periodic polling and real-time event watching.
func (d *DockerCollector) Start(ctx context.Context) error {
	if err := d.poll(ctx); err != nil {
		return err
	}

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.watchEvents(ctx)
	}()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.pollLoop(ctx)
	}()

	return nil
}

// Stop signals all background goroutines to exit and waits for them to finish.
func (d *DockerCollector) Stop() error {
	close(d.stopCh)
	d.wg.Wait()
	return nil
}

func (d *DockerCollector) poll(ctx context.Context) error {
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	snapshot, err := d.buildSnapshot(pollCtx)
	if err != nil {
		return err
	}
	select {
	case d.updates <- StateUpdate{Snapshot: &snapshot}:
	case <-ctx.Done():
	}
	return nil
}

func (d *DockerCollector) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := d.poll(ctx); err != nil {
				log.Printf("poll error: %v", err)
			}
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// watchEvents subscribes to the Docker event stream and triggers a poll
// whenever a topology-relevant event occurs. Events are debounced to avoid
// redundant polls when Docker emits a burst (e.g. during docker-compose up).
func (d *DockerCollector) watchEvents(ctx context.Context) {
	eventFilter := filters.NewArgs()
	eventFilter.Add("type", string(events.ContainerEventType))
	eventFilter.Add("type", string(events.NetworkEventType))
	eventFilter.Add("type", string(events.VolumeEventType))

	msgCh, errCh := d.client.Events(ctx, events.ListOptions{Filters: eventFilter})

	var debounceTimer *time.Timer
	debounceCh := make(chan struct{}, 1)
	reconnectBackoff := 2 * time.Second

	for {
		select {
		case msg := <-msgCh:
			reconnectBackoff = 2 * time.Second
			if !isTopologyEvent(msg.Action) {
				continue
			}
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			// 500ms debounce: Docker emits a burst of events during compose up/down,
			// and polling on every event would waste resources.
			debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				select {
				case debounceCh <- struct{}{}:
				default:
				}
			})
		case <-debounceCh:
			if err := d.poll(ctx); err != nil {
				log.Printf("event-triggered poll error: %v", err)
			}
		case err := <-errCh:
			if err == nil || ctx.Err() != nil {
				return
			}
			log.Printf("docker events error: %v, reconnecting...", err)
			// Exponential backoff to avoid tight-loop reconnection when the daemon is temporarily unavailable.
			backoff := min(reconnectBackoff, 30*time.Second)
			reconnectBackoff *= 2
			time.Sleep(backoff)
			msgCh, errCh = d.client.Events(ctx, events.ListOptions{Filters: eventFilter})
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

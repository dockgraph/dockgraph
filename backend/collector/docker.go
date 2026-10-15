package collector

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
)

// DockerCollector monitors the local Docker daemon for container, network,
// and volume changes, producing graph snapshots on each topology change.
// In swarm mode it instead models the whole cluster as seen from a manager:
// services become nodes and task placement is tracked by polling.
type DockerCollector struct {
	client       DockerClient
	pollInterval time.Duration
	updates      chan StateUpdate
	stopCh       chan struct{}
	wg           sync.WaitGroup

	swarm            bool
	taskPollInterval time.Duration

	// pollMu serializes full polls. The periodic, event and task-poll
	// triggers run in separate goroutines; without it a slower, older
	// snapshot could be published after a newer one and roll state back.
	pollMu sync.Mutex

	// fpMu guards the fingerprint of the task set last snapshotted, compared
	// by the task poller to detect remote task changes.
	fpMu   sync.Mutex
	taskFP uint64
	hasFP  bool
}

// NewDockerCollector creates a collector that polls the Docker daemon at the
// given interval and also reacts to real-time Docker events.
func NewDockerCollector(cli DockerClient, pollInterval time.Duration) *DockerCollector {
	return &DockerCollector{
		client:       cli,
		pollInterval: pollInterval,
		updates:      make(chan StateUpdate, 16),
		stopCh:       make(chan struct{}),
	}
}

// EnableSwarm switches the collector to swarm mode: snapshots include swarm
// services, tasks and nodes, service/node events are watched, and the task
// list is polled at taskPollInterval to catch changes on remote nodes. Must be
// called before Start.
func (d *DockerCollector) EnableSwarm(taskPollInterval time.Duration) {
	d.swarm = true
	d.taskPollInterval = taskPollInterval
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

	if d.swarm && d.taskPollInterval > 0 {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			d.taskPollLoop(ctx)
		}()
	}

	return nil
}

// Stop signals all background goroutines to exit and waits for them to finish.
func (d *DockerCollector) Stop() error {
	close(d.stopCh)
	d.wg.Wait()
	return nil
}

// poll builds a snapshot and publishes it. Polls never overlap, so snapshots
// are published in the order they were observed.
func (d *DockerCollector) poll(ctx context.Context) error {
	d.pollMu.Lock()
	defer d.pollMu.Unlock()
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	snapshot, err := d.snapshot(pollCtx)
	if err != nil {
		return err
	}
	select {
	case d.updates <- StateUpdate{Snapshot: &snapshot}:
	case <-ctx.Done():
	}
	return nil
}

// snapshot builds the graph for the collector's mode.
func (d *DockerCollector) snapshot(ctx context.Context) (GraphSnapshot, error) {
	if !d.swarm {
		return d.buildSnapshot(ctx)
	}
	res, err := fetchSwarmResources(ctx, d.client)
	if err != nil {
		return GraphSnapshot{}, err
	}
	d.storeTaskFingerprint(taskFingerprint(res.tasks, res.nodes))
	return buildSwarmSnapshot(res), nil
}

// storeTaskFingerprint records the fingerprint of the task set last snapshotted.
func (d *DockerCollector) storeTaskFingerprint(fp uint64) {
	d.fpMu.Lock()
	defer d.fpMu.Unlock()
	d.taskFP = fp
	d.hasFP = true
}

// taskSetChanged reports whether fp differs from the last snapshotted task set.
func (d *DockerCollector) taskSetChanged(fp uint64) bool {
	d.fpMu.Lock()
	defer d.fpMu.Unlock()
	return !d.hasFP || d.taskFP != fp
}

// taskPollLoop periodically lists swarm tasks and nodes and re-snapshots only
// when either set changed. Task transitions on remote nodes emit no events on the
// manager, so this is how rescheduling and scaling become visible promptly.
func (d *DockerCollector) taskPollLoop(ctx context.Context) {
	ticker := time.NewTicker(d.taskPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.checkTasks(ctx)
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkTasks runs one task-poll iteration.
func (d *DockerCollector) checkTasks(ctx context.Context) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tasks, err := d.client.TaskList(callCtx, swarm.TaskListOptions{})
	if err != nil {
		log.Printf("task poll error: %v", err)
		return
	}
	nodes, err := d.client.NodeList(callCtx, swarm.NodeListOptions{})
	if err != nil {
		log.Printf("node poll error: %v", err)
		return
	}
	if !d.taskSetChanged(taskFingerprint(tasks, nodes)) {
		return
	}
	if err := d.poll(ctx); err != nil {
		log.Printf("task-triggered poll error: %v", err)
	}
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

// topologyEventFilter selects the Docker event types that can change the
// graph. Swarm mode adds service and node events, which the manager emits
// cluster-wide.
func topologyEventFilter(swarmMode bool) filters.Args {
	f := filters.NewArgs()
	f.Add("type", string(events.ContainerEventType))
	f.Add("type", string(events.NetworkEventType))
	f.Add("type", string(events.VolumeEventType))
	if swarmMode {
		f.Add("type", string(events.ServiceEventType))
		f.Add("type", string(events.NodeEventType))
	}
	return f
}

// watchEvents subscribes to the Docker event stream and triggers a poll
// whenever a topology-relevant event occurs. Events are debounced to avoid
// redundant polls when Docker emits a burst (e.g. during docker-compose up).
func (d *DockerCollector) watchEvents(ctx context.Context) {
	eventFilter := topologyEventFilter(d.swarm)

	msgCh, errCh := d.client.Events(ctx, events.ListOptions{Filters: eventFilter})

	db := newDebouncer(500 * time.Millisecond)
	defer db.stop()
	reconnectBackoff := 2 * time.Second

	for {
		select {
		case msg := <-msgCh:
			reconnectBackoff = 2 * time.Second
			if !isTopologyEvent(msg.Action) {
				continue
			}
			db.trigger()
		case <-db.notify:
			if err := d.poll(ctx); err != nil {
				log.Printf("event-triggered poll error: %v", err)
			}
		case err := <-errCh:
			if err == nil || ctx.Err() != nil {
				return
			}
			log.Printf("docker events error: %v, reconnecting...", err)
			db.stop()
			// Exponential backoff to avoid tight-loop reconnection when the daemon is temporarily unavailable.
			backoff := min(reconnectBackoff, 30*time.Second)
			reconnectBackoff *= 2
			select {
			case <-time.After(backoff):
			case <-d.stopCh:
				return
			case <-ctx.Done():
				return
			}
			msgCh, errCh = d.client.Events(ctx, events.ListOptions{Filters: eventFilter})
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

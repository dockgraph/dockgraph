package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/dockgraph/dockgraph/collector"
)

// selfExcludeValue is the label value marking dockgraph's own container.
const selfExcludeValue = "true"

// aggregateLine is one log line tagged with its source container.
type aggregateLine struct {
	Container string `json:"container"`
	Stream    string `json:"stream"`
	Line      string `json:"line"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ContainerLister lists containers (used to enumerate log sources).
type ContainerLister interface {
	ContainerList(ctx context.Context, options containertypes.ListOptions) ([]containertypes.Summary, error)
}

// EventSubscriber streams Docker events (used to add/remove live followers).
type EventSubscriber interface {
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
}

// containerDisplayName returns the human name without Docker's leading slash.
func containerDisplayName(c containertypes.Summary) string {
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	if len(c.ID) >= 12 {
		return c.ID[:12]
	}
	return c.ID
}

// isSelf reports whether a container is dockgraph's own (excluded from logs).
func isSelf(labels map[string]string) bool {
	return labels[collector.SelfExcludeLabel] == selfExcludeValue
}

// collectHistory fetches up to `limit` recent lines (before `before`, if set) from
// every non-self container, tags each with its container, and merge-sorts ascending
// by timestamp. One container failing is skipped (best-effort), not fatal.
func collectHistory(ctx context.Context, lister ContainerLister, logger ContainerLogger, before string, limit int) ([]aggregateLine, error) {
	summaries, err := lister.ContainerList(ctx, containertypes.ListOptions{})
	if err != nil {
		return nil, err
	}

	var all []aggregateLine
	for _, c := range summaries {
		if isSelf(c.Labels) {
			continue
		}
		opts := containertypes.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Timestamps: true,
			Tail:       strconv.Itoa(limit),
		}
		if before != "" {
			opts.Until = before
		}
		rc, err := logger.ContainerLogs(ctx, c.ID, opts)
		if err != nil {
			continue
		}
		name := containerDisplayName(c)
		for _, e := range readLogLines(rc, limit) {
			all = append(all, aggregateLine{Container: name, Stream: e.Stream, Line: e.Line, Timestamp: e.Timestamp})
		}
		rc.Close()
	}

	// RFC3339Nano UTC timestamps sort correctly as strings.
	sort.SliceStable(all, func(i, j int) bool { return all[i].Timestamp < all[j].Timestamp })
	return all, nil
}

// HandleAggregateLogsHistory returns a handler for GET /api/logs/history.
// Returns a merged, time-sorted JSON page of log lines across all containers.
func HandleAggregateLogsHistory(lister ContainerLister, logger ContainerLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 200
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 1000 {
				limit = n
			}
		}
		before := r.URL.Query().Get("before")

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		lines, err := collectHistory(ctx, lister, logger, before, limit)
		if err != nil {
			jsonError(w, "failed to read logs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"lines": lines})
	}
}

// logAggregator fans multiple per-container log followers into one channel.
type logAggregator struct {
	logger    ContainerLogger
	since     string
	out       chan aggregateLine
	mu        sync.Mutex
	followers map[string]context.CancelFunc
}

func newLogAggregator(logger ContainerLogger, since string) *logAggregator {
	return &logAggregator{
		logger:    logger,
		since:     since,
		out:       make(chan aggregateLine, 256),
		followers: make(map[string]context.CancelFunc),
	}
}

// add starts a follower for a container (no-op if already followed).
func (a *logAggregator) add(parent context.Context, id, name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.followers[id]; ok {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	a.followers[id] = cancel
	go a.follow(ctx, id, name)
}

// remove cancels and forgets a container's follower.
func (a *logAggregator) remove(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if cancel, ok := a.followers[id]; ok {
		cancel()
		delete(a.followers, id)
	}
}

// follow streams one container's logs, tagging and forwarding each line.
// A read error (container gone, cancelled) ends the follower without affecting others.
func (a *logAggregator) follow(ctx context.Context, id, name string) {
	opts := containertypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       "0",
	}
	if a.since != "" {
		opts.Since = a.since
	}
	rc, err := a.logger.ContainerLogs(ctx, id, opts)
	if err != nil {
		return
	}
	defer rc.Close()

	dlr := &dockerLogReader{reader: rc}
	for {
		streamType, payload, err := dlr.next()
		if err != nil {
			return
		}
		scanner := bufio.NewScanner(bytes.NewReader(payload))
		for scanner.Scan() {
			e := parseLogEntry(streamType, scanner.Text())
			select {
			case a.out <- aggregateLine{Container: name, Stream: e.Stream, Line: e.Line, Timestamp: e.Timestamp}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// containerEventsFilter limits the event stream to container lifecycle events.
func containerEventsFilter() filters.Args {
	f := filters.NewArgs()
	f.Add("type", "container")
	return f
}

// HandleAggregateLogs returns a handler for GET /api/logs.
// Streams every non-self container's logs as merged SSE, adding/removing
// followers as containers start/die.
func HandleAggregateLogs(lister ContainerLister, logger ContainerLogger, eventsSub EventSubscriber) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		flusher.Flush()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		agg := newLogAggregator(logger, r.URL.Query().Get("since"))

		// Seed followers from the currently running, non-self containers.
		if summaries, err := lister.ContainerList(ctx, containertypes.ListOptions{}); err == nil {
			for _, c := range summaries {
				if isSelf(c.Labels) || c.State != stateRunning {
					continue
				}
				agg.add(ctx, c.ID, containerDisplayName(c))
			}
		}

		// React to container lifecycle: add followers on start, remove on stop/die.
		msgCh, errCh := eventsSub.Events(ctx, events.ListOptions{Filters: containerEventsFilter()})
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case err := <-errCh:
					if err != nil {
						return
					}
				case msg, ok := <-msgCh:
					if !ok {
						return
					}
					switch string(msg.Action) {
					case "start":
						agg.add(ctx, msg.Actor.ID, msg.Actor.Attributes["name"])
					case "die", "destroy", "stop", "kill":
						agg.remove(msg.Actor.ID)
					}
				}
			}
		}()

		// Writer loop: drain merged lines to SSE until the client disconnects.
		for {
			select {
			case <-ctx.Done():
				return
			case line := <-agg.out:
				data, _ := json.Marshal(line)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

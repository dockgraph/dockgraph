package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
)

const (
	streamStdout = "stdout"
	streamStderr = "stderr"
)

// ContainerLogger is the subset of the Docker API needed for container logs.
type ContainerLogger interface {
	ContainerLogs(ctx context.Context, containerID string, options containertypes.LogsOptions) (io.ReadCloser, error)
}

// logEntry is a single parsed log line.
type logEntry struct {
	Stream    string `json:"stream"`
	Line      string `json:"line"`
	Timestamp string `json:"timestamp,omitempty"`
}

// logOpener opens a Docker log stream (multiplexed or raw, see
// logLineReader) for one container or service; ContainerLogs and
// ServiceLogs share this shape. Callers must request timestamps, which
// logLineReader relies on.
type logOpener func(ctx context.Context, id string, options containertypes.LogsOptions) (io.ReadCloser, error)

// HandleContainerLogsHistory returns a handler for GET /api/containers/{id}/logs/history.
// Returns a paginated JSON array of log lines, newest last.
func HandleContainerLogsHistory(logger ContainerLogger) http.HandlerFunc {
	return handleLogsHistory("container", logger.ContainerLogs)
}

// HandleContainerLogs returns a handler for GET /api/containers/{id}/logs.
// Streams container logs as Server-Sent Events.
// Accepts optional `since` query param to only stream lines after a timestamp.
func HandleContainerLogs(logger ContainerLogger) http.HandlerFunc {
	return handleLogsStream("container", logger.ContainerLogs)
}

// handleLogsHistory serves a paginated JSON page of a resource's log lines.
// kind ("container" or "service") only shapes error messages.
func handleLogsHistory(kind string, open logOpener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validResourceName.MatchString(id) {
			jsonError(w, "invalid "+kind+" ID", http.StatusBadRequest)
			return
		}

		limit := 200
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 1000 {
				limit = n
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		lines, err := fetchLogHistory(ctx, open, id, kind == "service", r.URL.Query().Get("before"), limit)
		if err != nil {
			log.Printf("%s logs history %s: %v", kind, id, err)
			jsonError(w, "failed to read logs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"lines": lines,
		})
	}
}

// handleLogsStream streams a resource's logs as Server-Sent Events.
// kind ("container" or "service") only shapes error messages.
func handleLogsStream(kind string, open logOpener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validResourceName.MatchString(id) {
			jsonError(w, "invalid "+kind+" ID", http.StatusBadRequest)
			return
		}

		opts := containertypes.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: true,
		}

		if since := r.URL.Query().Get("since"); since != "" {
			// Resume after the client's newest line, replaying what was
			// logged since (see resumeTailLines).
			opts.Since = afterTimestamp(since)
			opts.Tail = strconv.Itoa(resumeTailLines)
		} else {
			tail := "100"
			if v := r.URL.Query().Get("tail"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 1000 {
					tail = strconv.Itoa(n)
				}
			}
			opts.Tail = tail
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		reader, err := open(ctx, id, opts)
		if err != nil {
			log.Printf("%s logs %s: %v", kind, id, err)
			jsonError(w, "failed to open logs", http.StatusInternalServerError)
			return
		}
		defer reader.Close()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		if _, ok := w.(http.Flusher); !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		sw := newStreamWriter(w)

		// Flush headers immediately so the EventSource client sees the
		// connection as open even when no log lines have arrived yet.
		sw.Flush()

		streamDockerLogs(sw, reader)
	}
}

// maxHistoryPageBytes bounds the line text of one page of log history, so a
// page of long lines costs no more memory than one of short lines. The
// newest lines are kept, so paging back with the oldest line's timestamp
// misses nothing. A variable so tests can lower it.
var maxHistoryPageBytes = 512 << 10

// readLogLines parses a Docker log stream into at most limit log entries,
// the newest of which fit in maxHistoryPageBytes.
func readLogLines(reader io.Reader, limit int) []logEntry {
	lr := newLogLineReader(reader)
	var lines []logEntry
	size := 0
	for n := 0; n < limit; n++ {
		stream, line, err := lr.next()
		if err != nil {
			logStreamError(err)
			break
		}
		e := parseLogEntry(stream, line)
		lines = append(lines, e)
		// Trimmed only once twice over budget, so each line is copied a
		// bounded number of times.
		if size += e.size(); size > 2*maxHistoryPageBytes {
			lines, size = newestWithin(lines, maxHistoryPageBytes)
		}
	}
	lines, _ = newestWithin(lines, maxHistoryPageBytes)
	return lines
}

// historyLine is a line of a page of log history.
type historyLine interface {
	stamp() string
	size() int
}

func (e logEntry) stamp() string { return e.Timestamp }
func (e logEntry) size() int     { return len(e.Line) }

// newestWithin returns the newest lines (the last ones) whose text fits in
// budget, keeping at least one so a page always moves on, and their size.
func newestWithin[T historyLine](lines []T, budget int) ([]T, int) {
	size, start := 0, len(lines)
	for start > 0 {
		s := lines[start-1].size()
		if size+s > budget && start < len(lines) {
			break
		}
		size += s
		start--
	}
	if start == 0 {
		return lines, size
	}
	// Copied, so the dropped lines' text can be freed.
	return append([]T(nil), lines[start:]...), size
}

// logStreamError logs a log stream that ended on a corrupt frame. The end
// of a stream, or of the request that read it, is not worth a line.
func logStreamError(err error) {
	if errors.Is(err, errInvalidLogFrame) || errors.Is(err, errLogFrameTooLarge) {
		log.Printf("log stream: %v", err)
	}
}

// eachLogEntry parses a Docker log stream, calling fn with each entry until
// the stream ends (see logStreamError) or fn returns false.
func eachLogEntry(reader io.Reader, fn func(logEntry) bool) {
	lr := newLogLineReader(reader)
	for {
		stream, line, err := lr.next()
		if err != nil {
			logStreamError(err)
			return
		}
		if !fn(parseLogEntry(stream, line)) {
			return
		}
	}
}

// resumeTailLines bounds the backlog replayed when a log stream resumes from
// a timestamp (?since=, or a service stream reopening). Docker's Tail "0"
// means "only new lines" and drops Since, skipping everything logged in
// between; a bounded tail replays it without rescanning a whole large log
// file on every open.
const resumeTailLines = 10000

// afterTimestamp returns the position just after since: Docker's Since is
// inclusive, and the line at since was already delivered. A value that isn't
// an RFC 3339 timestamp (Docker also accepts unix seconds) is passed through.
func afterTimestamp(since string) string {
	ts, err := time.Parse(time.RFC3339Nano, since)
	if err != nil {
		return since
	}
	return ts.Add(time.Nanosecond).Format(time.RFC3339Nano)
}

// serviceHistoryTailFactor widens the per-task tail requested for service log
// history, since the service endpoint ignores Until and the lines before the
// cursor must be found by filtering a larger window. serviceHistoryMaxLines
// caps that per-task tail.
const (
	serviceHistoryTailFactor = 10
	serviceHistoryMaxLines   = 10000
)

// fetchLogHistory reads up to limit log lines older than before (if set).
// Container logs honour Until and Tail directly; Until includes the line at
// before, which the client already has, so it is dropped: once the history
// is exhausted, a page comes back empty. Service logs ignore Until, apply
// Tail per task and group lines by task, so for them a wider window is read
// in full, lines at or after before are dropped, and the newest limit kept
// in order.
func fetchLogHistory(ctx context.Context, open logOpener, id string, service bool, before string, limit int) ([]logEntry, error) {
	tail := limit
	switch {
	case service && before != "":
		tail = min(limit*serviceHistoryTailFactor, serviceHistoryMaxLines)
	case before != "":
		// One more for the line at before, so that a page is still full
		// once it is dropped: an aggregate page tells a source that may hold
		// older lines by its full page.
		tail = limit + 1
	}
	opts := containertypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       strconv.Itoa(tail),
		Until:      before,
	}
	reader, err := open(ctx, id, opts)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	if !service {
		lines := dropFromCursor(readLogLines(reader, tail), before)
		if len(lines) > limit {
			lines = lines[len(lines)-limit:]
		}
		return lines, nil
	}
	return readNewestLinesBefore(reader, before, limit), nil
}

// dropFromCursor drops the lines whose timestamp is at or after before, when
// it parses as RFC 3339, keeping the others in order.
func dropFromCursor(lines []logEntry, before string) []logEntry {
	cut, err := time.Parse(time.RFC3339Nano, before)
	if err != nil {
		return lines
	}
	return slices.DeleteFunc(lines, func(l logEntry) bool {
		ts, err := time.Parse(time.RFC3339Nano, l.Timestamp)
		return err == nil && !ts.Before(cut)
	})
}

// readNewestLinesBefore reads a whole log stream and returns its newest limit
// lines older than before. The stream is read to the end rather than cut at a
// line count: Docker sends a service's tasks one after another, so a cut would
// drop whole later tasks, often holding the newest lines.
func readNewestLinesBefore(reader io.Reader, before string, limit int) []logEntry {
	kept := newNewestLines[logEntry](before, limit)
	eachLogEntry(reader, func(e logEntry) bool {
		kept.add(e)
		return true
	})
	return kept.result()
}

// newestLinesBefore drops lines whose timestamp is at or after before (when
// it parses as RFC 3339), sorts the rest by timestamp and keeps the newest limit.
func newestLinesBefore(lines []logEntry, before string, limit int) []logEntry {
	kept := newNewestLines[logEntry](before, limit)
	for _, l := range lines {
		kept.add(l)
	}
	return kept.result()
}

// newestLines keeps the newest limit lines older than a cursor, within
// maxHistoryPageBytes, out of any number added. It holds at most twice
// that, trimming back whenever it fills, so memory stays bounded however
// long the stream is.
type newestLines[T historyLine] struct {
	cut    time.Time
	hasCut bool
	limit  int
	lines  []T
	size   int
}

func newNewestLines[T historyLine](before string, limit int) *newestLines[T] {
	n := &newestLines[T]{limit: limit}
	if cut, err := time.Parse(time.RFC3339Nano, before); err == nil {
		n.cut, n.hasCut = cut, true
	}
	return n
}

// add keeps l unless its timestamp is at or after the cursor.
func (n *newestLines[T]) add(l T) {
	if n.hasCut {
		if ts, err := time.Parse(time.RFC3339Nano, l.stamp()); err == nil && !ts.Before(n.cut) {
			return
		}
	}
	n.lines = append(n.lines, l)
	n.size += l.size()
	if len(n.lines) >= 2*n.limit || n.size > 2*maxHistoryPageBytes {
		n.trim()
	}
}

// trim sorts the held lines by timestamp and drops all but the newest limit
// that fit in maxHistoryPageBytes. The sort is stable and earlier survivors
// stay ahead of later lines, so lines sharing a timestamp keep their stream
// order.
func (n *newestLines[T]) trim() {
	// Docker's fixed-width RFC 3339 UTC timestamps sort correctly as strings.
	sort.SliceStable(n.lines, func(i, j int) bool { return n.lines[i].stamp() < n.lines[j].stamp() })
	if len(n.lines) > n.limit {
		n.lines = append(n.lines[:0], n.lines[len(n.lines)-n.limit:]...)
	}
	n.lines, n.size = newestWithin(n.lines, maxHistoryPageBytes)
}

// result returns the kept lines, oldest first.
func (n *newestLines[T]) result() []T {
	n.trim()
	return n.lines
}

// findTimestampEnd returns the index of the space separating a Docker
// log timestamp from the message, or -1 if no timestamp is found.
// Docker format: "2006-01-02T15:04:05.999999999Z message..."
func findTimestampEnd(line string) int {
	if len(line) < 20 {
		return -1
	}
	if line[0] < '0' || line[0] > '9' || line[4] != '-' || line[10] != 'T' {
		return -1
	}
	for i := 19; i < len(line); i++ {
		if line[i] == ' ' {
			return i
		}
	}
	return -1
}

// parseLogEntry splits a raw Docker log line into a structured entry.
func parseLogEntry(streamType, raw string) logEntry {
	entry := logEntry{Stream: streamType, Line: raw}
	if idx := findTimestampEnd(raw); idx > 0 {
		entry.Timestamp = raw[:idx]
		entry.Line = raw[idx+1:]
	}
	return entry
}

// streamDockerLogs reads a Docker log stream and writes SSE events until
// the stream ends or the client stops reading. Each line is sent as a
// JSON-encoded SSE data event with pre-split timestamp.
func streamDockerLogs(sw *streamWriter, reader io.Reader) {
	eachLogEntry(reader, func(e logEntry) bool {
		data, _ := json.Marshal(e)
		return sw.send(data) == nil
	})
}

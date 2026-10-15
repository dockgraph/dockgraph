package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

// dockerLogReader reads Docker's multiplexed log stream and yields one frame at a time.
// Docker log stream format: 8-byte header [type(1) + padding(3) + size(4)] + payload.
type dockerLogReader struct {
	reader io.Reader
	header [8]byte
}

// next reads the next frame from the Docker log stream.
// Returns the stream type ("stdout"/"stderr"), the raw payload, and any error.
func (r *dockerLogReader) next() (string, []byte, error) {
	if _, err := io.ReadFull(r.reader, r.header[:]); err != nil {
		return "", nil, err
	}

	streamType := streamStdout
	if r.header[0] == 2 {
		streamType = streamStderr
	}

	size := binary.BigEndian.Uint32(r.header[4:8])
	payload := make([]byte, size)
	if _, err := io.ReadFull(r.reader, payload); err != nil {
		return "", nil, err
	}

	return streamType, payload, nil
}

// logOpener opens a Docker multiplexed log stream for one container or
// service; ContainerLogs and ServiceLogs share this shape.
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
			opts.Since = since
			opts.Tail = "0"
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

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		disableWriteDeadline(w)

		// Flush headers immediately so the EventSource client sees the
		// connection as open even when no log lines have arrived yet.
		flusher.Flush()

		streamDockerLogs(w, flusher, reader)
	}
}

// readLogLines parses Docker's multiplexed log stream into a slice of log entries.
func readLogLines(reader io.Reader, limit int) []logEntry {
	dlr := &dockerLogReader{reader: reader}
	var lines []logEntry

	for len(lines) < limit {
		streamType, payload, err := dlr.next()
		if err != nil {
			break
		}

		scanner := bufio.NewScanner(bytes.NewReader(payload))
		for scanner.Scan() && len(lines) < limit {
			lines = append(lines, parseLogEntry(streamType, scanner.Text()))
		}
	}

	return lines
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
// Container logs honour Until and Tail directly. Service logs ignore Until,
// apply Tail per task and group lines by task, so for them a wider window is
// read in full, lines at or after before are dropped, and the newest limit
// kept in order.
func fetchLogHistory(ctx context.Context, open logOpener, id string, service bool, before string, limit int) ([]logEntry, error) {
	tail := limit
	if service && before != "" {
		tail = min(limit*serviceHistoryTailFactor, serviceHistoryMaxLines)
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
		return readLogLines(reader, limit), nil
	}
	return readNewestLinesBefore(reader, before, limit), nil
}

// readNewestLinesBefore reads a whole log stream and returns its newest limit
// lines older than before. The stream is read to the end rather than cut at a
// line count: Docker sends a service's tasks one after another, so a cut would
// drop whole later tasks, often holding the newest lines.
func readNewestLinesBefore(reader io.Reader, before string, limit int) []logEntry {
	kept := newNewestLines(before, limit)
	dlr := &dockerLogReader{reader: reader}
	for {
		streamType, payload, err := dlr.next()
		if err != nil {
			break
		}
		scanner := bufio.NewScanner(bytes.NewReader(payload))
		for scanner.Scan() {
			kept.add(parseLogEntry(streamType, scanner.Text()))
		}
	}
	return kept.result()
}

// newestLinesBefore drops lines whose timestamp is at or after before (when
// it parses as RFC 3339), sorts the rest by timestamp and keeps the newest limit.
func newestLinesBefore(lines []logEntry, before string, limit int) []logEntry {
	kept := newNewestLines(before, limit)
	for _, l := range lines {
		kept.add(l)
	}
	return kept.result()
}

// newestLines keeps the newest limit lines older than a cursor out of any
// number added. It holds at most twice limit lines, trimming back to the
// newest limit whenever it fills, so memory stays bounded however long the
// stream is.
type newestLines struct {
	cut    time.Time
	hasCut bool
	limit  int
	lines  []logEntry
}

func newNewestLines(before string, limit int) *newestLines {
	n := &newestLines{limit: limit}
	if cut, err := time.Parse(time.RFC3339Nano, before); err == nil {
		n.cut, n.hasCut = cut, true
	}
	return n
}

// add keeps l unless its timestamp is at or after the cursor.
func (n *newestLines) add(l logEntry) {
	if n.hasCut {
		if ts, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil && !ts.Before(n.cut) {
			return
		}
	}
	n.lines = append(n.lines, l)
	if len(n.lines) >= 2*n.limit {
		n.trim()
	}
}

// trim sorts the held lines by timestamp and drops all but the newest limit.
// The sort is stable and earlier survivors stay ahead of later lines, so
// lines sharing a timestamp keep their stream order.
func (n *newestLines) trim() {
	// Docker's fixed-width RFC 3339 UTC timestamps sort correctly as strings.
	sort.SliceStable(n.lines, func(i, j int) bool { return n.lines[i].Timestamp < n.lines[j].Timestamp })
	if len(n.lines) > n.limit {
		n.lines = append(n.lines[:0], n.lines[len(n.lines)-n.limit:]...)
	}
}

// result returns the kept lines, oldest first.
func (n *newestLines) result() []logEntry {
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

// disableWriteDeadline lifts the server's WriteTimeout for a long-lived SSE
// response. http.Server.WriteTimeout is an absolute deadline from the start
// of the request, so without this every log stream is cut after it expires
// even while lines are flowing. Streams still end when the client
// disconnects (request context). Writers that don't support deadlines, such
// as test recorders, are left unchanged.
func disableWriteDeadline(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
}

// streamDockerLogs reads Docker's multiplexed log stream and writes SSE events.
// Each line is sent as a JSON-encoded SSE data event with pre-split timestamp.
func streamDockerLogs(w http.ResponseWriter, flusher http.Flusher, reader io.Reader) {
	dlr := &dockerLogReader{reader: reader}

	for {
		streamType, payload, err := dlr.next()
		if err != nil {
			return
		}

		scanner := bufio.NewScanner(bytes.NewReader(payload))
		for scanner.Scan() {
			data, _ := json.Marshal(parseLogEntry(streamType, scanner.Text()))
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

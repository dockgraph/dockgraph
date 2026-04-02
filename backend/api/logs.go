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

// HandleContainerLogsHistory returns a handler for GET /api/containers/{id}/logs/history.
// Returns a paginated JSON array of log lines, newest last.
func HandleContainerLogsHistory(logger ContainerLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validResourceName.MatchString(id) {
			http.Error(w, `{"error":"invalid container ID"}`, http.StatusBadRequest)
			return
		}

		limit := 200
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 1000 {
				limit = n
			}
		}

		opts := containertypes.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Timestamps: true,
			Tail:       strconv.Itoa(limit),
		}

		if before := r.URL.Query().Get("before"); before != "" {
			opts.Until = before
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		reader, err := logger.ContainerLogs(ctx, id, opts)
		if err != nil {
			log.Printf("logs history %s: %v", id, err)
			http.Error(w, `{"error":"failed to read logs"}`, http.StatusInternalServerError)
			return
		}
		defer reader.Close()

		lines := readLogLines(reader, limit)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"lines": lines,
		})
	}
}

// HandleContainerLogs returns a handler for GET /api/containers/{id}/logs.
// Streams container logs as Server-Sent Events.
// Accepts optional `since` query param to only stream lines after a timestamp.
func HandleContainerLogs(logger ContainerLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validResourceName.MatchString(id) {
			http.Error(w, `{"error":"invalid container ID"}`, http.StatusBadRequest)
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

		reader, err := logger.ContainerLogs(ctx, id, opts)
		if err != nil {
			log.Printf("logs %s: %v", id, err)
			http.Error(w, `{"error":"failed to open logs"}`, http.StatusInternalServerError)
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

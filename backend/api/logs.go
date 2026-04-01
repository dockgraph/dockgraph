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

	containertypes "github.com/docker/docker/api/types/container"
)

// ContainerLogger is the subset of the Docker API needed for container logs.
type ContainerLogger interface {
	ContainerLogs(ctx context.Context, containerID string, options containertypes.LogsOptions) (io.ReadCloser, error)
}

// HandleContainerLogs returns a handler for GET /api/containers/{id}/logs.
// Streams container logs as Server-Sent Events.
func HandleContainerLogs(logger ContainerLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validResourceName.MatchString(id) {
			http.Error(w, `{"error":"invalid container ID"}`, http.StatusBadRequest)
			return
		}

		tail := "100"
		if v := r.URL.Query().Get("tail"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 1000 {
				tail = strconv.Itoa(n)
			}
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		reader, err := logger.ContainerLogs(ctx, id, containertypes.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Tail:       tail,
			Timestamps: true,
		})
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

// streamDockerLogs reads Docker's multiplexed log stream and writes SSE events.
// Docker log stream format: 8-byte header [type(1) + padding(3) + size(4)] + payload.
func streamDockerLogs(w http.ResponseWriter, flusher http.Flusher, reader io.Reader) {
	header := make([]byte, 8)

	for {
		if _, err := io.ReadFull(reader, header); err != nil {
			return
		}

		streamType := "stdout"
		if header[0] == 2 {
			streamType = "stderr"
		}

		size := binary.BigEndian.Uint32(header[4:8])
		payload := make([]byte, size)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return
		}

		scanner := bufio.NewScanner(bytes.NewReader(payload))
		for scanner.Scan() {
			line := scanner.Text()
			data, _ := json.Marshal(map[string]string{
				"stream": streamType,
				"line":   line,
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

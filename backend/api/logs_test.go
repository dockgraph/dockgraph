package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
)

// buildLogFrame constructs a single multiplexed Docker log frame.
// streamType: 1 = stdout, 2 = stderr.
func buildLogFrame(streamType byte, data string) []byte {
	header := make([]byte, 8)
	header[0] = streamType
	binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
	return append(header, []byte(data)...)
}

// stubContainerLogger implements ContainerLogger for tests.
type stubContainerLogger struct {
	data []byte
	err  error
	opts containertypes.LogsOptions // captured from last call
}

func (s *stubContainerLogger) ContainerLogs(_ context.Context, _ string, opts containertypes.LogsOptions) (io.ReadCloser, error) {
	s.opts = opts
	if s.err != nil {
		return nil, s.err
	}
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

// --- dockerLogReader tests ---

func TestDockerLogReaderStdout(t *testing.T) {
	msg := "hello from stdout\n"
	data := buildLogFrame(1, msg)
	dlr := &dockerLogReader{reader: bytes.NewReader(data)}

	stream, payload, err := dlr.next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream != "stdout" {
		t.Errorf("stream = %q, want %q", stream, "stdout")
	}
	if string(payload) != msg {
		t.Errorf("payload = %q, want %q", string(payload), msg)
	}
}

func TestDockerLogReaderStderr(t *testing.T) {
	msg := "error happened\n"
	data := buildLogFrame(2, msg)
	dlr := &dockerLogReader{reader: bytes.NewReader(data)}

	stream, payload, err := dlr.next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream != "stderr" {
		t.Errorf("stream = %q, want %q", stream, "stderr")
	}
	if string(payload) != msg {
		t.Errorf("payload = %q, want %q", string(payload), msg)
	}
}

func TestDockerLogReaderMultipleFrames(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(buildLogFrame(1, "first\n"))
	buf.Write(buildLogFrame(2, "second\n"))
	buf.Write(buildLogFrame(1, "third\n"))

	dlr := &dockerLogReader{reader: &buf}

	expected := []struct {
		stream  string
		payload string
	}{
		{"stdout", "first\n"},
		{"stderr", "second\n"},
		{"stdout", "third\n"},
	}

	for i, want := range expected {
		stream, payload, err := dlr.next()
		if err != nil {
			t.Fatalf("frame %d: unexpected error: %v", i, err)
		}
		if stream != want.stream {
			t.Errorf("frame %d: stream = %q, want %q", i, stream, want.stream)
		}
		if string(payload) != want.payload {
			t.Errorf("frame %d: payload = %q, want %q", i, string(payload), want.payload)
		}
	}

	// After all frames, next should return EOF.
	_, _, err := dlr.next()
	if err != io.EOF && err != io.ErrUnexpectedEOF {
		t.Errorf("expected EOF after all frames, got %v", err)
	}
}

func TestDockerLogReaderEOFOnEmptyInput(t *testing.T) {
	dlr := &dockerLogReader{reader: bytes.NewReader(nil)}

	_, _, err := dlr.next()
	if err != io.EOF && err != io.ErrUnexpectedEOF {
		t.Errorf("expected EOF on empty input, got %v", err)
	}
}

func TestDockerLogReaderPartialHeader(t *testing.T) {
	// Only 4 bytes — not enough for the 8-byte header.
	dlr := &dockerLogReader{reader: bytes.NewReader([]byte{1, 0, 0, 0})}

	_, _, err := dlr.next()
	if err == nil {
		t.Error("expected error on partial header, got nil")
	}
}

func TestDockerLogReaderPartialPayload(t *testing.T) {
	// Header says 100 bytes, but only 5 bytes of payload follow.
	header := make([]byte, 8)
	header[0] = 1
	binary.BigEndian.PutUint32(header[4:], 100)
	data := slices.Concat(header, []byte("short"))

	dlr := &dockerLogReader{reader: bytes.NewReader(data)}

	_, _, err := dlr.next()
	if err == nil {
		t.Error("expected error on truncated payload, got nil")
	}
}

// --- findTimestampEnd tests ---

func TestFindTimestampEndValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "RFC3339Nano with Z",
			input: "2024-01-15T10:30:45.123456789Z hello world",
			want:  30, // position of the space after 'Z'
		},
		{
			name:  "RFC3339Nano with offset",
			input: "2024-01-15T10:30:45.123456789+00:00 hello world",
			want:  35,
		},
		{
			name:  "second precision",
			input: "2024-01-15T10:30:45Z message here",
			want:  20,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findTimestampEnd(tc.input)
			if got != tc.want {
				t.Errorf("findTimestampEnd(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestFindTimestampEndInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"too short", "short"},
		{"no timestamp prefix", "hello this is just a log line without timestamp"},
		{"missing dash at pos 4", "2024X01-15T10:30:45.123Z message"},
		{"missing T at pos 10", "2024-01-15X10:30:45.123Z message"},
		{"starts with letter", "abcd-01-15T10:30:45.123Z message"},
		{"empty string", ""},
		{"exactly 19 chars no space", "2024-01-15T10:30:45"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findTimestampEnd(tc.input)
			if got != -1 {
				t.Errorf("findTimestampEnd(%q) = %d, want -1", tc.input, got)
			}
		})
	}
}

func TestFindTimestampEndNoSpaceAfterTimestamp(t *testing.T) {
	// Valid timestamp prefix but no trailing space — should return -1.
	input := "2024-01-15T10:30:45.123456789Z"
	got := findTimestampEnd(input)
	if got != -1 {
		t.Errorf("findTimestampEnd(%q) = %d, want -1", input, got)
	}
}

// --- parseLogEntry tests ---

func TestParseLogEntryWithTimestamp(t *testing.T) {
	raw := "2024-01-15T10:30:45.123456789Z container started"
	entry := parseLogEntry("stdout", raw)

	if entry.Stream != "stdout" {
		t.Errorf("stream = %q, want %q", entry.Stream, "stdout")
	}
	if entry.Timestamp != "2024-01-15T10:30:45.123456789Z" {
		t.Errorf("timestamp = %q, want %q", entry.Timestamp, "2024-01-15T10:30:45.123456789Z")
	}
	if entry.Line != "container started" {
		t.Errorf("line = %q, want %q", entry.Line, "container started")
	}
}

func TestParseLogEntryWithoutTimestamp(t *testing.T) {
	raw := "just a plain log line"
	entry := parseLogEntry("stderr", raw)

	if entry.Stream != "stderr" {
		t.Errorf("stream = %q, want %q", entry.Stream, "stderr")
	}
	if entry.Timestamp != "" {
		t.Errorf("timestamp = %q, want empty", entry.Timestamp)
	}
	if entry.Line != raw {
		t.Errorf("line = %q, want %q", entry.Line, raw)
	}
}

func TestParseLogEntryStdoutVsStderr(t *testing.T) {
	raw := "2024-01-15T10:30:45.000000000Z message"

	stdout := parseLogEntry("stdout", raw)
	if stdout.Stream != "stdout" {
		t.Errorf("stdout entry stream = %q", stdout.Stream)
	}

	stderr := parseLogEntry("stderr", raw)
	if stderr.Stream != "stderr" {
		t.Errorf("stderr entry stream = %q", stderr.Stream)
	}
}

// --- readLogLines tests ---

func TestReadLogLinesSingleFrame(t *testing.T) {
	data := buildLogFrame(1, "2024-01-15T10:30:45.000000000Z line one\n2024-01-15T10:30:46.000000000Z line two\n")
	lines := readLogLines(bytes.NewReader(data), 100)

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0].Line != "line one" {
		t.Errorf("lines[0].Line = %q, want %q", lines[0].Line, "line one")
	}
	if lines[1].Line != "line two" {
		t.Errorf("lines[1].Line = %q, want %q", lines[1].Line, "line two")
	}
}

func TestReadLogLinesMultipleFrames(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(buildLogFrame(1, "2024-01-15T10:30:45.000000000Z stdout line\n"))
	buf.Write(buildLogFrame(2, "2024-01-15T10:30:46.000000000Z stderr line\n"))

	lines := readLogLines(&buf, 100)

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0].Stream != "stdout" {
		t.Errorf("lines[0].Stream = %q, want stdout", lines[0].Stream)
	}
	if lines[1].Stream != "stderr" {
		t.Errorf("lines[1].Stream = %q, want stderr", lines[1].Stream)
	}
}

func TestReadLogLinesRespectsLimit(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 10; i++ {
		buf.Write(buildLogFrame(1, fmt.Sprintf("2024-01-15T10:30:%02d.000000000Z line %d\n", i, i)))
	}

	lines := readLogLines(&buf, 3)

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
}

func TestReadLogLinesEOFBeforeLimit(t *testing.T) {
	data := buildLogFrame(1, "2024-01-15T10:30:45.000000000Z only one\n")
	lines := readLogLines(bytes.NewReader(data), 100)

	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if lines[0].Line != "only one" {
		t.Errorf("lines[0].Line = %q, want %q", lines[0].Line, "only one")
	}
}

func TestReadLogLinesEmptyInput(t *testing.T) {
	lines := readLogLines(bytes.NewReader(nil), 100)
	if len(lines) != 0 {
		t.Errorf("got %d lines, want 0", len(lines))
	}
}

func TestReadLogLinesMultipleLinesPerFrame(t *testing.T) {
	payload := "2024-01-15T10:00:01.000000000Z A\n" +
		"2024-01-15T10:00:02.000000000Z B\n" +
		"2024-01-15T10:00:03.000000000Z C\n"
	data := buildLogFrame(1, payload)

	lines := readLogLines(bytes.NewReader(data), 2)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (limit should stop mid-frame)", len(lines))
	}
}

// --- HandleContainerLogsHistory HTTP tests ---

func newTestServer(logger *stubContainerLogger) *httptest.Server {
	hub := NewHub()
	fs := fstest.MapFS{"index.html": {Data: []byte("ok")}}
	handler := NewServer(hub, fs, &stubHealth{}, nil, &stubDockerAPI{logger: logger})
	return httptest.NewServer(handler)
}

// stubDockerAPI implements DockerAPI by delegating log calls to the embedded logger.
type stubDockerAPI struct {
	logger *stubContainerLogger
}

func (s *stubDockerAPI) ContainerLogs(ctx context.Context, containerID string, opts containertypes.LogsOptions) (io.ReadCloser, error) {
	return s.logger.ContainerLogs(ctx, containerID, opts)
}

func (s *stubDockerAPI) ContainerInspect(_ context.Context, _ string) (containertypes.InspectResponse, error) {
	return containertypes.InspectResponse{}, fmt.Errorf("not implemented")
}

func (s *stubDockerAPI) VolumeInspect(_ context.Context, _ string) (volumetypes.Volume, error) {
	return volumetypes.Volume{}, fmt.Errorf("not implemented")
}

func (s *stubDockerAPI) NetworkInspect(_ context.Context, _ string, _ networktypes.InspectOptions) (networktypes.Inspect, error) {
	return networktypes.Inspect{}, fmt.Errorf("not implemented")
}

func TestLogsHistoryValidRequest(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(buildLogFrame(1, "2024-01-15T10:30:45.000000000Z hello\n"))
	buf.Write(buildLogFrame(2, "2024-01-15T10:30:46.000000000Z world\n"))

	stub := &stubContainerLogger{data: buf.Bytes()}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/abc123/logs/history")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var result struct {
		Lines []logEntry `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(result.Lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(result.Lines))
	}
	if result.Lines[0].Line != "hello" {
		t.Errorf("lines[0].Line = %q, want %q", result.Lines[0].Line, "hello")
	}
	if result.Lines[0].Stream != "stdout" {
		t.Errorf("lines[0].Stream = %q, want stdout", result.Lines[0].Stream)
	}
	if result.Lines[1].Stream != "stderr" {
		t.Errorf("lines[1].Stream = %q, want stderr", result.Lines[1].Stream)
	}
}

func TestLogsHistoryInvalidContainerID(t *testing.T) {
	stub := &stubContainerLogger{data: nil}
	srv := newTestServer(stub)
	defer srv.Close()

	tests := []string{
		"/api/containers/$bad/logs/history",
		"/api/containers/ab;rm/logs/history",
		"/api/containers/%20space/logs/history",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want 400; body = %s", resp.StatusCode, body)
			}
		})
	}
}

func TestLogsHistoryCustomLimit(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 10; i++ {
		buf.Write(buildLogFrame(1, fmt.Sprintf("2024-01-15T10:%02d:00.000000000Z line %d\n", i, i)))
	}

	stub := &stubContainerLogger{data: buf.Bytes()}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/mycontainer/logs/history?limit=5")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Lines []logEntry `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != 5 {
		t.Errorf("got %d lines, want 5", len(result.Lines))
	}

	// Verify the tail option was passed to Docker.
	if stub.opts.Tail != "5" {
		t.Errorf("Docker Tail option = %q, want %q", stub.opts.Tail, "5")
	}
}

func TestLogsHistoryInvalidLimitDefaults(t *testing.T) {
	stub := &stubContainerLogger{data: buildLogFrame(1, "2024-01-15T10:30:00.000000000Z ok\n")}
	srv := newTestServer(stub)
	defer srv.Close()

	tests := []struct {
		name     string
		query    string
		wantTail string
	}{
		{"zero", "?limit=0", "200"},
		{"negative", "?limit=-5", "200"},
		{"over max", "?limit=9999", "200"},
		{"non-numeric", "?limit=abc", "200"},
		{"empty value", "?limit=", "200"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs/history" + tc.query)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if stub.opts.Tail != tc.wantTail {
				t.Errorf("Docker Tail = %q, want %q", stub.opts.Tail, tc.wantTail)
			}
		})
	}
}

func TestLogsHistoryLimitClampedTo1000(t *testing.T) {
	stub := &stubContainerLogger{data: buildLogFrame(1, "2024-01-15T10:30:00.000000000Z ok\n")}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs/history?limit=1000")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if stub.opts.Tail != "1000" {
		t.Errorf("Docker Tail = %q, want %q", stub.opts.Tail, "1000")
	}
}

func TestLogsHistoryBeforeParam(t *testing.T) {
	stub := &stubContainerLogger{data: buildLogFrame(1, "2024-01-15T10:30:00.000000000Z ok\n")}
	srv := newTestServer(stub)
	defer srv.Close()

	before := "2024-01-15T10:00:00Z"
	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs/history?before=" + before)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if stub.opts.Until != before {
		t.Errorf("Docker Until = %q, want %q", stub.opts.Until, before)
	}
}

func TestLogsHistoryDockerError(t *testing.T) {
	stub := &stubContainerLogger{err: fmt.Errorf("container not found")}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs/history")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
		if body["error"] == "" {
			t.Error("expected error message in response body")
		}
	}
}

func TestLogsHistoryEmptyLogs(t *testing.T) {
	stub := &stubContainerLogger{data: nil}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs/history")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Lines []logEntry `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	// The handler returns a nil slice which encodes as JSON null.
	// Both nil and empty are acceptable; just verify we got valid JSON.
	_ = result.Lines
}

func TestLogsHistoryTimestampsEnabled(t *testing.T) {
	stub := &stubContainerLogger{data: buildLogFrame(1, "2024-01-15T10:30:00.000000000Z ok\n")}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs/history")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if !stub.opts.Timestamps {
		t.Error("expected Timestamps option to be true")
	}
	if !stub.opts.ShowStdout {
		t.Error("expected ShowStdout to be true")
	}
	if !stub.opts.ShowStderr {
		t.Error("expected ShowStderr to be true")
	}
}

// --- HandleContainerLogs (SSE streaming) tests ---

func TestLogsStreamSSEHeaders(t *testing.T) {
	data := buildLogFrame(1, "2024-01-15T10:30:45.000000000Z streaming line\n")
	stub := &stubContainerLogger{data: data}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	expectedHeaders := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"Connection":        "keep-alive",
		"X-Accel-Buffering": "no",
	}

	for header, want := range expectedHeaders {
		got := resp.Header.Get(header)
		if got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestLogsStreamInvalidContainerID(t *testing.T) {
	stub := &stubContainerLogger{data: nil}
	srv := newTestServer(stub)
	defer srv.Close()

	tests := []string{
		"/api/containers/$bad/logs",
		"/api/containers/ab;rm/logs",
		"/api/containers/%20space/logs",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want 400; body = %s", resp.StatusCode, body)
			}
		})
	}
}

func TestLogsStreamDockerError(t *testing.T) {
	stub := &stubContainerLogger{err: fmt.Errorf("container gone")}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestLogsStreamValidResponseContainsSSEData(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(buildLogFrame(1, "2024-01-15T10:30:45.000000000Z line one\n"))
	buf.Write(buildLogFrame(2, "2024-01-15T10:30:46.000000000Z line two\n"))

	stub := &stubContainerLogger{data: buf.Bytes()}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// SSE events are formatted as "data: {...}\n\n".
	if !strings.Contains(bodyStr, "data: ") {
		t.Error("expected SSE data events in response body")
	}

	// Parse individual SSE events.
	events := strings.Split(strings.TrimSpace(bodyStr), "\n\n")
	if len(events) < 2 {
		t.Fatalf("got %d SSE events, want at least 2", len(events))
	}

	// Validate first event.
	first := strings.TrimPrefix(events[0], "data: ")
	var entry logEntry
	if err := json.Unmarshal([]byte(first), &entry); err != nil {
		t.Fatalf("failed to parse first SSE event: %v", err)
	}
	if entry.Stream != "stdout" {
		t.Errorf("first event stream = %q, want stdout", entry.Stream)
	}
	if entry.Line != "line one" {
		t.Errorf("first event line = %q, want %q", entry.Line, "line one")
	}

	// Validate second event.
	second := strings.TrimPrefix(events[1], "data: ")
	if err := json.Unmarshal([]byte(second), &entry); err != nil {
		t.Fatalf("failed to parse second SSE event: %v", err)
	}
	if entry.Stream != "stderr" {
		t.Errorf("second event stream = %q, want stderr", entry.Stream)
	}
}

func TestLogsStreamSinceParam(t *testing.T) {
	stub := &stubContainerLogger{data: buildLogFrame(1, "2024-01-15T10:30:00.000000000Z ok\n")}
	srv := newTestServer(stub)
	defer srv.Close()

	since := "2024-01-15T10:00:00Z"
	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs?since=" + since)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if stub.opts.Since != since {
		t.Errorf("Docker Since = %q, want %q", stub.opts.Since, since)
	}
	if stub.opts.Tail != "0" {
		t.Errorf("Docker Tail = %q, want %q (should be 0 when since is set)", stub.opts.Tail, "0")
	}
}

func TestLogsStreamDefaultTail(t *testing.T) {
	stub := &stubContainerLogger{data: buildLogFrame(1, "2024-01-15T10:30:00.000000000Z ok\n")}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if stub.opts.Tail != "100" {
		t.Errorf("Docker Tail = %q, want %q (default)", stub.opts.Tail, "100")
	}
	if !stub.opts.Follow {
		t.Error("expected Follow to be true for streaming endpoint")
	}
}

func TestLogsStreamCustomTail(t *testing.T) {
	stub := &stubContainerLogger{data: buildLogFrame(1, "2024-01-15T10:30:00.000000000Z ok\n")}
	srv := newTestServer(stub)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs?tail=50")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if stub.opts.Tail != "50" {
		t.Errorf("Docker Tail = %q, want %q", stub.opts.Tail, "50")
	}
}

func TestLogsStreamInvalidTailIgnored(t *testing.T) {
	stub := &stubContainerLogger{data: buildLogFrame(1, "2024-01-15T10:30:00.000000000Z ok\n")}
	srv := newTestServer(stub)
	defer srv.Close()

	tests := []struct {
		name  string
		query string
	}{
		{"negative", "?tail=-1"},
		{"zero", "?tail=0"},
		{"over max", "?tail=5000"},
		{"non-numeric", "?tail=abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(srv.URL + "/api/containers/testcontainer/logs" + tc.query)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			// Invalid tail values should fall back to default "100".
			if stub.opts.Tail != "100" {
				t.Errorf("Docker Tail = %q, want default %q", stub.opts.Tail, "100")
			}
		})
	}
}

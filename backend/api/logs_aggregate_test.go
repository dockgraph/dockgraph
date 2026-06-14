package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
)

// syncWriter is a minimal, goroutine-safe http.ResponseWriter+Flusher for
// testing streaming handlers: the handler writes from its own goroutine while
// the test polls body() concurrently.
type syncWriter struct {
	mu     sync.Mutex
	header http.Header
	buf    bytes.Buffer
}

func newSyncWriter() *syncWriter { return &syncWriter{header: http.Header{}} }

func (s *syncWriter) Header() http.Header { return s.header }
func (s *syncWriter) WriteHeader(int)     {}
func (s *syncWriter) Flush()              {}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncWriter) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// frame encodes one line as a Docker multiplexed-stream frame (stdout=1).
func frame(line string) []byte {
	payload := []byte(line + "\n")
	hdr := make([]byte, 8)
	hdr[0] = 1
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(payload)))
	return append(hdr, payload...)
}

// frameStr is frame() as a string for map literals.
func frameStr(line string) string { return string(frame(line)) }

// mockLister returns a fixed container list.
type mockLister struct{ summaries []containertypes.Summary }

func (m mockLister) ContainerList(_ context.Context, _ containertypes.ListOptions) ([]containertypes.Summary, error) {
	return m.summaries, nil
}

// mockLogger returns canned multiplexed log bytes per container ID.
type mockLogger struct{ byID map[string]string }

func (m mockLogger) ContainerLogs(_ context.Context, id string, _ containertypes.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.byID[id])), nil
}

func TestCollectHistory_MergeSortsAndExcludesSelf(t *testing.T) {
	lister := mockLister{summaries: []containertypes.Summary{
		{ID: "a", Names: []string{"/web"}},
		{ID: "b", Names: []string{"/db"}},
		{ID: "self", Names: []string{"/dockgraph"}, Labels: map[string]string{"dockgraph.self": "true"}},
	}}
	logger := mockLogger{byID: map[string]string{
		"a":    frameStr("2026-06-11T10:00:02.000000000Z web-line"),
		"b":    frameStr("2026-06-11T10:00:01.000000000Z db-line"),
		"self": frameStr("2026-06-11T10:00:00.000000000Z self-line"),
	}}

	lines, err := collectHistory(context.Background(), lister, logger, "", 50)
	if err != nil {
		t.Fatalf("collectHistory: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 lines (self excluded), got %d: %+v", len(lines), lines)
	}
	// merge-sorted ascending by timestamp: db (…01) before web (…02)
	if lines[0].Container != "db" || lines[1].Container != "web" {
		t.Fatalf("wrong order: %+v", lines)
	}
	if lines[0].Line != "db-line" {
		t.Fatalf("timestamp not split off: %q", lines[0].Line)
	}
}

func TestHandleAggregateLogsHistory(t *testing.T) {
	lister := mockLister{summaries: []containertypes.Summary{
		{ID: "a", Names: []string{"/web"}},
		{ID: "b", Names: []string{"/db"}},
	}}
	logger := mockLogger{byID: map[string]string{
		"a": frameStr("2026-06-11T10:00:02.000000000Z web-line"),
		"b": frameStr("2026-06-11T10:00:01.000000000Z db-line"),
	}}
	h := HandleAggregateLogsHistory(lister, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/logs/history?limit=10", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Lines []aggregateLine `json:"lines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Lines) != 2 || body.Lines[0].Container != "db" {
		t.Fatalf("unexpected lines: %+v", body.Lines)
	}
}

// followLogger blocks (Follow semantics) after replaying canned bytes.
type followLogger struct{ byID map[string]string }

func (f followLogger) ContainerLogs(ctx context.Context, id string, _ containertypes.LogsOptions) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write([]byte(f.byID[id]))
		<-ctx.Done() // stay open like a real follow until cancelled
		pw.Close()
	}()
	return pr, nil
}

func TestLogAggregator_AddForwardsTaggedLines(t *testing.T) {
	logger := followLogger{byID: map[string]string{
		"a": frameStr("2026-06-11T10:00:00.000000000Z hello"),
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agg := newLogAggregator(logger, "")
	agg.add(ctx, "a", "web")

	select {
	case line := <-agg.out:
		if line.Container != "web" || line.Line != "hello" {
			t.Fatalf("bad line: %+v", line)
		}
	case <-time.After(time.Second):
		t.Fatal("no line received")
	}

	agg.remove("a") // must not panic; follower cancelled
}

// noEvents yields no Docker events (closed channels on ctx done).
type noEvents struct{}

func (noEvents) Events(ctx context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
	msg := make(chan events.Message)
	errc := make(chan error)
	go func() { <-ctx.Done(); close(msg); close(errc) }()
	return msg, errc
}

func TestHandleAggregateLogs_StreamsInitialContainers(t *testing.T) {
	lister := mockLister{summaries: []containertypes.Summary{
		{ID: "a", Names: []string{"/web"}, State: "running"},
	}}
	logger := followLogger{byID: map[string]string{
		"a": frameStr("2026-06-11T10:00:00.000000000Z hello"),
	}}
	h := HandleAggregateLogs(lister, logger, noEvents{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil).WithContext(ctx)
	rec := newSyncWriter()

	done := make(chan struct{})
	go func() { h(rec, req); close(done) }()

	deadline := time.After(2 * time.Second)
	for {
		if strings.Contains(rec.body(), "hello") {
			cancel()
			break
		}
		select {
		case <-deadline:
			t.Fatalf("did not stream line; body=%q", rec.body())
		case <-time.After(20 * time.Millisecond):
		}
	}
	<-done
}

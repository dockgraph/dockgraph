package api

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/gorilla/websocket"
)

func TestSlotLimiter(t *testing.T) {
	l := newSlotLimiter(3, 2)
	for i := range 2 {
		if !l.acquire("a") {
			t.Fatalf("client a should get slot %d", i+1)
		}
	}
	if l.acquire("a") {
		t.Error("client a should be refused a third slot")
	}
	if !l.acquire("b") {
		t.Error("client b should get the last slot")
	}
	if l.acquire("c") {
		t.Error("client c should be refused: all slots taken")
	}
	l.release("a")
	if !l.acquire("c") {
		t.Error("a released slot should be reusable")
	}
	l.release("a")
	l.release("b")
	l.release("c")
	if l.total != 0 || len(l.perClient) != 0 {
		t.Errorf("after releasing everything: total %d, clients %v", l.total, l.perClient)
	}
}

func TestLimitStreamsRefusesOverLimitAndReleases(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 4)
	blocking := func(http.ResponseWriter, *http.Request) {
		entered <- struct{}{}
		<-release
	}
	outer := newSlotLimiter(10, 10)
	inner := newSlotLimiter(1, 1)
	h := limitConcurrent(func(*http.Request) string { return "client" }, blocking, outer, inner)

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/logs", nil))
		close(done)
	}()
	<-entered

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
		t.Errorf("second stream: status %d, want 503 with Retry-After", w.Code)
	}
	if outer.total != 1 {
		t.Errorf("a refused stream must give back the slots it took: outer total %d", outer.total)
	}

	close(release)
	<-done
	if outer.total != 0 || inner.total != 0 {
		t.Errorf("slots not released: outer %d inner %d", outer.total, inner.total)
	}
}

// A client that stops reading must not hold its stream open: the write
// deadline ends it, which closes the Docker log stream behind it.
func TestLogStreamEndsWhenClientStopsReading(t *testing.T) {
	prev := streamWriteTimeout
	streamWriteTimeout = 200 * time.Millisecond
	t.Cleanup(func() { streamWriteTimeout = prev })

	closed := make(chan struct{})
	logger := &endlessLogger{closed: closed}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/containers/{id}/logs", HandleContainerLogs(logger))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Shrink the receive buffer so the server's writes block quickly.
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetReadBuffer(4096)
	}
	if _, err := io.WriteString(conn, "GET /api/containers/web/logs HTTP/1.1\r\nHost: localhost\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || !strings.Contains(status, "200") {
		t.Fatalf("status line %q, %v", status, err)
	}
	// Stop reading. The server must give up within a few write timeouts.
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("log stream still open after the client stopped reading")
	}
}

// endlessLogger serves a log stream that never ends, and reports when the
// handler closes it.
type endlessLogger struct {
	closed chan struct{}
}

func (e *endlessLogger) ContainerLogs(context.Context, string, containertypes.LogsOptions) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	line := buildLogFrame(1, "2026-10-28T10:00:00.000000000Z "+strings.Repeat("x", 1000)+"\n")
	go func() {
		for {
			if _, err := pw.Write(line); err != nil {
				return
			}
		}
	}()
	return &closeNotifier{ReadCloser: pr, pw: pw, closed: e.closed}, nil
}

type closeNotifier struct {
	io.ReadCloser
	pw     *io.PipeWriter
	closed chan struct{}
}

func (c *closeNotifier) Close() error {
	_ = c.pw.Close()
	close(c.closed)
	return c.ReadCloser.Close()
}

func TestHubLimitsConnectionsPerClient(t *testing.T) {
	hub := NewHub()
	hub.MaxClientsPerKey = 2
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var conns []*websocket.Conn
	for range 2 {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conns = append(conns, ws)
	}
	waitFor(t, 2*time.Second, func() bool { return hub.ClientCount() == 2 })

	extra, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer extra.Close()
	extra.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := extra.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseTryAgainLater) {
		t.Errorf("third connection: err = %v, want close 1013", err)
	}

	conns[0].Close()
	waitFor(t, 2*time.Second, func() bool { return hub.ClientCount() == 1 })
	again, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer again.Close()
	waitFor(t, 2*time.Second, func() bool { return hub.ClientCount() == 2 })
	conns[1].Close()
}

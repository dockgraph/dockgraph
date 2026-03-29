package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dockgraph/dockgraph/collector"
	"github.com/gorilla/websocket"
)

// waitFor polls until condition returns true or the timeout expires.
func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		if condition() {
			return
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for condition")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func dialHub(t *testing.T, hub *Hub) (*httptest.Server, *websocket.Conn) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWS(w, r)
	}))
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial error: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool { return hub.ClientCount() >= 1 })
	return server, ws
}

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	server, ws := dialHub(t, hub)
	defer server.Close()
	defer ws.Close()

	msg := collector.StateMessage{
		Type: "snapshot",
		Snapshot: &collector.GraphSnapshot{
			Nodes: []collector.Node{
				{ID: "container:test", Type: "container", Name: "test"},
			},
		},
	}
	hub.Broadcast(msg)

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var wireMsg collector.WireMessage
	if err := ws.ReadJSON(&wireMsg); err != nil {
		t.Fatalf("read error: %v", err)
	}

	if wireMsg.Type != "snapshot" {
		t.Errorf("expected snapshot, got %s", wireMsg.Type)
	}
	if wireMsg.Version != 1 {
		t.Errorf("expected version 1, got %d", wireMsg.Version)
	}
}

func TestBroadcastDelta(t *testing.T) {
	hub := NewHub()
	server, ws := dialHub(t, hub)
	defer server.Close()
	defer ws.Close()

	msg := collector.StateMessage{
		Type: "delta",
		Delta: &collector.DeltaUpdate{
			NodesRemoved: []string{"container:old"},
		},
	}
	hub.Broadcast(msg)

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var wireMsg collector.WireMessage
	if err := ws.ReadJSON(&wireMsg); err != nil {
		t.Fatalf("read error: %v", err)
	}
	if wireMsg.Type != "delta" {
		t.Errorf("expected delta, got %s", wireMsg.Type)
	}
}

func TestBroadcastUnknownTypeIsNoop(t *testing.T) {
	hub := NewHub()
	server, ws := dialHub(t, hub)
	defer server.Close()
	defer ws.Close()

	hub.Broadcast(collector.StateMessage{Type: "unknown"})

	// Send a valid message after to prove the connection is still alive
	hub.Broadcast(collector.StateMessage{
		Type:  "delta",
		Delta: &collector.DeltaUpdate{},
	})

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var wireMsg collector.WireMessage
	if err := ws.ReadJSON(&wireMsg); err != nil {
		t.Fatalf("read error: %v", err)
	}
	if wireMsg.Type != "delta" {
		t.Errorf("expected delta (skipping unknown), got %s", wireMsg.Type)
	}
}

func TestBroadcastStoresSnapshot(t *testing.T) {
	hub := NewHub()

	// Broadcast a snapshot before any client connects
	hub.Broadcast(collector.StateMessage{
		Type: "snapshot",
		Snapshot: &collector.GraphSnapshot{
			Nodes: []collector.Node{
				{ID: "container:stored", Type: "container", Name: "stored"},
			},
		},
	})

	// Now connect — client should receive the stored snapshot immediately
	server, ws := dialHub(t, hub)
	defer server.Close()
	defer ws.Close()

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var wireMsg collector.WireMessage
	if err := ws.ReadJSON(&wireMsg); err != nil {
		t.Fatalf("read error: %v", err)
	}
	if wireMsg.Type != "snapshot" {
		t.Errorf("expected stored snapshot on connect, got %s", wireMsg.Type)
	}
}

func TestClientCount(t *testing.T) {
	hub := NewHub()

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}

	server, ws := dialHub(t, hub)
	defer server.Close()

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	ws.Close()
	waitFor(t, 2*time.Second, func() bool { return hub.ClientCount() == 0 })

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", hub.ClientCount())
	}
}

func TestCheckOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"empty origin allowed", "", "localhost:8080", true},
		{"matching http", "http://localhost:8080", "localhost:8080", true},
		{"matching https", "https://localhost:8080", "localhost:8080", true},
		{"mismatched origin", "http://evil.com", "localhost:8080", false},
		{"wrong scheme", "https://localhost:8080", "localhost:8081", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/ws", nil)
			r.Host = tc.host
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if got := checkOrigin(r); got != tc.want {
				t.Errorf("checkOrigin(%q, host=%q) = %v, want %v", tc.origin, tc.host, got, tc.want)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	hub := NewHub()
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	tests := []struct {
		header, want string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
	}
	for _, tt := range tests {
		if got := resp.Header.Get(tt.header); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestBroadcastDeltaUpdatesCurrentSnapshot(t *testing.T) {
	hub := NewHub()

	initial := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "running"},
		},
	}
	hub.Broadcast(collector.StateMessage{
		Type:     "snapshot",
		Snapshot: initial,
	})

	hub.mu.RLock()
	if hub.current == nil || len(hub.current.Nodes) != 1 {
		hub.mu.RUnlock()
		t.Fatal("expected current to be set after snapshot broadcast")
	}
	hub.mu.RUnlock()

	// Delta message with snapshot attached should update h.current
	updated := &collector.GraphSnapshot{
		Nodes: []collector.Node{
			{ID: "container:web", Type: "container", Name: "web", Status: "exited"},
		},
	}
	hub.Broadcast(collector.StateMessage{
		Type:     "delta",
		Delta:    &collector.DeltaUpdate{NodesUpdated: []collector.Node{{ID: "container:web", Type: "container", Name: "web", Status: "exited"}}},
		Snapshot: updated,
	})

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if hub.current == nil {
		t.Fatal("expected current to be updated after delta")
	}
	if hub.current.Nodes[0].Status != "exited" {
		t.Errorf("expected current to reflect delta, got status=%s", hub.current.Nodes[0].Status)
	}
}

func TestHubRejectsOverLimit(t *testing.T) {
	hub := NewHub()
	hub.MaxClients = 2

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var conns []*websocket.Conn
	for i := 0; i < 2; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("connection %d failed: %v", i, err)
		}
		conns = append(conns, ws)
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	waitFor(t, 2*time.Second, func() bool { return hub.ClientCount() >= 2 })

	// Third connection upgrades but is immediately closed with CloseTryAgainLater
	ws3, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("expected upgrade to succeed before close: %v", err)
	}
	defer ws3.Close()

	// The server closes the connection immediately — read should fail
	_, _, err = ws3.ReadMessage()
	if err == nil {
		t.Fatal("expected read error on over-limit connection")
	}
	closeErr, ok := err.(*websocket.CloseError)
	if ok && closeErr.Code != websocket.CloseTryAgainLater {
		t.Errorf("expected CloseTryAgainLater (1013), got %d", closeErr.Code)
	}
}

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
	time.Sleep(50 * time.Millisecond) // let the hub register the client
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
	time.Sleep(100 * time.Millisecond) // let readPump detect disconnect

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
	handler := NewServer(hub, fstest.MapFS{"index.html": {Data: []byte("ok")}}, nil)
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

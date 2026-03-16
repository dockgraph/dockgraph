package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dockgraph/docker-flow/collector"
	"github.com/gorilla/websocket"
)

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWS(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

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

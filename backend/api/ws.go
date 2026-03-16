package api

import (
	"log"
	"net/http"
	"sync"

	"github.com/dockgraph/docker-flow/collector"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsClient struct {
	conn   *websocket.Conn
	sendCh chan collector.WireMessage
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]bool
	current *collector.GraphSnapshot
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*wsClient]bool),
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	client := &wsClient{
		conn:   conn,
		sendCh: make(chan collector.WireMessage, 32),
	}

	h.mu.Lock()
	h.clients[client] = true
	snapshot := h.current
	h.mu.Unlock()

	if snapshot != nil {
		client.sendCh <- collector.NewSnapshotMessage(*snapshot)
	}

	go func() {
		defer conn.Close()
		for msg := range client.sendCh {
			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		}
	}()

	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, client)
			h.mu.Unlock()
			close(client.sendCh)
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *Hub) Broadcast(msg collector.StateMessage) {
	var wire collector.WireMessage
	if msg.Type == "snapshot" && msg.Snapshot != nil {
		wire = collector.NewSnapshotMessage(*msg.Snapshot)
		h.mu.Lock()
		h.current = msg.Snapshot
		h.mu.Unlock()
	} else if msg.Type == "delta" && msg.Delta != nil {
		wire = collector.NewDeltaMessage(*msg.Delta)
	} else {
		return
	}

	h.mu.RLock()
	clients := make([]*wsClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.sendCh <- wire:
		default:
		}
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

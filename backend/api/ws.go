// Package api provides the HTTP server, WebSocket hub, and static file
// serving for the dockgraph web interface.
package api

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dockgraph/dockgraph/collector"
	"github.com/gorilla/websocket"
)

const (
	pongTimeout = 60 * time.Second
	pingPeriod  = 30 * time.Second
)

// checkOrigin validates WebSocket upgrade requests by comparing the Origin
// header against the request Host. Empty origins (non-browser clients) are allowed.
func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	host := r.Host
	return origin == "http://"+host || origin == "https://"+host
}

var upgrader = websocket.Upgrader{
	CheckOrigin: checkOrigin,
}

type wsClient struct {
	conn   *websocket.Conn
	sendCh chan collector.WireMessage
}

// Hub manages WebSocket client connections and broadcasts state updates.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]bool
	current *collector.GraphSnapshot
}

// NewHub creates an empty WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*wsClient]bool),
	}
}

// HandleWS upgrades an HTTP request to a WebSocket connection and registers
// the client with the hub. Each connection gets a writer pump (outbound messages
// and pings) and a reader pump (pong handling and disconnect detection).
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	c := &wsClient{
		conn:   conn,
		sendCh: make(chan collector.WireMessage, 32),
	}

	h.mu.Lock()
	h.clients[c] = true
	snapshot := h.current
	h.mu.Unlock()

	if snapshot != nil {
		c.sendCh <- collector.NewSnapshotMessage(*snapshot)
	}

	go c.writePump()
	go c.readPump(h)
}

// writePump sends queued state messages to the WebSocket connection
// and issues periodic pings to detect stale connections.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.sendCh:
			if !ok {
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}

// readPump keeps the connection alive by handling pong responses. When the
// client disconnects (read error), it unregisters from the hub and closes
// the send channel to shut down the writer.
func (c *wsClient) readPump(h *Hub) {
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		close(c.sendCh)
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// Broadcast sends a state message to all connected WebSocket clients.
// Messages are dropped for slow clients to prevent backpressure.
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

// ClientCount returns the number of active WebSocket connections.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

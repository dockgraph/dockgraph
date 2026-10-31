// Package api provides the HTTP server, WebSocket hub, and static file
// serving for the dockgraph web interface.
package api

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dockgraph/dockgraph/auth"
	"github.com/dockgraph/dockgraph/collector"
	"github.com/gorilla/websocket"
)

const (
	pongTimeout = 60 * time.Second
	pingPeriod  = 30 * time.Second
	// maxClientMessageSize caps a data message from a client. The UI sends
	// none (pongs are control frames, which the limit does not cover), and
	// gorilla buffers a whole message in memory before ReadMessage returns,
	// so the cap bounds the memory one message can take.
	maxClientMessageSize = 512
)

// checkOrigin validates WebSocket upgrade requests by comparing the Origin
// header against the request Host. Only same-origin browser requests and
// non-browser clients (which never send an Origin header) are accepted. A
// foreign name made to resolve here passes this check, so the Host is
// vetted before it, by hostCheck or the session cookie (see HostAllowlist).
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
	conn      *websocket.Conn
	key       string // per-client limit bucket (see Hub.ClientKey)
	sendCh    chan collector.WireMessage
	done      chan struct{}
	issuedAt  time.Time // JWT issued-at (zero when auth disabled)
	expiresAt time.Time // JWT expiry (zero when auth disabled)
}

// Hub manages WebSocket client connections and broadcasts state updates.
type Hub struct {
	mu           sync.RWMutex
	clients      map[*wsClient]bool
	current      *collector.GraphSnapshot
	closed       bool
	MaxClients   int                                      // 0 means no bound
	CheckExpired func(issuedAt, expiresAt time.Time) bool // nil when auth disabled
	// MaxClientsPerKey bounds the connections sharing one ClientKey, so one
	// client cannot take every MaxClients slot; 0 means no bound.
	MaxClientsPerKey int
	// ClientKey buckets a connection for MaxClientsPerKey; nil buckets by
	// the peer address.
	ClientKey func(*http.Request) string

	// slots counts connections against MaxClients and MaxClientsPerKey,
	// built on first use so the bounds can be set after NewHub.
	slotsOnce sync.Once
	slots     *slotLimiter
}

// NewHub creates an empty WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:          make(map[*wsClient]bool),
		MaxClients:       128,
		MaxClientsPerKey: maxWSClientsPerClient,
	}
}

// limiter returns the hub's connection slots.
func (h *Hub) limiter() *slotLimiter {
	h.slotsOnce.Do(func() {
		h.slots = newSlotLimiter(h.MaxClients, h.MaxClientsPerKey)
	})
	return h.slots
}

// clientKey returns the per-client bucket of r.
func (h *Hub) clientKey(r *http.Request) string {
	if h.ClientKey != nil {
		return h.ClientKey(r)
	}
	return auth.TrustedProxies(nil).ClientKey(r)
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

	var iat, exp time.Time
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		iat = claims.IssuedAt
		exp = claims.ExpiresAt
	}

	c := &wsClient{
		conn:      conn,
		key:       h.clientKey(r),
		sendCh:    make(chan collector.WireMessage, 32),
		done:      make(chan struct{}),
		issuedAt:  iat,
		expiresAt: exp,
	}

	if !h.limiter().acquire(c.key) {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "too many connections"))
		conn.Close()
		return
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		h.limiter().release(c.key)
		conn.Close()
		return
	}
	h.clients[c] = true
	snapshot := h.current
	h.mu.Unlock()

	if snapshot != nil {
		c.sendCh <- collector.NewSnapshotMessage(*snapshot)
	}

	go c.writePump(h)
	go c.readPump(h)
}

// writePump sends queued state messages to the WebSocket connection
// and issues periodic pings to detect stale connections. Exits when
// the done channel is closed by readPump, or when a write fails or times
// out; it then closes the connection and frees its slot.
func (c *wsClient) writePump(h *Hub) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("writePump recovered: %v", r)
		}
		ticker.Stop()
		c.conn.Close()
		// Freed only now: a writer stuck on a client that stopped reading
		// still holds the connection and its message.
		h.limiter().release(c.key)
	}()
	for {
		select {
		case msg := <-c.sendCh:
			_ = c.conn.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}

// readPump keeps the connection alive by handling pong responses. When the
// client disconnects (read error), it unregisters from the hub and closes
// the done channel to shut down the writer, which frees the slot.
func (c *wsClient) readPump(h *Hub) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("readPump recovered: %v", r)
		}
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		close(c.done)
	}()

	c.conn.SetReadLimit(maxClientMessageSize)
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
	switch {
	case msg.Type == collector.MsgTypeSnapshot && msg.Snapshot != nil:
		wire = collector.NewSnapshotMessage(*msg.Snapshot)
	case msg.Type == collector.MsgTypeDelta && msg.Delta != nil:
		wire = collector.NewDeltaMessage(*msg.Delta)
	default:
		return
	}

	// Update stored snapshot and copy client list under a single lock to
	// prevent a new client from seeing a stale snapshot between the update
	// and the broadcast.
	h.mu.Lock()
	if msg.Snapshot != nil {
		h.current = msg.Snapshot
	}
	clients := make([]*wsClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, client := range clients {
		select {
		case client.sendCh <- wire:
		default:
		}
	}
}

// BroadcastStats sends a stats snapshot to all connected clients.
func (h *Hub) BroadcastStats(snap collector.StatsSnapshot) {
	wire := collector.NewStatsMessage(snap)
	h.sendToAll(wire)
}

// snapshotClients returns a copy of the current client list under a read lock.
func (h *Hub) snapshotClients() []*wsClient {
	h.mu.RLock()
	clients := make([]*wsClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	return clients
}

// sendToAll delivers a wire message to every connected client.
// Messages are dropped for slow consumers to prevent backpressure.
func (h *Hub) sendToAll(wire collector.WireMessage) {
	for _, client := range h.snapshotClients() {
		select {
		case client.sendCh <- wire:
		default:
		}
	}
}

// Shutdown sends a close frame to all connected clients and closes their
// connections so that the reader and writer goroutines exit cleanly.
// After Shutdown, the hub rejects new connections.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	h.closed = true
	clients := make([]*wsClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	// The close frame waits for a writer stuck on a client that stopped
	// reading; every client is closed at once, within one shared second, so
	// shutdown doesn't take a second per stuck client.
	deadline := time.Now().Add(time.Second)
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"),
				deadline,
			)
			_ = c.conn.Close()
		}()
	}
	wg.Wait()
}

// ClientCount returns the number of active WebSocket connections.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// SweepExpiredClients checks all connected clients and disconnects those
// with expired or invalidated sessions. Sends an auth_expired message
// before closing the connection.
func (h *Hub) SweepExpiredClients() {
	if h.CheckExpired == nil {
		return
	}

	h.mu.RLock()
	var expired []*wsClient
	for client := range h.clients {
		if client.issuedAt.IsZero() {
			continue
		}
		if h.CheckExpired(client.issuedAt, client.expiresAt) {
			expired = append(expired, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range expired {
		msg := collector.WireMessage{Type: "auth_expired", Version: collector.ProtocolVersion}
		select {
		case client.sendCh <- msg:
		default:
		}
		// Close after writePump has time to flush the auth_expired message.
		// 100ms is sufficient for local writes; the writePump processes
		// sendCh in a tight select loop. For typical deployment sizes
		// (MaxClients=128) this spawns at most 128 short-lived goroutines.
		go func(c *wsClient) {
			time.Sleep(100 * time.Millisecond)
			c.conn.Close()
		}(client)
	}
}

// StartSessionSweep begins periodic session validation.
// Does nothing when CheckExpired is nil (auth disabled).
func (h *Hub) StartSessionSweep(ctx context.Context, interval time.Duration) {
	if h.CheckExpired == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.SweepExpiredClients()
			}
		}
	}()
}

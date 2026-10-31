package api

import (
	"log"
	"math"
	"net/http"
	"sync"
)

// Bounds on concurrent log streams (Server-Sent Events). A log stream holds
// a Docker log stream open for as long as the client stays, and an
// aggregate one holds one per running container plus an event subscription,
// so these bound the memory they take in DockGraph and the Docker daemon,
// overall and per client.
const (
	maxLogStreams          = 64
	maxLogStreamsPerClient = 8
	// maxAggregateStreams applies on top of the above to /api/logs, which
	// multiplies into one Docker stream per container.
	maxAggregateStreams          = 8
	maxAggregateStreamsPerClient = 4
)

// Bounds on concurrent log history requests. Each reads Docker log streams
// (one per container or service for /api/logs/history) and holds a page of
// lines while it runs; a page is bounded (maxHistoryPageBytes), so these
// bound the memory history can take overall.
const (
	maxLogHistoryRequests          = 8
	maxLogHistoryRequestsPerClient = 4
)

// Bounds on concurrent container and volume inspect requests. A local
// inspect is cheap, but one for a container on another node holds the
// agent's answer (up to maxRemoteInspectBytes) while it is masked again, and
// a volume inspect asks every node's agent at once.
const (
	maxInspectRequests          = 16
	maxInspectRequestsPerClient = 8
)

// Bounds on concurrent stats history queries, each a few MiB (see
// collector.StatsHistory.Query); a dashboard holds one at a time.
const (
	maxStatsHistoryRequests          = 8
	maxStatsHistoryRequestsPerClient = 4
)

// maxWSClientsPerClient bounds the WebSocket connections of one client
// (each browser tab holds one), within Hub.MaxClients overall.
const maxWSClientsPerClient = 16

// slotLimiter counts concurrent holders of a resource, overall and per
// client.
type slotLimiter struct {
	mu           sync.Mutex
	total        int
	perClient    map[string]int
	maxTotal     int
	maxPerClient int
}

// newSlotLimiter bounds holders to maxTotal overall and maxPerClient per
// client; a bound of 0 or less means none.
func newSlotLimiter(maxTotal, maxPerClient int) *slotLimiter {
	if maxTotal <= 0 {
		maxTotal = math.MaxInt
	}
	if maxPerClient <= 0 {
		maxPerClient = math.MaxInt
	}
	return &slotLimiter{perClient: make(map[string]int), maxTotal: maxTotal, maxPerClient: maxPerClient}
}

// acquire takes a slot for client, or reports false when either bound is
// reached. Each successful acquire must be paired with one release.
func (l *slotLimiter) acquire(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= l.maxTotal {
		return false
	}
	if l.perClient[client] >= l.maxPerClient {
		logPerClientLimit(client)
		return false
	}
	l.total++
	l.perClient[client]++
	return true
}

var perClientLimitOnce sync.Once

// logPerClientLimit says, once, that a client hit a per-client limit:
// behind an unlisted reverse proxy every user is the same client.
func logPerClientLimit(client string) {
	perClientLimitOnce.Do(func() {
		log.Printf("WARN  client %s reached a per-client connection limit; if users reach DockGraph through a reverse proxy, list it in DG_TRUSTED_PROXIES", client)
	})
}

// release returns a slot taken by acquire.
func (l *slotLimiter) release(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.total--
	if l.perClient[client] <= 1 {
		delete(l.perClient, client)
	} else {
		l.perClient[client]--
	}
}

// limitConcurrent serves next only while the client has a free slot in every
// one of limiters, answering 503 otherwise.
func limitConcurrent(clientKey func(*http.Request) string, next http.HandlerFunc, limiters ...*slotLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := clientKey(r)
		for i, l := range limiters {
			if !l.acquire(client) {
				for _, held := range limiters[:i] {
					held.release(client)
				}
				w.Header().Set("Retry-After", "5")
				jsonError(w, "too many concurrent requests; retry shortly", http.StatusServiceUnavailable)
				return
			}
		}
		defer func() {
			for _, l := range limiters {
				l.release(client)
			}
		}()
		next(w, r)
	}
}

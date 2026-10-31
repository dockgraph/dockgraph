package auth

import (
	"container/list"
	"slices"
	"sync"
	"time"
)

// maxRateLimitKeys bounds how many clients the limiter tracks at once, so
// requests from ever-new addresses cannot grow it without limit. When it is
// full of clients seen within the window, the one seen least recently is
// forgotten to make room: refusing new clients instead would let a flood of
// addresses lock everyone else out.
const maxRateLimitKeys = 10000

// RateLimiter implements per-key sliding window rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*list.Element // key to its element of order
	// order holds the buckets most recently active first, so the stale and
	// the least recently active ones are found at the back in constant time
	// rather than by scanning every client.
	order   list.List
	limit   int
	window  time.Duration
	maxKeys int
}

// bucket is the state of one client.
type bucket struct {
	key string
	// times holds the attempts counted against the limit, oldest first, in
	// a slice whose capacity is the limit, so recording one never allocates.
	times []time.Time
	// lastSeen is the latest attempt, allowed or refused: a client that keeps
	// trying while refused is still active and is not the first forgotten.
	lastSeen time.Time
	// refusedAt is the refusal last reported as the first of its window.
	refusedAt time.Time
}

// NewRateLimiter creates a rate limiter allowing limit requests per window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string]*list.Element),
		limit:    limit,
		window:   window,
		maxKeys:  maxRateLimitKeys,
	}
}

// Allow records an attempt and returns true if it is within the limit.
func (rl *RateLimiter) Allow(key string) bool {
	allowed, _ := rl.attempt(key)
	return allowed
}

// attempt is Allow that also reports whether a refused attempt is key's
// first refusal within the window, so that refusals can be logged once per
// client rather than once per request.
func (rl *RateLimiter) attempt(key string) (allowed, firstRefusal bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	rl.removeStale(cutoff)

	b := rl.touch(key, now)
	b.expire(cutoff)
	if len(b.times) >= rl.limit {
		firstRefusal = !b.refusedAt.After(cutoff)
		if firstRefusal {
			b.refusedAt = now
		}
		return false, firstRefusal
	}
	b.times = append(b.times, now)
	return true, false
}

// touch returns key's bucket, marked as the most recently active, creating
// it if key is new. A full table makes room by handing the bucket of the
// least recently active client to key. The caller holds rl.mu.
func (rl *RateLimiter) touch(key string, now time.Time) *bucket {
	e, ok := rl.attempts[key]
	switch {
	case ok:
		rl.order.MoveToFront(e)
	case len(rl.attempts) >= rl.maxKeys:
		e = rl.order.Back()
		b := e.Value.(*bucket)
		delete(rl.attempts, b.key)
		*b = bucket{key: key, times: b.times[:0]}
		rl.order.MoveToFront(e)
		rl.attempts[key] = e
	default:
		e = rl.order.PushFront(&bucket{key: key, times: make([]time.Time, 0, rl.limit)})
		rl.attempts[key] = e
	}
	b := e.Value.(*bucket)
	b.lastSeen = now
	return b
}

// RetryAfter returns seconds until the oldest attempt expires from the window.
func (rl *RateLimiter) RetryAfter(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	e, ok := rl.attempts[key]
	if !ok {
		return 0
	}
	attempts := e.Value.(*bucket).times
	if len(attempts) == 0 {
		return 0
	}
	seconds := int(time.Until(attempts[0].Add(rl.window)).Seconds()) + 1
	if seconds < 1 {
		return 1
	}
	return seconds
}

// Forget withdraws key's most recent attempt, for an attempt that was
// turned away before it could be checked (see ErrBusy).
func (rl *RateLimiter) Forget(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := rl.attempts[key]
	if !ok {
		return
	}
	b := e.Value.(*bucket)
	switch len(b.times) {
	case 0:
	case 1:
		rl.remove(e)
	default:
		b.times = b.times[:len(b.times)-1]
	}
}

// Cleanup removes the clients not seen within the window.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.removeStale(time.Now().Add(-rl.window))
}

// removeStale drops the clients not seen since cutoff, whose attempts have
// all expired. They are at the back of rl.order, so this stops at the first
// client still active. The caller holds rl.mu.
func (rl *RateLimiter) removeStale(cutoff time.Time) {
	for e := rl.order.Back(); e != nil; e = rl.order.Back() {
		if e.Value.(*bucket).lastSeen.After(cutoff) {
			return
		}
		rl.remove(e)
	}
}

// remove drops the client of e. The caller holds rl.mu.
func (rl *RateLimiter) remove(e *list.Element) {
	delete(rl.attempts, e.Value.(*bucket).key)
	rl.order.Remove(e)
}

// expire drops the attempts at or before cutoff, in place.
func (b *bucket) expire(cutoff time.Time) {
	n := 0
	for n < len(b.times) && !b.times[n].After(cutoff) {
		n++
	}
	b.times = slices.Delete(b.times, 0, n)
}

package auth

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Errorf("attempt %d should be allowed", i+1)
		}
	}
}

func TestRateLimiterBlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		rl.Allow("1.2.3.4")
	}
	if rl.Allow("1.2.3.4") {
		t.Error("4th attempt should be blocked")
	}
}

func TestRateLimiterPerIP(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	rl.Allow("1.1.1.1")
	if rl.Allow("1.1.1.1") {
		t.Error("same IP should be blocked")
	}
	if !rl.Allow("2.2.2.2") {
		t.Error("different IP should be allowed")
	}
}

func TestRateLimiterRetryAfter(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	rl.Allow("1.2.3.4")
	rl.Allow("1.2.3.4")
	seconds := rl.RetryAfter("1.2.3.4")
	if seconds < 1 || seconds > 61 {
		t.Errorf("RetryAfter = %d, want 1-61", seconds)
	}
}

func TestRateLimiterRetryAfterUnknownIP(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	if s := rl.RetryAfter("unknown"); s != 0 {
		t.Errorf("RetryAfter for unknown IP = %d, want 0", s)
	}
}

// A full table makes room by forgetting the client seen least recently, so
// a flood of new addresses can't lock other clients out.
func TestRateLimiterBoundsTrackedKeys(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	rl.maxKeys = 2
	for _, k := range []string{"a", "b"} {
		if !rl.Allow(k) {
			t.Fatalf("client %s should be allowed", k)
		}
		time.Sleep(time.Millisecond) // distinct timestamps
	}
	if !rl.Allow("c") {
		t.Fatal("a new client should be allowed while the table is full")
	}
	if n := len(rl.attempts); n != 2 {
		t.Errorf("tracking %d keys, want 2", n)
	}
	if _, kept := rl.attempts["a"]; kept {
		t.Error("the least recently seen client should have been forgotten")
	}
	if rl.Allow("b") {
		t.Error("a tracked client keeps its spent budget")
	}
}

// A refused attempt is activity too: a client that keeps trying is not the
// one forgotten to make room, a client that went quiet is.
func TestRateLimiterRefusedAttemptKeepsClientTracked(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	rl.maxKeys = 2
	rl.Allow("a")
	rl.Allow("b")
	if rl.Allow("a") {
		t.Fatal("a's second attempt should be refused")
	}
	rl.Allow("c")
	if _, kept := rl.attempts["a"]; !kept {
		t.Error("a client still trying should stay tracked")
	}
	if _, kept := rl.attempts["b"]; kept {
		t.Error("the client quiet the longest should have been forgotten")
	}
}

// Making room in a full table is constant work: a stream of new clients
// must not make every attempt walk, or reallocate, the whole table under
// the lock.
func TestRateLimiterFullTableAllowDoesNotAllocate(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	keys := make([]string, maxRateLimitKeys+200)
	for i := range keys {
		keys[i] = "client" + strconv.Itoa(i)
	}
	for _, k := range keys[:maxRateLimitKeys] {
		rl.Allow(k)
	}

	next := maxRateLimitKeys
	if n := testing.AllocsPerRun(100, func() {
		rl.Allow(keys[next])
		next++
	}); n != 0 {
		t.Errorf("Allow for a new client on a full table: %.0f allocations, want 0", n)
	}
	if n := testing.AllocsPerRun(100, func() { rl.Allow(keys[next-1]) }); n != 0 {
		t.Errorf("Allow for a known client: %.0f allocations, want 0", n)
	}
	if n := len(rl.attempts); n != maxRateLimitKeys {
		t.Errorf("tracking %d keys, want %d", n, maxRateLimitKeys)
	}
}

// Each client's first refusal in a window is reported, and only that one,
// so refusals can be logged without one client filling the log.
func TestRateLimiterReportsFirstRefusalPerWindow(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)
	check := func(step string, wantAllowed, wantFirst bool) {
		t.Helper()
		allowed, first := rl.attempt("a")
		if allowed != wantAllowed || first != wantFirst {
			t.Errorf("%s: allowed %v, first refusal %v; want %v, %v", step, allowed, first, wantAllowed, wantFirst)
		}
	}
	check("first attempt", true, false)
	check("over the limit", false, true)
	check("still over the limit", false, false)
	time.Sleep(60 * time.Millisecond)
	check("next window", true, false)
	check("over the limit again", false, true)
}

// Concurrent attempts, refunds and cleanups keep the table consistent and
// within its bound.
func TestRateLimiterConcurrentUse(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	rl.maxKeys = 50
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range 2000 {
				key := strconv.Itoa((g*31 + i) % 120)
				if rl.Allow(key) && i%3 == 0 {
					rl.Forget(key)
				}
				rl.RetryAfter(key)
				if i%500 == 0 {
					rl.Cleanup()
				}
			}
		})
	}
	wg.Wait()
	if n := len(rl.attempts); n > rl.maxKeys || n != rl.order.Len() {
		t.Errorf("tracking %d keys in an order of %d, want equal and at most %d", n, rl.order.Len(), rl.maxKeys)
	}
}

func TestRateLimiterForget(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	rl.Allow("a")
	rl.Forget("a")
	if !rl.Allow("a") {
		t.Error("a forgotten attempt must not count")
	}
	rl.Forget("unknown") // no-op
}

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)
	rl.Allow("1.2.3.4")
	if rl.Allow("1.2.3.4") {
		t.Error("should be blocked before cleanup")
	}
	time.Sleep(60 * time.Millisecond)
	rl.Cleanup()
	if n := len(rl.attempts); n != 0 {
		t.Errorf("tracking %d keys after cleanup, want 0", n)
	}
	if !rl.Allow("1.2.3.4") {
		t.Error("should be allowed after window expires and cleanup")
	}
}

package auth

import (
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

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)
	rl.Allow("1.2.3.4")
	if rl.Allow("1.2.3.4") {
		t.Error("should be blocked before cleanup")
	}
	time.Sleep(60 * time.Millisecond)
	rl.Cleanup()
	if !rl.Allow("1.2.3.4") {
		t.Error("should be allowed after window expires and cleanup")
	}
}

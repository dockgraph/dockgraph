package auth

import (
	"sync"
	"time"
)

// RateLimiter implements per-key sliding window rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a rate limiter allowing limit requests per window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow records an attempt and returns true if it is within the limit.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	recent := filterAfter(rl.attempts[key], cutoff)

	if len(recent) >= rl.limit {
		rl.attempts[key] = recent
		return false
	}
	rl.attempts[key] = append(recent, now)
	return true
}

// RetryAfter returns seconds until the oldest attempt expires from the window.
func (rl *RateLimiter) RetryAfter(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	attempts := rl.attempts[key]
	if len(attempts) == 0 {
		return 0
	}
	seconds := int(time.Until(attempts[0].Add(rl.window)).Seconds()) + 1
	if seconds < 1 {
		return 1
	}
	return seconds
}

// Cleanup removes stale entries older than the window.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for key, attempts := range rl.attempts {
		recent := filterAfter(attempts, cutoff)
		if len(recent) == 0 {
			delete(rl.attempts, key)
		} else {
			rl.attempts[key] = recent
		}
	}
}

func filterAfter(times []time.Time, cutoff time.Time) []time.Time {
	var result []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			result = append(result, t)
		}
	}
	return result
}

package auth

import (
	"sync"
	"time"
)

// SessionManager tracks session invalidation via a timestamp.
// All tokens issued before the invalidation time are considered invalid.
type SessionManager struct {
	mu            sync.RWMutex
	invalidatedAt time.Time
}

// NewSessionManager creates a session manager with no invalidation.
func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

// Invalidate marks all tokens issued before now as invalid.
func (s *SessionManager) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidatedAt = time.Now()
}

// IsValid returns true if a token issued at the given time is still valid.
func (s *SessionManager) IsValid(issuedAt time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.invalidatedAt.IsZero() || issuedAt.After(s.invalidatedAt)
}

// InvalidatedAt returns the time of the last invalidation, or zero.
func (s *SessionManager) InvalidatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.invalidatedAt
}

package auth

import (
	"testing"
	"time"
)

func TestSessionManagerDefaultValid(t *testing.T) {
	sm := NewSessionManager()
	if !sm.IsValid(time.Now()) {
		t.Error("new session manager should consider all tokens valid")
	}
}

func TestSessionManagerInvalidate(t *testing.T) {
	sm := NewSessionManager()
	before := time.Now().Add(-time.Second)
	sm.Invalidate()
	if sm.IsValid(before) {
		t.Error("token issued before invalidation should be invalid")
	}
}

func TestSessionManagerValidAfterInvalidation(t *testing.T) {
	sm := NewSessionManager()
	sm.Invalidate()
	time.Sleep(10 * time.Millisecond)
	after := time.Now()
	if !sm.IsValid(after) {
		t.Error("token issued after invalidation should be valid")
	}
}

func TestSessionManagerInvalidatedAt(t *testing.T) {
	sm := NewSessionManager()
	if !sm.InvalidatedAt().IsZero() {
		t.Error("new session manager should have zero invalidation time")
	}
	sm.Invalidate()
	if sm.InvalidatedAt().IsZero() {
		t.Error("after invalidation, time should be non-zero")
	}
}

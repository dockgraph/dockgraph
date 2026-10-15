package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
)

func TestResolveMode(t *testing.T) {
	manager := swarm.Info{LocalNodeState: swarm.LocalNodeStateActive, ControlAvailable: true}
	worker := swarm.Info{LocalNodeState: swarm.LocalNodeStateActive}
	none := swarm.Info{LocalNodeState: swarm.LocalNodeStateInactive}

	tests := []struct {
		name      string
		requested string
		info      swarm.Info
		want      string
		wantErr   bool
	}{
		{"auto_manager", "auto", manager, "swarm", false},
		{"auto_no_swarm", "auto", none, "standalone", false},
		{"auto_unknown_state_rejected", "auto", swarm.Info{}, "", true},
		{"auto_locked_rejected", "auto", swarm.Info{LocalNodeState: swarm.LocalNodeStateLocked}, "", true},
		{"auto_pending_rejected", "auto", swarm.Info{LocalNodeState: swarm.LocalNodeStatePending}, "", true},
		{"auto_error_rejected", "auto", swarm.Info{LocalNodeState: swarm.LocalNodeStateError}, "", true},
		{"auto_worker_rejected", "auto", worker, "", true},
		{"swarm_manager", "swarm", manager, "swarm", false},
		{"swarm_worker_rejected", "swarm", worker, "", true},
		{"swarm_no_swarm_rejected", "swarm", none, "", true},
		{"standalone_forced_on_manager", "standalone", manager, "standalone", false},
		{"agent_on_worker", "agent", worker, "agent", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMode(tt.requested, tt.info)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// flakyInfo fails the first n calls, then reports a swarm manager.
func flakyInfo(n int) (func(context.Context) (system.Info, error), *int) {
	calls := 0
	return func(context.Context) (system.Info, error) {
		calls++
		if calls <= n {
			return system.Info{}, errors.New("daemon not ready")
		}
		return system.Info{
			Name:  "mgr",
			Swarm: swarm.Info{LocalNodeState: swarm.LocalNodeStateActive, ControlAvailable: true},
		}, nil
	}, &calls
}

var fastRetry = retryPolicy{attempts: 3, timeout: time.Second, backoff: time.Millisecond}

func TestDetectModeRetriesTransientInfoFailure(t *testing.T) {
	info, calls := flakyInfo(2)
	mode, sw, host, err := detectMode(context.Background(), "auto", info, fastRetry)
	if err != nil {
		t.Fatal(err)
	}
	if mode != "swarm" || !sw.ControlAvailable || host != "mgr" || *calls != 3 {
		t.Errorf("mode %q host %q calls %d", mode, host, *calls)
	}
}

func TestDetectModeInfoFailure(t *testing.T) {
	tests := []struct {
		requested string
		want      string
		wantErr   bool
	}{
		{"auto", "", true}, // never guessed as standalone
		{"swarm", "", true},
		{"standalone", "standalone", false},
		{"agent", "agent", false},
	}
	for _, tt := range tests {
		t.Run(tt.requested, func(t *testing.T) {
			info, calls := flakyInfo(fastRetry.attempts)
			mode, _, _, err := detectMode(context.Background(), tt.requested, info, fastRetry)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if mode != tt.want {
				t.Errorf("mode %q, want %q", mode, tt.want)
			}
			if *calls != fastRetry.attempts {
				t.Errorf("calls %d, want %d", *calls, fastRetry.attempts)
			}
		})
	}
}

func TestDetectModeStopsRetryingOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	info, calls := flakyInfo(100)
	slow := retryPolicy{attempts: 5, timeout: time.Second, backoff: time.Hour}
	if _, _, _, err := detectMode(ctx, "auto", info, slow); err == nil {
		t.Fatal("expected error")
	}
	if *calls != 1 {
		t.Errorf("calls %d, want 1", *calls)
	}
}

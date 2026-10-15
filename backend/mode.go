package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	"github.com/dockgraph/dockgraph/collector"
)

// resolveMode turns the configured DG_MODE into the effective runtime mode
// given the daemon's swarm state. Auto picks swarm on an active manager and
// standalone only when the node is known to be outside a swarm (inactive).
// Any other state (pending, locked, error, or unknown) is rejected rather
// than guessed as standalone, which would hide the cluster. A swarm worker
// cannot see the cluster, so it is rejected unless running as an agent;
// explicit swarm mode requires a manager. Explicit standalone is always
// honoured.
func resolveMode(requested string, sw swarm.Info) (string, error) {
	active := sw.LocalNodeState == swarm.LocalNodeStateActive
	manager := active && sw.ControlAvailable

	switch requested {
	case collector.ModeAgent, collector.ModeStandalone:
		return requested, nil
	case collector.ModeSwarm:
		if !manager {
			return "", fmt.Errorf("DG_MODE=swarm requires a swarm manager node (local node state %q)", sw.LocalNodeState)
		}
		return collector.ModeSwarm, nil
	default:
		if manager {
			return collector.ModeSwarm, nil
		}
		if active {
			return "", fmt.Errorf("this node is a swarm worker: run dockgraph on a manager, or set DG_MODE=agent (or DG_MODE=standalone for local-only view)")
		}
		if sw.LocalNodeState == swarm.LocalNodeStateInactive {
			return collector.ModeStandalone, nil
		}
		return "", fmt.Errorf("cannot pick a mode: local swarm node state is %q (set DG_MODE=standalone or DG_MODE=swarm explicitly)", sw.LocalNodeState)
	}
}

// retryPolicy bounds the daemon info query at startup.
type retryPolicy struct {
	attempts int           // total tries, at least 1
	timeout  time.Duration // per try
	backoff  time.Duration // first wait between tries, doubled each time
}

// infoRetry rides out a daemon that is still starting (about 30 seconds in
// total) before giving up.
var infoRetry = retryPolicy{attempts: 5, timeout: 10 * time.Second, backoff: 2 * time.Second}

// detectMode resolves DG_MODE against the daemon's swarm state, retrying the
// info query on failure. Auto and swarm need that state, so a query that
// keeps failing is an error: guessing standalone would silently hide the
// cluster. Standalone and agent don't depend on it and continue with a
// warning.
//
// It also returns the daemon's hostname (Info().Name). Swarm fills a node's
// Description.Hostname from that same value, and per-node agents report it
// too, so it matches the hostname swarm node graph nodes and per-node stats
// aggregates are keyed by without an extra NodeInspect call.
func detectMode(ctx context.Context, requested string, info func(context.Context) (system.Info, error), policy retryPolicy) (string, swarm.Info, string, error) {
	raw, err := queryInfo(ctx, info, policy)
	if err != nil {
		if requested == collector.ModeStandalone || requested == collector.ModeAgent {
			log.Printf("WARN  failed to query docker info: %v", err)
			return requested, swarm.Info{}, "", nil
		}
		return "", swarm.Info{}, "", fmt.Errorf("query docker info (needed for DG_MODE=%s): %w", requested, err)
	}
	mode, err := resolveMode(requested, raw.Swarm)
	if err != nil {
		return "", swarm.Info{}, "", err
	}
	return mode, raw.Swarm, raw.Name, nil
}

// queryInfo calls info up to policy.attempts times with exponential backoff,
// returning the last error if every try fails or ctx ends.
func queryInfo(ctx context.Context, info func(context.Context) (system.Info, error), policy retryPolicy) (system.Info, error) {
	wait := policy.backoff
	var lastErr error
	for attempt := 1; ; attempt++ {
		tryCtx, cancel := context.WithTimeout(ctx, policy.timeout)
		raw, err := info(tryCtx)
		cancel()
		if err == nil {
			return raw, nil
		}
		lastErr = err
		if attempt >= policy.attempts {
			return system.Info{}, lastErr
		}
		log.Printf("WARN  docker info failed (attempt %d/%d), retrying in %s: %v", attempt, policy.attempts, wait, err)
		select {
		case <-ctx.Done():
			return system.Info{}, lastErr
		case <-time.After(wait):
		}
		wait *= 2
	}
}

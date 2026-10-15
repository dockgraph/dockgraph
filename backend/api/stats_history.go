package api

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/dockgraph/dockgraph/collector"
)

// scopeNodes selects the per-node aggregate series of the stats history.
const scopeNodes = "nodes"

var validRanges = map[string]time.Duration{
	"5m":  5 * time.Minute,
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
}

// HandleStatsHistory returns a handler for GET /api/stats/history?range={5m|1h|6h|24h}.
// An optional ?stack= keeps only the series of containers belonging to that
// compose project / swarm stack. History is keyed by container name only, so
// stack membership is resolved from the current container list (lister); a
// container that has since been removed can no longer be attributed. In swarm
// mode (services non-nil) the stack's service aggregates, keyed by service
// name, and its task containers on any node ({service}.*) are kept too.
//
// Per-node aggregates (collector.NodeStatsPrefix keys) are excluded by
// default so workload charts are unchanged; ?scope=nodes returns only them.
// Node history is not per-stack, so scope=nodes with ?stack= is rejected.
func HandleStatsHistory(history *collector.StatsHistory, lister ContainerLister, services ServiceLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rangeStr := r.URL.Query().Get("range")
		if rangeStr == "" {
			rangeStr = "1h"
		}

		dur, ok := validRanges[rangeStr]
		if !ok {
			jsonError(w, "invalid range: use 5m, 1h, 6h, or 24h", http.StatusBadRequest)
			return
		}

		stack := r.URL.Query().Get("stack")
		if stack != "" && !validStackName.MatchString(stack) {
			jsonError(w, "invalid stack", http.StatusBadRequest)
			return
		}

		scope := r.URL.Query().Get("scope")
		if scope != "" && scope != scopeNodes {
			jsonError(w, "invalid scope: use nodes", http.StatusBadRequest)
			return
		}
		if scope == scopeNodes && stack != "" {
			jsonError(w, "scope=nodes cannot be combined with stack", http.StatusBadRequest)
			return
		}

		now := time.Now()
		result := history.Query(now.Add(-dur), now)

		wantNodes := scope == scopeNodes
		for name := range result.Containers {
			if collector.IsNodeSeries(name) != wantNodes {
				delete(result.Containers, name)
			}
		}

		if stack != "" {
			members, err := stackContainerNames(r.Context(), lister, stack)
			if err != nil {
				jsonError(w, "failed to resolve stack", http.StatusInternalServerError)
				return
			}
			svcNames, err := stackServiceNames(r.Context(), services, stack)
			if err != nil {
				jsonError(w, "failed to resolve stack", http.StatusInternalServerError)
				return
			}
			for name := range result.Containers {
				if !members[name] && !isServiceSeries(name, svcNames) {
					delete(result.Containers, name)
				}
			}
		}

		resolution := 3
		if dur > time.Hour {
			resolution = 30
		}

		resp := map[string]any{
			"range":      rangeStr,
			"resolution": resolution,
			"timestamps": result.Timestamps,
			"containers": result.Containers,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// stackContainerNames returns the names of all containers (running or not)
// whose compose project or stack namespace is stack.
func stackContainerNames(ctx context.Context, lister ContainerLister, stack string) (map[string]bool, error) {
	names := make(map[string]bool)
	if lister == nil {
		return names, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	summaries, err := lister.ContainerList(ctx, containertypes.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	for _, c := range summaries {
		if collector.ProjectOf(c.Labels) == stack {
			names[containerDisplayName(c)] = true
		}
	}
	return names, nil
}

// stackServiceNames returns the names of the swarm services deployed in stack.
func stackServiceNames(ctx context.Context, services ServiceLister, stack string) (map[string]bool, error) {
	names := make(map[string]bool)
	if services == nil {
		return names, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	list, err := services.ServiceList(ctx, swarm.ServiceListOptions{})
	if err != nil {
		return nil, err
	}
	for _, svc := range list {
		if collector.ProjectOf(svc.Spec.Labels) == stack {
			names[svc.Spec.Name] = true
		}
	}
	return names, nil
}

// taskSuffix matches what follows "{service}." in a swarm task container
// name: the slot (replicated) or the node ID (global), then the task ID.
// Swarm object IDs are 25 lowercase base-36 characters.
var taskSuffix = regexp.MustCompile(`^(?:[0-9]+|[a-z0-9]{25})\.[a-z0-9]{25}$`)

// isServiceSeries reports whether a stats series belongs to one of the
// services: its aggregate (the service name) or one of its task containers
// ({service}.{slot|nodeID}.{taskID}). Container names may contain dots, so a
// bare "{service}." prefix is not enough: "shop_api.backup" is not a task.
func isServiceSeries(name string, services map[string]bool) bool {
	if services[name] {
		return true
	}
	for svc := range services {
		prefix := svc + "."
		if strings.HasPrefix(name, prefix) && taskSuffix.MatchString(strings.TrimPrefix(name, prefix)) {
			return true
		}
	}
	return false
}

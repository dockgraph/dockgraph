package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/dockgraph/dockgraph/collector"
)

// ManagerLister is the subset of the swarm API needed to read the managers.
type ManagerLister interface {
	NodeList(ctx context.Context, options swarm.NodeListOptions) ([]swarm.Node, error)
}

// swarmManager is one manager's place in the swarm's raft consensus.
type swarmManager struct {
	// ID is the swarm node ID. Unlike the name, it is unique: a manager that
	// left and rejoined keeps a stale entry, still a raft member, under the
	// same hostname.
	ID string `json:"id"`
	// Name is the node's hostname, as the graph names it (swarmnode:{name}).
	Name   string `json:"name"`
	Leader bool   `json:"leader"`
	// Reachability is "reachable", "unreachable" or "unknown".
	Reachability string `json:"reachability"`
	// Addr is the manager's raft address (host:port).
	Addr string `json:"addr,omitempty"`
}

// managersTimeout bounds the lookup: without quorum the swarm may be slow to
// fail, and the panel asks again every few seconds.
const managersTimeout = 5 * time.Second

// noLeaderMessage is the swarm's error when no manager leads: fewer than a
// majority of the managers are reachable, so the swarm can't change anything.
const noLeaderMessage = "does not have a leader"

// HandleSwarmManagers returns a handler for GET /api/swarm/managers: each
// manager's raft reachability and the leader, read from the swarm on every
// request. The graph keeps its last snapshot while the swarm API doesn't
// answer, so it can't tell that quorum is lost; this endpoint can.
func HandleSwarmManagers(lister ManagerLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), managersTimeout)
		defer cancel()

		nodes, err := lister.NodeList(ctx, swarm.NodeListOptions{
			Filters: filters.NewArgs(filters.Arg("role", string(swarm.NodeRoleManager))),
		})
		if err != nil {
			log.Printf("swarm managers: %v", err)
			if strings.Contains(err.Error(), noLeaderMessage) {
				jsonError(w, "the swarm has no leader: fewer than a majority of its managers are reachable", http.StatusServiceUnavailable)
				return
			}
			jsonError(w, "swarm managers unavailable", http.StatusBadGateway)
			return
		}

		managers := make([]swarmManager, 0, len(nodes))
		for _, n := range nodes {
			m := swarmManager{ID: n.ID, Name: collector.SwarmNodeName(n), Reachability: string(swarm.ReachabilityUnknown)}
			if s := n.ManagerStatus; s != nil {
				m.Leader = s.Leader
				m.Addr = s.Addr
				if s.Reachability != "" {
					m.Reachability = string(s.Reachability)
				}
			}
			managers = append(managers, m)
		}
		sort.Slice(managers, func(i, j int) bool {
			if managers[i].Leader != managers[j].Leader {
				return managers[i].Leader
			}
			if managers[i].Name != managers[j].Name {
				return managers[i].Name < managers[j].Name
			}
			return managers[i].ID < managers[j].ID
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"managers": managers})
	}
}

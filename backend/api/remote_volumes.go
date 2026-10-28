package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/dockgraph/dockgraph/collector"
)

// volumeLookupTimeout bounds the search for a volume across the swarm
// nodes, so a frozen node cannot hold the detail panel. A variable so tests
// can lower it.
var volumeLookupTimeout = 5 * time.Second

// RemoteAgentLister lists the agents on other swarm nodes.
// *collector.AgentPool implements it.
type RemoteAgentLister interface {
	RemoteAgents() []collector.RemoteAgent
}

// volumeLookup is one node's answer to a volume search: the volume's
// inspect response when the node has it, or the error that kept it from
// answering.
type volumeLookup struct {
	node string
	resp map[string]any
	err  error
}

// inspectVolume returns the volume inspect handler. Swarm volumes are
// node-local: a service's named volume exists on every node that ran one of
// its tasks, and the graph shows it whether or not the server's node has
// it. With agents, every node is asked at once. The response describes the
// server's own copy when it has one, otherwise the first node's by
// hostname; "node" names that node and "nodes" lists every node holding a
// volume of that name.
func (p *AgentProxy) inspectVolume(inspector VolumeInspector) http.HandlerFunc {
	if p == nil || p.Agents == nil {
		return HandleVolumeInspect(inspector)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !validResourceName.MatchString(name) {
			jsonError(w, "invalid volume name", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), volumeLookupTimeout)
		defer cancel()

		agents := p.Agents.RemoteAgents()
		// The local daemon goes first so its copy is preferred; agents come
		// sorted by hostname.
		lookups := make([]volumeLookup, len(agents)+1)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			lookups[0] = p.lookupLocalVolume(ctx, inspector, name)
		}()
		for i, a := range agents {
			wg.Add(1)
			go func() {
				defer wg.Done()
				lookups[i+1] = p.lookupRemoteVolume(ctx, a, name)
			}()
		}
		wg.Wait()

		var found []volumeLookup
		failed, rejected := false, p.tokenRejected()
		for _, l := range lookups {
			switch {
			case l.err != nil:
				log.Printf("inspect volume %s on %s: %v", name, l.node, l.err)
				failed = true
				rejected = rejected || errors.Is(l.err, errAgentAuth)
			case l.resp != nil:
				found = append(found, l)
			}
		}
		if len(found) == 0 {
			// A node that did not answer, or whose agent refused the token,
			// may hold the volume.
			switch {
			case rejected:
				jsonError(w, errAgentAuth.Error(), http.StatusBadGateway)
			case failed:
				jsonError(w, "volume not found on the reachable nodes", http.StatusBadGateway)
			default:
				jsonError(w, "volume not found", http.StatusNotFound)
			}
			return
		}

		nodes := make([]string, 0, len(found))
		for _, l := range found {
			nodes = append(nodes, l.node)
		}
		sort.Strings(nodes)
		resp := found[0].resp
		resp["node"] = found[0].node
		resp["nodes"] = nodes

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// lookupLocalVolume inspects the volume on the server's own daemon.
func (p *AgentProxy) lookupLocalVolume(ctx context.Context, inspector VolumeInspector, name string) volumeLookup {
	l := volumeLookup{node: p.LocalHostname}
	vol, err := inspector.VolumeInspect(ctx, name)
	switch {
	case err == nil:
		l.resp = buildVolumeResponse(vol)
	case !cerrdefs.IsNotFound(err):
		l.err = err
	}
	return l
}

// lookupRemoteVolume asks one agent for the volume. A 404 means the node
// does not have it, which is not an error.
func (p *AgentProxy) lookupRemoteVolume(ctx context.Context, a collector.RemoteAgent, name string) volumeLookup {
	l := volumeLookup{node: a.Hostname}
	target := a.BaseURL + collector.AgentAPIPrefix + "/volumes/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		l.err = err
		return l
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.transport().RoundTrip(req)
	if err != nil {
		l.err = err
		return l
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxRemoteResponseBytes)).Decode(&l.resp); err != nil {
			l.err = fmt.Errorf("decoding agent response: %w", err)
		}
	case http.StatusNotFound:
	case http.StatusUnauthorized, http.StatusForbidden:
		l.err = errAgentAuth
	default:
		l.err = fmt.Errorf("agent returned status %d", resp.StatusCode)
	}
	return l
}

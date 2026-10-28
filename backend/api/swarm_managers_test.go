package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types/swarm"
)

// stubManagerLister implements ManagerLister for tests and records the options.
type stubManagerLister struct {
	nodes []swarm.Node
	err   error
	opts  swarm.NodeListOptions
}

func (s *stubManagerLister) NodeList(_ context.Context, opts swarm.NodeListOptions) ([]swarm.Node, error) {
	s.opts = opts
	return s.nodes, s.err
}

func managerNode(id, hostname string, leader bool, reachability swarm.Reachability) swarm.Node {
	n := swarm.Node{ID: id}
	n.Description.Hostname = hostname
	n.Spec.Role = swarm.NodeRoleManager
	n.ManagerStatus = &swarm.ManagerStatus{Leader: leader, Reachability: reachability, Addr: "10.0.0.1:2377"}
	return n
}

func getSwarmManagers(t *testing.T, lister ManagerLister) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/swarm/managers", HandleSwarmManagers(lister))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/swarm/managers", nil))
	return w
}

func TestHandleSwarmManagers_ListsManagersLeaderFirst(t *testing.T) {
	noStatus := swarm.Node{ID: "n4"} // no hostname, no manager status yet
	noStatus.Spec.Role = swarm.NodeRoleManager
	lister := &stubManagerLister{nodes: []swarm.Node{
		managerNode("n2", "m-b", false, swarm.ReachabilityReachable),
		managerNode("n1", "m-a", false, swarm.ReachabilityUnreachable),
		managerNode("n3", "m-c", true, swarm.ReachabilityReachable),
		noStatus,
	}}

	w := getSwarmManagers(t, lister)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !lister.opts.Filters.ExactMatch("role", "manager") || lister.opts.Filters.Len() != 1 {
		t.Errorf("filters = %v, want role=manager", lister.opts.Filters)
	}

	var body struct {
		Managers []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Leader       bool   `json:"leader"`
			Reachability string `json:"reachability"`
			Addr         string `json:"addr"`
		} `json:"managers"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var got []string
	for _, m := range body.Managers {
		got = append(got, m.Name+":"+m.Reachability)
	}
	// Leader first, then by name; named like the graph (ID without a hostname).
	want := []string{"m-c:reachable", "m-a:unreachable", "m-b:reachable", "n4:unknown"}
	if len(got) != len(want) {
		t.Fatalf("managers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("managers = %v, want %v", got, want)
		}
	}
	if !body.Managers[0].Leader || body.Managers[1].Leader || body.Managers[0].Addr != "10.0.0.1:2377" {
		t.Errorf("leader flags or addr wrong: %+v", body.Managers)
	}
	// Node IDs tell apart entries that share a hostname (a stale one left by a rejoin).
	if body.Managers[0].ID != "n3" || body.Managers[3].ID != "n4" {
		t.Errorf("ids = %+v, want n3 first and n4 last", body.Managers)
	}
}

func TestHandleSwarmManagers_KeepsManagersSharingAHostname(t *testing.T) {
	// A manager that left and rejoined: its old entry is still a raft member,
	// so both count towards quorum.
	lister := &stubManagerLister{nodes: []swarm.Node{
		managerNode("old", "m2", false, swarm.ReachabilityUnreachable),
		managerNode("new", "m2", true, swarm.ReachabilityReachable),
	}}
	w := getSwarmManagers(t, lister)
	var body struct {
		Managers []struct {
			ID string `json:"id"`
		} `json:"managers"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Managers) != 2 || body.Managers[0].ID != "new" || body.Managers[1].ID != "old" {
		t.Errorf("managers = %+v, want new (leader) then old", body.Managers)
	}
}

func TestHandleSwarmManagers_Errors(t *testing.T) {
	noLeader := errors.New("rpc error: code = Unknown desc = The swarm does not have a leader. " +
		"It's possible that too few managers are online. Make sure more than half of the managers are online.")
	for name, tc := range map[string]struct {
		err      error
		wantCode int
		wantMsg  string
	}{
		"quorum lost": {noLeader, http.StatusServiceUnavailable, "the swarm has no leader: fewer than a majority of its managers are reachable"},
		"other":       {context.DeadlineExceeded, http.StatusBadGateway, "swarm managers unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			w := getSwarmManagers(t, &stubManagerLister{err: tc.err})
			var body map[string]string
			_ = json.NewDecoder(w.Body).Decode(&body)
			if w.Code != tc.wantCode || body["error"] != tc.wantMsg {
				t.Errorf("got %d %q, want %d %q", w.Code, body["error"], tc.wantCode, tc.wantMsg)
			}
		})
	}
}

package collector

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
)

// captureLog redirects the standard logger for the test and resets the
// warnings already logged, so each test starts fresh.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	warned.Clear()
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		warned.Clear()
	})
	return &buf
}

func TestBuildStatsSnapshotWarnsOnceOnNameCollision(t *testing.T) {
	buf := captureLog(t)
	samples := []ContainerSample{
		{ID: "c1", Name: "shop_web", Stats: ContainerStats{CPUPercent: 1}}, // standalone container
		{ID: "c2", Name: "shop_web.1.task1", Labels: map[string]string{swarmServiceNameLabel: "shop_web"}, Stats: ContainerStats{CPUPercent: 5}},
	}
	for range 3 { // one warning across polls
		BuildStatsSnapshot(samples)
	}
	if n := strings.Count(buf.String(), `both named "shop_web"`); n != 1 {
		t.Errorf("warnings = %d, want 1; log:\n%s", n, buf.String())
	}
}

func TestBuildSwarmNodeNodesWarnsOnlyForTwoLiveNodes(t *testing.T) {
	node := func(id string, state swarm.NodeState) swarm.Node {
		n := swarm.Node{ID: id}
		n.Description.Hostname = "worker-1"
		n.Status.State = state
		return n
	}

	t.Run("stale entry of a rejoined node", func(t *testing.T) {
		buf := captureLog(t)
		buildSwarmNodeNodes([]swarm.Node{node("old", swarm.NodeStateDown), node("new", swarm.NodeStateReady)})
		if buf.Len() != 0 {
			t.Errorf("unexpected warning: %s", buf.String())
		}
	})

	t.Run("two ready nodes", func(t *testing.T) {
		buf := captureLog(t)
		for range 2 {
			buildSwarmNodeNodes([]swarm.Node{node("a", swarm.NodeStateReady), node("b", swarm.NodeStateReady)})
		}
		if n := strings.Count(buf.String(), `hostname "worker-1"`); n != 1 {
			t.Errorf("warnings = %d, want 1; log:\n%s", n, buf.String())
		}
	})
}

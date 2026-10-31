package collector

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
)

// remoteStub is a RemoteSampler returning fixed samples and reporting nodes.
type remoteStub struct {
	samples []ContainerSample
	nodes   []string
}

func (r remoteStub) Samples(context.Context) []ContainerSample { return r.samples }

func (r remoteStub) ReportingNodes() []string { return r.nodes }

func TestStatsCollectorEmitsIdleReportingNodes(t *testing.T) {
	sc := NewStatsCollector(&stubDockerClient{}, time.Second, 2)
	sc.SetLocalNode("n-local", "mgr")
	sc.SetRemote(remoteStub{nodes: []string{"idle-wrk"}})

	snap := sc.poll(context.Background())
	for _, key := range []string{"node:mgr", "node:idle-wrk"} {
		if got, ok := snap.Stats[key]; !ok || got != (ContainerStats{}) {
			t.Errorf("%s = %+v (present %v), want zero entry", key, got, ok)
		}
	}
}

func TestStatsCollectorMergesRemoteWithoutDoubleCounting(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: containerSummaries("aaaaaaaaaaaa", "shop_web.1.t1", map[string]string{
			swarmTaskIDLabel: "t1", swarmServiceNameLabel: "shop_web",
		}),
		statsFn: func(string) (containerStatsReader, error) { return fakeStatsBody(stats), nil },
	}
	sc := NewStatsCollector(cli, time.Hour, 2)
	sc.SetRemote(remoteStub{samples: []ContainerSample{
		// Same container reported by the local agent: must be ignored.
		taskSample("aaaaaaaaaaaa", "shop_web.1.t1", "shop_web", "t1", 999, 999),
		taskSample("bbbbbbbbbbbb", "shop_web.2.t2", "shop_web", "t2", 5, 300),
	}})

	snap := sc.poll(context.Background())
	local := snap.Stats["shop_web.1.t1"]
	if local.CPUPercent == 999 {
		t.Fatal("remote duplicate overrode local sample")
	}
	agg, ok := snap.Stats["shop_web"]
	if !ok {
		t.Fatalf("missing service aggregate: %v", snap.Stats)
	}
	if agg.CPUPercent != local.CPUPercent+5 || agg.MemUsage != local.MemUsage+300 {
		t.Errorf("aggregate %+v, local %+v", agg, local)
	}
	if _, ok := snap.Stats["shop_web.2.t2"]; !ok {
		t.Error("missing remote task stats")
	}
}

func TestBuildStatsSnapshotAggregates(t *testing.T) {
	snap := BuildStatsSnapshot([]ContainerSample{
		taskSample("a", "svc.1.x", "svc", "x", 10, 100),
		{
			ID: "b", Name: "svc.2.y", Labels: map[string]string{swarmServiceNameLabel: "svc"},
			Stats: ContainerStats{CPUPercent: 5, CPUThrottled: 40, MemUsage: 50, MemLimit: 1000, NetRx: 7, PIDs: 3},
		},
		{ID: "c", Name: "standalone", Stats: ContainerStats{CPUPercent: 1}},
		// A container named like a service keeps its own entry.
		{ID: "d", Name: "other", Stats: ContainerStats{CPUPercent: 2}},
		taskSample("e", "other.1.z", "other", "z", 30, 30),
	})
	agg := snap.Stats["svc"]
	want := ContainerStats{CPUPercent: 15, CPUThrottled: 40, MemUsage: 150, MemLimit: 2000, NetRx: 7, PIDs: 3}
	if agg != want {
		t.Errorf("aggregate = %+v, want %+v", agg, want)
	}
	if snap.Stats["other"].CPUPercent != 2 {
		t.Errorf("container entry clobbered by aggregate: %+v", snap.Stats["other"])
	}
	if len(snap.Stats) != 6 {
		t.Errorf("want 6 entries, got %d: %v", len(snap.Stats), snap.Stats)
	}
}

func TestPollSamplesCarriesSwarmLabels(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: containerSummaries("aaaaaaaaaaaa", "shop_web.1.t1", map[string]string{
			swarmTaskIDLabel: "t1", swarmServiceNameLabel: "shop_web", "unrelated": "x",
		}),
		statsFn: func(string) (containerStatsReader, error) { return fakeStatsBody(stats), nil },
	}
	samples := PollSamples(context.Background(), cli, 2)
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	s := samples[0]
	if s.ID != "aaaaaaaaaaaa" || s.TaskID() != "t1" || s.ServiceName() != "shop_web" {
		t.Errorf("unexpected sample %+v", s)
	}
	if _, leaked := s.Labels["unrelated"]; leaked {
		t.Error("unrelated labels must not be copied")
	}
}

// containerStatsReader shortens the stats stub signature.
type containerStatsReader = containertypes.StatsResponseReader

// containerSummaries returns one running container with the given labels.
func containerSummaries(id, name string, labels map[string]string) []containertypes.Summary {
	return []containertypes.Summary{{ID: id, Names: []string{"/" + name}, State: StateRunning, Labels: labels}}
}

func TestStatsCollectorStampsLocalSamples(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: containerSummaries("aaaaaaaaaaaa", "shop_web.1.t1", map[string]string{
			swarmTaskIDLabel: "t1", swarmServiceNameLabel: "shop_web",
		}),
		statsFn: func(string) (containerStatsReader, error) { return fakeStatsBody(stats), nil },
	}

	// Without a local node (standalone), no node aggregate appears.
	sc := NewStatsCollector(cli, time.Hour, 2)
	for key := range sc.poll(context.Background()).Stats {
		if IsNodeSeries(key) {
			t.Errorf("unexpected node series %q without a local node", key)
		}
	}

	// Local node only, no remote sampler.
	sc = NewStatsCollector(cli, time.Hour, 2)
	sc.SetLocalNode("n-local", "mgr")
	snap := sc.poll(context.Background())
	local := snap.Stats["shop_web.1.t1"]
	if got, ok := snap.Stats["node:mgr"]; !ok || got != local {
		t.Errorf("node:mgr = %+v (present %v), want %+v", got, ok, local)
	}

	// Local and remote: each node gets its own aggregate.
	sc = NewStatsCollector(cli, time.Hour, 2)
	sc.SetLocalNode("n-local", "mgr")
	remote := taskSample("bbbbbbbbbbbb", "shop_web.2.t2", "shop_web", "t2", 5, 300)
	remote.NodeID, remote.NodeHostname = "n-remote", "wrk"
	sc.SetRemote(remoteStub{samples: []ContainerSample{remote}})
	snap = sc.poll(context.Background())
	if got := snap.Stats["node:mgr"]; got != local {
		t.Errorf("node:mgr = %+v, want %+v", got, local)
	}
	if got := snap.Stats["node:wrk"]; got.CPUPercent != 5 || got.MemUsage != 300 {
		t.Errorf("node:wrk = %+v", got)
	}
	if agg := snap.Stats["shop_web"]; agg.MemUsage != local.MemUsage+300 {
		t.Errorf("service aggregate polluted or wrong: %+v", agg)
	}
}

func TestBuildStatsSnapshotNodeAggregates(t *testing.T) {
	onNode := func(s ContainerSample, id, host string) ContainerSample {
		s.NodeID, s.NodeHostname = id, host
		return s
	}
	snap := BuildStatsSnapshot([]ContainerSample{
		onNode(taskSample("a", "svc.1.x", "svc", "x", 10, 100), "n1", "mgr"),
		onNode(taskSample("b", "svc.2.y", "svc", "y", 20, 200), "n2", "wrk"),
		onNode(ContainerSample{
			ID: "c", Name: "standalone",
			Stats: ContainerStats{CPUPercent: 1, CPUThrottled: 60, MemUsage: 10, MemLimit: 500, NetTx: 4, BlockRead: 9, PIDs: 2},
		}, "n1", "mgr"),
		// Unattributed sample: counted in no node.
		taskSample("d", "svc.3.z", "svc", "z", 100, 1000),
	})

	wantMgr := ContainerStats{CPUPercent: 11, CPUThrottled: 60, MemUsage: 110, MemLimit: 1500, NetTx: 4, BlockRead: 9, PIDs: 2}
	if got := snap.Stats["node:mgr"]; got != wantMgr {
		t.Errorf("node:mgr = %+v, want %+v", got, wantMgr)
	}
	wantWrk := ContainerStats{CPUPercent: 20, MemUsage: 200, MemLimit: 1000}
	if got := snap.Stats["node:wrk"]; got != wantWrk {
		t.Errorf("node:wrk = %+v, want %+v", got, wantWrk)
	}
	// The service aggregate is unaffected by node attribution.
	if got := snap.Stats["svc"]; got.CPUPercent != 130 || got.MemUsage != 1300 {
		t.Errorf("svc aggregate = %+v", got)
	}
	// 4 containers + 1 service + 2 nodes.
	if len(snap.Stats) != 7 {
		t.Errorf("want 7 entries, got %d: %v", len(snap.Stats), snap.Stats)
	}
}

func TestIsNodeSeries(t *testing.T) {
	for key, want := range map[string]bool{"node:mgr": true, "node:": true, "node": false, "web": false, "shop_web.1.x": false} {
		if got := IsNodeSeries(key); got != want {
			t.Errorf("IsNodeSeries(%q) = %v, want %v", key, got, want)
		}
	}
}

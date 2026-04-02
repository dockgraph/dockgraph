package collector

import (
	"context"
	"fmt"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
)

// --- pollAllStats ---

func TestPollAllStatsFiltersRunningOnly(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{ID: "aaaaaaaaaaaa", Names: []string{"/running-app"}, State: "running"},
			{ID: "bbbbbbbbbbbb", Names: []string{"/stopped-app"}, State: "exited"},
			{ID: "cccccccccccc", Names: []string{"/created-app"}, State: "created"},
		},
		statsFn: func(_ string) (containertypes.StatsResponseReader, error) {
			return fakeStatsBody(stats), nil
		},
	}

	snap := pollAllStats(context.Background(), cli, 4)

	if len(snap.Stats) != 1 {
		t.Fatalf("expected 1 stat entry (running only), got %d", len(snap.Stats))
	}
	if _, ok := snap.Stats["running-app"]; !ok {
		t.Error("expected running-app in stats")
	}
}

func TestPollAllStatsExcludesSelfLabeled(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{ID: "aaaaaaaaaaaa", Names: []string{"/user-app"}, State: "running"},
			{
				ID:     "bbbbbbbbbbbb",
				Names:  []string{"/dockgraph"},
				State:  "running",
				Labels: map[string]string{SelfExcludeLabel: "true"},
			},
		},
		statsFn: func(_ string) (containertypes.StatsResponseReader, error) {
			return fakeStatsBody(stats), nil
		},
	}

	snap := pollAllStats(context.Background(), cli, 4)

	if len(snap.Stats) != 1 {
		t.Fatalf("expected 1 stat entry (self-excluded filtered), got %d", len(snap.Stats))
	}
	if _, ok := snap.Stats["user-app"]; !ok {
		t.Error("expected user-app in stats")
	}
}

func TestPollAllStatsEmptyContainerList(t *testing.T) {
	cli := &stubDockerClient{
		containers: []containertypes.Summary{},
	}

	snap := pollAllStats(context.Background(), cli, 4)

	if snap.Stats == nil {
		t.Fatal("expected non-nil stats map")
	}
	if len(snap.Stats) != 0 {
		t.Errorf("expected 0 stat entries, got %d", len(snap.Stats))
	}
}

func TestPollAllStatsContainerListError(t *testing.T) {
	cli := errClient("containers")

	snap := pollAllStats(context.Background(), cli, 4)

	if snap.Stats == nil {
		t.Fatal("expected non-nil stats map on error")
	}
	if len(snap.Stats) != 0 {
		t.Errorf("expected 0 entries on error, got %d", len(snap.Stats))
	}
}

func TestPollAllStatsAccumulatesMultipleContainers(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{ID: "aaaaaaaaaaaa", Names: []string{"/web"}, State: "running"},
			{ID: "bbbbbbbbbbbb", Names: []string{"/db"}, State: "running"},
			{ID: "cccccccccccc", Names: []string{"/cache"}, State: "running"},
		},
		statsFn: func(_ string) (containertypes.StatsResponseReader, error) {
			return fakeStatsBody(stats), nil
		},
	}

	snap := pollAllStats(context.Background(), cli, 4)

	if len(snap.Stats) != 3 {
		t.Fatalf("expected 3 stat entries, got %d", len(snap.Stats))
	}
	for _, name := range []string{"web", "db", "cache"} {
		if _, ok := snap.Stats[name]; !ok {
			t.Errorf("expected %s in stats", name)
		}
	}
}

func TestPollAllStatsSkipsContainersWithNoNames(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{ID: "aaaaaaaaaaaa", Names: []string{"/valid"}, State: "running"},
			{ID: "bbbbbbbbbbbb", Names: []string{}, State: "running"},
		},
		statsFn: func(_ string) (containertypes.StatsResponseReader, error) {
			return fakeStatsBody(stats), nil
		},
	}

	snap := pollAllStats(context.Background(), cli, 4)

	if len(snap.Stats) != 1 {
		t.Fatalf("expected 1 stat entry (nameless filtered), got %d", len(snap.Stats))
	}
}

func TestPollAllStatsTrimsSlashPrefix(t *testing.T) {
	stats := sampleStatsResponse(1_000_000, 10_000_000, 2)
	cli := &stubDockerClient{
		containers: []containertypes.Summary{
			{ID: "aaaaaaaaaaaa", Names: []string{"/my-container"}, State: "running"},
		},
		statsFn: func(_ string) (containertypes.StatsResponseReader, error) {
			return fakeStatsBody(stats), nil
		},
	}

	snap := pollAllStats(context.Background(), cli, 4)

	if _, ok := snap.Stats["my-container"]; !ok {
		t.Error("expected key without leading slash: my-container")
	}
}

// --- fetchOneStats ---

func TestFetchOneStatsSuccess(t *testing.T) {
	stats := containertypes.StatsResponse{
		CPUStats: containertypes.CPUStats{
			CPUUsage:    containertypes.CPUUsage{TotalUsage: 5_000_000_000},
			SystemUsage: 20_000_000_000,
			OnlineCPUs:  4,
		},
		PreCPUStats: containertypes.CPUStats{
			CPUUsage:    containertypes.CPUUsage{TotalUsage: 4_000_000_000},
			SystemUsage: 10_000_000_000,
		},
		MemoryStats: containertypes.MemoryStats{
			Usage: 100_000_000,
			Limit: 512_000_000,
		},
		Networks: map[string]containertypes.NetworkStats{
			"eth0": {RxBytes: 5000, TxBytes: 3000, RxErrors: 1, TxErrors: 2},
		},
		BlkioStats: containertypes.BlkioStats{
			IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
				{Op: "read", Value: 1024},
				{Op: "write", Value: 2048},
			},
		},
		PidsStats: containertypes.PidsStats{Current: 42},
	}

	cli := &stubDockerClient{
		statsFn: func(_ string) (containertypes.StatsResponseReader, error) {
			return fakeStatsBody(stats), nil
		},
	}

	cs, ok := fetchOneStats(context.Background(), cli, "aaaaaaaaaaaa")
	if !ok {
		t.Fatal("expected ok=true for successful fetch")
	}

	// CPU: (1e9/10e9)*4*100 = 40
	if cs.CPUPercent != 40.0 {
		t.Errorf("CPUPercent: expected 40.0, got %f", cs.CPUPercent)
	}
	if cs.MemUsage != 100_000_000 {
		t.Errorf("MemUsage: expected 100000000, got %d", cs.MemUsage)
	}
	if cs.MemLimit != 512_000_000 {
		t.Errorf("MemLimit: expected 512000000, got %d", cs.MemLimit)
	}
	if cs.NetRx != 5000 {
		t.Errorf("NetRx: expected 5000, got %d", cs.NetRx)
	}
	if cs.NetTx != 3000 {
		t.Errorf("NetTx: expected 3000, got %d", cs.NetTx)
	}
	if cs.NetRxErrors != 1 {
		t.Errorf("NetRxErrors: expected 1, got %d", cs.NetRxErrors)
	}
	if cs.NetTxErrors != 2 {
		t.Errorf("NetTxErrors: expected 2, got %d", cs.NetTxErrors)
	}
	if cs.BlockRead != 1024 {
		t.Errorf("BlockRead: expected 1024, got %d", cs.BlockRead)
	}
	if cs.BlockWrite != 2048 {
		t.Errorf("BlockWrite: expected 2048, got %d", cs.BlockWrite)
	}
	if cs.PIDs != 42 {
		t.Errorf("PIDs: expected 42, got %d", cs.PIDs)
	}
}

func TestFetchOneStatsDockerError(t *testing.T) {
	cli := &stubDockerClient{
		statsFn: func(_ string) (containertypes.StatsResponseReader, error) {
			return containertypes.StatsResponseReader{}, fmt.Errorf("container not found")
		},
	}

	_, ok := fetchOneStats(context.Background(), cli, "aaaaaaaaaaaa")
	if ok {
		t.Fatal("expected ok=false when Docker returns error")
	}
}

func TestFetchOneStatsInvalidJSON(t *testing.T) {
	cli := &stubDockerClient{
		statsFn: func(_ string) (containertypes.StatsResponseReader, error) {
			return fakeStatsBodyRaw([]byte(`{invalid json`)), nil
		},
	}

	_, ok := fetchOneStats(context.Background(), cli, "aaaaaaaaaaaa")
	if ok {
		t.Fatal("expected ok=false for malformed JSON response")
	}
}

// sampleStatsResponse builds a minimal StatsResponse for test fixtures.
func sampleStatsResponse(cpuDelta, systemDelta uint64, onlineCPUs uint32) containertypes.StatsResponse {
	return containertypes.StatsResponse{
		CPUStats: containertypes.CPUStats{
			CPUUsage:    containertypes.CPUUsage{TotalUsage: 10_000_000 + cpuDelta},
			SystemUsage: 100_000_000 + systemDelta,
			OnlineCPUs:  onlineCPUs,
		},
		PreCPUStats: containertypes.CPUStats{
			CPUUsage:    containertypes.CPUUsage{TotalUsage: 10_000_000},
			SystemUsage: 100_000_000,
		},
		MemoryStats: containertypes.MemoryStats{
			Usage: 64_000_000,
			Limit: 256_000_000,
		},
		PidsStats: containertypes.PidsStats{Current: 10},
	}
}

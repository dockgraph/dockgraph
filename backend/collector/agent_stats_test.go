package collector

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Samples past the bound are skipped, not decoded: one that couldn't be
// decoded at all is passed over.
func TestDecodeAgentStatsSkipsSamplesPastTheBound(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"nodeId":"node-a","samples":[`)
	for i := range maxAgentSamples {
		fmt.Fprintf(&b, `{"id":"%016x","name":"c%d"},`, i, i)
	}
	b.WriteString(`{"id":5,"name":["not","a","sample"]}],"extra":{"ignored":true}}`)

	stats, total, err := decodeAgentStats([]byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if stats.NodeID != "node-a" || len(stats.Samples) != maxAgentSamples || total != maxAgentSamples+1 {
		t.Errorf("node %q, %d samples of %d, want node-a, %d of %d", stats.NodeID, len(stats.Samples), total, maxAgentSamples, maxAgentSamples+1)
	}
	for _, bad := range []string{``, `[]`, `{"samples":{}}`, `{"samples":[1,2]`} {
		if _, _, err := decodeAgentStats([]byte(bad)); err == nil {
			t.Errorf("%q: accepted", bad)
		}
	}
	if stats, _, err := decodeAgentStats([]byte(`{"samples":null}`)); err != nil || len(stats.Samples) != 0 {
		t.Errorf("null samples: %v, %d samples", err, len(stats.Samples))
	}
}

// CPU figures an agent reports are bounded to what a host can show, so sums
// across containers stay finite: +Inf cannot be encoded as JSON, and one
// such value would fail every stats message and history query.
func TestDecodeAgentStatsBoundsCPU(t *testing.T) {
	body := `{"samples":[
		{"id":"a","name":"a","nodeHostname":"w1","stats":{"cpuPercent":1.7976931348623157e308,"cpuThrottled":1e300,"memUsage":10}},
		{"id":"b","name":"b","nodeHostname":"w1","stats":{"cpuPercent":1.7976931348623157e308,"cpuThrottled":-5}},
		{"id":"c","name":"c","stats":{"cpuPercent":-3,"cpuThrottled":40}},
		{"id":"d","name":"d","stats":{"cpuPercent":250.5,"cpuThrottled":100}}
	]}`
	stats, _, err := decodeAgentStats([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	want := []ContainerStats{
		{CPUPercent: maxCPUPercent, CPUThrottled: 100, MemUsage: 10},
		{CPUPercent: maxCPUPercent, CPUThrottled: 0},
		{CPUPercent: 0, CPUThrottled: 40},
		{CPUPercent: 250.5, CPUThrottled: 100},
	}
	for i, s := range stats.Samples {
		if s.Stats != want[i] {
			t.Errorf("sample %s: stats %+v, want %+v", s.ID, s.Stats, want[i])
		}
	}

	snap := BuildStatsSnapshot(stats.Samples, "w1")
	if _, err := json.Marshal(NewStatsMessage(snap)); err != nil {
		t.Errorf("stats message: %v", err)
	}
	h := NewStatsHistory(time.Hour)
	h.Record(time.Now().Add(-2*time.Hour), snap)
	h.Record(time.Now().Add(-2*time.Hour+time.Second), snap)
	h.Downsample(30 * time.Second)
	if _, err := json.Marshal(h.Query(time.Time{}, time.Now(), nil)); err != nil {
		t.Errorf("history query: %v", err)
	}
}

// The streaming decode reads an answer as json.Unmarshal into AgentStats
// would: keys in any case, the last of repeated keys, null as empty, and
// nothing after the object.
func TestDecodeAgentStatsMatchesUnmarshal(t *testing.T) {
	for _, body := range []string{
		`{"NodeId":"n","Samples":[{"id":"a","name":"a"}]}`,
		`{"samples":[{"id":"a","name":"a"}],"samples":[{"id":"b","name":"b"}]}`,
		`{"samples":[{"id":"a","name":"a"}],"samples":null}`,
		`null`,
		`{}`,
	} {
		var want AgentStats
		if err := json.Unmarshal([]byte(body), &want); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		got, total, err := decodeAgentStats([]byte(body))
		if err != nil {
			t.Errorf("%s: %v", body, err)
			continue
		}
		if got.NodeID != want.NodeID || len(got.Samples) != len(want.Samples) || total != len(want.Samples) {
			t.Errorf("%s: got %q with %d samples (total %d), want %q with %d", body, got.NodeID, len(got.Samples), total, want.NodeID, len(want.Samples))
			continue
		}
		for i := range want.Samples {
			if got.Samples[i].ID != want.Samples[i].ID {
				t.Errorf("%s: sample %d = %q, want %q", body, i, got.Samples[i].ID, want.Samples[i].ID)
			}
		}
	}
	if _, _, err := decodeAgentStats([]byte(`{"samples":[]} {"more":1}`)); err == nil {
		t.Error("trailing data accepted")
	}
}

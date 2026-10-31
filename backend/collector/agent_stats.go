package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Samples polls /stats on every remote agent concurrently and returns their
// samples, each stamped with the agent's node ID and hostname. Agents that
// fail or time out are skipped for this round. It also
// records which node owns each sampled container for LocateContainer.
func (p *AgentPool) Samples(ctx context.Context) []ContainerSample {
	agents := p.remoteEndpoints()
	if len(agents) == 0 {
		p.mu.Lock()
		p.owners = make(map[string]string)
		p.names = make(map[string]string)
		p.tasks = make(map[string]string)
		p.placements = make(map[string]string)
		p.reporting = nil
		p.mu.Unlock()
		return nil
	}

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		samples   []ContainerSample
		owners    = make(map[string]string)
		names     = make(map[string]string)
		tasks     = make(map[string]string)
		reporters = make(map[string]string) // container or task ID -> the node reporting it
		reporting []string
	)
	for nodeID, a := range agents {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, err := p.get(ctx, a.addr, "/stats", maxAgentStatsBytes)
			var resp AgentStats
			var total int
			if err == nil {
				resp, total, err = decodeAgentStats(body)
			}
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("agents: %s (node %s) /stats: %v", a.addr, a.hostname, err)
				}
				return
			}
			if total > maxAgentSamples {
				log.Printf("agents: %s (node %s) reported %d containers; using the first %d", a.addr, a.hostname, total, maxAgentSamples)
			}
			mu.Lock()
			defer mu.Unlock()
			reporting = append(reporting, a.hostname)
			for _, s := range resp.Samples {
				if s.ID == "" || s.Name == "" {
					continue
				}
				// Attribute the sample to the node the pool discovered the
				// agent on; whatever the agent reported is overridden.
				s.NodeID = nodeID
				s.NodeHostname = a.hostname
				samples = append(samples, s)
				claim(reporters, "container:"+s.ID, nodeID)
				if tid := s.TaskID(); tid != "" {
					claim(reporters, "task:"+tid, nodeID)
					claim(tasks, s.ID, tid)
					claim(tasks, s.Name, tid)
					claim(tasks, tid, tid)
				} else {
					claim(owners, s.ID, nodeID)
					claim(names, s.Name, nodeID)
				}
			}
		}()
	}
	wg.Wait()
	samples = slices.DeleteFunc(samples, func(s ContainerSample) bool {
		return reportedByOthers(reporters, s)
	})

	sort.Strings(reporting)
	p.mu.Lock()
	p.owners = owners
	p.names = names
	p.tasks = tasks
	for taskID := range p.placements {
		if _, sampled := tasks[taskID]; !sampled {
			delete(p.placements, taskID)
		}
	}
	p.reporting = reporting
	p.mu.Unlock()
	return samples
}

// reportedByOthers reports whether another node also reported s's container
// or task. A container runs on one node and a task on one node, so all but
// one such report are wrong, and nothing in them tells which: their stats
// are dropped rather than shown for the wrong node.
func reportedByOthers(reporters map[string]string, s ContainerSample) bool {
	if reporters["container:"+s.ID] == "" {
		return true
	}
	tid := s.TaskID()
	return tid != "" && reporters["task:"+tid] == ""
}

// claim records value (a node or task ID) for key, or "" once another value
// has been claimed for it too.
func claim(claims map[string]string, key, value string) {
	if claimed, seen := claims[key]; seen && claimed != value {
		claims[key] = ""
		return
	}
	claims[key] = value
}

// ReportingNodes returns the hostnames of the remote agents that answered
// the last Samples round, including those that reported no containers.
func (p *AgentPool) ReportingNodes() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]string(nil), p.reporting...)
}

// decodeAgentStats decodes an agent's /stats answer as json.Unmarshal into
// AgentStats would, but keeps only the first maxAgentSamples samples; total
// counts them all. Samples past the bound are skipped without being
// decoded, so an answer reporting more containers costs little more than
// one at the bound.
func decodeAgentStats(body []byte) (stats AgentStats, total int, err error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	switch {
	case err != nil:
		return stats, 0, err
	case tok == nil:
		// A null answer holds nothing.
	case tok != json.Delim('{'):
		return stats, 0, fmt.Errorf("want an object, got %v", tok)
	default:
		if total, err = decodeAgentStatsFields(dec, &stats); err != nil {
			return stats, 0, err
		}
	}
	if _, err := dec.Token(); err != io.EOF {
		return stats, 0, errors.New("data after the answer")
	}
	return stats, total, nil
}

// decodeAgentStatsFields decodes the fields of an AgentStats object, whose
// opening brace dec has read, through its closing one. Keys match in any
// case and the last of a repeated key wins, as for json.Unmarshal.
func decodeAgentStatsFields(dec *json.Decoder, stats *AgentStats) (total int, err error) {
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return 0, err
		}
		key, _ := tok.(string)
		switch {
		case strings.EqualFold(key, "nodeId"):
			err = dec.Decode(&stats.NodeID)
		case strings.EqualFold(key, "samples"):
			stats.Samples = nil
			total, err = decodeSamples(dec, &stats.Samples)
		default:
			err = dec.Decode(new(json.RawMessage))
		}
		if err != nil {
			return 0, err
		}
	}
	return total, expectDelim(dec, '}')
}

// decodeSamples decodes a JSON array (or null) of samples into samples, up
// to maxAgentSamples of them, and returns how many it held.
func decodeSamples(dec *json.Decoder, samples *[]ContainerSample) (int, error) {
	tok, err := dec.Token()
	if err != nil || tok == nil {
		return 0, err // a null list holds none
	}
	if tok != json.Delim('[') {
		return 0, fmt.Errorf("samples: want a list, got %v", tok)
	}
	n := 0
	for ; dec.More(); n++ {
		if len(*samples) >= maxAgentSamples {
			if err := dec.Decode(new(json.RawMessage)); err != nil {
				return 0, err
			}
			continue
		}
		var s ContainerSample
		if err := dec.Decode(&s); err != nil {
			return 0, err
		}
		s.Stats = boundCPU(s.Stats)
		*samples = append(*samples, s)
	}
	return n, expectDelim(dec, ']')
}

// maxCPUPercent is the highest CPU usage a container can show: 100% of each
// of 4096 CPUs. DockGraph computes the figure as 100% per CPU used.
const maxCPUPercent = 100 * 4096

// boundCPU clamps the CPU figures an agent reported to what a host can
// show: usage to [0, maxCPUPercent], throttling to [0, 100]. Aggregates add
// many containers' usage, and a sum that overflows to +Inf cannot be
// encoded as JSON, which would fail every stats message and history query.
// Decoded JSON numbers are finite, so only the bounds need checking.
func boundCPU(s ContainerStats) ContainerStats {
	s.CPUPercent = min(max(s.CPUPercent, 0), maxCPUPercent)
	s.CPUThrottled = min(max(s.CPUThrottled, 0), 100)
	return s
}

// expectDelim reads the next JSON token and checks it is delim.
func expectDelim(dec *json.Decoder, delim json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != delim {
		return fmt.Errorf("want %v, got %v", delim, tok)
	}
	return nil
}

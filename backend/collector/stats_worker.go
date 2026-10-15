package collector

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
)

// ContainerSample is one container's latest resource usage together with its
// identity. Per-node agents serve samples so the swarm server can attribute
// them to tasks and services. Labels carries only the swarm task and service
// labels (sampleLabelKeys), never the full label set. NodeID and
// NodeHostname name the swarm node the container runs on; they are stamped by
// the swarm server (StatsCollector for local samples, AgentPool for remote
// ones) and are empty outside swarm mode.
type ContainerSample struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Labels       map[string]string `json:"labels,omitempty"`
	NodeID       string            `json:"nodeId,omitempty"`
	NodeHostname string            `json:"nodeHostname,omitempty"`
	Stats        ContainerStats    `json:"stats"`
}

// NodeStatsPrefix prefixes the reserved stats keys of per-node aggregates:
// "node:{hostname}". Container and service names cannot contain ':', so these
// keys never collide with a container or service series.
const NodeStatsPrefix = "node:"

// IsNodeSeries reports whether a stats key is a per-node aggregate rather
// than a container or service series.
func IsNodeSeries(key string) bool { return strings.HasPrefix(key, NodeStatsPrefix) }

// TaskID returns the swarm task the sampled container runs, or "".
func (s ContainerSample) TaskID() string { return s.Labels[swarmTaskIDLabel] }

// ServiceName returns the swarm service the sampled container belongs to, or "".
func (s ContainerSample) ServiceName() string { return s.Labels[swarmServiceNameLabel] }

// sampleLabelKeys are the container labels copied into a ContainerSample.
var sampleLabelKeys = []string{swarmTaskIDLabel, swarmServiceNameLabel}

// pollAllStats fetches stats for all running containers and keys them by
// container name, adding per-service aggregates for swarm task containers.
func pollAllStats(ctx context.Context, cli DockerClient, maxWorkers int) StatsSnapshot {
	return BuildStatsSnapshot(PollSamples(ctx, cli, maxWorkers))
}

// PollSamples fetches stats for all running containers on the local daemon
// using a bounded worker pool. Errors on individual containers are logged and
// skipped. Samples are sorted by container name.
func PollSamples(ctx context.Context, cli DockerClient, maxWorkers int) []ContainerSample {
	containers, err := cli.ContainerList(ctx, containertypes.ListOptions{})
	if err != nil {
		log.Printf("stats: failed to list containers: %v", err)
		return []ContainerSample{}
	}
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	var (
		mu      sync.Mutex
		results = make([]ContainerSample, 0, len(containers))
		wg      sync.WaitGroup
		sem     = make(chan struct{}, maxWorkers)
	)

	for _, c := range containers {
		if c.State != StateRunning {
			continue
		}
		if isSelfExcluded(c.Labels) {
			continue
		}
		if len(c.Names) == 0 {
			continue
		}

		sample := ContainerSample{ID: c.ID, Name: strings.TrimPrefix(c.Names[0], "/")}
		for _, k := range sampleLabelKeys {
			if v := c.Labels[k]; v != "" {
				if sample.Labels == nil {
					sample.Labels = make(map[string]string, len(sampleLabelKeys))
				}
				sample.Labels[k] = v
			}
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cs, ok := fetchOneStats(ctx, cli, sample.ID)
			if !ok {
				return
			}
			sample.Stats = cs
			mu.Lock()
			results = append(results, sample)
			mu.Unlock()
		}()
	}

	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results
}

// BuildStatsSnapshot keys samples by container name and adds aggregate
// entries: one per swarm service, keyed by service name, so service graph
// nodes get stats under their name; and one per swarm node, keyed
// NodeStatsPrefix+hostname, for samples stamped with a node. Aggregates sum
// CPU, memory, network, block I/O and PIDs across their containers; CPU
// throttling takes the worst container. A container whose name equals a
// service name keeps its own entry. When several samples share a container
// name (standalone containers named alike on different nodes), the first one
// keeps the name-keyed entry: mergeSamples puts local samples first, so the
// local container wins. Every sample still counts in its service and node
// aggregates. Every hostname in reportingNodes gets a
// node aggregate even without samples: its agent answered, but DockGraph's
// own containers are never sampled, so an otherwise empty node has none.
// Other nodes without samples (e.g. no agent) get no aggregate.
func BuildStatsSnapshot(samples []ContainerSample, reportingNodes ...string) StatsSnapshot {
	stats := make(map[string]ContainerStats, len(samples))
	services := make(map[string]ContainerStats)
	nodes := make(map[string]ContainerStats)
	for _, host := range reportingNodes {
		if host != "" {
			nodes[NodeStatsPrefix+host] = ContainerStats{}
		}
	}
	for _, s := range samples {
		if _, taken := stats[s.Name]; !taken {
			stats[s.Name] = s.Stats
		}
		if svc := s.ServiceName(); svc != "" && !IsNodeSeries(svc) {
			services[svc] = addStats(services[svc], s.Stats)
		}
		if s.NodeHostname != "" {
			key := NodeStatsPrefix + s.NodeHostname
			nodes[key] = addStats(nodes[key], s.Stats)
		}
	}
	for name, agg := range services {
		if _, taken := stats[name]; !taken {
			stats[name] = agg
		} else {
			warnOnce("stats-name:"+name, "stats: a container and a swarm service are both named %q; "+
				"stats are keyed by name, so the service shows the container's usage (see README, Known limitations)", name)
		}
	}
	for key, agg := range nodes {
		stats[key] = agg
	}
	return StatsSnapshot{Stats: stats}
}

// addStats adds one container's usage to an aggregate.
func addStats(agg, s ContainerStats) ContainerStats {
	agg.CPUPercent += s.CPUPercent
	agg.CPUThrottled = max(agg.CPUThrottled, s.CPUThrottled)
	agg.MemUsage += s.MemUsage
	agg.MemLimit += s.MemLimit
	agg.NetRx += s.NetRx
	agg.NetTx += s.NetTx
	agg.NetRxErrors += s.NetRxErrors
	agg.NetTxErrors += s.NetTxErrors
	agg.BlockRead += s.BlockRead
	agg.BlockWrite += s.BlockWrite
	agg.PIDs += s.PIDs
	return agg
}

// mergeSamples appends remote samples to local ones, dropping any remote
// sample for a container already sampled (same ID) so a container is never
// counted twice in its service and node aggregates. Remote containers that
// only share a name with another sample (e.g. standalone containers named
// alike on different nodes) are kept, so their node still reports their
// usage; local samples come first, so BuildStatsSnapshot gives the
// name-keyed entry to the local container.
func mergeSamples(local, remote []ContainerSample) []ContainerSample {
	if len(remote) == 0 {
		return local
	}
	seenIDs := make(map[string]bool, len(local)+len(remote))
	for _, s := range local {
		seenIDs[s.ID] = true
	}
	merged := make([]ContainerSample, 0, len(local)+len(remote))
	merged = append(merged, local...)
	for _, s := range remote {
		if s.ID == "" || seenIDs[s.ID] {
			continue
		}
		seenIDs[s.ID] = true
		merged = append(merged, s)
	}
	return merged
}

// fetchOneStats retrieves stats for a single container with a 10-second timeout.
func fetchOneStats(ctx context.Context, cli DockerClient, containerID string) (ContainerStats, bool) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := cli.ContainerStats(callCtx, containerID, false)
	if err != nil {
		log.Printf("stats: container %s: %v", shortID(containerID), err)
		return ContainerStats{}, false
	}
	defer resp.Body.Close()

	var raw containertypes.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		log.Printf("stats: decode %s: %v", shortID(containerID), err)
		return ContainerStats{}, false
	}

	rx, tx, rxErr, txErr := sumNetworkIO(raw.Networks)
	blockRead, blockWrite := sumBlockIO(raw.BlkioStats)

	return ContainerStats{
		CPUPercent:   calcCPUPercent(&raw),
		CPUThrottled: calcCPUThrottle(&raw),
		MemUsage:     calcMemUsage(raw.MemoryStats),
		MemLimit:     raw.MemoryStats.Limit,
		NetRx:        rx,
		NetTx:        tx,
		NetRxErrors:  rxErr,
		NetTxErrors:  txErr,
		BlockRead:    blockRead,
		BlockWrite:   blockWrite,
		PIDs:         raw.PidsStats.Current,
	}, true
}

// shortID truncates a container ID to Docker's 12-character short form.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

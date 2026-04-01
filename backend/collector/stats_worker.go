package collector

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
)

// pollAllStats fetches stats for all running containers using a bounded worker pool.
// Errors on individual containers are logged and skipped.
func pollAllStats(ctx context.Context, cli DockerClient, maxWorkers int) StatsSnapshot {
	containers, err := cli.ContainerList(ctx, containertypes.ListOptions{})
	if err != nil {
		log.Printf("stats: failed to list containers: %v", err)
		return StatsSnapshot{Stats: map[string]ContainerStats{}}
	}

	var (
		mu      sync.Mutex
		results = make(map[string]ContainerStats)
		wg      sync.WaitGroup
		sem     = make(chan struct{}, maxWorkers)
	)

	for _, c := range containers {
		if c.State != "running" {
			continue
		}
		if c.Labels[SelfExcludeLabel] == "true" {
			continue
		}
		if len(c.Names) == 0 {
			continue
		}

		name := strings.TrimPrefix(c.Names[0], "/")
		id := c.ID

		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cs, ok := fetchOneStats(ctx, cli, id)
			if !ok {
				return
			}
			mu.Lock()
			results[name] = cs
			mu.Unlock()
		}()
	}

	wg.Wait()
	return StatsSnapshot{Stats: results}
}

// fetchOneStats retrieves stats for a single container with a 10-second timeout.
func fetchOneStats(ctx context.Context, cli DockerClient, containerID string) (ContainerStats, bool) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := cli.ContainerStats(callCtx, containerID, false)
	if err != nil {
		log.Printf("stats: container %s: %v", containerID[:12], err)
		return ContainerStats{}, false
	}
	defer resp.Body.Close()

	var raw containertypes.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		log.Printf("stats: decode %s: %v", containerID[:12], err)
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

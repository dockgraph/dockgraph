package api

import (
	"context"
	"log"
	"sync"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	imagetypes "github.com/docker/docker/api/types/image"
	systemtypes "github.com/docker/docker/api/types/system"
)

// CachedSystemData holds pre-fetched Docker system data, polled on background timers
// to avoid expensive API calls on every HTTP request.
type CachedSystemData struct {
	mu sync.RWMutex

	info      *systemtypes.Info
	diskUsage *dockertypes.DiskUsage
	images    []imagetypes.Summary
}

// NewCachedSystemData creates a cache and starts background polling goroutines.
// The caller should cancel ctx to stop polling.
func NewCachedSystemData(ctx context.Context, source SystemAPI, infoInterval, diskInterval time.Duration) *CachedSystemData {
	c := &CachedSystemData{}

	// Initial fetch (best-effort, don't block startup).
	go c.fetchInfo(ctx, source)
	go c.fetchDisk(ctx, source)

	go c.pollLoop(ctx, source, infoInterval, diskInterval)
	return c
}

func (c *CachedSystemData) pollLoop(ctx context.Context, source SystemAPI, infoInterval, diskInterval time.Duration) {
	infoTicker := time.NewTicker(infoInterval)
	diskTicker := time.NewTicker(diskInterval)
	defer infoTicker.Stop()
	defer diskTicker.Stop()

	for {
		select {
		case <-infoTicker.C:
			c.fetchInfo(ctx, source)
		case <-diskTicker.C:
			c.fetchDisk(ctx, source)
		case <-ctx.Done():
			return
		}
	}
}

func (c *CachedSystemData) fetchInfo(ctx context.Context, source SystemAPI) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := source.Info(callCtx)
	if err != nil {
		log.Printf("system cache: info fetch failed: %v", err)
		return
	}
	c.mu.Lock()
	c.info = &info
	c.mu.Unlock()
}

func (c *CachedSystemData) fetchDisk(ctx context.Context, source SystemAPI) {
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	du, err := source.DiskUsage(callCtx, dockertypes.DiskUsageOptions{})
	if err != nil {
		log.Printf("system cache: disk usage fetch failed: %v", err)
		return
	}

	images, err := source.ImageList(callCtx, imagetypes.ListOptions{All: true})
	if err != nil {
		log.Printf("system cache: image list fetch failed: %v", err)
		return
	}

	c.mu.Lock()
	c.diskUsage = &du
	c.images = images
	c.mu.Unlock()
}

// Info returns cached system info.
func (c *CachedSystemData) Info(_ context.Context) (systemtypes.Info, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.info == nil {
		return systemtypes.Info{}, nil
	}
	return *c.info, nil
}

// DiskUsage returns cached disk usage.
func (c *CachedSystemData) DiskUsage(_ context.Context, _ dockertypes.DiskUsageOptions) (dockertypes.DiskUsage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.diskUsage == nil {
		return dockertypes.DiskUsage{}, nil
	}
	return *c.diskUsage, nil
}

// ImageList returns cached image list.
func (c *CachedSystemData) ImageList(_ context.Context, _ imagetypes.ListOptions) ([]imagetypes.Summary, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.images, nil
}

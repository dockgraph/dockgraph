package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	imagetypes "github.com/docker/docker/api/types/image"
	systemtypes "github.com/docker/docker/api/types/system"
)

// SystemInfoProvider fetches Docker daemon information.
type SystemInfoProvider interface {
	Info(ctx context.Context) (systemtypes.Info, error)
}

// SystemDiskUsageProvider fetches Docker disk usage statistics.
type SystemDiskUsageProvider interface {
	DiskUsage(ctx context.Context, options dockertypes.DiskUsageOptions) (dockertypes.DiskUsage, error)
}

// ImageLister fetches Docker image summaries.
type ImageLister interface {
	ImageList(ctx context.Context, options imagetypes.ListOptions) ([]imagetypes.Summary, error)
}

// HandleSystemInfo returns a handler for GET /api/system/info.
func HandleSystemInfo(provider SystemInfoProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		info, err := provider.Info(ctx)
		if err != nil {
			log.Printf("system info: %v", err)
			jsonError(w, "failed to fetch system info", http.StatusInternalServerError)
			return
		}

		resp := map[string]any{
			"dockerVersion": info.ServerVersion,
			"os":            info.OperatingSystem,
			"arch":          info.Architecture,
			"kernel":        info.KernelVersion,
			"storageDriver": info.Driver,
			"cpus":          info.NCPU,
			"memTotal":      info.MemTotal,
			"cgroupVersion": info.CgroupVersion,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// HandleSystemDiskUsage returns a handler for GET /api/system/disk-usage.
func HandleSystemDiskUsage(provider SystemDiskUsageProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		du, err := provider.DiskUsage(ctx, dockertypes.DiskUsageOptions{})
		if err != nil {
			log.Printf("disk usage: %v", err)
			jsonError(w, "failed to fetch disk usage", http.StatusInternalServerError)
			return
		}

		var imgTotal, imgReclaimable int64
		for _, img := range du.Images {
			imgTotal += img.Size
			if len(img.RepoTags) == 0 || (len(img.RepoTags) == 1 && img.RepoTags[0] == "<none>:<none>") {
				imgReclaimable += img.Size
			}
		}

		var ctrTotal, ctrReclaimable int64
		for _, c := range du.Containers {
			ctrTotal += c.SizeRw
			if c.State != "running" {
				ctrReclaimable += c.SizeRw
			}
		}

		var volTotal, volReclaimable int64
		for _, v := range du.Volumes {
			if v.UsageData.Size > 0 {
				volTotal += v.UsageData.Size
			}
			if v.UsageData.RefCount == 0 && v.UsageData.Size > 0 {
				volReclaimable += v.UsageData.Size
			}
		}

		var cacheTotal, cacheReclaimable int64
		for _, b := range du.BuildCache {
			cacheTotal += b.Size
			if !b.InUse {
				cacheReclaimable += b.Size
			}
		}

		resp := map[string]any{
			"images":     map[string]any{"total": imgTotal, "count": len(du.Images), "reclaimable": imgReclaimable},
			"containers": map[string]any{"total": ctrTotal, "count": len(du.Containers), "reclaimable": ctrReclaimable},
			"volumes":    map[string]any{"total": volTotal, "count": len(du.Volumes), "reclaimable": volReclaimable},
			"buildCache": map[string]any{"total": cacheTotal, "count": len(du.BuildCache), "reclaimable": cacheReclaimable},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// HandleImages returns a handler for GET /api/images.
func HandleImages(lister ImageLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		images, err := lister.ImageList(ctx, imagetypes.ListOptions{All: true})
		if err != nil {
			log.Printf("image list: %v", err)
			jsonError(w, "failed to list images", http.StatusInternalServerError)
			return
		}

		var totalSize int64
		var dangling int
		var danglingSize int64
		tags := make(map[string]bool)

		for _, img := range images {
			totalSize += img.Size
			isDangling := len(img.RepoTags) == 0 || (len(img.RepoTags) == 1 && img.RepoTags[0] == "<none>:<none>")
			if isDangling {
				dangling++
				danglingSize += img.Size
			}
			for _, tag := range img.RepoTags {
				if tag != "<none>:<none>" {
					tags[tag] = true
				}
			}
		}

		resp := map[string]any{
			"total":        len(images),
			"totalSize":    totalSize,
			"dangling":     dangling,
			"danglingSize": danglingSize,
			"uniqueTags":   len(tags),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

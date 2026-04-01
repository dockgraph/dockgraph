package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	volumetypes "github.com/docker/docker/api/types/volume"
)

// VolumeInspector is the subset of the Docker API needed for volume inspection.
type VolumeInspector interface {
	VolumeInspect(ctx context.Context, volumeID string) (volumetypes.Volume, error)
}

// HandleVolumeInspect returns a handler for GET /api/volumes/{name}.
func HandleVolumeInspect(inspector VolumeInspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !validResourceName.MatchString(name) {
			http.Error(w, `{"error":"invalid volume name"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		vol, err := inspector.VolumeInspect(ctx, name)
		if err != nil {
			log.Printf("inspect volume %s: %v", name, err)
			http.Error(w, `{"error":"volume not found"}`, http.StatusNotFound)
			return
		}

		resp := buildVolumeResponse(vol)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func buildVolumeResponse(vol volumetypes.Volume) map[string]any {
	resp := map[string]any{
		"name":       vol.Name,
		"driver":     vol.Driver,
		"mountpoint": vol.Mountpoint,
		"createdAt":  vol.CreatedAt,
		"status":     vol.Status,
		"labels":     vol.Labels,
		"scope":      vol.Scope,
		"options":    vol.Options,
	}

	if vol.UsageData != nil {
		resp["usageSize"] = vol.UsageData.Size
		resp["usageRefCount"] = vol.UsageData.RefCount
	}

	return resp
}

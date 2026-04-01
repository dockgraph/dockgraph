package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
)

// ContainerInspector is the subset of the Docker API needed for container inspection.
type ContainerInspector interface {
	ContainerInspect(ctx context.Context, containerID string) (containertypes.InspectResponse, error)
}

var validContainerID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

// sensitiveKeyPatterns matches env var keys that should be masked.
var sensitiveKeyPatterns = []string{
	"PASSWORD", "SECRET", "KEY", "TOKEN", "CREDENTIAL",
	"API_KEY", "APIKEY", "PRIVATE", "AUTH", "CERT",
	"SSL_", "TLS_", "ENCRYPT", "ACCESS_KEY", "SESSION",
}

func isSensitiveKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, pattern := range sensitiveKeyPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

func filterEnvVars(envList []string) []map[string]string {
	result := make([]map[string]string, 0, len(envList))
	for _, e := range envList {
		k, v, _ := strings.Cut(e, "=")
		if isSensitiveKey(k) {
			v = "********"
		}
		result = append(result, map[string]string{"key": k, "value": v})
	}
	return result
}

// HandleContainerInspect returns a handler for GET /api/containers/{id}.
func HandleContainerInspect(inspector ContainerInspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validContainerID.MatchString(id) {
			http.Error(w, `{"error":"invalid container ID"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		info, err := inspector.ContainerInspect(ctx, id)
		if err != nil {
			log.Printf("inspect %s: %v", id, err)
			http.Error(w, `{"error":"container not found"}`, http.StatusNotFound)
			return
		}

		resp := buildInspectResponse(info)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func buildInspectResponse(info containertypes.InspectResponse) map[string]any {
	name := strings.TrimPrefix(info.Name, "/")

	resp := map[string]any{
		"name":          name,
		"image":         info.Config.Image,
		"status":        info.State.Status,
		"running":       info.State.Running,
		"paused":        info.State.Paused,
		"restarting":    info.State.Restarting,
		"oomKilled":     info.State.OOMKilled,
		"pid":           info.State.Pid,
		"exitCode":      info.State.ExitCode,
		"startedAt":     info.State.StartedAt,
		"finishedAt":    info.State.FinishedAt,
		"cmd":           info.Config.Cmd,
		"entrypoint":    info.Config.Entrypoint,
		"workingDir":    info.Config.WorkingDir,
		"user":          info.Config.User,
		"env":           filterEnvVars(info.Config.Env),
		"labels":        info.Config.Labels,
		"restartPolicy": info.HostConfig.RestartPolicy,
		"networkMode":   string(info.HostConfig.NetworkMode),
		"ports":         buildPorts(info),
		"mounts":        buildMounts(info),
		"networks":      buildNetworks(info),
		"security":      buildSecurity(info),
		"resources":     buildResources(info),
	}

	if info.State.Health != nil {
		resp["health"] = buildHealth(info)
	}

	return resp
}

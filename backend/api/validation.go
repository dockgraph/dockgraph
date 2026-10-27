package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/dockgraph/dockgraph/secrets"
)

// validResourceName matches safe Docker resource identifiers (container names,
// volume names, network names). Used across all inspect and log handlers.
var validResourceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

// validStackName matches compose project / swarm stack names used by the
// ?stack= filters. Unlike validResourceName it accepts a single character,
// since compose allows one-character project names.
var validStackName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// jsonError writes a JSON error response with the correct Content-Type header.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// inspectError answers a failed Docker inspect of a `kind` resource. Only a
// missing resource is a 404; any other error (a daemon or manager timeout)
// says nothing about whether it exists, and must not read as "not found".
func inspectError(w http.ResponseWriter, kind string, err error) {
	if cerrdefs.IsNotFound(err) {
		jsonError(w, kind+" not found", http.StatusNotFound)
		return
	}
	jsonError(w, kind+" inspect failed", http.StatusBadGateway)
}

func filterEnvVars(envList []string) []map[string]string {
	result := make([]map[string]string, 0, len(envList))
	for _, e := range envList {
		k, v, _ := strings.Cut(e, "=")
		if secrets.IsSensitiveKey(k) {
			v = secrets.Masked
		}
		result = append(result, map[string]string{"key": k, "value": v})
	}
	return result
}

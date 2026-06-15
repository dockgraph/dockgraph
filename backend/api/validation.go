package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/dockgraph/dockgraph/secrets"
)

// validResourceName matches safe Docker resource identifiers (container names,
// volume names, network names). Used across all inspect and log handlers.
var validResourceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

// jsonError writes a JSON error response with the correct Content-Type header.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
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

package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

const maskedValue = "********"

// validResourceName matches safe Docker resource identifiers (container names,
// volume names, network names). Used across all inspect and log handlers.
var validResourceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

// sensitiveKeyPatterns matches env var keys that should be masked in API responses.
var sensitiveKeyPatterns = []string{
	"PASSWORD", "SECRET", "KEY", "TOKEN", "CREDENTIAL",
	"API_KEY", "APIKEY", "PRIVATE", "AUTH", "CERT",
	"SSL_", "TLS_", "ENCRYPT", "ACCESS_KEY", "SESSION",
}

// jsonError writes a JSON error response with the correct Content-Type header.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
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
			v = maskedValue
		}
		result = append(result, map[string]string{"key": k, "value": v})
	}
	return result
}

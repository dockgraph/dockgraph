// Package secrets centralises the masking of sensitive values so every code
// path that exposes environment variables — running containers and parsed
// compose services alike — hides credentials consistently.
package secrets

import "strings"

// Masked is the placeholder substituted for sensitive values.
const Masked = "********"

// sensitiveKeyPatterns matches env var keys whose values should be masked.
var sensitiveKeyPatterns = []string{
	"PASSWORD", "SECRET", "KEY", "TOKEN", "CREDENTIAL",
	"API_KEY", "APIKEY", "PRIVATE", "AUTH", "CERT",
	"SSL_", "TLS_", "ENCRYPT", "ACCESS_KEY", "SESSION",
}

// IsSensitiveKey reports whether an env var key looks like a credential and
// should have its value masked before leaving the server.
func IsSensitiveKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, pattern := range sensitiveKeyPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// MaskLabels returns a copy of labels with credential-looking values masked,
// using the same key rule as environment variables. Labels often carry
// reverse-proxy settings such as Traefik basic-auth user hashes.
func MaskLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		if IsSensitiveKey(k) {
			v = Masked
		}
		out[k] = v
	}
	return out
}

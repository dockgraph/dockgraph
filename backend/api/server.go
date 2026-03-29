package api

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

// HealthChecker tests whether the backing service is reachable.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// NewServer sets up the HTTP routes for the application:
//   - GET /healthz — checks Docker daemon connectivity
//   - GET /ws      — upgrades to WebSocket for live graph updates
//   - /*           — serves the embedded frontend SPA
func NewServer(hub *Hub, staticFS fs.FS, health HealthChecker) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		err := health.HealthCheck(r.Context())
		if err != nil {
			log.Printf("healthcheck failed: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "docker unreachable")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	mux.HandleFunc("GET /ws", hub.HandleWS)
	mux.HandleFunc("/", spaHandler(staticFS))

	return securityHeaders(mux)
}

// securityHeaders adds baseline security headers to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}

// spaHandler serves static files and falls back to index.html for client-side routing.
func spaHandler(fsys fs.FS) http.HandlerFunc {
	fileServer := http.FileServerFS(fsys)

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = strings.TrimPrefix(path, "/")
		}

		if _, err := fs.Stat(fsys, path); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}
}

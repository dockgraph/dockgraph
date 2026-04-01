package api

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/dockgraph/dockgraph/auth"
)

// HealthChecker tests whether the backing service is reachable.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// DockerAPI groups the Docker client interfaces needed by API handlers.
type DockerAPI interface {
	ContainerInspector
	ContainerLogger
	VolumeInspector
	NetworkInspector
}

// NewServer sets up the HTTP routes for the application.
// Pass nil for authService to disable authentication.
func NewServer(hub *Hub, staticFS fs.FS, health HealthChecker, authService *auth.Service, docker DockerAPI) http.Handler {
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

	if authService != nil {
		mux.HandleFunc("POST /api/login", authService.Login())
		mux.HandleFunc("POST /api/logout", authService.Logout())
		mux.HandleFunc("GET /api/auth/check", authService.Check())
	}

	mux.HandleFunc("GET /ws", hub.HandleWS)
	if docker != nil {
		mux.HandleFunc("GET /api/containers/{id}/logs/history", HandleContainerLogsHistory(docker))
		mux.HandleFunc("GET /api/containers/{id}/logs", HandleContainerLogs(docker))
		mux.HandleFunc("GET /api/containers/{id}", HandleContainerInspect(docker))
		mux.HandleFunc("GET /api/volumes/{name}", HandleVolumeInspect(docker))
		mux.HandleFunc("GET /api/networks/{name}", HandleNetworkInspect(docker))
	}
	mux.HandleFunc("/", spaHandler(staticFS))

	handler := securityHeaders(mux)
	if authService != nil {
		handler = authService.Middleware(handler)
	}

	return handler
}

// securityHeaders adds baseline security headers to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:")
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

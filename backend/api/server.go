package api

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/dockgraph/dockgraph/auth"
	"github.com/dockgraph/dockgraph/collector"
)

// HealthChecker tests whether the backing service is reachable.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// DockerAPI groups the Docker client interfaces needed by API handlers.
type DockerAPI interface {
	ContainerInspector
	ContainerLogger
	ContainerLister
	EventSubscriber
	VolumeInspector
	NetworkInspector
	ServiceInspector
	ServiceLogger
	ServiceLister
}

// SystemAPI groups Docker system-level interfaces for dashboard handlers.
type SystemAPI interface {
	SystemInfoProvider
	SystemDiskUsageProvider
	ImageLister
}

// NewServer sets up the HTTP routes for the application.
// Pass nil for authService to disable authentication. mode is the resolved
// runtime mode (collector.ModeStandalone or collector.ModeSwarm); swarm mode
// enables the service endpoints and cluster-wide service logs.
func NewServer(hub *Hub, staticFS fs.FS, health HealthChecker, authService *auth.Service, docker DockerAPI, system SystemAPI, statsHistory *collector.StatsHistory, eventHistory *collector.EventHistory, mode string, opts ...ServerOption) http.Handler {
	var o serverOptions
	for _, opt := range opts {
		opt(&o)
	}
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
		// Service logs are only aggregated in swarm mode; a standalone daemon
		// rejects swarm API calls.
		var services StackServiceAPI
		if mode == collector.ModeSwarm {
			services = docker
			mux.HandleFunc("GET /api/services/{id}/logs/history", HandleServiceLogsHistory(docker))
			mux.HandleFunc("GET /api/services/{id}/logs", HandleServiceLogs(docker))
			mux.HandleFunc("GET /api/services/{id}", HandleServiceInspect(docker, docker))
		}
		// Containers on other swarm nodes are served by their node's agent.
		remote := o.agentProxy
		mux.HandleFunc("GET /api/containers/{id}/logs/history", remote.wrap(HandleContainerLogsHistory(docker), docker, "/logs/history", false))
		mux.HandleFunc("GET /api/containers/{id}/logs", remote.wrap(HandleContainerLogs(docker), docker, "/logs", true))
		mux.HandleFunc("GET /api/logs/history", HandleAggregateLogsHistory(docker, docker, services))
		mux.HandleFunc("GET /api/logs", HandleAggregateLogs(docker, docker, docker, services))
		mux.HandleFunc("GET /api/containers/{id}", remote.wrap(HandleContainerInspect(docker), docker, "", false))
		mux.HandleFunc("GET /api/volumes/{name}", HandleVolumeInspect(docker))
		mux.HandleFunc("GET /api/networks/{name}", HandleNetworkInspect(docker))
	}
	if system != nil {
		mux.HandleFunc("GET /api/system/info", HandleSystemInfo(system, mode))
		mux.HandleFunc("GET /api/system/disk-usage", HandleSystemDiskUsage(system))
		mux.HandleFunc("GET /api/images", HandleImages(system))
	}
	if statsHistory != nil {
		var lister ContainerLister
		var serviceLister ServiceLister
		if docker != nil {
			lister = docker
			if mode == collector.ModeSwarm {
				serviceLister = docker
			}
		}
		mux.HandleFunc("GET /api/stats/history", HandleStatsHistory(statsHistory, lister, serviceLister))
	}
	if eventHistory != nil {
		mux.HandleFunc("GET /api/events/recent", HandleRecentEvents(eventHistory))
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
			path = indexHTMLPath
		} else {
			path = strings.TrimPrefix(path, "/")
		}

		if _, err := fs.Stat(fsys, path); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}
}

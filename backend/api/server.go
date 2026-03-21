package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/docker/docker/client"
)

// NewServer sets up the HTTP routes for the application:
//   - GET /healthz — checks Docker daemon connectivity
//   - GET /ws      — upgrades to WebSocket for live graph updates
//   - /*           — serves the embedded frontend SPA
func NewServer(hub *Hub, staticFS fs.FS, dockerCli client.APIClient) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, err := dockerCli.Ping(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "docker unreachable: %v", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	mux.HandleFunc("GET /ws", hub.HandleWS)
	mux.HandleFunc("/", spaHandler(staticFS))

	return mux
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

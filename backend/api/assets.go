package api

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// During a rolling update the routing mesh can send index.html from one
// build and its hashed assets to a replica of another, whose binary embeds
// only its own. Replicas therefore fetch an asset they lack from their
// peers: whichever build the page came from, some replica still has it.

const (
	assetsPrefix = "assets/"
	// peerHeader marks a request made by a peer, which must answer from its
	// own files only: otherwise two replicas missing the same asset would
	// ask each other forever.
	peerHeader       = "X-Dockgraph-Peer"
	peerFetchTimeout = 5 * time.Second
)

// PeerFinder returns the base URLs (http://host:port) of the other server
// replicas.
type PeerFinder func(ctx context.Context) []string

// WithAssetPeers lets the server fetch static assets missing from its
// embedded build from the other replicas.
func WithAssetPeers(find PeerFinder) ServerOption {
	return func(o *serverOptions) { o.assetPeers = find }
}

// assetFromPeers serves the asset at path (no leading slash) from the first
// peer that has it, and reports whether it did. The request's cookie goes
// along: replicas share the session key, so a peer accepts it.
func assetFromPeers(w http.ResponseWriter, r *http.Request, find PeerFinder, path string) bool {
	ctx, cancel := context.WithTimeout(r.Context(), peerFetchTimeout)
	defer cancel()
	for _, base := range find(ctx) {
		target, err := url.JoinPath(base, path)
		if err != nil {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			continue
		}
		req.Header.Set(peerHeader, "1")
		if c := r.Header.Get("Cookie"); c != "" {
			req.Header.Set("Cookie", c)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteResponseBytes+1))
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || int64(len(body)) > maxRemoteResponseBytes {
			continue
		}
		h := w.Header()
		for _, k := range []string{"Content-Type", "Cache-Control", "Last-Modified", "Etag"} {
			if v := resp.Header.Get(k); v != "" {
				h.Set(k, v)
			}
		}
		h.Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
		return true
	}
	return false
}

// spaHandler serves static files and falls back to index.html for
// client-side routing. A missing file under assets/ is never index.html:
// the browser would take it for a broken script or stylesheet. It is looked
// up on the peers, and is a plain 404 if none has it.
func spaHandler(fsys fs.FS, peers PeerFinder) http.HandlerFunc {
	fileServer := http.FileServerFS(fsys)

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = indexHTMLPath
		} else {
			path = strings.TrimPrefix(path, "/")
		}

		if _, err := fs.Stat(fsys, path); err != nil {
			if strings.HasPrefix(path, assetsPrefix) {
				if peers != nil && r.Header.Get(peerHeader) == "" && assetFromPeers(w, r, peers, path) {
					return
				}
				http.NotFound(w, r)
				return
			}
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}
}

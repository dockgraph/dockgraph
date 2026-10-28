package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// peer starts a replica whose build holds the given files.
func peer(t *testing.T, files fstest.MapFS) string {
	t.Helper()
	srv := httptest.NewServer(spaHandler(files, nil))
	t.Cleanup(srv.Close)
	return srv.URL
}

func finder(urls ...string) PeerFinder {
	return func(context.Context) []string { return urls }
}

func getPath(h http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// A replica of the new build gets the old build's asset from a peer that still
// has it, instead of answering index.html for a script request.
func TestSPAHandlerFetchesMissingAssetFromPeer(t *testing.T) {
	old := peer(t, fstest.MapFS{"assets/app-old.js": {Data: []byte("old()")}})
	h := spaHandler(fstest.MapFS{"index.html": {Data: []byte("<html>new</html>")}}, finder("http://127.0.0.1:1", old))

	w := getPath(h, "/assets/app-old.js", nil)
	if w.Code != http.StatusOK || w.Body.String() != "old()" {
		t.Errorf("got %d %q, want the peer's asset", w.Code, w.Body)
	}
}

func TestSPAHandlerMissingAssetIsNotIndexHTML(t *testing.T) {
	h := spaHandler(fstest.MapFS{"index.html": {Data: []byte("<html>")}}, finder(peer(t, fstest.MapFS{})))
	if w := getPath(h, "/assets/gone.js", nil); w.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", w.Code)
	}
	nopeers := spaHandler(fstest.MapFS{"index.html": {Data: []byte("<html>")}}, nil)
	if w := getPath(nopeers, "/assets/gone.js", nil); w.Code != http.StatusNotFound {
		t.Errorf("no peers: status %d, want 404", w.Code)
	}
	if w := getPath(nopeers, "/some/route", nil); w.Body.String() != "<html>" {
		t.Errorf("client route got %q, want index.html", w.Body)
	}
}

// A request made by a peer never asks further peers, so replicas missing the
// same asset cannot loop.
func TestSPAHandlerPeerRequestDoesNotRecurse(t *testing.T) {
	has := peer(t, fstest.MapFS{"assets/a.js": {Data: []byte("a")}})
	h := spaHandler(fstest.MapFS{}, finder(has))
	if w := getPath(h, "/assets/a.js", map[string]string{peerHeader: "1"}); w.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", w.Code)
	}
}

func TestSPAHandlerForwardsCookieToPeer(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Cookie")
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()
	h := spaHandler(fstest.MapFS{}, finder(srv.URL))
	getPath(h, "/assets/a.js", map[string]string{"Cookie": "dg_session=abc"})
	if got != "dg_session=abc" {
		t.Errorf("peer saw cookie %q", got)
	}
}

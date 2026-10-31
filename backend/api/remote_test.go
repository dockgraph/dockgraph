package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/dockgraph/dockgraph/agentauth"
	"github.com/dockgraph/dockgraph/auth"
)

// stubLocator maps container IDs to agent base URLs.
type stubLocator struct {
	mu       sync.Mutex
	urls     map[string]string // found by LocateContainer
	calls    int               // LocateContainer calls
	rejected bool              // CredentialsRejected result
}

func (s *stubLocator) CredentialsRejected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rejected
}

func (s *stubLocator) LocateContainer(_ context.Context, id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	u, ok := s.urls[id]
	return u, ok
}

// agentRecorder is a fake agent recording the last request it received.
type agentRecorder struct {
	mu      sync.Mutex
	path    string
	query   string
	auth    string
	cookie  string
	status  int
	body    string
	ctype   string
	chunked bool          // flush headers first so the body is sent chunked
	blockCh chan struct{} // when set, stream one event then block until closed
}

func (a *agentRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.path, a.query = r.URL.Path, r.URL.RawQuery
	a.auth, a.cookie = r.Header.Get("Authorization"), r.Header.Get("Cookie")
	status, body, ctype, chunked, block := a.status, a.body, a.ctype, a.chunked, a.blockCh
	a.mu.Unlock()
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", ctype)
	w.WriteHeader(status)
	if chunked {
		w.(http.Flusher).Flush()
	}
	_, _ = io.WriteString(w, body)
	if block != nil {
		w.(http.Flusher).Flush()
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}
}

func newProxiedServer(t *testing.T, agent http.Handler, locator *stubLocator) *httptest.Server {
	t.Helper()
	agentSrv := httptest.NewServer(agent)
	t.Cleanup(agentSrv.Close)
	for id := range locator.urls {
		locator.urls[id] = agentSrv.URL
	}
	docker := &stubDockerAPI{logger: &stubContainerLogger{}} // ContainerInspect always fails: nothing is local
	handler := NewServer(NewHub(), fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, docker, nil, nil, nil, "swarm",
		WithAgentProxy(&AgentProxy{Locator: locator, Auth: agentauth.Token("tok")}))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestAgentProxyInspect(t *testing.T) {
	agent := &agentRecorder{body: `{"name":"shop_web.2.t2"}`, ctype: "application/json"}
	locator := &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}}
	srv := newProxiedServer(t, agent, locator)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/containers/shop_web.2.t2", nil)
	req.Header.Set("Cookie", "dg_session=browser-secret")
	req.Header.Set("Authorization", "Bearer user-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK || string(body) != `{"name":"shop_web.2.t2"}` {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
	if agent.path != "/agent/v1/containers/shop_web.2.t2" {
		t.Errorf("agent path %q", agent.path)
	}
	if agent.auth != "Bearer tok" {
		t.Errorf("agent auth %q, want server token", agent.auth)
	}
	if agent.cookie != "" {
		t.Errorf("browser cookie forwarded: %q", agent.cookie)
	}
}

func TestAgentProxyLogsHistoryKeepsQuery(t *testing.T) {
	agent := &agentRecorder{body: `{"lines":[]}`, ctype: "application/json"}
	locator := &stubLocator{urls: map[string]string{"abcdef123456": ""}}
	srv := newProxiedServer(t, agent, locator)

	resp, err := http.Get(srv.URL + "/api/containers/abcdef123456/logs/history?limit=5&before=2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if agent.path != "/agent/v1/containers/abcdef123456/logs/history" || agent.query != "limit=5&before=2026-01-01T00:00:00Z" {
		t.Errorf("agent got %s?%s", agent.path, agent.query)
	}
}

func TestAgentProxyLogsStreamFlushesAndCancels(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	agent := &agentRecorder{body: "data: {\"line\":\"hi\"}\n\n", ctype: "text/event-stream", blockCh: block}
	locator := &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}}
	srv := newProxiedServer(t, agent, locator)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/containers/shop_web.2.t2/logs?tail=10", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	buf := make([]byte, 64)
	n, err := resp.Body.Read(buf)
	if err != nil || !strings.Contains(string(buf[:n]), `"line":"hi"`) {
		t.Fatalf("first event not flushed: %q %v", buf[:n], err)
	}
	if agent.path != "/agent/v1/containers/shop_web.2.t2/logs" || agent.query != "tail=10" {
		t.Errorf("agent got %s?%s", agent.path, agent.query)
	}
}

// Every route that reaches an agent signs its request, and the request the
// agent receives (after the reverse proxy rewrote host, path and headers)
// is exactly what was signed.
func TestAgentProxySignedRequestsVerify(t *testing.T) {
	keyText, err := agentauth.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	key, _ := agentauth.ParsePrivateKey(keyText)
	publicKeys, _ := agentauth.ParsePublicKeys(agentauth.PublicKeyText(key))
	verifier := agentauth.NewVerifier(publicKeys)

	var mu sync.Mutex
	var verified []error
	agentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := verifier.Verify(r)
		mu.Lock()
		verified = append(verified, err)
		mu.Unlock()
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"name":"x"}`)
	}))
	t.Cleanup(agentSrv.Close)

	proxy := &AgentProxy{
		Locator: &stubLocator{urls: map[string]string{"shop_web.2.t2": agentSrv.URL}},
		Agents:  stubAgents{{Hostname: "worker-1", BaseURL: agentSrv.URL}},
		Auth:    agentauth.NewSigner(key),
	}
	docker := &stubDockerAPI{logger: &stubContainerLogger{}} // nothing is local
	srv := httptest.NewServer(NewServer(NewHub(), fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, docker, nil, nil, nil, "swarm",
		WithAgentProxy(proxy)))
	t.Cleanup(srv.Close)

	for _, path := range []string{
		"/api/containers/shop_web.2.t2",
		"/api/containers/shop_web.2.t2/logs/history?limit=5&before=2026-01-01T00:00:00.000000000Z",
		"/api/containers/shop_web.2.t2/logs?tail=10",
		"/api/volumes/shop_data",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d", path, resp.StatusCode)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(verified) != 4 {
		t.Fatalf("agent received %d requests, want 4", len(verified))
	}
	for i, err := range verified {
		if err != nil {
			t.Errorf("request %d: %v", i+1, err)
		}
	}
}

func TestAgentProxyAgentAuthFailureIsBadGateway(t *testing.T) {
	agent := &agentRecorder{status: http.StatusUnauthorized, body: `{"error":"unauthorized"}`, ctype: "application/json"}
	locator := &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}}
	srv := newProxiedServer(t, agent, locator)

	resp, err := http.Get(srv.URL + "/api/containers/shop_web.2.t2")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway || !strings.Contains(string(body), "DG_AGENT_TOKEN") {
		t.Errorf("status %d body %s, want 502 naming the token", resp.StatusCode, body)
	}
}

// When agents reject the token they are never discovered, so their
// containers cannot be located at all: that must not read as "not found".
func TestAgentProxyTokenRejectedIsBadGateway(t *testing.T) {
	locator := &stubLocator{urls: map[string]string{}, rejected: true}
	srv := newProxiedServer(t, &agentRecorder{}, locator)

	resp, err := http.Get(srv.URL + "/api/containers/abcdef123456")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway || !strings.Contains(string(body), "DG_AGENT_TOKEN") {
		t.Errorf("status %d body %s, want 502 naming the token", resp.StatusCode, body)
	}
}

// setRemoteCaps lowers both one-shot response caps for a test.
func setRemoteCaps(t *testing.T, limit int64) {
	t.Helper()
	oldResponse, oldInspect := maxRemoteResponseBytes, maxRemoteInspectBytes
	maxRemoteResponseBytes, maxRemoteInspectBytes = limit, limit
	t.Cleanup(func() { maxRemoteResponseBytes, maxRemoteInspectBytes = oldResponse, oldInspect })
}

func TestAgentProxyOversizedResponseIsBadGateway(t *testing.T) {
	setRemoteCaps(t, 64)

	big := `{"name":"` + strings.Repeat("x", 100) + `"}`
	for _, path := range []string{"/api/containers/shop_web.2.t2", "/api/containers/shop_web.2.t2/logs/history"} {
		for _, chunked := range []bool{false, true} {
			agent := &agentRecorder{body: big, ctype: "application/json", chunked: chunked}
			locator := &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}}
			srv := newProxiedServer(t, agent, locator)

			resp, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadGateway {
				t.Errorf("%s chunked=%v: status %d body %s, want 502", path, chunked, resp.StatusCode, body)
			}
		}
	}
}

// An inspect answer is decoded to be masked again, so its cap is the lower
// of the two.
func TestAgentProxyInspectCapIsLower(t *testing.T) {
	if maxRemoteInspectBytes >= maxRemoteResponseBytes {
		t.Errorf("inspect cap %d, want below the response cap %d", maxRemoteInspectBytes, maxRemoteResponseBytes)
	}
}

func TestAgentProxyChunkedResponseAtCapPassesThrough(t *testing.T) {
	want := `{"name":"shop_web.2.t2"}`
	setRemoteCaps(t, int64(len(want)))

	agent := &agentRecorder{body: want, ctype: "application/json", chunked: true}
	locator := &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}}
	srv := newProxiedServer(t, agent, locator)

	resp, err := http.Get(srv.URL + "/api/containers/shop_web.2.t2")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != want {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
}

func TestAgentProxyUnreachableAgent(t *testing.T) {
	locator := &stubLocator{urls: map[string]string{"shop_web.2.t2": "http://127.0.0.1:1"}}
	proxy := &AgentProxy{Locator: locator, Auth: agentauth.Token("tok")}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/containers/{id}", proxy.wrap(HandleContainerInspect(&stubContainerInspector{err: errNotFoundStub}), &stubContainerInspector{err: errNotFoundStub}, "", false))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/containers/shop_web.2.t2", nil))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status %d, want 502", rec.Code)
	}
}

func TestAgentProxyLocalAndUnknownServedLocally(t *testing.T) {
	locator := &stubLocator{urls: map[string]string{}}
	proxy := &AgentProxy{Locator: locator, Auth: agentauth.Token("tok")}

	// Local container: served locally, locator not consulted.
	local := &stubContainerInspector{result: validContainerInspectResponse()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/containers/{id}", proxy.wrap(HandleContainerInspect(local), local, "", false))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/containers/web", nil))
	if rec.Code != http.StatusOK || locator.calls != 0 {
		t.Errorf("local: status %d, locator calls %d", rec.Code, locator.calls)
	}

	// Unknown everywhere: the local handler answers 404.
	missing := &stubContainerInspector{result: containertypes.InspectResponse{}, err: errNotFoundStub}
	mux = http.NewServeMux()
	mux.HandleFunc("GET /api/containers/{id}", proxy.wrap(HandleContainerInspect(missing), missing, "", false))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/containers/ghost", nil))
	if rec.Code != http.StatusNotFound || locator.calls != 1 {
		t.Errorf("unknown: status %d, locator calls %d", rec.Code, locator.calls)
	}

	// A local daemon error other than not-found says nothing about where
	// the container runs: the local handler reports it, and the locator,
	// which could find a same-named container elsewhere, isn't consulted.
	locator.urls["web"] = "http://127.0.0.1:1"
	flaky := &stubContainerInspector{err: context.DeadlineExceeded}
	mux = http.NewServeMux()
	mux.HandleFunc("GET /api/containers/{id}", proxy.wrap(HandleContainerInspect(flaky), flaky, "", false))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/containers/web", nil))
	if locator.calls != 1 {
		t.Errorf("transient local error: locator calls %d, want 1 (unchanged)", locator.calls)
	}

	// Invalid IDs never reach the locator.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/containers/-bad", nil))
	if rec.Code != http.StatusBadRequest || locator.calls != 1 {
		t.Errorf("invalid: status %d, locator calls %d", rec.Code, locator.calls)
	}
}

// countingInspector counts ContainerInspect calls.
type countingInspector struct {
	stubContainerInspector
	mu    sync.Mutex
	calls int
}

func (c *countingInspector) ContainerInspect(ctx context.Context, id string) (containertypes.InspectResponse, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return c.stubContainerInspector.ContainerInspect(ctx, id)
}

// A container the local daemon has is served locally even when the locator
// would place it on another node.
func TestAgentProxyLocalContainerWinsOverLocator(t *testing.T) {
	agent := &agentRecorder{body: `{"name":"from-agent"}`, ctype: "application/json"}
	agentSrv := httptest.NewServer(agent)
	t.Cleanup(agentSrv.Close)
	locator := &stubLocator{urls: map[string]string{"web": agentSrv.URL}}
	proxy := &AgentProxy{Locator: locator, Auth: agentauth.Token("tok")}
	inspector := &countingInspector{stubContainerInspector: stubContainerInspector{result: validContainerInspectResponse()}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/containers/{id}", proxy.wrap(HandleContainerInspect(inspector), inspector, "", false))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/containers/web", nil))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "from-agent") {
		t.Fatalf("status %d body %s, want the local container", rec.Code, rec.Body.String())
	}
	if locator.calls != 0 || agent.path != "" {
		t.Errorf("locator calls %d, agent path %q; want neither for a local container", locator.calls, agent.path)
	}
}

// Only a fixed content type passes from an agent to the browser, so an
// agent's answer can never be read as a page or script of this origin.
func TestAgentProxyFixesResponseHeaders(t *testing.T) {
	for _, tc := range []struct {
		name, path, ctype string
		status            int
		want              string
	}{
		{"one-shot as HTML", "/api/containers/shop_web.2.t2", "text/html", http.StatusOK, "application/json"},
		{"history as script", "/api/containers/shop_web.2.t2/logs/history", "application/javascript", http.StatusOK, "application/json"},
		{"stream", "/api/containers/shop_web.2.t2/logs", "text/html", http.StatusOK, "text/event-stream"},
		{"stream error", "/api/containers/shop_web.2.t2/logs", "text/html", http.StatusInternalServerError, "application/json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := &agentRecorder{body: `{}`, ctype: tc.ctype, status: tc.status}
			locator := &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}}
			srv := newProxiedServer(t, agent, locator)

			resp, err := http.Get(srv.URL + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if got := resp.Header.Get("Content-Type"); got != tc.want {
				t.Errorf("Content-Type = %q, want %q", got, tc.want)
			}
			if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
		})
	}
}

func TestAgentProxyRedirectIsBadGateway(t *testing.T) {
	agent := &agentRecorder{status: http.StatusFound, ctype: "text/html"}
	locator := &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}}
	srv := newProxiedServer(t, agent, locator)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(srv.URL + "/api/containers/shop_web.2.t2")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status %d, want 502", resp.StatusCode)
	}
}

// proxiedRoutes are the routes that reach a container's agent: one-shot
// inspect and log history, and the log stream.
var proxiedRoutes = []string{
	"/api/containers/shop_web.2.t2",
	"/api/containers/shop_web.2.t2/logs/history",
	"/api/containers/shop_web.2.t2/logs",
}

// An agent that switches protocols gets a 502 on every route: the browser's
// connection is never handed over to the agent.
func TestAgentProxyRefusesProtocolSwitch(t *testing.T) {
	agent := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n\r\nHTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 5\r\n\r\nhello")
		_ = buf.Flush()
		_, _ = io.Copy(io.Discard, conn) // hold the connection until the server drops it
	})
	srv := newProxiedServer(t, agent, &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}})
	client := &http.Client{Timeout: 5 * time.Second}

	for _, path := range proxiedRoutes {
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadGateway || strings.Contains(string(body), "hello") {
			t.Errorf("%s: status %d body %q, want 502", path, resp.StatusCode, body)
		}
	}
}

// Informational responses from an agent (103 Early Hints) don't reach the
// browser, and the final response keeps the server's security headers.
func TestAgentProxyDropsInformationalResponses(t *testing.T) {
	agent := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Link", "</x.js>; rel=preload; as=script")
		w.Header().Set("Set-Cookie", "dg_session=from-agent")
		w.WriteHeader(http.StatusEarlyHints)
		w.Header().Del("Link")
		w.Header().Del("Set-Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	})
	srv := newProxiedServer(t, agent, &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}})

	for _, path := range proxiedRoutes {
		var informational []int
		trace := &httptrace.ClientTrace{Got1xxResponse: func(code int, _ textproto.MIMEHeader) error {
			informational = append(informational, code)
			return nil
		}}
		ctx, cancel := context.WithTimeout(httptrace.WithClientTrace(context.Background(), trace), 5*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			t.Fatalf("%s: %v", path, err)
		}
		if path == proxiedRoutes[2] {
			buf := make([]byte, 2)
			_, _ = resp.Body.Read(buf) // the stream stays open; its headers are what matters
		} else {
			_, _ = io.ReadAll(resp.Body)
		}
		resp.Body.Close()
		cancel()

		if len(informational) != 0 {
			t.Errorf("%s: informational responses reached the client: %v", path, informational)
		}
		for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options"} {
			if resp.Header.Get(h) == "" {
				t.Errorf("%s: final response lost %s", path, h)
			}
		}
		if c := resp.Header.Get("Set-Cookie"); c != "" {
			t.Errorf("%s: agent cookie reached the client: %q", path, c)
		}
	}
}

// Trailers an agent sends after its body are dropped like its headers.
func TestAgentProxyDropsTrailers(t *testing.T) {
	agent := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Trailer", "X-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
		w.Header().Set("X-Agent", "announced")
		w.Header().Set(http.TrailerPrefix+"Set-Cookie", "dg_session=from-agent")
	})
	srv := newProxiedServer(t, agent, &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}})
	client := &http.Client{Timeout: 5 * time.Second}

	for _, path := range proxiedRoutes {
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if len(resp.Trailer) != 0 {
			t.Errorf("%s: agent trailers reached the client: %v", path, resp.Trailer)
		}
	}
}

// An agent's error answer on the log stream route is bounded like a
// one-shot response; only a 200 stream is unbounded.
func TestAgentProxyStreamErrorIsBounded(t *testing.T) {
	setRemoteCaps(t, 64)

	agent := &agentRecorder{status: http.StatusInternalServerError, ctype: "application/json", body: `{"error":"` + strings.Repeat("x", 100) + `"}`}
	srv := newProxiedServer(t, agent, &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}})

	resp, err := http.Get(srv.URL + "/api/containers/shop_web.2.t2/logs")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway || strings.Contains(string(body), "xxxx") {
		t.Errorf("status %d body %q, want 502 without the agent's body", resp.StatusCode, body)
	}
}

// The server masks an agent's inspect answer as it masks its own, so
// credentials stay masked whichever agent version answers.
func TestAgentProxyMasksAgentInspect(t *testing.T) {
	agent := &agentRecorder{ctype: "application/json", body: `{"name":"shop_web.2.t2","image":"shop/web:1",` +
		`"env":[{"key":"DATABASE_URL","value":"postgres://app:pw-in-env@db/app"},{"key":"LOG_LEVEL","value":"debug"}],` +
		`"labels":{"app.db.password":"pw-in-label","tier":"web"},` +
		`"entrypoint":["app","--password"],"cmd":["pw-in-cmd"],` +
		`"health":{"status":"healthy","failingStreak":0,"log":[{"exitCode":0,"output":"token=pw-in-health"}]}}`}
	srv := newProxiedServer(t, agent, &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}})

	resp, err := http.Get(srv.URL + "/api/containers/shop_web.2.t2")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "pw-in-") {
		t.Errorf("agent credentials reached the client: %s", body)
	}
	for _, kept := range []string{`"shop_web.2.t2"`, `"shop/web:1"`, `"debug"`, `"web"`, `"healthy"`} {
		if !strings.Contains(string(body), kept) {
			t.Errorf("%s missing from %s", kept, body)
		}
	}
}

// Agents answer a success with 200 only; any other success status would
// skip the masking of a 200, so it is refused.
func TestAgentProxyRefusesOtherSuccessStatuses(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusNonAuthoritativeInfo, http.StatusPartialContent} {
		agent := &agentRecorder{status: status, ctype: "application/json", body: `{"env":[{"key":"DB_PASSWORD","value":"pw-in-env"}]}`}
		srv := newProxiedServer(t, agent, &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}})
		resp, err := http.Get(srv.URL + "/api/containers/shop_web.2.t2")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadGateway || strings.Contains(string(body), "pw-in-") {
			t.Errorf("agent %d: status %d body %s, want 502", status, resp.StatusCode, body)
		}
	}
}

// An agent answer that isn't an inspect object is refused rather than
// passed through unmasked.
func TestAgentProxyRefusesMalformedAgentInspect(t *testing.T) {
	agent := &agentRecorder{ctype: "application/json", body: `["not", "an", "object"]`}
	srv := newProxiedServer(t, agent, &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}})

	resp, err := http.Get(srv.URL + "/api/containers/shop_web.2.t2")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status %d, want 502", resp.StatusCode)
	}
}

// Requests made for clients are bounded per agent, so client traffic can't
// fill the agent's replay cache; over the bound the client gets a 503.
func TestAgentProxyBoundsRequestsPerAgent(t *testing.T) {
	oldRate, oldBurst := agentRequestRate, agentRequestBurst
	agentRequestRate, agentRequestBurst = 0.001, 2
	t.Cleanup(func() { agentRequestRate, agentRequestBurst = oldRate, oldBurst })

	agent := &agentRecorder{body: `{"name":"shop_web.2.t2"}`, ctype: "application/json"}
	srv := newProxiedServer(t, agent, &stubLocator{urls: map[string]string{"shop_web.2.t2": ""}})

	var codes []int
	for range 3 {
		resp, err := http.Get(srv.URL + "/api/containers/shop_web.2.t2")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		codes = append(codes, resp.StatusCode)
		if resp.StatusCode == http.StatusServiceUnavailable && resp.Header.Get("Retry-After") == "" {
			t.Error("503 without Retry-After")
		}
	}
	if codes[0] != http.StatusOK || codes[1] != http.StatusOK || codes[2] != http.StatusServiceUnavailable {
		t.Errorf("statuses %v, want [200 200 503]", codes)
	}
}

// One client gets only a share of an agent's requests, so it can't use them
// up for everyone else.
func TestAgentProxyBoundsEachClientsShare(t *testing.T) {
	oldRate, oldBurst := agentClientRequestRate, agentClientRequestBurst
	agentClientRequestRate, agentClientRequestBurst = 0.001, 2
	t.Cleanup(func() { agentClientRequestRate, agentClientRequestBurst = oldRate, oldBurst })

	agentSrv := httptest.NewServer(&agentRecorder{body: `{"name":"shop_web.2.t2"}`, ctype: "application/json"})
	t.Cleanup(agentSrv.Close)
	proxies, err := auth.ParseTrustedProxies("127.0.0.1/32")
	if err != nil {
		t.Fatal(err)
	}
	docker := &stubDockerAPI{logger: &stubContainerLogger{}}
	srv := httptest.NewServer(NewServer(NewHub(), fstest.MapFS{"index.html": {Data: []byte("ok")}}, &stubHealth{}, nil, docker, nil, nil, nil, "swarm",
		WithTrustedProxies(proxies),
		WithAgentProxy(&AgentProxy{Locator: &stubLocator{urls: map[string]string{"shop_web.2.t2": agentSrv.URL}}, Auth: agentauth.Token("tok")})))
	t.Cleanup(srv.Close)

	get := func(client string) int {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/containers/shop_web.2.t2", nil)
		req.Header.Set("X-Forwarded-For", client)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if a := []int{get("203.0.113.1"), get("203.0.113.1"), get("203.0.113.1")}; !slices.Equal(a, []int{200, 200, 503}) {
		t.Errorf("client a: %v, want [200 200 503]", a)
	}
	if b := get("203.0.113.2"); b != http.StatusOK {
		t.Errorf("client b after a used its share: %d, want 200", b)
	}
}

func TestAgentProxySharesTransport(t *testing.T) {
	p := &AgentProxy{Locator: &stubLocator{}}
	if a, b := p.transport(), p.transport(); a == nil || a != b {
		t.Error("wrapped routes must share one transport")
	}
	custom := &http.Transport{}
	p = &AgentProxy{Locator: &stubLocator{}, Transport: custom}
	if p.transport() != custom {
		t.Error("a configured Transport must be used")
	}
}

func TestAgentProxyNilIsPassthrough(t *testing.T) {
	var p *AgentProxy
	called := false
	h := p.wrap(func(http.ResponseWriter, *http.Request) { called = true }, nil, "", false)
	h(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Error("nil proxy must return the local handler")
	}
}

var errNotFoundStub = &notFoundErr{}

type notFoundErr struct{}

func (*notFoundErr) Error() string { return "No such container" }

// NotFound marks the error as Docker's not-found (errdefs.IsNotFound).
func (*notFoundErr) NotFound() {}

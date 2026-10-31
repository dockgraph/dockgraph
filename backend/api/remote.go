package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/dockgraph/dockgraph/agentauth"
	"github.com/dockgraph/dockgraph/collector"
	"golang.org/x/time/rate"
)

// Remote proxy timeouts. Streams (SSE logs) have no overall deadline and end
// when the client disconnects; one-shot requests are bounded.
const (
	remoteDialTimeout    = 5 * time.Second
	remoteHeaderTimeout  = 15 * time.Second
	remoteRequestTimeout = 20 * time.Second
	localProbeTimeout    = 5 * time.Second
)

// maxRemoteResponseBytes caps a proxied one-shot agent response (a page of
// log history, an error), maxRemoteInspectBytes a container inspect, which
// is decoded to be masked again. An agent's page of history is bounded by
// maxHistoryPageBytes of line text, which JSON escaping can grow several
// times. Variables so tests can lower them.
var (
	maxRemoteResponseBytes int64 = 4 << 20
	maxRemoteInspectBytes  int64 = 1 << 20
)

// agentRequestRate and agentRequestBurst bound the requests the server
// makes to one agent for clients (inspect, logs, volumes). An agent keeps
// every request it accepted for as long as its signature stays valid (see
// agentauth.MaxClockSkew), up to a fixed number, and refuses the server
// while full; at this rate client traffic stays far below that number.
// Variables so tests can lower them.
var (
	agentRequestRate  rate.Limit = 100 // per second
	agentRequestBurst            = 100
	// agentClientRequestRate and agentClientRequestBurst bound one client's
	// share of an agent's requests, so that one client can't use them up for
	// everyone else.
	agentClientRequestRate  rate.Limit = 25 // per second
	agentClientRequestBurst            = 25
)

// maxAgentLimiters bounds the per-agent limiters kept; agent addresses
// change as nodes come and go, so the set is started afresh when full.
const maxAgentLimiters = 1024

// errAgentBusy reports a request over agentRequestRate.
var errAgentBusy = errors.New("too many requests for this node's agent; retry shortly")

// errAgentResponseTooLarge reports an agent response over the cap.
var errAgentResponseTooLarge = errors.New("agent response too large")

// errAgentAuth reports that an agent rejected the server's credentials.
var errAgentAuth = errors.New("agent rejected the server's credentials (check DG_AGENT_KEY on the server against DG_AGENT_PUBLIC_KEY on the agents, or DG_AGENT_TOKEN without keys)")

// RemoteContainerLocator finds the agent serving a container that runs on
// another swarm node. *collector.AgentPool implements it.
type RemoteContainerLocator interface {
	// LocateContainer returns the agent base URL (http://host:port) for
	// container id, or false when the container is local or unknown. It may
	// query the swarm.
	LocateContainer(ctx context.Context, id string) (baseURL string, ok bool)
	// CredentialsRejected reports whether agents refused the server's
	// credentials. Their nodes are then unknown, so their containers cannot
	// be located.
	CredentialsRejected() bool
}

// AgentProxy forwards container inspect and log requests for containers on
// other swarm nodes to the agent running there.
type AgentProxy struct {
	Locator RemoteContainerLocator
	// Auth adds the server's credentials to every request for an agent.
	// Required.
	Auth agentauth.Authorizer
	// Agents lists the agents on other nodes, for lookups that must ask
	// every node (node-local volumes). Optional: without it volumes are
	// only inspected on the server's node.
	Agents RemoteAgentLister
	// LocalHostname names the server's node in multi-node responses.
	LocalHostname string
	// Transport defaults to a transport with dial and response-header
	// timeouts, shared by every proxied route.
	Transport http.RoundTripper

	rtOnce sync.Once
	rt     http.RoundTripper

	// clientKey tells clients apart for their share of an agent's requests;
	// NewServer sets it. Without it every request counts as one client's.
	clientKey func(*http.Request) string

	limitersMu     sync.Mutex
	limiters       map[string]*rate.Limiter // by agent host:port
	clientLimiters map[string]*rate.Limiter // by agent host:port and client
}

// allowRequest reports whether a request of r's client may go to the agent
// at host now (see agentRequestRate and agentClientRequestRate).
func (p *AgentProxy) allowRequest(r *http.Request, host string) bool {
	client := ""
	if p.clientKey != nil {
		client = p.clientKey(r)
	}
	p.limitersMu.Lock()
	defer p.limitersMu.Unlock()
	perClient := limiterFor(&p.clientLimiters, host+" "+client, agentClientRequestRate, agentClientRequestBurst)
	return perClient.Allow() && limiterFor(&p.limiters, host, agentRequestRate, agentRequestBurst).Allow()
}

// limiterFor returns the limiter of key in *limiters, adding one allowing
// limit with burst. The set is started afresh once it holds
// maxAgentLimiters. The caller holds p.limitersMu.
func limiterFor(limiters *map[string]*rate.Limiter, key string, limit rate.Limit, burst int) *rate.Limiter {
	if l, ok := (*limiters)[key]; ok {
		return l
	}
	if *limiters == nil || len(*limiters) >= maxAgentLimiters {
		*limiters = make(map[string]*rate.Limiter)
	}
	l := rate.NewLimiter(limit, burst)
	(*limiters)[key] = l
	return l
}

// WithAgentProxy routes container inspect/logs for containers that are not
// on the local daemon to their node's agent.
func WithAgentProxy(p *AgentProxy) ServerOption {
	return func(o *serverOptions) { o.agentProxy = p }
}

// transport returns Transport or, built once so every wrapped route shares
// its connection pool, the default transport.
func (p *AgentProxy) transport() http.RoundTripper {
	p.rtOnce.Do(func() {
		p.rt = p.Transport
		if p.rt == nil {
			p.rt = &http.Transport{
				Proxy:                 nil,
				DialContext:           (&net.Dialer{Timeout: remoteDialTimeout}).DialContext,
				ResponseHeaderTimeout: remoteHeaderTimeout,
				IdleConnTimeout:       90 * time.Second,
				MaxIdleConnsPerHost:   4,
			}
		}
	})
	return p.rt
}

// wrap returns a handler that proxies {id} to the owning agent at suffix
// (e.g. "/logs") under /agent/v1/containers/{id} when the container runs on
// another node, and otherwise serves it locally. The local daemon is asked
// first: a container it has is always served locally, and only one it
// reports not found is looked up on other nodes. stream disables the
// overall request deadline for long-lived responses.
func (p *AgentProxy) wrap(local http.HandlerFunc, inspector ContainerInspector, suffix string, stream bool) http.HandlerFunc {
	if p == nil || p.Locator == nil {
		return local
	}
	rt := p.transport()
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validResourceName.MatchString(id) {
			local(w, r)
			return
		}

		probeCtx, cancel := context.WithTimeout(r.Context(), localProbeTimeout)
		_, err := inspector.ContainerInspect(probeCtx, id)
		cancel()
		// Any error other than not-found (a timeout, a struggling daemon)
		// says nothing about where the container runs, and searching then
		// could proxy to a same-named container elsewhere: the local handler
		// reports the error instead.
		if !cerrdefs.IsNotFound(err) {
			local(w, r)
			return
		}
		base, ok := p.Locator.LocateContainer(r.Context(), id)
		if !ok {
			// The container may be on a node whose agent refused the server's
			// credentials: name the misconfiguration instead of answering
			// "not found".
			if p.credentialsRejected() {
				jsonError(w, errAgentAuth.Error(), http.StatusBadGateway)
				return
			}
			local(w, r)
			return
		}
		target, err := url.Parse(base)
		if err != nil || target.Host == "" {
			log.Printf("agent proxy: bad agent URL %q for %s", base, id)
			local(w, r)
			return
		}
		if !p.allowRequest(r, target.Host) {
			w.Header().Set("Retry-After", "1")
			jsonError(w, errAgentBusy.Error(), http.StatusServiceUnavailable)
			return
		}

		if stream {
			w = newStreamWriter(w)
		} else {
			ctx, cancel := context.WithTimeout(r.Context(), remoteRequestTimeout)
			defer cancel()
			r = r.WithContext(ctx)
		}
		w = newAgentResponseWriter(w)

		rp := &httputil.ReverseProxy{
			Transport:     rt,
			FlushInterval: -1, // flush SSE events immediately
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.Out.URL.Scheme = target.Scheme
				pr.Out.URL.Host = target.Host
				pr.Out.URL.Path = collector.AgentAPIPrefix + "/containers/" + url.PathEscape(id) + suffix
				pr.Out.URL.RawPath = ""
				pr.Out.URL.RawQuery = r.URL.RawQuery
				pr.Out.Host = target.Host
				// Never forward the browser's cookies or credentials.
				pr.Out.Header = http.Header{}
				if accept := pr.In.Header.Get("Accept"); accept != "" {
					pr.Out.Header.Set("Accept", accept)
				}
				// Last: a signature covers the final host and URL. An
				// unsigned request is refused by the agent and reported as
				// errAgentAuth.
				if err := p.Auth.Authorize(pr.Out); err != nil {
					log.Printf("agent proxy %s%s: %v", id, suffix, err)
				}
			},
			// Rejected credentials are a server misconfiguration; don't
			// surface them as a 401, which the UI would treat as its own
			// session.
			ModifyResponse: func(resp *http.Response) error {
				switch {
				case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
					return errAgentAuth
				case resp.StatusCode < 400 && resp.StatusCode != http.StatusOK:
					// No agent endpoint answers anything else below 400: a
					// protocol switch would hand the client's connection to
					// the agent, and another success status skip masking.
					return errUnexpectedAgentResponse
				}
				resp.Header = agentResponseHeader(resp.StatusCode, stream)
				// Only a log stream is unbounded by design; inspect, log
				// pages and errors are read whole and bounded.
				if stream && resp.StatusCode == http.StatusOK {
					return nil
				}
				limit := maxRemoteResponseBytes
				if suffix == "" {
					limit = maxRemoteInspectBytes
				}
				body, err := readBounded(resp, limit)
				if err != nil {
					return err
				}
				if suffix == "" && resp.StatusCode == http.StatusOK {
					if body, err = remaskInspect(body); err != nil {
						return fmt.Errorf("agent inspect answer: %w", err)
					}
				}
				setBody(resp, body)
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
				if r.Context().Err() != nil && stream {
					return // client went away
				}
				log.Printf("agent proxy %s%s: %v", id, suffix, err)
				msg := "agent unreachable"
				if errors.Is(err, errAgentAuth) {
					msg = errAgentAuth.Error()
				}
				jsonError(w, msg, http.StatusBadGateway)
			},
		}
		rp.ServeHTTP(w, r)
	}
}

// errUnexpectedAgentResponse reports an agent answer no agent endpoint sends.
var errUnexpectedAgentResponse = errors.New("unexpected agent response")

// agentResponseHeader returns the only headers a proxied agent response
// keeps: a content type fixed by the route (an SSE stream, or JSON for
// one-shot responses and errors), whatever the agent sent. The browser thus
// never treats an agent's answer as a page or script of this origin.
func agentResponseHeader(status int, stream bool) http.Header {
	h := http.Header{}
	if stream && status == http.StatusOK {
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Accel-Buffering", "no")
		return h
	}
	h.Set("Content-Type", "application/json")
	return h
}

// credentialsRejected reports whether agents refused the server's
// credentials, which hides their nodes' containers and volumes from the
// server.
func (p *AgentProxy) credentialsRejected() bool {
	return p.Locator != nil && p.Locator.CredentialsRejected()
}

// readBounded reads a one-shot agent response fully, before any header
// reaches the client, so an oversized body becomes a 502 instead of a 200
// with truncated JSON. Chunked responses (unknown length) are read up to one
// byte past the cap to detect overflow.
func readBounded(resp *http.Response, limit int64) ([]byte, error) {
	if resp.ContentLength > limit {
		return nil, errAgentResponseTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errAgentResponseTooLarge
	}
	return body, nil
}

// setBody replaces resp's body with body, sent with a fixed length.
func setBody(resp *http.Response, body []byte) {
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.TransferEncoding = nil
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

// agentResponseHeaders are the headers of a proxied agent response that
// reach the client: those agentResponseHeader and setBody set, or
// jsonError for a proxy error.
var agentResponseHeaders = []string{"Content-Type", "Content-Length", "Cache-Control", "X-Accel-Buffering"}

// agentResponseWriter passes a proxied agent response on with nothing but
// its final status, its body and agentResponseHeaders. httputil's
// ReverseProxy would otherwise forward an agent's informational responses
// (103 Early Hints) with all their headers, clearing the client's header
// map after each, and the trailers an agent sends after its body. The
// writer offers no Hijack, so a connection can't be handed to an agent.
type agentResponseWriter struct {
	w      http.ResponseWriter
	header http.Header // what the proxy sets; only agentResponseHeaders reach w
	wrote  bool
}

func newAgentResponseWriter(w http.ResponseWriter) *agentResponseWriter {
	return &agentResponseWriter{w: w, header: http.Header{}}
}

func (a *agentResponseWriter) Header() http.Header { return a.header }

// WriteHeader sends the final status with the kept headers and drops
// informational ones.
func (a *agentResponseWriter) WriteHeader(code int) {
	if a.wrote || code < http.StatusOK {
		return
	}
	a.wrote = true
	for _, k := range agentResponseHeaders {
		if v := a.header.Values(k); len(v) > 0 {
			a.w.Header()[k] = v
		}
	}
	// Whatever is set from now on would be sent as trailers.
	a.header = http.Header{}
	a.w.WriteHeader(code)
}

func (a *agentResponseWriter) Write(p []byte) (int, error) {
	if !a.wrote {
		a.WriteHeader(http.StatusOK)
	}
	return a.w.Write(p)
}

// FlushError flushes a log stream's events; http.ResponseController (used
// by ReverseProxy) calls it.
func (a *agentResponseWriter) FlushError() error {
	return http.NewResponseController(a.w).Flush()
}

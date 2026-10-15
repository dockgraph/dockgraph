package api

import (
	"bytes"
	"context"
	"errors"
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
	"github.com/dockgraph/dockgraph/collector"
)

// Remote proxy timeouts. Streams (SSE logs) have no overall deadline and end
// when the client disconnects; one-shot requests are bounded.
const (
	remoteDialTimeout    = 5 * time.Second
	remoteHeaderTimeout  = 15 * time.Second
	remoteRequestTimeout = 20 * time.Second
	localProbeTimeout    = 5 * time.Second
)

// maxRemoteResponseBytes caps a proxied one-shot agent response. A variable
// so tests can lower it.
var maxRemoteResponseBytes int64 = 16 << 20

// errAgentResponseTooLarge reports an agent response over the cap.
var errAgentResponseTooLarge = errors.New("agent response too large")

// errAgentAuth reports that an agent rejected the shared token.
var errAgentAuth = errors.New("agent rejected token (check DG_AGENT_TOKEN on server and agents)")

// RemoteContainerLocator finds the agent serving a container that runs on
// another swarm node. *collector.AgentPool implements it.
type RemoteContainerLocator interface {
	// LocateCached returns the agent base URL (http://host:port) for
	// container id when already known under a key no local container can
	// share (e.g. its ID), without any Docker call.
	LocateCached(id string) (baseURL string, ok bool)
	// LocateContainer returns the agent base URL (http://host:port) for
	// container id, or false when the container is local or unknown. It may
	// query the swarm.
	LocateContainer(ctx context.Context, id string) (baseURL string, ok bool)
}

// AgentProxy forwards container inspect and log requests for containers on
// other swarm nodes to the agent running there.
type AgentProxy struct {
	Locator RemoteContainerLocator
	// Token is the bearer secret presented to agents.
	Token string
	// Transport defaults to a transport with dial and response-header
	// timeouts, shared by every proxied route.
	Transport http.RoundTripper

	rtOnce sync.Once
	rt     http.RoundTripper
}

// ServerOption customises NewServer.
type ServerOption func(*serverOptions)

type serverOptions struct {
	agentProxy *AgentProxy
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
// another node, and otherwise serves it locally. A container the locator
// already knows is proxied without a Docker call; any other is served
// locally unless the local daemon reports it not found, and only then does
// the locator search further. stream disables the overall request deadline and the server
// write deadline for long-lived responses.
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

		base, ok := p.Locator.LocateCached(id)
		if !ok {
			probeCtx, cancel := context.WithTimeout(r.Context(), localProbeTimeout)
			_, err := inspector.ContainerInspect(probeCtx, id)
			cancel()
			// Only a container the local daemon doesn't have is looked up on
			// other nodes. Any other error (a timeout, a struggling daemon)
			// says nothing about where it runs, and searching then could
			// proxy to a same-named container elsewhere: the local handler
			// reports the error instead.
			if !cerrdefs.IsNotFound(err) {
				local(w, r)
				return
			}
			base, ok = p.Locator.LocateContainer(r.Context(), id)
		}
		if !ok {
			local(w, r)
			return
		}
		target, err := url.Parse(base)
		if err != nil || target.Host == "" {
			log.Printf("agent proxy: bad agent URL %q for %s", base, id)
			local(w, r)
			return
		}

		if stream {
			disableWriteDeadline(w)
		} else {
			ctx, cancel := context.WithTimeout(r.Context(), remoteRequestTimeout)
			defer cancel()
			r = r.WithContext(ctx)
		}

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
				pr.Out.Header.Set("Authorization", "Bearer "+p.Token)
				if accept := pr.In.Header.Get("Accept"); accept != "" {
					pr.Out.Header.Set("Accept", accept)
				}
			},
			// A rejected token is a server misconfiguration; don't surface
			// it as a 401, which the UI would treat as its own session.
			ModifyResponse: func(resp *http.Response) error {
				if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					return errAgentAuth
				}
				// One-shot responses (inspect, log pages) are bounded; only
				// the SSE stream is unbounded by design.
				if !stream {
					return bufferBounded(resp)
				}
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
				if r.Context().Err() != nil && stream {
					return // client went away
				}
				log.Printf("agent proxy %s%s: %v", id, suffix, err)
				jsonError(w, "agent unreachable", http.StatusBadGateway)
			},
		}
		rp.ServeHTTP(w, r)
	}
}

// bufferBounded reads a one-shot agent response fully, before any header
// reaches the client, so an oversized body becomes a 502 instead of a 200
// with truncated JSON. Chunked responses (unknown length) are read up to one
// byte past the cap to detect overflow.
func bufferBounded(resp *http.Response) error {
	if resp.ContentLength > maxRemoteResponseBytes {
		return errAgentResponseTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteResponseBytes+1))
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	if int64(len(body)) > maxRemoteResponseBytes {
		return errAgentResponseTooLarge
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.TransferEncoding = nil
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

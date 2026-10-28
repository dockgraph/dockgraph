package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/dockgraph/dockgraph/collector"
)

// ownerParam names the query parameter carrying the node that holds a
// node-local network, as the graph's Node.Owner reports it.
const ownerParam = "node"

// inspectNetwork returns the network inspect handler. A node-local network
// exists on one node's daemon only, and with the server on every manager the
// request may reach a replica other than the one whose graph showed it,
// which would answer 404 or describe its own same-named network. The graph
// names the owner (?node=); a request for another node's network is
// forwarded to the agent there, and never answered from this daemon. Without
// ?node= (swarm-scoped networks) the replica answers itself.
func (p *AgentProxy) inspectNetwork(local http.HandlerFunc) http.HandlerFunc {
	if p == nil {
		return local
	}
	rt := p.transport()
	return func(w http.ResponseWriter, r *http.Request) {
		owner := r.URL.Query().Get(ownerParam)
		if owner == "" || owner == p.LocalHostname {
			local(w, r)
			return
		}
		name := r.PathValue("name")
		if !validResourceName.MatchString(name) || !validHostname.MatchString(owner) {
			jsonError(w, "invalid network or node name", http.StatusBadRequest)
			return
		}
		base := p.agentFor(owner)
		if base == "" {
			// The agent may be unknown because it refused the server's
			// credentials: name the misconfiguration.
			if p.credentialsRejected() {
				jsonError(w, errAgentAuth.Error(), http.StatusBadGateway)
				return
			}
			jsonError(w, "network is on node "+owner+", whose agent is not reachable", http.StatusBadGateway)
			return
		}
		target, err := url.Parse(base)
		if err != nil || target.Host == "" {
			log.Printf("agent proxy: bad agent URL %q for node %s", base, owner)
			jsonError(w, "agent unreachable", http.StatusBadGateway)
			return
		}
		if !p.allowRequest(r, target.Host) {
			w.Header().Set("Retry-After", "1")
			jsonError(w, errAgentBusy.Error(), http.StatusServiceUnavailable)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), remoteRequestTimeout)
		defer cancel()
		r = r.WithContext(ctx)
		w = newAgentResponseWriter(w)

		rp := &httputil.ReverseProxy{
			Transport: rt,
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.Out.URL.Scheme = target.Scheme
				pr.Out.URL.Host = target.Host
				pr.Out.URL.Path = collector.AgentAPIPrefix + "/networks/" + url.PathEscape(name)
				pr.Out.URL.RawPath = ""
				pr.Out.URL.RawQuery = ""
				pr.Out.Host = target.Host
				// Never forward the browser's cookies or credentials.
				pr.Out.Header = http.Header{}
				pr.Out.Header.Set("Accept", "application/json")
				// Last: a signature covers the final host and URL.
				if err := p.Auth.Authorize(pr.Out); err != nil {
					log.Printf("agent proxy network %s on %s: %v", name, owner, err)
				}
			},
			ModifyResponse: func(resp *http.Response) error {
				switch {
				case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
					return errAgentAuth
				case resp.StatusCode < 400 && resp.StatusCode != http.StatusOK:
					return errUnexpectedAgentResponse
				}
				resp.Header = agentResponseHeader(resp.StatusCode, false)
				body, err := readBounded(resp, maxRemoteInspectBytes)
				if err != nil {
					return err
				}
				if resp.StatusCode == http.StatusOK {
					if body, err = remaskNetwork(body); err != nil {
						return fmt.Errorf("agent network answer: %w", err)
					}
				}
				setBody(resp, body)
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
				log.Printf("agent proxy network %s on %s: %v", name, owner, err)
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

// agentFor returns the base URL of the agent on the node with the hostname,
// or "" when none is known.
func (p *AgentProxy) agentFor(hostname string) string {
	if p.Agents == nil {
		return ""
	}
	for _, a := range p.Agents.RemoteAgents() {
		if a.Hostname == hostname {
			return a.BaseURL
		}
	}
	return ""
}

package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/dockgraph/dockgraph/agentauth"
)

// AgentAPIPrefix is the path prefix of every per-node agent endpoint.
const AgentAPIPrefix = "/agent/v1"

// AgentInfo is the payload of GET /agent/v1/info: the swarm node an agent
// runs on.
type AgentInfo struct {
	NodeID   string `json:"nodeId"`
	Hostname string `json:"hostname"`
}

// AgentStats is the payload of GET /agent/v1/stats: the latest samples of
// the containers running on the agent's node.
type AgentStats struct {
	NodeID  string            `json:"nodeId"`
	Samples []ContainerSample `json:"samples"`
}

// Agent pool defaults.
const (
	defaultAgentResolveInterval = 30 * time.Second
	defaultAgentChangeInterval  = 5 * time.Second
	// agentRetryInitial is the first retry delay while no agent is known (e.g.
	// the server started before the agent tasks registered in DNS). It
	// doubles up to ResolveInterval.
	agentRetryInitial          = 2 * time.Second
	defaultAgentRequestTimeout = 5 * time.Second
	agentResolveTimeout        = 5 * time.Second
	agentTaskLookupTimeout     = 5 * time.Second
	// maxAgentInfoBytes bounds an agent's /info answer.
	maxAgentInfoBytes = 64 << 10
	// maxAgentSamples bounds the containers one agent may report per poll:
	// every sample is kept in the stats history, so a report must cost no
	// more than a busy node's.
	maxAgentSamples = 2000
)

// maxAgentStatsBytes bounds an agent's /stats answer; maxAgentSamples full
// samples fit in it. A variable so tests can lower it.
var maxAgentStatsBytes int64 = 4 << 20

// errAgentUnauthorized reports an agent that rejected the server's
// credentials.
var errAgentUnauthorized = errors.New("agent rejected the server's credentials")

// HostResolver resolves a DNS name to addresses; *net.Resolver satisfies it.
type HostResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// TaskLister lists swarm tasks; used to find which node runs a container.
type TaskLister interface {
	TaskList(ctx context.Context, options swarm.TaskListOptions) ([]swarm.Task, error)
}

// AgentPoolConfig configures an AgentPool.
type AgentPoolConfig struct {
	// Addr is the DNS name resolving to every agent task (e.g. tasks.agent),
	// optionally with ":port" overriding Port.
	Addr string
	// Port is the agent HTTP port used when Addr carries none.
	Port string
	// Auth adds the server's credentials (a signature, the token or both)
	// to every request for an agent. Required.
	Auth agentauth.Authorizer
	// LocalNodeID is this server's swarm node. Its agent is not polled for
	// stats (the local stats worker already covers it) and its containers
	// are never proxied.
	LocalNodeID string
	// Resolver defaults to net.DefaultResolver. A resolved entry that already
	// has a port (host:port) is used as is.
	Resolver HostResolver
	// Tasks is used to locate stopped or not-yet-sampled task containers.
	// Optional.
	Tasks TaskLister
	// Client defaults to an http.Client with RequestTimeout.
	Client *http.Client
	// ResolveInterval defaults to 30s.
	ResolveInterval time.Duration
	// ChangeInterval is how often, between full refreshes, the agent DNS
	// name is re-resolved to pick up agents that came or went (e.g. a node
	// joined). Defaults to 5s.
	ChangeInterval time.Duration
	// RequestTimeout bounds each /info and /stats call (default 5s).
	RequestTimeout time.Duration
}

// agentEndpoint is one discovered agent.
type agentEndpoint struct {
	addr     string // host:port
	hostname string
}

// AgentPool discovers per-node agents through DNS, maps each to its swarm
// node, polls their stats and locates containers running on other nodes.
type AgentPool struct {
	cfg AgentPoolConfig

	mu     sync.RWMutex
	agents map[string]agentEndpoint // nodeID → agent
	// resolved holds the addresses the agent DNS name resolved to at the
	// last refresh, to detect agents that came or went in between.
	resolved []string
	// verified maps agent addresses to the node the swarm confirmed each
	// runs on, reused when the swarm can't be asked.
	verified map[string]string
	// What the last remote stats poll reported. "" marks a key agents
	// reported differently: an agent vouches only for its own node, and
	// names are unique per node only. Task containers map to their task,
	// not a node: the swarm, not the agent, says where a task runs.
	owners map[string]string // standalone container ID → node
	names  map[string]string // standalone container name → node
	tasks  map[string]string // task container ID or name, or task ID → task ID
	// placements caches where the swarm runs each sampled task: a task never
	// moves to another node. Pruned to the sampled tasks every poll.
	placements map[string]string // task ID → node
	// reporting lists the hostnames of agents that answered the last remote
	// stats poll, even with no samples (e.g. only the agent runs there).
	reporting []string

	lastCount int           // agents found by the previous refresh; -1 before the first
	rejected  int           // agents that rejected the credentials at the last refresh
	retryMin  time.Duration // first retry delay while no agent is known
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewAgentPool creates a pool; call Start to begin discovery.
func NewAgentPool(cfg AgentPoolConfig) *AgentPool {
	if cfg.Resolver == nil {
		cfg.Resolver = net.DefaultResolver
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = defaultAgentRequestTimeout
	}
	if cfg.Client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		cfg.Client = &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: transport,
			// No agent endpoint redirects; following one would let an agent
			// point the server at any address.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	if cfg.ResolveInterval <= 0 {
		cfg.ResolveInterval = defaultAgentResolveInterval
	}
	if cfg.ChangeInterval <= 0 {
		cfg.ChangeInterval = defaultAgentChangeInterval
	}
	return &AgentPool{
		cfg:        cfg,
		agents:     make(map[string]agentEndpoint),
		verified:   make(map[string]string),
		owners:     make(map[string]string),
		names:      make(map[string]string),
		tasks:      make(map[string]string),
		placements: make(map[string]string),
		lastCount:  -1,
		retryMin:   agentRetryInitial,
	}
}

// Start resolves agents immediately, then every ResolveInterval until ctx is
// cancelled or Stop is called. While no agent is known it retries sooner,
// from retryMin doubling up to ResolveInterval: the server often starts
// before the global agent tasks are registered in DNS. In between, the DNS
// name is checked every ChangeInterval, and an agent that came or went
// (e.g. a node joined) triggers a refresh at once.
func (p *AgentPool) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	p.Refresh(ctx)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		retry := p.retryMin
		for {
			delay := p.cfg.ResolveInterval
			if p.agentCount() > 0 {
				retry = p.retryMin
			} else if retry < delay {
				delay = retry
				retry *= 2
			}
			if !p.waitForRefresh(ctx, delay) {
				return
			}
			p.Refresh(ctx)
		}
	}()
}

// waitForRefresh waits until delay has passed or the agents' DNS answer
// changes, checking it every ChangeInterval. It returns false when ctx is
// cancelled.
func (p *AgentPool) waitForRefresh(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	check := time.NewTicker(p.cfg.ChangeInterval)
	defer check.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return true
		case <-check.C:
			if p.agentsChanged(ctx) {
				return true
			}
		}
	}
}

// agentCount returns how many agents the last refresh found.
func (p *AgentPool) agentCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.agents)
}

// Stop ends discovery and waits for the background loop to exit.
func (p *AgentPool) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

// Refresh re-resolves the agent DNS name and queries every address's /info
// to map nodes to agents. Unreachable agents are dropped until they answer.
func (p *AgentPool) Refresh(ctx context.Context) {
	host, addrs, err := p.resolve(ctx)
	if err != nil && ctx.Err() != nil {
		return
	}
	// A transient resolver failure (timeout, SERVFAIL) must not drop every
	// known agent until the next refresh; only an authoritative "no such
	// host" (no agent task running) empties the pool.
	if err != nil && !isDNSNotFound(err) {
		log.Printf("agents: resolving %s: %v; keeping %d known agent(s)", host, err, len(p.remoteEndpoints()))
		return
	}

	infos := make([]*AgentInfo, len(addrs))
	unauthorized := make([]bool, len(addrs))
	var wg sync.WaitGroup
	for i, addr := range addrs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var info AgentInfo
			if getErr := p.getJSON(ctx, addr, "/info", maxAgentInfoBytes, &info); getErr != nil {
				// Rejections are summarised once below rather than logged
				// per agent on every refresh.
				if errors.Is(getErr, errAgentUnauthorized) {
					unauthorized[i] = true
				} else {
					log.Printf("agents: %s /info: %v", addr, getErr)
				}
				return
			}
			if info.NodeID == "" {
				log.Printf("agents: %s reported no swarm node ID", addr)
				return
			}
			infos[i] = &info
		}()
	}
	wg.Wait()

	verified := p.verifyNodes(ctx, addrs, infos)
	agents := make(map[string]agentEndpoint, len(addrs))
	for i, info := range infos {
		if info == nil || verified[addrs[i]] != info.NodeID {
			continue
		}
		if _, dup := agents[info.NodeID]; dup {
			continue
		}
		agents[info.NodeID] = agentEndpoint{addr: addrs[i], hostname: info.Hostname}
	}

	p.mu.Lock()
	p.agents = agents
	p.resolved = addrs
	p.verified = verified
	changed := len(agents) != p.lastCount
	p.lastCount = len(agents)
	p.mu.Unlock()

	if changed {
		if len(agents) == 0 {
			reason := ""
			switch {
			case err != nil && isDNSNotFound(err):
				// Normal right after deploy: agent tasks not registered yet.
				reason = " (no agent task registered yet)"
			case err != nil:
				reason = fmt.Sprintf(" (%v)", err)
			}
			log.Printf("agents: 0 via %s%s; stats and logs limited to this node until agents are found", host, reason)
		} else {
			log.Printf("agents: %d via %s", len(agents), host)
		}
	}

	rejected := 0
	for _, u := range unauthorized {
		if u {
			rejected++
		}
	}
	p.recordRejections(host, rejected, len(agents))
}

// verifyNodes returns, for each agent address whose /info answered, the
// node it runs on once the swarm confirms it: a running task on the node the
// agent names must hold the agent's address. The node decides which
// containers the agent is asked about, so an agent's word alone is not
// taken. When the task list can't be read, earlier confirmations stand.
// Without a task lister (Tasks unset) claims are taken as they are.
func (p *AgentPool) verifyNodes(ctx context.Context, addrs []string, infos []*AgentInfo) map[string]string {
	verified := make(map[string]string, len(addrs))
	if p.cfg.Tasks == nil {
		for i, info := range infos {
			if info != nil {
				verified[addrs[i]] = info.NodeID
			}
		}
		return verified
	}

	nodesByIP, err := p.taskAddresses(ctx)
	p.mu.RLock()
	previous := p.verified
	p.mu.RUnlock()
	for i, info := range infos {
		if info == nil {
			continue
		}
		addr := addrs[i]
		if err != nil {
			if previous[addr] == info.NodeID {
				verified[addr] = info.NodeID
			}
			continue
		}
		if ip := hostIP(addr); ip.IsValid() && nodesByIP[ip][info.NodeID] {
			verified[addr] = info.NodeID
		} else {
			warnOnce("agent-node:"+addr+":"+info.NodeID,
				"agents: %s claims node %s, but no running task on that node has its address; ignoring it", addr, info.NodeID)
		}
	}
	if err != nil {
		log.Printf("agents: verifying agent nodes: %v; keeping %d earlier confirmation(s)", err, len(verified))
	}
	return verified
}

// taskAddresses maps every address of a running task's network attachments
// to the nodes running such a task.
func (p *AgentPool) taskAddresses(ctx context.Context) (map[netip.Addr]map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, agentTaskLookupTimeout)
	defer cancel()
	tasks, err := p.cfg.Tasks.TaskList(ctx, swarm.TaskListOptions{
		Filters: filters.NewArgs(filters.Arg("desired-state", string(swarm.TaskStateRunning))),
	})
	if err != nil {
		return nil, err
	}
	nodes := make(map[netip.Addr]map[string]bool)
	for _, t := range tasks {
		for _, a := range t.NetworksAttachments {
			for _, cidr := range a.Addresses {
				prefix, err := netip.ParsePrefix(cidr)
				if err != nil {
					continue
				}
				ip := prefix.Addr().Unmap()
				if nodes[ip] == nil {
					nodes[ip] = make(map[string]bool)
				}
				nodes[ip][t.NodeID] = true
			}
		}
	}
	return nodes, nil
}

// hostIP returns the IP of a host:port agent address.
func hostIP(addr string) netip.Addr {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}

// recordRejections stores how many agents rejected the server's credentials
// and logs when that number changes: a key or token mismatch hides every
// container on those nodes, so it must be named plainly, but only once.
// Recovery is only announced when agents actually accepted the credentials,
// not when the rejecting ones merely went away.
func (p *AgentPool) recordRejections(host string, rejected, accepted int) {
	p.mu.Lock()
	prev := p.rejected
	p.rejected = rejected
	p.mu.Unlock()

	switch {
	case rejected == prev:
	case rejected > 0:
		log.Printf("agents: %d agent(s) via %s rejected the server's credentials; check that the server's DG_AGENT_KEY matches the agents' DG_AGENT_PUBLIC_KEY (or, without keys, that DG_AGENT_TOKEN matches), and the node clocks", rejected, host)
	case accepted > 0:
		log.Printf("agents: all agents via %s accept the server's credentials again", host)
	}
}

// CredentialsRejected reports whether an agent rejected the server's
// credentials at the last refresh. Containers on its node can then not be
// reached, which callers should report as a misconfiguration rather than as
// missing containers.
func (p *AgentPool) CredentialsRejected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.rejected > 0
}

// resolve looks the agent DNS name up and returns the host it resolved and
// the agent addresses (host:port), sorted. A resolved entry that already
// carries a port is kept as is.
func (p *AgentPool) resolve(ctx context.Context) (string, []string, error) {
	host, port := p.cfg.Addr, p.cfg.Port
	if h, pt, err := net.SplitHostPort(p.cfg.Addr); err == nil {
		host, port = h, pt
	}

	rctx, cancel := context.WithTimeout(ctx, agentResolveTimeout)
	ips, err := p.cfg.Resolver.LookupHost(rctx, host)
	cancel()

	addrs := make([]string, 0, len(ips))
	for _, ip := range ips {
		if _, _, splitErr := net.SplitHostPort(ip); splitErr == nil {
			addrs = append(addrs, ip)
		} else {
			addrs = append(addrs, net.JoinHostPort(ip, port))
		}
	}
	sort.Strings(addrs)
	return host, addrs, err
}

// agentsChanged reports whether the agent DNS name now resolves to other
// addresses than at the last refresh: an agent task started or stopped. A
// failed lookup reports no change; the next full refresh deals with it.
func (p *AgentPool) agentsChanged(ctx context.Context) bool {
	_, addrs, err := p.resolve(ctx)
	if err != nil && !isDNSNotFound(err) {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !slices.Equal(addrs, p.resolved)
}

// isDNSNotFound reports whether err is an authoritative "host not found".
// Errors that are not *net.DNSError (e.g. from a stub resolver) count as
// not found so the pool never keeps agents it cannot vouch for.
func isDNSNotFound(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsNotFound
	}
	return true
}

// remoteEndpoints returns the discovered agents except the local node's.
func (p *AgentPool) remoteEndpoints() map[string]agentEndpoint {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]agentEndpoint, len(p.agents))
	for nodeID, a := range p.agents {
		if nodeID != p.cfg.LocalNodeID {
			out[nodeID] = a
		}
	}
	return out
}

// RemoteAgent is a discovered agent on another swarm node.
type RemoteAgent struct {
	Hostname string
	BaseURL  string // http://host:port
}

// RemoteAgents returns the agents on other nodes, sorted by hostname, for
// lookups that must ask every node (e.g. node-local volumes).
func (p *AgentPool) RemoteAgents() []RemoteAgent {
	endpoints := p.remoteEndpoints()
	out := make([]RemoteAgent, 0, len(endpoints))
	for _, a := range endpoints {
		out = append(out, RemoteAgent{Hostname: a.hostname, BaseURL: "http://" + a.addr})
	}
	// Hostnames can repeat (see buildSwarmNodeNodes); the URL keeps the
	// order deterministic.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hostname != out[j].Hostname {
			return out[i].Hostname < out[j].Hostname
		}
		return out[i].BaseURL < out[j].BaseURL
	})
	return out
}

// errAgentAnswerTooLarge reports an agent answer over its size bound.
var errAgentAnswerTooLarge = errors.New("answer too large")

// getJSON performs an authenticated GET on an agent endpoint and decodes
// its JSON answer, at most limit bytes, into out.
func (p *AgentPool) getJSON(ctx context.Context, addr, path string, limit int64, out any) error {
	body, err := p.get(ctx, addr, path, limit)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// get performs an authenticated GET on an agent endpoint and returns its
// answer, refused when over limit bytes.
func (p *AgentPool) get(ctx context.Context, addr, path string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+AgentAPIPrefix+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if err := p.cfg.Auth.Authorize(req); err != nil {
		return nil, err
	}
	resp, err := p.cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, errAgentUnauthorized
		}
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return nil, errAgentAnswerTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errAgentAnswerTooLarge
	}
	return body, nil
}

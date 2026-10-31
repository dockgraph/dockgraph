# DockGraph — Backend

Go server that monitors Docker infrastructure in real time and serves a graph topology over WebSocket. Watches the Docker daemon for container, network, and volume changes while simultaneously parsing Docker Compose files to include services that haven't started yet.

On a Docker Swarm manager it models swarm services, tasks and nodes instead of individual containers, and the same binary runs as a per-node agent (`DG_MODE=agent`) that serves each node's container stats, inspect and logs to the server.

## Requirements

- Go 1.26+
- Docker daemon (accessible via socket or TCP)

## Quick Start

```bash
go build -o dockgraph .
./dockgraph
```

The server starts on `:7800` by default and serves the embedded frontend SPA.

## Configuration

All configuration is via environment variables, listed in the [root README](../README.md#environment-variables). Compose paths are auto-detected from the container's own bind mounts (excluding the Docker socket); set `DG_COMPOSE_PATH` only to override them.

See the [root README](../README.md#docker-swarm) for deploying the server and agents with `stack.yml`.

## Project Structure

```
backend/
├── main.go              # Bootstrap: wires collectors, state manager, HTTP server (or the agent)
├── config.go            # Environment-based configuration, *_FILE secret loading
├── mode.go              # Resolves DG_MODE against the daemon's swarm state
├── agentkeys.go         # --agent-keygen / --agent-pubkey, and the agent credentials each side uses
├── agent/
│   └── agent.go             # Per-node agent (DG_MODE=agent): stats, inspect, logs for signed requests
├── agentauth/
│   ├── agentauth.go         # Agent key pairs and request signing (Ed25519), the older bearer token
│   └── verify.go            # Agent-side check: signature, clock skew, addressed host, nonce replay
├── api/
│   ├── server.go            # HTTP routes: /healthz, /ws, /login, resource APIs, SPA fallback
│   ├── ws.go                # WebSocket hub with ping/pong heartbeat
│   ├── containers.go        # Container list and stats endpoints
│   ├── containers_detail.go # Container inspect endpoint
│   ├── networks.go          # Network inspect endpoint
│   ├── volumes.go           # Volume inspect endpoint
│   ├── logs.go              # Container log history and SSE streaming endpoints
│   ├── log_reader.go        # Reads Docker log streams line by line, multiplexed or raw (TTY)
│   ├── logs_aggregate.go    # Aggregated log history and SSE streaming (stack-scoped, swarm services)
│   ├── limits.go            # Per-client limits on concurrent streams and heavy requests
│   ├── sse.go               # Server-Sent Events writer with a deadline per write
│   ├── hosts.go             # Host header allowlist (DG_ALLOWED_HOSTS)
│   ├── services.go          # Swarm service inspect and service log endpoints
│   ├── swarm_managers.go    # Swarm manager list endpoint
│   ├── remote.go            # Proxies container inspect/logs to the agent on the owning node
│   ├── remote_mask.go       # Masks an agent's inspect answer again
│   ├── remote_volumes.go    # Looks node-local swarm volumes up on every node
│   ├── events.go            # Recent Docker events endpoint
│   ├── stats_history.go     # Stats time-series endpoint (stack scope, per-node scope)
│   ├── system.go            # System info, disk usage, and image list endpoints
│   ├── system_cache.go      # Caching wrapper for system API calls
│   ├── const.go             # Shared JSON field names and constants
│   └── validation.go        # Request validation helpers
├── auth/
│   ├── handlers.go          # Login/logout HTTP handlers
│   ├── clientip.go          # Client address and scheme behind trusted proxies (DG_TRUSTED_PROXIES)
│   ├── middleware.go         # JWT authentication middleware
│   ├── jwt.go               # JWT token creation and validation
│   ├── password.go          # Argon2id password hashing
│   ├── ratelimit.go         # Login rate limiting
│   └── session.go           # Session management
├── collector/
│   ├── types.go             # Core types: Node, Edge, GraphSnapshot, Collector interface
│   ├── docker.go            # Docker collector: poll loop + event stream watcher
│   ├── docker_snapshot.go   # Fetches Docker resources, assembles graph snapshot
│   ├── swarm_snapshot.go    # Swarm snapshot: service, task and swarm node graph nodes
│   ├── docker_helpers.go    # Predicates: self-exclusion, topology events, status
│   ├── node_builder.go      # Shared node constructors and network classification
│   ├── const.go             # Shared state, node type and mode constants
│   ├── compose.go           # Compose collector: file discovery + filesystem watcher
│   ├── compose_parser.go    # Parse entry points; builds graph nodes and edges from a loaded project
│   ├── compose_limits.go    # Bounds of one parse, and bounded reads of compose files
│   ├── compose_header.go    # Reads a file's name, includes and extends before it is loaded
│   ├── compose_include.go   # Expands include itself, without reading env files
│   ├── compose_extends.go   # Checks extends files and counts what they copy
│   ├── compose_sandbox.go   # Runs each parse in a child process; its limits in compose_sandbox_linux.go
│   ├── race_on.go           # raceBuild, with race_off.go: whether the race detector is built in
│   ├── mounts.go            # Auto-detects compose paths from container bind mounts
│   ├── ports.go             # Merges published ports into ranges (PortRanges)
│   ├── textcap.go           # Caps names from untrusted sources (CapText)
│   ├── warn_once.go         # Logs a warning once per key
│   ├── stats_collector.go   # Container stats collection orchestrator
│   ├── stats_worker.go      # Stats polling worker; merges remote samples, service and node aggregates
│   ├── agent_pool.go        # Discovers agents via DNS and confirms their nodes with the swarm
│   ├── agent_stats.go       # Polls agents' stats each round and decodes their answers
│   ├── agent_locate.go      # Finds the agent of the node running a container
│   ├── stats_calc.go        # CPU/memory/network stats calculation
│   ├── stats_types.go       # Stats data types
│   ├── stats_history.go     # Ring buffer for stats time-series (dashboard charts)
│   ├── event_history.go     # Ring buffer for recent Docker events
│   └── debounce.go          # Timer-based debounce helper
├── state/
│   ├── manager.go           # Merges Docker + Compose snapshots, notifies subscribers
│   └── diff.go              # Snapshot diffing for incremental delta updates
├── secrets/
│   ├── secrets.go           # Masks credentials in env, labels, options and command lines (shared by api + collector)
│   ├── keys.go              # Which keys name credentials
│   ├── masker.go            # Masks values and command lines word by word
│   └── formats.go           # URL, DSN, JSON and XML credential patterns
└── frontend/
    ├── embed.go             # Embeds built frontend assets into the binary
    ├── login.html           # Login page, served without a session
    └── login.js             # Login page script (kept out of the page for the CSP)
```

## Architecture

### Data Collection

Two independent collectors run concurrently:

**DockerCollector** connects to the Docker daemon via the API client. It performs an initial poll on startup and then watches the event stream for topology-relevant changes (container create/destroy, network connect/disconnect). Events are debounced at 500ms to avoid redundant polls during burst operations like `docker compose up`.

**ComposeCollector** auto-detects compose files from the container's own bind mounts, parses them into graph nodes, and watches for filesystem changes via `fsnotify`. This surfaces services that are defined but not yet running, giving the UI a complete view of the intended topology. As with `docker compose -f`, a compose file's relative paths (extends files, bind mounts) are relative to its own directory, and an included file's to its own.

Parsing is bounded, since whoever can write a watched directory controls its files: compose, included and extended files must be regular files with at most 64 YAML documents, and a parse reads at most 1 MiB of them in total (a project of 400 services with a few dozen settings each takes about 750 KiB); includes nest at most 10 deep, with at most 256 included-file references; at most 64 distinct extends files are read, and extends may copy at most 100,000 YAML nodes.

Published port ranges are listed as one mapping (`collector.PortRanges`), as `docker ps` lists them, for compose services, running containers and swarm services, and in the inspect answers; Docker, swarm and the compose loader list a range port by port. A mapping of a range has `hostEnd` and `containerEnd`; a host range for one container port (`"8000-8010:80"`, Docker picking one of the host ports) has `hostEnd` alone. The compose loader still turns each published port into structures of its own, so a compose file whose services publish more than about 150,000 ports in all does not fit in a parse's memory.

Each file is parsed in a child process, DockGraph's own binary started with `__parse-compose` (`collector/compose_sandbox.go`): it reads the request as JSON on stdin, writes the graph as JSON on stdout (at most 8 MiB) and is killed after 30 seconds. On Linux the child limits itself before parsing (`collector/compose_sandbox_linux.go`): 256 MiB of mapped memory, 60 seconds of CPU, no file growing past 0 bytes, and an OOM score of 1000, so the kernel ends it first should the container run out of memory; it is also killed should DockGraph end first. With Landlock (Linux 5.13 and later, in binaries built without cgo, as release images are, and where the container runtime's seccomp profile allows it), it can read only files under the compose paths, a path naming a file standing for its directory, and is denied what else the kernel's Landlock ABI covers: any write, execution, TCP (ABI 4), abstract UNIX sockets outside the process and, on kernels with Landlock erratum 2 fixed, signals to other processes (ABI 6), and UNIX socket files such as the Docker socket (ABI 9). A compose, include or extends file outside the compose paths, or reached through a symlink leading out of them, fails to load with `permission denied`. The child's environment is `GOMAXPROCS=2` and DockGraph's `TZ` and `HOME`, nothing else. What the loader logs is passed on to DockGraph's log, and how parses are restricted is logged with the first parse that returns. A file whose parse process fails (it is killed, crashes or cannot start) keeps the services it last had, since that may have nothing to do with the file; a file the loader refuses loses them. The collector starts without waiting for its first scan, and a scan that `Stop` cuts short sends no graph. Tests parse in-process; `CGO_ENABLED=0 go test ./collector -args -compose-child` runs them through the child.

Both collectors implement the `Collector` interface and emit `StateUpdate` values on a channel.

### Swarm Mode

`DG_MODE` is resolved at startup against the daemon's swarm state (`mode.go`). In `swarm` mode the DockerCollector builds its snapshot from the swarm API (`swarm_snapshot.go`):

- **Services** become `service` nodes placed in their overlay networks, with replica counts, mode (replicated/global/job), update status and their tasks. Their status is `running`, `degraded`, `updating`, `stopped` (scaled to zero) or, for a job whose tasks all finished, `completed`; jobs also report their completed task count, since swarm leaves a finished job at 0/1 or 0/0 replicas. Each task carries the hostname of the node it runs on. The stack comes from the `com.docker.stack.namespace` label.
- **Nodes** become `swarmnode` nodes with role, leader flag, availability, state, address, engine version and capacity (CPU and memory). A hostname appearing twice (a node that left and rejoined) keeps only the ready or most recent entry.
- Service and node events trigger a re-snapshot, and tasks are polled every `DG_SWARM_POLL_INTERVAL` since task changes emit no events.
- Stack files mounted with `DG_COMPOSE_PATH` show services that are not deployed yet, as with compose.

**Agents.** With `DG_MODE=agent`, the binary serves only the agent API under `/agent/v1` (info, stats, container inspect and logs, volume inspect) plus `/healthz`. Every request must be signed with the server's private key (`agentauth`): the `Dockgraph-Signature` header covers the method, host, path and query, a timestamp and a nonce, and the agent accepts it only within two minutes of its own clock, addressed to one of its own IP addresses, and once. Agents hold only the public key, so none of them can sign. Agents without a public key fall back to the deprecated shared token; a server with both a key and a token sends both, for stacks moving to keys. The server's `AgentPool` resolves `DG_AGENT_ADDR` (`tasks.agent`) every 30 seconds, keeping known agents on transient DNS errors, and polls every agent each stats round. In between it re-resolves the name every 5 seconds and refreshes at once when an agent appears or disappears (a node joined or left), so a new node's containers don't wait for the next full refresh. An agent that rejects the server's credentials is reported once, not on every refresh. Remote samples are merged with local ones (local wins on a name collision), stamped with their node, and aggregated per service and per node under `node:{hostname}`. Every node whose agent answered gets an aggregate, even when it runs no other container.

An agent's `/info` names its node, and the pool only uses the agent for that node once the swarm runs a task holding the agent's address there; earlier confirmations are kept while the task list can't be read.

**Proxy.** Container inspect and log requests for a container on another node are forwarded to that node's agent (`api/remote.go`). The local daemon is always asked first; a task container is then located through the swarm task list (stopped ones included, which no agent reports), and any other container through the agents' last stats reports. A name, ID or ID prefix whose matches point at several nodes, or at a task on one node and a standalone report on another, is not located, nor is a reported container while the task list can't be read. Proxied responses keep only their final status and a content type fixed by the route (JSON, or `text/event-stream` for a log stream): informational responses, protocol switches and trailers from an agent never reach the client. The server masks the credential fields of an agent's inspect answer again (`api/remote_mask.go`). Inspect answers are capped at 1 MiB and other non-streaming responses at 4 MiB; log streams stay open until the client disconnects or stops reading. Requests made for clients are limited to 100 per second per agent, below what an agent's replay cache holds, and to 25 per second per agent for any one client (`503` beyond). Stats answers are capped at 4 MiB and 2,000 containers per agent, and a container or task reported by several nodes gets no stats. Swarm volumes are node-local, so a volume inspect asks the local daemon and every agent at once, bounded by 5 seconds (`api/remote_volumes.go`): the response describes the server's own copy when it has one, otherwise the first node's by hostname, and adds `node` (the node described) and `nodes` (every node holding a volume of that name). An agent's volume answer is capped at 1 MiB and masked again.

### State Merging

The `state.Manager` receives snapshots from both collectors and merges them into a unified graph. Docker data takes precedence for nodes that exist in both sources (it has actual runtime state), while compose-only metadata like `Source` and `NetworkID` is preserved when Docker doesn't provide it. The merged graph is broadcast to all WebSocket subscribers.

### WebSocket Protocol

Clients connect to `/ws` and receive a `WireMessage` envelope:

```json
{
  "type": "snapshot",
  "version": 3,
  "data": {
    "nodes": [
      { "id": "container:web-1", "type": "container", "name": "web-1", "status": "running", ... }
    ],
    "edges": [
      { "id": "e:dep:web-1:db-1", "type": "depends_on", "source": "container:web-1", "target": "container:db-1" }
    ]
  }
}
```

The initial message is always a full `snapshot`. Subsequent updates may be `snapshot` or `delta` (incremental adds/removes/updates).

Every message carries the protocol version (`collector.ProtocolVersion`). Version 2 added the `service` and `swarmnode` node types; version 3 lists a published port range as one mapping (`hostEnd`, `containerEnd`). The frontend accepts only its own version. On a mismatch it reloads once to fetch a matching bundle, and ignores the messages if the mismatch persists.

### Graph Model

Nodes use namespaced IDs to prevent collisions:

| Type | ID Format | Example |
|------|-----------|---------|
| Container | `container:{name}` | `container:web-1` |
| Network | `network:{name}` | `network:myapp_default` |
| Volume | `volume:{name}` | `volume:myapp_data` |
| Swarm service | `service:{name}` | `service:shop_web` |
| Swarm node | `swarmnode:{hostname}` | `swarmnode:manager-1` |

Edge types:

| Type | Meaning |
|------|---------|
| `depends_on` | Compose service dependency |
| `volume_mount` | Container mounts a named volume |
| `secondary_network` | Container connected to a non-primary network |

### Authentication

When `DG_PASSWORD` or `DG_PASSWORD_FILE` is set, all endpoints (except `/healthz`) require a valid JWT session token. The `auth` package handles:

- **Password hashing** with Argon2id (constant-time comparison). One verification runs at a time; a login that waits more than 5 seconds for its turn gets `503`. Pre-hashed passwords with parameters argon2 can't run safely (t or p of 0, t above 10, memory above 128 MiB, short salt or key, or a malformed parameter field) are refused at startup.
- **JWT tokens** with HMAC-SHA256, signed with an ephemeral key generated at startup
- **Rate limiting** on the login endpoint: 5 attempts per minute per client, IPv6 clients bucketed by /64, at most 10,000 clients tracked. Behind a proxy listed in `DG_TRUSTED_PROXIES`, the client is the rightmost `X-Forwarded-For` entry that is not a trusted proxy, and `X-Forwarded-Proto: https` marks the session cookie `Secure`.
- **Session management** — tokens expire after 7 days and are invalidated on server restart
- **Cross-origin protection** — state-changing requests sent cross-origin by a browser are rejected (`net/http.CrossOriginProtection`)

Independent of authentication, every response carries the security headers (CSP without inline scripts, `X-Frame-Options`, `nosniff`), including the login page and `401`s. Without a password, only requests whose `Host` is an IP address, `localhost`/`*.localhost` or a name in `DG_ALLOWED_HOSTS` are served (`api/hosts.go`); `/healthz` answers any host.

### Limits

- Log streams (SSE): 64 at once and 8 per client; aggregate `/api/logs` streams additionally 8 and 4 per client. Over the limit: `503` with `Retry-After`. Each stream write has a 30-second deadline, so a client that stops reading is dropped.
- Log history (`/logs/history` routes): 8 requests at once and 4 per client. A page holds the newest lines, at most 512 KiB of them, so it can hold fewer lines than asked for; the UI pages back until a page is empty, and a failed request doesn't end the history. A container page leaves out the line at the cursor, which Docker's `until` includes. An aggregate page is the newest lines across all sources, ending where the page of a source that may hold older lines ends, so the next page misses none.
- Container and volume inspect: 16 requests at once and 8 per client. Stats history: 8 and 4; a query covering more than 120,000 container-points returns every n-th point.
- WebSocket: 128 connections, 16 per client; client messages are capped at 512 bytes. Writes have the same 30-second deadline, and a connection holds its slot until its writer has stopped. Shutdown closes every connection within one second.
- Log frames over 1 MiB are rejected rather than allocated. A log line is cut after 16 KiB of message, the size Docker's own drivers split messages at, and its timestamp: a TTY container's (raw) log joins those pieces back into one line, and drivers such as awslogs send longer ones. Without a TTY and with the json-file or local driver, Docker sends the rest of a long line as further lines.
- Stats history: at most 300,000 container samples overall; with many containers it covers less than 24 hours.

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Returns `200 ok` if Docker daemon is reachable |
| `POST` | `/api/login` | Authenticate with password, returns JWT token |
| `GET` | `/login.js` | Login page script (served without a session) |
| `POST` | `/api/logout` | Invalidate current session |
| `GET` | `/api/auth/check` | Verify session validity |
| `GET` | `/ws` | WebSocket upgrade for live graph and stats updates |
| `GET` | `/api/containers/:id` | Container inspect (detailed info); proxied to the owning node's agent in swarm mode |
| `GET` | `/api/containers/:id/logs` | Stream container logs (SSE); `?since=` resumes right after the client's newest line, without a gap |
| `GET` | `/api/containers/:id/logs/history` | Paginated log history (JSON) |
| `GET` | `/api/logs` | Stream all containers' logs merged (SSE); `?stack=` scopes to one stack |
| `GET` | `/api/logs/history` | Paginated merged log history across all containers (JSON); `?stack=` scopes to one stack; sources are read in parallel, each with a timeout |
| `GET` | `/api/services/:id` | Swarm service inspect with tasks and their nodes (swarm mode) |
| `GET` | `/api/services/:id/logs` | Stream a swarm service's logs from all its tasks (SSE, swarm mode) |
| `GET` | `/api/services/:id/logs/history` | Paginated swarm service log history (JSON, swarm mode) |
| `GET` | `/api/swarm/managers` | Managers' raft reachability and leader, read live (swarm mode); `503` when the swarm has no leader |
| `GET` | `/api/networks/:name` | Network inspect (IPAM, containers on this node); in swarm mode also its services cluster-wide, with VIPs and task addresses |
| `GET` | `/api/volumes/:name` | Volume inspect (driver, usage, labels); searched on every node in swarm mode with agents |
| `GET` | `/api/system/info` | Docker host system information, including its `hostname` |
| `GET` | `/api/system/disk-usage` | Docker disk usage breakdown |
| `GET` | `/api/images` | Docker image list |
| `GET` | `/api/stats/history` | Stats time-series for dashboard charts; `?stack=` keeps one stack's series, `?scope=nodes` returns the per-node aggregates instead |
| `GET` | `/api/events/recent` | Recent Docker events |
| `GET` | `/*` | Serves embedded frontend SPA with client-side routing fallback |

## Testing

```bash
go test ./...
```

Tests cover node/edge builders, compose YAML parsing, swarm snapshots, state merge logic, the agent API and request signing, agent discovery and stats merging, the remote proxy, config loading, and WebSocket broadcast integration.

An opt-in end-to-end test deploys the server and agents on a local swarm: `make swarm-smoke` from the repository root (see `test/swarm_smoke.sh`). Multi-node failure scenarios live in `clustertest/` behind the `clustertest` build tag, so `go test ./...` skips them; they run against the Docker-in-Docker cluster from `make swarm-cluster-up` with `make swarm-cluster-test`.

## Dependencies

| Package | Purpose |
|---------|---------|
| [`docker/docker`](https://pkg.go.dev/github.com/docker/docker) | Docker Engine API client |
| [`compose-spec/compose-go`](https://pkg.go.dev/github.com/compose-spec/compose-go/v2) | Docker Compose file parsing |
| [`go.yaml.in/yaml/v4`](https://pkg.go.dev/go.yaml.in/yaml/v4) | Reading a compose file's `include` entries (the YAML library compose-go uses) |
| [`gorilla/websocket`](https://pkg.go.dev/github.com/gorilla/websocket) | WebSocket connections |
| [`fsnotify/fsnotify`](https://pkg.go.dev/github.com/fsnotify/fsnotify) | Filesystem event watching |
| [`containerd/errdefs`](https://pkg.go.dev/github.com/containerd/errdefs) | Telling Docker API errors apart (not found, …) |
| [`golang.org/x/crypto`](https://pkg.go.dev/golang.org/x/crypto/argon2) | Argon2id password hashing |
| [`golang.org/x/sync`](https://pkg.go.dev/golang.org/x/sync/errgroup) | Fetching snapshot resources concurrently |
| [`golang.org/x/sys`](https://pkg.go.dev/golang.org/x/sys/unix) | Landlock and resource limits of the compose parse process |
| [`golang.org/x/time`](https://pkg.go.dev/golang.org/x/time/rate) | Rate limiting requests to agents |

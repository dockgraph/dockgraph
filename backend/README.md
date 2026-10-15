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

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DG_BIND_ADDR` | `0.0.0.0` | Listen address (`127.0.0.1` to restrict to localhost) |
| `DG_PORT` | `7800` | HTTP listen port |
| `DG_POLL_INTERVAL` | `30s` | Docker API polling interval (Go duration) |
| `DG_COMPOSE_PATH` | _(auto-detect)_ | Override: comma-separated compose files or directories to scan |
| `DG_PASSWORD` | _(disabled)_ | Password for UI and WebSocket access (plaintext or argon2id hash) |
| `DG_PASSWORD_FILE` | _(none)_ | File holding the password, e.g. a Docker secret; `DG_PASSWORD` wins if both are set. An unreadable or empty file aborts startup |
| `DG_STATS_INTERVAL` | `3s` | Container stats poll interval (Go duration) |
| `DG_STATS_WORKERS` | `50` | Max concurrent stats API calls |
| `DG_MODE` | `auto` | `auto`, `standalone`, `swarm` or `agent`. `auto` picks `swarm` on a swarm manager and `standalone` outside a swarm, and refuses to start on a swarm worker, on a pending, locked or errored swarm node, or when the daemon can't be queried (after retrying for about 30 seconds) |
| `DG_SWARM_POLL_INTERVAL` | `5s` | Swarm task poll interval (Go duration, min `1s`) |
| `DG_AGENT_PORT` | `7801` | Per-node agent HTTP port (agents listen on it, the server dials it) |
| `DG_AGENT_ADDR` | `tasks.agent` | DNS name resolving to every agent task, optionally `host:port` |
| `DG_AGENT_TOKEN` | _(none)_ | Shared bearer secret between server and agents; required in `agent` mode. Without it the swarm server runs without agents |
| `DG_AGENT_TOKEN_FILE` | _(none)_ | File holding the agent token, e.g. a Docker secret; `DG_AGENT_TOKEN` wins if both are set |

Compose paths are auto-detected from the container's own bind mounts (excluding the Docker socket). Set `DG_COMPOSE_PATH` only if you need to override this behavior. Prefix an entry with `name=` (e.g. `shop=/stacks/shop.yml`) to set its compose project or swarm stack name.

See the [root README](../README.md#docker-swarm) for deploying the server and agents with `stack.yml`.

## Project Structure

```
backend/
├── main.go              # Bootstrap: wires collectors, state manager, HTTP server (or the agent)
├── config.go            # Environment-based configuration, *_FILE secret loading
├── mode.go              # Resolves DG_MODE against the daemon's swarm state
├── agent/
│   └── agent.go             # Per-node agent (DG_MODE=agent): token-protected stats, inspect, logs
├── api/
│   ├── server.go            # HTTP routes: /healthz, /ws, /login, resource APIs, SPA fallback
│   ├── ws.go                # WebSocket hub with ping/pong heartbeat
│   ├── containers.go        # Container list and stats endpoints
│   ├── containers_detail.go # Container inspect endpoint
│   ├── networks.go          # Network inspect endpoint
│   ├── volumes.go           # Volume inspect endpoint
│   ├── logs.go              # Container log history and SSE streaming endpoints
│   ├── logs_aggregate.go    # Aggregated log history and SSE streaming (stack-scoped, swarm services)
│   ├── services.go          # Swarm service inspect and service log endpoints
│   ├── remote.go            # Proxies container inspect/logs to the agent on the owning node
│   ├── events.go            # Recent Docker events endpoint
│   ├── stats_history.go     # Stats time-series endpoint (stack scope, per-node scope)
│   ├── system.go            # System info, disk usage, and image list endpoints
│   ├── system_cache.go      # Caching wrapper for system API calls
│   └── validation.go        # Request validation helpers
├── auth/
│   ├── handlers.go          # Login/logout HTTP handlers
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
│   ├── compose_parser.go    # Parses compose YAML into graph nodes and edges
│   ├── mounts.go            # Auto-detects compose paths from container bind mounts
│   ├── stats_collector.go   # Container stats collection orchestrator
│   ├── stats_worker.go      # Stats polling worker; merges remote samples, service and node aggregates
│   ├── agent_pool.go        # Discovers agents via DNS and collects their stats each round
│   ├── stats_calc.go        # CPU/memory/network stats calculation
│   ├── stats_types.go       # Stats data types
│   ├── stats_history.go     # Ring buffer for stats time-series (dashboard charts)
│   ├── event_history.go     # Ring buffer for recent Docker events
│   └── debounce.go          # Timer-based debounce helper
├── state/
│   ├── manager.go           # Merges Docker + Compose snapshots, notifies subscribers
│   └── diff.go              # Snapshot diffing for incremental delta updates
├── secrets/
│   └── secrets.go           # Masks credential-like env values (shared by api + collector)
└── frontend/
    └── embed.go             # Embeds built frontend assets into the binary
```

## Architecture

### Data Collection

Two independent collectors run concurrently:

**DockerCollector** connects to the Docker daemon via the API client. It performs an initial poll on startup and then watches the event stream for topology-relevant changes (container create/destroy, network connect/disconnect). Events are debounced at 500ms to avoid redundant polls during burst operations like `docker compose up`.

**ComposeCollector** auto-detects compose files from the container's own bind mounts, parses them into graph nodes, and watches for filesystem changes via `fsnotify`. This surfaces services that are defined but not yet running, giving the UI a complete view of the intended topology.

Both collectors implement the `Collector` interface and emit `StateUpdate` values on a channel.

### Swarm Mode

`DG_MODE` is resolved at startup against the daemon's swarm state (`mode.go`). In `swarm` mode the DockerCollector builds its snapshot from the swarm API (`swarm_snapshot.go`):

- **Services** become `service` nodes placed in their overlay networks, with replica counts, mode (replicated/global/job), update status and their tasks. Each task carries the hostname of the node it runs on. The stack comes from the `com.docker.stack.namespace` label.
- **Nodes** become `swarmnode` nodes with role, leader flag, availability, state, address, engine version and capacity (CPU and memory). A hostname appearing twice (a node that left and rejoined) keeps only the ready or most recent entry.
- Service and node events trigger a re-snapshot, and tasks are polled every `DG_SWARM_POLL_INTERVAL` since task changes emit no events.
- Stack files mounted with `DG_COMPOSE_PATH` show services that are not deployed yet, as with compose.

**Agents.** With `DG_MODE=agent`, the binary serves only the agent API under `/agent/v1` (info, stats, container inspect and logs) plus `/healthz`, and every request must carry the shared token. The server's `AgentPool` resolves `DG_AGENT_ADDR` (`tasks.agent`) periodically, keeping known agents on transient DNS errors, and polls every agent each stats round. Remote samples are merged with local ones (local wins on a name collision), stamped with their node, and aggregated per service and per node under `node:{hostname}`. Every node whose agent answered gets an aggregate, even when it runs no other container.

**Proxy.** Container inspect and log requests for a container on another node are forwarded to that node's agent (`api/remote.go`). Non-streaming responses are capped at 16 MiB; log streams stay open until the client disconnects.

### State Merging

The `state.Manager` receives snapshots from both collectors and merges them into a unified graph. Docker data takes precedence for nodes that exist in both sources (it has actual runtime state), while compose-only metadata like `Source` and `NetworkID` is preserved when Docker doesn't provide it. The merged graph is broadcast to all WebSocket subscribers.

### WebSocket Protocol

Clients connect to `/ws` and receive a `WireMessage` envelope:

```json
{
  "type": "snapshot",
  "version": 2,
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

Every message carries the protocol version (`collector.ProtocolVersion`). Version 2 added the `service` and `swarmnode` node types. The frontend accepts only its own version. On a mismatch it reloads once to fetch a matching bundle, and ignores the messages if the mismatch persists.

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

- **Password hashing** with Argon2id (constant-time comparison)
- **JWT tokens** with HMAC-SHA256, signed with an ephemeral key generated at startup
- **Rate limiting** on the login endpoint to prevent brute-force attacks
- **Session management** — tokens expire after 7 days and are invalidated on server restart

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Returns `200 ok` if Docker daemon is reachable |
| `POST` | `/api/login` | Authenticate with password, returns JWT token |
| `POST` | `/api/logout` | Invalidate current session |
| `GET` | `/api/auth/check` | Verify session validity |
| `GET` | `/ws` | WebSocket upgrade for live graph and stats updates |
| `GET` | `/api/containers/:id` | Container inspect (detailed info); proxied to the owning node's agent in swarm mode |
| `GET` | `/api/containers/:id/logs` | Stream container logs (SSE) |
| `GET` | `/api/containers/:id/logs/history` | Paginated log history (JSON) |
| `GET` | `/api/logs` | Stream all containers' logs merged (SSE); `?stack=` scopes to one stack |
| `GET` | `/api/logs/history` | Paginated merged log history across all containers (JSON); `?stack=` scopes to one stack |
| `GET` | `/api/services/:id` | Swarm service inspect with tasks and their nodes (swarm mode) |
| `GET` | `/api/services/:id/logs` | Stream a swarm service's logs from all its tasks (SSE, swarm mode) |
| `GET` | `/api/services/:id/logs/history` | Paginated swarm service log history (JSON, swarm mode) |
| `GET` | `/api/networks/:name` | Network inspect (IPAM, containers) |
| `GET` | `/api/volumes/:name` | Volume inspect (driver, usage, labels) |
| `GET` | `/api/system/info` | Docker host system information |
| `GET` | `/api/system/disk-usage` | Docker disk usage breakdown |
| `GET` | `/api/images` | Docker image list |
| `GET` | `/api/stats/history` | Stats time-series for dashboard charts; `?stack=` keeps one stack's series, `?scope=nodes` returns the per-node aggregates instead |
| `GET` | `/api/events/recent` | Recent Docker events |
| `GET` | `/*` | Serves embedded frontend SPA with client-side routing fallback |

## Testing

```bash
go test ./...
```

Tests cover node/edge builders, compose YAML parsing, swarm snapshots, state merge logic, the agent API, agent discovery and stats merging, the remote proxy, config loading, and WebSocket broadcast integration.

An opt-in end-to-end test deploys the server and agents on a local swarm: `make swarm-smoke` from the repository root (see `test/swarm_smoke.sh`).

## Dependencies

| Package | Purpose |
|---------|---------|
| [`docker/docker`](https://pkg.go.dev/github.com/docker/docker) | Docker Engine API client |
| [`compose-spec/compose-go`](https://pkg.go.dev/github.com/compose-spec/compose-go/v2) | Docker Compose file parsing |
| [`gorilla/websocket`](https://pkg.go.dev/github.com/gorilla/websocket) | WebSocket connections |
| [`fsnotify/fsnotify`](https://pkg.go.dev/github.com/fsnotify/fsnotify) | Filesystem event watching |

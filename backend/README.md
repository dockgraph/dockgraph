# DockGraph — Backend

Go server that monitors Docker infrastructure in real time and serves a graph topology over WebSocket. Watches the Docker daemon for container, network, and volume changes while simultaneously parsing Docker Compose files to include services that haven't started yet.

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
| `DG_PASSWORD` | _(disabled)_ | Password for UI and WebSocket access |

Compose paths are auto-detected from the container's own bind mounts (excluding the Docker socket). Set `DG_COMPOSE_PATH` only if you need to override this behavior.

## Project Structure

```
backend/
├── main.go              # Bootstrap: wires collectors, state manager, HTTP server
├── config.go            # Environment-based configuration
├── api/
│   ├── server.go            # HTTP routes: /healthz, /ws, /login, resource APIs, SPA fallback
│   ├── ws.go                # WebSocket hub with ping/pong heartbeat
│   ├── containers.go        # Container list and stats endpoints
│   ├── containers_detail.go # Container inspect endpoint
│   ├── networks.go          # Network inspect endpoint
│   ├── volumes.go           # Volume inspect endpoint
│   ├── logs.go              # Container log streaming endpoint
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
│   ├── docker_helpers.go    # Predicates: self-exclusion, topology events, status
│   ├── node_builder.go      # Shared node constructors and network classification
│   ├── compose.go           # Compose collector: file discovery + filesystem watcher
│   ├── compose_parser.go    # Parses compose YAML into graph nodes and edges
│   ├── mounts.go            # Auto-detects compose paths from container bind mounts
│   ├── stats_collector.go   # Container stats collection orchestrator
│   ├── stats_worker.go      # Per-container stats polling worker
│   ├── stats_calc.go        # CPU/memory/network stats calculation
│   └── stats_types.go       # Stats data types
├── state/
│   ├── manager.go           # Merges Docker + Compose snapshots, notifies subscribers
│   └── diff.go              # Snapshot diffing for incremental delta updates
└── frontend/
    └── embed.go             # Embeds built frontend assets into the binary
```

## Architecture

### Data Collection

Two independent collectors run concurrently:

**DockerCollector** connects to the Docker daemon via the API client. It performs an initial poll on startup and then watches the event stream for topology-relevant changes (container create/destroy, network connect/disconnect). Events are debounced at 500ms to avoid redundant polls during burst operations like `docker compose up`.

**ComposeCollector** auto-detects compose files from the container's own bind mounts, parses them into graph nodes, and watches for filesystem changes via `fsnotify`. This surfaces services that are defined but not yet running, giving the UI a complete view of the intended topology.

Both collectors implement the `Collector` interface and emit `StateUpdate` values on a channel.

### State Merging

The `state.Manager` receives snapshots from both collectors and merges them into a unified graph. Docker data takes precedence for nodes that exist in both sources (it has actual runtime state), while compose-only metadata like `Source` and `NetworkID` is preserved when Docker doesn't provide it. The merged graph is broadcast to all WebSocket subscribers.

### WebSocket Protocol

Clients connect to `/ws` and receive a `WireMessage` envelope:

```json
{
  "type": "snapshot",
  "version": 1,
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

### Graph Model

Nodes use namespaced IDs to prevent collisions:

| Type | ID Format | Example |
|------|-----------|---------|
| Container | `container:{name}` | `container:web-1` |
| Network | `network:{name}` | `network:myapp_default` |
| Volume | `volume:{name}` | `volume:myapp_data` |

Edge types:

| Type | Meaning |
|------|---------|
| `depends_on` | Compose service dependency |
| `volume_mount` | Container mounts a named volume |
| `secondary_network` | Container connected to a non-primary network |

### Authentication

When `DG_PASSWORD` is set, all endpoints (except `/healthz`) require a valid JWT session token. The `auth` package handles:

- **Password hashing** with Argon2id (constant-time comparison)
- **JWT tokens** with HMAC-SHA256, signed with an ephemeral key generated at startup
- **Rate limiting** on the login endpoint to prevent brute-force attacks
- **Session management** — tokens expire after 7 days and are invalidated on server restart

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Returns `200 ok` if Docker daemon is reachable |
| `POST` | `/login` | Authenticate with password, returns JWT token |
| `GET` | `/ws` | WebSocket upgrade for live graph and stats updates |
| `GET` | `/api/containers` | List containers with basic stats |
| `GET` | `/api/containers/:name` | Container inspect (detailed info) |
| `GET` | `/api/containers/:name/logs` | Stream container logs (SSE) |
| `GET` | `/api/networks/:name` | Network inspect (IPAM, containers) |
| `GET` | `/api/volumes/:name` | Volume inspect (driver, usage, labels) |
| `GET` | `/*` | Serves embedded frontend SPA with client-side routing fallback |

## Testing

```bash
go test ./...
```

Tests cover node/edge builders, compose YAML parsing, state merge logic, and WebSocket broadcast integration.

## Dependencies

| Package | Purpose |
|---------|---------|
| [`docker/docker`](https://pkg.go.dev/github.com/docker/docker) | Docker Engine API client |
| [`compose-spec/compose-go`](https://pkg.go.dev/github.com/compose-spec/compose-go/v2) | Docker Compose file parsing |
| [`gorilla/websocket`](https://pkg.go.dev/github.com/gorilla/websocket) | WebSocket connections |
| [`fsnotify/fsnotify`](https://pkg.go.dev/github.com/fsnotify/fsnotify) | Filesystem event watching |

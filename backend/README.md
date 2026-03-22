# Docker Flow — Backend

Go server that monitors Docker infrastructure in real time and serves a graph topology over WebSocket. Watches the Docker daemon for container, network, and volume changes while simultaneously parsing Docker Compose files to include services that haven't started yet.

## Requirements

- Go 1.26+
- Docker daemon (accessible via socket or TCP)

## Quick Start

```bash
go build -o docker-flow .
./docker-flow
```

The server starts on `:7800` by default and serves the embedded frontend SPA.

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DF_PORT` | `7800` | HTTP listen port |
| `DF_POLL_INTERVAL` | `30s` | Docker API polling interval (Go duration) |
| `DF_COMPOSE_DIR` | `/app/compose` | Directory to scan for `docker-compose.yml` files |

## Project Structure

```
backend/
├── main.go              # Bootstrap: wires collectors, state manager, HTTP server
├── config.go            # Environment-based configuration
├── api/
│   ├── server.go            # HTTP routes: /healthz, /ws, SPA fallback
│   └── ws.go                # WebSocket hub with ping/pong heartbeat
├── collector/
│   ├── types.go             # Core types: Node, Edge, GraphSnapshot, Collector interface
│   ├── docker.go            # Docker collector: poll loop + event stream watcher
│   ├── docker_snapshot.go   # Fetches Docker resources, assembles graph snapshot
│   ├── docker_helpers.go    # Predicates: self-exclusion, topology events, status
│   ├── node_builder.go      # Shared node constructors and network classification
│   ├── compose.go           # Compose collector: file discovery + filesystem watcher
│   └── compose_parser.go    # Parses compose YAML into graph nodes and edges
├── state/
│   └── manager.go           # Merges Docker + Compose snapshots, notifies subscribers
└── frontend/
    └── embed.go             # Embeds built frontend assets into the binary
```

## Architecture

### Data Collection

Two independent collectors run concurrently:

**DockerCollector** connects to the Docker daemon via the API client. It performs an initial poll on startup and then watches the event stream for topology-relevant changes (container create/destroy, network connect/disconnect). Events are debounced at 500ms to avoid redundant polls during burst operations like `docker compose up`.

**ComposeCollector** scans a directory for `docker-compose.yml` files, parses them into graph nodes, and watches for filesystem changes via `fsnotify`. This surfaces services that are defined but not yet running, giving the UI a complete view of the intended topology.

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

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Returns `200 ok` if Docker daemon is reachable |
| `GET` | `/ws` | WebSocket upgrade for live graph updates |
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

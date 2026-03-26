# DockGraph

Real-time Docker infrastructure visualizer. See your containers, networks, volumes, and their relationships as an interactive graph that updates live as your infrastructure changes.

<!-- TODO: Add screenshot/GIF of DockGraph running with the demo stack -->

## Features

- **Live topology graph** — containers, networks, and volumes rendered as an interactive, zoomable graph
- **Real-time updates** — watches the Docker event stream; the graph reflects changes within seconds
- **Compose-aware** — parses compose files to show services that haven't started yet
- **Network grouping** — containers are visually grouped by their primary network
- **Dependency visualization** — `depends_on` edges with animated flow dots for running services
- **Volume relationships** — named volume mounts shown as edges between volumes and containers
- **Multi-network support** — secondary network connections rendered as cross-group edges
- **Dark/light theme** — toggle between themes, persisted in localStorage
- **Click-to-highlight** — click any node or edge to highlight its connections, fading unrelated elements
- **Single binary** — frontend is embedded into the Go binary; one container, no external dependencies
- **Self-excluding** — DockGraph hides its own container from the graph

## Quick Start

```bash
docker run -d \
  -p 7800:7800 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  --label dockgraph.self=true \
  dockgraph/dockgraph
```

Open [http://localhost:7800](http://localhost:7800).

### Adding to your Docker Compose stack

Add DockGraph as a service in your existing `compose.yml`:

```yaml
services:
  dockgraph:
    image: dockgraph/dockgraph:latest
    ports:
      - "7800:7800"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./compose.yml:/app/compose/compose.yml:ro
    labels:
      dockgraph.self: "true"
```

The compose file mount is optional — it lets DockGraph show services that aren't running yet.

## Demo

A ~46-service simulated SaaS platform is included for showcasing DockGraph with a realistic topology. See [`demo/README.md`](demo/README.md) for setup and architecture.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DF_PORT` | `7800` | HTTP listen port |
| `DF_POLL_INTERVAL` | `30s` | Docker API polling interval |
| `DF_COMPOSE_DIR` | `/app/compose` | Directory to scan for compose files |

## How It Works

DockGraph runs two collectors concurrently:

1. **Docker collector** — polls the Docker API and watches the event stream for container, network, and volume changes
2. **Compose collector** — parses compose files and watches for filesystem changes

Both feed into a state manager that merges their outputs (Docker runtime data takes precedence) and broadcasts the unified graph over WebSocket. The React frontend receives these updates and renders the topology using the [ELK](https://www.eclipse.org/elk/) layout algorithm.

For implementation details, see the [backend](backend/README.md) and [frontend](frontend/README.md) READMEs.

## Development

### Prerequisites

- Go 1.26+
- Node.js 24+
- Docker daemon

### Using Make

```bash
make build          # Build frontend + backend
make test           # Run Go tests
make lint           # Run all linters (golangci-lint + eslint)
make docker         # Build Docker image locally
make docker-up      # Start with Docker Compose
make help           # Show all available targets
```

### Manual setup

```bash
# Backend
cd backend
go build -o dockgraph .
./dockgraph

# Frontend
cd frontend
npm install
npm run dev
```

The Vite dev server proxies `/ws` and `/healthz` to the backend at `localhost:7800`.

### Running Tests

```bash
make test
# or
cd backend && go test ./...
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go, Docker Engine API, gorilla/websocket |
| Frontend | React 19, TypeScript, React Flow, ELK.js |
| Build | Vite, multi-stage Dockerfile |
| Runtime | distroless/static (production image) |

## Contributing

Contributions are welcome. Please read the [contributing guide](CONTRIBUTING.md) before submitting a pull request.

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md) code of conduct.

## License

This project is licensed under the [MIT License](LICENSE).

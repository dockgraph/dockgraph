# Docker Flow

Real-time Docker infrastructure visualizer. See your containers, networks, volumes, and their relationships as an interactive graph that updates live as your infrastructure changes.

<!-- TODO: Add screenshot/GIF of Docker Flow running with the demo stack -->

## Features

- **Live topology graph** — containers, networks, and volumes rendered as an interactive, zoomable graph
- **Real-time updates** — watches the Docker event stream; the graph reflects changes within seconds
- **Compose-aware** — parses `docker-compose.yml` files to show services that haven't started yet
- **Network grouping** — containers are visually grouped by their primary network
- **Dependency visualization** — `depends_on` edges with animated flow dots for running services
- **Volume relationships** — named volume mounts shown as edges between volumes and containers
- **Multi-network support** — secondary network connections rendered as cross-group edges
- **Dark/light theme** — toggle between themes, persisted in localStorage
- **Click-to-highlight** — click any node or edge to highlight its connections, fading unrelated elements
- **Single binary** — frontend is embedded into the Go binary; one container, no external dependencies
- **Self-excluding** — Docker Flow hides its own container from the graph

## Quick Start

```bash
docker run -d \
  -p 7800:7800 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  ghcr.io/dockgraph/docker-flow
```

Open [http://localhost:7800](http://localhost:7800).

To include Compose file definitions (shows services that aren't running yet):

```bash
docker run -d \
  -p 7800:7800 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v ./docker-compose.yml:/app/compose/docker-compose.yml:ro \
  ghcr.io/dockgraph/docker-flow
```

Or use Docker Compose:

```bash
git clone https://github.com/dockgraph/docker-flow.git
cd docker-flow
docker compose up -d
```

## Demo

A ~46-service simulated SaaS platform is included for showcasing Docker Flow with a realistic topology. See [`demo/README.md`](demo/README.md) for setup and architecture.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DF_PORT` | `7800` | HTTP listen port |
| `DF_POLL_INTERVAL` | `30s` | Docker API polling interval |
| `DF_COMPOSE_DIR` | `/app/compose` | Directory to scan for compose files |

## How It Works

Docker Flow runs two collectors concurrently:

1. **Docker collector** — polls the Docker API and watches the event stream for container, network, and volume changes
2. **Compose collector** — parses `docker-compose.yml` files and watches for filesystem changes

Both feed into a state manager that merges their outputs (Docker runtime data takes precedence) and broadcasts the unified graph over WebSocket. The React frontend receives these updates and renders the topology using the [ELK](https://www.eclipse.org/elk/) layout algorithm.

For implementation details, see the [backend](backend/README.md) and [frontend](frontend/README.md) READMEs.

## Development

### Prerequisites

- Go 1.26+
- Node.js 24+
- Docker daemon

### Backend

```bash
cd backend
go build -o docker-flow .
./docker-flow
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

The Vite dev server proxies `/ws` and `/healthz` to the backend at `localhost:7800`.

### Docker Build

```bash
docker compose up --build
```

### Running Tests

```bash
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

Contributions are welcome. Please open an issue to discuss changes before submitting a pull request.

## License

MIT

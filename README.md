<div align="center" width="100%">
    <img src="https://raw.githubusercontent.com/dockgraph/dockgraph/main/.github/assets/logo.svg" width="128" alt="DockGraph" />
</div>

# DockGraph

Real-time Docker infrastructure visualizer. See your containers, networks, volumes, and their relationships as an interactive graph that updates live as your infrastructure changes.

[![GitHub Repo stars](https://img.shields.io/github/stars/dockgraph/dockgraph?logo=github&style=flat)](https://github.com/dockgraph/dockgraph)
[![License](https://img.shields.io/badge/license-BSL--1.1-blue.svg)](https://github.com/dockgraph/dockgraph/blob/main/LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/dockgraph/dockgraph?logo=docker)](https://hub.docker.com/r/dockgraph/dockgraph/tags)
[![Docker Image Version (latest semver)](https://img.shields.io/docker/v/dockgraph/dockgraph/latest?logo=docker&label=docker%20image%20ver.)](https://hub.docker.com/r/dockgraph/dockgraph/tags)
[![GitHub last commit (branch)](https://img.shields.io/github/last-commit/dockgraph/dockgraph/main?logo=github)](https://github.com/dockgraph/dockgraph/commits/main/)
[![codecov](https://codecov.io/github/dockgraph/dockgraph/graph/badge.svg?token=TGEJFE4CMY)](https://codecov.io/github/dockgraph/dockgraph)

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/artemkozak)

<div align="center" width="100%">
    <img src="https://raw.githubusercontent.com/dockgraph/dockgraph/main/.github/assets/screenshot.png" width="1280" alt="DockGraph screenshot" />
</div>

## Features

- **Live topology graph** — containers, networks, and volumes rendered as an interactive, zoomable graph
- **Table view** — alternative tabular view with sortable columns, grouping by compose project / network / status / driver, and collapsible groups
- **Dashboard view** — 13-card monitoring dashboard with resource charts, top consumers, event timeline, alerts, disk usage, images, and compose project overview
- **Detail panels** — click any resource to inspect stats, ports, mounts, environment, labels, logs, health checks, and network configuration
- **Real-time updates** — watches the Docker event stream; the graph reflects changes within seconds
- **Compose-aware** — parses compose files to show services that haven't started yet
- **Network grouping** — containers are visually grouped by their primary network
- **Dependency visualization** — `depends_on` edges with animated flow dots for running services
- **Volume relationships** — named volume mounts shown as edges between volumes and containers
- **Multi-network support** — secondary network connections rendered as cross-group edges
- **Search and filter** — filter resources by name, type, or status with real-time results across both views
- **Dark/light theme** — toggle between themes, persisted in localStorage
- **Click-to-highlight** — click any node or edge to highlight its connections, fading unrelated elements
- **Password protection** — optional authentication with Argon2id hashing and JWT sessions
- **Single binary** — frontend is embedded into the Go binary; one container, no external dependencies
- **Self-excluding** — DockGraph hides its own container, networks, and volumes from the graph

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
      - "7800:7800" # Web UI
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro # Docker API access
      - ./compose.yml:/compose/compose.yml:ro # Optional: show services before they start
    labels:
      dockgraph.self: "true" # Hide DockGraph from its own graph
```

Compose file mounts are optional — they let DockGraph show services defined in your compose files even when they aren't running yet. Just mount a file or directory and DockGraph picks it up automatically.

## Demo

Three demo stacks of increasing complexity are included for showcasing DockGraph at different scales — from a 5-service web app to a ~46-service SaaS platform. See [`demo/README.md`](demo/README.md) for setup and architecture.

## Configuration

DockGraph auto-detects compose files from mounted volumes — no extra configuration needed. Any bind-mounted file or directory (except the Docker socket) is scanned recursively for `.yml`/`.yaml` files.

```yaml
# Single file
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
  - ./compose.yml:/compose/compose.yml:ro           # auto-detected

# Entire directory (all .yml/.yaml files picked up recursively)
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
  - ./stacks:/compose/stacks:ro                     # auto-detected

# Multiple mounts
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
  - ./frontend.yml:/compose/frontend.yml:ro          # auto-detected
  - ./infra:/compose/infra:ro                        # auto-detected

# Override auto-detection with DG_COMPOSE_PATH
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
  - ./stacks:/compose/stacks:ro
environment:
  DG_COMPOSE_PATH: "/compose/stacks/production.yml"  # scan only this file
```

### Environment variables

| Variable           | Default         | Description                                                                          |
| ------------------ | --------------- | ------------------------------------------------------------------------------------ |
| `DG_BIND_ADDR`     | `0.0.0.0`       | Listen address (`127.0.0.1` to restrict to localhost)                                |
| `DG_PORT`          | `7800`          | HTTP listen port                                                                     |
| `DG_POLL_INTERVAL` | `30s`           | Docker API polling interval                                                          |
| `DG_COMPOSE_PATH`  | _(auto-detect)_ | Override: comma-separated list of compose files or directories to scan               |
| `DG_PASSWORD`      | _(disabled)_    | Password for UI and WebSocket access; when set, requires login to view the dashboard |
| `DG_STATS_INTERVAL`| `3s`            | Container stats poll interval (Go duration)                                          |
| `DG_STATS_WORKERS` | `50`            | Max concurrent stats API calls                                                       |

## Security Considerations

DockGraph requires access to the Docker daemon socket to read container, network, and volume state. Be aware of the following:

- **Password protection.** Set `DG_PASSWORD` to require authentication for the web UI and WebSocket connections. When set, all access goes through a login page — the dashboard and its data are not served until the correct password is provided. Sessions last 7 days and are invalidated on server restart.
  ```yaml
  environment:
    DG_PASSWORD: "your-secure-password"
  ```
  When `DG_PASSWORD` is not set, DockGraph runs without authentication (suitable for localhost or trusted networks).
- **Bind to localhost** when running on a shared network or production host:
  ```yaml
  environment:
    DG_BIND_ADDR: "127.0.0.1"
  ```
- **Use a reverse proxy** (nginx, Caddy, Traefik) for TLS termination if exposing DockGraph beyond your local network. DockGraph serves plain HTTP — the reverse proxy handles HTTPS.
- **Docker socket access** is read-only (`:ro`), but any process that can read the socket can inspect all Docker resources on the host. Run DockGraph in a network-isolated environment or behind a firewall.
- **Read-only API.** DockGraph cannot start, stop, or modify containers. It only observes topology.

## How It Works

DockGraph runs two collectors concurrently:

1. **Docker collector** — polls the Docker API and watches the event stream for container, network, and volume changes
2. **Compose collector** — auto-detects compose files from mounted volumes, parses them, and watches for filesystem changes

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
make test           # Run all tests (backend + frontend)
make test-coverage  # Run tests with coverage reports (enforces thresholds)
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
make test              # Run all tests
make test-coverage     # Run with coverage (backend profile + frontend thresholds)
```

## Tech Stack

| Component | Technology                               |
| --------- | ---------------------------------------- |
| Backend   | Go, Docker Engine API, gorilla/websocket |
| Frontend  | React 19, TypeScript, React Flow, ELK.js |
| Build     | Vite, multi-stage Dockerfile             |
| Runtime   | distroless/static (production image)     |

## Motivation

Existing Docker UIs focus on container management, not on understanding how your infrastructure fits together. DockGraph was born out of the need to see the full picture — containers, networks, volumes, and their relationships — at a glance, updating in real time as things change.

If you find this project useful, please consider giving it a ⭐ — it helps others discover it.

## Contributing

Contributions are welcome. Please read the [contributing guide](CONTRIBUTING.md) before submitting a pull request.

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md) code of conduct.

## License

This project is licensed under the [Business Source License 1.1](LICENSE). You are free to use, modify, and redistribute the software, including in production. The only restriction is offering it as a hosted service or embedding it as a feature in a commercial product. Each version converts to Apache License 2.0 four years after its release.

<div align="center" width="100%">
    <img src="https://raw.githubusercontent.com/dockgraph/dockgraph/main/.github/assets/logo.svg" width="128" alt="DockGraph" />
</div>

# DockGraph

Real-time Docker infrastructure visualizer. See your containers, networks, volumes, and their relationships as an interactive graph that updates live as your infrastructure changes. Works on a single Docker host and across a Docker Swarm cluster.

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
- **Table view** — alternative tabular view with sortable columns, grouping by compose project / stack / network / status / driver, and collapsible groups; a Services tab lists swarm services with their replica counts
- **Dashboard view** — 13-card monitoring dashboard with resource charts, top consumers, event timeline, alerts, disk usage, images, and compose project / stack overview
- **Global logs** — a unified, time-ordered log stream that aggregates every container into one view to trace an event across services; filter by text (literal or regex) or container, drill in with per-row filter actions, scroll back through merged history alongside the live tail, and find within (Ctrl+F)
- **Pop-out log windows** — open any container's logs in a floating, movable and resizable window; drag windows together into tabs, minimize them to a dock, and search within each
- **Detail panels** — click any resource to inspect stats, ports, mounts, environment, labels, logs, health checks, and network configuration; cross-references (dependencies, networks, mounted volumes) link straight to the related resource, and any value is click-to-copy
- **Real-time updates** — watches the Docker event stream; the graph reflects changes within seconds
- **Compose-aware** — parses compose files to show services that haven't started yet, with the same detail panel as running containers (process config, environment, labels, ports, dependencies, and volume mounts) and clickable cross-references into the resources they'll create
- **Docker Swarm** — on a swarm manager, services are shown with their replica counts, tasks and the nodes they run on, grouped by stack; a lightweight per-node agent brings container stats, inspect and logs from every node of the cluster ([details](#docker-swarm))
- **Stack scoping** — a header selector scopes the graph, table, dashboard, logs and stats to one compose project or swarm stack (shareable via `?stack=`)
- **Swarm node view** — switch the graph from networks to nodes to see which tasks run where: managers and workers in their own groups, node health and resource bars, a control-plane link showing worker health, and overlay-network links between services on different nodes; a Swarm Nodes dashboard card and per-node history charts complete the cluster picture
- **Network grouping** — containers are visually grouped by their primary network
- **Dependency visualization** — `depends_on` edges with animated flow dots for running services
- **Volume relationships** — named volume mounts shown as edges between volumes and containers
- **Multi-network support** — secondary network connections rendered as cross-group edges
- **Search and filter** — filter resources by name, type, or status with real-time results across both views
- **Dark/light theme** — toggle between themes, persisted in localStorage
- **Click-to-highlight** — click any node or edge to highlight its connections, fading unrelated elements
- **Password protection** — optional authentication with Argon2id hashing and JWT sessions; the password can come from a Docker secret (`DG_PASSWORD_FILE`)
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

Three demo stacks of increasing complexity are included for showcasing DockGraph at different scales — from a 5-service web app to a ~46-service SaaS platform. Two Docker Swarm stacks (`make demo-swarm`) showcase swarm mode, the stack selector and the node view. See [`demo/README.md`](demo/README.md) for setup and architecture.

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
| `DG_COMPOSE_PATH`  | _(auto-detect)_ | Override: comma-separated list of compose/stack files or directories to scan; prefix an entry with `name=` (e.g. `shop=/stacks/shop.yml`) to set its project/stack name |
| `DG_PASSWORD`      | _(disabled)_    | Password for UI and WebSocket access; when set, requires login to view the dashboard |
| `DG_PASSWORD_FILE` | _(none)_        | File to read the password from (e.g. a Docker secret; plaintext or argon2id hash); `DG_PASSWORD` wins if both are set. The server refuses to start if the file is unreadable or empty |
| `DG_STATS_INTERVAL`| `3s`            | Container stats poll interval (Go duration)                                          |
| `DG_STATS_WORKERS` | `50`            | Max concurrent stats API calls                                                       |
| `DG_MODE`          | `auto`          | `auto`, `standalone`, `swarm` or `agent`. `auto` picks `swarm` on a swarm manager and `standalone` outside a swarm; it refuses to start on a swarm worker, on a pending, locked or errored swarm node, or when the daemon can't be queried after retrying for about 30 seconds |
| `DG_SWARM_POLL_INTERVAL` | `5s`      | Swarm task poll interval (Go duration, min `1s`)                                     |
| `DG_AGENT_PORT`    | `7801`          | Per-node agent HTTP port (agents listen on it, the server dials it)                 |
| `DG_AGENT_ADDR`    | `tasks.agent`   | DNS name resolving to every agent task, optionally `host:port`                       |
| `DG_AGENT_TOKEN`   | _(none)_        | Shared bearer secret between server and agents; required in `agent` mode. Without it the swarm server runs without agents |
| `DG_AGENT_TOKEN_FILE` | _(none)_     | File to read the agent token from (e.g. a Docker secret); `DG_AGENT_TOKEN` wins if both are set |

## Docker Swarm

On a swarm manager, DockGraph shows **services** (with replica counts, tasks and the nodes they run on) grouped by **stack**, instead of individual containers. Use the stack selector to scope the graph, logs and stats to one stack.

[`stack.yml`](stack.yml) deploys two services:

- **`server`** — the UI and API, pinned to a manager (`node.role == manager`), published on port 7800 through the routing mesh.
- **`agent`** — a global service (one task per node) that exposes that node's container stats, inspect and logs to the server. It publishes no port and is only reachable on the stack's internal overlay network.

```bash
# On a manager: create the shared agent token once
openssl rand -hex 32 | docker secret create dg_agent_token -

# Deploy (or: make swarm-deploy)
docker stack deploy -c stack.yml dockgraph

# Open http://<any-node>:7800
```

Both services carry the `dockgraph.self=true` label (as a service label under `deploy.labels` and as a container label) so DockGraph hides itself. `DG_AGENT_ADDR` defaults to `tasks.agent`, which resolves to every agent task inside the stack; change it only if you rename the `agent` service.

To show services from stack files that are not deployed yet, mount them into the `server` service (the path must exist on the manager it runs on) and name each file after its stack, as you would with `docker stack deploy -c file <name>`:

```yaml
    volumes:
      - /opt/stacks/shop.yml:/stacks/shop.yml:ro
    environment:
      DG_COMPOSE_PATH: "shop=/stacks/shop.yml"
```

Demo stacks for swarm are in [`demo/stack-small.yml`](demo/stack-small.yml) and [`demo/stack-medium.yml`](demo/stack-medium.yml) (see [`demo/README.md`](demo/README.md)).

### Swarm views

- **Stack selector** — the header selector scopes every view to one stack (or to standalone containers). The choice lives in the URL (`?stack=`), so a scoped view can be bookmarked or shared.
- **Graph by network** (default) — services appear inside their overlay networks like containers do, with replica badges. Click a service to open its detail panel: mode, replicas, the task table with the node of each task, and live service logs merged across nodes.
- **Graph by node** — the **Network | Node** toggle above the graph (`?group=node`) lays out one box per swarm node, with a card for each service listing the tasks it runs there. Managers (leader first) and workers sit in separate, draggable groups. Each box shows the node's role, state, availability and CPU/memory against its capacity; inactive nodes are dimmed. Managers fan out one control link per worker, coloured and animated by that worker's state, and a badge centred between the two groups reads `control / raft tcp 2377 / ready/total ok` (or the number of workers down). Hovering a service highlights it on every node, and selecting it draws links to services it shares an overlay network with on other nodes. Click a node header for its detail panel.
- **Table** — a **Services** tab lists swarm services with their replica counts, and the grouping options include the stack.
- **Dashboard** — a **Swarm Nodes** card lists every node with its role, task count and CPU/memory bars, and the resource history charts switch between **Workload** and **Node** series.

### Remote task limits

- **Without agents** (no `DG_AGENT_TOKEN`), topology, replica counts and service logs still cover the whole cluster (they come from the manager's swarm API), but container stats, container inspect and container logs are only available for tasks on the server's own node.
- **With agents**, the server resolves `tasks.agent` periodically, polls each agent for stats and proxies container inspect/logs to the node running the container. A node whose agent is down or unreachable simply has no stats until it comes back.
- Agents need the same token as the server; a mismatched token is rejected and that node's data is missing.
- Running the standalone image on a swarm worker is refused in `auto` mode; set `DG_MODE=agent` there (or `DG_MODE=standalone` for a local-only view).

### Known limitations

- **Stats are keyed by name.** A standalone container named exactly like a swarm service (e.g. a container named `shop_web` next to the `shop_web` service) shares its stats entry, and the service shows the container's usage. Compose names its containers `project-service-N`, so this needs a manually named container. The server logs a warning when it happens.
- **Swarm nodes are keyed by hostname**, which hides the stale entry a node leaves behind when it leaves and rejoins the swarm. Two live nodes with the same hostname (e.g. cloned VMs) therefore show as one, with their tasks and stats combined. Give each node a unique hostname; the server logs a warning when two ready nodes share one.

## Security Considerations

DockGraph requires access to the Docker daemon socket to read container, network, and volume state. Be aware of the following:

- **Password protection.** Set `DG_PASSWORD` to require authentication for the web UI and WebSocket connections. When set, all access goes through a login page — the dashboard and its data are not served until the correct password is provided. Sessions last 7 days and are invalidated on server restart.
  ```yaml
  environment:
    DG_PASSWORD: "your-secure-password"
  ```
  To keep the password out of the environment, store it in a file (such as a Docker secret) and point `DG_PASSWORD_FILE` at it. Surrounding whitespace is trimmed, and the file may hold a plaintext password or an argon2id hash. The server refuses to start if the file can't be read or is empty, so a broken secret never disables authentication.
  ```sh
  printf '%s' 'your-secure-password' | docker secret create dg_password -
  ```
  ```yaml
  environment:
    DG_PASSWORD_FILE: "/run/secrets/dg_password"
  secrets:
    - dg_password
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
- **Docker Swarm.** In a swarm, the Docker socket is mounted on **every node** (the agent is a global service), and access to the socket is equivalent to root on that host — even mounted `:ro`, since the Docker API is not restricted by the mount mode. Treat the `dockgraph` stack as privileged on the whole cluster:
  - Keep the agent port (`7801`) on the stack's internal overlay network. Never publish it; `stack.yml` does not.
  - The agent refuses to start without a token, and every agent request needs it as a bearer token. Use a long random value stored as a Docker secret (`dg_agent_token`), not an inline `DG_AGENT_TOKEN`.
  - Overlay traffic between nodes is unencrypted by default; enable `encrypted: "true"` on the internal network (see `stack.yml`) if nodes communicate over an untrusted network.
  - Set `DG_PASSWORD_FILE` (backed by a `dg_password` Docker secret, see `stack.yml`) or `DG_PASSWORD` on the `server` service, since its published port is reachable on every node through the routing mesh.
- **Secret masking.** Environment values whose keys look like credentials (`PASSWORD`, `SECRET`, `KEY`, `TOKEN`, `AUTH`, …) are masked before leaving the server — for both running containers and parsed compose services — so they're never sent to the browser.

## How It Works

DockGraph runs two collectors concurrently:

1. **Docker collector** — polls the Docker API and watches the event stream for container, network, and volume changes
2. **Compose collector** — auto-detects compose files from mounted volumes, parses them, and watches for filesystem changes

Both feed into a state manager that merges their outputs (Docker runtime data takes precedence) and broadcasts the unified graph over WebSocket. The React frontend receives these updates and renders the topology using the [ELK](https://www.eclipse.org/elk/) layout algorithm.

In swarm mode, the Docker collector reads the swarm API instead: services, tasks and nodes, watched through service and node events plus a task poll. The server finds the per-node agents through the `tasks.agent` DNS name, merges their container stats with its own into per-service and per-node aggregates, and forwards container inspect and log requests to the node that runs the container.

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
make swarm-deploy   # Deploy stack.yml to a swarm (needs the dg_agent_token secret)
make demo-swarm     # Deploy the swarm demo stacks
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
make swarm-smoke       # Swarm end-to-end smoke test (opt-in, not run in CI)
```

The swarm smoke test builds the image, deploys `stack.yml` and a throwaway demo stack on the local daemon, and checks the API, the WebSocket snapshot, the node view data (swarm nodes, task placement, per-node stats) and `DG_PASSWORD_FILE` login from a Docker secret. On an existing swarm it only removes what it created. Outside a swarm, `make swarm-smoke` runs `docker swarm init` first and `docker swarm leave --force` at the end; `./test/swarm_smoke.sh` without `--yes` refuses to do that.

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

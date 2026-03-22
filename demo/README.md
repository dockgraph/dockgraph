# Demo: Acme Platform

A simulated SaaS platform with ~46 services across 7 networks for showcasing Docker Flow with a realistic production-like topology.

## Quick Start

```bash
# From the project root
cd demo
docker compose up -d

# Or use the start script (also creates a "search-indexer" in stopped state)
./start.sh
```

Then start Docker Flow from the project root to visualize it:

```bash
docker compose up -d
# Open http://localhost:7800
```

Tear down when done:

```bash
docker compose -f demo/docker-compose.yml down -v
```

## Architecture

**7 networks, 20 volumes, ~46 services across 6 tiers:**

| Network | Tier | Services |
|---------|------|----------|
| `public` | Edge | nginx-lb, web-app, admin-panel, api-gateway |
| `api` | Application | auth, user, product, order, payment, inventory, notification, search, media, pricing, recommendation |
| `workers` | Background | order-worker, email-worker, invoice-worker, analytics-worker, search-indexer, report-scheduler |
| `data` | Storage | postgres (primary + replica), redis (cache + sessions), mongo, elasticsearch, minio, vault |
| `messaging` | Messaging | rabbitmq, kafka, zookeeper |
| `monitoring` | Observability | prometheus, grafana, loki, tempo, alertmanager, node-exporter, cadvisor |
| `ci` | CI/CD | ci-server, ci-runner-1, ci-runner-2, artifact-store, ci-postgres, ci-redis |

## What It Demonstrates

| Feature | How it appears |
|---------|----------------|
| **Network isolation** | 7 colored network groups with clear tier separation |
| **Multi-network containers** | api-gateway (public + api), order-service (api + messaging), prometheus (monitoring + api) |
| **Deep dependency chains** | nginx-lb → api-gateway → order-service → postgres, rabbitmq |
| **Shared databases** | Multiple API services all depending on postgres + redis |
| **Volume mounts** | 20 named volumes across data, monitoring, and CI tiers |
| **Message queues** | Workers connected to rabbitmq/kafka via the messaging network |
| **Animated dependencies** | `depends_on` edges show flowing dots for ~31 active dependency relationships |
| **Exposed ports** | nginx :80/:443, grafana :3000, rabbitmq :15672, minio :9001 |
| **Container states** | Running, stopped, and created containers all visible with distinct styling |
| **Profiled services** | `search-indexer` created but not started (use `./start.sh`) |

## Profiled Services

Some services are behind Compose profiles for on-demand use:

```bash
# Start with the database backup service
docker compose --profile with-backup up -d

# Start with the staging proxy
docker compose --profile with-staging up -d

# Start with the load tester
docker compose --profile with-loadtest up -d
```

## Resource Usage

- Application services use `busybox` with `sleep infinity` — near-zero CPU and memory.
- Infrastructure services (postgres, redis, prometheus, grafana, rabbitmq) use real images for realistic health checks and status data.
- Total memory footprint is roughly ~1.5 GB with all services running.

## Experimenting

Try these to see Docker Flow react in real time:

```bash
# Stop a service and watch it turn grey
docker stop acme-platform-order-service-1

# Bring it back
docker start acme-platform-order-service-1

# Scale up a worker
docker compose -f demo/docker-compose.yml up -d --scale email-worker=3

# Take down an entire tier
docker compose -f demo/docker-compose.yml stop kafka zookeeper
```

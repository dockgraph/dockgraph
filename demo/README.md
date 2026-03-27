# Demo Stacks

Four compose stacks of increasing complexity for showcasing DockGraph at different scales.

| Stack      | File                 | Services | Networks | Volumes | Description                                                 |
| ---------- | -------------------- | -------- | -------- | ------- | ----------------------------------------------------------- |
| **Small**  | `compose-small.yml`  | 5        | 2        | 2       | Classic web app: nginx, app, postgres, redis, worker        |
| **Medium** | `compose-medium.yml` | ~15      | 4        | 5       | E-commerce platform: edge, API services, workers, messaging |
| **Large**  | `compose-large.yml`  | ~46      | 7        | 19      | Full SaaS platform: 6 tiers with CI/CD and observability    |
| **Stress** | `compose-stress.yml` | ~700     | 10       | 5       | Stress test: 15 service types across 10 networks            |

## Quick Start

From the project root:

```bash
make demo-small          # Start small demo (5 services)
make demo-medium         # Start medium demo (~15 services)
make demo-large          # Start large demo (~46 services)
make demo                # Start all three stacks

make demo-small-down     # Stop small demo and remove volumes
make demo-medium-down    # Stop medium demo and remove volumes
make demo-large-down     # Stop large demo and remove volumes
make demo-down           # Stop all demos and remove volumes
```

Then start DockGraph to visualize:

```bash
docker compose up -d
# Open http://localhost:7800
```

## Small Stack

**5 services, 2 networks, 1 volume** — a classic three-tier web app.

```
nginx → app → postgres
              → redis
         worker → postgres, redis
```

| Network    | Services                     |
| ---------- | ---------------------------- |
| `frontend` | nginx, app                   |
| `backend`  | app, postgres, redis, worker |

## Medium Stack

**~15 services, 4 networks, 5 volumes** — a mid-size e-commerce platform with microservices, async workers, and a message queue.

| Network     | Tier        | Services                              |
| ----------- | ----------- | ------------------------------------- |
| `public`    | Edge        | nginx, web-app, api-gateway           |
| `api`       | Application | auth, product, order, notification    |
| `data`      | Storage     | postgres, redis, elasticsearch, minio |
| `messaging` | Messaging   | rabbitmq, order-worker, email-worker  |

## Large Stack

**~46 services, 7 networks, 19 volumes** — a full production-like SaaS platform across 6 tiers.

| Network      | Tier          | Services                                                                                             |
| ------------ | ------------- | ---------------------------------------------------------------------------------------------------- |
| `public`     | Edge          | nginx-lb, web-app, admin-panel, api-gateway                                                          |
| `api`        | Application   | auth, user, product, order, payment, inventory, notification, search, media, pricing, recommendation |
| `workers`    | Background    | order-worker, email-worker, invoice-worker, analytics-worker, search-indexer, report-scheduler       |
| `data`       | Storage       | postgres (primary + replica), redis (cache + sessions), mongo, elasticsearch, minio, vault           |
| `messaging`  | Messaging     | rabbitmq, kafka, zookeeper                                                                           |
| `monitoring` | Observability | prometheus, grafana, loki, tempo, alertmanager, node-exporter, cadvisor                              |
| `ci`         | CI/CD         | ci-server, ci-runner-1, ci-runner-2, artifact-store, ci-postgres, ci-redis                           |

### Profiled Services

Some services in the large stack are behind Compose profiles:

```bash
docker compose -f demo/compose-large.yml --profile ops up -d         # db-backup
docker compose -f demo/compose-large.yml --profile staging up -d     # staging-proxy
docker compose -f demo/compose-large.yml --profile testing up -d     # load-tester
```

## Stress Test

**~700 services, 10 networks, 5 volumes** — a synthetic compose file for testing frontend rendering performance at scale.

> **Do not start this stack.** It exists only as a compose file for the backend to parse. To test, mount it as the compose file source and let DockGraph parse it — no containers need to be running.

## Resource Usage

- Application services use `busybox` with `sleep infinity` — near-zero CPU and memory.
- Infrastructure services (postgres, redis, prometheus, grafana, rabbitmq) use real images for realistic health checks.
- Estimated memory: small ~200 MB, medium ~500 MB, large ~1.5 GB.

## Experimenting

Try these to see DockGraph react in real time:

```bash
# Stop a service and watch it turn grey
docker stop demo-medium-order-service-1

# Scale up a worker
docker compose -f demo/compose-medium.yml up -d --scale email-worker=3

# Take down a tier in the large stack
docker compose -f demo/compose-large.yml stop kafka zookeeper
```

# Demo: Acme Platform

A simulated large-scale SaaS platform (~40 services) for showcasing Docker Flow.

## Architecture

**6 isolated networks, 15 volumes, 38 services across 5 tiers:**

| Network      | Services                                                                              |
|--------------|---------------------------------------------------------------------------------------|
| `public`     | nginx-lb, web-app, admin-panel, api-gateway                                           |
| `api`        | auth, user, product, order, payment, inventory, notification, search, media, pricing, recommendation |
| `workers`    | order-worker, email-worker, invoice-worker, analytics-worker, search-indexer, report-scheduler |
| `data`       | postgres (primary + replica), redis (cache + sessions), mongo, elasticsearch, minio, vault |
| `messaging`  | rabbitmq, kafka, zookeeper                                                            |
| `monitoring` | prometheus, grafana, loki, tempo, alertmanager, node-exporter, cadvisor                |

## What it demonstrates

| Feature                  | How it appears                                                          |
|--------------------------|-------------------------------------------------------------------------|
| **Network isolation**    | 6 colored network groups with clear tier separation                     |
| **Multi-network**        | api-gateway (public + api), order-service (api + messaging), prometheus (monitoring + api), etc. |
| **Deep dependency chains** | nginx-lb → api-gateway → order-service → postgres, rabbitmq            |
| **Shared databases**     | Multiple API services all depending on postgres + redis                 |
| **Volume mounts**        | 15 named volumes across data and monitoring tiers                       |
| **Message queues**       | Workers connected to rabbitmq/kafka via messaging network               |
| **Animated traffic**     | depends_on edges show animated dots for ~30 dependency relationships    |
| **Exposed ports**        | nginx :80/:443, grafana :3000, rabbitmq :15672, minio :9001            |

## Usage

```bash
# Start the demo stack
docker compose -f demo/docker-compose.yml up -d

# Start Docker Flow to visualize it
docker compose up -d

# Open http://localhost:7800

# Tear down when done
docker compose -f demo/docker-compose.yml down -v
```

## Notes

- Application services use `busybox` (sleep infinity) — near-zero resource usage.
- Postgres, Redis, Prometheus, Grafana, and RabbitMQ use real images for realistic status data.
- Stop individual services to see stopped-state visualization in the graph.

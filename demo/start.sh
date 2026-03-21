#!/bin/sh
set -e

COMPOSE_FILE="$(dirname "$0")/docker-compose.yml"
PROJECT="acme-platform"

# Start all default-profile services
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" up -d

# Create profiled services without starting them (shows "created" state)
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" --profile with-indexer create search-indexer

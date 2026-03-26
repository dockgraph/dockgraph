#!/bin/sh
set -e

DIR="$(dirname "$0")"

echo "Starting small demo (5 services)..."
docker compose -f "$DIR/compose-small.yml" up -d

echo "Starting medium demo (~15 services)..."
docker compose -f "$DIR/compose-medium.yml" up -d

echo "Starting large demo (~46 services)..."
docker compose -f "$DIR/compose-large.yml" up -d

# Create profiled services without starting them (shows "created" state)
docker compose -f "$DIR/compose-large.yml" --profile with-indexer create search-indexer

echo ""
echo "All demos running. Open http://localhost:7800 to visualize."

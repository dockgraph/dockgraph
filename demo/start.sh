#!/bin/sh
set -e

DIR="$(dirname "$0")"
MODE="${1:-compose}"

usage() {
  echo "Usage: $0 [compose|swarm]"
  echo "  compose  Start the small, medium and large compose demos (default)"
  echo "  swarm    Deploy the small and medium swarm demo stacks"
}

start_compose() {
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
}

start_swarm() {
  state="$(docker info --format '{{.Swarm.LocalNodeState}}')"
  manager="$(docker info --format '{{.Swarm.ControlAvailable}}')"
  if [ "$state" != "active" ] || [ "$manager" != "true" ]; then
    echo "This node is not a swarm manager. Run 'docker swarm init' first." >&2
    exit 1
  fi

  # External overlay network shared by both demo stacks
  docker network inspect demo_shared >/dev/null 2>&1 ||
    docker network create -d overlay --attachable demo_shared

  echo "Deploying small swarm stack (6 services)..."
  docker stack deploy -c "$DIR/stack-small.yml" swarm-small

  echo "Deploying medium swarm stack (14 services)..."
  docker stack deploy -c "$DIR/stack-medium.yml" swarm-medium

  echo ""
  echo "Swarm demos deployed. Deploy DockGraph with 'make swarm-deploy'"
  echo "(see README.md#docker-swarm), then open http://localhost:7800"
  echo "and pick a stack from the selector."
}

case "$MODE" in
  compose) start_compose ;;
  swarm) start_swarm ;;
  -h | --help) usage ;;
  *)
    usage >&2
    exit 1
    ;;
esac

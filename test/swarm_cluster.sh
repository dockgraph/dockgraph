#!/usr/bin/env bash
# Multi-node swarm test cluster built from Docker-in-Docker containers.
#
# Starts MANAGERS + WORKERS docker:dind containers on a dedicated bridge
# network, joins them into one swarm, loads the locally built DockGraph image
# into every node, and deploys stack.yml (server + global agent) plus the test
# workload in test/cluster/workload.yml. The scenarios in backend/clustertest
# then drive it (pause nodes, drain, rotate secrets...).
#
# Unlike test/swarm_smoke.sh this exercises the server <-> agent path: the
# server never proxies to the agent of its own node, so a single-node swarm
# cannot test remote stats, logs or inspect.
#
# Requirements and side effects:
#   - A rootful Docker daemon. Each node runs with --privileged (Docker in
#     Docker needs it), which is root-equivalent on the host: run this only
#     on a machine you control. With rootless Docker installed alongside,
#     prefix the commands with DOCKER_CONTEXT=default.
#   - About 2 GB of memory for the default 5 nodes. docker:dind and the
#     workload images are pulled once from Docker Hub.
#   - Only resources labelled dockgraph.cluster are created or removed;
#     nothing else on the daemon is touched.
#   - Tested on Linux and WSL2.
#
# Usage: test/swarm_cluster.sh <command> [args]
#   up [--skip-build]   create the cluster and deploy (idempotent per node)
#   down                remove every node and the cluster network
#   status              show nodes, services and the UI URLs
#   reload              rebuild the image, load it into every node and
#                       force-update the DockGraph services
#   add-node <name> [worker|manager] [hostname]
#                       start one more node and join it (hostname defaults
#                       to name; pass an existing one to test duplicates)
#   load <node>         load the test images into one node
#
# Env: MANAGERS (3), WORKERS (2, at least 2), IMAGE (dockgraph:cluster-test),
#      DIND_IMAGE (docker:28-dind), UI_PORT (17900; manager N publishes the
#      UI on 127.0.0.1:UI_PORT+N), SUBNET (172.31.231.0/24, the cluster
#      network; change it if it overlaps an existing network)
set -euo pipefail

cd "$(dirname "$0")/.."

# Git Bash on Windows rewrites arguments that look like paths (/stack.yml);
# they are paths inside the nodes, so keep them as written.
export MSYS_NO_PATHCONV=1

MANAGERS="${MANAGERS:-3}"
WORKERS="${WORKERS:-2}"
IMAGE="${IMAGE:-dockgraph:cluster-test}"
DIND_IMAGE="${DIND_IMAGE:-docker:28-dind}"
UI_PORT="${UI_PORT:-17900}"
SUBNET="${SUBNET:-172.31.231.0/24}"
PREFIX="dgc"
NET="dgc-net"
LABEL="dockgraph.cluster=dgc"
LEADER="$PREFIX-m1"
STACK="dg"
WORKLOAD_STACK="work"
SECRET="dg_agent_token"
WORKLOAD_IMAGES=(busybox:1.37 nginx:1.29-alpine)

# ── Helpers ───────────────────────────────────────────────────

die() {
  echo "ERROR: $*" >&2
  exit 1
}

usage() {
  # The header comment block, without the shebang and the leading "# ".
  awk 'NR == 1 { next } /^#/ { sub(/^# ?/, ""); print; next } { exit }' "$0"
}

# on <node> <docker args...>: run a docker command on a cluster node.
on() {
  local node="$1"; shift
  docker exec "$node" docker "$@"
}

managers() { for i in $(seq 1 "$MANAGERS"); do echo "$PREFIX-m$i"; done; }
workers() { for i in $(seq 1 "$WORKERS"); do echo "$PREFIX-w$i"; done; }

# running_managers: the manager nodes that exist, whatever MANAGERS says now.
running_managers() {
  docker ps --filter "label=$LABEL" --format '{{.Names}}' | grep -E "^$PREFIX-m[0-9]+$" | sort -t m -k 2 -n
}

node_ip() {
  docker inspect -f "{{(index .NetworkSettings.Networks \"$NET\").IPAddress}}" "$1"
}

require_rootful() {
  docker info >/dev/null 2>&1 || die "cannot reach the Docker daemon"
  if docker info --format '{{.SecurityOptions}}' | grep -q rootless; then
    die "the active Docker daemon is rootless; use a rootful one (e.g. DOCKER_CONTEXT=default)"
  fi
}

# require_count <name> <value> <min>: fail unless value is an integer >= min.
require_count() {
  case "$2" in
    ''|*[!0-9]*) die "$1 must be a number, got '$2'" ;;
  esac
  [ "$2" -ge "$3" ] || die "$1 must be at least $3, got $2"
}

# wait_daemon <node>: wait until the node's inner dockerd answers.
wait_daemon() {
  for _ in $(seq 1 60); do
    on "$1" info >/dev/null 2>&1 && return 0
    sleep 1
  done
  docker logs --tail 30 "$1" >&2 || true
  die "dockerd in $1 did not start"
}

# start_node <name> <hostname> [docker run args...]: start a dind node
# unless it already exists.
start_node() {
  local name="$1" hostname="$2"; shift 2
  if docker inspect "$name" >/dev/null 2>&1; then
    return 0
  fi
  echo "Starting node $name (hostname $hostname)..."
  # No TLS and only the unix socket: the nodes are driven via docker exec,
  # so no daemon port is exposed on the cluster network.
  docker run -d --privileged --name "$name" --hostname "$hostname" \
    --network "$NET" --label "$LABEL" -e DOCKER_TLS_CERTDIR= "$@" \
    "$DIND_IMAGE" --host=unix:///var/run/docker.sock >/dev/null
}

# load_images <node>: copy the DockGraph and workload images into a node so
# nothing is pulled from inside the cluster.
load_images() {
  echo "Loading images into $1..."
  docker save "$IMAGE" "${WORKLOAD_IMAGES[@]}" | docker exec -i "$1" docker load -q >/dev/null
}

# join <node> <worker|manager>: join a node to the swarm led by the first
# manager unless it is already a member.
join() {
  local node="$1" role="$2" token
  if [ "$(on "$node" info --format '{{.Swarm.LocalNodeState}}')" = "active" ]; then
    return 0
  fi
  token=$(on "$LEADER" swarm join-token -q "$role")
  on "$node" swarm join --advertise-addr "$(node_ip "$node")" \
    --token "$token" "$(node_ip "$LEADER"):2377" >/dev/null
  echo "$node joined as $role"
}

# wait_converged <stack>: wait until every service of the stack runs its
# desired number of tasks. Jobs ("0/3 (3/3 completed)") are skipped: once
# finished they run nothing by design.
wait_converged() {
  local stack="$1" pending
  for _ in $(seq 1 90); do
    pending=$(on "$LEADER" service ls --filter "label=com.docker.stack.namespace=$stack" \
      --format '{{.Replicas}}' | grep -v completed | awk -F'[/ ]' '$1 != $2' | wc -l | tr -d ' ')
    if [ "$pending" = "0" ]; then
      echo "Stack $stack converged"
      return 0
    fi
    sleep 2
  done
  on "$LEADER" service ls --filter "label=com.docker.stack.namespace=$stack" >&2
  die "stack $stack did not converge"
}

# ── Commands ──────────────────────────────────────────────────

cmd_up() {
  local skip_build=false
  for arg in "$@"; do
    case "$arg" in
      --skip-build) skip_build=true ;;
      *) die "unknown argument: $arg" ;;
    esac
  done
  require_count MANAGERS "$MANAGERS" 1
  # The workload pins a service to dgc-w1, and the scenarios break a second
  # worker while the first keeps running.
  require_count WORKERS "$WORKERS" 2

  if [ "$skip_build" = false ]; then
    echo "Building $IMAGE..."
    docker build -q -t "$IMAGE" . >/dev/null
  fi
  docker image inspect "$IMAGE" >/dev/null 2>&1 || die "image $IMAGE not found (drop --skip-build)"
  for img in "${WORKLOAD_IMAGES[@]}"; do
    docker image inspect "$img" >/dev/null 2>&1 || docker pull -q "$img" >/dev/null
  done

  # An explicit subnet lets a partition test reconnect a node with its old
  # IP, which its swarm advertise address is bound to.
  if ! docker network inspect "$NET" >/dev/null 2>&1; then
    docker network create --label "$LABEL" --subnet "$SUBNET" "$NET" >/dev/null ||
      die "cannot create network $NET on $SUBNET; if it overlaps an existing network, pick a free range with SUBNET=..."
  fi

  local i=1
  for m in $(managers); do
    start_node "$m" "$m" -p "127.0.0.1:$((UI_PORT + i)):7800"
    i=$((i + 1))
  done
  for w in $(workers); do start_node "$w" "$w"; done

  for n in $(managers) $(workers); do wait_daemon "$n"; done

  if [ "$(on "$LEADER" info --format '{{.Swarm.LocalNodeState}}')" != "active" ]; then
    on "$LEADER" swarm init --advertise-addr "$(node_ip "$LEADER")" >/dev/null
    echo "Swarm initialised on $LEADER"
  fi
  for m in $(managers | tail -n +2); do join "$m" manager; done
  for w in $(workers); do join "$w" worker; done

  # Loading in parallel: each node is a separate daemon.
  local pids=()
  for n in $(managers) $(workers); do
    load_images "$n" & pids+=($!)
  done
  for p in "${pids[@]}"; do wait "$p"; done

  # The token is generated inside the leader, so the host needs no openssl.
  if ! on "$LEADER" secret inspect "$SECRET" >/dev/null 2>&1; then
    docker exec "$LEADER" sh -c \
      "head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | docker secret create $SECRET -" >/dev/null
  fi

  docker cp stack.yml "$LEADER:/stack.yml"
  docker cp test/cluster/workload.yml "$LEADER:/workload.yml"
  docker exec -e DG_IMAGE="$IMAGE" "$LEADER" \
    docker stack deploy --detach=true --resolve-image never -c /stack.yml "$STACK" >/dev/null
  on "$LEADER" stack deploy --detach=true --resolve-image never -c /workload.yml "$WORKLOAD_STACK" >/dev/null
  wait_converged "$STACK"
  wait_converged "$WORKLOAD_STACK"
  cmd_status
}

cmd_down() {
  local ids
  ids=$(docker ps -aq --filter "label=$LABEL")
  if [ -n "$ids" ]; then
    echo "Removing cluster nodes..."
    # -v drops each node's anonymous /var/lib/docker volume.
    # shellcheck disable=SC2086
    docker rm -fv $ids >/dev/null
  fi
  if docker network inspect "$NET" >/dev/null 2>&1; then
    docker network rm "$NET" >/dev/null
  fi
  echo "Cluster removed"
}

cmd_status() {
  docker inspect "$LEADER" >/dev/null 2>&1 || die "no cluster running (start it with: $0 up)"
  on "$LEADER" node ls
  echo ""
  on "$LEADER" service ls
  echo ""
  # The published ports, as created, whatever UI_PORT says now.
  for m in $(running_managers); do
    echo "UI via $m: http://$(docker port "$m" 7800 | head -n 1)"
  done
}

cmd_reload() {
  echo "Rebuilding $IMAGE..."
  docker build -q -t "$IMAGE" . >/dev/null
  local pids=()
  for n in $(docker ps --filter "label=$LABEL" --format '{{.Names}}'); do
    load_images "$n" & pids+=($!)
  done
  for p in "${pids[@]}"; do wait "$p"; done
  for svc in server agent; do
    on "$LEADER" service update --force --detach=false --quiet "${STACK}_$svc" >/dev/null
  done
  wait_converged "$STACK"
}

cmd_add_node() {
  local name="${1:?usage: add-node <name> [worker|manager] [hostname]}"
  local role="${2:-worker}" hostname="${3:-${1}}"
  case "$role" in worker|manager) ;; *) die "role must be worker or manager" ;; esac
  start_node "$name" "$hostname"
  wait_daemon "$name"
  load_images "$name"
  join "$name" "$role"
}

command="${1:-}"
shift || true
case "$command" in
  -h|--help|"") usage; exit 0 ;;
esac
require_rootful
case "$command" in
  up) cmd_up "$@" ;;
  down) cmd_down ;;
  status) cmd_status ;;
  reload) cmd_reload ;;
  add-node) cmd_add_node "$@" ;;
  load) load_images "${1:?usage: load <node>}" ;;
  *) die "unknown command: $command" ;;
esac

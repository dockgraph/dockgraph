#!/usr/bin/env bash
# Swarm end-to-end smoke test (opt-in, not run in CI).
#
# Deploys stack.yml (server + global agent) and a small throwaway demo stack
# on the local Docker daemon, checks the HTTP API, the WebSocket snapshot,
# the per-node (swarm node view) graph nodes and stats, and password loading
# from a Docker secret (DG_PASSWORD_FILE), then tears everything down.
#
# If the daemon is not in a swarm, the script refuses to run unless --yes is
# given; it then runs `docker swarm init` and `docker swarm leave --force` at
# the end. An existing swarm is never left; only the resources this script
# created are removed. The demo stack publishes no ports, so it can run next
# to the swarm-small / swarm-medium demo stacks. Intended for a single-node
# swarm: the locally built image is not available on other nodes.
#
# Usage: test/swarm_smoke.sh [--yes] [--skip-build]
#   --yes         allow initialising (and afterwards leaving) a swarm
#   --skip-build  reuse an existing $IMAGE
#
# Env: SMOKE_PORT (default 17800), IMAGE (default dockgraph:swarm-smoke)
set -euo pipefail

cd "$(dirname "$0")/.."

IMAGE="${IMAGE:-dockgraph:swarm-smoke}"
SMOKE_PORT="${SMOKE_PORT:-17800}"
STACK="dgsmoke"
DEMO_STACK="dgsmoke-demo"
SECRET="dg_agent_token"
PW_SECRET="dgsmoke_password"
BASE_URL="http://localhost:$SMOKE_PORT"

ASSUME_YES=false
SKIP_BUILD=false
for arg in "$@"; do
  case "$arg" in
    --yes) ASSUME_YES=true ;;
    --skip-build) SKIP_BUILD=true ;;
    -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

INITIALISED_SWARM=false
CREATED_SECRET=false
CREATED_PW_SECRET=false
TMP_DIR=$(mktemp -d)

# ── Helpers ───────────────────────────────────────────────────

fail() {
  echo "FAIL: $*"
  echo "--- server logs ---"
  docker service logs --tail 50 "${STACK}_server" 2>/dev/null || true
  echo "--- agent logs ---"
  docker service logs --tail 20 "${STACK}_agent" 2>/dev/null || true
  exit 1
}

# wait_stack_removed <stack>: docker stack rm is asynchronous; wait until
# its services and networks are gone so secrets/networks can be removed.
wait_stack_removed() {
  local stack="$1"
  for _ in $(seq 1 30); do
    if [ -z "$(docker service ls -q --filter "label=com.docker.stack.namespace=$stack")" ] &&
       [ -z "$(docker network ls -q --filter "label=com.docker.stack.namespace=$stack")" ]; then
      return 0
    fi
    sleep 2
  done
  echo "WARN: stack $stack still being removed"
}

cleanup() {
  echo ""
  echo "Cleaning up..."
  rm -rf "$TMP_DIR"
  docker stack rm "$DEMO_STACK" >/dev/null 2>&1 || true
  docker stack rm "$STACK" >/dev/null 2>&1 || true
  if [ "$INITIALISED_SWARM" = true ]; then
    echo "Leaving the swarm this script initialised..."
    docker swarm leave --force >/dev/null 2>&1 || true
    return
  fi
  wait_stack_removed "$DEMO_STACK"
  wait_stack_removed "$STACK"
  if [ "$CREATED_SECRET" = true ]; then
    docker secret rm "$SECRET" >/dev/null 2>&1 || true
  fi
  if [ "$CREATED_PW_SECRET" = true ]; then
    docker secret rm "$PW_SECRET" >/dev/null 2>&1 || true
  fi
}

# wait_converged <stack>: wait until every service of the stack runs its
# desired number of tasks (e.g. "2/2").
wait_converged() {
  local stack="$1" pending
  for _ in $(seq 1 60); do
    pending=$(docker service ls --filter "label=com.docker.stack.namespace=$stack" \
      --format '{{.Replicas}}' | awk -F'[/ ]' '$1 != $2' | wc -l | tr -d ' ')
    if [ "$pending" = "0" ] && [ -n "$(docker service ls -q --filter "label=com.docker.stack.namespace=$stack")" ]; then
      echo "Stack $stack converged"
      return 0
    fi
    sleep 2
  done
  docker service ls --filter "label=com.docker.stack.namespace=$stack"
  fail "stack $stack did not converge"
}

# wait_healthy: wait until the server answers /healthz.
wait_healthy() {
  local attempts=1
  until curl -s "$BASE_URL/healthz" | grep -q ok; do
    [ "$attempts" -ge 30 ] && fail "health check did not pass after $attempts attempts"
    sleep 2
    attempts=$((attempts + 1))
  done
  echo "Health check: OK ($attempts attempt(s))"
}

# http_code <url> [curl args...]: print the HTTP status of a GET.
http_code() {
  local url="$1"; shift
  curl -s -o /dev/null -w "%{http_code}" "$@" "$url" || true
}

# assert_http <label> <url> <expected_code> [curl args...]
assert_http() {
  local label="$1" url="$2" expected="$3" code
  shift 3
  code=$(http_code "$url" "$@")
  [ "$code" = "$expected" ] || fail "$label — expected $expected, got $code"
  echo "$label: OK ($code)"
}

# wait_body_match <label> <url> <pattern> [tries]: poll url until its body
# contains the fixed string pattern.
wait_body_match() {
  local label="$1" url="$2" pattern="$3" tries="${4:-30}" body=""
  for _ in $(seq 1 "$tries"); do
    body=$(curl -s "$url" || true)
    if grep -q -F -- "$pattern" <<<"$body"; then
      echo "$label: OK"
      return 0
    fi
    sleep 2
  done
  fail "$label — '$pattern' not found in $url: ${body:0:300}"
}

# ws_snapshot <file>: capture the first seconds of the WebSocket stream (the
# initial snapshot, as raw frames) into file.
ws_snapshot() {
  curl -s -N --max-time 3 -o "$1" \
    -H "Upgrade: websocket" \
    -H "Connection: Upgrade" \
    -H "Sec-WebSocket-Version: 13" \
    -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
    "$BASE_URL/ws" || true
}

# ── Swarm state ───────────────────────────────────────────────

echo "=== DockGraph Swarm Smoke Test ==="

state=$(docker info --format '{{.Swarm.LocalNodeState}}')
case "$state" in
  active)
    if [ "$(docker info --format '{{.Swarm.ControlAvailable}}')" != "true" ]; then
      echo "This node is a swarm worker; run the smoke test on a manager." >&2
      exit 1
    fi
    echo "Using the existing swarm (it will NOT be left afterwards)."
    echo "Stacks $STACK and $DEMO_STACK will be deployed and removed; UI port $SMOKE_PORT."
    ;;
  inactive)
    if [ "$ASSUME_YES" != true ]; then
      echo "WARNING: this Docker daemon is not part of a swarm." >&2
      echo "The smoke test needs to run 'docker swarm init' and, at the end," >&2
      echo "'docker swarm leave --force'. Re-run with --yes to allow this." >&2
      exit 1
    fi
    echo "WARNING: initialising a single-node swarm (will leave it at the end)..."
    docker swarm init >/dev/null 2>&1 || docker swarm init --advertise-addr 127.0.0.1 >/dev/null
    INITIALISED_SWARM=true
    ;;
  *)
    echo "Unexpected swarm state '$state'; aborting." >&2
    exit 1
    ;;
esac
trap cleanup EXIT

for stack in "$STACK" "$DEMO_STACK"; do
  if [ -n "$(docker service ls -q --filter "label=com.docker.stack.namespace=$stack")" ]; then
    echo "Stack $stack already exists (leftover run?); remove it first: docker stack rm $stack" >&2
    trap - EXIT
    rm -rf "$TMP_DIR"
    exit 1
  fi
done

NODE_HOST=$(docker node inspect self --format '{{.Description.Hostname}}')
echo "Local swarm node: $NODE_HOST"

# ── Build ─────────────────────────────────────────────────────

if [ "$SKIP_BUILD" = false ]; then
  echo "Building image $IMAGE..."
  docker build -t "$IMAGE" .
fi

# ── DG_PASSWORD_FILE: unreadable or empty file is fatal ───────
# LoadConfig runs before --healthcheck, so this exits without a server.
# /dev/null exists in the image and is an empty file.

out=$(docker run --rm -e DG_PASSWORD_FILE=/nonexistent/password "$IMAGE" --healthcheck 2>&1) &&
  fail "unreadable DG_PASSWORD_FILE did not abort startup"
grep -q "cannot read DG_PASSWORD_FILE" <<<"$out" ||
  fail "unreadable DG_PASSWORD_FILE: unexpected output: $out"
echo "Unreadable DG_PASSWORD_FILE is fatal: OK"

out=$(docker run --rm -e DG_PASSWORD_FILE=/dev/null "$IMAGE" --healthcheck 2>&1) &&
  fail "empty DG_PASSWORD_FILE did not abort startup"
grep -q "DG_PASSWORD_FILE is empty" <<<"$out" ||
  fail "empty DG_PASSWORD_FILE: unexpected output: $out"
echo "Empty DG_PASSWORD_FILE is fatal: OK"

# ── Deploy DockGraph ──────────────────────────────────────────

if ! docker secret inspect "$SECRET" >/dev/null 2>&1; then
  echo "Creating secret $SECRET..."
  openssl rand -hex 32 | docker secret create "$SECRET" - >/dev/null
  CREATED_SECRET=true
fi

echo "Deploying $STACK..."
DG_IMAGE="$IMAGE" DG_HTTP_PORT="$SMOKE_PORT" \
  docker stack deploy --resolve-image never -c stack.yml "$STACK" >/dev/null
wait_converged "$STACK"

# ── Server assertions ─────────────────────────────────────────

echo "Waiting for $BASE_URL/healthz..."
wait_healthy

info=$(curl -s "$BASE_URL/api/system/info")
echo "$info" | grep -q '"mode":"swarm"' || fail "system info mode is not swarm: $info"
echo "System info mode=swarm: OK"

# Probe each container's own healthcheck (the agent serves /healthz on its
# unpublished port, reachable only from inside).
for svc in server agent; do
  cid=$(docker ps -q --filter "label=com.docker.swarm.service.name=${STACK}_${svc}" | head -n1)
  [ -n "$cid" ] || fail "no local ${svc} container"
  docker exec "$cid" /dockgraph --healthcheck || fail "${svc} healthcheck failed"
  echo "${svc} healthcheck: OK"
done

# ── Node view: per-node stats ─────────────────────────────────
# Checked before the demo stack exists: on a fresh swarm the local agent then
# samples no container (DockGraph's own are skipped), and the node must still
# report an aggregate instead of showing "no agent".

wait_body_match "Node stats aggregate node:$NODE_HOST" \
  "$BASE_URL/api/stats/history?range=5m&scope=nodes" "\"node:$NODE_HOST\""
assert_http "scope=nodes with stack rejected" \
  "$BASE_URL/api/stats/history?scope=nodes&stack=$DEMO_STACK" 400
assert_http "Unknown scope rejected" "$BASE_URL/api/stats/history?scope=bogus" 400
if curl -s "$BASE_URL/api/stats/history?range=5m" | grep -q '"node:'; then
  fail "workload stats history leaks node:* series"
fi
echo "Workload history excludes node:* series: OK"

# ── Demo stack ────────────────────────────────────────────────
# Throwaway stack without published ports: a replicated service, a global
# (per-node) service and a second service sharing its overlay network.

cat >"$TMP_DIR/demo.yml" <<'EOF'
networks:
  mesh:
    driver: overlay
services:
  web:
    image: nginx:alpine
    networks: [mesh]
    deploy:
      replicas: 2
  api:
    image: busybox
    command: sh -c "sleep infinity"
    networks: [mesh]
  log-shipper:
    image: busybox
    command: sh -c "sleep infinity"
    networks: [mesh]
    deploy:
      mode: global
EOF

echo "Deploying $DEMO_STACK..."
docker stack deploy -c "$TMP_DIR/demo.yml" "$DEMO_STACK" >/dev/null
wait_converged "$DEMO_STACK"

web_id=$(docker service inspect --format '{{.ID}}' "${DEMO_STACK}_web")
assert_http "Service inspect" "$BASE_URL/api/services/$web_id" 200
assert_http "Stack-scoped stats" "$BASE_URL/api/stats/history?stack=$DEMO_STACK" 200
wait_body_match "Stack-scoped stats list ${DEMO_STACK}_web" \
  "$BASE_URL/api/stats/history?range=5m&stack=$DEMO_STACK" "\"${DEMO_STACK}_web"
assert_http "Node view page (?group=node)" "$BASE_URL/?group=node" 200

snapshot="$TMP_DIR/snapshot"
found=false
for _ in $(seq 1 15); do
  ws_snapshot "$snapshot"
  if grep -a -q "service:${DEMO_STACK}_web" "$snapshot" &&
     grep -a -q "service:${DEMO_STACK}_log-shipper" "$snapshot"; then
    found=true
    break
  fi
  sleep 2
done
[ "$found" = true ] || fail "WebSocket snapshot lacks ${DEMO_STACK} services"
echo "WebSocket snapshot lists demo services: OK"

if grep -a -q "service:${STACK}_" "$snapshot"; then
  fail "WebSocket snapshot includes DockGraph's own services (self-exclusion)"
fi
echo "Self-exclusion: OK"

# ── Node view: swarm node graph nodes and task placement ──────

grep -a -q "\"id\":\"swarmnode:$NODE_HOST\"" "$snapshot" ||
  fail "WebSocket snapshot lacks swarmnode:$NODE_HOST"
echo "Swarm node graph node swarmnode:$NODE_HOST: OK"

expected_nodes=$(docker node ls --format '{{.Hostname}}' | sort -u | wc -l | tr -d ' ')
actual_nodes=$(grep -a -o '"id":"swarmnode:[^"]*"' "$snapshot" | sort -u | wc -l | tr -d ' ')
[ "$expected_nodes" = "$actual_nodes" ] ||
  fail "expected $expected_nodes swarmnode graph nodes, got $actual_nodes"
echo "One swarmnode per cluster hostname ($actual_nodes): OK"

grep -a -q '"role":"manager"' "$snapshot" || fail "no swarm node with role manager"
grep -a -q '"leader":true' "$snapshot" || fail "no swarm node flagged as leader"
echo "Manager role and leader flag: OK"

grep -a -q "\"nodeHostname\":\"$NODE_HOST\"" "$snapshot" ||
  fail "no task placed on $NODE_HOST (nodeHostname) in the snapshot"
echo "Task placement by node hostname: OK"

# ── DG_PASSWORD_FILE from a Docker secret ─────────────────────

password=$(openssl rand -hex 16)
if docker secret inspect "$PW_SECRET" >/dev/null 2>&1; then
  fail "secret $PW_SECRET already exists (leftover run?); remove it first"
fi
printf '%s' "$password" | docker secret create "$PW_SECRET" - >/dev/null
CREATED_PW_SECRET=true

echo "Enabling DG_PASSWORD_FILE on ${STACK}_server..."
docker service update --detach=false --quiet \
  --secret-add "source=$PW_SECRET,target=dg_password" \
  --env-add "DG_PASSWORD_FILE=/run/secrets/dg_password" \
  "${STACK}_server" >/dev/null
wait_healthy

assert_http "API without session" "$BASE_URL/api/system/info" 401
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" \
  -d '{"password":"wrong"}' "$BASE_URL/api/login" || true)
[ "$code" = "401" ] || fail "login with wrong password — expected 401, got $code"
echo "Login with wrong password: OK ($code)"

jar="$TMP_DIR/cookies"
code=$(curl -s -o /dev/null -w "%{http_code}" -c "$jar" -X POST -H "Content-Type: application/json" \
  -d "{\"password\":\"$password\"}" "$BASE_URL/api/login" || true)
[ "$code" = "200" ] || fail "login with secret password — expected 200, got $code"
echo "Login with password from secret: OK ($code)"
assert_http "API with session" "$BASE_URL/api/system/info" 200 -b "$jar"

echo ""
echo "=== All swarm smoke tests passed ==="

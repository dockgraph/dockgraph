#!/usr/bin/env bash
set -euo pipefail

CONTAINER_NAME="dockgraph-test"
HOST_PORT=7801
BASE_URL="http://localhost:$HOST_PORT"

# ── Helpers ───────────────────────────────────────────────────

cleanup() {
  echo "Cleaning up..."
  docker stop "$CONTAINER_NAME" 2>/dev/null || true
  docker rm "$CONTAINER_NAME" 2>/dev/null || true
}

# Assert that an HTTP request returns the expected status code.
#   assert_http <label> <url> <expected_code> [extra_curl_args...]
assert_http() {
  local label="$1" url="$2" expected="$3"
  shift 3
  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" "$@" "$url" || true)
  if [ "$code" != "$expected" ]; then
    echo "FAIL: $label — expected $expected, got $code"
    exit 1
  fi
  echo "$label: OK ($code)"
}

# ── Socket detection ─────────────────────────────────────────

# The Docker socket location varies by setup:
#   - DOCKER_HOST env var (explicit override, e.g. "unix:///custom/path")
#   - docker context (WSL2 rootless uses /mnt/wslg/runtime-dir/docker.sock)
#   - /var/run/docker.sock (standard Linux/macOS default)
# The ${var#unix://} syntax strips the "unix://" prefix to get a bare path.
if [ -n "${DOCKER_HOST:-}" ]; then
  DOCKER_SOCK="${DOCKER_HOST#unix://}"
else
  DOCKER_SOCK="$(docker context inspect --format '{{.Endpoints.docker.Host}}' 2>/dev/null || true)"
  DOCKER_SOCK="${DOCKER_SOCK#unix://}"
fi
DOCKER_SOCK="${DOCKER_SOCK:-/var/run/docker.sock}"

# ── Build & run ──────────────────────────────────────────────

SKIP_BUILD=false
if [ "${1:-}" = "--skip-build" ]; then
  SKIP_BUILD=true
fi

echo "=== DockGraph Smoke Test ==="

if [ "$SKIP_BUILD" = false ]; then
  echo "Building image..."
  docker build -t dockgraph:test .
fi

# Set up cleanup before starting so a failed `docker run` still cleans up.
trap cleanup EXIT

echo "Starting container (socket: $DOCKER_SOCK)..."
docker run -d --name "$CONTAINER_NAME" \
  -p "$HOST_PORT":7800 \
  -v "$DOCKER_SOCK":/var/run/docker.sock:ro \
  dockgraph:test

# ── Assertions ───────────────────────────────────────────────

# Poll the health endpoint until the server is ready (up to 10 seconds).
# The binary needs a moment to connect to the Docker daemon and start listening.
echo "Waiting for startup..."
attempts=0
until curl -s "$BASE_URL/healthz" | grep -q ok; do
  attempts=$((attempts + 1))
  if [ "$attempts" -ge 10 ]; then
    echo "FAIL: health check did not pass after ${attempts}s"
    docker logs "$CONTAINER_NAME"
    exit 1
  fi
  sleep 1
done
echo "Health check passed"

assert_http "Static file serving" "$BASE_URL/" 200

# Verify WebSocket upgrade with the required headers per RFC 6455.
# The Sec-WebSocket-Key is a static base64 value — the server only checks
# that the header is present, not its content.
# --max-time 5: after the 101 upgrade, the connection stays open (it's a
# WebSocket). curl doesn't speak WebSocket framing, so it hangs reading
# binary data. The 101 status is captured from headers immediately;
# --max-time just caps how long curl sits on the open connection.
assert_http "WebSocket upgrade" "$BASE_URL/ws" 101 \
  --max-time 5 \
  -H "Upgrade: websocket" \
  -H "Connection: Upgrade" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ=="

echo ""
echo "=== All smoke tests passed ==="

#!/usr/bin/env bash
set -euo pipefail

echo "=== DockGraph Smoke Test ==="

echo "Building image..."
docker build -t dockgraph:test .

echo "Starting container..."
docker run -d --name dockgraph-test \
  -p 7801:7800 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  dockgraph:test

cleanup() {
  echo "Cleaning up..."
  docker stop dockgraph-test 2>/dev/null || true
  docker rm dockgraph-test 2>/dev/null || true
}
trap cleanup EXIT

echo "Waiting for startup..."
for i in $(seq 1 10); do
  if curl -s http://localhost:7801/healthz | grep -q ok; then
    echo "Health check passed"
    break
  fi
  if [ "$i" -eq 10 ]; then
    echo "FAIL: health check did not pass"
    docker logs dockgraph-test
    exit 1
  fi
  sleep 1
done

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:7801/)
if [ "$HTTP_CODE" != "200" ]; then
  echo "FAIL: expected 200 for /, got $HTTP_CODE"
  exit 1
fi
echo "Static file serving: OK ($HTTP_CODE)"

WS_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Upgrade: websocket" \
  -H "Connection: Upgrade" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  http://localhost:7801/ws)
if [ "$WS_CODE" != "101" ]; then
  echo "FAIL: expected 101 for /ws upgrade, got $WS_CODE"
  exit 1
fi
echo "WebSocket upgrade: OK ($WS_CODE)"

echo ""
echo "=== All smoke tests passed ==="

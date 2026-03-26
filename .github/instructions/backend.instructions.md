---
applyTo: "backend/**/*.go"
---

# Backend Review Rules (Go)

## Style and Formatting

- Code must pass `golangci-lint` with the project config (`.golangci.yml`).
- Formatting is enforced by `gofumpt` (stricter than `gofmt`).
- Imports should be grouped: stdlib, then external, then internal packages.

## Error Handling

- Every error must be handled explicitly. Use `fmt.Errorf("context: %w", err)`
  to wrap errors with context.
- `log.Fatalf` is acceptable only during startup (before the server is listening).
  After startup, errors should be returned or logged without crashing.
- In HTTP handlers, always set the appropriate status code before writing the body.

## Concurrency

- Goroutines must be cancellable via `context.Context`.
- Shared state must be protected by a mutex or communicated through channels.
- Prefer channels for producer-consumer patterns (see `collector.Updates()`).
- Watch for goroutine leaks: every `go func()` needs a shutdown path.

## Docker API Usage

- The Docker socket is mounted read-only. Never introduce write operations
  (container create/start/stop/remove) — this is a read-only visualizer.
- Use `client.WithAPIVersionNegotiation()` for Docker client creation.
- Filter out DockGraph's own container using the `dockgraph.self=true` label.

## WebSocket Protocol

- The wire format is defined in `collector/types.go` (`WireMessage`,
  `GraphSnapshot`, `DeltaUpdate`). Changes here are breaking for all clients.
- Messages use JSON encoding. Field names use camelCase for JavaScript
  compatibility.
- The `version` field exists for future protocol evolution — it must be
  incremented for breaking changes.

## Testing

- Table-driven tests are preferred for functions with multiple input/output cases.
- Test files live alongside the code they test (`_test.go` suffix).
- Use `testdata/` directories for fixture files.
- Tests should not depend on a running Docker daemon unless explicitly marked
  as integration tests.

## Dependencies

- Direct dependencies are intentionally minimal. New dependencies need justification.
- Current core deps: `docker/docker`, `compose-spec/compose-go`,
  `gorilla/websocket`, `fsnotify`.

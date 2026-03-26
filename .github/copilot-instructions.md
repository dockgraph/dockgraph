# DockGraph — Review Instructions

DockGraph is a real-time Docker infrastructure visualizer. It monitors containers,
networks, and volumes via the Docker API and Compose file parsing, then renders
an interactive topology graph over WebSocket.

## Architecture

- **Backend (Go):** Two concurrent collectors (Docker API + Compose file watcher)
  feed a state manager that merges snapshots and broadcasts deltas over WebSocket.
- **Frontend (React + TypeScript):** Receives WebSocket updates and renders the
  graph using React Flow with ELK.js for automatic layout.
- **Single binary:** The frontend is embedded into the Go binary at build time.
  One container, no external dependencies.

## Design Principles

When reviewing, flag violations of these principles:

- **Single Responsibility** — each function/type does one thing. If it needs "and"
  to describe, it should be split.
- **Fail Fast** — detect errors early, report clearly. Don't let bad state propagate.
- **Least Surprise** — code should behave as the reader expects. Naming and behavior
  must be predictable.
- **Composition over Inheritance** — prefer composing small, focused types.
- **DRY, but not premature** — extract shared logic only after the third repetition.
  Two similar blocks are fine.
- **YAGNI** — don't build for hypothetical futures. Solve the current problem.

## Code Quality Expectations

- Comments explain *why*, not *what*. Flag redundant comments that restate the code.
- Public functions and types should have doc comments.
- No magic values — constants should be named and centralized.
- Error handling must be explicit. Silently swallowed errors are a bug.
- Security-sensitive code (HTTP handlers, WebSocket, Docker socket access) deserves
  extra scrutiny.

## What to Watch For

- **Breaking changes** to the WebSocket wire format (`WireMessage`, `GraphSnapshot`,
  `DeltaUpdate`) — these affect all connected clients.
- **Goroutine leaks** — every goroutine must have a clear shutdown path via context
  cancellation.
- **Docker socket security** — the socket is mounted read-only. Flag any code that
  attempts write operations against the Docker API.
- **Frontend performance** — unnecessary re-renders in React components, especially
  in the graph rendering pipeline (`FlowCanvas`, `useDockGraph`).
- **Dependency additions** — new dependencies should be justified. This project
  values a small dependency footprint.

## Commit and PR Standards

- Commits follow [Conventional Commits](https://www.conventionalcommits.org/):
  `feat:`, `fix:`, `refactor:`, `docs:`, etc.
- PRs should be small and focused — one concern per PR.
- All CI checks (lint + test) must pass before merge.

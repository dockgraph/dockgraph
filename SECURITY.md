# Security Policy

## Supported Versions

We use [Semantic Versioning](https://semver.org/).

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |

## Security Model

DockGraph is a **read-only** monitoring tool. It reads the Docker daemon socket to observe infrastructure state and exposes a WebSocket API that streams topology updates to connected clients.

**What DockGraph can see:** container names, images, status, network topology, volume mounts, port mappings, compose file structure.

**What DockGraph cannot do:** start, stop, create, or modify any Docker resource. The socket is mounted read-only.

### Authentication

DockGraph supports optional password protection via the `DG_PASSWORD` environment variable. When set:

- All HTTP and WebSocket access requires authentication through a login page.
- Sessions use JWT tokens stored in HTTP-only, SameSite cookies (7-day expiry).
- Passwords are hashed with Argon2id. Plaintext passwords in the environment variable are hashed at startup and never stored in memory.
- Login attempts are rate-limited (5 per minute per IP).
- Server restarts invalidate all active sessions (ephemeral signing key).

When `DG_PASSWORD` is not set, DockGraph runs without authentication. A warning is logged at startup.

### Deployment guidance

- Set `DG_PASSWORD` when DockGraph is accessible beyond localhost.
- Bind to `127.0.0.1` (`DG_BIND_ADDR`) when running on shared or production hosts.
- Use a reverse proxy (nginx, Caddy, Traefik) for TLS termination — DockGraph serves plain HTTP.
- The Docker socket grants read access to all Docker resources on the host — treat DockGraph's port with the same care as your Docker API.

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Please report security issues through [GitHub Security Advisories](https://github.com/dockgraph/dockgraph/security/advisories/new). This allows us to assess the issue, prepare a fix, and coordinate disclosure without putting users at risk.

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### What to expect

- **Acknowledgment** as soon as possible
- **Status updates** as we investigate
- **Fix timeline** depends on severity, but we aim to patch critical issues quickly

We follow [coordinated disclosure](https://en.wikipedia.org/wiki/Coordinated_vulnerability_disclosure) — we'll work with you on timing before any public announcement.

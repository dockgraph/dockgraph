# Security Policy

## Supported Versions

We use [Semantic Versioning](https://semver.org/).

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |

## Security Model

DockGraph is a **read-only** monitoring tool. It reads the Docker daemon socket to observe infrastructure state and exposes a WebSocket API that streams topology updates to connected clients.

**What DockGraph can see:** container names, images, status, configuration (environment, labels, commands, with credentials masked), logs, network topology, volume mounts, port mappings, compose file structure.

**What DockGraph does:** only read. It never starts, stops, creates or modifies a Docker resource. The Docker API it is given, however, is not read-only: mounting the socket `:ro` protects only the socket file, and every API call, including creating privileged containers, still works through it. Treat DockGraph's container as root-equivalent on the host, or put a read-only Docker API proxy between DockGraph and the socket.

**Compose files** are parsed one at a time in a separate process, limited to 30 seconds and 256 MiB of memory and given none of DockGraph's environment but its time zone and home directory. Where the kernel provides Landlock, it can read only files under the compose paths, write nothing and, on Linux 6.7 and later, open no TCP connection. Only kernels whose Landlock also covers UNIX sockets (ABI 9) keep it from connecting to one, such as the Docker socket; the compose parser itself opens none.

### Authentication

DockGraph supports optional password protection via the `DG_PASSWORD` environment variable (or `DG_PASSWORD_FILE`, e.g. a Docker secret). When set:

- All HTTP and WebSocket access requires authentication through a login page.
- Sessions use JWT tokens stored in HTTP-only, SameSite cookies (7-day expiry), marked `Secure` over HTTPS, including behind a proxy listed in `DG_TRUSTED_PROXIES`.
- Passwords are hashed with Argon2id, and one verification runs at a time. A plaintext `DG_PASSWORD` is hashed at startup; the variable itself stays in the process environment and in `docker inspect`, so prefer `DG_PASSWORD_FILE` or a pre-hashed value.
- Login attempts are rate-limited to 5 per minute per client (IPv6 clients per /64). Behind a reverse proxy, list it in `DG_TRUSTED_PROXIES` so clients are told apart by `X-Forwarded-For`; otherwise they all share the proxy's budget.
- State-changing requests from other origins are rejected.
- Server restarts invalidate all active sessions (ephemeral signing key).

When `DG_PASSWORD` is not set, DockGraph runs without authentication and logs a warning at startup. It then serves only requests addressed to an IP address, `localhost` or a host name listed in `DG_ALLOWED_HOSTS`.

### Deployment guidance

- Set `DG_PASSWORD` whenever DockGraph's port is reachable from anything but the host itself, and remember that other containers on the host can reach published ports.
- To keep DockGraph local, publish its port on the host's loopback (`-p 127.0.0.1:7800:7800`). `DG_BIND_ADDR` is the address inside the container: setting it to `127.0.0.1` there makes the UI unreachable.
- Use a reverse proxy (nginx, Caddy, Traefik) for TLS termination — DockGraph serves plain HTTP — and list it in `DG_TRUSTED_PROXIES`.
- Credentials in container configuration are masked, but logs are not: access to DockGraph is access to whatever your containers print.
- In a swarm, keep the internal overlay network encrypted (the `stack.yml` default) and authenticate the server to its agents with agent keys: the private key as a Docker secret given only to the server, the public key to the agents. The older shared agent token is deprecated.

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

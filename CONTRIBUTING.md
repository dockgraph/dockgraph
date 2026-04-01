# Contributing to DockGraph

Thanks for your interest in contributing! This document covers how to get started.

## Before You Start

- **Questions or ideas?** Start a [Discussion](https://github.com/dockgraph/dockgraph/discussions) — it's the best place for open-ended conversations, feature proposals, and help.
- **Found a bug?** Open an [issue](https://github.com/dockgraph/dockgraph/issues/new?template=bug_report.yml).
- **Planning a large change?** Please discuss it first (issue or discussion) before submitting a PR. This saves everyone's time.

### Accepted without prior discussion

- Bug fixes with a clear reproduction
- Documentation improvements
- Small, focused improvements

### Needs discussion first

- New features
- Large refactors
- Changes to the graph layout or data model

### Won't be merged

- Breaking changes without a migration path
- Changes that don't pass CI
- Unrelated or out-of-scope features

## Development Setup

See the [Development section in README.md](README.md#development) for prerequisites, build commands, and test instructions.

## Code Style

- **Go**: Follow standard `gofmt` conventions. Run `make lint-backend` before committing.
- **TypeScript/React**: Follow ESLint config. Run `make lint-frontend` before committing.
- **Commits**: Use [Conventional Commits](https://www.conventionalcommits.org/) — `feat:`, `fix:`, `refactor:`, `docs:`, etc.

## Pull Requests

1. Fork the repo and create a branch from `main`
   - Use the format `type/short-description` — e.g., `fix/edge-rendering`, `feat/container-logs`, `docs/contributing`
2. Make your changes in focused, logical commits
3. Ensure tests pass and lint is clean
4. Open a PR using the provided template
5. Link the relevant issue

Keep PRs small and focused. One PR should address one concern.

## License

By contributing, you agree that your contributions will be licensed under the [Business Source License 1.1](LICENSE).

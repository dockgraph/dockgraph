# ── Stage 1: Build frontend ──────────────────────────────────
FROM node:26-alpine@sha256:c6ad9e9c1d80f6eb607c26b4b0a132ff7a2344fad1c78ea3baa2a7d4c8de6d94 AS frontend
WORKDIR /app/frontend

# Install dependencies first (cached layer — only re-runs when lockfile changes)
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

# Copy source and build the production bundle
COPY frontend/ ./
RUN --mount=type=cache,target=/app/frontend/node_modules/.vite npm run build


# ── Stage 2: Build backend ───────────────────────────────────
FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS backend
ENV GOTOOLCHAIN=auto
ENV CGO_ENABLED=0
ENV GOOS=linux
WORKDIR /app/backend

# Download modules first (cached layer — only re-runs when go.mod/go.sum change)
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy source and embed the frontend build output into the binary
COPY backend/ ./
COPY --from=frontend /app/frontend/dist ./frontend/dist

# VERSION declared late to avoid busting the cache for layers above
ARG VERSION=dev
# Compile a static, stripped binary with the version stamped in
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w -X main.Version=${VERSION}" -o dockgraph .


# ── Stage 3: Runtime ─────────────────────────────────────────
FROM gcr.io/distroless/static@sha256:448810357cc8e67b7208988abdba0f5ec3d3aebee0a5b13eb624979792cffc0f
COPY --from=backend /app/backend/dockgraph /dockgraph
EXPOSE 7800
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=5s \
  CMD ["/dockgraph", "--healthcheck"]
ENTRYPOINT ["/dockgraph"]

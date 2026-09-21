# ── Stage 1: Build frontend ──────────────────────────────────
FROM node:26-alpine@sha256:2c45bdcbf63561a54da9549612084b43ca309854a4110c87857d609ddeb61c9e AS frontend
WORKDIR /app/frontend

# Install dependencies first (cached layer — only re-runs when lockfile changes)
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

# Copy source and build the production bundle
COPY frontend/ ./
RUN --mount=type=cache,target=/app/frontend/node_modules/.vite npm run build


# ── Stage 2: Build backend ───────────────────────────────────
FROM golang:1.27-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS backend
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
FROM gcr.io/distroless/static@sha256:58133991db06659feaabe0f4e97a35cebf15ef4ea08f8a4c6d2ee5f75e4aa6a0
COPY --from=backend /app/backend/dockgraph /dockgraph
EXPOSE 7800
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=5s \
  CMD ["/dockgraph", "--healthcheck"]
ENTRYPOINT ["/dockgraph"]

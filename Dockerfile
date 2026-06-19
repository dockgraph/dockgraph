# ── Stage 1: Build frontend ──────────────────────────────────
FROM node:26-alpine@sha256:a2dc166a387cc6ca1e62d0c8e265e49ca985d6e60abc9fe6e6c3d6ce8e63f606 AS frontend
WORKDIR /app/frontend

# Install dependencies first (cached layer — only re-runs when lockfile changes)
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

# Copy source and build the production bundle
COPY frontend/ ./
RUN --mount=type=cache,target=/app/frontend/node_modules/.vite npm run build


# ── Stage 2: Build backend ───────────────────────────────────
FROM golang:1.26-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS backend
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
FROM gcr.io/distroless/static@sha256:3592aa8171c77482f62bbc4164e6a2d141c6122554ace66e5cc910cadb961ff0
COPY --from=backend /app/backend/dockgraph /dockgraph
EXPOSE 7800
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=5s \
  CMD ["/dockgraph", "--healthcheck"]
ENTRYPOINT ["/dockgraph"]

# ── Stage 1: Build frontend ──────────────────────────────────
FROM node:26-alpine@sha256:0b36e8c136b94cd4fcf02188228e76c31ad5872eef3fec8cbd2eee500cfd9e80 AS frontend
WORKDIR /app/frontend

# Install dependencies first (cached layer — only re-runs when lockfile changes)
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

# Copy source and build the production bundle
COPY frontend/ ./
RUN --mount=type=cache,target=/app/frontend/node_modules/.vite npm run build


# ── Stage 2: Build backend ───────────────────────────────────
FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS backend
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
FROM gcr.io/distroless/static@sha256:034df5c1737e8241f6ed6423a12e720ee0f6d6f088609ee172f98f6f2166ef5d
COPY --from=backend /app/backend/dockgraph /dockgraph
EXPOSE 7800
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=5s \
  CMD ["/dockgraph", "--healthcheck"]
ENTRYPOINT ["/dockgraph"]

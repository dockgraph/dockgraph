# ── Stage 1: Build frontend ──────────────────────────────────
FROM node:26-alpine@sha256:3ad34ca6292aec4a91d8ddeb9229e29d9c2f689efd0dd242860889ac71842eba AS frontend
WORKDIR /app/frontend

# Install dependencies first (cached layer — only re-runs when lockfile changes)
COPY frontend/package.json frontend/package-lock.json ./
# TODO: remove --legacy-peer-deps once typescript-eslint supports TypeScript 6
RUN --mount=type=cache,target=/root/.npm npm ci --legacy-peer-deps

# Copy source and build the production bundle
COPY frontend/ ./
RUN --mount=type=cache,target=/app/frontend/node_modules/.vite npm run build


# ── Stage 2: Build backend ───────────────────────────────────
FROM golang:1.26-alpine@sha256:7a3e50096189ad57c9f9f865e7e4aa8585ed1585248513dc5cda498e2f41812c AS backend
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

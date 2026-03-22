# Stage 1: Build frontend
FROM node:24-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.25-alpine AS backend
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o docker-flow .

# Stage 3: Runtime
FROM gcr.io/distroless/static:latest
COPY --from=backend /app/docker-flow /docker-flow
EXPOSE 7800
ENTRYPOINT ["/docker-flow"]

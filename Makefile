.PHONY: all build build-backend build-frontend test test-coverage lint lint-backend lint-frontend vet fmt dev docker docker-up docker-down ci tidy clean help \
	demo demo-down demo-small demo-small-down demo-medium demo-medium-down demo-large demo-large-down

GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null)
VERSION ?= $(if $(GIT_SHA),dev-$(GIT_SHA),dev)

all: build ## Build everything

build: build-frontend build-backend ## Build frontend then backend

build-backend: ## Build Go backend
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.Version=$(VERSION)" -o dockgraph .

build-frontend: ## Build React frontend
	cd frontend && npm ci --legacy-peer-deps && npm run build

test: ## Run all tests
	cd backend && go test -race ./...
	cd frontend && npm test

test-coverage: ## Run tests with coverage (enforces thresholds)
	cd backend && go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
	cd frontend && npx vitest run --coverage

lint: lint-backend lint-frontend ## Run all linters

lint-backend: ## Lint Go code
	cd backend && golangci-lint run ./...

lint-frontend: ## Lint TypeScript code
	cd frontend && npm run lint

vet: ## Run go vet
	cd backend && go vet ./...

fmt: ## Format Go code
	cd backend && golangci-lint fmt ./...

ci: lint tidy test build ## Run all checks (mirrors GitHub CI pipeline)

tidy: ## Verify go.mod is tidy
	cd backend && go mod tidy && git diff --exit-code go.mod go.sum

dev: ## Start development servers (backend + frontend)
	@echo "Start backend: cd backend && go run ."
	@echo "Start frontend: cd frontend && npm run dev"

docker: ## Build Docker image
	docker build -t dockgraph/dockgraph:local --build-arg VERSION=$(VERSION) .

docker-up: ## Start with Docker Compose
	docker compose up --build

docker-down: ## Stop Docker Compose
	docker compose down

# ── Demo stacks ───────────────────────────────────────────────

demo: demo-small demo-medium demo-large ## Start all demo stacks

demo-down: demo-small-down demo-medium-down demo-large-down ## Stop all demo stacks and remove volumes

demo-small: ## Start small demo (5 services)
	docker compose -f demo/compose-small.yml up -d

demo-small-down: ## Stop small demo and remove volumes
	docker compose -f demo/compose-small.yml down -v

demo-medium: ## Start medium demo (~15 services)
	docker compose -f demo/compose-medium.yml up -d

demo-medium-down: ## Stop medium demo and remove volumes
	docker compose -f demo/compose-medium.yml down -v

demo-large: ## Start large demo (~46 services)
	docker compose -f demo/compose-large.yml up -d

demo-large-down: ## Stop large demo and remove volumes
	docker compose -f demo/compose-large.yml down -v

# ── Cleanup ───────────────────────────────────────────────────

clean: ## Remove build artifacts
	rm -f backend/dockgraph
	rm -rf frontend/dist

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

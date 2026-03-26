.PHONY: all build build-backend build-frontend test lint lint-backend lint-frontend vet fmt dev docker docker-up docker-down ci clean help

GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null)
VERSION ?= $(if $(GIT_SHA),dev-$(GIT_SHA),dev)

all: build ## Build everything

build: build-frontend build-backend ## Build frontend then backend

build-backend: ## Build Go backend
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.Version=$(VERSION)" -o dockgraph .

build-frontend: ## Build React frontend
	cd frontend && npm ci --legacy-peer-deps && npm run build

test: ## Run all tests
	cd backend && go test ./...

lint: lint-backend lint-frontend ## Run all linters

lint-backend: ## Lint Go code
	cd backend && golangci-lint run ./...

lint-frontend: ## Lint TypeScript code
	cd frontend && npm run lint

vet: ## Run go vet
	cd backend && go vet ./...

fmt: ## Format Go code
	cd backend && golangci-lint fmt ./...

ci: lint test ## Run all checks (lint + test)

dev: ## Start development servers (backend + frontend)
	@echo "Start backend: cd backend && go run ."
	@echo "Start frontend: cd frontend && npm run dev"

docker: ## Build Docker image
	docker build -t dockgraph/dockgraph:local --build-arg VERSION=$(VERSION) .

docker-up: ## Start with Docker Compose
	docker compose up --build

docker-down: ## Stop Docker Compose
	docker compose down

clean: ## Remove build artifacts
	rm -f backend/dockgraph
	rm -rf frontend/dist

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

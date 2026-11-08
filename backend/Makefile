.PHONY: help db-up db-down api worker test lint build build-cli migrate

# Development-only defaults. Never use this key outside your machine; set your own
# SECRET_KEY (openssl rand -base64 32) and leave ALLOW_PRIVATE_NETWORK unset in production.
export SECRET_KEY ?= ZGV2LW9ubHktdGVzdGZvcmdlLWtleS0zMi1ieXRlcyE=
export ALLOW_PRIVATE_NETWORK ?= true

COMPOSE := docker compose -f ../compose.yaml

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

db-up: ## Start local Postgres (port 5433) and wait until healthy
	$(COMPOSE) up -d --wait postgres

db-down: ## Stop local services
	$(COMPOSE) down

api: db-up ## Run the Go API on :8080
	go run ./cmd/api serve

worker: db-up ## Run the run worker (executes queued runs)
	go run ./cmd/worker

migrate: db-up ## Apply database migrations
	go run ./cmd/api migrate

test: db-up ## Run backend tests (needs Postgres)
	go test ./...

lint: ## Vet the backend
	go vet ./...

build: ## Build backend binaries into bin/
	go build -o bin/ ./cmd/...

build-cli: ## Build the testforge CLI/MCP binary into bin/
	go build -ldflags "-X testforge/internal/cli.Version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/testforge ./cmd/testforge

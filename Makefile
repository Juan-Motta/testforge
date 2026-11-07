.PHONY: help db-up db-down api worker web test test-api lint-api build-api migrate

# Development-only defaults. Never use this key outside your machine; set your own
# SECRET_KEY (openssl rand -base64 32) and leave ALLOW_PRIVATE_NETWORK unset in production.
export SECRET_KEY ?= ZGV2LW9ubHktdGVzdGZvcmdlLWtleS0zMi1ieXRlcyE=
export ALLOW_PRIVATE_NETWORK ?= true

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

db-up: ## Start local Postgres (port 5433) and wait until healthy
	docker compose up -d --wait postgres

db-down: ## Stop local services
	docker compose down

api: db-up ## Run the Go API on :8080
	cd backend && go run ./cmd/api serve

worker: db-up ## Run the run worker (executes queued runs)
	cd backend && go run ./cmd/worker

migrate: db-up ## Apply database migrations
	cd backend && go run ./cmd/api migrate

web: ## Run the Next.js app on :3000
	pnpm --filter web dev

test: test-api ## Run all tests

test-api: db-up ## Run backend tests (needs Postgres)
	cd backend && go test ./...

lint-api: ## Vet the backend
	cd backend && go vet ./...

build-api: ## Build backend binaries into backend/bin
	cd backend && go build -o bin/ ./cmd/...

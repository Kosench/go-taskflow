BINARY_NAME=go-taskflow
DOCKER_COMPOSE=docker-compose
GO=go
GOFLAGS=-v
PROTO_DIR=proto
OUT_DIR=bin

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@egrep '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

.PHONY: build
build: ## Build all binaries
	@echo "$(GREEN)Building binaries...$(NC)"
	@mkdir -p $(OUT_DIR)
	@$(GO) build $(GOFLAGS) -o $(OUT_DIR)/api ./cmd/api
	@$(GO) build $(GOFLAGS) -o $(OUT_DIR)/worker ./cmd/worker
	@$(GO) build $(GOFLAGS) -o $(OUT_DIR)/migrator ./cmd/migrator
	@echo "$(GREEN)Build complete!$(NC)"

.PHONY: run-api
run-api: ## Run API server
	@echo "$(GREEN)Starting API server...$(NC)"
	@$(GO) run ./cmd/api

.PHONY: run-worker
run-worker: ## Run worker
	@echo "$(GREEN)Starting worker...$(NC)"
	@$(GO) run ./cmd/worker

.PHONY: test
test: ## Run unit tests
	@echo "$(GREEN)Running tests...$(NC)"
	@$(GO) test -v -race -short ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(NC)"
	@$(GO) test -v -race -tags=integration ./tests/integration/...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	@$(GO) test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

.PHONY: lint
lint: ## Run linter
	@echo "$(GREEN)Running linter...$(NC)"
	@golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	@$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	@$(GO) vet ./...

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning...$(NC)"
	@rm -rf $(OUT_DIR)
	@rm -f coverage.txt coverage.html
	@echo "$(GREEN)Clean complete!$(NC)"

.PHONY: docker-up
docker-up: ## Start docker containers
	@echo "$(GREEN)Starting Docker containers...$(NC)"
	@$(DOCKER_COMPOSE) up -d

.PHONY: docker-down
docker-down: ## Stop docker containers
	@echo "$(YELLOW)Stopping Docker containers...$(NC)"
	@$(DOCKER_COMPOSE) down

.PHONY: docker-logs
docker-logs: ## Show docker logs
	@$(DOCKER_COMPOSE) logs -f

.PHONY: proto
proto: ## Generate protobuf code
	@echo "$(GREEN)Generating protobuf code...$(NC)"
	@protoc --go_out=. --go-grpc_out=. $(PROTO_DIR)/*.proto

.PHONY: migrate-up
migrate-up: ## Run database migrations up
	@echo "$(GREEN)Running migrations...$(NC)"
	@$(GO) run ./cmd/migrator up

.PHONY: migrate-down
migrate-down: ## Run database migrations down
	@echo "$(YELLOW)Rolling back migrations...$(NC)"
	@$(GO) run ./cmd/migrator down

.PHONY: migrate-create
migrate-create: ## Create a new migration (usage: make migrate-create name=migration_name)
	@echo "$(GREEN)Creating migration: $(name)$(NC)"
	@migrate create -ext sql -dir migrations -seq $(name)

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "$(GREEN)Installing development tools...$(NC)"
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "$(GREEN)Tools installed!$(NC)"

.PHONY: check
check: fmt vet lint test ## Run all checks
	@echo "$(GREEN)All checks passed!$(NC)"

.PHONY: infra-up
infra-up: ## Start infrastructure (PostgreSQL, Kafka, etc.)
	@./scripts/infra.sh up

.PHONY: infra-down
infra-down: ## Stop infrastructure
	@./scripts/infra.sh down

.PHONY: infra-restart
infra-restart: ## Restart infrastructure
	@./scripts/infra.sh restart

.PHONY: infra-logs
infra-logs: ## Show infrastructure logs
	@./scripts/infra.sh logs

.PHONY: infra-status
infra-status: ## Show infrastructure status
	@./scripts/infra.sh status

.PHONY: infra-clean
infra-clean: ## Clean infrastructure (WARNING: deletes data)
	@./scripts/infra.sh clean

.PHONY: kafka-topics
kafka-topics: ## Create Kafka topics
	@./scripts/create-topics.sh

.PHONY: migrate-up
migrate-up: ## Run database migrations up
	@echo "$(GREEN)Running migrations up...$(NC)"
	@go run ./cmd/migrator -command=up

.PHONY: migrate-down
migrate-down: ## Run database migrations down
	@echo "$(YELLOW)Running migrations down...$(NC)"
	@go run ./cmd/migrator -command=down

.PHONY: migrate-drop
migrate-drop: ## Drop all migrations (DANGER!)
	@echo "$(RED)Dropping all migrations...$(NC)"
	@go run ./cmd/migrator -command=drop

.PHONY: migrate-version
migrate-version: ## Show current migration version
	@echo "$(GREEN)Current migration version:$(NC)"
	@go run ./cmd/migrator -command=version

.PHONY: migrate-create
migrate-create: ## Create a new migration (usage: make migrate-create name=migration_name)
	@echo "$(GREEN)Creating migration: $(name)$(NC)"
	@migrate create -ext sql -dir migrations -seq $(name)

.PHONY: all
all: clean build test ## Clean, build and test

.DEFAULT_GOAL := help
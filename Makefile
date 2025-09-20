# Developer Mesh Makefile
# Run 'make help' to see all available commands

.DEFAULT_GOAL := help
.PHONY: help
help: ## Show this help message
	@echo "Developer Mesh - Available Commands"
	@echo "==============================="
	@echo ""
	@echo "🚀 Quick Start:"
	@echo "  make local-docker   # Full Docker environment setup + tests"
	@echo "  make dev            # Basic Docker environment setup"
	@echo "  make test-e2e-local # Run E2E tests against local services"
	@echo ""
	@echo "📦 Common Workflows:"
	@echo "  make test           # Run all unit tests"
	@echo "  make pre-commit     # Run all checks before committing"
	@echo "  make build          # Build all applications"
	@echo "  make lint           # Run code linters"
	@echo ""
	@echo "🧪 Testing:"
	@echo "  make test-e2e       # Run all E2E tests"
	@echo "  make quick-test     # Quick single agent test"
	@echo "  make fix-multiagent # Test multi-agent workflow fix"
	@echo ""
	@echo "🔧 Environment Management:"
	@echo "  make env-check      # Check current environment"
	@echo "  make env-local      # Switch to local Docker environment"
	@echo "  make env-aws        # Switch to AWS environment"
	@echo ""
	@echo "🌐 SSH Tunnels (AWS):"
	@echo "  make tunnel-all     # Create all SSH tunnels"
	@echo "  make tunnel-status  # Check tunnel status"
	@echo "  make tunnel-kill    # Terminate all tunnels"
	@echo ""
	@echo "📚 Documentation:"
	@echo "  docs/TROUBLESHOOTING.md      # Common issues and solutions"
	@echo "  docs/ENVIRONMENT_SWITCHING.md # How to switch environments"
	@echo "  docs/LOCAL_DEVELOPMENT.md     # Local development guide"
	@echo ""
	@echo "📊 All Commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ==============================================================================
# Variables
# ==============================================================================
# See docs/ENVIRONMENT_SWITCHING.md for environment configuration details
# ==============================================================================

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOWORKCMD=$(GOCMD) work
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Application directories
EDGE_MCP_DIR=./apps/edge-mcp
REST_API_DIR=./apps/rest-api
WORKER_DIR=./apps/worker

# Binary names
EDGE_MCP_BINARY=edge-mcp
REST_API_BINARY=rest-api
WORKER_BINARY=worker

# Docker configuration
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
DOCKER_COMPOSE=docker-compose -f docker-compose.local.yml

# Other configuration
SWAG_VERSION=v1.16.2

# ==============================================================================
# Primary Commands
# ==============================================================================

.PHONY: all
all: clean test build ## Clean, test, and build everything

.PHONY: dev
dev: dev-setup docker-up wait-for-healthy ## Start development environment with Docker
	@echo "✅ Development environment is ready!"
	@echo ""
	@echo "Services available at:"
	@echo "  Edge MCP: http://localhost:8085"
	@echo "  REST API: http://localhost:8081"
	@echo "  Mock Server: http://localhost:8082"
	@echo "  PostgreSQL: localhost:5432"
	@echo "  Redis: localhost:6379"
	@echo ""
	@echo "Run 'make test-e2e-local' to test against local services"

.PHONY: dev-native
dev-native: dev-setup ## Setup for running services locally (without Docker)
	@echo "Starting services locally requires PostgreSQL and Redis running."
	@echo "Run: brew services start postgresql redis"
	@echo "Then run each service: make run-edge-mcp, run-rest-api, run-worker"

.PHONY: mcp-check
mcp-check: ## Reminder to use MCP tools instead of CLI
	@./scripts/mcp-validation-check.sh

.PHONY: pre-commit
pre-commit: mcp-check fmt lint test-coverage security-check ## Run all pre-commit checks
	@echo "Checking test coverage..."
	@coverage=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$coverage < 85" | bc) -eq 1 ]; then \
		echo "❌ Test coverage $$coverage% is below 85%"; \
		exit 1; \
	else \
		echo "✅ Test coverage $$coverage% meets requirement"; \
	fi
	@echo "✅ All pre-commit checks passed!"

# ==============================================================================
# Setup & Installation
# ==============================================================================

.PHONY: install-tools
install-tools: ## Install all development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	@echo "✅ Tools installed"

.PHONY: dev-setup
dev-setup: ## Setup development environment
	@if [ ! -f .env ]; then \
		echo "Creating .env file from example..."; \
		cp .env.example .env; \
		echo "✅ Created .env file - please update with your values"; \
	else \
		echo "✅ .env file already exists"; \
	fi
	@echo "💡 To generate development TLS certificates, run: make dev-certs"

.PHONY: dev-certs
dev-certs: ## Generate development TLS certificates
	@echo "🔐 Generating development TLS certificates..."
	@./scripts/certs/generate-dev-certs.sh
	@echo "✅ Certificates generated. Run 'source certs/dev/env-exports.sh' to load environment variables."

# ==============================================================================
# Build Commands
# ==============================================================================

.PHONY: build
build: build-edge-mcp build-rest-api build-worker ## Build all applications

.PHONY: build-edge-mcp
build-edge-mcp: ## Build Edge MCP binary
	@echo "Building Edge MCP..."
	@cd apps/edge-mcp && go build -o ../../bin/edge-mcp ./cmd/server
	@echo "✅ Edge MCP built: bin/edge-mcp"

.PHONY: build-edge-mcp-all
build-edge-mcp-all: ## Build Edge MCP for all platforms
	@echo "Building Edge MCP for all platforms..."
	@mkdir -p dist
	@cd apps/edge-mcp && \
		GOOS=darwin GOARCH=amd64 go build -o ../../dist/edge-mcp-darwin-amd64 ./cmd/server && \
		GOOS=darwin GOARCH=arm64 go build -o ../../dist/edge-mcp-darwin-arm64 ./cmd/server && \
		GOOS=linux GOARCH=amd64 go build -o ../../dist/edge-mcp-linux-amd64 ./cmd/server && \
		GOOS=linux GOARCH=arm64 go build -o ../../dist/edge-mcp-linux-arm64 ./cmd/server && \
		GOOS=windows GOARCH=amd64 go build -o ../../dist/edge-mcp-windows-amd64.exe ./cmd/server && \
		GOOS=windows GOARCH=arm64 go build -o ../../dist/edge-mcp-windows-arm64.exe ./cmd/server
	@echo "✅ Built Edge MCP for all platforms in dist/"

.PHONY: install-edge-mcp
install-edge-mcp: build-edge-mcp ## Install Edge MCP to /usr/local/bin
	@echo "Installing Edge MCP..."
	@sudo cp bin/edge-mcp /usr/local/bin/edge-mcp
	@sudo chmod +x /usr/local/bin/edge-mcp
	@echo "✅ Edge MCP installed to /usr/local/bin/edge-mcp"
	@edge-mcp --version || echo "Run 'edge-mcp --version' to verify installation"

.PHONY: build-rest-api
build-rest-api: ## Build REST API
	$(GOBUILD) -o $(REST_API_DIR)/$(REST_API_BINARY) -v $(REST_API_DIR)/cmd/api

.PHONY: build-worker
build-worker: ## Build worker service
	$(GOBUILD) -o $(WORKER_DIR)/$(WORKER_BINARY) -v $(WORKER_DIR)/cmd/worker

.PHONY: clean
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f $(EDGE_MCP_DIR)/$(EDGE_MCP_BINARY) $(REST_API_DIR)/$(REST_API_BINARY) $(WORKER_DIR)/$(WORKER_BINARY)

# ==============================================================================
# Testing Commands
# ==============================================================================

.PHONY: test
test: ## Run all unit tests (excludes integration tests and Redis-dependent tests)
	@echo "Running unit tests..."
	@cd apps/edge-mcp && $(GOTEST) -v -short ./... && cd ../.. && \
	cd apps/rest-api && $(GOTEST) -v -short ./... && cd ../.. && \
	cd apps/worker && $(GOTEST) -v -short ./... && cd ../.. && \
	cd apps/mockserver && $(GOTEST) -v -short ./... && cd ../.. && \
	cd pkg && $(GOTEST) -v -short $$(go list ./... | grep -v embedding/cache) && cd ..

.PHONY: test-with-services
test-with-services: start-test-env ## Run unit tests that require Redis/PostgreSQL
	@echo "Running unit tests with Docker services..."
	@TEST_REDIS_ADDR=127.0.0.1:6379 $(GOTEST) -v -short ./apps/edge-mcp/... ./apps/rest-api/... ./apps/worker/... ./pkg/... || (make stop-test-env && exit 1)
	@make stop-test-env

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	$(GOTEST) -coverprofile=coverage.out ./apps/edge-mcp/... ./apps/rest-api/... ./apps/worker/... ./pkg/...
	$(GOCMD) tool cover -func=coverage.out

.PHONY: test-coverage-html
test-coverage-html: test-coverage ## Generate HTML coverage report
	$(GOCMD) tool cover -html=coverage.out

# Integration and E2E tests are defined later in the file with Docker support

.PHONY: test-all
test-all: test test-integration test-e2e ## Run all tests (unit, integration, and E2E)

.PHONY: test-embedding
test-embedding: ## Run embedding-specific tests
	@echo "Running embedding system tests..."
	@$(GOTEST) -v ./pkg/repository/model_catalog/... \
		./pkg/repository/tenant_models/... \
		./pkg/repository/embedding_usage/... \
		./apps/rest-api/internal/services/*model*.go

.PHONY: start-test-env
start-test-env: ## Start test environment (Redis + PostgreSQL in Docker)
	@echo "Starting test environment..."
	@docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for services to be ready..."
	@sleep 3
	@docker-compose -f docker-compose.test.yml ps

.PHONY: stop-test-env
stop-test-env: ## Stop test environment
	@echo "Stopping test environment..."
	@docker-compose -f docker-compose.test.yml down -v

.PHONY: test-integration test-int
test-integration test-int: start-test-env ## Run integration tests
	@echo "Running integration tests with Docker services..."
	@export ENABLE_INTEGRATION_TESTS=true && \
	export TEST_REDIS_ADDR=127.0.0.1:6379 && \
	export TEST_DATABASE_URL="postgres://test:test@127.0.0.1:5433/test?sslmode=disable" && \
	echo "Running integration tests for each module..." && \
	(cd apps/edge-mcp && $(GOTEST) -tags=integration -v ./... || exit 1) && \
	(cd apps/rest-api && $(GOTEST) -tags=integration -v ./... || exit 1) && \
	(cd apps/worker && $(GOTEST) -tags=integration -v ./... || exit 1) && \
	(cd apps/mockserver && $(GOTEST) -tags=integration -v ./... || exit 1) && \
	(cd pkg && $(GOTEST) -tags=integration -v ./... || exit 1) || \
	(make stop-test-env && exit 1)
	@make stop-test-env

.PHONY: test-redis-lifecycle
test-redis-lifecycle: ## Run Redis lifecycle integration tests with testcontainers
	@echo "Running Redis lifecycle integration tests with testcontainers..."
	@cd pkg/webhook && $(GOTEST) -tags=integration -v -run TestWithRealRedis ./...


.PHONY: bench
bench: ## Run benchmarks (PACKAGE=./pkg/embedding)
	$(GOTEST) -bench=. -benchmem ${PACKAGE:-./pkg/embedding}

# ==============================================================================
# Code Quality & Security
# ==============================================================================

.PHONY: lint
lint: ## Run linters
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@echo "Running golangci-lint on all workspace modules..."
	@cd pkg && golangci-lint run ./...
	@cd apps/edge-mcp && golangci-lint run ./...
	@cd apps/rest-api && golangci-lint run ./...
	@cd apps/worker && golangci-lint run ./...
	@cd apps/mockserver && golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code (excludes .claude templates which are not valid Go)
	@find . -name "*.go" -not -path "./.claude/*" -not -path "./vendor/*" -not -path "./.git/*" | xargs $(GOFMT) -w -s
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	@find . -name "*.go" -not -path "./.claude/*" -not -path "./vendor/*" -not -path "./.git/*" | xargs goimports -w

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet for all modules..."
	@cd apps/edge-mcp && $(GOVET) ./... || exit 1
	@cd apps/rest-api && $(GOVET) ./... || exit 1
	@cd apps/worker && $(GOVET) ./... || exit 1
	@cd apps/mockserver && $(GOVET) ./... || exit 1
	@cd pkg && $(GOVET) ./... || exit 1
	@echo "✅ All vet checks passed"

.PHONY: security-check
security-check: ## Run security checks
	@./.github/scripts/security-check.sh

# ==============================================================================
# Docker Commands
# ==============================================================================

.PHONY: docker-up
docker-up: ## Start all services with Docker Compose (rebuilds images)
	@echo "Building and starting services with Docker Compose..."
	@echo "Using Docker BuildKit for fast parallel builds..."
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 $(DOCKER_COMPOSE) up -d --build

.PHONY: down
down: ## Stop all Docker services
	$(DOCKER_COMPOSE) down

.PHONY: down-volumes
down-volumes: ## Stop all Docker services and remove volumes
	$(DOCKER_COMPOSE) down -v

.PHONY: logs
logs: ## View Docker logs (service=<name> to filter)
	$(DOCKER_COMPOSE) logs -f $(service)

.PHONY: restart
restart: ## Restart Docker services (service=<name> for specific service)
	$(DOCKER_COMPOSE) restart $(service)

.PHONY: ps
ps: ## Show running Docker services
	$(DOCKER_COMPOSE) ps

# ==============================================================================
# Database Commands
# ==============================================================================

.PHONY: db-shell
db-shell: ## Open PostgreSQL shell
	psql "postgresql://${DATABASE_USER:-dev}:${DATABASE_PASSWORD:-dev}@${DATABASE_HOST:-localhost}:${DATABASE_PORT:-5432}/${DATABASE_NAME:-dev}?sslmode=${DATABASE_SSL_MODE:-disable}"

.PHONY: migrate-create
migrate-create: ## Create new migration (name=migration_name)
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate create -ext sql -dir migrations -seq $(name)

.PHONY: migrate-up
migrate-up: ## Run all pending migrations
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "postgresql://${DATABASE_USER:-dev}:${DATABASE_PASSWORD:-dev}@${DATABASE_HOST:-localhost}:${DATABASE_PORT:-5432}/${DATABASE_NAME:-dev}?sslmode=${DATABASE_SSL_MODE:-disable}" -path apps/rest-api/migrations/sql up

.PHONY: migrate-up-docker
migrate-up-docker: ## Run migrations for Docker environment
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "postgresql://dev:dev@localhost:5432/dev?sslmode=disable" -path apps/rest-api/migrations/sql up

.PHONY: migrate-down
migrate-down: ## Rollback last migration
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "postgresql://${DATABASE_USER:-dev}:${DATABASE_PASSWORD:-dev}@${DATABASE_HOST:-localhost}:${DATABASE_PORT:-5432}/${DATABASE_NAME:-dev}?sslmode=${DATABASE_SSL_MODE:-disable}" -path apps/rest-api/migrations/sql down 1

.PHONY: migrate-status
migrate-status: ## Show migration status
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "postgresql://${DATABASE_USER:-dev}:${DATABASE_PASSWORD:-dev}@${DATABASE_HOST:-localhost}:${DATABASE_PORT:-5432}/${DATABASE_NAME:-dev}?sslmode=${DATABASE_SSL_MODE:-disable}" -path apps/rest-api/migrations/sql version
# ==============================================================================
# Development Helpers
# ==============================================================================

# Include .env file if it exists
-include .env
# Include .env.local if it exists (overrides .env)
-include .env.local
export

.PHONY: run-edge-mcp
run-edge-mcp: ## Run Edge MCP server locally
	cd $(EDGE_MCP_DIR) && MCP_CONFIG_FILE=../../configs/config.development.yaml ./$(EDGE_MCP_BINARY)

.PHONY: run-rest-api
run-rest-api: ## Run REST API locally
	cd $(REST_API_DIR) && MCP_CONFIG_FILE=../../configs/config.development.yaml ./$(REST_API_BINARY)

.PHONY: run-worker
run-worker: ## Run worker service locally
	cd $(WORKER_DIR) && MCP_CONFIG_FILE=../../configs/config.development.yaml ./$(WORKER_BINARY)

.PHONY: stop-edge-mcp
stop-edge-mcp: ## Stop Edge MCP server
	@echo "Stopping Edge MCP server..."
	@pkill -f "edge-mcp.*MCP_CONFIG_FILE" || echo "Edge MCP server not running"

.PHONY: stop-rest-api
stop-rest-api: ## Stop REST API
	@echo "Stopping REST API..."
	@pkill -f "rest-api.*MCP_CONFIG_FILE" || echo "REST API not running"

.PHONY: stop-worker
stop-worker: ## Stop worker service
	@echo "Stopping worker..."
	@pkill -f "worker.*MCP_CONFIG_FILE" || echo "Worker not running"

.PHONY: stop-all
stop-all: stop-edge-mcp stop-rest-api stop-worker ## Stop all services
	@echo "All services stopped"

.PHONY: swagger
swagger: ## Generate Swagger documentation
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	cd $(EDGE_MCP_DIR) && swag init -g ./cmd/server/main.go -o ./docs --parseDependency --parseInternal
	cd $(REST_API_DIR) && swag init -g ./cmd/api/main.go -o ./docs --parseDependency --parseInternal
	@echo "Swagger UI available at:"
	@echo "  Edge MCP: http://localhost:8085/swagger/index.html"
	@echo "  REST API: http://localhost:8081/swagger/index.html"

.PHONY: deps
deps: ## Update and sync dependencies
	$(GOMOD) tidy
	cd $(EDGE_MCP_DIR) && $(GOMOD) tidy
	cd $(REST_API_DIR) && $(GOMOD) tidy
	cd $(WORKER_DIR) && $(GOMOD) tidy
	$(GOWORKCMD) sync

# ==============================================================================
# Monitoring & Debugging
# ==============================================================================

.PHONY: health
health: ## Check health of all services
	@echo "Checking service health..."
	@curl -sf http://localhost:8080/health | jq . || echo "MCP Server: ❌ Not responding"
	@curl -sf http://localhost:8081/health | jq . || echo "REST API: ❌ Not responding"
	@curl -sf http://localhost:8082/health >/dev/null 2>&1 && echo "Mock Server: ✅ Healthy" || echo "Mock Server: ⚠️  Not responding (optional)"
	@if command -v docker >/dev/null 2>&1 && docker ps | grep -q database; then \
		docker exec $$(docker ps -q -f name=database | head -1) pg_isready -U dev > /dev/null 2>&1 && echo "PostgreSQL: ✅ Ready" || echo "PostgreSQL: ❌ Not ready"; \
	fi
	@if command -v docker >/dev/null 2>&1 && docker ps | grep -q redis; then \
		docker exec $$(docker ps -q -f name=redis | head -1) redis-cli ping > /dev/null 2>&1 && echo "Redis: ✅ Ready" || echo "Redis: ❌ Not ready"; \
	fi

.PHONY: metrics
metrics: ## Open Grafana dashboard
	open http://localhost:3000

.PHONY: profile-cpu
profile-cpu: ## Profile CPU usage (DURATION=30s)
	go tool pprof -http=:8090 http://localhost:8080/debug/pprof/profile?seconds=${DURATION:-30}

.PHONY: profile-mem
profile-mem: ## Profile memory usage
	go tool pprof -http=:8090 http://localhost:8080/debug/pprof/heap

# ==============================================================================
# Advanced Commands
# ==============================================================================

.PHONY: generate-adapter
generate-adapter: ## Generate new adapter (NAME=adapter_name)
	@if [ -z "${NAME}" ]; then echo "Error: NAME is required. Usage: make generate-adapter NAME=harness"; exit 1; fi
	@mkdir -p pkg/adapters/${NAME}
	@echo "package ${NAME}" > pkg/adapters/${NAME}/adapter.go
	@echo "" >> pkg/adapters/${NAME}/adapter.go
	@echo "// TODO: Implement ${NAME} adapter" >> pkg/adapters/${NAME}/adapter.go
	@echo "package ${NAME}_test" > pkg/adapters/${NAME}/adapter_test.go
	@echo "package ${NAME}" > pkg/adapters/${NAME}/config.go
	@echo "" >> pkg/adapters/${NAME}/config.go
	@echo "type Config struct {" >> pkg/adapters/${NAME}/config.go
	@echo "    // TODO: Add configuration fields" >> pkg/adapters/${NAME}/config.go
	@echo "}" >> pkg/adapters/${NAME}/config.go
	@echo "✅ Adapter skeleton created at pkg/adapters/${NAME}/"

.PHONY: load-test
load-test: ## Run load tests (requires k6)
	@which k6 > /dev/null || (echo "Error: k6 not installed. Run: brew install k6" && exit 1)
	k6 run --vus ${USERS:-10} --duration ${DURATION:-30s} scripts/k6-load-test.js

# ==============================================================================
# E2E Testing Commands
# ==============================================================================
# Examples:
#   make test-e2e-local                    # Run all tests against local Docker
#   make test-e2e-single                   # Run only single agent tests
#   make test-e2e-multi                    # Run only multi-agent tests
#   make quick-test                        # Run a quick smoke test
#   E2E_DEBUG=true make test-e2e-local    # Run with debug logging
# ==============================================================================

.PHONY: test-e2e
test-e2e: ## Run E2E tests against running services
	@echo "Running E2E tests..."
	@if [ ! -d "test/e2e" ]; then \
		echo "❌ Error: test/e2e directory not found"; \
		exit 1; \
	fi
	@cd test/e2e && $(MAKE) test

.PHONY: test-e2e-local
test-e2e-local: health-check-silent ## Run E2E tests against local Docker services
	@echo "Running E2E tests against local environment..."
	@if [ ! -f "test/e2e/.env.local" ]; then \
		echo "⚠️  E2E test configuration not found. Running setup..."; \
		$(MAKE) test-e2e-setup; \
	fi
	@cd test/e2e && E2E_ENVIRONMENT=local $(MAKE) test-local

.PHONY: test-e2e-single
test-e2e-single: ## Run single agent E2E tests
	@cd test/e2e && $(MAKE) test-single

.PHONY: test-e2e-multi
test-e2e-multi: ## Run multi-agent E2E tests
	@cd test/e2e && $(MAKE) test-multi

.PHONY: test-e2e-setup
test-e2e-setup: ## Setup E2E test environment configuration
	@echo "Setting up E2E test environment..."
	@mkdir -p test/e2e
	@if [ ! -f test/e2e/.env.example ]; then \
		echo "# E2E Test Configuration" > test/e2e/.env.example; \
		echo "E2E_API_KEY=your-api-key-here" >> test/e2e/.env.example; \
		echo "#MCP_BASE_URL=mcp.dev-mesh.io" >> test/e2e/.env.example; \
		echo "#API_BASE_URL=api.dev-mesh.io" >> test/e2e/.env.example; \
	fi
	@if [ ! -f test/e2e/.env.local ]; then \
		cp test/e2e/.env.example test/e2e/.env.local; \
		if command -v sed >/dev/null 2>&1; then \
			if [ "$$(uname)" = "Darwin" ]; then \
				sed -i '' 's/your-api-key-here/dev-admin-key-1234567890/g' test/e2e/.env.local; \
				sed -i '' 's|#MCP_BASE_URL=.*|MCP_BASE_URL=http://localhost:8080|g' test/e2e/.env.local; \
				sed -i '' 's|#API_BASE_URL=.*|API_BASE_URL=http://localhost:8081|g' test/e2e/.env.local; \
			else \
				sed -i 's/your-api-key-here/dev-admin-key-1234567890/g' test/e2e/.env.local; \
				sed -i 's|#MCP_BASE_URL=.*|MCP_BASE_URL=http://localhost:8080|g' test/e2e/.env.local; \
				sed -i 's|#API_BASE_URL=.*|API_BASE_URL=http://localhost:8081|g' test/e2e/.env.local; \
			fi; \
		fi; \
		echo "E2E_TENANT_ID=00000000-0000-0000-0000-000000000001" >> test/e2e/.env.local; \
		echo "E2E_DEBUG=true" >> test/e2e/.env.local; \
		echo "E2E_ENVIRONMENT=local" >> test/e2e/.env.local; \
		echo "✅ Created test/e2e/.env.local"; \
	else \
		echo "✅ test/e2e/.env.local already exists"; \
	fi

# ==============================================================================
# Health & Validation Commands
# ==============================================================================
# Commands to check service health and validate environment:
#
# Examples:
#   make health                            # Check all services with output
#   make validate-services                 # Detailed service validation
#   make validate-env                      # Full environment validation
#   make wait-for-healthy                  # Wait for services to be ready
#   TIMEOUT=120 make wait-for-healthy      # Custom timeout (seconds)
# ==============================================================================

.PHONY: health-check-silent
health-check-silent: ## Silent health check for scripts (returns exit code only)
	@curl -sf http://localhost:8080/health > /dev/null && \
	 curl -sf http://localhost:8081/health > /dev/null


.PHONY: wait-for-healthy
wait-for-healthy: ## Wait for all services to become healthy (TIMEOUT=60)
	@echo "Waiting for services to become healthy..."
	@TIMEOUT=$${TIMEOUT:-60}; \
	ELAPSED=0; \
	while [ $$ELAPSED -lt $$TIMEOUT ]; do \
		if $(MAKE) health-check-silent 2>/dev/null; then \
			echo "✅ All services are healthy (took $${ELAPSED}s)"; \
			exit 0; \
		fi; \
		if [ $$ELAPSED -eq 0 ]; then \
			echo "⏳ Waiting for services..."; \
		elif [ $$((ELAPSED % 10)) -eq 0 ]; then \
			echo "⏳ Still waiting... ($${ELAPSED}s/$${TIMEOUT}s)"; \
		fi; \
		sleep 1; \
		ELAPSED=$$((ELAPSED + 1)); \
	done; \
	echo "❌ Services failed to become healthy after $${TIMEOUT}s"; \
	$(MAKE) health; \
	exit 1

# ==============================================================================
# Environment Management
# ==============================================================================
# The system supports two primary environments:
#   1. Local Docker: All services run in Docker containers
#   2. Local AWS: Services run locally but use AWS backends via SSH tunnels
#
# Examples:
#   make env-check                         # Show current environment
#   make env-local && make local-docker    # Switch to Docker environment
#   make env-aws && make local-aws         # Switch to AWS environment
# ==============================================================================

.PHONY: env-check
env-check: ## Check current environment configuration
	@echo "Current Environment Configuration:"
	@echo "================================="
	@echo "ENVIRONMENT: $${ENVIRONMENT:-not set}"
	@echo "EDGE_MCP_URL: $${EDGE_MCP_URL:-not set}"
	@echo "REST_API_URL: $${REST_API_URL:-not set}"
	@echo "DATABASE_HOST: $${DATABASE_HOST:-not set}:$${DATABASE_PORT:-5432}"
	@echo "REDIS_ADDR: $${REDIS_ADDR:-not set}"
	@echo "AWS_REGION: $${AWS_REGION:-not set}"
	@echo "USE_REAL_AWS: $${USE_REAL_AWS:-not set}"
	@echo "USE_LOCALSTACK: $${USE_LOCALSTACK:-not set}"
	@if [ -n "$${ADMIN_API_KEY}" ]; then \
		echo "ADMIN_API_KEY: $${ADMIN_API_KEY:0:20}..."; \
	else \
		echo "ADMIN_API_KEY: not set"; \
	fi
	@if [ -n "$${GITHUB_ACCESS_TOKEN}" ]; then \
		echo "GITHUB_ACCESS_TOKEN: $${GITHUB_ACCESS_TOKEN:0:20}..."; \
	else \
		echo "GITHUB_ACCESS_TOKEN: not set"; \
	fi
	@echo ""
	@if [ -f .env.local ]; then \
		echo "📝 .env.local exists (local overrides active)"; \
	fi

.PHONY: env-local
env-local: ## Configure environment for local Docker development
	@echo "Configuring for local Docker environment..."
	@if [ -f .env ] && [ ! -f .env.backup ]; then \
		cp .env .env.backup; \
		echo "📦 Backed up .env to .env.backup"; \
	fi
	@echo "# Local Docker Environment Overrides" > .env.local
	@echo "# Generated by 'make env-local' on $$(date)" >> .env.local
	@echo "" >> .env.local
	@echo "ENVIRONMENT=local" >> .env.local
	@echo "DATABASE_HOST=database" >> .env.local
	@echo "DATABASE_PORT=5432" >> .env.local
	@echo "DATABASE_NAME=dev" >> .env.local
	@echo "DATABASE_USER=dev" >> .env.local
	@echo "DATABASE_PASSWORD=dev" >> .env.local
	@echo "DATABASE_SSL_MODE=disable" >> .env.local
	@echo "REDIS_HOST=redis" >> .env.local
	@echo "REDIS_PORT=6379" >> .env.local
	@echo "REDIS_ADDR=redis:6379" >> .env.local
	@echo "USE_LOCALSTACK=true" >> .env.local
	@echo "USE_REAL_AWS=false" >> .env.local
	@echo "AWS_ENDPOINT_URL=http://localstack:4566" >> .env.local
	@echo "✅ Created .env.local for Docker environment"
	@echo "💡 Docker Compose will use these overrides automatically"

.PHONY: env-aws
env-aws: ## Configure environment for AWS development (using your existing .env)
	@echo "Using AWS development environment from .env"
	@if [ -f .env.local ]; then \
		rm .env.local; \
		echo "🗑️  Removed .env.local (AWS config from .env will be used)"; \
	else \
		echo "✅ No .env.local found (AWS config from .env will be used)"; \
	fi

# ==============================================================================
# SSH Tunnel Management (AWS Environment)
# ==============================================================================
# SSH tunnels allow local development against AWS RDS and ElastiCache
# Prerequisites: SSH_KEY_PATH, NAT_INSTANCE_IP, RDS_ENDPOINT, ELASTICACHE_ENDPOINT
#
# Examples:
#   make tunnel-all                        # Create all tunnels
#   make tunnel-rds                        # Only RDS tunnel (localhost:5432)
#   make tunnel-redis                      # Only Redis tunnel (localhost:6379)
#   make tunnel-status                     # Check active tunnels
#   make tunnel-kill                       # Terminate all tunnels
# ==============================================================================

# Check if required environment variables are set
.PHONY: check-ssh-vars
check-ssh-vars:
	@if [ -z "$${SSH_KEY_PATH}" ] || [ -z "$${NAT_INSTANCE_IP}" ]; then \
		echo "❌ Error: SSH_KEY_PATH and NAT_INSTANCE_IP must be set in .env"; \
		echo "Example:"; \
		echo "  SSH_KEY_PATH=~/.ssh/bastion-key.pem"; \
		echo "  NAT_INSTANCE_IP=54.x.x.x"; \
		exit 1; \
	fi
	@if [ ! -f "$${SSH_KEY_PATH}" ]; then \
		echo "❌ Error: SSH key not found at $${SSH_KEY_PATH}"; \
		exit 1; \
	fi

.PHONY: tunnel-rds
tunnel-rds: check-ssh-vars ## Create SSH tunnel to RDS (localhost:5432)
	@if lsof -ti:5432 >/dev/null 2>&1; then \
		echo "⚠️  Port 5432 already in use. Checking for existing tunnel..."; \
		if ps aux | grep -v grep | grep -q "ssh.*-L 5432:.*5432"; then \
			echo "✅ RDS tunnel already active"; \
			exit 0; \
		else \
			echo "❌ Port 5432 is in use by another process"; \
			lsof -ti:5432 | xargs ps -p | grep -v PID || true; \
			exit 1; \
		fi; \
	fi
	@echo "Creating SSH tunnel to RDS..."
	@ssh -f -N -L 5432:$(RDS_ENDPOINT):5432 \
		-o StrictHostKeyChecking=no \
		-o UserKnownHostsFile=/dev/null \
		-o ExitOnForwardFailure=yes \
		-i $(SSH_KEY_PATH) \
		ec2-user@$(NAT_INSTANCE_IP) && \
	echo "✅ RDS tunnel created on localhost:5432" || \
	echo "❌ Failed to create RDS tunnel"

.PHONY: tunnel-redis
tunnel-redis: check-ssh-vars ## Create SSH tunnel to ElastiCache (localhost:6379)
	@if lsof -ti:6379 >/dev/null 2>&1; then \
		echo "⚠️  Port 6379 already in use. Checking for existing tunnel..."; \
		if ps aux | grep -v grep | grep -q "ssh.*-L 6379:.*6379"; then \
			echo "✅ Redis tunnel already active"; \
			exit 0; \
		else \
			echo "❌ Port 6379 is in use by another process"; \
			lsof -ti:6379 | xargs ps -p | grep -v PID || true; \
			exit 1; \
		fi; \
	fi
	@echo "Creating SSH tunnel to ElastiCache..."
	@ssh -f -N -L 6379:$(ELASTICACHE_ENDPOINT):6379 \
		-o StrictHostKeyChecking=no \
		-o UserKnownHostsFile=/dev/null \
		-o ExitOnForwardFailure=yes \
		-i $(SSH_KEY_PATH) \
		ec2-user@$(NAT_INSTANCE_IP) && \
	echo "✅ Redis tunnel created on localhost:6379" || \
	echo "❌ Failed to create Redis tunnel"

.PHONY: tunnel-all
tunnel-all: tunnel-rds tunnel-redis ## Create all SSH tunnels

.PHONY: tunnel-status
tunnel-status: ## Check SSH tunnel status
	@echo "SSH Tunnel Status:"
	@echo "=================="
	@if ps aux | grep -v grep | grep -q "ssh.*-L 5432:.*5432"; then \
		echo "✅ RDS tunnel: Active (localhost:5432)"; \
	else \
		echo "❌ RDS tunnel: Not active"; \
	fi
	@if ps aux | grep -v grep | grep -q "ssh.*-L 6379:.*6379"; then \
		echo "✅ Redis tunnel: Active (localhost:6379)"; \
	else \
		echo "❌ Redis tunnel: Not active"; \
	fi
	@echo ""
	@echo "Active SSH tunnels:"
	@ps aux | grep -E "ssh.*-L" | grep -v grep || echo "  No active tunnels"

.PHONY: tunnel-kill
tunnel-kill: ## Kill all SSH tunnels
	@echo "Terminating SSH tunnels..."
	@if ps aux | grep -v grep | grep -q "ssh.*-L"; then \
		pkill -f "ssh.*-L" && echo "✅ SSH tunnels terminated" || echo "⚠️  Some tunnels may still be active"; \
	else \
		echo "ℹ️  No active SSH tunnels found"; \
	fi

# ==============================================================================
# Test Data Management
# ==============================================================================
# Commands for seeding and managing test data:
#
# Examples:
#   make seed-test-data                    # Seed initial test data
#   make reset-test-data                   # Clear execution history
#   make create-seed-script                # Generate seed SQL script
#
# The seed data includes:
#   - Test tenants (00000000-0000-0000-0000-000000000001, -000002)
#   - Test agents (code-agent, security-agent, devops-agent)
#   - Test models (claude-3-opus, gpt-4)
#   - No tool configurations (tools should be added dynamically)
# ==============================================================================

.PHONY: seed-test-data
seed-test-data: ## Seed test data for local development
	@echo "Seeding test data..."
	@if [ ! -f scripts/db/seed-test-data.sql ]; then \
		echo "Creating seed data script..."; \
		$(MAKE) create-seed-script; \
	fi
	@if [ "$${USE_REAL_AWS}" = "true" ] || [ "$${DATABASE_HOST}" = "localhost" ]; then \
		echo "Seeding data to AWS RDS via localhost tunnel..."; \
		PGPASSWORD=$${DATABASE_PASSWORD} psql -h localhost -p 5432 -U $${DATABASE_USER} -d $${DATABASE_NAME} -f scripts/db/seed-test-data.sql 2>&1 | grep -E "(INSERT|already exists|NOTICE)" || true; \
	else \
		echo "Seeding data to Docker PostgreSQL..."; \
		docker exec -i $$(docker ps -q -f name=database | head -1) psql -U dev -d dev < scripts/db/seed-test-data.sql 2>&1 | grep -E "(INSERT|already exists|NOTICE)" || true; \
	fi
	@echo "✅ Test data seeded successfully"

.PHONY: create-seed-script
create-seed-script: ## Create the seed data SQL script
	@mkdir -p scripts/db
	@if [ ! -f scripts/db/seed-test-data.sql ]; then \
		./scripts/local/create-seed-script.sh; \
		echo "✅ Created scripts/db/seed-test-data.sql"; \
	else \
		echo "✅ scripts/db/seed-test-data.sql already exists"; \
	fi

.PHONY: reset-test-data
reset-test-data: ## Reset test data to clean state
	@echo "Resetting test data..."
	@read -p "⚠️  This will delete test data. Continue? [y/N] " -n 1 -r; \
	echo ""; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		if [ "$${USE_REAL_AWS}" = "true" ] || [ "$${DATABASE_HOST}" = "localhost" ]; then \
			PGPASSWORD=$${DATABASE_PASSWORD} psql -h localhost -p 5432 -U $${DATABASE_USER} -d $${DATABASE_NAME} -c \
				"DELETE FROM tasks WHERE tenant_id IN ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002'); \
				 DELETE FROM workflow_executions WHERE tenant_id IN ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002'); \
				 DELETE FROM workflows WHERE tenant_id IN ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002'); \
				 DELETE FROM tool_executions WHERE tenant_id IN ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002');"; \
		else \
			docker exec -i $$(docker ps -q -f name=database | head -1) psql -U dev -d dev -c \
				"DELETE FROM tasks WHERE tenant_id IN ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002'); \
				 DELETE FROM workflow_executions WHERE tenant_id IN ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002'); \
				 DELETE FROM workflows WHERE tenant_id IN ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002'); \
				 DELETE FROM tool_executions WHERE tenant_id IN ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002');"; \
		fi; \
		echo "✅ Test data reset"; \
	else \
		echo "❌ Reset cancelled"; \
	fi

# ==============================================================================
# Complete Local Development Workflows
# ==============================================================================
# These are one-command setups for different development scenarios:
#
# Docker-only workflow (no AWS dependencies):
#   make local-docker                      # Full setup with Docker
#   make test-e2e-local                    # Run tests
#   make docker-reset                      # Reset everything
#
# AWS-integrated workflow (uses real AWS services):
#   make local-aws                         # Setup with AWS backends
#   make run-edge-mcp                      # In terminal 1
#   make run-rest-api                      # In terminal 2
#   make run-worker                        # In terminal 3
#   make test-e2e-local                    # Run tests
# ==============================================================================

.PHONY: local-docker
local-docker: docker-clean wait-for-healthy migrate-up-docker seed-test-data test-e2e-setup ## Complete local Docker setup with E2E tests
	@echo ""
	@echo "🚀 Local Docker environment ready!"
	@echo "================================="
	@echo "Services:"
	@echo "  MCP Server:   http://localhost:8080"
	@echo "  REST API:     http://localhost:8081"
	@echo "  Mock Server:  http://localhost:8082"
	@echo "  PostgreSQL:   localhost:5432 (user: dev, pass: dev)"
	@echo "  Redis:        localhost:6379"
	@echo ""
	@echo "Next steps:"
	@echo "  make test-e2e-local    # Run E2E tests"
	@echo "  make logs service=edge-mcp    # View specific service logs"
	@echo "  make health           # Check service health"

.PHONY: local-aws
local-aws: env-aws validate-environment tunnel-all local-aws-wait test-e2e-setup ## Local development with AWS services
	@echo ""
	@echo "🚀 Local AWS environment ready!"
	@echo "==============================="
	@echo "Using AWS Services:"
	@echo "  RDS:          via SSH tunnel on localhost:5432"
	@echo "  ElastiCache:  via SSH tunnel on localhost:6379"
	@echo "  S3:           Direct AWS access"
	@echo "  Bedrock:      Direct AWS access"
	@echo ""
	@echo "Start services manually:"
	@echo "  make run-edge-mcp      # Terminal 1"
	@echo "  make run-rest-api      # Terminal 2"
	@echo "  make run-worker        # Terminal 3 (optional)"
	@echo ""
	@echo "Or use Docker with AWS backends:"
	@echo "  make up"
	@echo ""
	@echo "Then run tests:"
	@echo "  make test-e2e-aws"

.PHONY: local-aws-wait
local-aws-wait: ## Wait for AWS services via tunnels
	@echo "Checking AWS service connectivity..."
	@ATTEMPTS=0; \
	MAX_ATTEMPTS=10; \
	while [ $$ATTEMPTS -lt $$MAX_ATTEMPTS ]; do \
		if pg_isready -h localhost -p 5432 -U $${DATABASE_USER:-dbadmin} >/dev/null 2>&1; then \
			echo "✅ RDS is accessible via tunnel"; \
			break; \
		fi; \
		ATTEMPTS=$$((ATTEMPTS + 1)); \
		echo "⏳ Waiting for RDS tunnel... ($$ATTEMPTS/$$MAX_ATTEMPTS)"; \
		sleep 2; \
	done; \
	if [ $$ATTEMPTS -eq $$MAX_ATTEMPTS ]; then \
		echo "❌ RDS tunnel failed to connect"; \
		exit 1; \
	fi
	@redis-cli -h localhost -p 6379 ping >/dev/null 2>&1 && echo "✅ Redis is accessible via tunnel" || echo "⚠️  Redis not accessible (optional)"

.PHONY: docker-clean
docker-clean: ## Clean and restart Docker environment
	@echo "Cleaning Docker environment..."
	$(DOCKER_COMPOSE) down -v
	@docker system prune -f --volumes 2>/dev/null || true
	$(DOCKER_COMPOSE) up -d
	@echo "✅ Docker environment cleaned and restarted"

.PHONY: test-e2e-aws
test-e2e-aws: test-e2e-setup ## Run E2E tests against services with AWS backends
	@echo "Running E2E tests with AWS backends..."
	@cd test/e2e && E2E_ENVIRONMENT=aws $(MAKE) test

# ==============================================================================
# Validation Commands
# ==============================================================================

.PHONY: validate-environment
validate-environment: ## Validate environment configuration
	@echo "Validating environment configuration..."
	@./scripts/local/validate-environment.sh || (echo "❌ Environment validation failed" && exit 1)

.PHONY: validate-all
validate-all: validate-environment health ## Validate everything
	@echo "✅ All validations passed"

# ==============================================================================
# Quick Development Commands
# ==============================================================================

.PHONY: quick-test
quick-test: ## Quick single test for rapid iteration
	@if ! $(MAKE) health-check-silent 2>/dev/null; then \
		echo "❌ Services not running. Start with 'make dev' or 'make up'"; \
		exit 1; \
	fi
	@echo "Running quick test..."
	@cd test/e2e && \
	E2E_ENVIRONMENT=local \
	E2E_API_KEY=$${E2E_API_KEY:-dev-admin-key-1234567890} \
	MCP_BASE_URL=$${MCP_BASE_URL:-http://localhost:8080} \
	API_BASE_URL=$${API_BASE_URL:-http://localhost:8081} \
	ginkgo -v --focus="Single Agent.*Basic Operations.*should register agent and receive acknowledgment" ./scenarios

.PHONY: fix-multiagent
fix-multiagent: ## Test the multi-agent workflow fix
	@if ! $(MAKE) health-check-silent 2>/dev/null; then \
		echo "❌ Services not running. Start with 'make dev' or 'make up'"; \
		exit 1; \
	fi
	@echo "Testing multi-agent workflow creation..."
	@cd test/e2e && \
	E2E_ENVIRONMENT=local \
	E2E_API_KEY=$${E2E_API_KEY:-dev-admin-key-1234567890} \
	MCP_BASE_URL=$${MCP_BASE_URL:-http://localhost:8080} \
	API_BASE_URL=$${API_BASE_URL:-http://localhost:8081} \
	ginkgo -v --focus="Code Review Workflow" ./scenarios


.PHONY: reset-all
reset-all: docker-clean reset-test-data ## Reset everything (Docker + test data)
	@echo "✅ Complete reset done"

# ==============================================================================
# IDE Agent Testing
# ==============================================================================

.PHONY: test-agent-mcp
test-agent-mcp: ## Test agent GitHub operations via MCP (read-only, with embeddings)
	@echo "Testing agent MCP integration with GitHub..."
	@bash ./scripts/test-agent-github-mcp.sh

.PHONY: test-ide-github
test-ide-github: ## Test local MCP client to DevMesh connection (simulates IDE integration)
	@echo "Testing local MCP client connection to DevMesh server..."
	@./scripts/test-ide-github-integration.sh

.PHONY: demo-ide-agent
demo-ide-agent: ## Run IDE agent demo with GitHub
	@echo "Running IDE agent demo..."
	@go run examples/ide_agent_github_demo.go


.PHONY: all test test-single test-multi test-performance test-local test-ci install-deps clean help

# Load .env file if it exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Variables
GINKGO := $(shell which ginkgo 2>/dev/null)
GO := go
TIMEOUT := 30m
PARALLEL := 5
REPORT_DIR := test-results

# Default target
all: test

# Install test dependencies
install-deps:
	@echo "Installing test dependencies..."
	cd ../.. && $(GO) install github.com/onsi/ginkgo/v2/ginkgo@v2.23.3
	$(GO) mod download

# Run all E2E tests
test: install-deps
	@echo "Running all E2E tests..."
	@mkdir -p $(REPORT_DIR)
	$(GINKGO) -v \
		-r \
		--procs=$(PARALLEL) \
		--timeout=$(TIMEOUT) \
		--flake-attempts=2 \
		--json-report=report.json \
		--junit-report=junit.xml \
		.

# Run single agent tests
test-single: install-deps
	@echo "Running single agent tests..."
	@mkdir -p $(REPORT_DIR)
	$(GINKGO) -v \
		-r \
		--focus="Single Agent" \
		--timeout=$(TIMEOUT) \
		--json-report=single-report.json \
		--junit-report=single-junit.xml \
		.

# Run multi-agent tests
test-multi: install-deps
	@echo "Running multi-agent tests..."
	@mkdir -p $(REPORT_DIR)
	$(GINKGO) -v \
		-r \
		--focus="Multi-Agent" \
		--timeout=$(TIMEOUT) \
		--json-report=multi-report.json \
		--junit-report=multi-junit.xml \
		.

# Run performance tests
test-performance: install-deps
	@echo "Running performance tests..."
	@mkdir -p $(REPORT_DIR)
	$(GINKGO) -v \
		-r \
		--focus="Performance" \
		--timeout=60m \
		--json-report=performance-report.json \
		--junit-report=performance-junit.xml \
		.

# Run tests against local environment
test-local:
	@echo "Running tests against local environment..."
	@if [ -f .env.local ]; then \
		echo "Loading configuration from .env.local"; \
		set -a; source .env.local; set +a; \
	fi; \
	E2E_ENVIRONMENT=$${E2E_ENVIRONMENT:-local} \
	MCP_BASE_URL=$${MCP_BASE_URL:-http://localhost:8080} \
	API_BASE_URL=$${API_BASE_URL:-http://localhost:8081} \
	E2E_API_KEY=$${E2E_API_KEY:-dev-admin-key-1234567890} \
	E2E_DEBUG=$${E2E_DEBUG:-true} \
	$(MAKE) test

# Run tests in CI mode
test-ci:
	@echo "Running tests in CI mode..."
	@mkdir -p $(REPORT_DIR)
	E2E_PARALLEL_TESTS=3 \
	E2E_REPORT_DIR=$(REPORT_DIR) \
	$(GINKGO) -v \
		-r \
		--procs=3 \
		--timeout=$(TIMEOUT) \
		--flake-attempts=3 \
		--keep-going \
		--race \
		--trace \
		--json-report=ci-report.json \
		--junit-report=ci-junit.xml \
		.

# Clean test artifacts
clean:
	@echo "Cleaning test artifacts..."
	@rm -rf $(REPORT_DIR)
	@rm -f *.log
	@rm -f *.out

# Watch mode for development
watch: install-deps
	@echo "Running tests in watch mode..."
	$(GINKGO) watch -v .

# Generate test report
report:
	@echo "Generating test report..."
	@if [ -f "$(REPORT_DIR)/report.json" ]; then \
		echo "Test results available at $(REPORT_DIR)/report.html"; \
	else \
		echo "No test results found. Run 'make test' first."; \
	fi

# Run specific test by name
test-specific:
ifndef TEST_NAME
	@echo "Please specify TEST_NAME. Example: make test-specific TEST_NAME='should connect multiple agents'"
else
	@echo "Running specific test: $(TEST_NAME)"
	$(GINKGO) -v \
		--focus="$(TEST_NAME)" \
		--timeout=$(TIMEOUT) \
		.
endif

# Benchmark tests
benchmark: install-deps
	@echo "Running benchmark tests..."
	@mkdir -p $(REPORT_DIR)
	$(GO) test -bench=. -benchmem -benchtime=10s -run=^$$ ./... | tee $(REPORT_DIR)/benchmark.txt

# Coverage report
coverage: install-deps
	@echo "Running tests with coverage..."
	@mkdir -p $(REPORT_DIR)
	$(GINKGO) -v \
		--cover \
		--coverprofile=$(REPORT_DIR)/coverage.out \
		--output-dir=$(REPORT_DIR) \
		.
	$(GO) tool cover -html=$(REPORT_DIR)/coverage.out -o $(REPORT_DIR)/coverage.html
	@echo "Coverage report available at $(REPORT_DIR)/coverage.html"

# Validate test setup
validate:
	@echo "Validating E2E test setup..."
	@echo "Checking environment variables..."
	@[ -n "$$MCP_BASE_URL" ] && echo "✓ MCP_BASE_URL is set" || echo "✗ MCP_BASE_URL is not set"
	@[ -n "$$API_BASE_URL" ] && echo "✓ API_BASE_URL is set" || echo "✗ API_BASE_URL is not set"
	@[ -n "$$E2E_API_KEY" ] && echo "✓ E2E_API_KEY is set" || echo "✗ E2E_API_KEY is not set"
	@echo ""
	@echo "Checking tools..."
	@which ginkgo > /dev/null && echo "✓ ginkgo is installed" || echo "✗ ginkgo is not installed"
	@which go > /dev/null && echo "✓ go is installed" || echo "✗ go is not installed"

# Help
help:
	@echo "Developer Mesh E2E Test Suite"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all               Run all E2E tests (default)"
	@echo "  test              Run all E2E tests"
	@echo "  test-single       Run single agent tests only"
	@echo "  test-multi        Run multi-agent tests only"
	@echo "  test-performance  Run performance tests only"
	@echo "  test-local        Run tests against local environment"
	@echo "  test-ci           Run tests in CI mode"
	@echo "  test-specific     Run specific test by name (TEST_NAME=...)"
	@echo "  watch             Run tests in watch mode"
	@echo "  benchmark         Run benchmark tests"
	@echo "  coverage          Run tests with coverage report"
	@echo "  report            Generate HTML test report"
	@echo "  validate          Validate test setup"
	@echo "  clean             Clean test artifacts"
	@echo "  help              Show this help message"
	@echo ""
	@echo "Environment Variables:"
	@echo "  MCP_BASE_URL      MCP server URL (default: mcp.dev-mesh.io)"
	@echo "  API_BASE_URL      API server URL (default: api.dev-mesh.io)"
	@echo "  E2E_API_KEY       API key for authentication"
	@echo "  E2E_TENANT_ID     Tenant ID for test isolation"
	@echo "  E2E_DEBUG         Enable debug logging (true/false)"
	@echo "  E2E_PARALLEL_TESTS Number of parallel test specs"
	@echo ""
	@echo "Examples:"
	@echo "  make test                          # Run all tests"
	@echo "  make test-performance              # Run performance tests only"
	@echo "  E2E_DEBUG=true make test-single    # Run single agent tests with debug"
	@echo "  make test-specific TEST_NAME='should handle reconnection' # Run specific test"
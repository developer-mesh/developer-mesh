# Edge MCP Makefile

.PHONY: all build test clean run docker-build docker-run help

# Variables
BINARY_NAME=edge-mcp
VERSION?=1.0.0
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -s -w"

# Default target
all: test build

# Build the binary
build:
	@echo "Building ${BINARY_NAME}..."
	@CGO_ENABLED=0 go build ${LDFLAGS} -o bin/${BINARY_NAME} cmd/server/main.go
	@echo "Build complete: bin/${BINARY_NAME}"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -cover ./...

# Run integration tests
test-integration: build
	@echo "Running integration tests..."
	@../../scripts/test-edge-mcp.sh

# Run the server
run: build
	@echo "Starting Edge MCP..."
	@./bin/${BINARY_NAME}

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean -cache

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t edge-mcp:${VERSION} -t edge-mcp:latest .

# Run Docker container
docker-run: docker-build
	@echo "Running Docker container..."
	@docker run -p 8082:8082 \
		-e EDGE_MCP_API_KEY=${EDGE_MCP_API_KEY} \
		-e CORE_PLATFORM_URL=${CORE_PLATFORM_URL} \
		edge-mcp:latest

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@gofmt -s -w .

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run ./...

# Generate test coverage report
coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Cross-platform builds
build-all:
	@echo "Building for all platforms..."
	@GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-darwin-amd64 cmd/server/main.go
	@GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-darwin-arm64 cmd/server/main.go
	@GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-linux-amd64 cmd/server/main.go
	@GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-linux-arm64 cmd/server/main.go
	@GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-windows-amd64.exe cmd/server/main.go
	@echo "Cross-platform builds complete"

# Help target
help:
	@echo "Edge MCP Makefile targets:"
	@echo "  make build       - Build the binary"
	@echo "  make test        - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make run         - Build and run the server"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run  - Run Docker container"
	@echo "  make deps        - Install dependencies"
	@echo "  make fmt         - Format code"
	@echo "  make lint        - Lint code"
	@echo "  make coverage    - Generate test coverage report"
	@echo "  make build-all   - Build for all platforms"
	@echo "  make help        - Show this help message"

# Default help
.DEFAULT_GOAL := help
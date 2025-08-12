# Edge MCP Implementation Plan

## Executive Summary

This document outlines the implementation plan for creating lightweight Edge MCPs (Model Context Protocol servers) that can run locally on developer machines without infrastructure dependencies, while also completing the tenant tool credentials migration for proper multi-tenant isolation.

## Goals

1. **Complete Tenant Tool Credentials Migration** - Enable tenant-specific tool authentication
2. **Create Edge MCP Template** - Lightweight, standalone MCP servers from existing codebase
3. **Enable Direct IDE Integration** - Allow Claude Code, Cursor, and Windsurf to connect directly to local Edge MCPs
4. **Maintain Zero Infrastructure** - Edge MCPs run without Redis, PostgreSQL, or cloud services

## Architecture Overview

```
Developer Machine
├── IDE (Claude Code/Cursor/Windsurf)
│   ├── → Edge MCP: GitHub (localhost:8082)
│   ├── → Edge MCP: Kubernetes (localhost:8083)
│   └── → Edge MCP: Custom Tools (localhost:8084)
│
└── Optional: Connect to Core Platform (remote)
    └── Full DevOps MCP with Redis, PostgreSQL, AWS
```

## Phase 1: Tenant Tool Credentials Migration (Day 1)

### 1.1 Database Migration
**File**: `apps/rest-api/migrations/sql/000026_tenant_tool_credentials.up.sql`

```sql
-- Enable tenant-specific tool credentials
BEGIN;

-- Add tenant-specific credentials column to tool_configurations
ALTER TABLE mcp.tool_configurations 
ADD COLUMN IF NOT EXISTS tenant_credentials JSONB DEFAULT '{}';

-- Create index for efficient tenant+tool lookups
CREATE INDEX IF NOT EXISTS idx_tool_configurations_tenant_credentials 
ON mcp.tool_configurations(tenant_id, tool_name) 
WHERE tenant_credentials IS NOT NULL;

-- Add comment for documentation
COMMENT ON COLUMN mcp.tool_configurations.tenant_credentials IS 
'Encrypted tenant-specific credentials for tool access. Structure: {"tenant_uuid": {"api_key": "encrypted_value", "secret": "encrypted_value"}}';

-- Migrate existing credentials to tenant-specific format (if any exist)
UPDATE mcp.tool_configurations 
SET tenant_credentials = jsonb_build_object(
    tenant_id::text, 
    COALESCE(credential_config, '{}'::jsonb)
)
WHERE credential_config IS NOT NULL 
AND tenant_credentials = '{}'::jsonb;

COMMIT;
```

### 1.2 Down Migration
**File**: `apps/rest-api/migrations/sql/000026_tenant_tool_credentials.down.sql`

```sql
BEGIN;
ALTER TABLE mcp.tool_configurations DROP COLUMN IF EXISTS tenant_credentials;
DROP INDEX IF EXISTS idx_tool_configurations_tenant_credentials;
COMMIT;
```

### 1.3 Service Layer Updates
- Update `DynamicToolsService` to handle tenant-specific credentials
- Modify credential encryption/decryption logic
- Add validation for tenant credential access

## Phase 2: Edge MCP Template Creation (Day 1-2)

### 2.1 Directory Structure
```
edge-mcp-template/
├── cmd/
│   └── edge-mcp/
│       └── main.go              # Minimal entry point
├── internal/
│   ├── server/
│   │   ├── mcp_handler.go       # Simplified MCP protocol handler
│   │   └── websocket.go         # WebSocket server
│   ├── cache/
│   │   └── memory.go            # In-memory cache implementation
│   └── tools/
│       └── registry.go          # Tool registration system
├── pkg/
│   └── protocol/
│       └── mcp.go               # MCP protocol types
├── examples/
│   ├── github-mcp/
│   │   ├── main.go
│   │   ├── tools.go
│   │   └── config.yaml
│   ├── kubernetes-mcp/
│   │   ├── main.go
│   │   ├── tools.go
│   │   └── config.yaml
│   └── filesystem-mcp/
│       ├── main.go
│       ├── tools.go
│       └── config.yaml
├── go.mod                        # Go module definition
├── go.sum                        # Go module checksums
├── config.yaml                  # Minimal configuration
├── Dockerfile                    # For containerized deployment
├── Makefile
└── README.md
```

### 2.2 Core Components to Extract - DETAILED

**Critical Files to Copy (with modifications needed):**

1. **MCP Protocol Handler** (`apps/mcp-server/internal/api/mcp_protocol.go`)
   - Lines to keep: 1-100 (core types), 500-800 (message handling)
   - Remove: Redis cache calls (lines 82-86), database queries, REST API client calls
   - Replace: `h.restAPIClient` calls with local tool execution

2. **WebSocket Server** (`apps/mcp-server/internal/api/websocket/server.go`)
   - Keep: WebSocket upgrade logic, connection handling
   - Remove: Database repository initialization, Redis connections
   - Simplify: Authentication to basic API key check

3. **Connection Management** (`apps/mcp-server/internal/api/websocket/connection.go`)
   - Keep: Connection state, message routing
   - Remove: Agent registry database calls

4. **Memory Cache** (`pkg/common/cache/memory.go`)
   - Use as-is, no modifications needed

5. **Logger** (`pkg/observability/logger.go`)
   - Keep: Basic logging functions
   - Remove: Distributed tracing, metrics collection

**Specific Imports to Remove:**
```go
// Remove these imports from extracted files:
import (
    "github.com/developer-mesh/developer-mesh/pkg/database"
    "github.com/developer-mesh/developer-mesh/pkg/repository"
    "github.com/developer-mesh/developer-mesh/pkg/clients"
    "github.com/developer-mesh/developer-mesh/pkg/common/aws"
    "github.com/go-redis/redis/v8"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)
```

**New go.mod for Edge MCP:**
```go
module github.com/developer-mesh/edge-mcp

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/gorilla/websocket v1.5.1
    github.com/google/uuid v1.5.0
    github.com/spf13/viper v1.18.2
)
```

### 2.3 Components to Remove
- ❌ PostgreSQL connections
- ❌ Redis dependencies  
- ❌ S3 context manager
- ❌ AWS integrations
- ❌ Database migrations
- ❌ Complex authentication (use simple API keys)
- ❌ Distributed tracing
- ❌ Metrics collection (optional)

## Phase 3: Implementation Steps (Day 2-3)

### 3.1 Create Base Edge MCP with Complete Details

```go
// edge-mcp-template/cmd/edge-mcp/main.go
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "edge-mcp/internal/server"
    "edge-mcp/internal/cache"
)

func main() {
    // Command line flags
    port := flag.String("port", "8082", "Port to listen on")
    apiKey := flag.String("api-key", "", "API key for authentication (optional)")
    logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
    flag.Parse()
    
    // Check if port is available
    if !isPortAvailable(*port) {
        // Try next port
        nextPort := findAvailablePort(parsePort(*port) + 1)
        log.Printf("Port %s is in use, using %d instead", *port, nextPort)
        *port = fmt.Sprintf("%d", nextPort)
    }
    
    // Create in-memory cache (no Redis!)
    memCache := cache.NewMemoryCache(1000, 5*time.Minute)
    
    // Create MCP server with minimal config
    srv := server.NewEdgeMCPServer(&server.Config{
        Port:     *port,
        Cache:    memCache,
        APIKey:   *apiKey,
        LogLevel: *logLevel,
        Name:     "edge-mcp",
        Version:  "1.0.0",
    })
    
    // Register your tools here
    srv.RegisterTool("example.hello", handleHello)
    srv.RegisterTool("example.echo", handleEcho)
    
    // Setup health check endpoint
    srv.RegisterHealthCheck("/health", func() error {
        // Custom health check logic
        return nil
    })
    
    // Start server
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    if err := srv.Start(ctx); err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Edge MCP server started on ws://localhost:%s/ws", *port)
    log.Printf("Health check available at http://localhost:%s/health", *port)
    
    // Graceful shutdown handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    <-sigChan
    log.Println("Shutting down Edge MCP server...")
    
    // Give connections 5 seconds to close
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer shutdownCancel()
    
    if err := srv.Shutdown(shutdownCtx); err != nil {
        log.Printf("Error during shutdown: %v", err)
    }
    
    log.Println("Edge MCP server stopped")
}

func isPortAvailable(port string) bool {
    ln, err := net.Listen("tcp", ":"+port)
    if err != nil {
        return false
    }
    ln.Close()
    return true
}

func findAvailablePort(start int) int {
    for port := start; port < 65535; port++ {
        if isPortAvailable(fmt.Sprintf("%d", port)) {
            return port
        }
    }
    return 0
}

func parsePort(port string) int {
    var p int
    fmt.Sscanf(port, "%d", &p)
    return p
}
```

### 3.2 Simplified MCP Handler with Full Protocol Support

```go
// edge-mcp-template/internal/server/mcp_handler.go
package server

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"
)

// MCP Protocol Version
const MCPProtocolVersion = "2025-06-18"

// MCP Error Codes (JSON-RPC 2.0)
const (
    ErrorParseError     = -32700
    ErrorInvalidRequest = -32600
    ErrorMethodNotFound = -32601
    ErrorInvalidParams  = -32602
    ErrorInternalError  = -32603
)

type MCPHandler struct {
    tools      map[string]ToolDefinition
    toolsMu    sync.RWMutex
    cache      cache.Cache
    logger     logger.Logger
    sessions   map[string]*Session
    sessionsMu sync.RWMutex
}

type ToolDefinition struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"inputSchema"`
    Handler     ToolHandler           `json:"-"`
}

type ToolHandler func(ctx context.Context, args json.RawMessage) (interface{}, error)

type Session struct {
    ID           string
    Initialized  bool
    ClientInfo   ClientInfo
    CreatedAt    time.Time
    LastActivity time.Time
}

type ClientInfo struct {
    Name    string `json:"name"`
    Version string `json:"version"`
}

// HandleMessage processes incoming MCP messages
func (h *MCPHandler) HandleMessage(ctx context.Context, sessionID string, msg *MCPMessage) (*MCPResponse, error) {
    // Update session activity
    h.updateSessionActivity(sessionID)
    
    // Log the incoming message for debugging
    h.logger.Debug("Received MCP message", map[string]interface{}{
        "method":     msg.Method,
        "id":         msg.ID,
        "session_id": sessionID,
    })
    
    // Route based on method
    switch msg.Method {
    case "initialize":
        return h.handleInitialize(ctx, sessionID, msg)
    case "initialized":
        return h.handleInitialized(ctx, sessionID, msg)
    case "ping":
        return h.handlePing(ctx, msg)
    case "tools/list":
        return h.handleToolsList(ctx, sessionID, msg)
    case "tools/call":
        return h.handleToolCall(ctx, sessionID, msg)
    case "resources/list":
        return h.handleResourcesList(ctx, msg)
    case "resources/read":
        return h.handleResourceRead(ctx, msg)
    case "shutdown":
        return h.handleShutdown(ctx, sessionID, msg)
    default:
        return h.errorResponse(msg.ID, ErrorMethodNotFound, 
            fmt.Sprintf("Method not found: %s", msg.Method), nil)
    }
}

// handleInitialize handles the MCP initialization handshake
func (h *MCPHandler) handleInitialize(ctx context.Context, sessionID string, msg *MCPMessage) (*MCPResponse, error) {
    var params struct {
        ProtocolVersion string     `json:"protocolVersion"`
        ClientInfo      ClientInfo `json:"clientInfo"`
    }
    
    if err := json.Unmarshal(msg.Params, &params); err != nil {
        return h.errorResponse(msg.ID, ErrorInvalidParams, "Invalid initialize params", err)
    }
    
    // Validate protocol version
    if params.ProtocolVersion != MCPProtocolVersion {
        return h.errorResponse(msg.ID, ErrorInvalidRequest, 
            fmt.Sprintf("Unsupported protocol version: %s (expected %s)", 
                params.ProtocolVersion, MCPProtocolVersion), nil)
    }
    
    // Create or update session
    h.sessionsMu.Lock()
    h.sessions[sessionID] = &Session{
        ID:           sessionID,
        Initialized:  false, // Will be set true on 'initialized' message
        ClientInfo:   params.ClientInfo,
        CreatedAt:    time.Now(),
        LastActivity: time.Now(),
    }
    h.sessionsMu.Unlock()
    
    // Return capabilities
    return &MCPResponse{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result: map[string]interface{}{
            "protocolVersion": MCPProtocolVersion,
            "serverInfo": map[string]interface{}{
                "name":    "edge-mcp",
                "version": "1.0.0",
            },
            "capabilities": map[string]interface{}{
                "tools": map[string]interface{}{
                    "listChanged": true,
                },
                "resources": map[string]interface{}{
                    "subscribe":    false, // Edge MCPs don't need subscriptions
                    "listChanged": false,
                },
                "prompts": map[string]interface{}{},
                "logging": map[string]interface{}{},
            },
        },
    }, nil
}

// handleToolCall executes a tool
func (h *MCPHandler) handleToolCall(ctx context.Context, sessionID string, msg *MCPMessage) (*MCPResponse, error) {
    var params struct {
        Name      string          `json:"name"`
        Arguments json.RawMessage `json:"arguments"`
    }
    
    if err := json.Unmarshal(msg.Params, &params); err != nil {
        return h.errorResponse(msg.ID, ErrorInvalidParams, "Invalid tool call params", err)
    }
    
    // Get tool definition
    h.toolsMu.RLock()
    tool, exists := h.tools[params.Name]
    h.toolsMu.RUnlock()
    
    if !exists {
        return h.errorResponse(msg.ID, ErrorMethodNotFound, 
            fmt.Sprintf("Tool not found: %s", params.Name), nil)
    }
    
    // Execute tool with timeout
    execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    result, err := tool.Handler(execCtx, params.Arguments)
    if err != nil {
        return h.errorResponse(msg.ID, ErrorInternalError, 
            fmt.Sprintf("Tool execution failed: %v", err), err)
    }
    
    // Return successful result
    return &MCPResponse{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result: map[string]interface{}{
            "content": []map[string]interface{}{
                {
                    "type": "text",
                    "text": fmt.Sprintf("%v", result),
                },
            },
        },
    }, nil
}

// errorResponse creates a standard JSON-RPC error response
func (h *MCPHandler) errorResponse(id interface{}, code int, message string, err error) (*MCPResponse, error) {
    errorData := map[string]interface{}{}
    if err != nil {
        errorData["details"] = err.Error()
    }
    
    return &MCPResponse{
        JSONRPC: "2.0",
        ID:      id,
        Error: &MCPError{
            Code:    code,
            Message: message,
            Data:    errorData,
        },
    }, nil
}
```

## Phase 4: Example Edge MCPs (Day 3-4)

### 4.1 GitHub Edge MCP
```go
// edge-mcp-template/examples/github-mcp/main.go
func main() {
    srv := server.NewEdgeMCPServer(&server.Config{
        Port: "8082",
        Name: "github-edge-mcp",
    })
    
    // Register GitHub tools
    srv.RegisterTool("github.create_issue", createIssue)
    srv.RegisterTool("github.create_pr", createPR)
    srv.RegisterTool("github.list_repos", listRepos)
    srv.RegisterTool("github.get_pr_status", getPRStatus)
    
    srv.Start()
}
```

### 4.2 Kubernetes Edge MCP
```go
// edge-mcp-template/examples/kubernetes-mcp/main.go
func main() {
    srv := server.NewEdgeMCPServer(&server.Config{
        Port: "8083",
        Name: "kubernetes-edge-mcp",
    })
    
    // Register K8s tools
    srv.RegisterTool("k8s.list_pods", listPods)
    srv.RegisterTool("k8s.deploy", deploy)
    srv.RegisterTool("k8s.scale", scale)
    srv.RegisterTool("k8s.logs", getLogs)
    
    srv.Start()
}
```

### 4.3 File System Edge MCP
```go
// edge-mcp-template/examples/filesystem-mcp/main.go
func main() {
    srv := server.NewEdgeMCPServer(&server.Config{
        Port: "8084",
        Name: "filesystem-edge-mcp",
    })
    
    // Register file system tools
    srv.RegisterTool("fs.read", readFile)
    srv.RegisterTool("fs.write", writeFile)
    srv.RegisterTool("fs.list", listFiles)
    srv.RegisterTool("fs.search", searchFiles)
    
    srv.Start()
}
```

## Phase 5: IDE Integration (Day 4)

### 5.1 Claude Code Configuration
```json
// .claude/config.json
{
  "mcpServers": {
    "github": {
      "url": "ws://localhost:8082/ws",
      "apiKey": "optional-local-key"
    },
    "kubernetes": {
      "url": "ws://localhost:8083/ws"
    },
    "filesystem": {
      "url": "ws://localhost:8084/ws"
    }
  }
}
```

### 5.2 Cursor Configuration
```json
// .cursor/mcp-config.json
{
  "servers": [
    {
      "name": "github",
      "endpoint": "ws://localhost:8082/ws"
    }
  ]
}
```

## Phase 6: Testing Strategy (Day 4-5)

### 6.1 Unit Tests
- Test MCP protocol compliance
- Test tool registration and execution
- Test in-memory cache operations
- Test WebSocket connection handling

### 6.2 Integration Tests
- Test Claude Code connection
- Test tool discovery
- Test tool execution
- Test multiple concurrent connections

### 6.3 E2E Tests with Correct Protocol
```bash
#!/bin/bash
# test-edge-mcp.sh - Complete E2E test script

set -e

# Configuration
PORT="${PORT:-8082}"
WS_URL="ws://localhost:${PORT}/ws"

# Start edge MCP
echo "Starting Edge MCP on port $PORT..."
./edge-mcp -port $PORT &
MCP_PID=$!

# Wait for server to start
sleep 2

# Function to test MCP method
test_mcp() {
    local method="$1"
    local params="$2"
    local id="$3"
    
    echo "Testing: $method"
    echo "{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":$params,\"id\":$id}" | \
        websocat -n1 "$WS_URL" 2>/dev/null || true
}

# Test initialization sequence
echo "1. Testing initialization..."
INIT_RESPONSE=$(test_mcp "initialize" '{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-client","version":"1.0.0"}}' 1)
echo "Response: $INIT_RESPONSE"

# Send initialized confirmation
echo "2. Confirming initialization..."
test_mcp "initialized" '{}' 2

# Test tools/list
echo "3. Listing tools..."
test_mcp "tools/list" '{}' 3

# Test tool execution
echo "4. Executing tool..."
test_mcp "tools/call" '{"name":"example.hello","arguments":{"name":"World"}}' 4

# Test ping
echo "5. Testing ping..."
test_mcp "ping" '{}' 5

# Test graceful shutdown
echo "6. Testing shutdown..."
test_mcp "shutdown" '{}' 6

# Cleanup
sleep 1
if kill -0 $MCP_PID 2>/dev/null; then
    kill $MCP_PID
fi

echo "E2E tests completed successfully!"
```

## Phase 7: Build and Deployment Configuration

### 7.1 Makefile
```makefile
# edge-mcp-template/Makefile

.PHONY: build run test clean docker-build docker-run

# Variables
BINARY_NAME=edge-mcp
GO_FILES=$(shell find . -name '*.go' -type f)
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S')"

# Build the binary
build: $(GO_FILES)
	@echo "Building Edge MCP..."
	go build ${LDFLAGS} -o bin/${BINARY_NAME} cmd/edge-mcp/main.go
	@echo "Binary built: bin/${BINARY_NAME}"

# Run the server
run: build
	./bin/${BINARY_NAME} -port 8082

# Run with custom port
run-custom:
	./bin/${BINARY_NAME} -port $(PORT)

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -cover ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	go clean

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t edge-mcp:${VERSION} -t edge-mcp:latest .

# Run Docker container
docker-run:
	docker run -d -p 8082:8082 --name edge-mcp edge-mcp:latest

# Stop Docker container
docker-stop:
	docker stop edge-mcp && docker rm edge-mcp

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Cross-compile for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-darwin-amd64 cmd/edge-mcp/main.go
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-darwin-arm64 cmd/edge-mcp/main.go
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-linux-amd64 cmd/edge-mcp/main.go
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-windows-amd64.exe cmd/edge-mcp/main.go
```

### 7.2 Dockerfile
```dockerfile
# edge-mcp-template/Dockerfile

# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o edge-mcp cmd/edge-mcp/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 edge-mcp && \
    adduser -D -u 1000 -G edge-mcp edge-mcp

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/edge-mcp .
COPY --from=builder /app/config.yaml .

# Change ownership
RUN chown -R edge-mcp:edge-mcp /app

# Switch to non-root user
USER edge-mcp

# Expose port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

# Run the binary
ENTRYPOINT ["./edge-mcp"]
CMD ["-port", "8082"]
```

### 7.3 Configuration File
```yaml
# edge-mcp-template/config.yaml
server:
  port: 8082
  host: "0.0.0.0"
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s

websocket:
  max_message_size: 1048576  # 1MB
  ping_interval: 30s
  pong_timeout: 60s
  write_timeout: 10s

cache:
  max_items: 1000
  default_ttl: 5m
  cleanup_interval: 10m

logging:
  level: info  # debug, info, warn, error
  format: json # json, text
  output: stdout # stdout, file
  file: edge-mcp.log

auth:
  enabled: false
  api_key: ""  # Set via environment variable EDGE_MCP_API_KEY

tools:
  execution_timeout: 30s
  max_concurrent: 10

health:
  enabled: true
  path: /health
```

## Phase 8: Authentication and Core Platform Integration

### 8.1 Simple Authentication for Edge MCPs
```go
// edge-mcp-template/internal/auth/simple_auth.go
package auth

import (
    "net/http"
    "strings"
)

type SimpleAuthenticator struct {
    apiKey string
}

func NewSimpleAuthenticator(apiKey string) *SimpleAuthenticator {
    return &SimpleAuthenticator{apiKey: apiKey}
}

// Authenticate checks the Authorization header or query parameter
func (a *SimpleAuthenticator) Authenticate(r *http.Request) bool {
    // If no API key is configured, allow all requests (local dev mode)
    if a.apiKey == "" {
        return true
    }
    
    // Check Authorization header
    authHeader := r.Header.Get("Authorization")
    if strings.HasPrefix(authHeader, "Bearer ") {
        token := strings.TrimPrefix(authHeader, "Bearer ")
        if token == a.apiKey {
            return true
        }
    }
    
    // Check query parameter (for WebSocket connections)
    if r.URL.Query().Get("api_key") == a.apiKey {
        return true
    }
    
    return false
}
```

### 8.2 Optional Core Platform Connection
```go
// edge-mcp-template/internal/core/client.go
package core

// CorePlatformClient connects to the main DevOps MCP platform for shared services
type CorePlatformClient struct {
    baseURL string
    apiKey  string
    client  *http.Client
}

func NewCorePlatformClient(baseURL, apiKey string) *CorePlatformClient {
    return &CorePlatformClient{
        baseURL: baseURL,
        apiKey:  apiKey,
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}

// RegisterTool registers this Edge MCP's tools with the core platform
func (c *CorePlatformClient) RegisterTool(tool ToolDefinition) error {
    // Optional: Register with core platform for discovery
    // This allows the core platform to know about edge tools
    return nil
}

// GetSharedContext retrieves shared context from core platform
func (c *CorePlatformClient) GetSharedContext(contextID string) (*Context, error) {
    // Optional: Get shared context from core platform
    return nil, nil
}
```

## Phase 9: Documentation and Testing (Day 5)

### 9.1 Quick Start Guide
```markdown
# Edge MCP Quick Start

## Installation
1. Clone the template: `git clone https://github.com/developer-mesh/edge-mcp-template`
2. Install dependencies: `go mod download`
3. Build: `make build`
4. Run: `./bin/edge-mcp -port 8082`

## Connect from Claude Code
Add to your `.claude/config.json`:
{
  "mcpServers": {
    "my-edge-mcp": {
      "url": "ws://localhost:8082/ws"
    }
  }
}

## Create Your First Tool
1. Define the tool function
2. Register it in main.go
3. Restart the server
4. Tool is now available in Claude Code!
```

### 9.2 Tool Development Guide
```go
// Example tool implementation
func handleGitStatus(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path string `json:"path"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }
    
    // Execute git status
    cmd := exec.CommandContext(ctx, "git", "status", "--short")
    cmd.Dir = params.Path
    
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("git status failed: %w", err)
    }
    
    return string(output), nil
}

// Register in main.go
srv.RegisterTool("git.status", handleGitStatus)
```

### 9.3 Troubleshooting Guide

| Problem | Solution |
|---------|----------|
| Port already in use | Edge MCP auto-finds next available port |
| WebSocket connection drops | Check firewall, ensure localhost is accessible |
| Tool not found | Verify tool is registered with correct name |
| Authentication fails | Check API key in config matches server |
| High memory usage | Reduce cache size in config |
| Tool timeout | Increase execution_timeout in config |

## Implementation Timeline

| Day | Phase | Tasks | Deliverables |
|-----|-------|-------|-------------|
| 1 | Phase 1 & 2.1 | Tenant credentials migration, Template structure | Migration files, Directory structure |
| 2 | Phase 2.2-2.3 & 3.1 | Extract components, Create base Edge MCP | Working minimal Edge MCP |
| 3 | Phase 3.2 & 4.1-4.2 | Simplified handler, GitHub & K8s MCPs | Example implementations |
| 4 | Phase 4.3 & 5 & 6.1 | File system MCP, IDE integration, Unit tests | Complete examples, IDE configs |
| 5 | Phase 6.2-6.3 & 7 | Integration tests, Documentation | Full test suite, Documentation |

## Success Criteria

1. ✅ Tenant tool credentials migration successfully applied
2. ✅ Edge MCP template runs without Redis/PostgreSQL
3. ✅ At least 3 example Edge MCPs working
4. ✅ Claude Code successfully connects to Edge MCPs
5. ✅ Documentation complete and clear
6. ✅ All tests passing
7. ✅ Performance: <100ms tool execution latency
8. ✅ Memory usage: <50MB per Edge MCP instance

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Protocol incompatibility | Test against MCP spec 2024-11-05 |
| Memory leaks in cache | Implement proper TTL and eviction |
| WebSocket connection drops | Implement reconnection logic |
| Tool execution failures | Add proper error handling and logging |
| IDE configuration complexity | Provide automated setup scripts |

## Future Enhancements

1. **Edge MCP Hub** - Central registry for discovering Edge MCPs
2. **Tool Marketplace** - Share Edge MCP implementations
3. **Cloud Sync** - Optional sync with core platform
4. **Plugin System** - Dynamic tool loading
5. **GUI Configuration** - Visual tool builder
6. **Monitoring Dashboard** - Local performance monitoring

## Implementation Commands - Step by Step

### Step 1: Create Edge MCP Template from Core
```bash
# Create new directory structure
mkdir -p edge-mcp-template/{cmd/edge-mcp,internal/{server,cache,tools},pkg/protocol,examples}

# Copy essential files from core (from project root)
cp apps/mcp-server/internal/api/mcp_protocol.go edge-mcp-template/internal/server/mcp_handler.go
cp apps/mcp-server/internal/api/websocket/server.go edge-mcp-template/internal/server/websocket.go
cp pkg/common/cache/memory.go edge-mcp-template/internal/cache/memory.go
cp pkg/observability/logger.go edge-mcp-template/internal/server/logger.go

# Create go.mod
cd edge-mcp-template
go mod init github.com/developer-mesh/edge-mcp
go get github.com/gin-gonic/gin@v1.9.1
go get github.com/gorilla/websocket@v1.5.1
go get github.com/google/uuid@v1.5.0
```

### Step 2: Clean Up Extracted Files
```bash
# Remove Redis imports
sed -i '' '/redis/d' internal/server/mcp_handler.go
sed -i '' '/database/d' internal/server/mcp_handler.go
sed -i '' '/repository/d' internal/server/mcp_handler.go

# Remove AWS imports
sed -i '' '/aws/d' internal/server/mcp_handler.go
sed -i '' '/s3/d' internal/server/mcp_handler.go

# Simplify the files (manual editing required)
# - Remove database queries
# - Replace Redis cache with memory cache
# - Remove complex auth, use simple API key
```

### Step 3: Create Main Entry Point
```bash
# Create main.go with the template from Phase 3.1
cat > cmd/edge-mcp/main.go << 'EOF'
// Insert main.go content from Phase 3.1
EOF
```

### Step 4: Build and Test
```bash
# Build the Edge MCP
go build -o bin/edge-mcp cmd/edge-mcp/main.go

# Run it
./bin/edge-mcp -port 8082

# Test with websocat
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1.0"}},"id":1}' | websocat ws://localhost:8082/ws
```

### Step 5: Create Example Implementations
```bash
# Copy template for each example
cp -r edge-mcp-template edge-mcp-github
cp -r edge-mcp-template edge-mcp-kubernetes
cp -r edge-mcp-template edge-mcp-filesystem

# Customize each with specific tools
```

## Key Technical Decisions

1. **Protocol Version**: Using MCP 2025-06-18 (current standard in codebase)
2. **Cache**: In-memory only, no Redis dependency
3. **Authentication**: Simple Bearer token, no complex JWT
4. **Storage**: No database, everything in memory
5. **Dependencies**: Minimal - just gin, websocket, uuid
6. **Configuration**: YAML file with environment variable overrides
7. **Logging**: Simple structured logging to stdout
8. **Health Check**: Basic HTTP endpoint for monitoring
9. **Graceful Shutdown**: 5-second timeout for connections to close
10. **Port Management**: Auto-find next available port if configured port is in use

## Critical Success Factors

1. **Must maintain MCP protocol compatibility** - Test with actual Claude Code
2. **Must start in <1 second** - No heavy initialization
3. **Must use <50MB RAM** - Efficient memory management
4. **Must handle connection drops gracefully** - Implement reconnection
5. **Must provide clear error messages** - Help developers debug
6. **Must work without any infrastructure** - True zero-dependency
7. **Must be cross-platform** - Build for Mac, Linux, Windows
8. **Must have simple configuration** - Sensible defaults

## Conclusion

This enhanced implementation plan provides all the technical details needed to create lightweight, standalone Edge MCPs. The plan includes:

- **Complete code examples** with error handling and edge cases
- **Build configuration** with Makefile and Dockerfile
- **Testing strategies** with correct protocol versions
- **Authentication mechanism** for security
- **Step-by-step commands** for implementation
- **Troubleshooting guide** for common issues

By following this plan, developers can create Edge MCPs that:
- Run locally without Redis or PostgreSQL
- Connect directly to Claude Code, Cursor, or Windsurf
- Execute tools with <100ms latency
- Use minimal system resources
- Maintain full MCP protocol compatibility

Expected completion: 5 working days
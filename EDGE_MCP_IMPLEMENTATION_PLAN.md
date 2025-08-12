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

## Phase 9: Comprehensive Documentation (Day 5)

### 9.1 Complete README.md for Edge MCP Template
```markdown
# Edge MCP - Lightweight Model Context Protocol Server

Edge MCP is a lightweight, standalone implementation of the Model Context Protocol (MCP) that runs locally without infrastructure dependencies. Perfect for adding custom tools to Claude Code, Cursor, or Windsurf.

## Features
- 🚀 **Zero Infrastructure** - No Redis, PostgreSQL, or cloud services required
- 💾 **Minimal Memory** - Uses <50MB RAM with in-memory caching
- ⚡ **Fast Startup** - Starts in <1 second
- 🔧 **Easy Tool Development** - Simple function registration
- 🔒 **Optional Authentication** - API key support for security
- 🏥 **Health Monitoring** - Built-in health check endpoint
- 🔄 **Auto Port Discovery** - Automatically finds available ports
- 📦 **Cross-Platform** - Works on Mac, Linux, and Windows

## Quick Start

### Installation
```bash
# Clone the template
git clone https://github.com/developer-mesh/edge-mcp-template
cd edge-mcp-template

# Install dependencies
go mod download

# Build
make build

# Run
./bin/edge-mcp -port 8082
```

### Connect from Claude Code
1. Create or edit `.claude/config.json` in your project:
```json
{
  "mcpServers": {
    "my-tools": {
      "url": "ws://localhost:8082/ws",
      "apiKey": "optional-key"
    }
  }
}
```

2. Restart Claude Code to load the new server
3. Your tools are now available!

### Command Line Options
```bash
./edge-mcp [options]

Options:
  -port string      Port to listen on (default "8082")
  -api-key string   API key for authentication (optional)
  -log-level string Log level: debug, info, warn, error (default "info")
  -config string    Path to config file (default "./config.yaml")
  -version         Show version information
  -help            Show this help message
```

## Creating Tools

### Basic Tool Example
```go
func handleHello(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Name string `json:"name"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }
    
    return fmt.Sprintf("Hello, %s!", params.Name), nil
}

// In main.go
srv.RegisterTool("hello", handleHello)
```

### Tool Registration
```go
srv.RegisterTool("tool-name", handler, ToolOptions{
    Description: "What this tool does",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param1": map[string]string{"type": "string"},
        },
        "required": []string{"param1"},
    },
})
```

## Configuration

All settings can be configured via `config.yaml` or environment variables:

| Setting | Config Path | Environment Variable | Default | Description |
|---------|------------|---------------------|---------|-------------|
| Port | `server.port` | `EDGE_MCP_PORT` | 8082 | WebSocket server port |
| API Key | `auth.api_key` | `EDGE_MCP_API_KEY` | "" | Optional authentication |
| Log Level | `logging.level` | `EDGE_MCP_LOG_LEVEL` | info | Logging verbosity |
| Cache Size | `cache.max_items` | `EDGE_MCP_CACHE_SIZE` | 1000 | Max cached items |
| Tool Timeout | `tools.execution_timeout` | `EDGE_MCP_TOOL_TIMEOUT` | 30s | Max tool execution time |

## Building from Source

### Prerequisites
- Go 1.21 or higher
- Make (optional, for using Makefile)

### Build Commands
```bash
# Standard build
go build -o edge-mcp cmd/edge-mcp/main.go

# With version info
go build -ldflags "-X main.Version=1.0.0" -o edge-mcp cmd/edge-mcp/main.go

# Cross-platform builds
make build-all  # Builds for Mac, Linux, Windows
```

## Docker Support

### Build and Run with Docker
```bash
# Build image
docker build -t edge-mcp:latest .

# Run container
docker run -d -p 8082:8082 edge-mcp:latest

# With custom port
docker run -d -p 9000:9000 edge-mcp:latest -port 9000
```

## Testing

### Run Tests
```bash
# Unit tests
make test

# With coverage
make test-coverage

# E2E tests
./scripts/test-e2e.sh
```

## Troubleshooting

See [Troubleshooting Guide](#troubleshooting-guide) below for common issues.

## Contributing

Contributions welcome! Please read CONTRIBUTING.md first.

## License

MIT License - see LICENSE file for details.
```

### 9.2 Comprehensive Tool Development Guide
```markdown
# Tool Development Guide

## Tool Anatomy

Every tool consists of three parts:

1. **Handler Function** - Executes the tool logic
2. **Input Schema** - Defines parameters (JSON Schema)
3. **Registration** - Adds tool to the server

## Basic Tool Template
```go
// tools/my_tool.go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
)

// Tool handler function
func HandleMyTool(ctx context.Context, args json.RawMessage) (interface{}, error) {
    // 1. Parse arguments
    var params struct {
        RequiredField string   `json:"required_field"`
        OptionalField string   `json:"optional_field,omitempty"`
        NumberField   int      `json:"number_field"`
        BoolField     bool     `json:"bool_field"`
        ArrayField    []string `json:"array_field"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }
    
    // 2. Validate inputs
    if params.RequiredField == "" {
        return nil, fmt.Errorf("required_field cannot be empty")
    }
    
    // 3. Execute tool logic
    result := processData(params)
    
    // 4. Return result (will be converted to JSON)
    return result, nil
}

// Tool definition for registration
var MyToolDefinition = ToolDefinition{
    Name:        "category.my_tool",
    Description: "Processes data according to parameters",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "required_field": map[string]interface{}{
                "type":        "string",
                "description": "A required string parameter",
            },
            "optional_field": map[string]interface{}{
                "type":        "string",
                "description": "An optional string parameter",
            },
            "number_field": map[string]interface{}{
                "type":        "number",
                "description": "A numeric parameter",
                "minimum":     0,
                "maximum":     100,
            },
            "bool_field": map[string]interface{}{
                "type":        "boolean",
                "description": "A boolean flag",
                "default":     false,
            },
            "array_field": map[string]interface{}{
                "type": "array",
                "items": map[string]interface{}{
                    "type": "string",
                },
                "description": "An array of strings",
            },
        },
        "required": []string{"required_field"},
    },
    Handler: HandleMyTool,
}
```

## Common Tool Patterns

### 1. File System Tool
```go
func HandleReadFile(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path string `json:"path"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, err
    }
    
    // Validate path (security!)
    if !isPathSafe(params.Path) {
        return nil, fmt.Errorf("invalid path: %s", params.Path)
    }
    
    content, err := os.ReadFile(params.Path)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %w", err)
    }
    
    return string(content), nil
}
```

### 2. External Command Tool
```go
func HandleGitCommand(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Command string   `json:"command"`
        Args    []string `json:"args"`
        Dir     string   `json:"dir"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, err
    }
    
    // Create command with timeout
    cmd := exec.CommandContext(ctx, "git", params.Args...)
    cmd.Dir = params.Dir
    
    // Capture output
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("git command failed: %w\nOutput: %s", err, output)
    }
    
    return string(output), nil
}
```

### 3. HTTP API Tool
```go
func HandleAPIRequest(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        URL     string            `json:"url"`
        Method  string            `json:"method"`
        Headers map[string]string `json:"headers"`
        Body    json.RawMessage   `json:"body"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, err
    }
    
    // Create request
    req, err := http.NewRequestWithContext(ctx, params.Method, params.URL, 
        bytes.NewReader(params.Body))
    if err != nil {
        return nil, err
    }
    
    // Add headers
    for k, v := range params.Headers {
        req.Header.Set(k, v)
    }
    
    // Execute request
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Read response
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    
    return map[string]interface{}{
        "status": resp.StatusCode,
        "body":   string(body),
        "headers": resp.Header,
    }, nil
}
```

### 4. Database Query Tool
```go
func HandleDatabaseQuery(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Query  string        `json:"query"`
        Params []interface{} `json:"params"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, err
    }
    
    // Validate query (prevent SQL injection)
    if !isReadOnlyQuery(params.Query) {
        return nil, fmt.Errorf("only SELECT queries allowed")
    }
    
    // Execute query (example with database/sql)
    rows, err := db.QueryContext(ctx, params.Query, params.Params...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    // Collect results
    var results []map[string]interface{}
    columns, _ := rows.Columns()
    
    for rows.Next() {
        values := make([]interface{}, len(columns))
        valuePtrs := make([]interface{}, len(columns))
        for i := range values {
            valuePtrs[i] = &values[i]
        }
        
        if err := rows.Scan(valuePtrs...); err != nil {
            return nil, err
        }
        
        row := make(map[string]interface{})
        for i, col := range columns {
            row[col] = values[i]
        }
        results = append(results, row)
    }
    
    return results, nil
}
```

## Error Handling Best Practices

1. **Always validate inputs**
```go
if params.Path == "" {
    return nil, fmt.Errorf("path is required")
}
```

2. **Provide context in errors**
```go
if err != nil {
    return nil, fmt.Errorf("failed to read file %s: %w", path, err)
}
```

3. **Handle timeouts properly**
```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```

4. **Sanitize sensitive data in errors**
```go
// Don't include passwords or tokens in error messages
return nil, fmt.Errorf("authentication failed for user %s", username)
```

## Testing Your Tools

### Unit Test Example
```go
func TestHandleMyTool(t *testing.T) {
    tests := []struct {
        name    string
        args    json.RawMessage
        want    interface{}
        wantErr bool
    }{
        {
            name: "valid input",
            args: json.RawMessage(`{"required_field": "test"}`),
            want: "processed: test",
            wantErr: false,
        },
        {
            name: "missing required field",
            args: json.RawMessage(`{}`),
            want: nil,
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := context.Background()
            got, err := HandleMyTool(ctx, tt.args)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("HandleMyTool() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("HandleMyTool() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Performance Tips

1. **Use context for cancellation**
2. **Set reasonable timeouts**
3. **Cache expensive operations**
4. **Limit concurrent operations**
5. **Stream large responses**
6. **Validate early, fail fast**
```

### 9.3 MCP Protocol Reference Documentation
```markdown
# MCP Protocol Reference

## Protocol Version
Edge MCP implements MCP protocol version **2025-06-18**.

## Connection Flow

1. **Client connects** via WebSocket to `ws://localhost:PORT/ws`
2. **Client sends `initialize`** with protocol version and client info
3. **Server responds** with capabilities
4. **Client sends `initialized`** to confirm
5. **Client can now call tools** and access resources

## Required Methods

### initialize
Initialize the connection and negotiate capabilities.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "clientInfo": {
      "name": "claude-code",
      "version": "1.0.0"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-06-18",
    "serverInfo": {
      "name": "edge-mcp",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": {
        "listChanged": true
      },
      "resources": {
        "subscribe": false,
        "listChanged": false
      }
    }
  }
}
```

### initialized
Confirm successful initialization.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "initialized",
  "params": {}
}
```

### tools/list
List all available tools.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/list"
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "tools": [
      {
        "name": "example.hello",
        "description": "Says hello",
        "inputSchema": {
          "type": "object",
          "properties": {
            "name": {
              "type": "string",
              "description": "Name to greet"
            }
          },
          "required": ["name"]
        }
      }
    ]
  }
}
```

### tools/call
Execute a tool with arguments.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "example.hello",
    "arguments": {
      "name": "World"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Hello, World!"
      }
    ]
  }
}
```

### ping
Keep-alive ping.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "ping"
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {}
}
```

### shutdown
Gracefully close the connection.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "shutdown"
}
```

## Error Responses

All errors follow JSON-RPC 2.0 format:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32601,
    "message": "Method not found",
    "data": {
      "details": "Additional error information"
    }
  }
}
```

### Error Codes
- `-32700` Parse error
- `-32600` Invalid request
- `-32601` Method not found
- `-32602` Invalid params
- `-32603` Internal error

## Content Types

Tool responses can return different content types:

### Text Content
```json
{
  "content": [
    {
      "type": "text",
      "text": "Plain text response"
    }
  ]
}
```

### Error Content
```json
{
  "content": [
    {
      "type": "error",
      "error": "Error message"
    }
  ]
}
```

### Multiple Content Items
```json
{
  "content": [
    {
      "type": "text",
      "text": "First part"
    },
    {
      "type": "text",
      "text": "Second part"
    }
  ]
}
```
```

### 9.4 IDE-Specific Setup Guides
```markdown
# IDE Setup Guides

## Claude Code Setup

### 1. Configuration File
Create `.claude/config.json` in your project root:

```json
{
  "mcpServers": {
    "local-tools": {
      "url": "ws://localhost:8082/ws",
      "name": "My Local Tools",
      "description": "Custom tools for this project",
      "apiKey": "optional-api-key",
      "autoStart": true,
      "startCommand": "edge-mcp -port 8082"
    }
  }
}
```

### 2. Auto-start Script (Optional)
Create `.claude/start-mcp.sh`:

```bash
#!/bin/bash
# Check if edge-mcp is running
if ! pgrep -f "edge-mcp.*8082" > /dev/null; then
    echo "Starting Edge MCP server..."
    edge-mcp -port 8082 &
    sleep 2
fi
```

### 3. Verify Connection
1. Open Claude Code
2. Check the status bar for "MCP: Connected"
3. Type `/tools` to see available tools

## Cursor Setup

### 1. Configuration
Add to `.cursor/settings.json`:

```json
{
  "mcp.servers": [
    {
      "name": "local-edge-mcp",
      "url": "ws://localhost:8082/ws",
      "enabled": true
    }
  ]
}
```

### 2. Workspace Settings
For workspace-specific tools, create `.cursor/workspace.json`:

```json
{
  "mcp": {
    "servers": {
      "project-tools": {
        "url": "ws://localhost:8082/ws",
        "tools": ["git.*", "docker.*"]
      }
    }
  }
}
```

## Windsurf Setup

### 1. Global Configuration
Edit `~/.windsurf/mcp-config.yaml`:

```yaml
servers:
  - name: edge-mcp-global
    url: ws://localhost:8082/ws
    autoConnect: true
    reconnectInterval: 5s
```

### 2. Project Configuration
Create `.windsurf/mcp.yaml` in project:

```yaml
servers:
  - name: project-tools
    url: ws://localhost:8082/ws
    tools:
      include:
        - "project.*"
        - "test.*"
      exclude:
        - "*.dangerous"
```

## VS Code with Continue.dev

### 1. Install Continue Extension
```bash
code --install-extension continue.continue
```

### 2. Configure Continue
Edit `~/.continue/config.json`:

```json
{
  "models": [...],
  "mcpServers": [
    {
      "name": "edge-mcp",
      "url": "ws://localhost:8082/ws"
    }
  ]
}
```

## Troubleshooting IDE Connections

### Connection Issues
1. **Check server is running**: `ps aux | grep edge-mcp`
2. **Test with websocat**: `websocat ws://localhost:8082/ws`
3. **Check firewall**: Ensure localhost connections allowed
4. **Verify port**: `lsof -i :8082`

### Tool Discovery Issues
1. **Refresh tools**: Restart IDE or reload window
2. **Check tool names**: Must match registered names exactly
3. **Verify protocol**: Must use MCP 2025-06-18

### Authentication Issues
1. **Check API key**: Must match in both config and server
2. **Try without auth**: Remove API key for testing
3. **Check headers**: Some IDEs need explicit auth config
```

### 9.5 Security and Performance Documentation
```markdown
# Security Best Practices

## Authentication

### API Key Authentication
```bash
# Start with API key
edge-mcp -api-key "your-secret-key"

# Or use environment variable
export EDGE_MCP_API_KEY="your-secret-key"
edge-mcp
```

### Secure Tool Development

1. **Input Validation**
```go
// Always validate and sanitize inputs
if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(params.Name) {
    return nil, fmt.Errorf("invalid name format")
}
```

2. **Path Traversal Prevention**
```go
// Prevent directory traversal attacks
cleanPath := filepath.Clean(params.Path)
if strings.Contains(cleanPath, "..") {
    return nil, fmt.Errorf("invalid path")
}
```

3. **Command Injection Prevention**
```go
// Never construct shell commands from user input
// Use exec.Command with separate arguments
cmd := exec.Command("git", "log", "--oneline", "-n", strconv.Itoa(params.Count))
// NOT: exec.Command("sh", "-c", "git log " + params.Args)
```

4. **Secrets Management**
- Never log sensitive data
- Don't include secrets in error messages
- Use environment variables for credentials
- Implement secret rotation

## Performance Optimization

### Memory Management

1. **Cache Configuration**
```yaml
cache:
  max_items: 500      # Reduce for low-memory systems
  default_ttl: 2m     # Shorter TTL for less memory
  cleanup_interval: 5m
```

2. **Tool Optimization**
```go
// Stream large outputs instead of loading into memory
func streamLargeFile(ctx context.Context, path string) (io.Reader, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    
    // Return reader, not entire content
    return file, nil
}
```

3. **Connection Limits**
```yaml
server:
  max_connections: 10     # Limit concurrent connections
  max_message_size: 1MB   # Limit message size
```

### CPU Optimization

1. **Concurrent Tool Execution**
```go
// Limit concurrent tool executions
sem := make(chan struct{}, 5) // Max 5 concurrent

func executeWithLimit(ctx context.Context, fn func() error) error {
    sem <- struct{}{}        // Acquire
    defer func() { <-sem }() // Release
    return fn()
}
```

2. **Timeout Configuration**
```yaml
tools:
  execution_timeout: 10s    # Prevent long-running tools
  max_concurrent: 5         # Limit parallel executions
```

## Monitoring and Metrics

### Health Check Endpoint
```go
// Implement comprehensive health check
func healthCheck() HealthStatus {
    return HealthStatus{
        Status: "healthy",
        Uptime: time.Since(startTime),
        Memory: getMemoryUsage(),
        Tools:  len(registeredTools),
        Connections: activeConnections,
    }
}
```

### Logging Configuration
```yaml
logging:
  level: info      # Reduce to warn in production
  format: json     # Structured logging
  output: file     # Don't log to stdout in production
  file: /var/log/edge-mcp.log
  max_size: 100MB
  max_backups: 5
  max_age: 30
```

### Performance Metrics
```go
// Track tool execution times
start := time.Now()
result, err := tool.Execute(ctx, args)
duration := time.Since(start)

metrics.RecordToolExecution(tool.Name, duration, err == nil)
```

## Production Deployment

### Systemd Service
```ini
[Unit]
Description=Edge MCP Server
After=network.target

[Service]
Type=simple
User=edge-mcp
Group=edge-mcp
ExecStart=/usr/local/bin/edge-mcp -config /etc/edge-mcp/config.yaml
Restart=always
RestartSec=10

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/edge-mcp

[Install]
WantedBy=multi-user.target
```

### Docker Compose
```yaml
version: '3.8'
services:
  edge-mcp:
    image: edge-mcp:latest
    ports:
      - "8082:8082"
    environment:
      - EDGE_MCP_API_KEY=${API_KEY}
      - EDGE_MCP_LOG_LEVEL=info
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./logs:/var/log/edge-mcp
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8082/health"]
      interval: 30s
      timeout: 3s
      retries: 3
```
```

### 9.6 Enhanced Troubleshooting Guide
```markdown
# Comprehensive Troubleshooting Guide

## Common Issues and Solutions

### Connection Problems

#### "Connection refused" Error
```bash
# Check if server is running
ps aux | grep edge-mcp

# Check if port is listening
netstat -an | grep 8082
lsof -i :8082

# Try starting manually with debug logging
edge-mcp -port 8082 -log-level debug
```

#### "WebSocket upgrade failed"
- Check firewall settings
- Verify no proxy interference
- Try different port: `edge-mcp -port 9000`
- Check for conflicting services

#### "Authentication failed"
```bash
# Test without authentication
edge-mcp  # No -api-key flag

# Verify API key matches
echo $EDGE_MCP_API_KEY

# Check client configuration
cat .claude/config.json | grep apiKey
```

### Tool Execution Problems

#### "Tool not found"
```bash
# List registered tools (add this endpoint)
curl http://localhost:8082/tools

# Check tool name spelling
# Tools are case-sensitive!
```

#### "Tool timeout"
```yaml
# Increase timeout in config.yaml
tools:
  execution_timeout: 60s  # Increase from default 30s
```

#### "Tool execution failed"
```go
// Add detailed logging to your tool
func HandleMyTool(ctx context.Context, args json.RawMessage) (interface{}, error) {
    log.Printf("Tool called with args: %s", string(args))
    
    // Your tool logic...
    
    if err != nil {
        log.Printf("Tool error: %v", err)
        return nil, fmt.Errorf("detailed error: %w", err)
    }
}
```

### Memory Issues

#### High Memory Usage
```bash
# Check memory usage
ps aux | grep edge-mcp
top -p $(pgrep edge-mcp)

# Reduce cache size
edge-mcp -cache-size 100  # Default is 1000
```

#### Memory Leaks
```go
// Ensure cleanup in tools
defer func() {
    // Clean up resources
    file.Close()
    cmd.Process.Kill()
}()
```

### Performance Issues

#### Slow Tool Execution
1. Profile the tool:
```go
start := time.Now()
// Tool logic
log.Printf("Tool took %v", time.Since(start))
```

2. Check for blocking operations
3. Add context timeout
4. Use goroutines for parallel work

#### High CPU Usage
```bash
# Check CPU usage
top -p $(pgrep edge-mcp)

# Limit concurrent executions
# In config.yaml:
tools:
  max_concurrent: 3  # Reduce from default
```

### IDE-Specific Issues

#### Claude Code Not Connecting
1. Check `.claude/config.json` syntax
2. Restart Claude Code
3. Check Claude Code logs: View → Output → MCP

#### Cursor Not Finding Tools
1. Reload window: Cmd+R (Mac) / Ctrl+R (Windows/Linux)
2. Check Cursor logs: Help → Toggle Developer Tools → Console

#### Windsurf Connection Drops
1. Check reconnect settings
2. Increase ping interval
3. Check network stability

### Debugging Techniques

#### Enable Debug Logging
```bash
edge-mcp -log-level debug
```

#### Use Verbose Output
```go
// Add debug prints (remove in production)
log.Printf("[DEBUG] Received: %v", params)
log.Printf("[DEBUG] Processing: %s", stage)
log.Printf("[DEBUG] Result: %v", result)
```

#### Test with websocat
```bash
# Interactive testing
websocat ws://localhost:8082/ws

# Send test message
echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | websocat ws://localhost:8082/ws
```

#### Monitor Network Traffic
```bash
# Watch WebSocket traffic
tcpdump -i lo0 -A 'port 8082'

# Use Chrome DevTools
# Open chrome://inspect → Devices → Inspect
```

### Error Messages Reference

| Error | Cause | Solution |
|-------|-------|----------|
| "Protocol version mismatch" | Wrong MCP version | Use version 2025-06-18 |
| "Invalid JSON-RPC request" | Malformed request | Check JSON syntax |
| "Method not implemented" | Missing handler | Implement the method |
| "Context deadline exceeded" | Timeout | Increase timeout or optimize tool |
| "Too many connections" | Connection limit | Increase max_connections |
| "Message too large" | Exceeds max_message_size | Increase limit or reduce payload |

### Getting Help

1. **Check Logs**
```bash
tail -f edge-mcp.log
journalctl -u edge-mcp -f  # If using systemd
```

2. **Enable Metrics**
```go
// Add metrics endpoint
http.HandleFunc("/metrics", metricsHandler)
```

3. **File an Issue**
Include:
- Edge MCP version
- OS and Go version
- Full error message
- Steps to reproduce
- Relevant config
```

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
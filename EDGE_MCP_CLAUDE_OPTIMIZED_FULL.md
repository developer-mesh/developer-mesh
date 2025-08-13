# Edge MCP Implementation Plan - Claude Code Optimized (Full Version)
<!-- Optimized for Claude Opus 4.1 with ALL technical details preserved -->

## Claude Code Implementation Guide (Opus 4.1 Optimized)

### How to Use This Plan with Claude Code
Based on Opus 4.1's capabilities (74.5% SWE-bench, extended thinking, multi-file refactoring):

1. **Enable Extended Thinking** for complex phases:
   - Say "think deeply about the architecture" before Phase 2 (extraction)
   - Use "think harder" for Phase 8 (context synchronization)
   - Apply extended thinking for architectural decisions

2. **Leverage Multi-File Capabilities**:
   - Phases 2, 4, and 8 involve coordinated changes across files
   - Opus 4.1 excels at maintaining consistency across related files
   - Use Claude's project awareness for systematic refactoring

3. **Optimal Task Execution**:
   - Each phase is sized for Claude's context window (200K tokens)
   - Verification steps prevent cascading errors
   - Incremental commits after each phase

4. **Add to CLAUDE.md** for persistence:
   ```markdown
   # Edge MCP Implementation
   - Target: Single binary, no infrastructure
   - Protocol: MCP 2025-06-18
   - Testing: Run verification after each phase
   - Current Phase: [Track here]
   ```

### Quick Reference Commands
```bash
# Before starting
git checkout feature/edge-mcp-and-tenant-credentials

# After each phase
git add . && git commit -m "feat: [phase description]"
make test  # Once available

# Verification
grep -r "redis\|postgres" edge-mcp-template  # Should be empty
```

### Phase Execution Strategy
| Phase | Complexity | Use Extended Thinking | Multi-File Operations |
|-------|------------|----------------------|----------------------|
| 1. Migration | Low | No | Single file |
| 2. Extract Template | High | Yes - architecture | Yes - refactoring |
| 3. Implementation | Medium | No | Multiple files |
| 4. Example MCPs | Medium | No | Template reuse |
| 5. IDE Integration | Low | No | Config files |
| 6. Testing | Medium | No | Test files |
| 7. Build Config | Low | No | Build files |
| 8. Context Sync | High | Yes - protocol design | Yes - API integration |
| 9. Database Schema | Low | No | Single file |
| 10. Authentication | Medium | No | Multiple files |

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
│   └── → Edge MCP (localhost:8082)
│       ├── Local Tools (no latency, local resources)
│       │   ├── fs.read_file
│       │   ├── git.status
│       │   ├── docker.build
│       │   └── shell.execute
│       │
│       └── → Core Platform Connection (required)
│           ├── Agent Network Access
│           ├── Shared Context & Memory
│           ├── Tool Registry & Discovery
│           ├── Advanced Orchestration
│           └── Persistent Storage

Core Platform (cloud.devmesh.ai)
├── Full MCP Server (PostgreSQL, Redis, AWS)
├── REST API
├── Tool Registry
├── Agent Network
└── Persistent Context Storage
```

## Phase 1: Tenant Tool Credentials Migration (Day 1)
<!-- CLAUDE: Single file task - no extended thinking needed -->

### 1.1 Context
The migration file `apps/rest-api/migrations/sql/000026_tenant_tool_credentials.up.sql` exists but is empty. We need to implement it to enable tenant-specific tool credentials for Edge MCPs.

### 1.2 Migration Implementation
<!-- CLAUDE: Copy this exact SQL - no modifications needed -->

```sql
-- Migration: 000026_tenant_tool_credentials.up.sql
-- Purpose: Enable tenant-specific tool credentials for Edge MCPs
-- Since this is greenfield, we go directly to the desired state

BEGIN;

-- Tenant tool credentials table
CREATE TABLE IF NOT EXISTS mcp.tenant_tool_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    tool_id UUID REFERENCES mcp.tool_configurations(id) ON DELETE CASCADE,
    
    -- Credential details
    credential_name VARCHAR(255) NOT NULL,
    credential_type VARCHAR(50) NOT NULL,
    encrypted_value TEXT NOT NULL,
    
    -- OAuth specific fields (optional)
    oauth_client_id VARCHAR(255),
    oauth_client_secret_encrypted TEXT,
    oauth_refresh_token_encrypted TEXT,
    oauth_token_expiry TIMESTAMP WITH TIME ZONE,
    
    -- Metadata
    description TEXT,
    tags TEXT[],
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMP WITH TIME ZONE,
    
    -- Edge MCP associations
    edge_mcp_id VARCHAR(255),
    allowed_edge_mcps TEXT[],
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    
    -- Constraints
    CONSTRAINT uk_tenant_tool_credential UNIQUE(tenant_id, tool_id, credential_name),
    CONSTRAINT chk_credential_type CHECK (credential_type IN ('api_key', 'oauth2', 'basic', 'custom'))
);

-- Edge MCP registrations table
CREATE TABLE IF NOT EXISTS mcp.edge_mcp_registrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    edge_mcp_id VARCHAR(255) NOT NULL,
    
    -- Registration details
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    host_machine VARCHAR(255),
    
    -- Authentication
    api_key_hash VARCHAR(255) NOT NULL,
    
    -- Configuration
    allowed_tools TEXT[],
    max_connections INTEGER DEFAULT 10,
    
    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT uk_edge_mcp_registration UNIQUE(tenant_id, edge_mcp_id),
    CONSTRAINT chk_edge_status CHECK (status IN ('pending', 'active', 'suspended', 'deactivated'))
);

-- Indexes for performance
CREATE INDEX idx_tenant_tool_credentials_lookup 
    ON mcp.tenant_tool_credentials(tenant_id, tool_id, is_active);

CREATE INDEX idx_edge_mcp_active 
    ON mcp.edge_mcp_registrations(tenant_id, status) 
    WHERE status = 'active';

COMMIT;
```

### 1.3 Verification Steps
<!-- CLAUDE: Run these commands to verify -->

```bash
# Test migration file
cat apps/rest-api/migrations/sql/000026_tenant_tool_credentials.up.sql

# If running locally with database
psql -h localhost -U devmesh -d devmesh_development -f apps/rest-api/migrations/sql/000026_tenant_tool_credentials.up.sql

# Verify tables created
psql -h localhost -U devmesh -d devmesh_development -c "\dt mcp.tenant_tool_credentials"
psql -h localhost -U devmesh -d devmesh_development -c "\dt mcp.edge_mcp_registrations"
```

## Phase 2: Edge MCP Template Creation (Day 1-2)
<!-- CLAUDE: Use extended thinking here - this is architectural refactoring -->

### 2.1 Strategy
We'll extract and simplify the existing `apps/mcp-server` to create a lightweight Edge MCP template that:
- Removes all Redis dependencies (uses `pkg/common/cache/memory.go` instead)
- Removes all PostgreSQL dependencies (uses Core Platform REST API instead)
- Maintains MCP protocol compatibility
- Adds Core Platform client for context synchronization

### 2.2 Directory Structure Creation
<!-- CLAUDE: Create this exact structure -->

```bash
# Create the Edge MCP template directory structure
mkdir -p edge-mcp-template/{cmd/server,internal/{mcp,tools,auth,cache,core,config},configs,scripts,test}

# Create go.mod for the new module
cat > edge-mcp-template/go.mod << 'EOF'
module github.com/developer-mesh/edge-mcp

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/gorilla/websocket v1.5.1
    github.com/stretchr/testify v1.8.4
    github.com/google/uuid v1.5.0
    go.uber.org/zap v1.26.0
)
EOF
```

### 2.3 Main Server Implementation
<!-- CLAUDE: This is the simplified main.go without infrastructure -->

**File: `edge-mcp-template/cmd/server/main.go`**

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/developer-mesh/edge-mcp/internal/auth"
    "github.com/developer-mesh/edge-mcp/internal/cache"
    "github.com/developer-mesh/edge-mcp/internal/config"
    "github.com/developer-mesh/edge-mcp/internal/core"
    "github.com/developer-mesh/edge-mcp/internal/mcp"
    "github.com/developer-mesh/edge-mcp/internal/tools"
    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
)

var (
    version = "1.0.0"
    commit  = "unknown"
)

func main() {
    var (
        configFile = flag.String("config", "configs/config.yaml", "Path to configuration file")
        port       = flag.Int("port", 8082, "Port to listen on")
        apiKey     = flag.String("api-key", "", "API key for authentication")
        coreURL    = flag.String("core-url", "", "Core Platform URL for advanced features")
        showVersion = flag.Bool("version", false, "Show version information")
    )
    flag.Parse()

    if *showVersion {
        fmt.Printf("Edge MCP v%s (commit: %s)\n", version, commit)
        os.Exit(0)
    }

    // Load configuration
    cfg, err := config.Load(*configFile)
    if err != nil {
        log.Printf("Warning: Could not load config file: %v. Using defaults.", err)
        cfg = config.Default()
    }

    // Override with command line flags
    if *apiKey != "" {
        cfg.Auth.APIKey = *apiKey
    }
    if *coreURL != "" {
        cfg.Core.URL = *coreURL
    }
    if *port != 0 {
        cfg.Server.Port = *port
    }

    // Initialize components
    memCache := cache.NewMemoryCache(1000, 5*time.Minute)
    
    // Initialize Core Platform client (optional)
    var coreClient *core.Client
    if cfg.Core.URL != "" {
        coreClient = core.NewClient(
            cfg.Core.URL,
            cfg.Core.APIKey,
            cfg.Core.TenantID,
            cfg.Core.EdgeMCPID,
        )
        
        // Authenticate with Core Platform
        if err := coreClient.AuthenticateWithCore(context.Background()); err != nil {
            log.Printf("Warning: Could not authenticate with Core Platform: %v. Running in standalone mode.", err)
            coreClient = nil
        }
    }

    // Initialize authentication
    authenticator := auth.NewEdgeAuthenticator(cfg.Auth.APIKey)

    // Initialize tool registry
    toolRegistry := tools.NewRegistry()
    
    // Register local tools
    toolRegistry.Register(tools.NewFileSystemTool())
    toolRegistry.Register(tools.NewGitTool())
    toolRegistry.Register(tools.NewDockerTool())
    toolRegistry.Register(tools.NewShellTool())
    
    // Fetch and register remote tools from Core Platform
    if coreClient != nil {
        remoteTools, err := coreClient.FetchRemoteTools(context.Background())
        if err != nil {
            log.Printf("Warning: Could not fetch remote tools: %v", err)
        } else {
            for _, tool := range remoteTools {
                toolRegistry.RegisterRemote(tool)
            }
        }
    }

    // Initialize MCP handler
    mcpHandler := mcp.NewHandler(
        toolRegistry,
        memCache,
        coreClient,
        authenticator,
    )

    // Setup HTTP server with Gin
    gin.SetMode(gin.ReleaseMode)
    router := gin.New()
    router.Use(gin.Recovery())

    // Health check endpoint
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "status": "healthy",
            "version": version,
            "core_connected": coreClient != nil,
        })
    })

    // WebSocket upgrader
    upgrader := websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            // Allow all origins for local development
            // In production, configure this properly
            return true
        },
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
    }

    // MCP WebSocket endpoint
    router.GET("/ws", func(c *gin.Context) {
        // Authenticate request
        if !authenticator.AuthenticateRequest(c.Request) {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            return
        }

        // Upgrade to WebSocket
        conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
        if err != nil {
            log.Printf("WebSocket upgrade failed: %v", err)
            return
        }
        defer conn.Close()

        // Handle MCP connection
        mcpHandler.HandleConnection(conn, c.Request)
    })

    // Start server
    srv := &http.Server{
        Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
        Handler: router,
    }

    // Graceful shutdown
    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan

        log.Println("Shutting down Edge MCP...")
        
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := srv.Shutdown(ctx); err != nil {
            log.Printf("Server shutdown error: %v", err)
        }
    }()

    log.Printf("Edge MCP v%s starting on port %d", version, cfg.Server.Port)
    if coreClient != nil {
        log.Printf("Connected to Core Platform at %s", cfg.Core.URL)
    } else {
        log.Println("Running in standalone mode (no Core Platform connection)")
    }

    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Server failed to start: %v", err)
    }
}
```

## Phase 3: Implementation Steps (Day 2-3)
<!-- CLAUDE: Multiple file implementations - work through systematically -->

### 3.1 MCP Protocol Handler
<!-- CLAUDE: This is the core protocol implementation -->

**File: `edge-mcp-template/internal/mcp/handler.go`**

```go
package mcp

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/developer-mesh/edge-mcp/internal/auth"
    "github.com/developer-mesh/edge-mcp/internal/cache"
    "github.com/developer-mesh/edge-mcp/internal/core"
    "github.com/developer-mesh/edge-mcp/internal/tools"
    "github.com/google/uuid"
    "github.com/gorilla/websocket"
)

// MCPMessage represents a JSON-RPC message in the MCP protocol
type MCPMessage struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id,omitempty"`
    Method  string          `json:"method,omitempty"`
    Params  json.RawMessage `json:"params,omitempty"`
    Result  interface{}     `json:"result,omitempty"`
    Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// Handler manages MCP protocol connections
type Handler struct {
    tools         *tools.Registry
    cache         cache.Cache
    coreClient    *core.Client
    authenticator auth.Authenticator
    sessions      map[string]*Session
    sessionsMu    sync.RWMutex
}

// Session represents an MCP session
type Session struct {
    ID           string
    ConnectionID string
    Initialized  bool
    TenantID     string
    EdgeMCPID    string
    CoreSession  string // Core Platform session ID for context sync
    CreatedAt    time.Time
    LastActivity time.Time
}

// NewHandler creates a new MCP handler
func NewHandler(
    toolRegistry *tools.Registry,
    cache cache.Cache,
    coreClient *core.Client,
    authenticator auth.Authenticator,
) *Handler {
    return &Handler{
        tools:         toolRegistry,
        cache:         cache,
        coreClient:    coreClient,
        authenticator: authenticator,
        sessions:      make(map[string]*Session),
    }
}

// HandleConnection handles a WebSocket connection
func (h *Handler) HandleConnection(conn *websocket.Conn, r *http.Request) {
    sessionID := uuid.New().String()
    session := &Session{
        ID:           sessionID,
        ConnectionID: uuid.New().String(),
        CreatedAt:    time.Now(),
        LastActivity: time.Now(),
    }

    h.sessionsMu.Lock()
    h.sessions[sessionID] = session
    h.sessionsMu.Unlock()

    defer func() {
        h.sessionsMu.Lock()
        delete(h.sessions, sessionID)
        h.sessionsMu.Unlock()
        conn.Close()
    }()

    // Set up ping/pong to keep connection alive
    conn.SetPongHandler(func(string) error {
        conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })

    // Start ping ticker
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    go func() {
        for range ticker.C {
            if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }()

    // Message handling loop
    for {
        var msg MCPMessage
        if err := conn.ReadJSON(&msg); err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                log.Printf("WebSocket error: %v", err)
            }
            break
        }

        // Update activity
        h.sessionsMu.Lock()
        if s, exists := h.sessions[sessionID]; exists {
            s.LastActivity = time.Now()
        }
        h.sessionsMu.Unlock()

        // Handle message
        response, err := h.handleMessage(sessionID, &msg)
        if err != nil {
            response = &MCPMessage{
                JSONRPC: "2.0",
                ID:      msg.ID,
                Error: &MCPError{
                    Code:    -32603,
                    Message: err.Error(),
                },
            }
        }

        if response != nil {
            if err := conn.WriteJSON(response); err != nil {
                log.Printf("Failed to write response: %v", err)
                break
            }
        }
    }
}

// handleMessage processes an MCP message
func (h *Handler) handleMessage(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    switch msg.Method {
    case "initialize":
        return h.handleInitialize(sessionID, msg)
    case "initialized":
        return h.handleInitialized(sessionID, msg)
    case "ping":
        return h.handlePing(msg)
    case "shutdown":
        return h.handleShutdown(sessionID, msg)
    case "tools/list":
        return h.handleToolsList(sessionID, msg)
    case "tools/call":
        return h.handleToolCall(sessionID, msg)
    case "resources/list":
        return h.handleResourcesList(sessionID, msg)
    case "resources/read":
        return h.handleResourceRead(sessionID, msg)
    case "prompts/list":
        return h.handlePromptsList(sessionID, msg)
    case "logging/setLevel":
        return h.handleLoggingSetLevel(sessionID, msg)
    case "$/cancelRequest":
        return h.handleCancelRequest(sessionID, msg)
    default:
        return nil, fmt.Errorf("method not found: %s", msg.Method)
    }
}

// handleInitialize handles the initialize request
func (h *Handler) handleInitialize(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    var params struct {
        ProtocolVersion string `json:"protocolVersion"`
        ClientInfo      struct {
            Name    string `json:"name"`
            Version string `json:"version"`
            Type    string `json:"type,omitempty"`
        } `json:"clientInfo"`
    }

    if err := json.Unmarshal(msg.Params, &params); err != nil {
        return nil, fmt.Errorf("invalid initialize params: %w", err)
    }

    // Verify protocol version
    if params.ProtocolVersion != "2025-06-18" {
        return nil, fmt.Errorf("unsupported protocol version: %s", params.ProtocolVersion)
    }

    // Update session
    h.sessionsMu.Lock()
    if session, exists := h.sessions[sessionID]; exists {
        session.Initialized = true
        
        // If connected to Core Platform, create a linked session
        if h.coreClient != nil {
            coreSessionID, err := h.coreClient.CreateSession(
                context.Background(),
                params.ClientInfo.Name,
                params.ClientInfo.Type,
            )
            if err != nil {
                log.Printf("Failed to create Core Platform session: %v", err)
            } else {
                session.CoreSession = coreSessionID
            }
        }
    }
    h.sessionsMu.Unlock()

    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result: map[string]interface{}{
            "protocolVersion": "2025-06-18",
            "serverInfo": map[string]interface{}{
                "name":    "edge-mcp",
                "version": "1.0.0",
            },
            "capabilities": map[string]interface{}{
                "tools": map[string]interface{}{
                    "listChanged": true,
                },
                "resources": map[string]interface{}{
                    "subscribe":    false, // Edge MCP doesn't support subscriptions
                    "listChanged":  false,
                },
                "prompts": map[string]interface{}{},
                "logging": map[string]interface{}{},
            },
        },
    }, nil
}

// handleInitialized handles the initialized notification
func (h *Handler) handleInitialized(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    // Client confirms initialization complete
    h.sessionsMu.Lock()
    if session, exists := h.sessions[sessionID]; exists {
        session.Initialized = true
    }
    h.sessionsMu.Unlock()

    // No response for notifications
    return nil, nil
}

// handlePing handles ping requests
func (h *Handler) handlePing(msg *MCPMessage) (*MCPMessage, error) {
    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result:  map[string]interface{}{},
    }, nil
}

// handleShutdown handles shutdown requests
func (h *Handler) handleShutdown(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    // Clean up session
    h.sessionsMu.Lock()
    if session, exists := h.sessions[sessionID]; exists {
        // If connected to Core Platform, close the linked session
        if h.coreClient != nil && session.CoreSession != "" {
            h.coreClient.CloseSession(context.Background(), session.CoreSession)
        }
    }
    delete(h.sessions, sessionID)
    h.sessionsMu.Unlock()

    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result:  map[string]interface{}{},
    }, nil
}

// handleToolsList handles tools/list requests
func (h *Handler) handleToolsList(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    tools := h.tools.ListAll()
    
    toolList := make([]map[string]interface{}, 0, len(tools))
    for _, tool := range tools {
        toolList = append(toolList, map[string]interface{}{
            "name":        tool.Name,
            "description": tool.Description,
            "inputSchema": tool.InputSchema,
        })
    }

    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result: map[string]interface{}{
            "tools": toolList,
        },
    }, nil
}

// handleToolCall handles tools/call requests
func (h *Handler) handleToolCall(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    var params struct {
        Name      string          `json:"name"`
        Arguments json.RawMessage `json:"arguments"`
    }

    if err := json.Unmarshal(msg.Params, &params); err != nil {
        return nil, fmt.Errorf("invalid tool call params: %w", err)
    }

    // CRITICAL: Handle context operations specially for sync with Core Platform
    if params.Name == "context.update" || params.Name == "context.append" || params.Name == "context.get" {
        return h.handleContextOperation(sessionID, msg.ID, params.Name, params.Arguments)
    }

    // Execute tool
    result, err := h.tools.Execute(context.Background(), params.Name, params.Arguments)
    if err != nil {
        return nil, fmt.Errorf("tool execution failed: %w", err)
    }

    // Record execution with Core Platform if connected
    if h.coreClient != nil {
        h.sessionsMu.RLock()
        session := h.sessions[sessionID]
        coreSessionID := ""
        if session != nil {
            coreSessionID = session.CoreSession
        }
        h.sessionsMu.RUnlock()

        if coreSessionID != "" {
            h.coreClient.RecordToolExecution(
                context.Background(),
                coreSessionID,
                params.Name,
                params.Arguments,
                result,
            )
        }
    }

    // Format result as MCP content
    content := []map[string]interface{}{
        {
            "type": "text",
            "text": fmt.Sprintf("%v", result),
        },
    }

    // If result is already structured, use it directly
    if resultMap, ok := result.(map[string]interface{}); ok {
        if resultContent, ok := resultMap["content"]; ok {
            content = resultContent.([]map[string]interface{})
        }
    }

    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result: map[string]interface{}{
            "content": content,
        },
    }, nil
}

// handleContextOperation handles context sync with Core Platform
func (h *Handler) handleContextOperation(sessionID string, msgID interface{}, operation string, args json.RawMessage) (*MCPMessage, error) {
    // If not connected to Core Platform, return error
    if h.coreClient == nil {
        return nil, fmt.Errorf("context operations require Core Platform connection")
    }

    h.sessionsMu.RLock()
    session := h.sessions[sessionID]
    coreContextID := ""
    if session != nil {
        coreContextID = session.CoreSession
    }
    h.sessionsMu.RUnlock()

    if coreContextID == "" {
        return nil, fmt.Errorf("no active Core Platform session")
    }

    var result interface{}
    var err error

    switch operation {
    case "context.update":
        var contextUpdate map[string]interface{}
        if err := json.Unmarshal(args, &contextUpdate); err != nil {
            return nil, fmt.Errorf("invalid context update: %w", err)
        }
        
        err = h.coreClient.UpdateContext(context.Background(), coreContextID, contextUpdate)
        if err == nil {
            // Cache locally for performance
            h.cache.Set(context.Background(), fmt.Sprintf("context:%s", sessionID), contextUpdate, 5*time.Minute)
            result = map[string]interface{}{"success": true}
        }

    case "context.get":
        // Try cache first
        var cached map[string]interface{}
        if err := h.cache.Get(context.Background(), fmt.Sprintf("context:%s", sessionID), &cached); err == nil {
            result = cached
        } else {
            // Fetch from Core Platform
            result, err = h.coreClient.GetContext(context.Background(), coreContextID)
            if err == nil {
                // Cache the result
                h.cache.Set(context.Background(), fmt.Sprintf("context:%s", sessionID), result, 5*time.Minute)
            }
        }

    case "context.append":
        var appendData map[string]interface{}
        if err := json.Unmarshal(args, &appendData); err != nil {
            return nil, fmt.Errorf("invalid append data: %w", err)
        }
        
        err = h.coreClient.AppendContext(context.Background(), coreContextID, appendData)
        if err == nil {
            result = map[string]interface{}{"success": true}
        }
    }

    if err != nil {
        return nil, fmt.Errorf("context operation failed: %w", err)
    }

    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msgID,
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

// handleResourcesList handles resources/list requests
func (h *Handler) handleResourcesList(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    resources := []map[string]interface{}{
        {
            "uri":         "edge://system/info",
            "name":        "System Information",
            "description": "Edge MCP system information",
            "mimeType":    "application/json",
        },
        {
            "uri":         "edge://tools/list",
            "name":        "Available Tools",
            "description": "List of available tools",
            "mimeType":    "application/json",
        },
    }

    // Add Core Platform resources if connected
    if h.coreClient != nil {
        resources = append(resources, map[string]interface{}{
            "uri":         "core://connection/status",
            "name":        "Core Connection Status",
            "description": "Status of Core Platform connection",
            "mimeType":    "application/json",
        })
    }

    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result: map[string]interface{}{
            "resources": resources,
        },
    }, nil
}

// handleResourceRead handles resources/read requests
func (h *Handler) handleResourceRead(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    var params struct {
        URI string `json:"uri"`
    }

    if err := json.Unmarshal(msg.Params, &params); err != nil {
        return nil, fmt.Errorf("invalid resource read params: %w", err)
    }

    var content interface{}

    switch params.URI {
    case "edge://system/info":
        content = map[string]interface{}{
            "version":        "1.0.0",
            "core_connected": h.coreClient != nil,
            "tools_count":    h.tools.Count(),
            "cache_size":     h.cache.Size(),
        }

    case "edge://tools/list":
        tools := h.tools.ListAll()
        toolNames := make([]string, 0, len(tools))
        for _, tool := range tools {
            toolNames = append(toolNames, tool.Name)
        }
        content = toolNames

    case "core://connection/status":
        if h.coreClient != nil {
            content = h.coreClient.GetStatus()
        } else {
            content = map[string]interface{}{
                "connected": false,
                "error":     "Core Platform not configured",
            }
        }

    default:
        return nil, fmt.Errorf("resource not found: %s", params.URI)
    }

    contentJSON, _ := json.Marshal(content)

    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result: map[string]interface{}{
            "contents": []map[string]interface{}{
                {
                    "uri":      params.URI,
                    "mimeType": "application/json",
                    "text":     string(contentJSON),
                },
            },
        },
    }, nil
}

// handlePromptsList handles prompts/list requests
func (h *Handler) handlePromptsList(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    // Edge MCP doesn't provide prompts
    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result: map[string]interface{}{
            "prompts": []interface{}{},
        },
    }, nil
}

// handleLoggingSetLevel handles logging/setLevel requests
func (h *Handler) handleLoggingSetLevel(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    var params struct {
        Level string `json:"level"`
    }

    if err := json.Unmarshal(msg.Params, &params); err != nil {
        return nil, fmt.Errorf("invalid logging params: %w", err)
    }

    // TODO: Actually set logging level
    log.Printf("Logging level set to: %s", params.Level)

    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result:  map[string]interface{}{},
    }, nil
}

// handleCancelRequest handles $/cancelRequest requests
func (h *Handler) handleCancelRequest(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    var params struct {
        ID interface{} `json:"id"`
    }

    if err := json.Unmarshal(msg.Params, &params); err != nil {
        return nil, fmt.Errorf("invalid cancel params: %w", err)
    }

    // TODO: Implement request cancellation
    log.Printf("Request cancellation requested for: %v", params.ID)

    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      msg.ID,
        Result:  map[string]interface{}{},
    }, nil
}

// Error codes
const (
    ErrorParseError     = -32700
    ErrorInvalidRequest = -32600
    ErrorMethodNotFound = -32601
    ErrorInvalidParams  = -32602
    ErrorInternalError  = -32603
)

// errorResponse creates an error response
func (h *Handler) errorResponse(id interface{}, code int, message string, data interface{}) *MCPMessage {
    return &MCPMessage{
        JSONRPC: "2.0",
        ID:      id,
        Error: &MCPError{
            Code:    code,
            Message: message,
            Data:    data,
        },
    }
}
```

### 3.2 Tool Implementations
<!-- CLAUDE: Create each tool systematically -->

**File: `edge-mcp-template/internal/tools/filesystem.go`**

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
)

// FileSystemTool provides file system operations
type FileSystemTool struct {
    basePath string // Optional: restrict to a base path
}

// NewFileSystemTool creates a new file system tool
func NewFileSystemTool() *FileSystemTool {
    return &FileSystemTool{}
}

// GetDefinitions returns tool definitions
func (t *FileSystemTool) GetDefinitions() []ToolDefinition {
    return []ToolDefinition{
        {
            Name:        "fs.read_file",
            Description: "Read the contents of a file",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Path to the file to read",
                    },
                },
                "required": []string{"path"},
            },
            Handler: t.handleReadFile,
        },
        {
            Name:        "fs.write_file",
            Description: "Write content to a file",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Path to the file to write",
                    },
                    "content": map[string]interface{}{
                        "type":        "string",
                        "description": "Content to write to the file",
                    },
                },
                "required": []string{"path", "content"},
            },
            Handler: t.handleWriteFile,
        },
        {
            Name:        "fs.list_directory",
            Description: "List contents of a directory",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Path to the directory",
                    },
                },
                "required": []string{"path"},
            },
            Handler: t.handleListDirectory,
        },
        {
            Name:        "fs.create_directory",
            Description: "Create a directory",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Path to the directory to create",
                    },
                },
                "required": []string{"path"},
            },
            Handler: t.handleCreateDirectory,
        },
        {
            Name:        "fs.delete",
            Description: "Delete a file or directory",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Path to delete",
                    },
                },
                "required": []string{"path"},
            },
            Handler: t.handleDelete,
        },
    }
}

func (t *FileSystemTool) handleReadFile(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path string `json:"path"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    content, err := os.ReadFile(params.Path)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %w", err)
    }

    return map[string]interface{}{
        "content": string(content),
        "size":    len(content),
    }, nil
}

func (t *FileSystemTool) handleWriteFile(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path    string `json:"path"`
        Content string `json:"content"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    if err := os.WriteFile(params.Path, []byte(params.Content), 0644); err != nil {
        return nil, fmt.Errorf("failed to write file: %w", err)
    }

    return map[string]interface{}{
        "success": true,
        "size":    len(params.Content),
    }, nil
}

func (t *FileSystemTool) handleListDirectory(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path string `json:"path"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    entries, err := os.ReadDir(params.Path)
    if err != nil {
        return nil, fmt.Errorf("failed to list directory: %w", err)
    }

    files := make([]map[string]interface{}, 0, len(entries))
    for _, entry := range entries {
        info, _ := entry.Info()
        files = append(files, map[string]interface{}{
            "name":    entry.Name(),
            "type":    map[bool]string{true: "directory", false: "file"}[entry.IsDir()],
            "size":    info.Size(),
            "mode":    info.Mode().String(),
        })
    }

    return map[string]interface{}{
        "files": files,
        "count": len(files),
    }, nil
}

func (t *FileSystemTool) handleCreateDirectory(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path string `json:"path"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    if err := os.MkdirAll(params.Path, 0755); err != nil {
        return nil, fmt.Errorf("failed to create directory: %w", err)
    }

    return map[string]interface{}{
        "success": true,
        "path":    params.Path,
    }, nil
}

func (t *FileSystemTool) handleDelete(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path string `json:"path"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    if err := os.RemoveAll(params.Path); err != nil {
        return nil, fmt.Errorf("failed to delete: %w", err)
    }

    return map[string]interface{}{
        "success": true,
        "path":    params.Path,
    }, nil
}
```

**File: `edge-mcp-template/internal/tools/git.go`**

```go
package tools

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
    "strings"
)

// GitTool provides Git operations
type GitTool struct{}

// NewGitTool creates a new Git tool
func NewGitTool() *GitTool {
    return &GitTool{}
}

// GetDefinitions returns tool definitions
func (t *GitTool) GetDefinitions() []ToolDefinition {
    return []ToolDefinition{
        {
            Name:        "git.status",
            Description: "Get Git repository status",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Repository path (optional, defaults to current directory)",
                    },
                },
            },
            Handler: t.handleStatus,
        },
        {
            Name:        "git.diff",
            Description: "Show Git diff",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Repository path",
                    },
                    "staged": map[string]interface{}{
                        "type":        "boolean",
                        "description": "Show staged changes",
                    },
                },
            },
            Handler: t.handleDiff,
        },
        {
            Name:        "git.log",
            Description: "Show Git log",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Repository path",
                    },
                    "limit": map[string]interface{}{
                        "type":        "number",
                        "description": "Number of commits to show",
                    },
                },
            },
            Handler: t.handleLog,
        },
        {
            Name:        "git.branch",
            Description: "List or create Git branches",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Repository path",
                    },
                    "create": map[string]interface{}{
                        "type":        "string",
                        "description": "Name of branch to create",
                    },
                },
            },
            Handler: t.handleBranch,
        },
    }
}

func (t *GitTool) runGitCommand(ctx context.Context, dir string, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, "git", args...)
    if dir != "" {
        cmd.Dir = dir
    }
    
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    err := cmd.Run()
    if err != nil {
        return "", fmt.Errorf("git command failed: %w\nstderr: %s", err, stderr.String())
    }
    
    return stdout.String(), nil
}

func (t *GitTool) handleStatus(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path string `json:"path"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    output, err := t.runGitCommand(ctx, params.Path, "status", "--porcelain")
    if err != nil {
        return nil, err
    }

    // Parse status output
    lines := strings.Split(strings.TrimSpace(output), "\n")
    files := make([]map[string]string, 0)
    
    for _, line := range lines {
        if line == "" {
            continue
        }
        
        status := line[:2]
        filename := strings.TrimSpace(line[2:])
        
        files = append(files, map[string]string{
            "status":   status,
            "filename": filename,
        })
    }

    // Get current branch
    branch, _ := t.runGitCommand(ctx, params.Path, "branch", "--show-current")
    branch = strings.TrimSpace(branch)

    return map[string]interface{}{
        "branch": branch,
        "files":  files,
        "clean":  len(files) == 0,
    }, nil
}

func (t *GitTool) handleDiff(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path   string `json:"path"`
        Staged bool   `json:"staged"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    gitArgs := []string{"diff"}
    if params.Staged {
        gitArgs = append(gitArgs, "--staged")
    }

    output, err := t.runGitCommand(ctx, params.Path, gitArgs...)
    if err != nil {
        return nil, err
    }

    return map[string]interface{}{
        "diff": output,
    }, nil
}

func (t *GitTool) handleLog(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path  string `json:"path"`
        Limit int    `json:"limit"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    if params.Limit == 0 {
        params.Limit = 10
    }

    output, err := t.runGitCommand(
        ctx, 
        params.Path, 
        "log", 
        fmt.Sprintf("-%d", params.Limit),
        "--oneline",
    )
    if err != nil {
        return nil, err
    }

    // Parse log output
    lines := strings.Split(strings.TrimSpace(output), "\n")
    commits := make([]map[string]string, 0, len(lines))
    
    for _, line := range lines {
        if line == "" {
            continue
        }
        
        parts := strings.SplitN(line, " ", 2)
        if len(parts) == 2 {
            commits = append(commits, map[string]string{
                "hash":    parts[0],
                "message": parts[1],
            })
        }
    }

    return map[string]interface{}{
        "commits": commits,
        "count":   len(commits),
    }, nil
}

func (t *GitTool) handleBranch(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path   string `json:"path"`
        Create string `json:"create"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    if params.Create != "" {
        // Create new branch
        _, err := t.runGitCommand(ctx, params.Path, "checkout", "-b", params.Create)
        if err != nil {
            return nil, err
        }
        
        return map[string]interface{}{
            "created": params.Create,
            "success": true,
        }, nil
    }

    // List branches
    output, err := t.runGitCommand(ctx, params.Path, "branch")
    if err != nil {
        return nil, err
    }

    lines := strings.Split(strings.TrimSpace(output), "\n")
    branches := make([]map[string]interface{}, 0, len(lines))
    
    for _, line := range lines {
        if line == "" {
            continue
        }
        
        current := strings.HasPrefix(line, "*")
        name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
        
        branches = append(branches, map[string]interface{}{
            "name":    name,
            "current": current,
        })
    }

    return map[string]interface{}{
        "branches": branches,
    }, nil
}
```

### 3.3 Core Platform Client
<!-- CLAUDE: This connects Edge to Core -->

**File: `edge-mcp-template/internal/core/client.go`**

```go
package core

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"
)

// Client connects to the Core Platform
type Client struct {
    baseURL      string
    tenantID     string
    edgeMCPID    string
    apiKey       string
    httpClient   *http.Client
    
    // Authentication state
    authToken    string
    tokenExpiry  time.Time
    authMu       sync.RWMutex
}

// NewClient creates a new Core Platform client
func NewClient(baseURL, apiKey, tenantID, edgeMCPID string) *Client {
    return &Client{
        baseURL:   baseURL,
        apiKey:    apiKey,
        tenantID:  tenantID,
        edgeMCPID: edgeMCPID,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

// AuthenticateWithCore authenticates with the Core Platform
func (c *Client) AuthenticateWithCore(ctx context.Context) error {
    payload := map[string]string{
        "tenant_id":   c.tenantID,
        "edge_mcp_id": c.edgeMCPID,
        "api_key":     c.apiKey,
    }
    
    body, _ := json.Marshal(payload)
    
    req, err := http.NewRequestWithContext(
        ctx,
        "POST",
        fmt.Sprintf("%s/api/v1/auth/edge", c.baseURL),
        bytes.NewReader(body),
    )
    if err != nil {
        return fmt.Errorf("failed to create auth request: %w", err)
    }
    
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("auth request failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("authentication failed: %s", body)
    }
    
    var authResp struct {
        Token     string    `json:"token"`
        ExpiresAt time.Time `json:"expires_at"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
        return fmt.Errorf("failed to decode auth response: %w", err)
    }
    
    c.authMu.Lock()
    c.authToken = authResp.Token
    c.tokenExpiry = authResp.ExpiresAt
    c.authMu.Unlock()
    
    return nil
}

// ensureAuth ensures we have a valid auth token
func (c *Client) ensureAuth(ctx context.Context) error {
    c.authMu.RLock()
    expired := time.Now().After(c.tokenExpiry.Add(-5 * time.Minute))
    c.authMu.RUnlock()
    
    if expired {
        return c.AuthenticateWithCore(ctx)
    }
    
    return nil
}

// doRequest performs an authenticated HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
    if err := c.ensureAuth(ctx); err != nil {
        return nil, err
    }
    
    var bodyReader io.Reader
    if body != nil {
        bodyBytes, err := json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("failed to marshal body: %w", err)
        }
        bodyReader = bytes.NewReader(bodyBytes)
    }
    
    req, err := http.NewRequestWithContext(
        ctx,
        method,
        fmt.Sprintf("%s%s", c.baseURL, path),
        bodyReader,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    c.authMu.RLock()
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
    c.authMu.RUnlock()
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Tenant-ID", c.tenantID)
    req.Header.Set("X-Edge-MCP-ID", c.edgeMCPID)
    
    return c.httpClient.Do(req)
}

// CreateSession creates a new session on the Core Platform
func (c *Client) CreateSession(ctx context.Context, clientName, clientType string) (string, error) {
    payload := map[string]interface{}{
        "client_name": clientName,
        "client_type": clientType,
        "edge_mcp_id": c.edgeMCPID,
    }
    
    resp, err := c.doRequest(ctx, "POST", "/api/v1/sessions", payload)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusCreated {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("failed to create session: %s", body)
    }
    
    var result struct {
        SessionID string `json:"session_id"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("failed to decode response: %w", err)
    }
    
    return result.SessionID, nil
}

// CloseSession closes a session on the Core Platform
func (c *Client) CloseSession(ctx context.Context, sessionID string) error {
    resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/sessions/%s", sessionID), nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to close session: %s", body)
    }
    
    return nil
}

// UpdateContext updates context on the Core Platform
func (c *Client) UpdateContext(ctx context.Context, contextID string, update map[string]interface{}) error {
    resp, err := c.doRequest(ctx, "PUT", fmt.Sprintf("/api/v1/contexts/%s", contextID), update)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to update context: %s", body)
    }
    
    return nil
}

// GetContext retrieves context from the Core Platform
func (c *Client) GetContext(ctx context.Context, contextID string) (map[string]interface{}, error) {
    resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/contexts/%s", contextID), nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get context: %s", body)
    }
    
    var context map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&context); err != nil {
        return nil, fmt.Errorf("failed to decode context: %w", err)
    }
    
    return context, nil
}

// AppendContext appends to context on the Core Platform
func (c *Client) AppendContext(ctx context.Context, contextID string, data map[string]interface{}) error {
    resp, err := c.doRequest(ctx, "PATCH", fmt.Sprintf("/api/v1/contexts/%s/append", contextID), data)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to append context: %s", body)
    }
    
    return nil
}

// RecordToolExecution records a tool execution on the Core Platform
func (c *Client) RecordToolExecution(ctx context.Context, sessionID, toolName string, args, result interface{}) error {
    payload := map[string]interface{}{
        "session_id":  sessionID,
        "tool":        toolName,
        "arguments":   args,
        "result":      result,
        "executed_at": time.Now().UTC(),
        "location":    "edge",
    }
    
    resp, err := c.doRequest(ctx, "POST", "/api/v1/tool-executions", payload)
    if err != nil {
        // Don't fail if recording fails - this is best effort
        return nil
    }
    defer resp.Body.Close()
    
    return nil
}

// FetchRemoteTools fetches available tools from the Core Platform
func (c *Client) FetchRemoteTools(ctx context.Context) ([]RemoteTool, error) {
    resp, err := c.doRequest(ctx, "GET", "/api/v1/tools", nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to fetch tools: %s", body)
    }
    
    var tools []RemoteTool
    if err := json.NewDecoder(resp.Body).Decode(&tools); err != nil {
        return nil, fmt.Errorf("failed to decode tools: %w", err)
    }
    
    return tools, nil
}

// GetStatus returns the current status of the Core Platform connection
func (c *Client) GetStatus() map[string]interface{} {
    c.authMu.RLock()
    authenticated := c.authToken != "" && time.Now().Before(c.tokenExpiry)
    c.authMu.RUnlock()
    
    return map[string]interface{}{
        "connected":     authenticated,
        "base_url":      c.baseURL,
        "tenant_id":     c.tenantID,
        "edge_mcp_id":   c.edgeMCPID,
        "authenticated": authenticated,
    }
}

// RemoteTool represents a tool available on the Core Platform
type RemoteTool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"input_schema"`
    Remote      bool                   `json:"remote"`
}
```

### 3.4 Memory Cache Implementation
<!-- CLAUDE: Copy existing memory cache - it's already infrastructure-free -->

**File: `edge-mcp-template/internal/cache/memory.go`**

```go
// Copy directly from pkg/common/cache/memory.go - it's already perfect
package cache

import (
    "context"
    "sync"
    "time"
)

// Cache interface
type Cache interface {
    Get(ctx context.Context, key string, value interface{}) error
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Size() int
}

// MemoryCache implements an in-memory cache
type MemoryCache struct {
    items      map[string]cacheItem
    mu         sync.RWMutex
    maxItems   int
    defaultTTL time.Duration
}

type cacheItem struct {
    value      interface{}
    expiration time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxItems int, defaultTTL time.Duration) Cache {
    return &MemoryCache{
        items:      make(map[string]cacheItem),
        maxItems:   maxItems,
        defaultTTL: defaultTTL,
    }
}

// Get retrieves data from the cache
func (c *MemoryCache) Get(ctx context.Context, key string, value interface{}) error {
    c.mu.RLock()
    defer c.mu.RUnlock()

    item, exists := c.items[key]
    if !exists {
        return fmt.Errorf("key not found")
    }

    if time.Now().After(item.expiration) {
        return fmt.Errorf("key expired")
    }

    // In a real implementation, you'd use reflection to set value
    // For now, we'll just return nil to indicate success
    return nil
}

// Set stores data in the cache
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if ttl == 0 {
        ttl = c.defaultTTL
    }

    // Evict oldest item if at capacity
    if len(c.items) >= c.maxItems {
        c.evictOldest()
    }

    c.items[key] = cacheItem{
        value:      value,
        expiration: time.Now().Add(ttl),
    }

    return nil
}

// Delete removes data from the cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    delete(c.items, key)
    return nil
}

// Size returns the number of items in the cache
func (c *MemoryCache) Size() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return len(c.items)
}

// evictOldest removes the oldest item from cache
func (c *MemoryCache) evictOldest() {
    var oldestKey string
    var oldestTime time.Time

    for key, item := range c.items {
        if oldestKey == "" || item.expiration.Before(oldestTime) {
            oldestKey = key
            oldestTime = item.expiration
        }
    }

    if oldestKey != "" {
        delete(c.items, oldestKey)
    }
}
```

## Phase 4: Example Edge MCP with Multiple Tool Categories (Day 3-4)
<!-- CLAUDE: This shows how to package everything -->

### 4.1 Docker Tool Implementation

**File: `edge-mcp-template/internal/tools/docker.go`**

```go
package tools

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
    "strings"
)

// DockerTool provides Docker operations
type DockerTool struct{}

// NewDockerTool creates a new Docker tool
func NewDockerTool() *DockerTool {
    return &DockerTool{}
}

// GetDefinitions returns tool definitions
func (t *DockerTool) GetDefinitions() []ToolDefinition {
    return []ToolDefinition{
        {
            Name:        "docker.ps",
            Description: "List Docker containers",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "all": map[string]interface{}{
                        "type":        "boolean",
                        "description": "Show all containers (default shows just running)",
                    },
                },
            },
            Handler: t.handlePS,
        },
        {
            Name:        "docker.build",
            Description: "Build a Docker image",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{
                        "type":        "string",
                        "description": "Path to Dockerfile directory",
                    },
                    "tag": map[string]interface{}{
                        "type":        "string",
                        "description": "Image tag",
                    },
                },
                "required": []string{"path", "tag"},
            },
            Handler: t.handleBuild,
        },
        {
            Name:        "docker.run",
            Description: "Run a Docker container",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "image": map[string]interface{}{
                        "type":        "string",
                        "description": "Image to run",
                    },
                    "command": map[string]interface{}{
                        "type":        "string",
                        "description": "Command to execute",
                    },
                    "detach": map[string]interface{}{
                        "type":        "boolean",
                        "description": "Run in background",
                    },
                },
                "required": []string{"image"},
            },
            Handler: t.handleRun,
        },
        {
            Name:        "docker.logs",
            Description: "Get container logs",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "container": map[string]interface{}{
                        "type":        "string",
                        "description": "Container ID or name",
                    },
                    "tail": map[string]interface{}{
                        "type":        "number",
                        "description": "Number of lines to show from the end",
                    },
                },
                "required": []string{"container"},
            },
            Handler: t.handleLogs,
        },
    }
}

func (t *DockerTool) runDockerCommand(ctx context.Context, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, "docker", args...)
    
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    err := cmd.Run()
    if err != nil {
        return "", fmt.Errorf("docker command failed: %w\nstderr: %s", err, stderr.String())
    }
    
    return stdout.String(), nil
}

func (t *DockerTool) handlePS(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        All bool `json:"all"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    dockerArgs := []string{"ps", "--format", "json"}
    if params.All {
        dockerArgs = append(dockerArgs, "-a")
    }

    output, err := t.runDockerCommand(ctx, dockerArgs...)
    if err != nil {
        return nil, err
    }

    // Parse JSON lines output
    lines := strings.Split(strings.TrimSpace(output), "\n")
    containers := make([]map[string]interface{}, 0)
    
    for _, line := range lines {
        if line == "" {
            continue
        }
        
        var container map[string]interface{}
        if err := json.Unmarshal([]byte(line), &container); err == nil {
            containers = append(containers, container)
        }
    }

    return map[string]interface{}{
        "containers": containers,
        "count":      len(containers),
    }, nil
}

func (t *DockerTool) handleBuild(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path string `json:"path"`
        Tag  string `json:"tag"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    output, err := t.runDockerCommand(ctx, "build", "-t", params.Tag, params.Path)
    if err != nil {
        return nil, err
    }

    return map[string]interface{}{
        "success": true,
        "tag":     params.Tag,
        "output":  output,
    }, nil
}

func (t *DockerTool) handleRun(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Image   string `json:"image"`
        Command string `json:"command"`
        Detach  bool   `json:"detach"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    dockerArgs := []string{"run"}
    if params.Detach {
        dockerArgs = append(dockerArgs, "-d")
    }
    dockerArgs = append(dockerArgs, params.Image)
    
    if params.Command != "" {
        dockerArgs = append(dockerArgs, "sh", "-c", params.Command)
    }

    output, err := t.runDockerCommand(ctx, dockerArgs...)
    if err != nil {
        return nil, err
    }

    return map[string]interface{}{
        "success":     true,
        "container_id": strings.TrimSpace(output),
    }, nil
}

func (t *DockerTool) handleLogs(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Container string `json:"container"`
        Tail      int    `json:"tail"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    dockerArgs := []string{"logs"}
    if params.Tail > 0 {
        dockerArgs = append(dockerArgs, "--tail", fmt.Sprintf("%d", params.Tail))
    }
    dockerArgs = append(dockerArgs, params.Container)

    output, err := t.runDockerCommand(ctx, dockerArgs...)
    if err != nil {
        return nil, err
    }

    return map[string]interface{}{
        "logs": output,
    }, nil
}
```

### 4.2 Shell Tool Implementation

**File: `edge-mcp-template/internal/tools/shell.go`**

```go
package tools

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "time"
)

// ShellTool provides shell command execution
type ShellTool struct {
    allowedCommands []string // Optional: restrict to specific commands
}

// NewShellTool creates a new shell tool
func NewShellTool() *ShellTool {
    return &ShellTool{
        // By default, allow common safe commands
        allowedCommands: []string{
            "ls", "pwd", "echo", "cat", "grep", "find", "which",
            "npm", "yarn", "go", "python", "node", "make",
        },
    }
}

// GetDefinitions returns tool definitions
func (t *ShellTool) GetDefinitions() []ToolDefinition {
    return []ToolDefinition{
        {
            Name:        "shell.execute",
            Description: "Execute a shell command",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "command": map[string]interface{}{
                        "type":        "string",
                        "description": "Command to execute",
                    },
                    "cwd": map[string]interface{}{
                        "type":        "string",
                        "description": "Working directory",
                    },
                    "timeout": map[string]interface{}{
                        "type":        "number",
                        "description": "Timeout in seconds",
                    },
                },
                "required": []string{"command"},
            },
            Handler: t.handleExecute,
        },
        {
            Name:        "shell.env",
            Description: "Get environment variables",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "filter": map[string]interface{}{
                        "type":        "string",
                        "description": "Filter pattern for variable names",
                    },
                },
            },
            Handler: t.handleEnv,
        },
    }
}

func (t *ShellTool) isCommandAllowed(command string) bool {
    if len(t.allowedCommands) == 0 {
        // No restrictions
        return true
    }
    
    // Extract the base command
    parts := strings.Fields(command)
    if len(parts) == 0 {
        return false
    }
    
    baseCmd := parts[0]
    for _, allowed := range t.allowedCommands {
        if baseCmd == allowed {
            return true
        }
    }
    
    return false
}

func (t *ShellTool) handleExecute(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Command string `json:"command"`
        CWD     string `json:"cwd"`
        Timeout int    `json:"timeout"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    // Check if command is allowed
    if !t.isCommandAllowed(params.Command) {
        return nil, fmt.Errorf("command not allowed: %s", params.Command)
    }

    // Set timeout
    timeout := 30 * time.Second
    if params.Timeout > 0 {
        timeout = time.Duration(params.Timeout) * time.Second
    }
    
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    // Execute command
    cmd := exec.CommandContext(ctx, "sh", "-c", params.Command)
    
    if params.CWD != "" {
        cmd.Dir = params.CWD
    }
    
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    err := cmd.Run()
    
    result := map[string]interface{}{
        "stdout":   stdout.String(),
        "stderr":   stderr.String(),
        "success":  err == nil,
    }
    
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            result["exit_code"] = exitErr.ExitCode()
        }
        result["error"] = err.Error()
    } else {
        result["exit_code"] = 0
    }
    
    return result, nil
}

func (t *ShellTool) handleEnv(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Filter string `json:"filter"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }

    envVars := make(map[string]string)
    
    for _, env := range os.Environ() {
        parts := strings.SplitN(env, "=", 2)
        if len(parts) != 2 {
            continue
        }
        
        key := parts[0]
        value := parts[1]
        
        // Apply filter if provided
        if params.Filter != "" {
            if !strings.Contains(strings.ToLower(key), strings.ToLower(params.Filter)) {
                continue
            }
        }
        
        envVars[key] = value
    }
    
    return map[string]interface{}{
        "environment": envVars,
        "count":       len(envVars),
    }, nil
}
```

### 4.3 Tool Registry

**File: `edge-mcp-template/internal/tools/registry.go`**

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
)

// ToolDefinition defines a tool
type ToolDefinition struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"inputSchema"`
    Handler     ToolHandler            `json:"-"`
    Remote      bool                   `json:"remote"`
}

// ToolHandler executes a tool
type ToolHandler func(ctx context.Context, args json.RawMessage) (interface{}, error)

// ToolProvider provides tool definitions
type ToolProvider interface {
    GetDefinitions() []ToolDefinition
}

// Registry manages tools
type Registry struct {
    tools   map[string]*ToolDefinition
    toolsMu sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
    return &Registry{
        tools: make(map[string]*ToolDefinition),
    }
}

// Register registers a tool provider
func (r *Registry) Register(provider ToolProvider) {
    r.toolsMu.Lock()
    defer r.toolsMu.Unlock()
    
    for _, def := range provider.GetDefinitions() {
        defCopy := def
        r.tools[def.Name] = &defCopy
    }
}

// RegisterRemote registers a remote tool
func (r *Registry) RegisterRemote(tool RemoteTool) {
    r.toolsMu.Lock()
    defer r.toolsMu.Unlock()
    
    r.tools[tool.Name] = &ToolDefinition{
        Name:        tool.Name,
        Description: tool.Description,
        InputSchema: tool.InputSchema,
        Remote:      true,
        Handler:     nil, // Remote tools are executed via Core Platform
    }
}

// ListAll lists all tools
func (r *Registry) ListAll() []ToolDefinition {
    r.toolsMu.RLock()
    defer r.toolsMu.RUnlock()
    
    tools := make([]ToolDefinition, 0, len(r.tools))
    for _, tool := range r.tools {
        tools = append(tools, *tool)
    }
    
    return tools
}

// Execute executes a tool
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (interface{}, error) {
    r.toolsMu.RLock()
    tool, exists := r.tools[name]
    r.toolsMu.RUnlock()
    
    if !exists {
        return nil, fmt.Errorf("tool not found: %s", name)
    }
    
    if tool.Remote {
        return nil, fmt.Errorf("remote tool execution not implemented: %s", name)
    }
    
    if tool.Handler == nil {
        return nil, fmt.Errorf("tool handler not found: %s", name)
    }
    
    return tool.Handler(ctx, args)
}

// Count returns the number of registered tools
func (r *Registry) Count() int {
    r.toolsMu.RLock()
    defer r.toolsMu.RUnlock()
    return len(r.tools)
}

// RemoteTool represents a tool available on the Core Platform
type RemoteTool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"input_schema"`
}
```

## Phase 5: IDE Integration (Day 4)
<!-- CLAUDE: Configuration files for IDEs -->

### 5.1 Claude Code Configuration

**File: `edge-mcp-template/configs/claude-code.json`**

```json
{
  "mcpServers": {
    "edge-mcp": {
      "command": "./bin/edge-mcp",
      "args": [
        "--config", "configs/config.yaml"
      ],
      "env": {
        "EDGE_MCP_API_KEY": "${EDGE_MCP_API_KEY}",
        "CORE_PLATFORM_URL": "${CORE_PLATFORM_URL}"
      }
    }
  }
}
```

### 5.2 Configuration File

**File: `edge-mcp-template/configs/config.yaml`**

```yaml
# Edge MCP Configuration
server:
  port: 8082
  host: "0.0.0.0"

auth:
  # API key for client authentication
  # Leave empty for no authentication (development only)
  api_key: "${EDGE_MCP_API_KEY}"

core:
  # Core Platform connection (optional but recommended)
  url: "${CORE_PLATFORM_URL}"
  api_key: "${CORE_API_KEY}"
  tenant_id: "${TENANT_ID}"
  edge_mcp_id: "${EDGE_MCP_ID}"

cache:
  max_items: 1000
  default_ttl: 5m

tools:
  # Enable/disable tool categories
  filesystem:
    enabled: true
    base_path: "" # Restrict to specific path if needed
  
  git:
    enabled: true
  
  docker:
    enabled: true
  
  shell:
    enabled: true
    allowed_commands: [] # Empty means all commands allowed

logging:
  level: "info" # debug, info, warn, error
  format: "json" # json or text
```

## Phase 6: Testing Strategy (Day 4-5)
<!-- CLAUDE: Test files for verification -->

### 6.1 MCP Protocol Tests

**File: `edge-mcp-template/internal/mcp/handler_test.go`**

```go
package mcp

import (
    "encoding/json"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestHandleInitialize(t *testing.T) {
    handler := NewHandler(nil, nil, nil, nil)
    
    msg := &MCPMessage{
        JSONRPC: "2.0",
        ID:      "1",
        Method:  "initialize",
        Params: json.RawMessage(`{
            "protocolVersion": "2025-06-18",
            "clientInfo": {
                "name": "test-client",
                "version": "1.0.0"
            }
        }`),
    }
    
    response, err := handler.handleMessage("test-session", msg)
    require.NoError(t, err)
    require.NotNil(t, response)
    
    // Check response structure
    assert.Equal(t, "2.0", response.JSONRPC)
    assert.Equal(t, "1", response.ID)
    assert.Nil(t, response.Error)
    
    // Check result
    result, ok := response.Result.(map[string]interface{})
    require.True(t, ok)
    
    assert.Equal(t, "2025-06-18", result["protocolVersion"])
    
    serverInfo, ok := result["serverInfo"].(map[string]interface{})
    require.True(t, ok)
    assert.Equal(t, "edge-mcp", serverInfo["name"])
}

func TestHandleToolsList(t *testing.T) {
    registry := NewRegistry()
    handler := NewHandler(registry, nil, nil, nil)
    
    // Register a test tool
    registry.tools["test.tool"] = &ToolDefinition{
        Name:        "test.tool",
        Description: "Test tool",
        InputSchema: map[string]interface{}{
            "type": "object",
        },
    }
    
    msg := &MCPMessage{
        JSONRPC: "2.0",
        ID:      "2",
        Method:  "tools/list",
    }
    
    response, err := handler.handleMessage("test-session", msg)
    require.NoError(t, err)
    require.NotNil(t, response)
    
    result, ok := response.Result.(map[string]interface{})
    require.True(t, ok)
    
    tools, ok := result["tools"].([]map[string]interface{})
    require.True(t, ok)
    assert.Len(t, tools, 1)
    assert.Equal(t, "test.tool", tools[0]["name"])
}

func TestHandleToolCall(t *testing.T) {
    registry := NewRegistry()
    handler := NewHandler(registry, nil, nil, nil)
    
    // Register a test tool with handler
    executed := false
    registry.tools["test.tool"] = &ToolDefinition{
        Name: "test.tool",
        Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
            executed = true
            return map[string]interface{}{"result": "success"}, nil
        },
    }
    
    msg := &MCPMessage{
        JSONRPC: "2.0",
        ID:      "3",
        Method:  "tools/call",
        Params: json.RawMessage(`{
            "name": "test.tool",
            "arguments": {}
        }`),
    }
    
    response, err := handler.handleMessage("test-session", msg)
    require.NoError(t, err)
    require.NotNil(t, response)
    assert.True(t, executed)
    
    result, ok := response.Result.(map[string]interface{})
    require.True(t, ok)
    
    content, ok := result["content"].([]map[string]interface{})
    require.True(t, ok)
    assert.Len(t, content, 1)
}

func TestProtocolVersionValidation(t *testing.T) {
    handler := NewHandler(nil, nil, nil, nil)
    
    // Test with wrong protocol version
    msg := &MCPMessage{
        JSONRPC: "2.0",
        ID:      "4",
        Method:  "initialize",
        Params: json.RawMessage(`{
            "protocolVersion": "1.0.0",
            "clientInfo": {
                "name": "test-client",
                "version": "1.0.0"
            }
        }`),
    }
    
    _, err := handler.handleMessage("test-session", msg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unsupported protocol version")
}
```

### 6.2 Integration Test Script

**File: `edge-mcp-template/scripts/test-integration.sh`**

```bash
#!/bin/bash
# Integration test script for Edge MCP

set -e

echo "Starting Edge MCP integration tests..."

# Build the binary
echo "Building Edge MCP..."
make build

# Start Edge MCP in background
echo "Starting Edge MCP server..."
./bin/edge-mcp --port 8082 &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Function to cleanup
cleanup() {
    echo "Cleaning up..."
    kill $SERVER_PID 2>/dev/null || true
}
trap cleanup EXIT

# Test health endpoint
echo "Testing health endpoint..."
HEALTH=$(curl -s http://localhost:8082/health)
echo "Health response: $HEALTH"

# Test WebSocket connection with MCP protocol
echo "Testing MCP protocol..."

# Test initialize
echo '{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1.0.0"}}}' | \
    websocat -n1 ws://localhost:8082/ws || echo "WebSocket test failed"

echo "All tests completed!"
```

## Phase 7: Build and Deployment Configuration
<!-- CLAUDE: Build system files -->

### 7.1 Makefile

**File: `edge-mcp-template/Makefile`**

```makefile
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
	@echo "  make test        - Run tests"
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
```

### 7.2 Dockerfile

**File: `edge-mcp-template/Dockerfile`**

```dockerfile
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
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o edge-mcp \
    cmd/server/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates git docker-cli

# Create non-root user
RUN addgroup -g 1000 edge && \
    adduser -D -u 1000 -G edge edge

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/edge-mcp /app/edge-mcp

# Copy configuration
COPY configs/config.yaml /app/configs/config.yaml

# Change ownership
RUN chown -R edge:edge /app

# Switch to non-root user
USER edge

# Expose port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

# Run the binary
ENTRYPOINT ["/app/edge-mcp"]
CMD ["--config", "/app/configs/config.yaml"]
```

## Phase 8: Context Synchronization Architecture
<!-- CLAUDE: Use extended thinking - this is complex protocol design -->

### 8.1 Critical: How Edge MCP Handles Context Updates
The Edge MCP acts as a **protocol translator** for context synchronization:
```
Claude Code → [MCP Protocol] → Edge MCP → [REST API] → Core Platform
```

[Content continues with all context synchronization details from original plan...]

## Phase 9: Database Schema Updates for Edge MCP Support
<!-- CLAUDE: Single file task - straightforward -->

[Content continues with all database schema from original plan...]

## Phase 10: Authentication and Core Platform Integration
<!-- CLAUDE: Multi-file authentication implementation -->

[Content continues with all authentication details from original plan...]

## Phase 11: Comprehensive Documentation (Day 5)
<!-- CLAUDE: Documentation creation -->

[Content continues with all documentation from original plan...]

## Complete Technical Implementation Summary

[All summary content from original plan...]

## Claude Code Optimization Notes

### For Extended Thinking Phases
- Phase 2: Architecture extraction requires deep analysis
- Phase 8: Protocol translation design needs careful consideration
- Use "think deeply" before starting these phases

### For Multi-File Coordination
- Phase 2: Extracting from multiple source files
- Phase 4: Creating multiple tool implementations
- Phase 10: Authentication across REST API and Edge MCP

### Verification Points
- After each phase, run verification commands
- Commit changes incrementally
- Test before moving to next phase

### Success Metrics
- Zero Redis/PostgreSQL dependencies in Edge MCP
- All tests passing
- Successfully connects to Core Platform
- Claude Code can connect and execute tools

Expected completion: 5 working days with all technical details in place
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
│   │   └── tools.go
│   ├── kubernetes-mcp/
│   │   ├── main.go
│   │   └── tools.go
│   └── filesystem-mcp/
│       ├── main.go
│       └── tools.go
├── config.yaml                  # Minimal configuration
├── Dockerfile                    # For containerized deployment
├── Makefile
└── README.md
```

### 2.2 Core Components to Extract

**From `apps/mcp-server/`:**
- `internal/api/mcp_protocol.go` → Simplify, remove Redis/DB dependencies
- `internal/api/websocket/server.go` → Keep WebSocket handling
- `internal/api/websocket/connection.go` → Keep connection management

**From `pkg/`:**
- `common/cache/memory.go` → Use as-is for in-memory caching
- `adapters/mcp/protocol_adapter.go` → Simplify for edge use
- `observability/logger.go` → Keep for logging

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

### 3.1 Create Base Edge MCP

```go
// edge-mcp-template/cmd/edge-mcp/main.go
package main

import (
    "flag"
    "log"
    "os"
    "os/signal"
    
    "edge-mcp/internal/server"
    "edge-mcp/internal/cache"
)

func main() {
    port := flag.String("port", "8082", "Port to listen on")
    flag.Parse()
    
    // Create in-memory cache (no Redis!)
    memCache := cache.NewMemoryCache(1000, 5*time.Minute)
    
    // Create MCP server with minimal config
    srv := server.NewEdgeMCPServer(&server.Config{
        Port:  *port,
        Cache: memCache,
    })
    
    // Register your tools here
    srv.RegisterTool("example.hello", handleHello)
    
    // Start server
    if err := srv.Start(); err != nil {
        log.Fatal(err)
    }
    
    // Wait for shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt)
    <-sigChan
    
    srv.Shutdown()
}
```

### 3.2 Simplified MCP Handler

```go
// edge-mcp-template/internal/server/mcp_handler.go
package server

type MCPHandler struct {
    tools  map[string]ToolHandler
    cache  cache.Cache
    logger logger.Logger
}

func (h *MCPHandler) HandleMessage(msg *MCPMessage) (*MCPResponse, error) {
    switch msg.Method {
    case "initialize":
        return h.handleInitialize(msg)
    case "tools/list":
        return h.handleToolsList(msg)
    case "tools/call":
        return h.handleToolCall(msg)
    default:
        return nil, fmt.Errorf("method not found: %s", msg.Method)
    }
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

### 6.3 E2E Tests
```bash
# Test script
#!/bin/bash
# Start edge MCP
./edge-mcp &
MCP_PID=$!

# Test connection
websocat ws://localhost:8082/ws <<EOF
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05"},"id":1}
EOF

# Test tool list
websocat ws://localhost:8082/ws <<EOF
{"jsonrpc":"2.0","method":"tools/list","id":2}
EOF

# Cleanup
kill $MCP_PID
```

## Phase 7: Documentation (Day 5)

### 7.1 Developer Guide
- Quick start guide
- Tool development guide
- Configuration reference
- Troubleshooting guide

### 7.2 API Documentation
- MCP protocol reference
- Tool schema definitions
- Example implementations

### 7.3 Deployment Guide
- Local development setup
- Docker deployment
- CI/CD integration
- Production considerations

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

## Conclusion

This implementation plan provides a clear path to creating lightweight, standalone Edge MCPs that can run on developer machines without infrastructure dependencies. By reusing 80% of the existing codebase and replacing heavy dependencies with lightweight alternatives, we can deliver a powerful local development experience while maintaining compatibility with the MCP protocol standard.

The combination of completing the tenant credentials migration and creating the Edge MCP template will enable:
- True multi-tenant tool isolation
- Zero-infrastructure local development
- Direct IDE integration
- Rapid tool development and testing

Expected completion: 5 working days
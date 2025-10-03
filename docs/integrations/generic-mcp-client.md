# Generic MCP Client Integration Guide

This guide explains how to integrate any MCP-compatible client with Edge MCP using the standard Model Context Protocol (MCP).

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [MCP Protocol Basics](#mcp-protocol-basics)
4. [Connection Flow](#connection-flow)
5. [Implementation Examples](#implementation-examples)
6. [Protocol Reference](#protocol-reference)
7. [Best Practices](#best-practices)
8. [Troubleshooting](#troubleshooting)

## Overview

Edge MCP implements the **MCP 2025-06-18 specification**, providing a standards-compliant WebSocket interface for AI agents and automation tools. Any client that implements the MCP protocol can integrate with Edge MCP.

### Key Features

- **JSON-RPC 2.0**: Standard request-response protocol
- **WebSocket Transport**: Real-time bidirectional communication
- **Tool Execution**: Execute single or batch tools
- **Resource Access**: Query system resources and state
- **Response Streaming**: Automatic streaming for large payloads (>32KB)
- **Context Management**: Session-based context tracking
- **Error Handling**: Semantic errors with recovery guidance

## Prerequisites

### Required

- MCP-compatible client library or framework
- Edge MCP server running and accessible
- Valid API key for authentication
- WebSocket client capability (wss:// for production, ws:// for development)

### Optional

- Core Platform connection (for dynamic tool discovery)
- Redis (for distributed caching)
- OpenTelemetry collector (for distributed tracing)

## MCP Protocol Basics

### Transport Layer

Edge MCP uses WebSocket as the transport layer:

- **Development:** `ws://localhost:8082/ws`
- **Production:** `wss://edge-mcp.your-domain.com/ws`

### Message Format

All messages follow JSON-RPC 2.0 format:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "method_name",
  "params": {}
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {}
}
```

**Error:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid request",
    "data": {
      "recovery_steps": ["Step 1", "Step 2"],
      "retry_after": 5.0
    }
  }
}
```

### Authentication

All WebSocket connections require authentication via HTTP headers:

**Bearer Token (Recommended):**
```
Authorization: Bearer dev-admin-key-1234567890
```

**API Key Header:**
```
X-API-Key: dev-admin-key-1234567890
```

## Connection Flow

### 1. Establish WebSocket Connection

Connect to Edge MCP with authentication header:

```javascript
const ws = new WebSocket('ws://localhost:8082/ws', {
  headers: {
    'Authorization': 'Bearer dev-admin-key-1234567890'
  }
});
```

### 2. Send Initialize Message

Client must send `initialize` to start MCP session:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "clientInfo": {
      "name": "my-client",
      "version": "1.0.0"
    }
  }
}
```

### 3. Receive Capabilities

Server responds with its capabilities:

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
        "subscribe": true,
        "listChanged": true
      },
      "prompts": {
        "listChanged": false
      }
    }
  }
}
```

### 4. Send Initialized Confirmation

Client confirms initialization:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "initialized",
  "params": {}
}
```

### 5. Session Active

Connection is now ready for tool calls, resource queries, and other operations.

### 6. Keepalive

Server sends `ping` every 30 seconds. Client should respond with `pong` (or client's WebSocket library handles automatically).

## Implementation Examples

### Go Client

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
)

type MCPMessage struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id,omitempty"`
    Method  string          `json:"method,omitempty"`
    Params  json.RawMessage `json:"params,omitempty"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *MCPError       `json:"error,omitempty"`
}

type MCPError struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

func main() {
    // Connect with authentication
    header := http.Header{}
    header.Set("Authorization", "Bearer dev-admin-key-1234567890")

    ctx := context.Background()
    conn, _, err := websocket.Dial(ctx, "ws://localhost:8082/ws", &websocket.DialOptions{
        HTTPHeader: header,
    })
    if err != nil {
        panic(err)
    }
    defer conn.Close(websocket.StatusNormalClosure, "")

    // Initialize
    initMsg := MCPMessage{
        JSONRPC: "2.0",
        ID:      1,
        Method:  "initialize",
        Params: json.RawMessage(`{
            "protocolVersion": "2025-06-18",
            "clientInfo": {"name": "go-client", "version": "1.0.0"}
        }`),
    }
    wsjson.Write(ctx, conn, initMsg)

    var response MCPMessage
    wsjson.Read(ctx, conn, &response)

    // Send initialized
    confirmMsg := MCPMessage{
        JSONRPC: "2.0",
        ID:      2,
        Method:  "initialized",
        Params:  json.RawMessage(`{}`),
    }
    wsjson.Write(ctx, conn, confirmMsg)

    // List tools
    listToolsMsg := MCPMessage{
        JSONRPC: "2.0",
        ID:      3,
        Method:  "tools/list",
        Params:  json.RawMessage(`{}`),
    }
    wsjson.Write(ctx, conn, listToolsMsg)

    wsjson.Read(ctx, conn, &response)
    fmt.Printf("Tools: %s\n", response.Result)

    // Call a tool
    callToolMsg := MCPMessage{
        JSONRPC: "2.0",
        ID:      4,
        Method:  "tools/call",
        Params: json.RawMessage(`{
            "name": "github_get_repository",
            "arguments": {
                "owner": "developer-mesh",
                "repo": "developer-mesh"
            }
        }`),
    }
    wsjson.Write(ctx, conn, callToolMsg)

    wsjson.Read(ctx, conn, &response)
    fmt.Printf("Result: %s\n", response.Result)
}
```

### Python Client

```python
import asyncio
import json
import websockets

async def edge_mcp_client():
    # Connect with authentication
    uri = "ws://localhost:8082/ws"
    headers = {
        "Authorization": "Bearer dev-admin-key-1234567890"
    }

    async with websockets.connect(uri, extra_headers=headers) as websocket:
        # Initialize
        init_msg = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2025-06-18",
                "clientInfo": {
                    "name": "python-client",
                    "version": "1.0.0"
                }
            }
        }
        await websocket.send(json.dumps(init_msg))
        response = json.loads(await websocket.recv())
        print("Initialized:", response)

        # Send initialized confirmation
        confirmed_msg = {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "initialized",
            "params": {}
        }
        await websocket.send(json.dumps(confirmed_msg))

        # List tools
        list_tools_msg = {
            "jsonrpc": "2.0",
            "id": 3,
            "method": "tools/list",
            "params": {}
        }
        await websocket.send(json.dumps(list_tools_msg))
        tools_response = json.loads(await websocket.recv())
        print("Tools:", tools_response["result"])

        # Call a tool
        call_tool_msg = {
            "jsonrpc": "2.0",
            "id": 4,
            "method": "tools/call",
            "params": {
                "name": "github_list_issues",
                "arguments": {
                    "owner": "developer-mesh",
                    "repo": "developer-mesh",
                    "state": "open"
                }
            }
        }
        await websocket.send(json.dumps(call_tool_msg))
        result = json.loads(await websocket.recv())
        print("Tool result:", result)

asyncio.run(edge_mcp_client())
```

### TypeScript/Node.js Client

```typescript
import WebSocket from 'ws';

interface MCPMessage {
  jsonrpc: string;
  id?: number;
  method?: string;
  params?: any;
  result?: any;
  error?: {
    code: number;
    message: string;
    data?: any;
  };
}

async function connectEdgeMCP() {
  const ws = new WebSocket('ws://localhost:8082/ws', {
    headers: {
      'Authorization': 'Bearer dev-admin-key-1234567890'
    }
  });

  ws.on('open', () => {
    // Initialize
    const initMsg: MCPMessage = {
      jsonrpc: '2.0',
      id: 1,
      method: 'initialize',
      params: {
        protocolVersion: '2025-06-18',
        clientInfo: {
          name: 'typescript-client',
          version: '1.0.0'
        }
      }
    };
    ws.send(JSON.stringify(initMsg));
  });

  ws.on('message', (data: string) => {
    const msg: MCPMessage = JSON.parse(data);
    console.log('Received:', msg);

    if (msg.id === 1 && msg.result) {
      // Send initialized confirmation
      const confirmedMsg: MCPMessage = {
        jsonrpc: '2.0',
        id: 2,
        method: 'initialized',
        params: {}
      };
      ws.send(JSON.stringify(confirmedMsg));

      // List tools
      const listToolsMsg: MCPMessage = {
        jsonrpc: '2.0',
        id: 3,
        method: 'tools/list',
        params: {}
      };
      ws.send(JSON.stringify(listToolsMsg));
    }

    if (msg.id === 3 && msg.result) {
      // Call a tool
      const callToolMsg: MCPMessage = {
        jsonrpc: '2.0',
        id: 4,
        method: 'tools/call',
        params: {
          name: 'github_get_repository',
          arguments: {
            owner: 'developer-mesh',
            repo: 'developer-mesh'
          }
        }
      };
      ws.send(JSON.stringify(callToolMsg));
    }
  });

  ws.on('error', (error) => {
    console.error('WebSocket error:', error);
  });
}

connectEdgeMCP();
```

## Protocol Reference

### Core Methods

#### initialize

Starts MCP session and negotiates protocol version.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "clientInfo": {
      "name": "client-name",
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
      "tools": {"listChanged": true},
      "resources": {"subscribe": true, "listChanged": true},
      "prompts": {"listChanged": false}
    }
  }
}
```

#### initialized

Confirms initialization complete.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "initialized",
  "params": {}
}
```

#### tools/list

Lists all available tools.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/list",
  "params": {}
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
        "name": "github_get_repository",
        "description": "Get GitHub repository information",
        "category": "repository",
        "tags": ["read", "github"],
        "inputSchema": {
          "type": "object",
          "properties": {
            "owner": {"type": "string"},
            "repo": {"type": "string"}
          },
          "required": ["owner", "repo"]
        }
      }
    ]
  }
}
```

#### tools/call

Executes a single tool.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "github_get_repository",
    "arguments": {
      "owner": "developer-mesh",
      "repo": "developer-mesh"
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
        "text": "{\"name\":\"developer-mesh\",\"full_name\":\"developer-mesh/developer-mesh\",\"description\":\"DevOps MCP Platform\",\"stars\":42,\"forks\":7,...}"
      }
    ]
  }
}
```

#### tools/batch

Executes multiple tools in parallel or sequentially.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/batch",
  "params": {
    "tools": [
      {
        "id": "call-1",
        "name": "github_get_repository",
        "arguments": {"owner": "developer-mesh", "repo": "developer-mesh"}
      },
      {
        "id": "call-2",
        "name": "github_list_issues",
        "arguments": {"owner": "developer-mesh", "repo": "developer-mesh", "state": "open"}
      }
    ],
    "parallel": true
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "results": [
      {"id": "call-1", "status": "success", "result": {...}, "duration_ms": 145.2},
      {"id": "call-2", "status": "success", "result": {...}, "duration_ms": 167.8}
    ],
    "duration_ms": 180.5,
    "success_count": 2,
    "error_count": 0,
    "parallel": true
  }
}
```

#### resources/list

Lists available resources.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "resources/list",
  "params": {}
}
```

#### resources/read

Reads a specific resource.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "resources/read",
  "params": {
    "uri": "devmesh://agents/tenant-123"
  }
}
```

#### context.update

Updates session context.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "context.update",
  "params": {
    "context": {
      "repository": "developer-mesh/developer-mesh",
      "environment": "production"
    },
    "merge": true
  }
}
```

#### context.get

Retrieves current session context.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 9,
  "method": "context.get",
  "params": {}
}
```

### Notification Methods

#### ping

Server sends ping for keepalive (every 30 seconds).

```json
{
  "jsonrpc": "2.0",
  "method": "ping",
  "params": {}
}
```

Client should respond with pong (or WebSocket library handles automatically).

#### $/progress

Progress notification during long-running operations.

```json
{
  "jsonrpc": "2.0",
  "method": "$/progress",
  "params": {
    "token": "operation-123",
    "progress": 45.5,
    "message": "Processing 45 of 100 items"
  }
}
```

#### $/logMessage

Log streaming during tool execution.

```json
{
  "jsonrpc": "2.0",
  "method": "$/logMessage",
  "params": {
    "level": "info",
    "message": "Connecting to GitHub API",
    "timestamp": "2025-01-15T10:30:45Z"
  }
}
```

### Error Codes

Edge MCP uses standard JSON-RPC error codes plus MCP-specific codes:

| Code | Description |
|------|-------------|
| -32700 | Parse error |
| -32600 | Invalid request |
| -32601 | Method not found |
| -32602 | Invalid params |
| -32603 | Internal error |
| -32000 | Server error |
| 400 | Bad request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not found |
| 429 | Rate limit exceeded |
| 500 | Internal server error |
| 503 | Service unavailable |

## Best Practices

### 1. Connection Management

- **Implement Reconnection Logic**: WebSocket connections can drop. Implement exponential backoff reconnection.
- **Handle Keepalive**: Respond to ping messages or let WebSocket library handle automatically.
- **Graceful Shutdown**: Send `shutdown` method before closing connection.

### 2. Error Handling

- **Parse Error Responses**: Extract `error.data` for recovery steps and retry_after information.
- **Implement Retries**: Use exponential backoff for retryable errors (429, 503).
- **Log Errors**: Include request ID and session ID in logs for debugging.

### 3. Performance Optimization

- **Use Batch Operations**: Combine independent tool calls into `tools/batch` requests.
- **Cache Results**: Cache responses locally when appropriate.
- **Parallel Execution**: Set `parallel: true` in batch requests for independent operations.

### 4. Security

- **Secure WebSocket**: Use `wss://` (not `ws://`) in production.
- **Protect API Keys**: Never hardcode API keys. Use environment variables or secrets management.
- **Validate Inputs**: Validate tool arguments before sending to Edge MCP.

### 5. Resource Management

- **Close Connections**: Always close WebSocket connections when done.
- **Limit Concurrent Requests**: Don't overwhelm server with too many concurrent requests.
- **Monitor Rate Limits**: Check rate limit headers and implement backoff.

### 6. Observability

- **Request IDs**: Generate unique request IDs for tracing.
- **Structured Logging**: Log all requests, responses, and errors with context.
- **Metrics**: Track success/failure rates, latency, and throughput.

## Troubleshooting

### Connection Refused

**Problem:** WebSocket connection fails immediately

**Solutions:**
1. Verify Edge MCP is running:
   ```bash
   curl http://localhost:8082/health/ready
   ```
2. Check firewall rules
3. Verify WebSocket URL format (`ws://` or `wss://`)

### Authentication Failed

**Problem:** 401 Unauthorized error

**Solutions:**
1. Verify API key is in header:
   ```
   Authorization: Bearer your-api-key
   ```
2. Check API key format (alphanumeric + hyphen/underscore only)
3. Verify API key is configured in Edge MCP

### Protocol Version Mismatch

**Problem:** Initialization fails with version error

**Solutions:**
1. Use supported protocol version: `2025-06-18`
2. Check Edge MCP version compatibility

### Tool Not Found

**Problem:** Tool call returns 404 error

**Solutions:**
1. List available tools with `tools/list`
2. Verify tool name is correct (case-sensitive)
3. Check if Core Platform is connected (for dynamic tools)

### Rate Limited

**Problem:** 429 Too Many Requests error

**Solutions:**
1. Implement exponential backoff using `retry_after` value in error response
2. Reduce request rate
3. Use batch operations to reduce total requests

## Related Documentation

- [Claude Code Integration](./claude-code.md)
- [Cursor Integration](./cursor.md)
- [Windsurf Integration](./windsurf.md)
- [Troubleshooting Guide](./troubleshooting.md)
- [OpenAPI Specification](../openapi/edge-mcp.yaml)
- [Example Clients](../openapi/examples/)

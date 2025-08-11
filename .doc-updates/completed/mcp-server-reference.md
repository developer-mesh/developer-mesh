<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:29:14
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# MCP Server API Reference

This document provides comprehensive reference documentation for the MCP (Model Context Protocol) Server API. The MCP Server implements the Model Context Protocol specification and provides additional functionality for DevOps tool integration.

## Table of Contents

- [Overview](#overview)
- [Base URL and Versioning](#base-url-and-versioning)
- [Authentication](#authentication)
- [Rate Limiting](#rate-limiting)
- [Error Handling](#error-handling)
- [API Endpoints](#api-endpoints)
  - [Health & Monitoring](#health--monitoring)
  - [MCP Context Management](#mcp-context-management)
  - [Tool Integration](#tool-integration)
  - [Agent Management](#agent-management)
  - [Model Management](#model-management)
  - [Embedding Operations](#embedding-operations)
  - [GitHub Tools (MCP Protocol)](#github-tools-mcp-protocol)
  - [Webhooks](#webhooks)
  - [Relationship Management](#relationship-management)
- [WebSocket Support](#websocket-support) <!-- Source: pkg/models/websocket/binary.go -->
- [SDK Support](#sdk-support)

## Overview

The MCP Server API implements the Model Context Protocol specification, providing a standardized interface for AI agents to interact with DevOps tools. It acts as a bridge between AI models and various DevOps platforms while maintaining conversation context and enabling tool execution.

### Key Features

- **MCP Protocol Compliance**: Full implementation of the Model Context Protocol specification
- **Context Management**: Advanced context storage and retrieval with conversation history
- **Tool Integration**: Unified interface for GitHub, Harness, SonarQube, Artifactory, and Xray
- **Multi-Agent Embeddings**: Agent-specific embedding generation with intelligent provider routing
- **Real-time Updates**: WebSocket support for streaming responses <!-- Source: pkg/models/websocket/binary.go -->
- **Multi-tenancy**: Built-in tenant isolation for enterprise deployments
- **Binary Protocol**: High-performance WebSocket communication with automatic compression <!-- Source: pkg/models/websocket/binary.go -->
- **AI Agent Orchestration**: Support for multiple concurrent AI agents with task routing <!-- Source: pkg/services/assignment_engine.go -->

## Base URL and Versioning

```
Base URL: https://api.mcp-server.example.com
Current Version: v1
Full Base Path: https://api.mcp-server.example.com/api/v1
```

### Version Negotiation

The API supports version negotiation through:
- URL path: `/api/v1/...`
- Accept header: `Accept: application/vnd.mcp.v1+json`
- Custom header: `X-API-Version: 1`

## Authentication

The MCP Server supports two authentication methods:

### API Key Authentication

```http
Authorization: Bearer your-api-key-here
# or
Authorization: your-api-key-here
```

### JWT Authentication

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Example Request

```bash
curl -H "Authorization: Bearer your-api-key" \
     https://api.mcp-server.example.com/api/v1/tools
```

## Rate Limiting

Rate limits are applied per user/API key:

- **Default**: 1000 requests per hour
- **Burst**: 50 requests per minute
- **Tool Execution**: 100 requests per hour

Rate limit headers:
```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1703001600
```

## Error Handling

All error responses follow a consistent format:

```json
{
  "error": "Error message",
  "code": "ERROR_CODE",
  "details": {
    "field": "Additional context"
  }
}
```

Common HTTP status codes:
- `200 OK`: Successful request
- `201 Created`: Resource created
- `400 Bad Request`: Invalid request parameters
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error

## API Endpoints

### Health & Monitoring

#### Health Check

Check the health status of all MCP server components.

```http
GET /health
```

**Response**
```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "cache": "healthy",
    "context_manager": "healthy",
    "rest_api_client": "healthy",
    "vector_database": "healthy"
  }
}
```

#### API Info

Get API version and available endpoints.

```http
GET /api/v1/
```

**Response**
```json
{
  "version": "v1",
  "status": "operational",
  "apis": ["agent", "model", "vector", "mcp", "tools"]
}
```

### MCP Context Management

The MCP context endpoints implement the Model Context Protocol for managing conversation contexts.

#### Create Context

Create a new conversation context.

```http
POST /api/v1/mcp/context
```

**Request Body**
```json
{
  "agent_id": "agent-123",
  "session_id": "session-456",
  "content": [
    {
      "role": "user",
      "content": "Hello, I need help with my CI/CD pipeline"
    }
  ],
  "metadata": {
    "source": "web-ui",
    "tool_context": ["github", "harness"]
  }
}
```

**Response**
```json
{
  "message": "context created",
  "id": "ctx-789",
  "context": {
    "id": "ctx-789",
    "agent_id": "agent-123",
    "session_id": "session-456",
    "content": [...],
    "created_at": "2024-01-15T10:00:00Z",
    "updated_at": "2024-01-15T10:00:00Z"
  }
}
```

#### Get Context

Retrieve a specific context by ID.

```http
GET /api/v1/mcp/context/:id
```

**Response**
```json
{
  "id": "ctx-789",
  "agent_id": "agent-123",
  "session_id": "session-456",
  "content": [
    {
      "role": "user",
      "content": "Hello, I need help with my CI/CD pipeline"
    },
    {
      "role": "assistant",
      "content": "I'd be happy to help with your CI/CD pipeline. What specific aspect would you like assistance with?"
    }
  ],
  "metadata": {
    "source": "web-ui",
    "tool_context": ["github", "harness"]
  },
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T10:05:00Z"
}
```

#### Update Context

Update an existing context with new messages.

```http
PUT /api/v1/mcp/context/:id
```

**Request Body**
```json
{
  "content": [
    {
      "role": "assistant",
      "content": "I've checked your pipeline configuration..."
    }
  ],
  "options": {
    "replace_content": false,
    "truncate_to_size": 100000
  }
}
```

**Response**
```json
{
  "id": "ctx-789",
  "agent_id": "agent-123",
  "session_id": "session-456",
  "content": [...],
  "updated_at": "2024-01-15T10:10:00Z"
}
```

#### Delete Context

Remove a context and all associated data.

```http
DELETE /api/v1/mcp/context/:id
```

**Response**
```json
{
  "status": "deleted"
}
```

#### List Contexts

List contexts filtered by agent or session.

```http
GET /api/v1/mcp/contexts?agent_id=agent-123&session_id=session-456
```

**Response**
```json
{
  "contexts": [
    {
      "id": "ctx-789",
      "agent_id": "agent-123",
      "session_id": "session-456",
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:10:00Z",
      "message_count": 5
    }
  ]
}
```

#### Search Within Context

Search for specific content within a context.

```http
POST /api/v1/mcp/context/:id/search
```

**Request Body**
```json
{
  "query": "pipeline configuration"
}
```

**Response**
```json
{
  "results": [
    {
      "role": "user",
      "content": "Can you check my pipeline configuration?",
      "timestamp": "2024-01-15T10:02:00Z"
    },
    {
      "role": "assistant", 
      "content": "I've checked your pipeline configuration and found...",
      "timestamp": "2024-01-15T10:03:00Z"
    }
  ]
}
```

#### Summarize Context

Generate an AI-powered summary of the context.

```http
GET /api/v1/mcp/context/:id/summary
```

**Response**
```json
{
  "summary": "The conversation focused on troubleshooting a CI/CD pipeline issue. The user needed help with a failed deployment in their Harness pipeline. Key points discussed: 1) Pipeline was failing at the approval stage, 2) Missing environment variables were identified as the root cause, 3) Solution provided to add the required variables in the pipeline configuration."
}
```

### Tool Integration

#### List Available Tools

Get all available DevOps tools.

```http
GET /api/v1/tools
```

**Response**
```json
{
  "tools": [
    {
      "name": "github",
      "description": "GitHub integration for repository, pull request, and code management",
      "actions": [
        "create_issue",
        "close_issue",
        "create_pull_request",
        "merge_pull_request",
        "add_comment",
        "archive_repository"
      ],
      "safety_notes": "Cannot delete repositories for safety reasons"
    },
    {
      "name": "harness",
      "description": "Harness CI/CD integration for builds and deployments",
      "actions": [
        "trigger_pipeline",
        "get_pipeline_status",
        "stop_pipeline",
        "rollback_deployment"
      ],
      "safety_notes": "Cannot delete production feature flags for safety reasons"
    }
  ],
  "_links": {
    "self": "https://api.mcp-server.example.com/api/v1/tools"
  }
}
```

#### Get Tool Details

Get detailed information about a specific tool.

```http
GET /api/v1/tools/:tool
```

**Response**
```json
{
  "name": "github",
  "description": "GitHub integration for repository, pull request, and code management",
  "actions": [
    "create_issue",
    "close_issue",
    "create_pull_request",
    "merge_pull_request",
    "add_comment",
    "archive_repository"
  ],
  "safety_notes": "Cannot delete repositories for safety reasons",
  "_links": {
    "self": "https://api.mcp-server.example.com/api/v1/tools/github",
    "actions": "https://api.mcp-server.example.com/api/v1/tools/github/actions",
    "queries": "https://api.mcp-server.example.com/api/v1/tools/github/queries"
  }
}
```

#### List Tool Actions

Get available actions for a tool.

```http
GET /api/v1/tools/:tool/actions
```

**Response**
```json
{
  "tool": "github",
  "allowed_actions": [
    "create_issue",
    "close_issue",
    "create_pull_request",
    "merge_pull_request",
    "add_comment",
    "get_repository",
    "list_repositories",
    "get_pull_request",
    "list_pull_requests",
    "get_issue",
    "list_issues",
    "archive_repository"
  ],
  "disallowed_actions": [
    "delete_repository",
    "delete_branch",
    "delete_organization"
  ],
  "safety_notes": "Repository deletion is restricted for safety reasons, but archiving is allowed."
}
```

#### Get Action Details

Get detailed information about a specific action.

```http
GET /api/v1/tools/:tool/actions/:action
```

**Response**
```json
{
  "name": "create_issue",
  "description": "Creates a new issue in a GitHub repository",
  "parameters": {
    "owner": "Repository owner (organization or user)",
    "repo": "Repository name",
    "title": "Issue title",
    "body": "Issue description",
    "labels": "Array of label names",
    "assignees": "Array of usernames to assign"
  },
  "required_parameters": ["owner", "repo", "title"],
  "example": {
    "owner": "octocat",
    "repo": "hello-world",
    "title": "Bug in login form",
    "body": "The login form doesn't submit when using Safari",
    "labels": ["bug", "frontend"]
  },
  "_links": {
    "self": "https://api.mcp-server.example.com/api/v1/tools/github/actions/create_issue",
    "tool": "https://api.mcp-server.example.com/api/v1/tools/github",
    "actions": "https://api.mcp-server.example.com/api/v1/tools/github/actions"
  }
}
```

#### Execute Tool Action

Execute an action on a tool.

```http
POST /api/v1/tools/:tool/actions/:action?context_id=ctx-789
```

**Request Body**
```json
{
  "owner": "octocat",
  "repo": "hello-world",
  "title": "Bug in login form",
  "body": "The login form doesn't submit when using Safari",
  "labels": ["bug", "frontend"]
}
```

**Response**
```json
{
  "status": "success",
  "message": "Executed create_issue action on github tool",
  "tool": "github",
  "action": "create_issue",
  "result": {
    "issue_number": 42,
    "html_url": "https://github.com/octocat/hello-world/issues/42"
  },
  "_links": {
    "self": "https://api.mcp-server.example.com/api/v1/tools/github/actions/create_issue",
    "tool": "https://api.mcp-server.example.com/api/v1/tools/github"
  }
}
```

#### Query Tool Data

Query data from a tool.

```http
POST /api/v1/tools/:tool/queries?context_id=ctx-789
```

**Request Body**
```json
{
  "query_type": "list_repositories",
  "filters": {
    "organization": "octocat",
    "visibility": "public",
    "sort": "updated"
  }
}
```

**Response**
```json
{
  "status": "success",
  "message": "Queried data from github tool",
  "tool": "github",
  "query_params": {...},
  "data": [
    {
      "id": "1",
      "name": "hello-world",
      "full_name": "octocat/hello-world",
      "private": false,
      "updated_at": "2024-01-15T09:00:00Z"
    }
  ]
}
```

### Agent Management

#### Create Agent

Create a new AI agent configuration.

```http
POST /api/v1/agents
```

**Request Body**
```json
{
  "name": "DevOps Assistant",
  "description": "AI agent for DevOps automation",
  "model": "gpt-4",
  "capabilities": ["github", "harness", "sonarqube"],
  "configuration": {
    "temperature": 0.7,
    "max_tokens": 2000
  }
}
```

**Response**
```json
{
  "id": "agent-123",
  "name": "DevOps Assistant",
  "description": "AI agent for DevOps automation",
  "model": "gpt-4",
  "capabilities": ["github", "harness", "sonarqube"],
  "configuration": {...},
  "created_at": "2024-01-15T10:00:00Z"
}
```

#### List Agents

List all agents for the current tenant.

```http
GET /api/v1/agents
```

**Response**
```json
{
  "agents": [
    {
      "id": "agent-123",
      "name": "DevOps Assistant",
      "description": "AI agent for DevOps automation",
      "model": "gpt-4",
      "capabilities": ["github", "harness", "sonarqube"],
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

#### Update Agent

Update an agent configuration.

```http
PUT /api/v1/agents/:id
```

**Request Body**
```json
{
  "name": "DevOps Assistant Pro",
  "configuration": {
    "temperature": 0.8,
    "max_tokens": 3000
  }
}
```

**Response**
```json
{
  "id": "agent-123",
  "name": "DevOps Assistant Pro",
  "description": "AI agent for DevOps automation",
  "model": "gpt-4",
  "capabilities": ["github", "harness", "sonarqube"],
  "configuration": {...},
  "updated_at": "2024-01-15T11:00:00Z"
}
```

### Model Management

#### Create Model

Register a new AI model.

```http
POST /api/v1/models
```

**Request Body**
```json
{
  "name": "gpt-4-turbo",
  "provider": "openai",
  "type": "chat",
  "configuration": {
    "api_endpoint": "https://api.openai.com/v1/chat/completions",
    "max_context_length": 128000,
    "supports_functions": true
  }
}
```

**Response**
```json
{
  "id": "model-456",
  "name": "gpt-4-turbo",
  "provider": "openai",
  "type": "chat",
  "configuration": {...},
  "created_at": "2024-01-15T10:00:00Z"
}
```

#### List Models

List all available models.

```http
GET /api/v1/models
```

**Response**
```json
{
  "models": [
    {
      "id": "model-456",
      "name": "gpt-4-turbo",
      "provider": "openai",
      "type": "chat",
      "created_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

#### Update Model

Update model configuration.

```http
PUT /api/v1/models/:id
```

**Request Body**
```json
{
  "configuration": {
    "max_context_length": 200000,
    "supports_vision": true
  }
}
```

**Response**
```json
{
  "id": "model-456",
  "name": "gpt-4-turbo",
  "provider": "openai",
  "type": "chat",
  "configuration": {...},
  "updated_at": "2024-01-15T11:00:00Z"
}
```

## Embedding Operations

The MCP Server provides embedding operations through the multi-agent embedding system. All requests are routed to the REST API's v2 embedding endpoints.

### Generate Embedding

**Request:**
```json
{
  "action": "generate_embedding",
  "agent_id": "claude-assistant",
  "text": "Content to embed",
  "context_id": "ctx_123"
}
```

**Response:**
```json
{
  "embedding_id": "550e8400-e29b-41d4-a716-446655440000",
  "model_used": "text-embedding-3-large",
  "provider": "openai",
  "dimensions": 3072,
  "cached": false,
  "cost_usd": 0.00013
}
```

### Search

**Request:**
```json
{
  "action": "search",
  "agent_id": "claude-assistant",
  "query": "kubernetes deployment",
  "limit": 10
}
```

### Cross-Model Search

**Request:**
```json
{
  "action": "cross_model_search",
  "query": "deployment strategies",
  "include_models": ["text-embedding-3-small", "voyage-2"],
  "limit": 20
}
```

### Provider Health Check

**Request:**
```json
{
  "action": "provider_health"
}
```

All embedding operations require an `agent_id` to determine which models and strategies to use.

### GitHub Tools (MCP Protocol)

These endpoints implement MCP protocol-compliant tools for GitHub operations.

#### List GitHub Tools

Get available GitHub tools following MCP protocol.

```http
GET /api/v1/tools/github
```

**Response**
```json
{
  "tools": [
    {
      "name": "github_create_issue",
      "description": "Create a new issue in a GitHub repository",
      "inputSchema": {
        "type": "object",
        "properties": {
          "owner": {"type": "string"},
          "repo": {"type": "string"},
          "title": {"type": "string"},
          "body": {"type": "string"}
        },
        "required": ["owner", "repo", "title"]
      }
    }
  ]
}
```

#### Get Tool Schema

Get the MCP schema for a specific GitHub tool.

```http
GET /api/v1/tools/github/:tool_name
```

**Response**
```json
{
  "name": "github_create_issue",
  "description": "Create a new issue in a GitHub repository",
  "inputSchema": {
    "type": "object",
    "properties": {
      "owner": {
        "type": "string",
        "description": "Repository owner"
      },
      "repo": {
        "type": "string",
        "description": "Repository name"
      },
      "title": {
        "type": "string",
        "description": "Issue title"
      },
      "body": {
        "type": "string",
        "description": "Issue body"
      }
    },
    "required": ["owner", "repo", "title"]
  }
}
```

#### Execute GitHub Tool

Execute a GitHub tool following MCP protocol.

```http
POST /api/v1/tools/github/:tool_name
```

**Request Body**
```json
{
  "arguments": {
    "owner": "octocat",
    "repo": "hello-world",
    "title": "Bug report",
    "body": "Description of the bug"
  }
}
```

**Response**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Successfully created issue #42"
    }
  ],
  "isError": false
}
```

### Webhooks

#### GitHub Webhook

Receive GitHub webhook events.

```http
POST /api/webhooks/github
```

**Headers**
```http
X-GitHub-Event: push
X-Hub-Signature-256: sha256=...
```

**Request Body**
```json
{
  "ref": "refs/heads/main",
  "repository": {
    "name": "hello-world",
    "full_name": "octocat/hello-world"
  },
  "pusher": {
    "name": "octocat"
  }
}
```

**Response**
```json
{
  "status": "accepted",
  "message": "Webhook processed successfully"
}
```

### Relationship Management

#### Create Relationship

Create a relationship between entities (contexts, tools, etc.).

```http
POST /api/v1/relationships
```

**Request Body**
```json
{
  "source_type": "context",
  "source_id": "ctx-789",
  "target_type": "tool_execution",
  "target_id": "exec-123",
  "relationship_type": "triggered_by"
}
```

**Response**
```json
{
  "id": "rel-456",
  "source_type": "context",
  "source_id": "ctx-789",
  "target_type": "tool_execution",
  "target_id": "exec-123",
  "relationship_type": "triggered_by",
  "created_at": "2024-01-15T10:00:00Z"
}
```

## WebSocket Support <!-- Source: pkg/models/websocket/binary.go -->

The MCP Server provides WebSocket connections for real-time AI agent communication with an optimized binary protocol for high-performance message exchange. <!-- Source: pkg/models/websocket/binary.go -->

### Connection Establishment

```javascript
const ws = new WebSocket('wss://api.mcp-server.example.com/ws'); <!-- Source: pkg/models/websocket/binary.go -->

ws.on('open', () => {
  // Send MCP initialization
  ws.send(JSON.stringify({
    jsonrpc: "2.0",
    method: "initialize",
    params: {
      protocolVersion: "0.1.0",
      capabilities: {
        tools: {},
        prompts: {},
        resources: {}
      },
      clientInfo: {
        name: "devops-agent",
        version: "1.0.0"
      }
    },
    id: 1
  }));
});
```

### Binary Protocol (High Performance) <!-- Source: pkg/models/websocket/binary.go -->

For messages larger than 1KB, the MCP Server automatically uses a binary protocol for improved performance: <!-- Source: pkg/models/websocket/binary.go -->

#### Binary Message Format

```
┌─────────────┬─────────┬──────────┬─────────────┬──────────┬──────────────┬──────────────┬────────────┐
│ Magic (4B)  │ Ver (1B)│ Type(1B) │ Method (2B) │ Flags(1B)│ Reserved(3B) │ Payload(4B)  │ ReqID (8B) │
├─────────────┼─────────┼──────────┼─────────────┼──────────┼──────────────┼──────────────┼────────────┤
│ "MCPW"      │ 0x01    │ 0x01-0x07│ 0x0000-FFFF │ 0bXXXXXXXX│ 0x000000     │ Size (uint32)│ ID (uint64)│
└─────────────┴─────────┴──────────┴─────────────┴──────────┴──────────────┴──────────────┴────────────┘
```

**Header Fields (24 bytes):**
- **Magic** (4 bytes): "MCPW" identifier (0x4D435057)
- **Version** (1 byte): Protocol version (currently 0x01)
- **Type** (1 byte): Message type
  - 0x01: Request
  - 0x02: Response
  - 0x03: Notification
  - 0x04: Error
  - 0x05: Ping
  - 0x06: Pong
  - 0x07: Close
- **Method** (2 bytes): Method enum for fast routing
- **Flags** (1 byte): 
  - Bit 0: Compression enabled (gzip)
  - Bit 1: Encryption enabled
  - Bit 2-7: Reserved
- **Reserved** (3 bytes): Future use
- **PayloadLen** (4 bytes): Payload size (max ~4GB)
- **RequestID** (8 bytes): Request identifier for correlation

#### Method Enums (Implemented Operations)

```go
const (
    MethodInitialize      uint16 = 0x0001
    MethodToolList        uint16 = 0x0002  // List tools
    MethodToolExecute     uint16 = 0x0003  // Execute tool
    MethodContextGet      uint16 = 0x0004  // Get context
    MethodContextSet      uint16 = 0x0005  // Set context
    MethodLogMessage      uint16 = 0x0006  // Log message
    // Additional methods may be added in future versions
)
```

### Performance Features

1. **Compression Support**: Messages can be gzip compressed (configurable threshold)
2. **Binary Encoding**: ~70% smaller than JSON for typical messages
3. **Connection Pooling**: Reusable connections per agent
4. **Message Batching**: Multiple operations in single message
5. **Zero-Copy Parsing**: Direct memory access for performance

### WebSocket Message Flow <!-- Source: pkg/models/websocket/binary.go -->

```javascript
// Client sends binary message for large payloads
if (payload.length > 1024) {
  const binaryMsg = encodeDBinaryMessage({
    type: MessageType.Request,
    method: MethodCallTool,
    payload: payload,
    requestId: generateRequestId()
  });
  ws.send(binaryMsg);
} else {
  // Small messages use JSON
  ws.send(JSON.stringify(message));
}

// Server automatically handles both formats
ws.on('message', (data) => {
  if (data[0] === 0x4D && data[1] === 0x43 && data[2] === 0x50 && data[3] === 0x57) { // "MCPW" magic
    const msg = decodeBinaryMessage(data);
    handleBinaryMessage(msg);
  } else {
    const msg = JSON.parse(data);
    handleJSONMessage(msg);
  }
});
```

### Real-time Features

#### Agent Registration
```json
{
  "method": "agent.register",
  "params": {
    "name": "code-analyzer",
    "capabilities": ["code_analysis", "security_scan"],
    "model": "gpt-4"
  }
}
```

#### Task Assignment Notification
```json
{
  "method": "notification",
  "params": {
    "type": "task.assigned",
    "task": {
      "id": "task-123",
      "type": "code_review",
      "priority": "high"
    }
  }
}
```

#### Collaborative Workspace Updates
```json
{
  "method": "workspace.update",
  "params": {
    "type": "document.changed",
    "document_id": "doc-456",
    "changes": [...] // CRDT operations
  }
}
```

### Connection Management

```javascript
// Heartbeat for connection health
setInterval(() => {
  if (ws.readyState === WebSocket.OPEN) { <!-- Source: pkg/models/websocket/binary.go -->
    ws.send(new Uint8Array([
      0x4D, 0x43, 0x50, 0x57, // Magic "MCPW"
      0x01,                   // Version
      0x05,                   // Type: Ping
      0x00, 0x00,            // Method (unused)
      0x00,                   // Flags
      0x00, 0x00, 0x00,      // Reserved
      0x00, 0x00, 0x00, 0x00, // No payload
      ...BigInt(Date.now()).toString(16).padStart(16, '0').match(/.{2}/g).map(b => parseInt(b, 16))
    ]));
  }
}, 30000);

// Handle connection errors
ws.on('error', (error) => {
  console.error('WebSocket error:', error); <!-- Source: pkg/models/websocket/binary.go -->
  // Implement exponential backoff reconnection
});
```

### WebSocket Message Types <!-- Source: pkg/models/websocket/binary.go -->

#### MCP Protocol Messages
- `initialize`: Initialize MCP session
- `initialized`: Confirmation of initialization
- `tools/list`: List available tools
- `tools/call`: Execute a tool
- `resources/list`: List available resources
- `resources/read`: Read a resource
- `prompts/list`: List available prompts
- `prompts/get`: Get a specific prompt
- `completion/create`: Create a completion

#### Agent Orchestration Messages
- `agent.register`: Register new agent
- `agent.status`: Update agent status
- `agent.heartbeat`: Keep-alive signal
- `task.assign`: Assign task to agent
- `task.update`: Update task progress
- `task.complete`: Mark task complete

#### Collaboration Messages
- `workspace.join`: Join collaborative workspace
- `document.lock`: Lock document for editing
- `document.update`: CRDT-based document update
- `cursor.position`: Share cursor position
- `selection.change`: Share selection changes

## SDK Support

Official SDKs are available for:

- **Go**: `github.com/developer-mesh/developer-mesh/pkg/client`
- **Python**: `pip install mcp-server-sdk`
- **JavaScript/TypeScript**: `npm install @mcp/server-sdk`
- **Java**: `com.mcp:server-sdk:1.0.0`

### Go SDK Example

```go
import (
    "github.com/developer-mesh/developer-mesh/pkg/client/rest"
)

// Create client
client := rest.NewClient("https://api.mcp-server.example.com", "your-api-key")

// Create context
ctx, err := client.MCP().CreateContext(&models.Context{
    AgentID: "agent-123",
    Content: []models.ContextItem{
        {Role: "user", Content: "Hello"},
    },
})

// Execute tool
result, err := client.Tools().Execute("github", "create_issue", map[string]interface{}{
    "owner": "octocat",
    "repo": "hello-world",
    "title": "Test issue",
})
```

### Python SDK Example

```python
from mcp_server_sdk import MCPClient

# Create client
client = MCPClient(
    base_url="https://api.mcp-server.example.com",
    api_key="your-api-key"
)

# Create context
context = client.mcp.create_context(
    agent_id="agent-123",
    content=[{"role": "user", "content": "Hello"}]
)

# Execute tool
result = client.tools.execute(
    tool="github",
    action="create_issue",
    params={
        "owner": "octocat",
        "repo": "hello-world",
        "title": "Test issue"
    }
)
```

## Additional Resources

- [MCP Protocol Specification](https://github.com/anthropics/mcp)
- [API Playground](https://api.mcp-server.example.com/swagger)
- [Integration Examples](https://github.com/developer-mesh/developer-mesh/examples)

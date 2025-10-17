<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:45:37
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Agent Registration Guide

> **Purpose**: Step-by-step guide for registering AI agents with the Developer Mesh platform
> **Audience**: Developers integrating AI agents
> **Scope**: WebSocket connection and agent registration <!-- Source: pkg/models/websocket/binary.go -->

## Overview

This guide explains how to register AI agents with the Developer Mesh platform. The platform now supports **Universal Agent Registration** which allows any type of agent (IDE, Slack, monitoring, CI/CD, custom) to register and collaborate.

### Registration Process

1. **Connect via WebSocket** with authentication (API key or JWT token) <!-- Source: pkg/models/websocket/binary.go -->
2. **Agent ID is assigned automatically** by the server based on your authentication
3. **Register your agent's capabilities** through WebSocket messages <!-- Source: pkg/models/websocket/binary.go -->
4. **Organization isolation** is automatically enforced based on your credentials

### New Universal Agent System Features

- **Multi-Agent Types**: Support for IDE, Slack, monitoring, CI/CD, and custom agents
- **Capability-Based Discovery**: Agents find each other by capabilities, not type
- **Organization Isolation**: Strict tenant isolation with cross-org access control
- **Dynamic Manifests**: Flexible agent configuration with requirements and auth
- **Message Routing**: Seamless cross-agent communication (IDE→Jira, Slack→IDE)

## Important: WebSocket Client Requirements <!-- Source: pkg/models/websocket/binary.go -->

⚠️ **Critical**: All WebSocket clients MUST request the `mcp.v1` subprotocol during connection. Without this, the server will reject your connection with HTTP 426 Upgrade Required. <!-- Source: pkg/models/websocket/binary.go -->

### No Official SDK Yet

The Developer Mesh project doesn't provide an official client SDK yet. This guide shows how to connect using standard WebSocket libraries following the patterns used in the project's test suite. <!-- Source: pkg/models/websocket/binary.go -->

## How Agent IDs Work

When you connect to the MCP server:
- **With JWT token**: Your agent ID will be your user ID from the JWT
- **With API key**: A new UUID will be generated as your agent ID
- **No manual ID required**: The server handles ID assignment automatically
- **Organization Binding**: Your agent is automatically bound to your organization
- **Tenant Isolation**: Agents can only see/communicate with agents in same org (unless explicitly allowed)

## Quick Start (Go)

Using `github.com/coder/websocket` (the library used by Developer Mesh): <!-- Source: pkg/models/websocket/binary.go -->

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    
    "github.com/coder/websocket" <!-- Source: pkg/models/websocket/binary.go -->
)

func main() {
    ctx := context.Background()
    
    // CRITICAL: Include the mcp.v1 subprotocol
    dialOpts := &websocket.DialOptions{ <!-- Source: pkg/models/websocket/binary.go -->
        Subprotocols: []string{"mcp.v1"}, // REQUIRED!
        HTTPHeader: http.Header{
            "Authorization": []string{"Bearer " + apiKey},
        },
    }
    
    // For local development
    conn, _, err := websocket.Dial(ctx, "ws://localhost:8080/ws", dialOpts) <!-- Source: pkg/models/websocket/binary.go -->
    // For production
    // conn, _, err := websocket.Dial(ctx, "wss://mcp.dev-mesh.io/ws", dialOpts) <!-- Source: pkg/models/websocket/binary.go -->
    
    if err != nil {
        log.Fatal("dial failed:", err)
    }
    defer conn.Close(websocket.StatusNormalClosure, "") <!-- Source: pkg/models/websocket/binary.go -->
    
    // Register agent with universal system
    registration := map[string]interface{}{
        "type":         "agent.universal.register",  // New universal registration
        "name":         "My Agent",
        "agent_type":   "ide",  // ide, slack, monitoring, cicd, custom
        "capabilities": []string{"code_analysis", "documentation", "debugging"},
        "requirements": map[string]interface{}{  // NEW: Agent requirements
            "min_memory": "2GB",
            "apis":       []string{"github", "jira"},
        },
        "metadata": map[string]interface{}{
            "version": "1.0.0",
            "model":   "gpt-4",  // Optional: AI model if applicable
        },
    }
    
    // Send registration as JSON
    if err := wsjson.Write(ctx, conn, registration); err != nil {
        log.Fatal("send failed:", err)
    }
    
    // Read response
    var response map[string]interface{}
    if err := wsjson.Read(ctx, conn, &response); err != nil {
        log.Fatal("read failed:", err)
    }
    
    if response["type"] == "agent.registered" {
        fmt.Printf("Agent registered! ID: %s\n", response["agent_id"])
        fmt.Printf("Organization: %s\n", response["organization_id"])
        fmt.Printf("Manifest ID: %s\n", response["manifest_id"])
    }
}
```

## Quick Start (Other Languages)

### JavaScript/TypeScript
```javascript
// IMPORTANT: Include mcp.v1 in subprotocols array
const ws = new WebSocket('wss://mcp.dev-mesh.io/ws', ['mcp.v1']); <!-- Source: pkg/models/websocket/binary.go -->

// Note: Authorization headers can't be set directly in browser WebSocket API <!-- Source: pkg/models/websocket/binary.go -->
// You may need to use a query parameter or handle auth after connection
ws.onopen = () => {
    ws.send(JSON.stringify({
        type: 'agent.universal.register',  // Universal registration
        name: 'My Agent',
        agent_type: 'ide',  // Specify agent type
        capabilities: ['code_analysis', 'documentation', 'refactoring'],
        requirements: {
            min_memory: '2GB',
            apis: ['github', 'jira']
        },
        metadata: { version: '1.0.0' }
    }));
};

ws.onmessage = (event) => {
    const response = JSON.parse(event.data);
    if (response.type === 'agent.registered') {
        console.log('Agent registered! ID:', response.agent_id);
    }
};
```

### Python
```python
import asyncio
import json
import websockets <!-- Source: pkg/models/websocket/binary.go -->

async def register_agent():
    headers = {
        "Authorization": f"Bearer {api_key}"
    }
    
    # IMPORTANT: Include mcp.v1 subprotocol
    async with websockets.connect( <!-- Source: pkg/models/websocket/binary.go -->
        'wss://mcp.dev-mesh.io/ws',
        subprotocols=['mcp.v1'],
        extra_headers=headers
    ) as websocket: <!-- Source: pkg/models/websocket/binary.go -->
        # Send universal registration
        await websocket.send(json.dumps({ <!-- Source: pkg/models/websocket/binary.go -->
            'type': 'agent.universal.register',
            'name': 'My Agent',
            'agent_type': 'monitoring',  # monitoring agent example
            'capabilities': ['metrics', 'alerts', 'health_checks'],
            'requirements': {
                'prometheus_compatible': True,
                'scrape_interval': '30s'
            },
            'metadata': {'version': '1.0.0'}
        }))
        
        # Receive response
        response = json.loads(await websocket.recv()) <!-- Source: pkg/models/websocket/binary.go -->
        if response['type'] == 'agent.registered':
            print(f"Agent registered! ID: {response['agent_id']}")

asyncio.run(register_agent())
```

## Complete Go Example (Based on Test Agent)

Here's a more complete example based on the project's test agent implementation:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"
    
    "github.com/coder/websocket" <!-- Source: pkg/models/websocket/binary.go -->
    "github.com/coder/websocket/wsjson" <!-- Source: pkg/models/websocket/binary.go -->
)

type Agent struct {
    conn         *websocket.Conn <!-- Source: pkg/models/websocket/binary.go -->
    agentID      string
    name         string
    capabilities []string
    
    mu           sync.RWMutex
    connected    bool
}

func NewAgent(name string, capabilities []string) *Agent {
    return &Agent{
        name:         name,
        capabilities: capabilities,
    }
}

func (a *Agent) Connect(ctx context.Context, wsURL, apiKey string) error {
    dialOpts := &websocket.DialOptions{ <!-- Source: pkg/models/websocket/binary.go -->
        Subprotocols: []string{"mcp.v1"}, // REQUIRED!
        HTTPHeader: http.Header{
            "Authorization": []string{"Bearer " + apiKey},
        },
    }
    
    conn, _, err := websocket.Dial(ctx, wsURL, dialOpts) <!-- Source: pkg/models/websocket/binary.go -->
    if err != nil {
        return fmt.Errorf("websocket dial failed: %w", err) <!-- Source: pkg/models/websocket/binary.go -->
    }
    
    a.conn = conn
    a.connected = true
    
    // Start message handler
    go a.handleMessages(ctx)
    
    // Register agent
    return a.register(ctx)
}

func (a *Agent) register(ctx context.Context) error {
    msg := map[string]interface{}{
        "type":         "agent.register",
        "name":         a.name,
        "capabilities": a.capabilities,
        "metadata": map[string]interface{}{
            "version":   "1.0.0",
            "timestamp": time.Now().Unix(),
        },
    }
    
    if err := wsjson.Write(ctx, a.conn, msg); err != nil {
        return fmt.Errorf("failed to send registration: %w", err)
    }
    
    // Wait for registration response
    var response map[string]interface{}
    if err := wsjson.Read(ctx, a.conn, &response); err != nil {
        return fmt.Errorf("failed to read response: %w", err)
    }
    
    if response["type"] == "agent.registered" {
        a.agentID = response["agent_id"].(string)
        log.Printf("Agent registered successfully! ID: %s", a.agentID)
        return nil
    }
    
    return fmt.Errorf("registration failed: %v", response)
}

func (a *Agent) handleMessages(ctx context.Context) {
    for {
        var msg map[string]interface{}
        err := wsjson.Read(ctx, a.conn, &msg)
        if err != nil {
            log.Printf("Read error: %v", err)
            a.connected = false
            return
        }
        
        msgType, _ := msg["type"].(string)
        switch msgType {
        case "task.execute":
            a.handleTask(ctx, msg)
        case "ping":
            a.sendPong(ctx)
        default:
            log.Printf("Received message type: %s", msgType)
        }
    }
}

func (a *Agent) handleTask(ctx context.Context, msg map[string]interface{}) {
    taskID := msg["task_id"].(string)
    content := msg["content"].(string)
    
    log.Printf("Processing task %s: %s", taskID, content)
    
    // Send task result
    result := map[string]interface{}{
        "type":    "task.result",
        "task_id": taskID,
        "result": map[string]interface{}{
            "status": "completed",
            "output": "Task processed successfully",
        },
    }
    
    if err := wsjson.Write(ctx, a.conn, result); err != nil {
        log.Printf("Failed to send result: %v", err)
    }
}

func (a *Agent) sendPong(ctx context.Context) {
    pong := map[string]interface{}{"type": "pong"}
    if err := wsjson.Write(ctx, a.conn, pong); err != nil {
        log.Printf("Failed to send pong: %v", err)
    }
}

func (a *Agent) Close() error {
    if a.conn != nil {
        return a.conn.Close(websocket.StatusNormalClosure, "") <!-- Source: pkg/models/websocket/binary.go -->
    }
    return nil
}

func main() {
    agent := NewAgent("Example Agent", []string{"code_analysis", "documentation"})
    
    ctx := context.Background()
    
    // Connect to local development server
    err := agent.Connect(ctx, "ws://localhost:8080/ws", "your-api-key")
    // For production: wss://mcp.dev-mesh.io/ws
    
    if err != nil {
        log.Fatal("Failed to connect:", err)
    }
    defer agent.Close()
    
    log.Printf("Agent %s connected and ready!", agent.agentID)
    
    // Keep the agent running
    select {}
}
```

## WebSocket Message Protocol <!-- Source: pkg/models/websocket/binary.go -->

### Universal Registration Message
```json
{
    "type": "agent.universal.register",
    "name": "Agent Name",
    "agent_type": "ide|slack|monitoring|cicd|custom",
    "capabilities": ["capability1", "capability2"],
    "requirements": {
        "min_memory": "2GB",
        "apis": ["github", "jira"],
        "custom_requirement": "value"
    },
    "connection_config": {  // Optional
        "heartbeat_interval": "30s",
        "reconnect_on_failure": true
    },
    "metadata": {
        "version": "1.0.0",
        "model": "claude-3",
        "custom_field": "value"
    }
}
```

### Agent Discovery by Capability
```json
{
    "type": "agent.universal.discover",
    "capability": "issue_management",  // Find agents with this capability
    "agent_type": "jira",  // Optional: filter by type
    "organization_id": "org-uuid"  // Optional: explicit org (admin only)
}
```

### Cross-Agent Message
```json
{
    "type": "agent.universal.message",
    "source_agent": "slack-bot-1",
    "target_capability": "code_assistance",  // Route by capability
    "target_agent": "vscode-agent-2",  // Or specific agent
    "message_type": "code.help",
    "priority": 5,  // 1-10, higher = more important
    "payload": {
        "question": "How to implement OAuth?",
        "context": "Node.js application"
    }
}
```

### Registration Response
```json
{
    "type": "agent.registered",
    "agent_id": "generated-uuid",
    "manifest_id": "manifest-uuid",  // NEW: Manifest tracking
    "organization_id": "org-uuid",  // NEW: Organization binding
    "name": "Agent Name",
    "agent_type": "ide",
    "capabilities": ["capability1", "capability2"],
    "registered_at": "2024-01-20T10:00:00Z",
    "isolation_mode": "strict"  // NEW: Tenant isolation mode
}
```

### Discovery Response
```json
{
    "type": "agents.discovered",
    "agents": [
        {
            "agent_id": "ide-agent-1",
            "agent_type": "ide",
            "name": "VS Code Agent",
            "capabilities": ["code_completion", "debugging"],
            "status": "online",
            "workload": 3  // Current task count
        },
        {
            "agent_id": "jira-agent-2",
            "agent_type": "jira",
            "name": "Jira Integration",
            "capabilities": ["issue_management", "sprint_planning"],
            "status": "online",
            "workload": 1
        }
    ],
    "total": 2,
    "filtered_by_organization": true  // Indicates org isolation applied
}
```

### Error Response
```json
{
    "type": "error",
    "error": "Registration failed: reason",
    "code": 400
}
```

## Common Issues and Solutions

### HTTP 426 Upgrade Required
**Problem**: Connection rejected with 426 error  
**Solution**: Ensure you're including `Subprotocols: ["mcp.v1"]` in your dial options

### Authentication Failed
**Problem**: 401 Unauthorized error  
**Solution**: 
- Verify your API key is valid
- Ensure Authorization header format is `Bearer <token>`
- For JWT tokens, check expiration

### Connection Drops Immediately
**Problem**: Connection established but closes immediately  
**Solution**:
- Check server logs for detailed error messages
- Verify all required headers are being sent
- Ensure the WebSocket endpoint URL is correct (`/ws`) <!-- Source: pkg/models/websocket/binary.go -->

### Using Wrong WebSocket Library <!-- Source: pkg/models/websocket/binary.go -->
**Problem**: Code examples from other sources use different libraries  
**Solution**: While the server works with any compliant WebSocket client, this project's tests use `github.com/coder/websocket` for Go. You can use any library that supports: <!-- Source: pkg/models/websocket/binary.go -->
- Custom subprotocols
- Custom headers for authentication
- JSON message encoding/decoding

## Binary Protocol Support <!-- Source: pkg/models/websocket/binary.go -->

The MCP server supports a binary protocol for improved performance. This is automatically negotiated based on message size and type. The test agent implementation includes binary protocol support if you need this feature. <!-- Source: pkg/models/websocket/binary.go -->

## Organization Isolation and Tenant Security

### How Isolation Works

1. **Automatic Organization Binding**: 
   - Agents are bound to the organization of their API key/JWT
   - Cannot be overridden by the agent
   - Enforced at the database level

2. **Strict Isolation Mode**:
   - Organizations can enable strict isolation
   - Prevents ALL cross-organization communication
   - Even if agents know each other's IDs

3. **Capability-Based Discovery**:
   - Only discovers agents within same organization
   - Admin override available for special cases
   - Filtered at the query level for performance

4. **Message Routing Security**:
   - Cross-org messages blocked by default
   - Explicit allow-list for partner organizations
   - All attempts logged for audit

### Rate Limiting and Circuit Breakers

The universal agent system includes sophisticated protection:

1. **Multi-Level Rate Limiting**:
   - Per-agent limits (default: 10 RPS)
   - Per-tenant limits (default: 100 RPS)
   - Per-capability limits (default: 50 RPS)
   - Burst capacity with sliding windows

2. **Circuit Breakers**:
   - Per-agent breakers (trip after 3 failures)
   - Per-capability breakers (trip after 10 failures)
   - Automatic recovery after timeout
   - Health marking for persistent failures

3. **Tenant Configuration**:
   - Custom rate limits per tenant
   - Feature flags for capabilities
   - Service token management
   - CORS origin control

### Example: Strict Tenant Isolation

```go
// Organization with strict isolation
org := &Organization{
    ID:               uuid.New(),
    Name:             "Secure Corp",
    StrictlyIsolated: true,  // No cross-org access
}

// Agents in this org cannot:
// - Discover agents in other orgs
// - Send messages to other orgs
// - Receive messages from other orgs
// Even if explicitly addressed
```

## Agent Types and Use Cases

### IDE Agents
```json
{
    "agent_type": "ide",
    "capabilities": [
        "code_completion",
        "code_analysis",
        "refactoring",
        "debugging",
        "test_generation"
    ],
    "examples": ["VS Code", "IntelliJ", "Neovim"]
}
```

### Slack/Chat Agents
```json
{
    "agent_type": "slack",
    "capabilities": [
        "notifications",
        "alerts",
        "messaging",
        "incident_response",
        "team_coordination"
    ],
    "examples": ["Slack Bot", "Teams Bot", "Discord Bot"]
}
```

### Monitoring Agents
```json
{
    "agent_type": "monitoring",
    "capabilities": [
        "metrics",
        "alerts",
        "health_checks",
        "performance_analysis",
        "anomaly_detection"
    ],
    "examples": ["Prometheus", "DataDog", "New Relic"]
}
```

### CI/CD Agents
```json
{
    "agent_type": "cicd",
    "capabilities": [
        "build_execution",
        "test_execution",
        "deployment",
        "pipeline_management",
        "artifact_management"
    ],
    "examples": ["Jenkins", "GitHub Actions", "GitLab CI"]
}
```

## Cross-Agent Communication Patterns

### IDE → Jira
```javascript
// IDE agent creates Jira issue from code comment
{
    "type": "agent.universal.message",
    "source_agent": "vscode-agent",
    "target_capability": "issue_management",
    "message_type": "issue.create",
    "payload": {
        "title": "Fix authentication bug",
        "description": "User login fails with valid credentials",
        "priority": "high",
        "labels": ["bug", "authentication"]
    }
}
```

### Monitoring → Slack
```javascript
// Monitoring agent sends critical alert to Slack
{
    "type": "agent.universal.message",
    "source_agent": "prometheus-agent",
    "target_capability": "notifications",
    "message_type": "alert.critical",
    "priority": 10,
    "payload": {
        "metric": "cpu_usage",
        "value": 95.5,
        "threshold": 90,
        "host": "prod-api-01",
        "message": "CPU usage critical on production API server"
    }
}
```

### Slack → IDE
```javascript
// Slack bot requests code help from IDE
{
    "type": "agent.universal.message",
    "source_agent": "slack-bot",
    "target_capability": "code_assistance",
    "message_type": "code.help",
    "payload": {
        "user": "@developer",
        "question": "How to implement rate limiting?",
        "language": "Go",
        "channel": "#dev-help"
    }
}
```

## Next Steps

- Review [WebSocket Client Requirements](../WEBSOCKET_CLIENT_REQUIREMENTS.md) for detailed protocol requirements <!-- Source: pkg/models/websocket/binary.go -->
- See the [test agent implementation](../../test/e2e/agent/agent.go) for a complete example
- Check [MCP Server API Reference](../api-reference/mcp-server-reference.md) for all message types
- Learn about [Task Routing Algorithms](./task-routing-algorithms.md) for task distribution <!-- Source: pkg/services/assignment_engine.go -->
- Read [Multi-Tenant API Implementation](../MULTI_TENANT_API_IMPLEMENTATION_PLAN.md) for tenant isolation details

## Note on SDK Development


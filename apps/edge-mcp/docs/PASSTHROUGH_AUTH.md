# Pass-Through Authentication Implementation Guide

## Overview

Pass-through authentication allows Edge MCP to forward user-specific credentials (like GitHub Personal Access Tokens) to the Core Platform, which then uses these credentials to execute actions on the user's behalf, as the user, within external services.

## Current Implementation Status

⚠️ **NOT YET IMPLEMENTED** - Edge MCP currently does not support pass-through authentication. This document describes the required implementation.

## Required Changes

### 1. Update Session Structure

```go
// internal/mcp/handler.go
type Session struct {
    ID           string
    ConnectionID string
    Initialized  bool
    TenantID     string
    EdgeMCPID    string
    CoreSession  string
    CreatedAt    time.Time
    LastActivity time.Time
    
    // ADD: Passthrough authentication
    PassthroughAuth *models.PassthroughAuthBundle
}
```

### 2. Extract Headers on Connection

```go
// internal/mcp/handler.go
func (h *Handler) HandleConnection(conn *websocket.Conn, r *http.Request) {
    sessionID := uuid.New().String()
    
    // Extract passthrough auth from headers
    passthroughAuth := h.extractPassthroughAuth(r)
    
    session := &Session{
        ID:              sessionID,
        ConnectionID:    uuid.New().String(),
        CreatedAt:       time.Now(),
        LastActivity:    time.Now(),
        PassthroughAuth: passthroughAuth,
    }
    // ...
}

func (h *Handler) extractPassthroughAuth(r *http.Request) *models.PassthroughAuthBundle {
    bundle := &models.PassthroughAuthBundle{
        Credentials:   make(map[string]*models.PassthroughCredential),
        CustomHeaders: make(map[string]string),
    }
    
    // Extract GitHub token
    if token := r.Header.Get("X-GitHub-Token"); token != "" {
        bundle.Credentials["github"] = &models.PassthroughCredential{
            Type:  "bearer",
            Token: token,
        }
    }
    
    // Extract generic user token
    if token := r.Header.Get("X-User-Token"); token != "" {
        bundle.Credentials["*"] = &models.PassthroughCredential{
            Type:  "bearer",
            Token: token,
        }
    }
    
    // Extract AWS credentials
    if accessKey := r.Header.Get("X-AWS-Access-Key"); accessKey != "" {
        if secretKey := r.Header.Get("X-AWS-Secret-Key"); secretKey != "" {
            bundle.Credentials["aws"] = &models.PassthroughCredential{
                Type: "aws_signature",
                Properties: map[string]string{
                    "access_key": accessKey,
                    "secret_key": secretKey,
                    "region":     r.Header.Get("X-AWS-Region"),
                },
            }
        }
    }
    
    // Extract other service-specific tokens
    for key, values := range r.Header {
        if strings.HasPrefix(key, "X-Service-") && strings.HasSuffix(key, "-Token") {
            serviceName := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(key, "X-Service-"), "-Token"))
            bundle.Credentials[serviceName] = &models.PassthroughCredential{
                Type:  "bearer",
                Token: values[0],
            }
        }
    }
    
    if len(bundle.Credentials) == 0 {
        return nil
    }
    
    return bundle
}
```

### 3. Update Core Client to Forward Passthrough Auth

```go
// internal/core/client.go
func (c *Client) createProxyHandlerWithPassthrough(toolName string, passthroughAuth *models.PassthroughAuthBundle) tools.ToolHandler {
    return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
        if !c.connected {
            return nil, fmt.Errorf("not connected to Core Platform")
        }
        
        payload := map[string]interface{}{
            "tool":            toolName,
            "arguments":       args,
            "passthrough_auth": passthroughAuth,
        }
        
        resp, err := c.doRequest(ctx, "POST", "/api/v1/tools/execute", payload)
        // ... rest of implementation
    }
}
```

### 4. Pass Session Auth to Tool Execution

```go
// internal/mcp/handler.go
func (h *Handler) handleToolCall(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    session := h.sessions[sessionID]
    
    // When executing tool, pass the session's passthrough auth
    if session.PassthroughAuth != nil {
        // Modify tool execution to include passthrough auth
        // This requires updating the tool registry to support passthrough
    }
    // ...
}
```

## How It Works

### Authentication Flow

```
1. User configures IDE with their personal tokens:
   - GitHub PAT
   - AWS credentials
   - Other service tokens

2. IDE connects to Edge MCP via WebSocket, sending tokens in headers:
   WebSocket Headers:
   - X-GitHub-Token: ghp_xxxxxxxxxxxx
   - X-AWS-Access-Key: AKIAXXXXXXXX
   - X-AWS-Secret-Key: xxxxxxxxxx

3. Edge MCP extracts and stores these tokens in the session

4. When executing a tool, Edge MCP forwards tokens to Core Platform:
   POST /api/v1/tools/execute
   {
     "tool": "github.create_pr",
     "arguments": {...},
     "passthrough_auth": {
       "credentials": {
         "github": {
           "type": "bearer",
           "token": "ghp_xxxxxxxxxxxx"
         }
       }
     }
   }

5. Core Platform uses the user's token instead of service credentials

6. Action is performed as the user, with full attribution
```

## IDE Configuration Examples

### Claude Code with Passthrough Auth

```json
{
  "mcpServers": {
    "devmesh": {
      "command": "edge-mcp",
      "args": ["--port", "8082"],
      "env": {
        "DEV_MESH_URL": "${DEV_MESH_URL}",
        "DEV_MESH_API_KEY": "${DEV_MESH_API_KEY}"
      },
      "headers": {
        "X-GitHub-Token": "${GITHUB_TOKEN}",
        "X-AWS-Access-Key": "${AWS_ACCESS_KEY_ID}",
        "X-AWS-Secret-Key": "${AWS_SECRET_ACCESS_KEY}"
      }
    }
  }
}
```

### Environment Setup

```bash
# Core Platform credentials (required)
export DEV_MESH_URL="https://api.devmesh.io"
export DEV_MESH_API_KEY="devmesh_xxx..."  # Your API key from organization registration
# Note: Tenant ID is automatically determined from your API key

# User-specific tokens for passthrough (optional but recommended)
export GITHUB_TOKEN="ghp_your_personal_access_token"
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
```

## Security Considerations

### Token Protection
- Tokens are never logged or stored persistently
- Tokens are only held in memory for the session duration
- Tokens are transmitted over TLS/WSS connections
- Tokens are masked in any debug output

### Audit Trail
- All actions using passthrough auth are logged with user attribution
- Failed authentication attempts are logged
- Token usage is tracked per user and service

### Token Validation
- Edge MCP validates token format before forwarding
- Core Platform validates tokens with the actual service
- Invalid tokens result in clear error messages

## Benefits of Passthrough Authentication

1. **User Attribution**: Actions are performed as the actual user, maintaining proper audit trails in GitHub, AWS, etc.

2. **Permission Scoping**: Users can only perform actions their personal tokens allow

3. **No Shared Credentials**: Each user uses their own credentials, eliminating shared service account risks

4. **Token Rotation**: Users can rotate their personal tokens without affecting others

5. **Compliance**: Meets security requirements for user-level authentication

## Testing Passthrough Auth

### Manual Test with curl

```bash
# Test WebSocket connection with passthrough headers
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "X-GitHub-Token: ghp_your_token" \
  -H "X-AWS-Access-Key: AKIAXXXXXXXX" \
  -H "X-AWS-Secret-Key: xxxxxxxxxx" \
  http://localhost:8082/ws
```

### Test Tool Execution

```json
// After connecting, send tool execution request
{
  "jsonrpc": "2.0",
  "id": "test-1",
  "method": "tools/call",
  "params": {
    "name": "github.create_pr",
    "arguments": {
      "repo": "myorg/myrepo",
      "title": "Test PR",
      "body": "Created with my personal token"
    }
  }
}

// The PR will be created as YOU, not as a service account
```

## Migration Path

### Phase 1: Header Extraction (Current Priority)
- Implement header extraction in Edge MCP
- Store passthrough auth in session
- Log when passthrough tokens are detected

### Phase 2: Core Platform Integration
- Update Core Client to forward passthrough auth
- Modify tool execution to include auth bundle
- Test with non-critical tools first

### Phase 3: Full Rollout
- Enable for all tools that support passthrough
- Update documentation
- Provide migration guide for users

## Troubleshooting

### "No passthrough credentials provided"
- Check that tokens are set in environment variables
- Verify IDE configuration includes headers
- Ensure Edge MCP is built with passthrough support

### "Invalid GitHub token"
- Verify token has required scopes
- Check token hasn't expired
- Ensure token format is correct (starts with `ghp_`)

### "Tool doesn't support passthrough"
- Some tools may only work with service credentials
- Check tool configuration in DevMesh dashboard
- Contact support to enable passthrough for the tool
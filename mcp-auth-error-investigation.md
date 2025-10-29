# MCP GitHub Integration Authentication Error - Root Cause Analysis

## Error Message
```
status 500: {"error":"failed to execute operation: failed to get client: failed to extract auth token: no authentication token found"}
```

## Investigation Summary

Conducted thorough investigation of the MCP GitHub integration authentication flow to identify why the MCP tool `mcp__devmesh__github_create_pull_request` fails with authentication error despite the passthrough auth mechanism being in place.

---

## Root Cause Identified

**The REST API endpoint `/api/v1/tools/:toolId/execute` is NOT passing the passthrough authentication to the tool registry.**

### Evidence Chain

#### 1. Error Origin (pkg/tools/providers/github/github_provider.go:782)

The error originates from `GitHubProvider.extractAuthToken()` method which tries multiple sources for authentication:

```go
func (p *GitHubProvider) extractAuthToken(ctx context.Context, params map[string]interface{}) (string, error) {
    // Try from ProviderContext first (standard provider pattern)
    if pctx, ok := providers.FromContext(ctx); ok && pctx != nil && pctx.Credentials != nil {
        if pctx.Credentials.Token != "" {
            return pctx.Credentials.Token, nil
        }
        // Also check custom credentials map
        if token, ok := pctx.Credentials.Custom["token"]; ok && token != "" {
            return token, nil
        }
        // Check for GitHub-specific key
        if token, ok := pctx.Credentials.Custom["github_token"]; ok && token != "" {
            return token, nil
        }
    }

    // Try passthrough auth from params
    if auth, ok := params["__passthrough_auth"].(map[string]interface{}); ok {
        if encryptedToken, ok := auth["encrypted_token"].(string); ok {
            // Decrypt the token...
            return token, nil
        }
        if token, ok := auth["token"].(string); ok {
            return token, nil
        }
    }

    // Try direct token from params
    if token, ok := params["token"].(string); ok {
        return token, nil
    }

    // Try from context
    if token, ok := ctx.Value("github_token").(string); ok {
        return token, nil
    }

    return "", fmt.Errorf("no authentication token found")  // <-- ERROR THROWN HERE
}
```

**All 4 authentication sources are failing**, which means no credentials are being passed to the provider.

#### 2. EnhancedToolRegistry Sets Up ProviderContext (pkg/services/enhanced_tool_registry.go:584-595)

The tool registry DOES create a ProviderContext with credentials:

```go
// Create provider context
pctx := &providers.ProviderContext{
    TenantID:       orgTool.TenantID,
    OrganizationID: orgTool.OrganizationID,
    Credentials: &providers.ProviderCredentials{
        Token:  credentials["token"],
        APIKey: credentials["api_key"],
        Email:  credentials["email"],
        Custom: credentials,
    },
}
ctx = providers.WithContext(ctx, pctx)

// Execute operation
startTime := time.Now()
result, err := provider.ExecuteOperation(ctx, operation, params)
```

**This works correctly when credentials are stored in the database**, but when using passthrough auth (user's own credentials), there are no stored credentials.

#### 3. Edge MCP Proxy Handler Sends Passthrough Auth (apps/edge-mcp/internal/core/client.go:611-630)

The Edge MCP proxy handler DOES extract passthrough auth from context and includes it in the REST API request:

```go
// Include passthrough auth if available
if passthroughAuth != nil {
    payload["passthrough_auth"] = passthroughAuth

    // Log detailed passthrough auth info
    authInfo := map[string]interface{}{
        "tool":             toolName,
        "has_passthrough":  true,
        "credential_count": len(passthroughAuth.Credentials),
    }

    // Log each credential provider and token length (without exposing tokens)
    for provider, cred := range passthroughAuth.Credentials {
        if cred != nil {
            authInfo[provider+"_token_len"] = len(cred.Token)
        }
    }

    c.logger.Info("Including passthrough auth in tool execution", authInfo)
}

// Use the correct endpoint with tool ID
endpoint := fmt.Sprintf("/api/v1/tools/%s/execute", toolID)
resp, err := c.doRequest(ctx, "POST", endpoint, payload)
```

**So the passthrough auth IS being sent to the REST API.**

#### 4. THE BROKEN LINK: REST API Handler (apps/rest-api/internal/api/enhanced_tools_api.go:317-348)

**This is where the authentication is lost!**

```go
// ExecuteOrganizationTool executes an action on an organization tool
func (api *EnhancedToolsAPI) ExecuteOrganizationTool(c *gin.Context) {
    orgID := c.Param("orgId")
    toolID := c.Param("toolId")
    tenantID := c.GetString("tenant_id")

    _ = orgID // For future org-level authorization

    var req models.ToolExecutionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // BUG: This calls ExecuteTool WITHOUT passthrough auth!
    result, err := api.toolRegistry.ExecuteTool(
        c.Request.Context(),
        tenantID,
        toolID,
        req.Action,
        req.Parameters,  // <-- ONLY PARAMETERS, NO PASSTHROUGH AUTH!
    )
    if err != nil {
        api.logger.Error("Failed to execute tool", map[string]interface{}{
            "tool_id": toolID,
            "action":  req.Action,
            "error":   err.Error(),
        })
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, result)
}
```

**The `req` variable contains `PassthroughAuth` field (see pkg/models/dynamic_tools.go:100), but the handler is NOT using it!**

#### 5. ToolExecutionRequest Model HAS PassthroughAuth (pkg/models/dynamic_tools.go:95-101)

```go
// ToolExecutionRequest represents a request to execute a tool action
type ToolExecutionRequest struct {
    Action          string                 `json:"action"`
    Parameters      map[string]interface{} `json:"parameters,omitempty"`
    Headers         map[string]string      `json:"headers,omitempty"`
    Timeout         int                    `json:"timeout,omitempty"` // in seconds
    PassthroughAuth *PassthroughAuthBundle `json:"passthrough_auth,omitempty"`  // <-- FIELD EXISTS!
}
```

**The model supports it, the handler just doesn't use it!**

#### 6. ExecuteToolWithPassthrough EXISTS (pkg/services/enhanced_tool_registry.go & enhanced_tools_api.go:90-97)

```go
// ExecuteToolInternalWithPassthrough executes a tool operation with passthrough auth (used internally by DynamicToolsAPI)
func (api *EnhancedToolsAPI) ExecuteToolInternalWithPassthrough(
    ctx context.Context,
    tenantID, toolID, action string,
    params map[string]interface{},
    passthroughAuth *models.PassthroughAuthBundle,
) (interface{}, error) {
    return api.toolRegistry.ExecuteToolWithPassthrough(ctx, tenantID, toolID, action, params, passthroughAuth)
}
```

**The method exists but the handler doesn't call it!**

---

## Complete Authentication Flow

### Current (Broken) Flow

```
MCP Client (Claude Code)
  ↓ [includes GitHub token in passthrough_auth]
Edge MCP Handler (handleToolCall)
  ↓ [extracts passthrough auth from session]
  ↓ [adds to context: ctx.Value(PassthroughAuthKey)]
Edge MCP Proxy Handler (createProxyHandler)
  ↓ [extracts passthrough auth from context]
  ↓ [includes in payload: payload["passthrough_auth"] = passthroughAuth]
  ↓ [POST /api/v1/tools/{toolID}/execute with payload]
REST API Handler (ExecuteOrganizationTool)
  ↓ [binds JSON to req.PassthroughAuth] ✅
  ↓ [calls api.toolRegistry.ExecuteTool WITHOUT req.PassthroughAuth] ❌ BUG!
EnhancedToolRegistry.ExecuteTool
  ↓ [tries to get stored credentials from database]
  ↓ [NO STORED CREDENTIALS for passthrough mode]
  ↓ [creates ProviderContext with EMPTY credentials]
GitHubProvider.GetClient
  ↓ [calls extractAuthToken]
  ↓ [finds NO credentials in ProviderContext]
  ↓ [finds NO passthrough_auth in params]
  ↓ [finds NO token in params]
  ↓ [finds NO token in context]
  ↓ ERROR: "no authentication token found"
```

### Fixed Flow

```
MCP Client (Claude Code)
  ↓ [includes GitHub token in passthrough_auth]
Edge MCP Handler (handleToolCall)
  ↓ [extracts passthrough auth from session]
  ↓ [adds to context: ctx.Value(PassthroughAuthKey)]
Edge MCP Proxy Handler (createProxyHandler)
  ↓ [extracts passthrough auth from context]
  ↓ [includes in payload: payload["passthrough_auth"] = passthroughAuth]
  ↓ [POST /api/v1/tools/{toolID}/execute with payload]
REST API Handler (ExecuteOrganizationTool)
  ↓ [binds JSON to req.PassthroughAuth] ✅
  ↓ [calls api.toolRegistry.ExecuteToolWithPassthrough WITH req.PassthroughAuth] ✅ FIXED!
EnhancedToolRegistry.ExecuteToolWithPassthrough
  ↓ [receives passthroughAuth parameter]
  ↓ [extracts credentials from passthroughAuth.Credentials["github"]]
  ↓ [creates ProviderContext with passthrough credentials]
GitHubProvider.GetClient
  ↓ [calls extractAuthToken]
  ↓ [finds credentials in ProviderContext] ✅
  ↓ SUCCESS: Creates GitHub client with user's token
```

---

## The Fix

### File: apps/rest-api/internal/api/enhanced_tools_api.go

**Change line 330-336 from:**
```go
result, err := api.toolRegistry.ExecuteTool(
    c.Request.Context(),
    tenantID,
    toolID,
    req.Action,
    req.Parameters,
)
```

**To:**
```go
// Use ExecuteToolWithPassthrough if passthrough auth is provided
var result interface{}
var err error

if req.PassthroughAuth != nil {
    result, err = api.toolRegistry.ExecuteToolWithPassthrough(
        c.Request.Context(),
        tenantID,
        toolID,
        req.Action,
        req.Parameters,
        req.PassthroughAuth,
    )
} else {
    result, err = api.toolRegistry.ExecuteTool(
        c.Request.Context(),
        tenantID,
        toolID,
        req.Action,
        req.Parameters,
    )
}
```

---

## Why This Wasn't Caught Earlier

1. **Stored credentials work fine** - Tools with credentials stored in the database (encrypted) work correctly because `ExecuteTool` retrieves them
2. **Passthrough auth is newer** - The passthrough auth mechanism was added later for IDE/Claude Code integration
3. **The model supports it** - `ToolExecutionRequest` has the `PassthroughAuth` field, but the handler doesn't use it
4. **Method exists but unused** - `ExecuteToolWithPassthrough` exists but the endpoint handler doesn't call it

---

## Impact

### What's Broken
- ❌ All MCP tools that rely on passthrough auth (user's own credentials)
- ❌ Claude Code → MCP → REST API → GitHub provider flow
- ❌ IDE integrations using passthrough auth
- ❌ Any tool execution from Edge MCP that includes passthrough auth

### What Still Works
- ✅ Tools with stored credentials in the database
- ✅ Direct REST API calls with stored credentials
- ✅ Internal tool execution with stored credentials

---

## Testing the Fix

After applying the fix, test with:

```bash
# Using MCP tool from Claude Code
mcp__devmesh__github_create_pull_request({
  "owner": "developer-mesh",
  "repo": "developer-mesh",
  "title": "Test PR",
  "head": "feature/test",
  "base": "main",
  "body": "Test body"
})
```

Expected: PR is created using the user's GitHub token from passthrough auth.

---

## Additional Considerations

### Should We Always Use PassthroughAuth if Available?

**YES!** The fix should prioritize passthrough auth over stored credentials:

```go
// Use passthrough auth if provided (higher priority than stored credentials)
if req.PassthroughAuth != nil {
    api.logger.Info("Executing tool with passthrough auth", map[string]interface{}{
        "tool_id":    toolID,
        "action":     req.Action,
        "providers":  len(req.PassthroughAuth.Credentials),
    })
    result, err = api.toolRegistry.ExecuteToolWithPassthrough(
        c.Request.Context(),
        tenantID,
        toolID,
        req.Action,
        req.Parameters,
        req.PassthroughAuth,
    )
} else {
    api.logger.Debug("Executing tool with stored credentials", map[string]interface{}{
        "tool_id": toolID,
        "action":  req.Action,
    })
    result, err = api.toolRegistry.ExecuteTool(
        c.Request.Context(),
        tenantID,
        toolID,
        req.Action,
        req.Parameters,
    )
}
```

### Other Endpoints to Check

Similar fix may be needed in:
- `apps/rest-api/internal/api/dynamic_tools_api.go` (if it has execute endpoints)
- Any other endpoints that execute tools

---

## Summary

**Root Cause**: The REST API endpoint handler `ExecuteOrganizationTool` receives passthrough auth in the request body but doesn't pass it to the tool registry, causing authentication to fail for all MCP tool executions that rely on user credentials.

**Fix**: Update the handler to check for `req.PassthroughAuth` and call `ExecuteToolWithPassthrough` instead of `ExecuteTool` when passthrough auth is present.

**Impact**: This is a critical bug affecting all Claude Code and IDE integrations using passthrough auth for MCP tools.

**Confidence**: 100% - Complete evidence chain traced through the entire call stack from MCP client to GitHub provider.

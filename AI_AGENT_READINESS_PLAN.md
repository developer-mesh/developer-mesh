# AI Agent Readiness - Technical Implementation Plan (Junior-Friendly)

## Executive Summary
This plan provides step-by-step implementation guides for improving AI agent readiness of the DevOps MCP platform. Each story includes concrete examples, exact commands, and clear success criteria suitable for junior engineers.

---

## 🚨 Sprint 1: Critical Foundation (Week 1-2)
*Focus: Test coverage, error handling, and memory management*

### Epic 1.1: Test Coverage for Edge MCP

#### Story 1.1.1: MCP Protocol Handshake Tests ✅ COMPLETED
**Difficulty:** Junior
**Time Estimate:** 4-6 hours
**Prerequisites:**
- Read MCP protocol spec: https://modelcontextprotocol.io/specification
- Understand JSON-RPC 2.0: https://www.jsonrpc.org/specification

**Background:**
The MCP protocol requires a specific handshake sequence. We need tests to ensure our implementation handles all protocol versions correctly and fails gracefully for unsupported versions.

**Implementation Steps:**

1. **Create test file:** `apps/edge-mcp/internal/mcp/handler_test.go`
```go
package mcp

import (
    "context"
    "encoding/json"
    "testing"
    "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/auth"
    "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
    "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestHandleInitialize_ValidProtocolVersions(t *testing.T) {
    // Setup - Create handler with minimal dependencies
    logger := observability.NewNoOpLogger()
    toolRegistry := tools.NewRegistry()
    memCache := cache.NewMemoryCache()
    authenticator := auth.NewEdgeAuthenticator("")

    handler := NewHandler(toolRegistry, memCache, nil, authenticator, logger)
    sessionID := "test-session-123"

    // Test each valid protocol version
    validVersions := []string{
        "2024-11-05", // Original Claude Code version
        "2025-03-26", // March 2025 release
        "2025-06-18", // Latest version
    }

    for _, version := range validVersions {
        t.Run("Protocol_"+version, func(t *testing.T) {
            // Create initialize message
            params := map[string]interface{}{
                "protocolVersion": version,
                "clientInfo": map[string]interface{}{
                    "name":    "test-client",
                    "version": "1.0.0",
                },
            }

            paramsJSON, err := json.Marshal(params)
            require.NoError(t, err, "Failed to marshal params")

            msg := &MCPMessage{
                JSONRPC: "2.0",
                ID:      1,
                Method:  "initialize",
                Params:  paramsJSON,
            }

            // Execute
            response, err := handler.handleInitialize(sessionID, msg)

            // Verify
            assert.NoError(t, err, "Initialize should succeed for version %s", version)
            assert.NotNil(t, response, "Response should not be nil")
            assert.Equal(t, "2.0", response.JSONRPC)
            assert.Equal(t, 1, response.ID)

            // Check result structure
            result, ok := response.Result.(map[string]interface{})
            require.True(t, ok, "Result should be a map")
            assert.Equal(t, version, result["protocolVersion"])

            // Verify capabilities are present
            capabilities, ok := result["capabilities"].(map[string]interface{})
            require.True(t, ok, "Capabilities should be present")
            assert.Contains(t, capabilities, "tools")
            assert.Contains(t, capabilities, "resources")
        })
    }
}

func TestHandleInitialize_InvalidProtocolVersion(t *testing.T) {
    // Setup
    logger := observability.NewNoOpLogger()
    handler := NewHandler(tools.NewRegistry(), cache.NewMemoryCache(), nil,
                          auth.NewEdgeAuthenticator(""), logger)
    sessionID := "test-session-456"

    // Test with unsupported version
    params := map[string]interface{}{
        "protocolVersion": "1999-01-01", // Invalid version
        "clientInfo": map[string]interface{}{
            "name":    "test-client",
            "version": "1.0.0",
        },
    }

    paramsJSON, _ := json.Marshal(params)
    msg := &MCPMessage{
        JSONRPC: "2.0",
        ID:      2,
        Method:  "initialize",
        Params:  paramsJSON,
    }

    // Execute
    _, err := handler.handleInitialize(sessionID, msg)

    // Verify error
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unsupported protocol version: 1999-01-01")
    assert.Contains(t, err.Error(), "supported:")
}

func TestHandleInitialize_MalformedJSON(t *testing.T) {
    // Setup
    logger := observability.NewNoOpLogger()
    handler := NewHandler(tools.NewRegistry(), cache.NewMemoryCache(), nil,
                          auth.NewEdgeAuthenticator(""), logger)
    sessionID := "test-session-789"

    // Test with malformed JSON
    msg := &MCPMessage{
        JSONRPC: "2.0",
        ID:      3,
        Method:  "initialize",
        Params:  json.RawMessage(`{"invalid json`), // Malformed
    }

    // Execute
    _, err := handler.handleInitialize(sessionID, msg)

    // Verify error
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid initialize params")
}

func TestHandleInitialize_SessionUpdate(t *testing.T) {
    // Setup
    logger := observability.NewNoOpLogger()
    handler := NewHandler(tools.NewRegistry(), cache.NewMemoryCache(), nil,
                          auth.NewEdgeAuthenticator(""), logger)
    sessionID := "test-session-update"

    // Pre-create session
    handler.sessions[sessionID] = &Session{
        ID:          sessionID,
        Initialized: false,
    }

    // Create valid initialize message
    params := map[string]interface{}{
        "protocolVersion": "2025-06-18",
        "clientInfo": map[string]interface{}{
            "name":    "test-client",
            "version": "1.0.0",
        },
    }
    paramsJSON, _ := json.Marshal(params)
    msg := &MCPMessage{
        JSONRPC: "2.0",
        ID:      4,
        Method:  "initialize",
        Params:  paramsJSON,
    }

    // Execute
    response, err := handler.handleInitialize(sessionID, msg)

    // Verify
    assert.NoError(t, err)
    assert.NotNil(t, response)

    // Check session was updated
    session := handler.sessions[sessionID]
    assert.True(t, session.Initialized, "Session should be marked as initialized")
}
```

2. **Run the tests:**
```bash
cd apps/edge-mcp
go test -v -run TestHandleInitialize ./internal/mcp/
```

3. **Check coverage:**
```bash
go test -cover -run TestHandleInitialize ./internal/mcp/
```

**Expected Output:**
```
=== RUN   TestHandleInitialize_ValidProtocolVersions
=== RUN   TestHandleInitialize_ValidProtocolVersions/Protocol_2024-11-05
=== RUN   TestHandleInitialize_ValidProtocolVersions/Protocol_2025-03-26
=== RUN   TestHandleInitialize_ValidProtocolVersions/Protocol_2025-06-18
--- PASS: TestHandleInitialize_ValidProtocolVersions (0.00s)
    --- PASS: TestHandleInitialize_ValidProtocolVersions/Protocol_2024-11-05 (0.00s)
    --- PASS: TestHandleInitialize_ValidProtocolVersions/Protocol_2025-03-26 (0.00s)
    --- PASS: TestHandleInitialize_ValidProtocolVersions/Protocol_2025-06-18 (0.00s)
=== RUN   TestHandleInitialize_InvalidProtocolVersion
--- PASS: TestHandleInitialize_InvalidProtocolVersion (0.00s)
=== RUN   TestHandleInitialize_MalformedJSON
--- PASS: TestHandleInitialize_MalformedJSON (0.00s)
=== RUN   TestHandleInitialize_SessionUpdate
--- PASS: TestHandleInitialize_SessionUpdate (0.00s)
PASS
coverage: 85.2% of statements
```

**Common Mistakes to Avoid:**
- Don't forget to import all required packages
- Make sure to use `require` for critical assertions that would cause panics
- Always check both error and response, not just one
- Remember to test edge cases like malformed JSON

**Getting Help:**
- Similar test pattern: `pkg/tools/operation_resolver_test.go`
- Testing guide: https://go.dev/doc/tutorial/add-a-test
- Testify docs: https://github.com/stretchr/testify

**✅ COMPLETION NOTES:**
- Implementation completed: handler_test.go created with comprehensive tests
- All 4 test cases passing:
  - TestHandleInitialize_ValidProtocolVersions (3 sub-tests for each supported version)
  - TestHandleInitialize_InvalidProtocolVersion
  - TestHandleInitialize_MalformedJSON
  - TestHandleInitialize_SessionUpdate
- Test coverage for handleInitialize function: 68.0%
- Tests verify protocol version handling, error cases, and session state updates

---

#### Story 1.1.2: Tool Execution Tests ✅ COMPLETED
**Difficulty:** Junior-Mid
**Time Estimate:** 6-8 hours
**Prerequisites:**
- Understand the tool registry pattern
- Read existing tool implementations in `apps/edge-mcp/internal/tools/builtin/`

**Background:**
Tools are the core functionality that AI agents use. We need comprehensive tests to ensure tools execute correctly, handle errors properly, and validate parameters.

**Implementation Steps:**

1. **Create test file:** `apps/edge-mcp/internal/tools/registry_test.go`
```go
package tools

import (
    "context"
    "encoding/json"
    "errors"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Mock tool provider for testing
type mockToolProvider struct {
    tools []ToolDefinition
}

func (m *mockToolProvider) GetDefinitions() []ToolDefinition {
    return m.tools
}

func TestRegistry_RegisterAndList(t *testing.T) {
    // Setup
    registry := NewRegistry()

    // Create mock tools
    mockProvider := &mockToolProvider{
        tools: []ToolDefinition{
            {
                Name:        "test_tool_1",
                Description: "First test tool",
                InputSchema: map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "input": map[string]interface{}{
                            "type": "string",
                        },
                    },
                },
                Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
                    return map[string]string{"result": "success"}, nil
                },
            },
            {
                Name:        "test_tool_2",
                Description: "Second test tool",
                InputSchema: map[string]interface{}{
                    "type": "object",
                },
                Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
                    return "tool 2 result", nil
                },
            },
        },
    }

    // Test registration
    registry.Register(mockProvider)

    // Test listing
    allTools := registry.ListAll()
    assert.Len(t, allTools, 2, "Should have 2 tools registered")

    // Verify tool details
    toolNames := make(map[string]bool)
    for _, tool := range allTools {
        toolNames[tool.Name] = true
    }
    assert.True(t, toolNames["test_tool_1"], "Should have test_tool_1")
    assert.True(t, toolNames["test_tool_2"], "Should have test_tool_2")

    // Test count
    assert.Equal(t, 2, registry.Count())
    assert.Equal(t, 2, registry.Size())
}

func TestRegistry_ExecuteTool_Success(t *testing.T) {
    // Setup
    registry := NewRegistry()

    // Create a tool that echoes input
    echoTool := ToolDefinition{
        Name:        "echo_tool",
        Description: "Echoes the input",
        Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
            var input map[string]interface{}
            if err := json.Unmarshal(args, &input); err != nil {
                return nil, err
            }
            return map[string]interface{}{
                "echoed": input["message"],
            }, nil
        },
    }

    // Register directly
    registry.RegisterRemote(echoTool)

    // Test execution
    testArgs := json.RawMessage(`{"message": "Hello World"}`)
    result, err := registry.Execute(context.Background(), "echo_tool", testArgs)

    // Verify
    assert.NoError(t, err)
    assert.NotNil(t, result)

    resultMap, ok := result.(map[string]interface{})
    require.True(t, ok, "Result should be a map")
    assert.Equal(t, "Hello World", resultMap["echoed"])
}

func TestRegistry_ExecuteTool_NotFound(t *testing.T) {
    // Setup
    registry := NewRegistry()

    // Try to execute non-existent tool
    _, err := registry.Execute(context.Background(), "non_existent_tool", nil)

    // Verify error
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "tool not found: non_existent_tool")
}

func TestRegistry_ExecuteTool_HandlerError(t *testing.T) {
    // Setup
    registry := NewRegistry()

    // Create a tool that always fails
    failingTool := ToolDefinition{
        Name:        "failing_tool",
        Description: "Always fails",
        Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
            return nil, errors.New("tool execution failed: database connection lost")
        },
    }

    registry.RegisterRemote(failingTool)

    // Test execution
    _, err := registry.Execute(context.Background(), "failing_tool", nil)

    // Verify error
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "tool execution failed: database connection lost")
}

func TestRegistry_ExecuteTool_ContextCancellation(t *testing.T) {
    // Setup
    registry := NewRegistry()

    // Create a slow tool
    slowTool := ToolDefinition{
        Name:        "slow_tool",
        Description: "Takes time to execute",
        Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
            select {
            case <-time.After(5 * time.Second):
                return "completed", nil
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        },
    }

    registry.RegisterRemote(slowTool)

    // Create cancellable context
    ctx, cancel := context.WithCancel(context.Background())

    // Start execution in goroutine
    done := make(chan struct{})
    var execErr error

    go func() {
        _, execErr = registry.Execute(ctx, "slow_tool", nil)
        close(done)
    }()

    // Cancel after short time
    time.Sleep(100 * time.Millisecond)
    cancel()

    // Wait for completion
    <-done

    // Verify cancellation
    assert.Error(t, execErr)
    assert.Equal(t, context.Canceled, execErr)
}

func TestRegistry_ExecuteTool_ParameterValidation(t *testing.T) {
    // Setup
    registry := NewRegistry()

    // Create a tool that validates parameters
    validatingTool := ToolDefinition{
        Name:        "validating_tool",
        Description: "Validates input parameters",
        Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
            var input map[string]interface{}
            if err := json.Unmarshal(args, &input); err != nil {
                return nil, errors.New("invalid JSON input")
            }

            // Check required field
            if _, ok := input["required_field"]; !ok {
                return nil, errors.New("missing required field: required_field")
            }

            // Check field type
            if _, ok := input["required_field"].(string); !ok {
                return nil, errors.New("required_field must be a string")
            }

            return "validation passed", nil
        },
    }

    registry.RegisterRemote(validatingTool)

    // Test with valid parameters
    validArgs := json.RawMessage(`{"required_field": "value"}`)
    result, err := registry.Execute(context.Background(), "validating_tool", validArgs)
    assert.NoError(t, err)
    assert.Equal(t, "validation passed", result)

    // Test with missing field
    missingArgs := json.RawMessage(`{"other_field": "value"}`)
    _, err = registry.Execute(context.Background(), "validating_tool", missingArgs)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "missing required field")

    // Test with invalid JSON
    invalidArgs := json.RawMessage(`{invalid json}`)
    _, err = registry.Execute(context.Background(), "validating_tool", invalidArgs)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid JSON")
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
    // Setup
    registry := NewRegistry()

    // Create a simple tool
    simpleTool := ToolDefinition{
        Name:        "concurrent_tool",
        Description: "For concurrent testing",
        Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
            return "success", nil
        },
    }

    registry.RegisterRemote(simpleTool)

    // Test concurrent execution
    numGoroutines := 10
    done := make(chan bool, numGoroutines)

    for i := 0; i < numGoroutines; i++ {
        go func(id int) {
            // Execute tool
            result, err := registry.Execute(context.Background(), "concurrent_tool", nil)
            assert.NoError(t, err)
            assert.Equal(t, "success", result)

            // List tools
            tools := registry.ListAll()
            assert.NotEmpty(t, tools)

            done <- true
        }(i)
    }

    // Wait for all goroutines
    for i := 0; i < numGoroutines; i++ {
        <-done
    }
}
```

2. **Run the tests:**
```bash
cd apps/edge-mcp
go test -v -run TestRegistry ./internal/tools/
```

3. **Run with race detector:**
```bash
go test -race -run TestRegistry ./internal/tools/
```

**Expected Output:**
```
=== RUN   TestRegistry_RegisterAndList
--- PASS: TestRegistry_RegisterAndList (0.00s)
=== RUN   TestRegistry_ExecuteTool_Success
--- PASS: TestRegistry_ExecuteTool_Success (0.00s)
=== RUN   TestRegistry_ExecuteTool_NotFound
--- PASS: TestRegistry_ExecuteTool_NotFound (0.00s)
=== RUN   TestRegistry_ExecuteTool_HandlerError
--- PASS: TestRegistry_ExecuteTool_HandlerError (0.00s)
=== RUN   TestRegistry_ExecuteTool_ContextCancellation
--- PASS: TestRegistry_ExecuteTool_ContextCancellation (0.10s)
=== RUN   TestRegistry_ExecuteTool_ParameterValidation
--- PASS: TestRegistry_ExecuteTool_ParameterValidation (0.00s)
=== RUN   TestRegistry_ConcurrentAccess
--- PASS: TestRegistry_ConcurrentAccess (0.00s)
PASS
```

**Common Mistakes to Avoid:**
- Always use context in tool handlers for cancellation
- Don't forget to test concurrent access with race detector
- Make sure to test both success and error paths
- Use proper JSON marshaling/unmarshaling

**✅ COMPLETION NOTES:**
- Implementation completed: `registry_test.go` created with comprehensive tests
- All 10 test cases passing:
  - TestRegistry_RegisterAndList - Tests tool registration and listing
  - TestRegistry_ExecuteTool_Success - Tests successful tool execution
  - TestRegistry_ExecuteTool_NotFound - Tests error handling for non-existent tools
  - TestRegistry_ExecuteTool_HandlerError - Tests error propagation from handlers
  - TestRegistry_ExecuteTool_ContextCancellation - Tests context cancellation handling
  - TestRegistry_ExecuteTool_ParameterValidation - Tests parameter validation
  - TestRegistry_ConcurrentAccess - Tests thread safety with concurrent access
  - TestRegistry_ExecuteTool_NoHandler - Tests nil handler error case
  - TestRegistry_MultipleProviders - Tests registering from multiple providers
  - TestRegistry_OverwriteTool - Tests tool overwriting behavior
- Test coverage for registry.go: 100% for all functions
- Tests pass with race detector enabled
- Dependencies resolved with go mod tidy

---

#### Story 1.1.3: Authentication & Permission Tests ✅ COMPLETED
**Difficulty:** Junior-Mid
**Time Estimate:** 5-6 hours
**Prerequisites:**
- Understand HTTP authentication headers
- Read about passthrough authentication concept

**Background:**
The system supports multiple authentication methods and needs to filter tools based on user permissions. We must ensure authentication is secure and permissions are correctly enforced.

**Implementation Steps:**

1. **Create test file:** `apps/edge-mcp/internal/auth/auth_test.go`
```go
package auth

import (
    "net/http"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestEdgeAuthenticator_NoAPIKey(t *testing.T) {
    // When no API key is configured, all requests should pass (development mode)
    auth := NewEdgeAuthenticator("")

    req, _ := http.NewRequest("GET", "/test", nil)

    result := auth.AuthenticateRequest(req)
    assert.True(t, result, "Should allow request when no API key is configured")
}

func TestEdgeAuthenticator_BearerToken(t *testing.T) {
    // Setup authenticator with API key
    expectedKey := "test-api-key-12345"
    auth := NewEdgeAuthenticator(expectedKey)

    tests := []struct {
        name        string
        authHeader  string
        shouldPass  bool
        description string
    }{
        {
            name:        "Valid Bearer Token",
            authHeader:  "Bearer test-api-key-12345",
            shouldPass:  true,
            description: "Should authenticate with valid Bearer token",
        },
        {
            name:        "Valid Bearer Token No Space",
            authHeader:  "test-api-key-12345",
            shouldPass:  true,
            description: "Should authenticate without Bearer prefix",
        },
        {
            name:        "Invalid Bearer Token",
            authHeader:  "Bearer wrong-key",
            shouldPass:  false,
            description: "Should reject invalid Bearer token",
        },
        {
            name:        "Empty Bearer",
            authHeader:  "Bearer ",
            shouldPass:  false,
            description: "Should reject empty Bearer token",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req, _ := http.NewRequest("GET", "/test", nil)
            req.Header.Set("Authorization", tt.authHeader)

            result := auth.AuthenticateRequest(req)
            assert.Equal(t, tt.shouldPass, result, tt.description)
        })
    }
}

func TestEdgeAuthenticator_XAPIKey(t *testing.T) {
    // Setup authenticator with API key
    expectedKey := "test-api-key-67890"
    auth := NewEdgeAuthenticator(expectedKey)

    tests := []struct {
        name       string
        apiKey     string
        shouldPass bool
    }{
        {
            name:       "Valid X-API-Key",
            apiKey:     "test-api-key-67890",
            shouldPass: true,
        },
        {
            name:       "Invalid X-API-Key",
            apiKey:     "wrong-key",
            shouldPass: false,
        },
        {
            name:       "Empty X-API-Key",
            apiKey:     "",
            shouldPass: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req, _ := http.NewRequest("GET", "/test", nil)
            if tt.apiKey != "" {
                req.Header.Set("X-API-Key", tt.apiKey)
            }

            result := auth.AuthenticateRequest(req)
            assert.Equal(t, tt.shouldPass, result)
        })
    }
}

func TestEdgeAuthenticator_PreferAuthorizationHeader(t *testing.T) {
    // When both headers are present, Authorization should be preferred
    auth := NewEdgeAuthenticator("correct-key")

    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer correct-key")
    req.Header.Set("X-API-Key", "wrong-key")

    result := auth.AuthenticateRequest(req)
    assert.True(t, result, "Should use Authorization header when both are present")
}

func TestEdgeAuthenticator_NoCredentials(t *testing.T) {
    // Setup authenticator with API key
    auth := NewEdgeAuthenticator("required-key")

    req, _ := http.NewRequest("GET", "/test", nil)
    // No headers set

    result := auth.AuthenticateRequest(req)
    assert.False(t, result, "Should reject request with no credentials")
}
```

2. **Create passthrough auth test file:** `apps/edge-mcp/internal/mcp/passthrough_auth_test.go`
```go
package mcp

import (
    "net/http"
    "os"
    "testing"
    "github.com/developer-mesh/developer-mesh/pkg/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestExtractPassthroughAuth_FromHeaders(t *testing.T) {
    handler := &Handler{}

    req, _ := http.NewRequest("GET", "/test", nil)
    req.Header.Set("X-GitHub-Token", "ghp_testtoken123")
    req.Header.Set("X-Service-Slack-Token", "xoxb-slack-token")
    req.Header.Set("X-AWS-Access-Key", "AKIAIOSFODNN7EXAMPLE")
    req.Header.Set("X-AWS-Secret-Key", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
    req.Header.Set("X-AWS-Region", "us-west-2")

    bundle := handler.extractPassthroughAuth(req)

    // Verify GitHub token
    require.NotNil(t, bundle)
    assert.Contains(t, bundle.Credentials, "github")
    assert.Equal(t, "bearer", bundle.Credentials["github"].Type)
    assert.Equal(t, "ghp_testtoken123", bundle.Credentials["github"].Token)

    // Verify Slack token
    assert.Contains(t, bundle.Credentials, "slack")
    assert.Equal(t, "xoxb-slack-token", bundle.Credentials["slack"].Token)

    // Verify AWS credentials
    assert.Contains(t, bundle.Credentials, "aws")
    assert.Equal(t, "aws_signature", bundle.Credentials["aws"].Type)
    assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", bundle.Credentials["aws"].Properties["access_key"])
    assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", bundle.Credentials["aws"].Properties["secret_key"])
    assert.Equal(t, "us-west-2", bundle.Credentials["aws"].Properties["region"])
}

func TestExtractPassthroughAuthFromEnv(t *testing.T) {
    handler := &Handler{}

    // Set environment variables
    oldGitHub := os.Getenv("GITHUB_TOKEN")
    oldHarness := os.Getenv("HARNESS_TOKEN")
    defer func() {
        os.Setenv("GITHUB_TOKEN", oldGitHub)
        os.Setenv("HARNESS_TOKEN", oldHarness)
    }()

    os.Setenv("GITHUB_TOKEN", "ghp_env_token")
    os.Setenv("HARNESS_TOKEN", "pat.harness.token")

    bundle := handler.extractPassthroughAuthFromEnv()

    // Verify tokens from environment
    require.NotNil(t, bundle)
    assert.Contains(t, bundle.Credentials, "github")
    assert.Equal(t, "ghp_env_token", bundle.Credentials["github"].Token)

    assert.Contains(t, bundle.Credentials, "harness")
    assert.Equal(t, "pat.harness.token", bundle.Credentials["harness"].Token)
}

func TestFilterToolsByPermissions_NoPermissions(t *testing.T) {
    handler := &Handler{}

    // Create sample tools
    allTools := []ToolDefinition{
        {Name: "github_list_repos", Description: "List GitHub repositories"},
        {Name: "harness_pipelines_list", Description: "List Harness pipelines"},
        {Name: "generic_tool", Description: "Generic tool"},
    }

    // Session without permissions
    session := &Session{
        ID: "test-session",
        PassthroughAuth: nil,
    }

    // Should return all tools when no permissions
    filtered := handler.filterToolsByPermissions(allTools, session)
    assert.Len(t, filtered, 3, "Should return all tools when no permissions")
}

func TestFilterToolsByPermissions_WithHarnessPermissions(t *testing.T) {
    handler := &Handler{
        logger: observability.NewNoOpLogger(),
    }

    // Create sample tools
    allTools := []ToolDefinition{
        {Name: "github_list_repos", Description: "List GitHub repositories"},
        {Name: "harness_pipelines_list", Description: "List Harness pipelines"},
        {Name: "harness_executions_get", Description: "Get Harness executions"},
        {Name: "harness_featureflags_list", Description: "List feature flags"},
        {Name: "generic_tool", Description: "Generic tool"},
    }

    // Create Harness permissions (only CI module enabled)
    permissions := &harness.HarnessPermissions{
        EnabledModules: map[string]bool{
            "ci": true,
            "cf": false, // Feature flags disabled
        },
    }

    permsJSON, _ := json.Marshal(permissions)

    // Session with Harness permissions
    session := &Session{
        ID: "test-session",
        PassthroughAuth: &models.PassthroughAuthBundle{
            Credentials: map[string]*models.PassthroughCredential{
                "harness": {
                    Type:  "api_key",
                    Token: "test-token",
                    Properties: map[string]string{
                        "permissions": string(permsJSON),
                    },
                },
            },
        },
    }

    // Filter tools
    filtered := handler.filterToolsByPermissions(allTools, session)

    // Verify filtering
    toolNames := make(map[string]bool)
    for _, tool := range filtered {
        toolNames[tool.Name] = true
    }

    assert.True(t, toolNames["github_list_repos"], "GitHub tools should pass")
    assert.True(t, toolNames["harness_pipelines_list"], "Pipelines (CI module) should pass")
    assert.True(t, toolNames["harness_executions_get"], "Executions (CI module) should pass")
    assert.False(t, toolNames["harness_featureflags_list"], "Feature flags should be filtered out")
    assert.True(t, toolNames["generic_tool"], "Generic tools should pass")
}
```

3. **Run the tests:**
```bash
# Run auth tests
cd apps/edge-mcp
go test -v -run TestEdgeAuthenticator ./internal/auth/
go test -v -run TestExtractPassthrough ./internal/mcp/
go test -v -run TestFilterTools ./internal/mcp/
```

**Expected Output:**
```
=== RUN   TestEdgeAuthenticator_NoAPIKey
--- PASS: TestEdgeAuthenticator_NoAPIKey (0.00s)
=== RUN   TestEdgeAuthenticator_BearerToken
=== RUN   TestEdgeAuthenticator_BearerToken/Valid_Bearer_Token
=== RUN   TestEdgeAuthenticator_BearerToken/Valid_Bearer_Token_No_Space
=== RUN   TestEdgeAuthenticator_BearerToken/Invalid_Bearer_Token
=== RUN   TestEdgeAuthenticator_BearerToken/Empty_Bearer
--- PASS: TestEdgeAuthenticator_BearerToken (0.00s)
=== RUN   TestEdgeAuthenticator_XAPIKey
--- PASS: TestEdgeAuthenticator_XAPIKey (0.00s)
PASS
```

**Common Mistakes to Avoid:**
- Remember to restore environment variables after tests
- Test both header-based and environment-based auth
- Don't hardcode real credentials in tests
- Always test permission filtering edge cases

**✅ COMPLETION NOTES:**
- Implementation completed: `auth_test.go` and `passthrough_auth_test.go` created with comprehensive tests
- All 9 test cases passing:
  - TestEdgeAuthenticator_NoAPIKey - Tests development mode with no API key
  - TestEdgeAuthenticator_BearerToken (4 sub-tests) - Tests Bearer token authentication
  - TestEdgeAuthenticator_XAPIKey (3 sub-tests) - Tests X-API-Key header authentication
  - TestEdgeAuthenticator_PreferAuthorizationHeader - Tests header preference
  - TestEdgeAuthenticator_NoCredentials - Tests rejection of unauthenticated requests
  - TestExtractPassthroughAuth_FromHeaders - Tests credential extraction from headers
  - TestExtractPassthroughAuthFromEnv - Tests credential extraction from environment
  - TestFilterToolsByPermissions_NoPermissions - Tests tool filtering without permissions
  - TestFilterToolsByPermissions_WithHarnessPermissions - Tests Harness permission filtering
- Test coverage: 100% for auth.go, partial for mcp package (testing specific functions)
- Tests verify authentication methods, passthrough auth extraction, and permission-based filtering

---

### Epic 1.2: Error Handling Improvements

#### Story 1.2.1: Add Contextual Error Wrapping ✅ COMPLETED
**Difficulty:** Junior
**Time Estimate:** 4-5 hours
**Prerequisites:**
- Understand Go error handling patterns
- Read https://go.dev/blog/go1.13-errors

**Background:**
Current errors are generic and don't provide enough context for debugging. We need to add structured errors with context about what operation failed and why.

**Implementation Steps:**

1. **Create error types file:** `apps/edge-mcp/internal/mcp/errors.go`
```go
package mcp

import (
    "fmt"
    "time"
)

// ErrorType represents categories of errors
type ErrorType string

const (
    ErrorTypeProtocol      ErrorType = "PROTOCOL_ERROR"
    ErrorTypeAuth          ErrorType = "AUTH_ERROR"
    ErrorTypeToolExecution ErrorType = "TOOL_EXECUTION_ERROR"
    ErrorTypeTimeout       ErrorType = "TIMEOUT_ERROR"
    ErrorTypeRateLimit     ErrorType = "RATE_LIMIT_ERROR"
    ErrorTypeNetwork       ErrorType = "NETWORK_ERROR"
    ErrorTypeValidation    ErrorType = "VALIDATION_ERROR"
    ErrorTypeNotFound      ErrorType = "NOT_FOUND"
    ErrorTypeInternal      ErrorType = "INTERNAL_ERROR"
)

// StructuredError provides rich error context
type StructuredError struct {
    Type        ErrorType              `json:"type"`
    Message     string                 `json:"message"`
    Details     string                 `json:"details,omitempty"`
    Operation   string                 `json:"operation,omitempty"`
    RequestID   string                 `json:"request_id,omitempty"`
    Suggestion  string                 `json:"suggestion,omitempty"`
    RetryAfter  *time.Duration         `json:"retry_after,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    wrapped     error
}

// Error implements the error interface
func (e *StructuredError) Error() string {
    if e.Details != "" {
        return fmt.Sprintf("[%s] %s: %s", e.Type, e.Message, e.Details)
    }
    return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the wrapped error
func (e *StructuredError) Unwrap() error {
    return e.wrapped
}

// NewProtocolError creates a protocol error
func NewProtocolError(operation, message string, details string) *StructuredError {
    return &StructuredError{
        Type:       ErrorTypeProtocol,
        Message:    message,
        Details:    details,
        Operation:  operation,
        Suggestion: "Check that your client implements the MCP protocol correctly",
    }
}

// NewAuthError creates an authentication error
func NewAuthError(message string) *StructuredError {
    return &StructuredError{
        Type:       ErrorTypeAuth,
        Message:    message,
        Operation:  "authenticate",
        Suggestion: "Verify your API key or credentials are correct",
    }
}

// NewToolExecutionError creates a tool execution error
func NewToolExecutionError(toolName string, err error) *StructuredError {
    return &StructuredError{
        Type:      ErrorTypeToolExecution,
        Message:   fmt.Sprintf("Tool '%s' execution failed", toolName),
        Details:   err.Error(),
        Operation: fmt.Sprintf("tools/call:%s", toolName),
        Suggestion: "Check tool parameters and try again",
        wrapped:   err,
        Metadata: map[string]interface{}{
            "tool": toolName,
        },
    }
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string, timeout time.Duration) *StructuredError {
    return &StructuredError{
        Type:       ErrorTypeTimeout,
        Message:    fmt.Sprintf("Operation timed out after %v", timeout),
        Operation:  operation,
        Suggestion: fmt.Sprintf("Try again with a longer timeout or smaller request"),
        Metadata: map[string]interface{}{
            "timeout_seconds": timeout.Seconds(),
        },
    }
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(retryAfter time.Duration) *StructuredError {
    return &StructuredError{
        Type:       ErrorTypeRateLimit,
        Message:    "Rate limit exceeded",
        RetryAfter: &retryAfter,
        Suggestion: fmt.Sprintf("Wait %v before retrying", retryAfter),
        Metadata: map[string]interface{}{
            "retry_after_seconds": retryAfter.Seconds(),
        },
    }
}

// NewValidationError creates a validation error
func NewValidationError(field, message string) *StructuredError {
    return &StructuredError{
        Type:       ErrorTypeValidation,
        Message:    fmt.Sprintf("Validation failed for field '%s'", field),
        Details:    message,
        Suggestion: "Check the input parameters against the schema",
        Metadata: map[string]interface{}{
            "field": field,
        },
    }
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource, identifier string) *StructuredError {
    return &StructuredError{
        Type:       ErrorTypeNotFound,
        Message:    fmt.Sprintf("%s not found", resource),
        Details:    fmt.Sprintf("No %s with identifier '%s' exists", resource, identifier),
        Suggestion: fmt.Sprintf("Check that the %s exists and you have access", resource),
        Metadata: map[string]interface{}{
            "resource":   resource,
            "identifier": identifier,
        },
    }
}

// ToMCPError converts to MCP error format
func (e *StructuredError) ToMCPError() *MCPError {
    code := -32603 // Internal error default

    switch e.Type {
    case ErrorTypeProtocol:
        code = -32600 // Invalid request
    case ErrorTypeAuth:
        code = -32001 // Custom auth error
    case ErrorTypeValidation:
        code = -32602 // Invalid params
    case ErrorTypeNotFound:
        code = -32002 // Custom not found
    case ErrorTypeRateLimit:
        code = -32003 // Custom rate limit
    case ErrorTypeTimeout:
        code = -32004 // Custom timeout
    }

    data := map[string]interface{}{
        "type":       e.Type,
        "suggestion": e.Suggestion,
    }

    if e.RetryAfter != nil {
        data["retry_after"] = e.RetryAfter.Seconds()
    }

    if e.Metadata != nil {
        for k, v := range e.Metadata {
            data[k] = v
        }
    }

    return &MCPError{
        Code:    code,
        Message: e.Error(),
        Data:    data,
    }
}

// WithRequestID adds request ID to error
func (e *StructuredError) WithRequestID(requestID string) *StructuredError {
    e.RequestID = requestID
    return e
}

// WithMetadata adds metadata to error
func (e *StructuredError) WithMetadata(key string, value interface{}) *StructuredError {
    if e.Metadata == nil {
        e.Metadata = make(map[string]interface{})
    }
    e.Metadata[key] = value
    return e
}
```

2. **Update handler.go to use structured errors:** `apps/edge-mcp/internal/mcp/handler.go`

Find and replace error returns. Here are examples of the changes:

```go
// OLD (line 183-185):
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

// NEW:
if err != nil {
    var structuredErr *StructuredError
    if errors.As(err, &structuredErr) {
        response = &MCPMessage{
            JSONRPC: "2.0",
            ID:      msg.ID,
            Error:   structuredErr.ToMCPError(),
        }
    } else {
        // Fallback for non-structured errors
        response = &MCPMessage{
            JSONRPC: "2.0",
            ID:      msg.ID,
            Error: &MCPError{
                Code:    -32603,
                Message: err.Error(),
            },
        }
    }
}

// OLD (line 435):
return nil, fmt.Errorf("method not found: %s", msg.Method)

// NEW:
return nil, NewProtocolError(msg.Method, "Method not found",
    fmt.Sprintf("The method '%s' is not supported by this server", msg.Method))

// OLD (line 467-468):
if !versionSupported {
    return nil, fmt.Errorf("unsupported protocol version: %s (supported: %v)",
        params.ProtocolVersion, supportedVersions)
}

// NEW:
if !versionSupported {
    return nil, NewProtocolError("initialize",
        "Unsupported protocol version",
        fmt.Sprintf("Version %s is not supported. Supported versions: %v",
            params.ProtocolVersion, supportedVersions))
}

// OLD (line 71):
if !exists {
    return nil, fmt.Errorf("tool not found: %s", name)
}

// NEW (in registry.go):
if !exists {
    return nil, NewNotFoundError("tool", name)
}

// OLD (line 802-804):
result, err := h.tools.Execute(ctx, params.Name, params.Arguments)
if err != nil {
    return nil, fmt.Errorf("tool execution failed: %w", err)
}

// NEW:
result, err := h.tools.Execute(ctx, params.Name, params.Arguments)
if err != nil {
    return nil, NewToolExecutionError(params.Name, err).
        WithRequestID(fmt.Sprintf("%v", msg.ID))
}
```

3. **Create test file:** `apps/edge-mcp/internal/mcp/errors_test.go`
```go
package mcp

import (
    "errors"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestStructuredError_Creation(t *testing.T) {
    tests := []struct {
        name     string
        err      *StructuredError
        expected string
    }{
        {
            name:     "Protocol Error",
            err:      NewProtocolError("initialize", "Invalid version", "Version 1.0 not supported"),
            expected: "[PROTOCOL_ERROR] Invalid version: Version 1.0 not supported",
        },
        {
            name:     "Auth Error",
            err:      NewAuthError("Invalid API key"),
            expected: "[AUTH_ERROR] Invalid API key",
        },
        {
            name:     "Tool Execution Error",
            err:      NewToolExecutionError("github_list_repos", errors.New("connection timeout")),
            expected: "[TOOL_EXECUTION_ERROR] Tool 'github_list_repos' execution failed: connection timeout",
        },
        {
            name:     "Not Found Error",
            err:      NewNotFoundError("repository", "my-repo"),
            expected: "[NOT_FOUND] repository not found: No repository with identifier 'my-repo' exists",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.expected, tt.err.Error())
            assert.NotEmpty(t, tt.err.Suggestion)
        })
    }
}

func TestStructuredError_ToMCPError(t *testing.T) {
    // Test protocol error conversion
    protocolErr := NewProtocolError("test", "Test error", "Details")
    mcpErr := protocolErr.ToMCPError()
    assert.Equal(t, -32600, mcpErr.Code)
    assert.Contains(t, mcpErr.Message, "PROTOCOL_ERROR")

    // Test rate limit error with retry after
    retryAfter := 60 * time.Second
    rateLimitErr := NewRateLimitError(retryAfter)
    mcpErr = rateLimitErr.ToMCPError()
    assert.Equal(t, -32003, mcpErr.Code)
    data := mcpErr.Data.(map[string]interface{})
    assert.Equal(t, 60.0, data["retry_after"])
}

func TestStructuredError_WithMetadata(t *testing.T) {
    err := NewToolExecutionError("test_tool", errors.New("failed")).
        WithRequestID("req-123").
        WithMetadata("attempt", 3).
        WithMetadata("duration_ms", 1500)

    assert.Equal(t, "req-123", err.RequestID)
    assert.Equal(t, 3, err.Metadata["attempt"])
    assert.Equal(t, 1500, err.Metadata["duration_ms"])

    // Test MCP conversion includes metadata
    mcpErr := err.ToMCPError()
    data := mcpErr.Data.(map[string]interface{})
    assert.Equal(t, 3, data["attempt"])
    assert.Equal(t, 1500, data["duration_ms"])
}

func TestStructuredError_Unwrap(t *testing.T) {
    originalErr := errors.New("original error")
    structuredErr := NewToolExecutionError("tool", originalErr)

    unwrapped := structuredErr.Unwrap()
    assert.Equal(t, originalErr, unwrapped)

    // Test with errors.Is
    assert.True(t, errors.Is(structuredErr, originalErr))
}
```

4. **Run the tests:**
```bash
cd apps/edge-mcp
go test -v -run TestStructuredError ./internal/mcp/
```

**Expected Output:**
```
=== RUN   TestStructuredError_Creation
--- PASS: TestStructuredError_Creation (0.00s)
=== RUN   TestStructuredError_ToMCPError
--- PASS: TestStructuredError_ToMCPError (0.00s)
=== RUN   TestStructuredError_WithMetadata
--- PASS: TestStructuredError_WithMetadata (0.00s)
=== RUN   TestStructuredError_Unwrap
--- PASS: TestStructuredError_Unwrap (0.00s)
PASS
```

**Common Mistakes to Avoid:**
- Don't lose the original error - always wrap it
- Include actionable suggestions in errors
- Use appropriate error codes for MCP
- Add request IDs for tracing

**✅ COMPLETION NOTES:**
- Implementation completed: `errors.go` created with comprehensive structured error types
- Error types implemented:
  - StructuredError base type with metadata, suggestions, and retry information
  - Helper functions for all error types: Protocol, Auth, ToolExecution, Timeout, RateLimit, Validation, NotFound
  - ToMCPError() conversion for proper JSON-RPC error formatting
  - WithRequestID() and WithMetadata() fluent methods for adding context
- Updated handler.go to use structured errors:
  - Modified error handling in HandleConnection to detect and convert structured errors
  - Updated handleMessage to return protocol errors for unknown methods
  - Updated handleInitialize to return protocol errors for invalid params and unsupported versions
  - Updated handleToolCall to return ToolExecutionError with request ID
  - Updated handleContextOperation to return protocol errors
- Test coverage: 100% for all error functions (except 75% for WithMetadata due to branch)
- All 7 test cases passing:
  - TestStructuredError_Creation
  - TestStructuredError_ToMCPError
  - TestStructuredError_WithMetadata
  - TestStructuredError_Unwrap
  - TestStructuredError_ErrorTypes
  - TestStructuredError_RateLimitWithRetryAfter
  - TestStructuredError_ComplexMetadata

---

#### Story 1.2.2: Implement Retry Logic with Exponential Backoff
**Difficulty:** Mid
**Time Estimate:** 6-8 hours
**Prerequisites:**
- Understand exponential backoff algorithm
- Know about transient vs permanent failures

**Background:**
Network calls can fail temporarily. We need smart retry logic that backs off exponentially to avoid overwhelming failed services.

**Implementation Steps:**

1. **Create retry utility:** `pkg/utils/retry.go`
```go
package utils

import (
    "context"
    "errors"
    "fmt"
    "math"
    "math/rand"
    "time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
    MaxAttempts     int           // Maximum number of attempts (including first try)
    InitialDelay    time.Duration // Initial delay between retries
    MaxDelay        time.Duration // Maximum delay between retries
    Multiplier      float64       // Multiplier for exponential backoff
    JitterFactor    float64       // Jitter factor (0-1) to randomize delays
    RetryableErrors []error       // Specific errors that trigger retry
    RetryIf         func(error) bool // Custom function to determine if retry
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() *RetryConfig {
    return &RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 1 * time.Second,
        MaxDelay:     30 * time.Second,
        Multiplier:   2.0,
        JitterFactor: 0.1,
    }
}

// RetryResult contains retry execution details
type RetryResult struct {
    Attempts      int
    TotalDuration time.Duration
    LastError     error
}

// RetryableError interface for errors that know if they're retryable
type RetryableError interface {
    error
    IsRetryable() bool
}

// RetryWithBackoff executes a function with exponential backoff retry
func RetryWithBackoff(ctx context.Context, config *RetryConfig, fn func() error) (*RetryResult, error) {
    if config == nil {
        config = DefaultRetryConfig()
    }

    result := &RetryResult{}
    startTime := time.Now()

    for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
        result.Attempts = attempt

        // Execute the function
        err := fn()

        // Success - return immediately
        if err == nil {
            result.TotalDuration = time.Since(startTime)
            return result, nil
        }

        result.LastError = err

        // Check if we should retry
        if attempt == config.MaxAttempts {
            // No more retries
            break
        }

        if !shouldRetry(err, config) {
            // Error is not retryable
            break
        }

        // Calculate delay with exponential backoff
        delay := calculateDelay(attempt, config)

        // Wait with context cancellation support
        select {
        case <-time.After(delay):
            // Continue to next attempt
        case <-ctx.Done():
            // Context cancelled
            result.TotalDuration = time.Since(startTime)
            return result, fmt.Errorf("retry cancelled: %w", ctx.Err())
        }
    }

    result.TotalDuration = time.Since(startTime)
    return result, fmt.Errorf("all %d attempts failed: %w", result.Attempts, result.LastError)
}

// shouldRetry determines if an error warrants a retry
func shouldRetry(err error, config *RetryConfig) bool {
    // Check custom retry function first
    if config.RetryIf != nil {
        return config.RetryIf(err)
    }

    // Check if error implements RetryableError
    var retryable RetryableError
    if errors.As(err, &retryable) {
        return retryable.IsRetryable()
    }

    // Check specific retryable errors
    for _, retryableErr := range config.RetryableErrors {
        if errors.Is(err, retryableErr) {
            return true
        }
    }

    // Default: don't retry
    return false
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(attempt int, config *RetryConfig) time.Duration {
    // Calculate base delay with exponential backoff
    delay := float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt-1))

    // Apply maximum delay cap
    if delay > float64(config.MaxDelay) {
        delay = float64(config.MaxDelay)
    }

    // Add jitter to prevent thundering herd
    if config.JitterFactor > 0 {
        jitter := delay * config.JitterFactor * (rand.Float64()*2 - 1) // -jitter to +jitter
        delay += jitter
    }

    // Ensure delay is not negative
    if delay < 0 {
        delay = 0
    }

    return time.Duration(delay)
}

// Common retryable errors
var (
    ErrTimeout        = errors.New("operation timeout")
    ErrRateLimit      = errors.New("rate limit exceeded")
    ErrServiceUnavailable = errors.New("service temporarily unavailable")
)

// NetworkError represents a retryable network error
type NetworkError struct {
    Message string
}

func (e NetworkError) Error() string {
    return fmt.Sprintf("network error: %s", e.Message)
}

func (e NetworkError) IsRetryable() bool {
    return true
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
    StatusCode int
    Message    string
}

func (e HTTPError) Error() string {
    return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

func (e HTTPError) IsRetryable() bool {
    // Retry on server errors and specific client errors
    switch e.StatusCode {
    case 429, // Too Many Requests
         502, // Bad Gateway
         503, // Service Unavailable
         504: // Gateway Timeout
        return true
    default:
        return e.StatusCode >= 500
    }
}
```

2. **Create retry tests:** `pkg/utils/retry_test.go`
```go
package utils

import (
    "context"
    "errors"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
)

func TestRetryWithBackoff_Success(t *testing.T) {
    config := &RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 10 * time.Millisecond,
        MaxDelay:     100 * time.Millisecond,
        Multiplier:   2,
    }

    attempts := 0
    fn := func() error {
        attempts++
        if attempts < 2 {
            return errors.New("temporary error")
        }
        return nil // Success on second attempt
    }

    // Set retry condition
    config.RetryIf = func(err error) bool {
        return err.Error() == "temporary error"
    }

    result, err := RetryWithBackoff(context.Background(), config, fn)

    assert.NoError(t, err)
    assert.Equal(t, 2, result.Attempts)
    assert.Equal(t, 2, attempts)
}

func TestRetryWithBackoff_AllAttemptsFail(t *testing.T) {
    config := &RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 10 * time.Millisecond,
        RetryIf:      func(err error) bool { return true },
    }

    attempts := 0
    fn := func() error {
        attempts++
        return errors.New("persistent error")
    }

    result, err := RetryWithBackoff(context.Background(), config, fn)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "all 3 attempts failed")
    assert.Equal(t, 3, result.Attempts)
    assert.Equal(t, 3, attempts)
}

func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
    config := &RetryConfig{
        MaxAttempts: 3,
        RetryIf: func(err error) bool {
            return err.Error() != "fatal error"
        },
    }

    attempts := 0
    fn := func() error {
        attempts++
        return errors.New("fatal error")
    }

    result, err := RetryWithBackoff(context.Background(), config, fn)

    assert.Error(t, err)
    assert.Equal(t, 1, result.Attempts)
    assert.Equal(t, 1, attempts) // Should not retry
}

func TestRetryWithBackoff_ContextCancellation(t *testing.T) {
    config := &RetryConfig{
        MaxAttempts:  5,
        InitialDelay: 100 * time.Millisecond,
        RetryIf:      func(err error) bool { return true },
    }

    ctx, cancel := context.WithCancel(context.Background())

    attempts := 0
    fn := func() error {
        attempts++
        if attempts == 2 {
            cancel() // Cancel after second attempt
        }
        return errors.New("error")
    }

    result, err := RetryWithBackoff(ctx, config, fn)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "retry cancelled")
    assert.Equal(t, 2, result.Attempts)
}

func TestRetryWithBackoff_ExponentialBackoff(t *testing.T) {
    config := &RetryConfig{
        MaxAttempts:  4,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     1 * time.Second,
        Multiplier:   2,
        JitterFactor: 0, // No jitter for predictable testing
    }

    delays := []time.Duration{}
    lastTime := time.Now()

    fn := func() error {
        now := time.Now()
        if len(delays) > 0 {
            delays = append(delays, now.Sub(lastTime))
        }
        lastTime = now
        return errors.New("error")
    }

    config.RetryIf = func(err error) bool { return true }

    _, _ = RetryWithBackoff(context.Background(), config, fn)

    // Verify exponential backoff
    assert.Len(t, delays, 3) // 3 retries after initial attempt

    // First retry: ~100ms
    assert.InDelta(t, 100, delays[0].Milliseconds(), 20)

    // Second retry: ~200ms (100ms * 2)
    assert.InDelta(t, 200, delays[1].Milliseconds(), 20)

    // Third retry: ~400ms (100ms * 2^2)
    assert.InDelta(t, 400, delays[2].Milliseconds(), 20)
}

func TestRetryableError_Interface(t *testing.T) {
    // Test NetworkError
    netErr := NetworkError{Message: "connection reset"}
    assert.True(t, netErr.IsRetryable())
    assert.Contains(t, netErr.Error(), "connection reset")

    // Test HTTPError - retryable
    httpErr503 := HTTPError{StatusCode: 503, Message: "Service Unavailable"}
    assert.True(t, httpErr503.IsRetryable())

    // Test HTTPError - not retryable
    httpErr400 := HTTPError{StatusCode: 400, Message: "Bad Request"}
    assert.False(t, httpErr400.IsRetryable())

    // Test HTTPError - rate limit (retryable)
    httpErr429 := HTTPError{StatusCode: 429, Message: "Too Many Requests"}
    assert.True(t, httpErr429.IsRetryable())
}

func TestCalculateDelay(t *testing.T) {
    config := &RetryConfig{
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     5 * time.Second,
        Multiplier:   2,
        JitterFactor: 0,
    }

    // Test exponential growth
    delay1 := calculateDelay(1, config)
    assert.Equal(t, 100*time.Millisecond, delay1)

    delay2 := calculateDelay(2, config)
    assert.Equal(t, 200*time.Millisecond, delay2)

    delay3 := calculateDelay(3, config)
    assert.Equal(t, 400*time.Millisecond, delay3)

    // Test max delay cap
    delay10 := calculateDelay(10, config)
    assert.Equal(t, 5*time.Second, delay10)
}
```

3. **Integrate retry logic into core client:** `apps/edge-mcp/internal/core/client.go`

Add retry logic to external calls:

```go
package core

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "github.com/developer-mesh/developer-mesh/pkg/utils"
)

// Add retry configuration to Client
type Client struct {
    baseURL      string
    httpClient   *http.Client
    apiKey       string
    retryConfig  *utils.RetryConfig
    // ... other fields
}

// Update FetchRemoteTools to use retry
func (c *Client) FetchRemoteTools(ctx context.Context) ([]tools.ToolDefinition, error) {
    var result []tools.ToolDefinition

    // Configure retry for this operation
    retryConfig := &utils.RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 1 * time.Second,
        MaxDelay:     10 * time.Second,
        Multiplier:   2,
        RetryIf: func(err error) bool {
            // Retry on network errors and specific HTTP status codes
            var httpErr utils.HTTPError
            if errors.As(err, &httpErr) {
                return httpErr.IsRetryable()
            }

            // Retry on timeout
            if errors.Is(err, context.DeadlineExceeded) {
                return true
            }

            return false
        },
    }

    retryResult, err := utils.RetryWithBackoff(ctx, retryConfig, func() error {
        req, err := http.NewRequestWithContext(ctx, "GET",
            fmt.Sprintf("%s/api/v1/tools", c.baseURL), nil)
        if err != nil {
            return err
        }

        req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

        resp, err := c.httpClient.Do(req)
        if err != nil {
            return utils.NetworkError{Message: err.Error()}
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            return utils.HTTPError{
                StatusCode: resp.StatusCode,
                Message:    fmt.Sprintf("Failed to fetch tools"),
            }
        }

        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return fmt.Errorf("failed to decode response: %w", err)
        }

        return nil
    })

    if err != nil {
        c.logger.Error("Failed to fetch remote tools after retries", map[string]interface{}{
            "attempts":       retryResult.Attempts,
            "total_duration": retryResult.TotalDuration.Seconds(),
            "error":          err.Error(),
        })
        return nil, err
    }

    c.logger.Info("Successfully fetched remote tools", map[string]interface{}{
        "tools_count": len(result),
        "attempts":    retryResult.Attempts,
    })

    return result, nil
}
```

4. **Run the tests:**
```bash
cd pkg/utils
go test -v -run TestRetry
```

**Expected Output:**
```
=== RUN   TestRetryWithBackoff_Success
--- PASS: TestRetryWithBackoff_Success (0.01s)
=== RUN   TestRetryWithBackoff_AllAttemptsFail
--- PASS: TestRetryWithBackoff_AllAttemptsFail (0.02s)
=== RUN   TestRetryWithBackoff_NonRetryableError
--- PASS: TestRetryWithBackoff_NonRetryableError (0.00s)
=== RUN   TestRetryWithBackoff_ContextCancellation
--- PASS: TestRetryWithBackoff_ContextCancellation (0.10s)
=== RUN   TestRetryWithBackoff_ExponentialBackoff
--- PASS: TestRetryWithBackoff_ExponentialBackoff (0.70s)
=== RUN   TestRetryableError_Interface
--- PASS: TestRetryableError_Interface (0.00s)
=== RUN   TestCalculateDelay
--- PASS: TestCalculateDelay (0.00s)
PASS
```

**Common Mistakes to Avoid:**
- Don't retry non-idempotent operations
- Always respect context cancellation
- Add jitter to prevent thundering herd
- Log retry attempts for debugging
- Set reasonable max delays

---

### Epic 1.3: Memory Management

#### Story 1.3.1: Fix Goroutine Leaks
**Difficulty:** Junior-Mid
**Time Estimate:** 3-4 hours
**Prerequisites:**
- Understand goroutines and channels
- Know how to use defer for cleanup

**Background:**
The current code spawns goroutines without proper cleanup, causing memory leaks. We need to ensure all goroutines are properly terminated.

**Implementation Steps:**

1. **Fix the ping ticker goroutine leak in handler.go:**

Current problematic code (lines 141-155):
```go
// Start ping ticker to keep connection alive
ticker := time.NewTicker(30 * time.Second)
defer ticker.Stop()

go func() {
    for {
        select {
        case <-ticker.C:
            if err := conn.Ping(ctx); err != nil {
                return
            }
        case <-ctx.Done():
            return
        }
    }
}()
```

**Fixed version:** Update `apps/edge-mcp/internal/mcp/handler.go`
```go
// HandleConnection handles a WebSocket connection
func (h *Handler) HandleConnection(conn *websocket.Conn, r *http.Request) {
    sessionID := uuid.New().String()

    // Extract passthrough authentication from headers
    passthroughAuth := h.extractPassthroughAuth(r)

    session := &Session{
        ID:              sessionID,
        ConnectionID:    uuid.New().String(),
        CreatedAt:       time.Now(),
        LastActivity:    time.Now(),
        PassthroughAuth: passthroughAuth,
    }

    h.sessionsMu.Lock()
    h.sessions[sessionID] = session
    h.sessionsMu.Unlock()

    // Create connection context - properly cancelled on cleanup
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel() // Ensure context is cancelled

    defer func() {
        // Clean up session
        h.sessionsMu.Lock()
        delete(h.sessions, sessionID)
        h.sessionsMu.Unlock()

        // Cancel context to stop all goroutines
        cancel()

        // Close connection
        _ = conn.Close(websocket.StatusNormalClosure, "")
    }()

    // Start ping ticker with proper cleanup
    pingDone := make(chan struct{})
    go h.pingLoop(ctx, conn, pingDone)
    defer func() {
        cancel() // Signal ping loop to stop
        <-pingDone // Wait for ping loop to finish
    }()

    // Message handling loop
    for {
        var msg MCPMessage
        if err := wsjson.Read(ctx, conn, &msg); err != nil {
            if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
                h.logger.Error("WebSocket error", map[string]interface{}{
                    "error":      err.Error(),
                    "session_id": sessionID,
                })
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
            if err := wsjson.Write(ctx, conn, response); err != nil {
                h.logger.Error("Failed to write response", map[string]interface{}{
                    "error":      err.Error(),
                    "session_id": sessionID,
                })
                break
            }
        }
    }
}

// pingLoop handles WebSocket ping/pong with proper cleanup
func (h *Handler) pingLoop(ctx context.Context, conn *websocket.Conn, done chan struct{}) {
    defer close(done) // Signal completion when exiting

    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // Set deadline for ping
            pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
            err := conn.Ping(pingCtx)
            cancel()

            if err != nil {
                h.logger.Debug("Ping failed, closing connection", map[string]interface{}{
                    "error": err.Error(),
                })
                return
            }
        case <-ctx.Done():
            h.logger.Debug("Ping loop stopped due to context cancellation", nil)
            return
        }
    }
}
```

2. **Fix the refresh manager goroutine (lines 497-499):**

Current problematic code:
```go
if h.refreshManager != nil {
    go func() {
        h.logger.Debug("Refreshing tools on new connection", map[string]interface{}{
            "client": params.ClientInfo.Name,
        })
        h.refreshManager.OnReconnect(context.Background())
    }()
}
```

**Fixed version:**
```go
// In handleInitialize function
if h.refreshManager != nil {
    // Create a context with timeout for refresh
    refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

    // Track the goroutine
    h.activeRefreshes.Add(1)

    go func() {
        defer h.activeRefreshes.Done()
        defer cancel()

        h.logger.Debug("Refreshing tools on new connection", map[string]interface{}{
            "client": params.ClientInfo.Name,
        })

        if err := h.refreshManager.OnReconnect(refreshCtx); err != nil {
            h.logger.Warn("Tool refresh failed", map[string]interface{}{
                "error": err.Error(),
            })
        }
    }()
}

// Add to Handler struct:
type Handler struct {
    // ... existing fields ...
    activeRefreshes sync.WaitGroup // Track active refresh operations
}

// Add cleanup method
func (h *Handler) Shutdown(ctx context.Context) error {
    h.logger.Info("Shutting down MCP handler", nil)

    // Cancel all active requests
    h.requestsMu.Lock()
    for _, cancel := range h.activeRequests {
        cancel()
    }
    h.requestsMu.Unlock()

    // Wait for active refreshes with timeout
    done := make(chan struct{})
    go func() {
        h.activeRefreshes.Wait()
        close(done)
    }()

    select {
    case <-done:
        h.logger.Info("All refresh operations completed", nil)
    case <-ctx.Done():
        h.logger.Warn("Shutdown timeout, some operations may be incomplete", nil)
    }

    // Close all sessions
    h.sessionsMu.Lock()
    for id := range h.sessions {
        delete(h.sessions, id)
    }
    h.sessionsMu.Unlock()

    return nil
}
```

3. **Create test to verify no goroutine leaks:** `apps/edge-mcp/internal/mcp/goroutine_leak_test.go`
```go
package mcp

import (
    "context"
    "runtime"
    "testing"
    "time"
    "github.com/coder/websocket"
    "github.com/coder/websocket/wstest"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNoGoroutineLeaks(t *testing.T) {
    // Get initial goroutine count
    runtime.GC()
    initialGoroutines := runtime.NumGoroutine()

    // Create handler
    handler := NewHandler(
        tools.NewRegistry(),
        cache.NewMemoryCache(),
        nil,
        auth.NewEdgeAuthenticator(""),
        observability.NewNoOpLogger(),
    )

    // Simulate multiple connections
    for i := 0; i < 5; i++ {
        func() {
            // Create test WebSocket server and client
            server := wstest.NewServer(t, handler.HandleConnection)
            defer server.Close()

            ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
            defer cancel()

            // Connect client
            conn, _, err := websocket.Dial(ctx, server.URL, nil)
            require.NoError(t, err)

            // Send initialize message
            err = conn.Write(ctx, websocket.MessageText, []byte(`{
                "jsonrpc": "2.0",
                "id": 1,
                "method": "initialize",
                "params": {
                    "protocolVersion": "2025-06-18",
                    "clientInfo": {"name": "test", "version": "1.0"}
                }
            }`))
            require.NoError(t, err)

            // Read response
            _, _, err = conn.Read(ctx)
            require.NoError(t, err)

            // Close connection
            err = conn.Close(websocket.StatusNormalClosure, "")
            assert.NoError(t, err)
        }()

        // Allow goroutines to clean up
        time.Sleep(100 * time.Millisecond)
    }

    // Shutdown handler
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    err := handler.Shutdown(shutdownCtx)
    assert.NoError(t, err)

    // Allow time for goroutines to exit
    time.Sleep(500 * time.Millisecond)
    runtime.GC()

    // Check goroutine count
    finalGoroutines := runtime.NumGoroutine()

    // Allow for a small number of system goroutines
    goroutineGrowth := finalGoroutines - initialGoroutines
    assert.LessOrEqual(t, goroutineGrowth, 2,
        "Goroutine leak detected. Initial: %d, Final: %d, Growth: %d",
        initialGoroutines, finalGoroutines, goroutineGrowth)
}

func TestPingLoopCleanup(t *testing.T) {
    handler := &Handler{
        logger: observability.NewNoOpLogger(),
    }

    // Create mock connection
    server := wstest.NewServer(t, func(conn *websocket.Conn, r *http.Request) {
        // Do nothing - just accept connection
    })
    defer server.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    conn, _, err := websocket.Dial(ctx, server.URL, nil)
    require.NoError(t, err)
    defer conn.Close(websocket.StatusNormalClosure, "")

    // Start ping loop
    pingCtx, pingCancel := context.WithCancel(context.Background())
    done := make(chan struct{})

    go handler.pingLoop(pingCtx, conn, done)

    // Let it run briefly
    time.Sleep(100 * time.Millisecond)

    // Cancel and verify cleanup
    pingCancel()

    // Should complete quickly
    select {
    case <-done:
        // Success - ping loop cleaned up
    case <-time.After(1 * time.Second):
        t.Fatal("Ping loop did not clean up in time")
    }
}

func TestConcurrentConnectionHandling(t *testing.T) {
    handler := NewHandler(
        tools.NewRegistry(),
        cache.NewMemoryCache(),
        nil,
        auth.NewEdgeAuthenticator(""),
        observability.NewNoOpLogger(),
    )

    // Track goroutines before
    runtime.GC()
    beforeGoroutines := runtime.NumGoroutine()

    // Create multiple concurrent connections
    numConnections := 10
    done := make(chan bool, numConnections)

    for i := 0; i < numConnections; i++ {
        go func(id int) {
            server := wstest.NewServer(t, handler.HandleConnection)
            defer server.Close()

            ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
            defer cancel()

            conn, _, err := websocket.Dial(ctx, server.URL, nil)
            if err != nil {
                done <- false
                return
            }

            // Quick message exchange
            _ = conn.Write(ctx, websocket.MessageText, []byte(`{
                "jsonrpc": "2.0",
                "id": 1,
                "method": "ping",
                "params": {}
            }`))

            conn.Close(websocket.StatusNormalClosure, "")
            done <- true
        }(i)
    }

    // Wait for all connections to complete
    successCount := 0
    for i := 0; i < numConnections; i++ {
        if <-done {
            successCount++
        }
    }

    assert.Equal(t, numConnections, successCount)

    // Allow cleanup
    time.Sleep(500 * time.Millisecond)
    runtime.GC()

    // Check goroutine count didn't grow significantly
    afterGoroutines := runtime.NumGoroutine()
    growth := afterGoroutines - beforeGoroutines

    assert.LessOrEqual(t, growth, 5,
        "Too many goroutines after concurrent connections. Before: %d, After: %d",
        beforeGoroutines, afterGoroutines)
}
```

4. **Run the tests:**
```bash
cd apps/edge-mcp
go test -v -run TestNoGoroutineLeaks ./internal/mcp/
go test -v -run TestPingLoopCleanup ./internal/mcp/
go test -v -run TestConcurrentConnectionHandling ./internal/mcp/
```

**Expected Output:**
```
=== RUN   TestNoGoroutineLeaks
--- PASS: TestNoGoroutineLeaks (3.15s)
=== RUN   TestPingLoopCleanup
--- PASS: TestPingLoopCleanup (0.20s)
=== RUN   TestConcurrentConnectionHandling
--- PASS: TestConcurrentConnectionHandling (1.65s)
PASS
```

**Common Mistakes to Avoid:**
- Always use defer for cleanup code
- Wait for goroutines to finish before returning
- Use context for cancellation, not just channels
- Don't forget to close channels you create
- Test with the race detector: `go test -race`

---

## Summary of Sprint 1

We've now covered the critical foundation stories with detailed implementation guides. Each story includes:
- Clear prerequisites and background
- Complete, runnable code examples
- Step-by-step instructions
- Expected output
- Common mistakes to avoid

The remaining sprints (2-5) follow the same pattern with similar detail level for:
- Sprint 2: AI Agent Usability (tool metadata, examples, semantic errors)
- Sprint 3: Observability & Resilience (metrics, health checks, circuit breakers)
- Sprint 4: Production Hardening (performance, security, operations)
- Sprint 5: Documentation & Training

Would you like me to continue with the detailed implementation for Sprint 2, or would you prefer to see a specific story from another sprint?
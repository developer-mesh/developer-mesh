# Edge MCP Completion Plan - Opus 4.1 Optimized
<!-- Leveraging Opus 4.1's 74.5% SWE-bench capabilities for systematic implementation -->

## 🎯 Implementation Philosophy
**CRITICAL: Read existing code before implementing. Never assume, always verify.**

### Key Principles
1. **Reuse First**: Check `pkg/` for existing components before creating new ones
2. **Pattern Consistency**: Follow project patterns (adapter/service/repository)
3. **Security by Default**: All command execution must be sandboxed
4. **Error Context**: Always wrap errors with `fmt.Errorf("context: %w", err)`
5. **Observability**: Use structured logging via `observability.Logger`

## 📦 Existing Components to Reuse

### From Analysis of Codebase
```go
// HTTP Client with Circuit Breaker
pkg/clients/rest_api_client.go       // RESTAPIClient with full resilience

// Security & Validation
pkg/security/encryption.go           // EncryptionService for credentials
pkg/tools/passthrough_validator.go   // Token/API key validation patterns

// Command Execution Pattern (from workflow_service_impl.go)
cmd := exec.CommandContext(ctx, interpreter, tmpFile.Name())
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true,  // Process group for cleanup
}

// Observability
pkg/observability/logger.go          // Structured logging
pkg/observability/metrics.go         // Metrics collection

// Models & Types
pkg/models/                          // Shared data models
```

## 🔨 Phase 1: Local Tool Implementations (Day 1)
<!-- Use extended thinking for security design -->

### 1.1 Create Secure Command Executor
**File**: `apps/edge-mcp/internal/executor/command.go`

```go
package executor

import (
    "context"
    "os/exec"
    "syscall"
    "time"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

type CommandExecutor struct {
    logger        observability.Logger
    workDir      string
    maxTimeout   time.Duration
    allowedPaths []string  // Sandbox paths
}

// IMPLEMENT: Based on pkg/services/workflow_service_impl.go:5869-5894
func (e *CommandExecutor) Execute(ctx context.Context, command string, args []string) (*Result, error) {
    // 1. Validate command is allowed
    // 2. Set timeout context
    // 3. Create exec.CommandContext
    // 4. Set SysProcAttr for security
    // 5. Capture stdout/stderr
    // 6. Execute with proper cleanup
}
```

### 1.2 Implement Git Tool
**File**: `apps/edge-mcp/internal/tools/git.go`

```go
// BEFORE IMPLEMENTING: Check pkg/adapters/github for patterns
// REUSE: CommandExecutor from 1.1

func (t *GitTool) handleStatus(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path string `json:"path,omitempty"`
    }
    
    // Use CommandExecutor
    result, err := t.executor.Execute(ctx, "git", []string{"status", "--porcelain=v2"})
    
    // Parse git output into structured response
    return parseGitStatus(result.Stdout), nil
}

func (t *GitTool) handleDiff(ctx context.Context, args json.RawMessage) (interface{}, error) {
    // Similar pattern with git diff --unified=3
}
```

### 1.3 Implement Docker Tool
**File**: `apps/edge-mcp/internal/tools/docker.go`

```go
// SECURITY: Docker commands need extra validation
func (t *DockerTool) handleBuild(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Path       string   `json:"path"`
        Dockerfile string   `json:"dockerfile,omitempty"`
        Tags       []string `json:"tags,omitempty"`
        BuildArgs  map[string]string `json:"buildArgs,omitempty"`
    }
    
    // Validate path is within allowed directories
    // Build docker command with security constraints
    // Execute and stream output
}

func (t *DockerTool) handlePs(ctx context.Context, args json.RawMessage) (interface{}, error) {
    // docker ps --format json
    // Parse and return container list
}
```

### 1.4 Implement Shell Tool (Most Sensitive)
**File**: `apps/edge-mcp/internal/tools/shell.go`

```go
// CRITICAL: Must implement strict sandboxing
func (t *ShellTool) handleExecute(ctx context.Context, args json.RawMessage) (interface{}, error) {
    var params struct {
        Command     string   `json:"command"`
        Args        []string `json:"args"`
        WorkDir     string   `json:"workDir,omitempty"`
        Env         []string `json:"env,omitempty"`
        AllowSudo   bool     `json:"allowSudo,omitempty"` // Default: false
    }
    
    // 1. Validate command against allowlist
    // 2. Check workDir is within sandbox
    // 3. Filter environment variables
    // 4. Never allow sudo unless explicitly configured
    // 5. Use CommandExecutor with strict timeout
}
```

### Verification Commands
```bash
# Test Git tool
curl -X POST http://localhost:8082/ws \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"git.status"},"id":1}'

# Test with security violations (should fail)
curl -X POST http://localhost:8082/ws \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"shell.execute","arguments":{"command":"rm","args":["-rf","/"]}},"id":1}'
```

## 🔗 Phase 2: Core Platform Integration (Day 2)
<!-- Reuse existing HTTP client patterns -->

### 2.1 Enhance Core Client with REST API
**File**: `apps/edge-mcp/internal/core/client.go`

```go
// REUSE: Pattern from pkg/clients/rest_api_client.go

import (
    "github.com/developer-mesh/developer-mesh/pkg/clients"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Update Client struct to use resilient HTTP client
type Client struct {
    baseURL      string
    httpClient   *http.Client  // With circuit breaker
    logger       observability.Logger
    circuitBreaker *clients.CircuitBreaker  // Reuse from pkg/clients
}

func (c *Client) AuthenticateWithCore(ctx context.Context) error {
    // POST /api/v1/auth/edge-mcp
    req := AuthRequest{
        EdgeMCPID: c.edgeMCPID,
        APIKey:    c.apiKey,
    }
    
    // Use circuit breaker pattern
    return c.doWithCircuitBreaker(ctx, func() error {
        return c.post(ctx, "/api/v1/auth/edge-mcp", req, nil)
    })
}

func (c *Client) FetchRemoteTools(ctx context.Context) ([]tools.ToolDefinition, error) {
    // GET /api/v1/tools?edge_mcp=true
    // Parse response into tool definitions
}
```

### 2.2 Implement Session Management
**File**: `apps/edge-mcp/internal/core/session.go`

```go
// Session management with Core Platform
type SessionManager struct {
    client   *Client
    sessions map[string]*Session  // Local cache
    mu       sync.RWMutex
}

func (s *SessionManager) CreateSession(ctx context.Context, clientInfo ClientInfo) (string, error) {
    // POST /api/v1/sessions
    // Store session locally and in Core
}
```

### 2.3 Implement Context Synchronization
**File**: `apps/edge-mcp/internal/core/context_sync.go`

```go
// PATTERN: Similar to apps/mcp-server/internal/core/context_manager.go

type ContextSync struct {
    client     *Client
    cache      cache.Cache  // Local cache for performance
    syncInterval time.Duration
}

func (cs *ContextSync) UpdateContext(ctx context.Context, sessionID string, data map[string]interface{}) error {
    // 1. Update local cache immediately
    // 2. Queue for sync to Core Platform
    // 3. Handle offline mode gracefully
}
```

### Verification
```bash
# Test Core Platform connection
edge-mcp --core-url=http://localhost:8081 --api-key=test-key

# Check logs for successful auth
tail -f edge-mcp.log | grep "Core Platform authenticated"
```

## 🔌 Phase 3: MCP Protocol Enhancements (Day 2)

### 3.1 Implement Logging Handler
**File**: `apps/edge-mcp/internal/mcp/handler.go`

```go
// Update handleLoggingSetLevel
func (h *Handler) handleLoggingSetLevel(sessionID string, msg *MCPMessage) (*MCPMessage, error) {
    var params struct {
        Level string `json:"level"`
    }
    
    // Map MCP levels to observability levels
    levelMap := map[string]string{
        "debug": "DEBUG",
        "info":  "INFO",
        "warn":  "WARN",
        "error": "ERROR",
    }
    
    if level, ok := levelMap[params.Level]; ok {
        h.logger.SetLevel(level)
    }
}
```

### 3.2 Implement Request Cancellation
**File**: `apps/edge-mcp/internal/mcp/cancellation.go`

```go
type CancellationManager struct {
    activeRequests map[interface{}]context.CancelFunc
    mu             sync.RWMutex
}

func (cm *CancellationManager) Track(id interface{}, cancel context.CancelFunc) {
    cm.mu.Lock()
    cm.activeRequests[id] = cancel
    cm.mu.Unlock()
}

func (cm *CancellationManager) Cancel(id interface{}) error {
    cm.mu.Lock()
    defer cm.mu.Unlock()
    
    if cancel, ok := cm.activeRequests[id]; ok {
        cancel()
        delete(cm.activeRequests, id)
        return nil
    }
    return fmt.Errorf("request %v not found", id)
}
```

## 📝 Phase 4: Documentation & Examples (Day 3)

### 4.1 Main README
**File**: `apps/edge-mcp/README.md`

```markdown
# Edge MCP - Lightweight Local MCP Server

## Quick Start
\`\`\`bash
# Build
make build-edge-mcp

# Run standalone
./edge-mcp --port=8082

# Run with Core Platform
./edge-mcp --core-url=https://platform.devmesh.ai --api-key=$API_KEY
\`\`\`

## Architecture
[Include architecture diagram]

## Security Model
- All commands run in sandboxed environment
- Configurable allow/deny lists
- No sudo by default
```

### 4.2 IDE Configuration Files

#### Claude Code Configuration
**File**: `.claude/mcp.json`
```json
{
  "mcpServers": {
    "edge-mcp": {
      "command": "edge-mcp",
      "args": ["--port", "8082"],
      "env": {
        "EDGE_MCP_API_KEY": "${EDGE_MCP_API_KEY}"
      }
    }
  }
}
```

#### Cursor Configuration
**File**: `.cursor/mcp.json`
```json
{
  "mcp": {
    "servers": [{
      "name": "edge-mcp",
      "url": "ws://localhost:8082/ws",
      "apiKey": "${CURSOR_MCP_API_KEY}"
    }]
  }
}
```

### 4.3 Example Edge MCPs

#### GitHub Edge MCP
**File**: `examples/github-mcp/main.go`
```go
// Specialized Edge MCP for GitHub operations
// Includes: PR management, issue tracking, code search
```

#### AWS Edge MCP
**File**: `examples/aws-mcp/main.go`
```go
// Edge MCP for AWS operations
// Includes: S3, Lambda, CloudWatch tools
```

## 🧪 Phase 5: Testing (Day 3-4)

### 5.1 Integration Tests
**File**: `apps/edge-mcp/internal/mcp/handler_test.go`

```go
func TestEdgeMCPIntegration(t *testing.T) {
    // Test full MCP protocol flow
    // Test tool execution
    // Test Core Platform sync
    // Test offline mode
}
```

### 5.2 Security Tests
**File**: `apps/edge-mcp/internal/executor/command_test.go`

```go
func TestCommandExecutorSecurity(t *testing.T) {
    tests := []struct {
        name        string
        command     string
        shouldFail  bool
    }{
        {"Valid git command", "git status", false},
        {"Dangerous rm command", "rm -rf /", true},
        {"Path traversal", "cat ../../../../etc/passwd", true},
    }
}
```

### 5.3 End-to-End Tests
**File**: `test/e2e/edge_mcp_test.go`

```go
func TestEdgeMCPWithClaudeCode(t *testing.T) {
    // Start Edge MCP
    // Connect with MCP client
    // Execute tools
    // Verify results
}
```

## 📋 Implementation Checklist

### Day 1: Foundation
- [ ] Create CommandExecutor with security
- [ ] Implement Git tool (status, diff)
- [ ] Implement Docker tool (build, ps)
- [ ] Implement Shell tool with strict sandboxing
- [ ] Add input validation for all tools

### Day 2: Integration
- [ ] Update Core Client with REST API calls
- [ ] Implement authentication flow
- [ ] Add session management
- [ ] Implement context synchronization
- [ ] Add offline mode support

### Day 3: Protocol & Docs
- [ ] Complete MCP protocol handlers
- [ ] Add request cancellation
- [ ] Create comprehensive README
- [ ] Add IDE configuration files
- [ ] Create example Edge MCPs

### Day 4: Testing & Polish
- [ ] Write integration tests
- [ ] Add security tests
- [ ] Create E2E tests
- [ ] Performance optimization
- [ ] Final documentation review

## 🚀 Verification Steps

### After Each Phase
```bash
# Run tests
cd apps/edge-mcp && go test ./...

# Check for security issues
gosec ./...

# Verify no infrastructure dependencies
go list -m all | grep -E "redis|postgres" # Should be empty

# Test with Claude Code
claude-code connect ws://localhost:8082/ws
```

### Final Verification
```bash
# Full build
make build-edge-mcp

# Run all tests
make test-edge-mcp

# Security scan
make security-check

# Integration test with Core Platform
./scripts/test-edge-mcp-integration.sh
```

## 🎯 Success Criteria
1. ✅ All local tools working securely
2. ✅ Core Platform integration functional
3. ✅ MCP protocol fully implemented
4. ✅ Zero infrastructure dependencies
5. ✅ Claude Code can connect and execute tools
6. ✅ Security tests pass
7. ✅ Documentation complete

## 📚 Reference Implementation Patterns

### From Codebase Analysis
```go
// Error handling (always add context)
if err != nil {
    return nil, fmt.Errorf("failed to execute tool %s: %w", toolName, err)
}

// Defer cleanup with error handling
defer func() {
    if err := resource.Close(); err != nil {
        logger.Warn("Failed to close resource", map[string]interface{}{
            "error": err.Error(),
        })
    }
}()

// Context-first signatures
func Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)

// Structured logging
logger.Info("Tool executed", map[string]interface{}{
    "tool": toolName,
    "duration": time.Since(start),
    "success": err == nil,
})
```

## 🔒 Security Considerations

### Command Execution
- **Never** execute commands without validation
- **Always** use context with timeout
- **Always** set process group for cleanup
- **Never** allow sudo unless explicitly configured
- **Always** validate paths are within sandbox

### API Security
- Validate all API keys with regex pattern
- Use circuit breaker for external calls
- Implement rate limiting
- Encrypt sensitive data at rest

### Input Validation
```go
// From pkg/tools/passthrough_validator.go
apiKeyRegex := regexp.MustCompile(`^[A-Za-z0-9\-_]{20,}$`)
if !apiKeyRegex.MatchString(apiKey) {
    return fmt.Errorf("invalid API key format")
}
```

## 🎮 Claude Code Usage Tips

### For Extended Thinking
- Phase 1.4 (Shell tool): "Think deeply about security implications"
- Phase 2.3 (Context sync): "Think harder about offline/online sync strategy"

### For Multi-File Operations
- Phase 1: Creating multiple tool implementations
- Phase 2: Updating multiple Core Platform integration files
- Phase 4: Creating documentation across multiple files

### Incremental Development
```bash
# After each implementation
git add . && git commit -m "feat: [description]"
make test
make lint
```

## 📞 Getting Help
- Check existing patterns in `pkg/`
- Review similar implementations in `apps/mcp-server`
- Use `grep -r "pattern" .` to find examples
- Read test files for usage patterns

---
**Remember**: Never assume. Always read existing code first. Follow the patterns.
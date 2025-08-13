# Edge MCP Quick Reference - Start Here! 🚀

## 🔍 Before You Code - ALWAYS Check These First

### 1. Existing Packages to Reuse
```bash
# Find HTTP client patterns
grep -r "http.Client" pkg/

# Find command execution patterns  
grep -r "exec.Command" pkg/

# Find validation patterns
grep -r "Validate\|Sanitize" pkg/

# Find error wrapping patterns
grep -r "fmt.Errorf.*%w" pkg/ | head -5
```

### 2. Key Files to Study
```go
// Command execution with security
pkg/services/workflow_service_impl.go:5869-5894

// HTTP client with resilience
pkg/clients/rest_api_client.go

// Validation patterns
pkg/tools/passthrough_validator.go

// Encryption for credentials
pkg/security/encryption.go

// Tool implementation pattern
apps/mcp-server/internal/api/tools/github/
```

## 🏗️ Implementation Order

### Step 1: Create Secure Command Executor (30 min)
```bash
# Create the executor package
mkdir -p apps/edge-mcp/internal/executor
touch apps/edge-mcp/internal/executor/command.go

# Copy security patterns from:
grep -A 20 "SysProcAttr" pkg/services/workflow_service_impl.go
```

**Key Security Features:**
- `context.WithTimeout()` for all commands
- `SysProcAttr{Setpgid: true}` for process isolation
- Validate commands against allowlist
- Sandbox working directories

### Step 2: Implement Git Tool (20 min)
```go
// Start with these commands
git status --porcelain=v2    // Structured output
git diff --unified=3          // Standard diff format
git log --oneline -n 10      // Recent commits

// Parse output into JSON
type GitStatus struct {
    Branch     string           `json:"branch"`
    Modified   []string         `json:"modified"`
    Untracked  []string         `json:"untracked"`
    Staged     []string         `json:"staged"`
}
```

### Step 3: Implement Docker Tool (20 min)
```go
// Safe docker commands
docker ps --format "{{json .}}"     // JSON output
docker images --format "{{json .}}"  // JSON output
docker build --no-cache              // Prevent cache issues

// Validate build context path
if !strings.HasPrefix(filepath.Clean(path), allowedPath) {
    return fmt.Errorf("path outside allowed directory")
}
```

### Step 4: Implement Shell Tool (30 min) ⚠️ CRITICAL
```go
// STRICT Security Requirements
var allowedCommands = map[string]bool{
    "ls": true, "cat": true, "grep": true, "find": true,
    "echo": true, "pwd": true, "which": true,
    // NEVER: rm, sudo, chmod, chown, kill
}

// Path validation
func isPathSafe(path string) bool {
    cleaned := filepath.Clean(path)
    return !strings.Contains(cleaned, "..")
}
```

### Step 5: Core Platform Client (45 min)
```go
// Reuse circuit breaker from pkg/clients
import "github.com/developer-mesh/developer-mesh/pkg/clients"

// API Endpoints
POST /api/v1/auth/edge-mcp          // Authenticate
GET  /api/v1/tools?edge_mcp=true    // Fetch tools
POST /api/v1/sessions                // Create session
PUT  /api/v1/context/{session}      // Update context
GET  /api/v1/context/{session}      // Get context
```

## 🧪 Testing Each Component

### Test Git Tool
```bash
# Quick test
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"git.status"},"id":1}' | \
  websocat ws://localhost:8082/ws
```

### Test Docker Tool
```bash
# List containers
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"docker.ps"},"id":1}' | \
  websocat ws://localhost:8082/ws
```

### Test Shell Tool (with security)
```bash
# Should work
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"shell.execute","arguments":{"command":"ls"}},"id":1}' | \
  websocat ws://localhost:8082/ws

# Should fail (security violation)
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"shell.execute","arguments":{"command":"rm","args":["-rf","/"]}},"id":1}' | \
  websocat ws://localhost:8082/ws
```

## 🛠️ Common Patterns to Follow

### Error Handling
```go
// ALWAYS wrap with context
if err != nil {
    return nil, fmt.Errorf("failed to execute git status: %w", err)
}
```

### Resource Cleanup
```go
// ALWAYS defer cleanup with error check
defer func() {
    if err := conn.Close(); err != nil {
        logger.Warn("Failed to close connection", map[string]interface{}{
            "error": err.Error(),
        })
    }
}()
```

### Logging
```go
// Use structured logging
logger.Info("Command executed", map[string]interface{}{
    "command": cmd,
    "duration": time.Since(start),
    "success": err == nil,
})
```

### Context Usage
```go
// Context ALWAYS first parameter
func Execute(ctx context.Context, cmd string) error {
    // Set timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    return exec.CommandContext(ctx, cmd).Run()
}
```

## 🚫 Never Do These

### Security Anti-Patterns
```go
// ❌ NEVER - Command injection vulnerable
cmd := exec.Command("sh", "-c", userInput)

// ✅ CORRECT - Safe command execution
cmd := exec.Command(binary, args...)

// ❌ NEVER - Unrestricted file access
content, _ := os.ReadFile(userPath)

// ✅ CORRECT - Validated path access
if !isPathSafe(userPath) {
    return fmt.Errorf("invalid path")
}

// ❌ NEVER - No timeout
cmd := exec.Command("long-running-process")

// ✅ CORRECT - Always use timeout
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
cmd := exec.CommandContext(ctx, "process")
```

## 📝 Git Workflow

### After Each Component
```bash
# Format and lint
make fmt
make lint

# Test
cd apps/edge-mcp && go test ./...

# Commit
git add .
git commit -m "feat(edge-mcp): implement [component]"
```

### Before Moving to Next Phase
```bash
# Verify no infrastructure deps
go list -m all | grep -E "redis|postgres"  # Should be empty

# Security check
gosec ./apps/edge-mcp/...

# Run edge-mcp
cd apps/edge-mcp && go run cmd/server/main.go --port=8082
```

## 🔥 Quick Fixes

### Import Errors
```bash
go work sync
cd apps/edge-mcp && go mod tidy
```

### Missing Size() Method
```go
// Add to any cache mock
func (m *MockCache) Size() int {
    return len(m.data)
}
```

### Linting Issues
```bash
make fmt          # Format code
make vet          # Check vet issues  
make lint         # Run golangci-lint
```

## 🎯 Success Verification

### Component Complete Checklist
- [ ] Tests pass: `go test ./...`
- [ ] No security issues: `gosec ./...`
- [ ] Linting clean: `make lint`
- [ ] Error handling with context
- [ ] Resource cleanup with defer
- [ ] Structured logging used
- [ ] Timeouts on all operations

### Final Integration Test
```bash
# Start Edge MCP
./edge-mcp --port=8082 --api-key=test-key

# Connect with Claude Code
# Add to .claude/mcp.json and restart Claude Code

# Test all tools work
./scripts/test-edge-mcp-tools.sh
```

---
**Remember**: Read first, assume never. When in doubt, grep the codebase!
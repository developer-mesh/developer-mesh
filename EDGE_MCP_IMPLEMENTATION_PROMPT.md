# 🎯 Edge MCP Implementation Prompt - Opus 4.1 Optimized
<!-- CRITICAL: This prompt is engineered to prevent hallucinations and ensure correct implementation -->

## 📋 MANDATORY PRE-FLIGHT CHECKLIST
**YOU MUST COMPLETE THIS CHECKLIST BEFORE WRITING ANY CODE**

```bash
# 1. VERIFY you are on correct branch
git status | grep "feature/edge-mcp-and-tenant-credentials"

# 2. READ the implementation plans
cat EDGE_MCP_COMPLETION_PLAN.md | head -100
cat EDGE_MCP_QUICK_REFERENCE.md | head -50

# 3. VERIFY project structure
ls -la apps/edge-mcp/
ls -la pkg/

# 4. FIND existing patterns (DO NOT SKIP)
grep -r "exec.CommandContext" pkg/ --include="*.go" | head -5
grep -r "http.Client" pkg/clients/ --include="*.go" | head -5
grep -r "fmt.Errorf.*%w" pkg/ --include="*.go" | head -5
```

## 🧠 OPUS 4.1 COGNITIVE INSTRUCTIONS

### THINKING MODE ACTIVATION
Before EACH implementation task, you MUST:
1. Say: "Let me think deeply about the security and architecture implications..."
2. Use extended thinking for:
   - Security boundaries (Phase 1.4 - Shell tool)
   - Architecture decisions (Phase 2 - Core Platform)
   - Concurrency patterns (Phase 2.3 - Context sync)

### ANTI-HALLUCINATION PROTOCOL
**NEVER write code without first:**
```bash
# For EVERY new function you write:
1. grep -r "similar_function_pattern" pkg/
2. Read at least 3 existing examples
3. Verify import paths exist
4. Check if pkg already has this functionality
```

**Example - Before implementing command execution:**
```bash
# MANDATORY - Find existing patterns first
grep -r "exec.Command" pkg/ apps/ --include="*.go" -A 5 -B 5
grep -r "SysProcAttr" pkg/ apps/ --include="*.go" -A 10
grep -r "CommandContext.*timeout" pkg/ apps/ --include="*.go"

# READ the found patterns
cat pkg/services/workflow_service_impl.go | sed -n '5869,5894p'
```

## 🏗️ IMPLEMENTATION INSTRUCTIONS

### PHASE 1: Secure Command Executor [CRITICAL SECURITY]

#### Step 1.1: Read Before Writing
```bash
# MANDATORY - Study existing command execution
cat pkg/services/workflow_service_impl.go | sed -n '5869,5894p'

# Find timeout patterns
grep -r "context.WithTimeout" pkg/ --include="*.go" | head -3

# Find process group patterns
grep -r "Setpgid" pkg/ --include="*.go"
```

#### Step 1.2: Create Command Executor
**File**: `apps/edge-mcp/internal/executor/command.go`

```go
// BEFORE WRITING: Verify these imports exist
// go doc os/exec.CommandContext
// go doc syscall.SysProcAttr

package executor

import (
    "bytes"
    "context"
    "fmt"
    "os/exec"
    "strings"
    "syscall"
    "time"
    
    // VERIFY: Check this import path exists
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

// CommandExecutor provides secure command execution
type CommandExecutor struct {
    logger       observability.Logger
    maxTimeout   time.Duration
    workDir      string
    allowedPaths []string
    allowedCmds  map[string]bool  // Whitelist of commands
}

// SECURITY: This is the most critical function
func (e *CommandExecutor) Execute(ctx context.Context, command string, args []string) (*Result, error) {
    // STEP 1: Validate command is allowed
    if !e.allowedCmds[command] {
        return nil, fmt.Errorf("command not allowed: %s", command)
    }
    
    // STEP 2: Create timeout context (MANDATORY)
    ctx, cancel := context.WithTimeout(ctx, e.maxTimeout)
    defer cancel()
    
    // STEP 3: Create command with context
    cmd := exec.CommandContext(ctx, command, args...)
    
    // STEP 4: Set security attributes (COPY from workflow_service_impl.go:5884)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,  // Create new process group
    }
    
    // STEP 5: Set working directory with validation
    if e.workDir != "" {
        cmd.Dir = e.workDir
    }
    
    // STEP 6: Capture output
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    // STEP 7: Execute with logging
    start := time.Now()
    err := cmd.Run()
    
    // STEP 8: Log execution (use structured logging pattern)
    e.logger.Info("Command executed", map[string]interface{}{
        "command":  command,
        "duration": time.Since(start),
        "success":  err == nil,
    })
    
    return &Result{
        Stdout: stdout.String(),
        Stderr: stderr.String(),
        Error:  err,
    }, nil
}
```

#### Step 1.3: Verify Implementation
```bash
# Test compilation
cd apps/edge-mcp && go build ./internal/executor/

# Run security check
gosec ./internal/executor/

# Verify no unsafe patterns
grep -r "exec.Command(" internal/executor/  # Should find CommandContext only
```

### PHASE 2: Tool Implementations

#### Git Tool - READ FIRST
```bash
# Find git command patterns
grep -r "git status" apps/ pkg/ --include="*.go"
grep -r "git diff" apps/ pkg/ --include="*.go"

# Check if we already have git parsing
find pkg/ -name "*git*" -type f | xargs grep -l "Parse"
```

#### Implementation Template
```go
// File: apps/edge-mcp/internal/tools/git.go

// BEFORE: Verify executor import works
import (
    "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/executor"
)

func (t *GitTool) handleStatus(ctx context.Context, args json.RawMessage) (interface{}, error) {
    // STEP 1: Parse arguments
    var params struct {
        Path string `json:"path,omitempty"`
    }
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }
    
    // STEP 2: Validate path
    if params.Path != "" && !t.isPathSafe(params.Path) {
        return nil, fmt.Errorf("invalid path: %s", params.Path)
    }
    
    // STEP 3: Execute git command
    result, err := t.executor.Execute(ctx, "git", []string{"status", "--porcelain=v2"})
    if err != nil {
        return nil, fmt.Errorf("git status failed: %w", err)
    }
    
    // STEP 4: Parse output
    return t.parseGitStatus(result.Stdout), nil
}
```

### PHASE 3: Core Platform Integration

#### MANDATORY: Check Existing HTTP Clients First
```bash
# Find ALL http client implementations
find pkg/ -name "*.go" | xargs grep -l "http.Client" | head -5

# READ the main REST client
cat pkg/clients/rest_api_client.go | head -200

# Find circuit breaker patterns
grep -r "CircuitBreaker" pkg/ --include="*.go" -B 5 -A 10
```

#### Reuse Existing Client
```go
// File: apps/edge-mcp/internal/core/client.go

// DO NOT CREATE NEW - REUSE EXISTING
import (
    "github.com/developer-mesh/developer-mesh/pkg/clients"
)

// Enhance existing Client to use resilient patterns
type Client struct {
    baseURL    string
    tenantID   string
    edgeMCPID  string
    apiKey     string
    
    // REUSE existing client with circuit breaker
    restClient clients.RESTAPIClient  // From pkg/clients
    logger     observability.Logger
}
```

## 🔍 VERIFICATION PROTOCOL

### After EACH File Creation
```bash
# 1. Verify imports
go list -json ./... | jq '.Imports[]' | grep -v '"github.com/developer-mesh'

# 2. Check compilation
go build ./...

# 3. Run security scan
gosec ./...

# 4. Check for hallucinated functions
# For each function you call, verify it exists:
go doc <package>.<function>
```

### After EACH Phase
```bash
# 1. Run tests
go test ./... -v

# 2. Check no infrastructure dependencies
go list -m all | grep -E "redis|postgres"  # Must be empty for edge-mcp

# 3. Verify patterns match project
grep -r "fmt.Errorf" . --include="*.go" | grep -v "%w"  # Should be none

# 4. Commit with descriptive message
git add .
git commit -m "feat(edge-mcp): [specific description]"
```

## 🚨 HALLUCINATION PREVENTION RULES

### RULE 1: Never Assume Package Exists
```bash
# WRONG - Assuming package exists
import "github.com/some/package"

# CORRECT - First verify
find pkg/ -type f -name "*.go" | xargs grep "some/package"
# If not found, check if we have similar functionality
grep -r "similar_functionality" pkg/
```

### RULE 2: Never Assume Function Signature
```bash
# WRONG - Guessing function signature
result := SomeFunction(param1, param2)

# CORRECT - First check signature
go doc package.SomeFunction
# Or grep for usage
grep -r "SomeFunction(" pkg/ --include="*.go"
```

### RULE 3: Never Create When Exists
```bash
# Before creating ANY new functionality:
1. grep -r "functionality_keywords" pkg/
2. Check if pkg/common has it
3. Check if pkg/utils has it
4. Only create if truly needed
```

### RULE 4: Copy Exact Patterns
```bash
# When you find a pattern, copy it EXACTLY
# Example: Error handling
grep -r "defer func()" pkg/ --include="*.go" -A 5 | head -20
# Copy the EXACT pattern, don't modify
```

## 📊 PROGRESS TRACKING

### Use TodoWrite Tool
After completing each component:
```go
TodoWrite([{
    "content": "Implement CommandExecutor with security",
    "status": "completed",
    "id": "executor"
}])
```

### Checkpoint Questions
Before moving to next component, answer:
1. Does it compile? `go build ./...`
2. Are imports valid? `go list ./...`
3. Security checked? `gosec ./...`
4. Tests written? `go test ./...`
5. Patterns followed? Check against pkg/ examples

## 🎮 CLAUDE CODE OPTIMIZATION

### Multi-File Operations
When implementing tools (Phase 1):
1. Open all tool files simultaneously
2. Implement shared interfaces first
3. Use consistent error handling across all

### Use Project Search
Instead of assuming:
```
Cmd+Shift+F: "exec.Command"  # Find all usages
Cmd+Shift+F: "http.Client"   # Find client patterns
```

### Leverage IntelliSense
- Hover over imports to verify they exist
- Use autocomplete to find correct function names
- Check parameter types before calling

## 🏁 FINAL CHECKLIST

Before considering implementation complete:

- [ ] All commands execute in sandboxed environment
- [ ] All HTTP calls use circuit breaker
- [ ] All errors wrapped with context
- [ ] All resources cleaned up with defer
- [ ] All operations have timeouts
- [ ] No hardcoded values
- [ ] No DEBUG prints
- [ ] Tests cover happy and error paths
- [ ] Security scan passes
- [ ] No Redis/PostgreSQL dependencies

## 💡 WHEN STUCK

### Find Similar Code
```bash
# Find similar functionality
grep -r "what_you_need" pkg/ apps/ --include="*.go"

# Find interface definitions
grep -r "type.*Interface" pkg/ --include="*.go"

# Find test examples
grep -r "Test.*what_you_need" pkg/ apps/ --include="*.go"
```

### Read Documentation
```bash
# Read package documentation
go doc github.com/developer-mesh/developer-mesh/pkg/clients

# Read specific function
go doc github.com/developer-mesh/developer-mesh/pkg/clients.NewRESTAPIClient
```

### Check Git History
```bash
# See how similar features were implemented
git log --oneline --grep="implement" | head -10
git show <commit_hash>
```

---

## 🔴 CRITICAL: STOP CONDITIONS

**STOP and seek clarification if:**
1. You're about to import a package not in go.mod
2. You're creating functionality that might exist in pkg/
3. You're unsure about security implications
4. You can't find existing patterns for what you're doing
5. Tests are failing and you're not sure why

**Remember**: It's better to grep 10 times than hallucinate once.

---

# START IMPLEMENTATION

Begin with Phase 1.1: Read existing command execution patterns, then implement CommandExecutor exactly as shown above.
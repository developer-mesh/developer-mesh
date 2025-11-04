# Multi-Agent Orchestration Implementation Prompt

## Instructions for AI Assistant

When asked to "complete Phase X of ORCHESTRATION_IMPLEMENTATION_PLAN.md", you MUST:

1. **FIRST** - Read the specified phase section from ORCHESTRATION_IMPLEMENTATION_PLAN.md
2. **EXTRACT** - Automatically extract the phase name, priority, timeline, and components
3. **IMPLEMENT** - Follow the implementation principles below to complete the phase
4. **VALIDATE** - Ensure all changes follow the reuse hierarchy

You do NOT need any additional information beyond the phase number. Everything needed is in the plan.

## Core Implementation Principles (CRITICAL - MUST FOLLOW)

### 1. Package Reuse Hierarchy
**ALWAYS follow this decision tree:**
```
1. Can I enhance an existing package? → DO THAT
2. Can I extend an existing service? → DO THAT
3. Can I add to existing middleware? → DO THAT
4. Can I use existing patterns? → DO THAT
5. Only if none of above → Create new file
```

### 2. Existing Packages to Check FIRST
Before implementing ANY functionality, check these packages:
- `/pkg/services/assignment_engine.go` - Task assignment logic
- `/pkg/services/enhanced_tool_registry.go` - Tool/agent discovery
- `/pkg/services/workflow_service.go` - Workflow orchestration
- `/pkg/webhook/` - Webhook handling and orchestration
- `/pkg/middleware/validation.go` - Validation logic
- `/pkg/observability/` - Metrics, logging, tracing
- `/pkg/repository/` - Database operations
- `/pkg/core/semantic_context_manager_impl.go` - Context management
- `/pkg/resilience/` - Circuit breakers, retry logic
- `/pkg/utils/retry.go` - Retry patterns
- `/pkg/security/` - Encryption services
- `/pkg/auth/` - Authentication patterns

### 3. Patterns to Follow
- **Authentication**: Use existing Edge-MCP authentication (`/apps/edge-mcp/internal/auth/`)
- **Error Handling**: Use existing resilience patterns (`/pkg/resilience/`, `/pkg/utils/retry.go`)
- **Logging**: Use existing observability.Logger (NEVER fmt.Printf)
- **Database**: Use repository pattern with sqlx (NEVER raw SQL)
- **Caching**: Use existing tiered cache (`/apps/edge-mcp/internal/cache/`)
- **Validation**: Extend middleware/validation.go
- **Metrics**: Enhance prometheus_metrics.go

## Implementation Steps for This Phase

### Step 1: Review Phase Requirements
1. Read the specific phase section in `ORCHESTRATION_IMPLEMENTATION_PLAN.md`
2. Note all components marked as "ENHANCE EXISTING"
3. Identify which existing files need modification
4. List any genuinely new files needed (should be minimal)

### Step 2: Pre-Implementation Checks
Before writing ANY code, verify:
- [ ] Have I checked if this functionality already exists?
- [ ] Have I identified which existing package to enhance?
- [ ] Have I reviewed existing patterns in that package?
- [ ] Have I checked for similar implementations elsewhere?
- [ ] Am I following the existing code style and patterns?

### Step 3: Implementation Guidelines

#### When ENHANCING existing packages:
```go
// 1. Add to the existing file, don't create new
// 2. Follow the existing patterns in that file
// 3. Reuse existing structs and interfaces
// 4. Extend existing functions where possible
// Example: Adding to assignment_engine.go

// GOOD - Extends existing struct
func (ae *AssignmentEngine) NewOrchestrationMethod() error {
    // Uses existing engine fields and methods
    return ae.existingMethod()
}

// BAD - Creates duplicate functionality
type NewAssignmentEngine struct {
    // Duplicates existing engine
}
```

#### When adding database operations:
```go
// ALWAYS use existing repository pattern
// NEVER create new repository files if one exists for that domain

// GOOD - Enhance existing repository
func (r *AgentRepository) GetOrchestrationAgents(ctx context.Context) ([]*Agent, error) {
    // Follows existing repository patterns
    return r.db.SelectContext(ctx, &agents, query)
}

// BAD - Creating new repository for same domain
type OrchestrationRepository struct {
    // Duplicates agent repository functionality
}
```

#### When adding API endpoints:
```go
// Add to existing handlers where logical
// Only create new handler files for completely new domains

// GOOD - Add to existing task_handler.go
func (h *TaskHandler) HandleOrchestration(c *gin.Context) {
    // Reuses existing handler infrastructure
}

// BAD - New handler for related functionality
type OrchestrationHandler struct {
    // Should be part of TaskHandler
}
```

### Step 4: Testing Requirements
- [ ] Unit tests follow existing patterns in `*_test.go` files
- [ ] Integration tests added to `/test/integration/` if needed
- [ ] All tests pass: `make test`
- [ ] No new lint errors: `make lint`
- [ ] Coverage maintained above 80% for modified code

### Step 5: Validation Checklist
Before considering this phase complete:
- [ ] All "ENHANCE EXISTING" items modified existing files
- [ ] No unnecessary new packages created
- [ ] Existing patterns and styles followed
- [ ] All dependencies injected (no globals)
- [ ] Error handling uses existing patterns
- [ ] Logging uses observability.Logger
- [ ] Database operations use repository pattern
- [ ] Authentication uses Edge-MCP patterns
- [ ] Tests written and passing
- [ ] Documentation updated in `/docs/` if needed

## Common Pitfalls to AVOID

### ❌ DON'T DO THIS:
1. **Creating new packages when existing ones work**
   ```go
   // BAD
   package orchestration  // New package

   // GOOD
   package services  // Enhance existing
   ```

2. **Duplicating functionality**
   ```go
   // BAD - New retry logic
   func orchestratorRetry() { ... }

   // GOOD - Use existing
   retry.RetryWithBackoff(...)
   ```

3. **Creating new auth patterns**
   ```go
   // BAD - New JWT implementation
   func validateOrchestrationToken() { ... }

   // GOOD - Use Edge-MCP auth
   authenticator.AuthenticateRequest(r)
   ```

4. **New logging implementations**
   ```go
   // BAD
   fmt.Printf("Orchestration started")

   // GOOD
   logger.Info("Orchestration started", map[string]interface{}{...})
   ```

## Phase-Specific Notes
Phase-specific requirements and considerations are automatically extracted from ORCHESTRATION_IMPLEMENTATION_PLAN.md for each phase. No manual notes needed.

## Verification Commands
Run these after implementing each component:
```bash
# Test the specific package
cd pkg/services && go test -v ./...

# Run all tests
make test

# Check for issues
make lint

# Verify build
make build

# Full pre-commit check
make pre-commit
```

## Questions to Ask Before Proceeding
1. Is there existing code I can enhance instead of creating new?
2. Am I following the patterns already in the codebase?
3. Have I checked the existing packages list above?
4. Am I duplicating any functionality?
5. Can I achieve this with configuration instead of code?

## Getting Help
- Check existing implementations: `grep -r "similar_function" pkg/`
- Review patterns: Look at similar files in the same package
- Test patterns: Check `*_test.go` files for testing approaches
- Ask: "How is this already done in the codebase?"

---

## Example Usage of This Prompt

**User says**: "Following @ORCHESTRATION_IMPLEMENTATION_PROMPT.md complete Phase 1 of @ORCHESTRATION_IMPLEMENTATION_PLAN.md"

**AI Assistant automatically**:
1. Reads Phase 1 section from ORCHESTRATION_IMPLEMENTATION_PLAN.md
2. Extracts: "Core Infrastructure & Task Assignment" (🔴 CRITICAL, Week 1-2)
3. Identifies components to enhance (assignment_engine.go, etc.)
4. Implements following the reuse hierarchy
5. Validates all changes use existing patterns
6. Runs tests and verification commands

No manual input needed - everything is extracted from the plan automatically.

## Remember: The Goal
**Enhance and extend existing packages** rather than creating new ones. Every new file or package should be justified as absolutely necessary and not duplicating existing functionality.
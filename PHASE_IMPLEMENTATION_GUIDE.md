# Phase-by-Phase Implementation Guide

## How to Use the Implementation Prompt

For each phase, use this template to start your implementation:

```markdown
I need to implement [PHASE NAME] from the ORCHESTRATION_IMPLEMENTATION_PLAN.md.

Please help me implement this phase following these requirements:
1. Use the ORCHESTRATION_IMPLEMENTATION_PROMPT.md guidelines
2. This is [PHASE NUMBER] with [PRIORITY LEVEL] priority
3. Focus on enhancing existing packages as specified in the plan
4. Ensure all code follows existing patterns in the codebase

Specific components to implement:
- [List specific components from that phase]

Please start by:
1. Reviewing what existing packages we'll enhance
2. Showing me the implementation with explanations
3. Including tests for all new functionality
```

## Phase Implementation Order

### 🔴 Phase 1: Core Infrastructure & Task Assignment (CRITICAL)
**When to implement**: First - foundational
**Key packages to enhance**:
- `/pkg/services/assignment_engine.go`
- `/apps/rest-api/internal/api/` (new handler only)

**Example prompt**:
```
I need to implement Phase 1: Core Infrastructure & Task Assignment from the ORCHESTRATION_IMPLEMENTATION_PLAN.md.

This includes:
1. Initializing orchestration services in MCP server
2. Enhancing assignment engine with new strategies
3. Creating task API endpoints

Please follow the ORCHESTRATION_IMPLEMENTATION_PROMPT.md and enhance existing packages.
```

### 🟡 Phase 2: Gateway Orchestrators (HIGH)
**When to implement**: After Phase 1
**Key packages to enhance**:
- `/pkg/services/workflow_service.go`
- `/pkg/webhook/`

### 🟢 Phase 3: Domain Coordinators (MEDIUM)
**When to implement**: After core orchestrators work
**Key packages to enhance**:
- `/pkg/services/` (add domain_coordinator.go)

### 🔴 Phase 4: Agent Registration & Discovery (CRITICAL)
**When to implement**: Parallel with Phase 1-2
**Key packages to enhance**:
- `/pkg/services/enhanced_tool_registry.go`
- `/pkg/middleware/validation.go`
- `/pkg/services/agent_service_impl.go`

### 🟢 Phase 5: Monitoring & Observability (MEDIUM)
**When to implement**: After core functionality
**Key packages to enhance**:
- `/pkg/observability/prometheus_metrics.go`
- `/pkg/observability/tracing.go`

### 🟡 Phase 6: Authentication & Authorization (HIGH)
**When to implement**: Early, with Phase 1
**Key packages to enhance**:
- Use existing Edge-MCP auth
- `/apps/edge-mcp/internal/auth/`

### 🔴 Phase 7: Error Handling & Resilience (CRITICAL)
**When to implement**: Throughout all phases
**Key packages to enhance**:
- `/pkg/resilience/circuit_breaker.go`
- `/pkg/utils/retry.go`

### 🟢 Phase 8: Observability Enhancement (MEDIUM)
**When to implement**: Ongoing with each phase
**Key packages to enhance**:
- `/pkg/observability/logger.go`
- `/pkg/observability/tracing.go`

### 🔴 Phase 9: Performance & Memory Management (CRITICAL)
**When to implement**: After Phase 4
**Key packages to enhance**:
- `/pkg/core/semantic_context_manager_impl.go`
- `/apps/edge-mcp/internal/cache/`

### 🟢 Phase 10: Cost Management (MEDIUM)
**When to implement**: After core complete
**Key packages to enhance**:
- Add to existing metrics/tracking

### 🟡 Phase 11: Security Hardening (HIGH)
**When to implement**: Throughout
**Key packages to enhance**:
- `/pkg/security/`
- `/pkg/auth/`

### 🟢 Phase 12: Deployment & Operations (MEDIUM)
**When to implement**: Final phase
**Key packages to enhance**:
- Configuration updates
- Kubernetes manifests

### 🟢 Phase 13: Testing Framework (MEDIUM)
**When to implement**: Parallel with each phase
**Key packages to enhance**:
- `/test/functional/`
- `/test/integration/`

### 🟡 Phase 14: Agent SDK (HIGH)
**When to implement**: After Phase 4
**Key packages to enhance**:
- Wrap existing Edge-MCP client

## Validation After Each Phase

After implementing each phase, run:

```bash
# 1. Run tests for modified packages
cd pkg/services && go test -v ./...

# 2. Run all tests
make test

# 3. Check lint
make lint

# 4. Full pre-commit
make pre-commit

# 5. Check coverage for new code
go test -cover ./pkg/services/...
```

## Red Flags That You're Doing It Wrong

🚨 **STOP if you find yourself**:
1. Creating a new package directory
2. Writing retry logic (use existing)
3. Implementing new auth patterns
4. Creating new logger implementations
5. Writing new database connection code
6. Building new caching mechanisms
7. Duplicating existing structs/interfaces

## Success Criteria for Each Phase

✅ **You've succeeded when**:
1. All ENHANCE EXISTING items modified existing files
2. Minimal new files created (only where absolutely necessary)
3. All tests pass
4. No duplicate functionality introduced
5. Existing patterns consistently followed
6. Code review would show mostly green (modifications) not red (new files)

## Quick Reference: Where Things Go

| Functionality | Enhance This File/Package |
|--------------|---------------------------|
| Task assignment logic | `/pkg/services/assignment_engine.go` |
| Agent discovery | `/pkg/services/enhanced_tool_registry.go` |
| Workflow orchestration | `/pkg/services/workflow_service.go` |
| Webhook handling | `/pkg/webhook/` |
| Validation | `/pkg/middleware/validation.go` |
| Metrics | `/pkg/observability/prometheus_metrics.go` |
| Logging enhancements | `/pkg/observability/logger.go` |
| Context management | `/pkg/core/semantic_context_manager_impl.go` |
| Circuit breakers | `/pkg/resilience/circuit_breaker.go` |
| Retry logic | `/pkg/utils/retry.go` |
| Agent versioning | `/pkg/services/agent_service_impl.go` |
| Authentication | `/apps/edge-mcp/internal/auth/` |
| Caching | `/apps/edge-mcp/internal/cache/` |

## Remember

**Every line of code should answer**: "Why am I not enhancing an existing file instead?"

If you can't answer that convincingly, you should be enhancing an existing file.
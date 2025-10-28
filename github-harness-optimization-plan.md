# GitHub & Harness MCP Tool Optimization Plan

## Problem Statement

User is seeing **388 tools** for just GitHub and Harness combined, which is excessive and consumes significant context tokens.

### Current State Analysis

Based on code review:
- **GitHub**: All 10 toolsets enabled by default = ~92 tools
- **Harness**: 20 modules enabled = estimated ~296 tools (if each module has ~15 operations)
- **Total**: ~388 tools

---

## Root Cause

### GitHub (`pkg/tools/providers/github/github_provider.go:374-398`)

```go
// enableDefaultToolsets enables the default set of toolsets
func (p *GitHubProvider) enableDefaultToolsets() {
    // Enable all toolsets by default to expose full GitHub functionality
    defaultToolsets := []string{
        "repos",           // 21 tools - TOO MANY
        "issues",          // 11 tools - OK
        "pull_requests",   // 13 tools - KEEP MOST
        "actions",         // 13 tools - OK
        "security",        // 12 tools - OK
        "collaboration",   // 6 tools  - REDUCE
        "git",             // 10 tools - REDUCE
        "organizations",   // 1 tool   - OK
        "discussions",     // 4 tools  - OPTIONAL
    }
    // ALL ENABLED BY DEFAULT ❌
}
```

### Harness (`pkg/tools/providers/harness/harness_provider.go:216-250`)

```go
enabledModules: map[HarnessModule]bool{
    // 20 modules enabled with potentially 10-20 operations each
    // = 200-400 individual MCP tools being generated
}
```

---

## Optimization Strategy

### Phase 1: GitHub - Disable Non-Essential Toolsets

**Keep Essential (Developer Workflow)**:
- ✅ `context` (1 tool) - Always enabled, provides user info
- ✅ `repos` (21 tools) - Core Git operations
- ✅ `issues` (11 tools) - Issue tracking
- ✅ `pull_requests` (13 tools) - Code review
- ✅ `actions` (13 tools) - CI/CD workflows
- ✅ `security` (12 tools) - Code scanning, Dependabot

**Disable Non-Essential**:
- ❌ `collaboration` (6 tools) - Notifications, gists (use GitHub UI)
- ❌ `git` (10 tools) - Low-level Git objects (advanced users only)
- ❌ `organizations` (1 tool) - User search (limited value)
- ❌ `discussions` (4 tools) - GitHub Discussions (use GitHub UI)

**Result**: 92 → **71 tools** (23% reduction, -21 tools)

---

### Phase 2: Harness - Tool Grouping Analysis

Harness appears to be generating individual MCP tools for each API operation. We need to:

1. **Verify actual tool count** - Count how many mcp__devmesh__harness_* tools exist
2. **Group operations** - Instead of exposing every operation, group them:
   - Example: `harness_pipeline_list`, `harness_pipeline_get`, `harness_pipeline_create`
   - Should become: `harness_pipelines` with action parameter

3. **Disable admin modules** - Already done in source at lines 216-250

**Expected Result**: If currently ~296 tools → Target **~40-50 grouped tools** (83% reduction)

---

## Implementation Plan

### Step 1: GitHub Optimization

**File**: `pkg/tools/providers/github/github_provider.go`

**Change**: Line 379-389

```go
// OLD:
defaultToolsets := []string{
    "repos", "issues", "pull_requests", "actions",
    "security", "collaboration", "git",
    "organizations", "discussions",
}

// NEW:
defaultToolsets := []string{
    "repos",          // Core repository operations
    "issues",         // Issue tracking and management
    "pull_requests",  // Pull request workflows
    "actions",        // CI/CD pipelines
    "security",       // Security scanning and alerts
    // Removed: collaboration, git, organizations, discussions
}
```

**Justification**:
- **collaboration**: Notifications/gists better managed in GitHub UI
- **git**: Low-level operations (blobs, trees) rarely needed by developers
- **organizations**: Single user search tool, limited value
- **discussions**: Better managed in GitHub UI, not core workflow

---

### Step 2: Harness Optimization Investigation

**Tasks**:
1. Count actual Harness tools being generated
2. Identify if tools are individual operations or grouped
3. Determine if OpenAPI spec is being used to auto-generate tools
4. Propose grouping strategy if needed

**File to Check**: `pkg/tools/providers/harness/` - how are tools actually registered?

---

### Step 3: Verify Tool Exposure

**Check**: `apps/edge-mcp/internal/mcp/handler.go:1109`

This line fetches tenant-specific tools from REST API:
```go
tenantTools, err := h.coreClient.FetchToolsForTenant(ctx, session.TenantID)
```

**Verify**:
1. What does `FetchToolsForTenant` return?
2. Does it call all provider `GetAIOptimizedDefinitions()` methods?
3. Are providers properly filtering tools?

---

## Expected Impact

| Provider | Before | After | Reduction |
|----------|--------|-------|-----------|
| **GitHub** | ~92 | ~71 | -21 tools (23%) |
| **Harness** | ~296 | ~45 | -251 tools (85%) |
| **Artifactory** | 8 | 5 | -3 tools (37%) |
| **TOTAL** | ~396 | ~121 | **-275 tools (69%)** |

**Context Token Savings**: ~6,875 tokens per interaction (69% reduction)

---

## Implementation Steps

### Immediate Actions

1. **Implement GitHub optimization** (15 minutes)
   - Modify `github_provider.go:379-389`
   - Remove 4 toolsets from defaultToolsets
   - Test build

2. **Investigate Harness tool generation** (30 minutes)
   - Count actual tools
   - Understand registration mechanism
   - Identify if OpenAPI auto-generation is happening

3. **Create optimization for Harness** (45 minutes)
   - Apply similar grouping strategy as Artifactory
   - Focus on developer workflows only
   - Remove admin operations

### Testing

```bash
# 1. Build
make build

# 2. Start Edge MCP
./bin/edge-mcp

# 3. Connect and check tool count
# (Use MCP client to call tools/list)

# 4. Verify tool count is ~121 (not 388)
```

---

## Success Criteria

✅ **Tool count ≤ 150** total across all providers
✅ **GitHub ≤ 75 tools**
✅ **Harness ≤ 50 tools**
✅ **All builds pass**
✅ **Core developer workflows maintained**
✅ **No functionality loss for developer tasks**

---

## Next Steps

1. **Approve this plan**
2. **Implement GitHub optimization** (quick win)
3. **Investigate Harness tool structure**
4. **Create Harness optimization based on findings**
5. **Test and deploy**

---

## Questions for User

1. Are you okay with removing GitHub collaboration/git/organizations/discussions toolsets?
2. Do you use GitHub Discussions or Gists in your developer workflow?
3. Are there specific Harness operations you rely on that we should ensure remain accessible?


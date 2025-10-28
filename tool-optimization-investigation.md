# Tool Optimization Investigation Report

## Issue Summary

**Problem**: After implementing GitHub toolset optimization and rebuilding/redeploying the app, the user still sees 388 tools available (unchanged from before optimization).

**Expected**: Tool count should have reduced by approximately 21 tools (GitHub optimization).

**Status**: ✅ **ROOT CAUSE IDENTIFIED**

---

## Investigation Findings

### 1. Code Changes Are Deployed ✅

**Verified**: `pkg/tools/providers/github/github_provider.go` lines 378-390

The code changes ARE present and correct:
```go
// Enable core developer workflow toolsets by default
defaultToolsets := []string{
    "repos",          // Repository operations (21 tools)
    "issues",         // Issue tracking and management (11 tools)
    "pull_requests",  // Pull request workflows (13 tools)
    "actions",        // CI/CD pipelines (13 tools)
    "security",       // Security scanning and alerts (12 tools)
    // Disabled toolsets: collaboration, git, organizations, discussions
}
```

### 2. Tool Discovery Flow ✅

**Flow Traced**:
```
MCP Client Request
  ↓
Edge MCP Handler (handleToolsList)
  ↓
Core Client (FetchToolsForTenant)
  ↓
REST API: GET /api/v1/tools?tenant_id=X&edge_mcp=true
  ↓
EnhancedToolRegistry (GetToolsForTenant)
  ↓
Database Query (dynamic_tools table)
  ↓
Returns: Cached/Stored Tool Records
```

**Key Files**:
- `apps/edge-mcp/internal/mcp/handler.go:1095-1172` - handleToolsList
- `apps/edge-mcp/internal/core/client.go:378-428` - FetchToolsForTenant
- `pkg/services/enhanced_tool_registry.go:273-303` - GetToolsForTenant

### 3. Root Cause Identified 🔍

**The Problem**:

Tools are stored in the database when first registered, NOT generated dynamically from providers on each request.

**Evidence**:

1. **Provider changes only affect NEW registrations**:
   - `GitHubProvider.GetToolDefinitions()` (line 441) filters by enabled toolsets
   - But this method is only called during INITIAL tool registration
   - Once tools are in the database, they're served from cache

2. **Existing tools remain unchanged**:
   - User's GitHub and Harness tools were registered BEFORE optimization
   - These tool records exist in the `dynamic_tools` table
   - REST API returns these database records, NOT fresh provider definitions

3. **Tool persistence location**:
   - `pkg/services/enhanced_tool_registry.go:290` - Reads from `dynamicToolRepo`
   - `apps/rest-api/internal/services/dynamic_tools_service.go` - Stores discovered tools in DB
   - Once stored, tools persist until explicitly deleted or re-registered

### 4. Why 388 Tools Still Appear

**Breakdown**:
```
GitHub (stored in DB):     ~92 tools  (ALL 9 toolsets from original registration)
Harness (stored in DB):   ~296 tools  (ALL 20 modules from original registration)
TOTAL:                    ~388 tools  (unchanged)
```

**Why provider changes didn't help**:
- Provider optimization reduces tools for NEW registrations only
- User's tools were registered BEFORE optimization
- Database records contain the FULL set of operations from original discovery
- MCP server serves these cached database records

---

## Solution Options

### Option 1: Re-Register Tools (Recommended)

**Steps**:
1. Delete existing GitHub and Harness tool records from database
2. Re-register tools using the optimized providers
3. New registration will only create tools for enabled toolsets

**Pros**:
- Clean solution using standard workflow
- Leverages existing optimization code
- No manual database manipulation

**Cons**:
- Requires re-authentication if credentials were stored
- Brief downtime while tools are re-registered

**Implementation**:
```bash
# Via REST API (as tenant admin)
# 1. List current tools
curl -H "Authorization: Bearer <api-key>" \
  http://localhost:8081/api/v1/tools?tenant_id=<tenant-id>

# 2. Delete GitHub tool
curl -X DELETE -H "Authorization: Bearer <api-key>" \
  http://localhost:8081/api/v1/tools/<github-tool-id>

# 3. Delete Harness tool
curl -X DELETE -H "Authorization: Bearer <api-key>" \
  http://localhost:8081/api/v1/tools/<harness-tool-id>

# 4. Re-register GitHub (will use new optimized provider)
curl -X POST -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github",
    "tool_type": "github",
    "config": {
      "credentials": {...}
    }
  }' \
  http://localhost:8081/api/v1/tools

# 5. Re-register Harness (after Harness optimization)
# (Similar to GitHub registration)
```

### Option 2: Direct Database Cleanup (Advanced)

**Steps**:
1. Identify tool records to modify/delete
2. Update database directly to remove unwanted operations
3. Clear any caches

**Pros**:
- No re-authentication needed
- Can be very surgical (remove specific operations)

**Cons**:
- Requires direct database access
- Risk of data corruption
- May miss cache invalidation

**Implementation**:
```sql
-- Connect to database
psql -h localhost -U devmesh -d devmesh_development

-- Find GitHub tool
SELECT id, tool_name, config FROM dynamic_tools
WHERE tool_name LIKE '%github%' AND tenant_id = '<tenant-id>';

-- Option A: Delete entire tool (requires re-registration)
DELETE FROM dynamic_tools WHERE id = '<github-tool-id>';

-- Option B: Update config to remove operations (complex)
-- Not recommended - better to re-register
```

### Option 3: Implement Dynamic Tool Generation (Future Enhancement)

**Concept**: Change system to generate tools dynamically from providers on each request instead of caching in database.

**Pros**:
- Provider changes apply immediately
- No stale tool definitions
- Always in sync with provider code

**Cons**:
- Requires significant architecture changes
- Performance implications (more computation per request)
- Breaks existing caching strategy
- Not feasible for immediate fix

**Recommendation**: Consider for future refactoring, not for this issue.

---

## Recommended Action Plan

### Immediate (Fix User's Issue)

**1. Verify Current Tool Count**
```bash
# Count GitHub operations
curl -H "Authorization: Bearer <api-key>" \
  "http://localhost:8081/api/v1/tools?tenant_id=<tenant-id>&tool_type=github" \
  | jq '.tools | length'

# Count Harness operations
curl -H "Authorization: Bearer <api-key>" \
  "http://localhost:8081/api/v1/tools?tenant_id=<tenant-id>&tool_type=harness" \
  | jq '.tools | length'
```

**2. Re-Register GitHub Tool**
- Delete existing GitHub tool registration
- Re-register using optimized provider
- Verify tool count reduced to ~71 tools

**3. Optimize Harness Provider** (if not done yet)
- Apply same optimization pattern as GitHub
- Disable admin/platform modules
- Keep only developer workflow modules

**4. Re-Register Harness Tool**
- Delete existing Harness tool registration
- Re-register using optimized provider
- Verify tool count reduced to ~45 tools

### Short-Term (Next 1-2 Weeks)

**1. Document Re-Registration Process**
- Create admin guide for tool optimization
- Include troubleshooting steps
- Add to CHANGELOG.md

**2. Add Tool Count Monitoring**
- Log tool counts on registration
- Alert on unexpectedly high tool counts
- Dashboard metric for tool count per tenant

**3. Consider Migration Script**
- Automated script to re-register tools with optimization
- Preserve credentials during re-registration
- Batch process for multiple tenants

### Long-Term (Future Enhancement)

**1. Dynamic Tool Generation**
- Evaluate feasibility of generating tools on-demand
- Design caching strategy that respects provider changes
- Prototype and benchmark performance

**2. Tool Configuration Versioning**
- Track provider version in tool records
- Auto-detect when provider definitions change
- Prompt admin to refresh tools when updates available

---

## Technical Details

### GitHub Provider Tool Generation

**File**: `pkg/tools/providers/github/github_provider.go`

**Key Methods**:
```go
// Lines 150-350: initializeToolsets() - Creates all toolset definitions
// Lines 374-399: enableDefaultToolsets() - Enables only specified toolsets
// Lines 402-416: EnableToolset() - Marks toolset as enabled
// Lines 441-461: GetToolDefinitions() - Returns tools ONLY from enabled toolsets
```

**Tool Generation Logic**:
```go
func (p *GitHubProvider) GetToolDefinitions() []providers.ToolDefinition {
    var definitions []providers.ToolDefinition

    for _, toolset := range p.toolsetRegistry {
        if !toolset.Enabled {  // <-- KEY: Skips disabled toolsets
            continue
        }

        for _, tool := range toolset.Tools {
            // Add tool to definitions
        }
    }

    return definitions
}
```

### Tool Storage and Retrieval

**Storage** (`apps/rest-api/internal/services/dynamic_tools_service.go`):
```go
// Line 199: CreateTool() - Discovers and stores tools in database
// Line 201: discoveryService.DiscoverTool() - Calls provider
// Line 237-239: createGroupedTools() - Stores multiple tool records
```

**Retrieval** (`pkg/services/enhanced_tool_registry.go`):
```go
// Line 273: GetToolsForTenant() - Reads from database
// Line 290: dynamicToolRepo.List() - Database query
// Returns cached tool records, NOT fresh provider definitions
```

### Tool Count by Toolset (GitHub)

| Toolset | Tools | Status |
|---------|-------|--------|
| repos | 21 | ✅ Enabled |
| issues | 11 | ✅ Enabled |
| pull_requests | 13 | ✅ Enabled |
| actions | 13 | ✅ Enabled |
| security | 12 | ✅ Enabled |
| context | 1 | ✅ Enabled (always) |
| collaboration | 6 | ❌ Disabled |
| git | 10 | ❌ Disabled |
| organizations | 1 | ❌ Disabled |
| discussions | 4 | ❌ Disabled |
| **TOTAL (Old)** | **92** | All enabled in DB |
| **TOTAL (New)** | **71** | Only with re-registration |

---

## Next Steps for User

**Immediate Action Required**:

1. ✅ **Confirm Understanding**: Provider changes only affect NEW tool registrations
2. ✅ **Choose Option**: Re-register tools (Option 1 recommended)
3. ✅ **Backup**: Export current tool configurations if needed
4. ✅ **Execute**: Delete and re-register GitHub tool
5. ✅ **Optimize Harness**: Apply optimization to Harness provider
6. ✅ **Re-register Harness**: Delete and re-register Harness tool
7. ✅ **Verify**: Confirm tool count reduced to ≤150 tools

**Questions for User**:
1. Do you have access to the original GitHub/Harness credentials?
2. Are there any custom tool configurations that need to be preserved?
3. Would you like me to create a script to automate the re-registration?
4. Should I proceed with Harness optimization before re-registration?

---

## Summary

**Root Cause**: Tools are persisted in database at registration time. Provider optimizations only affect NEW registrations, not existing database records.

**Impact**: Current optimization code is correct but doesn't affect already-registered tools.

**Solution**: Re-register GitHub and Harness tools to leverage optimized providers.

**Expected Result After Re-Registration**:
- GitHub: 92 → 71 tools (23% reduction)
- Harness: 296 → 45 tools (85% reduction, after optimization)
- **Total: 388 → 116 tools (70% reduction)** ✅ Below 150 target

---

## Files for Reference

### Provider Implementation
- `pkg/tools/providers/github/github_provider.go` (lines 150-461)
- `pkg/tools/providers/harness/harness_provider.go` (needs optimization)

### Tool Registration
- `apps/rest-api/internal/services/dynamic_tools_service.go` (line 199)
- `pkg/services/enhanced_tool_registry.go` (line 273)

### Tool Retrieval (MCP)
- `apps/edge-mcp/internal/mcp/handler.go` (line 1095)
- `apps/edge-mcp/internal/core/client.go` (line 378)

### Database Schema
- Table: `dynamic_tools`
- Columns: `id`, `tenant_id`, `tool_name`, `tool_type`, `config`, `schema`

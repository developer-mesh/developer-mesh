# Harness.io Provider Implementation Plan

## Overview
Implement a comprehensive Harness.io provider for DevMesh that follows the templated tools pattern, provides intelligent permission filtering, and exposes granular MCP tools for AI agents.

## Architecture Analysis

### Harness API Structure
The Harness API is organized into multiple modules:
- **v1** (407 endpoints): Core platform APIs (pipelines, projects, organizations)
- **ng** (252 endpoints): Next-gen platform features
- **ccm** (141 endpoints): Cloud Cost Management
- **gitops** (110 endpoints): GitOps operations
- **iacm** (107 endpoints): Infrastructure as Code Management
- **code** (88 endpoints): Code repository management
- **pipeline** (67 endpoints): Pipeline-specific operations
- **cv** (65 endpoints): Continuous Verification
- **gateway** (65 endpoints): API Gateway management
- **sto** (22 endpoints): Security Testing Orchestration
- **cf** (22 endpoints): Feature Flags

### Authentication
- **Method**: API Key authentication via `x-api-key` header
- **Token Format**: Generated from Harness Platform user profile
- **Scope Discovery**: Will need to probe API endpoints to discover permissions

## Implementation Components

### 1. Provider Structure (`pkg/tools/providers/harness/`)
```
harness/
├── harness_provider.go          # Main provider implementation
├── harness_openapi.json          # Embedded OpenAPI spec (385KB)
├── operation_mappings.go         # Module-specific operation mappings
├── permission_discoverer.go      # Harness-specific permission discovery
└── ai_definitions.go             # AI-optimized tool definitions
```

### 2. Core Provider Features

#### Base Provider Implementation
- Extends `BaseProvider` with Harness-specific logic
- Implements `StandardToolProvider` interface
- Supports multiple API versions (v1, v2, ng)
- Automatic OpenAPI spec caching with fallback

#### Module-Based Tool Organization
Unlike GitHub's flat structure, Harness will use module-based organization:

```go
type HarnessModule string

const (
    ModulePipeline   HarnessModule = "pipeline"
    ModuleProject    HarnessModule = "project"
    ModuleConnector  HarnessModule = "connector"
    ModuleCCM        HarnessModule = "ccm"
    ModuleGitOps     HarnessModule = "gitops"
    ModuleIaCM       HarnessModule = "iacm"
    ModuleCV         HarnessModule = "cv"
    ModuleSTO        HarnessModule = "sto"
    ModuleFF         HarnessModule = "cf"
)
```

### 3. Operation Mappings

#### Pipeline Operations
```go
"pipelines/list": {
    PathTemplate: "/v1/orgs/{org}/projects/{project}/pipelines",
    RequiredParams: []string{"org", "project"},
    OptionalParams: []string{"page", "limit", "sort", "order"},
},
"pipelines/get": {
    PathTemplate: "/v1/orgs/{org}/projects/{project}/pipelines/{pipeline}",
    RequiredParams: []string{"org", "project", "pipeline"},
},
"pipelines/create": {
    PathTemplate: "/v1/orgs/{org}/projects/{project}/pipelines",
    Method: "POST",
    RequiredParams: []string{"org", "project", "name"},
},
"pipelines/execute": {
    PathTemplate: "/pipeline/api/pipeline/execute/{identifier}",
    Method: "POST",
    RequiredParams: []string{"identifier"},
},
"pipelines/validate": {
    PathTemplate: "/v1/orgs/{org}/projects/{project}/pipelines/{pipeline}/validate",
    Method: "POST",
    RequiredParams: []string{"org", "project", "pipeline"},
}
```

#### Project & Organization Operations
```go
"projects/list": {
    PathTemplate: "/v1/orgs/{org}/projects",
    RequiredParams: []string{"org"},
},
"projects/create": {
    PathTemplate: "/v1/orgs/{org}/projects",
    Method: "POST",
    RequiredParams: []string{"org", "identifier", "name"},
},
"orgs/list": {
    PathTemplate: "/v1/orgs",
},
"orgs/get": {
    PathTemplate: "/v1/orgs/{org}",
    RequiredParams: []string{"org"},
}
```

#### Connector Operations
```go
"connectors/list": {
    PathTemplate: "/v1/orgs/{org}/projects/{project}/connectors",
    RequiredParams: []string{"org", "project"},
},
"connectors/create": {
    PathTemplate: "/v1/orgs/{org}/projects/{project}/connectors",
    Method: "POST",
    RequiredParams: []string{"org", "project", "identifier", "type"},
},
"connectors/validate": {
    PathTemplate: "/ng/api/connectors/testConnection",
    Method: "POST",
    RequiredParams: []string{"identifier"},
}
```

### 4. AI-Optimized Tool Definitions

```go
AIOptimizedToolDefinition{
    Name: "harness_pipelines",
    Category: "CI/CD",
    Description: "Manage Harness CI/CD pipelines including creation, execution, and monitoring",
    SemanticTags: []string{"pipeline", "cicd", "deployment", "build", "workflow"},
    UsageExamples: []Example{
        {
            Scenario: "Execute a deployment pipeline",
            Input: map[string]interface{}{
                "action": "execute",
                "identifier": "deploy-prod",
                "inputs": map[string]interface{}{
                    "branch": "main",
                    "services": ["api", "frontend"],
                },
            },
        },
        {
            Scenario: "List all pipelines in a project",
            Input: map[string]interface{}{
                "action": "list",
                "org": "engineering",
                "project": "platform",
            },
        },
    },
    Capabilities: &ToolCapabilities{
        Capabilities: []Capability{
            {Action: "create", Resource: "pipelines"},
            {Action: "execute", Resource: "pipelines"},
            {Action: "monitor", Resource: "executions"},
            {Action: "rollback", Resource: "deployments"},
        },
        RateLimits: &RateLimitInfo{
            RequestsPerMinute: 100,
            Description: "Standard tier rate limits",
        },
    },
}
```

### 5. Permission Discovery Strategy

Since Harness doesn't expose OAuth scopes in headers like GitHub, we'll:

1. **Probe Key Endpoints**: Test access to different modules
2. **Parse Error Responses**: Extract permission info from 403 responses
3. **Cache Discovered Permissions**: Store module access per API key
4. **Progressive Discovery**: Start with core modules, expand as needed

```go
func (p *HarnessProvider) DiscoverPermissions(ctx context.Context, apiKey string) (*DiscoveredPermissions, error) {
    modules := []struct{
        name string
        endpoint string
    }{
        {"projects", "/v1/orgs"},
        {"pipelines", "/pipeline/api/pipelines/list"},
        {"connectors", "/ng/api/connectors/listV2"},
        {"ccm", "/ccm/api/graphql"},
        {"gitops", "/gitops/api/v1/agents"},
        {"sto", "/sto/api/v2/scans"},
    }
    
    permissions := &DiscoveredPermissions{
        Modules: make(map[string]bool),
    }
    
    for _, module := range modules {
        if hasAccess := p.probeEndpoint(ctx, module.endpoint, apiKey); hasAccess {
            permissions.Modules[module.name] = true
        }
    }
    
    return permissions, nil
}
```

### 6. Tool Expansion for MCP

Each Harness module expands into multiple MCP tools:

| Provider | Module | Expanded Tools |
|----------|--------|---------------|
| Harness | Pipelines | `harness_pipelines_list`, `harness_pipelines_create`, `harness_pipelines_execute`, `harness_pipelines_validate`, `harness_pipelines_approve` |
| Harness | Projects | `harness_projects_list`, `harness_projects_create`, `harness_projects_update`, `harness_projects_delete` |
| Harness | Connectors | `harness_connectors_list`, `harness_connectors_create`, `harness_connectors_validate`, `harness_connectors_test` |
| Harness | GitOps | `harness_gitops_apps_list`, `harness_gitops_sync`, `harness_gitops_rollback` |
| Harness | CCM | `harness_ccm_costs_get`, `harness_ccm_budgets_list`, `harness_ccm_recommendations` |
| Harness | STO | `harness_sto_scans_list`, `harness_sto_vulnerabilities_get`, `harness_sto_exemptions_create` |

### 7. Resilience Patterns

#### Circuit Breaker Configuration
```go
circuitBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "harness-api",
    MaxRequests: 3,
    Interval:    10 * time.Second,
    Timeout:     60 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
        return counts.Requests >= 3 && failureRatio >= 0.6
    },
    OnStateChange: func(name string, from, to gobreaker.State) {
        logger.Info("Circuit breaker state change", map[string]interface{}{
            "name": name,
            "from": from.String(),
            "to": to.String(),
        })
    },
})
```

#### Rate Limiting
- Respect Harness rate limits (varies by tier)
- Implement exponential backoff for 429 responses
- Queue requests during rate limit windows

### 8. Implementation Steps

1. **Create Provider Structure**
   - Copy GitHub provider as template
   - Adapt for Harness-specific needs
   - Embed OpenAPI spec

2. **Define Operation Mappings**
   - Map all critical operations for each module
   - Group by module for better organization
   - Include proper parameter definitions

3. **Implement Permission Discovery**
   - Create Harness-specific discoverer
   - Test with different API key permissions
   - Cache discovered permissions

4. **Build AI Definitions**
   - Create semantic descriptions for each module
   - Include real-world usage examples
   - Define capabilities and limitations

5. **Add Resilience Patterns**
   - Implement circuit breaker
   - Add rate limiting
   - Include retry logic

6. **Testing**
   - Unit tests for all operations
   - Integration tests with mock server
   - Permission filtering tests
   - Circuit breaker behavior tests

### 9. Database Migration

```sql
-- Add Harness templates
INSERT INTO mcp.tool_templates (
    id,
    provider_name,
    provider_version,
    display_name,
    description,
    category,
    required_credentials,
    is_public,
    is_active
) VALUES (
    gen_random_uuid(),
    'harness',
    'v1',
    'Harness Platform',
    'Complete Harness Software Delivery Platform integration',
    'CI/CD',
    ARRAY['api_key'],
    true,
    true
);
```

### 10. Configuration Example

```yaml
provider: harness
instance_name: harness-prod
display_name: "Harness Production"
config:
  base_url: "https://app.harness.io"
  account_id: "YOUR_ACCOUNT_ID"
  modules:
    - pipelines
    - projects
    - connectors
    - gitops
    - ccm
  rate_limits:
    requests_per_minute: 100
  circuit_breaker:
    enabled: true
    failure_threshold: 5
    timeout: 60s
credentials:
  api_key: "pat.YOUR_ACCOUNT_ID.xxxxxx"
```

## Benefits of This Implementation

1. **Module-Based Organization**: Aligns with Harness's architecture
2. **Intelligent Permission Filtering**: Only shows operations user can access
3. **AI-Friendly**: Clear semantic descriptions and examples
4. **Resilient**: Circuit breakers and rate limiting prevent cascading failures
5. **Scalable**: Can easily add new modules as Harness expands
6. **Cached**: OpenAPI spec and permissions cached for performance

## Next Steps

1. Implement the base provider structure
2. Create comprehensive operation mappings
3. Build permission discovery system
4. Add AI-optimized definitions
5. Test with real Harness accounts
6. Document usage patterns
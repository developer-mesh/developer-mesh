<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:39:28
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Multi-API Discovery Enhancement

## Overview
The system can discover and create tools for multiple APIs from a single documentation portal. When given a documentation portal like `https://apidocs.harness.io/`, it discovers and creates tools for all available APIs.

## Implementation

### 1. Enhanced Discovery Service

```go
// pkg/tools/adapters/multi_api_discovery.go

type MultiAPIDiscoveryResult struct {
    BaseURL         string
    DiscoveredAPIs  []APIDefinition
    Status          DiscoveryStatus
}

type APIDefinition struct {
    Name        string
    Description string
    SpecURL     string
    OpenAPISpec *openapi3.T
    Category    string // e.g., "Platform", "Chaos", "CI/CD"
}

func (s *DiscoveryService) DiscoverMultipleAPIs(ctx context.Context, baseURL string) (*MultiAPIDiscoveryResult, error) {
    // 1. Fetch the main page
    // 2. Look for patterns like:
    //    - Multiple OpenAPI/Swagger links
    //    - API categories or sections
    //    - Version-specific endpoints
    // 3. For each discovered API spec:
    //    - Download and parse it
    //    - Extract metadata (name, description)
    //    - Create an APIDefinition
    
    return result, nil
}
```

### 2. Pattern Recognition for API Portals

```go
// Common patterns for API documentation portals
var portalPatterns = []PortalPattern{
    {
        Name: "Harness",
        Patterns: []string{
            "/api/docs/spec/*",
            "/api/*/swagger.json",
            "/docs/api/*/spec.yaml",
        },
        CategoryExtractor: extractHarnessCategory,
    },
    {
        Name: "Generic",
        Patterns: []string{
            "*/openapi*.json",
            "*/swagger*.json",
            "*/api-docs/*",
        },
    },
}
```

### 3. Bulk Tool Creation

```go
func (s *DynamicToolsService) CreateToolsFromMultipleAPIs(
    ctx context.Context,
    tenantID string,
    result *MultiAPIDiscoveryResult,
    baseConfig tools.ToolConfig,
) ([]tools.Tool, error) {
    var createdTools []tools.Tool
    
    for _, api := range result.DiscoveredAPIs {
        // Create a tool for each discovered API
        toolConfig := baseConfig
        toolConfig.Name = fmt.Sprintf("%s - %s", baseConfig.Name, api.Name)
        toolConfig.OpenAPIURL = api.SpecURL
        
        tool, err := s.CreateTool(ctx, tenantID, toolConfig)
        if err != nil {
            // Log error but continue with other APIs
            continue
        }
        createdTools = append(createdTools, tool)
    }
    
    return createdTools, nil
}
```

### 4. Example Usage for Harness

When a user provides `https://apidocs.harness.io/`:

1. The system would discover:
   - Platform API: `https://apidocs.harness.io/api/platform/swagger.json`
   - Chaos API: `https://apidocs.harness.io/api/chaos/swagger.json`
   - CI/CD API: `https://apidocs.harness.io/api/ci/swagger.json`

2. Create separate tools:
   - "Harness - Platform API"
   - "Harness - Chaos API"
   - "Harness - CI/CD API"

### 5. API Endpoint for Bulk Discovery

```yaml
# swagger endpoint
/api/v1/tools/discover-multiple:
  post:
    summary: Discover and create multiple tools from an API portal
    requestBody:
      content:
        application/json:
          schema:
            type: object
            properties:
              portal_url:
                type: string
                example: "https://apidocs.harness.io/"
              name_prefix:
                type: string
                example: "Harness"
              auto_create:
                type: boolean
                default: false
                description: "If true, automatically create tools for all discovered APIs"
    responses:
      200:
        description: Discovery results
        content:
          application/json:
            schema:
              type: object
              properties:
                discovered_apis:
                  type: array
                  items:
                    type: object
                    properties:
                      name: string
                      spec_url: string
                      description: string
                created_tools:
                  type: array
                  items:
                    $ref: '#/components/schemas/Tool'
```

## Current Implementation Status

✅ **Implemented Features:**
1. **Enhanced HTML parsing** to recognize API portal patterns
2. **Category detection** to group related APIs
3. **Bulk tool creation** with proper error handling
4. **Portal patterns storage** in the learning system
5. **API endpoints** at `/api/v1/tools/discover-multiple` and `/api/v1/tools/discover-multiple/create`

⚠️ **Not Yet Implemented:**
- **UI support** for reviewing discovered APIs before creation (API-only currently)

## Benefits

- One-click integration with complex API ecosystems
- Automatic tool organization by category
- Reduced manual configuration

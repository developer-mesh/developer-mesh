# OpenAPI Sync Guide

## Overview

This guide explains how to keep the OpenAPI/Swagger documentation in sync with the actual API implementation in the Developer Mesh platform. Maintaining accurate API documentation is crucial for API consumers, SDK generation, and developer experience.

## OpenAPI Structure

### File Organization

```
docs/swagger/
├── openapi.yaml           # Main OpenAPI specification
├── core/                  # Core API endpoints
│   ├── agents.yaml       # Agent management
│   ├── auth.yaml         # Authentication
│   ├── contexts.yaml     # Context management
│   ├── embeddings_v2.yaml # Embedding operations
│   ├── health.yaml       # Health checks
│   ├── models.yaml       # Model management
│   ├── search.yaml       # Search operations
│   ├── tools.yaml        # Tool integration
│   ├── vectors.yaml      # Vector operations
│   ├── webhooks.yaml     # Webhook endpoints
│   ├── relationships.yaml # Relationship management
│   ├── workflows.yaml    # Workflow orchestration
│   └── tasks.yaml        # Task management
├── tools/                 # Tool-specific endpoints
│   └── github/
│       └── api.yaml      # GitHub tool operations
└── README.md             # Swagger documentation
```

### Modular Design

The OpenAPI specification is split into modules for maintainability:
- Main `openapi.yaml` imports endpoint definitions via `$ref`
- Each module focuses on a specific domain
- Shared schemas are defined in component files

## Synchronization Process

### 1. When Adding New Endpoints

When implementing a new API endpoint:

1. **Update the handler code** with Swagger annotations:
```go
// CreateAgent godoc
// @Summary Create a new AI agent
// @Description Register a new AI agent with the orchestration platform
// @Tags agents
// @Accept json
// @Produce json
// @Param agent body models.CreateAgentRequest true "Agent configuration"
// @Success 201 {object} models.Agent
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /api/v1/agents [post]
// @Security ApiKeyAuth
func (h *Handler) CreateAgent(c *gin.Context) {
    // Implementation
}
```

2. **Update the OpenAPI YAML** file:
```yaml
/agents:
  post:
    summary: Create a new AI agent
    description: Register a new AI agent with the orchestration platform
    tags:
      - agents
    security:
      - ApiKeyAuth: []
    requestBody:
      required: true
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/CreateAgentRequest'
    responses:
      '201':
        description: Agent created successfully
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Agent'
      '400':
        $ref: '#/components/responses/BadRequest'
      '409':
        $ref: '#/components/responses/Conflict'
```

### 2. When Modifying Endpoints

1. **Update handler annotations** to reflect changes
2. **Update YAML definitions** for:
   - Path parameters
   - Query parameters
   - Request body schema
   - Response schemas
   - Status codes

3. **Update examples** if request/response format changes

### 3. When Removing Endpoints

1. **Mark as deprecated** first:
```yaml
/old-endpoint:
  get:
    deprecated: true
    x-deprecation-date: "2024-03-01"
    x-removal-date: "2024-06-01"
    description: |
      **DEPRECATED**: This endpoint will be removed on 2024-06-01.
      Use `/api/v1/new-endpoint` instead.
```

2. **Remove after deprecation period**

## Validation Tools

### 1. Swagger Code Generation

Use `swaggo/swag` to generate OpenAPI from code annotations:

```bash
# Install swag
go install github.com/swaggo/swag/cmd/swag@latest

# Generate OpenAPI from annotations
swag init -g cmd/api/main.go -o docs/swagger/generated

# Compare with manual YAML
diff docs/swagger/openapi.yaml docs/swagger/generated/swagger.yaml
```

### 2. OpenAPI Validation Script

Create a validation script `scripts/validate-openapi.sh`:

```bash
#!/bin/bash

# Validate OpenAPI specification
npx @openapitools/openapi-generator-cli validate \
  -i docs/swagger/openapi.yaml

# Check for missing endpoints
echo "Checking for missing endpoints..."
grep -r "@Router" apps/*/internal/api --include="*.go" | \
  sed 's/.*@Router \(.*\) \[.*/\1/' | \
  sort -u > /tmp/code-endpoints.txt

yq eval '.paths | keys | .[]' docs/swagger/openapi.yaml | \
  sort -u > /tmp/openapi-endpoints.txt

echo "Endpoints in code but not in OpenAPI:"
comm -23 /tmp/code-endpoints.txt /tmp/openapi-endpoints.txt

echo "Endpoints in OpenAPI but not in code:"
comm -13 /tmp/code-endpoints.txt /tmp/openapi-endpoints.txt
```

### 3. Automated Testing

Add tests to verify OpenAPI accuracy:

```go
func TestOpenAPISync(t *testing.T) {
    // Load OpenAPI spec
    spec, err := loadOpenAPISpec("docs/swagger/openapi.yaml")
    require.NoError(t, err)
    
    // Test each endpoint exists
    router := setupRouter()
    for path, methods := range spec.Paths {
        for method := range methods {
            // Verify endpoint is registered
            assert.True(t, 
                router.HasRoute(method, path),
                "Missing route: %s %s", method, path,
            )
        }
    }
}
```

## Best Practices

### 1. Consistent Naming

- Use consistent naming for operations, parameters, and schemas
- Follow REST conventions:
  - GET `/resources` - List resources
  - GET `/resources/{id}` - Get single resource
  - POST `/resources` - Create resource
  - PUT `/resources/{id}` - Update resource
  - PATCH `/resources/{id}` - Partial update
  - DELETE `/resources/{id}` - Delete resource

### 2. Schema Definitions

Define reusable schemas in components:

```yaml
components:
  schemas:
    Agent:
      type: object
      required:
        - id
        - name
        - status
      properties:
        id:
          type: string
          format: uuid
          example: "550e8400-e29b-41d4-a716-446655440000"
        name:
          type: string
          example: "code-analyzer"
        status:
          $ref: '#/components/schemas/AgentStatus'
```

### 3. Examples

Provide realistic examples:

```yaml
examples:
  CreateAgentExample:
    value:
      name: "code-analyzer"
      type: "analyzer"
      capabilities:
        - "code_analysis"
        - "security_scan"
      endpoint: "ws://agent.internal:8080"
```

### 4. Error Responses

Standardize error responses:

```yaml
components:
  responses:
    BadRequest:
      description: Bad request
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
          example:
            error:
              code: "VALIDATION_ERROR"
              message: "Invalid agent configuration"
              details:
                field: "name"
                reason: "Name must be alphanumeric"
```

## Continuous Integration

### GitHub Actions Workflow

Create `.github/workflows/openapi-sync.yml`:

```yaml
name: OpenAPI Sync Check

on:
  pull_request:
    paths:
      - 'apps/**/api/**/*.go'
      - 'docs/swagger/**/*.yaml'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Install tools
        run: |
          go install github.com/swaggo/swag/cmd/swag@latest
          npm install -g @openapitools/openapi-generator-cli
      
      - name: Generate OpenAPI from code
        run: |
          swag init -g cmd/api/main.go -o /tmp/generated
      
      - name: Validate OpenAPI
        run: |
          openapi-generator-cli validate -i docs/swagger/openapi.yaml
      
      - name: Check sync
        run: |
          ./scripts/validate-openapi.sh
      
      - name: Comment on PR
        if: failure()
        uses: actions/github-script@v6
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: '⚠️ OpenAPI specification is out of sync with code. Please run `make swagger` to update.'
            })
```

## Makefile Commands

Add these commands to your Makefile:

```makefile
# Generate OpenAPI from code annotations
swagger-gen:
	@echo "Generating OpenAPI from code..."
	@swag init -g cmd/api/main.go -o docs/swagger/generated

# Validate OpenAPI specification
swagger-validate:
	@echo "Validating OpenAPI specification..."
	@npx @openapitools/openapi-generator-cli validate -i docs/swagger/openapi.yaml

# Check if OpenAPI is in sync with code
swagger-sync: swagger-gen
	@echo "Checking OpenAPI sync..."
	@./scripts/validate-openapi.sh

# Generate SDK from OpenAPI
swagger-sdk:
	@echo "Generating Go SDK..."
	@openapi-generator-cli generate \
		-i docs/swagger/openapi.yaml \
		-g go \
		-o pkg/client/generated \
		--additional-properties=packageName=mcpclient

# Start Swagger UI locally
swagger-ui:
	@echo "Starting Swagger UI at http://localhost:8082"
	@docker run -p 8082:8080 \
		-e SWAGGER_JSON=/api/openapi.yaml \
		-v $(PWD)/docs/swagger:/api \
		swaggerapi/swagger-ui
```

## Common Issues and Solutions

### 1. Path Parameter Mismatch

**Issue**: Path parameter in code doesn't match OpenAPI
```go
// Code: /agents/:agentId
// OpenAPI: /agents/{id}
```

**Solution**: Use consistent parameter names:
```go
// @Router /api/v1/agents/{id} [get]
// @Param id path string true "Agent ID"
r.GET("/agents/:id", handler.GetAgent)
```

### 2. Missing Security Definitions

**Issue**: Endpoint requires auth but OpenAPI doesn't specify
```yaml
# Missing security section
/protected-endpoint:
  get:
    summary: Protected endpoint
```

**Solution**: Add security requirements:
```yaml
/protected-endpoint:
  get:
    summary: Protected endpoint
    security:
      - ApiKeyAuth: []
      - BearerAuth: []
```

### 3. Schema Drift

**Issue**: Model changes not reflected in OpenAPI
```go
// Added new field to Agent model
type Agent struct {
    ID           string    `json:"id"`
    Name         string    `json:"name"`
    WorkloadInfo *Workload `json:"workload"` // New field
}
```

**Solution**: Update schema definition:
```yaml
Agent:
  type: object
  properties:
    id:
      type: string
    name:
      type: string
    workload:
      $ref: '#/components/schemas/Workload'
```

## Version Management

### API Versioning Strategy

1. **URL Versioning**: `/api/v1/`, `/api/v2/`
2. **Deprecation Headers**: 
   ```yaml
   x-api-deprecated: "true"
   x-api-deprecation-date: "2024-03-01"
   x-api-sunset-date: "2024-06-01"
   ```

3. **Migration Guides**: Document breaking changes

### OpenAPI Version Updates

When releasing new API versions:

1. Create new version directory:
   ```
   docs/swagger/
   ├── v1/
   │   └── openapi.yaml
   └── v2/
       └── openapi.yaml
   ```

2. Maintain both versions during transition
3. Update SDK generation for each version

## References

- [OpenAPI Specification](https://swagger.io/specification/)
- [Swaggo Documentation](https://github.com/swaggo/swag)
- [OpenAPI Generator](https://openapi-generator.tech/)
- [Swagger UI](https://swagger.io/tools/swagger-ui/)

---

*For more information, visit [docs.developer-mesh.com](https://docs.developer-mesh.com)*
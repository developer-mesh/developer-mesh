<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:33:14
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Swagger/OpenAPI Integration Guide

This guide explains how the Developer Mesh Platform uses Swagger/OpenAPI for API documentation and how to maintain and extend it as new tools are integrated.

## Architecture Overview

### 1. Documentation Structure

```
docs/swagger/
├── openapi.yaml              # Main entry point
├── common/                   # Shared components
│   ├── schemas.yaml         # Reusable data models
│   ├── parameters.yaml      # Common parameters
│   └── responses.yaml       # Standard responses
├── core/                    # Core API specs
│   ├── auth.yaml           # Authentication endpoints
│   ├── agents.yaml         
│   ├── contexts.yaml       
│   ├── health.yaml         
│   ├── models.yaml         
│   ├── relationships.yaml  
│   ├── search.yaml         
│   ├── tools.yaml          
│   ├── vectors.yaml        
│   └── webhooks.yaml       
└── tools/                   # Tool-specific specs
    ├── github/             
    ├── harness/            
    ├── sonarqube/          
    ├── artifactory/        
    └── xray/               
```

### 2. Integration Methods

#### Method 1: OpenAPI Specification Files (Recommended)
- **Pros**: Clean separation, supports $ref, works with any framework
- **Cons**: Manual sync required between code and specs
- **Use Case**: Complex APIs, multiple services, tool-specific documentation

#### Method 2: Code Annotations (swaggo)
- **Pros**: Documentation lives with code, auto-generated
- **Cons**: Limited OpenAPI 3.0 support, Go-specific
- **Use Case**: Simple APIs, single service documentation

#### Method 3: Hybrid Approach (Current Implementation)
- OpenAPI specs for comprehensive documentation
- Swaggo annotations for quick inline documentation
- Manual sync ensures accuracy

## Adding Documentation for New Tools

### Step 1: Create Tool Directory

```bash
mkdir -p docs/swagger/tools/sonarqube
```

### Step 2: Create Tool API Specification

Create `docs/swagger/tools/sonarqube/api.yaml`:

```yaml
paths:
  /tools/sonarqube:
    get:
      tags:
        - SonarQube
      summary: List SonarQube MCP tools
      operationId: listSonarQubeTools
      # ... rest of specification
```

### Step 3: Create Tool Schemas

Create `docs/swagger/tools/sonarqube/schemas.yaml`:

```yaml
components:
  schemas:
    SonarQubeProject:
      type: object
      properties:
        key:
          type: string
        name:
          type: string
        qualifier:
          type: string
        # ... properties
```

### Step 4: Update Main OpenAPI File

Add to `docs/swagger/openapi.yaml`:

```yaml
tags:
  - name: SonarQube
    description: SonarQube code quality operations

paths:
  /tools/sonarqube:
    $ref: './tools/sonarqube/api.yaml#/paths/~1tools~1sonarqube'
```

### Step 5: Add Code Annotations

In your handler file:

```go
// @Summary Execute SonarQube tool
// @Description Execute a SonarQube code quality operation
// @Tags SonarQube
// @Accept json
// @Produce json
// @Param tool_name path string true "SonarQube tool name"
// @Param request body SonarQubeExecutionRequest true "Execution request"
// @Success 200 {object} SonarQubeExecutionResponse
// @Router /tools/sonarqube/{tool_name} [post]
func executeSonarQubeTool(c *gin.Context) {
    // Implementation
}
```

## Authentication Documentation

### Security Schemes

Define authentication methods in the main OpenAPI file:

```yaml
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: Authorization
      description: API key authentication
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: JWT bearer token authentication

# Apply globally
security:
  - ApiKeyAuth: []
  - BearerAuth: []
```

### Documenting Auth Endpoints

Authentication endpoints should be clearly marked:

```yaml
/auth/login:
  post:
    tags:
      - Authentication
    summary: Authenticate user
    security: []  # No auth required for login
    responses:
      '200':
        description: Login successful
        headers:
          X-RateLimit-Limit:
            $ref: '#/components/headers/RateLimitLimit'
```

### Rate Limiting Documentation

Always document rate limiting headers:

```yaml
responses:
  '429':
    description: Rate limit exceeded
    headers:
      X-RateLimit-Limit:
        description: Request limit per window
        schema:
          type: integer
      Retry-After:
        description: Seconds until retry
        schema:
          type: integer
```

## Best Practices

### 1. Use References Extensively

```yaml
# Bad - Duplication
/tools/github/create_issue:
  post:
    responses:
      '401':
        description: Unauthorized
        content:
          application/json:
            schema:
              type: object
              properties:
                error:
                  type: string

# Good - Reference
/tools/github/create_issue:
  post:
    responses:
      '401':
        $ref: '../../common/responses.yaml#/components/responses/Unauthorized'
```

### 2. Provide Rich Examples

```yaml
requestBody:
  content:
    application/json:
      schema:
        $ref: '#/components/schemas/PipelineRequest'
      examples:
        simple_pipeline:
          summary: Simple build pipeline
          value:
            name: "build-and-test"
            steps: ["checkout", "build", "test"]
        complex_pipeline:
          summary: Full CI/CD pipeline
          value:
            name: "deploy-production"
            steps: ["checkout", "build", "test", "deploy"]
            environment: "prod"
            approvals_required: true
```

### 3. Document All Error Scenarios

```yaml
responses:
  '200':
    description: Success
  '400':
    $ref: '#/components/responses/BadRequest'
  '401':
    $ref: '#/components/responses/Unauthorized'
  '403':
    description: Pipeline requires approval
    content:
      application/json:
        schema:
          $ref: '#/components/schemas/ApprovalRequired'
  '409':
    description: Pipeline already running
  '422':
    description: Invalid pipeline configuration
```

### 4. Use Consistent Naming

- Operations: `list{Tool}Tools`, `get{Tool}ToolSchema`, `execute{Tool}Tool`
- Schemas: `{Tool}Pipeline`, `{Tool}Execution`, `{Tool}Result`
- Parameters: Use common parameters from `common/parameters.yaml`

### 5. Version Tool APIs Independently

```yaml
x-tool-version: "1.2.0"
x-tool-deprecated-operations:
  - name: "gitlab_create_merge_request"
    deprecated: "2024-01-15"
    sunset: "2024-07-15"
    alternative: "gitlab_create_pull_request"
```

## Validation and Testing

### 1. Validate OpenAPI Spec

```bash
# Install validator
npm install -g @apidevtools/swagger-cli

# Validate
swagger-cli validate docs/swagger/openapi.yaml
```

### 2. Test with Mock Server

```bash
# Install Prism
npm install -g @stoplight/prism-cli

# Run mock server
prism mock docs/swagger/openapi.yaml
```

### 3. Generate SDKs

```bash
# Generate Go client
openapi-generator generate \
  -i docs/swagger/openapi.yaml \
  -g go \
  -o sdk/go \
  --additional-properties=packageName=mcpclient

# Generate Python client  
openapi-generator generate \
  -i docs/swagger/openapi.yaml \
  -g python \
  -o sdk/python \
  --additional-properties=packageName=mcp_client
```

## Swagger UI Configuration

### 1. Serve Documentation

```bash
# Using Make
make swagger-serve

# Using Docker
docker run -p 8080:8080 \
  -e SWAGGER_JSON=/docs/openapi.yaml \
  -v $(pwd)/docs/swagger:/docs \
  swaggerapi/swagger-ui
```

### 2. Configure in Application

```go
// In server setup
func SetupSwaggerDocs(router *gin.Engine) {
    // Serve OpenAPI specs
    router.Static("/docs/swagger", "./docs/swagger")
    
    // Swagger UI
    url := ginSwagger.URL("/docs/swagger/openapi.yaml")
    router.GET("/swagger/*any", ginSwagger.WrapHandler(
        swaggerFiles.Handler, url))
}
```

### 3. Enable Try-It-Out

Configure CORS for Swagger UI:

```go
corsConfig := cors.Config{
    AllowOrigins:     []string{"*"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Authorization", "Content-Type"},
    ExposeHeaders:    []string{"X-Total-Count"},
    AllowCredentials: true,
}
```

## Maintenance Workflow

### 1. When Adding Endpoints

1. Update OpenAPI spec first
2. Implement endpoint
3. Add swaggo annotations
4. Validate specification
5. Update SDK if needed

### 2. Review Checklist

- [ ] All endpoints documented
- [ ] Examples provided
- [ ] Error responses defined
- [ ] Security requirements specified
- [ ] Rate limits documented
- [ ] Deprecation notices added
- [ ] Changelog updated

### 3. CI/CD Integration

```yaml
# .github/workflows/api-docs.yml
name: API Documentation
on: [push, pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Validate OpenAPI
        run: |
          npm install -g @apidevtools/swagger-cli
          swagger-cli validate docs/swagger/openapi.yaml
      
      - name: Generate Swagger
        run: |
          make swagger
          
      - name: Check for changes
        run: |
          git diff --exit-code docs/
```

## Common Issues and Solutions

### 1. Circular References

**Problem**: `$ref` cycles cause validation errors

**Solution**: Use `allOf` pattern:
```yaml
# Instead of circular reference
ContextWithAgent:
  allOf:
    - $ref: '#/components/schemas/Context'
    - type: object
      properties:
        agent:
          $ref: '#/components/schemas/AgentSummary'
```

### 2. Path Parameter Encoding

**Problem**: Special characters in paths need escaping

**Solution**: Use `~1` for `/` in $ref:
```yaml
$ref: './tools/github/api.yaml#/paths/~1tools~1github~1{tool_name}'
```

### 3. Inconsistent Models

**Problem**: Same concept with different schemas

**Solution**: Create shared schemas in `common/schemas.yaml`

### 4. Large Specifications

**Problem**: Single file becomes unwieldy

**Solution**: Split by domain and use references

## Future Enhancements

1. **AsyncAPI**: Document webhook events and async operations
2. **GraphQL**: Add GraphQL schema for advanced queries
3. **gRPC**: Support Protocol Buffers for internal services
4. **Postman**: Auto-generate Postman collections
5. **API Gateway**: Export for AWS API Gateway or Kong
6. **Documentation Portal**: Build custom documentation site

## Resources

- [OpenAPI Specification](https://spec.openapis.org/oas/v3.0.3)
- [Swagger Editor](https://editor.swagger.io/)
- [swaggo Documentation](https://github.com/swaggo/swag)
- [OpenAPI Generator](https://openapi-generator.tech/)

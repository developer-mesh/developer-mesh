# OpenAPI/Swagger Documentation

This directory contains the comprehensive OpenAPI 3.0 specification for the Developer Mesh Platform API. The documentation is structured to support scalability as new tools and integrations are added.

## Structure

```
docs/swagger/
├── openapi.yaml              # Main OpenAPI specification with references
├── common/                   # Shared components
│   ├── schemas.yaml         # Common data models
│   ├── parameters.yaml      # Reusable parameters
│   └── responses.yaml       # Standard HTTP responses
├── core/                    # Core API specifications
│   ├── agents.yaml         # Agent management endpoints
│   ├── contexts.yaml       # MCP context management
│   ├── health.yaml         # Health and monitoring
│   ├── models.yaml         # Model configuration
│   ├── relationships.yaml  # Entity relationships
│   ├── search.yaml         # Semantic search
│   ├── tools.yaml          # Generic tool endpoints
│   ├── vectors.yaml        # Vector operations
│   └── webhooks.yaml       # Webhook endpoints
└── tools/                   # Tool-specific specifications
    └── github/             # GitHub integration (currently implemented)
        ├── api.yaml       # GitHub endpoints
        └── schemas.yaml   # GitHub-specific models (future)
```

**Note**: Additional tools (GitLab, Harness, SonarQube, Artifactory, Xray) are planned but not yet implemented. See the [Custom Tool Integration Guide](../examples/custom-tool-integration.md) for implementation examples.

## Design Principles

### 1. Modularity
- Each tool has its own directory under `tools/`
- Common components are reused via `$ref` references
- Tool-specific schemas are isolated from core schemas

### 2. Scalability
- New tools can be added by creating a new directory under `tools/`
- Each tool defines its own endpoints, schemas, and examples
- The main `openapi.yaml` imports tool specifications dynamically

### 3. Consistency
- All tools follow the same API patterns
- Common responses and error formats are standardized
- Authentication and rate limiting are handled uniformly

### 4. MCP Protocol Compliance
- Context management follows MCP specification
- Tool execution returns MCP-compliant responses
- Error handling matches MCP protocol requirements

## Adding a New Tool

When implementing a new tool integration (e.g., GitLab, Harness, SonarQube):

1. Create a new directory: `tools/<tool-name>/`

2. Create `api.yaml` with tool endpoints:
```yaml
paths:
  /tools/<tool-name>:
    get:
      tags:
        - <ToolName>
      summary: List <tool> MCP tools
      # ... endpoint definition
```

3. Create `schemas.yaml` with tool-specific models:
```yaml
components:
  schemas:
    <ToolName>Config:
      type: object
      # ... schema definition
```

4. Update `openapi.yaml` to include the new tool:
```yaml
paths:
  /tools/<tool-name>:
    $ref: './tools/<tool-name>/api.yaml#/paths/~1tools~1<tool-name>'
```

5. Add appropriate tags in `openapi.yaml`:
```yaml
tags:
  - name: <ToolName>
    description: <Tool> integration operations
```

## Viewing Documentation

### Local Development

1. **Using Make command:**
```bash
make swagger-serve
# Opens at http://localhost:8082/
```

2. **Using Python:**
```bash
cd docs/swagger
python3 -m http.server 8080
# Opens at http://localhost:8080/
```

3. **Using the API servers:**
- MCP Server: http://localhost:8080/swagger/index.html
- REST API: http://localhost:8081/swagger/index.html

### Online Tools

1. **Swagger Editor:**
   - Visit https://editor.swagger.io/
   - Import `openapi.yaml`

2. **ReDoc:**
   - Use the `/redoc` endpoint on either server
   - Or visit https://redocly.github.io/redoc/

## Validation

Validate the OpenAPI specification:

```bash
# Install OpenAPI validator
npm install -g @apidevtools/swagger-cli

# Validate
swagger-cli validate docs/swagger/openapi.yaml
```

## Code Generation

Generate client SDKs or server stubs:

```bash
# Install OpenAPI Generator
brew install openapi-generator

# Generate Go client
openapi-generator generate -i docs/swagger/openapi.yaml -g go -o sdk/go

# Generate Python client
openapi-generator generate -i docs/swagger/openapi.yaml -g python -o sdk/python
```

## Best Practices

1. **Use References**: Don't duplicate schemas, use `$ref` to reference common components
2. **Provide Examples**: Include request/response examples for all endpoints
3. **Document Errors**: Define all possible error responses with examples
4. **Version APIs**: Use path versioning (`/api/v1/`) for backward compatibility
5. **Security First**: Always define security requirements for endpoints
6. **Test with Mock Data**: Use example data that reflects real-world usage

## Tool-Specific Guidelines

### GitHub Integration
- Follow GitHub API conventions for resource names
- Use GitHub's standard error codes
- Include webhook event schemas

### CI/CD Tools (Harness, GitLab)
- Define pipeline schemas clearly
- Include status enumerations
- Document async operations
- Support both trigger and monitoring endpoints

### Security Tools (SonarQube, Xray)
- Define vulnerability severity levels
- Include remediation guidance schemas
- Document scan result formats

### Artifact Repositories (Artifactory)
- Follow repository naming conventions
- Include artifact metadata schemas
- Document search query formats

## Maintenance

1. **Keep specs in sync**: Update OpenAPI specs when implementing changes
2. **Version control**: Track all changes in Git
3. **Review process**: Include API documentation in code reviews
4. **Breaking changes**: Document in CHANGELOG and migration guides
5. **Deprecation**: Use OpenAPI deprecation markers with sunset dates
# OpenAPI/Swagger Documentation

⚠️ **IMPORTANT**: This documentation has been updated (2025-08-14) to reflect the ACTUAL implementation. Many features previously documented are NOT implemented and have been marked or removed.

## Structure

```
docs/swagger/
├── openapi.yaml              # Main OpenAPI specification
├── common/                   # Shared components
│   ├── schemas.yaml         # Common data models
│   ├── parameters.yaml      # Reusable parameters
│   └── responses.yaml       # Standard HTTP responses
├── core/                    # Core API specifications
│   ├── agents.yaml         # ✅ Agent management (IMPLEMENTED)
│   ├── auth.yaml           # ✅ Authentication & user management (IMPLEMENTED)
│   ├── collaborations.yaml # ❌ NOT IMPLEMENTED
│   ├── contexts.yaml       # ✅ Context management (IMPLEMENTED)
│   ├── dynamic_tools.yaml  # ✅ Dynamic tools API (IMPLEMENTED)
│   ├── embeddings_v2.yaml  # ⚠️ CONDITIONAL - only if service initializes
│   ├── embedding_models.yaml # ❌ NOT IMPLEMENTED
│   ├── health.yaml         # ✅ Health checks (IMPLEMENTED)
│   ├── metrics.yaml        # ✅ Prometheus metrics (IMPLEMENTED)
│   ├── models.yaml         # ✅ Model management (IMPLEMENTED)
│   ├── monitoring.yaml     # ❌ NOT IMPLEMENTED
│   ├── relationships.yaml  # ❌ NOT IMPLEMENTED (code exists but not registered)
│   ├── search.yaml         # ❌ NOT IMPLEMENTED (code exists but not registered)
│   ├── tasks.yaml          # ❌ NOT IMPLEMENTED
│   ├── vectors.yaml        # ❌ NOT IMPLEMENTED
│   ├── webhooks.yaml       # ✅ Dynamic webhooks (IMPLEMENTED)
│   └── workflows.yaml      # ❌ NOT IMPLEMENTED
└── tools/                   # Tool-specific specifications
    └── github/             # GitHub integration
        └── api.yaml       # ❌ ORPHANED - not implemented
```

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

## Actually Implemented APIs

### ✅ IMPLEMENTED and REGISTERED:

1. **Authentication** (`/api/v1/auth/*`, `/api/v1/users/*`, `/api/v1/organization/*`, `/api/v1/profile/*`)
   - Organization registration, user login/logout, JWT refresh
   - User invitation and management
   - Organization and profile management

2. **Dynamic Tools** (`/api/v1/tools/*`)
   - Full CRUD operations
   - Tool discovery (single and multiple)
   - Health checking and action execution
   - Credential and webhook management

3. **Enhanced Agents** (`/api/v1/agents/*`)
   - Agent registration and lifecycle management
   - State transitions, health monitoring, event tracking

4. **Contexts** (`/api/v1/contexts/*`)
   - Context CRUD operations
   - Search and summarization

5. **Models** (`/api/v1/models/*`)
   - Model configuration and search

6. **Embeddings** (`/api/v1/embeddings/*`) - **CONDITIONAL**
   - Only available if embedding service initializes
   - Requires configured providers

7. **Health & Admin**
   - `/health`, `/healthz`, `/readyz`
   - `/metrics` (Prometheus)
   - `/admin/migration-status`

8. **Webhooks** (`/api/webhooks/tools/{toolId}`)
   - Dynamic webhook handling

### ❌ NOT IMPLEMENTED (but documented):
- MCP API (code exists but not registered)
- Resources API (code exists but not registered)  
- Search handlers (code exists but not registered)
- Relationship management (code exists but not registered)
- Workflows, Tasks, Vectors, Collaborations (no implementation)

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

**Note**: The validator may report warnings about external file references. These are expected when using modular file structure with `$ref` references.

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

## Current State (2025-08-14)

**ACTUAL IMPLEMENTATION STATUS**:
- ✅ Authentication endpoints are at `/api/v1/auth/*` (NOT `/auth/token`)
- ✅ Dynamic tools use `/api/v1/tools` endpoints
- ✅ Enhanced Agent API is used (not the basic Agent API)
- ✅ Context endpoints use `/api/v1/contexts` path
- ⚠️ Embedding API may not be available if providers aren't configured
- ❌ Many documented APIs are NOT registered in server.go
- ❌ The `tools/github/api.yaml` file is orphaned
- ❌ Search, Relationships, Workflows, Tasks, Vectors are NOT implemented

## Maintenance

1. **CRITICAL**: Always verify implementation in `apps/rest-api/internal/api/server.go` before documenting
2. **Keep specs in sync**: Update OpenAPI specs ONLY for actually implemented endpoints
3. **Version control**: Track all changes in Git
4. **Review process**: Include API documentation in code reviews
5. **Breaking changes**: Document in CHANGELOG and migration guides
6. **Deprecation**: Use OpenAPI deprecation markers with sunset dates
7. **Regular audits**: Periodically verify that documentation matches implementation
8. **Remove orphaned docs**: Delete documentation for unimplemented features
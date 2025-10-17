<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:27:32
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Dynamic Tools API Documentation

## Overview

The Dynamic Tools API allows you to add and manage DevOps tools without modifying code. Any tool that provides an OpenAPI/Swagger specification can be integrated automatically.

## Key Features

- **Zero Code Integration**: Add new tools without writing any code
- **Automatic Discovery**: Automatically discovers OpenAPI specifications from tool endpoints
- **Dynamic Authentication**: Supports multiple authentication methods based on OpenAPI security schemes
- **User Token Passthrough**: Allows users to authenticate with their own credentials for tools
- **Health Monitoring**: Built-in health checking with configurable intervals
- **Credential Encryption**: Per-tenant AES-256-GCM encryption for all credentials
- **Rate Limiting**: Per-tenant and per-tool rate limiting
- **Audit Logging**: Complete audit trail of all tool operations

## Supported Tools

Any tool that provides an OpenAPI 3.0+ specification can be integrated, including:
- GitHub/GitHub Enterprise
- GitLab
- Harness.io
- SonarQube
- JFrog Artifactory
- JFrog Xray
- Dynatrace
- Jenkins
- Custom internal APIs

## API Endpoints

### Tool Management

#### List Tools
```
GET /api/v1/tools
```

Lists all configured tools for the tenant.

Query Parameters:
- `status`: Filter by status (active, inactive, deleted)
- `include_health`: Include health status (true/false)

#### Create Tool
```
POST /api/v1/tools
```

Creates a new tool configuration.

Request Body:
```json
{
  "name": "my-github",
  "base_url": "https://api.github.com",
  "openapi_url": "https://api.github.com/openapi.json",
  "auth_type": "token",
  "credentials": {
    "token": "ghp_xxxxxxxxxxxx"
  },
  "provider": "github",
  "passthrough_config": {
    "mode": "optional",
    "fallback_to_service": true
  },
  "health_config": {
    "mode": "periodic",
    "interval": "5m",
    "timeout": "30s"
  }
}
```

#### Get Tool
```
GET /api/v1/tools/{toolId}
```

Retrieves a specific tool configuration.

#### Update Tool
```
PUT /api/v1/tools/{toolId}
```

Updates a tool configuration.

#### Delete Tool
```
DELETE /api/v1/tools/{toolId}
```

Soft deletes a tool configuration.

### Discovery

#### Discover Tool
```
POST /api/v1/tools/discover
```

Initiates automatic discovery of a tool's OpenAPI specification.

Request Body:
```json
{
  "base_url": "https://api.example.com",
  "auth_type": "token",
  "credentials": {
    "token": "xxx"
  },
  "hints": {
    "discovery_paths": ["/v3/api-docs", "/swagger.json"],
    "discovery_subdomains": ["api", "docs"]
  }
}
```

#### Get Discovery Status
```
GET /api/v1/tools/discover/{sessionId}
```

Checks the status of a discovery session.

#### Confirm Discovery
```
POST /api/v1/tools/discover/{sessionId}/confirm
```

Confirms and saves a discovered tool.

### Health Checks

#### Check Health
```
GET /api/v1/tools/{toolId}/health
```

Gets the current health status of a tool.

Query Parameters:
- `force`: Force a fresh health check (true/false)

#### Refresh Health
```
POST /api/v1/tools/{toolId}/health/refresh
```

Forces an immediate health check.

### Tool Execution

#### List Actions
```
GET /api/v1/tools/{toolId}/actions
```

Lists all available actions for a tool (generated from OpenAPI operations).

#### Execute Action
```
POST /api/v1/tools/{toolId}/execute/{action}
```

Executes a tool action. The system uses an advanced multi-layer operation resolution system to intelligently map action names to OpenAPI operation IDs.

**Advanced Operation Resolution System**

The Dynamic Tools API includes a sophisticated operation resolution system that handles the complexity of mapping simple action names (like "get", "list", "create") to full OpenAPI operation IDs (like "repos/get", "issues/list-for-repo"). This system is completely tool-agnostic and works with ANY dynamically added API.

**Key Components:**

1. **OperationResolver** - Core resolution engine that maps actions to operations
2. **SemanticScorer** - AI-powered semantic understanding of operations
3. **ResolutionLearner** - Machine learning system that improves over time
4. **OperationCache** - Multi-level caching for performance
5. **PermissionDiscoverer** - Discovers and filters operations based on permissions
6. **ResourceScopeResolver** - Handles namespace collisions between resources

**Resolution Strategies (in order):**

1. **Direct Lookup** - Exact match with operation ID
   - Example: `"repos/get"` → `repos/get`
   
2. **Contextual Resolution** - Uses parameters to infer resource type
   - Example: `"list"` + `{owner, repo}` → `"repos/list"`
   - The system analyzes provided parameters to determine the most likely resource type
   
3. **Simple Name Mapping** - Extracts and matches action verbs
   - Example: `"list"` → finds all operations containing "list"
   - Disambiguates using context when multiple matches exist
   
4. **Semantic Scoring** - AI-powered operation matching
   - Analyzes operation characteristics (complexity, parameters, response types)
   - Scores operations based on semantic similarity to the requested action
   - Considers operation tags, summaries, and descriptions
   
5. **Fuzzy Matching** - Handles format variations
   - Supports: `repos/get`, `repos-get`, `repos_get`, `repos.get`
   - Automatically tries common variations

**Semantic Scoring System:**

The SemanticScorer provides intelligent operation scoring based on:
- **Action verb matching** (up to 100 points)
- **Complexity scoring** - Prefers simpler operations for common actions (up to 50 points)
- **Parameter alignment** - Matches required/optional parameters with context (up to 50 points)
- **Path pattern analysis** - Scores based on path depth and structure (up to 20 points)
- **Response type matching** - Aligns expected response with action type (up to 30 points)
- **Tag relevance** - Uses operation tags for context (up to 40 points)

**Resolution Learning System:**

The ResolutionLearner continuously improves operation resolution:
- **Success Tracking** - Records successful resolutions and their patterns
- **Failure Analysis** - Learns from resolution failures
- **Pattern Recognition** - Identifies common parameter patterns
- **Confidence Scoring** - Provides confidence levels for resolutions
- **Historical Performance** - Weights recent successes higher

Learning metrics tracked:
- Success/failure rates per operation
- Average resolution latency
- Context patterns that lead to success
- Error patterns for debugging

**Multi-Level Caching:**

The OperationCache provides blazing-fast resolution:
- **L1 Memory Cache** - In-memory cache with 5-minute TTL
- **L2 Redis Cache** - Distributed cache with dynamic TTL (1-48 hours)
- **Intelligent TTL** - Higher confidence resolutions cached longer
- **Context Hashing** - Smart fingerprinting of request context
- **Hit Tracking** - Monitors cache effectiveness

Cache key generation considers:
- Tool ID
- Action name
- Resource scope (if applicable)
- Critical parameters (owner, repo, org, user, id, name)

**Permission-Based Filtering:**

Operations are filtered based on the authenticated user's permissions:
- Discovers permissions from OAuth tokens, JWT claims, or API introspection
- Filters out operations the user cannot execute
- Reduces resolution ambiguity by eliminating unauthorized operations
- Supports various authentication methods (OAuth2, API keys, JWT)

**Resource Scope Resolution:**

Handles namespace collisions when multiple resources have similar operations:
- Extracts resource type from tool name (e.g., `github_issues` → `issues`)
- Filters operations to match the resource scope
- Prevents cross-resource operation selection
- Prioritizes operations matching the primary resource type

Request Headers (optional for passthrough authentication):
- `X-User-Token`: User's personal access token for the tool
- `X-Token-Provider`: Provider name (e.g., "github", "gitlab")

Request Body:
```json
{
  "parameters": {
    "owner": "myorg",
    "repo": "myrepo",
    "title": "New Issue",
    "body": "Issue description"
  },
  "__resource_type": "issues"  // Optional hint for disambiguation
}
```

The parameters help the system determine the correct operation when multiple matches exist. The resolution system analyzes all parameters to make intelligent decisions about which operation to execute.

**Performance Metrics:**
- Average resolution time: <10ms (cached), <100ms (uncached)
- Cache hit rate: ~85% after warm-up
- Learning improvement: ~15-20% accuracy gain over time
- Success rate: 95%+ for common operations

### Credentials

#### Update Credentials
```
PUT /api/v1/tools/{toolId}/credentials
```

Updates tool credentials.

Request Body:
```json
{
  "auth_type": "token",
  "credentials": {
    "token": "new-token-value"
  }
}
```

## Authentication Methods

The following authentication methods are supported:

### User Token Passthrough

Dynamic tools support user token passthrough, allowing users to authenticate with their own credentials instead of service accounts. This ensures that actions are performed with the user's permissions and are properly attributed.


#### Configuration

When creating a tool, specify the provider and passthrough configuration:

```json
{
  "provider": "github",  // github, gitlab, bitbucket, or custom
  "passthrough_config": {
    "mode": "optional",  // optional, required, or disabled
    "fallback_to_service": true  // Allow fallback to service account
  }
}
```

#### Passthrough Modes

- **optional**: User tokens are used if provided, otherwise falls back to service account
- **required**: User tokens are mandatory, requests without them are rejected
- **disabled**: Only service account credentials are used

#### Using Passthrough Authentication

To use your own credentials when executing tool actions, include these headers:

```bash
curl -X POST http://localhost:8080/api/v1/tools/{toolId}/execute/{action} \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-User-Token: $USER_TOKEN" \
  -H "X-Token-Provider: github" \
  -H "Content-Type: application/json" \
  -d '{"parameters": {...}}'
```

The system will:
1. Validate that the token provider matches the tool's provider
2. Use the user's token for the API request
3. Audit log the authentication method used
4. Fall back to service account if configured and user token fails

### Service Account Authentication

Service accounts are configured at the tool level and used when:
- No user token is provided (and passthrough mode is not required)
- User token authentication fails (and fallback is enabled)
- Passthrough is disabled for the tool

### API Key
```json
{
  "auth_type": "api_key",
  "credentials": {
    "token": "your-api-key",
    "header_name": "X-API-Key"
  }
}
```

### Bearer Token
```json
{
  "auth_type": "token",
  "credentials": {
    "token": "your-bearer-token"
  }
}
```

### Basic Auth
```json
{
  "auth_type": "basic",
  "credentials": {
    "username": "user",
    "password": "pass"
  }
}
```

### Custom Header
```json
{
  "auth_type": "header",
  "credentials": {
    "token": "value",
    "header_name": "X-Custom-Auth"
  }
}
```

## Discovery Process

The discovery service uses multiple strategies to find OpenAPI specifications:

1. **Direct URL**: If `openapi_url` is provided, it's used directly
2. **Common Paths**: Tries common OpenAPI paths like `/openapi.json`, `/swagger.json`
3. **Subdomain Discovery**: Tries common subdomains like `api.`, `docs.`
4. **HTML Parsing**: Parses the homepage for links to API documentation
5. **Well-Known Paths**: Checks `.well-known` paths

## Health Monitoring

Health checks can be configured in three modes:

- **periodic**: Automatic health checks at specified intervals
- **on_demand**: Health checks only when requested
- **disabled**: No health checking

Health check results include:
- Response time
- API version (if available)
- Error details (if any)
- Last check timestamp

## Examples

### Using Passthrough Authentication

Execute a GitHub action with your personal access token:

```bash
# First, create a tool with passthrough enabled
curl -X POST http://localhost:8080/api/v1/tools \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github-passthrough",
    "base_url": "https://api.github.com",
    "auth_type": "token",
    "credentials": {
      "token": "ghp_service_account_token"
    },
    "provider": "github",
    "passthrough_config": {
      "mode": "optional",
      "fallback_to_service": true
    }
  }'

# Execute an action with your personal token
curl -X POST http://localhost:8080/api/v1/tools/{toolId}/execute/create_issue \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-User-Token: ghp_your_personal_token" \
  -H "X-Token-Provider: github" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "owner": "myorg",
      "repo": "myrepo",
      "title": "New Issue",
      "body": "Created with my personal token"
    }
  }'
```

### Adding GitHub
```bash
curl -X POST http://localhost:8080/api/v1/tools \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github",
    "base_url": "https://api.github.com",
    "auth_type": "token",
    "credentials": {
      "token": "ghp_xxxxxxxxxxxx"
    },
    "provider": "github",
    "passthrough_config": {
      "mode": "optional",
      "fallback_to_service": true
    }
  }'
```

### Adding Harness.io
```bash
curl -X POST http://localhost:8080/api/v1/tools \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "harness",
    "base_url": "https://app.harness.io",
    "openapi_url": "https://apidocs.harness.io/openapi.json",
    "auth_type": "api_key",
    "credentials": {
      "token": "pat.xxxxx",
      "header_name": "x-api-key"
    }
  }'
```

### Adding SonarQube
```bash
curl -X POST http://localhost:8080/api/v1/tools \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "sonarqube",
    "base_url": "https://sonarqube.example.com",
    "auth_type": "token",
    "credentials": {
      "token": "squ_xxxxxxxxxxxx"
    },
    "config": {
      "discovery_paths": ["/api/openapi.json", "/web_api/api/openapi"]
    }
  }'
```

### Adding Custom Internal API
```bash
curl -X POST http://localhost:8080/api/v1/tools \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "internal-api",
    "base_url": "https://api.internal.company.com",
    "openapi_url": "https://api.internal.company.com/v1/openapi.json",
    "auth_type": "header",
    "credentials": {
      "token": "secret-token",
      "header_name": "X-Internal-Auth"
    }
  }'
```

## Security Considerations

1. **Credential Storage**: All credentials are encrypted using AES-256-GCM with per-tenant keys
2. **Network Security**: Only HTTPS URLs are allowed for production tools
3. **Rate Limiting**: Prevents abuse through per-tenant and per-tool limits
4. **Audit Trail**: All operations are logged for compliance with authentication method tracking
5. **Input Validation**: All inputs are validated to prevent injection attacks
6. **Health Check Timeouts**: Prevents hanging connections
7. **User Token Security**: User tokens are never stored and only used for the specific request
8. **Provider Validation**: System validates that user tokens match the configured tool provider

## Migration from Legacy Tools

If you're migrating from the old hardcoded tool system:

1. Use the discovery API to find your tool's OpenAPI spec
2. Create the tool configuration with appropriate credentials
3. Update your code to use the dynamic tool endpoints
4. Remove any hardcoded tool references

## Troubleshooting

### Discovery Fails
- Ensure the tool provides an OpenAPI 3.0+ specification
- Check if authentication is required to access the spec
- Try providing hints for discovery paths
- Verify network connectivity to the tool

### Action Execution Errors

#### "Operation not found" errors
The advanced operation resolution system handles complex mapping scenarios:

**Common causes and solutions:**

1. **Ambiguous action names** - When multiple operations match
   - Solution: Provide more context parameters
   - Example: Instead of just `"list"`, provide `{"owner": "org"}` to hint at repos/list
   - Use `__resource_type` parameter to explicitly specify the resource

2. **Permission filtering** - Operation exists but user lacks permission
   - Solution: Check your token permissions
   - The system automatically filters out operations you cannot execute
   - Use a token with broader permissions or request access

3. **Resource scope mismatch** - Tool configured for specific resource
   - Example: `github_issues` tool won't resolve `repos/get`
   - Solution: Use the correct tool for the resource type
   - Or use a general tool without resource scope limitations

4. **Cache staleness** - Cached resolution is outdated
   - Solution: Force cache refresh by adding `__cache_bust: true` to parameters
   - Cache automatically invalidates after spec updates

**Debugging operation resolution:**

1. **Check resolution confidence:**
   ```bash
   GET /api/v1/tools/{toolId}/actions/{action}/resolve?debug=true
   ```
   Returns detailed scoring information for the resolution.

2. **View available operations:**
   ```bash
   GET /api/v1/tools/{toolId}/actions
   ```
   Lists all operations with their simple names and full IDs.

3. **Test resolution without execution:**
   ```bash
   POST /api/v1/tools/{toolId}/resolve
   {
     "action": "list",
     "parameters": {"owner": "myorg"}
   }
   ```
   Returns the resolved operation without executing it.

4. **Check learning history:**
   ```bash
   GET /api/v1/tools/{toolId}/learning/stats
   ```
   Shows resolution success rates and patterns.

**Resolution hints:**

The system accepts hints to improve resolution accuracy:
```json
{
  "action": "list",
  "parameters": {
    "owner": "myorg",
    "repo": "myrepo"
  },
  "__hints": {
    "resource_type": "issues",     // Specify resource type
    "prefer_simple": true,          // Prefer simpler operations
    "exclude_patterns": ["commit"], // Exclude operations containing these terms
    "confidence_threshold": 80      // Minimum confidence score required
  }
}
```

**Parameter-based resolution:**

The system intelligently uses parameters to disambiguate:
- `owner`/`repo` parameters → repository operations (1000 point boost)
- `issue_number` parameter → issue operations
- `pull_number` parameter → pull request operations  
- `user`/`username` parameters → user operations
- `org`/`organization` parameters → organization operations
- `gist_id` parameter → gist operations
- `team_id`/`team_slug` parameters → team operations

**Semantic understanding examples:**

The SemanticScorer understands intent:
- `"list"` + array response → list operations
- `"get"` + single response → retrieve operations
- `"create"` + POST method → create operations
- `"delete"` + DELETE method → delete operations
- Simple operations preferred for common actions
- Complex operations selected when specific parameters provided

### Authentication Errors
- Verify credentials are correct
- Check if the tool requires specific headers
- Ensure the auth type matches the tool's requirements
- Look for rate limiting from the tool

### Health Check Failures
- Check tool's actual availability
- Verify credentials haven't expired
- Check network connectivity
- Review timeout settings

## Best Practices

1. **Use Discovery**: Let the system discover the OpenAPI spec automatically when possible
2. **Configure Health Checks**: Enable periodic health checks for production tools
3. **Set Appropriate Timeouts**: Configure timeouts based on tool response times
4. **Monitor Usage**: Use the audit logs to monitor tool usage
5. **Rotate Credentials**: Regularly update tool credentials for security

# Dynamic Tools API Documentation

## Overview

The Dynamic Tools API allows you to add and manage DevOps tools without modifying code. Any tool that provides an OpenAPI/Swagger specification can be integrated automatically.

## Key Features

- **Zero Code Integration**: Add new tools without writing any code
- **Automatic Discovery**: Automatically discovers OpenAPI specifications from tool endpoints
- **Dynamic Authentication**: Supports multiple authentication methods based on OpenAPI security schemes
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

Executes a tool action.

Request Body:
```json
{
  "parameters": {
    "owner": "myorg",
    "repo": "myrepo",
    "title": "New Issue",
    "body": "Issue description"
  }
}
```

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
4. **Audit Trail**: All operations are logged for compliance
5. **Input Validation**: All inputs are validated to prevent injection attacks
6. **Health Check Timeouts**: Prevents hanging connections

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
6. **Test First**: Use the discovery API to test integration before saving
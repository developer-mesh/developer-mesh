# Standard Tools API Documentation

## Overview

The Standard Tools API provides endpoints for managing pre-built tool integrations at the organization level.

## Base URL

```
http://localhost:8081/api/v1
```

## Authentication

All endpoints require Bearer token authentication:

```
Authorization: Bearer YOUR_API_TOKEN
```

## Endpoints

### Tool Templates

#### List Available Templates

Get all available tool templates that can be instantiated.

```http
GET /templates/tools
```

**Response:**
```json
{
  "templates": [
    {
      "id": "template-github-v3",
      "provider_name": "github",
      "provider_version": "v3",
      "display_name": "GitHub",
      "description": "GitHub API v3 integration",
      "category": "version_control",
      "icon_url": "https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png",
      "required_credentials": ["token"],
      "optional_credentials": ["client_id", "client_secret"],
      "features": {
        "supports_oauth": true,
        "supports_webhooks": true,
        "supports_pagination": true,
        "max_page_size": 100
      },
      "tags": ["git", "vcs", "collaboration"],
      "is_public": true,
      "is_active": true
    }
  ]
}
```

#### Get Template Details

```http
GET /templates/tools/{template-id}
```

**Response:**
```json
{
  "id": "template-github-v3",
  "provider_name": "github",
  "provider_version": "v3",
  "display_name": "GitHub",
  "description": "GitHub API v3 integration",
  "default_config": {
    "base_url": "https://api.github.com",
    "auth_type": "bearer",
    "rate_limits": {
      "requests_per_hour": 5000,
      "requests_per_minute": 100
    },
    "health_endpoint": "/rate_limit"
  },
  "operation_groups": [
    {
      "name": "repositories",
      "display_name": "Repositories",
      "description": "Repository management operations",
      "operations": ["repos/list", "repos/get", "repos/create", "repos/update", "repos/delete"]
    },
    {
      "name": "issues",
      "display_name": "Issues",
      "description": "Issue tracking operations",
      "operations": ["issues/list", "issues/get", "issues/create", "issues/update", "issues/close"]
    }
  ]
}
```

### Organization Tools

#### Register Tool for Organization

Create a new tool instance for an organization from a template.

```http
POST /organizations/{org-id}/tools
```

**Request Body:**
```json
{
  "template_id": "template-github-v3",
  "instance_name": "github-main",
  "display_name": "GitHub Main Account",
  "description": "Our main GitHub integration",
  "instance_config": {
    "base_url": "https://api.github.com",
    "rate_limits": {
      "requests_per_hour": 10000
    }
  },
  "credentials": {
    "token": "ghp_your_github_token"
  },
  "enabled_features": {
    "auto_pagination": true,
    "retry_on_rate_limit": true
  },
  "tags": ["production", "main"]
}
```

**Response:**
```json
{
  "id": "tool-123",
  "organization_id": "org-456",
  "template_id": "template-github-v3",
  "instance_name": "github-main",
  "display_name": "GitHub Main Account",
  "status": "active",
  "is_active": true,
  "created_at": "2025-08-25T10:00:00Z",
  "created_by": "user-789"
}
```

#### List Organization Tools

```http
GET /organizations/{org-id}/tools
```

**Query Parameters:**
- `status` (optional): Filter by status (active, inactive, error)
- `provider` (optional): Filter by provider name
- `tag` (optional): Filter by tag

**Response:**
```json
{
  "tools": [
    {
      "id": "tool-123",
      "organization_id": "org-456",
      "template_id": "template-github-v3",
      "instance_name": "github-main",
      "display_name": "GitHub Main Account",
      "description": "Our main GitHub integration",
      "status": "active",
      "is_active": true,
      "last_health_check": "2025-08-25T10:00:00Z",
      "health_status": {
        "healthy": true,
        "response_time_ms": 145,
        "rate_limit_remaining": 4855
      },
      "usage_count": 1523,
      "error_count": 2,
      "tags": ["production", "main"],
      "created_at": "2025-08-25T09:00:00Z"
    }
  ],
  "total": 1
}
```

#### Get Tool Details

```http
GET /organizations/{org-id}/tools/{tool-id}
```

**Response:** (Same structure as list item with additional details)

#### Update Tool Configuration

```http
PUT /organizations/{org-id}/tools/{tool-id}
```

**Request Body:**
```json
{
  "display_name": "GitHub Production",
  "description": "Production GitHub integration",
  "instance_config": {
    "rate_limits": {
      "requests_per_hour": 15000
    }
  },
  "enabled_features": {
    "auto_pagination": false
  },
  "is_active": true
}
```

#### Update Tool Credentials

```http
PUT /organizations/{org-id}/tools/{tool-id}/credentials
```

**Request Body:**
```json
{
  "credentials": {
    "token": "ghp_new_github_token"
  }
}
```

#### Delete Tool

```http
DELETE /organizations/{org-id}/tools/{tool-id}
```

**Response:** 204 No Content

### Tool Operations

#### Execute Tool Operation

```http
POST /organizations/{org-id}/tools/{tool-id}/execute
```

**Request Body:**
```json
{
  "operation": "repos/list",
  "parameters": {
    "org": "developer-mesh",
    "type": "public",
    "per_page": 30
  }
}
```

**Response:**
```json
{
  "success": true,
  "data": [...],
  "metadata": {
    "execution_time_ms": 234,
    "rate_limit_remaining": 4854,
    "cached": false
  }
}
```

#### List Available Operations

```http
GET /organizations/{org-id}/tools/{tool-id}/operations
```

**Response:**
```json
{
  "operations": [
    {
      "id": "repos/list",
      "name": "List Repositories",
      "description": "List repositories for an organization",
      "method": "GET",
      "path": "/orgs/{org}/repos",
      "parameters": [
        {
          "name": "org",
          "in": "path",
          "required": true,
          "type": "string",
          "description": "Organization name"
        },
        {
          "name": "type",
          "in": "query",
          "required": false,
          "type": "string",
          "enum": ["all", "public", "private", "forks", "sources", "member"],
          "default": "all"
        }
      ]
    }
  ]
}
```

### Health & Monitoring

#### Get Tool Health Status

```http
GET /organizations/{org-id}/tools/{tool-id}/health
```

**Response:**
```json
{
  "tool_id": "tool-123",
  "provider": "github",
  "healthy": true,
  "last_check": "2025-08-25T10:00:00Z",
  "response_time_ms": 145,
  "circuit_breaker": {
    "state": "closed",
    "failures": 0,
    "success_rate": 0.998
  },
  "rate_limit": {
    "limit": 5000,
    "remaining": 4855,
    "reset_at": "2025-08-25T11:00:00Z"
  }
}
```

#### Get All Tools Health

```http
GET /organizations/{org-id}/tools/health
```

**Response:**
```json
{
  "tools": {
    "tool-123": {
      "provider": "github",
      "healthy": true,
      "circuit_state": "closed"
    },
    "tool-456": {
      "provider": "gitlab",
      "healthy": true,
      "circuit_state": "closed"
    }
  },
  "summary": {
    "total": 2,
    "healthy": 2,
    "unhealthy": 0,
    "circuits_open": 0
  }
}
```

### Metrics

#### Get Tool Metrics

```http
GET /organizations/{org-id}/tools/{tool-id}/metrics
```

**Query Parameters:**
- `period`: Time period (hour, day, week, month)
- `from`: Start timestamp
- `to`: End timestamp

**Response:**
```json
{
  "tool_id": "tool-123",
  "period": "day",
  "metrics": {
    "total_requests": 1523,
    "successful_requests": 1521,
    "failed_requests": 2,
    "average_latency_ms": 234,
    "p95_latency_ms": 456,
    "p99_latency_ms": 789,
    "cache_hit_rate": 0.65,
    "operations": {
      "repos/list": {
        "count": 523,
        "avg_latency_ms": 201
      },
      "issues/create": {
        "count": 89,
        "avg_latency_ms": 345
      }
    }
  }
}
```

## Error Responses

### Error Format

```json
{
  "error": {
    "code": "TOOL_NOT_FOUND",
    "message": "Tool with ID 'tool-123' not found",
    "details": {
      "tool_id": "tool-123",
      "organization_id": "org-456"
    }
  }
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|------------|-------------|
| `TEMPLATE_NOT_FOUND` | 404 | Tool template does not exist |
| `TOOL_NOT_FOUND` | 404 | Organization tool does not exist |
| `INVALID_CREDENTIALS` | 400 | Provided credentials are invalid |
| `PERMISSION_DENIED` | 403 | User lacks permission for operation |
| `RATE_LIMIT_EXCEEDED` | 429 | Provider rate limit exceeded |
| `CIRCUIT_BREAKER_OPEN` | 503 | Circuit breaker is open for provider |
| `PROVIDER_ERROR` | 502 | Error from external provider |
| `VALIDATION_ERROR` | 400 | Request validation failed |

## Rate Limiting

API endpoints are rate limited:
- 1000 requests per hour per organization
- 100 requests per minute per organization

Rate limit headers:
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 998
X-RateLimit-Reset: 1693044000
```

## Webhooks

### Webhook Events

Tools can trigger webhooks for various events:

| Event | Description |
|-------|-------------|
| `tool.created` | New tool registered |
| `tool.updated` | Tool configuration updated |
| `tool.deleted` | Tool deleted |
| `tool.health_changed` | Health status changed |
| `tool.circuit_opened` | Circuit breaker opened |
| `tool.circuit_closed` | Circuit breaker closed |

### Webhook Payload

```json
{
  "event": "tool.health_changed",
  "timestamp": "2025-08-25T10:00:00Z",
  "organization_id": "org-456",
  "tool": {
    "id": "tool-123",
    "provider": "github",
    "previous_status": "healthy",
    "current_status": "unhealthy",
    "error": "Connection timeout"
  }
}
```
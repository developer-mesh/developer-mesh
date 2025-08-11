# REST API Reference

Complete API reference for the Developer Mesh REST API service.

## Overview

The REST API service provides comprehensive data management, AI agent orchestration, and search capabilities for the Developer Mesh platform. It serves as the HTTP gateway for AI agents, tool integrations, and multi-model embedding operations.

### Base URL
```
Production: https://api.developer-mesh.com/api/v1
Staging:    https://staging-api.developer-mesh.com/api/v1
Local:      http://localhost:8081/api/v1
```

### Authentication

All API endpoints (except health checks) require authentication:

```bash
# API Key Authentication
curl -H "X-API-Key: your-api-key" https://api.developer-mesh.com/api/v1/contexts

# JWT Bearer Token
curl -H "Authorization: Bearer eyJhbGc..." https://api.developer-mesh.com/api/v1/contexts
```

### Rate Limiting

| Tier | Requests/Minute | Requests/Day |
|------|----------------|--------------|
| Free | 60 | 10,000 |
| Pro | 300 | 100,000 |
| Enterprise | Custom | Unlimited |

Rate limit headers:
```http
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 299
X-RateLimit-Reset: 1642521600
Retry-After: 60
```

## Health & Monitoring Endpoints

### Health Check
Check the health status of all components.

```http
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "redis": "healthy",
    "vector_db": "healthy"
  },
  "timestamp": "2024-01-20T10:30:00Z"
}
```

### Metrics
Prometheus-compatible metrics endpoint.

```http
GET /metrics
```

## Context Management API

### Create Context
Create a new context for storing conversation or document data.

```http
POST /api/v1/contexts
```

**Request Body:**
```json
{
  "agent_id": "agent-123",
  "session_id": "session-456",
  "content": [
    {
      "type": "message",
      "role": "user",
      "content": "Hello, I need help with OAuth 2.0"
    }
  ],
  "metadata": {
    "source": "chat",
    "language": "en"
  }
}
```

**Response (201 Created):**
```json
{
  "id": "ctx-789",
  "agent_id": "agent-123",
  "session_id": "session-456",
  "created_at": "2024-01-20T10:30:00Z",
  "updated_at": "2024-01-20T10:30:00Z",
  "_links": {
    "self": "/api/v1/contexts/ctx-789",
    "search": "/api/v1/contexts/ctx-789/search",
    "summary": "/api/v1/contexts/ctx-789/summary"
  }
}
```

### List Contexts
Retrieve all contexts with optional filtering.

```http
GET /api/v1/contexts?agent_id=agent-123&limit=20&offset=0
```

**Query Parameters:**
- `agent_id` (optional): Filter by agent ID
- `session_id` (optional): Filter by session ID
- `limit` (optional): Number of results (default: 50, max: 100)
- `offset` (optional): Pagination offset
- `sort` (optional): Sort field (created_at, updated_at)
- `order` (optional): Sort order (asc, desc)

**Response:**
```json
{
  "contexts": [
    {
      "id": "ctx-789",
      "agent_id": "agent-123",
      "session_id": "session-456",
      "created_at": "2024-01-20T10:30:00Z",
      "updated_at": "2024-01-20T10:30:00Z"
    }
  ],
  "total": 156,
  "limit": 20,
  "offset": 0,
  "_links": {
    "self": "/api/v1/contexts?agent_id=agent-123&limit=20&offset=0",
    "next": "/api/v1/contexts?agent_id=agent-123&limit=20&offset=20",
    "first": "/api/v1/contexts?agent_id=agent-123&limit=20&offset=0",
    "last": "/api/v1/contexts?agent_id=agent-123&limit=20&offset=140"
  }
}
```

### Get Context
Retrieve a specific context by ID.

```http
GET /api/v1/contexts/:contextID
```

**Response:**
```json
{
  "id": "ctx-789",
  "agent_id": "agent-123",
  "session_id": "session-456",
  "content": [
    {
      "type": "message",
      "role": "user",
      "content": "Hello, I need help with OAuth 2.0"
    }
  ],
  "metadata": {
    "source": "chat",
    "language": "en"
  },
  "created_at": "2024-01-20T10:30:00Z",
  "updated_at": "2024-01-20T10:30:00Z"
}
```

### Update Context
Update an existing context.

```http
PUT /api/v1/contexts/:contextID
```

**Request Body:**
```json
{
  "content": [
    {
      "type": "message",
      "role": "assistant",
      "content": "I'll help you understand OAuth 2.0..."
    }
  ],
  "metadata": {
    "updated_by": "assistant-1"
  }
}
```

### Delete Context
Delete a context and all associated data.

```http
DELETE /api/v1/contexts/:contextID
```

**Response (204 No Content)**

### Get Context Summary
Generate an AI-powered summary of a context.

```http
GET /api/v1/contexts/:contextID/summary
```

**Response:**
```json
{
  "summary": "Discussion about OAuth 2.0 implementation, covering authorization flows, token management, and security best practices.",
  "key_topics": ["OAuth 2.0", "Authorization", "Security"],
  "word_count": 1250,
  "message_count": 15
}
```

### Search Within Context
Search for specific content within a context.

```http
POST /api/v1/contexts/:contextID/search
```

**Request Body:**
```json
{
  "query": "authorization flow",
  "limit": 10
}
```

## Tool Integration API

### List Available Tools
Get all available tool integrations.

```http
GET /api/v1/tools
```

**Response:**
```json
{
  "tools": [
    {
      "id": "github",
      "name": "GitHub",
      "description": "GitHub repository management",
      "version": "1.0",
      "_links": {
        "self": "/api/v1/tools/github",
        "actions": "/api/v1/tools/github/actions"
      }
    }
  ],
  "count": 5
}
```

### Get Tool Details
Get detailed information about a specific tool.

```http
GET /api/v1/tools/:tool
```

**Response:**
```json
{
  "id": "github",
  "name": "GitHub",
  "description": "GitHub integration for repository management",
  "version": "1.0",
  "vendor": "GitHub",
  "auth_methods": ["API Key", "OAuth"],
  "capabilities": ["issues", "pull_requests", "webhooks"],
  "_links": {
    "self": "/api/v1/tools/github",
    "actions": "/api/v1/tools/github/actions"
  }
}
```

### List Tool Actions
Get available actions for a tool.

```http
GET /api/v1/tools/:tool/actions
```

**Response:**
```json
{
  "tool": "github",
  "actions": [
    {
      "name": "create_issue",
      "display_name": "Create Issue",
      "description": "Create a new GitHub issue",
      "_links": {
        "self": "/api/v1/tools/github/actions/create_issue"
      }
    }
  ],
  "count": 12
}
```

### Execute Tool Action
Execute a specific action on a tool.

```http
POST /api/v1/tools/:tool/actions/:action
```

**Request Body:**
```json
{
  "repository": "owner/repo",
  "title": "Bug: Login fails with special characters",
  "body": "Users cannot login when password contains...",
  "labels": ["bug", "high-priority"]
}
```

**Response:**
```json
{
  "status": "success",
  "result": {
    "issue_number": 42,
    "html_url": "https://github.com/owner/repo/issues/42"
  },
  "_links": {
    "self": "/api/v1/tools/github/actions/create_issue",
    "result": "https://github.com/owner/repo/issues/42"
  }
}
```

### Query Tool Data
Execute queries against tool data.

```http
POST /api/v1/tools/:tool/queries
```

**Request Body:**
```json
{
  "query_type": "search_issues",
  "parameters": {
    "repository": "owner/repo",
    "state": "open",
    "labels": ["bug"]
  }
}
```

## Agent Management API

The Agent API manages AI agent lifecycle, capabilities, and workload tracking.

### Agent Management
**Note**: Agent registration is done via WebSocket connection to the MCP server (port 8080), not through the REST API. The REST API provides agent management capabilities after registration.

### Create Agent (Management)
Create agent configuration after WebSocket registration.

```http
POST /api/v1/agents
```

**Request Body:**
```json
{
  "name": "code-analyzer",
  "type": "analyzer",
  "status": "active",
  "capabilities": [
    "code_analysis",
    "security_scan",
    "performance_profiling"
  ],
  "endpoint": "ws://agent.internal:8080",
  "metadata": {
    "model": "gpt-4",
    "version": "1.0",
    "max_concurrent_tasks": 5
  }
}
```

**Response (201 Created):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "code-analyzer",
  "type": "analyzer",
  "status": "active",
  "capabilities": [...],
  "endpoint": "ws://agent.internal:8080",
  "created_at": "2024-01-20T10:30:00Z",
  "last_heartbeat": "2024-01-20T10:30:00Z",
  "workload": {
    "current_tasks": 0,
    "max_tasks": 5,
    "cpu_usage": 0.0,
    "memory_usage": 0.0
  }
}
```

### List Agents
Get all registered agents with filtering and workload information.

```http
GET /api/v1/agents?status=active&capability=code_analysis&limit=20
```

**Query Parameters:**
- `status`: Filter by status (active, inactive, draining)
- `capability`: Filter by capability
- `type`: Filter by agent type
- `sort`: Sort by field (workload, created_at)
- `limit`: Results per page
- `offset`: Pagination offset

**Response:**
```json
{
  "agents": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "code-analyzer",
      "type": "analyzer",
      "status": "active",
      "capabilities": ["code_analysis", "security_scan"],
      "workload": {
        "current_tasks": 2,
        "max_tasks": 5,
        "utilization": 0.4
      },
      "last_heartbeat": "2024-01-20T10:35:00Z"
    }
  ],
  "total": 15,
  "_links": {
    "self": "/api/v1/agents?status=active&limit=20",
    "next": "/api/v1/agents?status=active&limit=20&offset=20"
  }
}
```

### Get Agent Details
Get detailed information about a specific agent.

```http
GET /api/v1/agents/:id
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "code-analyzer",
  "type": "analyzer",
  "status": "active",
  "capabilities": [
    "code_analysis",
    "security_scan",
    "performance_profiling"
  ],
  "endpoint": "ws://agent.internal:8080",
  "metadata": {
    "model": "gpt-4",
    "version": "1.0",
    "max_concurrent_tasks": 5
  },
  "workload": {
    "current_tasks": 2,
    "completed_tasks": 156,
    "failed_tasks": 3,
    "average_task_duration": "45s",
    "cpu_usage": 0.35,
    "memory_usage": 0.42
  },
  "performance": {
    "success_rate": 0.98,
    "average_response_time": "1.2s",
    "tasks_per_hour": 45
  },
  "created_at": "2024-01-20T10:30:00Z",
  "updated_at": "2024-01-20T10:35:00Z",
  "last_heartbeat": "2024-01-20T10:35:00Z"
}
```

### Update Agent Status
Update an agent's status or configuration.

```http
PATCH /api/v1/agents/:id
```

**Request Body:**
```json
{
  "status": "draining",
  "metadata": {
    "maintenance_mode": true
  }
}
```

### Get Agent Workload
Get real-time workload information for an agent.

```http
GET /api/v1/agents/:id/workload
```

**Response:**
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "current_tasks": [
    {
      "task_id": "task-123",
      "type": "code_analysis",
      "started_at": "2024-01-20T10:33:00Z",
      "progress": 0.75
    }
  ],
  "queued_tasks": 3,
  "utilization": 0.4,
  "estimated_availability": "2024-01-20T10:38:00Z"
}
```

### Send Agent Heartbeat
Update agent heartbeat (usually done by agent itself).

```http
POST /api/v1/agents/:id/heartbeat
```

**Request Body:**
```json
{
  "status": "active",
  "workload": {
    "current_tasks": 2,
    "cpu_usage": 0.35,
    "memory_usage": 0.42
  }
}
```

## Model Management API

### List Available Models
Get all available embedding and completion models.

```http
GET /api/v1/models?type=embedding&provider=openai
```

**Query Parameters:**
- `type`: Filter by model type (embedding, completion)
- `provider`: Filter by provider (openai, bedrock, anthropic)
- `capability`: Filter by capability (text, code, multimodal)

**Response:**
```json
{
  "models": [
    {
      "id": "text-embedding-3-small",
      "name": "OpenAI Text Embedding 3 Small",
      "provider": "openai",
      "type": "embedding",
      "dimensions": 1536,
      "max_tokens": 8191,
      "cost_per_1k_tokens": 0.02,
      "capabilities": ["text", "code"],
      "supported_languages": ["multi"],
      "performance": {
        "latency_ms_p50": 120,
        "latency_ms_p99": 250
      }
    },
    {
      "id": "text-embedding-3-large",
      "name": "OpenAI Text Embedding 3 Large",
      "provider": "openai",
      "type": "embedding",
      "dimensions": 3072,
      "max_tokens": 8191,
      "cost_per_1k_tokens": 0.13,
      "capabilities": ["text", "code"],
      "quality_score": 0.95
    },
    {
      "id": "amazon.titan-embed-text-v1",
      "name": "Amazon Titan Text Embeddings",
      "provider": "bedrock",
      "type": "embedding",
      "dimensions": 1536,
      "max_tokens": 8000,
      "cost_per_1k_tokens": 0.10,
      "capabilities": ["text"]
    }
  ],
  "total": 3
}
```

### Get Model Details
Get detailed information about a specific model.

```http
GET /api/v1/models/:id
```

**Response:**
```json
{
  "id": "text-embedding-3-small",
  "name": "OpenAI Text Embedding 3 Small",
  "provider": "openai",
  "type": "embedding",
  "dimensions": 1536,
  "max_tokens": 8191,
  "cost_per_1k_tokens": 0.02,
  "capabilities": ["text", "code"],
  "configuration": {
    "api_endpoint": "https://api.openai.com/v1/embeddings",
    "requires_api_key": true,
    "supports_batch": true,
    "max_batch_size": 100
  },
  "performance": {
    "average_latency_ms": 150,
    "p99_latency_ms": 300,
    "throughput_per_second": 50,
    "uptime_percentage": 99.9
  },
  "usage_stats": {
    "total_requests": 45231,
    "total_tokens": 3425610,
    "error_rate": 0.001
  }
}
```

### Compare Models
Compare multiple models for decision making.

```http
POST /api/v1/models/compare
```

**Request Body:**
```json
{
  "model_ids": [
    "text-embedding-3-small",
    "text-embedding-3-large",
    "amazon.titan-embed-text-v1"
  ],
  "criteria": ["cost", "quality", "latency"]
}
```

**Response:**
```json
{
  "comparison": [
    {
      "model_id": "text-embedding-3-small",
      "scores": {
        "cost": 1.0,
        "quality": 0.85,
        "latency": 0.9,
        "overall": 0.92
      },
      "pros": ["Lowest cost", "Fast response"],
      "cons": ["Lower quality than large model"]
    },
    {
      "model_id": "text-embedding-3-large",
      "scores": {
        "cost": 0.15,
        "quality": 1.0,
        "latency": 0.7,
        "overall": 0.62
      },
      "pros": ["Highest quality", "Best for complex queries"],
      "cons": ["Most expensive", "Slower"]
    }
  ],
  "recommendation": {
    "balanced": "text-embedding-3-small",
    "quality_first": "text-embedding-3-large",
    "cost_optimized": "text-embedding-3-small"
  }
}
```

## Embedding Operations API

The Developer Mesh provides a sophisticated multi-agent embedding system with intelligent routing, cross-model compatibility, and cost optimization.

### Key Features

- **Multi-Provider Support**: OpenAI, AWS Bedrock, Anthropic, Google, Voyage
- **Agent-Specific Routing**: Each agent has its own embedding strategy
- **Smart Provider Selection**: Based on cost, quality, speed, and availability
- **Cross-Model Search**: Search across embeddings from different models
- **Dimension Adaptation**: Automatic dimension normalization for compatibility
- **Cost Tracking**: Real-time cost monitoring and limits

### Generate Embedding
Generate an embedding using agent-specific configuration.

```http
POST /api/v1/embeddings
```

**Request Body:**
```json
{
  "agent_id": "claude-assistant",
  "text": "Implement OAuth 2.0 authorization flow",
  "task_type": "code_search",
  "context_id": "ctx_123",
  "metadata": {
    "language": "typescript",
    "framework": "express"
  }
}
```

**Response:**
```json
{
  "embedding_id": "550e8400-e29b-41d4-a716-446655440001",
  "model_used": "text-embedding-3-large",
  "provider": "openai",
  "dimensions": 3072,
  "normalized_dimensions": 1536,
  "cached": false,
  "cost_usd": 0.00013,
  "processing_time_ms": 125,
  "metadata": {
    "agent_id": "claude-assistant",
    "context_id": "ctx_123"
  }
}
```

### Batch Generate Embeddings
Generate multiple embeddings in a single request.

```http
POST /api/v1/embeddings/batch
```

**Request Body:**
```json
{
  "agent_id": "claude-assistant",
  "texts": [
    "OAuth 2.0 authorization",
    "JWT token validation",
    "API rate limiting"
  ],
  "task_type": "documentation"
}
```

**Response:**
```json
{
  "embeddings": [
    {
      "embedding_id": "550e8400-e29b-41d4-a716-446655440002",
      "text_index": 0,
      "model_used": "text-embedding-3-small",
      "cached": true
    },
    ...
  ],
  "total_cost_usd": 0.00006,
  "cached_count": 1,
  "generated_count": 2
}
```

### Search Embeddings
Search for similar content using vector similarity.

```http
POST /api/v1/embeddings/search
```

**Request Body:**
```json
{
  "agent_id": "claude-assistant",
  "query": "implement authentication",
  "model": "text-embedding-3-small",
  "limit": 10,
  "filters": {
    "context_id": "ctx_123",
    "metadata.language": "typescript"
  },
  "min_similarity": 0.7
}
```

**Response:**
```json
{
  "results": [
    {
      "embedding_id": "550e8400-e29b-41d4-a716-446655440003",
      "similarity_score": 0.92,
      "text": "OAuth 2.0 authorization flow implementation",
      "metadata": {
        "context_id": "ctx_123",
        "language": "typescript"
      }
    }
  ],
  "total_results": 10,
  "search_time_ms": 45
}
```

### Cross-Model Search
Search across embeddings from different models.

```http
POST /api/v1/embeddings/search/cross-model
```

**Request Body:**
```json
{
  "query": "kubernetes deployment strategies",
  "search_model": "text-embedding-3-small",
  "include_models": [
    "text-embedding-3-large",
    "voyage-2",
    "amazon.titan-embed-text-v1"
  ],
  "limit": 20,
  "merge_strategy": "weighted_score"
}
```

**Response:**
```json
{
  "results": [
    {
      "embedding_id": "550e8400-e29b-41d4-a716-446655440004",
      "similarity_score": 0.89,
      "original_model": "voyage-2",
      "adaptation_quality": 0.95,
      "text": "Blue-green deployment in Kubernetes"
    }
  ],
  "models_searched": 3,
  "total_results": 20
}
```

### Get Embedding Provider Health
Check health status of embedding providers.

```http
GET /api/v1/embeddings/providers/health
```

**Response:**
```json
{
  "providers": [
    {
      "name": "openai",
      "status": "healthy",
      "latency_ms": 120,
      "success_rate": 0.99,
      "cost_per_1k_tokens": 0.02
    },
    {
      "name": "bedrock",
      "status": "degraded",
      "latency_ms": 450,
      "success_rate": 0.85,
      "error": "High latency detected"
    }
  ],
  "circuit_breakers": {
    "bedrock": "half_open"
  }
}
```

### Get Usage Statistics
Get embedding usage statistics for billing and monitoring.

```http
GET /api/v1/embeddings/usage?agent_id=claude-assistant&period=24h
```

**Response:**
```json
{
  "agent_id": "claude-assistant",
  "period": "24h",
  "usage": {
    "total_embeddings": 1543,
    "total_tokens": 125420,
    "total_cost_usd": 2.51,
    "cache_hit_rate": 0.34,
    "by_provider": {
      "openai": {
        "count": 1200,
        "cost_usd": 2.40
      },
      "bedrock": {
        "count": 343,
        "cost_usd": 0.11
      }
    },
    "by_model": {
      "text-embedding-3-small": 1000,
      "text-embedding-3-large": 200,
      "amazon.titan-embed-text-v1": 343
    }
  }
}
```

## Vector & Search API

### Store Vector
Store a vector embedding with metadata.

```http
POST /api/v1/vectors
```

**Request Body:**
```json
{
  "id": "vec-123",
  "embedding": [0.1, 0.2, 0.3, ...],
  "metadata": {
    "source": "github",
    "type": "issue",
    "language": "python",
    "repository": "owner/repo"
  },
  "model": "text-embedding-3-small",
  "text": "Original text for reference"
}
```

**Response:**
```json
{
  "id": "vec-123",
  "dimensions": 1536,
  "created_at": "2024-01-20T10:30:00Z"
}
```

### Search Vectors
Search for similar vectors using cosine similarity.

```http
POST /api/v1/vectors/search
```

**Request Body:**
```json
{
  "query_vector": [0.1, 0.2, 0.3, ...],
  "filters": {
    "metadata.type": "issue",
    "metadata.language": "python"
  },
  "top_k": 10,
  "include_metadata": true,
  "min_score": 0.7
}
```

**Response:**
```json
{
  "results": [
    {
      "id": "vec-123",
      "score": 0.95,
      "metadata": {
        "source": "github",
        "type": "issue",
        "language": "python"
      },
      "text": "Python authentication implementation"
    }
  ],
  "search_time_ms": 23
}
```

### Hybrid Search
Combine vector similarity with keyword matching.

```http
POST /api/v1/search/hybrid
```

**Request Body:**
```json
{
  "query": "OAuth 2.0 implementation in Python",
  "vector_weight": 0.7,
  "keyword_weight": 0.3,
  "filters": {
    "context_id": "ctx-123",
    "created_after": "2024-01-01T00:00:00Z"
  },
  "limit": 20
}
```

**Response:**
```json
{
  "results": [
    {
      "id": "result-1",
      "score": 0.89,
      "vector_score": 0.92,
      "keyword_score": 0.81,
      "content": "OAuth 2.0 flow implementation using Python Flask",
      "highlights": [
        "<em>OAuth 2.0</em> flow implementation using <em>Python</em> Flask"
      ]
    }
  ],
  "total_results": 20,
  "facets": {
    "metadata.language": {
      "python": 15,
      "javascript": 3,
      "go": 2
    }
  }
}
```

### Find Similar
Find items similar to a reference item.

```http
POST /api/v1/search/similar
```

**Request Body:**
```json
{
  "reference_id": "vec-123",
  "limit": 10,
  "diversity": 0.3
}
```

**Response:**
```json
{
  "reference": {
    "id": "vec-123",
    "text": "OAuth 2.0 Python implementation"
  },
  "similar_items": [
    {
      "id": "vec-456",
      "score": 0.91,
      "text": "Python OAuth2 client library"
    }
  ]
}
```

## MCP Protocol Support

The REST API implements the Model Context Protocol through the standard context endpoints. MCP-specific functionality is integrated into the existing context management API rather than being a separate endpoint set.

## Webhook API

The Webhook API receives and processes events from external services.

### GitHub Webhook
Receive GitHub webhook events.

```http
POST /api/webhooks/github
```

**Headers:**
```http
X-GitHub-Event: issues
X-Hub-Signature-256: sha256=abcdef123456...
X-GitHub-Delivery: 12345-67890-abcdef
Content-Type: application/json
```

**Request Body (Issue Event):**
```json
{
  "action": "opened",
  "issue": {
    "id": 1234567890,
    "number": 42,
    "title": "Bug: Authentication fails",
    "body": "When trying to login...",
    "state": "open",
    "user": {
      "login": "octocat"
    }
  },
  "repository": {
    "id": 987654321,
    "name": "hello-world",
    "owner": {
      "login": "octocat"
    }
  }
}
```

**Response:**
```json
{
  "status": "accepted",
  "message": "Webhook processed successfully",
  "event_id": "evt-123",
  "actions_triggered": [
    "store_issue",
    "generate_embeddings",
    "notify_agents"
  ]
}
```

### Configure Webhook
Configure webhook settings for an organization/repository.

```http
PUT /api/webhooks/github/config
```

**Request Body:**
```json
{
  "repository": "octocat/hello-world",
  "events": ["issues", "pull_request", "push"],
  "process_options": {
    "auto_embed": true,
    "extract_relationships": true,
    "notify_agents": true
  },
  "filters": {
    "branch_pattern": "main|develop",
    "label_include": ["bug", "feature"],
    "label_exclude": ["wontfix"]
  }
}
```

### List Webhook Events
Get processed webhook events.

```http
GET /api/webhooks/events?repository=octocat/hello-world&limit=20
```

**Response:**
```json
{
  "events": [
    {
      "id": "evt-123",
      "source": "github",
      "type": "issues",
      "action": "opened",
      "repository": "octocat/hello-world",
      "received_at": "2024-01-20T10:30:00Z",
      "processed_at": "2024-01-20T10:30:01Z",
      "status": "processed",
      "summary": "New issue #42 opened"
    }
  ],
  "total": 156,
  "_links": {
    "self": "/api/v1/webhooks/events?repository=octocat/hello-world&limit=20",
    "next": "/api/v1/webhooks/events?repository=octocat/hello-world&limit=20&offset=20"
  }
}
```

## Relationship API (Planned)

**Note**: The Relationship API is designed but not yet implemented.

The Relationship API will track connections between entities (issues, PRs, commits, etc.).

### Future: Create Relationship
Create a relationship between entities.

```http
POST /api/v1/relationships
```

**Request Body:**
```json
{
  "from_entity": {
    "type": "issue",
    "owner": "octocat",
    "repo": "hello-world",
    "id": "123"
  },
  "to_entity": {
    "type": "pull_request",
    "owner": "octocat",
    "repo": "hello-world",
    "id": "456"
  },
  "relationship_type": "fixes",
  "properties": {
    "confidence": 0.95,
    "extracted_from": "pr_description"
  }
}
```

**Response:**
```json
{
  "id": "rel-789",
  "from_entity": {...},
  "to_entity": {...},
  "relationship_type": "fixes",
  "properties": {...},
  "bidirectional": true,
  "created_at": "2024-01-20T10:30:00Z"
}
```

### Get Entity Relationships
Get all relationships for an entity.

```http
GET /api/v1/entities/:type/:owner/:repo/:id/relationships?direction=both
```

**Query Parameters:**
- `direction`: Filter by direction (incoming, outgoing, both)
- `type`: Filter by relationship type
- `entity_type`: Filter by related entity type

**Response:**
```json
{
  "entity": {
    "type": "issue",
    "owner": "octocat",
    "repo": "hello-world",
    "id": "123"
  },
  "relationships": [
    {
      "id": "rel-789",
      "type": "fixed_by",
      "direction": "incoming",
      "related_entity": {
        "type": "pull_request",
        "id": "456",
        "title": "Fix authentication bug"
      }
    },
    {
      "id": "rel-790",
      "type": "references",
      "direction": "outgoing",
      "related_entity": {
        "type": "issue",
        "id": "789",
        "title": "Related security issue"
      }
    }
  ],
  "total": 2
}
```

### Get Relationship Graph
Get the relationship graph for an entity.

```http
GET /api/v1/entities/:type/:owner/:repo/:id/graph?depth=2&include_types=issue,pull_request
```

**Query Parameters:**
- `depth`: Graph traversal depth (default: 1, max: 3)
- `include_types`: Entity types to include
- `relationship_types`: Relationship types to follow

**Response:**
```json
{
  "root": {
    "type": "issue",
    "id": "123",
    "title": "Authentication bug"
  },
  "nodes": [
    {
      "id": "node-1",
      "entity": {
        "type": "pull_request",
        "id": "456",
        "title": "Fix auth bug"
      },
      "depth": 1
    },
    {
      "id": "node-2",
      "entity": {
        "type": "commit",
        "id": "abc123",
        "message": "Fix authentication logic"
      },
      "depth": 2
    }
  ],
  "edges": [
    {
      "from": "root",
      "to": "node-1",
      "type": "fixed_by"
    },
    {
      "from": "node-1",
      "to": "node-2",
      "type": "contains"
    }
  ],
  "stats": {
    "total_nodes": 3,
    "total_edges": 2,
    "max_depth_reached": 2
  }
}
```

### Delete Relationship
Remove a relationship between entities.

```http
DELETE /api/v1/relationships/:id
```

## Error Responses

All errors follow a consistent format:

```json
{
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "Context ctx-999 not found",
    "details": {
      "resource_type": "context",
      "resource_id": "ctx-999"
    }
  },
  "request_id": "req-abc123",
  "timestamp": "2024-01-20T10:30:00Z"
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Missing or invalid authentication |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `RESOURCE_NOT_FOUND` | 404 | Resource does not exist |
| `VALIDATION_ERROR` | 400 | Invalid request data |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Server error |

## Workflow & Task API (Planned)

**Note**: The workflow and task APIs are designed but not yet implemented. Task assignment currently happens through WebSocket messages to the MCP server.

### Future: Create Workflow
Define a multi-step workflow for AI agents.

```http
POST /api/v1/workflows
```

**Request Body:**
```json
{
  "name": "code-review-workflow",
  "description": "Automated code review process",
  "type": "dag",
  "steps": [
    {
      "id": "analyze-code",
      "name": "Analyze Code",
      "agent_capability": "code_analysis",
      "timeout": "5m",
      "retry": {
        "max_attempts": 3,
        "backoff": "exponential"
      }
    },
    {
      "id": "security-scan",
      "name": "Security Scan",
      "agent_capability": "security_scan",
      "depends_on": ["analyze-code"],
      "timeout": "10m"
    },
    {
      "id": "generate-report",
      "name": "Generate Report",
      "agent_capability": "report_generation",
      "depends_on": ["analyze-code", "security-scan"],
      "timeout": "2m"
    }
  ],
  "triggers": [
    {
      "type": "webhook",
      "event": "pull_request.opened"
    }
  ]
}
```

### Execute Workflow
Start a workflow execution.

```http
POST /api/v1/workflows/:id/executions
```

**Request Body:**
```json
{
  "input": {
    "repository": "octocat/hello-world",
    "pull_request": 123,
    "commit_sha": "abc123def456"
  },
  "priority": "high",
  "metadata": {
    "triggered_by": "webhook",
    "user": "octocat"
  }
}
```

### Create Task
Create a standalone task for agent execution.

```http
POST /api/v1/tasks
```

**Request Body:**
```json
{
  "type": "code_review",
  "priority": "high",
  "requirements": {
    "capabilities": ["code_analysis", "python"],
    "min_success_rate": 0.95
  },
  "input": {
    "repository": "octocat/hello-world",
    "file_path": "src/auth.py",
    "analysis_type": "security"
  },
  "constraints": {
    "max_cost_usd": 0.50,
    "deadline": "2024-01-20T11:00:00Z"
  }
}
```

### Get Task Status
Get current status of a task.

```http
GET /api/v1/tasks/:id
```

**Response:**
```json
{
  "id": "task-123",
  "type": "code_review",
  "status": "in_progress",
  "assigned_agent": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "code-analyzer"
  },
  "progress": {
    "percent_complete": 75,
    "current_step": "analyzing_security_patterns",
    "steps_completed": 3,
    "total_steps": 4
  },
  "started_at": "2024-01-20T10:35:00Z",
  "estimated_completion": "2024-01-20T10:40:00Z",
  "cost_so_far": 0.12
}
```

## SDK Examples

### JavaScript/TypeScript
```typescript
import { DevOpsMCPClient } from '@developer-mesh/rest-client';

const client = new DevOpsMCPClient({
  apiKey: 'your-api-key',
  baseURL: 'https://api.developer-mesh.com/api/v1'
});

// Register an agent
const agent = await client.agents.register({
  name: 'my-code-analyzer',
  capabilities: ['code_analysis', 'python', 'javascript'],
  endpoint: 'wss://my-agent.example.com'
});

// Create context
const context = await client.contexts.create({
  agent_id: agent.id,
  content: [{ type: 'message', role: 'user', content: 'Analyze my Python code' }]
});

// Generate embedding
const embedding = await client.embeddings.generate({
  agent_id: agent.id,
  text: 'OAuth 2.0 implementation',
  task_type: 'code_search'
});

// Search across models
const results = await client.embeddings.searchCrossModel({
  query: 'authentication patterns',
  include_models: ['text-embedding-3-small', 'voyage-2'],
  limit: 10
});
```

### Python
```python
from devops_mcp import RestClient

client = RestClient(
    api_key="your-api-key",
    base_url="https://api.developer-mesh.com/api/v1"
)

# Create context
context = client.contexts.create(
    agent_id="agent-123",
    content=[{"type": "message", "role": "user", "content": "Hello"}]
)

# Execute tool action
result = client.tools.execute_action(
    tool="github",
    action="create_issue",
    params={
        "repository": "owner/repo",
        "title": "Bug report",
        "body": "Description"
    }
)
```

### Go
```go
import "github.com/developer-mesh/developer-mesh/pkg/client/rest"

client := rest.NewClient(
    rest.WithAPIKey("your-api-key"),
    rest.WithBaseURL("https://api.developer-mesh.com/api/v1"),
)

// Create context
ctx, err := client.Contexts.Create(context.Background(), &CreateContextRequest{
    AgentID: "agent-123",
    Content: []ContextItem{{Type: "message", Role: "user", Content: "Hello"}},
})

// Search
results, err := client.Search.Query(context.Background(), &SearchRequest{
    Query: "OAuth 2.0",
    ContextIDs: []string{ctx.ID},
})
```

## Advanced Features

### Distributed Task Assignment

The REST API supports sophisticated task assignment strategies:

```http
POST /api/v1/tasks/assign
```

**Request Body:**
```json
{
  "task": {
    "type": "distributed_analysis",
    "requirements": ["code_analysis", "security_scan", "performance_profiling"]
  },
  "strategy": "capability_match_with_load_balancing",
  "constraints": {
    "max_agents": 3,
    "prefer_colocated": true,
    "cost_limit_usd": 1.0
  }
}
```

### Agent Collaboration

Enable multiple agents to work together:

```http
POST /api/v1/collaborations
```

**Request Body:**
```json
{
  "name": "security-audit",
  "coordinator_agent": "agent-123",
  "participant_agents": ["agent-456", "agent-789"],
  "collaboration_type": "parallel_with_aggregation",
  "shared_context": "ctx-999"
}
```

### Real-time Monitoring

Monitor system performance and agent activity:

```http
GET /api/v1/monitoring/dashboard
```

**Response:**
```json
{
  "system_health": "healthy",
  "active_agents": 15,
  "tasks_in_progress": 47,
  "average_task_duration": "2m15s",
  "embedding_cache_hit_rate": 0.68,
  "total_cost_last_hour": 12.45,
  "alerts": [
    {
      "type": "high_latency",
      "agent": "agent-789",
      "message": "Response time exceeds threshold"
    }
  ]
}
```

---

*For more information, visit [docs.developer-mesh.com](https://docs.developer-mesh.com)*
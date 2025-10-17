<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:28:37
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Embedding API Reference

The multi-agent embedding system provides sophisticated embedding generation and search capabilities with provider failover, cost optimization, and cross-model compatibility.

## Overview

Each AI agent can have customized embedding configurations including:
- Preferred models for different task types
- Embedding strategy (quality, speed, cost, balanced)
- Cost constraints and rate limits
- Fallback behavior

## API Endpoints

### Generate Embedding

`POST /api/v1/embeddings`

Generate an embedding using agent-specific configuration.

**Headers:**
- `X-API-Key`: Your API key (required)
- `X-Tenant-ID`: Tenant UUID (required)

**Request Body:**
```json
{
  "agent_id": "research-001",
  "text": "Your text to embed",
  "task_type": "general_qa",  // Optional: general_qa, search_document, search_query, classification, clustering
  "metadata": {                // Optional
    "source": "document",
    "document_id": "doc-123"
  }
}
```

**Response:**
```json
{
  "embedding_id": "550e8400-e29b-41d4-a716-446655440000",
  "request_id": "req-123",
  "model_used": "text-embedding-3-small",
  "provider": "openai",
  "dimensions": 1536,
  "normalized_dimensions": 1536,
  "cost_usd": 0.00002,
  "tokens_used": 150,
  "generation_time_ms": 245,
  "cached": false,
  "metadata": {
    "source": "document",
    "document_id": "doc-123"
  }
}
```

### Batch Generate Embeddings

Generate embeddings for multiple texts in a single request.

**Endpoint:** `POST /api/v1/embeddings/batch`

**Request Body:**
```json
[
  {
    "agent_id": "research-001",
    "text": "First text to embed",
    "task_type": "search_document",
    "metadata": {"doc_id": "1"}
  },
  {
    "agent_id": "research-001",
    "text": "Second text to embed",
    "task_type": "search_document",
    "metadata": {"doc_id": "2"}
  }
]
```

**Response:**
```json
{
  "embeddings": [
    {
      "embedding_id": "550e8400-e29b-41d4-a716-446655440001",
      "request_id": "req-124",
      "model_used": "text-embedding-3-small",
      "provider": "openai",
      "dimensions": 1536,
      "cost_usd": 0.00002,
      "tokens_used": 120,
      "generation_time_ms": 198,
      "cached": false,
      "metadata": {"doc_id": "1"}
    },
    {
      "embedding_id": "550e8400-e29b-41d4-a716-446655440002",
      "request_id": "req-125",
      "model_used": "text-embedding-3-small",
      "provider": "openai",
      "dimensions": 1536,
      "cost_usd": 0.00002,
      "tokens_used": 135,
      "generation_time_ms": 210,
      "cached": false,
      "metadata": {"doc_id": "2"}
    }
  ],
  "count": 2
}
```

### Search Embeddings

Search for similar embeddings using vector similarity.

**Endpoint:** `POST /api/v1/embeddings/search`

**Request Body:**
```json
{
  "query": "search query text",
  "agent_id": "research-001",
  "limit": 10,
  "min_similarity": 0.7,
  "metadata_filter": {
    "source": "document"
  }
}
```

**Response:**
```json
{
  "status": 501,
  "error": {
    "code": "INTERNAL_SERVER_ERROR",
    "message": "Search functionality not yet implemented"
  }
}
```

*Note: Search functionality is pending implementation.*

### Cross-Model Search

Search across embeddings created by different models.

**Endpoint:** `POST /api/v1/embeddings/search/cross-model`

**Request Body:**
```json
{
  "query": "search query text",
  "search_model": "text-embedding-3-large",
  "include_models": ["text-embedding-3-large", "text-embedding-3-small"],
  "exclude_models": ["text-embedding-ada-002"],
  "limit": 20,
  "min_similarity": 0.75,
  "task_type": "research"
}
```

**Response:**
```json
{
  "status": 501,
  "error": {
    "code": "INTERNAL_SERVER_ERROR",
    "message": "Cross-model search functionality not yet implemented"
  }
}
```

*Note: Cross-model search is pending implementation.*

### Get Provider Health

Check the health status of all configured embedding providers.

**Endpoint:** `GET /api/v1/embeddings/providers/health`

**Response:**
```json
{
  "providers": {
    "openai": {
      "name": "openai",
      "status": "healthy",
      "circuit_breaker_state": "closed",
      "failure_count": 0
    },
    "bedrock": {
      "name": "bedrock",
      "status": "unhealthy",
      "error": "provider not configured",
      "circuit_breaker_state": "open",
      "failure_count": 5
    }
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Agent Configuration Endpoints

### Create Agent Configuration

Create a new embedding configuration for an agent.

**Endpoint:** `POST /api/v1/embeddings/agents`

**Request Body:**
```json
{
  "agent_id": "new-agent-001",
  "embedding_strategy": "balanced",
  "model_preferences": {
    "general_qa": {
      "primary_models": ["text-embedding-3-small"],
      "fallback_models": ["text-embedding-ada-002"]
    }
  },
  "constraints": {
    "max_tokens_per_request": 8000,
    "max_cost_per_day": 25.0,
    "preferred_dimensions": 1536,
    "allow_dimension_reduction": true
  },
  "fallback_behavior": {
    "enabled": true,
    "max_retries": 3,
    "retry_delay": "1s",
    "use_cache_on_failure": true
  },
  "metadata": {
    "team": "research",
    "project": "knowledge-base"
  }
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440003",
  "agent_id": "new-agent-001",
  "version": 1,
  "embedding_strategy": "balanced",
  "model_preferences": {
    "general_qa": {
      "primary_models": ["text-embedding-3-small"],
      "fallback_models": ["text-embedding-ada-002"]
    }
  },
  "constraints": {
    "max_tokens_per_request": 8000,
    "max_cost_per_day": 25.0,
    "preferred_dimensions": 1536,
    "allow_dimension_reduction": true
  },
  "is_active": true,
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T10:00:00Z"
}
```

### Get Agent Configuration

Retrieve the current configuration for an agent.

**Endpoint:** `GET /api/v1/embeddings/agents/{agentId}`

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440003",
  "agent_id": "research-001",
  "version": 2,
  "embedding_strategy": "quality",
  "model_preferences": {
    "general_qa": {
      "primary_models": ["text-embedding-3-large"],
      "fallback_models": ["text-embedding-3-small"]
    }
  },
  "constraints": {
    "max_tokens_per_request": 8000,
    "max_cost_per_day": 50.0,
    "preferred_dimensions": 3072,
    "allow_dimension_reduction": false
  },
  "is_active": true,
  "created_at": "2024-01-10T08:00:00Z",
  "updated_at": "2024-01-14T15:30:00Z"
}
```

### Update Agent Configuration

Update an agent's embedding configuration.

**Endpoint:** `PUT /api/v1/embeddings/agents/{agentId}`

**Request Body:**
```json
{
  "embedding_strategy": "quality",
  "model_preferences": {
    "general_qa": {
      "primary_models": ["text-embedding-3-large"],
      "fallback_models": ["text-embedding-3-small"]
    }
  },
  "constraints": {
    "max_cost_per_day": 75.0,
    "preferred_dimensions": 3072
  }
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440004",
  "agent_id": "research-001",
  "version": 3,
  "embedding_strategy": "quality",
  "model_preferences": {
    "general_qa": {
      "primary_models": ["text-embedding-3-large"],
      "fallback_models": ["text-embedding-3-small"]
    }
  },
  "constraints": {
    "max_tokens_per_request": 8000,
    "max_cost_per_day": 75.0,
    "preferred_dimensions": 3072,
    "allow_dimension_reduction": false
  },
  "is_active": true,
  "created_at": "2024-01-10T08:00:00Z",
  "updated_at": "2024-01-15T11:00:00Z"
}
```

### Get Agent Models

Get the models assigned to an agent for a specific task type.

**Endpoint:** `GET /api/v1/embeddings/agents/{agentId}/models`

**Response:**
```json
{
  "agent_id": "research-001",
  "task_type": "general_qa",
  "primary_models": [
    "text-embedding-3-large",
    "amazon.titan-embed-text-v2:0"
  ],
  "fallback_models": [
    "text-embedding-3-small",
    "text-embedding-ada-002"
  ]
}
```

### Get Agent Costs

Get cost metrics for an agent.

**Endpoint:** `GET /api/v1/embeddings/agents/{agentId}/costs`

**Response:**
```json
{
  "status": 503,
  "error": {
    "code": "SERVICE_UNAVAILABLE",
    "message": "Cost metrics not yet implemented"
  }
}
```

*Note: Cost tracking is pending implementation.*

## Error Responses

All error responses follow this format:

```json
{
  "code": "ERROR_CODE",
  "message": "Human-readable error message",
  "details": {
    "field": "additional context"
  },
  "trace_id": "550e8400-e29b-41d4-a716-446655440005"
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request parameters |
| `UNAUTHORIZED` | 401 | Missing or invalid authentication |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `INTERNAL_SERVER_ERROR` | 500 | Server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |

## Rate Limiting

API requests are rate-limited per API key:

- **Default**: 100 requests per minute
- **Premium**: 1000 requests per minute

Rate limit headers:
- `X-RateLimit-Limit`: Request limit
- `X-RateLimit-Remaining`: Remaining requests
- `X-RateLimit-Reset`: Reset timestamp

## Best Practices

1. **Agent Configuration**
   - Use task-specific model preferences
   - Set appropriate cost limits
   - Enable fallback behavior for production

2. **Performance**
   - Use batch endpoints for multiple embeddings
   - Cache embeddings when possible
   - Monitor provider health status

3. **Cost Optimization**
   - Use smaller models for high-volume tasks
   - Set daily cost limits per agent
   - Monitor cost metrics regularly

4. **Error Handling**
   - Implement exponential backoff for retries
   - Handle provider failures gracefully

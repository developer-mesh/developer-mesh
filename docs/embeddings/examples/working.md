# Embedding API - Working Examples

This document contains ONLY the embedding API endpoints that are actually implemented and functional.

## Authentication

All API endpoints require authentication via API key:

```bash
# Using API key from organization registration
export API_KEY="devmesh_xxxxxxxxxxxxx"
export BASE_URL="http://localhost:8081"
```

## Working Endpoints

### 1. Generate Embedding

Generate a vector embedding for text.

```bash
curl -X POST "$BASE_URL/api/v1/embeddings" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "DevOps automation with AI agents",
    "agent_id": "my-agent-id",
    "task_type": "general_qa"
  }'
```

**Response:**
```json
{
  "embedding": [0.123, -0.456, ...],
  "dimensions": 1536,
  "model": "text-embedding-3-small",
  "provider": "openai",
  "metadata": {
    "tokens": 8,
    "processing_time_ms": 125
  }
}
```

### 2. Batch Generate Embeddings

Generate embeddings for multiple texts at once.

```bash
curl -X POST "$BASE_URL/api/v1/embeddings/batch" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "texts": [
      "First text to embed",
      "Second text to embed",
      "Third text to embed"
    ],
    "agent_id": "my-agent-id"
  }'
```

**Response:**
```json
{
  "embeddings": [
    {
      "embedding": [0.123, -0.456, ...],
      "dimensions": 1536
    },
    {
      "embedding": [0.234, -0.567, ...],
      "dimensions": 1536
    },
    {
      "embedding": [0.345, -0.678, ...],
      "dimensions": 1536
    }
  ],
  "model": "text-embedding-3-small",
  "provider": "openai",
  "metadata": {
    "total_tokens": 24,
    "processing_time_ms": 250
  }
}
```

### 3. Search Embeddings

Search for similar embeddings using vector similarity.

```bash
curl -X POST "$BASE_URL/api/v1/embeddings/search" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How to implement CI/CD pipelines",
    "agent_id": "my-agent-id",
    "limit": 5,
    "threshold": 0.7
  }'
```

**Response:**
```json
{
  "results": [
    {
      "id": "emb_123",
      "text": "CI/CD pipeline implementation guide",
      "similarity": 0.92,
      "metadata": {
        "created_at": "2024-01-15T10:00:00Z"
      }
    },
    {
      "id": "emb_456",
      "text": "Jenkins pipeline configuration",
      "similarity": 0.85,
      "metadata": {
        "created_at": "2024-01-14T15:30:00Z"
      }
    }
  ],
  "query_embedding_model": "text-embedding-3-small",
  "search_time_ms": 45
}
```

### 4. Cross-Model Search

Search across embeddings from different models (if configured).

```bash
curl -X POST "$BASE_URL/api/v1/embeddings/search/cross-model" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Kubernetes deployment strategies",
    "agent_id": "my-agent-id",
    "models": ["text-embedding-3-small", "text-embedding-ada-002"],
    "limit": 10
  }'
```

### 5. Get Provider Health

Check the health status of embedding providers.

```bash
curl -X GET "$BASE_URL/api/v1/embeddings/providers/health" \
  -H "Authorization: Bearer $API_KEY"
```

**Response:**
```json
{
  "providers": {
    "openai": {
      "status": "healthy",
      "last_check": "2024-01-15T10:00:00Z",
      "response_time_ms": 150
    },
    "bedrock": {
      "status": "healthy",
      "last_check": "2024-01-15T10:00:00Z",
      "response_time_ms": 200
    }
  }
}
```

## Agent Configuration Endpoints

### 6. Create Agent Configuration

Configure embedding preferences for an agent.

```bash
curl -X POST "$BASE_URL/api/v1/embeddings/agents" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "my-new-agent",
    "name": "My AI Agent",
    "preferred_model": "text-embedding-3-small",
    "fallback_model": "text-embedding-ada-002",
    "task_types": ["general_qa", "code_search"]
  }'
```

### 7. Get Agent Configuration

```bash
curl -X GET "$BASE_URL/api/v1/embeddings/agents/my-agent-id" \
  -H "Authorization: Bearer $API_KEY"
```

### 8. Update Agent Configuration

```bash
curl -X PUT "$BASE_URL/api/v1/embeddings/agents/my-agent-id" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "preferred_model": "text-embedding-3-large",
    "max_tokens": 8192
  }'
```

### 9. Get Agent Models

Get available models for a specific agent.

```bash
curl -X GET "$BASE_URL/api/v1/embeddings/agents/my-agent-id/models" \
  -H "Authorization: Bearer $API_KEY"
```

### 10. Get Agent Costs

Get cost analysis for an agent's embedding usage.

```bash
curl -X GET "$BASE_URL/api/v1/embeddings/agents/my-agent-id/costs" \
  -H "Authorization: Bearer $API_KEY"
```

## Error Responses

### 400 Bad Request
```json
{
  "code": "BAD_REQUEST",
  "message": "Invalid request: text field is required"
}
```

### 401 Unauthorized
```json
{
  "code": "UNAUTHORIZED",
  "message": "Invalid or missing API key"
}
```

### 500 Internal Server Error
```json
{
  "code": "INTERNAL_ERROR",
  "message": "Failed to generate embedding"
}
```

## Important Notes

1. **Tenant ID**: The tenant ID is automatically extracted from your API key authentication
2. **Agent ID**: You should create agent configurations for different use cases
3. **Task Types**: Available task types include:
   - `general_qa` - General question answering
   - `code_search` - Code similarity search
   - `document_search` - Document retrieval
   - `semantic_search` - Semantic similarity

## Prerequisites

For these endpoints to work, you need:

1. **Embedding Provider Configuration**: At least one embedding provider (OpenAI, Bedrock, etc.) must be configured in your environment
2. **API Keys**: Valid API keys for the embedding providers
3. **Database**: PostgreSQL with pgvector extension for storing embeddings

## Configuration

Embedding providers are configured via environment variables or configuration files:

```yaml
# config.yaml
embedding:
  providers:
    openai:
      enabled: true
      api_key: ${OPENAI_API_KEY}
      models:
        - text-embedding-3-small
        - text-embedding-3-large
    bedrock:
      enabled: true
      region: us-east-1
      models:
        - amazon.titan-embed-text-v1
```

## What's NOT Working

The following features exist in code but are NOT accessible via API:

- ❌ Model Catalog Management (`/api/v1/embedding-models/catalog/*`)
- ❌ Tenant Model Configuration (`/api/v1/tenant-models/*`)
- ❌ Usage Tracking and Quotas
- ❌ Model Selection API
- ❌ Per-tenant model limits

These features require the `ModelCatalogAPI` to be registered in the server, which is currently not done.
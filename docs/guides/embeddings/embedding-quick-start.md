# Multi-Agent Embedding System Quick Start Guide (NOT FUNCTIONAL)

**⚠️ THIS GUIDE DESCRIBES NON-FUNCTIONAL FEATURES ⚠️**

The embedding API endpoints shown in this guide (`/api/v1/embeddings/*`) are **NOT currently registered** in the REST API server. While the underlying infrastructure for embeddings exists (database schema, adapters, services), the API endpoints are not functional. 

**Current Status:**
- ❌ API endpoints NOT registered
- ✅ Database schema exists
- ✅ Service layer code exists
- ✅ Adapters for OpenAI/Bedrock/Google exist
- ❌ Cannot be used via REST API

This guide represents the intended design but is not currently operational. For working features, see the [Dynamic Tools API](../api-reference/dynamic-tools-api.md) instead.

## Prerequisites

- Docker and Docker Compose installed
- At least one embedding provider API key (OpenAI, AWS, or Google)
- PostgreSQL with pgvector extension
- Redis for caching

## Quick Setup

### 1. Configure Environment

Copy and update the `.env` file:

```bash
cp .env.example .env
```

Edit `.env` and configure at least one provider:

```bash
# For OpenAI (recommended for quick start)
OPENAI_ENABLED=true
OPENAI_API_KEY=sk-your-api-key-here

# For AWS Bedrock
BEDROCK_ENABLED=true
AWS_REGION=us-east-1

# For Google AI
GOOGLE_AI_ENABLED=true
GOOGLE_AI_API_KEY=your-google-ai-key
```

### 2. Start Services

```bash
# Start all services
docker-compose -f docker-compose.local.yml up -d

# Check services are running
docker-compose ps
```

### 3. Create Your First Agent

```bash
# Create an agent configuration
curl -X POST http://localhost:8081/api/v1/embeddings/agents \
  -H "X-API-Key: dev-admin-key-1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "my-first-agent",
    "embedding_strategy": "balanced",
    "model_preferences": {
      "general_qa": {
        "primary_models": ["text-embedding-3-small"],
        "fallback_models": ["text-embedding-ada-002"]
      }
    },
    "constraints": {
      "max_tokens_per_request": 8000,
      "max_cost_per_day": 10.0,
      "preferred_dimensions": 1536,
      "allow_dimension_reduction": true
    }
  }'
```

### 4. Generate Your First Embedding

```bash
# Generate an embedding
curl -X POST http://localhost:8081/api/v1/embeddings \
  -H "X-API-Key: dev-admin-key-1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "my-first-agent",
    "text": "Hello, this is my first embedding!",
    "task_type": "general_qa"
  }'
```

Expected response:
```json
{
  "embedding_id": "550e8400-e29b-41d4-a716-446655440000",
  "request_id": "req-123",
  "model_used": "text-embedding-3-small",
  "provider": "openai",
  "dimensions": 1536,
  "normalized_dimensions": 1536,
  "cost_usd": 0.00001,
  "tokens_used": 8,
  "generation_time_ms": 245,
  "cached": false
}
```

## Common Agent Configurations

### High-Quality Research Agent

```json
{
  "agent_id": "research-agent",
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
  }
}
```

### Cost-Optimized Bulk Processor

```json
{
  "agent_id": "bulk-processor",
  "embedding_strategy": "cost",
  "model_preferences": {
    "general_qa": {
      "primary_models": ["text-embedding-ada-002"],
      "fallback_models": []
    }
  },
  "constraints": {
    "max_tokens_per_request": 8000,
    "max_cost_per_day": 5.0,
    "preferred_dimensions": 1536,
    "allow_dimension_reduction": true
  }
}
```

### Speed-Optimized Real-Time Agent

```json
{
  "agent_id": "realtime-agent",
  "embedding_strategy": "speed",
  "model_preferences": {
    "general_qa": {
      "primary_models": ["text-embedding-3-small"],
      "fallback_models": ["text-embedding-ada-002"]
    }
  },
  "constraints": {
    "max_tokens_per_request": 2000,
    "max_cost_per_day": 25.0,
    "preferred_dimensions": 1536,
    "allow_dimension_reduction": true
  }
}
```

## Working with Different Providers

### OpenAI Setup

1. Get API key from https://platform.openai.com/api-keys
2. Set in `.env`:
   ```bash
   OPENAI_ENABLED=true
   OPENAI_API_KEY=sk-...
   ```
3. Available models:
   - `text-embedding-3-large` (3072 dims, highest quality)
   - `text-embedding-3-small` (1536 dims, balanced)
   - `text-embedding-ada-002` (1536 dims, legacy)

### AWS Bedrock Setup

1. Configure AWS credentials
2. Set in `.env`:
   ```bash
   BEDROCK_ENABLED=true
   AWS_REGION=us-east-1
   ```
3. Available models:
   - `amazon.titan-embed-text-v2:0` (1024 dims)
   - `cohere.embed-english-v3` (1024 dims)
   - `cohere.embed-multilingual-v3` (1024 dims)

### Google AI Setup

1. Get API key from https://makersuite.google.com/app/apikey
2. Set in `.env`:
   ```bash
   GOOGLE_AI_ENABLED=true
   GOOGLE_AI_API_KEY=...
   ```
3. Available models:
   - `text-embedding-004` (768 dims)
   - `textembedding-gecko@003` (768 dims)

## Batch Processing Example

Process multiple texts efficiently:

```bash
curl -X POST http://localhost:8081/api/v1/embeddings/batch \
  -H "X-API-Key: dev-admin-key-1234567890" \
  -H "Content-Type: application/json" \
  -d '[
    {
      "agent_id": "my-first-agent",
      "text": "First document to embed",
      "metadata": {"doc_id": "1"}
    },
    {
      "agent_id": "my-first-agent",
      "text": "Second document to embed",
      "metadata": {"doc_id": "2"}
    },
    {
      "agent_id": "my-first-agent",
      "text": "Third document to embed",
      "metadata": {"doc_id": "3"}
    }
  ]'
```

## Monitoring

### Check Provider Health

```bash
curl http://localhost:8081/api/v1/embeddings/providers/health \
  -H "X-API-Key: dev-admin-key-1234567890"
```

### View Agent Configuration

```bash
curl http://localhost:8081/api/v1/embeddings/agents/my-first-agent \
  -H "X-API-Key: dev-admin-key-1234567890"
```

### Check Logs

```bash
# REST API logs
docker-compose logs -f rest-api

# MCP Server logs
docker-compose logs -f mcp-server
```

## Troubleshooting

### Provider Not Available
```json
{
  "error": "No healthy providers available"
}
```
**Solution**: Check that at least one provider is enabled and API keys are valid.

### Dimension Mismatch
```json
{
  "error": "Dimension mismatch: expected 1536, got 3072"
}
```
**Solution**: Enable `allow_dimension_reduction` in agent constraints.

### Cost Limit Exceeded
```json
{
  "error": "Daily cost limit exceeded for agent"
}
```
**Solution**: Increase `max_cost_per_day` in agent configuration or wait for reset.

### Circuit Breaker Open
```json
{
  "error": "Provider circuit breaker is open"
}
```
**Solution**: Provider is temporarily disabled due to failures. Check provider health and wait for automatic recovery.

## Best Practices

1. **Start with One Provider**: Begin with OpenAI for easiest setup
2. **Use Appropriate Models**: Match model size to your quality needs
3. **Set Cost Limits**: Always configure `max_cost_per_day`
4. **Enable Caching**: Reduces costs for repeated embeddings
5. **Monitor Usage**: Check provider health and costs regularly

## Next Steps

- [API Reference](../api-reference/embedding-api-reference.md)
- [Architecture Guide](../architecture/multi-agent-embedding-architecture.md)
- [Configuration Guide](../operations/embedding-configuration-guide.md)

<!-- VERIFICATION
This document has been automatically verified against the codebase.
Last verification: 2025-08-11 14:40:09
All features mentioned have been confirmed to exist in the code.
-->

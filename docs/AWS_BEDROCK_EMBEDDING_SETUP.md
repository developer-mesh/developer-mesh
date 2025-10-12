# AWS Bedrock Embedding Model Setup Guide

This guide explains how to configure AWS Bedrock Titan embedding models for use with DevMesh semantic context management.

## Prerequisites

- PostgreSQL database running with DevMesh schema
- AWS Bedrock access configured (credentials in environment or AWS config)
- REST API running on port 8081

## Architecture Overview

DevMesh uses a multi-tenant embedding model management system with three main components:

1. **embedding_model_catalog** - Global registry of available embedding models
2. **tenant_embedding_models** - Per-tenant model access and quota configuration
3. **agent_embedding_preferences** - Agent-specific model selection preferences

## Quick Setup for AWS Bedrock Titan

You can configure AWS Bedrock models using **either REST API or direct database access**.

### Option 1: REST API Configuration (Recommended)

✅ **Model Catalog API** endpoints are now available and fully functional:
- `GET /api/v1/embedding-models/catalog` - List all available embedding models
- `GET /api/v1/tenant-models` - List tenant-specific model configurations

You can query these endpoints to verify model availability before using direct database configuration.

### Option 2: Database Configuration Script

Run the provided SQL configuration script:

```bash
docker exec -i devops-mcp-database-1 psql -U devmesh -d devmesh_development \
  < scripts/db/configure-bedrock-embeddings.sql
```

This script:
- Ensures Titan models are available in the catalog
- Configures tenant `00000000-0000-0000-0000-000000000001` to use Titan V2 as default
- Sets up Titan V1 as a fallback
- Displays configuration summary

### Option 2: Manual SQL (Advanced)

If you prefer manual setup or need to configure a specific tenant, use this SQL:

```sql
-- 1. Add AWS Bedrock Titan models to the catalog
INSERT INTO mcp.embedding_model_catalog (
    id, model_id, provider, provider_display_name,
    max_input_tokens, dimensions, cost_per_million_tokens,
    model_type, normalization_type, similarity_metric,
    supports_batching, max_batch_size, is_available,
    created_at, updated_at
) VALUES
    -- Titan Text Embeddings V2 (Latest, recommended)
    (
        gen_random_uuid(),
        'amazon.titan-embed-text-v2:0',
        'aws_bedrock',
        'AWS Bedrock',
        8192,  -- max tokens
        1024,  -- dimensions
        0.02,  -- $0.02 per million tokens
        'text',
        'l2',
        'cosine',
        true,
        128,   -- max batch size
        true,
        NOW(),
        NOW()
    ),
    -- Titan Text Embeddings V1 (Fallback)
    (
        gen_random_uuid(),
        'amazon.titan-embed-text-v1',
        'aws_bedrock',
        'AWS Bedrock',
        8192,
        1536,  -- larger dimensions
        0.02,
        'text',
        'l2',
        'cosine',
        true,
        128,
        true,
        NOW(),
        NOW()
    )
ON CONFLICT (provider, model_id) DO UPDATE SET
    is_available = true,
    cost_per_million_tokens = EXCLUDED.cost_per_million_tokens,
    updated_at = NOW();

-- 2. Get the model ID we just created
DO $$
DECLARE
    titan_v2_id UUID;
    target_tenant_id UUID := '00000000-0000-0000-0000-000000000001'; -- Your tenant ID
BEGIN
    -- Get Titan V2 model ID
    SELECT id INTO titan_v2_id
    FROM mcp.embedding_model_catalog
    WHERE model_id = 'amazon.titan-embed-text-v2:0';

    -- 3. Configure the tenant to use Titan V2
    INSERT INTO mcp.tenant_embedding_models (
        tenant_id,
        model_id,
        is_enabled,
        is_default,
        monthly_token_limit,
        daily_token_limit,
        monthly_request_limit,
        priority,
        created_at,
        updated_at
    ) VALUES (
        target_tenant_id,
        titan_v2_id,
        true,        -- enabled
        true,        -- default model for this tenant
        10000000,    -- 10M tokens per month
        1000000,     -- 1M tokens per day
        100000,      -- 100k requests per month
        100,         -- high priority
        NOW(),
        NOW()
    )
    ON CONFLICT (tenant_id, model_id) DO UPDATE SET
        is_enabled = true,
        is_default = true,
        priority = EXCLUDED.priority,
        updated_at = NOW();

    RAISE NOTICE 'Configured Titan V2 (%) for tenant %', titan_v2_id, target_tenant_id;
END $$;
```

### Step 3: Verify Configuration

```sql
-- Check that models are configured
SELECT
    c.model_id,
    c.provider,
    c.dimensions,
    c.is_available,
    tm.is_enabled,
    tm.is_default,
    tm.monthly_token_limit
FROM mcp.embedding_model_catalog c
LEFT JOIN mcp.tenant_embedding_models tm ON c.id = tm.model_id
WHERE c.provider = 'aws_bedrock'
    AND (tm.tenant_id = '00000000-0000-0000-0000-000000000001' OR tm.tenant_id IS NULL);
```

Expected output:
```
model_id                        | provider    | dimensions | is_available | is_enabled | is_default | monthly_token_limit
--------------------------------+-------------+------------+--------------+------------+------------+--------------------
amazon.titan-embed-text-v2:0    | aws_bedrock | 1024       | t            | t          | t          | 10000000
```

## Testing Embeddings

### Important: Manual vs Automatic Embedding Generation

**Current Status**: The REST API (`/api/v1/contexts`) does NOT automatically generate embeddings when context items are created. Embeddings must be generated manually via the `/api/v1/embeddings` endpoint.

**Why**: Automatic embedding generation is implemented in the semantic context manager (`pkg/core/semantic_context_manager_impl.go`) but is not currently integrated with the REST API context handler. The semantic context manager requires explicit initialization with an embedding client.

**Embedding Generation Options**:
1. **Manual Generation** (✅ working now): Use `/api/v1/embeddings` API
2. **Automatic Generation** (⏳ requires integration): Wire up semantic context manager

### Option A: Manual Embedding Generation (Recommended for Now)

Generate embeddings for text using the embedding API:

```bash
# Generate a single embedding
curl -X POST http://localhost:8081/api/v1/embeddings \
  -H "Authorization: Bearer dev-admin-key-1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "What is AWS Bedrock and how does it work with embeddings?",
    "agent_id": "test-agent-001"
  }'
```

Expected response:
```json
{
  "embedding_id": "c0c8012f-e83b-4f54-8a6c-97743b73ca22",
  "model_used": "amazon.titan-embed-text-v2:0",
  "provider": "bedrock",
  "dimensions": 1024,
  "cost_usd": 0.00000028,
  "tokens_used": 14,
  "generation_time_ms": 541
}
```

### Option B: Context Creation (For Reference)

While contexts can be created, they do NOT automatically generate embeddings:

### 1. Create a Context

```bash
curl -X POST http://localhost:8081/api/v1/contexts \
  -H "Authorization: Bearer dev-admin-key-1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "conversation",
    "metadata": {
      "purpose": "testing bedrock embeddings"
    }
  }'
```

Save the returned `id` from the response.

### 2. Add Content to Context

**⚠️ Important**: This does NOT automatically generate embeddings. You must use `/api/v1/embeddings` separately.

```bash
curl -X PUT http://localhost:8081/api/v1/contexts/{CONTEXT_ID} \
  -H "Authorization: Bearer dev-admin-key-1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "content": [
      {
        "role": "user",
        "content": "What is AWS Bedrock and how does it work with embeddings?"
      },
      {
        "role": "assistant",
        "content": "AWS Bedrock is a fully managed service that provides access to foundation models including Titan embeddings which convert text into vector representations for semantic search."
      }
    ]
  }'
```

### 3. Generate Embeddings (Manual Step Required)

After creating context content, generate embeddings using the embedding API:

```bash
# Single embedding generation
curl -X POST http://localhost:8081/api/v1/embeddings \
  -H "Authorization: Bearer dev-admin-key-1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "What is AWS Bedrock and how does it work with embeddings?",
    "agent_id": "test-agent-001"
  }'

# Batch embedding generation (now supports string agent IDs)
curl -X POST http://localhost:8081/api/v1/embeddings/batch \
  -H "Authorization: Bearer dev-admin-key-1234567890" \
  -H "Content-Type: application/json" \
  -d '[
    {"text": "First text to embed", "agent_id": "test-agent-001"},
    {"text": "Second text to embed", "agent_id": "test-agent-001"}
  ]'
```

✅ **Note**: Both single and batch endpoints now support string agent IDs (e.g., "test-agent-001") as well as UUIDs. If no agent configuration exists in the database, the system gracefully falls back to default model selection.

### 4. Verify Embeddings (For Future Automatic Integration)

```sql
-- Check for embeddings
SELECT
    ci.content,
    e.model_id,
    em.model_id as model_name,
    e.vector IS NOT NULL as has_vector,
    array_length(e.vector, 1) as vector_dimensions
FROM mcp.context_items ci
JOIN mcp.context_embeddings ce ON ce.context_id = ci.context_id
JOIN mcp.embeddings e ON e.id = ce.embedding_id
JOIN mcp.embedding_model_catalog em ON em.id = e.model_id
WHERE ci.context_id = '{CONTEXT_ID}'
ORDER BY ci.sequence_number;
```

Expected output should show:
- `has_vector`: true
- `vector_dimensions`: 1024 (for Titan V2)
- `model_name`: amazon.titan-embed-text-v2:0

**Note**: This verification step will only work once automatic embedding generation is integrated with the REST API.

### 5. Check Embedding Metrics

```sql
-- View usage statistics
SELECT
    model_id,
    COUNT(*) as embedding_count,
    SUM(token_count) as total_tokens,
    AVG(latency_ms) as avg_latency_ms,
    SUM(cost) as total_cost
FROM mcp.embedding_metrics
WHERE tenant_id = '00000000-0000-0000-0000-000000000001'
    AND created_at >= NOW() - INTERVAL '1 day'
GROUP BY model_id;
```

## Environment Variables

Ensure AWS credentials are configured for Bedrock access:

```bash
# Option 1: AWS credentials file (~/.aws/credentials)
[default]
aws_access_key_id = YOUR_ACCESS_KEY
aws_secret_access_key = YOUR_SECRET_KEY
aws_region = us-east-1

# Option 2: Environment variables
export AWS_ACCESS_KEY_ID=YOUR_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_KEY
export AWS_REGION=us-east-1

# Option 3: Docker Compose (docker-compose.local.yml)
environment:
  - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
  - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
  - AWS_REGION=us-east-1
```

## Troubleshooting

### No embeddings generated

1. **Check if model is enabled for tenant:**
   ```sql
   SELECT c.model_id, c.model_name, tm.is_enabled, tm.is_default, tm.priority
   FROM mcp.embedding_model_catalog c
   JOIN mcp.tenant_embedding_models tm ON c.id = tm.model_id
   WHERE tm.tenant_id = '00000000-0000-0000-0000-000000000001'
   ORDER BY tm.priority DESC;
   ```

2. **Check embedding queue (usually empty for sync generation):**
   ```sql
   SELECT * FROM mcp.embedding_queue
   WHERE status = 'pending'
   ORDER BY created_at DESC LIMIT 10;
   ```
   **Note**: Embeddings are generated synchronously via `/api/v1/embeddings`, so the queue is typically empty.

3. **Check REST API logs:**
   ```bash
   docker logs devops-mcp-rest-api-1 --tail 50 | grep -i "embedding\|error"
   ```

4. **Verify embeddings are being stored:**
   ```sql
   SELECT COUNT(*) as total,
          MAX(embedding_created_at) as last_created
   FROM mcp.embeddings;
   ```

### Checking Provider Health

✅ The Bedrock provider health check has been improved and should now accurately reflect the provider status:

```bash
# Check provider health
curl -X GET http://localhost:8081/api/v1/embeddings/providers/health \
  -H "Authorization: Bearer dev-admin-key-1234567890"
```

Expected response:
```json
{
  "providers": {
    "bedrock": {
      "name": "bedrock",
      "status": "healthy",
      "circuit_breaker_state": "closed"
    }
  },
  "timestamp": "2025-10-12T00:00:00.000Z"
}
```

The health check now only fails for actual authentication or connectivity issues, not for model-specific errors. If you see "unhealthy" status, verify your AWS credentials and network connectivity.

### Quota limits exceeded

Check current usage:
```sql
SELECT
    tenant_id,
    SUM(tokens_used) as tokens_this_month,
    COUNT(*) as requests_this_month
FROM mcp.embedding_usage_tracking
WHERE created_at >= date_trunc('month', CURRENT_DATE)
GROUP BY tenant_id;
```

Compare with limits:
```sql
SELECT tenant_id, monthly_token_limit, monthly_request_limit
FROM mcp.tenant_embedding_models
WHERE tenant_id = '00000000-0000-0000-0000-000000000001';
```

### Model selection issues

Test the model selection function:
```sql
SELECT * FROM mcp.get_embedding_model_for_request(
    '00000000-0000-0000-0000-000000000001'::UUID,  -- tenant_id
    NULL,  -- agent_id (optional)
    NULL,  -- task_type (optional)
    NULL   -- requested_model_id (optional)
);
```

## Cost Optimization

AWS Bedrock Titan pricing (as of 2025):
- **Titan Text Embeddings V2**: $0.02 per million tokens
- **Titan Text Embeddings V1**: $0.02 per million tokens

To reduce costs:

1. **Enable caching:**
   ```sql
   UPDATE mcp.tenant_embedding_models
   SET cache_ttl_seconds = 86400  -- 24 hours
   WHERE tenant_id = '00000000-0000-0000-0000-000000000001';
   ```

2. **Set conservative limits:**
   ```sql
   UPDATE mcp.tenant_embedding_models
   SET
       daily_token_limit = 100000,      -- 100k tokens/day
       monthly_token_limit = 2000000    -- 2M tokens/month
   WHERE tenant_id = '00000000-0000-0000-0000-000000000001';
   ```

3. **Use batch processing:**
   - Embeddings are automatically batched (up to 128 items)
   - No additional configuration needed

## Advanced Configuration

### Agent-Specific Preferences

Configure different models for different agents:

```sql
INSERT INTO mcp.agent_embedding_preferences (
    tenant_id,
    agent_id,
    selection_strategy,
    primary_model_id,
    fallback_model_ids,
    max_cost_per_request,
    monthly_budget
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'your-agent-id',
    'cost_optimized',  -- or 'latency_optimized', 'quality_optimized'
    (SELECT id FROM mcp.embedding_model_catalog WHERE model_id = 'amazon.titan-embed-text-v2:0'),
    ARRAY[(SELECT id FROM mcp.embedding_model_catalog WHERE model_id = 'amazon.titan-embed-text-v1')],
    0.001,  -- max $0.001 per request
    10.00   -- $10/month budget
);
```

### Multiple Models

Enable multiple providers as fallbacks:

```sql
-- Enable OpenAI as fallback
INSERT INTO mcp.tenant_embedding_models (tenant_id, model_id, is_enabled, priority)
SELECT
    '00000000-0000-0000-0000-000000000001',
    id,
    true,
    50  -- lower priority than Titan
FROM mcp.embedding_model_catalog
WHERE model_id = 'text-embedding-ada-002';
```

## References

- [AWS Bedrock Titan Documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/titan-embedding-models.html)
- [DevMesh Semantic Context Implementation](./proposals/SEMANTIC_CONTEXT_COMPLETE_IMPLEMENTATION.md)
- [Multi-Tenant Embedding Management](../migrations/)

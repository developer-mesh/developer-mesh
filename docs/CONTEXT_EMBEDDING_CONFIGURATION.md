# Context Embedding Configuration Guide

## Overview

This guide documents the configuration for automatic context embedding generation in DevMesh. Context embeddings are generated asynchronously by the Worker service when context items are created or updated via the REST API.

## Architecture Flow

```
User/Agent → REST API → Context Manager → Queue Event
                                              ↓
                                         Redis Stream
                                              ↓
                                      Worker Service
                                              ↓
                                  Context Embedding Processor
                                              ↓
                              AWS Bedrock / OpenAI / Google AI
                                              ↓
                                       PostgreSQL + pgvector
```

## Configuration System

DevMesh uses Viper for configuration management with the following precedence:
1. Environment variables (highest priority)
2. Configuration file (YAML)
3. Default values (lowest priority)

### Environment Variable Prefix

All configuration options can be set via environment variables with the `MCP_` prefix:
- Format: `MCP_SECTION_SUBSECTION_KEY=value`
- Example: `MCP_EMBEDDING_PROVIDERS_BEDROCK_ENABLED=true`

### Variable Interpolation

Configuration files support `${VAR:-default}` syntax for environment variable interpolation:
```yaml
embedding:
  providers:
    bedrock:
      region: ${AWS_REGION:-us-east-1}
```

## Embedding Provider Configuration

### AWS Bedrock (Recommended)

**Required Configuration:**
```bash
# Enable Bedrock provider
MCP_EMBEDDING_PROVIDERS_BEDROCK_ENABLED=true

# AWS Region where Bedrock is available
MCP_EMBEDDING_PROVIDERS_BEDROCK_REGION=us-east-1

# Optional: Custom endpoint (for testing/VPC endpoints)
MCP_EMBEDDING_PROVIDERS_BEDROCK_ENDPOINT=https://bedrock-runtime.us-east-1.amazonaws.com
```

**Supported Models:**
- `amazon.titan-embed-text-v2:0` (default, 1024 dimensions)
- `amazon.titan-embed-text-v1` (1536 dimensions)
- `cohere.embed-english-v3` (1024 dimensions)
- `cohere.embed-multilingual-v3` (1024 dimensions)

**Authentication:**

**Development (Local/Docker):**
```bash
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your_access_key_id
AWS_SECRET_ACCESS_KEY=your_secret_access_key
```

**Production (Recommended - IAM Roles):**
```bash
# No credentials needed - use IAM roles
# ECS Task Role or EC2 Instance Profile
AWS_REGION=us-east-1
```

**Region Format Validation:**
- Pattern: `[a-z]{2}-[a-z]+-[0-9]`
- Examples: `us-east-1`, `eu-west-2`, `ap-southeast-1`
- Minimum length: 9 characters

### OpenAI (Optional)

**Required Configuration:**
```bash
# Enable OpenAI provider
MCP_EMBEDDING_PROVIDERS_OPENAI_ENABLED=true

# OpenAI API Key (required)
MCP_EMBEDDING_PROVIDERS_OPENAI_API_KEY=sk-...
```

**Supported Models:**
- `text-embedding-ada-002` (1536 dimensions)
- `text-embedding-3-small` (1536 dimensions)
- `text-embedding-3-large` (3072 dimensions)

### Google AI (Optional)

**Required Configuration:**
```bash
# Enable Google AI provider
MCP_EMBEDDING_PROVIDERS_GOOGLE_ENABLED=true

# Google AI API Key (required)
MCP_EMBEDDING_PROVIDERS_GOOGLE_API_KEY=AIza...

# Optional: Custom endpoint
MCP_EMBEDDING_PROVIDERS_GOOGLE_ENDPOINT=https://generativelanguage.googleapis.com
```

**Supported Models:**
- `embedding-001` (768 dimensions)
- `text-embedding-004` (768 dimensions)

## Processing Configuration

### Chunk Size

Controls the maximum size of text chunks for embedding generation.

```bash
# Default: 1000 characters
CONTEXT_CHUNK_SIZE=1000
```

**Recommendations:**
- **Minimum:** 100 characters (too small reduces semantic meaning)
- **Maximum:** 8000 characters (Bedrock Titan v2 limit)
- **Optimal:** 500-1500 characters (balances context and performance)

**Model-Specific Limits:**
- AWS Bedrock Titan: 8192 characters
- OpenAI: 8191 tokens (~32,000 characters)
- Google AI: 2048 tokens (~8,000 characters)

### Batch Size

Controls how many embeddings are generated concurrently.

```bash
# Default: 10 embeddings per batch
EMBEDDING_BATCH_SIZE=10
```

**Recommendations:**
- **Low traffic:** 5-10 (reduces memory usage)
- **High traffic:** 20-50 (increases throughput)
- **Maximum:** 100 (rate limiting considerations)

### Default Model

Specifies the default embedding model when tenant/agent preferences aren't set.

```bash
# Default: amazon.titan-embed-text-v2:0
DEFAULT_EMBEDDING_MODEL=amazon.titan-embed-text-v2:0
```

## Docker Compose Configuration

### Local Development

```yaml
worker:
  environment:
    # AWS Configuration
    - AWS_REGION=us-east-1
    - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
    - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}

    # Bedrock Provider
    - MCP_EMBEDDING_PROVIDERS_BEDROCK_ENABLED=true
    - MCP_EMBEDDING_PROVIDERS_BEDROCK_REGION=us-east-1

    # Processing Options
    - DEFAULT_EMBEDDING_MODEL=amazon.titan-embed-text-v2:0
    - CONTEXT_CHUNK_SIZE=1000
    - EMBEDDING_BATCH_SIZE=10
```

### Production Deployment

```yaml
worker:
  environment:
    # Use IAM roles - no credentials needed
    - AWS_REGION=us-east-1

    # Bedrock Provider
    - MCP_EMBEDDING_PROVIDERS_BEDROCK_ENABLED=true
    - MCP_EMBEDDING_PROVIDERS_BEDROCK_REGION=us-east-1
    - MCP_EMBEDDING_PROVIDERS_BEDROCK_ENDPOINT=https://bedrock-runtime.us-east-1.amazonaws.com

    # Processing Options
    - DEFAULT_EMBEDDING_MODEL=amazon.titan-embed-text-v2:0
    - CONTEXT_CHUNK_SIZE=1500
    - EMBEDDING_BATCH_SIZE=25
```

## Configuration Validation

The Worker service validates configuration on startup:

### Validation Rules

1. **Bedrock Enabled:**
   - `BEDROCK_REGION` must be set
   - Region must match AWS format (e.g., `us-east-1`)
   - Region length must be at least 9 characters

2. **OpenAI Enabled:**
   - `OPENAI_API_KEY` must be set and non-empty

3. **Google AI Enabled:**
   - `GOOGLE_AI_API_KEY` must be set and non-empty

### Validation Errors

If validation fails, the Worker logs a warning and disables context embeddings:

```
WARN Failed to validate embedding configuration, context embeddings disabled
  error: bedrock region is required when bedrock is enabled
```

## Security Best Practices

### Development

✅ **DO:**
- Use `.env` files (gitignored) for local credentials
- Use `${VAR}` syntax in docker-compose for secrets
- Rotate API keys regularly

❌ **DON'T:**
- Commit credentials to version control
- Share AWS access keys in plain text
- Use production credentials in development

### Production

✅ **DO:**
- Use IAM roles (ECS Task Roles, EC2 Instance Profiles)
- Use AWS Secrets Manager for API keys
- Enable CloudTrail for audit logging
- Use VPC endpoints for Bedrock access
- Rotate IAM credentials automatically

❌ **DON'T:**
- Use long-lived access keys in production
- Grant overly broad IAM permissions
- Disable TLS/SSL for connections
- Log sensitive credentials

### Required IAM Permissions (Bedrock)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream"
      ],
      "Resource": [
        "arn:aws:bedrock:*::foundation-model/amazon.titan-embed-*",
        "arn:aws:bedrock:*::foundation-model/cohere.embed-*"
      ]
    }
  ]
}
```

## Monitoring and Observability

### Logs

**Startup Logs:**
```
INFO Initializing embedding service
  provider: bedrock
  region: us-east-1
  has_endpoint: false

INFO Context embedding processor initialized successfully
  provider: bedrock
  region: us-east-1
```

**Processing Logs:**
```
INFO Published context embedding event
  context_id: ctx_123
  item_count: 5

INFO Processing context embeddings
  context_id: ctx_123
  item_count: 5

INFO Completed context embedding generation
  context_id: ctx_123
  items_processed: 5
  duration_ms: 1250
```

**Error Logs:**
```
WARN Failed to generate embedding
  error: rate limit exceeded
  context_id: ctx_123
  item_id: item_456

ERROR Failed to link embedding to context
  error: database connection lost
  context_id: ctx_123
  embedding_id: emb_789
```

### Metrics

The Worker exposes Prometheus metrics:

- `context_embeddings_generated_total{tenant_id}` - Counter of embeddings created
- `context_embedding_generation_duration_seconds{tenant_id}` - Histogram of generation time
- `context_embedding_errors_total{tenant_id,error_type}` - Counter of failures
- `context_embedding_queue_depth` - Gauge of pending events

### Health Checks

**Worker Health Endpoint:**
```bash
curl http://localhost:8088/health
```

**Response:**
```json
{
  "status": "healthy",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "queue": "ok",
    "embedding_service": "ok"
  }
}
```

## Troubleshooting

### Common Issues

#### 1. Embeddings Not Generated

**Symptom:** Context items created but no embeddings in database

**Diagnosis:**
```bash
# Check worker logs
docker-compose logs -f worker

# Check queue depth
redis-cli xlen webhook_events

# Verify Bedrock enabled
docker-compose exec worker env | grep BEDROCK
```

**Solutions:**
- Ensure `MCP_EMBEDDING_PROVIDERS_BEDROCK_ENABLED=true`
- Verify AWS credentials are valid
- Check AWS region supports Bedrock
- Confirm Worker service is running

#### 2. AWS Authentication Errors

**Symptom:** `failed to create embedding service: UnrecognizedClientException`

**Solutions:**
- Verify AWS credentials: `aws sts get-caller-identity`
- Check IAM permissions include `bedrock:InvokeModel`
- Ensure region is correct and Bedrock is available
- Use IAM roles instead of access keys in production

#### 3. Rate Limiting

**Symptom:** `rate limit exceeded` errors in logs

**Solutions:**
- Reduce `EMBEDDING_BATCH_SIZE` to 5-10
- Implement exponential backoff (already built-in)
- Request quota increase from AWS
- Consider using multiple AWS accounts/regions

#### 4. High Latency

**Symptom:** Slow embedding generation (>5 seconds per item)

**Solutions:**
- Check network connectivity to Bedrock
- Use VPC endpoints for faster access
- Reduce `CONTEXT_CHUNK_SIZE` to 500-800
- Increase `EMBEDDING_BATCH_SIZE` to 20-30
- Consider using faster models (Titan v2)

#### 5. Memory Issues

**Symptom:** Worker OOM crashes or high memory usage

**Solutions:**
- Reduce `EMBEDDING_BATCH_SIZE` to 5
- Reduce `CONTEXT_CHUNK_SIZE` to 500
- Increase Worker memory allocation
- Check for embedding cache issues

## Testing Configuration

### Local Testing

```bash
# 1. Set environment variables
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=your_key
export AWS_SECRET_ACCESS_KEY=your_secret

# 2. Start services
docker-compose up -d

# 3. Create a context with items
curl -X POST http://localhost:8081/api/v1/contexts \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "test-agent",
    "content": [
      {"role": "user", "content": "Hello, how are you?"},
      {"role": "assistant", "content": "I am doing well, thank you!"}
    ]
  }'

# 4. Check worker logs
docker-compose logs -f worker | grep "embedding"

# 5. Verify embeddings in database
docker-compose exec database psql -U devmesh -d devmesh_development -c \
  "SELECT COUNT(*) FROM mcp.context_embeddings;"
```

### Configuration Validation Test

```bash
# Test invalid region
docker-compose exec worker sh -c '
  export MCP_EMBEDDING_PROVIDERS_BEDROCK_ENABLED=true
  export MCP_EMBEDDING_PROVIDERS_BEDROCK_REGION=invalid
  /app/worker
'
# Expected: "bedrock region format appears invalid: invalid"

# Test missing API key
docker-compose exec worker sh -c '
  export MCP_EMBEDDING_PROVIDERS_OPENAI_ENABLED=true
  export MCP_EMBEDDING_PROVIDERS_OPENAI_API_KEY=
  /app/worker
'
# Expected: "openai API key is required when openai is enabled"
```

## Performance Tuning

### Recommendations by Workload

| Workload | Batch Size | Chunk Size | Concurrency |
|----------|-----------|-----------|-------------|
| Low (< 100 items/day) | 5 | 1000 | 5 |
| Medium (100-1000 items/day) | 10 | 1000 | 10 |
| High (1000-10000 items/day) | 25 | 1500 | 20 |
| Very High (> 10000 items/day) | 50 | 1500 | 50 |

### Cost Optimization

**AWS Bedrock Pricing (Titan Embed v2):**
- $0.10 per 1M input tokens
- Average context item: ~200 tokens
- 1000 embeddings: ~$0.02

**Optimization Strategies:**
1. Enable deduplication (already built-in)
2. Cache embeddings (already built-in)
3. Filter out system messages (already implemented)
4. Use smaller chunk sizes for short items
5. Batch similar contexts together

## Migration Guide

### From Ad-Hoc Config to config.Load()

**Before (Old Approach):**
```go
embeddingConfig := &config.Config{
    Embedding: config.EmbeddingConfig{
        Providers: config.ProvidersConfig{
            Bedrock: config.BedrockConfig{
                Enabled: true,
                Region:  os.Getenv("AWS_REGION"),
            },
        },
    },
}
```

**After (New Approach):**
```go
embeddingConfig, err := config.Load()
if err != nil {
    logger.Warn("Failed to load configuration", ...)
}

if err := validateEmbeddingConfig(&embeddingConfig.Embedding); err != nil {
    logger.Warn("Invalid configuration", ...)
}
```

**Benefits:**
- Supports all configuration sources (env, file, defaults)
- Respects environment variable interpolation
- Validates configuration on load
- Enables optional parameters (endpoint, custom models)
- Consistent with REST API configuration

## References

- [AWS Bedrock Documentation](https://docs.aws.amazon.com/bedrock/)
- [OpenAI Embeddings API](https://platform.openai.com/docs/guides/embeddings)
- [Google AI Embeddings](https://ai.google.dev/docs/embeddings_guide)
- [pgvector Documentation](https://github.com/pgvector/pgvector)
- [Viper Configuration](https://github.com/spf13/viper)

## Support

For issues or questions:
1. Check logs: `docker-compose logs -f worker`
2. Verify configuration: `docker-compose exec worker env | grep EMBEDDING`
3. Test connectivity: `aws bedrock list-foundation-models --region us-east-1`
4. Review metrics: `curl http://localhost:8088/metrics`
5. File issue: https://github.com/developer-mesh/developer-mesh/issues

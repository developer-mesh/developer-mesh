# Embedding Models API Examples

## Authentication
All API endpoints require authentication via API key in the header:
```bash
export API_KEY="your-api-key-here"
export BASE_URL="http://localhost:8081"
```

## Model Catalog Management

### List Available Models
```bash
# List all available models with pagination
curl -X GET "$BASE_URL/api/v1/embedding-models/catalog?page=1&limit=20" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"

# Filter by provider
curl -X GET "$BASE_URL/api/v1/embedding-models/catalog?provider=bedrock&available_only=true" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"

# Filter by model type
curl -X GET "$BASE_URL/api/v1/embedding-models/catalog?type=embedding" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"
```

### Get Specific Model Details
```bash
# Get model by ID
curl -X GET "$BASE_URL/api/v1/embedding-models/catalog/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"
```

### Create New Model
```bash
curl -X POST "$BASE_URL/api/v1/embedding-models/catalog" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "openai",
    "model_name": "Text Embedding 3 Small",
    "model_id": "text-embedding-3-small",
    "model_version": "3.0",
    "dimensions": 1536,
    "max_tokens": 8191,
    "cost_per_million_tokens": 0.02,
    "model_type": "embedding",
    "capabilities": {
      "supports_batching": true,
      "max_batch_size": 2048,
      "encoding": "cl100k_base"
    }
  }'
```

### Update Model
```bash
curl -X PUT "$BASE_URL/api/v1/embedding-models/catalog/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "is_available": false,
    "is_deprecated": true,
    "cost_per_million_tokens": 0.015
  }'
```

### Delete Model
```bash
curl -X DELETE "$BASE_URL/api/v1/embedding-models/catalog/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: $API_KEY"
```

### List Providers
```bash
curl -X GET "$BASE_URL/api/v1/embedding-models/providers" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"
```

## Tenant Model Management

### List Tenant Models
```bash
# List all models for current tenant
curl -X GET "$BASE_URL/api/v1/tenant-models" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"

# List only enabled models
curl -X GET "$BASE_URL/api/v1/tenant-models?enabled_only=true" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"
```

### Configure Model for Tenant
```bash
curl -X POST "$BASE_URL/api/v1/tenant-models" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "550e8400-e29b-41d4-a716-446655440000",
    "is_enabled": true,
    "is_default": false,
    "monthly_token_limit": 10000000,
    "daily_token_limit": 500000,
    "monthly_request_limit": 10000,
    "priority": 100
  }'
```

### Update Tenant Model Configuration
```bash
curl -X PUT "$BASE_URL/api/v1/tenant-models/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "is_enabled": true,
    "monthly_token_limit": 20000000,
    "daily_token_limit": 1000000,
    "priority": 150
  }'
```

### Set Default Model
```bash
curl -X POST "$BASE_URL/api/v1/tenant-models/550e8400-e29b-41d4-a716-446655440000/set-default" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"
```

### Remove Model from Tenant
```bash
curl -X DELETE "$BASE_URL/api/v1/tenant-models/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: $API_KEY"
```

## Usage and Quotas

### Get Usage Statistics
```bash
# Get monthly usage
curl -X GET "$BASE_URL/api/v1/tenant-models/usage?period=month" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"

# Get usage for specific model
curl -X GET "$BASE_URL/api/v1/tenant-models/usage?model_id=550e8400-e29b-41d4-a716-446655440000&period=day" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"
```

### Get Quota Status
```bash
curl -X GET "$BASE_URL/api/v1/tenant-models/quotas" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json"
```

## Model Selection

### Select Best Model for Request
```bash
curl -X POST "$BASE_URL/api/v1/embedding-models/select" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "650e8400-e29b-41d4-a716-446655440001",
    "task_type": "code_search",
    "requested_model": "text-embedding-3-small",
    "token_estimate": 1500
  }'
```

## Response Examples

### Successful Model List Response
```json
{
  "models": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "provider": "bedrock",
      "model_name": "Amazon Titan Embedding V2",
      "model_id": "amazon.titan-embed-text-v2:0",
      "model_version": "2.0",
      "dimensions": 1024,
      "max_tokens": 8192,
      "model_type": "embedding",
      "cost_per_million_tokens": 0.02,
      "is_available": true,
      "is_deprecated": false,
      "requires_api_key": false,
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 15,
    "pages": 1
  }
}
```

### Model Selection Response
```json
{
  "model_id": "550e8400-e29b-41d4-a716-446655440000",
  "model_identifier": "amazon.titan-embed-text-v2:0",
  "provider": "bedrock",
  "dimensions": 1024,
  "cost_per_million_tokens": 0.02,
  "estimated_cost": 0.00003,
  "is_within_quota": true,
  "quota_remaining": 499850,
  "metadata": {
    "selection_reason": "default_model",
    "fallback_available": true
  }
}
```

### Usage Statistics Response
```json
{
  "period": "month",
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-31T23:59:59Z",
  "total_tokens": 5234567,
  "total_requests": 12345,
  "total_cost": 104.69,
  "by_model": {
    "550e8400-e29b-41d4-a716-446655440000": {
      "tokens": 3234567,
      "requests": 8345,
      "cost": 64.69
    },
    "650e8400-e29b-41d4-a716-446655440001": {
      "tokens": 2000000,
      "requests": 4000,
      "cost": 40.00
    }
  }
}
```

### Quota Status Response
```json
{
  "tenant_id": "750e8400-e29b-41d4-a716-446655440002",
  "quotas": [
    {
      "model_id": "550e8400-e29b-41d4-a716-446655440000",
      "monthly_token_limit": 10000000,
      "monthly_tokens_used": 5234567,
      "daily_token_limit": 500000,
      "daily_tokens_used": 234567,
      "monthly_request_limit": 10000,
      "monthly_requests_used": 4567,
      "is_within_limits": true
    }
  ]
}
```

## Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid request body",
  "details": "Dimensions must be positive"
}
```

### 401 Unauthorized
```json
{
  "error": "Tenant ID not found in context"
}
```

### 404 Not Found
```json
{
  "error": "Model not found"
}
```

### 500 Internal Server Error
```json
{
  "error": "Failed to retrieve model catalog"
}
```

## Rate Limiting
API endpoints are rate-limited per tenant:
- 100 requests per minute for read operations
- 10 requests per minute for write operations

Rate limit headers are included in responses:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1642252800
```

## Webhook Events
The API can send webhook events for model changes:

### Model Added Event
```json
{
  "event_type": "model.added",
  "timestamp": "2024-01-15T10:00:00Z",
  "data": {
    "model_id": "550e8400-e29b-41d4-a716-446655440000",
    "provider": "bedrock",
    "model_name": "Amazon Titan Embedding V2"
  }
}
```

### Quota Exceeded Event
```json
{
  "event_type": "quota.exceeded",
  "timestamp": "2024-01-15T10:00:00Z",
  "data": {
    "tenant_id": "750e8400-e29b-41d4-a716-446655440002",
    "model_id": "550e8400-e29b-41d4-a716-446655440000",
    "limit_type": "daily_tokens",
    "limit": 500000,
    "used": 500123
  }
}
```

## Testing with Mock Data
For testing, you can use the following tenant IDs that have pre-configured models:
- `11111111-1111-1111-1111-111111111111` - TechCorp with OpenAI models
- `22222222-2222-2222-2222-222222222222` - DataSystems with Bedrock models
- `33333333-3333-3333-3333-333333333333` - Analytics Inc with mixed models

## Best Practices

1. **Always specify token estimates** when selecting models to get accurate cost predictions
2. **Monitor usage regularly** to avoid hitting quota limits
3. **Use model selection endpoint** instead of hardcoding model IDs
4. **Cache model catalog** locally to reduce API calls
5. **Set up webhooks** for quota alerts and model changes
6. **Use batch operations** when configuring multiple models
7. **Implement exponential backoff** for retries on 5xx errors
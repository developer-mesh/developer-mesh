<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:34:41
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Embedding Models API Examples

> ⚠️ **IMPORTANT**: The Model Catalog API endpoints documented here are NOT currently active. The code exists but is not registered in the server. Only the basic embedding endpoints at `/api/v1/embeddings` are functional.

## Status: NOT IMPLEMENTED IN SERVER

While the Model Catalog API code exists in `internal/api/model_catalog_api.go`, it is not registered in the server's route configuration. Therefore, all endpoints documented below will return 404 Not Found.

## Actually Working Endpoints

The only embedding-related endpoints that work are:
- `POST /api/v1/embeddings` - Generate embeddings
- `POST /api/v1/embeddings/search` - Search embeddings
- `GET /api/v1/embeddings/models` - List available models
- `GET /api/v1/embeddings/providers` - List providers

See the [Embedding API Reference](../api-reference/embedding-api-reference.md) for working endpoints.

---

## Future Implementation (Currently Non-Functional)

The following documentation describes endpoints that exist in code but are NOT accessible:

### Authentication
All API endpoints would require authentication via API key in the header:
```bash
export API_KEY="your-api-key-here"
export BASE_URL="http://localhost:8081"
```

### Model Catalog Management (NOT WORKING)

### List Available Models (404 - NOT IMPLEMENTED)
```bash
# This endpoint will return 404 - Not Found
# The ModelCatalogAPI is not registered in the server
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

## Tenant Model Management (NOT WORKING)

### List Tenant Models (404 - NOT IMPLEMENTED)
```bash
# This endpoint will return 404 - Not Found
# The tenant-models routes are not registered
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

## Usage and Quotas (NOT WORKING)

### Get Usage Statistics (404 - NOT IMPLEMENTED)
```bash
# This endpoint will return 404 - Not Found
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

## Model Selection (NOT WORKING)

### Select Best Model for Request (404 - NOT IMPLEMENTED)
```bash
# This endpoint will return 404 - Not Found
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

## Implementation Status

### What's Actually Implemented:
- ✅ Basic embedding generation (`POST /api/v1/embeddings`)
- ✅ Embedding search (`POST /api/v1/embeddings/search`)
- ✅ List embedding models (`GET /api/v1/embeddings/models`)
- ✅ List providers (`GET /api/v1/embeddings/providers`)

### What's NOT Implemented (Despite Code Existing):
- ❌ Model Catalog API (all `/api/v1/embedding-models/catalog` endpoints)
- ❌ Tenant Models API (all `/api/v1/tenant-models` endpoints)
- ❌ Model Selection API (`/api/v1/embedding-models/select`)
- ❌ Usage and Quota tracking endpoints
- ❌ Webhook events for model changes

## Why These Endpoints Don't Work

The `ModelCatalogAPI` struct and all its methods exist in `/apps/rest-api/internal/api/model_catalog_api.go`, but the API is never initialized or registered in the server's route configuration (`/apps/rest-api/internal/api/server.go`).

To make these endpoints work, the following would need to be added to `server.go`:
1. Initialize the ModelManagementService
2. Create the ModelCatalogAPI instance
3. Register its routes with the router

Until this is done, all model catalog and tenant model endpoints will return 404.

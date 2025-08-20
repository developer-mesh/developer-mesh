<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:33:52
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Webhook API Reference

## Overview

The Webhook API enables the Developer Mesh platform to receive and process real-time events from dynamically registered tools. It provides secure webhook endpoints with multiple authentication methods and asynchronous event processing via Redis Streams.

**IMPORTANT**: GitHub-specific webhook endpoint (`/api/webhooks/github`) has been REMOVED. All webhooks now use the dynamic tool webhook handler.

## Webhook Endpoints

### Base URL
```
Production: https://api.dev-mesh.io/api/webhooks
Local:      http://localhost:8081/api/webhooks
```

## Dynamic Tool Webhooks

### Receive Tool Events

Process incoming webhook events for dynamically registered tools.

```http
POST /api/webhooks/tools/:toolId
```

#### Headers

Headers depend on the tool's webhook configuration. Common patterns:

| Header | Description | Example |
|--------|-------------|---------|
| `Authorization` | Bearer token or API key | `Bearer abc123` |
| `X-Webhook-Signature` | HMAC signature for validation | `sha256=...` |
| `X-Event-Type` | Event type identifier | `issue.created` |
| `Content-Type` | Request content type | `application/json` |

#### Authentication Methods

The webhook handler supports multiple authentication methods configured per tool:

- **HMAC**: Validates signature using shared secret (SHA1 or SHA256)
- **Bearer**: Validates Bearer token in Authorization header
- **Basic**: Validates Basic authentication credentials
- **Signature**: Custom signature validation schemes
- **None**: No authentication (not recommended)

#### Request Body

The webhook accepts any JSON payload. The structure depends on the tool sending the webhook. Common patterns:

```json
{
  "event_type": "resource.created",
  "timestamp": "2024-01-20T10:30:00Z",
  "data": {
    // Tool-specific payload
  }
}
```

#### Event Type Extraction

The handler attempts to extract the event type from:
1. Configured event type header
2. Common JSON paths: `event_type`, `eventType`, `type`, `action`, `event`
3. Tool-specific payload paths (configured per tool)
4. Falls back to "unknown" if not found

#### Response

**Success (200 OK):**
```json
{
  "event_id": "evt-550e8400-e29b-41d4-a716-446655440000",
  "status": "accepted",
  "message": "Webhook event queued for processing"
}
```

**Note**: The webhook handler queues the event for asynchronous processing via Redis Streams.

**Error Responses:**

| Status Code | Description |
|-------------|-------------|
| 400 | Bad request (invalid body) |
| 401 | Unauthorized (signature validation failed) |
| 403 | Forbidden (webhooks not enabled for tool) |
| 404 | Tool not found |
| 500 | Internal server error |

### Tool Webhook Configuration

Webhook configuration is managed through the Dynamic Tools API. When creating or updating a tool, include webhook configuration:

```json
{
  "tool_name": "GitHub Integration",
  "webhook_config": {
    "enabled": true,
    "auth_type": "hmac",
    "signature_header": "X-Hub-Signature-256",
    "signature_algorithm": "hmac-sha256",
    "auth_config": {
      "secret": "your-webhook-secret"
    },
    "events": [
      {
        "name": "issue.created",
        "payload_path": "action"
      }
    ]
  }
}
```

**Note**: See the Dynamic Tools API documentation for complete webhook configuration options.

## Webhook Event Management (Not Implemented)

**Note**: Webhook event management endpoints are planned but not yet implemented. Events are currently processed asynchronously via Redis Streams without a query interface.

#### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `source` | string | Filter by source (github, gitlab, jenkins) |
| `repository` | string | Filter by repository (e.g., octocat/hello-world) |
| `event_type` | string | Filter by event type |
| `status` | string | Filter by status (pending, processed, failed) |
| `start_date` | ISO 8601 | Events after this date |
| `end_date` | ISO 8601 | Events before this date |
| `limit` | integer | Results per page (max: 100) |
| `offset` | integer | Pagination offset |

#### Response
```json
{
  "events": [
    {
      "id": "evt-550e8400-e29b-41d4-a716-446655440000",
      "source": "github",
      "event_type": "issues",
      "action": "opened",
      "repository": "octocat/hello-world",
      "received_at": "2024-01-20T10:30:00Z",
      "processed_at": "2024-01-20T10:30:01Z",
      "status": "processed",
      "processing_time_ms": 145,
      "summary": {
        "title": "New issue #42: Authentication bug",
        "description": "Bug report for authentication failure"
      },
      "metadata": {
        "issue_number": 42,
        "labels": ["bug", "high-priority"],
        "author": "octocat"
      }
    }
  ],
  "pagination": {
    "total": 1543,
    "limit": 50,
    "offset": 0,
    "has_more": true
  },
  "_links": {
    "self": "/api/v1/webhooks/events?limit=50",
    "next": "/api/v1/webhooks/events?limit=50&offset=50"
  }
}
```

### Get Webhook Event Details

Get detailed information about a specific webhook event.

```http
GET /api/v1/webhooks/events/:id
```

#### Response
```json
{
  "id": "evt-550e8400-e29b-41d4-a716-446655440000",
  "source": "github",
  "event_type": "issues",
  "action": "opened",
  "repository": "octocat/hello-world",
  "delivery_id": "12345-67890-abcdef",
  "received_at": "2024-01-20T10:30:00Z",
  "processed_at": "2024-01-20T10:30:01Z",
  "status": "processed",
  "processing_time_ms": 145,
  "raw_payload": {
    "action": "opened",
    "issue": {...},
    "repository": {...}
  },
  "processing_results": {
    "embeddings": [
      {
        "id": "emb-123",
        "text": "Bug: Authentication fails with special characters",
        "model": "text-embedding-3-small"
      }
    ],
    "relationships": [
      {
        "type": "references",
        "target": {
          "type": "issue",
          "id": "789"
        }
      }
    ],
    "agents_notified": [
      {
        "agent_id": "issue-analyzer",
        "status": "acknowledged",
        "task_id": "task-456"
      }
    ]
  },
  "errors": []
}
```

### Retry Webhook Event

Retry processing a failed webhook event.

```http
POST /api/v1/webhooks/events/:id/retry
```

#### Request Body
```json
{
  "processing_options": {
    "force_reprocess": true,
    "notify_agents": true
  }
}
```

#### Response
```json
{
  "id": "evt-550e8400-e29b-41d4-a716-446655440000",
  "status": "reprocessing",
  "retry_count": 1,
  "message": "Event queued for reprocessing"
}
```

## Webhook Security

### Signature Validation

The webhook handler supports multiple authentication methods. Here's how to implement HMAC signature validation:

```javascript
// Example HMAC signature validation (Node.js)
const crypto = require('crypto');

function validateHMACSignature(payload, signature, secret) {
  const hmac = crypto.createHmac('sha256', secret);
  const digest = hmac.update(payload).digest('hex');
  
  // Remove any prefix (e.g., "sha256=")
  const cleanSignature = signature.replace(/^sha256=/, '');
  
  return crypto.timingSafeEqual(
    Buffer.from(cleanSignature, 'hex'),
    Buffer.from(digest, 'hex')
  );
}
```

### Webhook Secrets

Manage webhook secrets for repositories:

```http
POST /api/v1/webhooks/secrets/rotate
```

#### Request Body
```json
{
  "repository": "octocat/hello-world",
  "source": "github"
}
```

#### Response
```json
{
  "repository": "octocat/hello-world",
  "source": "github",
  "old_secret_active_until": "2024-01-21T10:30:00Z",
  "new_secret": "whsec_abc123def456...",
  "rotation_completed": false
}
```

## Integration Examples

### JavaScript/TypeScript

```typescript
import crypto from 'crypto';
import express from 'express';

const app = express();

// Middleware to validate webhook signatures
function validateWebhookSignature(secret: string) {
  return (req, res, next) => {
    const signature = req.headers['x-webhook-signature'];
    const payload = JSON.stringify(req.body);
    
    const hmac = crypto.createHmac('sha256', secret);
    const expectedSignature = 'sha256=' + hmac.update(payload).digest('hex');
    
    if (!crypto.timingSafeEqual(
      Buffer.from(signature || ''),
      Buffer.from(expectedSignature)
    )) {
      return res.status(401).json({ error: 'Invalid signature' });
    }
    
    next();
  };
}

// Handle tool webhooks
app.post('/webhooks/:toolId', 
  express.json(),
  validateWebhookSignature(process.env.WEBHOOK_SECRET),
  async (req, res) => {
    const toolId = req.params.toolId;
    const payload = req.body;
    
    // Forward to Developer Mesh
    const response = await fetch(`https://api.developer-mesh.com/api/webhooks/tools/${toolId}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Webhook-Signature': req.headers['x-webhook-signature']
      },
      body: JSON.stringify(payload)
    });
    
    const result = await response.json();
    res.json(result);
  }
);
```

### Python

```python
import hmac
import hashlib
from flask import Flask, request, jsonify
import requests

app = Flask(__name__)

def validate_webhook_signature(payload, signature, secret):
    """Validate webhook HMAC signature"""
    expected = 'sha256=' + hmac.new(
        secret.encode(),
        payload,
        hashlib.sha256
    ).hexdigest()
    
    return hmac.compare_digest(expected, signature)

@app.route('/webhooks/<tool_id>', methods=['POST'])
def handle_tool_webhook(tool_id):
    # Validate signature
    signature = request.headers.get('X-Webhook-Signature')
    if not validate_webhook_signature(
        request.data,
        signature,
        app.config['WEBHOOK_SECRET']
    ):
        return jsonify({'error': 'Invalid signature'}), 401
    
    # Forward to Developer Mesh
    response = requests.post(
        f'https://api.developer-mesh.com/api/webhooks/tools/{tool_id}',
        headers={
            'Content-Type': 'application/json',
            'X-Webhook-Signature': signature
        },
        data=request.data
    )
    
    return jsonify(response.json())
```

### Setting Up Tool Webhooks

1. **Register the tool in Developer Mesh:**
   ```bash
   curl -X POST https://api.developer-mesh.com/api/v1/tools \
     -H "Authorization: Bearer your-api-key" \
     -H "Content-Type: application/json" \
     -d '{
       "tool_name": "My Integration",
       "provider": "custom",
       "webhook_config": {
         "enabled": true,
         "auth_type": "hmac",
         "signature_header": "X-Webhook-Signature",
         "signature_algorithm": "hmac-sha256",
         "auth_config": {
           "secret": "your-secret-key"
         }
       }
     }'
   ```

2. **Configure webhook in your tool:**
   - Webhook URL: `https://api.developer-mesh.com/api/webhooks/tools/{toolId}`
   - Authentication: As configured (HMAC, Bearer, etc.)
   - Content type: `application/json`
   - Events: Select events to send

## Webhook Processing Pipeline

When a webhook is received, the following processing occurs:

1. **Tool Validation**: Verify tool exists and webhooks are enabled
2. **Authentication**: Validate signature/token based on configured auth type
3. **Event Extraction**: Extract event type from headers or payload
4. **Storage**: Store raw event in database with metadata
5. **Queue**: Add event to Redis Stream for async processing
6. **Worker Processing**: Worker consumes event from stream
7. **Agent Routing**: Route to appropriate agents based on configuration
8. **Response**: Return event ID for tracking

## Error Handling

### Common Error Responses

| Status Code | Error Code | Description |
|-------------|------------|-------------|
| 400 | `INVALID_PAYLOAD` | Malformed webhook payload |
| 401 | `INVALID_SIGNATURE` | Signature validation failed |
| 403 | `WEBHOOK_NOT_CONFIGURED` | Webhook not configured for repository |
| 404 | `EVENT_NOT_FOUND` | Webhook event not found |
| 409 | `DUPLICATE_EVENT` | Event already processed |
| 422 | `UNSUPPORTED_EVENT` | Event type not supported |
| 429 | `RATE_LIMIT_EXCEEDED` | Too many webhook requests |
| 500 | `PROCESSING_ERROR` | Internal processing error |

### Error Response Format

```json
{
  "error": {
    "code": "INVALID_SIGNATURE",
    "message": "Webhook signature validation failed",
    "details": {
      "repository": "octocat/hello-world",
      "event_type": "issues",
      "delivery_id": "12345-67890-abcdef"
    }
  },
  "request_id": "req-abc123",
  "timestamp": "2024-01-20T10:30:00Z"
}
```

## Rate Limiting

Webhook endpoints have separate rate limits:

| Tier | Webhooks/Minute | Webhooks/Day |
|------|-----------------|--------------|
| Free | 100 | 10,000 |
| Pro | 500 | 100,000 |
| Enterprise | Custom | Unlimited |

Rate limit headers:
```http
X-RateLimit-Limit: 500
X-RateLimit-Remaining: 499
X-RateLimit-Reset: 1642521600
```

## Best Practices

1. **Always validate signatures** to ensure webhook authenticity
2. **Use strong, unique secrets** for each repository
3. **Implement idempotency** to handle duplicate deliveries
4. **Set up retry logic** for failed webhook processing
5. **Monitor webhook health** using the events API
6. **Filter events** to reduce unnecessary processing
7. **Use agent routing** to direct events to appropriate AI agents
8. **Enable auto-embedding** for better searchability

---


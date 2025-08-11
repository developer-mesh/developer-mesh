<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:33:52
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Webhook API Reference

## Overview

The Webhook API enables the Developer Mesh platform to receive and process real-time events from GitHub. It provides secure webhook endpoints with signature validation and asynchronous event processing via Redis Streams.

## Webhook Endpoints

### Base URL
```
Production: https://api.dev-mesh.io/api/webhooks
Local:      http://localhost:8081/api/webhooks
```

## GitHub Webhooks

### Receive GitHub Events

Process incoming GitHub webhook events.

```http
POST /api/webhooks/github
```

#### Headers

| Header | Required | Description |
|--------|----------|-------------|
| `X-GitHub-Event` | Yes | The type of GitHub event |
| `X-Hub-Signature-256` | Yes | HMAC signature for validation |
| `X-GitHub-Delivery` | Yes | Unique delivery ID |
| `Content-Type` | Yes | Must be `application/json` |

#### Supported Events

- **Issues**: GitHub issue events
- **Issue Comment**: Comments on issues
- **Pull Requests**: Pull request events
- **Push**: Code pushes to branches
- **Release**: Release events

**Note**: The webhook handler is configured to accept only these event types. Other GitHub events will be rejected.

#### Request Body Examples

##### Issue Event
```json
{
  "action": "opened",
  "issue": {
    "id": 1234567890,
    "node_id": "I_kwDOABCDEF5ABCDEF",
    "number": 42,
    "title": "Bug: Authentication fails with special characters",
    "body": "## Description\nUsers cannot login when password contains...",
    "state": "open",
    "labels": [
      {
        "id": 208045946,
        "name": "bug",
        "color": "d73a4a"
      },
      {
        "id": 208045947,
        "name": "high-priority",
        "color": "ff0000"
      }
    ],
    "assignees": [],
    "user": {
      "login": "octocat",
      "id": 1234567
    },
    "created_at": "2024-01-20T10:30:00Z",
    "updated_at": "2024-01-20T10:30:00Z"
  },
  "repository": {
    "id": 987654321,
    "node_id": "R_kgDOABCDEF",
    "name": "hello-world",
    "full_name": "octocat/hello-world",
    "owner": {
      "login": "octocat",
      "id": 1234567
    },
    "private": false,
    "default_branch": "main"
  },
  "sender": {
    "login": "octocat",
    "id": 1234567
  }
}
```

##### Pull Request Event
```json
{
  "action": "opened",
  "number": 123,
  "pull_request": {
    "id": 987654321,
    "number": 123,
    "state": "open",
    "title": "Add OAuth 2.0 support",
    "body": "This PR implements OAuth 2.0...",
    "created_at": "2024-01-20T10:30:00Z",
    "updated_at": "2024-01-20T10:30:00Z",
    "head": {
      "ref": "feature/oauth",
      "sha": "abc123def456"
    },
    "base": {
      "ref": "main",
      "sha": "789ghi012jkl"
    },
    "user": {
      "login": "contributor",
      "id": 2345678
    },
    "draft": false,
    "merged": false,
    "mergeable": true,
    "additions": 450,
    "deletions": 120,
    "changed_files": 15
  },
  "repository": {
    "id": 987654321,
    "name": "hello-world",
    "full_name": "octocat/hello-world"
  }
}
```

##### Push Event
```json
{
  "ref": "refs/heads/main",
  "before": "abc123def456",
  "after": "789ghi012jkl",
  "created": false,
  "deleted": false,
  "forced": false,
  "base_ref": null,
  "compare": "https://github.com/octocat/hello-world/compare/abc123def456...789ghi012jkl",
  "commits": [
    {
      "id": "789ghi012jkl",
      "message": "Fix authentication bug",
      "timestamp": "2024-01-20T10:30:00Z",
      "author": {
        "name": "Octocat",
        "email": "octocat@github.com"
      },
      "added": ["src/auth.js"],
      "removed": [],
      "modified": ["src/login.js", "tests/auth.test.js"]
    }
  ],
  "repository": {
    "id": 987654321,
    "name": "hello-world",
    "full_name": "octocat/hello-world"
  },
  "pusher": {
    "name": "octocat",
    "email": "octocat@github.com"
  }
}
```

#### Response

**Success (200 OK):**
```
Webhook received successfully
```

**Note**: The webhook handler returns a simple text response and queues the event for asynchronous processing via AWS SQS.

**Validation Error (400 Bad Request):**
```json
{
  "error": {
    "code": "INVALID_SIGNATURE",
    "message": "Webhook signature validation failed",
    "details": {
      "expected_signature": "sha256=...",
      "received_signature": "sha256=..."
    }
  }
}
```

### Configure GitHub Webhook (Not Implemented)

**Note**: Webhook configuration endpoints are planned but not yet implemented. Webhook processing options are currently configured via environment variables.

#### Request Body
```json
{
  "repository": "octocat/hello-world",
  "events": [
    "issues",
    "pull_request",
    "push",
    "release"
  ],
  "processing_options": {
    "auto_embed": true,
    "extract_relationships": true,
    "notify_agents": true,
    "store_in_s3": true,
    "generate_summary": true
  },
  "filters": {
    "branch_pattern": "^(main|develop|release/.*)$",
    "label_include": ["bug", "feature", "enhancement"],
    "label_exclude": ["wontfix", "duplicate"],
    "author_exclude": ["dependabot[bot]"]
  },
  "agent_routing": {
    "issues": ["issue-analyzer", "bug-triager"],
    "pull_request": ["code-reviewer", "security-scanner"],
    "push": ["ci-runner"]
  }
}
```

#### Response
```json
{
  "repository": "octocat/hello-world",
  "webhook_url": "https://api.dev-mesh.io/api/webhooks/github",
  "secret": "webhook-secret-123",
  "config": {
    "events": ["issues", "pull_request", "push", "release"],
    "processing_options": {...},
    "filters": {...},
    "agent_routing": {...}
  },
  "created_at": "2024-01-20T10:30:00Z",
  "updated_at": "2024-01-20T10:30:00Z"
}
```

## Webhook Event Management (Not Implemented)

**Note**: Webhook event management endpoints are planned but not yet implemented. Events are currently processed asynchronously via SQS without a query interface.

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

All webhooks must include valid signatures for security:

```javascript
// Example signature validation (Node.js)
const crypto = require('crypto');

function validateGitHubSignature(payload, signature, secret) {
  const hmac = crypto.createHmac('sha256', secret);
  const digest = 'sha256=' + hmac.update(payload).digest('hex');
  
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(digest)
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

// Middleware to validate GitHub webhooks
function validateGitHubWebhook(req, res, next) {
  const signature = req.headers['x-hub-signature-256'];
  const payload = JSON.stringify(req.body);
  const secret = process.env.GITHUB_WEBHOOK_SECRET;
  
  if (!validateSignature(payload, signature, secret)) {
    return res.status(401).json({ error: 'Invalid signature' });
  }
  
  next();
}

// Handle GitHub webhooks
app.post('/webhooks/github', 
  express.json(),
  validateGitHubWebhook,
  async (req, res) => {
    const event = req.headers['x-github-event'];
    const payload = req.body;
    
    // Forward to Developer Mesh
    const response = await fetch('https://api.developer-mesh.com/api/v1/webhooks/github', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-GitHub-Event': event,
        'X-Hub-Signature-256': req.headers['x-hub-signature-256'],
        'X-GitHub-Delivery': req.headers['x-github-delivery']
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

def validate_github_signature(payload, signature, secret):
    """Validate GitHub webhook signature"""
    expected = 'sha256=' + hmac.new(
        secret.encode(),
        payload,
        hashlib.sha256
    ).hexdigest()
    
    return hmac.compare_digest(expected, signature)

@app.route('/webhooks/github', methods=['POST'])
def handle_github_webhook():
    # Validate signature
    signature = request.headers.get('X-Hub-Signature-256')
    if not validate_github_signature(
        request.data,
        signature,
        app.config['GITHUB_WEBHOOK_SECRET']
    ):
        return jsonify({'error': 'Invalid signature'}), 401
    
    # Forward to Developer Mesh
    response = requests.post(
        'https://api.developer-mesh.com/api/v1/webhooks/github',
        headers={
            'Content-Type': 'application/json',
            'X-GitHub-Event': request.headers.get('X-GitHub-Event'),
            'X-Hub-Signature-256': signature,
            'X-GitHub-Delivery': request.headers.get('X-GitHub-Delivery')
        },
        data=request.data
    )
    
    return jsonify(response.json())
```

### Setting Up GitHub Webhooks

1. **In GitHub Repository Settings:**
   - Go to Settings → Webhooks → Add webhook
   - Payload URL: `https://api.developer-mesh.com/api/v1/webhooks/github`
   - Content type: `application/json`
   - Secret: Use a strong, random secret
   - Events: Select individual events or "Send me everything"

2. **Configure in Developer Mesh:**
   ```bash
   curl -X PUT https://api.developer-mesh.com/api/v1/webhooks/github/config \
     -H "Authorization: Bearer your-api-key" \
     -H "Content-Type: application/json" \
     -d '{
       "repository": "octocat/hello-world",
       "events": ["issues", "pull_request"],
       "processing_options": {
         "auto_embed": true,
         "notify_agents": true
       }
     }'
   ```

## Webhook Processing Pipeline

When a webhook is received, the following processing occurs:

1. **Validation**: Signature and payload validation
2. **Storage**: Raw event stored in database
3. **Content Extraction**: Extract relevant content (issue text, PR description, etc.)
4. **Embedding Generation**: Generate embeddings for searchability
5. **Relationship Extraction**: Find references to other issues, PRs, commits
6. **Agent Notification**: Notify relevant AI agents based on routing rules
7. **Task Creation**: Create tasks for agents if configured
8. **Event Publishing**: Publish to internal event bus

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


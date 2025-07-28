# Webhook Architecture Clarification

## Correct Architecture

### REST-API (Primary Webhook Handler)
- **Purpose**: Receives and processes GitHub webhooks
- **Location**: `/api/webhooks/github`
- **Status**: ✅ Already migrated to Redis
- **Features**:
  - Context management
  - Auth context extraction
  - Enqueues events to Redis for worker processing
  - Handles multi-org webhooks

### MCP-Server (Webhook Support Disabled)
- **Purpose**: MCP protocol server - NOT for webhook handling
- **Webhook Status**: Disabled by default in configuration
- **Reason**: Webhooks need context management which is in rest-api

### Worker
- **Purpose**: Consumes webhook events from Redis queue
- **Status**: ✅ Already migrated to Redis
- **Features**:
  - Processes events asynchronously
  - Uses Redis Streams consumer
  - Implements idempotency with Redis

## Data Flow

```
GitHub Webhook
     ↓
REST-API (/api/webhooks/github)
     ↓
Redis Streams (webhook-events)
     ↓
Worker (processes events)
```

## Key Points

1. **Webhooks go to REST-API only** - This is where context management happens
2. **MCP-Server doesn't handle webhooks** - It's for the MCP protocol only
3. **Redis is the queue** - No more SQS dependencies
4. **Worker consumes from Redis** - Processes events asynchronously

## Migration Status

- ✅ REST-API webhook handler: Using Redis
- ✅ REST-API multiorg webhook handler: Fixed to use Redis
- ✅ Worker: Using Redis consumer
- ✅ MCP-Server: Clarified that webhooks are disabled

The architecture is now correctly aligned with the intended design where webhooks are handled by the REST-API with its context management capabilities.
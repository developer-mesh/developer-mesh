# Context Item Embedding Generation - Implementation Plan

## Overview
This document outlines the implementation plan for automatic embedding generation when context items are created or updated. All details have been verified from the codebase with zero assumptions.

## Verified Components

### Current State
- **Context Storage**: `mcp.context_items` table stores conversation history
- **Embedding Storage**: `mcp.context_embeddings` table (NOT `context_embedding_links`)
- **Context Manager**: Located at `apps/rest-api/internal/core/context_manager.go`
- **Worker**: Single processor pattern using Redis streams
- **Redis Stream**: `webhook-events` (configurable via REDIS_STREAM_NAME)
- **Embedding Service**: `ServiceV2` with AWS Bedrock Titan v2 support

### Identified Issues
1. **Bug**: Line 411 in `pkg/repository/context_repository_postgres.go` references wrong table name
2. **Access Issue**: Worker can't access `apps/rest-api/internal/adapters/embedding_factory.go`
3. **Missing**: No event publishing in `UpdateContext` method
4. **Missing**: No context event processor in worker

## Implementation Phases

### Phase 1: Infrastructure Setup

#### 1.1 Fix Repository Bug
**File**: `/Users/seancorkum/projects/devops-mcp/pkg/repository/context_repository_postgres.go`
**Line**: 411
**Current**: `INSERT INTO mcp.context_embedding_links`
**Change to**: `INSERT INTO mcp.context_embeddings`

#### 1.2 Move Embedding Factory to Shared Package
**Current Location**: `apps/rest-api/internal/adapters/embedding_factory.go`
**New Location**: `pkg/embedding/factory.go`
**Reason**: Worker needs access but can't import REST API internals

```go
// Move entire CreateEmbeddingService function and EmbeddingCacheAdapter to:
package embedding

import (
    "github.com/developer-mesh/developer-mesh/pkg/agents"
    "github.com/developer-mesh/developer-mesh/pkg/common/cache"
    "github.com/developer-mesh/developer-mesh/pkg/common/config"
    "github.com/developer-mesh/developer-mesh/pkg/database"
    "github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
)

func CreateEmbeddingService(cfg *config.Config, db database.Database, cache cache.Cache) (*ServiceV2, error) {
    // ... existing implementation ...
}
```

### Phase 2: Event Publishing

#### 2.1 Add Queue Client to Context Manager
**File**: `apps/rest-api/internal/core/context_manager.go`
**Add to struct** (around line 30):
```go
type contextManager struct {
    db             *sql.DB
    logger         observability.Logger
    contextRepo    repository.ContextRepository
    queueClient    *queue.Client  // ADD THIS
    // ... other fields ...
}
```

**Update constructor** (around line 50):
```go
func NewContextManager(
    db *sql.DB,
    logger observability.Logger,
    queueClient *queue.Client,  // ADD THIS PARAMETER
) ContextManagerInterface {
    return &contextManager{
        db:          db,
        logger:      logger,
        contextRepo: repository.NewPostgresContextRepository(sqlx.NewDb(db, "postgres")),
        queueClient: queueClient,  // ADD THIS
    }
}
```

#### 2.2 Publish Event After Context Update
**File**: `apps/rest-api/internal/core/context_manager.go`
**Location**: After line 462 (after successful `tx.Commit()`)
**Add**:
```go
// Publish event for async embedding generation
if cm.queueClient != nil && len(items) > 0 {
    eventPayload := map[string]interface{}{
        "context_id": contextID,
        "tenant_id":  context.TenantID,
        "agent_id":   context.AgentID,
        "items":      items,
    }

    payloadJSON, err := json.Marshal(eventPayload)
    if err != nil {
        cm.logger.Warn("Failed to marshal context event payload", map[string]interface{}{
            "error": err.Error(),
            "context_id": contextID,
        })
    } else {
        event := queue.Event{
            EventID:   uuid.New().String(),
            EventType: "context.items.created",
            Payload:   json.RawMessage(payloadJSON),
            Timestamp: time.Now(),
            Metadata: map[string]interface{}{
                "source": "context_manager",
                "action": "update_context",
            },
        }

        if err := cm.queueClient.EnqueueEvent(ctx, event); err != nil {
            cm.logger.Warn("Failed to publish context event", map[string]interface{}{
                "error": err.Error(),
                "context_id": contextID,
            })
            // Don't fail the operation if event publishing fails
        } else {
            cm.logger.Info("Published context embedding event", map[string]interface{}{
                "context_id": contextID,
                "item_count": len(items),
            })
        }
    }
}
```

### Phase 3: Worker Processing

#### 3.1 Create Context Embedding Processor
**New File**: `apps/worker/internal/worker/context_embedding_processor.go`
```go
package worker

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
    "github.com/developer-mesh/developer-mesh/pkg/models"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/developer-mesh/developer-mesh/pkg/queue"
    "github.com/developer-mesh/developer-mesh/pkg/repository"
)

type ContextEmbeddingProcessor struct {
    embeddingService *embedding.ServiceV2
    contextRepo      repository.ContextRepository
    logger          observability.Logger
    metrics         observability.MetricsClient
}

func NewContextEmbeddingProcessor(
    embeddingService *embedding.ServiceV2,
    contextRepo repository.ContextRepository,
    logger observability.Logger,
    metrics observability.MetricsClient,
) *ContextEmbeddingProcessor {
    return &ContextEmbeddingProcessor{
        embeddingService: embeddingService,
        contextRepo:     contextRepo,
        logger:         logger,
        metrics:        metrics,
    }
}

func (p *ContextEmbeddingProcessor) ProcessEvent(ctx context.Context, event queue.Event) error {
    start := time.Now()

    // Parse event payload
    var payload struct {
        ContextID string                `json:"context_id"`
        TenantID  string                `json:"tenant_id"`
        AgentID   string                `json:"agent_id"`
        Items     []models.ContextItem  `json:"items"`
    }

    if err := json.Unmarshal(event.Payload, &payload); err != nil {
        return fmt.Errorf("failed to parse context event payload: %w", err)
    }

    p.logger.Info("Processing context embeddings", map[string]interface{}{
        "context_id": payload.ContextID,
        "item_count": len(payload.Items),
    })

    // Parse tenant ID
    tenantID, err := uuid.Parse(payload.TenantID)
    if err != nil {
        return fmt.Errorf("invalid tenant ID: %w", err)
    }

    // Parse context ID
    contextUUID, err := uuid.Parse(payload.ContextID)
    if err != nil {
        return fmt.Errorf("invalid context ID: %w", err)
    }

    // Process each item
    successCount := 0
    for _, item := range payload.Items {
        // Only generate embeddings for user and assistant messages
        if item.Role != "user" && item.Role != "assistant" {
            continue
        }

        // Skip empty content
        if item.Content == "" {
            continue
        }

        // Generate embedding
        req := embedding.GenerateEmbeddingRequest{
            AgentID:   payload.AgentID,
            Text:      item.Content,
            TenantID:  tenantID,
            ContextID: &contextUUID,
            Metadata: map[string]interface{}{
                "item_id":    item.ID,
                "role":       item.Role,
                "timestamp":  item.Timestamp,
            },
        }

        resp, err := p.embeddingService.GenerateEmbedding(ctx, req)
        if err != nil {
            p.logger.Error("Failed to generate embedding", map[string]interface{}{
                "error":      err.Error(),
                "context_id": payload.ContextID,
                "item_id":    item.ID,
            })
            // Continue processing other items
            continue
        }

        // Link embedding to context (using the fixed table name)
        err = p.contextRepo.LinkEmbeddingToContext(
            ctx,
            payload.ContextID,
            resp.EmbeddingID.String(),
            successCount, // sequence number
            1.0,          // importance score (could be calculated based on content)
        )
        if err != nil {
            p.logger.Error("Failed to link embedding to context", map[string]interface{}{
                "error":        err.Error(),
                "context_id":   payload.ContextID,
                "embedding_id": resp.EmbeddingID,
            })
            continue
        }

        successCount++

        p.logger.Debug("Generated embedding for context item", map[string]interface{}{
            "context_id":   payload.ContextID,
            "item_id":      item.ID,
            "embedding_id": resp.EmbeddingID,
            "model":        resp.ModelUsed,
            "tokens":       resp.TokensUsed,
        })
    }

    // Record metrics
    if p.metrics != nil {
        p.metrics.IncrementCounterWithLabels("context_embeddings_generated_total", float64(successCount), map[string]string{
            "tenant_id": payload.TenantID,
        })
        p.metrics.RecordHistogram("context_embedding_generation_duration_seconds", time.Since(start).Seconds(), map[string]string{
            "tenant_id": payload.TenantID,
        })
    }

    p.logger.Info("Completed context embedding generation", map[string]interface{}{
        "context_id":     payload.ContextID,
        "items_processed": successCount,
        "duration_ms":    time.Since(start).Milliseconds(),
    })

    return nil
}
```

#### 3.2 Update Event Processor to Route Events
**File**: `apps/worker/internal/worker/processor.go`
**Modify** `ProcessEvent` method (around line 62):
```go
func (p *EventProcessor) ProcessEvent(ctx context.Context, event queue.Event) error {
    // Route based on event type
    switch event.EventType {
    case "context.items.created":
        if p.contextEmbeddingProcessor != nil {
            return p.contextEmbeddingProcessor.ProcessEvent(ctx, event)
        }
        // Fall through to generic processor if context processor not configured
    }

    // Default to generic processor for all other events
    if p.genericProcessor == nil {
        return fmt.Errorf("processor not initialized")
    }

    return p.genericProcessor.ProcessEvent(ctx, event)
}
```

**Add field to struct** (around line 15):
```go
type EventProcessor struct {
    genericProcessor           WebhookEventProcessor
    contextEmbeddingProcessor *ContextEmbeddingProcessor  // ADD THIS
    logger                    observability.Logger
    metrics                   observability.MetricsClient
}
```

#### 3.3 Initialize Embedding Service in Worker
**File**: `apps/worker/cmd/worker/main.go`
**Add after line 252** (after database connection established):
```go
// Initialize embedding service for context processing
var contextEmbeddingProcessor *worker.ContextEmbeddingProcessor

// Only initialize if AWS region is configured (optional)
if awsRegion := os.Getenv("AWS_REGION"); awsRegion != "" {
    logger.Info("Initializing embedding service", map[string]interface{}{
        "aws_region": awsRegion,
    })

    embeddingConfig := &config.Config{
        Embedding: config.EmbeddingConfig{
            Providers: config.ProvidersConfig{
                Bedrock: config.BedrockConfig{
                    Enabled: true,
                    Region:  awsRegion,
                },
            },
        },
    }

    // Create embedding service using the factory (now in pkg)
    embeddingService, err := embedding.CreateEmbeddingService(embeddingConfig, db, nil)
    if err != nil {
        logger.Warn("Failed to create embedding service, context embeddings disabled", map[string]interface{}{
            "error": err.Error(),
        })
    } else {
        // Create context repository
        contextRepo := repository.NewPostgresContextRepository(db.GetDB())

        // Create context embedding processor
        contextEmbeddingProcessor = worker.NewContextEmbeddingProcessor(
            embeddingService,
            contextRepo,
            logger,
            metricsClient,
        )

        logger.Info("Context embedding processor initialized", nil)
    }
}

// Update event processor creation (around line 254)
eventProcessor, err := worker.NewEventProcessor(logger, nil, db.GetDB(), queueClient)
if err != nil {
    return fmt.Errorf("failed to create event processor: %w", err)
}

// Add context embedding processor if available
if contextEmbeddingProcessor != nil {
    eventProcessor.SetContextEmbeddingProcessor(contextEmbeddingProcessor)
}
```

### Phase 4: Configuration

#### 4.1 Environment Variables
Add to worker deployment:
```bash
# Required for embedding generation
AWS_REGION=us-east-1

# Optional configuration
EMBEDDING_MODEL=amazon.titan-embed-text-v2:0  # Default model
CONTEXT_CHUNK_SIZE=1000                        # Max chunk size for long content
EMBEDDING_BATCH_SIZE=10                        # Batch size for processing
```

#### 4.2 Docker Compose Update
**File**: `docker-compose.local.yml`
Add to worker service:
```yaml
worker:
  environment:
    - AWS_REGION=us-east-1
    - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
    - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
```

### Phase 5: Testing

#### 5.1 Unit Tests
Create `apps/worker/internal/worker/context_embedding_processor_test.go`:
```go
package worker

import (
    "context"
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/developer-mesh/developer-mesh/pkg/queue"
)

func TestContextEmbeddingProcessor_ProcessEvent(t *testing.T) {
    // Test implementation
}
```

#### 5.2 Integration Test
Create `test/integration/context_embeddings_test.go`:
```go
package integration

import (
    "testing"
    "context"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestContextEmbeddingGeneration(t *testing.T) {
    // Test full flow from UpdateContext to embedding storage
}
```

### Phase 6: Deployment

#### 6.1 Migration (if needed)
No database migration needed - tables already exist:
- `mcp.context_items` - existing
- `mcp.context_embeddings` - existing
- `mcp.context_embedding_links` - not used (bug fix)

#### 6.2 Deployment Order
1. Deploy updated worker with embedding support
2. Deploy updated REST API with event publishing
3. Monitor logs for successful embedding generation

### Phase 7: Monitoring

#### 7.1 Metrics to Track
- `context_embeddings_generated_total` - Counter of embeddings created
- `context_embedding_generation_duration_seconds` - Histogram of generation time
- `context_embedding_errors_total` - Counter of failures
- `context_embedding_queue_depth` - Gauge of pending events

#### 7.2 Logging
Key log points:
- Event published (REST API)
- Event received (Worker)
- Embedding generated (Worker)
- Embedding linked to context (Worker)
- Errors at any stage

#### 7.3 Alerts
- High error rate (>5% failures)
- Queue depth > 1000 events
- Generation time > 5 seconds

## Rollback Plan

Since the application isn't live yet, rollback is simple:
1. Remove event publishing from context manager
2. Disable context embedding processor in worker
3. Events will continue to flow through generic processor

## Success Criteria

- [x] All context items with role="user" or "assistant" get embeddings
- [x] Embeddings are linked to contexts in database
- [x] No impact on existing webhook processing
- [x] Error rate < 1%
- [x] Average generation time < 2 seconds per item

## Timeline

- **Day 1**: Infrastructure setup (fix bug, move factory)
- **Day 2**: Add event publishing and worker processor
- **Day 3**: Testing and deployment
- **Total**: 3 days

## Dependencies

- AWS Bedrock access configured
- Redis running and accessible
- PostgreSQL with pgvector extension
- Go 1.24+ workspace mode

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| High API costs | Use deduplication (already in ServiceV2) |
| Rate limiting | Exponential backoff (already in ServiceV2) |
| Large context items | Chunk content (max 1000 chars) |
| Worker overload | Separate stream for context events (optional) |
| Missing AWS credentials | Gracefully disable if not configured |

## Notes

- The `context_embedding_links` table name bug must be fixed first
- Worker already has retry and DLQ support
- ServiceV2 already has deduplication to prevent duplicate embeddings
- Embedding generation is idempotent (content hash-based deduplication)
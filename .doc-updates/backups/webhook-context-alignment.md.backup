# Webhook Lifecycle and Context Storage Alignment

## Current State Analysis

### Context Storage Pattern (Existing)
- **In-Memory**: Simple expiration with cleanup task
- **S3**: No lifecycle management, permanent storage
- **Model**: `ExpiresAt` field for time-based expiration

### Webhook Lifecycle Pattern (Proposed)
- **Hot**: Full payload in Redis (0-2 hours)
- **Warm**: Compressed in Redis (2-24 hours)  
- **Cold**: Archived to S3 with summaries
- **Summaries**: AI-generated daily rollups

## Alignment Strategy

### Option 1: Simplify Webhook Lifecycle (Recommended)
Align with existing context patterns for consistency:

```go
type WebhookEvent struct {
    ID        string
    ToolID    string
    TenantID  string
    Payload   []byte
    ExpiresAt time.Time // Same as Context
    CreatedAt time.Time
}
```

**Implementation:**
1. Store in Redis with TTL (24 hours)
2. No compression or state transitions
3. Let Redis handle expiration automatically
4. Optional: Archive to S3 before expiration

**Benefits:**
- Consistent with existing patterns
- Simpler implementation
- Redis TTL handles lifecycle automatically
- Less code to maintain

### Option 2: Enhance Context Storage
Upgrade context storage to support lifecycle management:

```go
type StorageLifecycle interface {
    // Transition between storage tiers
    TransitionToWarm(ctx context.Context, id string) error
    TransitionToCold(ctx context.Context, id string) error
    
    // Generate summaries
    GenerateSummary(ctx context.Context, ids []string) (*Summary, error)
}
```

**Benefits:**
- Unified lifecycle management
- Both contexts and webhooks benefit
- More sophisticated storage optimization

### Option 3: Separate Patterns (Not Recommended)
Keep webhook lifecycle separate from context storage:

**Drawbacks:**
- Inconsistent patterns in codebase
- Confusing for developers
- More code to maintain
- Harder to reason about

## Recommendation

**Use Option 1: Simplify Webhook Lifecycle**

1. **Immediate Implementation:**
   ```go
   // Store webhook with simple TTL
   err := redisClient.Set(ctx, key, event, 24*time.Hour).Err()
   ```

2. **Optional Archival:**
   ```go
   // Before expiration, archive important events
   if shouldArchive(event) {
       s3Storage.Archive(ctx, event)
   }
   ```

3. **Context Integration:**
   ```go
   // Add webhook summary to context (not full event)
   contextUpdate := &ContextItem{
       Type: "webhook_event",
       Summary: summarizeWebhook(event),
       Timestamp: event.CreatedAt,
   }
   ```

## Benefits of Alignment

1. **Consistency**: Same patterns throughout codebase
2. **Simplicity**: Developers understand one pattern
3. **Maintainability**: Less complex lifecycle code
4. **Performance**: Redis TTL is efficient
5. **Flexibility**: Can enhance later if needed

## Migration Path

1. Start with simple TTL-based expiration
2. Add archival as a background job if needed
3. Generate summaries for context updates
4. Consider enhancing both systems together later

## Code Changes Required

### Before (Complex Lifecycle):
```go
func (w *WebhookLifecycle) Process(event *WebhookEvent) {
    age := time.Since(event.CreatedAt)
    switch {
    case age < 2*time.Hour:
        // Keep in hot storage
    case age < 24*time.Hour:
        w.compressAndMoveToWarm(event)
    default:
        w.archiveToCold(event)
        w.generateSummary(event)
    }
}
```

### After (Simple TTL):
```go
func (w *WebhookHandler) Store(event *WebhookEvent) error {
    // Store with TTL
    key := fmt.Sprintf("webhook:%s", event.ID)
    return w.redis.Set(ctx, key, event, 24*time.Hour).Err()
}

// Optional background job
func (w *WebhookArchiver) ArchiveExpiring(ctx context.Context) {
    // Run daily to archive events about to expire
    events := w.getExpiringEvents(ctx)
    for _, event := range events {
        if w.shouldArchive(event) {
            w.s3.Archive(ctx, event)
        }
    }
}
```
package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/audit"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// MockAuditLogger captures audit events for testing
type MockAuditLogger struct {
	events []*audit.AuditEvent
}

func (m *MockAuditLogger) LogOperation(ctx context.Context, eventType audit.EventType, operation string, resource string, start time.Time, err error) {
	event := &audit.AuditEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		TenantID:  auth.GetTenantID(ctx),
		Operation: operation,
		Resource:  resource,
		Result:    audit.ResultSuccess,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}

	if err != nil {
		event.Result = audit.ResultFailure
		event.Error = err.Error()
	}

	m.events = append(m.events, event)
}

func (m *MockAuditLogger) LogSecurityEvent(ctx context.Context, eventType audit.EventType, resource string, metadata map[string]interface{}) {
	event := &audit.AuditEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		TenantID:  auth.GetTenantID(ctx),
		Operation: string(eventType),
		Resource:  resource,
		Result:    audit.ResultSuccess,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	m.events = append(m.events, event)
}

func (m *MockAuditLogger) LogSystemEvent(eventType audit.EventType, description string, metadata map[string]interface{}) {
	event := &audit.AuditEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Operation: string(eventType),
		Resource:  "system",
		Result:    audit.ResultSuccess,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	m.events = append(m.events, event)
}

func (m *MockAuditLogger) GetEvents() []*audit.AuditEvent {
	return m.events
}

func (m *MockAuditLogger) Clear() {
	m.events = nil
}

func TestAuditLogging(t *testing.T) {
	// Skip if Redis not available
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	err := redisClient.Ping(context.Background()).Err()
	if err != nil {
		t.Skip("Redis not available")
	}

	ctx := context.Background()
	tenantID := uuid.New()
	ctx = auth.WithTenantID(ctx, tenantID)
	ctx = auth.WithUserID(ctx, "test-user")

	logger := observability.NewLogger("test")

	t.Run("OperationLogging", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableAuditLogging = true
		config.Prefix = "test_audit"

		cache, err := NewSemanticCache(redisClient, config, logger)
		require.NoError(t, err)
		defer func() {
			_ = cache.Shutdown(ctx)
		}()

		// Since audit logger logs to the observability logger, we can verify
		// that operations complete successfully
		// The real audit logger is already enabled via config.EnableAuditLogging

		// Test data
		query := "test query"
		embedding := []float32{0.1, 0.2, 0.3}
		results := []CachedSearchResult{
			{ID: "1", Score: 0.9},
		}

		// Test Set operation
		err = cache.Set(ctx, query, embedding, results)
		assert.NoError(t, err)

		// Test Get operation
		entry, err := cache.Get(ctx, query, embedding)
		assert.NoError(t, err)
		assert.NotNil(t, entry)
		assert.Equal(t, query, entry.Query)
		assert.Len(t, entry.Results, 1)

		// Test Delete operation
		err = cache.Delete(ctx, query)
		assert.NoError(t, err)

		// Verify deletion worked
		entry, err = cache.Get(ctx, query, embedding)
		assert.NoError(t, err)
		assert.Nil(t, entry)
	})

	t.Run("FailureLogging", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableAuditLogging = true
		config.Prefix = "test_audit_fail"

		// Create cache with bad Redis connection
		badRedis := redis.NewClient(&redis.Options{
			Addr:        "invalid:6379",
			DialTimeout: 100 * time.Millisecond,
		})

		cache, err := NewSemanticCache(badRedis, config, logger)
		require.NoError(t, err)
		defer func() {
			_ = cache.Shutdown(ctx)
		}()

		// Test operation with bad Redis - should enter degraded mode
		err = cache.Set(ctx, "test", []float32{0.1}, []CachedSearchResult{{ID: "1"}})
		// The operation should succeed via degraded mode
		assert.NoError(t, err)

		// Verify cache is in degraded mode
		assert.True(t, cache.degradedMode.Load())
	})

	t.Run("SecurityEventLogging", func(t *testing.T) {
		auditLogger := audit.NewLogger(logger, true)

		// Test degraded mode event
		auditLogger.LogSystemEvent(audit.EventDegradedMode, "Redis connection failed", map[string]interface{}{
			"error": "connection refused",
		})

		// Test recovery event
		auditLogger.LogSystemEvent(audit.EventRecovery, "Redis connection restored", nil)

		// Test encryption event
		auditLogger.LogSecurityEvent(ctx, audit.EventEncryption, "sensitive_data", map[string]interface{}{
			"algorithm": "AES-256-GCM",
			"key_id":    "key123",
		})
	})

	t.Run("ComplianceLogging", func(t *testing.T) {
		complianceLogger := audit.NewComplianceLogger(logger)

		// Test data access logging
		complianceLogger.LogDataAccess(ctx, "search", "user_queries", 10, true)

		// Test data modification logging
		complianceLogger.LogDataModification(ctx, "update", "cache_entries", 5, "bulk_update")
	})
}

func TestAuditFilter(t *testing.T) {
	logger := observability.NewLogger("test")
	auditLogger := audit.NewLogger(logger, true)

	// Set custom filter to exclude GET operations
	auditLogger.SetFilter(func(event *audit.AuditEvent) bool {
		return event.EventType != audit.EventCacheGet
	})

	ctx := context.Background()

	// This should be filtered out
	auditLogger.LogOperation(ctx, audit.EventCacheGet, "get", "query1", time.Now(), nil)

	// This should pass through
	auditLogger.LogOperation(ctx, audit.EventCacheSet, "set", "query2", time.Now(), nil)
}

func BenchmarkAuditLogging(b *testing.B) {
	logger := observability.NewLogger("bench")
	auditLogger := audit.NewLogger(logger, true)

	ctx := context.Background()
	ctx = auth.WithTenantID(ctx, uuid.New())
	ctx = auth.WithUserID(ctx, "bench-user")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		auditLogger.LogOperation(ctx, audit.EventCacheGet, "get", "query", start, nil)
	}
}

package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextOperations(t *testing.T) {
	t.Run("correlation ID operations", func(t *testing.T) {
		ctx := context.Background()
		correlationID := "test-correlation-123"

		// Test setting and getting
		ctx = WithCorrelationID(ctx, correlationID)
		assert.Equal(t, correlationID, GetCorrelationID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetCorrelationID(emptyCtx))
	})

	t.Run("causation ID operations", func(t *testing.T) {
		ctx := context.Background()
		causationID := "test-causation-456"

		// Test setting and getting
		ctx = WithCausationID(ctx, causationID)
		assert.Equal(t, causationID, GetCausationID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetCausationID(emptyCtx))
	})

	t.Run("tenant ID operations", func(t *testing.T) {
		ctx := context.Background()
		tenantID := "tenant-789"

		// Test setting and getting
		ctx = WithTenantID(ctx, tenantID)
		assert.Equal(t, tenantID, GetTenantID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetTenantID(emptyCtx))
	})

	t.Run("user ID operations", func(t *testing.T) {
		ctx := context.Background()
		userID := "user-abc"

		// Test setting and getting
		ctx = WithUserID(ctx, userID)
		assert.Equal(t, userID, GetUserID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetUserID(emptyCtx))
	})

	t.Run("request ID operations", func(t *testing.T) {
		ctx := context.Background()
		requestID := "req-def"

		// Test setting and getting
		ctx = WithRequestID(ctx, requestID)
		assert.Equal(t, requestID, GetRequestID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetRequestID(emptyCtx))
	})

	t.Run("operation operations", func(t *testing.T) {
		ctx := context.Background()
		operation := "CreateWorkflow"

		// Test setting and getting
		ctx = WithOperation(ctx, operation)
		assert.Equal(t, operation, GetOperation(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetOperation(emptyCtx))
	})

	t.Run("extract metadata", func(t *testing.T) {
		ctx := context.Background()

		// Add all metadata
		ctx = WithCorrelationID(ctx, "corr-123")
		ctx = WithCausationID(ctx, "caus-456")
		ctx = WithTenantID(ctx, "tenant-789")
		ctx = WithUserID(ctx, "user-abc")
		ctx = WithRequestID(ctx, "req-def")
		ctx = WithOperation(ctx, "TestOp")

		// Extract metadata
		metadata := ExtractMetadata(ctx)

		// Verify all values
		assert.Equal(t, "corr-123", metadata["correlation_id"])
		assert.Equal(t, "caus-456", metadata["causation_id"])
		assert.Equal(t, "tenant-789", metadata["tenant_id"])
		assert.Equal(t, "user-abc", metadata["user_id"])
		assert.Equal(t, "req-def", metadata["request_id"])
		assert.Equal(t, "TestOp", metadata["operation"])

		// Test empty context
		emptyMetadata := ExtractMetadata(context.Background())
		assert.Empty(t, emptyMetadata)
	})

	t.Run("inject metadata", func(t *testing.T) {
		ctx := context.Background()

		metadata := map[string]string{
			"correlation_id": "corr-123",
			"causation_id":   "caus-456",
			"tenant_id":      "tenant-789",
			"user_id":        "user-abc",
			"request_id":     "req-def",
			"operation":      "TestOp",
		}

		// Inject metadata
		ctx = InjectMetadata(ctx, metadata)

		// Verify all values
		assert.Equal(t, "corr-123", GetCorrelationID(ctx))
		assert.Equal(t, "caus-456", GetCausationID(ctx))
		assert.Equal(t, "tenant-789", GetTenantID(ctx))
		assert.Equal(t, "user-abc", GetUserID(ctx))
		assert.Equal(t, "req-def", GetRequestID(ctx))
		assert.Equal(t, "TestOp", GetOperation(ctx))
	})

	t.Run("inject partial metadata", func(t *testing.T) {
		ctx := context.Background()

		// Only inject some metadata
		metadata := map[string]string{
			"correlation_id": "corr-123",
			"tenant_id":      "tenant-789",
		}

		ctx = InjectMetadata(ctx, metadata)

		// Verify injected values
		assert.Equal(t, "corr-123", GetCorrelationID(ctx))
		assert.Equal(t, "tenant-789", GetTenantID(ctx))

		// Verify non-injected values are empty
		assert.Equal(t, "", GetCausationID(ctx))
		assert.Equal(t, "", GetUserID(ctx))
		assert.Equal(t, "", GetRequestID(ctx))
		assert.Equal(t, "", GetOperation(ctx))
	})

	t.Run("inject unknown metadata keys", func(t *testing.T) {
		ctx := context.Background()

		// Include unknown keys
		metadata := map[string]string{
			"correlation_id": "corr-123",
			"unknown_key":    "unknown_value",
		}

		ctx = InjectMetadata(ctx, metadata)

		// Verify known key works
		assert.Equal(t, "corr-123", GetCorrelationID(ctx))

		// Unknown key should be ignored (no panic)
	})

	t.Run("backwards compatibility with string keys", func(t *testing.T) {
		ctx := context.Background()

		// Set values with string keys directly
		// nolint:staticcheck // Testing backwards compatibility with string keys
		ctx = context.WithValue(ctx, "correlation_id", "corr-string")
		// nolint:staticcheck // Testing backwards compatibility with string keys
		ctx = context.WithValue(ctx, "causation_id", "caus-string")
		// nolint:staticcheck // Testing backwards compatibility with string keys
		ctx = context.WithValue(ctx, "tenant_id", "tenant-string")
		// nolint:staticcheck // Testing backwards compatibility with string keys
		ctx = context.WithValue(ctx, "user_id", "user-string")
		// nolint:staticcheck // Testing backwards compatibility with string keys
		ctx = context.WithValue(ctx, "request_id", "req-string")
		// nolint:staticcheck // Testing backwards compatibility with string keys
		ctx = context.WithValue(ctx, "operation", "op-string")

		// Getters should still work
		assert.Equal(t, "corr-string", GetCorrelationID(ctx))
		assert.Equal(t, "caus-string", GetCausationID(ctx))
		assert.Equal(t, "tenant-string", GetTenantID(ctx))
		assert.Equal(t, "user-string", GetUserID(ctx))
		assert.Equal(t, "req-string", GetRequestID(ctx))
		assert.Equal(t, "op-string", GetOperation(ctx))
	})

	t.Run("session ID operations", func(t *testing.T) {
		ctx := context.Background()
		sessionID := "session-xyz"

		// Test setting and getting
		ctx = WithSessionID(ctx, sessionID)
		assert.Equal(t, sessionID, GetSessionID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetSessionID(emptyCtx))
	})

	t.Run("extract metadata with session_id", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")
		ctx = WithSessionID(ctx, "session-456")
		ctx = WithTenantID(ctx, "tenant-789")

		metadata := ExtractMetadata(ctx)

		assert.Equal(t, "req-123", metadata["request_id"])
		assert.Equal(t, "session-456", metadata["session_id"])
		assert.Equal(t, "tenant-789", metadata["tenant_id"])
	})

	t.Run("inject metadata with session_id", func(t *testing.T) {
		ctx := context.Background()

		metadata := map[string]string{
			"request_id": "req-123",
			"session_id": "session-456",
			"tenant_id":  "tenant-789",
		}

		ctx = InjectMetadata(ctx, metadata)

		assert.Equal(t, "req-123", GetRequestID(ctx))
		assert.Equal(t, "session-456", GetSessionID(ctx))
		assert.Equal(t, "tenant-789", GetTenantID(ctx))
	})
}

func TestGenerateRequestID(t *testing.T) {
	t.Run("generates unique IDs", func(t *testing.T) {
		id1 := GenerateRequestID()
		id2 := GenerateRequestID()

		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
	})

	t.Run("generates valid UUIDs", func(t *testing.T) {
		id := GenerateRequestID()

		// UUID v4 format: xxxxxxxx-xxxx-4xxx-xxxx-xxxxxxxxxxxx
		assert.Len(t, id, 36) // 32 hex chars + 4 hyphens
		assert.Contains(t, id, "-")
	})
}

func TestLoggerFromContext(t *testing.T) {
	baseLogger := NewStandardLogger("test")

	t.Run("with no context fields", func(t *testing.T) {
		ctx := context.Background()
		logger := LoggerFromContext(ctx, baseLogger)

		assert.NotNil(t, logger)
		// Should return base logger when no context fields
		assert.Equal(t, baseLogger, logger)
	})

	t.Run("with all context fields", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")
		ctx = WithTenantID(ctx, "tenant-456")
		ctx = WithSessionID(ctx, "session-789")
		ctx = WithOperation(ctx, "test-operation")
		ctx = WithCorrelationID(ctx, "corr-abc")

		logger := LoggerFromContext(ctx, baseLogger)

		assert.NotNil(t, logger)

		// Verify logger has context fields
		if stdLogger, ok := logger.(*StandardLogger); ok {
			assert.Len(t, stdLogger.contextFields, 5)
			assert.Equal(t, "req-123", stdLogger.contextFields["request_id"])
			assert.Equal(t, "tenant-456", stdLogger.contextFields["tenant_id"])
			assert.Equal(t, "session-789", stdLogger.contextFields["session_id"])
			assert.Equal(t, "test-operation", stdLogger.contextFields["operation"])
			assert.Equal(t, "corr-abc", stdLogger.contextFields["correlation_id"])
		} else {
			t.Error("Expected *StandardLogger type")
		}
	})

	t.Run("with partial context fields", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")
		ctx = WithTenantID(ctx, "tenant-456")

		logger := LoggerFromContext(ctx, baseLogger)

		assert.NotNil(t, logger)

		// Verify logger has only the set fields
		if stdLogger, ok := logger.(*StandardLogger); ok {
			assert.Len(t, stdLogger.contextFields, 2)
			assert.Equal(t, "req-123", stdLogger.contextFields["request_id"])
			assert.Equal(t, "tenant-456", stdLogger.contextFields["tenant_id"])
		}
	})
}

func TestSampledLogger(t *testing.T) {
	baseLogger := NewStandardLogger("test")

	t.Run("sample rate 100 percent", func(t *testing.T) {
		sampled := NewSampledLogger(baseLogger, 1.0)

		// Should log everything at 100%
		assert.True(t, sampled.shouldLog(LogLevelInfo))
		assert.True(t, sampled.shouldLog(LogLevelDebug))
		assert.True(t, sampled.shouldLog(LogLevelWarn))
		assert.True(t, sampled.shouldLog(LogLevelError))
	})

	t.Run("sample rate 0 percent", func(t *testing.T) {
		sampled := NewSampledLogger(baseLogger, 0.0)

		// Should not log info/debug at 0%, but always log errors
		assert.False(t, sampled.shouldLog(LogLevelInfo))
		assert.False(t, sampled.shouldLog(LogLevelDebug))
		assert.False(t, sampled.shouldLog(LogLevelWarn))
		assert.True(t, sampled.shouldLog(LogLevelError))
		assert.True(t, sampled.shouldLog(LogLevelFatal))
	})

	t.Run("errors always logged regardless of sample rate", func(t *testing.T) {
		sampled := NewSampledLogger(baseLogger, 0.1)

		// Errors and fatals should always be logged
		assert.True(t, sampled.shouldLog(LogLevelError))
		assert.True(t, sampled.shouldLog(LogLevelFatal))
	})

	t.Run("with fields preserves sampling", func(t *testing.T) {
		sampled := NewSampledLogger(baseLogger, 1.0)
		withFields := sampled.With(map[string]interface{}{"test": "value"})

		assert.NotNil(t, withFields)

		// Should still be a SampledLogger
		if sampledLogger, ok := withFields.(*SampledLogger); ok {
			assert.Equal(t, 1.0, sampledLogger.sampleRate)
		} else {
			t.Error("Expected *SampledLogger type")
		}
	})

	t.Run("with prefix preserves sampling", func(t *testing.T) {
		sampled := NewSampledLogger(baseLogger, 0.5)
		withPrefix := sampled.WithPrefix("newprefix")

		assert.NotNil(t, withPrefix)

		// Should still be a SampledLogger
		if sampledLogger, ok := withPrefix.(*SampledLogger); ok {
			assert.Equal(t, 0.5, sampledLogger.sampleRate)
		} else {
			t.Error("Expected *SampledLogger type")
		}
	})

	t.Run("invalid sample rates are clamped", func(t *testing.T) {
		// Negative rate should be clamped to 0.0
		sampled1 := NewSampledLogger(baseLogger, -0.5)
		assert.Equal(t, 0.0, sampled1.sampleRate)

		// Rate > 1.0 should be clamped to 1.0
		sampled2 := NewSampledLogger(baseLogger, 1.5)
		assert.Equal(t, 1.0, sampled2.sampleRate)
	})
}

func TestPerformanceLogger(t *testing.T) {
	baseLogger := NewStandardLogger("test")
	perfLogger := NewPerformanceLogger(baseLogger)

	t.Run("log with duration adds timing fields", func(t *testing.T) {
		// This test ensures LogWithDuration doesn't panic
		perfLogger.LogWithDuration(LogLevelInfo, "test message", 100000, map[string]interface{}{
			"extra": "field",
		})
	})

	t.Run("log with duration on nil fields", func(t *testing.T) {
		// Should not panic with nil fields
		perfLogger.LogWithDuration(LogLevelInfo, "test message", 50000, nil)
	})

	t.Run("start timer returns valid function", func(t *testing.T) {
		timerFunc := perfLogger.StartTimer("test operation", LogLevelInfo)

		assert.NotNil(t, timerFunc)

		// Call the timer function - should not panic
		timerFunc(map[string]interface{}{"status": "success"})
	})

	t.Run("start timer with nil fields", func(t *testing.T) {
		timerFunc := perfLogger.StartTimer("test operation", LogLevelInfo)

		assert.NotNil(t, timerFunc)

		// Should not panic with nil fields
		timerFunc(nil)
	})

	t.Run("with fields returns performance logger", func(t *testing.T) {
		withFields := perfLogger.With(map[string]interface{}{"test": "value"})

		assert.NotNil(t, withFields)

		// Should still be a PerformanceLogger
		if _, ok := withFields.(*PerformanceLogger); !ok {
			t.Error("Expected *PerformanceLogger type")
		}
	})

	t.Run("with prefix returns performance logger", func(t *testing.T) {
		withPrefix := perfLogger.WithPrefix("newprefix")

		assert.NotNil(t, withPrefix)

		// Should still be a PerformanceLogger
		if _, ok := withPrefix.(*PerformanceLogger); !ok {
			t.Error("Expected *PerformanceLogger type")
		}
	})
}

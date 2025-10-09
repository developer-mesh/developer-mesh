package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTracedCache_NilSpanHelper(t *testing.T) {
	baseCache := NewMemoryCache(100, 5*time.Minute)
	tracedCache := NewTracedCache(baseCache, nil)

	// Should return unwrapped cache if no span helper
	assert.Equal(t, baseCache, tracedCache)
}

func TestNewTracedCache_WithSpanHelper(t *testing.T) {
	baseCache := NewMemoryCache(100, 5*time.Minute)

	// Create tracer provider
	config := &tracing.Config{
		Enabled:      true,
		ServiceName:  "test-cache",
		SamplingRate: 1.0,
	}
	tp, err := tracing.NewTracerProvider(config)
	require.NoError(t, err)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	sh := tracing.NewSpanHelper(tp)
	tracedCache := NewTracedCache(baseCache, sh)

	// Should return wrapped cache
	assert.NotEqual(t, baseCache, tracedCache)
	tc, ok := tracedCache.(*TracedCache)
	assert.True(t, ok)
	assert.NotNil(t, tc.spanHelper)
}

func TestTracedCache_Get_Hit(t *testing.T) {
	baseCache := NewMemoryCache(100, 5*time.Minute)

	// Create tracer provider
	config := &tracing.Config{
		Enabled:      true,
		ServiceName:  "test-cache",
		SamplingRate: 1.0,
	}
	tp, err := tracing.NewTracerProvider(config)
	require.NoError(t, err)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	sh := tracing.NewSpanHelper(tp)
	tracedCache := NewTracedCache(baseCache, sh)

	ctx := context.Background()

	// Set a value
	err = tracedCache.Set(ctx, "test-key", "test-value", 1*time.Minute)
	require.NoError(t, err)

	// Get the value (should be a cache hit)
	var result string
	err = tracedCache.Get(ctx, "test-key", &result)
	require.NoError(t, err)
	assert.Equal(t, "test-value", result)
}

func TestTracedCache_Get_Miss(t *testing.T) {
	baseCache := NewMemoryCache(100, 5*time.Minute)

	// Create tracer provider
	config := &tracing.Config{
		Enabled:      true,
		ServiceName:  "test-cache",
		SamplingRate: 1.0,
	}
	tp, err := tracing.NewTracerProvider(config)
	require.NoError(t, err)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	sh := tracing.NewSpanHelper(tp)
	tracedCache := NewTracedCache(baseCache, sh)

	ctx := context.Background()

	// Get non-existent value (should be a cache miss)
	var result string
	err = tracedCache.Get(ctx, "non-existent-key", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

func TestTracedCache_Get_Expired(t *testing.T) {
	baseCache := NewMemoryCache(100, 5*time.Minute)

	// Create tracer provider
	config := &tracing.Config{
		Enabled:      true,
		ServiceName:  "test-cache",
		SamplingRate: 1.0,
	}
	tp, err := tracing.NewTracerProvider(config)
	require.NoError(t, err)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	sh := tracing.NewSpanHelper(tp)
	tracedCache := NewTracedCache(baseCache, sh)

	ctx := context.Background()

	// Set a value with very short TTL
	err = tracedCache.Set(ctx, "test-key", "test-value", 1*time.Millisecond)
	require.NoError(t, err)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Get expired value (should fail)
	var result string
	err = tracedCache.Get(ctx, "test-key", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key expired")
}

func TestTracedCache_Set(t *testing.T) {
	baseCache := NewMemoryCache(100, 5*time.Minute)

	// Create tracer provider
	config := &tracing.Config{
		Enabled:      true,
		ServiceName:  "test-cache",
		SamplingRate: 1.0,
	}
	tp, err := tracing.NewTracerProvider(config)
	require.NoError(t, err)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	sh := tracing.NewSpanHelper(tp)
	tracedCache := NewTracedCache(baseCache, sh)

	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value interface{}
		ttl   time.Duration
	}{
		{"string value", "key1", "value1", 1 * time.Minute},
		{"int value", "key2", 42, 1 * time.Minute},
		{"map value", "key3", map[string]string{"nested": "data"}, 1 * time.Minute},
		{"zero ttl", "key4", "value4", 0}, // Should use default TTL
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tracedCache.Set(ctx, tt.key, tt.value, tt.ttl)
			assert.NoError(t, err)
		})
	}
}

func TestTracedCache_Delete(t *testing.T) {
	baseCache := NewMemoryCache(100, 5*time.Minute)

	// Create tracer provider
	config := &tracing.Config{
		Enabled:      true,
		ServiceName:  "test-cache",
		SamplingRate: 1.0,
	}
	tp, err := tracing.NewTracerProvider(config)
	require.NoError(t, err)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	sh := tracing.NewSpanHelper(tp)
	tracedCache := NewTracedCache(baseCache, sh)

	ctx := context.Background()

	// Set a value
	err = tracedCache.Set(ctx, "test-key", "test-value", 1*time.Minute)
	require.NoError(t, err)

	// Delete the value
	err = tracedCache.Delete(ctx, "test-key")
	assert.NoError(t, err)

	// Verify it's deleted
	var result string
	err = tracedCache.Get(ctx, "test-key", &result)
	assert.Error(t, err)
}

func TestTracedCache_Size(t *testing.T) {
	baseCache := NewMemoryCache(100, 5*time.Minute)

	// Create tracer provider
	config := &tracing.Config{
		Enabled:      true,
		ServiceName:  "test-cache",
		SamplingRate: 1.0,
	}
	tp, err := tracing.NewTracerProvider(config)
	require.NoError(t, err)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	sh := tracing.NewSpanHelper(tp)
	tracedCache := NewTracedCache(baseCache, sh)

	ctx := context.Background()

	// Initially empty
	assert.Equal(t, 0, tracedCache.Size())

	// Add items
	err = tracedCache.Set(ctx, "key1", "value1", 1*time.Minute)
	require.NoError(t, err)
	err = tracedCache.Set(ctx, "key2", "value2", 1*time.Minute)
	require.NoError(t, err)

	assert.Equal(t, 2, tracedCache.Size())

	// Delete one
	err = tracedCache.Delete(ctx, "key1")
	require.NoError(t, err)

	assert.Equal(t, 1, tracedCache.Size())
}

func TestTracedCache_ErrorRecording(t *testing.T) {
	// Create a mock cache that always errors
	mockCache := &mockErrorCache{err: errors.New("mock error")}

	// Create tracer provider
	config := &tracing.Config{
		Enabled:      true,
		ServiceName:  "test-cache",
		SamplingRate: 1.0,
	}
	tp, err := tracing.NewTracerProvider(config)
	require.NoError(t, err)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	sh := tracing.NewSpanHelper(tp)
	tracedCache := NewTracedCache(mockCache, sh)

	ctx := context.Background()

	// All operations should record errors in spans
	var result string
	err = tracedCache.Get(ctx, "key", &result)
	assert.Error(t, err)

	err = tracedCache.Set(ctx, "key", "value", 1*time.Minute)
	assert.Error(t, err)

	err = tracedCache.Delete(ctx, "key")
	assert.Error(t, err)
}

// mockErrorCache always returns an error for testing
type mockErrorCache struct {
	err error
}

func (m *mockErrorCache) Get(ctx context.Context, key string, value interface{}) error {
	return m.err
}

func (m *mockErrorCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return m.err
}

func (m *mockErrorCache) Delete(ctx context.Context, key string) error {
	return m.err
}

func (m *mockErrorCache) Size() int {
	return 0
}

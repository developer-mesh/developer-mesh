package cache

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tracing"
	"go.opentelemetry.io/otel/codes"
)

// TracedCache wraps a cache implementation with distributed tracing
type TracedCache struct {
	cache      Cache
	spanHelper *tracing.SpanHelper
}

// NewTracedCache creates a new traced cache wrapper
func NewTracedCache(cache Cache, spanHelper *tracing.SpanHelper) Cache {
	if spanHelper == nil {
		return cache // Return unwrapped if no tracer
	}
	return &TracedCache{
		cache:      cache,
		spanHelper: spanHelper,
	}
}

// Get retrieves a value from the cache with tracing
func (tc *TracedCache) Get(ctx context.Context, key string, value interface{}) error {
	ctx, span := tc.spanHelper.StartCacheOperationSpan(ctx, "get", key)
	defer span.End()

	err := tc.cache.Get(ctx, key, value)
	if err != nil {
		tc.spanHelper.RecordCacheHit(ctx, false) // Cache miss
		span.RecordError(err)
		// Don't set error status for cache misses as they're expected
		if err.Error() != "key not found: "+key && err.Error() != "key expired: "+key {
			span.SetStatus(codes.Error, "cache get failed")
		}
		return err
	}

	tc.spanHelper.RecordCacheHit(ctx, true) // Cache hit
	span.SetStatus(codes.Ok, "cache hit")
	return nil
}

// Set stores a value in the cache with tracing
func (tc *TracedCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	ctx, span := tc.spanHelper.StartCacheOperationSpan(ctx, "set", key)
	defer span.End()

	err := tc.cache.Set(ctx, key, value, ttl)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "cache set failed")
		return err
	}

	span.SetStatus(codes.Ok, "cache set successful")
	return nil
}

// Delete removes a key from the cache with tracing
func (tc *TracedCache) Delete(ctx context.Context, key string) error {
	ctx, span := tc.spanHelper.StartCacheOperationSpan(ctx, "delete", key)
	defer span.End()

	err := tc.cache.Delete(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "cache delete failed")
		return err
	}

	span.SetStatus(codes.Ok, "cache delete successful")
	return nil
}

// Size returns the number of items in the cache
func (tc *TracedCache) Size() int {
	// Size doesn't need tracing as it's a simple operation
	return tc.cache.Size()
}

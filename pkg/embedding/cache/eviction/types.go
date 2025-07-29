package eviction

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// VectorStore defines the interface for vector store operations needed by eviction
type VectorStore interface {
	GetTenantCacheStats(ctx context.Context, tenantID uuid.UUID) (*TenantCacheStats, error)
	GetLRUEntries(ctx context.Context, tenantID uuid.UUID, limit int) ([]LRUEntry, error)
	DeleteCacheEntry(ctx context.Context, tenantID uuid.UUID, cacheKey string) error
	GetTenantsWithCache(ctx context.Context) ([]uuid.UUID, error)
	CleanupStaleEntries(ctx context.Context, staleDuration time.Duration) (int64, error)
	GetGlobalCacheStats(ctx context.Context) (map[string]interface{}, error)
}

// TenantCacheStats represents cache statistics for a tenant
type TenantCacheStats struct {
	TenantID    uuid.UUID `db:"tenant_id"`
	EntryCount  int       `db:"entry_count"`
	TotalHits   int       `db:"total_hits"`
	OldestEntry time.Time `db:"oldest_entry"`
	NewestEntry time.Time `db:"newest_entry"`
}

// LRUEntry represents an entry for LRU eviction
type LRUEntry struct {
	CacheKey       string    `db:"cache_key"`
	QueryHash      string    `db:"query_hash"`
	LastAccessedAt time.Time `db:"last_accessed_at"`
	HitCount       int       `db:"hit_count"`
}

// RecoverMiddleware is a helper for panic recovery
func RecoverMiddleware(logger observability.Logger, operation string) func(func()) {
	return func(fn func()) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic recovered", map[string]interface{}{
					"operation": operation,
					"panic":     r,
				})
			}
		}()
		fn()
	}
}

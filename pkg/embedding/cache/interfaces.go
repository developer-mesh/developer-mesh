package cache

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/lru"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/tenant"
	"github.com/google/uuid"
)

// Type aliases for cleaner interface
type TenantCacheConfig = tenant.CacheTenantConfig

// Cache defines the interface for semantic cache operations.
// It provides methods for storing and retrieving search results based on
// query similarity, with support for batch operations and lifecycle management.
//
// Implementations must be safe for concurrent use by multiple goroutines.
type Cache interface {
	// Core operations
	Get(ctx context.Context, query string, embedding []float32) (*CacheEntry, error)
	Set(ctx context.Context, query string, embedding []float32, results []CachedSearchResult) error
	Delete(ctx context.Context, query string) error

	// Batch operations
	GetBatch(ctx context.Context, queries []string, embeddings [][]float32) ([]*CacheEntry, error)

	// Management
	Clear(ctx context.Context) error
	GetStats() *CacheStats
	Shutdown(ctx context.Context) error
}

// TenantCache extends Cache with tenant-aware operations.
// It provides multi-tenant isolation, per-tenant configuration,
// and tenant-specific management capabilities.
//
// All operations are isolated by tenant ID, which must be present
// in the context or explicitly provided.
type TenantCache interface {
	Cache

	// Tenant-specific operations
	GetWithTenant(ctx context.Context, tenantID uuid.UUID, query string, embedding []float32) (*CacheEntry, error)
	SetWithTenant(ctx context.Context, tenantID uuid.UUID, query string, embedding []float32, results []CachedSearchResult) error
	DeleteWithTenant(ctx context.Context, tenantID uuid.UUID, query string) error

	// Tenant management
	ClearTenant(ctx context.Context, tenantID uuid.UUID) error
	GetTenantStats(ctx context.Context, tenantID uuid.UUID) (*TenantCacheStats, error)
	GetTenantConfig(ctx context.Context, tenantID uuid.UUID) (*TenantCacheConfig, error)

	// LRU operations
	GetLRUManager() LRUManager
}

// LRUManager defines the interface for LRU eviction management.
// It tracks cache access patterns and performs eviction when storage
// limits are exceeded, supporting both tenant-specific and global policies.
//
// Implementations should efficiently track access times and handle
// high-throughput access tracking with minimal overhead.
type LRUManager interface {
	// Eviction operations
	EvictForTenant(ctx context.Context, tenantID uuid.UUID, targetBytes int64) error
	EvictGlobal(ctx context.Context, targetBytes int64) error

	// Tracking operations
	TrackAccess(tenantID uuid.UUID, key string)
	GetAccessScore(ctx context.Context, tenantID uuid.UUID, key string) (float64, error)
	GetLRUKeys(ctx context.Context, tenantID uuid.UUID, limit int) ([]string, error)

	// Statistics
	GetStats() map[string]interface{}
	GetTenantStats(ctx context.Context, tenantID uuid.UUID) (*lru.LRUStats, error)

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// CacheStore defines low-level cache storage operations.
// It provides a simple key-value interface for cache backends,
// abstracting the underlying storage implementation.
//
// Implementations may use Redis, Memcached, or other storage systems.
type CacheStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Batch operations
	MGet(ctx context.Context, keys []string) ([][]byte, error)
	MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error

	// Pattern operations
	Keys(ctx context.Context, pattern string) ([]string, error)
	DeletePattern(ctx context.Context, pattern string) error
}

// VectorSearchEngine defines vector similarity search operations.
// It provides methods for storing and searching high-dimensional vectors
// using approximate nearest neighbor algorithms.
//
// Implementations typically use specialized vector databases like pgvector
// or dedicated vector search engines.
type VectorSearchEngine interface {
	// Storage operations
	StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error
	DeleteEmbedding(ctx context.Context, id string) error

	// Search operations
	SearchSimilar(ctx context.Context, embedding []float32, threshold float32, limit int) ([]SearchResult, error)
	SearchSimilarWithFilter(ctx context.Context, embedding []float32, threshold float32, limit int, filter map[string]interface{}) ([]SearchResult, error)

	// Management
	GetIndexStats(ctx context.Context) (*IndexStats, error)
	OptimizeIndex(ctx context.Context) error
	HealthCheck(ctx context.Context) error
}

// CompressionEngine defines compression operations
type CompressionEngine interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	IsCompressed(data []byte) bool
	GetCompressionRatio(data []byte) (float64, error)
}

// EncryptionEngine defines encryption operations
type EncryptionEngine interface {
	Encrypt(data []byte, context string) ([]byte, error)
	Decrypt(data []byte, context string) ([]byte, error)
	RotateKey(oldContext, newContext string) error
}

// SearchResult represents a vector search result
type SearchResult struct {
	ID       string                 `json:"id"`
	Score    float32                `json:"score"`
	Metadata map[string]interface{} `json:"metadata"`
}

// IndexStats represents vector index statistics
type IndexStats struct {
	TotalVectors int64     `json:"total_vectors"`
	IndexSize    int64     `json:"index_size_bytes"`
	LastUpdated  time.Time `json:"last_updated"`
}

// LRUStats represents LRU manager statistics
type LRUStats struct {
	TotalKeys        int64         `json:"total_keys"`
	TotalBytes       int64         `json:"total_bytes"`
	EvictionCount    int64         `json:"eviction_count"`
	LastEviction     time.Time     `json:"last_eviction"`
	AverageAccessAge time.Duration `json:"average_access_age"`
}

// TenantCacheStats represents cache statistics for a tenant
type TenantCacheStats struct {
	Hits         int64     `json:"hits"`
	Misses       int64     `json:"misses"`
	Entries      int64     `json:"entries"`
	Bytes        int64     `json:"bytes"`
	LastAccessed time.Time `json:"last_accessed"`
}

// Ensure implementations satisfy interfaces
var (
	_ Cache       = (*SemanticCache)(nil)
	_ TenantCache = (*TenantAwareCache)(nil)
)

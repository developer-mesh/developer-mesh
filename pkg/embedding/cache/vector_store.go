package cache

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/eviction"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// VectorStore handles pgvector operations for semantic cache
type VectorStore struct {
	db      *sqlx.DB
	logger  observability.Logger
	metrics observability.MetricsClient
}

// NewVectorStore creates a new vector store instance
func NewVectorStore(db *sqlx.DB, logger observability.Logger, metrics observability.MetricsClient) *VectorStore {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.vector_store")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	return &VectorStore{
		db:      db,
		logger:  logger,
		metrics: metrics,
	}
}

// SimilarQuery represents a query similarity match from pgvector
type SimilarQueryResult struct {
	CacheKey       string    `db:"cache_key"`
	QueryHash      string    `db:"query_hash"`
	Similarity     float32   `db:"similarity"`
	HitCount       int       `db:"hit_count"`
	LastAccessedAt time.Time `db:"last_accessed_at"`
}

// StoreCacheEmbedding stores query embedding in pgvector
func (v *VectorStore) StoreCacheEmbedding(ctx context.Context, tenantID uuid.UUID, cacheKey, queryHash string, embedding []float32) error {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "vector_store.store_embedding")
	defer span.End()

	query := `
		INSERT INTO cache_metadata (tenant_id, cache_key, query_hash, embedding)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, cache_key) 
		DO UPDATE SET 
			embedding = EXCLUDED.embedding,
			hit_count = cache_metadata.hit_count + 1,
			last_accessed_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
	`

	startTime := time.Now()
	// Convert float32 slice to PostgreSQL array format
	_, err := v.db.ExecContext(ctx, query, tenantID, cacheKey, queryHash, pq.Array(embedding))
	duration := time.Since(startTime).Seconds()

	// Record metrics
	labels := map[string]string{
		"operation": "store_embedding",
		"status":    "success",
	}
	if err != nil {
		labels["status"] = "error"
		v.logger.Error("Failed to store embedding", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"cache_key": cacheKey,
		})
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	v.metrics.RecordHistogram("cache.vector_store.duration", duration, labels)
	return nil
}

// FindSimilarQueries finds cached queries with similar embeddings
func (v *VectorStore) FindSimilarQueries(ctx context.Context, tenantID uuid.UUID, embedding []float32, threshold float32, limit int) ([]SimilarQueryResult, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "vector_store.find_similar")
	defer span.End()

	// Use optimized query with index hints
	query := `
		SELECT 
			cache_key,
			query_hash,
			1 - (embedding <=> $2) as similarity,
			hit_count,
			last_accessed_at
		FROM cache_metadata
		WHERE tenant_id = $1
			AND is_active = true
			AND 1 - (embedding <=> $2) >= $3
		ORDER BY embedding <=> $2
		LIMIT $4
	`

	startTime := time.Now()
	rows, err := v.db.QueryContext(ctx, query, tenantID, pq.Array(embedding), threshold, limit)
	duration := time.Since(startTime).Seconds()

	// Record metrics
	labels := map[string]string{
		"operation": "find_similar",
		"status":    "success",
	}
	if err != nil {
		labels["status"] = "error"
		v.logger.Error("Vector search failed", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"threshold": threshold,
		})
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			v.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var results []SimilarQueryResult
	for rows.Next() {
		var sq SimilarQueryResult
		err := rows.Scan(&sq.CacheKey, &sq.QueryHash, &sq.Similarity, &sq.HitCount, &sq.LastAccessedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, sq)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	v.metrics.RecordHistogram("cache.vector_store.duration", duration, labels)
	v.metrics.RecordGauge("cache.vector_store.results", float64(len(results)), map[string]string{
		"tenant_id": tenantID.String(),
	})

	return results, nil
}

// UpdateAccessStats updates hit count and access time
func (v *VectorStore) UpdateAccessStats(ctx context.Context, tenantID uuid.UUID, cacheKey string) error {
	query := `
		UPDATE cache_metadata 
		SET hit_count = hit_count + 1,
			last_accessed_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = $1 AND cache_key = $2
	`

	_, err := v.db.ExecContext(ctx, query, tenantID, cacheKey)
	if err != nil {
		v.logger.Error("Failed to update access stats", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
			"cache_key": cacheKey,
		})
		return fmt.Errorf("failed to update access stats: %w", err)
	}

	return nil
}

// GetTenantCacheStats retrieves cache statistics for a tenant
func (v *VectorStore) GetTenantCacheStats(ctx context.Context, tenantID uuid.UUID) (*eviction.TenantCacheStats, error) {
	query := `
		SELECT 
			tenant_id,
			COUNT(*) as entry_count,
			COALESCE(SUM(hit_count), 0) as total_hits,
			MIN(created_at) as oldest_entry,
			MAX(created_at) as newest_entry
		FROM cache_metadata
		WHERE tenant_id = $1
		GROUP BY tenant_id
	`

	var stats eviction.TenantCacheStats
	err := v.db.GetContext(ctx, &stats, query, tenantID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return empty stats for tenant with no cache entries
			return &eviction.TenantCacheStats{
				TenantID:   tenantID,
				EntryCount: 0,
				TotalHits:  0,
			}, nil
		}
		return nil, fmt.Errorf("failed to get tenant cache stats: %w", err)
	}

	return &stats, nil
}

// GetLRUEntries retrieves the least recently used entries for a tenant
func (v *VectorStore) GetLRUEntries(ctx context.Context, tenantID uuid.UUID, limit int) ([]eviction.LRUEntry, error) {
	query := `
		SELECT 
			cache_key,
			query_hash,
			last_accessed_at,
			hit_count
		FROM cache_metadata
		WHERE tenant_id = $1
		ORDER BY last_accessed_at ASC, hit_count ASC
		LIMIT $2
	`

	var entries []eviction.LRUEntry
	err := v.db.SelectContext(ctx, &entries, query, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get LRU entries: %w", err)
	}

	return entries, nil
}

// DeleteCacheEntry deletes a cache entry from the database
func (v *VectorStore) DeleteCacheEntry(ctx context.Context, tenantID uuid.UUID, cacheKey string) error {
	query := `
		DELETE FROM cache_metadata
		WHERE tenant_id = $1 AND cache_key = $2
	`

	result, err := v.db.ExecContext(ctx, query, tenantID, cacheKey)
	if err != nil {
		return fmt.Errorf("failed to delete cache entry: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		v.logger.Warn("No cache entry found to delete", map[string]interface{}{
			"tenant_id": tenantID.String(),
			"cache_key": cacheKey,
		})
	}

	return nil
}

// GetTenantsWithCache retrieves all tenant IDs that have cache entries
func (v *VectorStore) GetTenantsWithCache(ctx context.Context) ([]uuid.UUID, error) {
	query := `
		SELECT DISTINCT tenant_id
		FROM cache_metadata
		ORDER BY tenant_id
	`

	var tenantIDs []uuid.UUID
	err := v.db.SelectContext(ctx, &tenantIDs, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenants with cache: %w", err)
	}

	return tenantIDs, nil
}

// CleanupStaleEntries removes entries older than the specified duration
func (v *VectorStore) CleanupStaleEntries(ctx context.Context, staleDuration time.Duration) (int64, error) {
	query := `
		DELETE FROM cache_metadata
		WHERE last_accessed_at < $1
	`

	cutoffTime := time.Now().Add(-staleDuration)
	result, err := v.db.ExecContext(ctx, query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup stale entries: %w", err)
	}

	rowsDeleted, _ := result.RowsAffected()

	if rowsDeleted > 0 {
		v.logger.Info("Cleaned up stale cache entries", map[string]interface{}{
			"rows_deleted": rowsDeleted,
			"cutoff_time":  cutoffTime,
		})

		v.metrics.IncrementCounterWithLabels("cache.cleanup.stale_entries", float64(rowsDeleted), map[string]string{
			"type": "stale",
		})
	}

	return rowsDeleted, nil
}

// GetGlobalCacheStats returns overall cache statistics
func (v *VectorStore) GetGlobalCacheStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(DISTINCT tenant_id) as tenant_count,
			COUNT(*) as total_entries,
			COALESCE(SUM(hit_count), 0) as total_hits,
			COALESCE(AVG(hit_count), 0) as avg_hits_per_entry,
			MIN(created_at) as oldest_entry,
			MAX(created_at) as newest_entry,
			MIN(last_accessed_at) as least_recently_accessed
		FROM cache_metadata
	`

	var stats struct {
		TenantCount           int       `db:"tenant_count"`
		TotalEntries          int       `db:"total_entries"`
		TotalHits             int64     `db:"total_hits"`
		AvgHitsPerEntry       float64   `db:"avg_hits_per_entry"`
		OldestEntry           time.Time `db:"oldest_entry"`
		NewestEntry           time.Time `db:"newest_entry"`
		LeastRecentlyAccessed time.Time `db:"least_recently_accessed"`
	}

	err := v.db.GetContext(ctx, &stats, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]interface{}{
				"tenant_count":       0,
				"total_entries":      0,
				"total_hits":         0,
				"avg_hits_per_entry": 0,
			}, nil
		}
		return nil, fmt.Errorf("failed to get global cache stats: %w", err)
	}

	return map[string]interface{}{
		"tenant_count":            stats.TenantCount,
		"total_entries":           stats.TotalEntries,
		"total_hits":              stats.TotalHits,
		"avg_hits_per_entry":      stats.AvgHitsPerEntry,
		"oldest_entry":            stats.OldestEntry,
		"newest_entry":            stats.NewestEntry,
		"least_recently_accessed": stats.LeastRecentlyAccessed,
	}, nil
}

// OptimizeIndexes performs maintenance on pgvector indexes
func (v *VectorStore) OptimizeIndexes(ctx context.Context) error {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "vector_store.optimize_indexes")
	defer span.End()

	v.logger.Info("Starting index optimization", nil)

	// Vacuum and analyze for better performance
	queries := []string{
		"VACUUM ANALYZE cache_metadata",
		"REINDEX INDEX CONCURRENTLY idx_cache_metadata_embedding_vector",
		"REINDEX INDEX CONCURRENTLY idx_cache_metadata_tenant_embedding",
		"REINDEX INDEX CONCURRENTLY idx_cache_metadata_tenant_accessed",
		"REINDEX INDEX CONCURRENTLY idx_cache_metadata_tenant_hits",
	}

	for _, query := range queries {
		startTime := time.Now()
		if _, err := v.db.ExecContext(ctx, query); err != nil {
			v.logger.Error("Failed to optimize index", map[string]interface{}{
				"query": query,
				"error": err.Error(),
			})
			// Continue with other optimizations
			continue
		}

		v.metrics.RecordHistogram("cache.vector_store.optimize_duration", time.Since(startTime).Seconds(), map[string]string{
			"operation": "optimize_index",
		})
	}

	v.logger.Info("Index optimization completed", nil)
	return nil
}

// HealthCheck verifies the vector store is functioning properly
func (v *VectorStore) HealthCheck(ctx context.Context) error {
	// Simple query to verify connectivity
	var result int
	err := v.db.GetContext(ctx, &result, "SELECT 1")
	if err != nil {
		return fmt.Errorf("vector store health check failed: %w", err)
	}

	// Check if pgvector extension is available
	var extensionExists bool
	err = v.db.GetContext(ctx, &extensionExists,
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')")
	if err != nil {
		return fmt.Errorf("failed to check pgvector extension: %w", err)
	}

	if !extensionExists {
		return fmt.Errorf("pgvector extension is not installed")
	}

	return nil
}

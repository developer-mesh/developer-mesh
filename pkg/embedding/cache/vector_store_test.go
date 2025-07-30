package cache_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

func setupMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	db := sqlx.NewDb(mockDB, "postgres")
	return db, mock
}

func TestVectorStore_StoreCacheEmbedding(t *testing.T) {
	db, mock := setupMockDB(t)
	defer func() { _ = db.Close() }()

	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	store := cache.NewVectorStore(db, logger, metrics)

	tenantID := uuid.New()
	cacheKey := "test_key"
	queryHash := "test_hash"
	embedding := []float32{0.1, 0.2, 0.3}

	// Mock the INSERT/UPDATE query
	mock.ExpectExec("INSERT INTO cache_metadata").
		WithArgs(tenantID, cacheKey, queryHash, pq.Array(embedding)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := store.StoreCacheEmbedding(context.Background(), tenantID, cacheKey, queryHash, embedding)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestVectorStore_FindSimilarQueries(t *testing.T) {
	db, mock := setupMockDB(t)
	defer func() { _ = db.Close() }()

	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	store := cache.NewVectorStore(db, logger, metrics)

	tenantID := uuid.New()
	embedding := []float32{0.1, 0.2, 0.3}
	threshold := float32(0.8)
	limit := 10

	// Mock query results
	rows := sqlmock.NewRows([]string{"cache_key", "query_hash", "similarity", "hit_count", "last_accessed_at"}).
		AddRow("key1", "hash1", 0.95, 10, time.Now()).
		AddRow("key2", "hash2", 0.85, 5, time.Now())

	mock.ExpectQuery("SELECT").
		WithArgs(tenantID, pq.Array(embedding), threshold, limit).
		WillReturnRows(rows)

	results, err := store.FindSimilarQueries(context.Background(), tenantID, embedding, threshold, limit)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "key1", results[0].CacheKey)
	assert.Equal(t, float32(0.95), results[0].Similarity)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestVectorStore_GetTenantCacheStats(t *testing.T) {
	db, mock := setupMockDB(t)
	defer func() { _ = db.Close() }()

	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	store := cache.NewVectorStore(db, logger, metrics)

	tenantID := uuid.New()

	// Test with data
	rows := sqlmock.NewRows([]string{"tenant_id", "entry_count", "total_hits", "oldest_entry", "newest_entry"}).
		AddRow(tenantID, 100, 500, time.Now().Add(-24*time.Hour), time.Now())

	mock.ExpectQuery("SELECT").
		WithArgs(tenantID).
		WillReturnRows(rows)

	stats, err := store.GetTenantCacheStats(context.Background(), tenantID)
	assert.NoError(t, err)
	assert.Equal(t, 100, stats.EntryCount)
	assert.Equal(t, 500, stats.TotalHits)

	// Test with no data
	mock.ExpectQuery("SELECT").
		WithArgs(tenantID).
		WillReturnError(sql.ErrNoRows)

	stats, err = store.GetTenantCacheStats(context.Background(), tenantID)
	assert.NoError(t, err)
	assert.Equal(t, 0, stats.EntryCount)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestVectorStore_GetLRUEntries(t *testing.T) {
	db, mock := setupMockDB(t)
	defer func() { _ = db.Close() }()

	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	store := cache.NewVectorStore(db, logger, metrics)

	tenantID := uuid.New()
	limit := 5

	rows := sqlmock.NewRows([]string{"cache_key", "query_hash", "last_accessed_at", "hit_count"}).
		AddRow("key1", "hash1", time.Now().Add(-2*time.Hour), 1).
		AddRow("key2", "hash2", time.Now().Add(-1*time.Hour), 2)

	mock.ExpectQuery("SELECT").
		WithArgs(tenantID, limit).
		WillReturnRows(rows)

	entries, err := store.GetLRUEntries(context.Background(), tenantID, limit)
	assert.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "key1", entries[0].CacheKey)
	assert.Equal(t, 1, entries[0].HitCount)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestVectorStore_DeleteCacheEntry(t *testing.T) {
	db, mock := setupMockDB(t)
	defer func() { _ = db.Close() }()

	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	store := cache.NewVectorStore(db, logger, metrics)

	tenantID := uuid.New()
	cacheKey := "test_key"

	mock.ExpectExec("DELETE FROM cache_metadata").
		WithArgs(tenantID, cacheKey).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.DeleteCacheEntry(context.Background(), tenantID, cacheKey)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestVectorStore_CleanupStaleEntries(t *testing.T) {
	db, mock := setupMockDB(t)
	defer func() { _ = db.Close() }()

	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	store := cache.NewVectorStore(db, logger, metrics)

	staleDuration := 24 * time.Hour

	mock.ExpectExec("DELETE FROM cache_metadata").
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 10))

	deleted, err := store.CleanupStaleEntries(context.Background(), staleDuration)
	assert.NoError(t, err)
	assert.Equal(t, int64(10), deleted)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

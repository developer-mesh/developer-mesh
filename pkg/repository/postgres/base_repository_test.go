package postgres

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
)

// mockCache implements cache.Cache for testing
type mockCache struct {
	data     map[string]interface{}
	mu       sync.RWMutex
	getErr   error
	setErr   error
	delErr   error
	getCalls int
	setCalls int
	delCalls int
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string]interface{}),
	}
}

func (m *mockCache) Get(ctx context.Context, key string, dest interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls++

	if m.getErr != nil {
		return m.getErr
	}

	val, exists := m.data[key]
	if !exists {
		return cache.ErrNotFound
	}

	// Simple type assertion for testing
	switch d := dest.(type) {
	case *string:
		*d = val.(string)
	case *int:
		*d = val.(int)
	}

	return nil
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalls++

	if m.setErr != nil {
		return m.setErr
	}

	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delCalls++

	if m.delErr != nil {
		return m.delErr
	}

	delete(m.data, key)
	return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.data[key]
	return exists, nil
}

func (m *mockCache) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]interface{})
	return nil
}

func (m *mockCache) Close() error {
	return nil
}

// Size returns the number of items in the cache
func (m *mockCache) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// mockMetricsClient implements observability.MetricsClient for testing
type mockMetricsClient struct {
	counters map[string]float64
	timers   map[string]time.Duration
	mu       sync.Mutex
}

func newMockMetricsClient() *mockMetricsClient {
	return &mockMetricsClient{
		counters: make(map[string]float64),
		timers:   make(map[string]time.Duration),
	}
}

func (m *mockMetricsClient) RecordEvent(source, eventType string)                                 {}
func (m *mockMetricsClient) RecordLatency(operation string, duration time.Duration)               {}
func (m *mockMetricsClient) RecordCounter(name string, value float64, labels map[string]string)   {}
func (m *mockMetricsClient) RecordGauge(name string, value float64, labels map[string]string)     {}
func (m *mockMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {}
func (m *mockMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {
}
func (m *mockMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
}
func (m *mockMetricsClient) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) StartTimer(name string, labels map[string]string) func() {
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.timers[name] = time.Since(time.Now())
	}
}
func (m *mockMetricsClient) IncrementCounter(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
}
func (m *mockMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
	m.IncrementCounter(name, value)
}
func (m *mockMetricsClient) RecordDuration(name string, duration time.Duration) {}
func (m *mockMetricsClient) Close() error                                       { return nil }

func setupBaseRepository(t *testing.T) (*BaseRepository, sqlmock.Sqlmock, *mockCache, *mockMetricsClient) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	sqlxDB := sqlx.NewDb(db, "postgres")

	cache := newMockCache()
	logger := observability.NewStandardLogger("test")
	tracer := observability.NoopStartSpan
	metrics := newMockMetricsClient()

	config := BaseRepositoryConfig{
		QueryTimeout: 5 * time.Second,
		MaxRetries:   3,
		CacheTimeout: 5 * time.Minute,
	}

	repo := NewBaseRepository(sqlxDB, sqlxDB, cache, logger, tracer, metrics, config)

	return repo, mock, cache, metrics
}

func TestBaseRepository_WithTransaction(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(sqlmock.Sqlmock)
		fn           func(tx *sqlx.Tx) error
		wantErr      bool
		checkMetrics func(*testing.T, *mockMetricsClient)
	}{
		{
			name: "successful transaction",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO test").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			fn: func(tx *sqlx.Tx) error {
				_, err := tx.Exec("INSERT INTO test VALUES (1)")
				return err
			},
			wantErr: false,
			checkMetrics: func(t *testing.T, m *mockMetricsClient) {
				assert.Equal(t, float64(1), m.counters["repository_transaction_commits"])
			},
		},
		{
			name: "failed transaction",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO test").WillReturnError(errors.New("insert failed"))
				mock.ExpectRollback()
			},
			fn: func(tx *sqlx.Tx) error {
				_, err := tx.Exec("INSERT INTO test VALUES (1)")
				return err
			},
			wantErr: true,
			checkMetrics: func(t *testing.T, m *mockMetricsClient) {
				assert.Equal(t, float64(1), m.counters["repository_transaction_rollbacks"])
			},
		},
		{
			name: "begin transaction error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("begin failed"))
			},
			fn: func(tx *sqlx.Tx) error {
				return nil
			},
			wantErr: true,
			checkMetrics: func(t *testing.T, m *mockMetricsClient) {
				assert.Equal(t, float64(1), m.counters["repository_transaction_errors"])
			},
		},
		{
			name: "commit error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO test").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
			},
			fn: func(tx *sqlx.Tx) error {
				_, err := tx.Exec("INSERT INTO test VALUES (1)")
				return err
			},
			wantErr: true,
			checkMetrics: func(t *testing.T, m *mockMetricsClient) {
				assert.Equal(t, float64(1), m.counters["repository_transaction_errors"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, _, metrics := setupBaseRepository(t)
			defer func() { _ = repo.writeDB.Close() }()

			tt.setupMock(mock)

			err := repo.WithTransaction(context.Background(), tt.fn)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.checkMetrics != nil {
				tt.checkMetrics(t, metrics)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBaseRepository_WithTransactionOptions(t *testing.T) {
	repo, mock, _, _ := setupBaseRepository(t)
	defer func() { _ = repo.writeDB.Close() }()

	opts := &types.TxOptions{
		Isolation: types.IsolationSerializable,
		ReadOnly:  true,
	}

	mock.ExpectBegin()
	mock.ExpectExec("SELECT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.WithTransactionOptions(context.Background(), opts, func(tx *sqlx.Tx) error {
		_, err := tx.Exec("SELECT 1")
		return err
	})

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBaseRepository_CacheOperations(t *testing.T) {
	t.Run("CacheGet", func(t *testing.T) {
		repo, _, mockCache, metrics := setupBaseRepository(t)
		defer func() { _ = repo.writeDB.Close() }()

		// Test cache hit
		mockCache.data["test-key"] = "test-value"
		var result string
		err := repo.CacheGet(context.Background(), "test-key", &result)
		assert.NoError(t, err)
		assert.Equal(t, "test-value", result)
		assert.Equal(t, float64(1), metrics.counters["repository_cache_operations"])

		// Test cache miss
		err = repo.CacheGet(context.Background(), "missing-key", &result)
		assert.Equal(t, cache.ErrNotFound, err)
		assert.Equal(t, float64(2), metrics.counters["repository_cache_operations"])

		// Test cache error
		mockCache.getErr = errors.New("cache error")
		err = repo.CacheGet(context.Background(), "test-key", &result)
		assert.Error(t, err)
		assert.Equal(t, float64(3), metrics.counters["repository_cache_operations"])
	})

	t.Run("CacheSet", func(t *testing.T) {
		repo, _, mockCache, metrics := setupBaseRepository(t)
		defer func() { _ = repo.writeDB.Close() }()

		// Test successful set
		err := repo.CacheSet(context.Background(), "new-key", "new-value", time.Minute)
		assert.NoError(t, err)
		assert.Equal(t, "new-value", mockCache.data["new-key"])
		assert.Equal(t, float64(1), metrics.counters["repository_cache_operations"])

		// Test set error
		mockCache.setErr = errors.New("set error")
		err = repo.CacheSet(context.Background(), "error-key", "value", 0)
		assert.Error(t, err)
		assert.Equal(t, float64(2), metrics.counters["repository_cache_operations"])
	})

	t.Run("CacheDelete", func(t *testing.T) {
		repo, _, mockCache, metrics := setupBaseRepository(t)
		defer func() { _ = repo.writeDB.Close() }()

		// Test successful delete
		mockCache.data["delete-key"] = "value"
		err := repo.CacheDelete(context.Background(), "delete-key")
		assert.NoError(t, err)
		_, exists := mockCache.data["delete-key"]
		assert.False(t, exists)
		assert.Equal(t, float64(1), metrics.counters["repository_cache_operations"])

		// Test delete error
		mockCache.delErr = errors.New("delete error")
		err = repo.CacheDelete(context.Background(), "error-key")
		assert.Error(t, err)
		assert.Equal(t, float64(2), metrics.counters["repository_cache_operations"])
	})
}

func TestBaseRepository_TranslateError(t *testing.T) {
	repo, _, _, _ := setupBaseRepository(t)
	defer func() { _ = repo.writeDB.Close() }()

	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: nil,
		},
		{
			name:     "sql.ErrNoRows",
			err:      sql.ErrNoRows,
			expected: interfaces.ErrNotFound,
		},
		{
			name:     "unique violation",
			err:      &pq.Error{Code: "23505"},
			expected: interfaces.ErrDuplicate,
		},
		{
			name:     "foreign key violation",
			err:      &pq.Error{Code: "23503"},
			expected: errors.Wrap(interfaces.ErrValidation, "foreign key constraint violation"),
		},
		{
			name:     "not null violation",
			err:      &pq.Error{Code: "23502"},
			expected: errors.Wrap(interfaces.ErrValidation, "required field missing"),
		},
		{
			name:     "check violation with constraint name",
			err:      &pq.Error{Code: "23514", Constraint: "workflows_valid_steps_check"},
			expected: errors.Wrap(interfaces.ErrValidation, "check constraint violation: workflows_valid_steps_check"),
		},
		{
			name:     "check violation without constraint name",
			err:      &pq.Error{Code: "23514"},
			expected: errors.Wrap(interfaces.ErrValidation, "check constraint violation: "),
		},
		{
			name:     "serialization failure",
			err:      &pq.Error{Code: "40001"},
			expected: interfaces.ErrOptimisticLock,
		},
		{
			name:     "unknown error",
			err:      errors.New("unknown error"),
			expected: errors.New("database error for test: unknown error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repo.TranslateError(tt.err, "test")

			if tt.expected == nil {
				assert.Nil(t, result)
			} else if errors.Is(tt.expected, interfaces.ErrValidation) {
				assert.True(t, errors.Is(result, interfaces.ErrValidation))
			} else {
				assert.Equal(t, tt.expected.Error(), result.Error())
			}
		})
	}
}

func TestBaseRepository_ExecuteWithCircuitBreaker(t *testing.T) {
	t.Run("without circuit breaker", func(t *testing.T) {
		repo, _, _, _ := setupBaseRepository(t)
		defer func() { _ = repo.writeDB.Close() }()

		called := false
		result, err := repo.ExecuteWithCircuitBreaker(context.Background(), "test", func() (interface{}, error) {
			called = true
			return "success", nil
		})

		assert.True(t, called)
		assert.NoError(t, err)
		assert.Equal(t, "success", result)
	})

	t.Run("with circuit breaker", func(t *testing.T) {
		repo, _, _, metrics := setupBaseRepository(t)
		defer func() { _ = repo.writeDB.Close() }()

		// Create a circuit breaker
		cbConfig := resilience.CircuitBreakerConfig{
			FailureThreshold: 3,
			ResetTimeout:     time.Second,
		}
		cb := resilience.NewCircuitBreaker("test", cbConfig, observability.NewStandardLogger("test"), metrics)
		repo.cb = cb

		// Test successful execution
		result, err := repo.ExecuteWithCircuitBreaker(context.Background(), "test-op", func() (interface{}, error) {
			return "cb-success", nil
		})

		assert.NoError(t, err)
		assert.Equal(t, "cb-success", result)

		// Test failed execution
		result, err = repo.ExecuteWithCircuitBreaker(context.Background(), "test-op", func() (interface{}, error) {
			return nil, errors.New("cb-error")
		})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, float64(1), metrics.counters["repository_circuit_breaker_errors"])
	})
}

func TestBaseRepository_ExecuteQuery(t *testing.T) {
	repo, _, _, metrics := setupBaseRepository(t)
	defer func() { _ = repo.writeDB.Close() }()

	t.Run("successful query", func(t *testing.T) {
		err := repo.ExecuteQuery(context.Background(), "test-query", func(ctx context.Context) error {
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, float64(1), metrics.counters["repository_query_success"])
	})

	t.Run("failed query", func(t *testing.T) {
		err := repo.ExecuteQuery(context.Background(), "test-query", func(ctx context.Context) error {
			return errors.New("query failed")
		})

		assert.Error(t, err)
		assert.Equal(t, float64(1), metrics.counters["repository_query_errors"])
	})

	t.Run("query timeout", func(t *testing.T) {
		repo.queryTimeout = 10 * time.Millisecond

		err := repo.ExecuteQuery(context.Background(), "test-query", func(ctx context.Context) error {
			select {
			case <-time.After(100 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
	})
}

func TestBaseRepository_ExecuteQueryWithRetry(t *testing.T) {
	repo, _, _, _ := setupBaseRepository(t)
	defer func() { _ = repo.writeDB.Close() }()
	repo.maxRetries = 3

	t.Run("successful on first attempt", func(t *testing.T) {
		attempts := 0
		err := repo.ExecuteQueryWithRetry(context.Background(), "test-query", func(ctx context.Context) error {
			attempts++
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("successful after retry", func(t *testing.T) {
		attempts := 0
		err := repo.ExecuteQueryWithRetry(context.Background(), "test-query", func(ctx context.Context) error {
			attempts++
			if attempts < 2 {
				return errors.New("transient error")
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 2, attempts)
	})

	t.Run("non-retryable errors", func(t *testing.T) {
		attempts := 0
		err := repo.ExecuteQueryWithRetry(context.Background(), "test-query", func(ctx context.Context) error {
			attempts++
			return interfaces.ErrNotFound
		})

		assert.Equal(t, interfaces.ErrNotFound, err)
		assert.Equal(t, 1, attempts) // Should not retry
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		attempts := 0
		err := repo.ExecuteQueryWithRetry(context.Background(), "test-query", func(ctx context.Context) error {
			attempts++
			return errors.New("persistent error")
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query failed after 3 attempts")
		assert.Equal(t, 3, attempts)
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0

		err := repo.ExecuteQueryWithRetry(ctx, "test-query", func(ctx context.Context) error {
			attempts++
			if attempts == 1 {
				cancel() // Cancel after first attempt
			}
			return errors.New("error")
		})

		assert.Equal(t, context.Canceled, err)
		assert.Equal(t, 1, attempts) // Should stop after context cancelled
	})
}

func TestBaseRepository_GetPreparedStatement(t *testing.T) {
	repo, mock, _, _ := setupBaseRepository(t)
	defer func() { _ = repo.writeDB.Close() }()

	query := "SELECT * FROM test WHERE id = :id"

	// Mock the prepared statement
	mock.ExpectPrepare("SELECT")

	// First call should prepare the statement
	stmt1, err := repo.GetPreparedStatement("test-stmt", query, repo.writeDB)
	assert.NoError(t, err)
	assert.NotNil(t, stmt1)
	assert.Len(t, repo.stmtCache, 1)

	// Second call should return cached statement
	stmt2, err := repo.GetPreparedStatement("test-stmt", query, repo.writeDB)
	assert.NoError(t, err)
	assert.Equal(t, stmt1, stmt2)
	assert.Len(t, repo.stmtCache, 1)
}

func TestBaseRepository_GetPreparedStatementConcurrent(t *testing.T) {
	repo, mock, _, _ := setupBaseRepository(t)
	defer func() { _ = repo.writeDB.Close() }()

	query := "SELECT * FROM test WHERE id = :id"
	mock.ExpectPrepare("SELECT")

	var wg sync.WaitGroup
	errors := make([]error, 10)
	statements := make([]*sqlx.NamedStmt, 10)

	// Run concurrent GetPreparedStatement calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			stmt, err := repo.GetPreparedStatement("concurrent-stmt", query, repo.writeDB)
			errors[idx] = err
			statements[idx] = stmt
		}(i)
	}

	wg.Wait()

	// Check all calls succeeded
	for i, err := range errors {
		assert.NoError(t, err, "Call %d failed", i)
		assert.NotNil(t, statements[i], "Statement %d is nil", i)
	}

	// Check only one statement was created
	assert.Len(t, repo.stmtCache, 1)

	// Check all returned the same statement
	for i := 1; i < 10; i++ {
		assert.Equal(t, statements[0], statements[i], "Statement %d differs from statement 0", i)
	}
}

func TestBaseRepository_Close(t *testing.T) {
	repo, mock, _, _ := setupBaseRepository(t)
	defer func() { _ = repo.writeDB.Close() }()

	// Add some prepared statements
	query1 := "SELECT * FROM test1"
	query2 := "SELECT * FROM test2"

	mock.ExpectPrepare("SELECT \\* FROM test1")
	mock.ExpectPrepare("SELECT \\* FROM test2")

	stmt1, err := repo.GetPreparedStatement("stmt1", query1, repo.writeDB)
	require.NoError(t, err)
	stmt2, err := repo.GetPreparedStatement("stmt2", query2, repo.writeDB)
	require.NoError(t, err)

	assert.Len(t, repo.stmtCache, 2)

	// Close the repository
	err = repo.Close()
	assert.NoError(t, err)
	assert.Len(t, repo.stmtCache, 0)

	// Verify statements are closed by checking they're no longer in cache
	assert.NotContains(t, repo.stmtCache, stmt1)
	assert.NotContains(t, repo.stmtCache, stmt2)
}

func TestBaseRepository_InvalidateCachePattern(t *testing.T) {
	repo, _, _, metrics := setupBaseRepository(t)
	defer func() { _ = repo.writeDB.Close() }()

	err := repo.InvalidateCachePattern(context.Background(), "user:*")
	assert.NoError(t, err)
	assert.Equal(t, float64(1), metrics.counters["repository_cache_invalidations"])
}

func TestBaseRepository_GetMetrics(t *testing.T) {
	repo, mock, _, _ := setupBaseRepository(t)
	defer func() { _ = repo.writeDB.Close() }()

	// Add some prepared statements
	mock.ExpectPrepare("SELECT")
	_, err := repo.GetPreparedStatement("test-stmt", "SELECT * FROM test", repo.writeDB)
	require.NoError(t, err)

	metrics := repo.GetMetrics()
	assert.Equal(t, 1, metrics["prepared_statements"])
}

func TestClassifyDBError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil error", nil, "none"},
		{"not found", sql.ErrNoRows, "not_found"},
		{"duplicate", interfaces.ErrDuplicate, "duplicate"},
		{"validation", interfaces.ErrValidation, "validation"},
		{"optimistic lock", interfaces.ErrOptimisticLock, "optimistic_lock"},
		{"timeout", context.DeadlineExceeded, "timeout"},
		{"cancelled", context.Canceled, "cancelled"},
		{"pq error", &pq.Error{Code: "23505"}, "23505"},
		{"unknown", errors.New("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyDBError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark tests
func BenchmarkBaseRepository_CacheGet(b *testing.B) {
	repo, _, mockCache, _ := setupBaseRepository(&testing.T{})
	defer func() { _ = repo.writeDB.Close() }()

	mockCache.data["bench-key"] = "bench-value"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result string
		_ = repo.CacheGet(ctx, "bench-key", &result)
	}
}

func BenchmarkBaseRepository_WithTransaction(b *testing.B) {
	db, mock, err := sqlmock.New()
	require.NoError(b, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewBaseRepository(
		sqlxDB, sqlxDB,
		newMockCache(),
		observability.NewStandardLogger("bench"),
		observability.NoopStartSpan,
		newMockMetricsClient(),
		BaseRepositoryConfig{},
	)

	// Set up expectations for all iterations
	for i := 0; i < b.N; i++ {
		mock.ExpectBegin()
		mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = repo.WithTransaction(ctx, func(tx *sqlx.Tx) error {
			_, err := tx.Exec("INSERT INTO bench VALUES (1)")
			return err
		})
	}
}

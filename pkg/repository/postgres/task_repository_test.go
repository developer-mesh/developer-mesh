package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/postgres"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
)

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

func TestTaskRepository_Create(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")

	// Create mock cache
	mockCache := &mockCache{}

	// Create mock metrics client
	metrics := newMockMetricsClient()

	// Create repository
	repo := postgres.NewTaskRepository(
		sqlxDB,
		sqlxDB, // Use same DB for read/write in tests
		mockCache,
		observability.NewNoopLogger(),
		observability.NoopStartSpan,
		metrics,
	)

	ctx := context.Background()
	task := &models.Task{
		TenantID:       uuid.New(),
		Type:           "test",
		Status:         models.TaskStatusPending,
		Priority:       models.TaskPriorityNormal,
		CreatedBy:      "test-agent",
		Title:          "Test Task",
		Description:    "Test Description",
		Parameters:     models.JSONMap{"key": "value"},
		MaxRetries:     3,
		TimeoutSeconds: 3600,
	}

	// Expect the query
	mock.ExpectPrepare("INSERT INTO tasks")
	mock.ExpectQuery("INSERT INTO tasks").
		WithArgs(
			sqlmock.AnyArg(), // ID
			task.TenantID,
			task.Type,
			task.Status,
			task.Priority,
			task.CreatedBy,
			nil, // AssignedTo
			nil, // ParentTaskID
			task.Title,
			task.Description,
			sqlmock.AnyArg(), // Parameters (JSON)
			task.MaxRetries,
			task.TimeoutSeconds,
			sqlmock.AnyArg(), // CreatedAt
			sqlmock.AnyArg(), // UpdatedAt
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(task.ID))

	// Create task
	err = repo.Create(ctx, task)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, task.ID)
	assert.Equal(t, 1, task.Version)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestTaskRepository_Get(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")

	// Create mock cache
	mockCache := &mockCache{
		getErr: cache.ErrNotFound,
	}

	// Create repository
	repo := postgres.NewTaskRepository(
		sqlxDB,
		sqlxDB,
		mockCache,
		observability.NewNoopLogger(),
		observability.NoopStartSpan,
		newMockMetricsClient(),
	)

	ctx := context.Background()
	taskID := uuid.New()

	// Expect the query
	columns := []string{
		"id", "tenant_id", "type", "status", "priority",
		"created_by", "assigned_to", "parent_task_id",
		"title", "description", "parameters", "result", "error",
		"max_retries", "retry_count", "timeout_seconds",
		"created_at", "assigned_at", "started_at", "completed_at",
		"updated_at", "deleted_at", "version",
	}

	mock.ExpectQuery("SELECT .* FROM tasks WHERE id = \\$1").
		WithArgs(taskID).
		WillReturnRows(
			sqlmock.NewRows(columns).
				AddRow(
					taskID,
					uuid.New(),
					"test",
					"pending",
					"normal",
					"test-agent",
					nil,
					nil,
					"Test Task",
					"Test Description",
					`{"key": "value"}`,
					nil,
					"", // error field - empty string instead of nil
					3,
					0,
					3600,
					time.Now(),
					nil,
					nil,
					nil,
					time.Now(),
					nil,
					1,
				),
		)

	// Get task
	task, err := repo.Get(ctx, taskID)
	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, taskID, task.ID)
	assert.Equal(t, "Test Task", task.Title)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestTaskRepository_UpdateWithVersion(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")

	// Create mock cache
	mockCache := &mockCache{}

	// Create repository
	repo := postgres.NewTaskRepository(
		sqlxDB,
		sqlxDB,
		mockCache,
		observability.NewNoopLogger(),
		observability.NoopStartSpan,
		newMockMetricsClient(),
	)

	ctx := context.Background()
	task := &models.Task{
		ID:             uuid.New(),
		TenantID:       uuid.New(),
		Type:           "test",
		Status:         models.TaskStatusInProgress,
		Priority:       models.TaskPriorityHigh,
		CreatedBy:      "test-agent",
		Title:          "Updated Task",
		Description:    "Updated Description",
		Parameters:     models.JSONMap{"key": "updated"},
		MaxRetries:     3,
		TimeoutSeconds: 3600,
		Version:        1,
	}

	// Expect the update query
	mock.ExpectPrepare("UPDATE tasks SET")
	mock.ExpectExec("UPDATE tasks SET").
		WithArgs(
			task.Type,
			task.Status,
			task.Priority,
			task.AssignedTo,
			task.Title,
			task.Description,
			sqlmock.AnyArg(), // Parameters (JSON)
			sqlmock.AnyArg(), // Result (JSON)
			task.Error,
			task.RetryCount,
			task.AssignedAt,
			task.StartedAt,
			task.CompletedAt,
			sqlmock.AnyArg(), // UpdatedAt
			2,                // New version
			task.ID,
			1, // Expected version
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Update task
	err = repo.UpdateWithVersion(ctx, task, 1)
	assert.NoError(t, err)
	assert.Equal(t, 2, task.Version)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestTaskRepository_Transaction(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")

	// Create mock cache
	mockCache := &mockCache{}

	// Create repository
	repo := postgres.NewTaskRepository(
		sqlxDB,
		sqlxDB,
		mockCache,
		observability.NewNoopLogger(),
		observability.NoopStartSpan,
		newMockMetricsClient(),
	)

	ctx := context.Background()

	// Expect transaction operations
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL statement_timeout =").
		WithArgs(5000).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	// Begin transaction
	tx, err := repo.BeginTx(ctx, &types.TxOptions{
		Isolation: types.IsolationSerializable,
		Timeout:   5 * time.Second,
	})
	require.NoError(t, err)

	// Commit transaction
	err = tx.Commit()
	assert.NoError(t, err)

	// Verify expectations
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

// Mock cache implementation
type mockCache struct {
	getErr error
	data   map[string]interface{}
}

func (m *mockCache) Get(ctx context.Context, key string, value interface{}) error {
	if m.getErr != nil {
		return m.getErr
	}
	if m.data != nil {
		if v, ok := m.data[key]; ok {
			// In real implementation, would unmarshal into value
			// For testing, we just simulate success
			_ = v
			return nil
		}
	}
	return cache.ErrNotFound
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	if m.data != nil {
		delete(m.data, key)
	}
	return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	if m.data != nil {
		_, ok := m.data[key]
		return ok, nil
	}
	return false, nil
}

func (m *mockCache) Flush(ctx context.Context) error {
	m.data = make(map[string]interface{})
	return nil
}

func (m *mockCache) Close() error {
	return nil
}

// Size returns the number of items in the cache
func (m *mockCache) Size() int {
	return len(m.data)
}

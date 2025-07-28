package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockQueueClientDLQ struct {
	mock.Mock
}

func (m *mockQueueClientDLQ) EnqueueEvent(ctx context.Context, event queue.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *mockQueueClientDLQ) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockQueueClientDLQ) GetQueueDepth(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockQueueClientDLQ) ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]queue.Event, []string, error) {
	args := m.Called(ctx, maxMessages, waitSeconds)
	return args.Get(0).([]queue.Event), args.Get(1).([]string), args.Error(2)
}

func (m *mockQueueClientDLQ) DeleteMessage(ctx context.Context, receiptHandle string) error {
	args := m.Called(ctx, receiptHandle)
	return args.Error(0)
}

func (m *mockQueueClientDLQ) Close() error {
	return nil
}

func TestDLQHandler_SendToDLQ(t *testing.T) {
	// Setup
	ctx := context.Background()

	// Create mock database
	mockDB, sqlMock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	db := sqlx.NewDb(mockDB, "postgres")

	// Create test event
	event := queue.Event{
		EventID:   "test-123",
		EventType: "push",
		Payload:   json.RawMessage(`{"test": "data"}`),
		Metadata: map[string]interface{}{
			"tool_id": "tool-123",
		},
	}

	testErr := errors.New("processing failed")

	// Set up expectations
	metadataJSON, _ := json.Marshal(event.Metadata)
	sqlMock.ExpectQuery(`INSERT INTO webhook_dlq`).
		WithArgs(
			event.EventID,
			event.EventType,
			event.Payload,
			testErr.Error(),
			0,
			sqlmock.AnyArg(), // created_at
			"pending",
			metadataJSON, // metadata as JSON
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("dlq-123"))

	// Create handler
	logger := observability.NewNoopLogger()
	metrics := observability.NewMetricsClient()
	handler := &DLQHandlerImpl{
		db:        db,
		logger:    logger,
		metrics:   metrics,
		eventRepo: nil, // We'll set this to nil for this test since we're not testing it
	}

	// Execute
	err = handler.SendToDLQ(ctx, event, testErr)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestDLQHandler_ProcessDLQ(t *testing.T) {
	// Setup
	ctx := context.Background()

	// Create mock database
	mockDB, sqlMock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	db := sqlx.NewDb(mockDB, "postgres")

	// Create mock queue client
	mockQueue := &mockQueueClientDLQ{}

	// Set up expectations for fetching DLQ entries
	rows := sqlmock.NewRows([]string{
		"id", "event_id", "event_type", "payload", "error_message",
		"retry_count", "last_retry_at", "created_at", "status", "metadata",
	}).AddRow(
		"dlq-123", "event-123", "push", json.RawMessage(`{"test": "data"}`), "error",
		1, nil, time.Now(), "pending", json.RawMessage(`{}`),
	)

	sqlMock.ExpectQuery(`SELECT .* FROM webhook_dlq WHERE status = 'pending'`).
		WillReturnRows(rows)

	// Expect update to retrying status
	sqlMock.ExpectExec(`UPDATE webhook_dlq SET status = 'retrying'`).
		WithArgs("dlq-123").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock successful enqueue
	mockQueue.On("EnqueueEvent", ctx, mock.Anything).Return(nil)

	// Expect update to resolved status
	sqlMock.ExpectExec(`UPDATE webhook_dlq SET status = \$2 WHERE id = \$1`).
		WithArgs("dlq-123", "resolved").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Create handler
	logger := observability.NewNoopLogger()
	metrics := observability.NewMetricsClient()
	handler := &DLQHandlerImpl{
		db:          db,
		logger:      logger,
		metrics:     metrics,
		queueClient: mockQueue,
	}

	// Execute
	err = handler.ProcessDLQ(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, sqlMock.ExpectationsWereMet())
	mockQueue.AssertExpectations(t)
}

func TestDLQHandler_RetryFromDLQ(t *testing.T) {
	// Setup
	ctx := context.Background()
	eventID := "event-123"

	// Create mock database
	mockDB, sqlMock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	db := sqlx.NewDb(mockDB, "postgres")

	// Create mock queue client
	mockQueue := &mockQueueClientDLQ{}

	// Set up expectations for fetching specific DLQ entry
	rows := sqlmock.NewRows([]string{
		"id", "event_id", "event_type", "payload", "error_message",
		"retry_count", "last_retry_at", "created_at", "status", "metadata",
	}).AddRow(
		"dlq-123", eventID, "push", json.RawMessage(`{"test": "data"}`), "error",
		1, nil, time.Now(), "pending", json.RawMessage(`{"tool_id": "tool-123"}`),
	)

	sqlMock.ExpectQuery(`SELECT .* FROM webhook_dlq WHERE event_id = \$1`).
		WithArgs(eventID).
		WillReturnRows(rows)

	// Expect update to retrying status
	sqlMock.ExpectExec(`UPDATE webhook_dlq SET status = 'retrying'`).
		WithArgs("dlq-123").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock successful enqueue
	mockQueue.On("EnqueueEvent", ctx, mock.Anything).Return(nil)

	// Expect update to resolved status
	sqlMock.ExpectExec(`UPDATE webhook_dlq SET status = \$2 WHERE id = \$1`).
		WithArgs("dlq-123", "resolved").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Create handler
	logger := observability.NewNoopLogger()
	metrics := observability.NewMetricsClient()
	handler := &DLQHandlerImpl{
		db:          db,
		logger:      logger,
		metrics:     metrics,
		queueClient: mockQueue,
	}

	// Execute
	err = handler.RetryFromDLQ(ctx, eventID)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, sqlMock.ExpectationsWereMet())
	mockQueue.AssertExpectations(t)
}

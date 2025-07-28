package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDLQHandler struct {
	mock.Mock
}

func (m *mockDLQHandler) SendToDLQ(ctx context.Context, event queue.Event, err error) error {
	args := m.Called(ctx, event, err)
	return args.Error(0)
}

func (m *mockDLQHandler) ProcessDLQ(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDLQHandler) RetryFromDLQ(ctx context.Context, eventID string) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

func TestRetryHandler_ExecuteWithRetry_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewNoopLogger()
	mockDLQ := &mockDLQHandler{}

	config := &RetryConfig{
		MaxRetries:      3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
	}

	handler := NewRetryHandler(config, logger, mockDLQ, nil)

	event := queue.Event{
		EventID:   "test-123",
		EventType: "push",
	}

	// Test successful execution
	callCount := 0
	err := handler.ExecuteWithRetry(ctx, event, func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
	mockDLQ.AssertNotCalled(t, "SendToDLQ")
}

func TestRetryHandler_ExecuteWithRetry_RetryableError(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewNoopLogger()
	mockDLQ := &mockDLQHandler{}

	config := &RetryConfig{
		MaxRetries:      3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
	}

	handler := NewRetryHandler(config, logger, mockDLQ, nil)

	event := queue.Event{
		EventID:   "test-123",
		EventType: "push",
	}

	// Test retryable error - succeeds on 3rd attempt
	callCount := 0
	testErr := errors.New("temporary error")
	mockDLQ.On("SendToDLQ", ctx, event, mock.Anything).Return(nil)

	err := handler.ExecuteWithRetry(ctx, event, func() error {
		callCount++
		if callCount < 3 {
			return testErr
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	mockDLQ.AssertNotCalled(t, "SendToDLQ")
}

func TestRetryHandler_ExecuteWithRetry_MaxRetriesExceeded(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewNoopLogger()
	mockDLQ := &mockDLQHandler{}

	config := &RetryConfig{
		MaxRetries:      2,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		MaxElapsedTime:  1 * time.Second,
	}

	handler := NewRetryHandler(config, logger, mockDLQ, nil)

	event := queue.Event{
		EventID:   "test-123",
		EventType: "push",
	}

	// Test max retries exceeded
	callCount := 0
	testErr := errors.New("persistent error")
	mockDLQ.On("SendToDLQ", ctx, event, mock.Anything).Return(nil)

	err := handler.ExecuteWithRetry(ctx, event, func() error {
		callCount++
		return testErr
	})

	assert.Error(t, err)
	assert.Equal(t, 3, callCount) // Initial + 2 retries
	mockDLQ.AssertCalled(t, "SendToDLQ", ctx, event, mock.Anything)
}

func TestRetryHandler_ExecuteWithRetry_NonRetryableError(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewNoopLogger()
	mockDLQ := &mockDLQHandler{}

	handler := NewRetryHandler(nil, logger, mockDLQ, nil)

	event := queue.Event{
		EventID:   "test-123",
		EventType: "push",
	}

	// Test non-retryable error
	callCount := 0
	testErr := errors.New("validation failed: invalid payload")
	mockDLQ.On("SendToDLQ", ctx, event, mock.Anything).Return(nil)

	err := handler.ExecuteWithRetry(ctx, event, func() error {
		callCount++
		return testErr
	})

	assert.Error(t, err)
	assert.Equal(t, 1, callCount) // No retries for non-retryable errors
	mockDLQ.AssertCalled(t, "SendToDLQ", ctx, event, mock.Anything)
}

func TestRetryHandler_isRetryableError(t *testing.T) {
	handler := &RetryHandler{}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "network error - retryable",
			err:  errors.New("connection refused"),
			want: true,
		},
		{
			name: "timeout error - retryable",
			err:  errors.New("request timeout"),
			want: true,
		},
		{
			name: "validation error - non-retryable",
			err:  errors.New("validation failed"),
			want: false,
		},
		{
			name: "tool not found - non-retryable",
			err:  errors.New("tool not found"),
			want: false,
		},
		{
			name: "unauthorized - non-retryable",
			err:  errors.New("unauthorized access"),
			want: false,
		},
		{
			name: "webhook disabled - non-retryable",
			err:  errors.New("webhook config is disabled"),
			want: false,
		},
		{
			name: "generic error - retryable",
			err:  errors.New("something went wrong"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.isRetryableError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRetryHandler_GetBackoffDuration(t *testing.T) {
	config := &RetryConfig{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
	}

	handler := &RetryHandler{config: config}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1600 * time.Millisecond},
		{10, 10 * time.Second}, // Should cap at max interval
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.attempt)), func(t *testing.T) {
			got := handler.GetBackoffDuration(tt.attempt)
			assert.Equal(t, tt.expected, got)
		})
	}
}

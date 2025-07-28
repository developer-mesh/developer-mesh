package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations
type mockToolConfigExtractor struct {
	mock.Mock
}

func (m *mockToolConfigExtractor) ExtractToolConfig(ctx context.Context, event queue.Event) (*models.DynamicTool, error) {
	args := m.Called(ctx, event)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DynamicTool), args.Error(1)
}

type mockEventTransformer struct {
	mock.Mock
}

func (m *mockEventTransformer) Transform(event queue.Event, rules map[string]interface{}) (queue.Event, error) {
	args := m.Called(event, rules)
	return args.Get(0).(queue.Event), args.Error(1)
}

func TestGenericWebhookProcessor_ProcessEvent_Success(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewNoopLogger()
	metrics := observability.NewMetricsClient()

	// Create test event
	event := queue.Event{
		EventID:   "test-123",
		EventType: "push",
		Payload:   json.RawMessage(`{"action": "push", "repository": {"name": "test-repo"}}`),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"tool_id": "tool-123",
		},
	}

	// Create test tool
	tool := &models.DynamicTool{
		ID:       "tool-123",
		ToolName: "test-tool",
		Provider: "github",
		Status:   "active",
		WebhookConfig: &models.ToolWebhookConfig{
			Enabled:               true,
			DefaultProcessingMode: "store_only",
		},
	}

	// Create mocks
	mockExtractor := &mockToolConfigExtractor{}
	mockExtractor.On("ExtractToolConfig", mock.Anything, event).Return(tool, nil)

	mockTransformer := &mockEventTransformer{}

	// Create processor with mocks
	processor := &GenericWebhookProcessor{
		logger:           logger,
		metrics:          metrics,
		metricsCollector: NewMetricsCollector(metrics, observability.GetTracer()),
		configExtractor:  mockExtractor,
		transformer:      mockTransformer,
		retryHandler: &RetryHandler{
			config: DefaultRetryConfig(),
			logger: logger,
			dlq:    nil,
		},
		eventRepo: nil, // Not testing event repo in this unit test
	}

	// Execute
	err := processor.ProcessEvent(ctx, event)

	// Assert
	assert.NoError(t, err)
	mockExtractor.AssertExpectations(t)
}

func TestGenericWebhookProcessor_ProcessEvent_ToolNotFound(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewNoopLogger()
	metrics := observability.NewMetricsClient()

	// Create test event
	event := queue.Event{
		EventID:   "test-123",
		EventType: "push",
		Payload:   json.RawMessage(`{"action": "push"}`),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"tool_id": "nonexistent",
		},
	}

	// Create mocks
	mockExtractor := &mockToolConfigExtractor{}
	mockExtractor.On("ExtractToolConfig", mock.Anything, event).Return(nil, errors.New("tool not found"))

	// Create processor with mocks
	processor := &GenericWebhookProcessor{
		logger:           logger,
		metrics:          metrics,
		metricsCollector: NewMetricsCollector(metrics, observability.GetTracer()),
		configExtractor:  mockExtractor,
		transformer:      &mockEventTransformer{},
		retryHandler: &RetryHandler{
			config: DefaultRetryConfig(),
			logger: logger,
			dlq:    nil,
		},
		eventRepo: nil, // Not testing event repo in this unit test
	}

	// Execute
	err := processor.ProcessEvent(ctx, event)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")
	mockExtractor.AssertExpectations(t)
}

func TestGenericWebhookProcessor_ValidateEvent(t *testing.T) {
	processor := &GenericWebhookProcessor{
		logger: observability.NewNoopLogger(),
	}

	tests := []struct {
		name    string
		event   queue.Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event",
			event: queue.Event{
				EventID:   "123",
				EventType: "push",
				Payload:   json.RawMessage(`{"test": "data"}`),
			},
			wantErr: false,
		},
		{
			name: "missing event ID",
			event: queue.Event{
				EventType: "push",
				Payload:   json.RawMessage(`{"test": "data"}`),
			},
			wantErr: true,
			errMsg:  "event ID is required",
		},
		{
			name: "missing event type",
			event: queue.Event{
				EventID: "123",
				Payload: json.RawMessage(`{"test": "data"}`),
			},
			wantErr: true,
			errMsg:  "event type is required",
		},
		{
			name: "missing payload",
			event: queue.Event{
				EventID:   "123",
				EventType: "push",
			},
			wantErr: true,
			errMsg:  "event payload is required",
		},
		{
			name: "invalid JSON payload",
			event: queue.Event{
				EventID:   "123",
				EventType: "push",
				Payload:   json.RawMessage(`{invalid json`),
			},
			wantErr: true,
			errMsg:  "invalid JSON payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ValidateEvent(tt.event)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenericWebhookProcessor_GetProcessingMode(t *testing.T) {
	processor := &GenericWebhookProcessor{}

	tests := []struct {
		name      string
		tool      *models.DynamicTool
		eventType string
		want      ProcessingMode
	}{
		{
			name:      "nil webhook config",
			tool:      &models.DynamicTool{},
			eventType: "push",
			want:      ModeStoreOnly,
		},
		{
			name: "event-specific mode",
			tool: &models.DynamicTool{
				WebhookConfig: &models.ToolWebhookConfig{
					Events: []models.WebhookEventConfig{
						{
							EventType:      "push",
							ProcessingMode: "store_and_forward",
						},
					},
				},
			},
			eventType: "push",
			want:      ModeStoreAndForward,
		},
		{
			name: "default mode",
			tool: &models.DynamicTool{
				WebhookConfig: &models.ToolWebhookConfig{
					DefaultProcessingMode: "transform_and_store",
				},
			},
			eventType: "pull_request",
			want:      ModeTransformAndStore,
		},
		{
			name: "event mode overrides default",
			tool: &models.DynamicTool{
				WebhookConfig: &models.ToolWebhookConfig{
					DefaultProcessingMode: "store_only",
					Events: []models.WebhookEventConfig{
						{
							EventType:      "push",
							ProcessingMode: "store_and_forward",
						},
					},
				},
			},
			eventType: "push",
			want:      ModeStoreAndForward,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processor.getProcessingMode(tt.tool, tt.eventType)
			assert.Equal(t, tt.want, got)
		})
	}
}

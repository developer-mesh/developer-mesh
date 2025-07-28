package core

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Using MockContextManager defined in mock_context_manager_test.go

// MockAdapter mocks the core.Adapter interface
type MockAdapter struct {
	mock.Mock
}

func (m *MockAdapter) Type() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAdapter) Health() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAdapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	args := m.Called(ctx, contextID, action, params)
	return args.Get(0), args.Error(1)
}

func (m *MockAdapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	args := m.Called(ctx, eventType, payload)
	return args.Error(0)
}

func (m *MockAdapter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAdapter) Version() string {
	args := m.Called()
	return args.String(0)
}

// Additional methods needed for tests but not in core.Adapter interface
func (m *MockAdapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	args := m.Called(ctx, query)
	return args.Get(0), args.Error(1)
}

func TestExecuteToolAction(t *testing.T) {
	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)

	// Create the bridge
	adapters := map[string]Adapter{
		"test-tool": mockAdapter,
	}

	bridge := NewAdapterContextBridge(mockContextManager, adapters)

	// Test data
	ctx := context.Background()
	contextID := "context-123"
	tool := "test-tool"
	action := "test-action"
	params := map[string]interface{}{
		"param1": "value1",
		"param2": 123,
	}

	// Expected result
	expectedResult := map[string]interface{}{
		"result": "success",
		"data":   "test-data",
	}

	// Mock context
	mockContext := &models.Context{
		ID:        contextID,
		AgentID:   "agent-123",
		SessionID: "session-123",
		Content:   []models.ContextItem{},
	}

	// Set up expectations
	mockContextManager.On("GetContext", ctx, contextID).Return(mockContext, nil)
	mockContextManager.On("UpdateContext", ctx, contextID, mock.Anything, mock.Anything).Return(mockContext, nil).Times(2)
	mockAdapter.On("ExecuteAction", ctx, contextID, action, params).Return(expectedResult, nil)

	// Execute the action
	result, err := bridge.ExecuteToolAction(ctx, contextID, tool, action, params)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)

	// Verify all expectations were met
	mockContextManager.AssertExpectations(t)
	mockAdapter.AssertExpectations(t)
}

func TestGetToolData(t *testing.T) {
	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)

	// Create the bridge
	adapters := map[string]Adapter{
		"test-tool": mockAdapter,
	}

	bridge := NewAdapterContextBridge(mockContextManager, adapters)

	// Test data
	ctx := context.Background()
	contextID := "context-123"
	tool := "test-tool"
	query := map[string]interface{}{
		"type": "search",
		"term": "test",
	}

	// Expected result
	expectedResult := map[string]interface{}{
		"results": []interface{}{
			map[string]interface{}{"id": "1", "name": "test1"},
			map[string]interface{}{"id": "2", "name": "test2"},
		},
	}

	// Mock context
	mockContext := &models.Context{
		ID:        contextID,
		AgentID:   "agent-123",
		SessionID: "session-123",
		Content:   []models.ContextItem{},
	}

	// Set up expectations
	mockContextManager.On("GetContext", ctx, contextID).Return(mockContext, nil)
	mockContextManager.On("UpdateContext", ctx, contextID, mock.Anything, mock.Anything).Return(mockContext, nil).Times(2)
	// GetToolData internally calls ExecuteAction with "getData" action
	mockAdapter.On("ExecuteAction", ctx, contextID, "getData", query).Return(expectedResult, nil)

	// Execute the query
	result, err := bridge.GetToolData(ctx, contextID, tool, query)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)

	// Verify all expectations were met
	mockAdapter.AssertExpectations(t)
}

func TestHandleToolWebhook(t *testing.T) {

	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)

	// Create the bridge
	adapters := map[string]Adapter{
		"test-tool": mockAdapter,
	}

	bridge := NewAdapterContextBridge(mockContextManager, adapters)

	// Test data
	ctx := context.Background()
	tool := "test-tool"
	eventType := "test-event"

	// Create a webhook payload with context IDs
	payload := map[string]interface{}{
		"event": "some-event",
		"data": map[string]interface{}{
			"key": "value",
		},
		"metadata": map[string]interface{}{
			"context_ids": []interface{}{"context-1", "context-2"},
		},
	}

	jsonPayload, marshalErr := json.Marshal(payload)
	assert.NoError(t, marshalErr)

	// Test contexts
	testContext1 := &models.Context{
		ID:        "context-1",
		AgentID:   "test-agent",
		ModelID:   "test-model",
		Content:   []models.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	testContext2 := &models.Context{
		ID:        "context-2",
		AgentID:   "test-agent",
		ModelID:   "test-model",
		Content:   []models.ContextItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Set up expectations for context manager
	mockContextManager.On("GetContext", ctx, "context-1").Return(testContext1, nil)
	mockContextManager.On("GetContext", ctx, "context-2").Return(testContext2, nil)

	// This is where the mismatch occurs - the actual implementation likely uses a specific context type
	// in the test we'd need to match that exactly
	mockContextManager.On("UpdateContext", ctx, "context-1", mock.Anything, mock.Anything).Return(testContext1, nil)
	mockContextManager.On("UpdateContext", ctx, "context-2", mock.Anything, mock.Anything).Return(testContext2, nil)

	// Set up expectations for adapter
	mockAdapter.On("HandleWebhook", ctx, eventType, jsonPayload).Return(nil)

	// Handle the webhook
	err := bridge.HandleToolWebhook(ctx, tool, eventType, jsonPayload)

	// Assertions
	assert.NoError(t, err)

	// Verify that all expectations were met
	mockContextManager.AssertExpectations(t)
	mockAdapter.AssertExpectations(t)
}

func TestExecuteToolAction_Error(t *testing.T) {
	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)

	// Create the bridge
	adapters := map[string]Adapter{
		"test-tool": mockAdapter,
	}

	bridge := NewAdapterContextBridge(mockContextManager, adapters)

	// Test data
	ctx := context.Background()
	contextID := "context-123"
	tool := "test-tool"
	action := "test-action"
	params := map[string]interface{}{"param": "value"}

	// Test context not found error
	mockContextManager.On("GetContext", ctx, contextID).Return(nil, assert.AnError)

	// Execute the action
	result, err := bridge.ExecuteToolAction(ctx, contextID, tool, action, params)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get context")

	// Verify expectations
	mockContextManager.AssertExpectations(t)
}

func TestGetToolData_Error(t *testing.T) {
	// Set up mocks
	mockContextManager := new(MockContextManager)
	mockAdapter := new(MockAdapter)

	// Create the bridge
	adapters := map[string]Adapter{
		"test-tool": mockAdapter,
	}

	bridge := NewAdapterContextBridge(mockContextManager, adapters)

	// Test data
	ctx := context.Background()
	contextID := "context-123"
	query := map[string]interface{}{"query": "test"}

	// Test adapter not found error
	result, err := bridge.GetToolData(ctx, contextID, "unknown-tool", query)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, result)

	// Verify expectations
	mockAdapter.AssertExpectations(t)
}

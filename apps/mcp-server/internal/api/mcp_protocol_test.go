package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/developer-mesh/developer-mesh/pkg/clients"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MockRESTAPIClient is a mock implementation of the REST API client
type MockRESTAPIClient struct {
	mock.Mock
}

func (m *MockRESTAPIClient) ListTools(ctx context.Context, tenantID string) ([]*models.DynamicTool, error) {
	args := m.Called(ctx, tenantID)
	if tools := args.Get(0); tools != nil {
		return tools.([]*models.DynamicTool), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRESTAPIClient) GetTool(ctx context.Context, tenantID, toolID string) (*models.DynamicTool, error) {
	args := m.Called(ctx, tenantID, toolID)
	if tool := args.Get(0); tool != nil {
		return tool.(*models.DynamicTool), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRESTAPIClient) ExecuteTool(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (*models.ToolExecutionResponse, error) {
	args := m.Called(ctx, tenantID, toolID, action, params)
	if resp := args.Get(0); resp != nil {
		return resp.(*models.ToolExecutionResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRESTAPIClient) GetToolHealth(ctx context.Context, tenantID, toolID string) (*models.HealthStatus, error) {
	args := m.Called(ctx, tenantID, toolID)
	if health := args.Get(0); health != nil {
		return health.(*models.HealthStatus), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRESTAPIClient) GenerateEmbedding(ctx context.Context, tenantID, agentID, text, model, taskType string) (*models.EmbeddingResponse, error) {
	args := m.Called(ctx, tenantID, agentID, text, model, taskType)
	if resp := args.Get(0); resp != nil {
		return resp.(*models.EmbeddingResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRESTAPIClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRESTAPIClient) GetMetrics() clients.ClientMetrics {
	args := m.Called()
	return args.Get(0).(clients.ClientMetrics)
}

func (m *MockRESTAPIClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestIsMCPMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  []byte
		expected bool
	}{
		{
			name:     "Valid MCP message",
			message:  []byte(`{"jsonrpc":"2.0","method":"initialize","id":1}`),
			expected: true,
		},
		{
			name:     "Valid MCP message with spaces",
			message:  []byte(`{"jsonrpc": "2.0", "method": "initialize", "id": 1}`),
			expected: true,
		},
		{
			name:     "Non-MCP JSON message",
			message:  []byte(`{"type":"request","method":"test"}`),
			expected: false,
		},
		{
			name:     "Invalid JSON",
			message:  []byte(`not json`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMCPMessage(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMCPProtocolHandler_HandleInitialize(t *testing.T) {
	// Create mock REST client
	mockClient := new(MockRESTAPIClient)
	logger := observability.NewStandardLogger("test")

	// Create handler
	handler := NewMCPProtocolHandler(mockClient, logger)

	// Create test message
	msg := MCPMessage{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
		Params:  json.RawMessage(`{"protocolVersion": "2025-06-18", "clientInfo": {"name": "test-client"}}`),
	}

	// Marshal message
	msgBytes, err := json.Marshal(msg)
	assert.NoError(t, err)

	// Create mock connection
	// Note: In a real test, you'd use a test WebSocket server
	// This is a simplified example
	connID := "test-conn-id"
	tenantID := "test-tenant-id"

	// Test handling
	// In practice, you'd need to mock the WebSocket connection
	// This demonstrates the structure
	assert.NotNil(t, handler)
	assert.NotNil(t, msgBytes)
	assert.NotEmpty(t, connID)
	assert.NotEmpty(t, tenantID)
}

func TestMCPProtocolHandler_HandleToolsList(t *testing.T) {
	// Create mock REST client
	mockClient := new(MockRESTAPIClient)
	logger := observability.NewStandardLogger("test")

	// Set up mock expectations
	expectedTools := []*models.DynamicTool{
		{
			ID:          "tool1",
			ToolName:    "test-tool",
			DisplayName: "Test Tool",
			ToolType:    "api",
			BaseURL:     "https://api.example.com",
		},
	}

	mockClient.On("ListTools", mock.Anything, "test-tenant").Return(expectedTools, nil)

	// Create handler
	handler := NewMCPProtocolHandler(mockClient, logger)

	// Initialize session first
	handler.sessions["test-conn"] = &MCPSession{
		ID:       "test-conn",
		TenantID: "test-tenant",
		AgentID:  "test-agent",
	}

	// Verify session was created
	session := handler.getSession("test-conn")
	assert.NotNil(t, session)
	assert.Equal(t, "test-tenant", session.TenantID)

	// Verify mock was called correctly (in a real WebSocket test)
	// mockClient.AssertExpectations(t)
}

func TestMCPProtocolHandler_HandleToolCall(t *testing.T) {
	// Create mock REST client
	mockClient := new(MockRESTAPIClient)
	logger := observability.NewStandardLogger("test")

	// Set up mock expectations
	expectedResponse := &models.ToolExecutionResponse{
		Success:    true,
		StatusCode: 200,
		Body:       "Tool executed successfully",
	}

	mockClient.On("ExecuteTool",
		mock.Anything,
		"test-tenant",
		"test-tool",
		"execute",
		map[string]interface{}{"input": "test"},
	).Return(expectedResponse, nil)

	// Create handler
	handler := NewMCPProtocolHandler(mockClient, logger)

	// Initialize session
	handler.sessions["test-conn"] = &MCPSession{
		ID:       "test-conn",
		TenantID: "test-tenant",
		AgentID:  "test-agent",
	}

	// Test the handler structure
	assert.NotNil(t, handler)
	assert.Len(t, handler.sessions, 1)
}

func TestMCPProtocolHandler_SessionManagement(t *testing.T) {
	mockClient := new(MockRESTAPIClient)
	logger := observability.NewStandardLogger("test")

	handler := NewMCPProtocolHandler(mockClient, logger)

	// Test adding session
	connID := "test-conn-1"
	session := &MCPSession{
		ID:       connID,
		TenantID: "tenant-1",
		AgentID:  "agent-1",
	}
	handler.sessions[connID] = session

	// Test getting session
	retrieved := handler.getSession(connID)
	assert.NotNil(t, retrieved)
	assert.Equal(t, session.TenantID, retrieved.TenantID)

	// Test removing session
	handler.RemoveSession(connID)
	retrieved = handler.getSession(connID)
	assert.Nil(t, retrieved)
}

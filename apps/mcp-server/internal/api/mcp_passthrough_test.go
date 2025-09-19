package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MockPassthroughServer for testing passthrough auth
type MockPassthroughServer struct {
	mock.Mock
}

func (m *MockPassthroughServer) GetConnectionPassthroughAuth(connID string) *models.PassthroughAuthBundle {
	args := m.Called(connID)
	if auth := args.Get(0); auth != nil {
		return auth.(*models.PassthroughAuthBundle)
	}
	return nil
}

func TestMCPPassthroughAuthentication(t *testing.T) {
	// Disable tool refresh for tests
	t.Setenv("DISABLE_TOOL_REFRESH", "true")

	logger := observability.NewNoopLogger()

	t.Run("Handler with server reference retrieves passthrough auth", func(t *testing.T) {
		// Create mock REST client
		mockClient := &MockRESTAPIClient{}

		// Create mock server with passthrough auth
		mockServer := &MockPassthroughServer{}
		passthroughAuth := &models.PassthroughAuthBundle{
			Credentials: map[string]*models.PassthroughCredential{
				"github": {
					Type:  "bearer",
					Token: "ghp_test123",
				},
			},
		}
		mockServer.On("GetConnectionPassthroughAuth", "conn-123").Return(passthroughAuth)

		// Create handler with server
		handler := NewMCPProtocolHandlerWithServer(mockClient, logger, mockServer)
		assert.NotNil(t, handler)
		assert.NotNil(t, handler.server)

		// Verify server method works
		auth := handler.server.GetConnectionPassthroughAuth("conn-123")
		assert.NotNil(t, auth)
		assert.Equal(t, "ghp_test123", auth.Credentials["github"].Token)

		mockServer.AssertExpectations(t)
	})

	t.Run("ExecuteToolWithAuth is called with passthrough auth", func(t *testing.T) {
		// Create mock REST client
		mockClient := &MockRESTAPIClient{}

		passthroughAuth := &models.PassthroughAuthBundle{
			Credentials: map[string]*models.PassthroughCredential{
				"github": {
					Type:  "bearer",
					Token: "ghp_secret789",
				},
			},
		}

		// Mock tool execution with auth
		mockClient.On("ExecuteToolWithAuth",
			mock.Anything,
			"tenant-789",
			"tool-123",
			"execute",
			mock.MatchedBy(func(params map[string]interface{}) bool {
				return params["test"] == "value"
			}),
			passthroughAuth,
		).Return(&models.ToolExecutionResponse{
			Success:    true,
			StatusCode: 200,
			Body:       map[string]interface{}{"result": "success"},
		}, nil)

		// Call the method directly
		response, err := mockClient.ExecuteToolWithAuth(
			context.TODO(),
			"tenant-789",
			"tool-123",
			"execute",
			map[string]interface{}{"test": "value"},
			passthroughAuth,
		)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.True(t, response.Success)
		assert.Equal(t, 200, response.StatusCode)

		mockClient.AssertExpectations(t)
	})

	t.Run("Handler without server works without passthrough auth", func(t *testing.T) {
		// Create mock REST client
		mockClient := &MockRESTAPIClient{}

		// Create handler without server (backward compatibility)
		handler := NewMCPProtocolHandler(mockClient, logger)
		assert.NotNil(t, handler)
		assert.Nil(t, handler.server)
	})
}

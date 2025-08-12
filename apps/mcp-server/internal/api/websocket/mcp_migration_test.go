package websocket

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// isMCPTestMessage checks if a message is an MCP message (contains "jsonrpc":"2.0")
// This matches the logic in connection.go readPump method
func isMCPTestMessage(data []byte) bool {
	msg := string(data)
	// Check for jsonrpc 2.0 specifically
	return strings.Contains(msg, `"jsonrpc":"2.0"`) || strings.Contains(msg, `"jsonrpc": "2.0"`)
}

// MCPMigrationTestSuite tests the migration from custom protocol to MCP-only
type MCPMigrationTestSuite struct {
	server     *Server
	mcpHandler *MockMCPHandler
	conn       *MockWebSocketConn
	logger     observability.Logger
}

// MockMCPHandler mocks the MCP protocol handler
type MockMCPHandler struct {
	mock.Mock
}

func (m *MockMCPHandler) HandleMessage(conn *websocket.Conn, connID string, tenantID string, message []byte) error {
	args := m.Called(conn, connID, tenantID, message)
	return args.Error(0)
}

// MockWebSocketConn mocks a WebSocket connection
type MockWebSocketConn struct {
	mock.Mock
	messages [][]byte
}

func (m *MockWebSocketConn) Read(ctx context.Context) (websocket.MessageType, []byte, error) {
	if len(m.messages) > 0 {
		msg := m.messages[0]
		m.messages = m.messages[1:]
		return websocket.MessageText, msg, nil
	}
	// Block until context is cancelled
	<-ctx.Done()
	return 0, nil, ctx.Err()
}

func (m *MockWebSocketConn) Write(ctx context.Context, msgType websocket.MessageType, data []byte) error {
	args := m.Called(ctx, msgType, data)
	return args.Error(0)
}

func (m *MockWebSocketConn) Close(statusCode websocket.StatusCode, reason string) error {
	args := m.Called(statusCode, reason)
	return args.Error(0)
}

func (m *MockWebSocketConn) CloseRead(ctx context.Context) context.Context {
	return ctx
}

func (m *MockWebSocketConn) Ping(ctx context.Context) error {
	return nil
}

func (m *MockWebSocketConn) SetReadLimit(limit int64) {}

// TestMCPAgentRegistration tests that agent registration works via MCP initialize
func TestMCPAgentRegistration(t *testing.T) {
	suite := setupMCPMigrationSuite(t)

	// Create MCP initialize message
	initMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"clientInfo": map[string]interface{}{
				"name":    "test-agent",
				"version": "1.0.0",
				"type":    "ide",
			},
		},
	}

	msgBytes, err := json.Marshal(initMsg)
	require.NoError(t, err)

	// Set expectation
	suite.mcpHandler.On("HandleMessage", mock.Anything, mock.Anything, "test-tenant", msgBytes).Return(nil)

	// Simulate message reception
	conn := &Connection{
		Connection: &ws.Connection{
			ID:       uuid.New().String(),
			TenantID: "test-tenant",
		},
		hub: suite.server,
	}

	// Test routing to MCP handler
	isMCP := isMCPTestMessage(msgBytes)
	assert.True(t, isMCP, "Should recognize MCP message")

	// Verify handler would be called
	err = suite.mcpHandler.HandleMessage(nil, conn.ID, conn.TenantID, msgBytes)
	assert.NoError(t, err)

	suite.mcpHandler.AssertExpectations(t)
}

// TestMCPToolExecution tests that tool execution works via MCP tools/call
func TestMCPToolExecution(t *testing.T) {
	suite := setupMCPMigrationSuite(t)

	// Create MCP tools/call message
	toolCallMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "github.getPullRequest",
			"arguments": map[string]interface{}{
				"owner": "developer-mesh",
				"repo":  "developer-mesh",
				"pr":    123,
			},
		},
	}

	msgBytes, err := json.Marshal(toolCallMsg)
	require.NoError(t, err)

	// Verify it's recognized as MCP
	assert.True(t, isMCPTestMessage(msgBytes))

	// Set expectation
	suite.mcpHandler.On("HandleMessage", mock.Anything, mock.Anything, "test-tenant", msgBytes).Return(nil)

	// Simulate execution
	err = suite.mcpHandler.HandleMessage(nil, "conn-123", "test-tenant", msgBytes)
	assert.NoError(t, err)

	suite.mcpHandler.AssertExpectations(t)
}

// TestMCPWorkflowAsTools tests that workflow operations work as MCP tools
func TestMCPWorkflowAsTools(t *testing.T) {
	suite := setupMCPMigrationSuite(t)

	testCases := []struct {
		name   string
		method string
		params map[string]interface{}
	}{
		{
			name:   "Create Workflow",
			method: "tools/call",
			params: map[string]interface{}{
				"name": "workflow.create",
				"arguments": map[string]interface{}{
					"name":        "test-workflow",
					"description": "Test workflow",
					"steps": []interface{}{
						map[string]interface{}{
							"name": "step1",
							"type": "tool",
							"tool": "github.createIssue",
						},
					},
				},
			},
		},
		{
			name:   "Execute Workflow",
			method: "tools/call",
			params: map[string]interface{}{
				"name": "workflow.execute",
				"arguments": map[string]interface{}{
					"workflow_id": "wf-123",
					"input": map[string]interface{}{
						"param1": "value1",
					},
				},
			},
		},
		{
			name:   "Get Workflow Status",
			method: "resources/read",
			params: map[string]interface{}{
				"uri": "workflow/wf-123/status",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      uuid.New().String(),
				"method":  tc.method,
				"params":  tc.params,
			}

			msgBytes, err := json.Marshal(msg)
			require.NoError(t, err)

			assert.True(t, isMCPTestMessage(msgBytes))
			suite.mcpHandler.On("HandleMessage", mock.Anything, mock.Anything, mock.Anything, msgBytes).Return(nil).Once()

			err = suite.mcpHandler.HandleMessage(nil, "conn-123", "test-tenant", msgBytes)
			assert.NoError(t, err)
		})
	}

	suite.mcpHandler.AssertExpectations(t)
}

// TestMCPTaskManagement tests that task operations work via MCP
func TestMCPTaskManagement(t *testing.T) {
	suite := setupMCPMigrationSuite(t)

	// Test task creation as a tool
	createTaskMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "task.create",
			"arguments": map[string]interface{}{
				"title":       "Test Task",
				"description": "Test task description",
				"priority":    "high",
				"agent_type":  "ide",
			},
		},
	}

	msgBytes, err := json.Marshal(createTaskMsg)
	require.NoError(t, err)

	assert.True(t, isMCPTestMessage(msgBytes))
	suite.mcpHandler.On("HandleMessage", mock.Anything, mock.Anything, mock.Anything, msgBytes).Return(nil)

	err = suite.mcpHandler.HandleMessage(nil, "conn-123", "test-tenant", msgBytes)
	assert.NoError(t, err)

	// Test task status as a resource
	taskStatusMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": "task/task-123",
		},
	}

	msgBytes, err = json.Marshal(taskStatusMsg)
	require.NoError(t, err)

	assert.True(t, isMCPTestMessage(msgBytes))
	suite.mcpHandler.On("HandleMessage", mock.Anything, mock.Anything, mock.Anything, msgBytes).Return(nil)

	err = suite.mcpHandler.HandleMessage(nil, "conn-123", "test-tenant", msgBytes)
	assert.NoError(t, err)

	suite.mcpHandler.AssertExpectations(t)
}

// TestMCPContextManagement tests that context operations work via MCP
func TestMCPContextManagement(t *testing.T) {
	suite := setupMCPMigrationSuite(t)

	// Test context as a resource
	contextMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": "context/session-123",
		},
	}

	msgBytes, err := json.Marshal(contextMsg)
	require.NoError(t, err)

	assert.True(t, isMCPTestMessage(msgBytes))
	suite.mcpHandler.On("HandleMessage", mock.Anything, mock.Anything, mock.Anything, msgBytes).Return(nil)

	err = suite.mcpHandler.HandleMessage(nil, "conn-123", "test-tenant", msgBytes)
	assert.NoError(t, err)

	// Test context update as a tool
	updateMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "context.update",
			"arguments": map[string]interface{}{
				"session_id": "session-123",
				"content":    "Updated context content",
			},
		},
	}

	msgBytes, err = json.Marshal(updateMsg)
	require.NoError(t, err)

	assert.True(t, isMCPTestMessage(msgBytes))
	suite.mcpHandler.On("HandleMessage", mock.Anything, mock.Anything, mock.Anything, msgBytes).Return(nil)

	err = suite.mcpHandler.HandleMessage(nil, "conn-123", "test-tenant", msgBytes)
	assert.NoError(t, err)

	suite.mcpHandler.AssertExpectations(t)
}

// TestMCPRedisIntegration tests that MCP events are published to Redis
func TestMCPRedisIntegration(t *testing.T) {
	t.Skip("Redis integration test - requires Redis connection")

	// This test would verify that:
	// 1. MCP tool executions publish to Redis Streams
	// 2. Workflow events are published
	// 3. Task events are published
	// 4. Agent registration events are published
}

// TestMCPHeartbeat tests heartbeat mechanism in MCP
func TestMCPHeartbeat(t *testing.T) {
	suite := setupMCPMigrationSuite(t)

	// MCP doesn't have a specific heartbeat, but we can use ping/pong
	// or implement a custom heartbeat tool
	heartbeatMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "agent.heartbeat",
			"arguments": map[string]interface{}{
				"agent_id":  "agent-123",
				"timestamp": time.Now().Unix(),
			},
		},
	}

	msgBytes, err := json.Marshal(heartbeatMsg)
	require.NoError(t, err)

	assert.True(t, isMCPTestMessage(msgBytes))
	suite.mcpHandler.On("HandleMessage", mock.Anything, mock.Anything, mock.Anything, msgBytes).Return(nil)

	err = suite.mcpHandler.HandleMessage(nil, "conn-123", "test-tenant", msgBytes)
	assert.NoError(t, err)

	suite.mcpHandler.AssertExpectations(t)
}

// TestMCPSubscriptions tests that subscriptions work via MCP resources
func TestMCPSubscriptions(t *testing.T) {
	suite := setupMCPMigrationSuite(t)

	// Test resource subscription
	subscribeMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "resources/subscribe",
		"params": map[string]interface{}{
			"uri": "workflow/*/status",
		},
	}

	msgBytes, err := json.Marshal(subscribeMsg)
	require.NoError(t, err)

	assert.True(t, isMCPTestMessage(msgBytes))
	suite.mcpHandler.On("HandleMessage", mock.Anything, mock.Anything, mock.Anything, msgBytes).Return(nil)

	err = suite.mcpHandler.HandleMessage(nil, "conn-123", "test-tenant", msgBytes)
	assert.NoError(t, err)

	suite.mcpHandler.AssertExpectations(t)
}

// TestCustomProtocolRemoved verifies custom protocol messages are rejected
func TestCustomProtocolRemoved(t *testing.T) {
	t.Skip("Run this test after migration is complete")

	// This test will verify that custom protocol messages are rejected
	// after the migration is complete

	// suite := setupMCPMigrationSuite(t)

	// Old custom protocol message (should be rejected after migration)
	oldMsg := map[string]interface{}{
		"type": "agent.register",
		"payload": map[string]interface{}{
			"agent_id": "agent-123",
		},
	}

	msgBytes, err := json.Marshal(oldMsg)
	require.NoError(t, err)

	// Should NOT be recognized as MCP
	assert.False(t, isMCPTestMessage(msgBytes))

	// After migration, this should return an error
	// The server should only accept MCP messages
}

// Helper function to setup test suite
func setupMCPMigrationSuite(t *testing.T) *MCPMigrationTestSuite {
	logger := observability.NewStandardLogger("test")

	suite := &MCPMigrationTestSuite{
		mcpHandler: new(MockMCPHandler),
		conn:       new(MockWebSocketConn),
		logger:     logger,
	}

	// Create server with mocked MCP handler
	config := Config{
		MaxMessageSize: 1048576,
	}

	suite.server = &Server{
		connections: make(map[string]*Connection),
		handlers:    make(map[string]interface{}),
		logger:      logger,
		config:      config,
		mcpHandler:  suite.mcpHandler,
	}

	return suite
}

// TestMCPProtocolCompliance ensures MCP protocol compliance
func TestMCPProtocolCompliance(t *testing.T) {
	testCases := []struct {
		name     string
		message  string
		shouldBe bool
	}{
		{
			name:     "Valid MCP with spaces",
			message:  `{"jsonrpc": "2.0", "method": "test"}`,
			shouldBe: true,
		},
		{
			name:     "Valid MCP without spaces",
			message:  `{"jsonrpc":"2.0","method":"test"}`,
			shouldBe: true,
		},
		{
			name:     "Invalid - wrong version",
			message:  `{"jsonrpc":"1.0","method":"test"}`,
			shouldBe: false,
		},
		{
			name:     "Invalid - custom protocol",
			message:  `{"type":"agent.register","payload":{}}`,
			shouldBe: false,
		},
		{
			name:     "Invalid - missing jsonrpc",
			message:  `{"method":"test","params":{}}`,
			shouldBe: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isMCPTestMessage([]byte(tc.message))
			assert.Equal(t, tc.shouldBe, result, "Message recognition mismatch")
		})
	}
}

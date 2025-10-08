package mcp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/auth"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleInitialize_ValidProtocolVersions(t *testing.T) {
	// Setup - Create handler with minimal dependencies
	logger := observability.NewNoopLogger()
	toolRegistry := tools.NewRegistry()
	memCache := cache.NewMemoryCache(100, 5*time.Minute)
	authenticator := auth.NewEdgeAuthenticator("")

	handler := NewHandler(toolRegistry, memCache, nil, authenticator, logger, nil, nil)
	sessionID := "test-session-123"

	// Test each valid protocol version
	validVersions := []string{
		"2024-11-05", // Original Claude Code version
		"2025-03-26", // March 2025 release
		"2025-06-18", // Latest version
	}

	for _, version := range validVersions {
		t.Run("Protocol_"+version, func(t *testing.T) {
			// Create initialize message
			params := map[string]interface{}{
				"protocolVersion": version,
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			}

			paramsJSON, err := json.Marshal(params)
			require.NoError(t, err, "Failed to marshal params")

			msg := &MCPMessage{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params:  paramsJSON,
			}

			// Execute
			response, err := handler.handleInitialize(sessionID, msg)

			// Verify
			assert.NoError(t, err, "Initialize should succeed for version %s", version)
			assert.NotNil(t, response, "Response should not be nil")
			assert.Equal(t, "2.0", response.JSONRPC)
			assert.Equal(t, 1, response.ID)

			// Check result structure
			result, ok := response.Result.(map[string]interface{})
			require.True(t, ok, "Result should be a map")
			assert.Equal(t, version, result["protocolVersion"])

			// Verify capabilities are present
			capabilities, ok := result["capabilities"].(map[string]interface{})
			require.True(t, ok, "Capabilities should be present")
			assert.Contains(t, capabilities, "tools")
			assert.Contains(t, capabilities, "resources")
		})
	}
}

func TestHandleInitialize_InvalidProtocolVersion(t *testing.T) {
	// Setup
	logger := observability.NewNoopLogger()
	handler := NewHandler(tools.NewRegistry(), cache.NewMemoryCache(100, 5*time.Minute), nil,
		auth.NewEdgeAuthenticator(""), logger, nil, nil)
	sessionID := "test-session-456"

	// Test with unsupported version
	params := map[string]interface{}{
		"protocolVersion": "1999-01-01", // Invalid version
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	}

	paramsJSON, _ := json.Marshal(params)
	msg := &MCPMessage{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "initialize",
		Params:  paramsJSON,
	}

	// Execute
	_, err := handler.handleInitialize(sessionID, msg)

	// Verify error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported version '1999-01-01'")
	assert.Contains(t, err.Error(), "supported:")
}

func TestHandleInitialize_MalformedJSON(t *testing.T) {
	// Setup
	logger := observability.NewNoopLogger()
	handler := NewHandler(tools.NewRegistry(), cache.NewMemoryCache(100, 5*time.Minute), nil,
		auth.NewEdgeAuthenticator(""), logger, nil, nil)
	sessionID := "test-session-789"

	// Test with malformed JSON
	msg := &MCPMessage{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "initialize",
		Params:  json.RawMessage(`{"invalid json`), // Malformed
	}

	// Execute
	_, err := handler.handleInitialize(sessionID, msg)

	// Verify error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid initialize params")
}

func TestHandleInitialize_SessionUpdate(t *testing.T) {
	// Setup
	logger := observability.NewNoopLogger()
	handler := NewHandler(tools.NewRegistry(), cache.NewMemoryCache(100, 5*time.Minute), nil,
		auth.NewEdgeAuthenticator(""), logger, nil, nil)
	sessionID := "test-session-update"

	// Pre-create session
	handler.sessions[sessionID] = &Session{
		ID:          sessionID,
		Initialized: false,
	}

	// Create valid initialize message
	params := map[string]interface{}{
		"protocolVersion": "2025-06-18",
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	}
	paramsJSON, _ := json.Marshal(params)
	msg := &MCPMessage{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "initialize",
		Params:  paramsJSON,
	}

	// Execute
	response, err := handler.handleInitialize(sessionID, msg)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Check session was updated
	session := handler.sessions[sessionID]
	assert.True(t, session.Initialized, "Session should be marked as initialized")
}

func TestHandleInitialize_CredentialStorage(t *testing.T) {
	// Setup
	logger := observability.NewNoopLogger()
	handler := NewHandler(tools.NewRegistry(), cache.NewMemoryCache(100, 5*time.Minute), nil,
		auth.NewEdgeAuthenticator(""), logger, nil, nil)
	sessionID := "test-session-creds"

	// Pre-create session
	handler.sessions[sessionID] = &Session{
		ID:          sessionID,
		Initialized: false,
	}

	// Create initialize message with credentials
	params := map[string]interface{}{
		"protocolVersion": "2025-06-18",
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
		"credentials": map[string]interface{}{
			"credentials": map[string]interface{}{
				"github": map[string]interface{}{
					"type":  "bearer",
					"token": "test-github-token-123",
				},
			},
		},
	}
	paramsJSON, _ := json.Marshal(params)
	msg := &MCPMessage{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "initialize",
		Params:  paramsJSON,
	}

	// Execute
	response, err := handler.handleInitialize(sessionID, msg)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Check session credentials were stored
	session := handler.sessions[sessionID]
	assert.True(t, session.Initialized, "Session should be marked as initialized")
	assert.NotNil(t, session.PassthroughAuth, "Session should have passthrough auth")
	assert.NotNil(t, session.PassthroughAuth.Credentials, "Session should have credentials")

	// Verify GitHub credential
	githubCred, exists := session.PassthroughAuth.Credentials["github"]
	assert.True(t, exists, "GitHub credential should exist")
	assert.Equal(t, "bearer", githubCred.Type, "Credential type should be bearer")
	assert.Equal(t, "test-github-token-123", githubCred.Token, "Token should match")
}

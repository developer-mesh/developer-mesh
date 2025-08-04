package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// TestAuthenticateRequestWithCustomHeader tests authentication with custom API key header
func TestAuthenticateRequestWithCustomHeader(t *testing.T) {
	logger := observability.NewNoopLogger()

	// Create auth config with custom header
	authConfig := auth.DefaultConfig()
	authConfig.APIKeyHeader = "X-API-Key"
	authConfig.JWTSecret = "test-secret"

	// Create auth service
	authService := auth.NewService(authConfig, nil, nil, logger)

	// Initialize test API key
	testAPIKey := "test-api-key-12345"
	authService.InitializeDefaultAPIKeys(map[string]string{
		testAPIKey: "admin",
	})

	// Create WebSocket server
	wsConfig := Config{
		MaxConnections: 10,
	}
	server := NewServer(authService, nil, logger, wsConfig)

	tests := []struct {
		name          string
		headers       map[string]string
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid API key in Authorization header with Bearer",
			headers: map[string]string{
				"Authorization": "Bearer " + testAPIKey,
			},
			expectError: false,
		},
		{
			name: "Valid API key in Authorization header without Bearer",
			headers: map[string]string{
				"Authorization": testAPIKey,
			},
			expectError: false,
		},
		{
			name: "Valid API key in custom X-API-Key header",
			headers: map[string]string{
				"X-API-Key": testAPIKey,
			},
			expectError: false,
		},
		{
			name:          "No authentication headers",
			headers:       map[string]string{},
			expectError:   true,
			errorContains: "no API key provided",
		},
		{
			name: "Invalid API key",
			headers: map[string]string{
				"Authorization": "Bearer invalid-key",
			},
			expectError:   true,
			errorContains: "invalid API key",
		},
		{
			name: "Invalid API key in custom header",
			headers: map[string]string{
				"X-API-Key": "invalid-key",
			},
			expectError:   true,
			errorContains: "invalid API key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Test authentication
			claims, err := server.authenticateRequest(req)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, claims)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, "00000000-0000-0000-0000-000000000001", claims.TenantID)
				assert.Contains(t, claims.Scopes, "admin")
			}
		})
	}
}

// TestAuthenticateRequestWithoutAuthService tests authentication when auth service is nil
func TestAuthenticateRequestWithoutAuthService(t *testing.T) {
	logger := observability.NewNoopLogger()

	// Create WebSocket server without auth service
	wsConfig := Config{
		MaxConnections: 10,
	}
	server := NewServer(nil, nil, logger, wsConfig)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	// Test authentication
	claims, err := server.authenticateRequest(req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication service unavailable")
	assert.Nil(t, claims)
}

// TestAuthConsistencyBetweenRESTAndWebSocket verifies that both REST and WebSocket
// accept the same authentication headers
func TestAuthConsistencyBetweenRESTAndWebSocket(t *testing.T) {
	logger := observability.NewNoopLogger()

	// Create auth config
	authConfig := auth.DefaultConfig()
	authConfig.APIKeyHeader = "X-API-Key"
	authConfig.JWTSecret = "test-secret"

	// Create auth service
	authService := auth.NewService(authConfig, nil, nil, logger)

	// Initialize test API keys
	testAPIKey := "test-api-key-12345"
	authService.InitializeDefaultAPIKeys(map[string]string{
		testAPIKey: "admin",
	})

	// Test various header formats
	headerTests := []map[string]string{
		{"Authorization": "Bearer " + testAPIKey},
		{"Authorization": testAPIKey},
		{"X-API-Key": testAPIKey},
	}

	for i, headers := range headerTests {
		// Test with REST API middleware
		ctx := context.Background()
		var user *auth.User
		var err error

		// Check which header is being used
		if authHeader := headers["Authorization"]; authHeader != "" {
			if authHeader[:7] == "Bearer " {
				user, err = authService.ValidateAPIKey(ctx, authHeader[7:])
			} else {
				user, err = authService.ValidateAPIKey(ctx, authHeader)
			}
		} else if apiKey := headers["X-API-Key"]; apiKey != "" {
			user, err = authService.ValidateAPIKey(ctx, apiKey)
		}

		assert.NoError(t, err, "REST API validation failed for header set %d", i)
		assert.NotNil(t, user, "REST API user is nil for header set %d", i)

		// Test with WebSocket
		wsConfig := Config{MaxConnections: 10}
		wsServer := NewServer(authService, nil, logger, wsConfig)

		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		claims, err := wsServer.authenticateRequest(req)
		assert.NoError(t, err, "WebSocket validation failed for header set %d", i)
		assert.NotNil(t, claims, "WebSocket claims is nil for header set %d", i)

		// Verify consistency - convert UUID to string for comparison
		assert.Equal(t, user.ID.String(), claims.UserID, "User ID mismatch for header set %d", i)
		assert.Equal(t, user.TenantID.String(), claims.TenantID, "Tenant ID mismatch for header set %d", i)
	}
}

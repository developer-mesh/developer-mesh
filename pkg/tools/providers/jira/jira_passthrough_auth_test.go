package jira

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Context key jiraTokenKey is now defined in jira_provider.go

func TestJiraProvider_ExtractAuthToken(t *testing.T) {
	logger := &observability.NoopLogger{}

	tests := []struct {
		name          string
		ctx           context.Context
		params        map[string]interface{}
		expectedEmail string
		expectedToken string
		expectedError bool
		errorContains string
	}{
		// Priority 1: ProviderContext with Token
		{
			name: "provider context with colon-separated token",
			ctx: providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{
					Token: "user@example.com:api-token-123",
				},
			}),
			params:        map[string]interface{}{},
			expectedEmail: "user@example.com",
			expectedToken: "api-token-123",
			expectedError: false,
		},
		{
			name: "provider context with metadata custom token",
			ctx: providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{},
				Metadata: map[string]interface{}{
					"token": "user@example.com:api-token-456",
				},
			}),
			params:        map[string]interface{}{},
			expectedEmail: "user@example.com",
			expectedToken: "api-token-456",
			expectedError: false,
		},
		{
			name: "provider context with email and api_token in metadata",
			ctx: providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{},
				Metadata: map[string]interface{}{
					"email":     "user@example.com",
					"api_token": "api-token-789",
				},
			}),
			params:        map[string]interface{}{},
			expectedEmail: "user@example.com",
			expectedToken: "api-token-789",
			expectedError: false,
		},
		{
			name: "provider context with access_token in metadata",
			ctx: providers.WithContext(context.Background(), &providers.ProviderContext{
				Credentials: &providers.ProviderCredentials{},
				Metadata: map[string]interface{}{
					"email":        "user@example.com",
					"access_token": "oauth-token-123",
				},
			}),
			params:        map[string]interface{}{},
			expectedEmail: "user@example.com",
			expectedToken: "oauth-token-123",
			expectedError: false,
		},

		// Priority 2: Passthrough auth from params
		{
			name: "passthrough auth with plain token",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"__passthrough_auth": map[string]interface{}{
					"token": "user@example.com:api-token-plain",
				},
			},
			expectedEmail: "user@example.com",
			expectedToken: "api-token-plain",
			expectedError: false,
		},
		{
			name: "passthrough auth with email and api_token",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"__passthrough_auth": map[string]interface{}{
					"email":     "user@example.com",
					"api_token": "api-token-pass",
				},
			},
			expectedEmail: "user@example.com",
			expectedToken: "api-token-pass",
			expectedError: false,
		},

		// Priority 3: Direct token parameter (backward compatibility)
		{
			name: "direct token parameter",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"token": "user@example.com:api-token-direct",
			},
			expectedEmail: "user@example.com",
			expectedToken: "api-token-direct",
			expectedError: false,
		},

		// Priority 4: Direct email/api_token parameters
		{
			name: "direct email and api_token parameters",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"email":     "user@example.com",
				"api_token": "api-token-direct-params",
			},
			expectedEmail: "user@example.com",
			expectedToken: "api-token-direct-params",
			expectedError: false,
		},
		{
			name: "direct email and access_token parameters",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"email":        "user@example.com",
				"access_token": "oauth-token-direct",
			},
			expectedEmail: "user@example.com",
			expectedToken: "oauth-token-direct",
			expectedError: false,
		},

		// Priority 5: Context value (legacy)
		{
			name:          "context value with token",
			ctx:           context.WithValue(context.Background(), jiraTokenKey, "user@example.com:api-token-ctx"),
			params:        map[string]interface{}{},
			expectedEmail: "user@example.com",
			expectedToken: "api-token-ctx",
			expectedError: false,
		},

		// Error cases
		{
			name:          "no credentials provided",
			ctx:           context.Background(),
			params:        map[string]interface{}{},
			expectedError: true,
			errorContains: "no authentication credentials found",
		},
		{
			name: "invalid token format - no colon",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"token": "invalid-token-no-colon",
			},
			expectedError: true,
			errorContains: "invalid token format",
		},
		{
			name: "missing api_token when email provided",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"email": "user@example.com",
			},
			expectedError: true,
			errorContains: "email provided but api_token missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewJiraProvider(logger, "test")

			email, token, err := provider.extractAuthToken(tt.ctx, tt.params)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedEmail, email, "Email mismatch")
				assert.Equal(t, tt.expectedToken, token, "Token mismatch")
			}
		})
	}
}

func TestJiraProvider_PassthroughAuth_WithEncryption(t *testing.T) {
	t.Skip("Skipping encryption test - requires complex setup with shared encryption keys")

	// This test would require:
	// 1. A shared encryption service instance between test and provider
	// 2. Or a way to inject the encryption service into the provider
	// 3. Or using the same deterministic key generation
	//
	// For now, the encryption/decryption functionality is tested
	// indirectly through integration tests and the actual encryption
	// service has its own unit tests.
}

func TestJiraProvider_PassthroughAuth_Priority(t *testing.T) {
	// This test verifies that the priority order is correctly followed
	logger := &observability.NoopLogger{}
	provider := NewJiraProvider(logger, "test")

	// Set up context with Priority 1 credentials
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "priority1@example.com:token1",
		},
	})

	// Also provide Priority 2, 3, and 4 credentials
	params := map[string]interface{}{
		"__passthrough_auth": map[string]interface{}{
			"token": "priority2@example.com:token2",
		},
		"token":     "priority3@example.com:token3",
		"email":     "priority4@example.com",
		"api_token": "token4",
	}

	// Priority 1 should win
	email, token, err := provider.extractAuthToken(ctx, params)
	require.NoError(t, err)
	assert.Equal(t, "priority1@example.com", email)
	assert.Equal(t, "token1", token)

	// Remove Priority 1, Priority 2 should win
	ctx = context.Background()
	email, token, err = provider.extractAuthToken(ctx, params)
	require.NoError(t, err)
	assert.Equal(t, "priority2@example.com", email)
	assert.Equal(t, "token2", token)

	// Remove Priority 2, Priority 3 should win
	delete(params, "__passthrough_auth")
	email, token, err = provider.extractAuthToken(ctx, params)
	require.NoError(t, err)
	assert.Equal(t, "priority3@example.com", email)
	assert.Equal(t, "token3", token)

	// Remove Priority 3, Priority 4 should win
	delete(params, "token")
	email, token, err = provider.extractAuthToken(ctx, params)
	require.NoError(t, err)
	assert.Equal(t, "priority4@example.com", email)
	assert.Equal(t, "token4", token)
}

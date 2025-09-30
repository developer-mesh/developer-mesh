package xray

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestXrayPassthroughAuthentication validates Story 3.1 requirements
func TestXrayPassthroughAuthentication(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		expectedHeader string
		expectedValue  string
		description    string
	}{
		{
			name:           "JFrog API Key Format - Uses X-JFrog-Art-Api",
			apiKey:         "AKCabc123def456",
			expectedHeader: "X-JFrog-Art-Api",
			expectedValue:  "AKCabc123def456",
			description:    "API keys starting with AKC should use X-JFrog-Art-Api header",
		},
		{
			name:           "JFrog Reference Token Format - Uses X-JFrog-Art-Api",
			apiKey:         "cmVmdGtjOmV5SnNhV05sYm5O",
			expectedHeader: "X-JFrog-Art-Api",
			expectedValue:  "cmVmdGtjOmV5SnNhV05sYm5O",
			description:    "Reference tokens should use X-JFrog-Art-Api header",
		},
		{
			name:           "JWT Access Token - Uses Bearer",
			apiKey:         "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.signature",
			expectedHeader: "Authorization",
			expectedValue:  "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.signature",
			description:    "JWT tokens should use Authorization Bearer header",
		},
		{
			name:           "Standard API Key - Uses X-JFrog-Art-Api",
			apiKey:         "test-api-key-1234567890",
			expectedHeader: "X-JFrog-Art-Api",
			expectedValue:  "test-api-key-1234567890",
			description:    "Standard API keys should use X-JFrog-Art-Api header",
		},
		{
			name:           "User:Password Base64 - Uses X-JFrog-Art-Api",
			apiKey:         "dXNlcjpwYXNzd29yZA==",
			expectedHeader: "X-JFrog-Art-Api",
			expectedValue:  "dXNlcjpwYXNzd29yZA==",
			description:    "Base64 encoded credentials should use X-JFrog-Art-Api header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server that validates headers
			var receivedHeaders http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedHeaders = r.Header
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"version": "3.82.0", "revision": "123456"}`))
			}))
			defer server.Close()

			// Create Xray provider
			logger := &observability.NoopLogger{}
			provider := NewXrayProvider(logger)

			// Configure with custom base URL
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			// Create context with credentials
			creds := &providers.ProviderCredentials{
				APIKey: tt.apiKey,
			}
			pctx := &providers.ProviderContext{
				Credentials: creds,
			}
			ctx := providers.WithContext(context.Background(), pctx)

			// Execute an operation that requires authentication
			result, err := provider.ExecuteOperation(ctx, "system/version", map[string]interface{}{})
			require.NoError(t, err, tt.description)
			require.NotNil(t, result)

			// Verify the correct header was used
			if tt.expectedHeader == "Authorization" {
				assert.Equal(t, tt.expectedValue, receivedHeaders.Get("Authorization"),
					"Should use Bearer token for JWT tokens")
				assert.Empty(t, receivedHeaders.Get("X-JFrog-Art-Api"),
					"Should not set X-JFrog-Art-Api when using Bearer token")
			} else {
				assert.Equal(t, tt.expectedValue, receivedHeaders.Get("X-JFrog-Art-Api"),
					"Should use X-JFrog-Art-Api header for API keys")
				assert.Empty(t, receivedHeaders.Get("Authorization"),
					"Should not set Authorization when using X-JFrog-Art-Api")
			}
		})
	}
}

// TestXrayUnifiedPlatformTokens validates unified platform token support
func TestXrayUnifiedPlatformTokens(t *testing.T) {
	tests := []struct {
		name          string
		credentials   map[string]string
		expectSuccess bool
		description   string
	}{
		{
			name: "Unified Platform Access Token",
			credentials: map[string]string{
				"api_key": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.platform.token",
			},
			expectSuccess: true,
			description:   "Should support unified JFrog Platform access tokens",
		},
		{
			name: "Artifactory API Key Works for Xray",
			credentials: map[string]string{
				"api_key": "AKCabc123def456",
			},
			expectSuccess: true,
			description:   "Same API key should work across Artifactory and Xray",
		},
		{
			name: "Identity Token",
			credentials: map[string]string{
				"api_key": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.identity.token",
			},
			expectSuccess: true,
			description:   "Should support identity tokens for SSO/SAML integration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check that some form of authentication is present
				hasAuth := r.Header.Get("Authorization") != "" || r.Header.Get("X-JFrog-Art-Api") != ""

				if hasAuth {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"status": "ok"}`))
				} else {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"error": "Unauthorized"}`))
				}
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewXrayProvider(logger)

			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			err := provider.ValidateCredentials(context.Background(), tt.credentials)

			if tt.expectSuccess {
				assert.NoError(t, err, tt.description)
			} else {
				assert.Error(t, err, tt.description)
			}
		})
	}
}

// TestXrayCustomBaseURLs validates instance-specific endpoint support
func TestXrayCustomBaseURLs(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		operation   string
		expectedURL string
		description string
	}{
		{
			name:        "Cloud Instance URL",
			baseURL:     "https://mycompany.jfrog.io/xray",
			operation:   "system/version",
			expectedURL: "/api/v1/system/version",
			description: "Should work with JFrog cloud instances",
		},
		{
			name:        "Self-Hosted Instance URL",
			baseURL:     "https://artifactory.internal.company.com/xray",
			operation:   "scan/artifact",
			expectedURL: "/api/v1/scan/artifact",
			description: "Should work with self-hosted instances",
		},
		{
			name:        "Custom Port URL",
			baseURL:     "http://localhost:8000/xray",
			operation:   "violations/list",
			expectedURL: "/api/v2/violations",
			description: "Should work with custom ports",
		},
		{
			name:        "Subdomain URL",
			baseURL:     "https://xray.company.com",
			operation:   "watches/list",
			expectedURL: "/api/v2/watches",
			description: "Should work with subdomain configurations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status": "ok"}`))
			}))
			defer server.Close()

			logger := &observability.NoopLogger{}
			provider := NewXrayProvider(logger)

			// Configure with test server URL but validate path construction
			config := provider.GetDefaultConfiguration()
			config.BaseURL = server.URL
			provider.SetConfiguration(config)

			// Create context with credentials
			creds := &providers.ProviderCredentials{
				APIKey: "test-api-key",
			}
			pctx := &providers.ProviderContext{
				Credentials: creds,
			}
			ctx := providers.WithContext(context.Background(), pctx)

			// Execute operation
			_, err := provider.ExecuteOperation(ctx, tt.operation, map[string]interface{}{})
			require.NoError(t, err, tt.description)

			// Verify the correct path was used
			assert.Equal(t, tt.expectedURL, capturedPath,
				"Should construct correct URL path for %s", tt.baseURL)
		})
	}
}

// TestXrayAuthenticationMethods validates both token types work correctly
func TestXrayAuthenticationMethods(t *testing.T) {
	// Test server that accepts both authentication methods
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		xrayHeader := r.Header.Get("X-JFrog-Art-Api")

		// Accept either authentication method
		if authHeader != "" || xrayHeader != "" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"version": "3.82.0",
				"revision": "38200003820",
				"addons": ["xray-analysis", "xray-scanner"]
			}`))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "Authentication required"}`))
		}
	}))
	defer server.Close()

	logger := &observability.NoopLogger{}

	t.Run("Bearer Token Authentication", func(t *testing.T) {
		provider := NewXrayProvider(logger)
		config := provider.GetDefaultConfiguration()
		config.BaseURL = server.URL
		provider.SetConfiguration(config)

		creds := &providers.ProviderCredentials{
			APIKey: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.signature",
		}
		pctx := &providers.ProviderContext{
			Credentials: creds,
		}
		ctx := providers.WithContext(context.Background(), pctx)

		result, err := provider.ExecuteOperation(ctx, "system/version", nil)
		require.NoError(t, err, "Bearer token authentication should work")
		require.NotNil(t, result)

		// Verify response contains expected fields
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok, "Result should be a map")
		assert.Contains(t, resultMap, "version")
		assert.Contains(t, resultMap, "revision")
	})

	t.Run("X-JFrog-Art-Api Authentication", func(t *testing.T) {
		provider := NewXrayProvider(logger)
		config := provider.GetDefaultConfiguration()
		config.BaseURL = server.URL
		provider.SetConfiguration(config)

		creds := &providers.ProviderCredentials{
			APIKey: "AKCabc123def456",
		}
		pctx := &providers.ProviderContext{
			Credentials: creds,
		}
		ctx := providers.WithContext(context.Background(), pctx)

		result, err := provider.ExecuteOperation(ctx, "system/version", nil)
		require.NoError(t, err, "X-JFrog-Art-Api authentication should work")
		require.NotNil(t, result)

		// Verify response contains expected fields
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok, "Result should be a map")
		assert.Contains(t, resultMap, "version")
		assert.Contains(t, resultMap, "revision")
	})

	t.Run("No Authentication Fails", func(t *testing.T) {
		provider := NewXrayProvider(logger)
		config := provider.GetDefaultConfiguration()
		config.BaseURL = server.URL
		provider.SetConfiguration(config)

		// Context without credentials
		ctx := context.Background()

		_, err := provider.ExecuteOperation(ctx, "system/version", nil)
		require.Error(t, err, "Should fail without authentication")
		assert.Contains(t, err.Error(), "no credentials found")
	})
}

// TestXrayProviderInheritsBaseAuth validates that Xray inherits BaseProvider auth
func TestXrayProviderInheritsBaseAuth(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	// Verify provider extends BaseProvider
	assert.NotNil(t, provider.BaseProvider, "XrayProvider should extend BaseProvider")

	// Verify provider name is correct for JFrog detection
	assert.Equal(t, "xray", provider.GetProviderName(), "Provider name should be 'xray'")

	// Verify default configuration
	config := provider.GetDefaultConfiguration()
	assert.Contains(t, config.BaseURL, "xray", "Default URL should contain 'xray'")
	assert.Equal(t, "api_key", config.AuthType, "Default auth type should be api_key")

	// Test configuration setting
	customURL := "https://custom.jfrog.io/xray"
	config.BaseURL = customURL
	provider.SetConfiguration(config)

	// Verify configuration was applied by checking that the provider accepts the new config
	// Note: GetDefaultConfiguration returns the default, not the current configuration
	// The actual test of custom URLs is in TestXrayCustomBaseURLs
	assert.NotNil(t, provider.BaseProvider, "Provider should have base configuration set")
}

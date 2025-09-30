package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
)

// TestJFrogAuthenticationHeaders tests the JFrog-specific authentication headers
func TestJFrogAuthenticationHeaders(t *testing.T) {
	tests := []struct {
		name           string
		providerName   string
		authType       string
		credentials    *ProviderCredentials
		expectedHeader string
		expectedValue  string
	}{
		{
			name:         "Artifactory with API key - X-JFrog-Art-Api header",
			providerName: "artifactory",
			authType:     "api_key",
			credentials: &ProviderCredentials{
				APIKey: "AKCabc123def456",
			},
			expectedHeader: "X-JFrog-Art-Api",
			expectedValue:  "AKCabc123def456",
		},
		{
			name:         "Artifactory with Bearer token",
			providerName: "artifactory",
			authType:     "bearer",
			credentials: &ProviderCredentials{
				Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ",
			},
			expectedHeader: "Authorization",
			expectedValue:  "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ",
		},
		{
			name:         "Xray with API key - X-JFrog-Art-Api header",
			providerName: "xray",
			authType:     "api_key",
			credentials: &ProviderCredentials{
				APIKey: "cmVmdGtjOmV5SnNhV05sYm5O",
			},
			expectedHeader: "X-JFrog-Art-Api",
			expectedValue:  "cmVmdGtjOmV5SnNhV05sYm5O",
		},
		{
			name:         "Xray with Bearer token",
			providerName: "xray",
			authType:     "bearer",
			credentials: &ProviderCredentials{
				Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkphbmUgRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ",
			},
			expectedHeader: "Authorization",
			expectedValue:  "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkphbmUgRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ",
		},
		{
			name:         "Artifactory with API key in bearer auth type",
			providerName: "artifactory",
			authType:     "bearer",
			credentials: &ProviderCredentials{
				APIKey: "AKCabc123def456",
			},
			expectedHeader: "X-JFrog-Art-Api",
			expectedValue:  "AKCabc123def456",
		},
		{
			name:         "Non-JFrog provider with API key - standard X-API-Key",
			providerName: "github",
			authType:     "api_key",
			credentials: &ProviderCredentials{
				APIKey: "ghp_abc123def456",
			},
			expectedHeader: "X-API-Key",
			expectedValue:  "ghp_abc123def456",
		},
		{
			name:         "Non-JFrog provider with Bearer token",
			providerName: "github",
			authType:     "bearer",
			credentials: &ProviderCredentials{
				Token: "ghp_xyz789",
			},
			expectedHeader: "Authorization",
			expectedValue:  "Bearer ghp_xyz789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a provider with the specified name
			logger := &observability.NoopLogger{}
			provider := NewBaseProvider(tt.providerName, "v1", "https://example.com", logger)
			provider.SetConfiguration(ProviderConfig{
				BaseURL:  "https://example.com",
				AuthType: tt.authType,
			})

			// Create context with credentials
			ctx := WithContext(context.Background(), &ProviderContext{
				TenantID:    "test-tenant",
				Credentials: tt.credentials,
			})

			// Create a test request
			req, err := http.NewRequest("GET", "https://example.com/test", nil)
			assert.NoError(t, err)

			// Apply authentication
			err = provider.applyAuthentication(ctx, req)
			assert.NoError(t, err)

			// Check the expected header
			assert.Equal(t, tt.expectedValue, req.Header.Get(tt.expectedHeader),
				"Expected header %s to have value %s", tt.expectedHeader, tt.expectedValue)
		})
	}
}

// TestDetectJFrogAuthType tests the JFrog authentication type detection
func TestDetectJFrogAuthType(t *testing.T) {
	tests := []struct {
		name         string
		credentials  *ProviderCredentials
		expectedType string
	}{
		{
			name: "API key with AKC prefix",
			credentials: &ProviderCredentials{
				APIKey: "AKCabc123def456",
			},
			expectedType: "artifactory_api_key",
		},
		{
			name: "API key with cmVm prefix",
			credentials: &ProviderCredentials{
				APIKey: "cmVmdGtjOmV5SnNhV05sYm5O",
			},
			expectedType: "artifactory_api_key",
		},
		{
			name: "Regular API key (no specific prefix)",
			credentials: &ProviderCredentials{
				APIKey: "regular_api_key_123",
			},
			expectedType: "artifactory_api_key",
		},
		{
			name: "JWT access token (with dots)",
			credentials: &ProviderCredentials{
				Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			},
			expectedType: "access_token",
		},
		{
			name: "Short token (not JWT)",
			credentials: &ProviderCredentials{
				Token: "short_token",
			},
			expectedType: "access_token",
		},
		{
			name:         "No credentials",
			credentials:  &ProviderCredentials{},
			expectedType: "unknown",
		},
		{
			name:         "Nil credentials",
			credentials:  nil,
			expectedType: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &observability.NoopLogger{}
			provider := NewBaseProvider("artifactory", "v1", "https://example.com", logger)

			authType := provider.detectJFrogAuthType(tt.credentials)
			assert.Equal(t, tt.expectedType, authType)
		})
	}
}

// TestJFrogAuthenticationFallback tests fallback behavior for JFrog authentication
func TestJFrogAuthenticationFallback(t *testing.T) {
	// Test server to capture the headers
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	tests := []struct {
		name           string
		providerName   string
		credentials    *ProviderCredentials
		expectedHeader string
		expectedPrefix string
	}{
		{
			name:         "Artifactory auto-detects API key",
			providerName: "artifactory",
			credentials: &ProviderCredentials{
				APIKey: "AKCabc123def456",
			},
			expectedHeader: "X-JFrog-Art-Api",
			expectedPrefix: "AKCabc123def456",
		},
		{
			name:         "Artifactory auto-detects access token",
			providerName: "artifactory",
			credentials: &ProviderCredentials{
				Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			},
			expectedHeader: "Authorization",
			expectedPrefix: "Bearer ",
		},
		{
			name:         "Xray auto-detects API key",
			providerName: "xray",
			credentials: &ProviderCredentials{
				APIKey: "cmVmdGtjOmV5SnNhV05sYm5O",
			},
			expectedHeader: "X-JFrog-Art-Api",
			expectedPrefix: "cmVmdGtjOmV5SnNhV05sYm5O",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create provider
			logger := &observability.NoopLogger{}
			provider := NewBaseProvider(tt.providerName, "v1", server.URL, logger)
			provider.SetConfiguration(ProviderConfig{
				BaseURL:  server.URL,
				AuthType: "bearer", // Use bearer to test the auto-detection
			})

			// Create context with credentials
			ctx := WithContext(context.Background(), &ProviderContext{
				TenantID:    "test-tenant",
				Credentials: tt.credentials,
			})

			// Execute a request
			_, err := provider.ExecuteHTTPRequest(ctx, "GET", "/test", nil, nil)
			assert.NoError(t, err)

			// Check the captured headers
			headerValue := capturedHeaders.Get(tt.expectedHeader)
			assert.NotEmpty(t, headerValue, "Expected header %s to be set", tt.expectedHeader)
			if tt.expectedPrefix != "" && tt.expectedHeader == "Authorization" {
				assert.Contains(t, headerValue, tt.expectedPrefix)
			} else if tt.expectedPrefix != "" {
				assert.Equal(t, tt.expectedPrefix, headerValue)
			}
		})
	}
}

// TestBackwardCompatibility tests that existing non-JFrog providers still work
func TestBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name           string
		providerName   string
		authType       string
		credentials    *ProviderCredentials
		expectedHeader string
		expectedValue  string
	}{
		{
			name:         "Harness with API key",
			providerName: "harness",
			authType:     "api_key",
			credentials: &ProviderCredentials{
				APIKey: "harness_api_key_123",
			},
			expectedHeader: "x-api-key",
			expectedValue:  "harness_api_key_123",
		},
		{
			name:         "Nexus with API key",
			providerName: "nexus",
			authType:     "api_key",
			credentials: &ProviderCredentials{
				APIKey: "nexus_api_key_456",
			},
			expectedHeader: "Authorization",
			expectedValue:  "NX-APIKEY nexus_api_key_456",
		},
		{
			name:         "Generic provider with bearer token",
			providerName: "generic",
			authType:     "bearer",
			credentials: &ProviderCredentials{
				Token: "generic_token_789",
			},
			expectedHeader: "Authorization",
			expectedValue:  "Bearer generic_token_789",
		},
		{
			name:         "Generic provider with basic auth",
			providerName: "generic",
			authType:     "basic",
			credentials: &ProviderCredentials{
				Username: "testuser",
				Password: "testpass",
			},
			expectedHeader: "Authorization",
			expectedValue:  "Basic dGVzdHVzZXI6dGVzdHBhc3M=", // base64 of testuser:testpass
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create provider
			logger := &observability.NoopLogger{}
			provider := NewBaseProvider(tt.providerName, "v1", "https://example.com", logger)
			provider.SetConfiguration(ProviderConfig{
				BaseURL:  "https://example.com",
				AuthType: tt.authType,
			})

			// Create context with credentials
			ctx := WithContext(context.Background(), &ProviderContext{
				TenantID:    "test-tenant",
				Credentials: tt.credentials,
			})

			// Create a test request
			req, err := http.NewRequest("GET", "https://example.com/test", nil)
			assert.NoError(t, err)

			// Apply authentication
			err = provider.applyAuthentication(ctx, req)
			assert.NoError(t, err)

			// Check the expected header
			assert.Equal(t, tt.expectedValue, req.Header.Get(tt.expectedHeader),
				"Expected header %s to have value %s", tt.expectedHeader, tt.expectedValue)
		})
	}
}

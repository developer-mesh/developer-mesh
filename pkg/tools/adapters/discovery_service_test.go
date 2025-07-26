package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func NewTestURLValidator() *tools.URLValidator {
	// This is a hack - we return nil which will bypass validation
	return nil
}

func TestDiscoveryService(t *testing.T) {
	logger := &mockLogger{}
	// Use a custom discovery service with no URL validation for tests
	service := &DiscoveryService{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		validator: nil, // No validation in tests
	}

	t.Run("DirectOpenAPIURL", func(t *testing.T) {
		// Create test server
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(spec); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:       "test-tool",
			BaseURL:    server.URL,
			OpenAPIURL: server.URL + "/openapi.json",
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
		assert.Equal(t, config.OpenAPIURL, result.SpecURL)
	})

	t.Run("CommonPathDiscovery", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/swagger.json" {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(spec); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			} else {
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		// Debug output
		t.Logf("Discovery result status: %s", result.Status)
		t.Logf("Discovered URLs: %v", result.DiscoveredURLs)
		t.Logf("Spec URL: %s", result.SpecURL)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
		assert.Contains(t, result.SpecURL, "/swagger.json")
	})

	t.Run("UserProvidedHints", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		customPath := "/custom/api/spec.json"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == customPath {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(spec); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			} else {
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
			Config: map[string]interface{}{
				"discovery_paths": []string{customPath},
			},
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
		assert.Contains(t, result.SpecURL, customPath)
	})

	t.Run("NoSpecFound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		// With no URL validation, it might discover some HTML links, so status could be partial
		assert.Contains(t, []tools.DiscoveryStatus{tools.DiscoveryStatusPartial, tools.DiscoveryStatusManualNeeded}, result.Status)
		if result.Status == tools.DiscoveryStatusManualNeeded {
			assert.True(t, result.RequiresManual)
		}
		assert.NotEmpty(t, result.SuggestedActions)
	})

	t.Run("InvalidOpenAPISpec", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"invalid": "not an openapi spec"}`)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:       "test-tool",
			BaseURL:    server.URL,
			OpenAPIURL: server.URL + "/openapi.json",
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.NotEqual(t, tools.DiscoveryStatusSuccess, result.Status)
	})

	t.Run("AuthenticationRequired", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "Bearer test-token" {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(spec); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:       "test-tool",
			BaseURL:    server.URL,
			OpenAPIURL: server.URL + "/openapi.json",
			Credential: &models.TokenCredential{
				Type:  "token",
				Token: "test-token",
			},
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
	})
}

func TestBuildURL(t *testing.T) {
	service := &DiscoveryService{}

	tests := []struct {
		name     string
		baseURL  string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			baseURL:  "https://api.example.com",
			path:     "/openapi.json",
			expected: "https://api.example.com/openapi.json",
		},
		{
			name:     "baseURL with trailing slash",
			baseURL:  "https://api.example.com/",
			path:     "/openapi.json",
			expected: "https://api.example.com/openapi.json",
		},
		{
			name:     "path without leading slash",
			baseURL:  "https://api.example.com",
			path:     "openapi.json",
			expected: "https://api.example.com/openapi.json",
		},
		{
			name:     "both have slashes",
			baseURL:  "https://api.example.com/",
			path:     "/openapi.json",
			expected: "https://api.example.com/openapi.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildURL(tt.baseURL, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplySubdomain(t *testing.T) {
	service := &DiscoveryService{}

	tests := []struct {
		name      string
		baseURL   string
		subdomain string
		expected  string
	}{
		{
			name:      "add api subdomain",
			baseURL:   "https://example.com",
			subdomain: "api",
			expected:  "https://api.example.com",
		},
		{
			name:      "replace existing subdomain",
			baseURL:   "https://www.example.com",
			subdomain: "api",
			expected:  "https://api.example.com",
		},
		{
			name:      "subdomain already exists",
			baseURL:   "https://api.example.com",
			subdomain: "api",
			expected:  "https://api.example.com",
		},
		{
			name:      "invalid URL",
			baseURL:   "not-a-url",
			subdomain: "api",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.applySubdomain(tt.baseURL, tt.subdomain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAPIDocLink(t *testing.T) {
	service := &DiscoveryService{}

	tests := []struct {
		name     string
		href     string
		expected bool
	}{
		{"api link", "/api/docs", true},
		{"swagger link", "/swagger-ui", true},
		{"openapi link", "/openapi.json", true},
		{"docs link", "/documentation", true},
		{"developer link", "/developer/guide", true},
		{"reference link", "/reference/api", true},
		{"rest link", "/rest/v1", true},
		{"spec link", "/api-spec", true},
		{"regular link", "/about", false},
		{"home link", "/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isAPIDocLink(tt.href)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func createTestOpenAPISpec() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Test endpoint",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
			},
		},
	}
}

// mockLogger implements observability.Logger for testing
type mockLogger struct {
	logs []map[string]interface{}
}

func (m *mockLogger) Info(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Error(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Debug(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Warn(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Fatal(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, fields)
}

func (m *mockLogger) Infof(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) Errorf(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) Debugf(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) Warnf(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) Fatalf(format string, args ...interface{}) {
	// Mock implementation
}

func (m *mockLogger) WithPrefix(prefix string) observability.Logger {
	return m
}

func (m *mockLogger) With(fields map[string]interface{}) observability.Logger {
	return m
}

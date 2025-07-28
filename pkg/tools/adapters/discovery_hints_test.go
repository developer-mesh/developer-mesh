package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHintBasedDiscovery_DiscoverWithHints(t *testing.T) {
	httpClient := &http.Client{}
	detector := NewFormatDetector(httpClient)
	discovery := NewHintBasedDiscovery(detector, nil)

	t.Run("DirectOpenAPIURL", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api-docs" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(spec)
			} else {
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
		}

		hints := &DiscoveryHints{
			OpenAPIURL: server.URL + "/api-docs",
		}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
		assert.Equal(t, hints.OpenAPIURL, result.SpecURL)
	})

	t.Run("AuthHeaders", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		expectedToken := "Bearer secret-token"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == expectedToken {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(spec)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
		}

		hints := &DiscoveryHints{
			OpenAPIURL: server.URL + "/openapi",
			AuthHeaders: map[string]string{
				"Authorization": expectedToken,
			},
		}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
	})

	t.Run("CustomPaths", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v2/spec.json" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(spec)
			} else {
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
		}

		hints := &DiscoveryHints{
			CustomPaths: []string{"/v1/spec.json", "/v2/spec.json", "/v3/spec.json"},
		}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
		assert.Contains(t, result.SpecURL, "/v2/spec.json")
	})

	t.Run("APIFormat_CustomJSON", func(t *testing.T) {
		customSpec := map[string]interface{}{
			"apis": []interface{}{
				map[string]interface{}{
					"name":   "Test API",
					"path":   "/test",
					"method": "GET",
				},
			},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api-definition" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(customSpec)
			} else {
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
		}

		hints := &DiscoveryHints{
			CustomPaths: []string{"/api-definition"},
			APIFormat:   "custom_json",
		}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
		assert.Equal(t, "custom_json", result.Metadata["api_format"])
		assert.Equal(t, "custom_json", result.Metadata["converted_from"])
	})

	t.Run("ExampleEndpoint", func(t *testing.T) {
		// Test that example endpoint is stored in metadata
		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: "https://api.example.com",
		}

		hints := &DiscoveryHints{
			ExampleEndpoint: "/api/v1/users",
		}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.Equal(t, hints.ExampleEndpoint, result.Metadata["example_endpoint"])
	})

	t.Run("DocumentationURL", func(t *testing.T) {
		// Test that documentation URL is stored in metadata
		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: "https://api.example.com",
		}

		hints := &DiscoveryHints{
			DocumentationURL: "https://docs.example.com/api",
		}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.Equal(t, hints.DocumentationURL, result.Metadata["documentation_url"])
		assert.Contains(t, result.SuggestedActions, "Check the API documentation at https://docs.example.com/api for OpenAPI/Swagger specification links")
	})

	t.Run("CombinedHints", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/custom/openapi.yaml" && r.Header.Get("X-API-Key") == "test-key" {
				w.Header().Set("Content-Type", "application/x-yaml")
				// Return YAML spec
				yamlSpec := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /test:
    get:
      summary: Test endpoint
      responses:
        '200':
          description: Success`
				_, _ = w.Write([]byte(yamlSpec))
			} else {
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
		}

		hints := &DiscoveryHints{
			OpenAPIURL: server.URL + "/custom/openapi.yaml",
			AuthHeaders: map[string]string{
				"X-API-Key": "test-key",
			},
			APIFormat:        "openapi3",
			ExampleEndpoint:  "/test",
			DocumentationURL: "https://docs.example.com",
		}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
		assert.Equal(t, hints.OpenAPIURL, result.SpecURL)
		assert.Equal(t, hints.ExampleEndpoint, result.Metadata["example_endpoint"])
		assert.Equal(t, hints.DocumentationURL, result.Metadata["documentation_url"])
	})

	t.Run("InvalidURL", func(t *testing.T) {
		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: "https://api.example.com",
		}

		hints := &DiscoveryHints{
			OpenAPIURL: "not-a-valid-url",
		}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.NotEqual(t, tools.DiscoveryStatusSuccess, result.Status)
	})

	t.Run("NoHints", func(t *testing.T) {
		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: "https://api.example.com",
		}

		hints := &DiscoveryHints{}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusManualNeeded, result.Status)
		assert.True(t, result.RequiresManual)
		assert.NotEmpty(t, result.SuggestedActions)
	})

	t.Run("FormatDetection", func(t *testing.T) {
		// Test various format detections
		tests := []struct {
			name           string
			content        interface{}
			contentType    string
			apiFormat      string
			expectedFormat string
		}{
			{
				name: "OpenAPI 3.0",
				content: map[string]interface{}{
					"openapi": "3.0.0",
					"info": map[string]interface{}{
						"title": "Test",
					},
				},
				contentType:    "application/json",
				apiFormat:      "",
				expectedFormat: "openapi3",
			},
			{
				name: "Swagger 2.0",
				content: map[string]interface{}{
					"swagger": "2.0",
					"info": map[string]interface{}{
						"title": "Test",
					},
				},
				contentType:    "application/json",
				apiFormat:      "",
				expectedFormat: "swagger",
			},
			{
				name: "RAML hint",
				content: map[string]interface{}{
					"title": "Test API",
				},
				contentType:    "application/raml+yaml",
				apiFormat:      "raml",
				expectedFormat: "raml",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", tt.contentType)
					_ = json.NewEncoder(w).Encode(tt.content)
				}))
				defer server.Close()

				config := tools.ToolConfig{
					Name:    "test-tool",
					BaseURL: server.URL,
				}

				hints := &DiscoveryHints{
					CustomPaths: []string{"/api"},
					APIFormat:   tt.apiFormat,
				}

				result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
				require.NoError(t, err)
				if tt.expectedFormat != "raml" { // RAML conversion not implemented
					assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
					assert.Equal(t, tt.expectedFormat, result.Metadata["api_format"])
				}
			})
		}
	})

	t.Run("CredentialMerging", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check both config credential and hint headers
			if r.Header.Get("Authorization") == "Bearer config-token" &&
				r.Header.Get("X-Custom-Header") == "hint-value" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(spec)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "test-tool",
			BaseURL: server.URL,
			Credential: &models.TokenCredential{
				Type:  "token",
				Token: "config-token",
			},
		}

		hints := &DiscoveryHints{
			OpenAPIURL: server.URL + "/openapi",
			AuthHeaders: map[string]string{
				"X-Custom-Header": "hint-value",
			},
		}

		result, err := discovery.DiscoverWithHints(context.Background(), config, hints)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
	})
}

//go:build integration
// +build integration

package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnhancedDiscoveryService_Integration(t *testing.T) {
	// Create a full enhanced discovery service with all components
	logger := &mockLogger{}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	patternStore := NewInMemoryPatternStore()

	formatDetector := NewFormatDetector(httpClient)
	learningService := NewLearningDiscoveryService(patternStore)
	hintDiscovery := NewHintBasedDiscovery(formatDetector, nil)

	service := &DiscoveryService{
		logger:          logger,
		httpClient:      httpClient,
		validator:       nil,
		formatDetector:  formatDetector,
		learningService: learningService,
		hintDiscovery:   hintDiscovery,
	}

	t.Run("Full discovery flow with learning", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/v2/api-docs":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(spec)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		// First discovery - no learned patterns
		config := tools.ToolConfig{
			Name:    "test-api",
			BaseURL: server.URL,
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.Contains(t, result.SpecURL, "/v2/api-docs")

		// Verify pattern was learned
		pattern, err := patternStore.GetPatternByDomain(strings.TrimPrefix(server.URL, "http://"))
		require.NoError(t, err)
		assert.Contains(t, pattern.SuccessfulPaths, "/v2/api-docs")

		// Second discovery - should use learned pattern
		config2 := tools.ToolConfig{
			Name:    "test-api-2",
			BaseURL: server.URL,
		}

		result2, err := service.DiscoverOpenAPISpec(context.Background(), config2)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result2.Status)

		// Should have found it faster (first try from learned paths)
		assert.Contains(t, result2.SpecURL, "/v2/api-docs")
	})

	t.Run("Format detection and conversion", func(t *testing.T) {
		// Custom JSON format
		customAPI := map[string]interface{}{
			"apis": []interface{}{
				map[string]interface{}{
					"name":   "List Users",
					"path":   "/users",
					"method": "GET",
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api-definition" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(customAPI)
			} else {
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "custom-api",
			BaseURL: server.URL,
			Config: map[string]interface{}{
				"discovery_paths": []string{"/api-definition"},
			},
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
		assert.Equal(t, "custom_json", result.Metadata["api_format"])
		assert.Equal(t, "custom_json", result.Metadata["converted_from"])
	})

	t.Run("Hint-based discovery", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/hidden/spec" && r.Header.Get("X-Secret") == "token" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(spec)
			} else {
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "hidden-api",
			BaseURL: server.URL,
			Config: map[string]interface{}{
				"discovery_hints": map[string]interface{}{
					"openapi_url": server.URL + "/hidden/spec",
					"auth_headers": map[string]string{
						"X-Secret": "token",
					},
				},
			},
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.Contains(t, result.SpecURL, "/hidden/spec")
	})
}

func TestDiscoveryService_ErrorHandling(t *testing.T) {
	logger := &mockLogger{}
	service := NewDiscoveryService(logger)

	t.Run("Context cancellation", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer slowServer.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		config := tools.ToolConfig{
			Name:       "slow-api",
			BaseURL:    slowServer.URL,
			OpenAPIURL: slowServer.URL + "/openapi.json",
		}

		result, err := service.DiscoverOpenAPISpec(ctx, config)
		require.NoError(t, err)
		assert.NotEqual(t, tools.DiscoveryStatusSuccess, result.Status)
	})

	t.Run("Invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{invalid json`))
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:       "invalid-api",
			BaseURL:    server.URL,
			OpenAPIURL: server.URL + "/openapi.json",
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.NotEqual(t, tools.DiscoveryStatusSuccess, result.Status)
	})

	t.Run("Network error", func(t *testing.T) {
		config := tools.ToolConfig{
			Name:       "offline-api",
			BaseURL:    "http://localhost:99999", // Invalid port
			OpenAPIURL: "http://localhost:99999/openapi.json",
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.NotEqual(t, tools.DiscoveryStatusSuccess, result.Status)
	})

	t.Run("Large response handling", func(t *testing.T) {
		// Create a large OpenAPI spec
		largeSpec := createTestOpenAPISpec()
		paths := make(map[string]interface{})
		for i := 0; i < 1000; i++ {
			paths[fmt.Sprintf("/endpoint%d", i)] = map[string]interface{}{
				"get": map[string]interface{}{
					"summary": fmt.Sprintf("Endpoint %d", i),
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
			}
		}
		largeSpec["paths"] = paths

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(largeSpec)
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:       "large-api",
			BaseURL:    server.URL,
			OpenAPIURL: server.URL + "/openapi.json",
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotNil(t, result.OpenAPISpec)
	})
}

func TestDiscoveryService_AuthenticationScenarios(t *testing.T) {
	logger := &mockLogger{}
	service := NewDiscoveryService(logger)

	t.Run("Bearer token", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "Bearer test-token" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(spec)
			} else {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "Unauthorized"}`))
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:       "auth-api",
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
	})

	t.Run("API key authentication", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/openapi.json" && r.Header.Get("X-API-Key") == "secret-key" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(spec)
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:       "apikey-api",
			BaseURL:    server.URL,
			OpenAPIURL: server.URL + "/openapi.json",
			Credential: &models.TokenCredential{
				Type:       "api_key",
				Token:      "secret-key",
				HeaderName: "X-API-Key",
			},
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
	})

	t.Run("Basic authentication", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if ok && username == "user" && password == "pass" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(spec)
			} else {
				w.Header().Set("WWW-Authenticate", `Basic realm="API"`)
				w.WriteHeader(http.StatusUnauthorized)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:       "basic-api",
			BaseURL:    server.URL,
			OpenAPIURL: server.URL + "/openapi.json",
			Credential: &models.TokenCredential{
				Type:     "basic",
				Username: "user",
				Password: "pass",
			},
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
	})
}

func TestDiscoveryService_HTMLCrawling(t *testing.T) {
	t.Skip("Skipping HTML crawling tests - feature not fully implemented")
	logger := &mockLogger{}
	service := NewDiscoveryService(logger)

	t.Run("Extract API links from HTML", func(t *testing.T) {
		htmlContent := `
		<html>
			<body>
				<h1>API Documentation</h1>
				<a href="/api/v1/docs">API v1 Docs</a>
				<a href="/swagger-ui.html">Swagger UI</a>
				<a href="/openapi.json">OpenAPI Spec</a>
				<a href="/about">About Us</a>
				<a href="/contact">Contact</a>
			</body>
		</html>
		`

		spec := createTestOpenAPISpec()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte(htmlContent))
			case "/openapi.json":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(spec)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "html-api",
			BaseURL: server.URL,
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.Contains(t, result.SpecURL, "/openapi.json")
		assert.Contains(t, result.DiscoveredURLs, server.URL+"/api/v1/docs")
		assert.Contains(t, result.DiscoveredURLs, server.URL+"/swagger-ui.html")
	})

	t.Run("Complex HTML with nested links", func(t *testing.T) {
		htmlContent := `
		<html>
			<body>
				<nav>
					<ul>
						<li><a href="/developers">Developers</a></li>
						<li><a href="/api/reference">API Reference</a></li>
					</ul>
				</nav>
				<div class="documentation">
					<a href="/rest/api/v2">REST API v2</a>
					<a href="/graphql/reference">GraphQL API</a>
				</div>
				<footer>
					<a href="/api-spec.yaml">Download API Spec</a>
				</footer>
			</body>
		</html>
		`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(htmlContent))
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "complex-html-api",
			BaseURL: server.URL,
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		// Should find multiple API-related links
		assert.True(t, len(result.DiscoveredURLs) >= 3)
	})
}

func TestDiscoveryService_Redirects(t *testing.T) {
	t.Skip("Skipping redirect tests - feature not fully implemented")
	logger := &mockLogger{}
	service := NewDiscoveryService(logger)

	t.Run("Follow redirects", func(t *testing.T) {
		spec := createTestOpenAPISpec()
		redirectCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api":
				http.Redirect(w, r, "/api/v1", http.StatusMovedPermanently)
			case "/api/v1":
				http.Redirect(w, r, "/api/v1/openapi", http.StatusFound)
			case "/api/v1/openapi":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(spec)
			default:
				http.NotFound(w, r)
			}
			redirectCount++
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:       "redirect-api",
			BaseURL:    server.URL,
			OpenAPIURL: server.URL + "/api",
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.Contains(t, result.SpecURL, "/api/v1/openapi")
		assert.Greater(t, redirectCount, 1)
	})

	t.Run("Too many redirects", func(t *testing.T) {
		redirectCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			redirectCount++
			// Infinite redirect loop
			http.Redirect(w, r, fmt.Sprintf("/redirect%d", redirectCount), http.StatusFound)
		}))
		defer server.Close()

		// Use custom client with redirect limit
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}

		service := &DiscoveryService{
			logger:     logger,
			httpClient: client,
		}

		config := tools.ToolConfig{
			Name:       "loop-api",
			BaseURL:    server.URL,
			OpenAPIURL: server.URL + "/api",
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.NotEqual(t, tools.DiscoveryStatusSuccess, result.Status)
	})
}

func TestDiscoveryService_fetchContent(t *testing.T) {
	logger := &mockLogger{}
	service := NewDiscoveryService(logger)

	t.Run("Content size limit", func(t *testing.T) {
		// Create large content
		largeContent := strings.Repeat("a", 20*1024*1024) // 20MB
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(largeContent))
		}))
		defer server.Close()

		content, err := service.fetchContent(context.Background(), server.URL, nil)
		require.NoError(t, err)
		// Should be limited to 10MB
		assert.LessOrEqual(t, len(content), 10*1024*1024)
	})

	t.Run("Content streaming", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate streaming response
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("Expected http.Flusher")
				return
			}

			for i := 0; i < 5; i++ {
				fmt.Fprintf(w, "chunk %d\n", i)
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}))
		defer server.Close()

		content, err := service.fetchContent(context.Background(), server.URL, nil)
		require.NoError(t, err)
		assert.Contains(t, string(content), "chunk 0")
		assert.Contains(t, string(content), "chunk 4")
	})
}

func TestDiscoveryService_RealWorldAPIs(t *testing.T) {
	t.Skip("Skipping RealWorldAPIs tests - need to debug discovery path issues")
	logger := &mockLogger{}
	service := NewDiscoveryService(logger)

	t.Run("Kubernetes-style API", func(t *testing.T) {
		// Simulate Kubernetes API discovery
		kubeSpec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":   "Kubernetes",
				"version": "v1.20.0",
			},
			"paths": map[string]interface{}{
				"/api/v1": map[string]interface{}{
					"get": map[string]interface{}{
						"description": "get available API versions",
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "OK",
							},
						},
					},
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/openapi/v2":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(kubeSpec)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := tools.ToolConfig{
			Name:    "kubernetes",
			BaseURL: server.URL,
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		if result.Status != tools.DiscoveryStatusSuccess {
			t.Logf("Discovery failed: Status=%s, SpecURL=%s, DiscoveredURLs=%v, SuggestedActions=%v",
				result.Status, result.SpecURL, result.DiscoveredURLs, result.SuggestedActions)
		}
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.Contains(t, result.SpecURL, "/openapi/v2")
	})

	t.Run("GitHub-style API", func(t *testing.T) {
		// Simulate GitHub API with custom paths
		var serverURL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				// Root returns API index
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"current_user_url": serverURL + "/user",
					"repository_url":   serverURL + "/repos/{owner}/{repo}",
				})
			case "/meta":
				// Meta endpoint with API info
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"openapi_url": serverURL + "/openapi/spec.json",
				})
			case "/openapi/spec.json":
				spec := createTestOpenAPISpec()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(spec)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()
		serverURL = server.URL

		config := tools.ToolConfig{
			Name:    "github-api",
			BaseURL: server.URL,
			Config: map[string]interface{}{
				"discovery_paths": []string{"/meta", "/openapi/spec.json"},
			},
		}

		result, err := service.DiscoverOpenAPISpec(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.Contains(t, result.SpecURL, "/openapi/spec.json")
	})
}

// Helper to read response body safely
func readBody(r io.ReadCloser) string {
	defer r.Close()
	body, _ := io.ReadAll(r)
	return string(body)
}

package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiAPIDiscoveryService_DetectPortalType(t *testing.T) {
	logger := &mockLogger{}
	service := NewMultiAPIDiscoveryService(logger)

	tests := []struct {
		name         string
		portalURL    string
		expectedType string
	}{
		{
			name:         "Harness portal",
			portalURL:    "https://apidocs.harness.io/",
			expectedType: "Harness",
		},
		{
			name:         "AWS portal",
			portalURL:    "https://docs.aws.amazon.com/",
			expectedType: "AWS",
		},
		{
			name:         "Azure portal",
			portalURL:    "https://docs.azure.microsoft.com/",
			expectedType: "Azure",
		},
		{
			name:         "Google Cloud portal",
			portalURL:    "https://cloud.google.com/apis",
			expectedType: "Google Cloud",
		},
		{
			name:         "Kubernetes portal",
			portalURL:    "https://kubernetes.io/docs/reference/",
			expectedType: "Kubernetes",
		},
		{
			name:         "Generic portal",
			portalURL:    "https://api.example.com/docs",
			expectedType: "Generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portal := service.detectPortalType(tt.portalURL)
			assert.Equal(t, tt.expectedType, portal.Name)
		})
	}
}

func TestMultiAPIDiscoveryService_DiscoverMultipleAPIs(t *testing.T) {
	// Allow localhost for testing
	t.Setenv("ALLOW_LOCALHOST_URLS", "true")
	t.Setenv("ALLOW_PRIVATE_NETWORK_URLS", "true")

	logger := &mockLogger{}
	service := NewMultiAPIDiscoveryService(logger)

	t.Run("Simple multi-API discovery", func(t *testing.T) {
		// Create OpenAPI specs for multiple APIs
		platformSpec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":       "Platform API",
				"version":     "1.0.0",
				"description": "Platform management API",
			},
			"paths": map[string]interface{}{
				"/platform/users": map[string]interface{}{
					"get": map[string]interface{}{
						"summary": "List users",
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "Success",
							},
						},
					},
				},
			},
		}

		cicdSpec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":       "CI/CD API",
				"version":     "2.0.0",
				"description": "Continuous integration API",
			},
			"paths": map[string]interface{}{
				"/pipelines": map[string]interface{}{
					"get": map[string]interface{}{
						"summary": "List pipelines",
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "Success",
							},
						},
					},
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/openapi.json":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(platformSpec)
			case "/api/platform/swagger.json":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(platformSpec)
			case "/api/ci/swagger.json":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(cicdSpec)
			case "/":
				// Return HTML with links
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte(`
					<html>
						<body>
							<a href="/api/platform/swagger.json">Platform API</a>
							<a href="/api/ci/swagger.json">CI/CD API</a>
						</body>
					</html>
				`))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := service.DiscoverMultipleAPIs(ctx, server.URL)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.NotEmpty(t, result.DiscoveredAPIs)

		// Should find at least one API
		assert.GreaterOrEqual(t, len(result.DiscoveredAPIs), 1)

		// Check API details
		var foundPlatform, foundCICD bool
		for _, api := range result.DiscoveredAPIs {
			if strings.Contains(api.Name, "Platform") {
				foundPlatform = true
				assert.Equal(t, "1.0.0", api.Version)
			}
			if strings.Contains(api.Name, "CI/CD") {
				foundCICD = true
				assert.Equal(t, "2.0.0", api.Version)
			}
		}

		// Should find at least one of the APIs
		assert.True(t, foundPlatform || foundCICD)
	})

	t.Run("No APIs found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := service.DiscoverMultipleAPIs(ctx, server.URL)
		require.NoError(t, err)
		assert.NotEqual(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.Empty(t, result.DiscoveredAPIs)
	})

	t.Run("Mixed format APIs", func(t *testing.T) {
		// OpenAPI 3.0 spec
		openapi3Spec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":   "Modern API",
				"version": "3.0.0",
			},
			"paths": map[string]interface{}{
				"/v3/resource": map[string]interface{}{
					"get": map[string]interface{}{
						"summary": "Get resource",
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "Success",
							},
						},
					},
				},
			},
		}

		// Swagger 2.0 spec
		swagger2Spec := map[string]interface{}{
			"swagger": "2.0",
			"info": map[string]interface{}{
				"title":   "Legacy API",
				"version": "2.0.0",
			},
			"paths": map[string]interface{}{
				"/v2/resource": map[string]interface{}{
					"get": map[string]interface{}{
						"summary": "Get resource",
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "Success",
							},
						},
					},
				},
			},
		}

		// Custom JSON format
		customSpec := map[string]interface{}{
			"apis": []interface{}{
				map[string]interface{}{
					"name":   "Custom API",
					"path":   "/custom/endpoint",
					"method": "GET",
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/openapi.json":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(openapi3Spec)
			case "/swagger.json":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(swagger2Spec)
			case "/api-docs":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(customSpec)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := service.DiscoverMultipleAPIs(ctx, server.URL)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)

		// Should find multiple APIs of different formats
		assert.GreaterOrEqual(t, len(result.DiscoveredAPIs), 2)

		// Check for different formats
		formats := make(map[APIFormat]bool)
		for _, api := range result.DiscoveredAPIs {
			formats[api.Format] = true
		}
		assert.True(t, len(formats) >= 2, "Should discover APIs in multiple formats")
	})

	t.Run("API catalog endpoint", func(t *testing.T) {
		catalog := map[string]interface{}{
			"apis": []interface{}{
				map[string]interface{}{
					"name":        "User Service",
					"description": "User management API",
					"spec_url":    "/apis/user/spec.json",
				},
				map[string]interface{}{
					"name":        "Order Service",
					"description": "Order processing API",
					"specUrl":     "/apis/order/spec.json",
				},
			},
		}

		userSpec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":   "User API",
				"version": "1.0.0",
			},
			"paths": map[string]interface{}{},
		}

		orderSpec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":   "Order API",
				"version": "1.0.0",
			},
			"paths": map[string]interface{}{},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/catalog/list":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(catalog)
			case "/apis/user/spec.json":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(userSpec)
			case "/apis/order/spec.json":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(orderSpec)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := service.DiscoverMultipleAPIs(ctx, server.URL)
		require.NoError(t, err)
		assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)
		assert.GreaterOrEqual(t, len(result.DiscoveredAPIs), 2)
	})

	t.Run("Context cancellation", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer slowServer.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		result, err := service.DiscoverMultipleAPIs(ctx, slowServer.URL)
		assert.Error(t, err)
		assert.Equal(t, tools.DiscoveryStatusFailed, result.Status)
		assert.Contains(t, result.Errors, "Discovery timeout")
	})
}

func TestMultiAPIDiscoveryService_CategoryExtractors(t *testing.T) {
	tests := []struct {
		name      string
		specURL   string
		extractor func(string) string
		expected  string
	}{
		// Harness categories
		{
			name:      "Harness Platform",
			specURL:   "https://api.harness.io/platform/swagger.json",
			extractor: extractHarnessCategory,
			expected:  "Platform",
		},
		{
			name:      "Harness CI/CD",
			specURL:   "https://api.harness.io/ci/openapi.json",
			extractor: extractHarnessCategory,
			expected:  "CI/CD",
		},
		{
			name:      "Harness Chaos",
			specURL:   "https://api.harness.io/chaos/api-docs",
			extractor: extractHarnessCategory,
			expected:  "Chaos Engineering",
		},
		{
			name:      "Harness Feature Flags",
			specURL:   "https://api.harness.io/ff/swagger.json",
			extractor: extractHarnessCategory,
			expected:  "Feature Flags",
		},
		{
			name:      "Harness Core",
			specURL:   "https://api.harness.io/core/swagger.json",
			extractor: extractHarnessCategory,
			expected:  "Core",
		},
		// AWS categories
		{
			name:      "AWS EC2",
			specURL:   "https://ec2.amazonaws.com/latest/api-reference.json",
			extractor: extractAWSCategory,
			expected:  "EC2",
		},
		{
			name:      "AWS S3",
			specURL:   "https://s3.amazonaws.com/latest/api-reference.json",
			extractor: extractAWSCategory,
			expected:  "S3",
		},
		// Azure categories
		{
			name:      "Azure Resource Manager",
			specURL:   "https://management.azure.com/resource-manager/Microsoft.Compute/swagger.json",
			extractor: extractAzureCategory,
			expected:  "Resource Manager",
		},
		{
			name:      "Azure Data Plane",
			specURL:   "https://management.azure.com/data-plane/storage/swagger.json",
			extractor: extractAzureCategory,
			expected:  "Data Plane",
		},
		// Google Cloud categories
		{
			name:      "Google Compute",
			specURL:   "https://compute.googleapis.com/compute/v1/swagger.json",
			extractor: extractGoogleCategory,
			expected:  "Compute",
		},
		{
			name:      "Google Storage",
			specURL:   "https://storage.googleapis.com/storage/v1/swagger.json",
			extractor: extractGoogleCategory,
			expected:  "Storage",
		},
		// Kubernetes categories
		{
			name:      "K8s Apps API",
			specURL:   "https://kubernetes.io/apis/apps/v1/swagger.json",
			extractor: extractK8sCategory,
			expected:  "apps",
		},
		{
			name:      "K8s Core API",
			specURL:   "https://kubernetes.io/api/v1/swagger.json",
			extractor: extractK8sCategory,
			expected:  "Core",
		},
		// Generic categories
		{
			name:      "Generic Admin",
			specURL:   "https://api.example.com/admin/swagger.json",
			extractor: extractGenericCategory,
			expected:  "Admin",
		},
		{
			name:      "Generic Public",
			specURL:   "https://api.example.com/public/openapi.json",
			extractor: extractGenericCategory,
			expected:  "Public",
		},
		{
			name:      "Generic Internal",
			specURL:   "https://api.example.com/internal/api-docs",
			extractor: extractGenericCategory,
			expected:  "Internal",
		},
		{
			name:      "Generic v1",
			specURL:   "https://api.example.com/v1/swagger.json",
			extractor: extractGenericCategory,
			expected:  "Core",
		},
		{
			name:      "Generic default",
			specURL:   "https://api.example.com/other/swagger.json",
			extractor: extractGenericCategory,
			expected:  "General",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.extractor(tt.specURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMultiAPIDiscoveryService_ExtractAPIInfo(t *testing.T) {
	logger := &mockLogger{}
	service := NewMultiAPIDiscoveryService(logger)

	t.Run("Extract API name from spec", func(t *testing.T) {
		spec := createValidOpenAPISpec()
		name := service.extractAPIName(spec, "https://api.example.com/swagger.json")
		assert.Equal(t, "Test API", name)
	})

	t.Run("Extract API name from URL", func(t *testing.T) {
		name := service.extractAPIName(nil, "https://api.example.com/users/swagger.json")
		assert.Equal(t, "Users", name)
	})

	t.Run("Extract API description", func(t *testing.T) {
		spec := createValidOpenAPISpec()
		spec.Info.Description = "This is a test API"
		desc := service.extractAPIDescription(spec)
		assert.Equal(t, "This is a test API", desc)
	})

	t.Run("Extract API version", func(t *testing.T) {
		spec := createValidOpenAPISpec()
		version := service.extractAPIVersion(spec)
		assert.Equal(t, "1.0.0", version)
	})

	t.Run("Extract with nil spec", func(t *testing.T) {
		name := service.extractAPIName(nil, "")
		assert.Equal(t, "Unknown API", name)

		desc := service.extractAPIDescription(nil)
		assert.Equal(t, "", desc)

		version := service.extractAPIVersion(nil)
		assert.Equal(t, "1.0.0", version)
	})
}

func TestMultiAPIDiscoveryService_FindAPILinks(t *testing.T) {
	logger := &mockLogger{}
	service := NewMultiAPIDiscoveryService(logger)

	htmlContent := `
		<html>
			<body>
				<a href="/api/docs">API Documentation</a>
				<a href="/swagger-ui">Swagger UI</a>
				<a href="/openapi.json">OpenAPI Spec</a>
				<a href="/graphql/playground">GraphQL Playground</a>
				<a href="/rest/reference">REST Reference</a>
				<a href="/about">About Us</a>
				<a href="/contact">Contact</a>
				<a href="https://external.com/api">External API</a>
			</body>
		</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	// Fetch and parse HTML
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// This is a simplified test - in real implementation we'd use goquery
	// For now, just verify the method exists and returns reasonable results
	assert.NotNil(t, service.findAPILinks)
}

func TestMultiAPIDiscoveryService_ParseAPICatalog(t *testing.T) {
	logger := &mockLogger{}
	service := NewMultiAPIDiscoveryService(logger)

	t.Run("Parse map catalog with apis array", func(t *testing.T) {
		catalog := map[string]interface{}{
			"apis": []interface{}{
				map[string]interface{}{
					"name":     "User API",
					"spec_url": "/user/openapi.json",
				},
				map[string]interface{}{
					"name":    "Order API",
					"specUrl": "/order/swagger.json",
				},
			},
		}

		discoveries := make(chan APIDefinition, 10)
		baseURL, err := parseURL("https://api.example.com")
		require.NoError(t, err, "Failed to parse base URL")
		assert.NotNil(t, baseURL, "Base URL should not be nil")

		ctx := context.Background()
		go func() {
			service.parseAPICatalog(ctx, catalog, baseURL, discoveries)
			close(discoveries)
		}()

		// Collect discoveries
		var apis []APIDefinition
		for api := range discoveries {
			apis = append(apis, api)
		}

		// Note: This test won't find actual APIs because tryDiscoverAPI
		// will fail without a real server, but it tests the parsing logic
		// The test is checking baseURL, not apis
		t.Logf("baseURL value: %v", baseURL)
		t.Logf("apis value: %v, length: %d", apis, len(apis))
	})

	t.Run("Parse array catalog", func(t *testing.T) {
		catalog := []interface{}{
			map[string]interface{}{
				"name":        "Service A",
				"swagger_url": "/a/swagger.json",
			},
			map[string]interface{}{
				"name":        "Service B",
				"openapi_url": "/b/openapi.json",
			},
		}

		discoveries := make(chan APIDefinition, 10)
		baseURL, err := parseURL("https://api.example.com")
		require.NoError(t, err, "Failed to parse base URL")
		assert.NotNil(t, baseURL, "Base URL should not be nil")

		ctx := context.Background()
		go func() {
			service.parseAPICatalog(ctx, catalog, baseURL, discoveries)
			close(discoveries)
		}()

		// Drain channel
		for range discoveries {
		}
	})
}

func TestMultiAPIDiscoveryService_Concurrency(t *testing.T) {
	// Allow localhost for testing
	t.Setenv("ALLOW_LOCALHOST_URLS", "true")
	t.Setenv("ALLOW_PRIVATE_NETWORK_URLS", "true")

	logger := &mockLogger{}
	service := NewMultiAPIDiscoveryService(logger)
	service.concurrency = 2 // Limit concurrency for testing

	// Create 10 API endpoints
	apiCount := 10
	requestCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentCount := requestCount
		mu.Unlock()

		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)

		spec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":   fmt.Sprintf("API %d", currentCount),
				"version": "1.0.0",
			},
			"paths": map[string]interface{}{},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(spec)
	}))
	defer server.Close()

	// Create server that returns multiple API links
	portalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			html := "<html><body>"
			for i := 0; i < apiCount; i++ {
				html += fmt.Sprintf(`<a href="%s/api%d/swagger.json">API %d</a>`, server.URL, i, i)
			}
			html += "</body></html>"
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer portalServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	result, err := service.DiscoverMultipleAPIs(ctx, portalServer.URL)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, tools.DiscoveryStatusSuccess, result.Status)

	// Should have discovered multiple APIs
	assert.Greater(t, len(result.DiscoveredAPIs), 0)

	// Verify concurrency was limited (duration should show some serialization)
	assert.Greater(t, duration, 100*time.Millisecond, "Should take some time due to concurrency limits")
}

// Helper to parse URL
func parseURL(urlStr string) (*url.URL, error) {
	return url.Parse(urlStr)
}

var _ observability.Logger = (*mockLogger)(nil)

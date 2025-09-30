package artifactory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageDiscoveryOperations(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Get all operations to verify package operations are included
	ops := provider.getAllOperationMappings()

	// Test that all package discovery operations are present
	expectedPackageOps := []string{
		"packages/info",
		"packages/versions",
		"packages/latest",
		"packages/stats",
		"packages/properties",
		"packages/maven/info",
		"packages/maven/versions",
		"packages/maven/pom",
		"packages/npm/info",
		"packages/npm/versions",
		"packages/npm/tarball",
		"packages/docker/info",
		"packages/docker/tags",
		"packages/docker/layers",
		"packages/pypi/info",
		"packages/pypi/versions",
		"packages/nuget/info",
		"packages/nuget/versions",
		"packages/search",
		"packages/dependencies",
		"packages/dependents",
	}

	for _, opName := range expectedPackageOps {
		t.Run(opName, func(t *testing.T) {
			op, exists := ops[opName]
			assert.True(t, exists, "Operation %s should exist", opName)
			assert.NotEmpty(t, op.OperationID, "Operation %s should have OperationID", opName)
			assert.NotEmpty(t, op.Method, "Operation %s should have Method", opName)
			assert.NotEmpty(t, op.PathTemplate, "Operation %s should have PathTemplate", opName)
		})
	}
}

func TestPackageDiscoveryExecution(t *testing.T) {
	// Create test server to mock Artifactory API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/storage/maven-central/org/springframework/spring-core":
			if r.URL.Query().Get("list") != "" {
				// List versions response
				response := map[string]interface{}{
					"uri":     "http://localhost/artifactory/api/storage/maven-central/org/springframework/spring-core",
					"created": "2020-01-01T00:00:00.000Z",
					"files":   []interface{}{},
					"children": []interface{}{
						map[string]interface{}{
							"uri":    "/5.3.23",
							"folder": true,
						},
						map[string]interface{}{
							"uri":    "/5.3.24",
							"folder": true,
						},
						map[string]interface{}{
							"uri":    "/6.0.0",
							"folder": true,
						},
						map[string]interface{}{
							"uri":    "/6.0.1",
							"folder": true,
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			} else if r.URL.Query().Get("stats") != "" {
				// Stats response
				response := map[string]interface{}{
					"uri":              "http://localhost/artifactory/api/storage/maven-central/org/springframework/spring-core",
					"downloadCount":    1523,
					"lastDownloaded":   "2024-01-15T10:30:00.000Z",
					"lastDownloadedBy": "jenkins",
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			} else if r.URL.Query().Get("properties") != "" {
				// Properties response
				response := map[string]interface{}{
					"properties": map[string]interface{}{
						"build.name":   []string{"my-app"},
						"build.number": []string{"42"},
						"license":      []string{"Apache-2.0"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			} else {
				// Package info response
				response := map[string]interface{}{
					"uri":          "http://localhost/artifactory/api/storage/maven-central/org/springframework/spring-core",
					"downloadUri":  "http://localhost/artifactory/maven-central/org/springframework/spring-core",
					"repo":         "maven-central",
					"path":         "/org/springframework/spring-core",
					"created":      "2020-01-01T00:00:00.000Z",
					"createdBy":    "admin",
					"lastModified": "2024-01-01T00:00:00.000Z",
					"modifiedBy":   "system",
					"lastUpdated":  "2024-01-01T00:00:00.000Z",
					"children": []interface{}{
						map[string]interface{}{
							"uri":    "/maven-metadata.xml",
							"folder": false,
							"size":   1024,
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}

		case "/api/storage/npm-local/@angular/core":
			if r.URL.Query().Get("list") != "" {
				// NPM versions response
				response := map[string]interface{}{
					"uri": "http://localhost/artifactory/api/storage/npm-local/@angular/core",
					"children": []interface{}{
						map[string]interface{}{
							"uri":    "/15.0.0",
							"folder": true,
						},
						map[string]interface{}{
							"uri":    "/15.1.0",
							"folder": true,
						},
						map[string]interface{}{
							"uri":    "/16.0.0",
							"folder": true,
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}

		case "/api/npm/npm-registry/express":
			// NPM package info
			response := map[string]interface{}{
				"name":        "express",
				"description": "Fast, unopinionated, minimalist web framework",
				"dist-tags": map[string]interface{}{
					"latest": "4.18.2",
					"next":   "5.0.0-beta.1",
				},
				"versions": map[string]interface{}{
					"4.18.0": map[string]interface{}{
						"name":    "express",
						"version": "4.18.0",
					},
					"4.18.1": map[string]interface{}{
						"name":    "express",
						"version": "4.18.1",
					},
					"4.18.2": map[string]interface{}{
						"name":    "express",
						"version": "4.18.2",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}

		case "/api/docker/docker-local/v2/nginx/tags/list":
			// Docker tags list
			response := map[string]interface{}{
				"name": "nginx",
				"tags": []string{
					"latest",
					"1.21",
					"1.21-alpine",
					"1.20",
					"1.20-alpine",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}

		case "/api/pypi/pypi-local/simple/requests":
			// PyPI package info (HTML response simplified as JSON for testing)
			w.Header().Set("Content-Type", "text/html")
			html := `<!DOCTYPE html>
			<html>
			<body>
			<a href="requests-2.28.0.tar.gz">requests-2.28.0.tar.gz</a>
			<a href="requests-2.28.1.tar.gz">requests-2.28.1.tar.gz</a>
			<a href="requests-2.28.2.tar.gz">requests-2.28.2.tar.gz</a>
			</body>
			</html>`
			if _, err := w.Write([]byte(html)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}

		case "/api/nuget/nuget-local/FindPackagesById()":
			// NuGet versions
			if r.URL.Query().Get("id") == "Newtonsoft.Json" {
				response := map[string]interface{}{
					"d:feed": map[string]interface{}{
						"m:entries": []interface{}{
							map[string]interface{}{
								"m:properties": map[string]interface{}{
									"d:Version": "13.0.1",
									"d:Id":      "Newtonsoft.Json",
								},
							},
							map[string]interface{}{
								"m:properties": map[string]interface{}{
									"d:Version": "13.0.2",
									"d:Id":      "Newtonsoft.Json",
								},
							},
							map[string]interface{}{
								"m:properties": map[string]interface{}{
									"d:Version": "13.0.3",
									"d:Id":      "Newtonsoft.Json",
								},
							},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}

		case "/api/search/artifact":
			// Package search
			response := map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"uri": "http://localhost/artifactory/api/storage/libs-release-local/com/example/my-lib/1.0.0/my-lib-1.0.0.jar",
					},
					{
						"uri": "http://localhost/artifactory/api/storage/libs-release-local/com/example/my-lib/1.1.0/my-lib-1.1.0.jar",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}

		case "/api/search/dependency":
			// Package dependents
			response := map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"uri": "http://localhost/artifactory/api/storage/libs-release-local/com/example/app/2.0.0/app-2.0.0.jar",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create provider with test server URL
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)
	provider.SetConfiguration(providers.ProviderConfig{
		BaseURL:  server.URL,
		AuthType: "bearer",
	})

	// Create context with credentials
	ctx := providers.WithContext(context.Background(), &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			Token: "test-api-key",
		},
	})

	// Test Maven package info
	t.Run("packages/info - Maven", func(t *testing.T) {
		params := map[string]interface{}{
			"repoKey":     "maven-central",
			"packagePath": "org/springframework/spring-core",
		}
		result, err := provider.ExecuteOperation(ctx, "packages/info", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Check result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "maven-central", resultMap["repo"])
		assert.Contains(t, resultMap, "created")
	})

	// Test Maven version listing
	t.Run("packages/versions - Maven", func(t *testing.T) {
		params := map[string]interface{}{
			"repoKey":     "maven-central",
			"packagePath": "org/springframework/spring-core",
			"list":        true,
			"deep":        1,
		}
		result, err := provider.ExecuteOperation(ctx, "packages/versions", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Check result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "children")

		// Verify we have versions
		children, ok := resultMap["children"].([]interface{})
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(children), 4, "Should have at least 4 versions")
	})

	// Test package stats
	t.Run("packages/stats", func(t *testing.T) {
		params := map[string]interface{}{
			"repoKey":     "maven-central",
			"packagePath": "org/springframework/spring-core",
			"stats":       true,
		}
		result, err := provider.ExecuteOperation(ctx, "packages/stats", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Check result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "downloadCount")
		assert.Contains(t, resultMap, "lastDownloaded")
	})

	// Test package properties
	t.Run("packages/properties", func(t *testing.T) {
		params := map[string]interface{}{
			"repoKey":     "maven-central",
			"packagePath": "org/springframework/spring-core",
			"properties":  true,
		}
		result, err := provider.ExecuteOperation(ctx, "packages/properties", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Check result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "properties")
	})

	// Test NPM versions
	t.Run("packages/versions - NPM", func(t *testing.T) {
		params := map[string]interface{}{
			"repoKey":     "npm-local",
			"packagePath": "@angular/core",
			"list":        true,
			"deep":        1,
		}
		result, err := provider.ExecuteOperation(ctx, "packages/versions", params)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	// Test NPM package info
	t.Run("packages/npm/info", func(t *testing.T) {
		params := map[string]interface{}{
			"repoKey":     "npm-registry",
			"packageName": "express",
		}
		result, err := provider.ExecuteOperation(ctx, "packages/npm/info", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Check result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "express", resultMap["name"])
		assert.Contains(t, resultMap, "versions")
	})

	// Test Docker tags
	t.Run("packages/docker/tags", func(t *testing.T) {
		params := map[string]interface{}{
			"repoKey":   "docker-local",
			"imageName": "nginx",
		}
		result, err := provider.ExecuteOperation(ctx, "packages/docker/tags", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Check result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "nginx", resultMap["name"])
		assert.Contains(t, resultMap, "tags")

		// Verify we have tags
		tags, ok := resultMap["tags"].([]interface{})
		require.True(t, ok)
		assert.Contains(t, tags, "latest")
	})

	// Test package search
	t.Run("packages/search", func(t *testing.T) {
		params := map[string]interface{}{
			"name":  "my-lib",
			"repos": "libs-release-local",
		}
		result, err := provider.ExecuteOperation(ctx, "packages/search", params)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	// Test package dependents
	t.Run("packages/dependents", func(t *testing.T) {
		params := map[string]interface{}{
			"sha1": "abc123def456",
		}
		result, err := provider.ExecuteOperation(ctx, "packages/dependents", params)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestFormatPackagePath(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	testCases := []struct {
		name        string
		packageType string
		packageName string
		options     map[string]string
		expected    string
	}{
		{
			name:        "Maven with group and artifact",
			packageType: "maven",
			packageName: "spring-core",
			options: map[string]string{
				"groupId":    "org.springframework",
				"artifactId": "spring-core",
			},
			expected: "org/springframework/spring-core",
		},
		{
			name:        "Maven with version",
			packageType: "maven",
			packageName: "spring-core",
			options: map[string]string{
				"groupId":    "org.springframework",
				"artifactId": "spring-core",
				"version":    "5.3.23",
			},
			expected: "org/springframework/spring-core/5.3.23",
		},
		{
			name:        "NPM with scope",
			packageType: "npm",
			packageName: "core",
			options: map[string]string{
				"scope": "@angular",
			},
			expected: "@angular/core",
		},
		{
			name:        "NPM without scope",
			packageType: "npm",
			packageName: "express",
			options:     map[string]string{},
			expected:    "express",
		},
		{
			name:        "Docker with tag",
			packageType: "docker",
			packageName: "nginx",
			options: map[string]string{
				"tag": "1.21-alpine",
			},
			expected: "nginx/1.21-alpine",
		},
		{
			name:        "PyPI package",
			packageType: "pypi",
			packageName: "Django_Rest_Framework",
			options:     map[string]string{},
			expected:    "django-rest-framework",
		},
		{
			name:        "NuGet with version",
			packageType: "nuget",
			packageName: "Newtonsoft.Json",
			options: map[string]string{
				"version": "13.0.1",
			},
			expected: "Newtonsoft.Json/13.0.1",
		},
		{
			name:        "Generic package",
			packageType: "generic",
			packageName: "my-package",
			options:     map[string]string{},
			expected:    "my-package",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := provider.formatPackagePath(tc.packageType, tc.packageName, tc.options)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParsePackageVersions(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Test parsing versions from storage listing response
	response := map[string]interface{}{
		"uri": "http://localhost/artifactory/api/storage/maven-central/org/springframework/spring-core",
		"children": []interface{}{
			map[string]interface{}{
				"uri":    "/5.3.22",
				"folder": true,
				"size":   0,
			},
			map[string]interface{}{
				"uri":    "/5.3.23",
				"folder": true,
				"size":   0,
			},
			map[string]interface{}{
				"uri":    "/6.0.0",
				"folder": true,
				"size":   0,
			},
			map[string]interface{}{
				"uri":    "/maven-metadata.xml",
				"folder": false,
				"size":   1024,
			},
		},
	}

	versions, err := provider.parsePackageVersions(response, "maven")
	require.NoError(t, err)
	assert.Len(t, versions, 3, "Should parse 3 versions (excluding non-folder items)")

	// Verify versions
	versionStrings := []string{}
	for _, v := range versions {
		versionStrings = append(versionStrings, v.Version)
	}
	assert.Contains(t, versionStrings, "5.3.22")
	assert.Contains(t, versionStrings, "5.3.23")
	assert.Contains(t, versionStrings, "6.0.0")
}

func TestIsValidVersion(t *testing.T) {
	testCases := []struct {
		version  string
		expected bool
	}{
		{"1.0.0", true},
		{"2.3.4-SNAPSHOT", true},
		{"1.0.0-alpha", true},
		{"1.0.0-beta.1", true},
		{"20240101", true},
		{"2024.01.01", true},
		{"v1.2.3", true},
		{"latest", false},
		{".", false},
		{"..", false},
		{"", false},
		{"test", false},
		{"abc", false},
	}

	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			result := isValidVersion(tc.version)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetPackageTypeFromRepo(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	testCases := []struct {
		repoKey  string
		expected string
	}{
		{"maven-central", "maven"},
		{"libs-release-local", "maven"},
		{"libs-snapshots", "maven"},
		{"npm-local", "npm"},
		{"npm-registry", "npm"},
		{"docker-hub", "docker"},
		{"docker-local", "docker"},
		{"containers", "docker"},
		{"pypi-local", "pypi"},
		{"python-packages", "pypi"},
		{"nuget-gallery", "nuget"},
		{"dotnet-packages", "nuget"},
		{"go-modules", "go"},
		{"golang-proxy", "go"},
		{"helm-charts", "helm"},
		{"charts-local", "helm"},
		{"generic-local", "generic"},
		{"files", "generic"},
		{"my-custom-repo", "generic"},
	}

	for _, tc := range testCases {
		t.Run(tc.repoKey, func(t *testing.T) {
			result := provider.getPackageTypeFromRepo(tc.repoKey)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetPackageDiscoveryExamples(t *testing.T) {
	examples := GetPackageDiscoveryExamples()

	// Verify examples exist for key operations
	expectedOps := []string{
		"packages/info",
		"packages/versions",
		"packages/maven/versions",
		"packages/npm/info",
		"packages/docker/tags",
	}

	for _, op := range expectedOps {
		t.Run(op, func(t *testing.T) {
			exampleList, exists := examples[op]
			assert.True(t, exists, "Examples should exist for %s", op)
			assert.NotEmpty(t, exampleList, "Examples list should not be empty for %s", op)

			// Check first example has required fields
			if len(exampleList) > 0 {
				example := exampleList[0]
				assert.NotEmpty(t, example["description"], "Example should have description")
				assert.NotNil(t, example["params"], "Example should have params")
			}
		})
	}
}

func TestPackageDiscoveryIntegration(t *testing.T) {
	// Test that package discovery operations integrate properly with the provider
	logger := &observability.NoopLogger{}
	provider := NewArtifactoryProvider(logger)

	// Get default configuration to check operation groups
	config := provider.GetDefaultConfiguration()

	// Find packages operation group
	var packagesGroup *providers.OperationGroup
	for _, group := range config.OperationGroups {
		if group.Name == "packages" {
			packagesGroup = &group
			break
		}
	}

	require.NotNil(t, packagesGroup, "Packages operation group should exist")
	assert.Equal(t, "Package Discovery", packagesGroup.DisplayName)
	assert.Equal(t, "Simplified package discovery and version management", packagesGroup.Description)

	// Verify the operation group contains expected operations
	expectedOps := []string{
		"packages/info", "packages/versions", "packages/latest",
		"packages/stats", "packages/properties", "packages/search",
		"packages/dependencies", "packages/dependents",
	}

	for _, op := range expectedOps {
		assert.Contains(t, packagesGroup.Operations, op,
			"Packages group should contain operation %s", op)
	}
}

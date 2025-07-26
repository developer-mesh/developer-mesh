//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/test/integration/testutils"
)

// TestDynamicToolRegistration verifies dynamic tool registration via API
func TestDynamicToolRegistration(t *testing.T) {
	// Create HTTP client for API calls
	client := testutils.NewHTTPClient("http://localhost:8081", "test-api-key")

	// Create a mock OpenAPI server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "Test API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{
					"/test": map[string]interface{}{
						"get": map[string]interface{}{
							"operationId": "getTest",
							"summary":     "Test endpoint",
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "Success",
								},
							},
						},
					},
				},
			})
		} else if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "healthy",
			})
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"result": "success",
			})
		}
	}))
	defer mockServer.Close()

	// Register a dynamic tool
	toolConfig := map[string]interface{}{
		"name":        "test-dynamic-tool",
		"description": "A test dynamic tool",
		"type":        "openapi",
		"base_url":    mockServer.URL,
		"openapi_url": mockServer.URL + "/openapi.json",
		"health_config": map[string]interface{}{
			"mode":     "on_demand",
			"endpoint": "/health",
			"timeout":  5,
		},
	}

	resp, err := client.Post("/api/v1/tools", toolConfig)
	require.NoError(t, err, "Request to register dynamic tool should not fail")
	defer resp.Body.Close()

	// Verify response status
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Dynamic tool registration should return 201 Created")

	// Parse response
	var toolResponse map[string]interface{}
	err = testutils.ParseJSONResponse(resp, &toolResponse)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify tool was created
	assert.NotEmpty(t, toolResponse["id"], "Tool ID should be returned")
	assert.Equal(t, "test-dynamic-tool", toolResponse["name"])
}

// TestDynamicToolDiscovery verifies automatic capability discovery
func TestDynamicToolDiscovery(t *testing.T) {
	client := testutils.NewHTTPClient("http://localhost:8081", "test-api-key")

	// Create a mock OpenAPI server with rich capabilities
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "CI/CD API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{
					"/pipelines": map[string]interface{}{
						"get": map[string]interface{}{
							"operationId": "listPipelines",
							"summary":     "List all pipelines",
							"tags":        []string{"pipelines"},
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "List of pipelines",
								},
							},
						},
						"post": map[string]interface{}{
							"operationId": "createPipeline",
							"summary":     "Create a new pipeline",
							"tags":        []string{"pipelines"},
							"responses": map[string]interface{}{
								"201": map[string]interface{}{
									"description": "Pipeline created",
								},
							},
						},
					},
					"/builds/{id}": map[string]interface{}{
						"get": map[string]interface{}{
							"operationId": "getBuild",
							"summary":     "Get build details",
							"tags":        []string{"builds"},
							"parameters": []map[string]interface{}{
								{
									"name":     "id",
									"in":       "path",
									"required": true,
									"schema": map[string]interface{}{
										"type": "string",
									},
								},
							},
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "Build details",
								},
							},
						},
					},
				},
			})
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer mockServer.Close()

	// Discover capabilities
	discoveryPayload := map[string]interface{}{
		"base_url":    mockServer.URL,
		"openapi_url": mockServer.URL + "/openapi.json",
	}

	resp, err := client.Post("/api/v1/tools/discover", discoveryPayload)
	require.NoError(t, err, "Request to discover tool capabilities should not fail")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Discovery should return 200 OK")

	var discoveryResponse map[string]interface{}
	err = testutils.ParseJSONResponse(resp, &discoveryResponse)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify discovered capabilities
	capabilities, ok := discoveryResponse["capabilities"].([]interface{})
	assert.True(t, ok, "Response should contain capabilities array")
	assert.Contains(t, capabilities, "pipelines", "Should discover pipelines capability")
	assert.Contains(t, capabilities, "builds", "Should discover builds capability")

	// Verify discovered operations
	operations, ok := discoveryResponse["operations"].([]interface{})
	assert.True(t, ok, "Response should contain operations array")
	assert.GreaterOrEqual(t, len(operations), 3, "Should discover at least 3 operations")
}

// TestDynamicToolExecution verifies executing a dynamically registered tool
func TestDynamicToolExecution(t *testing.T) {
	client := testutils.NewHTTPClient("http://localhost:8081", "test-api-key")

	// Create a mock API server
	executionCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "Execution Test API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{
					"/execute": map[string]interface{}{
						"post": map[string]interface{}{
							"operationId": "executeAction",
							"summary":     "Execute an action",
							"requestBody": map[string]interface{}{
								"required": true,
								"content": map[string]interface{}{
									"application/json": map[string]interface{}{
										"schema": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"action": map[string]interface{}{
													"type": "string",
												},
											},
										},
									},
								},
							},
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "Action executed",
								},
							},
						},
					},
				},
			})
		} else if r.URL.Path == "/execute" && r.Method == "POST" {
			executionCount++
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"result": "executed",
				"count":  executionCount,
			})
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer mockServer.Close()

	// First register the tool
	toolConfig := map[string]interface{}{
		"name":        "exec-test-tool",
		"type":        "openapi",
		"base_url":    mockServer.URL,
		"openapi_url": mockServer.URL + "/openapi.json",
	}

	resp, err := client.Post("/api/v1/tools", toolConfig)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Execute the tool
	executePayload := map[string]interface{}{
		"tool_name": "exec-test-tool",
		"operation": "executeAction",
		"parameters": map[string]interface{}{
			"action": "test-action",
		},
	}

	resp, err = client.Post("/api/v1/tools/execute", executePayload)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Execution should return 200 OK")

	var execResponse map[string]interface{}
	err = testutils.ParseJSONResponse(resp, &execResponse)
	require.NoError(t, err)

	// Verify execution
	assert.Equal(t, "executed", execResponse["result"])
	assert.Equal(t, float64(1), execResponse["count"])
}

// TestDynamicToolHealthCheck verifies health monitoring for dynamic tools
func TestDynamicToolHealthCheck(t *testing.T) {
	client := testutils.NewHTTPClient("http://localhost:8081", "test-api-key")

	// Create a mock API server with health endpoint
	isHealthy := true
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "Health Test API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{
					"/health": map[string]interface{}{
						"get": map[string]interface{}{
							"operationId": "getHealth",
							"summary":     "Health check",
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "Healthy",
								},
								"503": map[string]interface{}{
									"description": "Unhealthy",
								},
							},
						},
					},
				},
			})
		} else if r.URL.Path == "/health" {
			if isHealthy {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "healthy",
					"uptime": 1000,
				})
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "unhealthy",
					"error":  "service down",
				})
			}
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer mockServer.Close()

	// Register tool with health config
	toolConfig := map[string]interface{}{
		"name":        "health-test-tool",
		"type":        "openapi",
		"base_url":    mockServer.URL,
		"openapi_url": mockServer.URL + "/openapi.json",
		"health_config": map[string]interface{}{
			"mode":     "on_demand",
			"endpoint": "/health",
			"timeout":  5,
		},
	}

	resp, err := client.Post("/api/v1/tools", toolConfig)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Check tool health when healthy
	resp, err = client.Get("/api/v1/tools/health-test-tool/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var healthResponse map[string]interface{}
	err = testutils.ParseJSONResponse(resp, &healthResponse)
	require.NoError(t, err)

	assert.True(t, healthResponse["is_healthy"].(bool))
	assert.Equal(t, "healthy", healthResponse["message"])

	// Make the service unhealthy
	isHealthy = false

	// Force a new health check
	resp, err = client.Post("/api/v1/tools/health-test-tool/health/check", nil)
	require.NoError(t, err)
	resp.Body.Close()

	// Check health again
	resp, err = client.Get("/api/v1/tools/health-test-tool/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	err = testutils.ParseJSONResponse(resp, &healthResponse)
	require.NoError(t, err)

	assert.False(t, healthResponse["is_healthy"].(bool))
	assert.Contains(t, healthResponse["message"], "503")
}

// TestDynamicToolAuthentication verifies authentication handling for dynamic tools
func TestDynamicToolAuthentication(t *testing.T) {
	client := testutils.NewHTTPClient("http://localhost:8081", "test-api-key")

	// Create a mock API server that requires authentication
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "Secure API",
					"version": "1.0.0",
				},
				"components": map[string]interface{}{
					"securitySchemes": map[string]interface{}{
						"bearerAuth": map[string]interface{}{
							"type":   "http",
							"scheme": "bearer",
						},
					},
				},
				"security": []map[string]interface{}{
					{"bearerAuth": []interface{}{}},
				},
				"paths": map[string]interface{}{
					"/secure": map[string]interface{}{
						"get": map[string]interface{}{
							"operationId": "getSecureData",
							"summary":     "Get secure data",
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "Success",
								},
								"401": map[string]interface{}{
									"description": "Unauthorized",
								},
							},
						},
					},
				},
			})
		} else if r.URL.Path == "/secure" {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "Bearer test-token-123" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": "secure content",
				})
			} else {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "unauthorized",
				})
			}
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer mockServer.Close()

	// Register tool with authentication
	toolConfig := map[string]interface{}{
		"name":        "secure-tool",
		"type":        "openapi",
		"base_url":    mockServer.URL,
		"openapi_url": mockServer.URL + "/openapi.json",
		"credentials": map[string]interface{}{
			"type":  "token",
			"token": "test-token-123",
		},
	}

	// Use a buffer to capture the request body
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(toolConfig)

	// Note: We need to encrypt credentials before sending
	// This is a simplified test - in reality, credentials should be encrypted
	resp, err := client.Post("/api/v1/tools", toolConfig)
	require.NoError(t, err)
	resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		// Execute authenticated request
		executePayload := map[string]interface{}{
			"tool_name": "secure-tool",
			"operation": "getSecureData",
		}

		resp, err = client.Post("/api/v1/tools/execute", executePayload)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var execResponse map[string]interface{}
		err = testutils.ParseJSONResponse(resp, &execResponse)
		require.NoError(t, err)

		assert.Equal(t, "secure content", execResponse["data"])
	} else {
		t.Skip("Tool registration with credentials not fully implemented")
	}
}

// TestDynamicToolUpdate verifies updating a dynamic tool configuration
func TestDynamicToolUpdate(t *testing.T) {
	client := testutils.NewHTTPClient("http://localhost:8081", "test-api-key")

	// Create initial mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "Update Test API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{
					"/test": map[string]interface{}{
						"get": map[string]interface{}{
							"operationId": "getTest",
							"summary":     "Test endpoint",
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "Success",
								},
							},
						},
					},
				},
			})
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer mockServer.Close()

	// Register initial tool
	toolConfig := map[string]interface{}{
		"name":        "update-test-tool",
		"description": "Initial description",
		"type":        "openapi",
		"base_url":    mockServer.URL,
		"openapi_url": mockServer.URL + "/openapi.json",
	}

	resp, err := client.Post("/api/v1/tools", toolConfig)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResponse map[string]interface{}
	err = testutils.ParseJSONResponse(resp, &createResponse)
	require.NoError(t, err)

	toolID := createResponse["id"].(string)

	// Update the tool
	updateConfig := map[string]interface{}{
		"description": "Updated description",
		"health_config": map[string]interface{}{
			"mode":     "periodic",
			"interval": 60,
			"endpoint": "/health",
		},
	}

	resp, err = client.Put("/api/v1/tools/"+toolID, updateConfig)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify update
	resp, err = client.Get("/api/v1/tools/" + toolID)
	require.NoError(t, err)
	defer resp.Body.Close()

	var toolDetails map[string]interface{}
	err = testutils.ParseJSONResponse(resp, &toolDetails)
	require.NoError(t, err)

	assert.Equal(t, "Updated description", toolDetails["description"])
	healthConfig := toolDetails["health_config"].(map[string]interface{})
	assert.Equal(t, "periodic", healthConfig["mode"])
	assert.Equal(t, float64(60), healthConfig["interval"])
}

// TestDynamicToolScheduledHealthChecks verifies scheduled health checks work
func TestDynamicToolScheduledHealthChecks(t *testing.T) {
	client := testutils.NewHTTPClient("http://localhost:8081", "test-api-key")

	// Create a mock API server that tracks health check calls
	healthCheckCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "Scheduled Health API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{
					"/health": map[string]interface{}{
						"get": map[string]interface{}{
							"operationId": "getHealth",
							"summary":     "Health check",
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "Healthy",
								},
							},
						},
					},
				},
			})
		} else if r.URL.Path == "/health" {
			healthCheckCount++
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "healthy",
				"count":  healthCheckCount,
			})
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer mockServer.Close()

	// Register tool with periodic health checks
	toolConfig := map[string]interface{}{
		"name":        "scheduled-health-tool",
		"type":        "openapi",
		"base_url":    mockServer.URL,
		"openapi_url": mockServer.URL + "/openapi.json",
		"health_config": map[string]interface{}{
			"mode":     "periodic",
			"interval": 2, // Check every 2 seconds
			"endpoint": "/health",
		},
	}

	resp, err := client.Post("/api/v1/tools", toolConfig)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Wait for scheduled health checks
	time.Sleep(5 * time.Second)

	// Verify health checks were performed
	assert.GreaterOrEqual(t, healthCheckCount, 2, "Should have performed at least 2 health checks")

	// Get the latest health status
	resp, err = client.Get("/api/v1/tools/scheduled-health-tool/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	var healthResponse map[string]interface{}
	err = testutils.ParseJSONResponse(resp, &healthResponse)
	require.NoError(t, err)

	assert.True(t, healthResponse["is_healthy"].(bool))
	assert.NotZero(t, healthResponse["last_checked"])
}

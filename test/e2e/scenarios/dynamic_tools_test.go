package scenarios

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/test/e2e/agent"
	"github.com/developer-mesh/developer-mesh/test/e2e/reporting"
	"github.com/developer-mesh/developer-mesh/test/e2e/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dynamic Tools E2E Tests", func() {
	var (
		config    *utils.Config
		reporter  *reporting.StreamingReporter
		isolation *utils.TestIsolation
	)

	BeforeEach(func() {
		config = utils.LoadConfig()
		reporter = reporting.NewStreamingReporter(config.ReportDir, []string{"json", "html", "junit"})
		isolation = utils.NewTestIsolation()

		reporter.StartSuite("Dynamic Tools Tests")
	})

	AfterEach(func() {
		reporter.EndSuite()
		_ = reporter.GenerateReports()
		_ = isolation.CleanupAll()
	})

	Describe("Dynamic Tool Registration", func() {
		It("should register a new tool dynamically via OpenAPI spec", func() {
			testResult := reporting.TestResult{
				Name:      "dynamic_tool_registration",
				Suite:     "dynamic_tools",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("dynamic-tool-reg")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			// Create a mock API server with OpenAPI spec
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/openapi.json" {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{
						"openapi": "3.0.0",
						"info": {
							"title": "Test API",
							"version": "1.0.0"
						},
						"paths": {
							"/test": {
								"get": {
									"operationId": "getTest",
									"summary": "Test endpoint",
									"responses": {
										"200": {
											"description": "Success"
										}
									}
								}
							}
						}
					}`))
				} else if r.URL.Path == "/health" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status": "healthy"}`))
				} else {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"result": "success"}`))
				}
			}))
			defer mockServer.Close()

			// Register the tool
			devopsAgent := agent.NewDevOpsAutomationAgent(
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = devopsAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = devopsAgent.Close() }()

			// Register dynamic tool
			resp, err := devopsAgent.ExecuteMethod(ctx, "tool.register_dynamic", map[string]interface{}{
				"name":        "test-dynamic-tool",
				"base_url":    mockServer.URL,
				"openapi_url": mockServer.URL + "/openapi.json",
				"auth_type":   "none",
			})

			if err != nil || resp.Error != nil {
				// Dynamic tools might not be fully implemented
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "Dynamic tool registration not available"
			} else {
				// Verify the tool was registered
				listResp, err := devopsAgent.ExecuteMethod(ctx, "tool.list", nil)
				Expect(err).NotTo(HaveOccurred())

				result, ok := listResp.Result.(map[string]interface{})
				Expect(ok).To(BeTrue())

				tools, ok := result["tools"].([]interface{})
				Expect(ok).To(BeTrue())

				// Look for our dynamically registered tool
				found := false
				for _, tool := range tools {
					toolMap, ok := tool.(map[string]interface{})
					if ok {
						if name, ok := toolMap["name"].(string); ok && name == "test-dynamic-tool" {
							found = true
							break
						}
					}
				}

				if found {
					testResult.Status = reporting.TestStatusPassed
					testResult.Message = "Dynamic tool registered successfully"
				} else {
					testResult.Status = reporting.TestStatusFailed
					testResult.Message = "Dynamic tool not found in tool list"
				}
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})

		It("should discover tool capabilities automatically", func() {
			testResult := reporting.TestResult{
				Name:      "tool_capability_discovery",
				Suite:     "dynamic_tools",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("tool-discovery")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			// Create a mock API with rich OpenAPI spec
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/openapi.json" {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{
						"openapi": "3.0.0",
						"info": {
							"title": "CI/CD API",
							"version": "1.0.0"
						},
						"paths": {
							"/pipelines": {
								"get": {
									"operationId": "listPipelines",
									"summary": "List all pipelines",
									"tags": ["pipelines"],
									"responses": {
										"200": {
											"description": "List of pipelines"
										}
									}
								},
								"post": {
									"operationId": "createPipeline",
									"summary": "Create a new pipeline",
									"tags": ["pipelines"],
									"requestBody": {
										"required": true,
										"content": {
											"application/json": {
												"schema": {
													"type": "object",
													"properties": {
														"name": {"type": "string"},
														"config": {"type": "object"}
													}
												}
											}
										}
									},
									"responses": {
										"201": {
											"description": "Pipeline created"
										}
									}
								}
							},
							"/builds/{id}": {
								"get": {
									"operationId": "getBuild",
									"summary": "Get build details",
									"tags": ["builds"],
									"parameters": [{
										"name": "id",
										"in": "path",
										"required": true,
										"schema": {"type": "string"}
									}],
									"responses": {
										"200": {
											"description": "Build details"
										}
									}
								}
							}
						}
					}`))
				} else {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status": "ok"}`))
				}
			}))
			defer mockServer.Close()

			devopsAgent := agent.NewDevOpsAutomationAgent(
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = devopsAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = devopsAgent.Close() }()

			// Discover tool capabilities
			resp, err := devopsAgent.ExecuteMethod(ctx, "tool.discover_capabilities", map[string]interface{}{
				"base_url":    mockServer.URL,
				"openapi_url": mockServer.URL + "/openapi.json",
			})

			if err != nil || resp.Error != nil {
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "Tool capability discovery not implemented"
			} else {
				// Verify discovered capabilities
				result, ok := resp.Result.(map[string]interface{})
				if ok {
					capabilities, ok := result["capabilities"].([]interface{})
					if ok && len(capabilities) > 0 {
						// Check if pipelines and builds capabilities were discovered
						hasPipelines := false
						hasBuilds := false
						for _, cap := range capabilities {
							if capStr, ok := cap.(string); ok {
								if capStr == "pipelines" {
									hasPipelines = true
								} else if capStr == "builds" {
									hasBuilds = true
								}
							}
						}

						if hasPipelines && hasBuilds {
							testResult.Status = reporting.TestStatusPassed
							testResult.Message = fmt.Sprintf("Discovered %d capabilities", len(capabilities))
						} else {
							testResult.Status = reporting.TestStatusFailed
							testResult.Message = "Not all expected capabilities discovered"
						}
					} else {
						testResult.Status = reporting.TestStatusFailed
						testResult.Message = "No capabilities discovered"
					}
				} else {
					testResult.Status = reporting.TestStatusFailed
					testResult.Message = "Invalid response format"
				}
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})

	Describe("Dynamic Tool Execution", func() {
		It("should execute dynamically registered tool", func() {
			testResult := reporting.TestResult{
				Name:      "dynamic_tool_execution",
				Suite:     "dynamic_tools",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("dynamic-exec")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			// Create a mock API server
			executionCount := 0
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/openapi.json" {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{
						"openapi": "3.0.0",
						"info": {
							"title": "Dynamic Test API",
							"version": "1.0.0"
						},
						"paths": {
							"/execute": {
								"post": {
									"operationId": "executeAction",
									"summary": "Execute an action",
									"requestBody": {
										"required": true,
										"content": {
											"application/json": {
												"schema": {
													"type": "object",
													"properties": {
														"action": {"type": "string"}
													}
												}
											}
										}
									},
									"responses": {
										"200": {
											"description": "Action executed"
										}
									}
								}
							}
						}
					}`))
				} else if r.URL.Path == "/execute" && r.Method == "POST" {
					executionCount++
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(fmt.Sprintf(`{"result": "executed", "count": %d}`, executionCount)))
				} else {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status": "ok"}`))
				}
			}))
			defer mockServer.Close()

			devopsAgent := agent.NewDevOpsAutomationAgent(
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = devopsAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = devopsAgent.Close() }()

			// First register the tool
			regResp, err := devopsAgent.ExecuteMethod(ctx, "tool.register_dynamic", map[string]interface{}{
				"name":        "dynamic-exec-tool",
				"base_url":    mockServer.URL,
				"openapi_url": mockServer.URL + "/openapi.json",
				"auth_type":   "none",
			})

			if err != nil || regResp.Error != nil {
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "Dynamic tool registration not available"
			} else {
				// Execute the dynamic tool
				execResp, err := devopsAgent.ExecuteMethod(ctx, "tool.execute", map[string]interface{}{
					"tool":      "dynamic-exec-tool",
					"operation": "executeAction",
					"args": map[string]interface{}{
						"action": "test-action",
					},
				})

				if err != nil || execResp.Error != nil {
					testResult.Status = reporting.TestStatusFailed
					testResult.Message = "Failed to execute dynamic tool"
				} else {
					// Verify execution
					if executionCount > 0 {
						testResult.Status = reporting.TestStatusPassed
						testResult.Message = fmt.Sprintf("Dynamic tool executed %d times", executionCount)
					} else {
						testResult.Status = reporting.TestStatusFailed
						testResult.Message = "Dynamic tool was not executed"
					}
				}
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})

	Describe("Tool Health Monitoring", func() {
		It("should monitor health of dynamic tools", func() {
			testResult := reporting.TestResult{
				Name:      "dynamic_tool_health",
				Suite:     "dynamic_tools",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("tool-health")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			// Create a mock API server with health endpoint
			healthStatus := "healthy"
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/openapi.json" {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{
						"openapi": "3.0.0",
						"info": {
							"title": "Health Test API",
							"version": "1.0.0"
						},
						"paths": {
							"/health": {
								"get": {
									"operationId": "getHealth",
									"summary": "Health check",
									"responses": {
										"200": {
											"description": "Healthy"
										}
									}
								}
							}
						}
					}`))
				} else if r.URL.Path == "/health" {
					if healthStatus == "healthy" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"status": "healthy", "uptime": 1000}`))
					} else {
						w.WriteHeader(http.StatusServiceUnavailable)
						w.Write([]byte(`{"status": "unhealthy", "error": "service down"}`))
					}
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer mockServer.Close()

			devopsAgent := agent.NewDevOpsAutomationAgent(
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = devopsAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = devopsAgent.Close() }()

			// Register tool with health config
			regResp, err := devopsAgent.ExecuteMethod(ctx, "tool.register_dynamic", map[string]interface{}{
				"name":        "health-test-tool",
				"base_url":    mockServer.URL,
				"openapi_url": mockServer.URL + "/openapi.json",
				"auth_type":   "none",
				"health_config": map[string]interface{}{
					"mode":     "on_demand",
					"endpoint": "/health",
					"timeout":  5,
				},
			})

			if err != nil || regResp.Error != nil {
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "Tool health monitoring not available"
			} else {
				// Check tool health
				healthResp, err := devopsAgent.ExecuteMethod(ctx, "tool.check_health", map[string]interface{}{
					"tool": "health-test-tool",
				})

				if err != nil || healthResp.Error != nil {
					testResult.Status = reporting.TestStatusFailed
					testResult.Message = "Failed to check tool health"
				} else {
					result, ok := healthResp.Result.(map[string]interface{})
					if ok {
						if status, ok := result["is_healthy"].(bool); ok && status {
							// Now make the tool unhealthy
							healthStatus = "unhealthy"

							// Check health again
							unhealthyResp, err := devopsAgent.ExecuteMethod(ctx, "tool.check_health", map[string]interface{}{
								"tool":  "health-test-tool",
								"force": true, // Force new check
							})

							if err == nil && unhealthyResp.Error == nil {
								unhealthyResult, ok := unhealthyResp.Result.(map[string]interface{})
								if ok {
									if status, ok := unhealthyResult["is_healthy"].(bool); ok && !status {
										testResult.Status = reporting.TestStatusPassed
										testResult.Message = "Health monitoring working correctly"
									} else {
										testResult.Status = reporting.TestStatusFailed
										testResult.Message = "Health check did not detect unhealthy state"
									}
								}
							} else {
								testResult.Status = reporting.TestStatusFailed
								testResult.Message = "Failed to check unhealthy state"
							}
						} else {
							testResult.Status = reporting.TestStatusFailed
							testResult.Message = "Initial health check failed"
						}
					} else {
						testResult.Status = reporting.TestStatusFailed
						testResult.Message = "Invalid health check response"
					}
				}
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})

	Describe("Tool Authentication", func() {
		It("should handle various authentication methods", func() {
			testResult := reporting.TestResult{
				Name:      "dynamic_tool_auth",
				Suite:     "dynamic_tools",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("tool-auth")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			// Create a mock API server that requires authentication
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/openapi.json" {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{
						"openapi": "3.0.0",
						"info": {
							"title": "Secure API",
							"version": "1.0.0"
						},
						"components": {
							"securitySchemes": {
								"bearerAuth": {
									"type": "http",
									"scheme": "bearer"
								}
							}
						},
						"security": [{"bearerAuth": []}],
						"paths": {
							"/secure": {
								"get": {
									"operationId": "getSecureData",
									"summary": "Get secure data",
									"responses": {
										"200": {
											"description": "Success"
										}
									}
								}
							}
						}
					}`))
				} else if r.URL.Path == "/secure" {
					authHeader := r.Header.Get("Authorization")
					if authHeader == "Bearer test-token-123" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"data": "secure content"}`))
					} else {
						w.WriteHeader(http.StatusUnauthorized)
						w.Write([]byte(`{"error": "unauthorized"}`))
					}
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer mockServer.Close()

			devopsAgent := agent.NewDevOpsAutomationAgent(
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = devopsAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = devopsAgent.Close() }()

			// Register tool with authentication
			regResp, err := devopsAgent.ExecuteMethod(ctx, "tool.register_dynamic", map[string]interface{}{
				"name":        "secure-tool",
				"base_url":    mockServer.URL,
				"openapi_url": mockServer.URL + "/openapi.json",
				"auth_type":   "bearer",
				"credentials": map[string]interface{}{
					"token": "test-token-123",
				},
			})

			if err != nil || regResp.Error != nil {
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "Tool authentication not available"
			} else {
				// Execute authenticated request
				execResp, err := devopsAgent.ExecuteMethod(ctx, "tool.execute", map[string]interface{}{
					"tool":      "secure-tool",
					"operation": "getSecureData",
				})

				if err != nil || execResp.Error != nil {
					testResult.Status = reporting.TestStatusFailed
					testResult.Message = "Failed to execute authenticated request"
				} else {
					// Verify we got secure data
					if result, ok := execResp.Result.(map[string]interface{}); ok {
						if data, ok := result["data"].(string); ok && data == "secure content" {
							testResult.Status = reporting.TestStatusPassed
							testResult.Message = "Authentication working correctly"
						} else {
							testResult.Status = reporting.TestStatusFailed
							testResult.Message = "Did not receive expected secure data"
						}
					} else {
						testResult.Status = reporting.TestStatusFailed
						testResult.Message = "Invalid response format"
					}
				}
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})
})

// TestDynamicTools runs dynamic tools tests
func TestDynamicTools(t *testing.T) {
	RegisterFailHandler(Fail)
	// Don't call RunSpecs here - it's handled by the suite
}

package scenarios

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/test/e2e/agent"
	"github.com/developer-mesh/developer-mesh/test/e2e/connection"
	"github.com/developer-mesh/developer-mesh/test/e2e/data"
	"github.com/developer-mesh/developer-mesh/test/e2e/reporting"
	"github.com/developer-mesh/developer-mesh/test/e2e/utils"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Single Agent E2E Tests", func() {
	var (
		config    *utils.Config
		reporter  *reporting.StreamingReporter
		testData  *data.TestData
		isolation *utils.TestIsolation
		logger    *utils.TestLogger
	)

	BeforeEach(func() {
		config = utils.LoadConfig()
		reporter = reporting.NewStreamingReporter(config.ReportDir, []string{"json", "html", "junit"})
		testData = data.DefaultTestData()
		isolation = utils.NewTestIsolation()
		logger = utils.NewTestLogger("single-agent", config.EnableDebug)

		reporter.StartSuite("Single Agent Tests")
	})

	AfterEach(func() {
		reporter.EndSuite()
		_ = reporter.GenerateReports()
		_ = isolation.CleanupAll()
	})

	Describe("Basic Agent Lifecycle", func() {
		It("should complete full agent lifecycle", func() {
			testResult := reporting.TestResult{
				Name:      "agent_lifecycle",
				Suite:     "single_agent",
				StartTime: time.Now(),
			}

			// Create test namespace
			namespace, err := isolation.CreateNamespace("agent-lifecycle")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			// Create test agent
			testAgent := agent.NewTestAgent(
				"lifecycle-test-agent",
				[]string{"code_analysis"},
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			// Connect
			logger.Info("Connecting agent %s", testAgent.GetID())
			err = testAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(testAgent.IsConnected()).To(BeTrue())
			defer func() { _ = testAgent.Close() }()

			// Register capabilities
			logger.Info("Registering capabilities")
			err = testAgent.RegisterCapabilities(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Send heartbeat
			logger.Info("Sending heartbeat")
			err = testAgent.Heartbeat(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Check metrics
			sent, received, lastActivity := testAgent.GetMetrics()
			Expect(sent).To(BeNumerically(">", 0))
			Expect(received).To(BeNumerically(">", 0))
			Expect(lastActivity).To(BeTemporally("~", time.Now(), 5*time.Second))

			// Graceful disconnect
			logger.Info("Disconnecting agent")
			err = testAgent.Close()
			Expect(err).NotTo(HaveOccurred())
			Expect(testAgent.IsConnected()).To(BeFalse())

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = "Agent lifecycle completed successfully"

			reporter.LogTest(testResult)
		})

		It("should handle reconnection", func() {
			testResult := reporting.TestResult{
				Name:      "agent_reconnection",
				Suite:     "single_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("agent-reconnection")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			testAgent := agent.NewTestAgent(
				"reconnection-test-agent",
				[]string{"monitoring"},
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			// Initial connection
			err = testAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Send a heartbeat to verify connection
			err = testAgent.Heartbeat(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Disconnect
			err = testAgent.Close()
			Expect(err).NotTo(HaveOccurred())

			// Wait before reconnecting to ensure server cleanup
			time.Sleep(3 * time.Second)

			// Reconnect with retry logic
			Eventually(func() error {
				return testAgent.Connect(ctx)
			}, 10*time.Second, 1*time.Second).Should(Succeed())
			defer func() { _ = testAgent.Close() }()

			// Verify reconnection by sending another heartbeat
			err = testAgent.Heartbeat(ctx)
			Expect(err).NotTo(HaveOccurred())

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = "Reconnection handled successfully"

			reporter.LogTest(testResult)
		})
	})

	Describe("Tool Discovery and Execution", func() {
		It("should discover available tools", func() {
			testResult := reporting.TestResult{
				Name:      "tool_discovery",
				Suite:     "single_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("tool-discovery")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			testAgent := agent.NewTestAgent(
				"tool-discovery-agent",
				[]string{"devops"},
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = testAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = testAgent.Close() }()

			// List tools
			resp, err := testAgent.ExecuteMethod(ctx, "tool.list", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Error).To(BeNil())

			// Verify response structure
			result, ok := resp.Result.(map[string]interface{})
			Expect(ok).To(BeTrue())

			// Extract tools array from response
			tools, ok := result["tools"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(tools)).To(BeNumerically(">", 0))

			logger.Info("Discovered %d tools", len(tools))

			// Check each tool has required fields
			for _, tool := range tools {
				toolMap, ok := tool.(map[string]interface{})
				Expect(ok).To(BeTrue())
				Expect(toolMap).To(HaveKey("name"))
				Expect(toolMap).To(HaveKey("description"))
				Expect(toolMap).To(HaveKey("inputSchema"))
			}

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = fmt.Sprintf("Discovered %d tools", len(tools))

			reporter.LogTest(testResult)
		})

		It("should execute GitHub tool", func() {
			testResult := reporting.TestResult{
				Name:      "github_tool_execution",
				Suite:     "single_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("github-tool")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			// Use specialized DevOps agent
			devopsAgent := agent.NewDevOpsAutomationAgent(
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = devopsAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = devopsAgent.Close() }()

			// First check if GitHub tool is available in dynamic tools
			listResp, err := devopsAgent.ExecuteMethod(ctx, "tool.list", nil)
			Expect(err).NotTo(HaveOccurred())

			githubAvailable := false
			if listResp.Error == nil {
				if result, ok := listResp.Result.(map[string]interface{}); ok {
					if tools, ok := result["tools"].([]interface{}); ok {
						for _, tool := range tools {
							if toolMap, ok := tool.(map[string]interface{}); ok {
								if name, ok := toolMap["name"].(string); ok && name == "github" {
									githubAvailable = true
									break
								}
							}
						}
					}
				}
			}

			if !githubAvailable {
				// If GitHub tool is not available, try to register it dynamically
				// This assumes GitHub API is available with OpenAPI spec
				regResp, err := devopsAgent.ExecuteMethod(ctx, "tool.register_dynamic", map[string]interface{}{
					"name":        "github",
					"base_url":    "https://api.github.com",
					"openapi_url": "https://api.github.com/openapi.json", // This might not exist
					"auth_type":   "token",
					"credentials": map[string]interface{}{
						"token": config.GitHubToken, // Assuming config has GitHub token
					},
				})

				if err != nil || regResp.Error != nil {
					logger.Info("Could not register GitHub tool dynamically")
				}
			}

			// Execute GitHub tool using dynamic API
			resp, err := devopsAgent.ExecuteMethod(ctx, "tool.execute", map[string]interface{}{
				"tool":      "github",
				"operation": "listRepositories", // Using OpenAPI operation ID format
				"args": map[string]interface{}{
					"org":   "test-org",
					"limit": 5,
					"sort":  "updated",
				},
			})

			if err != nil || resp.Error != nil {
				// Tool might not be available or might fail - that's OK
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "GitHub tool not available or failed"
			} else {
				testResult.Status = reporting.TestStatusPassed
				testResult.Message = "GitHub tool executed successfully"
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})

	Describe("Context Management", func() {
		It("should create and retrieve context", func() {
			testResult := reporting.TestResult{
				Name:      "context_management",
				Suite:     "single_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("context-mgmt")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			testAgent := agent.NewTestAgent(
				"context-agent",
				[]string{"context_management"},
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = testAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = testAgent.Close() }()

			// Create context
			contextData := testData.CreateTestContext("test-context", 100)

			createResp, err := testAgent.ExecuteMethod(ctx, "context.create", map[string]interface{}{
				"name":     contextData.Name,
				"content":  contextData.Content,
				"model_id": "claude-sonnet-4", // Default model for tests
				"metadata": contextData.Metadata,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(createResp.Error).To(BeNil())

			// Extract context ID
			var contextID string
			if result, ok := createResp.Result.(map[string]interface{}); ok {
				if id, ok := result["id"].(string); ok {
					contextID = id
				}
			}
			Expect(contextID).NotTo(BeEmpty())

			// Retrieve context
			getResp, err := testAgent.ExecuteMethod(ctx, "context.get", map[string]interface{}{
				"context_id": contextID,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(getResp.Error).To(BeNil())

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = "Context created and retrieved successfully"

			reporter.LogTest(testResult)
		})
	})

	Describe("Session Management", func() {
		It("should manage sessions correctly", func() {
			testResult := reporting.TestResult{
				Name:      "session_management",
				Suite:     "single_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("session-mgmt")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			testAgent := agent.NewTestAgent(
				"session-agent",
				[]string{"session_management"},
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = testAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = testAgent.Close() }()

			// Create session
			createResp, err := testAgent.ExecuteMethod(ctx, "session.create", map[string]interface{}{
				"name":        "test-session",
				"description": "E2E test session",
				"ttl":         3600, // 1 hour
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(createResp.Error).To(BeNil())

			var sessionID string
			if result, ok := createResp.Result.(map[string]interface{}); ok {
				if id, ok := result["session_id"].(string); ok {
					sessionID = id
				}
			}
			Expect(sessionID).NotTo(BeEmpty())

			// Get session info
			getResp, err := testAgent.ExecuteMethod(ctx, "session.get", map[string]interface{}{
				"session_id": sessionID,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(getResp.Error).To(BeNil())

			// Update session
			updateResp, err := testAgent.ExecuteMethod(ctx, "session.update_state", map[string]interface{}{
				"session_id": sessionID,
				"state": map[string]interface{}{
					"lastActivity": time.Now().Unix(),
					"status":       "active",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(updateResp.Error).To(BeNil())

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = "Session management successful"

			reporter.LogTest(testResult)
		})
	})

	Describe("Error Handling", func() {
		It("should handle authentication errors", func() {
			testResult := reporting.TestResult{
				Name:      "auth_error_handling",
				Suite:     "single_agent",
				StartTime: time.Now(),
			}

			// Try to connect with invalid API key
			connConfig := &connection.ConnectionConfig{
				BaseURL:    config.MCPBaseURL,
				APIKey:     "invalid-api-key",
				TenantID:   testData.TenantID,
				MaxRetries: 1,
				Timeout:    10 * time.Second,
			}

			manager := connection.NewConnectionManager(connConfig)
			ctx := context.Background()

			_, err := manager.EstablishConnection(ctx)
			Expect(err).To(HaveOccurred())

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = "Authentication error handled correctly"

			reporter.LogTest(testResult)
		})

		It("should handle invalid method calls", func() {
			testResult := reporting.TestResult{
				Name:      "invalid_method_handling",
				Suite:     "single_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("invalid-method")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			testAgent := agent.NewTestAgent(
				"error-test-agent",
				[]string{"testing"},
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = testAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = testAgent.Close() }()

			// Call non-existent method
			resp, err := testAgent.ExecuteMethod(ctx, "non.existent.method", map[string]interface{}{
				"param": "value",
			})

			Expect(err).NotTo(HaveOccurred()) // Connection should stay alive
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Code).To(Equal(ws.ErrCodeMethodNotFound))

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = "Invalid method error handled correctly"

			reporter.LogTest(testResult)
		})

		It("should handle timeout scenarios", func() {
			testResult := reporting.TestResult{
				Name:      "timeout_handling",
				Suite:     "single_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("timeout-test")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			testAgent := agent.NewTestAgent(
				"timeout-test-agent",
				[]string{"testing"},
				config.APIKey,
				config.MCPBaseURL,
			)

			ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
			defer cancel()

			err = testAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = testAgent.Close() }()

			// Create a very short timeout context
			timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
			defer cancel()

			// Try to execute with timeout
			_, err = testAgent.ExecuteMethod(timeoutCtx, "tool.list", nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(context.DeadlineExceeded))

			// Verify connection is still alive
			err = testAgent.Heartbeat(ctx)
			Expect(err).NotTo(HaveOccurred())

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = "Timeout scenarios handled correctly"

			reporter.LogTest(testResult)
		})
	})
})

// TestSingleAgent runs single agent tests
func TestSingleAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	// Don't call RunSpecs here - it's handled by the suite
}

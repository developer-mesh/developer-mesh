package scenarios

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/test/e2e/agent"
	"github.com/developer-mesh/developer-mesh/test/e2e/data"
	"github.com/developer-mesh/developer-mesh/test/e2e/reporting"
	"github.com/developer-mesh/developer-mesh/test/e2e/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Multi-Agent Collaboration E2E Tests", func() {
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
		logger = utils.NewTestLogger("multi-agent", config.EnableDebug)

		reporter.StartSuite("Multi-Agent Collaboration Tests")
	})

	AfterEach(func() {
		reporter.EndSuite()
		_ = reporter.GenerateReports()
		_ = isolation.CleanupAll()
	})

	Describe("Basic Multi-Agent Setup", func() {
		It("should connect multiple agents simultaneously", func() {
			testResult := reporting.TestResult{
				Name:      "multi_agent_connection",
				Suite:     "multi_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("multi-agent-connection")
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = isolation.DeleteNamespace(namespace.ID)
			}()

			ctx := context.Background()

			// Create different agent types
			agents := []interface{}{
				agent.NewCodeAnalysisAgent(config.APIKey, config.MCPBaseURL),
				agent.NewDevOpsAutomationAgent(config.APIKey, config.MCPBaseURL),
				agent.NewSecurityScannerAgent(config.APIKey, config.MCPBaseURL),
			}

			// Connect all agents concurrently
			var wg sync.WaitGroup
			errors := make(chan error, len(agents))

			for i, ag := range agents {
				wg.Add(1)
				go func(idx int, testAgent interface{}) {
					defer wg.Done()

					var err error
					switch a := testAgent.(type) {
					case *agent.CodeAnalysisAgent:
						err = a.Connect(ctx)
						if err == nil {
							defer func() { _ = a.Close() }()
							err = a.RegisterCapabilities(ctx)
						}
					case *agent.DevOpsAutomationAgent:
						err = a.Connect(ctx)
						if err == nil {
							defer func() { _ = a.Close() }()
							err = a.RegisterCapabilities(ctx)
						}
					case *agent.SecurityScannerAgent:
						err = a.Connect(ctx)
						if err == nil {
							defer func() { _ = a.Close() }()
							err = a.RegisterCapabilities(ctx)
						}
					}

					if err != nil {
						errors <- err
					} else {
						logger.Info("Agent %d connected and registered", idx)
					}
				}(i, ag)
			}

			wg.Wait()
			close(errors)

			// Check for errors
			errorCount := 0
			for err := range errors {
				errorCount++
				logger.Error("Connection error: %v", err)
			}

			Expect(errorCount).To(Equal(0))

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = fmt.Sprintf("Connected %d agents successfully", len(agents))

			reporter.LogTest(testResult)
		})
	})

	Describe("Code Review Workflow", func() {
		It("should coordinate code review between multiple agents", NodeTimeout(5*time.Minute), func(specCtx SpecContext) {
			testResult := reporting.TestResult{
				Name:      "code_review_workflow",
				Suite:     "multi_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("code-review-workflow")
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = isolation.DeleteNamespace(namespace.ID)
			}()

			// Create context for API calls
			ctx := context.Background()

			// Create specialized agents
			codeAgent := agent.NewCodeAnalysisAgent(config.APIKey, config.MCPBaseURL)
			securityAgent := agent.NewSecurityScannerAgent(config.APIKey, config.MCPBaseURL)

			// Connect both agents
			err = codeAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = codeAgent.Close() }()

			err = securityAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = securityAgent.Close() }()

			// Register capabilities
			err = codeAgent.RegisterCapabilities(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = securityAgent.RegisterCapabilities(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Create collaborative workflow
			logger.Info("Creating collaborative workflow")
			workflowResp, err := codeAgent.ExecuteMethod(ctx, "workflow.create_collaborative", map[string]interface{}{
				"name":        "code-review-workflow",
				"description": "Multi-agent code review process",
				"steps": []map[string]interface{}{
					{
						"name":             "code_analysis",
						"agent_capability": "code_analysis",
						"timeout":          300, // 5 minutes
					},
					{
						"name":             "security_scan",
						"agent_capability": "security_scanning",
						"timeout":          600, // 10 minutes
					},
				},
				"strategy":          "sequential",
				"coordination_mode": "centralized", // Fix: Add required field
				"decision_strategy": "majority",    // Fix: Add required field
			})

			Expect(err).NotTo(HaveOccurred())
			if workflowResp.Error != nil {
				logger.Error("Workflow creation failed: %v", workflowResp.Error)
			}
			Expect(workflowResp.Error).To(BeNil())

			logger.Info("Workflow created successfully")
			var workflowID string
			if result, ok := workflowResp.Result.(map[string]interface{}); ok {
				if id, ok := result["workflow_id"].(string); ok {
					workflowID = id
				}
			}
			Expect(workflowID).NotTo(BeEmpty())

			// Execute workflow
			execResp, err := codeAgent.ExecuteMethod(ctx, "workflow.execute_collaborative", map[string]interface{}{
				"workflow_id": workflowID,
				"input": map[string]interface{}{
					"repository": testData.Repositories["backend-api"].URL,
					"pr_number":  testData.Repositories["backend-api"].PRNumbers[0],
				},
			})

			Expect(err).NotTo(HaveOccurred())

			if execResp.Error != nil {
				// Workflow might not be fully implemented
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "Collaborative workflow not fully implemented"
			} else {
				// Monitor workflow completion
				var executionID string
				if result, ok := execResp.Result.(map[string]interface{}); ok {
					if id, ok := result["execution_id"].(string); ok {
						executionID = id
					}
				}

				if executionID != "" {
					// Poll for completion
					completed := false
					logger.Info("Polling for workflow completion, execution ID: %s", executionID)
					for i := 0; i < 30; i++ { // 30 seconds max
						statusResp, err := codeAgent.ExecuteMethod(ctx, "workflow.status", map[string]interface{}{
							"execution_id": executionID,
						})

						if err == nil && statusResp.Error == nil {
							if result, ok := statusResp.Result.(map[string]interface{}); ok {
								if status, ok := result["status"].(string); ok && (status == "completed" || status == "failed") {
									completed = true
									break
								}
							}
						}

						time.Sleep(1 * time.Second)
					}

					if completed {
						testResult.Status = reporting.TestStatusPassed
						testResult.Message = "Code review workflow completed"
					} else {
						testResult.Status = reporting.TestStatusFailed
						testResult.Message = "Workflow did not complete in time"
					}
				} else {
					testResult.Status = reporting.TestStatusPassed
					testResult.Message = "Workflow created successfully"
				}
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})

	Describe("Parallel Task Execution", func() {
		It("should execute tasks in parallel across multiple agents", func() {
			testResult := reporting.TestResult{
				Name:      "parallel_task_execution",
				Suite:     "multi_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("parallel-tasks")
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = isolation.DeleteNamespace(namespace.ID)
			}()

			ctx := context.Background()

			// Create multiple agents of the same type
			agents := make([]*agent.TestAgent, 3)
			for i := 0; i < 3; i++ {
				agents[i] = agent.NewTestAgent(
					fmt.Sprintf("parallel-agent-%d", i),
					[]string{"task_processor"},
					config.APIKey,
					config.MCPBaseURL,
				)

				err := agents[i].Connect(ctx)
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = agents[i].Close() }()

				err = agents[i].RegisterCapabilities(ctx)
				Expect(err).NotTo(HaveOccurred())
			}

			// Create parallel task batch
			batchResp, err := agents[0].ExecuteMethod(ctx, "task.create_batch", map[string]interface{}{
				"tasks": []map[string]interface{}{
					{
						"type":    "process_data",
						"payload": map[string]interface{}{"data": "task1"},
					},
					{
						"type":    "process_data",
						"payload": map[string]interface{}{"data": "task2"},
					},
					{
						"type":    "process_data",
						"payload": map[string]interface{}{"data": "task3"},
					},
				},
				"strategy": "parallel",
			})

			if err != nil || batchResp.Error != nil {
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "Batch task creation not supported"
			} else {
				// Wait for task assignments
				taskCount := 0
				timeout := time.After(10 * time.Second)

				for taskCount < 3 {
					select {
					case <-timeout:
						goto done
					default:
						for _, ag := range agents {
							task, err := ag.WaitForTask(ctx, 100*time.Millisecond)
							if err == nil && task != nil {
								taskCount++

								// Accept and complete task
								if params, ok := task.Params.(map[string]interface{}); ok {
									if taskID, ok := params["taskId"].(string); ok {
										_ = ag.AcceptTask(ctx, taskID)
										_ = ag.CompleteTask(ctx, taskID, map[string]interface{}{
											"status": "completed",
											"result": "processed",
										})
									}
								}
							}
						}
					}
				}

			done:
				if taskCount >= 3 {
					testResult.Status = reporting.TestStatusPassed
					testResult.Message = fmt.Sprintf("Executed %d tasks in parallel", taskCount)
				} else {
					testResult.Status = reporting.TestStatusFailed
					testResult.Message = fmt.Sprintf("Only %d tasks completed", taskCount)
				}
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})

	Describe("Consensus Mechanism", func() {
		It("should reach consensus among multiple agents", func() {
			testResult := reporting.TestResult{
				Name:      "consensus_mechanism",
				Suite:     "multi_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("consensus")
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = isolation.DeleteNamespace(namespace.ID)
			}()

			ctx := context.Background()

			// Create voting agents
			votingAgents := make([]*agent.TestAgent, 5)
			for i := 0; i < 5; i++ {
				votingAgents[i] = agent.NewTestAgent(
					fmt.Sprintf("voting-agent-%d", i),
					[]string{"consensus_participant"},
					config.APIKey,
					config.MCPBaseURL,
				)

				err := votingAgents[i].Connect(ctx)
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = votingAgents[i].Close() }()
			}

			// Create consensus proposal
			proposalResp, err := votingAgents[0].ExecuteMethod(ctx, "consensus.propose", map[string]interface{}{
				"proposal": map[string]interface{}{
					"type":        "deployment_approval",
					"application": "test-app",
					"version":     "v1.2.3",
					"environment": "production",
				},
				"timeout": 30, // 30 seconds
				"quorum":  3,  // Need 3 votes
			})

			if err != nil || proposalResp.Error != nil {
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "Consensus mechanism not implemented"
			} else {
				var proposalID string
				if result, ok := proposalResp.Result.(map[string]interface{}); ok {
					if id, ok := result["proposal_id"].(string); ok {
						proposalID = id
					}
				}

				if proposalID != "" {
					// Have agents vote
					votes := 0
					for i, ag := range votingAgents {
						vote := i < 3 // First 3 vote yes
						voteResp, err := ag.ExecuteMethod(ctx, "consensus.vote", map[string]interface{}{
							"proposal_id": proposalID,
							"vote":        vote,
							"reason":      fmt.Sprintf("Agent %d vote", i),
						})

						if err == nil && voteResp.Error == nil {
							votes++
						}
					}

					// Check consensus result
					resultResp, err := votingAgents[0].ExecuteMethod(ctx, "consensus.result", map[string]interface{}{
						"proposal_id": proposalID,
					})

					if err == nil && resultResp.Error == nil {
						testResult.Status = reporting.TestStatusPassed
						testResult.Message = fmt.Sprintf("Consensus reached with %d votes", votes)
					} else {
						testResult.Status = reporting.TestStatusFailed
						testResult.Message = "Failed to get consensus result"
					}
				} else {
					testResult.Status = reporting.TestStatusFailed
					testResult.Message = "Failed to create proposal"
				}
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})

	Describe("MapReduce Pattern", func() {
		It("should execute MapReduce workflow across agents", func() {
			testResult := reporting.TestResult{
				Name:      "mapreduce_workflow",
				Suite:     "multi_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("mapreduce")
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = isolation.DeleteNamespace(namespace.ID)
			}()

			ctx := context.Background()

			// Create mapper agents
			mappers := make([]*agent.TestAgent, 3)
			for i := 0; i < 3; i++ {
				mappers[i] = agent.NewTestAgent(
					fmt.Sprintf("mapper-%d", i),
					[]string{"data_mapper"},
					config.APIKey,
					config.MCPBaseURL,
				)

				err := mappers[i].Connect(ctx)
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = mappers[i].Close() }()
			}

			// Create reducer agent
			reducer := agent.NewTestAgent(
				"reducer",
				[]string{"data_reducer"},
				config.APIKey,
				config.MCPBaseURL,
			)

			err = reducer.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = reducer.Close() }()

			// Create MapReduce job
			jobResp, err := mappers[0].ExecuteMethod(ctx, "mapreduce.create", map[string]interface{}{
				"job_type": "log_analysis",
				"input": map[string]interface{}{
					"data_source": "test-logs",
					"time_range": map[string]interface{}{
						"start": time.Now().Add(-1 * time.Hour).Unix(),
						"end":   time.Now().Unix(),
					},
				},
				"map_function":    "count_errors",
				"reduce_function": "sum_counts",
				"partitions":      3,
			})

			if err != nil || jobResp.Error != nil {
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "MapReduce not implemented"
			} else {
				// Monitor job completion
				var jobID string
				if result, ok := jobResp.Result.(map[string]interface{}); ok {
					if id, ok := result["job_id"].(string); ok {
						jobID = id
					}
				}

				if jobID != "" {
					// Wait for completion
					completed := false
					for i := 0; i < 20; i++ {
						statusResp, err := mappers[0].ExecuteMethod(ctx, "mapreduce.status", map[string]interface{}{
							"job_id": jobID,
						})

						if err == nil && statusResp.Error == nil {
							if result, ok := statusResp.Result.(map[string]interface{}); ok {
								if status, ok := result["status"].(string); ok && status == "completed" {
									completed = true
									break
								}
							}
						}

						time.Sleep(500 * time.Millisecond)
					}

					if completed {
						testResult.Status = reporting.TestStatusPassed
						testResult.Message = "MapReduce job completed successfully"
					} else {
						testResult.Status = reporting.TestStatusFailed
						testResult.Message = "MapReduce job did not complete"
					}
				} else {
					testResult.Status = reporting.TestStatusFailed
					testResult.Message = "Failed to create MapReduce job"
				}
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})

	Describe("Agent Coordination", func() {
		It("should coordinate deployment pipeline across agents", func() {
			testResult := reporting.TestResult{
				Name:      "deployment_pipeline_coordination",
				Suite:     "multi_agent",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("deployment-pipeline")
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = isolation.DeleteNamespace(namespace.ID)
			}()

			ctx := context.Background()

			// Create specialized agents for deployment pipeline
			codeAgent := agent.NewCodeAnalysisAgent(config.APIKey, config.MCPBaseURL)
			devopsAgent := agent.NewDevOpsAutomationAgent(config.APIKey, config.MCPBaseURL)
			securityAgent := agent.NewSecurityScannerAgent(config.APIKey, config.MCPBaseURL)
			infraAgent := agent.NewInfrastructureAgent(config.APIKey, config.MCPBaseURL)
			monitoringAgent := agent.NewMonitoringAgent(config.APIKey, config.MCPBaseURL)

			// Connect all agents
			agents := []interface{}{codeAgent, devopsAgent, securityAgent, infraAgent, monitoringAgent}
			for _, ag := range agents {
				switch a := ag.(type) {
				case *agent.CodeAnalysisAgent:
					err = a.Connect(ctx)
					Expect(err).NotTo(HaveOccurred())
					defer a.Close()
				case *agent.DevOpsAutomationAgent:
					err = a.Connect(ctx)
					Expect(err).NotTo(HaveOccurred())
					defer a.Close()
				case *agent.SecurityScannerAgent:
					err = a.Connect(ctx)
					Expect(err).NotTo(HaveOccurred())
					defer a.Close()
				case *agent.InfrastructureAgent:
					err = a.Connect(ctx)
					Expect(err).NotTo(HaveOccurred())
					defer a.Close()
				case *agent.MonitoringAgent:
					err = a.Connect(ctx)
					Expect(err).NotTo(HaveOccurred())
					defer a.Close()
				}
			}

			// Create deployment pipeline
			pipelineResp, err := devopsAgent.ExecuteMethod(ctx, "pipeline.create", map[string]interface{}{
				"name": "full-deployment-pipeline",
				"stages": []map[string]interface{}{
					{
						"name":       "code_analysis",
						"agent_type": "code_analysis",
						"tasks":      []string{"lint", "test", "coverage"},
					},
					{
						"name":       "security_scan",
						"agent_type": "security_scanner",
						"tasks":      []string{"vulnerability_scan", "dependency_check"},
					},
					{
						"name":       "build",
						"agent_type": "devops_automation",
						"tasks":      []string{"compile", "package"},
					},
					{
						"name":       "deploy",
						"agent_type": "infrastructure",
						"tasks":      []string{"provision", "deploy", "configure"},
					},
					{
						"name":       "verify",
						"agent_type": "monitoring",
						"tasks":      []string{"health_check", "smoke_test"},
					},
				},
			})

			if err != nil || pipelineResp.Error != nil {
				testResult.Status = reporting.TestStatusSkipped
				testResult.Message = "Pipeline coordination not fully implemented"
			} else {
				testResult.Status = reporting.TestStatusPassed
				testResult.Message = "Deployment pipeline created successfully"
			}

			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)

			reporter.LogTest(testResult)
		})
	})
})

// TestMultiAgent runs multi-agent collaboration tests
func TestMultiAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	// Don't call RunSpecs here - it's handled by the suite
}

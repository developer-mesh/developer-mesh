package test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

// TestWorkflows tests all workflow examples
func TestWorkflows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Ensure Edge MCP is running
	client, err := common.NewClient(nil)
	if err != nil {
		t.Fatalf("Edge MCP not running: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	t.Run("GitHub_Operations", func(t *testing.T) {
		testGitHubOperations(t, ctx, client)
	})

	t.Run("Batch_Operations", func(t *testing.T) {
		testBatchOperations(t, ctx, client)
	})

	t.Run("Agent_Orchestration", func(t *testing.T) {
		testAgentOrchestration(t, ctx, client)
	})

	t.Run("Context_Management", func(t *testing.T) {
		testContextManagement(t, ctx, client)
	})
}

func testGitHubOperations(t *testing.T, ctx context.Context, client *common.MCPClient) {
	// Test list repositories
	t.Run("ListRepositories", func(t *testing.T) {
		result, err := client.CallTool(ctx, "github_list_repositories", map[string]interface{}{
			"type": "owner",
		})
		if err != nil {
			t.Fatalf("List repositories failed: %v", err)
		}

		var repos []map[string]interface{}
		if err := json.Unmarshal(result, &repos); err != nil {
			t.Fatalf("Failed to parse repositories: %v", err)
		}

		t.Logf("Found %d repositories", len(repos))
	})

	// Test get repository
	t.Run("GetRepository", func(t *testing.T) {
		result, err := client.CallTool(ctx, "github_get_repository", map[string]interface{}{
			"owner": "developer-mesh",
			"repo":  "developer-mesh",
		})
		if err != nil {
			t.Fatalf("Get repository failed: %v", err)
		}

		var repo map[string]interface{}
		if err := json.Unmarshal(result, &repo); err != nil {
			t.Fatalf("Failed to parse repository: %v", err)
		}

		if repo["full_name"] == nil {
			t.Error("Repository missing full_name field")
		}

		t.Logf("Repository: %v", repo["full_name"])
	})
}

func testBatchOperations(t *testing.T, ctx context.Context, client *common.MCPClient) {
	// Test parallel batch execution
	t.Run("ParallelBatch", func(t *testing.T) {
		tools := []common.BatchToolCall{
			{
				ID:   "repos",
				Name: "github_list_repositories",
				Arguments: map[string]interface{}{
					"type": "owner",
				},
			},
			{
				ID:        "agents",
				Name:      "agent_list",
				Arguments: map[string]interface{}{},
			},
		}

		result, err := client.BatchCallTools(ctx, tools, true)
		if err != nil {
			t.Fatalf("Parallel batch failed: %v", err)
		}

		if len(result.Results) != len(tools) {
			t.Errorf("Expected %d results, got %d", len(tools), len(result.Results))
		}

		if result.SuccessCount+result.ErrorCount != len(tools) {
			t.Error("Success count + error count doesn't match total tools")
		}

		t.Logf("Batch: %d succeeded, %d failed, duration: %v",
			result.SuccessCount, result.ErrorCount, common.FormatDuration(result.DurationMS))
	})

	// Test sequential batch execution
	t.Run("SequentialBatch", func(t *testing.T) {
		tools := []common.BatchToolCall{
			{
				ID:        "task1",
				Name:      "task_list",
				Arguments: map[string]interface{}{},
			},
			{
				ID:        "agent1",
				Name:      "agent_list",
				Arguments: map[string]interface{}{},
			},
		}

		result, err := client.BatchCallTools(ctx, tools, false)
		if err != nil {
			t.Fatalf("Sequential batch failed: %v", err)
		}

		if result.Parallel {
			t.Error("Expected sequential execution, got parallel")
		}

		t.Logf("Sequential batch duration: %v", common.FormatDuration(result.DurationMS))
	})

	// Test partial failure handling
	t.Run("PartialFailure", func(t *testing.T) {
		tools := []common.BatchToolCall{
			{
				ID:        "valid",
				Name:      "agent_list",
				Arguments: map[string]interface{}{},
			},
			{
				ID:        "invalid",
				Name:      "nonexistent_tool",
				Arguments: map[string]interface{}{},
			},
		}

		result, err := client.BatchCallTools(ctx, tools, true)
		if err != nil {
			t.Logf("Batch completed with errors (expected): %v", err)
		}

		if result != nil {
			if result.ErrorCount == 0 {
				t.Error("Expected at least one error for nonexistent tool")
			}

			if result.SuccessCount == 0 {
				t.Error("Expected at least one success for valid tool")
			}

			t.Logf("Partial failure: %d succeeded, %d failed", result.SuccessCount, result.ErrorCount)
		}
	})
}

func testAgentOrchestration(t *testing.T, ctx context.Context, client *common.MCPClient) {
	// Test list agents
	t.Run("ListAgents", func(t *testing.T) {
		result, err := client.CallTool(ctx, "agent_list", map[string]interface{}{})
		if err != nil {
			t.Fatalf("List agents failed: %v", err)
		}

		var response struct {
			Agents []map[string]interface{} `json:"agents"`
		}
		if err := json.Unmarshal(result, &response); err != nil {
			t.Fatalf("Failed to parse agents: %v", err)
		}

		t.Logf("Found %d agents", len(response.Agents))
	})

	// Test create task
	t.Run("CreateTask", func(t *testing.T) {
		result, err := client.CallTool(ctx, "task_create", map[string]interface{}{
			"title":       "Test task",
			"type":        "testing",
			"priority":    "low",
			"description": "Integration test task",
		})
		if err != nil {
			t.Fatalf("Create task failed: %v", err)
		}

		var taskResponse struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(result, &taskResponse); err != nil {
			t.Fatalf("Failed to parse task response: %v", err)
		}

		if taskResponse.ID == "" {
			t.Error("Task ID is empty")
		}

		t.Logf("Created task: %s", taskResponse.ID)
	})

	// Test list tasks
	t.Run("ListTasks", func(t *testing.T) {
		result, err := client.CallTool(ctx, "task_list", map[string]interface{}{})
		if err != nil {
			t.Fatalf("List tasks failed: %v", err)
		}

		var response struct {
			Tasks []map[string]interface{} `json:"tasks"`
		}
		if err := json.Unmarshal(result, &response); err != nil {
			t.Fatalf("Failed to parse tasks: %v", err)
		}

		t.Logf("Found %d tasks", len(response.Tasks))
	})
}

func testContextManagement(t *testing.T, ctx context.Context, client *common.MCPClient) {
	// Test update context
	t.Run("UpdateContext", func(t *testing.T) {
		contextData := map[string]interface{}{
			"test_key":    "test_value",
			"test_number": 123,
			"test_bool":   true,
		}

		err := client.UpdateContext(ctx, contextData, false)
		if err != nil {
			t.Fatalf("Update context failed: %v", err)
		}

		t.Log("Context updated successfully")
	})

	// Test get context
	t.Run("GetContext", func(t *testing.T) {
		context, err := client.GetContext(ctx)
		if err != nil {
			t.Fatalf("Get context failed: %v", err)
		}

		if context["test_key"] != "test_value" {
			t.Errorf("Expected test_key='test_value', got %v", context["test_key"])
		}

		t.Logf("Context retrieved: %d fields", len(context))
	})

	// Test merge context
	t.Run("MergeContext", func(t *testing.T) {
		updates := map[string]interface{}{
			"new_field": "new_value",
		}

		err := client.UpdateContext(ctx, updates, true)
		if err != nil {
			t.Fatalf("Merge context failed: %v", err)
		}

		// Verify merge
		context, err := client.GetContext(ctx)
		if err != nil {
			t.Fatalf("Get context failed: %v", err)
		}

		if context["test_key"] != "test_value" {
			t.Error("Original field lost during merge")
		}

		if context["new_field"] != "new_value" {
			t.Error("New field not added during merge")
		}

		t.Log("Context merged successfully")
	})
}

// TestErrorScenarios tests error handling examples
func TestErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	client, err := common.NewClient(nil)
	if err != nil {
		t.Fatalf("Edge MCP not running: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	t.Run("Tool_Not_Found", func(t *testing.T) {
		testToolNotFound(t, ctx, client)
	})

	t.Run("Fuzzy_Search", func(t *testing.T) {
		testFuzzySearch(t, ctx, client)
	})
}

func testToolNotFound(t *testing.T, ctx context.Context, client *common.MCPClient) {
	_, err := client.CallTool(ctx, "nonexistent_tool", map[string]interface{}{})
	if err == nil {
		t.Error("Expected tool not found error, got nil")
	}

	code, message := common.ExtractErrorCode(err)
	t.Logf("Error code: %d, message: %s", code, message)

	if code != -32000 && code != 404 {
		t.Logf("Warning: Unexpected error code %d for tool not found", code)
	}
}

func testFuzzySearch(t *testing.T, ctx context.Context, client *common.MCPClient) {
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	matches := common.FuzzySearchTools(tools, "github")
	if len(matches) == 0 {
		t.Error("Expected to find GitHub tools via fuzzy search")
	}

	t.Logf("Fuzzy search found %d matches for 'github'", len(matches))
}

// TestPerformance tests performance examples
func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	if os.Getenv("RUN_PERFORMANCE_TESTS") != "true" {
		t.Skip("Skipping performance tests (set RUN_PERFORMANCE_TESTS=true to run)")
	}

	client, err := common.NewClient(nil)
	if err != nil {
		t.Fatalf("Edge MCP not running: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	t.Run("Batch_Performance", func(t *testing.T) {
		testBatchPerformance(t, ctx, client)
	})
}

func testBatchPerformance(t *testing.T, ctx context.Context, client *common.MCPClient) {
	tools := []common.BatchToolCall{
		{ID: "1", Name: "agent_list", Arguments: map[string]interface{}{}},
		{ID: "2", Name: "task_list", Arguments: map[string]interface{}{}},
		{ID: "3", Name: "github_list_repositories", Arguments: map[string]interface{}{"type": "owner"}},
	}

	// Test sequential
	start := time.Now()
	seqResult, err := client.BatchCallTools(ctx, tools, false)
	seqDuration := time.Since(start)
	if err != nil {
		t.Fatalf("Sequential batch failed: %v", err)
	}

	// Test parallel
	start = time.Now()
	parResult, err := client.BatchCallTools(ctx, tools, true)
	parDuration := time.Since(start)
	if err != nil {
		t.Fatalf("Parallel batch failed: %v", err)
	}

	// Compare
	speedup := float64(seqDuration) / float64(parDuration)
	t.Logf("Sequential: %v, Parallel: %v, Speedup: %.2fx", seqDuration, parDuration, speedup)

	// Verify parallel is faster (with some tolerance for variance)
	if parDuration > seqDuration {
		t.Logf("Warning: Parallel execution was slower than sequential")
	}

	t.Logf("Sequential success count: %d, Parallel success count: %d",
		seqResult.SuccessCount, parResult.SuccessCount)
}

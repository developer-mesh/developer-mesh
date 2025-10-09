package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

func main() {
	common.PrintSection("Batch Operations Workflow Example")
	common.PrintInfo("Demonstrates parallel and sequential batch execution")

	// Create MCP client
	client, err := common.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example 1: Parallel batch execution
	if err := parallelBatchExecution(ctx, client); err != nil {
		common.PrintError("Parallel batch failed", err)
	}

	// Example 2: Sequential batch execution
	if err := sequentialBatchExecution(ctx, client); err != nil {
		common.PrintError("Sequential batch failed", err)
	}

	// Example 3: Mixed operations batch
	if err := mixedOperationsBatch(ctx, client); err != nil {
		common.PrintError("Mixed operations batch failed", err)
	}

	// Example 4: Partial failure handling
	if err := partialFailureBatch(ctx, client); err != nil {
		common.PrintError("Partial failure batch failed", err)
	}

	common.PrintSection("Batch Operations Complete")
	common.PrintSuccess("All batch operations executed successfully")
}

func parallelBatchExecution(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Parallel Batch Execution")
	common.PrintInfo("Executing 3 independent GitHub operations in parallel")

	tools := []common.BatchToolCall{
		{
			ID:   "repos",
			Name: "github_list_repositories",
			Arguments: map[string]interface{}{
				"type": "owner",
			},
		},
		{
			ID:   "issues",
			Name: "github_list_issues",
			Arguments: map[string]interface{}{
				"owner": "developer-mesh",
				"repo":  "developer-mesh",
				"state": "open",
			},
		},
		{
			ID:   "prs",
			Name: "github_list_pull_requests",
			Arguments: map[string]interface{}{
				"owner": "developer-mesh",
				"repo":  "developer-mesh",
				"state": "open",
			},
		},
	}

	result, err := client.BatchCallTools(ctx, tools, true) // parallel = true
	if err != nil {
		return err
	}

	common.PrintSuccess(common.SummarizeBatchResult(result))

	// Print individual results
	for _, toolResult := range result.Results {
		status := "✓"
		if toolResult.Status != "success" {
			status = "✗"
		}
		fmt.Printf("  %s %s: %s (%s)\n",
			status,
			toolResult.ID,
			toolResult.Status,
			common.FormatDuration(toolResult.DurationMS))
	}

	return nil
}

func sequentialBatchExecution(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Sequential Batch Execution")
	common.PrintInfo("Executing dependent operations in sequence")

	tools := []common.BatchToolCall{
		{
			ID:   "get-repo",
			Name: "github_get_repository",
			Arguments: map[string]interface{}{
				"owner": "developer-mesh",
				"repo":  "developer-mesh",
			},
		},
		{
			ID:   "list-branches",
			Name: "github_list_branches",
			Arguments: map[string]interface{}{
				"owner": "developer-mesh",
				"repo":  "developer-mesh",
			},
		},
		{
			ID:   "list-commits",
			Name: "github_list_commits",
			Arguments: map[string]interface{}{
				"owner": "developer-mesh",
				"repo":  "developer-mesh",
				"sha":   "main",
			},
		},
	}

	result, err := client.BatchCallTools(ctx, tools, false) // parallel = false
	if err != nil {
		return err
	}

	common.PrintSuccess(common.SummarizeBatchResult(result))

	return nil
}

func mixedOperationsBatch(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Mixed Operations Batch")
	common.PrintInfo("Combining GitHub and Agent operations in one batch")

	tools := []common.BatchToolCall{
		{
			ID:   "github-repo",
			Name: "github_get_repository",
			Arguments: map[string]interface{}{
				"owner": "developer-mesh",
				"repo":  "developer-mesh",
			},
		},
		{
			ID:   "list-agents",
			Name: "agent_list",
			Arguments: map[string]interface{}{
				"status": "online",
			},
		},
		{
			ID:   "list-tasks",
			Name: "task_list",
			Arguments: map[string]interface{}{
				"status": "pending",
			},
		},
	}

	result, err := client.BatchCallTools(ctx, tools, true) // parallel = true
	if err != nil {
		return err
	}

	common.PrintSuccess(common.SummarizeBatchResult(result))

	return nil
}

func partialFailureBatch(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Partial Failure Handling")
	common.PrintInfo("Demonstrating graceful handling of partial batch failures")

	tools := []common.BatchToolCall{
		{
			ID:   "valid-1",
			Name: "github_list_repositories",
			Arguments: map[string]interface{}{
				"type": "owner",
			},
		},
		{
			ID:   "invalid",
			Name: "nonexistent_tool",
			Arguments: map[string]interface{}{
				"param": "value",
			},
		},
		{
			ID:        "valid-2",
			Name:      "agent_list",
			Arguments: map[string]interface{}{},
		},
	}

	result, err := client.BatchCallTools(ctx, tools, true) // parallel = true
	if err != nil {
		// Batch execution itself succeeded, but some tools failed
		common.PrintInfo(fmt.Sprintf("Batch completed with errors: %v", err))
	}

	if result != nil {
		common.PrintInfo(common.SummarizeBatchResult(result))

		// Analyze results
		fmt.Println("\nDetailed Results:")
		for _, toolResult := range result.Results {
			if toolResult.Status == "success" {
				var data interface{}
				if err := json.Unmarshal(toolResult.Result, &data); err == nil {
					fmt.Printf("  ✓ %s: Success (data available)\n", toolResult.ID)
				}
			} else {
				fmt.Printf("  ✗ %s: Failed - %v\n", toolResult.ID, toolResult.Error.Message)
			}
		}

		// Show how to handle partial failures
		successfulResults := []common.BatchToolResult{}
		for _, r := range result.Results {
			if r.Status == "success" {
				successfulResults = append(successfulResults, r)
			}
		}

		common.PrintInfo(fmt.Sprintf("\nProcessing %d successful results (ignoring %d failures)",
			len(successfulResults), result.ErrorCount))
	}

	return nil
}

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

func main() {
	common.PrintSection("Batch Parallel Execution Performance Example")
	common.PrintInfo("Comparing sequential vs parallel batch execution performance")

	// Create MCP client
	client, err := common.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Create test tools
	tools := []common.BatchToolCall{
		{ID: "call-1", Name: "github_list_repositories", Arguments: map[string]interface{}{"type": "owner"}},
		{ID: "call-2", Name: "github_list_issues", Arguments: map[string]interface{}{"owner": "developer-mesh", "repo": "developer-mesh", "state": "open"}},
		{ID: "call-3", Name: "github_list_pull_requests", Arguments: map[string]interface{}{"owner": "developer-mesh", "repo": "developer-mesh", "state": "open"}},
		{ID: "call-4", Name: "github_list_branches", Arguments: map[string]interface{}{"owner": "developer-mesh", "repo": "developer-mesh"}},
		{ID: "call-5", Name: "github_list_commits", Arguments: map[string]interface{}{"owner": "developer-mesh", "repo": "developer-mesh", "sha": "main"}},
	}

	// Test 1: Sequential execution
	sequentialResult := benchmarkSequential(ctx, client, tools)

	// Test 2: Parallel execution
	parallelResult := benchmarkParallel(ctx, client, tools)

	// Compare results
	comparePerformance(sequentialResult, parallelResult)

	common.PrintSection("Performance Comparison Complete")
	common.PrintSuccess("Demonstrated parallel execution performance benefits")
}

func benchmarkSequential(ctx context.Context, client *common.MCPClient, tools []common.BatchToolCall) *common.BatchResult {
	common.PrintSubsection("Sequential Execution Benchmark")

	start := time.Now()
	result, err := client.BatchCallTools(ctx, tools, false) // parallel = false
	elapsed := time.Since(start)

	if err != nil {
		common.PrintError("Sequential execution failed", err)
		return nil
	}

	common.PrintSuccess(common.SummarizeBatchResult(result))
	fmt.Printf("  Total wall clock time: %v\n", elapsed)
	fmt.Printf("  Average per tool: %v\n", elapsed/time.Duration(len(tools)))

	return result
}

func benchmarkParallel(ctx context.Context, client *common.MCPClient, tools []common.BatchToolCall) *common.BatchResult {
	common.PrintSubsection("Parallel Execution Benchmark")

	start := time.Now()
	result, err := client.BatchCallTools(ctx, tools, true) // parallel = true
	elapsed := time.Since(start)

	if err != nil {
		common.PrintError("Parallel execution failed", err)
		return nil
	}

	common.PrintSuccess(common.SummarizeBatchResult(result))
	fmt.Printf("  Total wall clock time: %v\n", elapsed)
	fmt.Printf("  Average per tool: %v\n", elapsed/time.Duration(len(tools)))

	return result
}

func comparePerformance(sequential, parallel *common.BatchResult) {
	if sequential == nil || parallel == nil {
		common.PrintInfo("Cannot compare performance (one or both tests failed)")
		return
	}

	common.PrintSubsection("Performance Comparison")

	fmt.Printf("\nSequential Execution:\n")
	fmt.Printf("  Total Duration: %s\n", common.FormatDuration(sequential.DurationMS))
	fmt.Printf("  Tools Executed: %d\n", len(sequential.Results))
	fmt.Printf("  Success Rate: %.1f%%\n", float64(sequential.SuccessCount)/float64(len(sequential.Results))*100)

	fmt.Printf("\nParallel Execution:\n")
	fmt.Printf("  Total Duration: %s\n", common.FormatDuration(parallel.DurationMS))
	fmt.Printf("  Tools Executed: %d\n", len(parallel.Results))
	fmt.Printf("  Success Rate: %.1f%%\n", float64(parallel.SuccessCount)/float64(len(parallel.Results))*100)

	// Calculate speedup
	if sequential.DurationMS > 0 {
		speedup := sequential.DurationMS / parallel.DurationMS
		improvement := ((sequential.DurationMS - parallel.DurationMS) / sequential.DurationMS) * 100

		fmt.Printf("\nPerformance Improvement:\n")
		fmt.Printf("  Speedup: %.2fx faster\n", speedup)
		fmt.Printf("  Time Saved: %.1f%%\n", improvement)
		fmt.Printf("  Absolute Time Saved: %s\n",
			common.FormatDuration(sequential.DurationMS-parallel.DurationMS))
	}

	fmt.Println("\nKey Takeaways:")
	fmt.Println("  • Parallel execution is ideal for independent operations")
	fmt.Println("  • Use sequential mode when operations have dependencies")
	fmt.Println("  • Typical speedup: 3-5x for I/O-bound operations")
	fmt.Println("  • Always measure performance for your specific use case")
}

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

func main() {
	common.PrintSection("Rate Limiting Error Handling Example")
	common.PrintInfo("Demonstrates handling rate limit errors with exponential backoff")

	// Create MCP client
	client, err := common.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example 1: Detect rate limit error
	detectRateLimitError(ctx, client)

	// Example 2: Exponential backoff retry
	exponentialBackoffRetry(ctx, client)

	// Example 3: Respect retry_after header
	respectRetryAfter(ctx, client)

	// Example 4: Batch operations to reduce rate limit impact
	batchToReduceRateLimit(ctx, client)

	common.PrintSection("Rate Limiting Handling Complete")
	common.PrintSuccess("Demonstrated rate limit error recovery strategies")
}

func detectRateLimitError(ctx context.Context, client *common.MCPClient) {
	common.PrintSubsection("Detecting Rate Limit Errors")

	// Make a sample call (this won't actually trigger rate limit in normal usage)
	_, err := client.CallTool(ctx, "github_list_repositories", map[string]interface{}{
		"type": "owner",
	})

	if err != nil {
		code, message := common.ExtractErrorCode(err)

		// Check for rate limit error (code 429)
		if code == 429 || strings.Contains(message, "rate limit") {
			common.PrintError("Rate limit exceeded", err)
			fmt.Printf("  Error Code: %d\n", code)
			fmt.Printf("  Error Message: %s\n", message)

			// Extract retry_after if available
			retryAfter := common.HandleRateLimitError(err)
			common.PrintInfo(fmt.Sprintf("Suggested retry after: %v", retryAfter))
		} else {
			common.PrintSuccess("No rate limit error detected")
		}
	} else {
		common.PrintSuccess("Request succeeded (no rate limit)")
	}
}

func exponentialBackoffRetry(ctx context.Context, client *common.MCPClient) {
	common.PrintSubsection("Exponential Backoff Retry Strategy")

	toolName := "github_list_repositories"
	args := map[string]interface{}{
		"type": "owner",
	}

	// Retry with exponential backoff
	err := common.RetryWithBackoff(func() error {
		_, err := client.CallTool(ctx, toolName, args)
		if err != nil {
			code, _ := common.ExtractErrorCode(err)
			if code == 429 {
				return err // Retryable error
			}
			// Non-retryable error, stop retrying
			return fmt.Errorf("non-retryable error: %w", err)
		}
		return nil
	}, 3, 1*time.Second)

	if err != nil {
		common.PrintError("Failed after retries", err)
	} else {
		common.PrintSuccess("Request succeeded (with retries)")
	}
}

func respectRetryAfter(ctx context.Context, client *common.MCPClient) {
	common.PrintSubsection("Respecting retry_after Header")

	makeRequestWithRetry := func(toolName string, args map[string]interface{}) error {
		maxRetries := 3
		baseDelay := 1 * time.Second

		for attempt := 0; attempt < maxRetries; attempt++ {
			_, err := client.CallTool(ctx, toolName, args)

			if err == nil {
				common.PrintSuccess("Request succeeded")
				return nil
			}

			code, _ := common.ExtractErrorCode(err)
			if code != 429 {
				// Not a rate limit error, don't retry
				return err
			}

			// Rate limit error - calculate backoff
			retryAfter := common.HandleRateLimitError(err)
			if retryAfter == 0 {
				// No retry_after specified, use exponential backoff
				retryAfter = baseDelay * time.Duration(1<<uint(attempt))
			}

			if attempt < maxRetries-1 {
				common.PrintInfo(fmt.Sprintf(
					"Rate limited (attempt %d/%d), waiting %v before retry",
					attempt+1, maxRetries, retryAfter))
				time.Sleep(retryAfter)
			}
		}

		return fmt.Errorf("failed after %d retries due to rate limiting", maxRetries)
	}

	err := makeRequestWithRetry("github_list_repositories", map[string]interface{}{
		"type": "owner",
	})

	if err != nil {
		common.PrintError("Request failed", err)
	}
}

func batchToReduceRateLimit(ctx context.Context, client *common.MCPClient) {
	common.PrintSubsection("Using Batch Operations to Reduce Rate Limit Impact")
	common.PrintInfo("Batching multiple operations into a single request")

	// Instead of making 3 separate calls, batch them
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

	common.PrintInfo("Making 3 API calls in one batch request...")

	result, err := client.BatchCallTools(ctx, tools, true)
	if err != nil {
		common.PrintError("Batch request failed", err)
		return
	}

	common.PrintSuccess(common.SummarizeBatchResult(result))
	fmt.Println("\nBenefits of batching:")
	fmt.Println("  • Single rate limit counter instead of 3")
	fmt.Println("  • Reduced network overhead")
	fmt.Println("  • Faster overall execution with parallel mode")

	// Show rate limit savings
	fmt.Printf("\nRate limit impact:\n")
	fmt.Printf("  Individual calls: 3 requests = 3 rate limit tokens\n")
	fmt.Printf("  Batch call: 1 request = 1 rate limit token\n")
	fmt.Printf("  Savings: 66%% reduction in rate limit usage\n")
}

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

func main() {
	common.PrintSection("Context Management Workflow Example")
	common.PrintInfo("Demonstrates session context creation and management")

	// Create MCP client
	client, err := common.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example 1: Create initial context
	if err := createInitialContext(ctx, client); err != nil {
		common.PrintError("Create initial context failed", err)
	}

	// Example 2: Update context (merge)
	if err := updateContextMerge(ctx, client); err != nil {
		common.PrintError("Update context (merge) failed", err)
	}

	// Example 3: Retrieve context
	if err := retrieveContext(ctx, client); err != nil {
		common.PrintError("Retrieve context failed", err)
	}

	// Example 4: Replace context
	if err := replaceContext(ctx, client); err != nil {
		common.PrintError("Replace context failed", err)
	}

	// Example 5: Context-aware operations
	if err := contextAwareOperations(ctx, client); err != nil {
		common.PrintError("Context-aware operations failed", err)
	}

	common.PrintSection("Context Management Complete")
	common.PrintSuccess("All context operations executed successfully")
}

func createInitialContext(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Creating Initial Session Context")

	contextData := map[string]interface{}{
		"user_id":        "user-123",
		"session_type":   "development",
		"repository":     "developer-mesh/developer-mesh",
		"current_branch": "main",
		"workflow_stage": "initial",
		"preferences": map[string]interface{}{
			"auto_commit":  false,
			"lint_on_save": true,
		},
	}

	err := client.UpdateContext(ctx, contextData, false) // replace = false (initial create)
	if err != nil {
		return err
	}

	common.PrintSuccess("Initial context created")
	common.PrettyPrint(contextData)

	return nil
}

func updateContextMerge(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Updating Context (Merge Mode)")

	// Add new fields while keeping existing ones
	updates := map[string]interface{}{
		"workflow_stage":   "code_review",
		"last_action":      "opened_pr",
		"pr_number":        123,
		"review_checklist": []string{"tests", "docs", "security"},
	}

	err := client.UpdateContext(ctx, updates, true) // merge = true
	if err != nil {
		return err
	}

	common.PrintSuccess("Context updated (merged)")
	fmt.Println("  New fields added:")
	for key := range updates {
		fmt.Printf("    • %s\n", key)
	}

	return nil
}

func retrieveContext(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Retrieving Current Context")

	context, err := client.GetContext(ctx)
	if err != nil {
		return err
	}

	common.PrintSuccess("Context retrieved")
	common.PrettyPrint(context)

	// Demonstrate accessing specific fields
	if repo, ok := context["repository"]; ok {
		common.PrintInfo(fmt.Sprintf("Current repository: %v", repo))
	}
	if stage, ok := context["workflow_stage"]; ok {
		common.PrintInfo(fmt.Sprintf("Current workflow stage: %v", stage))
	}

	return nil
}

func replaceContext(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Replacing Context (Full Replace)")

	// Completely replace context with new data
	newContext := map[string]interface{}{
		"user_id":        "user-123",
		"session_type":   "production",
		"repository":     "developer-mesh/edge-mcp",
		"current_branch": "feature/new-feature",
		"workflow_stage": "deployment",
		"deployment": map[string]interface{}{
			"environment": "staging",
			"version":     "1.2.3",
			"timestamp":   "2025-10-03T10:00:00Z",
		},
	}

	err := client.UpdateContext(ctx, newContext, false) // merge = false (replace)
	if err != nil {
		return err
	}

	common.PrintSuccess("Context replaced completely")
	common.PrintInfo("Old fields removed, new context:")
	common.PrettyPrint(newContext)

	return nil
}

func contextAwareOperations(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Context-Aware Operations")

	// Get current context to determine which operations to perform
	currentContext, err := client.GetContext(ctx)
	if err != nil {
		return err
	}

	// Extract repository from context
	repo, ok := currentContext["repository"].(string)
	if !ok {
		return fmt.Errorf("repository not found in context")
	}

	common.PrintInfo(fmt.Sprintf("Using repository from context: %s", repo))

	// Parse owner/repo
	var owner, repoName string
	fmt.Sscanf(repo, "%[^/]/%s", &owner, &repoName)

	// Perform operations based on context
	workflowStage := currentContext["workflow_stage"]
	common.PrintInfo(fmt.Sprintf("Current workflow stage: %v", workflowStage))

	switch workflowStage {
	case "deployment":
		common.PrintInfo("Executing deployment workflow...")

		// Example: List recent commits for deployment
		result, err := client.CallTool(ctx, "github_list_commits", map[string]interface{}{
			"owner":    owner,
			"repo":     repoName,
			"sha":      currentContext["current_branch"],
			"per_page": 5,
		})
		if err != nil {
			return err
		}

		common.PrintSuccess("Retrieved recent commits for deployment")
		fmt.Printf("  Result size: %d bytes\n", len(result))

		// Update context with deployment progress
		err = client.UpdateContext(ctx, map[string]interface{}{
			"deployment_status": "in_progress",
			"deployment_start":  "2025-10-03T10:05:00Z",
		}, true)
		if err != nil {
			return err
		}

		common.PrintSuccess("Updated deployment status in context")

	default:
		common.PrintInfo(fmt.Sprintf("No specific operations for stage: %v", workflowStage))
	}

	return nil
}

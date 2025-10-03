package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

func main() {
	common.PrintSection("Harness Pipeline Workflow Example")
	common.PrintInfo("Demonstrates Harness CI/CD pipeline operations using Edge MCP")
	common.PrintInfo("Note: Requires HARNESS_API_KEY and HARNESS_ACCOUNT_ID environment variables")

	// Create MCP client
	client, err := common.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example 1: List pipelines
	if err := listPipelines(ctx, client); err != nil {
		common.PrintError("List pipelines failed", err)
	}

	// Example 2: Get pipeline details
	if err := getPipeline(ctx, client, "example-pipeline"); err != nil {
		common.PrintError("Get pipeline failed", err)
	}

	// Example 3: List executions
	if err := listExecutions(ctx, client); err != nil {
		common.PrintError("List executions failed", err)
	}

	// Example 4: Get execution status
	if err := getExecutionStatus(ctx, client, "exec-123"); err != nil {
		common.PrintError("Get execution status failed", err)
	}

	common.PrintSection("Harness Pipeline Operations Complete")
	common.PrintSuccess("All Harness operations executed successfully")
}

func listPipelines(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Listing Harness Pipelines")

	result, err := client.CallTool(ctx, "harness_pipelines_list", map[string]interface{}{
		"org_identifier":     "default",
		"project_identifier": "default",
	})
	if err != nil {
		return err
	}

	var response struct {
		Pipelines []map[string]interface{} `json:"content"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return fmt.Errorf("failed to parse pipelines: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Found %d pipelines", len(response.Pipelines)))

	if len(response.Pipelines) > 0 {
		common.PrintInfo("Available pipelines:")
		for i, pipeline := range response.Pipelines {
			if i >= 5 {
				break
			}
			name := pipeline["name"]
			identifier := pipeline["identifier"]
			fmt.Printf("  %d. %s (ID: %s)\n", i+1, name, identifier)
		}
	}

	return nil
}

func getPipeline(ctx context.Context, client *common.MCPClient, pipelineID string) error {
	common.PrintSubsection(fmt.Sprintf("Getting Pipeline: %s", pipelineID))

	result, err := client.CallTool(ctx, "harness_pipelines_get", map[string]interface{}{
		"org_identifier":      "default",
		"project_identifier":  "default",
		"pipeline_identifier": pipelineID,
	})
	if err != nil {
		return err
	}

	var pipeline map[string]interface{}
	if err := json.Unmarshal(result, &pipeline); err != nil {
		return fmt.Errorf("failed to parse pipeline: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Pipeline: %s", pipeline["name"]))
	fmt.Printf("  Description: %v\n", pipeline["description"])
	fmt.Printf("  Identifier: %v\n", pipeline["identifier"])
	fmt.Printf("  Tags: %v\n", pipeline["tags"])

	return nil
}

func listExecutions(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Listing Pipeline Executions")

	result, err := client.CallTool(ctx, "harness_executions_list", map[string]interface{}{
		"org_identifier":     "default",
		"project_identifier": "default",
	})
	if err != nil {
		return err
	}

	var response struct {
		Executions []map[string]interface{} `json:"content"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return fmt.Errorf("failed to parse executions: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Found %d executions", len(response.Executions)))

	if len(response.Executions) > 0 {
		common.PrintInfo("Recent executions:")
		for i, exec := range response.Executions {
			if i >= 5 {
				break
			}
			status := exec["status"]
			pipelineName := exec["pipelineIdentifier"]
			startTime := exec["startTs"]
			fmt.Printf("  %d. %s - %s (started: %v)\n", i+1, pipelineName, status, startTime)
		}
	}

	return nil
}

func getExecutionStatus(ctx context.Context, client *common.MCPClient, executionID string) error {
	common.PrintSubsection(fmt.Sprintf("Getting Execution Status: %s", executionID))

	result, err := client.CallTool(ctx, "harness_executions_status", map[string]interface{}{
		"org_identifier":     "default",
		"project_identifier": "default",
		"execution_id":       executionID,
	})
	if err != nil {
		return err
	}

	var execution map[string]interface{}
	if err := json.Unmarshal(result, &execution); err != nil {
		return fmt.Errorf("failed to parse execution: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Execution: %s", executionID))
	fmt.Printf("  Status: %v\n", execution["status"])
	fmt.Printf("  Pipeline: %v\n", execution["pipelineIdentifier"])
	fmt.Printf("  Started: %v\n", execution["startTs"])
	fmt.Printf("  Duration: %v ms\n", execution["executionDurationMs"])

	return nil
}

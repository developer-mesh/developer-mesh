package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

func main() {
	common.PrintSection("Agent Orchestration Workflow Example")
	common.PrintInfo("Demonstrates multi-agent task management and orchestration")

	// Create MCP client
	client, err := common.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example 1: List available agents
	if err := listAgents(ctx, client); err != nil {
		common.PrintError("List agents failed", err)
	}

	// Example 2: Create tasks
	taskIDs, err := createTasks(ctx, client)
	if err != nil {
		common.PrintError("Create tasks failed", err)
	}

	// Example 3: Assign tasks to agents
	if err := assignTasks(ctx, client, taskIDs); err != nil {
		common.PrintError("Assign tasks failed", err)
	}

	// Example 4: Monitor task progress
	if err := monitorTasks(ctx, client, taskIDs); err != nil {
		common.PrintError("Monitor tasks failed", err)
	}

	common.PrintSection("Agent Orchestration Complete")
	common.PrintSuccess("All agent operations executed successfully")
}

func listAgents(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Listing Available Agents")

	result, err := client.CallTool(ctx, "agent_list", map[string]interface{}{})
	if err != nil {
		return err
	}

	var response struct {
		Agents []map[string]interface{} `json:"agents"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return fmt.Errorf("failed to parse agents: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Found %d agents", len(response.Agents)))

	if len(response.Agents) > 0 {
		fmt.Println("\nAgent Status:")
		for _, agent := range response.Agents {
			fmt.Printf("  • %s: %s (last heartbeat: %v)\n",
				agent["agent_id"],
				agent["status"],
				agent["last_heartbeat"])
		}
	} else {
		common.PrintInfo("No agents currently registered (standalone mode)")
	}

	return nil
}

func createTasks(ctx context.Context, client *common.MCPClient) ([]string, error) {
	common.PrintSubsection("Creating Tasks")

	tasks := []struct {
		Title    string
		Type     string
		Priority string
	}{
		{"Review PR #123", "code_review", "high"},
		{"Deploy to staging", "deployment", "medium"},
		{"Run security scan", "analysis", "high"},
	}

	var taskIDs []string

	for _, task := range tasks {
		result, err := client.CallTool(ctx, "task_create", map[string]interface{}{
			"title":       task.Title,
			"type":        task.Type,
			"priority":    task.Priority,
			"description": fmt.Sprintf("Auto-generated task: %s", task.Title),
		})
		if err != nil {
			common.PrintError(fmt.Sprintf("Failed to create task: %s", task.Title), err)
			continue
		}

		var taskResponse struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(result, &taskResponse); err != nil {
			common.PrintError("Failed to parse task response", err)
			continue
		}

		taskIDs = append(taskIDs, taskResponse.ID)
		common.PrintSuccess(fmt.Sprintf("Created task: %s (ID: %s)", task.Title, taskResponse.ID))
	}

	return taskIDs, nil
}

func assignTasks(ctx context.Context, client *common.MCPClient, taskIDs []string) error {
	common.PrintSubsection("Assigning Tasks to Agents")

	if len(taskIDs) == 0 {
		common.PrintInfo("No tasks to assign")
		return nil
	}

	// Get list of available agents
	result, err := client.CallTool(ctx, "agent_list", map[string]interface{}{
		"status": "online",
	})
	if err != nil {
		return err
	}

	var response struct {
		Agents []map[string]interface{} `json:"agents"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return fmt.Errorf("failed to parse agents: %w", err)
	}

	if len(response.Agents) == 0 {
		common.PrintInfo("No online agents available (tasks will remain unassigned)")
		return nil
	}

	// Assign each task to an agent (round-robin)
	for i, taskID := range taskIDs {
		agentIndex := i % len(response.Agents)
		agentID := response.Agents[agentIndex]["agent_id"].(string)

		_, err := client.CallTool(ctx, "task_assign", map[string]interface{}{
			"task_id":  taskID,
			"agent_id": agentID,
		})
		if err != nil {
			common.PrintError(fmt.Sprintf("Failed to assign task %s to agent %s", taskID, agentID), err)
			continue
		}

		common.PrintSuccess(fmt.Sprintf("Assigned task %s to agent %s", taskID, agentID))
	}

	return nil
}

func monitorTasks(ctx context.Context, client *common.MCPClient, taskIDs []string) error {
	common.PrintSubsection("Monitoring Task Status")

	if len(taskIDs) == 0 {
		common.PrintInfo("No tasks to monitor")
		return nil
	}

	// Get status of all tasks
	result, err := client.CallTool(ctx, "task_list", map[string]interface{}{})
	if err != nil {
		return err
	}

	var response struct {
		Tasks []map[string]interface{} `json:"tasks"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return fmt.Errorf("failed to parse tasks: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Found %d total tasks in system", len(response.Tasks)))

	// Display task summary
	statusCount := make(map[string]int)
	for _, task := range response.Tasks {
		status := task["status"].(string)
		statusCount[status]++
	}

	fmt.Println("\nTask Summary:")
	for status, count := range statusCount {
		fmt.Printf("  • %s: %d\n", status, count)
	}

	return nil
}

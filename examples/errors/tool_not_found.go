package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

func main() {
	common.PrintSection("Tool Not Found Error Handling Example")
	common.PrintInfo("Demonstrates handling and recovery from tool not found errors")

	// Create MCP client
	client, err := common.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example 1: Handle tool not found error
	handleToolNotFound(ctx, client)

	// Example 2: Fuzzy search for similar tools
	fuzzySearchAlternatives(ctx, client, "github_repo")

	// Example 3: Browse tools by category
	browseByCategory(ctx, client, "repository")

	// Example 4: Validate tool exists before calling
	validateBeforeCalling(ctx, client, "github_get_repository")

	common.PrintSection("Tool Discovery Complete")
	common.PrintSuccess("Demonstrated tool not found error recovery")
}

func handleToolNotFound(ctx context.Context, client *common.MCPClient) {
	common.PrintSubsection("Handling Tool Not Found Error")

	// Attempt to call a non-existent tool
	_, err := client.CallTool(ctx, "nonexistent_tool", map[string]interface{}{
		"param1": "value1",
	})

	if err != nil {
		code, message := common.ExtractErrorCode(err)
		common.PrintError("Tool not found", err)
		fmt.Printf("  Error Code: %d\n", code)
		fmt.Printf("  Error Message: %s\n", message)

		// Check if error is TOOL_NOT_FOUND
		if strings.Contains(message, "not found") || strings.Contains(message, "TOOL_NOT_FOUND") {
			common.PrintInfo("Recovery: Searching for alternative tools...")

			// Get all tools and search for similar ones
			tools, err := client.ListTools(ctx)
			if err != nil {
				common.PrintError("Failed to list tools", err)
				return
			}

			common.PrintSuccess(fmt.Sprintf("Available tools: %d", len(tools)))
			common.PrintInfo("Try using tools/list to discover available tools")
		}
	}
}

func fuzzySearchAlternatives(ctx context.Context, client *common.MCPClient, query string) {
	common.PrintSubsection(fmt.Sprintf("Fuzzy Search for: '%s'", query))

	// Get all tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		common.PrintError("Failed to list tools", err)
		return
	}

	// Fuzzy search
	matches := common.FuzzySearchTools(tools, query)

	if len(matches) > 0 {
		common.PrintSuccess(fmt.Sprintf("Found %d similar tools:", len(matches)))
		for i, tool := range matches {
			if i >= 5 {
				break
			}
			fmt.Printf("  %d. %s - %s\n", i+1, tool.Name, tool.Description)
			if len(tool.Tags) > 0 {
				fmt.Printf("     Tags: [%s]\n", strings.Join(tool.Tags, ", "))
			}
		}

		// Suggest the best match
		if len(matches) > 0 {
			common.PrintInfo(fmt.Sprintf("\nDid you mean: %s?", matches[0].Name))
		}
	} else {
		common.PrintInfo("No similar tools found. Try browsing by category.")
	}
}

func browseByCategory(ctx context.Context, client *common.MCPClient, category string) {
	common.PrintSubsection(fmt.Sprintf("Browsing Tools by Category: '%s'", category))

	// Get all tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		common.PrintError("Failed to list tools", err)
		return
	}

	// Filter by category
	categoryTools := common.FilterTools(tools, category)

	if len(categoryTools) > 0 {
		common.PrintSuccess(fmt.Sprintf("Found %d tools in category '%s':", len(categoryTools), category))
		for i, tool := range categoryTools {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(categoryTools)-10)
				break
			}
			fmt.Printf("  %d. %s - %s\n", i+1, tool.Name, tool.Description)
		}
	} else {
		common.PrintInfo(fmt.Sprintf("No tools found in category '%s'", category))

		// Show available categories
		categories := make(map[string]int)
		for _, tool := range tools {
			if tool.Category != "" {
				categories[tool.Category]++
			}
		}

		if len(categories) > 0 {
			common.PrintInfo("\nAvailable categories:")
			for cat, count := range categories {
				fmt.Printf("  • %s (%d tools)\n", cat, count)
			}
		}
	}
}

func validateBeforeCalling(ctx context.Context, client *common.MCPClient, toolName string) {
	common.PrintSubsection(fmt.Sprintf("Validating Tool Exists: '%s'", toolName))

	// Get all tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		common.PrintError("Failed to list tools", err)
		return
	}

	// Find the tool
	tool := common.FindTool(tools, toolName)

	if tool != nil {
		common.PrintSuccess(fmt.Sprintf("Tool '%s' exists", toolName))
		fmt.Printf("  Description: %s\n", tool.Description)
		fmt.Printf("  Category: %s\n", tool.Category)

		// Show required parameters
		if schema, ok := tool.InputSchema["properties"].(map[string]interface{}); ok {
			required := []string{}
			if reqList, ok := tool.InputSchema["required"].([]interface{}); ok {
				for _, r := range reqList {
					required = append(required, r.(string))
				}
			}

			if len(required) > 0 {
				fmt.Printf("  Required parameters: %s\n", strings.Join(required, ", "))
			}
		}

		// Now it's safe to call the tool
		common.PrintInfo("\nCalling validated tool...")
		result, err := client.CallTool(ctx, toolName, map[string]interface{}{
			"owner": "developer-mesh",
			"repo":  "developer-mesh",
		})
		if err != nil {
			common.PrintError("Tool execution failed", err)
		} else {
			common.PrintSuccess(fmt.Sprintf("Tool executed successfully, result size: %d bytes", len(result)))
		}
	} else {
		common.PrintError(fmt.Sprintf("Tool '%s' not found", toolName), fmt.Errorf("tool does not exist"))

		// Search for alternatives
		matches := common.FuzzySearchTools(tools, toolName)
		if len(matches) > 0 {
			common.PrintInfo("\nDid you mean one of these?")
			for i, match := range matches {
				if i >= 3 {
					break
				}
				fmt.Printf("  • %s\n", match.Name)
			}
		}
	}
}

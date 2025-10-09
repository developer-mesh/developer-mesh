package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

func main() {
	common.PrintSection("GitHub Operations Workflow Example")
	common.PrintInfo("Demonstrates GitHub repository operations using Edge MCP")

	// Create MCP client
	client, err := common.NewClient(nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example 1: List repositories
	if err := listRepositories(ctx, client); err != nil {
		common.PrintError("List repositories failed", err)
	}

	// Example 2: Get specific repository
	if err := getRepository(ctx, client, "developer-mesh", "developer-mesh"); err != nil {
		common.PrintError("Get repository failed", err)
	}

	// Example 3: List issues
	if err := listIssues(ctx, client, "developer-mesh", "developer-mesh"); err != nil {
		common.PrintError("List issues failed", err)
	}

	// Example 4: List pull requests
	if err := listPullRequests(ctx, client, "developer-mesh", "developer-mesh"); err != nil {
		common.PrintError("List pull requests failed", err)
	}

	// Example 5: Get commit information
	if err := getCommit(ctx, client, "developer-mesh", "developer-mesh", "main"); err != nil {
		common.PrintError("Get commit failed", err)
	}

	common.PrintSection("GitHub Operations Complete")
	common.PrintSuccess("All GitHub operations executed successfully")
}

func listRepositories(ctx context.Context, client *common.MCPClient) error {
	common.PrintSubsection("Listing Repositories")

	result, err := client.CallTool(ctx, "github_list_repositories", map[string]interface{}{
		"type": "owner",
		"sort": "updated",
	})
	if err != nil {
		return err
	}

	var repos []map[string]interface{}
	if err := json.Unmarshal(result, &repos); err != nil {
		return fmt.Errorf("failed to parse repositories: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Found %d repositories", len(repos)))
	if len(repos) > 0 {
		common.PrintInfo("First 3 repositories:")
		for i, repo := range repos {
			if i >= 3 {
				break
			}
			name := repo["name"]
			description := repo["description"]
			fmt.Printf("  %d. %s - %v\n", i+1, name, description)
		}
	}

	return nil
}

func getRepository(ctx context.Context, client *common.MCPClient, owner, repo string) error {
	common.PrintSubsection(fmt.Sprintf("Getting Repository: %s/%s", owner, repo))

	result, err := client.CallTool(ctx, "github_get_repository", map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	})
	if err != nil {
		return err
	}

	var repository map[string]interface{}
	if err := json.Unmarshal(result, &repository); err != nil {
		return fmt.Errorf("failed to parse repository: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Repository: %s", repository["full_name"]))
	fmt.Printf("  Description: %v\n", repository["description"])
	fmt.Printf("  Stars: %.0f\n", repository["stargazers_count"])
	fmt.Printf("  Forks: %.0f\n", repository["forks_count"])
	fmt.Printf("  Open Issues: %.0f\n", repository["open_issues_count"])
	fmt.Printf("  Language: %v\n", repository["language"])
	fmt.Printf("  Default Branch: %v\n", repository["default_branch"])

	return nil
}

func listIssues(ctx context.Context, client *common.MCPClient, owner, repo string) error {
	common.PrintSubsection(fmt.Sprintf("Listing Issues: %s/%s", owner, repo))

	result, err := client.CallTool(ctx, "github_list_issues", map[string]interface{}{
		"owner": owner,
		"repo":  repo,
		"state": "open",
	})
	if err != nil {
		return err
	}

	var issues []map[string]interface{}
	if err := json.Unmarshal(result, &issues); err != nil {
		return fmt.Errorf("failed to parse issues: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Found %d open issues", len(issues)))
	if len(issues) > 0 {
		common.PrintInfo("First 3 issues:")
		for i, issue := range issues {
			if i >= 3 {
				break
			}
			number := issue["number"]
			title := issue["title"]
			state := issue["state"]
			fmt.Printf("  #%.0f: %s (%s)\n", number, title, state)
		}
	}

	return nil
}

func listPullRequests(ctx context.Context, client *common.MCPClient, owner, repo string) error {
	common.PrintSubsection(fmt.Sprintf("Listing Pull Requests: %s/%s", owner, repo))

	result, err := client.CallTool(ctx, "github_list_pull_requests", map[string]interface{}{
		"owner": owner,
		"repo":  repo,
		"state": "open",
	})
	if err != nil {
		return err
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal(result, &prs); err != nil {
		return fmt.Errorf("failed to parse pull requests: %w", err)
	}

	common.PrintSuccess(fmt.Sprintf("Found %d open pull requests", len(prs)))
	if len(prs) > 0 {
		common.PrintInfo("First 3 pull requests:")
		for i, pr := range prs {
			if i >= 3 {
				break
			}
			number := pr["number"]
			title := pr["title"]
			state := pr["state"]
			fmt.Printf("  #%.0f: %s (%s)\n", number, title, state)
		}
	}

	return nil
}

func getCommit(ctx context.Context, client *common.MCPClient, owner, repo, ref string) error {
	common.PrintSubsection(fmt.Sprintf("Getting Latest Commit: %s/%s@%s", owner, repo, ref))

	// First, get the branch to get the latest commit SHA
	result, err := client.CallTool(ctx, "github_list_commits", map[string]interface{}{
		"owner":    owner,
		"repo":     repo,
		"sha":      ref,
		"per_page": 1,
	})
	if err != nil {
		return err
	}

	var commits []map[string]interface{}
	if err := json.Unmarshal(result, &commits); err != nil {
		return fmt.Errorf("failed to parse commits: %w", err)
	}

	if len(commits) == 0 {
		return fmt.Errorf("no commits found")
	}

	commit := commits[0]
	sha := commit["sha"]
	message := ""
	if commitData, ok := commit["commit"].(map[string]interface{}); ok {
		message = commitData["message"].(string)
	}

	common.PrintSuccess(fmt.Sprintf("Latest commit: %s", sha))
	fmt.Printf("  Message: %s\n", message)

	return nil
}

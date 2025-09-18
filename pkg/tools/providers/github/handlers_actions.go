package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/go-github/v74/github"
)

// Actions Handlers

// ListWorkflowsHandler handles listing workflows
type ListWorkflowsHandler struct {
	provider *GitHubProvider
}

func NewListWorkflowsHandler(p *GitHubProvider) *ListWorkflowsHandler {
	return &ListWorkflowsHandler{provider: p}
}

func (h *ListWorkflowsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_workflows",
		Description: "List all GitHub Actions workflow files in a repository's .github/workflows directory",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name without the .git extension (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of workflows to return per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
					"example":     50,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve (1-based pagination)",
					"default":     1,
					"minimum":     1,
					"example":     1,
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"category":    "github_actions",
			"scopes":      []string{"actions", "repo"},
			"rateLimit":   "standard",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"total_count": 2,
			"workflows": []map[string]interface{}{
				{
					"id":         161335,
					"name":       "CI",
					"path":       ".github/workflows/ci.yml",
					"state":      "active",
					"created_at": "2024-01-01T00:00:00Z",
					"updated_at": "2024-01-15T12:00:00Z",
				},
				{
					"id":         161336,
					"name":       "Deploy",
					"path":       ".github/workflows/deploy.yml",
					"state":      "active",
					"created_at": "2024-01-01T00:00:00Z",
					"updated_at": "2024-01-15T12:00:00Z",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"solution": "Verify the owner and repo names are correct and the repository exists.",
			},
			{
				"error":    "403 Forbidden",
				"solution": "Ensure you have read access to the repository and the 'actions' or 'repo' scope.",
			},
		},
		ExtendedHelp: "Lists all workflow files defined in the repository's .github/workflows directory. Each workflow represents an automated process that can be triggered by events. The response includes workflow IDs needed for other operations like triggering runs.",
	}
}

func (h *ListWorkflowsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	opts := &github.ListOptions{}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	workflows, _, err := client.Actions.ListWorkflows(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list workflows: %v", err)), nil
	}

	// Create simplified response to reduce token usage
	simplified := map[string]interface{}{
		"total_count": workflows.GetTotalCount(),
		"workflows":   make([]map[string]interface{}, 0, len(workflows.Workflows)),
	}

	for _, workflow := range workflows.Workflows {
		simplified["workflows"] = append(
			simplified["workflows"].([]map[string]interface{}),
			map[string]interface{}{
				"id":         workflow.GetID(),
				"name":       workflow.GetName(),
				"path":       workflow.GetPath(),
				"state":      workflow.GetState(),
				"created_at": workflow.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
				"updated_at": workflow.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
				"html_url":   workflow.GetHTMLURL(),
				"badge_url":  workflow.GetBadgeURL(),
			},
		)
	}

	data, _ := json.Marshal(simplified)
	return NewToolResult(string(data)), nil
}

// ListWorkflowRunsHandler handles listing workflow runs
type ListWorkflowRunsHandler struct {
	provider *GitHubProvider
}

func NewListWorkflowRunsHandler(p *GitHubProvider) *ListWorkflowRunsHandler {
	return &ListWorkflowRunsHandler{provider: p}
}

func (h *ListWorkflowRunsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_workflow_runs",
		Description: "List workflow runs for a repository or specific workflow with filtering options",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name without the .git extension (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"workflow_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional workflow ID (numeric) or filename (e.g., 'ci.yml') to filter runs",
					"example":     "ci.yml",
					"pattern":     "^[a-zA-Z0-9._-]+\\.(yml|yaml)$|^[0-9]+$",
				},
				"actor": map[string]interface{}{
					"type":        "string",
					"description": "Filter runs by the user who triggered them (GitHub username)",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Filter runs by branch name (e.g., 'main', 'develop')",
					"example":     "main",
				},
				"event": map[string]interface{}{
					"type":        "string",
					"description": "Filter runs by triggering event",
					"enum":        []interface{}{"push", "pull_request", "schedule", "workflow_dispatch", "repository_dispatch", "release", "deployment"},
					"example":     "push",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter runs by status",
					"enum":        []interface{}{"completed", "action_required", "cancelled", "failure", "neutral", "skipped", "stale", "success", "timed_out", "in_progress", "queued", "requested", "waiting", "pending"},
					"example":     "success",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of runs to return per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
					"example":     50,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve (1-based pagination)",
					"default":     1,
					"minimum":     1,
					"example":     1,
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
		Metadata: map[string]interface{}{
			"category":    "github_actions",
			"scopes":      []string{"actions", "repo"},
			"rateLimit":   "standard",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"total_count": 2,
			"workflow_runs": []map[string]interface{}{
				{
					"id":          1234567890,
					"name":        "CI",
					"status":      "completed",
					"conclusion":  "success",
					"workflow_id": 161335,
					"run_number":  42,
					"event":       "push",
					"head_branch": "main",
					"head_sha":    "abc123def456",
					"created_at":  "2024-01-15T12:00:00Z",
					"updated_at":  "2024-01-15T12:05:00Z",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"solution": "Verify the owner, repo, and workflow_id (if provided) are correct.",
			},
			{
				"error":    "422 Unprocessable Entity",
				"solution": "Check that filter parameters (status, event) use valid values from the enums.",
			},
			{
				"error":    "403 Forbidden",
				"solution": "Ensure you have read access to the repository and the 'actions' or 'repo' scope.",
			},
		},
		ExtendedHelp: "Lists workflow runs with various filtering options. You can filter by workflow, branch, status, event type, or the user who triggered the run. The response includes run IDs needed for operations like rerun, cancel, or viewing logs. Results are sorted by created_at in descending order (newest first).",
	}
}

func (h *ListWorkflowRunsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	opts := &github.ListWorkflowRunsOptions{}
	if actor, ok := params["actor"].(string); ok {
		opts.Actor = actor
	}
	if branch, ok := params["branch"].(string); ok {
		opts.Branch = branch
	}
	if event, ok := params["event"].(string); ok {
		opts.Event = event
	}
	if status, ok := params["status"].(string); ok {
		opts.Status = status
	}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	var runs *github.WorkflowRuns
	var err error

	if workflowIDStr, ok := params["workflow_id"].(string); ok {
		workflowID, _ := strconv.ParseInt(workflowIDStr, 10, 64)
		runs, _, err = client.Actions.ListWorkflowRunsByID(ctx, owner, repo, workflowID, opts)
	} else {
		runs, _, err = client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	}

	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list workflow runs: %v", err)), nil
	}

	// Create simplified response to reduce token usage
	simplified := map[string]interface{}{
		"total_count":   runs.GetTotalCount(),
		"workflow_runs": make([]map[string]interface{}, 0, len(runs.WorkflowRuns)),
	}

	for _, run := range runs.WorkflowRuns {
		simplified["workflow_runs"] = append(
			simplified["workflow_runs"].([]map[string]interface{}),
			simplifyWorkflowRun(run),
		)
	}

	data, _ := json.Marshal(simplified)
	return NewToolResult(string(data)), nil
}

// GetWorkflowRunHandler handles getting a specific workflow run
type GetWorkflowRunHandler struct {
	provider *GitHubProvider
}

func NewGetWorkflowRunHandler(p *GitHubProvider) *GetWorkflowRunHandler {
	return &GetWorkflowRunHandler{provider: p}
}

func (h *GetWorkflowRunHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_workflow_run",
		Description: "Get detailed information about a specific GitHub Actions workflow run including status, logs URL, and artifacts",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name without the .git extension (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"run_id": map[string]interface{}{
					"type":        "integer",
					"description": "Unique identifier of the workflow run (get from list_workflow_runs)",
					"example":     1234567890,
					"minimum":     1,
				},
			},
			"required": []interface{}{"owner", "repo", "run_id"},
		},
		Metadata: map[string]interface{}{
			"category":    "github_actions",
			"scopes":      []string{"actions", "repo"},
			"rateLimit":   "standard",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"id":             1234567890,
			"name":           "CI",
			"status":         "completed",
			"conclusion":     "success",
			"workflow_id":    161335,
			"workflow_url":   "https://api.github.com/repos/facebook/react/actions/workflows/161335",
			"run_number":     42,
			"run_attempt":    1,
			"event":          "push",
			"head_branch":    "main",
			"head_sha":       "abc123def456",
			"html_url":       "https://github.com/facebook/react/actions/runs/1234567890",
			"jobs_url":       "https://api.github.com/repos/facebook/react/actions/runs/1234567890/jobs",
			"logs_url":       "https://api.github.com/repos/facebook/react/actions/runs/1234567890/logs",
			"artifacts_url":  "https://api.github.com/repos/facebook/react/actions/runs/1234567890/artifacts",
			"created_at":     "2024-01-15T12:00:00Z",
			"updated_at":     "2024-01-15T12:05:00Z",
			"run_started_at": "2024-01-15T12:00:05Z",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"solution": "Verify the owner, repo, and run_id are correct. The workflow run must exist.",
			},
			{
				"error":    "403 Forbidden",
				"solution": "Ensure you have read access to the repository and the 'actions' or 'repo' scope.",
			},
		},
		ExtendedHelp: "Retrieves comprehensive details about a workflow run including its status, conclusion, timing, URLs for logs and artifacts, and metadata about the workflow and triggering event. Use this to monitor run progress or debug failed runs.",
	}
}

func (h *GetWorkflowRunHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	runID := int64(extractInt(params, "run_id"))

	run, _, err := client.Actions.GetWorkflowRunByID(ctx, owner, repo, runID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get workflow run: %v", err)), nil
	}

	data, _ := json.Marshal(run)
	return NewToolResult(string(data)), nil
}

// ListWorkflowJobsHandler handles listing workflow jobs
type ListWorkflowJobsHandler struct {
	provider *GitHubProvider
}

func NewListWorkflowJobsHandler(p *GitHubProvider) *ListWorkflowJobsHandler {
	return &ListWorkflowJobsHandler{provider: p}
}

func (h *ListWorkflowJobsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_workflow_jobs",
		Description: "List all jobs within a GitHub Actions workflow run, showing individual job status and details",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name without the .git extension (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"run_id": map[string]interface{}{
					"type":        "integer",
					"description": "Unique identifier of the workflow run (get from list_workflow_runs)",
					"example":     1234567890,
					"minimum":     1,
				},
				"filter": map[string]interface{}{
					"type":        "string",
					"description": "Filter jobs by attempt number",
					"enum":        []interface{}{"latest", "all"},
					"default":     "all",
					"example":     "latest",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Number of jobs to return per page (1-100)",
					"default":     30,
					"minimum":     1,
					"maximum":     100,
					"example":     50,
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve (1-based pagination)",
					"default":     1,
					"minimum":     1,
					"example":     1,
				},
			},
			"required": []interface{}{"owner", "repo", "run_id"},
		},
		Metadata: map[string]interface{}{
			"category":    "github_actions",
			"scopes":      []string{"actions", "repo"},
			"rateLimit":   "standard",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"total_count": 3,
			"jobs": []map[string]interface{}{
				{
					"id":            21511112,
					"run_id":        1234567890,
					"workflow_name": "CI",
					"name":          "test",
					"status":        "completed",
					"conclusion":    "success",
					"started_at":    "2024-01-15T12:00:10Z",
					"completed_at":  "2024-01-15T12:02:30Z",
					"steps": []map[string]interface{}{
						{
							"name":       "Set up Node",
							"status":     "completed",
							"conclusion": "success",
							"number":     1,
						},
						{
							"name":       "Run tests",
							"status":     "completed",
							"conclusion": "success",
							"number":     2,
						},
					},
					"runner_name":       "GitHub Actions 2",
					"runner_group_name": "GitHub Actions",
				},
			},
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"solution": "Verify the owner, repo, and run_id are correct. The workflow run must exist.",
			},
			{
				"error":    "403 Forbidden",
				"solution": "Ensure you have read access to the repository and the 'actions' or 'repo' scope.",
			},
		},
		ExtendedHelp: "Lists all jobs that are part of a workflow run. Each job represents a set of steps that execute on the same runner. The response includes job status, conclusion, timing, and the steps within each job. Use 'filter=latest' to see only the most recent attempt of each job when runs are retried.",
	}
}

func (h *ListWorkflowJobsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	runID := int64(extractInt(params, "run_id"))

	opts := &github.ListWorkflowJobsOptions{}
	if filter, ok := params["filter"].(string); ok {
		opts.Filter = filter
	}
	if perPage, ok := params["per_page"].(float64); ok {
		opts.PerPage = int(perPage)
	}
	if page, ok := params["page"].(float64); ok {
		opts.Page = int(page)
	}

	jobs, _, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list workflow jobs: %v", err)), nil
	}

	data, _ := json.Marshal(jobs)
	return NewToolResult(string(data)), nil
}

// RunWorkflowHandler handles triggering a workflow
type RunWorkflowHandler struct {
	provider *GitHubProvider
}

func NewRunWorkflowHandler(p *GitHubProvider) *RunWorkflowHandler {
	return &RunWorkflowHandler{provider: p}
}

func (h *RunWorkflowHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "run_workflow",
		Description: "Manually trigger a GitHub Actions workflow using workflow_dispatch event",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization (e.g., 'facebook', 'microsoft')",
					"example":     "octocat",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name (e.g., 'react', 'vscode')",
					"example":     "Hello-World",
					"pattern":     "^[a-zA-Z0-9._-]+$",
				},
				"workflow_id": map[string]interface{}{
					"type":        "string",
					"description": "Workflow ID (numeric) or workflow filename (e.g., 'ci.yml', 'deploy.yaml')",
					"example":     "ci.yml",
					"pattern":     "^[a-zA-Z0-9._-]+\\.(yml|yaml)$|^[0-9]+$",
				},
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Git reference (branch name, tag, or commit SHA) to run workflow on",
					"example":     "main",
					"minLength":   1,
				},
				"inputs": map[string]interface{}{
					"type":        "object",
					"description": "Input parameters defined in workflow file (workflow_dispatch.inputs)",
					"example": map[string]interface{}{
						"environment": "production",
						"version":     "1.2.3",
					},
					"additionalProperties": true,
				},
			},
			"required": []interface{}{"owner", "repo", "workflow_id", "ref"},
			"metadata": map[string]interface{}{
				"requiredScopes":    []string{"actions:write", "repo"},
				"minimumScopes":     []string{"actions:write"},
				"rateLimitCategory": "core",
				"requestsPerHour":   5000,
			},
			"responseExample": map[string]interface{}{
				"success": "Workflow triggered successfully",
				"error": map[string]interface{}{
					"message":           "Not Found",
					"documentation_url": "https://docs.github.com/rest/actions/workflows#create-a-workflow-dispatch-event",
				},
			},
			"commonErrors": []map[string]interface{}{
				{
					"code":     404,
					"reason":   "Workflow not found or doesn't have workflow_dispatch trigger",
					"solution": "Ensure workflow exists and has 'on: workflow_dispatch' in its configuration",
				},
				{
					"code":     422,
					"reason":   "Invalid ref or missing required inputs",
					"solution": "Verify branch/tag exists and all required workflow inputs are provided",
				},
				{
					"code":     403,
					"reason":   "Insufficient permissions to trigger workflows",
					"solution": "Ensure you have 'actions:write' permission for the repository",
				},
			},
			"extendedHelp": "Triggers a workflow that has 'workflow_dispatch' event configured. The workflow must exist in the specified ref. Inputs must match those defined in the workflow file. See https://docs.github.com/actions/using-workflows/manually-running-a-workflow",
		},
	}
}

func (h *RunWorkflowHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	workflowIDStr := extractString(params, "workflow_id")
	workflowID, _ := strconv.ParseInt(workflowIDStr, 10, 64)
	ref := extractString(params, "ref")

	event := github.CreateWorkflowDispatchEventRequest{
		Ref: ref,
	}

	if inputs, ok := params["inputs"].(map[string]interface{}); ok {
		event.Inputs = inputs
	}

	_, err := client.Actions.CreateWorkflowDispatchEventByID(ctx, owner, repo, workflowID, event)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to trigger workflow: %v", err)), nil
	}

	return NewToolResult("Workflow triggered successfully"), nil
}

// RerunWorkflowRunHandler handles rerunning a workflow
type RerunWorkflowRunHandler struct {
	provider *GitHubProvider
}

func NewRerunWorkflowRunHandler(p *GitHubProvider) *RerunWorkflowRunHandler {
	return &RerunWorkflowRunHandler{provider: p}
}

func (h *RerunWorkflowRunHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "rerun_workflow_run",
		Description: "Rerun a completed or failed GitHub Actions workflow run using the same commit SHA and inputs",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name without the .git extension (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"run_id": map[string]interface{}{
					"type":        "integer",
					"description": "Unique identifier of the workflow run to rerun (get from list_workflow_runs)",
					"example":     1234567890,
					"minimum":     1,
				},
			},
			"required": []interface{}{"owner", "repo", "run_id"},
		},
		Metadata: map[string]interface{}{
			"category":    "github_actions",
			"scopes":      []string{"actions"},
			"rateLimit":   "standard",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"message": "Workflow rerun initiated",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"solution": "Verify the owner, repo, and run_id are correct. The workflow run must exist.",
			},
			{
				"error":    "403 Forbidden",
				"solution": "Ensure you have 'actions' scope and write permissions to the repository.",
			},
			{
				"error":    "422 Unprocessable Entity",
				"solution": "The workflow run may still be in progress. Only completed or failed runs can be rerun.",
			},
		},
		ExtendedHelp: "Reruns an entire workflow run using the same commit SHA and workflow inputs. This is useful for retrying failed builds due to transient issues. All jobs in the workflow will be rerun. For partial reruns, use rerun_failed_jobs instead.",
	}
}

func (h *RerunWorkflowRunHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	runID := int64(extractInt(params, "run_id"))

	_, err := client.Actions.RerunWorkflowByID(ctx, owner, repo, runID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to rerun workflow: %v", err)), nil
	}

	return NewToolResult("Workflow rerun initiated"), nil
}

// CancelWorkflowRunHandler handles canceling a workflow run
type CancelWorkflowRunHandler struct {
	provider *GitHubProvider
}

func NewCancelWorkflowRunHandler(p *GitHubProvider) *CancelWorkflowRunHandler {
	return &CancelWorkflowRunHandler{provider: p}
}

func (h *CancelWorkflowRunHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "cancel_workflow_run",
		Description: "Cancel a running or queued GitHub Actions workflow run to stop execution and save resources",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner username or organization name (e.g., 'facebook', 'microsoft')",
					"example":     "facebook",
					"pattern":     "^[a-zA-Z0-9][a-zA-Z0-9-]*$",
					"minLength":   1,
					"maxLength":   39,
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name without the .git extension (e.g., 'react', 'vscode')",
					"example":     "react",
					"pattern":     "^[a-zA-Z0-9._-]+$",
					"minLength":   1,
					"maxLength":   100,
				},
				"run_id": map[string]interface{}{
					"type":        "integer",
					"description": "Unique identifier of the workflow run to cancel (get from list_workflow_runs)",
					"example":     1234567890,
					"minimum":     1,
				},
			},
			"required": []interface{}{"owner", "repo", "run_id"},
		},
		Metadata: map[string]interface{}{
			"category":    "github_actions",
			"scopes":      []string{"actions"},
			"rateLimit":   "standard",
			"destructive": false,
		},
		ResponseExample: map[string]interface{}{
			"message": "Workflow cancellation initiated",
		},
		CommonErrors: []map[string]interface{}{
			{
				"error":    "404 Not Found",
				"solution": "Verify the owner, repo, and run_id are correct. Check if the workflow run exists.",
			},
			{
				"error":    "409 Conflict",
				"solution": "The workflow run may already be completed or cancelled. Check the run status first.",
			},
			{
				"error":    "403 Forbidden",
				"solution": "Ensure you have 'actions' scope and write permissions to the repository.",
			},
		},
		ExtendedHelp: "Cancels a workflow run that is in progress. The cancellation is asynchronous - the API returns immediately but the actual cancellation may take a few moments. Already completed or failed runs cannot be cancelled.",
	}
}

func (h *CancelWorkflowRunHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	runID := int64(extractInt(params, "run_id"))

	_, err := client.Actions.CancelWorkflowRunByID(ctx, owner, repo, runID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to cancel workflow: %v", err)), nil
	}

	return NewToolResult("Workflow cancellation initiated"), nil
}

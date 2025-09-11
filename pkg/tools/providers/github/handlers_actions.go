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
		Description: "List workflows in a GitHub repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *ListWorkflowsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)

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

	data, _ := json.Marshal(workflows)
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
		Description: "List workflow runs in a GitHub repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"workflow_id": map[string]interface{}{
					"type":        "string",
					"description": "Workflow ID or filename",
				},
				"actor": map[string]interface{}{
					"type":        "string",
					"description": "Filter by actor username",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Filter by branch",
				},
				"event": map[string]interface{}{
					"type":        "string",
					"description": "Filter by event type",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter by status",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
			"required": []interface{}{"owner", "repo"},
		},
	}
}

func (h *ListWorkflowRunsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)

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

	data, _ := json.Marshal(runs)
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
		Description: "Get a specific workflow run",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"run_id": map[string]interface{}{
					"type":        "integer",
					"description": "Workflow run ID",
				},
			},
			"required": []interface{}{"owner", "repo", "run_id"},
		},
	}
}

func (h *GetWorkflowRunHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	runID := int64(params["run_id"].(float64))

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
		Description: "List jobs for a workflow run",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"run_id": map[string]interface{}{
					"type":        "integer",
					"description": "Workflow run ID",
				},
				"filter": map[string]interface{}{
					"type":        "string",
					"description": "Filter: latest or all",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number to retrieve",
				},
			},
			"required": []interface{}{"owner", "repo", "run_id"},
		},
	}
}

func (h *ListWorkflowJobsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	runID := int64(params["run_id"].(float64))

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
		Description: "Trigger a workflow dispatch event",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"workflow_id": map[string]interface{}{
					"type":        "string",
					"description": "Workflow ID or filename",
				},
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Branch or tag ref",
				},
				"inputs": map[string]interface{}{
					"type":        "object",
					"description": "Workflow inputs",
				},
			},
			"required": []interface{}{"owner", "repo", "workflow_id", "ref"},
		},
	}
}

func (h *RunWorkflowHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	workflowIDStr, _ := params["workflow_id"].(string)
	workflowID, _ := strconv.ParseInt(workflowIDStr, 10, 64)
	ref, _ := params["ref"].(string)

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
		Description: "Rerun a workflow run",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"run_id": map[string]interface{}{
					"type":        "integer",
					"description": "Workflow run ID",
				},
			},
			"required": []interface{}{"owner", "repo", "run_id"},
		},
	}
}

func (h *RerunWorkflowRunHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	runID := int64(params["run_id"].(float64))

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
		Description: "Cancel a workflow run",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "Repository owner",
				},
				"repo": map[string]interface{}{
					"type":        "string",
					"description": "Repository name",
				},
				"run_id": map[string]interface{}{
					"type":        "integer",
					"description": "Workflow run ID",
				},
			},
			"required": []interface{}{"owner", "repo", "run_id"},
		},
	}
}

func (h *CancelWorkflowRunHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner, _ := params["owner"].(string)
	repo, _ := params["repo"].(string)
	runID := int64(params["run_id"].(float64))

	_, err := client.Actions.CancelWorkflowRunByID(ctx, owner, repo, runID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to cancel workflow: %v", err)), nil
	}

	return NewToolResult("Workflow cancellation initiated"), nil
}

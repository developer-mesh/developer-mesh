package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/google/go-github/v74/github"
)

// Actions Extended Operations

// RerunFailedJobsHandler handles rerunning failed jobs in a workflow run
type RerunFailedJobsHandler struct {
	provider *GitHubProvider
}

func NewRerunFailedJobsHandler(p *GitHubProvider) *RerunFailedJobsHandler {
	return &RerunFailedJobsHandler{provider: p}
}

func (h *RerunFailedJobsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "rerun_failed_jobs",
		Description: "Re-run only failed jobs in workflow. Use when: retrying flaky tests, fixing specific failure, partial re-run.",
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

func (h *RerunFailedJobsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	runID := int64(extractInt(params, "run_id"))

	_, err := client.Actions.RerunFailedJobsByID(ctx, owner, repo, runID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to rerun failed jobs: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status": "rerun_initiated",
		"run_id": strconv.FormatInt(runID, 10),
	}), nil
}

// GetJobLogsHandler handles getting logs for a specific job
type GetJobLogsHandler struct {
	provider *GitHubProvider
}

func NewGetJobLogsHandler(p *GitHubProvider) *GetJobLogsHandler {
	return &GetJobLogsHandler{provider: p}
}

func (h *GetJobLogsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_job_logs",
		Description: "Get specific job logs (single job output). Use when: debugging job failure, checking specific step, analyzing job output.",
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
				"job_id": map[string]interface{}{
					"type":        "integer",
					"description": "Job ID",
				},
			},
			"required": []interface{}{"owner", "repo", "job_id"},
		},
	}
}

func (h *GetJobLogsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	jobID := int64(extractInt(params, "job_id"))

	url, _, err := client.Actions.GetWorkflowJobLogs(ctx, owner, repo, jobID, 2)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get job logs URL: %v", err)), nil
	}

	// Fetch logs from the URL
	resp, err := http.Get(url.String())
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to fetch job logs: %v", err)), nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	logs, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to read job logs: %v", err)), nil
	}

	return NewToolResult(map[string]interface{}{
		"job_id": jobID,
		"logs":   string(logs),
	}), nil
}

// GetWorkflowRunLogsHandler handles getting logs for an entire workflow run
type GetWorkflowRunLogsHandler struct {
	provider *GitHubProvider
}

func NewGetWorkflowRunLogsHandler(p *GitHubProvider) *GetWorkflowRunLogsHandler {
	return &GetWorkflowRunLogsHandler{provider: p}
}

func (h *GetWorkflowRunLogsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_workflow_run_logs",
		Description: "Get full workflow logs (all jobs, all steps). Use when: debugging failure, analyzing build output, troubleshooting CI/CD.",
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

func (h *GetWorkflowRunLogsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	runID := int64(extractInt(params, "run_id"))

	url, _, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, 2)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get workflow run logs URL: %v", err)), nil
	}

	return NewToolResult(map[string]interface{}{
		"run_id":       runID,
		"download_url": url.String(),
		"note":         "Download the logs as a zip file from the provided URL",
	}), nil
}

// GetWorkflowRunUsageHandler handles getting usage information for a workflow run
type GetWorkflowRunUsageHandler struct {
	provider *GitHubProvider
}

func NewGetWorkflowRunUsageHandler(p *GitHubProvider) *GetWorkflowRunUsageHandler {
	return &GetWorkflowRunUsageHandler{provider: p}
}

func (h *GetWorkflowRunUsageHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_workflow_run_usage",
		Description: "Get run billable time (minutes by OS). Use when: checking costs, analyzing usage, optimizing billing.",
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

func (h *GetWorkflowRunUsageHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	runID := int64(extractInt(params, "run_id"))

	usage, _, err := client.Actions.GetWorkflowRunUsageByID(ctx, owner, repo, runID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get workflow run usage: %v", err)), nil
	}

	data, _ := json.Marshal(usage)
	return NewToolResult(string(data)), nil
}

// ListArtifactsHandler handles listing artifacts for a repository or workflow run
type ListArtifactsHandler struct {
	provider *GitHubProvider
}

func NewListArtifactsHandler(p *GitHubProvider) *ListArtifactsHandler {
	return &ListArtifactsHandler{provider: p}
}

func (h *ListArtifactsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_artifacts",
		Description: "List artifacts from run (name, size, expired). Use when: downloading build outputs, accessing test results, fetching packages.",
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
					"description": "Optional workflow run ID to filter artifacts",
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

func (h *ListArtifactsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")

	pagination := ExtractPagination(params)
	opts := &github.ListArtifactsOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	var artifacts *github.ArtifactList
	var err error

	if runIDFloat, ok := params["run_id"].(float64); ok {
		// List artifacts for specific workflow run
		runID := int64(runIDFloat)
		artifacts, _, err = client.Actions.ListWorkflowRunArtifacts(ctx, owner, repo, runID, &opts.ListOptions)
	} else {
		// List all artifacts for repository
		artifacts, _, err = client.Actions.ListArtifacts(ctx, owner, repo, opts)
	}

	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list artifacts: %v", err)), nil
	}

	data, _ := json.Marshal(artifacts)
	return NewToolResult(string(data)), nil
}

// DownloadArtifactHandler handles downloading an artifact
type DownloadArtifactHandler struct {
	provider *GitHubProvider
}

func NewDownloadArtifactHandler(p *GitHubProvider) *DownloadArtifactHandler {
	return &DownloadArtifactHandler{provider: p}
}

func (h *DownloadArtifactHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "download_artifact",
		Description: "Download artifact by ID (returns zip). Use when: getting build artifacts, downloading test results, fetching packages.",
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
				"artifact_id": map[string]interface{}{
					"type":        "integer",
					"description": "Artifact ID",
				},
			},
			"required": []interface{}{"owner", "repo", "artifact_id"},
		},
	}
}

func (h *DownloadArtifactHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	artifactID := int64(extractInt(params, "artifact_id"))

	url, _, err := client.Actions.DownloadArtifact(ctx, owner, repo, artifactID, 2)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get artifact download URL: %v", err)), nil
	}

	return NewToolResult(map[string]interface{}{
		"artifact_id":  artifactID,
		"download_url": url.String(),
		"note":         "Download the artifact as a zip file from the provided URL",
	}), nil
}

// DeleteWorkflowRunLogsHandler handles deleting logs for a workflow run
type DeleteWorkflowRunLogsHandler struct {
	provider *GitHubProvider
}

func NewDeleteWorkflowRunLogsHandler(p *GitHubProvider) *DeleteWorkflowRunLogsHandler {
	return &DeleteWorkflowRunLogsHandler{provider: p}
}

func (h *DeleteWorkflowRunLogsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "delete_workflow_run_logs",
		Description: "Delete workflow logs (compliance/cleanup). Use when: removing sensitive logs, cleaning up storage, compliance.",
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

func (h *DeleteWorkflowRunLogsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := GetGitHubClientFromContext(ctx)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	runID := int64(extractInt(params, "run_id"))

	_, err := client.Actions.DeleteWorkflowRunLogs(ctx, owner, repo, runID)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to delete workflow run logs: %v", err)), nil
	}

	return NewToolResult(map[string]string{
		"status": "deleted",
		"run_id": strconv.FormatInt(runID, 10),
	}), nil
}

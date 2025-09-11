package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v74/github"
)

// Security Operations - Code Scanning, Dependabot, Secret Scanning

// ListCodeScanningAlertsHandler handles listing code scanning alerts
type ListCodeScanningAlertsHandler struct {
	provider *GitHubProvider
}

func NewListCodeScanningAlertsHandler(p *GitHubProvider) *ListCodeScanningAlertsHandler {
	return &ListCodeScanningAlertsHandler{provider: p}
}

func (h *ListCodeScanningAlertsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_code_scanning_alerts",
		Description: "List code scanning alerts for a repository",
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
				"state": map[string]interface{}{
					"type":        "string",
					"description": "State of the alert (open, closed, dismissed, fixed)",
				},
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Git reference (branch, tag, commit SHA)",
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

func (h *ListCodeScanningAlertsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	
	pagination := ExtractPagination(params)
	opts := &github.AlertListOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	if state := extractString(params, "state"); state != "" {
		opts.State = state
	}
	if ref := extractString(params, "ref"); ref != "" {
		opts.Ref = ref
	}

	alerts, _, err := client.CodeScanning.ListAlertsForRepo(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list code scanning alerts: %v", err)), nil
	}

	data, _ := json.Marshal(alerts)
	return NewToolResult(string(data)), nil
}

// GetCodeScanningAlertHandler handles getting a specific code scanning alert
type GetCodeScanningAlertHandler struct {
	provider *GitHubProvider
}

func NewGetCodeScanningAlertHandler(p *GitHubProvider) *GetCodeScanningAlertHandler {
	return &GetCodeScanningAlertHandler{provider: p}
}

func (h *GetCodeScanningAlertHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_code_scanning_alert",
		Description: "Get a specific code scanning alert",
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
				"alert_number": map[string]interface{}{
					"type":        "integer",
					"description": "Alert number",
				},
			},
			"required": []interface{}{"owner", "repo", "alert_number"},
		},
	}
}

func (h *GetCodeScanningAlertHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	alertNumber := int64(extractInt(params, "alert_number"))

	alert, _, err := client.CodeScanning.GetAlert(ctx, owner, repo, alertNumber)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get code scanning alert: %v", err)), nil
	}

	data, _ := json.Marshal(alert)
	return NewToolResult(string(data)), nil
}

// UpdateCodeScanningAlertHandler handles updating a code scanning alert
type UpdateCodeScanningAlertHandler struct {
	provider *GitHubProvider
}

func NewUpdateCodeScanningAlertHandler(p *GitHubProvider) *UpdateCodeScanningAlertHandler {
	return &UpdateCodeScanningAlertHandler{provider: p}
}

func (h *UpdateCodeScanningAlertHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_code_scanning_alert",
		Description: "Update a code scanning alert (dismiss or reopen)",
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
				"alert_number": map[string]interface{}{
					"type":        "integer",
					"description": "Alert number",
				},
				"state": map[string]interface{}{
					"type":        "string",
					"description": "State to set (open, dismissed)",
				},
				"dismissed_reason": map[string]interface{}{
					"type":        "string",
					"description": "Reason for dismissal (false_positive, wont_fix, used_in_tests)",
				},
				"dismissed_comment": map[string]interface{}{
					"type":        "string",
					"description": "Comment explaining dismissal",
				},
			},
			"required": []interface{}{"owner", "repo", "alert_number", "state"},
		},
	}
}

func (h *UpdateCodeScanningAlertHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	alertNumber := int64(extractInt(params, "alert_number"))
	state := extractString(params, "state")

	opts := &github.CodeScanningAlertState{
		State: state,
	}

	if reason := extractString(params, "dismissed_reason"); reason != "" {
		opts.DismissedReason = &reason
	}
	if comment := extractString(params, "dismissed_comment"); comment != "" {
		opts.DismissedComment = &comment
	}

	alert, _, err := client.CodeScanning.UpdateAlert(ctx, owner, repo, alertNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update code scanning alert: %v", err)), nil
	}

	data, _ := json.Marshal(alert)
	return NewToolResult(string(data)), nil
}

// ListDependabotAlertsHandler handles listing Dependabot alerts
type ListDependabotAlertsHandler struct {
	provider *GitHubProvider
}

func NewListDependabotAlertsHandler(p *GitHubProvider) *ListDependabotAlertsHandler {
	return &ListDependabotAlertsHandler{provider: p}
}

func (h *ListDependabotAlertsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_dependabot_alerts",
		Description: "List Dependabot security alerts for a repository",
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
				"state": map[string]interface{}{
					"type":        "string",
					"description": "State of the alert (open, dismissed, fixed)",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"description": "Severity of the alert (low, medium, high, critical)",
				},
				"ecosystem": map[string]interface{}{
					"type":        "string",
					"description": "Package ecosystem (npm, pip, maven, etc.)",
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

func (h *ListDependabotAlertsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	
	pagination := ExtractPagination(params)
	opts := &github.ListAlertsOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	if state := extractString(params, "state"); state != "" {
		opts.State = &state
	}
	if severity := extractString(params, "severity"); severity != "" {
		opts.Severity = &severity
	}
	if ecosystem := extractString(params, "ecosystem"); ecosystem != "" {
		opts.Ecosystem = &ecosystem
	}

	alerts, _, err := client.Dependabot.ListRepoAlerts(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list Dependabot alerts: %v", err)), nil
	}

	data, _ := json.Marshal(alerts)
	return NewToolResult(string(data)), nil
}

// GetDependabotAlertHandler handles getting a specific Dependabot alert
type GetDependabotAlertHandler struct {
	provider *GitHubProvider
}

func NewGetDependabotAlertHandler(p *GitHubProvider) *GetDependabotAlertHandler {
	return &GetDependabotAlertHandler{provider: p}
}

func (h *GetDependabotAlertHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_dependabot_alert",
		Description: "Get a specific Dependabot alert",
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
				"alert_number": map[string]interface{}{
					"type":        "integer",
					"description": "Alert number",
				},
			},
			"required": []interface{}{"owner", "repo", "alert_number"},
		},
	}
}

func (h *GetDependabotAlertHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	alertNumber := extractInt(params, "alert_number")

	alert, _, err := client.Dependabot.GetRepoAlert(ctx, owner, repo, alertNumber)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get Dependabot alert: %v", err)), nil
	}

	data, _ := json.Marshal(alert)
	return NewToolResult(string(data)), nil
}

// UpdateDependabotAlertHandler handles updating a Dependabot alert
type UpdateDependabotAlertHandler struct {
	provider *GitHubProvider
}

func NewUpdateDependabotAlertHandler(p *GitHubProvider) *UpdateDependabotAlertHandler {
	return &UpdateDependabotAlertHandler{provider: p}
}

func (h *UpdateDependabotAlertHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_dependabot_alert",
		Description: "Update a Dependabot alert (dismiss or reopen)",
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
				"alert_number": map[string]interface{}{
					"type":        "integer",
					"description": "Alert number",
				},
				"state": map[string]interface{}{
					"type":        "string",
					"description": "State to set (open, dismissed)",
				},
				"dismissed_reason": map[string]interface{}{
					"type":        "string",
					"description": "Reason for dismissal (fix_started, inaccurate, no_bandwidth, not_used, tolerable_risk)",
				},
				"dismissed_comment": map[string]interface{}{
					"type":        "string",
					"description": "Comment explaining dismissal",
				},
			},
			"required": []interface{}{"owner", "repo", "alert_number", "state"},
		},
	}
}

func (h *UpdateDependabotAlertHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	alertNumber := extractInt(params, "alert_number")
	state := extractString(params, "state")

	var dismissedReason *string
	var dismissedComment *string
	
	if reason := extractString(params, "dismissed_reason"); reason != "" {
		dismissedReason = &reason
	}
	if comment := extractString(params, "dismissed_comment"); comment != "" {
		dismissedComment = &comment
	}

	alert, _, err := client.Dependabot.UpdateAlert(ctx, owner, repo, alertNumber, &github.DependabotAlertState{
		State:            state,
		DismissedReason:  dismissedReason,
		DismissedComment: dismissedComment,
	})
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update Dependabot alert: %v", err)), nil
	}

	data, _ := json.Marshal(alert)
	return NewToolResult(string(data)), nil
}

// ListSecretScanningAlertsHandler handles listing secret scanning alerts
type ListSecretScanningAlertsHandler struct {
	provider *GitHubProvider
}

func NewListSecretScanningAlertsHandler(p *GitHubProvider) *ListSecretScanningAlertsHandler {
	return &ListSecretScanningAlertsHandler{provider: p}
}

func (h *ListSecretScanningAlertsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_secret_scanning_alerts",
		Description: "List secret scanning alerts for a repository",
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
				"state": map[string]interface{}{
					"type":        "string",
					"description": "State of the alert (open, resolved)",
				},
				"secret_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of secret detected",
				},
				"resolution": map[string]interface{}{
					"type":        "string",
					"description": "Resolution status (false_positive, wont_fix, revoked, used_in_tests)",
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

func (h *ListSecretScanningAlertsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	
	pagination := ExtractPagination(params)
	opts := &github.SecretScanningAlertListOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	if state := extractString(params, "state"); state != "" {
		opts.State = state
	}
	if secretType := extractString(params, "secret_type"); secretType != "" {
		opts.SecretType = secretType
	}
	if resolution := extractString(params, "resolution"); resolution != "" {
		opts.Resolution = resolution
	}

	alerts, _, err := client.SecretScanning.ListAlertsForRepo(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list secret scanning alerts: %v", err)), nil
	}

	data, _ := json.Marshal(alerts)
	return NewToolResult(string(data)), nil
}

// GetSecretScanningAlertHandler handles getting a specific secret scanning alert
type GetSecretScanningAlertHandler struct {
	provider *GitHubProvider
}

func NewGetSecretScanningAlertHandler(p *GitHubProvider) *GetSecretScanningAlertHandler {
	return &GetSecretScanningAlertHandler{provider: p}
}

func (h *GetSecretScanningAlertHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_secret_scanning_alert",
		Description: "Get a specific secret scanning alert",
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
				"alert_number": map[string]interface{}{
					"type":        "integer",
					"description": "Alert number",
				},
			},
			"required": []interface{}{"owner", "repo", "alert_number"},
		},
	}
}

func (h *GetSecretScanningAlertHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	alertNumber := int64(extractInt(params, "alert_number"))

	alert, _, err := client.SecretScanning.GetAlert(ctx, owner, repo, alertNumber)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to get secret scanning alert: %v", err)), nil
	}

	data, _ := json.Marshal(alert)
	return NewToolResult(string(data)), nil
}

// UpdateSecretScanningAlertHandler handles updating a secret scanning alert
type UpdateSecretScanningAlertHandler struct {
	provider *GitHubProvider
}

func NewUpdateSecretScanningAlertHandler(p *GitHubProvider) *UpdateSecretScanningAlertHandler {
	return &UpdateSecretScanningAlertHandler{provider: p}
}

func (h *UpdateSecretScanningAlertHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "update_secret_scanning_alert",
		Description: "Update a secret scanning alert (resolve or reopen)",
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
				"alert_number": map[string]interface{}{
					"type":        "integer",
					"description": "Alert number",
				},
				"state": map[string]interface{}{
					"type":        "string",
					"description": "State to set (open, resolved)",
				},
				"resolution": map[string]interface{}{
					"type":        "string",
					"description": "Resolution type (false_positive, wont_fix, revoked, used_in_tests)",
				},
				"resolution_comment": map[string]interface{}{
					"type":        "string",
					"description": "Comment explaining resolution",
				},
			},
			"required": []interface{}{"owner", "repo", "alert_number", "state"},
		},
	}
}

func (h *UpdateSecretScanningAlertHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	alertNumber := int64(extractInt(params, "alert_number"))
	state := extractString(params, "state")

	opts := &github.SecretScanningAlertUpdateOptions{
		State: state,
	}

	if resolution := extractString(params, "resolution"); resolution != "" {
		opts.Resolution = &resolution
	}
	if comment := extractString(params, "resolution_comment"); comment != "" {
		opts.ResolutionComment = &comment
	}

	alert, _, err := client.SecretScanning.UpdateAlert(ctx, owner, repo, alertNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to update secret scanning alert: %v", err)), nil
	}

	data, _ := json.Marshal(alert)
	return NewToolResult(string(data)), nil
}

// ListSecretScanningLocationsHandler handles listing locations for a secret scanning alert
type ListSecretScanningLocationsHandler struct {
	provider *GitHubProvider
}

func NewListSecretScanningLocationsHandler(p *GitHubProvider) *ListSecretScanningLocationsHandler {
	return &ListSecretScanningLocationsHandler{provider: p}
}

func (h *ListSecretScanningLocationsHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_secret_scanning_locations",
		Description: "List locations where a secret was detected",
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
				"alert_number": map[string]interface{}{
					"type":        "integer",
					"description": "Alert number",
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
			"required": []interface{}{"owner", "repo", "alert_number"},
		},
	}
}

func (h *ListSecretScanningLocationsHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	alertNumber := int64(extractInt(params, "alert_number"))
	
	pagination := ExtractPagination(params)
	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	locations, _, err := client.SecretScanning.ListLocationsForAlert(ctx, owner, repo, alertNumber, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list secret scanning locations: %v", err)), nil
	}

	data, _ := json.Marshal(locations)
	return NewToolResult(string(data)), nil
}

// ListSecurityAdvisoriesHandler handles listing security advisories for a repository
type ListSecurityAdvisoriesHandler struct {
	provider *GitHubProvider
}

func NewListSecurityAdvisoriesHandler(p *GitHubProvider) *ListSecurityAdvisoriesHandler {
	return &ListSecurityAdvisoriesHandler{provider: p}
}

func (h *ListSecurityAdvisoriesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_security_advisories",
		Description: "List security advisories for a repository",
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
				"state": map[string]interface{}{
					"type":        "string",
					"description": "Filter by state (published, closed, withdrawn, draft, triage)",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"description": "Filter by severity (critical, high, medium, low)",
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

func (h *ListSecurityAdvisoriesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	owner := extractString(params, "owner")
	repo := extractString(params, "repo")
	
	pagination := ExtractPagination(params)
	opts := &github.ListRepositorySecurityAdvisoriesOptions{
		ListCursorOptions: github.ListCursorOptions{
			PerPage: pagination.PerPage,
		},
	}

	if state := extractString(params, "state"); state != "" {
		opts.State = state
	}
	
	// Note: The API doesn't support severity filtering directly,
	// but we keep the parameter for potential client-side filtering
	severityFilter := extractString(params, "severity")

	advisories, _, err := client.SecurityAdvisories.ListRepositorySecurityAdvisories(ctx, owner, repo, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list security advisories: %v", err)), nil
	}

	// Apply client-side severity filtering if requested
	if severityFilter != "" {
		var filtered []*github.SecurityAdvisory
		for _, advisory := range advisories {
			if advisory.Severity != nil && *advisory.Severity == severityFilter {
				filtered = append(filtered, advisory)
			}
		}
		advisories = filtered
	}

	data, _ := json.Marshal(advisories)
	return NewToolResult(string(data)), nil
}

// ListGlobalSecurityAdvisoriesHandler handles listing global security advisories
type ListGlobalSecurityAdvisoriesHandler struct {
	provider *GitHubProvider
}

func NewListGlobalSecurityAdvisoriesHandler(p *GitHubProvider) *ListGlobalSecurityAdvisoriesHandler {
	return &ListGlobalSecurityAdvisoriesHandler{provider: p}
}

func (h *ListGlobalSecurityAdvisoriesHandler) GetDefinition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_global_security_advisories",
		Description: "List global security advisories from GitHub Advisory Database",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"ghsa_id": map[string]interface{}{
					"type":        "string",
					"description": "Filter by GHSA ID",
				},
				"cve_id": map[string]interface{}{
					"type":        "string",
					"description": "Filter by CVE ID",
				},
				"ecosystem": map[string]interface{}{
					"type":        "string",
					"description": "Filter by ecosystem (npm, pip, maven, nuget, composer, go, rust, erlang)",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"description": "Filter by severity (critical, high, medium, low)",
				},
				"cwes": map[string]interface{}{
					"type":        "array",
					"description": "Filter by CWE IDs",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"is_withdrawn": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to include withdrawn advisories",
				},
				"affects": map[string]interface{}{
					"type":        "array",
					"description": "Filter by packages affected",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"published": map[string]interface{}{
					"type":        "string",
					"description": "Filter by published date (YYYY-MM-DD)",
				},
				"updated": map[string]interface{}{
					"type":        "string",
					"description": "Filter by updated date (YYYY-MM-DD)",
				},
				"per_page": map[string]interface{}{
					"type":        "integer",
					"description": "Results per page (max 100)",
				},
			},
		},
	}
}

func (h *ListGlobalSecurityAdvisoriesHandler) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	client, ok := ctx.Value("github_client").(*github.Client)
	if !ok {
		return NewToolError("GitHub client not found in context"), nil
	}

	opts := &github.ListGlobalSecurityAdvisoriesOptions{}

	if ghsaID := extractString(params, "ghsa_id"); ghsaID != "" {
		opts.GHSAID = &ghsaID
	}
	if cveID := extractString(params, "cve_id"); cveID != "" {
		opts.CVEID = &cveID
	}
	if ecosystem := extractString(params, "ecosystem"); ecosystem != "" {
		opts.Ecosystem = &ecosystem
	}
	if severity := extractString(params, "severity"); severity != "" {
		opts.Severity = &severity
	}
	if isWithdrawn, ok := params["is_withdrawn"].(bool); ok {
		opts.IsWithdrawn = &isWithdrawn
	}

	// Handle pagination
	if perPage := extractInt(params, "per_page"); perPage > 0 {
		opts.PerPage = perPage
	}

	advisories, _, err := client.SecurityAdvisories.ListGlobalSecurityAdvisories(ctx, opts)
	if err != nil {
		return NewToolError(fmt.Sprintf("Failed to list global security advisories: %v", err)), nil
	}

	data, _ := json.Marshal(advisories)
	return NewToolResult(string(data)), nil
}
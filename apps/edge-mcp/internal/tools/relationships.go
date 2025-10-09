package tools

import (
	"fmt"
	"strings"
)

// ToolRelationship defines relationships between tools
type ToolRelationship struct {
	// Prerequisites are tools that must be executed before this tool
	Prerequisites []string `json:"prerequisites,omitempty"`

	// CommonlyUsedWith are tools frequently used together with this tool
	CommonlyUsedWith []string `json:"commonly_used_with,omitempty"`

	// NextSteps are recommended follow-up tools after this tool
	NextSteps []string `json:"next_steps,omitempty"`

	// Alternatives are tools that can be used instead of this tool
	Alternatives []string `json:"alternatives,omitempty"`

	// ConflictsWith are tools that should not be used with this tool
	ConflictsWith []string `json:"conflicts_with,omitempty"`
}

// IOCompatibility defines input/output type compatibility between tools
type IOCompatibility struct {
	InputType  DataType `json:"input_type"`
	OutputType DataType `json:"output_type"`
}

// DataType represents the type of data a tool accepts or produces
type DataType struct {
	Format      string                 `json:"format"`               // e.g., "json", "text", "binary"
	Schema      string                 `json:"schema"`               // e.g., "issue", "pull_request", "workflow"
	ContentType string                 `json:"content_type"`         // MIME type
	Properties  map[string]interface{} `json:"properties,omitempty"` // Additional schema properties
}

// WorkflowTemplate represents a suggested workflow of tools
type WorkflowTemplate struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Steps       []WorkflowStep `json:"steps"`
	Tags        []string       `json:"tags"`
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	Order       int                    `json:"order"`
	ToolName    string                 `json:"tool_name"`
	Description string                 `json:"description"`
	Required    bool                   `json:"required"`
	Condition   string                 `json:"condition,omitempty"`  // Condition for executing this step
	InputFrom   string                 `json:"input_from,omitempty"` // Previous step to get input from
	Parameters  map[string]interface{} `json:"parameters,omitempty"` // Default parameters
}

// RelationshipManager manages tool relationships and workflows
type RelationshipManager struct {
	relationships   map[string]*ToolRelationship
	ioCompatibility map[string]*IOCompatibility
	workflows       []WorkflowTemplate
}

// NewRelationshipManager creates a new relationship manager
func NewRelationshipManager() *RelationshipManager {
	rm := &RelationshipManager{
		relationships:   make(map[string]*ToolRelationship),
		ioCompatibility: make(map[string]*IOCompatibility),
		workflows:       make([]WorkflowTemplate, 0),
	}

	// Initialize with default relationships
	rm.initializeDefaultRelationships()
	rm.initializeDefaultWorkflows()
	rm.initializeIOCompatibility()

	return rm
}

// initializeDefaultRelationships sets up common tool relationships
func (rm *RelationshipManager) initializeDefaultRelationships() {
	// GitHub Issue relationships
	rm.relationships["github_get_issue"] = &ToolRelationship{
		Prerequisites:    []string{},
		CommonlyUsedWith: []string{"github_update_issue", "github_add_issue_comment", "github_list_issue_comments"},
		NextSteps:        []string{"github_update_issue", "github_add_issue_comment", "github_create_pull_request"},
		Alternatives:     []string{"github_search_issues"},
	}

	rm.relationships["github_create_issue"] = &ToolRelationship{
		Prerequisites:    []string{"github_get_repository"},
		CommonlyUsedWith: []string{"github_add_issue_comment", "github_update_issue"},
		NextSteps:        []string{"github_add_issue_comment", "github_assign_issue", "github_add_labels"},
		Alternatives:     []string{},
	}

	rm.relationships["github_list_issues"] = &ToolRelationship{
		Prerequisites:    []string{},
		CommonlyUsedWith: []string{"github_get_issue", "github_search_issues"},
		NextSteps:        []string{"github_get_issue", "github_update_issue", "github_create_issue"},
		Alternatives:     []string{"github_search_issues"},
	}

	// GitHub Pull Request relationships
	rm.relationships["github_create_pull_request"] = &ToolRelationship{
		Prerequisites:    []string{"github_create_branch", "github_push_files"},
		CommonlyUsedWith: []string{"github_add_pull_request_review_comment", "github_request_reviewers"},
		NextSteps:        []string{"github_request_reviewers", "github_add_pull_request_review_comment", "github_merge_pull_request"},
		Alternatives:     []string{},
	}

	rm.relationships["github_merge_pull_request"] = &ToolRelationship{
		Prerequisites:    []string{"github_get_pull_request", "github_get_pull_request_reviews"},
		CommonlyUsedWith: []string{"github_delete_branch"},
		NextSteps:        []string{"github_delete_branch", "github_create_release"},
		Alternatives:     []string{},
		ConflictsWith:    []string{"github_update_pull_request"}, // Can't update after merge
	}

	// GitHub Actions/Workflow relationships
	rm.relationships["github_run_workflow"] = &ToolRelationship{
		Prerequisites:    []string{"github_list_workflows"},
		CommonlyUsedWith: []string{"github_get_workflow_run", "github_list_workflow_jobs"},
		NextSteps:        []string{"github_get_workflow_run", "github_get_workflow_run_logs", "github_list_artifacts"},
		Alternatives:     []string{"github_rerun_workflow_run"},
	}

	rm.relationships["github_cancel_workflow_run"] = &ToolRelationship{
		Prerequisites:    []string{"github_get_workflow_run"},
		CommonlyUsedWith: []string{"github_rerun_workflow_run"},
		NextSteps:        []string{"github_rerun_workflow_run", "github_run_workflow"},
		Alternatives:     []string{},
		ConflictsWith:    []string{"github_get_workflow_run_logs"}, // Can't get logs while canceling
	}

	// Agent management relationships
	rm.relationships["agent_heartbeat"] = &ToolRelationship{
		Prerequisites:    []string{},
		CommonlyUsedWith: []string{"agent_status", "agent_list"},
		NextSteps:        []string{"task_assign", "task_get"},
		Alternatives:     []string{},
	}

	rm.relationships["task_assign"] = &ToolRelationship{
		Prerequisites:    []string{"task_create", "agent_list"},
		CommonlyUsedWith: []string{"task_get", "agent_status"},
		NextSteps:        []string{"task_get", "task_complete"},
		Alternatives:     []string{},
	}

	rm.relationships["task_complete"] = &ToolRelationship{
		Prerequisites:    []string{"task_assign", "task_get"},
		CommonlyUsedWith: []string{"context_update"},
		NextSteps:        []string{"task_create", "workflow_execute"},
		Alternatives:     []string{},
		ConflictsWith:    []string{"task_assign"}, // Can't reassign completed task
	}

	// Workflow relationships
	rm.relationships["workflow_create"] = &ToolRelationship{
		Prerequisites:    []string{},
		CommonlyUsedWith: []string{"workflow_list", "template_get"},
		NextSteps:        []string{"workflow_execute", "workflow_get"},
		Alternatives:     []string{"template_instantiate"},
	}

	rm.relationships["workflow_execute"] = &ToolRelationship{
		Prerequisites:    []string{"workflow_create"},
		CommonlyUsedWith: []string{"workflow_execution_get", "context_update"},
		NextSteps:        []string{"workflow_execution_get", "task_list"},
		Alternatives:     []string{},
	}

	rm.relationships["workflow_cancel"] = &ToolRelationship{
		Prerequisites:    []string{"workflow_execution_get"},
		CommonlyUsedWith: []string{},
		NextSteps:        []string{"workflow_execute"},
		Alternatives:     []string{},
		ConflictsWith:    []string{"workflow_execution_get"}, // Can't get status while canceling
	}

	// Context management relationships
	rm.relationships["context_update"] = &ToolRelationship{
		Prerequisites:    []string{},
		CommonlyUsedWith: []string{"context_get", "context_append"},
		NextSteps:        []string{"workflow_execute", "task_create"},
		Alternatives:     []string{"context_append"},
	}

	rm.relationships["context_get"] = &ToolRelationship{
		Prerequisites:    []string{},
		CommonlyUsedWith: []string{"context_update", "context_append"},
		NextSteps:        []string{"context_update"},
		Alternatives:     []string{},
	}

	// Template relationships
	rm.relationships["template_instantiate"] = &ToolRelationship{
		Prerequisites:    []string{"template_list", "template_get"},
		CommonlyUsedWith: []string{"workflow_execute"},
		NextSteps:        []string{"workflow_execute", "workflow_get"},
		Alternatives:     []string{"workflow_create"},
	}
}

// initializeDefaultWorkflows sets up common workflow templates
func (rm *RelationshipManager) initializeDefaultWorkflows() {
	// Code Review Workflow
	rm.workflows = append(rm.workflows, WorkflowTemplate{
		Name:        "code_review_workflow",
		Description: "Complete code review process from PR creation to merge",
		Category:    string(CategoryPullRequests),
		Tags:        []string{"review", "collaboration", "quality"},
		Steps: []WorkflowStep{
			{Order: 1, ToolName: "github_create_branch", Description: "Create feature branch", Required: true},
			{Order: 2, ToolName: "github_push_files", Description: "Push code changes", Required: true, InputFrom: "step_1"},
			{Order: 3, ToolName: "github_create_pull_request", Description: "Create PR for review", Required: true, InputFrom: "step_2"},
			{Order: 4, ToolName: "github_request_reviewers", Description: "Request code reviewers", Required: false, InputFrom: "step_3"},
			{Order: 5, ToolName: "github_run_workflow", Description: "Run CI/CD checks", Required: true, InputFrom: "step_3"},
			{Order: 6, ToolName: "github_get_pull_request_reviews", Description: "Check review status", Required: true, InputFrom: "step_3"},
			{Order: 7, ToolName: "github_merge_pull_request", Description: "Merge approved PR", Required: true, InputFrom: "step_6", Condition: "all_reviews_approved"},
			{Order: 8, ToolName: "github_delete_branch", Description: "Clean up feature branch", Required: false, InputFrom: "step_7"},
		},
	})

	// Issue Resolution Workflow
	rm.workflows = append(rm.workflows, WorkflowTemplate{
		Name:        "issue_resolution_workflow",
		Description: "Complete issue resolution from creation to closure",
		Category:    string(CategoryIssues),
		Tags:        []string{"bug", "fix", "tracking"},
		Steps: []WorkflowStep{
			{Order: 1, ToolName: "github_create_issue", Description: "Create new issue", Required: true},
			{Order: 2, ToolName: "github_add_labels", Description: "Add appropriate labels", Required: false, InputFrom: "step_1"},
			{Order: 3, ToolName: "github_assign_issue", Description: "Assign to developer", Required: true, InputFrom: "step_1"},
			{Order: 4, ToolName: "github_create_branch", Description: "Create fix branch", Required: true, InputFrom: "step_1"},
			{Order: 5, ToolName: "github_push_files", Description: "Push fix", Required: true, InputFrom: "step_4"},
			{Order: 6, ToolName: "github_create_pull_request", Description: "Create PR with fix", Required: true, InputFrom: "step_5", Parameters: map[string]interface{}{"closes_issue": true}},
			{Order: 7, ToolName: "github_merge_pull_request", Description: "Merge fix", Required: true, InputFrom: "step_6"},
			{Order: 8, ToolName: "github_update_issue", Description: "Close issue", Required: true, InputFrom: "step_1", Parameters: map[string]interface{}{"state": "closed"}},
		},
	})

	// Deployment Workflow
	rm.workflows = append(rm.workflows, WorkflowTemplate{
		Name:        "deployment_workflow",
		Description: "Deploy application through CI/CD pipeline",
		Category:    string(CategoryCICD),
		Tags:        []string{"deployment", "release", "production"},
		Steps: []WorkflowStep{
			{Order: 1, ToolName: "github_get_latest_release", Description: "Get current release", Required: true},
			{Order: 2, ToolName: "github_create_release", Description: "Create new release", Required: true},
			{Order: 3, ToolName: "github_run_workflow", Description: "Trigger deployment workflow", Required: true, InputFrom: "step_2", Parameters: map[string]interface{}{"workflow_id": "deploy.yml"}},
			{Order: 4, ToolName: "github_get_workflow_run", Description: "Monitor deployment", Required: true, InputFrom: "step_3"},
			{Order: 5, ToolName: "github_list_workflow_jobs", Description: "Check deployment jobs", Required: false, InputFrom: "step_4"},
			{Order: 6, ToolName: "github_get_workflow_run_logs", Description: "Get deployment logs", Required: false, InputFrom: "step_4", Condition: "on_failure"},
		},
	})

	// Multi-Agent Task Workflow
	rm.workflows = append(rm.workflows, WorkflowTemplate{
		Name:        "multi_agent_task_workflow",
		Description: "Coordinate multiple agents for complex task",
		Category:    string(CategoryAgent),
		Tags:        []string{"orchestration", "multi-agent", "coordination"},
		Steps: []WorkflowStep{
			{Order: 1, ToolName: "context_update", Description: "Set task context", Required: true},
			{Order: 2, ToolName: "task_create", Description: "Create main task", Required: true, InputFrom: "step_1"},
			{Order: 3, ToolName: "agent_list", Description: "Get available agents", Required: true},
			{Order: 4, ToolName: "task_assign", Description: "Assign to best agent", Required: true, InputFrom: "step_2,step_3"},
			{Order: 5, ToolName: "task_get", Description: "Monitor task progress", Required: true, InputFrom: "step_4"},
			{Order: 6, ToolName: "task_create", Description: "Create follow-up tasks", Required: false, InputFrom: "step_5", Condition: "needs_subtasks"},
			{Order: 7, ToolName: "task_complete", Description: "Complete main task", Required: true, InputFrom: "step_5"},
			{Order: 8, ToolName: "context_update", Description: "Update context with results", Required: true, InputFrom: "step_7"},
		},
	})
}

// initializeIOCompatibility sets up input/output type compatibility
func (rm *RelationshipManager) initializeIOCompatibility() {
	// Issue-related tools
	rm.ioCompatibility["github_get_issue"] = &IOCompatibility{
		InputType: DataType{
			Format: "json",
			Schema: "issue_identifier",
			Properties: map[string]interface{}{
				"issue_number": "integer",
				"owner":        "string",
				"repo":         "string",
			},
		},
		OutputType: DataType{
			Format:      "json",
			Schema:      "issue",
			ContentType: "application/json",
			Properties: map[string]interface{}{
				"id":     "integer",
				"number": "integer",
				"title":  "string",
				"body":   "string",
				"state":  "string",
			},
		},
	}

	rm.ioCompatibility["github_create_issue"] = &IOCompatibility{
		InputType: DataType{
			Format: "json",
			Schema: "issue_create",
			Properties: map[string]interface{}{
				"title":     "string",
				"body":      "string",
				"labels":    "array",
				"assignees": "array",
			},
		},
		OutputType: DataType{
			Format:      "json",
			Schema:      "issue",
			ContentType: "application/json",
		},
	}

	rm.ioCompatibility["github_list_issues"] = &IOCompatibility{
		InputType: DataType{
			Format: "json",
			Schema: "list_params",
			Properties: map[string]interface{}{
				"state":     "string",
				"labels":    "string",
				"sort":      "string",
				"direction": "string",
				"per_page":  "integer",
			},
		},
		OutputType: DataType{
			Format:      "json",
			Schema:      "issue_list",
			ContentType: "application/json",
			Properties: map[string]interface{}{
				"issues": "array[issue]",
				"total":  "integer",
			},
		},
	}

	// Pull Request tools
	rm.ioCompatibility["github_create_pull_request"] = &IOCompatibility{
		InputType: DataType{
			Format: "json",
			Schema: "pull_request_create",
			Properties: map[string]interface{}{
				"title": "string",
				"body":  "string",
				"head":  "string",
				"base":  "string",
			},
		},
		OutputType: DataType{
			Format:      "json",
			Schema:      "pull_request",
			ContentType: "application/json",
		},
	}

	// Workflow tools
	rm.ioCompatibility["workflow_create"] = &IOCompatibility{
		InputType: DataType{
			Format: "json",
			Schema: "workflow_definition",
			Properties: map[string]interface{}{
				"name":        "string",
				"description": "string",
				"steps":       "array",
			},
		},
		OutputType: DataType{
			Format:      "json",
			Schema:      "workflow",
			ContentType: "application/json",
			Properties: map[string]interface{}{
				"id":         "string",
				"name":       "string",
				"created_at": "timestamp",
			},
		},
	}

	rm.ioCompatibility["workflow_execute"] = &IOCompatibility{
		InputType: DataType{
			Format: "json",
			Schema: "workflow_execution_params",
			Properties: map[string]interface{}{
				"workflow_id": "string",
				"input":       "object",
			},
		},
		OutputType: DataType{
			Format:      "json",
			Schema:      "workflow_execution",
			ContentType: "application/json",
			Properties: map[string]interface{}{
				"execution_id": "string",
				"status":       "string",
			},
		},
	}

	// Task tools
	rm.ioCompatibility["task_create"] = &IOCompatibility{
		InputType: DataType{
			Format: "json",
			Schema: "task_definition",
			Properties: map[string]interface{}{
				"title":       "string",
				"description": "string",
				"priority":    "string",
				"type":        "string",
			},
		},
		OutputType: DataType{
			Format:      "json",
			Schema:      "task",
			ContentType: "application/json",
			Properties: map[string]interface{}{
				"id":         "string",
				"status":     "string",
				"created_at": "timestamp",
			},
		},
	}

	// Context tools
	rm.ioCompatibility["context_update"] = &IOCompatibility{
		InputType: DataType{
			Format: "json",
			Schema: "context_data",
			Properties: map[string]interface{}{
				"session_id": "string",
				"context":    "object",
				"merge":      "boolean",
			},
		},
		OutputType: DataType{
			Format:      "json",
			Schema:      "context",
			ContentType: "application/json",
			Properties: map[string]interface{}{
				"version":    "integer",
				"updated_at": "timestamp",
			},
		},
	}
}

// GetRelationship returns the relationships for a tool
func (rm *RelationshipManager) GetRelationship(toolName string) (*ToolRelationship, bool) {
	rel, exists := rm.relationships[toolName]
	return rel, exists
}

// GetIOCompatibility returns the I/O compatibility for a tool
func (rm *RelationshipManager) GetIOCompatibility(toolName string) (*IOCompatibility, bool) {
	io, exists := rm.ioCompatibility[toolName]
	return io, exists
}

// GetWorkflowsForCategory returns workflows for a specific category
func (rm *RelationshipManager) GetWorkflowsForCategory(category string) []WorkflowTemplate {
	var workflows []WorkflowTemplate
	for _, wf := range rm.workflows {
		if wf.Category == category {
			workflows = append(workflows, wf)
		}
	}
	return workflows
}

// GetWorkflowsWithTool returns workflows that include a specific tool
func (rm *RelationshipManager) GetWorkflowsWithTool(toolName string) []WorkflowTemplate {
	var workflows []WorkflowTemplate
	for _, wf := range rm.workflows {
		for _, step := range wf.Steps {
			if step.ToolName == toolName {
				workflows = append(workflows, wf)
				break
			}
		}
	}
	return workflows
}

// CheckCompatibility checks if two tools are compatible based on I/O types
func (rm *RelationshipManager) CheckCompatibility(sourceTool, targetTool string) (bool, string) {
	sourceIO, sourceExists := rm.ioCompatibility[sourceTool]
	targetIO, targetExists := rm.ioCompatibility[targetTool]

	if !sourceExists || !targetExists {
		return false, "compatibility information not available"
	}

	// Check if output of source matches input of target
	if sourceIO.OutputType.Schema == targetIO.InputType.Schema {
		return true, "direct compatibility"
	}

	// Check for known transformations
	if canTransform(sourceIO.OutputType.Schema, targetIO.InputType.Schema) {
		return true, fmt.Sprintf("compatible with transformation from %s to %s",
			sourceIO.OutputType.Schema, targetIO.InputType.Schema)
	}

	// Check for partial compatibility (same format, but only for related schemas)
	if sourceIO.OutputType.Format == targetIO.InputType.Format {
		// Only consider format compatible if schemas are related
		if isRelatedSchema(sourceIO.OutputType.Schema, targetIO.InputType.Schema) {
			return true, "format compatible, may need transformation"
		}
	}

	return false, fmt.Sprintf("incompatible types: %s -> %s",
		sourceIO.OutputType.Schema, targetIO.InputType.Schema)
}

// isRelatedSchema checks if two schemas are conceptually related
func isRelatedSchema(schema1, schema2 string) bool {
	// Define schema families - schemas that are related to each other
	families := [][]string{
		{"issue", "issue_identifier", "issue_create", "issue_list"},
		{"pull_request", "pull_request_identifier", "pull_request_create"},
		{"workflow", "workflow_definition", "workflow_execution_params", "workflow_execution"},
		{"task", "task_definition", "task_identifier"},
		{"context", "context_data"},
		{"agent", "agent_status"},
	}

	// Check if both schemas belong to the same family
	for _, family := range families {
		contains1, contains2 := false, false
		for _, schema := range family {
			if schema == schema1 {
				contains1 = true
			}
			if schema == schema2 {
				contains2 = true
			}
		}
		if contains1 && contains2 {
			return true
		}
	}

	return false
}

// canTransform checks if we can transform from one schema to another
func canTransform(fromSchema, toSchema string) bool {
	// Define known transformations
	transformations := map[string][]string{
		"issue":              {"issue_identifier", "issue_create", "pull_request_create"},
		"pull_request":       {"issue_create", "pull_request_identifier"},
		"workflow":           {"workflow_execution_params"},
		"workflow_execution": {"task_definition"},
		"task":               {"context_data"},
		"issue_list":         {"issue_identifier", "issue"},
		"context":            {"task_definition", "workflow_execution_params"},
	}

	if allowed, exists := transformations[fromSchema]; exists {
		for _, schema := range allowed {
			if schema == toSchema {
				return true
			}
		}
	}

	return false
}

// ValidateDependencies validates that all prerequisite tools are available
func (rm *RelationshipManager) ValidateDependencies(toolName string, availableTools []string) (bool, []string) {
	rel, exists := rm.relationships[toolName]
	if !exists {
		return true, nil // No relationships defined, assume valid
	}

	// Create a set of available tools for faster lookup
	available := make(map[string]bool)
	for _, tool := range availableTools {
		available[tool] = true
	}

	// Check prerequisites
	var missingPrereqs []string
	for _, prereq := range rel.Prerequisites {
		if !available[prereq] {
			missingPrereqs = append(missingPrereqs, prereq)
		}
	}

	return len(missingPrereqs) == 0, missingPrereqs
}

// ValidateWorkflow validates that all tools in a workflow are available
func (rm *RelationshipManager) ValidateWorkflow(workflow WorkflowTemplate, availableTools []string) (bool, []string) {
	available := make(map[string]bool)
	for _, tool := range availableTools {
		available[tool] = true
	}

	var missingTools []string
	for _, step := range workflow.Steps {
		if !available[step.ToolName] {
			missingTools = append(missingTools, step.ToolName)
		}
	}

	return len(missingTools) == 0, missingTools
}

// SuggestNextTools suggests the next tools to use based on the current tool
func (rm *RelationshipManager) SuggestNextTools(currentTool string) []string {
	rel, exists := rm.relationships[currentTool]
	if !exists {
		return []string{}
	}

	// Combine next steps and commonly used tools, removing duplicates
	suggestions := make(map[string]bool)
	for _, tool := range rel.NextSteps {
		suggestions[tool] = true
	}
	for _, tool := range rel.CommonlyUsedWith {
		suggestions[tool] = true
	}

	// Convert to slice
	result := make([]string, 0, len(suggestions))
	for tool := range suggestions {
		result = append(result, tool)
	}

	return result
}

// GetAlternatives returns alternative tools that can be used instead
func (rm *RelationshipManager) GetAlternatives(toolName string) []string {
	rel, exists := rm.relationships[toolName]
	if !exists {
		return []string{}
	}
	return rel.Alternatives
}

// CheckConflicts checks if two tools conflict with each other
func (rm *RelationshipManager) CheckConflicts(tool1, tool2 string) bool {
	// Check tool1's conflicts
	if rel, exists := rm.relationships[tool1]; exists {
		for _, conflict := range rel.ConflictsWith {
			if conflict == tool2 {
				return true
			}
		}
	}

	// Check tool2's conflicts
	if rel, exists := rm.relationships[tool2]; exists {
		for _, conflict := range rel.ConflictsWith {
			if conflict == tool1 {
				return true
			}
		}
	}

	return false
}

// GetWorkflowByName returns a workflow template by name
func (rm *RelationshipManager) GetWorkflowByName(name string) (*WorkflowTemplate, bool) {
	for _, wf := range rm.workflows {
		if wf.Name == name {
			return &wf, true
		}
	}
	return nil, false
}

// GetAllWorkflows returns all workflow templates
func (rm *RelationshipManager) GetAllWorkflows() []WorkflowTemplate {
	return rm.workflows
}

// GetWorkflowsWithTag returns workflows that have a specific tag
func (rm *RelationshipManager) GetWorkflowsWithTag(tag string) []WorkflowTemplate {
	var workflows []WorkflowTemplate
	for _, wf := range rm.workflows {
		for _, t := range wf.Tags {
			if strings.EqualFold(t, tag) {
				workflows = append(workflows, wf)
				break
			}
		}
	}
	return workflows
}

package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRelationshipManager(t *testing.T) {
	rm := NewRelationshipManager()

	assert.NotNil(t, rm)
	assert.NotNil(t, rm.relationships)
	assert.NotNil(t, rm.ioCompatibility)
	assert.NotEmpty(t, rm.workflows)

	// Check that default relationships are initialized
	_, exists := rm.GetRelationship("github_get_issue")
	assert.True(t, exists, "Default relationships should be initialized")

	// Check that workflows are initialized
	workflows := rm.GetAllWorkflows()
	assert.Greater(t, len(workflows), 0, "Default workflows should be initialized")
}

func TestGetRelationship(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name        string
		toolName    string
		expectExist bool
		checkFields func(t *testing.T, rel *ToolRelationship)
	}{
		{
			name:        "github_create_issue relationships",
			toolName:    "github_create_issue",
			expectExist: true,
			checkFields: func(t *testing.T, rel *ToolRelationship) {
				assert.Contains(t, rel.Prerequisites, "github_get_repository")
				assert.Contains(t, rel.CommonlyUsedWith, "github_add_issue_comment")
				assert.Contains(t, rel.NextSteps, "github_add_issue_comment")
			},
		},
		{
			name:        "github_merge_pull_request relationships",
			toolName:    "github_merge_pull_request",
			expectExist: true,
			checkFields: func(t *testing.T, rel *ToolRelationship) {
				assert.Contains(t, rel.Prerequisites, "github_get_pull_request")
				assert.Contains(t, rel.NextSteps, "github_delete_branch")
				assert.Contains(t, rel.ConflictsWith, "github_update_pull_request")
			},
		},
		{
			name:        "non-existent tool",
			toolName:    "non_existent_tool",
			expectExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel, exists := rm.GetRelationship(tt.toolName)
			assert.Equal(t, tt.expectExist, exists)
			if tt.expectExist && tt.checkFields != nil {
				tt.checkFields(t, rel)
			}
		})
	}
}

func TestGetIOCompatibility(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name        string
		toolName    string
		expectExist bool
		checkIO     func(t *testing.T, io *IOCompatibility)
	}{
		{
			name:        "github_get_issue IO",
			toolName:    "github_get_issue",
			expectExist: true,
			checkIO: func(t *testing.T, io *IOCompatibility) {
				assert.Equal(t, "json", io.InputType.Format)
				assert.Equal(t, "issue_identifier", io.InputType.Schema)
				assert.Equal(t, "json", io.OutputType.Format)
				assert.Equal(t, "issue", io.OutputType.Schema)
			},
		},
		{
			name:        "workflow_create IO",
			toolName:    "workflow_create",
			expectExist: true,
			checkIO: func(t *testing.T, io *IOCompatibility) {
				assert.Equal(t, "workflow_definition", io.InputType.Schema)
				assert.Equal(t, "workflow", io.OutputType.Schema)
			},
		},
		{
			name:        "non-existent tool",
			toolName:    "non_existent_tool",
			expectExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			io, exists := rm.GetIOCompatibility(tt.toolName)
			assert.Equal(t, tt.expectExist, exists)
			if tt.expectExist && tt.checkIO != nil {
				tt.checkIO(t, io)
			}
		})
	}
}

func TestCheckCompatibility(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name         string
		sourceTool   string
		targetTool   string
		expectCompat bool
		expectReason string
	}{
		{
			name:         "transformation compatibility",
			sourceTool:   "github_create_issue",
			targetTool:   "github_get_issue",
			expectCompat: true,
			expectReason: "compatible with transformation",
		},
		{
			name:         "format compatible",
			sourceTool:   "github_list_issues",
			targetTool:   "github_get_issue",
			expectCompat: true,
			expectReason: "compatible with transformation",
		},
		{
			name:         "workflow transformation",
			sourceTool:   "workflow_create",
			targetTool:   "workflow_execute",
			expectCompat: true,
			expectReason: "compatible with transformation",
		},
		{
			name:         "incompatible tools",
			sourceTool:   "github_get_issue",
			targetTool:   "workflow_create",
			expectCompat: false,
		},
		{
			name:         "non-existent source tool",
			sourceTool:   "non_existent",
			targetTool:   "github_get_issue",
			expectCompat: false,
			expectReason: "compatibility information not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compat, reason := rm.CheckCompatibility(tt.sourceTool, tt.targetTool)
			assert.Equal(t, tt.expectCompat, compat)
			if tt.expectReason != "" {
				assert.Contains(t, reason, tt.expectReason)
			}
		})
	}
}

func TestValidateDependencies(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name           string
		toolName       string
		availableTools []string
		expectValid    bool
		expectMissing  []string
	}{
		{
			name:           "all prerequisites available",
			toolName:       "github_create_issue",
			availableTools: []string{"github_get_repository", "github_create_issue"},
			expectValid:    true,
			expectMissing:  nil,
		},
		{
			name:           "missing prerequisites",
			toolName:       "github_create_issue",
			availableTools: []string{"github_create_issue"},
			expectValid:    false,
			expectMissing:  []string{"github_get_repository"},
		},
		{
			name:           "tool with no prerequisites",
			toolName:       "github_get_issue",
			availableTools: []string{},
			expectValid:    true,
			expectMissing:  nil,
		},
		{
			name:           "non-existent tool",
			toolName:       "non_existent",
			availableTools: []string{},
			expectValid:    true, // No relationships defined, assume valid
			expectMissing:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, missing := rm.ValidateDependencies(tt.toolName, tt.availableTools)
			assert.Equal(t, tt.expectValid, valid)
			assert.Equal(t, tt.expectMissing, missing)
		})
	}
}

func TestValidateWorkflow(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name           string
		workflowName   string
		availableTools []string
		expectValid    bool
		checkMissing   func(t *testing.T, missing []string)
	}{
		{
			name:         "all workflow tools available",
			workflowName: "code_review_workflow",
			availableTools: []string{
				"github_create_branch", "github_push_files", "github_create_pull_request",
				"github_request_reviewers", "github_run_workflow", "github_get_pull_request_reviews",
				"github_merge_pull_request", "github_delete_branch",
			},
			expectValid: true,
		},
		{
			name:           "some workflow tools missing",
			workflowName:   "code_review_workflow",
			availableTools: []string{"github_create_branch", "github_push_files"},
			expectValid:    false,
			checkMissing: func(t *testing.T, missing []string) {
				assert.Contains(t, missing, "github_create_pull_request")
				assert.Contains(t, missing, "github_merge_pull_request")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow, exists := rm.GetWorkflowByName(tt.workflowName)
			require.True(t, exists)

			valid, missing := rm.ValidateWorkflow(*workflow, tt.availableTools)
			assert.Equal(t, tt.expectValid, valid)
			if tt.checkMissing != nil {
				tt.checkMissing(t, missing)
			}
		})
	}
}

func TestSuggestNextTools(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name             string
		currentTool      string
		expectedContains []string
	}{
		{
			name:             "suggest after github_create_issue",
			currentTool:      "github_create_issue",
			expectedContains: []string{"github_add_issue_comment", "github_update_issue"},
		},
		{
			name:             "suggest after github_merge_pull_request",
			currentTool:      "github_merge_pull_request",
			expectedContains: []string{"github_delete_branch", "github_create_release"},
		},
		{
			name:             "non-existent tool",
			currentTool:      "non_existent",
			expectedContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := rm.SuggestNextTools(tt.currentTool)
			for _, expected := range tt.expectedContains {
				assert.Contains(t, suggestions, expected)
			}
		})
	}
}

func TestCheckConflicts(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name           string
		tool1          string
		tool2          string
		expectConflict bool
	}{
		{
			name:           "conflicting tools",
			tool1:          "github_merge_pull_request",
			tool2:          "github_update_pull_request",
			expectConflict: true,
		},
		{
			name:           "reverse order conflicting tools",
			tool1:          "github_update_pull_request",
			tool2:          "github_merge_pull_request",
			expectConflict: true,
		},
		{
			name:           "non-conflicting tools",
			tool1:          "github_get_issue",
			tool2:          "github_update_issue",
			expectConflict: false,
		},
		{
			name:           "non-existent tools",
			tool1:          "non_existent1",
			tool2:          "non_existent2",
			expectConflict: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflict := rm.CheckConflicts(tt.tool1, tt.tool2)
			assert.Equal(t, tt.expectConflict, conflict)
		})
	}
}

func TestGetWorkflowsForCategory(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name          string
		category      string
		expectCount   int
		checkWorkflow func(t *testing.T, workflows []WorkflowTemplate)
	}{
		{
			name:        "pull requests category",
			category:    string(CategoryPullRequests),
			expectCount: 1,
			checkWorkflow: func(t *testing.T, workflows []WorkflowTemplate) {
				assert.Equal(t, "code_review_workflow", workflows[0].Name)
			},
		},
		{
			name:        "issues category",
			category:    string(CategoryIssues),
			expectCount: 1,
			checkWorkflow: func(t *testing.T, workflows []WorkflowTemplate) {
				assert.Equal(t, "issue_resolution_workflow", workflows[0].Name)
			},
		},
		{
			name:        "non-existent category",
			category:    "non_existent",
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflows := rm.GetWorkflowsForCategory(tt.category)
			assert.Len(t, workflows, tt.expectCount)
			if tt.checkWorkflow != nil && len(workflows) > 0 {
				tt.checkWorkflow(t, workflows)
			}
		})
	}
}

func TestGetWorkflowsWithTool(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name            string
		toolName        string
		minExpectCount  int
		expectWorkflows []string
	}{
		{
			name:            "github_create_pull_request",
			toolName:        "github_create_pull_request",
			minExpectCount:  2,
			expectWorkflows: []string{"code_review_workflow", "issue_resolution_workflow"},
		},
		{
			name:            "task_create",
			toolName:        "task_create",
			minExpectCount:  1,
			expectWorkflows: []string{"multi_agent_task_workflow"},
		},
		{
			name:           "non-existent tool",
			toolName:       "non_existent",
			minExpectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflows := rm.GetWorkflowsWithTool(tt.toolName)
			assert.GreaterOrEqual(t, len(workflows), tt.minExpectCount)

			workflowNames := make([]string, len(workflows))
			for i, wf := range workflows {
				workflowNames[i] = wf.Name
			}

			for _, expected := range tt.expectWorkflows {
				assert.Contains(t, workflowNames, expected)
			}
		})
	}
}

func TestGetWorkflowsWithTag(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name           string
		tag            string
		minExpectCount int
	}{
		{
			name:           "review tag",
			tag:            "review",
			minExpectCount: 1,
		},
		{
			name:           "deployment tag",
			tag:            "deployment",
			minExpectCount: 1,
		},
		{
			name:           "orchestration tag",
			tag:            "orchestration",
			minExpectCount: 1,
		},
		{
			name:           "non-existent tag",
			tag:            "non_existent",
			minExpectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflows := rm.GetWorkflowsWithTag(tt.tag)
			assert.GreaterOrEqual(t, len(workflows), tt.minExpectCount)
		})
	}
}

func TestGetAlternatives(t *testing.T) {
	rm := NewRelationshipManager()

	tests := []struct {
		name                 string
		toolName             string
		expectedAlternatives []string
	}{
		{
			name:                 "github_get_issue alternatives",
			toolName:             "github_get_issue",
			expectedAlternatives: []string{"github_search_issues"},
		},
		{
			name:                 "workflow_create alternatives",
			toolName:             "workflow_create",
			expectedAlternatives: []string{"template_instantiate"},
		},
		{
			name:                 "tool with no alternatives",
			toolName:             "github_create_issue",
			expectedAlternatives: []string{},
		},
		{
			name:                 "non-existent tool",
			toolName:             "non_existent",
			expectedAlternatives: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alternatives := rm.GetAlternatives(tt.toolName)
			assert.Equal(t, tt.expectedAlternatives, alternatives)
		})
	}
}

func TestCanTransform(t *testing.T) {
	tests := []struct {
		name       string
		fromSchema string
		toSchema   string
		expectCan  bool
	}{
		{
			name:       "issue to issue_identifier",
			fromSchema: "issue",
			toSchema:   "issue_identifier",
			expectCan:  true,
		},
		{
			name:       "workflow to workflow_execution_params",
			fromSchema: "workflow",
			toSchema:   "workflow_execution_params",
			expectCan:  true,
		},
		{
			name:       "context to task_definition",
			fromSchema: "context",
			toSchema:   "task_definition",
			expectCan:  true,
		},
		{
			name:       "incompatible transformation",
			fromSchema: "issue",
			toSchema:   "workflow_definition",
			expectCan:  false,
		},
		{
			name:       "unknown schema",
			fromSchema: "unknown",
			toSchema:   "issue",
			expectCan:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canTransform(tt.fromSchema, tt.toSchema)
			assert.Equal(t, tt.expectCan, result)
		})
	}
}

func TestRegistryWithRelationships(t *testing.T) {
	registry := NewRegistry()

	// Register some mock tools
	mockProvider := &mockRelationshipToolProvider{
		tools: []ToolDefinition{
			{Name: "github_get_issue", Description: "Get an issue"},
			{Name: "github_create_issue", Description: "Create an issue"},
			{Name: "github_update_issue", Description: "Update an issue"},
			{Name: "github_get_repository", Description: "Get repository"},
		},
	}
	registry.Register(mockProvider)

	t.Run("ValidateToolDependencies", func(t *testing.T) {
		// Tool with satisfied dependencies
		valid, missing := registry.ValidateToolDependencies("github_create_issue")
		assert.True(t, valid)
		assert.Empty(t, missing)

		// Tool with no dependencies
		valid, missing = registry.ValidateToolDependencies("github_get_issue")
		assert.True(t, valid)
		assert.Empty(t, missing)
	})

	t.Run("SuggestNextTools", func(t *testing.T) {
		suggestions := registry.SuggestNextTools("github_get_issue")
		assert.Contains(t, suggestions, "github_update_issue")
	})

	t.Run("GetAlternativeTools", func(t *testing.T) {
		// Note: github_search_issues is not registered, so it won't be in alternatives
		alternatives := registry.GetAlternativeTools("github_get_issue")
		assert.Empty(t, alternatives) // Because github_search_issues is not registered
	})

	t.Run("CheckToolConflicts", func(t *testing.T) {
		// These tools don't conflict
		conflict := registry.CheckToolConflicts("github_get_issue", "github_update_issue")
		assert.False(t, conflict)
	})

	t.Run("EnrichToolWithRelationships", func(t *testing.T) {
		tool := ToolDefinition{
			Name:        "github_create_issue",
			Description: "Create an issue",
		}

		registry.EnrichToolWithRelationships(&tool)

		assert.Contains(t, tool.Prerequisites, "github_get_repository")
		assert.Contains(t, tool.CommonlyUsedWith, "github_add_issue_comment")
		assert.NotNil(t, tool.IOCompatibility)
	})

	t.Run("ListAllEnriched", func(t *testing.T) {
		tools := registry.ListAllEnriched()
		assert.Greater(t, len(tools), 0)

		// Find github_create_issue and check it's enriched
		for _, tool := range tools {
			if tool.Name == "github_create_issue" {
				assert.NotEmpty(t, tool.Prerequisites)
				assert.NotEmpty(t, tool.CommonlyUsedWith)
				break
			}
		}
	})
}

// mockRelationshipToolProvider implements the provider interface for testing
type mockRelationshipToolProvider struct {
	tools []ToolDefinition
}

func (m *mockRelationshipToolProvider) GetDefinitions() []ToolDefinition {
	return m.tools
}

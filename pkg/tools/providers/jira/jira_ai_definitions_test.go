package jira

import (
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJiraAIDefinitions(t *testing.T) {
	provider := &JiraProvider{
		enabledToolsets: make(map[string]bool),
		toolsetRegistry: make(map[string]*Toolset),
	}

	// Set up test toolsets
	setupTestToolsets(provider)

	definitions := provider.GetAIOptimizedDefinitions()

	// Should have at least 5 definitions (4 toolsets + help)
	assert.GreaterOrEqual(t, len(definitions), 5, "Should have at least 5 AI definitions")

	// Verify all definitions have required fields
	for _, def := range definitions {
		assert.NotEmpty(t, def.Name, "Definition name should not be empty")
		assert.NotEmpty(t, def.DisplayName, "Definition display name should not be empty")
		assert.NotEmpty(t, def.Category, "Definition category should not be empty")
		assert.NotEmpty(t, def.Description, "Definition description should not be empty")
		assert.NotEmpty(t, def.DetailedHelp, "Definition detailed help should not be empty")
		assert.NotEmpty(t, def.SemanticTags, "Definition semantic tags should not be empty")
		assert.NotEmpty(t, def.CommonPhrases, "Definition common phrases should not be empty")
		assert.NotEmpty(t, def.UsageExamples, "Definition usage examples should not be empty")

		// Verify usage examples structure
		for _, example := range def.UsageExamples {
			assert.NotEmpty(t, example.Scenario, "Example scenario should not be empty")
			assert.NotEmpty(t, example.Input, "Example input should not be empty")
			assert.NotEmpty(t, example.Explanation, "Example explanation should not be empty")
		}
	}
}

func TestJiraIssuesAIDefinition(t *testing.T) {
	provider := &JiraProvider{
		enabledToolsets: make(map[string]bool),
		toolsetRegistry: make(map[string]*Toolset),
	}

	// Enable only issues toolset
	provider.toolsetRegistry["issues"] = &Toolset{Name: "issues", Enabled: true}
	provider.enabledToolsets["issues"] = true

	definitions := provider.GetAIOptimizedDefinitions()

	// Find issues definition
	var issuesDefinition *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "jira_issues" {
			issuesDefinition = &def
			break
		}
	}

	require.NotNil(t, issuesDefinition, "Should find jira_issues definition")

	// Verify specific fields for issues
	assert.Equal(t, "jira_issues", issuesDefinition.Name)
	assert.Equal(t, "Jira Issue Management", issuesDefinition.DisplayName)
	assert.Equal(t, "Issue Tracking", issuesDefinition.Category)
	assert.Equal(t, "Issue Operations", issuesDefinition.Subcategory)
	assert.Contains(t, issuesDefinition.Description, "Create, read, update, and delete Jira issues")

	// Verify semantic tags include key terms
	assert.Contains(t, issuesDefinition.SemanticTags, "issue")
	assert.Contains(t, issuesDefinition.SemanticTags, "bug")
	assert.Contains(t, issuesDefinition.SemanticTags, "story")
	assert.Contains(t, issuesDefinition.SemanticTags, "task")
	assert.Contains(t, issuesDefinition.SemanticTags, "epic")
	assert.Contains(t, issuesDefinition.SemanticTags, "jira")

	// Verify common phrases
	assert.Contains(t, issuesDefinition.CommonPhrases, "create ticket")
	assert.Contains(t, issuesDefinition.CommonPhrases, "update issue")
	assert.Contains(t, issuesDefinition.CommonPhrases, "issue tracking")

	// Verify usage examples
	assert.GreaterOrEqual(t, len(issuesDefinition.UsageExamples), 3, "Should have at least 3 usage examples")

	// Verify first example (create bug)
	createExample := issuesDefinition.UsageExamples[0]
	assert.Equal(t, "Create a new bug report", createExample.Scenario)
	assert.Contains(t, createExample.Explanation, "Creates a high-priority bug")

	// Verify input structure
	input := createExample.Input
	assert.Equal(t, "create_issue", input["action"])

	fields, ok := input["fields"].(map[string]interface{})
	require.True(t, ok, "Fields should be a map")

	project, ok := fields["project"].(map[string]interface{})
	require.True(t, ok, "Project should be a map")
	assert.Equal(t, "PROJ", project["key"])

	// Verify capabilities
	require.NotNil(t, issuesDefinition.Capabilities, "Should have capabilities")
	assert.GreaterOrEqual(t, len(issuesDefinition.Capabilities.Capabilities), 4, "Should have at least 4 capabilities")

	// Check for specific capabilities
	capabilities := issuesDefinition.Capabilities.Capabilities
	hasCreate := false
	hasRead := false
	hasUpdate := false
	hasDelete := false

	for _, cap := range capabilities {
		if cap.Action == "create" && cap.Resource == "issues" {
			hasCreate = true
		}
		if cap.Action == "read" && cap.Resource == "issues" {
			hasRead = true
		}
		if cap.Action == "update" && cap.Resource == "issues" {
			hasUpdate = true
		}
		if cap.Action == "delete" && cap.Resource == "issues" {
			hasDelete = true
		}
	}

	assert.True(t, hasCreate, "Should have create capability")
	assert.True(t, hasRead, "Should have read capability")
	assert.True(t, hasUpdate, "Should have update capability")
	assert.True(t, hasDelete, "Should have delete capability")

	// Verify rate limits
	require.NotNil(t, issuesDefinition.Capabilities.RateLimits, "Should have rate limits")
	assert.Equal(t, 60, issuesDefinition.Capabilities.RateLimits.RequestsPerMinute)
	assert.Contains(t, issuesDefinition.Capabilities.RateLimits.Description, "Jira Cloud rate limits")
}

func TestJiraSearchAIDefinition(t *testing.T) {
	provider := &JiraProvider{
		enabledToolsets: make(map[string]bool),
		toolsetRegistry: make(map[string]*Toolset),
	}

	// Enable only search toolset
	provider.toolsetRegistry["search"] = &Toolset{Name: "search", Enabled: true}
	provider.enabledToolsets["search"] = true

	definitions := provider.GetAIOptimizedDefinitions()

	// Find search definition
	var searchDefinition *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "jira_search" {
			searchDefinition = &def
			break
		}
	}

	require.NotNil(t, searchDefinition, "Should find jira_search definition")

	// Verify specific fields for search
	assert.Equal(t, "jira_search", searchDefinition.Name)
	assert.Equal(t, "Jira Search & JQL", searchDefinition.DisplayName)
	assert.Equal(t, "Search", searchDefinition.Category)
	assert.Equal(t, "Query Operations", searchDefinition.Subcategory)
	assert.Contains(t, searchDefinition.Description, "JQL (Jira Query Language)")

	// Verify JQL-specific semantic tags
	assert.Contains(t, searchDefinition.SemanticTags, "jql")
	assert.Contains(t, searchDefinition.SemanticTags, "query")
	assert.Contains(t, searchDefinition.SemanticTags, "search")
	assert.Contains(t, searchDefinition.SemanticTags, "jira-query-language")

	// Verify JQL common phrases
	assert.Contains(t, searchDefinition.CommonPhrases, "jql query")
	assert.Contains(t, searchDefinition.CommonPhrases, "advanced search")
	assert.Contains(t, searchDefinition.CommonPhrases, "search issues")

	// Verify usage examples include JQL examples
	assert.GreaterOrEqual(t, len(searchDefinition.UsageExamples), 3, "Should have at least 3 usage examples")

	// Find JQL example
	foundJQLExample := false
	for _, example := range searchDefinition.UsageExamples {
		input := example.Input
		if jql, exists := input["jql"]; exists {
			jqlStr, ok := jql.(string)
			if ok && len(jqlStr) > 0 {
				foundJQLExample = true
				// Verify JQL contains typical patterns
				assert.Contains(t, jqlStr, "AND", "JQL should contain AND operator")
				break
			}
		}
	}
	assert.True(t, foundJQLExample, "Should have at least one example with JQL query")

	// Verify capabilities
	require.NotNil(t, searchDefinition.Capabilities, "Should have capabilities")
	capabilities := searchDefinition.Capabilities.Capabilities

	hasSearch := false
	for _, cap := range capabilities {
		if cap.Action == "search" && cap.Resource == "issues" {
			hasSearch = true
			break
		}
	}
	assert.True(t, hasSearch, "Should have search capability")
}

func TestJiraCommentsAIDefinition(t *testing.T) {
	provider := &JiraProvider{
		enabledToolsets: make(map[string]bool),
		toolsetRegistry: make(map[string]*Toolset),
	}

	// Enable only comments toolset
	provider.toolsetRegistry["comments"] = &Toolset{Name: "comments", Enabled: true}
	provider.enabledToolsets["comments"] = true

	definitions := provider.GetAIOptimizedDefinitions()

	// Find comments definition
	var commentsDefinition *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "jira_comments" {
			commentsDefinition = &def
			break
		}
	}

	require.NotNil(t, commentsDefinition, "Should find jira_comments definition")

	// Verify specific fields for comments
	assert.Equal(t, "jira_comments", commentsDefinition.Name)
	assert.Equal(t, "Jira Comments", commentsDefinition.DisplayName)
	assert.Equal(t, "Collaboration", commentsDefinition.Category)
	assert.Equal(t, "Issue Comments", commentsDefinition.Subcategory)
	assert.Contains(t, commentsDefinition.Description, "Add, retrieve, update, and delete comments")

	// Verify comment-specific semantic tags
	assert.Contains(t, commentsDefinition.SemanticTags, "comment")
	assert.Contains(t, commentsDefinition.SemanticTags, "discussion")
	assert.Contains(t, commentsDefinition.SemanticTags, "collaboration")

	// Verify comment common phrases
	assert.Contains(t, commentsDefinition.CommonPhrases, "add comment")
	assert.Contains(t, commentsDefinition.CommonPhrases, "update comment")
	assert.Contains(t, commentsDefinition.CommonPhrases, "team discussion")

	// Verify usage examples
	assert.GreaterOrEqual(t, len(commentsDefinition.UsageExamples), 3, "Should have at least 3 usage examples")

	// Verify capabilities include CRUD operations
	require.NotNil(t, commentsDefinition.Capabilities, "Should have capabilities")
	capabilities := commentsDefinition.Capabilities.Capabilities

	hasCreate := false
	hasRead := false
	hasUpdate := false
	hasDelete := false

	for _, cap := range capabilities {
		if cap.Resource == "comments" {
			switch cap.Action {
			case "create":
				hasCreate = true
			case "read":
				hasRead = true
			case "update":
				hasUpdate = true
			case "delete":
				hasDelete = true
			}
		}
	}

	assert.True(t, hasCreate, "Should have create capability for comments")
	assert.True(t, hasRead, "Should have read capability for comments")
	assert.True(t, hasUpdate, "Should have update capability for comments")
	assert.True(t, hasDelete, "Should have delete capability for comments")
}

func TestJiraWorkflowAIDefinition(t *testing.T) {
	provider := &JiraProvider{
		enabledToolsets: make(map[string]bool),
		toolsetRegistry: make(map[string]*Toolset),
	}

	// Enable only workflow toolset
	provider.toolsetRegistry["workflow"] = &Toolset{Name: "workflow", Enabled: true}
	provider.enabledToolsets["workflow"] = true

	definitions := provider.GetAIOptimizedDefinitions()

	// Find workflow definition
	var workflowDefinition *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "jira_workflow" {
			workflowDefinition = &def
			break
		}
	}

	require.NotNil(t, workflowDefinition, "Should find jira_workflow definition")

	// Verify specific fields for workflow
	assert.Equal(t, "jira_workflow", workflowDefinition.Name)
	assert.Equal(t, "Jira Workflow & Transitions", workflowDefinition.DisplayName)
	assert.Equal(t, "Workflow", workflowDefinition.Category)
	assert.Equal(t, "Issue Transitions", workflowDefinition.Subcategory)
	assert.Contains(t, workflowDefinition.Description, "Manage issue workflows, execute transitions")

	// Verify workflow-specific semantic tags
	assert.Contains(t, workflowDefinition.SemanticTags, "workflow")
	assert.Contains(t, workflowDefinition.SemanticTags, "transition")
	assert.Contains(t, workflowDefinition.SemanticTags, "status")
	assert.Contains(t, workflowDefinition.SemanticTags, "state")

	// Verify workflow common phrases
	assert.Contains(t, workflowDefinition.CommonPhrases, "transition issue")
	assert.Contains(t, workflowDefinition.CommonPhrases, "change status")
	assert.Contains(t, workflowDefinition.CommonPhrases, "workflow transition")

	// Verify usage examples
	assert.GreaterOrEqual(t, len(workflowDefinition.UsageExamples), 3, "Should have at least 3 usage examples")

	// Look for transition examples
	foundTransitionExample := false
	for _, example := range workflowDefinition.UsageExamples {
		input := example.Input
		if action, exists := input["action"]; exists {
			if actionStr, ok := action.(string); ok && actionStr == "transition_issue" {
				foundTransitionExample = true
				break
			}
		}
	}
	assert.True(t, foundTransitionExample, "Should have at least one transition example")

	// Verify capabilities
	require.NotNil(t, workflowDefinition.Capabilities, "Should have capabilities")
	capabilities := workflowDefinition.Capabilities.Capabilities

	hasTransitionRead := false
	hasTransitionExecute := false
	hasWorkflowRead := false

	for _, cap := range capabilities {
		if cap.Action == "read" && cap.Resource == "transitions" {
			hasTransitionRead = true
		}
		if cap.Action == "execute" && cap.Resource == "transitions" {
			hasTransitionExecute = true
		}
		if cap.Action == "read" && cap.Resource == "workflows" {
			hasWorkflowRead = true
		}
	}

	assert.True(t, hasTransitionRead, "Should have read capability for transitions")
	assert.True(t, hasTransitionExecute, "Should have execute capability for transitions")
	assert.True(t, hasWorkflowRead, "Should have read capability for workflows")
}

func TestJiraHelpAIDefinition(t *testing.T) {
	provider := &JiraProvider{
		enabledToolsets: make(map[string]bool),
		toolsetRegistry: make(map[string]*Toolset),
	}

	// Help is always included regardless of toolsets
	definitions := provider.GetAIOptimizedDefinitions()

	// Find help definition
	var helpDefinition *providers.AIOptimizedToolDefinition
	for _, def := range definitions {
		if def.Name == "jira_help" {
			helpDefinition = &def
			break
		}
	}

	require.NotNil(t, helpDefinition, "Should find jira_help definition")

	// Verify specific fields for help
	assert.Equal(t, "jira_help", helpDefinition.Name)
	assert.Equal(t, "Jira Help & Best Practices", helpDefinition.DisplayName)
	assert.Equal(t, "Help", helpDefinition.Category)
	assert.Equal(t, "Usage Guidance", helpDefinition.Subcategory)
	assert.Contains(t, helpDefinition.Description, "troubleshooting, and best practices")

	// Verify help contains JQL examples
	assert.Contains(t, helpDefinition.DetailedHelp, "JQL Query Examples")
	assert.Contains(t, helpDefinition.DetailedHelp, "project = PROJ")
	assert.Contains(t, helpDefinition.DetailedHelp, "Common Field Names")
	assert.Contains(t, helpDefinition.DetailedHelp, "Error Handling")
	assert.Contains(t, helpDefinition.DetailedHelp, "Best Practices")

	// Verify help-specific semantic tags
	assert.Contains(t, helpDefinition.SemanticTags, "help")
	assert.Contains(t, helpDefinition.SemanticTags, "troubleshooting")
	assert.Contains(t, helpDefinition.SemanticTags, "best-practices")
	assert.Contains(t, helpDefinition.SemanticTags, "jql-help")

	// Verify help common phrases
	assert.Contains(t, helpDefinition.CommonPhrases, "jira help")
	assert.Contains(t, helpDefinition.CommonPhrases, "jql examples")
	assert.Contains(t, helpDefinition.CommonPhrases, "jira best practices")
}

func TestDisabledToolsetsExcluded(t *testing.T) {
	provider := &JiraProvider{
		enabledToolsets: make(map[string]bool),
		toolsetRegistry: make(map[string]*Toolset),
	}

	// Set up some toolsets but only enable issues
	provider.toolsetRegistry["issues"] = &Toolset{Name: "issues", Enabled: true}
	provider.toolsetRegistry["search"] = &Toolset{Name: "search", Enabled: false}
	provider.toolsetRegistry["comments"] = &Toolset{Name: "comments", Enabled: false}
	provider.toolsetRegistry["workflow"] = &Toolset{Name: "workflow", Enabled: false}

	provider.enabledToolsets["issues"] = true

	definitions := provider.GetAIOptimizedDefinitions()

	// Should have issues definition + help (help is always included)
	assert.Equal(t, 2, len(definitions), "Should have exactly 2 definitions (issues + help)")

	// Verify only issues definition is present (besides help)
	definitionNames := make([]string, 0)
	for _, def := range definitions {
		definitionNames = append(definitionNames, def.Name)
	}

	assert.Contains(t, definitionNames, "jira_issues", "Should contain issues definition")
	assert.Contains(t, definitionNames, "jira_help", "Should contain help definition")
	assert.NotContains(t, definitionNames, "jira_search", "Should not contain search definition")
	assert.NotContains(t, definitionNames, "jira_comments", "Should not contain comments definition")
	assert.NotContains(t, definitionNames, "jira_workflow", "Should not contain workflow definition")
}

func TestAllToolsetsEnabled(t *testing.T) {
	provider := &JiraProvider{
		enabledToolsets: make(map[string]bool),
		toolsetRegistry: make(map[string]*Toolset),
	}

	// Enable all toolsets
	setupTestToolsets(provider)

	definitions := provider.GetAIOptimizedDefinitions()

	// Should have all 4 toolset definitions + help = 5 total
	assert.Equal(t, 5, len(definitions), "Should have exactly 5 definitions when all toolsets enabled")

	definitionNames := make([]string, 0)
	for _, def := range definitions {
		definitionNames = append(definitionNames, def.Name)
	}

	assert.Contains(t, definitionNames, "jira_issues", "Should contain issues definition")
	assert.Contains(t, definitionNames, "jira_search", "Should contain search definition")
	assert.Contains(t, definitionNames, "jira_comments", "Should contain comments definition")
	assert.Contains(t, definitionNames, "jira_workflow", "Should contain workflow definition")
	assert.Contains(t, definitionNames, "jira_help", "Should contain help definition")
}

func TestUsageExampleStructure(t *testing.T) {
	provider := &JiraProvider{
		enabledToolsets: make(map[string]bool),
		toolsetRegistry: make(map[string]*Toolset),
	}

	setupTestToolsets(provider)
	definitions := provider.GetAIOptimizedDefinitions()

	// Verify all examples have proper structure
	for _, definition := range definitions {
		for i, example := range definition.UsageExamples {
			t.Run(definition.Name+"_example_"+string(rune(i)), func(t *testing.T) {
				assert.NotEmpty(t, example.Scenario, "Example scenario should not be empty")
				assert.NotEmpty(t, example.Explanation, "Example explanation should not be empty")
				assert.NotNil(t, example.Input, "Example input should not be nil")

				// Verify input is a valid map
				input := example.Input

				// Most examples should have an action field
				if definition.Name != "jira_help" {
					action, hasAction := input["action"]
					if hasAction {
						_, ok := action.(string)
						assert.True(t, ok, "Action should be a string")
					}
				}
			})
		}
	}
}

// Helper function to set up test toolsets
func setupTestToolsets(provider *JiraProvider) {
	provider.toolsetRegistry["issues"] = &Toolset{Name: "issues", Enabled: true}
	provider.toolsetRegistry["search"] = &Toolset{Name: "search", Enabled: true}
	provider.toolsetRegistry["comments"] = &Toolset{Name: "comments", Enabled: true}
	provider.toolsetRegistry["workflow"] = &Toolset{Name: "workflow", Enabled: true}

	provider.enabledToolsets["issues"] = true
	provider.enabledToolsets["search"] = true
	provider.enabledToolsets["comments"] = true
	provider.enabledToolsets["workflow"] = true
}

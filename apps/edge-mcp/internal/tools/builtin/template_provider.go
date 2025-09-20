package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
)

// TemplateProvider provides workflow template tools
type TemplateProvider struct{}

// NewTemplateProvider creates a new template provider
func NewTemplateProvider() *TemplateProvider {
	return &TemplateProvider{}
}

// GetDefinitions returns the tool definitions for workflow templates
func (p *TemplateProvider) GetDefinitions() []tools.ToolDefinition {
	return []tools.ToolDefinition{
		{
			Name:        "template_list",
			Description: "List all available workflow templates with their categories and descriptions",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Filter templates by category",
						"enum":        []string{"deployment", "task-management", "monitoring", "security"},
					},
				},
			},
			Handler: p.handleList,
		},
		{
			Name:        "template_get",
			Description: "Get detailed information about a specific workflow template",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"template_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the template to retrieve",
					},
				},
				"required": []string{"template_id"},
			},
			Handler: p.handleGet,
		},
		{
			Name:        "template_instantiate",
			Description: "Create a workflow from a template with variable substitution",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"template_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the template to instantiate",
					},
					"variables": map[string]interface{}{
						"type":        "object",
						"description": "Variables to substitute in the template",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the instantiated workflow",
					},
				},
				"required": []string{"template_id", "name"},
			},
			Handler: p.handleInstantiate,
		},
	}
}

// handleList returns available workflow templates
func (p *TemplateProvider) handleList(ctx context.Context, args json.RawMessage) (interface{}, error) {
	rb := NewResponseBuilder()

	var params struct {
		Category string `json:"category,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return rb.Error(fmt.Errorf("invalid parameters: %w", err)), nil
	}

	templates := make([]map[string]interface{}, 0)
	for _, tmpl := range WorkflowTemplates {
		// Apply category filter if specified
		if params.Category != "" && tmpl.Category != params.Category {
			continue
		}

		templates = append(templates, map[string]interface{}{
			"id":                 tmpl.ID,
			"name":               tmpl.Name,
			"description":        tmpl.Description,
			"category":           tmpl.Category,
			"steps_count":        len(tmpl.Steps),
			"required_variables": tmpl.RequiredVariables,
			"optional_variables": tmpl.OptionalVariables,
		})
	}

	result := map[string]interface{}{
		"templates": templates,
		"count":     len(templates),
	}

	nextSteps := []string{"template_get", "template_instantiate"}
	return rb.Success(result, nextSteps...), nil
}

// handleGet returns details of a specific template
func (p *TemplateProvider) handleGet(ctx context.Context, args json.RawMessage) (interface{}, error) {
	rb := NewResponseBuilder()

	var params struct {
		TemplateID string `json:"template_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return rb.Error(fmt.Errorf("invalid parameters: %w", err)), nil
	}

	if params.TemplateID == "" {
		return rb.Error(fmt.Errorf("template_id is required")), nil
	}

	for _, tmpl := range WorkflowTemplates {
		if tmpl.ID == params.TemplateID {
			result := map[string]interface{}{
				"id":                 tmpl.ID,
				"name":               tmpl.Name,
				"description":        tmpl.Description,
				"category":           tmpl.Category,
				"steps":              tmpl.Steps,
				"required_variables": tmpl.RequiredVariables,
				"optional_variables": tmpl.OptionalVariables,
			}

			nextSteps := []string{"template_instantiate"}
			return rb.Success(result, nextSteps...), nil
		}
	}

	return rb.Error(fmt.Errorf("template not found: %s", params.TemplateID)), nil
}

// handleInstantiate creates a workflow from a template
func (p *TemplateProvider) handleInstantiate(ctx context.Context, args json.RawMessage) (interface{}, error) {
	rb := NewResponseBuilder()

	var params struct {
		TemplateID string                 `json:"template_id"`
		Variables  map[string]interface{} `json:"variables,omitempty"`
		Name       string                 `json:"name"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return rb.Error(fmt.Errorf("invalid parameters: %w", err)), nil
	}

	if params.TemplateID == "" {
		return rb.Error(fmt.Errorf("template_id is required")), nil
	}
	if params.Name == "" {
		return rb.Error(fmt.Errorf("name is required")), nil
	}

	// Find the template
	var template *WorkflowTemplate
	for _, tmpl := range WorkflowTemplates {
		if tmpl.ID == params.TemplateID {
			t := tmpl // Create a copy
			template = &t
			break
		}
	}

	if template == nil {
		return rb.Error(fmt.Errorf("template not found: %s", params.TemplateID)), nil
	}

	// Validate required variables
	missingVars := []string{}
	for _, reqVar := range template.RequiredVariables {
		if _, exists := params.Variables[reqVar.Name]; !exists {
			missingVars = append(missingVars, reqVar.Name)
		}
	}
	if len(missingVars) > 0 {
		return rb.Error(fmt.Errorf("missing required variables: %v", missingVars)), nil
	}

	// Apply defaults for optional variables
	finalVariables := make(map[string]interface{})
	for k, v := range params.Variables {
		finalVariables[k] = v
	}
	for _, optVar := range template.OptionalVariables {
		if _, exists := finalVariables[optVar.Name]; !exists && optVar.Default != nil {
			finalVariables[optVar.Name] = optVar.Default
		}
	}

	// Substitute variables in the template
	workflowSteps := make([]map[string]interface{}, 0, len(template.Steps))
	for _, step := range template.Steps {
		stepData := map[string]interface{}{
			"name":        step.Tool,
			"type":        "tool",
			"tool":        step.Tool,
			"description": step.Description,
			"input":       substituteVariables(step.Input, finalVariables),
		}
		if len(step.DependsOn) > 0 {
			stepData["depends_on"] = step.DependsOn
		}
		workflowSteps = append(workflowSteps, stepData)
	}

	result := map[string]interface{}{
		"workflow": map[string]interface{}{
			"name":        params.Name,
			"description": fmt.Sprintf("Instantiated from template: %s", template.Name),
			"steps":       workflowSteps,
			"template_id": params.TemplateID,
		},
		"message": fmt.Sprintf("Workflow '%s' created from template '%s'", params.Name, template.Name),
	}

	nextSteps := []string{"workflow_create", "workflow_execute"}
	return rb.Success(result, nextSteps...), nil
}

// substituteVariables replaces ${var} patterns with actual values
func substituteVariables(input map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range input {
		switch v := value.(type) {
		case string:
			// Check if it's a variable reference
			if len(v) > 3 && v[:2] == "${" && v[len(v)-1] == '}' {
				varName := v[2 : len(v)-1]
				if varValue, exists := variables[varName]; exists {
					result[key] = varValue
				} else {
					result[key] = v // Keep original if variable not provided
				}
			} else {
				result[key] = v
			}
		case map[string]interface{}:
			result[key] = substituteVariables(v, variables)
		default:
			result[key] = value
		}
	}

	return result
}

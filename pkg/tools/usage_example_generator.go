package tools

import (
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
)

// UsageExampleGenerator generates usage examples for AI agents
// Following 2025 best practices for LLM tool understanding
type UsageExampleGenerator struct {
	logger observability.Logger
}

// NewUsageExampleGenerator creates a new usage example generator
func NewUsageExampleGenerator(logger observability.Logger) *UsageExampleGenerator {
	return &UsageExampleGenerator{
		logger: logger,
	}
}

// UsageExample represents a complete usage example for AI understanding
type UsageExample struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Scenario    string                 `json:"scenario"`
	Operation   string                 `json:"operation"`
	Parameters  map[string]interface{} `json:"parameters"`
	Headers     map[string]string      `json:"headers,omitempty"`
	Request     interface{}            `json:"request,omitempty"`
	Response    interface{}            `json:"response,omitempty"`
	Explanation string                 `json:"explanation"`
	NextSteps   []string               `json:"next_steps,omitempty"`
}

// GenerateExamples generates usage examples for a tool
func (g *UsageExampleGenerator) GenerateExamples(spec *openapi3.T, toolName string) []UsageExample {
	examples := []UsageExample{}
	toolNameLower := strings.ToLower(toolName)

	// Generate tool-specific examples
	if strings.Contains(toolNameLower, "snyk") {
		examples = append(examples, g.generateSnykExamples()...)
	} else if strings.Contains(toolNameLower, "github") {
		examples = append(examples, g.generateGitHubExamples()...)
	} else {
		// Generate generic examples based on operations
		examples = append(examples, g.generateGenericExamples(spec)...)
	}

	return examples
}

// generateSnykExamples generates Snyk-specific examples
func (g *UsageExampleGenerator) generateSnykExamples() []UsageExample {
	return []UsageExample{
		{
			Title:       "Scan a Project for Vulnerabilities",
			Description: "Perform a security scan on a project to identify vulnerabilities",
			Scenario:    "You need to check if a project has any security vulnerabilities in its dependencies",
			Operation:   "test_project",
			Parameters: map[string]interface{}{
				"org_id":     "your-org-id",
				"project_id": "your-project-id",
			},
			Explanation: "This scans the project and returns a list of vulnerabilities with severity levels, affected packages, and remediation advice",
			NextSteps: []string{
				"Review critical and high severity vulnerabilities",
				"Apply suggested fixes using 'apply_remediation'",
				"Re-scan to verify fixes",
			},
		},
		{
			Title:       "Get Vulnerability Details",
			Description: "Retrieve detailed information about a specific vulnerability",
			Scenario:    "You found a critical vulnerability and need more information about it",
			Operation:   "get_issue",
			Parameters: map[string]interface{}{
				"issue_id": "SNYK-JS-LODASH-567746",
			},
			Explanation: "Returns comprehensive details including CVE, CVSS score, exploit maturity, affected versions, and remediation paths",
			NextSteps: []string{
				"Assess impact on your application",
				"Plan remediation strategy",
				"Check for available patches",
			},
		},
		{
			Title:       "Monitor Project Continuously",
			Description: "Set up continuous monitoring for new vulnerabilities",
			Scenario:    "You want to be notified when new vulnerabilities are discovered in your dependencies",
			Operation:   "create_monitor",
			Parameters: map[string]interface{}{
				"project_id": "your-project-id",
				"frequency":  "daily",
				"severity":   []string{"critical", "high"},
			},
			Explanation: "Creates a monitor that will check for new vulnerabilities based on your criteria and send notifications",
			NextSteps: []string{
				"Configure notification channels",
				"Set up automated workflows for critical issues",
				"Review monitoring reports regularly",
			},
		},
	}
}

// generateGitHubExamples generates GitHub-specific examples
func (g *UsageExampleGenerator) generateGitHubExamples() []UsageExample {
	return []UsageExample{
		{
			Title:       "Create a Pull Request",
			Description: "Create a new pull request to propose changes",
			Scenario:    "You've made changes in a feature branch and want to merge them",
			Operation:   "pulls_create",
			Parameters: map[string]interface{}{
				"owner": "repository-owner",
				"repo":  "repository-name",
				"title": "Add new feature",
				"head":  "feature-branch",
				"base":  "main",
				"body":  "This PR adds the new feature as discussed in issue #123",
			},
			Explanation: "Creates a pull request from your feature branch to the base branch, initiating the code review process",
			NextSteps: []string{
				"Request reviews from team members",
				"Address review comments",
				"Run CI checks",
				"Merge when approved",
			},
		},
		{
			Title:       "List Repository Issues",
			Description: "Get all open issues for a repository",
			Scenario:    "You want to see what issues need attention in your project",
			Operation:   "issues_list",
			Parameters: map[string]interface{}{
				"owner":  "repository-owner",
				"repo":   "repository-name",
				"state":  "open",
				"labels": "bug",
				"sort":   "created",
			},
			Explanation: "Returns a list of open issues labeled as 'bug', sorted by creation date",
			NextSteps: []string{
				"Prioritize critical issues",
				"Assign issues to team members",
				"Create pull requests to fix bugs",
			},
		},
	}
}

// generateGenericExamples generates examples based on common operations
func (g *UsageExampleGenerator) generateGenericExamples(spec *openapi3.T) []UsageExample {
	examples := []UsageExample{}

	if spec.Paths == nil {
		return examples
	}

	// Find common operation patterns
	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Generate example for GET list operations
			if strings.ToUpper(method) == "GET" && !strings.Contains(path, "{") {
				examples = append(examples, UsageExample{
					Title:       fmt.Sprintf("List %s", g.extractResourceName(path)),
					Description: operation.Summary,
					Scenario:    "Retrieve a list of resources",
					Operation:   operation.OperationID,
					Parameters: map[string]interface{}{
						"limit":  10,
						"offset": 0,
					},
					Explanation: "Returns a paginated list of resources",
					NextSteps: []string{
						"Process the results",
						"Fetch additional pages if needed",
						"Filter results as required",
					},
				})

				if len(examples) >= 3 {
					return examples
				}
			}

			// Generate example for POST create operations
			if strings.ToUpper(method) == "POST" && !strings.Contains(strings.ToLower(path), "search") {
				examples = append(examples, UsageExample{
					Title:       fmt.Sprintf("Create %s", g.extractResourceName(path)),
					Description: operation.Summary,
					Scenario:    "Create a new resource",
					Operation:   operation.OperationID,
					Parameters:  g.generateSampleParameters(operation),
					Explanation: "Creates a new resource and returns its details",
					NextSteps: []string{
						"Save the returned ID",
						"Configure the resource further",
						"Verify creation was successful",
					},
				})

				if len(examples) >= 3 {
					return examples
				}
			}
		}
	}

	return examples
}

// extractResourceName extracts resource name from path
func (g *UsageExampleGenerator) extractResourceName(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, part := range parts {
		if !strings.HasPrefix(part, "{") && part != "v1" && part != "v2" && part != "api" {
			return strings.Title(part)
		}
	}
	return "Resources"
}

// generateSampleParameters generates sample parameters for an operation
func (g *UsageExampleGenerator) generateSampleParameters(operation *openapi3.Operation) map[string]interface{} {
	params := make(map[string]interface{})

	// Extract required parameters
	for _, param := range operation.Parameters {
		if param.Value != nil && param.Value.Required {
			name := param.Value.Name

			// Generate appropriate sample value
			if param.Value.Example != nil {
				params[name] = param.Value.Example
			} else if param.Value.Schema != nil && param.Value.Schema.Value != nil {
				schema := param.Value.Schema.Value
				if schema.Example != nil {
					params[name] = schema.Example
				} else if len(schema.Enum) > 0 {
					params[name] = schema.Enum[0]
				} else {
					params[name] = g.generateSampleValue(name, schema)
				}
			}
		}
	}

	// Extract request body parameters
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		if content := operation.RequestBody.Value.Content.Get("application/json"); content != nil {
			if content.Schema != nil && content.Schema.Value != nil {
				if content.Example != nil {
					// Merge with existing params
					if exampleMap, ok := content.Example.(map[string]interface{}); ok {
						for k, v := range exampleMap {
							params[k] = v
						}
					}
				} else if content.Schema.Value.Properties != nil {
					// Generate from schema
					for propName, prop := range content.Schema.Value.Properties {
						if prop.Value != nil {
							// Check if required
							isRequired := false
							for _, req := range content.Schema.Value.Required {
								if req == propName {
									isRequired = true
									break
								}
							}

							if isRequired {
								if prop.Value.Example != nil {
									params[propName] = prop.Value.Example
								} else {
									params[propName] = g.generateSampleValue(propName, prop.Value)
								}
							}
						}
					}
				}
			}
		}
	}

	return params
}

// generateSampleValue generates a sample value based on name and schema
func (g *UsageExampleGenerator) generateSampleValue(name string, schema *openapi3.Schema) interface{} {
	nameLower := strings.ToLower(name)

	// Name-based generation
	if strings.Contains(nameLower, "id") {
		return "example-id-123"
	}
	if strings.Contains(nameLower, "name") {
		return "example-name"
	}
	if strings.Contains(nameLower, "email") {
		return "user@example.com"
	}
	if strings.Contains(nameLower, "url") {
		return "https://example.com"
	}
	if strings.Contains(nameLower, "description") {
		return "Example description"
	}

	// Type-based generation
	if schema.Type != nil {
		if schema.Type.Is("string") {
			if schema.Format == "date" {
				return "2025-01-01"
			}
			if schema.Format == "date-time" {
				return "2025-01-01T00:00:00Z"
			}
			return "example-string"
		}
		if schema.Type.Is("integer") {
			return 1
		}
		if schema.Type.Is("number") {
			return 1.0
		}
		if schema.Type.Is("boolean") {
			return true
		}
		if schema.Type.Is("array") {
			return []string{"item1", "item2"}
		}
		if schema.Type.Is("object") {
			return map[string]interface{}{"key": "value"}
		}
	}

	return "example-value"
}

// GenerateWorkflowExamples generates workflow examples showing how to chain operations
func (g *UsageExampleGenerator) GenerateWorkflowExamples(spec *openapi3.T, toolName string) []WorkflowExample {
	workflows := []WorkflowExample{}
	toolNameLower := strings.ToLower(toolName)

	if strings.Contains(toolNameLower, "snyk") {
		workflows = append(workflows, WorkflowExample{
			Name:        "Complete Security Audit",
			Description: "Perform a complete security audit of a project",
			Steps: []WorkflowStep{
				{
					Order:       1,
					Operation:   "authenticate",
					Description: "Authenticate with Snyk API",
					Parameters:  map[string]interface{}{"api_key": "${SNYK_API_KEY}"},
				},
				{
					Order:       2,
					Operation:   "import_project",
					Description: "Import project for scanning",
					Parameters:  map[string]interface{}{"repo_url": "${REPO_URL}"},
					SaveAs:      "project",
				},
				{
					Order:       3,
					Operation:   "test_project",
					Description: "Run vulnerability scan",
					Parameters:  map[string]interface{}{"project_id": "${project.id}"},
					SaveAs:      "scan_results",
				},
				{
					Order:       4,
					Operation:   "get_remediation",
					Description: "Get remediation advice",
					Parameters:  map[string]interface{}{"issues": "${scan_results.issues}"},
					SaveAs:      "remediation",
				},
				{
					Order:       5,
					Operation:   "apply_fixes",
					Description: "Apply automated fixes",
					Parameters:  map[string]interface{}{"fixes": "${remediation.automated}"},
				},
			},
		})
	} else if strings.Contains(toolNameLower, "github") {
		workflows = append(workflows, WorkflowExample{
			Name:        "Feature Development Workflow",
			Description: "Complete workflow for developing and merging a feature",
			Steps: []WorkflowStep{
				{
					Order:       1,
					Operation:   "create_branch",
					Description: "Create feature branch",
					Parameters:  map[string]interface{}{"name": "feature/new-feature"},
					SaveAs:      "branch",
				},
				{
					Order:       2,
					Operation:   "create_pull_request",
					Description: "Create PR for review",
					Parameters: map[string]interface{}{
						"head":  "${branch.name}",
						"base":  "main",
						"title": "Add new feature",
					},
					SaveAs: "pr",
				},
				{
					Order:       3,
					Operation:   "request_reviewers",
					Description: "Request code review",
					Parameters: map[string]interface{}{
						"pull_number": "${pr.number}",
						"reviewers":   []string{"reviewer1", "reviewer2"},
					},
				},
				{
					Order:       4,
					Operation:   "merge_pull_request",
					Description: "Merge after approval",
					Parameters: map[string]interface{}{
						"pull_number":  "${pr.number}",
						"merge_method": "squash",
					},
				},
			},
		})
	}

	return workflows
}

// WorkflowExample represents a multi-step workflow
type WorkflowExample struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	Order       int                    `json:"order"`
	Operation   string                 `json:"operation"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	SaveAs      string                 `json:"save_as,omitempty"`
}

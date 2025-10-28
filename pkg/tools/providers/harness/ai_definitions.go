package harness

import "github.com/developer-mesh/developer-mesh/pkg/tools/providers"

// GetAIOptimizedDefinitions returns AI-friendly tool definitions for Harness
// Only includes the 5 core developer workflow modules for context optimization
func (p *HarnessProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	var definitions []providers.AIOptimizedToolDefinition

	if p.enabledModules[ModulePipeline] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_pipelines",
			DisplayName: "Harness CI/CD Pipelines",
			Category:    "CI/CD",
			Subcategory: "Pipeline Management",
			Description: "Create, manage, execute, and monitor CI/CD pipelines in Harness. Supports multi-stage workflows with approval gates, parallel execution, and rollback capabilities.",
			DetailedHelp: `Harness Pipelines enable you to:
- Define multi-stage CI/CD workflows
- Execute builds, tests, and deployments
- Implement approval workflows
- Monitor pipeline executions
- Rollback failed deployments
- Validate pipeline configurations`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Execute a deployment pipeline to production",
					Input: map[string]interface{}{
						"action":     "execute",
						"identifier": "deploy-api-prod",
						"inputs": map[string]interface{}{
							"branch":       "main",
							"environment":  "production",
							"service":      "api-service",
							"skipApproval": false,
						},
					},
					Explanation: "Triggers the production deployment pipeline with approval gates",
				},
				{
					Scenario: "List all pipelines in a project",
					Input: map[string]interface{}{
						"action":  "list",
						"org":     "engineering",
						"project": "platform-services",
						"module":  "cd",
					},
					Explanation: "Retrieves all CD pipelines in the platform-services project",
				},
				{
					Scenario: "Create a new CI pipeline",
					Input: map[string]interface{}{
						"action":  "create",
						"org":     "engineering",
						"project": "mobile-app",
						"parameters": map[string]interface{}{
							"name":        "iOS Build Pipeline",
							"identifier":  "ios-build",
							"description": "Build and test iOS application",
							"stages": []map[string]interface{}{
								{
									"name": "Build",
									"type": "CI",
									"spec": map[string]interface{}{
										"infrastructure": "Kubernetes",
										"steps":          []string{"Clone", "Build", "Test"},
									},
								},
							},
						},
					},
					Explanation: "Creates a new CI pipeline for iOS application builds",
				},
			},
			SemanticTags: []string{
				"pipeline", "cicd", "deployment", "build", "workflow",
				"continuous-integration", "continuous-delivery", "devops",
				"automation", "orchestration", "release",
			},
			CommonPhrases: []string{
				"deploy to production", "run pipeline", "execute build",
				"trigger deployment", "start workflow", "validate pipeline",
				"rollback deployment", "check pipeline status",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Operation to perform (list, get, create, update, delete, execute, validate)",
						Examples:    []interface{}{"list", "execute", "create"},
					},
					"org": {
						Type:        "string",
						Description: "Organization identifier",
						Examples:    []interface{}{"engineering", "devops", "platform"},
					},
					"project": {
						Type:        "string",
						Description: "Project identifier within the organization",
						Examples:    []interface{}{"api-services", "mobile-app", "infrastructure"},
					},
					"pipeline": {
						Type:        "string",
						Description: "Pipeline identifier",
						Examples:    []interface{}{"deploy-prod", "build-staging", "test-suite"},
					},
				},
				Required: []string{"action", "org", "project"},
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "pipelines"},
					{Action: "read", Resource: "pipelines"},
					{Action: "update", Resource: "pipelines"},
					{Action: "delete", Resource: "pipelines"},
					{Action: "execute", Resource: "pipelines"},
					{Action: "validate", Resource: "pipeline-configs"},
					{Action: "rollback", Resource: "deployments"},
				},
				RateLimits: &providers.RateLimitInfo{
					RequestsPerMinute: 100,
					Description:       "Standard tier rate limits. Enterprise plans have higher limits.",
				},
				DataAccess: &providers.DataAccessPattern{
					Pagination:       true,
					MaxResults:       100,
					SupportedFilters: []string{"module", "status", "created_by", "tags"},
					SupportedSorts:   []string{"name", "created_at", "updated_at", "last_execution"},
				},
			},
			ComplexityLevel: "moderate",
		})
	}
	if p.enabledModules[ModuleSTO] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_security",
			DisplayName: "Harness Security Testing",
			Category:    "Security",
			Subcategory: "Vulnerability Management",
			Description: "Orchestrate security scans, manage vulnerabilities, and create exemptions. Integrates with multiple security scanners for comprehensive coverage.",
			UsageExamples: []providers.Example{
				{
					Scenario: "List critical vulnerabilities in a project",
					Input: map[string]interface{}{
						"action":  "list-vulnerabilities",
						"org":     "engineering",
						"project": "api-service",
						"parameters": map[string]interface{}{
							"severity": "CRITICAL",
							"limit":    50,
						},
					},
				},
				{
					Scenario: "Create a security exemption",
					Input: map[string]interface{}{
						"action": "create-exemption",
						"parameters": map[string]interface{}{
							"vulnerability_id": "CVE-2023-1234",
							"reason":           "False positive - not applicable to our use case",
							"expires_at":       "2024-06-01",
						},
					},
				},
			},
			SemanticTags:  []string{"security", "vulnerability", "scan", "CVE", "SAST", "SCA", "container-scanning"},
			CommonPhrases: []string{"security scan", "find vulnerabilities", "create exemption"},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "list", Resource: "scans"},
					{Action: "read", Resource: "vulnerabilities"},
					{Action: "create", Resource: "exemptions"},
				},
			},
		})
	}
	if p.enabledModules[ModuleService] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_services",
			DisplayName: "Harness Services",
			Category:    "Deployment",
			Subcategory: "Service Management",
			Description: "Manage application services, their configurations, and deployment settings in Harness.",
			DetailedHelp: `Harness Services represent the applications or microservices you deploy:
- Define service configurations and manifests
- Manage service versions and artifacts
- Configure service overrides
- Set deployment variables
- Link to artifact sources`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List all services in a project",
					Input: map[string]interface{}{
						"action":  "list",
						"org":     "engineering",
						"project": "platform",
					},
					Explanation: "Retrieves all services in the platform project",
				},
				{
					Scenario: "Create a new Kubernetes service",
					Input: map[string]interface{}{
						"action":  "create",
						"name":    "api-service",
						"type":    "Kubernetes",
						"project": "platform",
					},
					Explanation: "Creates a new Kubernetes service definition",
				},
			},
			SemanticTags:  []string{"service", "application", "microservice", "deployment"},
			CommonPhrases: []string{"deploy service", "create service", "configure application"},
		})
	}
	if p.enabledModules[ModulePullRequest] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_pullrequests",
			DisplayName: "Harness Code Pull Requests",
			Category:    "Development",
			Subcategory: "Code Review",
			Description: "Create, review, and merge pull requests in Harness Code repositories.",
			DetailedHelp: `Manage pull requests in Harness Code:
- Create new pull requests
- Review and approve changes
- Merge pull requests
- Track PR status
- Manage review comments`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a pull request",
					Input: map[string]interface{}{
						"action":       "create",
						"repo":         "platform-api",
						"title":        "Add authentication feature",
						"sourceBranch": "feature/auth",
						"targetBranch": "main",
					},
					Explanation: "Creates a new pull request for code review",
				},
			},
			SemanticTags:  []string{"pullrequest", "code-review", "merge", "git"},
			CommonPhrases: []string{"create PR", "review code", "merge changes"},
		})
	}
	if p.enabledModules[ModuleExecution] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_executions",
			DisplayName: "Harness Execution Management",
			Category:    "Operations",
			Subcategory: "Execution Tracking",
			Description: "Track, manage, and control pipeline executions.",
			DetailedHelp: `Execution management:
- Track execution status
- View execution history
- Abort running executions
- Rollback deployments
- Execution analytics`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Abort a running pipeline",
					Input: map[string]interface{}{
						"action":          "abort",
						"planExecutionId": "exec-12345",
						"reason":          "Detected issue in deployment",
					},
					Explanation: "Aborts a running pipeline execution",
				},
			},
			SemanticTags:  []string{"execution", "pipeline", "deployment", "rollback"},
			CommonPhrases: []string{"stop execution", "rollback deployment", "execution status"},
		})
	}

	return definitions
}

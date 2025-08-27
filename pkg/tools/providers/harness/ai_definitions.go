package harness

import "github.com/developer-mesh/developer-mesh/pkg/tools/providers"

// GetAIOptimizedDefinitions returns AI-friendly tool definitions for Harness
func (p *HarnessProvider) GetAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	var definitions []providers.AIOptimizedToolDefinition

	// Pipeline Management
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

	// Project & Organization Management
	if p.enabledModules[ModuleProject] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_projects",
			DisplayName: "Harness Projects & Organizations",
			Category:    "Platform",
			Subcategory: "Resource Management",
			Description: "Manage Harness organizations and projects. Projects group related pipelines, services, and resources within an organization.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a new project for microservices",
					Input: map[string]interface{}{
						"action": "create",
						"org":    "engineering",
						"parameters": map[string]interface{}{
							"identifier":  "microservices",
							"name":        "Microservices Platform",
							"description": "All microservices and their pipelines",
							"tags":        map[string]string{"team": "platform", "env": "multi"},
						},
					},
				},
				{
					Scenario: "List all projects with CD module enabled",
					Input: map[string]interface{}{
						"action":     "list",
						"org":        "engineering",
						"has_module": "CD",
					},
				},
			},
			SemanticTags:  []string{"project", "organization", "workspace", "tenant", "resource-management"},
			CommonPhrases: []string{"create project", "list projects", "organize resources"},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "projects"},
					{Action: "read", Resource: "projects"},
					{Action: "update", Resource: "projects"},
					{Action: "delete", Resource: "projects"},
					{Action: "list", Resource: "organizations"},
				},
			},
		})
	}

	// Connector Management
	if p.enabledModules[ModuleConnector] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_connectors",
			DisplayName: "Harness Connectors",
			Category:    "Integration",
			Subcategory: "External Services",
			Description: "Configure and manage connections to external services like Git providers, cloud platforms, artifact registries, and monitoring tools.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a GitHub connector",
					Input: map[string]interface{}{
						"action":  "create",
						"org":     "engineering",
						"project": "platform",
						"parameters": map[string]interface{}{
							"identifier": "github-main",
							"name":       "GitHub Main",
							"type":       "Github",
							"spec": map[string]interface{}{
								"url":            "https://github.com/myorg",
								"authentication": "PersonalAccessToken",
							},
						},
					},
				},
				{
					Scenario: "Validate a cloud connector",
					Input: map[string]interface{}{
						"action":     "validate",
						"identifier": "aws-prod",
					},
				},
			},
			SemanticTags:  []string{"connector", "integration", "github", "aws", "kubernetes", "docker"},
			CommonPhrases: []string{"connect to github", "setup aws", "add kubernetes cluster"},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "create", Resource: "connectors"},
					{Action: "validate", Resource: "connectors"},
					{Action: "update", Resource: "connector-credentials"},
				},
			},
		})
	}

	// GitOps Management
	if p.enabledModules[ModuleGitOps] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_gitops",
			DisplayName: "Harness GitOps",
			Category:    "GitOps",
			Subcategory: "Continuous Delivery",
			Description: "Manage GitOps agents, applications, and deployments. Implement pull-based deployments with automatic synchronization and drift detection.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Sync a GitOps application with its Git source",
					Input: map[string]interface{}{
						"action":   "sync",
						"app_name": "frontend-prod",
						"parameters": map[string]interface{}{
							"revision": "main",
							"prune":    true,
							"dryRun":   false,
						},
					},
				},
				{
					Scenario: "Rollback an application to previous version",
					Input: map[string]interface{}{
						"action":   "rollback",
						"app_name": "api-service",
						"parameters": map[string]interface{}{
							"targetRevision": "v1.2.3",
						},
					},
				},
			},
			SemanticTags:  []string{"gitops", "argocd", "flux", "kubernetes", "deployment", "sync"},
			CommonPhrases: []string{"sync application", "deploy with gitops", "rollback version"},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "sync", Resource: "applications"},
					{Action: "rollback", Resource: "applications"},
					{Action: "list", Resource: "agents"},
				},
			},
		})
	}

	// Security Testing Orchestration
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

	// Cloud Cost Management
	if p.enabledModules[ModuleCCM] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_cost",
			DisplayName: "Harness Cloud Cost Management",
			Category:    "FinOps",
			Subcategory: "Cost Optimization",
			Description: "Monitor cloud costs, manage budgets, identify anomalies, and get optimization recommendations across multiple cloud providers.",
			UsageExamples: []providers.Example{
				{
					Scenario: "Get cost overview for the last month",
					Input: map[string]interface{}{
						"action": "cost-overview",
						"parameters": map[string]interface{}{
							"period":     "LAST_30_DAYS",
							"group_by":   "SERVICE",
							"cloud_type": "AWS",
						},
					},
				},
				{
					Scenario: "List cost optimization recommendations",
					Input: map[string]interface{}{
						"action": "list-recommendations",
						"parameters": map[string]interface{}{
							"min_savings":   100,
							"resource_type": "EC2_INSTANCE",
						},
					},
				},
			},
			SemanticTags:  []string{"cost", "budget", "finops", "cloud-spend", "optimization", "anomaly"},
			CommonPhrases: []string{"cloud costs", "cost anomaly", "save money", "budget alert"},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "read", Resource: "costs"},
					{Action: "list", Resource: "budgets"},
					{Action: "read", Resource: "recommendations"},
					{Action: "detect", Resource: "anomalies"},
				},
			},
		})
	}

	return definitions
}

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

	// Service Management
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

	// Environment Management
	if p.enabledModules[ModuleEnvironment] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_environments",
			DisplayName: "Harness Environments",
			Category:    "Deployment",
			Subcategory: "Environment Configuration",
			Description: "Define and manage deployment environments like dev, staging, and production.",
			DetailedHelp: `Harness Environments represent deployment targets:
- Create environment definitions
- Configure environment variables
- Set environment-specific overrides
- Manage environment groups
- Define deployment restrictions`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a production environment",
					Input: map[string]interface{}{
						"action":      "create",
						"name":        "production",
						"type":        "Production",
						"description": "Production environment",
					},
					Explanation: "Creates a production environment configuration",
				},
			},
			SemanticTags:  []string{"environment", "deployment", "staging", "production", "development"},
			CommonPhrases: []string{"setup environment", "create production", "configure staging"},
		})
	}

	// Infrastructure Management
	if p.enabledModules[ModuleInfra] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_infrastructure",
			DisplayName: "Harness Infrastructure Definitions",
			Category:    "Infrastructure",
			Subcategory: "Infrastructure Management",
			Description: "Define infrastructure targets for deployments including Kubernetes clusters, cloud regions, and servers.",
			DetailedHelp: `Infrastructure Definitions specify where services are deployed:
- Kubernetes cluster configurations
- AWS/Azure/GCP regions
- Server groups
- Custom infrastructure
- Infrastructure provisioners`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Define a Kubernetes infrastructure",
					Input: map[string]interface{}{
						"action":      "create",
						"name":        "k8s-prod-cluster",
						"type":        "KubernetesDirect",
						"namespace":   "production",
						"connectorId": "k8s-connector",
					},
					Explanation: "Creates infrastructure definition for Kubernetes deployment",
				},
			},
			SemanticTags:  []string{"infrastructure", "kubernetes", "cloud", "deployment-target"},
			CommonPhrases: []string{"setup infrastructure", "configure cluster", "define deployment target"},
		})
	}

	// Pull Request Management
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

	// Repository Management
	if p.enabledModules[ModuleRepository] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_repositories",
			DisplayName: "Harness Code Repositories",
			Category:    "Development",
			Subcategory: "Source Control",
			Description: "Manage code repositories, branches, and commits in Harness Code.",
			DetailedHelp: `Harness Code repository management:
- List and manage repositories
- Browse branches
- View commit history
- Track code changes
- Repository settings`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List repository branches",
					Input: map[string]interface{}{
						"action": "branches",
						"repo":   "platform-api",
					},
					Explanation: "Lists all branches in the repository",
				},
			},
			SemanticTags:  []string{"repository", "git", "source-code", "version-control"},
			CommonPhrases: []string{"list repos", "show branches", "view commits"},
		})
	}

	// Registry Management
	if p.enabledModules[ModuleRegistry] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_registries",
			DisplayName: "Harness Artifact Registries",
			Category:    "Artifacts",
			Subcategory: "Registry Management",
			Description: "Manage artifact registries and container images for deployments.",
			DetailedHelp: `Artifact registry management:
- Configure Docker registries
- Manage artifact repositories
- List available artifacts
- Track artifact versions
- Registry credentials`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List artifacts in a registry",
					Input: map[string]interface{}{
						"action":         "artifacts",
						"connectorRef":   "dockerhub",
						"repositoryName": "myapp",
					},
					Explanation: "Lists all artifacts in the specified repository",
				},
			},
			SemanticTags:  []string{"registry", "artifacts", "docker", "container", "images"},
			CommonPhrases: []string{"list images", "configure registry", "find artifacts"},
		})
	}

	// Dashboard Management
	if p.enabledModules[ModuleDashboard] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_dashboards",
			DisplayName: "Harness Dashboards",
			Category:    "Monitoring",
			Subcategory: "Analytics",
			Description: "Access and manage dashboards for monitoring deployments and metrics.",
			DetailedHelp: `Dashboard capabilities:
- View deployment dashboards
- Access custom metrics
- Monitor pipeline performance
- Track deployment frequency
- Analyze failure rates`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Get dashboard data for last 7 days",
					Input: map[string]interface{}{
						"action":      "data",
						"dashboardId": "deployment-metrics",
						"startTime":   "-7d",
						"endTime":     "now",
					},
					Explanation: "Retrieves dashboard metrics for the past week",
				},
			},
			SemanticTags:  []string{"dashboard", "metrics", "monitoring", "analytics"},
			CommonPhrases: []string{"show dashboard", "view metrics", "deployment stats"},
		})
	}

	// Chaos Engineering
	if p.enabledModules[ModuleChaos] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_chaos",
			DisplayName: "Harness Chaos Engineering",
			Category:    "Testing",
			Subcategory: "Chaos Engineering",
			Description: "Run chaos experiments to test system resilience and reliability.",
			DetailedHelp: `Chaos Engineering features:
- Design chaos experiments
- Execute fault injection
- Monitor experiment results
- Analyze system resilience
- Generate chaos reports`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Run a pod delete experiment",
					Input: map[string]interface{}{
						"action":       "run",
						"experimentId": "pod-delete-test",
						"target":       "api-service",
					},
					Explanation: "Executes a chaos experiment to test pod failure recovery",
				},
			},
			SemanticTags:  []string{"chaos", "resilience", "fault-injection", "testing"},
			CommonPhrases: []string{"run chaos test", "test resilience", "inject failure"},
		})
	}

	// Supply Chain Security
	if p.enabledModules[ModuleSSCA] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_ssca",
			DisplayName: "Harness Supply Chain Security",
			Category:    "Security",
			Subcategory: "Supply Chain",
			Description: "Manage software supply chain security with SBOM generation and vulnerability scanning.",
			DetailedHelp: `Supply Chain Security features:
- Generate Software Bill of Materials (SBOM)
- Scan artifacts for vulnerabilities
- Track dependency risks
- Compliance reporting
- Security policy enforcement`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Generate SBOM for an artifact",
					Input: map[string]interface{}{
						"action":     "generate",
						"artifactId": "api-service:v1.2.3",
						"format":     "cyclonedx",
					},
					Explanation: "Generates an SBOM for the specified artifact",
				},
			},
			SemanticTags:  []string{"security", "sbom", "vulnerabilities", "compliance", "supply-chain"},
			CommonPhrases: []string{"generate sbom", "scan vulnerabilities", "security scan"},
		})
	}

	// Logs Management
	if p.enabledModules[ModuleLogs] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_logs",
			DisplayName: "Harness Logs",
			Category:    "Operations",
			Subcategory: "Logging",
			Description: "Access and stream execution logs from pipelines and deployments.",
			DetailedHelp: `Log management features:
- Download execution logs
- Stream logs in real-time
- Search log content
- Filter by execution
- Export log data`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Stream logs for a pipeline execution",
					Input: map[string]interface{}{
						"action": "stream",
						"key":    "pipeline-123-execution",
						"follow": true,
					},
					Explanation: "Streams logs in real-time for the execution",
				},
			},
			SemanticTags:  []string{"logs", "logging", "debugging", "troubleshooting"},
			CommonPhrases: []string{"show logs", "download logs", "stream output"},
		})
	}

	// Template Management
	if p.enabledModules[ModuleTemplate] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_templates",
			DisplayName: "Harness Templates",
			Category:    "Configuration",
			Subcategory: "Templates",
			Description: "Create and manage reusable templates for pipelines, stages, and steps.",
			DetailedHelp: `Template management:
- Create reusable templates
- Version template changes
- Share across projects
- Template library
- Template instantiation`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a deployment stage template",
					Input: map[string]interface{}{
						"action":       "create",
						"name":         "k8s-deployment-template",
						"templateType": "Stage",
						"description":  "Standard Kubernetes deployment stage",
					},
					Explanation: "Creates a reusable stage template",
				},
			},
			SemanticTags:  []string{"template", "reusable", "configuration", "library"},
			CommonPhrases: []string{"create template", "use template", "template library"},
		})
	}

	// Internal Developer Portal
	if p.enabledModules[ModuleIDP] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_idp",
			DisplayName: "Harness Internal Developer Portal",
			Category:    "Platform",
			Subcategory: "Developer Experience",
			Description: "Manage the internal developer portal with service catalog and scorecards.",
			DetailedHelp: `IDP features:
- Service catalog management
- Developer scorecards
- Entity relationships
- API documentation
- Developer metrics`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Get service scorecard",
					Input: map[string]interface{}{
						"action":      "get",
						"scorecardId": "service-maturity",
						"entityId":    "api-service",
					},
					Explanation: "Retrieves the maturity scorecard for a service",
				},
			},
			SemanticTags:  []string{"idp", "portal", "catalog", "scorecard", "developer-experience"},
			CommonPhrases: []string{"service catalog", "developer portal", "service scorecard"},
		})
	}

	// Audit Trail
	if p.enabledModules[ModuleAudit] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_audit",
			DisplayName: "Harness Audit Trail",
			Category:    "Compliance",
			Subcategory: "Audit",
			Description: "Track user activities, changes, and access for compliance and security.",
			DetailedHelp: `Audit trail features:
- Track all user actions
- Monitor configuration changes
- Access history
- Compliance reporting
- Security auditing`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List audit events for last 24 hours",
					Input: map[string]interface{}{
						"action":    "list",
						"startTime": "-24h",
						"filter":    "action=UPDATE",
					},
					Explanation: "Retrieves all update actions from the last day",
				},
			},
			SemanticTags:  []string{"audit", "compliance", "security", "tracking", "history"},
			CommonPhrases: []string{"audit trail", "who changed", "track changes"},
		})
	}

	// Database Operations
	if p.enabledModules[ModuleDatabase] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_database",
			DisplayName: "Harness Database Operations",
			Category:    "Data",
			Subcategory: "Database Management",
			Description: "Manage database schemas, migrations, and metadata.",
			DetailedHelp: `Database management:
- Schema discovery
- Migration tracking
- Database metadata
- Schema versioning
- Change management`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List database migrations",
					Input: map[string]interface{}{
						"action":  "migrations",
						"project": "platform",
						"status":  "pending",
					},
					Explanation: "Lists pending database migrations",
				},
			},
			SemanticTags:  []string{"database", "schema", "migrations", "data"},
			CommonPhrases: []string{"database schema", "run migrations", "schema changes"},
		})
	}

	// Feature Flags
	if p.enabledModules[ModuleFF] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_featureflags",
			DisplayName: "Harness Feature Flags",
			Category:    "Feature Management",
			Subcategory: "Feature Flags",
			Description: "Manage feature flags for progressive delivery and experimentation.",
			DetailedHelp: `Feature Flag capabilities:
- Create and manage flags
- Configure targeting rules
- Progressive rollouts
- A/B testing
- Kill switches`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Toggle a feature flag",
					Input: map[string]interface{}{
						"action":      "toggle",
						"identifier":  "new-ui-feature",
						"environment": "production",
						"enabled":     true,
					},
					Explanation: "Enables a feature flag in production",
				},
			},
			SemanticTags:  []string{"feature-flag", "toggle", "rollout", "experimentation"},
			CommonPhrases: []string{"enable feature", "toggle flag", "feature rollout"},
		})
	}

	// Continuous Verification
	if p.enabledModules[ModuleCV] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_cv",
			DisplayName: "Harness Continuous Verification",
			Category:    "Monitoring",
			Subcategory: "Service Reliability",
			Description: "Monitor service health with SLIs, SLOs, and automated verification.",
			DetailedHelp: `Continuous Verification features:
- Monitored service configuration
- Health source integration
- SLI/SLO management
- Automated rollback
- Performance baselines`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List SLOs for a service",
					Input: map[string]interface{}{
						"action":                     "slo/list",
						"monitoredServiceIdentifier": "api-service-prod",
					},
					Explanation: "Lists all SLOs configured for the service",
				},
			},
			SemanticTags:  []string{"monitoring", "sli", "slo", "verification", "reliability"},
			CommonPhrases: []string{"service health", "slo status", "monitor performance"},
		})
	}

	// Infrastructure as Code
	if p.enabledModules[ModuleIaCM] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_iacm",
			DisplayName: "Harness Infrastructure as Code",
			Category:    "Infrastructure",
			Subcategory: "IaC Management",
			Description: "Manage Terraform and OpenTofu workspaces with cost estimation.",
			DetailedHelp: `IaCM features:
- Terraform workspace management
- OpenTofu support
- Cost estimation
- Stack management
- Drift detection`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Estimate infrastructure costs",
					Input: map[string]interface{}{
						"action":      "cost-estimation",
						"workspaceId": "prod-infrastructure",
						"planFile":    "terraform.plan",
					},
					Explanation: "Estimates costs for infrastructure changes",
				},
			},
			SemanticTags:  []string{"terraform", "opentofu", "iac", "infrastructure-as-code"},
			CommonPhrases: []string{"terraform workspace", "estimate costs", "infrastructure code"},
		})
	}

	// Execution Management
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

	// Secrets Management
	if p.enabledModules[ModuleSecret] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_secrets",
			DisplayName: "Harness Secrets Management",
			Category:    "Security",
			Subcategory: "Secrets",
			Description: "Securely manage secrets, passwords, and sensitive configuration.",
			DetailedHelp: `Secrets management:
- Store encrypted secrets
- Reference management
- Secret rotation
- Access control
- Audit secret usage`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Create a new secret",
					Input: map[string]interface{}{
						"action":      "create",
						"identifier":  "db-password",
						"type":        "SecretText",
						"description": "Database password",
					},
					Explanation: "Creates a new encrypted secret",
				},
			},
			SemanticTags:  []string{"secrets", "credentials", "passwords", "encryption"},
			CommonPhrases: []string{"store secret", "create password", "manage credentials"},
		})
	}

	// User and Role Management
	if p.enabledModules[ModuleUser] {
		definitions = append(definitions, providers.AIOptimizedToolDefinition{
			Name:        "harness_users",
			DisplayName: "Harness User & Access Management",
			Category:    "Security",
			Subcategory: "Access Control",
			Description: "Manage users, groups, roles, and permissions for access control.",
			DetailedHelp: `User management features:
- User administration
- Group management
- Role-based access control
- Permission management
- Access policies`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List users in a group",
					Input: map[string]interface{}{
						"action":     "usergroups/get",
						"identifier": "developers",
					},
					Explanation: "Retrieves all users in the developers group",
				},
			},
			SemanticTags:  []string{"users", "rbac", "permissions", "access-control", "groups"},
			CommonPhrases: []string{"add user", "grant permission", "manage access"},
		})
	}

	return definitions
}

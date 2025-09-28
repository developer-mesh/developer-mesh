package xray

import "github.com/developer-mesh/developer-mesh/pkg/tools/providers"

// getXrayAIOptimizedDefinitions returns AI-optimized definitions for Xray operations
func getXrayAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	return []providers.AIOptimizedToolDefinition{
		// Scanning Operations
		{
			Name:        "xray_scan_artifact",
			DisplayName: "Scan Artifact for Vulnerabilities",
			Category:    "security_scanning",
			Subcategory: "artifact_scanning",
			Description: "Scan a specific artifact for security vulnerabilities, license violations, and operational risks",
			DetailedHelp: "Initiates a comprehensive security scan of an artifact stored in Artifactory. " +
				"The scan checks for known CVEs, license compliance issues, and operational risks. " +
				"Results include severity levels (Critical, High, Medium, Low) and remediation suggestions.",
			SemanticTags: []string{
				"scan", "vulnerability", "security", "cve", "artifact",
				"check", "analyze", "inspect", "audit", "assessment",
			},
			CommonPhrases: []string{
				"scan this artifact",
				"check for vulnerabilities",
				"security scan",
				"find CVEs",
				"check security issues",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"componentId": {
						Type:        "string",
						Description: "The component ID to scan (e.g., 'docker://myrepo/myimage:tag')",
						Examples:    []interface{}{"docker://myrepo/nginx:latest", "npm://lodash:4.17.21"},
						Template:    "{type}://{repo}/{name}:{version}",
					},
					"watch": {
						Type:        "string",
						Description: "Optional watch name to apply policies from",
						Examples:    []interface{}{"production-watch", "dev-watch"},
					},
				},
				Required: []string{"componentId"},
				Examples: []map[string]interface{}{
					{
						"componentId": "docker://docker-local/myapp:1.0.0",
						"watch":       "production-watch",
					},
				},
			},
			OutputSchema: &providers.ResponseSchema{
				Type:        "object",
				Description: "Scan results with vulnerabilities grouped by severity",
				Properties: map[string]interface{}{
					"scan_id":   "string - Unique scan identifier",
					"status":    "string - Scan status (completed, in_progress, failed)",
					"critical":  "array - Critical severity vulnerabilities",
					"high":      "array - High severity vulnerabilities",
					"medium":    "array - Medium severity vulnerabilities",
					"low":       "array - Low severity vulnerabilities",
					"licenses":  "array - License violations if any",
					"scan_time": "string - Time taken to complete scan",
				},
			},
			Capabilities: &providers.ToolCapabilities{
				Capabilities: []providers.Capability{
					{Action: "scan", Resource: "artifacts"},
					{Action: "analyze", Resource: "vulnerabilities"},
					{Action: "detect", Resource: "licenses"},
				},
				Limitations: []providers.Limitation{
					{
						Description: "Scan results depend on JFrog's vulnerability database updates",
						Workaround:  "Ensure Xray database is regularly updated",
					},
					{
						Description: "Large artifacts may take longer to scan",
						Workaround:  "Use async scanning with status checks for large artifacts",
					},
				},
				RateLimits: &providers.RateLimitInfo{
					RequestsPerMinute: 30,
					Description:       "Scanning operations are resource-intensive",
				},
			},
			UsageExamples: []providers.Example{
				{
					Scenario: "Scan a Docker image for vulnerabilities",
					Input: map[string]interface{}{
						"componentId": "docker://docker-prod/api-server:2.1.0",
					},
					Explanation: "Scans the specified Docker image for all known vulnerabilities",
				},
				{
					Scenario: "Scan npm package with policy enforcement",
					Input: map[string]interface{}{
						"componentId": "npm://express:4.18.0",
						"watch":       "nodejs-policy",
					},
					Explanation: "Scans npm package and applies policies from the specified watch",
				},
			},
			ComplexityLevel: "simple",
		},

		// Build Scanning
		{
			Name:        "xray_scan_build",
			DisplayName: "Scan Build for Vulnerabilities",
			Category:    "security_scanning",
			Subcategory: "build_scanning",
			Description: "Scan all artifacts in a build for security vulnerabilities",
			DetailedHelp: "Scans an entire build including all its artifacts and dependencies. " +
				"Provides aggregated security report for the complete build. " +
				"Useful for CI/CD pipeline integration to gate deployments based on security criteria.",
			SemanticTags: []string{
				"build", "scan", "ci", "cd", "pipeline",
				"deployment", "release", "security", "gate",
			},
			CommonPhrases: []string{
				"scan the build",
				"check build security",
				"scan all artifacts in build",
				"CI/CD security check",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"buildName": {
						Type:        "string",
						Description: "Name of the build to scan",
						Examples:    []interface{}{"my-app", "backend-service", "frontend"},
					},
					"buildNumber": {
						Type:        "string",
						Description: "Build number or version",
						Examples:    []interface{}{"1.0.0", "build-123", "2.3.4-SNAPSHOT"},
					},
				},
				Required: []string{"buildName", "buildNumber"},
			},
			ComplexityLevel: "simple",
		},

		// Artifact Summary
		{
			Name:        "xray_artifact_summary",
			DisplayName: "Get Artifact Security Summary",
			Category:    "security_scanning",
			Subcategory: "vulnerability_reports",
			Description: "Get a detailed security summary for specific artifacts",
			DetailedHelp: "Retrieves comprehensive security information for artifacts including " +
				"vulnerabilities, licenses, and component graph. Does not trigger a new scan, " +
				"returns cached scan results if available.",
			SemanticTags: []string{
				"summary", "report", "vulnerabilities", "status",
				"overview", "details", "information", "get", "retrieve",
			},
			CommonPhrases: []string{
				"get vulnerability report",
				"show security summary",
				"artifact security status",
				"vulnerability details",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"paths": {
						Type:        "array",
						Description: "Array of artifact paths to get summary for",
						ItemType:    "string",
						Examples:    []interface{}{[]string{"default/docker-local/nginx:latest", "default/npm-local/lodash/-/lodash-4.17.21.tgz"}},
					},
					"include_licenses": {
						Type:        "boolean",
						Description: "Include license information in summary",
						Examples:    []interface{}{true, false},
					},
				},
				Required: []string{"paths"},
			},
			ComplexityLevel: "simple",
		},

		// Component Intelligence
		{
			Name:        "xray_component_details",
			DisplayName: "Get Component Vulnerability Details",
			Category:    "vulnerability_intelligence",
			Subcategory: "component_analysis",
			Description: "Get detailed vulnerability information for a specific component",
			DetailedHelp: "Retrieves comprehensive vulnerability data for a component including " +
				"all known CVEs, CVSS scores, affected versions, and available fixes. " +
				"Useful for understanding the security posture of specific dependencies.",
			SemanticTags: []string{
				"component", "cve", "vulnerability", "details",
				"package", "dependency", "library", "framework",
			},
			CommonPhrases: []string{
				"check component vulnerabilities",
				"CVE details for package",
				"vulnerability info",
				"security details for library",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"component_id": {
						Type:        "string",
						Description: "Component identifier (package_type:name:version)",
						Examples:    []interface{}{"npm:lodash:4.17.21", "docker:nginx:1.21.0", "maven:org.springframework:spring-core:5.3.0"},
						Template:    "{package_type}:{name}:{version}",
					},
					"include_fixed_versions": {
						Type:        "boolean",
						Description: "Include information about fixed versions",
						Examples:    []interface{}{true, false},
					},
				},
				Required: []string{"component_id"},
			},
			ComplexityLevel: "moderate",
		},

		// Violations
		{
			Name:        "xray_list_violations",
			DisplayName: "List Security & License Violations",
			Category:    "compliance",
			Subcategory: "violation_management",
			Description: "List all security and license violations based on configured policies",
			DetailedHelp: "Retrieves violations detected by Xray policies including security vulnerabilities " +
				"that exceed severity thresholds and license violations. Results can be filtered by " +
				"type, severity, date range, watch, or policy.",
			SemanticTags: []string{
				"violations", "compliance", "policy", "breach",
				"non-compliant", "issues", "problems", "alerts",
			},
			CommonPhrases: []string{
				"show violations",
				"list policy breaches",
				"compliance issues",
				"security violations",
				"license violations",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"type": {
						Type:        "string",
						Description: "Type of violations to retrieve",
						Examples:    []interface{}{"security", "license", "operational_risk"},
					},
					"severity": {
						Type:        "string",
						Description: "Minimum severity level",
						Examples:    []interface{}{"critical", "major", "minor"},
					},
					"watch_name": {
						Type:        "string",
						Description: "Filter by specific watch",
						Examples:    []interface{}{"production-watch", "dev-watch"},
					},
					"policy_name": {
						Type:        "string",
						Description: "Filter by specific policy",
						Examples:    []interface{}{"no-high-vulns", "approved-licenses-only"},
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results",
						Examples:    []interface{}{50, 100, 200},
					},
				},
			},
			ComplexityLevel: "simple",
		},

		// Watch Management
		{
			Name:        "xray_create_watch",
			DisplayName: "Create Security Watch",
			Category:    "monitoring",
			Subcategory: "watch_management",
			Description: "Create a watch to continuously monitor repositories, builds, or projects",
			DetailedHelp: "Creates a watch that continuously monitors specified resources for security issues. " +
				"Watches can monitor repositories, builds, or projects and trigger actions when " +
				"violations are detected based on associated policies.",
			SemanticTags: []string{
				"watch", "monitor", "continuous", "surveillance",
				"tracking", "observe", "alert", "notification",
			},
			CommonPhrases: []string{
				"create watch",
				"monitor repository",
				"set up monitoring",
				"configure alerts",
				"watch for vulnerabilities",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"name": {
						Type:        "string",
						Description: "Unique name for the watch",
						Examples:    []interface{}{"prod-docker-watch", "npm-security-watch"},
					},
					"description": {
						Type:        "string",
						Description: "Description of what this watch monitors",
						Examples:    []interface{}{"Monitor production Docker images", "Watch npm dependencies"},
					},
					"repositories": {
						Type:        "array",
						Description: "List of repositories to watch",
						ItemType:    "string",
						Examples:    []interface{}{[]string{"docker-prod", "npm-local"}},
					},
					"policies": {
						Type:        "array",
						Description: "List of policies to apply",
						ItemType:    "string",
						Examples:    []interface{}{[]string{"no-critical-vulns", "approved-licenses"}},
					},
					"watch_recipients": {
						Type:        "array",
						Description: "Email addresses for notifications",
						ItemType:    "string",
						Examples:    []interface{}{[]string{"security@company.com", "devops@company.com"}},
					},
				},
				Required: []string{"name", "description"},
				AIHints: &providers.AIParameterHints{
					ConditionalRequirements: []providers.ConditionalRequirement{
						{If: "repositories is specified", Then: "policies should also be specified"},
					},
				},
			},
			ComplexityLevel: "moderate",
		},

		// Policy Management
		{
			Name:        "xray_create_policy",
			DisplayName: "Create Security Policy",
			Category:    "policy_management",
			Subcategory: "security_policies",
			Description: "Create a security or license policy to enforce compliance standards",
			DetailedHelp: "Creates a policy that defines security and compliance rules. Policies can " +
				"block downloads, fail builds, or send notifications when violations are detected. " +
				"Rules can be based on CVE severity, CVSS score, license type, or age of vulnerability.",
			SemanticTags: []string{
				"policy", "rules", "compliance", "governance",
				"standards", "requirements", "enforcement", "restrictions",
			},
			CommonPhrases: []string{
				"create security policy",
				"define rules",
				"set compliance standards",
				"enforce security requirements",
				"block vulnerabilities",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"name": {
						Type:        "string",
						Description: "Unique policy name",
						Examples:    []interface{}{"no-critical-cves", "gpl-license-block"},
					},
					"type": {
						Type:        "string",
						Description: "Policy type",
						Examples:    []interface{}{"security", "license", "operational_risk"},
					},
					"rules": {
						Type:        "array",
						Description: "Policy rules defining the criteria",
						ItemType:    "object",
						Examples: []interface{}{
							[]map[string]interface{}{
								{"criteria": "min_severity", "value": "critical", "action": "block"},
								{"criteria": "cvss_score", "value": 7.0, "action": "warn"},
							},
						},
					},
					"actions": {
						Type:        "object",
						Description: "Actions to take when policy is violated",
						Properties: map[string]providers.AIPropertySchema{
							"block_download": {
								Type:        "boolean",
								Description: "Block artifact download on violation",
							},
							"fail_build": {
								Type:        "boolean",
								Description: "Fail the build on violation",
							},
							"notify": {
								Type:        "array",
								Description: "Email addresses to notify",
								ItemType:    "string",
							},
						},
					},
				},
				Required: []string{"name", "type", "rules"},
			},
			ComplexityLevel: "complex",
		},

		// Reports
		{
			Name:        "xray_generate_vulnerability_report",
			DisplayName: "Generate Vulnerability Report",
			Category:    "reporting",
			Subcategory: "security_reports",
			Description: "Generate comprehensive vulnerability reports for repositories, builds, or components",
			DetailedHelp: "Creates detailed vulnerability reports that can be exported in various formats " +
				"(JSON, PDF, CSV). Reports include vulnerability details, severity distribution, " +
				"affected components, and remediation recommendations.",
			SemanticTags: []string{
				"report", "generate", "export", "document",
				"summary", "analysis", "assessment", "audit",
			},
			CommonPhrases: []string{
				"generate security report",
				"create vulnerability report",
				"export scan results",
				"security audit report",
				"compliance report",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"name": {
						Type:        "string",
						Description: "Report name",
						Examples:    []interface{}{"Q3-Security-Audit", "Production-Vulnerability-Report"},
					},
					"type": {
						Type:        "string",
						Description: "Report scope type",
						Examples:    []interface{}{"repo", "build", "component"},
					},
					"repositories": {
						Type:        "array",
						Description: "Repositories to include (if type=repo)",
						ItemType:    "string",
						Examples:    []interface{}{[]string{"docker-prod", "npm-prod"}},
					},
					"filters": {
						Type:        "object",
						Description: "Filters to apply to report data",
						Properties: map[string]providers.AIPropertySchema{
							"severity": {
								Type:     "array",
								ItemType: "string",
								Examples: []interface{}{[]string{"critical", "high"}},
							},
							"cve": {
								Type:     "array",
								ItemType: "string",
								Examples: []interface{}{[]string{"CVE-2021-44228", "CVE-2021-45046"}},
							},
						},
					},
					"format": {
						Type:        "string",
						Description: "Output format",
						Examples:    []interface{}{"json", "pdf", "csv"},
					},
				},
				Required: []string{"name", "type"},
			},
			ComplexityLevel: "moderate",
		},

		// System Operations
		{
			Name:        "xray_system_health",
			DisplayName: "Check Xray System Health",
			Category:    "system",
			Subcategory: "health_monitoring",
			Description: "Check the health and status of the Xray service",
			DetailedHelp: "Verifies that Xray service is running and accessible. Returns system " +
				"version, database connectivity status, and service health indicators.",
			SemanticTags: []string{
				"health", "status", "ping", "availability",
				"system", "service", "running", "operational",
			},
			CommonPhrases: []string{
				"check Xray status",
				"is Xray running",
				"system health",
				"service status",
			},
			InputSchema: providers.AIParameterSchema{
				Type:       "object",
				Properties: map[string]providers.AIPropertySchema{},
			},
			ComplexityLevel: "simple",
		},
	}
}

// GetXrayErrorResolutions provides resolution suggestions for common Xray errors
func GetXrayErrorResolutions() map[string]string {
	return map[string]string{
		"scan_in_progress": "Scan is still running. Check status with scan/status operation",
		"policy_violation":  "Artifact violates configured policies. Review violations with violations/list",
		"no_xray_data":     "No Xray data available for this artifact. Trigger a scan first",
		"watch_not_found":  "Watch does not exist. Create it with watches/create or list existing with watches/list",
		"policy_not_found": "Policy does not exist. Create it with policies/create or list existing with policies/list",
		"insufficient_permissions": "API key lacks required permissions. Ensure Xray access is enabled for this key",
		"component_not_found":      "Component not found in Xray database. It may not be scanned yet or doesn't exist",
	}
}

// GetXrayCapabilityDescriptions provides descriptions for Xray capabilities
func GetXrayCapabilityDescriptions() map[string]string {
	return map[string]string{
		"vulnerability_scanning": "Scan artifacts and builds for known security vulnerabilities (CVEs)",
		"license_compliance":     "Detect and enforce license compliance across dependencies",
		"operational_risk":       "Identify operational risks like outdated or unmaintained packages",
		"continuous_monitoring":  "Continuously monitor repositories with watches and policies",
		"policy_enforcement":     "Create and enforce security and compliance policies",
		"reporting":             "Generate comprehensive security and compliance reports",
		"component_intelligence": "Access detailed vulnerability data for specific components",
		"ignore_rules":          "Create rules to suppress false positives or accepted risks",
		"impact_analysis":       "Analyze the impact of vulnerabilities across your software supply chain",
	}
}
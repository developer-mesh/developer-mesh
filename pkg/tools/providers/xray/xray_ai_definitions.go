package xray

import "github.com/developer-mesh/developer-mesh/pkg/tools/providers"

// getXrayAIOptimizedDefinitions returns comprehensive AI-optimized definitions for all Xray operations
func getXrayAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	return []providers.AIOptimizedToolDefinition{
		// Scanning Operations
		{
			Name:        "xray_scanning",
			DisplayName: "Security Scanning Operations",
			Category:    "security",
			Subcategory: "vulnerability_scanning",
			Description: "Comprehensive security scanning for artifacts, builds, and components to detect vulnerabilities, license violations, and operational risks",
			DetailedHelp: `Xray scanning operations provide comprehensive security analysis:
- Artifact Scanning: Scan individual artifacts for vulnerabilities and license issues
- Build Scanning: Scan entire builds including all artifacts and dependencies
- Component Intelligence: Deep analysis of specific components and their vulnerabilities
- Dependency Graph: Analyze dependency relationships and impact chains
- License Compliance: Detect license violations and policy breaches
- Operational Risk: Identify outdated or unmaintained components

All scans return detailed vulnerability information with CVSS scores, severity levels, and remediation guidance.`,
			SemanticTags: []string{
				"scan", "vulnerability", "security", "cve", "artifact", "build", "component",
				"check", "analyze", "inspect", "audit", "assessment", "dependency", "license",
				"compliance", "risk", "operational", "cvss", "severity", "remediation",
			},
			CommonPhrases: []string{
				"scan this artifact", "check for vulnerabilities", "security scan", "find CVEs",
				"check security issues", "scan build", "analyze component", "dependency analysis",
				"license compliance", "security assessment", "vulnerability report",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "The scanning operation to perform",
						Examples:    []interface{}{"scan/artifact", "scan/build", "scan/status", "summary/artifact", "summary/build", "components/searchByCves", "graph/artifact"},
						Template:    "{category}/{operation}",
					},
					"parameters": {
						Type:        "object",
						Description: "Operation-specific parameters",
						Properties: map[string]providers.AIPropertySchema{
							"componentId": {
								Type:        "string",
								Description: "Component identifier for artifact scans (format: type://repo/name:version)",
								Examples:    []interface{}{"docker://docker-local/nginx:latest", "npm://npm-local/lodash:4.17.21", "maven://libs-release/com.mycompany:myapp:1.0.0"},
								Template:    "{type}://{repo}/{path}:{version}",
							},
							"buildName": {
								Type:        "string",
								Description: "Build name for build scanning operations",
								Examples:    []interface{}{"my-app", "backend-service", "frontend-app"},
							},
							"buildNumber": {
								Type:        "string",
								Description: "Build number or version for build operations",
								Examples:    []interface{}{"1.0.0", "build-123", "2.3.4-SNAPSHOT"},
							},
							"paths": {
								Type:        "array",
								Description: "Array of artifact paths for summary operations",
								ItemType:    "string",
								Examples:    []interface{}{[]string{"default/docker-local/nginx:latest", "default/npm-local/lodash/-/lodash-4.17.21.tgz"}},
							},
							"cves": {
								Type:        "array",
								Description: "List of CVE IDs for component intelligence searches",
								ItemType:    "string",
								Examples:    []interface{}{[]string{"CVE-2021-44228", "CVE-2021-45046", "CVE-2022-22965"}},
							},
							"component_id": {
								Type:        "string",
								Description: "Component identifier for intelligence operations (format: type:name:version)",
								Examples:    []interface{}{"npm:lodash:4.17.21", "docker:nginx:1.21.0", "maven:org.springframework:spring-core:5.3.0"},
								Template:    "{type}:{name}:{version}",
							},
							"watch": {
								Type:        "string",
								Description: "Watch name to apply policies from",
								Examples:    []interface{}{"production-watch", "dev-watch", "security-policy-watch"},
							},
							"include_licenses": {
								Type:        "boolean",
								Description: "Include license information in scan results",
								Examples:    []interface{}{true, false},
							},
							"include_fixed_versions": {
								Type:        "boolean",
								Description: "Include information about fixed versions for vulnerabilities",
								Examples:    []interface{}{true, false},
							},
						},
					},
				},
				Required: []string{"action"},
				AIHints: &providers.AIParameterHints{
					ParameterGrouping: map[string][]string{
						"artifact_scan": {"componentId", "watch", "include_licenses"},
						"build_scan":    {"buildName", "buildNumber", "watch"},
						"summary":       {"paths", "include_licenses"},
						"intelligence":  {"component_id", "cves", "include_fixed_versions"},
					},
					ConditionalRequirements: []providers.ConditionalRequirement{
						{If: "action=scan/artifact", Then: "componentId is required"},
						{If: "action=scan/build", Then: "buildName,buildNumber are required"},
						{If: "action contains summary", Then: "paths is required"},
						{If: "action contains components/search", Then: "cves or component_id is required"},
						{If: "action contains graph", Then: "componentId or buildName is required"},
					},
				},
			},
			UsageExamples: []providers.Example{
				{
					Scenario: "Scan a Docker image for vulnerabilities",
					Input: map[string]interface{}{
						"action": "scan/artifact",
						"parameters": map[string]interface{}{
							"componentId":      "docker://docker-prod/api-server:2.1.0",
							"watch":            "production-watch",
							"include_licenses": true,
						},
					},
					Explanation: "Initiates a comprehensive scan of the Docker image including vulnerability and license checks",
				},
				{
					Scenario: "Get artifact security summary without new scan",
					Input: map[string]interface{}{
						"action": "summary/artifact",
						"parameters": map[string]interface{}{
							"paths":            []string{"default/docker-local/nginx:latest", "default/npm-local/lodash/-/lodash-4.17.21.tgz"},
							"include_licenses": true,
						},
					},
					Explanation: "Retrieves cached security summary for multiple artifacts",
				},
				{
					Scenario: "Search for components affected by specific CVEs",
					Input: map[string]interface{}{
						"action": "components/searchByCves",
						"parameters": map[string]interface{}{
							"cves": []string{"CVE-2021-44228", "CVE-2021-45046"},
						},
					},
					Explanation: "Finds all components in your repositories affected by Log4j vulnerabilities",
				},
				{
					Scenario: "Analyze dependency graph for an artifact",
					Input: map[string]interface{}{
						"action": "graph/artifact",
						"parameters": map[string]interface{}{
							"componentId": "maven://libs-release/com.mycompany:myapp:1.0.0",
						},
					},
					Explanation: "Retrieves the complete dependency graph to understand impact chains",
				},
			},
			ComplexityLevel: "moderate",
		},

		// Compliance and Policy Management
		{
			Name:        "xray_compliance",
			DisplayName: "Security Compliance and Policy Management",
			Category:    "compliance",
			Subcategory: "policy_management",
			Description: "Comprehensive compliance management including violations, policies, watches, and ignore rules for security governance",
			DetailedHelp: `Xray compliance operations enable comprehensive security governance:
- Violations Management: List, review, and manage security and license violations
- Policy Management: Create, update, and enforce security policies with custom rules
- Watch Management: Set up continuous monitoring of repositories, builds, or projects
- Ignore Rules: Manage exceptions for false positives or accepted risks
- Compliance Reporting: Generate audit reports for security compliance

All operations support role-based access control and integrate with CI/CD for policy enforcement.`,
			SemanticTags: []string{
				"compliance", "policy", "violation", "watch", "monitor", "governance",
				"ignore", "rule", "enforcement", "audit", "breach", "security",
				"license", "operational", "risk", "gate", "ci", "cd", "continuous",
			},
			CommonPhrases: []string{
				"check compliance", "list violations", "create policy", "enforce security",
				"set up monitoring", "ignore false positive", "compliance report", "policy breach",
				"security governance", "watch repository", "violation summary",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "The compliance operation to perform",
						Examples:    []interface{}{"violations/list", "policies/create", "watches/create", "ignore-rules/create", "policies/list", "watches/list"},
						Template:    "{category}/{operation}",
					},
					"parameters": {
						Type:        "object",
						Description: "Operation-specific parameters",
						Properties: map[string]providers.AIPropertySchema{
							"type": {
								Type:        "string",
								Description: "Type of violation, policy, or watch",
								Examples:    []interface{}{"security", "license", "operational_risk"},
							},
							"severity": {
								Type:        "string",
								Description: "Minimum severity level for filtering",
								Examples:    []interface{}{"critical", "major", "minor"},
							},
							"name": {
								Type:        "string",
								Description: "Name for policies, watches, or ignore rules",
								Examples:    []interface{}{"no-critical-cves", "prod-docker-watch", "false-positive-rule"},
							},
							"description": {
								Type:        "string",
								Description: "Description for created resources",
								Examples:    []interface{}{"Block critical vulnerabilities", "Monitor production Docker images"},
							},
							"repositories": {
								Type:        "array",
								Description: "List of repositories to monitor or apply policies to",
								ItemType:    "string",
								Examples:    []interface{}{[]string{"docker-prod", "npm-local", "maven-release"}},
							},
							"policies": {
								Type:        "array",
								Description: "List of policies to apply to a watch",
								ItemType:    "string",
								Examples:    []interface{}{[]string{"no-critical-vulns", "approved-licenses-only"}},
							},
							"rules": {
								Type:        "array",
								Description: "Policy rules defining criteria and actions",
								ItemType:    "object",
								Examples: []interface{}{
									[]map[string]interface{}{
										{"criteria": "min_severity", "value": "critical", "action": "block"},
										{"criteria": "cvss_score", "value": 7.0, "action": "warn"},
									},
								},
							},
							"vulnerability": {
								Type:        "string",
								Description: "CVE ID for ignore rules",
								Examples:    []interface{}{"CVE-2021-44228", "CVE-2022-22965"},
								Template:    "CVE-YYYY-NNNNN",
							},
							"component": {
								Type:        "string",
								Description: "Component identifier for ignore rules",
								Examples:    []interface{}{"npm:lodash:4.17.21", "docker:nginx:1.21.0"},
							},
							"expiry_date": {
								Type:        "string",
								Description: "Expiry date for ignore rules (ISO format)",
								Examples:    []interface{}{"2025-12-31T23:59:59Z"},
								Template:    "YYYY-MM-DDTHH:MM:SSZ",
							},
							"watch_recipients": {
								Type:        "array",
								Description: "Email addresses for notifications",
								ItemType:    "string",
								Examples:    []interface{}{[]string{"security@company.com", "devops@company.com"}},
							},
						},
					},
				},
				Required: []string{"action"},
				AIHints: &providers.AIParameterHints{
					ParameterGrouping: map[string][]string{
						"violations": {"type", "severity", "watch_name", "policy_name"},
						"policies":   {"name", "type", "rules", "description"},
						"watches":    {"name", "description", "repositories", "policies", "watch_recipients"},
						"ignore":     {"vulnerability", "component", "expiry_date"},
					},
					ConditionalRequirements: []providers.ConditionalRequirement{
						{If: "action contains create", Then: "name is required"},
						{If: "action=policies/create", Then: "type,rules are required"},
						{If: "action=watches/create", Then: "description is required"},
						{If: "action=ignore-rules/create", Then: "vulnerability,component are required"},
					},
				},
			},
			UsageExamples: []providers.Example{
				{
					Scenario: "List critical security violations",
					Input: map[string]interface{}{
						"action": "violations/list",
						"parameters": map[string]interface{}{
							"type":     "security",
							"severity": "critical",
						},
					},
					Explanation: "Lists all critical security violations detected by policies",
				},
				{
					Scenario: "Create security policy to block critical vulnerabilities",
					Input: map[string]interface{}{
						"action": "policies/create",
						"parameters": map[string]interface{}{
							"name": "no-critical-cves",
							"type": "security",
							"rules": []map[string]interface{}{
								{"criteria": "min_severity", "value": "critical", "action": "block"},
								{"criteria": "cvss_score", "value": 9.0, "action": "block"},
							},
							"description": "Block downloads with critical vulnerabilities",
						},
					},
					Explanation: "Creates a policy that blocks artifacts with critical vulnerabilities",
				},
				{
					Scenario: "Set up continuous monitoring watch",
					Input: map[string]interface{}{
						"action": "watches/create",
						"parameters": map[string]interface{}{
							"name":             "prod-docker-watch",
							"description":      "Monitor production Docker repositories",
							"repositories":     []string{"docker-prod-local", "docker-prod-remote"},
							"policies":         []string{"no-critical-cves", "approved-licenses"},
							"watch_recipients": []string{"security@company.com"},
						},
					},
					Explanation: "Creates a watch that continuously monitors production Docker repositories",
				},
			},
			ComplexityLevel: "moderate",
		},

		// Reporting and Analytics
		{
			Name:        "xray_reporting",
			DisplayName: "Security Reports and Analytics",
			Category:    "reporting",
			Subcategory: "security_analytics",
			Description: "Comprehensive security reporting and analytics including vulnerability reports, metrics, trends, and compliance dashboards",
			DetailedHelp: `Xray reporting operations provide comprehensive security analytics:
- Vulnerability Reports: Detailed reports on security findings with various export formats (JSON, PDF, CSV, XML)
- License Reports: Compliance reports for license violations and usage
- SBOM Reports: Software Bill of Materials in SPDX and CycloneDX formats
- Operational Risk Reports: Analysis of outdated and unmaintained components
- Compliance Reports: Audit reports for standards like PCI-DSS, HIPAA, SOX
- Security Metrics: Real-time metrics on violations, scans, components, and exposure
- Trend Analysis: Historical trends and forecasting for security posture
- Dashboard Summaries: Aggregated metrics for executive reporting

All reports support filtering, scheduling, and automated delivery to stakeholders.`,
			SemanticTags: []string{
				"report", "analytics", "metrics", "dashboard", "trends", "summary",
				"vulnerability", "license", "compliance", "sbom", "audit", "export",
				"pdf", "csv", "json", "spdx", "cyclonedx", "operational", "risk",
				"violations", "scans", "components", "exposure", "forecast",
			},
			CommonPhrases: []string{
				"generate security report", "export vulnerability report", "compliance report",
				"security metrics", "violation trends", "dashboard summary", "audit report",
				"SBOM export", "license compliance report", "security analytics",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "The reporting operation to perform",
						Examples:    []interface{}{"reports/vulnerability", "reports/license", "reports/sbom", "metrics/violations", "metrics/dashboard", "reports/download"},
						Template:    "{category}/{operation}",
					},
					"parameters": {
						Type:        "object",
						Description: "Report-specific parameters",
						Properties: map[string]providers.AIPropertySchema{
							"name": {
								Type:        "string",
								Description: "Report name or identifier",
								Examples:    []interface{}{"Q1-Security-Audit", "Production-Vulnerability-Report", "SBOM-Export-2025"},
							},
							"type": {
								Type:        "string",
								Description: "Report scope type",
								Examples:    []interface{}{"repository", "build", "component", "global"},
							},
							"format": {
								Type:        "string",
								Description: "Export format for reports",
								Examples:    []interface{}{"json", "pdf", "csv", "xml", "spdx", "cyclonedx"},
							},
							"repositories": {
								Type:        "array",
								Description: "Repositories to include in report",
								ItemType:    "string",
								Examples:    []interface{}{[]string{"docker-prod", "npm-prod", "maven-release"}},
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
									"component": {
										Type:     "array",
										ItemType: "string",
										Examples: []interface{}{[]string{"npm:lodash", "maven:org.springframework"}},
									},
									"license": {
										Type:     "array",
										ItemType: "string",
										Examples: []interface{}{[]string{"GPL-3.0", "AGPL-3.0"}},
									},
									"date_range": {
										Type:     "object",
										Examples: []interface{}{map[string]string{"start": "2025-01-01", "end": "2025-03-31"}},
									},
								},
							},
							"time_range": {
								Type:        "string",
								Description: "Time range for metrics",
								Examples:    []interface{}{"7d", "30d", "90d", "1y"},
							},
							"group_by": {
								Type:        "string",
								Description: "Grouping for metrics",
								Examples:    []interface{}{"severity", "repository", "component_type", "date"},
							},
							"include_trends": {
								Type:        "boolean",
								Description: "Include trend analysis in metrics",
								Examples:    []interface{}{true, false},
							},
							"scheduled": {
								Type:        "boolean",
								Description: "Create as scheduled report",
								Examples:    []interface{}{true, false},
							},
							"frequency": {
								Type:        "string",
								Description: "Schedule frequency for reports",
								Examples:    []interface{}{"daily", "weekly", "monthly", "quarterly"},
							},
							"recipients": {
								Type:        "array",
								Description: "Email recipients for scheduled reports",
								ItemType:    "string",
								Examples:    []interface{}{[]string{"security@company.com", "compliance@company.com"}},
							},
						},
					},
				},
				Required: []string{"action"},
				AIHints: &providers.AIParameterHints{
					ParameterGrouping: map[string][]string{
						"reports":  {"name", "type", "format", "repositories", "filters", "scheduled"},
						"metrics":  {"time_range", "group_by", "include_trends", "filters"},
						"export":   {"format", "name"},
						"schedule": {"frequency", "recipients", "scheduled"},
					},
					ConditionalRequirements: []providers.ConditionalRequirement{
						{If: "action contains reports", Then: "name is required"},
						{If: "action contains export or download", Then: "format should be specified"},
						{If: "scheduled=true", Then: "frequency,recipients are required"},
						{If: "type=repository", Then: "repositories is required"},
					},
				},
			},
			UsageExamples: []providers.Example{
				{
					Scenario: "Generate comprehensive vulnerability report",
					Input: map[string]interface{}{
						"action": "reports/vulnerability",
						"parameters": map[string]interface{}{
							"name":         "Q1-Security-Audit",
							"type":         "repository",
							"repositories": []string{"docker-prod", "npm-prod"},
							"format":       "pdf",
							"filters": map[string]interface{}{
								"severity": []string{"critical", "high"},
							},
						},
					},
					Explanation: "Creates a PDF vulnerability report for production repositories focusing on critical and high severity issues",
				},
				{
					Scenario: "Export SBOM for compliance",
					Input: map[string]interface{}{
						"action": "reports/sbom",
						"parameters": map[string]interface{}{
							"name":         "Production-SBOM-2025",
							"type":         "repository",
							"repositories": []string{"docker-prod-local"},
							"format":       "spdx",
						},
					},
					Explanation: "Generates a Software Bill of Materials in SPDX format for compliance requirements",
				},
				{
					Scenario: "Get security metrics dashboard",
					Input: map[string]interface{}{
						"action": "metrics/dashboard",
						"parameters": map[string]interface{}{
							"time_range":     "30d",
							"include_trends": true,
						},
					},
					Explanation: "Retrieves comprehensive security metrics for the last 30 days with trend analysis",
				},
			},
			ComplexityLevel: "moderate",
		},

		// System Operations
		{
			Name:        "xray_system",
			DisplayName: "System Health and Configuration",
			Category:    "system",
			Subcategory: "monitoring",
			Description: "System health monitoring, version information, and service status for Xray infrastructure management",
			DetailedHelp: `Xray system operations provide infrastructure monitoring capabilities:
- Health Checks: Verify service availability and readiness for load balancers
- Version Information: Get service version, build info, and API compatibility
- Service Status: Monitor database connectivity, license status, and resource usage
- Configuration: Basic system configuration and endpoint discovery

These operations are essential for monitoring, alerting, and maintaining Xray service health.`,
			SemanticTags: []string{
				"system", "health", "ping", "status", "version", "info", "monitoring",
				"service", "availability", "readiness", "infrastructure", "operational",
			},
			CommonPhrases: []string{
				"check Xray health", "is Xray running", "system status", "service health",
				"Xray version", "system info", "health check", "service availability",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "System operation to perform",
						Examples:    []interface{}{"system/ping", "system/version", "system/info"},
						Template:    "system/{operation}",
					},
				},
				Required: []string{"action"},
			},
			UsageExamples: []providers.Example{
				{
					Scenario: "Quick health check for monitoring",
					Input: map[string]interface{}{
						"action": "system/ping",
					},
					Explanation: "Simple health check that returns OK when Xray is healthy, suitable for load balancer probes",
				},
				{
					Scenario: "Get system version and build information",
					Input: map[string]interface{}{
						"action": "system/version",
					},
					Explanation: "Returns detailed version information for compatibility and support purposes",
				},
			},
			ComplexityLevel: "simple",
		},
	}
}

// GetXrayErrorResolutions provides resolution suggestions for common Xray errors
func GetXrayErrorResolutions() map[string]string {
	return map[string]string{
		"scan_in_progress":         "Scan is still running. Check status with scan/status operation",
		"policy_violation":         "Artifact violates configured policies. Review violations with violations/list",
		"no_xray_data":             "No Xray data available for this artifact. Trigger a scan first",
		"watch_not_found":          "Watch does not exist. Create it with watches/create or list existing with watches/list",
		"policy_not_found":         "Policy does not exist. Create it with policies/create or list existing with policies/list",
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
		"reporting":              "Generate comprehensive security and compliance reports",
		"component_intelligence": "Access detailed vulnerability data for specific components",
		"ignore_rules":           "Create rules to suppress false positives or accepted risks",
		"impact_analysis":        "Analyze the impact of vulnerabilities across your software supply chain",
	}
}

package tools

import (
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
)

// CapabilityInferencer infers tool capabilities from OpenAPI specs
// Helps AI agents understand what a tool can do at a high level
type CapabilityInferencer struct {
	logger observability.Logger
}

// NewCapabilityInferencer creates a new capability inferencer
func NewCapabilityInferencer(logger observability.Logger) *CapabilityInferencer {
	return &CapabilityInferencer{
		logger: logger,
	}
}

// ToolCapability describes a high-level capability of a tool
type ToolCapability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Operations  []string `json:"operations"` // Operation IDs that implement this capability
	Examples    []string `json:"examples"`   // Example use cases
	Keywords    []string `json:"keywords"`   // Keywords for AI matching
}

// InferCapabilities analyzes an OpenAPI spec to infer tool capabilities
func (c *CapabilityInferencer) InferCapabilities(spec *openapi3.T, toolName string) []ToolCapability {
	capabilities := []ToolCapability{}
	toolNameLower := strings.ToLower(toolName)

	// Tool-specific capability inference
	if strings.Contains(toolNameLower, "snyk") {
		capabilities = append(capabilities, c.inferSnykCapabilities(spec)...)
	} else if strings.Contains(toolNameLower, "github") {
		capabilities = append(capabilities, c.inferGitHubCapabilities(spec)...)
	} else {
		// Generic capability inference based on operations
		capabilities = append(capabilities, c.inferGenericCapabilities(spec)...)
	}

	return capabilities
}

// inferSnykCapabilities infers Snyk-specific capabilities
func (c *CapabilityInferencer) inferSnykCapabilities(spec *openapi3.T) []ToolCapability {
	caps := []ToolCapability{
		{
			Name:        "Vulnerability Scanning",
			Description: "Scan projects, containers, and infrastructure for security vulnerabilities",
			Operations:  c.findOperationsByPattern(spec, []string{"test", "scan", "analyze"}),
			Examples: []string{
				"Scan npm packages for known vulnerabilities",
				"Check Docker images for security issues",
				"Analyze infrastructure as code for misconfigurations",
			},
			Keywords: []string{"scan", "test", "vulnerability", "security", "CVE", "CVSS"},
		},
		{
			Name:        "Dependency Management",
			Description: "Monitor and manage project dependencies for security and licensing issues",
			Operations:  c.findOperationsByPattern(spec, []string{"dependencies", "deps", "packages"}),
			Examples: []string{
				"List all dependencies with vulnerabilities",
				"Check for outdated packages",
				"Identify license compliance issues",
			},
			Keywords: []string{"dependency", "package", "library", "module", "npm", "maven", "pip"},
		},
		{
			Name:        "Remediation",
			Description: "Get fix recommendations and apply automated remediations",
			Operations:  c.findOperationsByPattern(spec, []string{"fix", "remediat", "patch", "upgrade"}),
			Examples: []string{
				"Get upgrade recommendations for vulnerable packages",
				"Apply automated security patches",
				"Generate fix pull requests",
			},
			Keywords: []string{"fix", "remediate", "patch", "upgrade", "resolve", "solution"},
		},
		{
			Name:        "Monitoring & Alerts",
			Description: "Set up continuous monitoring and receive alerts for new vulnerabilities",
			Operations:  c.findOperationsByPattern(spec, []string{"monitor", "watch", "alert", "notif"}),
			Examples: []string{
				"Monitor projects for new vulnerabilities",
				"Set up email alerts for critical issues",
				"Configure webhook notifications",
			},
			Keywords: []string{"monitor", "watch", "alert", "notify", "continuous", "real-time"},
		},
		{
			Name:        "Reporting",
			Description: "Generate security reports and compliance documentation",
			Operations:  c.findOperationsByPattern(spec, []string{"report", "export", "summary"}),
			Examples: []string{
				"Generate vulnerability report for stakeholders",
				"Export SBOM (Software Bill of Materials)",
				"Create compliance audit reports",
			},
			Keywords: []string{"report", "export", "summary", "audit", "compliance", "SBOM"},
		},
		{
			Name:        "Policy Enforcement",
			Description: "Define and enforce security policies across projects",
			Operations:  c.findOperationsByPattern(spec, []string{"policy", "policies", "rule"}),
			Examples: []string{
				"Block deployments with critical vulnerabilities",
				"Enforce license compliance policies",
				"Set severity thresholds for CI/CD",
			},
			Keywords: []string{"policy", "rule", "enforce", "compliance", "governance", "threshold"},
		},
	}

	return caps
}

// inferGitHubCapabilities infers GitHub-specific capabilities
func (c *CapabilityInferencer) inferGitHubCapabilities(spec *openapi3.T) []ToolCapability {
	caps := []ToolCapability{
		{
			Name:        "Repository Management",
			Description: "Create, configure, and manage GitHub repositories",
			Operations:  c.findOperationsByPattern(spec, []string{"repos", "repository"}),
			Examples: []string{
				"Create new repositories",
				"Configure repository settings",
				"Manage repository permissions",
				"Archive or delete repositories",
			},
			Keywords: []string{"repository", "repo", "create", "settings", "permissions"},
		},
		{
			Name:        "Code Review",
			Description: "Manage pull requests and code review processes",
			Operations:  c.findOperationsByPattern(spec, []string{"pull", "pr", "review"}),
			Examples: []string{
				"Create and manage pull requests",
				"Request and submit reviews",
				"Approve or request changes",
				"Merge pull requests",
			},
			Keywords: []string{"pull request", "PR", "review", "merge", "approve", "diff"},
		},
		{
			Name:        "Issue Tracking",
			Description: "Create and manage issues for bug tracking and feature requests",
			Operations:  c.findOperationsByPattern(spec, []string{"issue", "bug", "feature"}),
			Examples: []string{
				"Create and update issues",
				"Assign issues to team members",
				"Label and categorize issues",
				"Track issue progress",
			},
			Keywords: []string{"issue", "bug", "feature", "ticket", "task", "tracking"},
		},
		{
			Name:        "CI/CD Integration",
			Description: "Manage GitHub Actions and workflow automation",
			Operations:  c.findOperationsByPattern(spec, []string{"action", "workflow", "run", "job"}),
			Examples: []string{
				"Trigger workflow runs",
				"Monitor build status",
				"Manage secrets and variables",
				"Configure deployment workflows",
			},
			Keywords: []string{"actions", "workflow", "CI/CD", "build", "deploy", "automation"},
		},
		{
			Name:        "Release Management",
			Description: "Create and manage releases and tags",
			Operations:  c.findOperationsByPattern(spec, []string{"release", "tag", "version"}),
			Examples: []string{
				"Create new releases",
				"Generate release notes",
				"Upload release assets",
				"Manage semantic versioning",
			},
			Keywords: []string{"release", "tag", "version", "deploy", "publish", "artifact"},
		},
		{
			Name:        "Team Collaboration",
			Description: "Manage teams, collaborators, and permissions",
			Operations:  c.findOperationsByPattern(spec, []string{"team", "collaborator", "member"}),
			Examples: []string{
				"Add collaborators to repositories",
				"Create and manage teams",
				"Set granular permissions",
				"Manage organization members",
			},
			Keywords: []string{"team", "collaborator", "permission", "access", "member", "organization"},
		},
	}

	return caps
}

// inferGenericCapabilities infers capabilities from generic API patterns
func (c *CapabilityInferencer) inferGenericCapabilities(spec *openapi3.T) []ToolCapability {
	caps := []ToolCapability{}

	// Analyze operations to determine capabilities
	hasCreate := false
	hasRead := false
	hasUpdate := false
	hasDelete := false
	hasSearch := false
	hasList := false

	createOps := []string{}
	readOps := []string{}
	updateOps := []string{}
	deleteOps := []string{}
	searchOps := []string{}
	listOps := []string{}

	if spec.Paths != nil {
		for path, pathItem := range spec.Paths.Map() {
			for method, operation := range pathItem.Operations() {
				if operation == nil {
					continue
				}

				opID := operation.OperationID
				if opID == "" {
					opID = fmt.Sprintf("%s_%s", method, path)
				}

				methodUpper := strings.ToUpper(method)
				pathLower := strings.ToLower(path)

				// Categorize operations
				switch methodUpper {
				case "POST":
					if strings.Contains(pathLower, "search") || strings.Contains(pathLower, "query") {
						hasSearch = true
						searchOps = append(searchOps, opID)
					} else {
						hasCreate = true
						createOps = append(createOps, opID)
					}
				case "GET":
					if strings.Contains(path, "{") {
						hasRead = true
						readOps = append(readOps, opID)
					} else {
						hasList = true
						listOps = append(listOps, opID)
					}
				case "PUT", "PATCH":
					hasUpdate = true
					updateOps = append(updateOps, opID)
				case "DELETE":
					hasDelete = true
					deleteOps = append(deleteOps, opID)
				}
			}
		}
	}

	// Generate capabilities based on available operations
	if hasCreate {
		caps = append(caps, ToolCapability{
			Name:        "Resource Creation",
			Description: "Create new resources in the system",
			Operations:  createOps,
			Examples: []string{
				"Create new records",
				"Add new entities",
				"Initialize resources",
			},
			Keywords: []string{"create", "add", "new", "insert", "post"},
		})
	}

	if hasRead || hasList {
		ops := append(readOps, listOps...)
		caps = append(caps, ToolCapability{
			Name:        "Data Retrieval",
			Description: "Retrieve and query data from the system",
			Operations:  ops,
			Examples: []string{
				"Get resource details",
				"List available resources",
				"Query specific data",
			},
			Keywords: []string{"get", "read", "retrieve", "fetch", "list", "query"},
		})
	}

	if hasUpdate {
		caps = append(caps, ToolCapability{
			Name:        "Resource Updates",
			Description: "Modify and update existing resources",
			Operations:  updateOps,
			Examples: []string{
				"Update resource properties",
				"Modify configurations",
				"Change settings",
			},
			Keywords: []string{"update", "modify", "edit", "patch", "change", "set"},
		})
	}

	if hasDelete {
		caps = append(caps, ToolCapability{
			Name:        "Resource Deletion",
			Description: "Remove resources from the system",
			Operations:  deleteOps,
			Examples: []string{
				"Delete resources",
				"Remove entities",
				"Clean up data",
			},
			Keywords: []string{"delete", "remove", "destroy", "purge", "clean"},
		})
	}

	if hasSearch {
		caps = append(caps, ToolCapability{
			Name:        "Search & Filtering",
			Description: "Search and filter resources based on criteria",
			Operations:  searchOps,
			Examples: []string{
				"Search by keywords",
				"Filter by attributes",
				"Advanced queries",
			},
			Keywords: []string{"search", "filter", "query", "find", "lookup"},
		})
	}

	return caps
}

// findOperationsByPattern finds operations matching name patterns
func (c *CapabilityInferencer) findOperationsByPattern(spec *openapi3.T, patterns []string) []string {
	operations := []string{}

	if spec.Paths == nil {
		return operations
	}

	for path, pathItem := range spec.Paths.Map() {
		for _, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			opID := operation.OperationID
			if opID == "" {
				continue
			}

			opIDLower := strings.ToLower(opID)
			pathLower := strings.ToLower(path)

			// Check if operation matches any pattern
			for _, pattern := range patterns {
				patternLower := strings.ToLower(pattern)
				if strings.Contains(opIDLower, patternLower) || strings.Contains(pathLower, patternLower) {
					operations = append(operations, opID)
					break
				}
			}
		}
	}

	return operations
}

// InferPrimaryCapability determines the primary capability of a tool
func (c *CapabilityInferencer) InferPrimaryCapability(capabilities []ToolCapability, toolName string) string {
	if len(capabilities) == 0 {
		return "API Operations"
	}

	toolNameLower := strings.ToLower(toolName)

	// Tool-specific primary capabilities
	if strings.Contains(toolNameLower, "snyk") || strings.Contains(toolNameLower, "security") {
		return "Security Vulnerability Management"
	}
	if strings.Contains(toolNameLower, "github") {
		return "Code Repository Management"
	}
	if strings.Contains(toolNameLower, "jira") {
		return "Project and Issue Management"
	}
	if strings.Contains(toolNameLower, "slack") {
		return "Team Communication"
	}
	if strings.Contains(toolNameLower, "aws") || strings.Contains(toolNameLower, "azure") {
		return "Cloud Infrastructure Management"
	}

	// Return the first capability as primary
	return capabilities[0].Name
}

// MatchCapabilityToIntent matches user intent to tool capabilities
func (c *CapabilityInferencer) MatchCapabilityToIntent(intent string, capabilities []ToolCapability) *ToolCapability {
	intentLower := strings.ToLower(intent)

	// Score each capability based on keyword matches
	bestScore := 0
	var bestCapability *ToolCapability

	for i, cap := range capabilities {
		score := 0

		// Check name match
		if strings.Contains(intentLower, strings.ToLower(cap.Name)) {
			score += 10
		}

		// Check keyword matches
		for _, keyword := range cap.Keywords {
			if strings.Contains(intentLower, strings.ToLower(keyword)) {
				score += 5
			}
		}

		// Check description match
		if strings.Contains(intentLower, strings.ToLower(cap.Description)) {
			score += 3
		}

		// Check example matches
		for _, example := range cap.Examples {
			if strings.Contains(strings.ToLower(example), intentLower) {
				score += 2
			}
		}

		if score > bestScore {
			bestScore = score
			bestCapability = &capabilities[i]
		}
	}

	return bestCapability
}

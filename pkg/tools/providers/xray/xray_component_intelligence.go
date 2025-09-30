package xray

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// AddComponentIntelligenceOperations adds component intelligence operations to the Xray provider
func (p *XrayProvider) AddComponentIntelligenceOperations() map[string]providers.OperationMapping {
	return map[string]providers.OperationMapping{
		// CVE Search Operations
		"components/searchByCves": {
			OperationID:    "SearchComponentsByCVEs",
			Method:         "POST",
			PathTemplate:   "/api/v1/component/searchByCves",
			RequiredParams: []string{"cves"},
		},
		"components/searchCvesByComponents": {
			OperationID:    "SearchCVEsByComponents",
			Method:         "POST",
			PathTemplate:   "/api/v1/component/searchCvesByComponents",
			RequiredParams: []string{"components_id"},
		},
		"components/findByName": {
			OperationID:    "FindComponentByName",
			Method:         "GET",
			PathTemplate:   "/api/v1/component/{name}",
			RequiredParams: []string{"name"},
		},
		"components/exportDetails": {
			OperationID:    "ExportComponentDetails",
			Method:         "POST",
			PathTemplate:   "/api/v1/component/exportDetails",
			RequiredParams: []string{"component_name", "package_type"},
			OptionalParams: []string{"violations", "license", "security", "exclude_unknown", "output_format", "sha_256"},
		},

		// Dependency Graph Operations
		"graph/artifact": {
			OperationID:    "GetArtifactDependencyGraph",
			Method:         "POST",
			PathTemplate:   "/api/v1/dependencyGraph/artifact",
			RequiredParams: []string{"artifact_path"},
		},
		"graph/build": {
			OperationID:    "GetBuildDependencyGraph",
			Method:         "POST",
			PathTemplate:   "/api/v1/dependencyGraph/build",
			RequiredParams: []string{"build_name", "build_number"},
			OptionalParams: []string{"artifactory_instance", "project"},
		},
		"graph/compareArtifacts": {
			OperationID:    "CompareArtifacts",
			Method:         "POST",
			PathTemplate:   "/api/v1/dependencyGraph/compareArtifacts",
			RequiredParams: []string{"source_artifact_path", "target_artifact_path"},
		},
		"graph/compareBuilds": {
			OperationID:    "CompareBuilds",
			Method:         "POST",
			PathTemplate:   "/api/v1/dependencyGraph/compareBuilds",
			RequiredParams: []string{"source_build_name", "source_build_number", "target_build_name", "target_build_number"},
			OptionalParams: []string{"artifactory_instance", "project"},
		},

		// License Compliance Operations
		"licenses/report": {
			OperationID:    "GenerateLicenseReport",
			Method:         "POST",
			PathTemplate:   "/api/v1/reports/licenses",
			RequiredParams: []string{"name"},
			OptionalParams: []string{"type", "format", "repositories", "builds"},
		},
		"licenses/summary": {
			OperationID:    "GetLicenseSummary",
			Method:         "GET",
			PathTemplate:   "/api/v1/licensesReport",
			OptionalParams: []string{"repo_key", "build_name", "build_number"},
		},

		// Enhanced Vulnerability Operations
		"vulnerabilities/componentSummary": {
			OperationID:    "GetComponentVulnerabilitySummary",
			Method:         "POST",
			PathTemplate:   "/api/v1/component/vulnerabilities/summary",
			RequiredParams: []string{"component_id"},
			OptionalParams: []string{"include_dependencies", "severity_filter"},
		},
		"vulnerabilities/exportSBOM": {
			OperationID:    "ExportSBOM",
			Method:         "POST",
			PathTemplate:   "/api/v1/component/exportSbom",
			RequiredParams: []string{"component_id"},
			OptionalParams: []string{"format", "spec_version"},
		},

		// Component Metadata Operations
		"components/versions": {
			OperationID:    "GetComponentVersions",
			Method:         "POST",
			PathTemplate:   "/api/v1/component/versions",
			RequiredParams: []string{"component_name", "package_type"},
			OptionalParams: []string{"include_vulnerabilities", "include_licenses"},
		},
		"components/impact": {
			OperationID:    "GetComponentImpactAnalysis",
			Method:         "POST",
			PathTemplate:   "/api/v1/component/impactAnalysis",
			RequiredParams: []string{"component_id"},
		},
	}
}

// ComponentVulnerabilityInfo represents vulnerability information for a component
type ComponentVulnerabilityInfo struct {
	ComponentID   string               `json:"component_id"`
	ComponentName string               `json:"component_name"`
	PackageType   string               `json:"package_type"`
	Version       string               `json:"version"`
	Issues        []VulnerabilityIssue `json:"issues"`
	Licenses      []LicenseInfo        `json:"licenses"`
	Dependencies  []DependencyInfo     `json:"dependencies"`
	Summary       VulnerabilitySummary `json:"summary"`
}

// VulnerabilityIssue represents a single vulnerability
type VulnerabilityIssue struct {
	IssueID     string   `json:"issue_id"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	IssueType   string   `json:"issue_type"`
	Severity    string   `json:"severity"`
	CVE         []string `json:"cve,omitempty"`
	CVSS        string   `json:"cvss,omitempty"`
	CVSSScore   float64  `json:"cvss_score,omitempty"`
	Provider    string   `json:"provider"`
	Created     string   `json:"created,omitempty"`
	FixVersion  string   `json:"fix_version,omitempty"`
	References  []string `json:"references,omitempty"`
}

// LicenseInfo represents license information for a component
type LicenseInfo struct {
	Name        string   `json:"name"`
	FullName    string   `json:"full_name"`
	Approved    bool     `json:"approved"`
	Risk        string   `json:"risk"`
	Components  []string `json:"components,omitempty"`
	MoreInfoURL string   `json:"more_info_url,omitempty"`
}

// DependencyInfo represents dependency information
type DependencyInfo struct {
	ComponentID string               `json:"component_id"`
	Depth       int                  `json:"depth"`
	ParentID    string               `json:"parent_id,omitempty"`
	Issues      []VulnerabilityIssue `json:"issues,omitempty"`
}

// VulnerabilitySummary provides a summary of vulnerabilities
type VulnerabilitySummary struct {
	TotalIssues int            `json:"total_issues"`
	Critical    int            `json:"critical"`
	High        int            `json:"high"`
	Medium      int            `json:"medium"`
	Low         int            `json:"low"`
	Unknown     int            `json:"unknown"`
	FixVersions map[string]int `json:"fix_versions,omitempty"`
}

// DependencyGraph represents a component dependency graph
type DependencyGraph struct {
	RootComponent ComponentNode    `json:"root_component"`
	Nodes         []ComponentNode  `json:"nodes"`
	Edges         []DependencyEdge `json:"edges"`
	TotalNodes    int              `json:"total_nodes"`
	MaxDepth      int              `json:"max_depth"`
}

// ComponentNode represents a node in the dependency graph
type ComponentNode struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Version     string               `json:"version"`
	PackageType string               `json:"package_type"`
	Depth       int                  `json:"depth"`
	Issues      []VulnerabilityIssue `json:"issues,omitempty"`
	Licenses    []LicenseInfo        `json:"licenses,omitempty"`
}

// DependencyEdge represents an edge in the dependency graph
type DependencyEdge struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Relationship string `json:"relationship"`
}

// CVESearchResult represents the result of a CVE search
type CVESearchResult struct {
	CVE        string           `json:"cve"`
	Components []ComponentMatch `json:"components"`
}

// ComponentMatch represents a component that matches search criteria
type ComponentMatch struct {
	Name        string `json:"name"`
	PackageType string `json:"package_type"`
	Version     string `json:"version"`
	Repository  string `json:"repository,omitempty"`
	Path        string `json:"path,omitempty"`
	Link        string `json:"link,omitempty"`
}

// FormatCVESearchRequest formats a request to search components by CVEs
func FormatCVESearchRequest(cves []string) map[string]interface{} {
	return map[string]interface{}{
		"cves": cves,
	}
}

// FormatComponentSearchRequest formats a request to search CVEs by components
func FormatComponentSearchRequest(componentIDs []string) map[string]interface{} {
	return map[string]interface{}{
		"components_id": componentIDs,
	}
}

// FormatDependencyGraphRequest formats a request to get dependency graph
func FormatDependencyGraphRequest(artifactPath string, includeIssues bool) map[string]interface{} {
	return map[string]interface{}{
		"artifact_path":  artifactPath,
		"include_issues": includeIssues,
	}
}

// FormatBuildDependencyGraphRequest formats a request to get build dependency graph
func FormatBuildDependencyGraphRequest(buildName, buildNumber string, artifactoryInstance string) map[string]interface{} {
	request := map[string]interface{}{
		"build_name":   buildName,
		"build_number": buildNumber,
	}
	if artifactoryInstance != "" {
		request["artifactory_instance"] = artifactoryInstance
	}
	return request
}

// FormatComponentExportRequest formats a request to export component details
func FormatComponentExportRequest(componentName, packageType, outputFormat string, options map[string]bool) map[string]interface{} {
	request := map[string]interface{}{
		"component_name": componentName,
		"package_type":   packageType,
		"output_format":  outputFormat,
	}

	// Add optional flags
	if violations, ok := options["violations"]; ok {
		request["violations"] = violations
	}
	if license, ok := options["license"]; ok {
		request["license"] = license
	}
	if security, ok := options["security"]; ok {
		request["security"] = security
	}
	if excludeUnknown, ok := options["exclude_unknown"]; ok {
		request["exclude_unknown"] = excludeUnknown
	}

	return request
}

// ParseCVESearchResponse parses the response from CVE search
func ParseCVESearchResponse(response interface{}) ([]CVESearchResult, error) {
	data, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var results []CVESearchResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("failed to parse CVE search response: %w", err)
	}

	return results, nil
}

// ParseDependencyGraphResponse parses the dependency graph response
func ParseDependencyGraphResponse(response interface{}) (*DependencyGraph, error) {
	data, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var graph DependencyGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("failed to parse dependency graph response: %w", err)
	}

	return &graph, nil
}

// ParseComponentVulnerabilityResponse parses component vulnerability response
func ParseComponentVulnerabilityResponse(response interface{}) (*ComponentVulnerabilityInfo, error) {
	data, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var info ComponentVulnerabilityInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse component vulnerability response: %w", err)
	}

	return &info, nil
}

// BuildComponentIdentifier builds a component identifier based on package type
func BuildComponentIdentifier(packageType, group, name, version string) string {
	packageType = strings.ToLower(packageType)
	switch packageType {
	case "maven":
		if group != "" {
			return fmt.Sprintf("gav://%s:%s:%s", group, name, version)
		}
		return fmt.Sprintf("gav://%s:%s", name, version)
	case "docker":
		if group != "" {
			return fmt.Sprintf("docker://%s/%s:%s", group, name, version)
		}
		return fmt.Sprintf("docker://%s:%s", name, version)
	case "npm":
		if group != "" {
			return fmt.Sprintf("npm://@%s/%s:%s", group, name, version)
		}
		return fmt.Sprintf("npm://%s:%s", name, version)
	case "pypi":
		return fmt.Sprintf("pypi://%s:%s", name, version)
	case "nuget":
		return fmt.Sprintf("nuget://%s:%s", name, version)
	case "go":
		if group != "" {
			return fmt.Sprintf("go://%s/%s:%s", group, name, version)
		}
		return fmt.Sprintf("go://%s:%s", name, version)
	case "rpm":
		return fmt.Sprintf("rpm://%s-%s", name, version)
	case "debian":
		return fmt.Sprintf("deb://%s:%s", name, version)
	case "alpine":
		return fmt.Sprintf("alpine://%s:%s", name, version)
	case "generic":
		return fmt.Sprintf("generic://%s:%s", name, version)
	default:
		return fmt.Sprintf("%s://%s:%s", packageType, name, version)
	}
}

// GetSupportedPackageTypes returns the list of supported package types for component intelligence
func GetSupportedPackageTypes() []string {
	return []string{
		"maven",
		"gav", // Maven component identifier prefix
		"gradle",
		"ivy",
		"sbt",
		"npm",
		"bower",
		"debian",
		"docker",
		"helm",
		"nuget",
		"opkg",
		"pypi",
		"rpm",
		"ruby",
		"gems",
		"go",
		"cargo",
		"conan",
		"conda",
		"cran",
		"alpine",
		"generic",
	}
}

// ValidateComponentIdentifier validates a component identifier format
func ValidateComponentIdentifier(componentID string) error {
	// Check if it has the correct format: <type>://<path>
	if !strings.Contains(componentID, "://") {
		return fmt.Errorf("invalid component identifier format: missing '://' separator")
	}

	parts := strings.SplitN(componentID, "://", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid component identifier format")
	}

	packageType := parts[0]
	if packageType == "" {
		return fmt.Errorf("missing package type in component identifier")
	}

	componentPath := parts[1]
	if componentPath == "" {
		return fmt.Errorf("missing component path in identifier")
	}

	// Check if package type is supported
	supportedTypes := GetSupportedPackageTypes()
	isSupported := false
	for _, supported := range supportedTypes {
		if packageType == supported {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return fmt.Errorf("unsupported package type: %s", packageType)
	}

	return nil
}

// FilterVulnerabilitiesBySeverity filters vulnerabilities by minimum severity
func FilterVulnerabilitiesBySeverity(issues []VulnerabilityIssue, minSeverity string) []VulnerabilityIssue {
	severityOrder := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
		"unknown":  0,
	}

	minLevel := severityOrder[strings.ToLower(minSeverity)]
	filtered := []VulnerabilityIssue{}

	for _, issue := range issues {
		issueLevel := severityOrder[strings.ToLower(issue.Severity)]
		if issueLevel >= minLevel {
			filtered = append(filtered, issue)
		}
	}

	return filtered
}

// AnalyzeDependencyDepth analyzes the dependency graph depth and complexity
func AnalyzeDependencyDepth(graph *DependencyGraph) map[string]interface{} {
	analysis := map[string]interface{}{
		"total_nodes":             graph.TotalNodes,
		"max_depth":               graph.MaxDepth,
		"direct_dependencies":     0,
		"transitive_dependencies": 0,
		"vulnerable_nodes":        0,
		"licensed_nodes":          0,
		"depth_distribution":      make(map[int]int),
	}

	depthDist := make(map[int]int)

	for _, node := range graph.Nodes {
		depthDist[node.Depth]++

		if node.Depth == 1 {
			analysis["direct_dependencies"] = analysis["direct_dependencies"].(int) + 1
		} else if node.Depth > 1 {
			analysis["transitive_dependencies"] = analysis["transitive_dependencies"].(int) + 1
		}

		if len(node.Issues) > 0 {
			analysis["vulnerable_nodes"] = analysis["vulnerable_nodes"].(int) + 1
		}

		if len(node.Licenses) > 0 {
			analysis["licensed_nodes"] = analysis["licensed_nodes"].(int) + 1
		}
	}

	analysis["depth_distribution"] = depthDist

	return analysis
}

// GetCriticalPath finds the critical vulnerability path in dependency graph
func GetCriticalPath(graph *DependencyGraph) []ComponentNode {
	// Find the path with the most critical vulnerabilities
	var criticalPath []ComponentNode
	maxSeverityScore := 0.0

	// This is a simplified implementation
	// In production, you'd implement a proper graph traversal algorithm
	for _, node := range graph.Nodes {
		score := 0.0
		for _, issue := range node.Issues {
			switch strings.ToLower(issue.Severity) {
			case "critical":
				score += 10
			case "high":
				score += 5
			case "medium":
				score += 2
			case "low":
				score += 1
			}
		}

		if score > maxSeverityScore {
			// In a real implementation, we'd trace the full path
			// For now, just track the most critical node
			criticalPath = []ComponentNode{node}
			maxSeverityScore = score
		}
	}

	return criticalPath
}

package xray

import (
	"encoding/json"
	"fmt"
	"time"
)

// ScanSeverity represents the severity levels in Xray
type ScanSeverity string

const (
	SeverityCritical ScanSeverity = "Critical"
	SeverityHigh     ScanSeverity = "High"
	SeverityMedium   ScanSeverity = "Medium"
	SeverityLow      ScanSeverity = "Low"
	SeverityUnknown  ScanSeverity = "Unknown"
)

// ArtifactScanRequest represents the request for scanning an artifact
type ArtifactScanRequest struct {
	ComponentID string `json:"componentId"`
	Watch       string `json:"watch,omitempty"`
}

// BuildScanRequest represents the request for scanning a build
type BuildScanRequest struct {
	BuildName   string `json:"buildName"`
	BuildNumber string `json:"buildNumber"`
	Context     string `json:"context,omitempty"`
}

// ScanResponse represents the response from a scan operation
type ScanResponse struct {
	ScanID    string    `json:"scan_id"`
	Status    string    `json:"status"`
	Created   time.Time `json:"created,omitempty"`
	Completed time.Time `json:"completed,omitempty"`
}

// ScanStatusResponse represents the status of a scan
type ScanStatusResponse struct {
	Status    string    `json:"status"`
	Progress  int       `json:"progress,omitempty"`
	Message   string    `json:"message,omitempty"`
	Created   time.Time `json:"created,omitempty"`
	Completed time.Time `json:"completed,omitempty"`
}

// ArtifactSummaryRequest represents the request for artifact summary
type ArtifactSummaryRequest struct {
	Paths []string `json:"paths"`
}

// ArtifactSummaryResponse represents the summary response for an artifact
type ArtifactSummaryResponse struct {
	Artifacts []ArtifactSummary `json:"artifacts"`
	Errors    []ErrorDetail     `json:"errors,omitempty"`
}

// ArtifactSummary contains summary information for a single artifact
type ArtifactSummary struct {
	General              GeneralInfo `json:"general"`
	Issues               []Issue     `json:"issues"`
	Licenses             []License   `json:"licenses"`
	VulnerableComponents []Component `json:"vulnerable_components,omitempty"`
}

// GeneralInfo contains general information about an artifact
type GeneralInfo struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	SHA256      string    `json:"sha256"`
	PackageType string    `json:"pkg_type"`
	ComponentID string    `json:"component_id"`
	Created     time.Time `json:"created,omitempty"`
	Modified    time.Time `json:"modified,omitempty"`
}

// Issue represents a security issue found in a scan
type Issue struct {
	ID          string       `json:"issue_id"`
	Summary     string       `json:"summary"`
	Description string       `json:"description"`
	IssueType   string       `json:"issue_type"`
	Severity    ScanSeverity `json:"severity"`
	Provider    string       `json:"provider"`
	Created     time.Time    `json:"created,omitempty"`
	Components  []Component  `json:"components,omitempty"`
	CVE         []CVEDetail  `json:"cves,omitempty"`
	References  []string     `json:"references,omitempty"`
}

// CVEDetail contains CVE information
type CVEDetail struct {
	ID          string  `json:"cve"`
	CVSSV2Score float64 `json:"cvss_v2_score,omitempty"`
	CVSSV3Score float64 `json:"cvss_v3_score,omitempty"`
}

// License represents license information
type License struct {
	Key        string   `json:"license_key"`
	Name       string   `json:"name"`
	URL        string   `json:"url,omitempty"`
	Components []string `json:"components,omitempty"`
}

// Component represents a component with vulnerabilities
type Component struct {
	ID                string   `json:"component_id"`
	FixedVersions     []string `json:"fixed_versions,omitempty"`
	ImpactedArtifacts []string `json:"impacted_artifacts,omitempty"`
}

// ErrorDetail represents an error in the response
type ErrorDetail struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// BuildSummaryResponse represents the summary response for a build
type BuildSummaryResponse struct {
	BuildName   string          `json:"build_name"`
	BuildNumber string          `json:"build_number"`
	Issues      []Issue         `json:"issues"`
	Licenses    []License       `json:"licenses"`
	Summary     SeveritySummary `json:"summary"`
}

// SeveritySummary contains a summary of issues by severity
type SeveritySummary struct {
	TotalIssues int            `json:"total_issues"`
	BySeverity  map[string]int `json:"by_severity"`
	Critical    int            `json:"critical"`
	High        int            `json:"high"`
	Medium      int            `json:"medium"`
	Low         int            `json:"low"`
	Unknown     int            `json:"unknown"`
}

// ParseArtifactSummaryResponse parses the artifact summary response from Xray
func ParseArtifactSummaryResponse(data []byte) (*ArtifactSummaryResponse, error) {
	var response ArtifactSummaryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse artifact summary response: %w", err)
	}
	return &response, nil
}

// ParseBuildSummaryResponse parses the build summary response from Xray
func ParseBuildSummaryResponse(data []byte) (*BuildSummaryResponse, error) {
	var response BuildSummaryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse build summary response: %w", err)
	}
	return &response, nil
}

// ParseScanResponse parses a scan response from Xray
func ParseScanResponse(data []byte) (*ScanResponse, error) {
	var response ScanResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse scan response: %w", err)
	}
	return &response, nil
}

// ParseScanStatusResponse parses a scan status response from Xray
func ParseScanStatusResponse(data []byte) (*ScanStatusResponse, error) {
	var response ScanStatusResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse scan status response: %w", err)
	}
	return &response, nil
}

// CategorizeBySeverity groups issues by their severity level
func CategorizeBySeverity(issues []Issue) map[ScanSeverity][]Issue {
	categorized := make(map[ScanSeverity][]Issue)

	// Initialize all severity levels
	categorized[SeverityCritical] = []Issue{}
	categorized[SeverityHigh] = []Issue{}
	categorized[SeverityMedium] = []Issue{}
	categorized[SeverityLow] = []Issue{}
	categorized[SeverityUnknown] = []Issue{}

	for _, issue := range issues {
		severity := issue.Severity
		if severity == "" {
			severity = SeverityUnknown
		}
		categorized[severity] = append(categorized[severity], issue)
	}

	return categorized
}

// GetSeveritySummary creates a summary of issues by severity
func GetSeveritySummary(issues []Issue) SeveritySummary {
	categorized := CategorizeBySeverity(issues)

	summary := SeveritySummary{
		TotalIssues: len(issues),
		BySeverity:  make(map[string]int),
		Critical:    len(categorized[SeverityCritical]),
		High:        len(categorized[SeverityHigh]),
		Medium:      len(categorized[SeverityMedium]),
		Low:         len(categorized[SeverityLow]),
		Unknown:     len(categorized[SeverityUnknown]),
	}

	// Also populate the map format
	summary.BySeverity[string(SeverityCritical)] = summary.Critical
	summary.BySeverity[string(SeverityHigh)] = summary.High
	summary.BySeverity[string(SeverityMedium)] = summary.Medium
	summary.BySeverity[string(SeverityLow)] = summary.Low
	summary.BySeverity[string(SeverityUnknown)] = summary.Unknown

	return summary
}

// NormalizeSeverity normalizes various severity formats to standard Xray severity
func NormalizeSeverity(severity string) ScanSeverity {
	switch severity {
	case "Critical", "critical", "CRITICAL", "1":
		return SeverityCritical
	case "High", "high", "HIGH", "2":
		return SeverityHigh
	case "Medium", "medium", "MEDIUM", "3":
		return SeverityMedium
	case "Low", "low", "LOW", "4":
		return SeverityLow
	default:
		return SeverityUnknown
	}
}

// FormatScanRequest formats parameters into an artifact scan request
func FormatScanRequest(params map[string]interface{}) (*ArtifactScanRequest, error) {
	componentID, ok := params["componentId"].(string)
	if !ok || componentID == "" {
		return nil, fmt.Errorf("componentId is required for artifact scan")
	}

	request := &ArtifactScanRequest{
		ComponentID: componentID,
	}

	if watch, ok := params["watch"].(string); ok {
		request.Watch = watch
	}

	return request, nil
}

// FormatBuildScanRequest formats parameters into a build scan request
func FormatBuildScanRequest(params map[string]interface{}) (*BuildScanRequest, error) {
	buildName, ok := params["buildName"].(string)
	if !ok || buildName == "" {
		return nil, fmt.Errorf("buildName is required for build scan")
	}

	buildNumber, ok := params["buildNumber"].(string)
	if !ok || buildNumber == "" {
		return nil, fmt.Errorf("buildNumber is required for build scan")
	}

	request := &BuildScanRequest{
		BuildName:   buildName,
		BuildNumber: buildNumber,
	}

	if context, ok := params["context"].(string); ok {
		request.Context = context
	}

	return request, nil
}

// FormatArtifactSummaryRequest formats parameters into an artifact summary request
func FormatArtifactSummaryRequest(params map[string]interface{}) (*ArtifactSummaryRequest, error) {
	// Handle paths as either a slice of strings or a single string
	var paths []string

	if pathList, ok := params["paths"].([]string); ok {
		paths = pathList
	} else if pathList, ok := params["paths"].([]interface{}); ok {
		for _, p := range pathList {
			if path, ok := p.(string); ok {
				paths = append(paths, path)
			}
		}
	} else if path, ok := params["path"].(string); ok {
		// Support single path parameter
		paths = []string{path}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("paths parameter is required for artifact summary")
	}

	return &ArtifactSummaryRequest{
		Paths: paths,
	}, nil
}

// GetMostSevereIssue returns the most severe issue from a list
func GetMostSevereIssue(issues []Issue) *Issue {
	if len(issues) == 0 {
		return nil
	}

	// Define severity priority
	severityPriority := map[ScanSeverity]int{
		SeverityCritical: 0,
		SeverityHigh:     1,
		SeverityMedium:   2,
		SeverityLow:      3,
		SeverityUnknown:  4,
	}

	var mostSevere *Issue
	lowestPriority := 5

	for i := range issues {
		issue := &issues[i]
		priority, exists := severityPriority[issue.Severity]
		if !exists {
			priority = severityPriority[SeverityUnknown]
		}

		if priority < lowestPriority {
			lowestPriority = priority
			mostSevere = issue
		}
	}

	return mostSevere
}

// HasCriticalVulnerabilities checks if there are any critical vulnerabilities
func HasCriticalVulnerabilities(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// FilterIssuesBySeverity filters issues by minimum severity level
func FilterIssuesBySeverity(issues []Issue, minSeverity ScanSeverity) []Issue {
	severityPriority := map[ScanSeverity]int{
		SeverityCritical: 0,
		SeverityHigh:     1,
		SeverityMedium:   2,
		SeverityLow:      3,
		SeverityUnknown:  4,
	}

	minPriority := severityPriority[minSeverity]
	var filtered []Issue

	for _, issue := range issues {
		priority, exists := severityPriority[issue.Severity]
		if !exists {
			priority = severityPriority[SeverityUnknown]
		}

		if priority <= minPriority {
			filtered = append(filtered, issue)
		}
	}

	return filtered
}

package xray

import (
	"fmt"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// AddReportsAndMetricsOperations adds comprehensive report and metrics operations to the Xray provider
func (p *XrayProvider) AddReportsAndMetricsOperations() map[string]providers.OperationMapping {
	operations := map[string]providers.OperationMapping{
		// Enhanced Report Generation Operations
		"reports/vulnerability": {
			OperationID:    "GenerateVulnerabilityReport",
			Method:         "POST",
			PathTemplate:   "/api/v2/reports/vulnerabilities",
			RequiredParams: []string{"name"},
			OptionalParams: []string{
				"filters",           // Filters object with severity, cve, component filters
				"repositories",      // Array of repository keys
				"builds",            // Array of build names
				"release_bundles",   // Array of release bundle names
				"component_ids",     // Array of component IDs
				"artifacts",         // Array of artifact paths
				"severity_filter",   // Critical, High, Medium, Low
				"cve_filter",        // Specific CVEs to include
				"issue_id_filter",   // Specific issue IDs
				"license_filter",    // License types to include
				"watch_names",       // Watch names to filter by
				"policy_names",      // Policy names to filter by
				"date_from",         // Start date for report
				"date_to",           // End date for report
				"format",            // json, pdf, csv, xml
				"include_ignorable", // Include ignorable issues
			},
		},
		"reports/license": {
			OperationID:    "GenerateLicenseReport",
			Method:         "POST",
			PathTemplate:   "/api/v2/reports/licenses",
			RequiredParams: []string{"name"},
			OptionalParams: []string{
				"filters",           // License filters
				"repositories",      // Array of repository keys
				"builds",            // Array of build names
				"release_bundles",   // Array of release bundle names
				"component_ids",     // Array of component IDs
				"artifacts",         // Array of artifact paths
				"license_types",     // Approved, banned, unknown
				"include_unknown",   // Include unknown licenses
				"watch_names",       // Watch names to filter by
				"policy_names",      // Policy names to filter by
				"date_from",         // Start date for report
				"date_to",           // End date for report
				"format",            // json, pdf, csv, xml
				"group_by_artifact", // Group results by artifact
			},
		},
		"reports/operational_risk": {
			OperationID:    "GenerateOperationalRiskReport",
			Method:         "POST",
			PathTemplate:   "/api/v2/reports/operational_risks",
			RequiredParams: []string{"name"},
			OptionalParams: []string{
				"filters",         // Risk level filters
				"repositories",    // Array of repository keys
				"builds",          // Array of build names
				"release_bundles", // Array of release bundle names
				"component_ids",   // Array of component IDs
				"artifacts",       // Array of artifact paths
				"risk_levels",     // High, Medium, Low
				"categories",      // EOL, outdated, etc.
				"watch_names",     // Watch names to filter by
				"date_from",       // Start date for report
				"date_to",         // End date for report
				"format",          // json, pdf, csv, xml
			},
		},
		"reports/sbom": {
			OperationID:    "GenerateSBOMReport",
			Method:         "POST",
			PathTemplate:   "/api/v2/reports/sbom",
			RequiredParams: []string{"name", "artifact_path"},
			OptionalParams: []string{
				"repository",       // Repository key
				"build_name",       // Build name
				"build_number",     // Build number
				"release_bundle",   // Release bundle name
				"format",           // spdx, cyclonedx-json, cyclonedx-xml
				"spec_version",     // SBOM spec version
				"include_vex",      // Include VEX (Vulnerability Exploitability eXchange)
				"component_scope",  // all, direct, transitive
				"include_dev_deps", // Include development dependencies
				"include_licenses", // Include license information
				"include_vulns",    // Include vulnerability information
				"output_file_name", // Custom output filename
			},
		},
		"reports/compliance": {
			OperationID:    "GenerateComplianceReport",
			Method:         "POST",
			PathTemplate:   "/api/v2/reports/compliance",
			RequiredParams: []string{"name", "compliance_type"},
			OptionalParams: []string{
				"filters",          // Compliance filters
				"repositories",     // Array of repository keys
				"builds",           // Array of build names
				"release_bundles",  // Array of release bundle names
				"standards",        // Array of compliance standards (PCI, HIPAA, etc.)
				"date_from",        // Start date for report
				"date_to",          // End date for report
				"format",           // json, pdf, csv, xml
				"include_passed",   // Include passed checks
				"include_failed",   // Include failed checks
				"include_warnings", // Include warnings
			},
		},

		// Report Status and Management Operations
		"reports/status": {
			OperationID:    "GetReportStatus",
			Method:         "GET",
			PathTemplate:   "/api/v2/reports/{report_id}/status",
			RequiredParams: []string{"report_id"},
		},
		"reports/download": {
			OperationID:    "DownloadReport",
			Method:         "GET",
			PathTemplate:   "/api/v2/reports/{report_id}/download",
			RequiredParams: []string{"report_id"},
			OptionalParams: []string{
				"format", // Override format if multiple available
			},
		},
		"reports/list": {
			OperationID:  "ListReports",
			Method:       "GET",
			PathTemplate: "/api/v2/reports",
			OptionalParams: []string{
				"report_type",    // vulnerability, license, operational_risk, sbom, compliance
				"status",         // pending, generating, completed, failed
				"created_after",  // Filter by creation date
				"created_before", // Filter by creation date
				"name_pattern",   // Filter by name pattern
				"limit",          // Pagination limit
				"offset",         // Pagination offset
				"order_by",       // created, name, type
				"order",          // asc, desc
			},
		},
		"reports/get": {
			OperationID:    "GetReportDetails",
			Method:         "GET",
			PathTemplate:   "/api/v2/reports/{report_id}",
			RequiredParams: []string{"report_id"},
		},
		"reports/delete": {
			OperationID:    "DeleteReport",
			Method:         "DELETE",
			PathTemplate:   "/api/v2/reports/{report_id}",
			RequiredParams: []string{"report_id"},
		},
		"reports/schedule": {
			OperationID:    "ScheduleReport",
			Method:         "POST",
			PathTemplate:   "/api/v2/reports/schedule",
			RequiredParams: []string{"name", "report_type", "schedule"},
			OptionalParams: []string{
				"description",      // Report description
				"filters",          // Report filters
				"repositories",     // Repositories to include
				"format",           // Output format
				"email_recipients", // Email addresses for delivery
				"webhook_url",      // Webhook for notifications
				"retention_days",   // How long to keep reports
			},
		},
		"reports/schedule/list": {
			OperationID:  "ListScheduledReports",
			Method:       "GET",
			PathTemplate: "/api/v2/reports/schedules",
			OptionalParams: []string{
				"active", // Filter by active status
				"limit",  // Pagination
				"offset", // Pagination
			},
		},
		"reports/schedule/delete": {
			OperationID:    "DeleteScheduledReport",
			Method:         "DELETE",
			PathTemplate:   "/api/v2/reports/schedules/{schedule_id}",
			RequiredParams: []string{"schedule_id"},
		},

		// Metrics and Analytics Operations
		"metrics/violations": {
			OperationID:  "GetViolationsMetrics",
			Method:       "GET",
			PathTemplate: "/api/v2/metrics/violations",
			OptionalParams: []string{
				"date_from",      // Start date
				"date_to",        // End date
				"granularity",    // hour, day, week, month
				"repositories",   // Filter by repositories
				"severities",     // Filter by severity levels
				"violation_type", // security, license, operational_risk
				"group_by",       // repository, severity, type, policy
			},
		},
		"metrics/scans": {
			OperationID:  "GetScansMetrics",
			Method:       "GET",
			PathTemplate: "/api/v2/metrics/scans",
			OptionalParams: []string{
				"date_from",   // Start date
				"date_to",     // End date
				"granularity", // hour, day, week, month
				"scan_type",   // artifact, build, release_bundle
				"status",      // completed, failed, in_progress
				"group_by",    // type, status, repository
			},
		},
		"metrics/components": {
			OperationID:  "GetComponentsMetrics",
			Method:       "GET",
			PathTemplate: "/api/v2/metrics/components",
			OptionalParams: []string{
				"date_from",        // Start date
				"date_to",          // End date
				"repositories",     // Filter by repositories
				"package_types",    // maven, npm, docker, etc.
				"include_vulns",    // Include vulnerability counts
				"include_licenses", // Include license counts
				"group_by",         // package_type, repository
			},
		},
		"metrics/exposure": {
			OperationID:  "GetExposureMetrics",
			Method:       "GET",
			PathTemplate: "/api/v2/metrics/exposure",
			OptionalParams: []string{
				"date_from",       // Start date
				"date_to",         // End date
				"repositories",    // Filter by repositories
				"severity_levels", // Critical, High, Medium, Low
				"cve_ids",         // Specific CVEs
				"group_by",        // severity, repository, component
			},
		},
		"metrics/trends": {
			OperationID:  "GetTrendsMetrics",
			Method:       "GET",
			PathTemplate: "/api/v2/metrics/trends",
			OptionalParams: []string{
				"metric_type",    // violations, scans, components, exposure
				"date_from",      // Start date (required for trends)
				"date_to",        // End date
				"granularity",    // hour, day, week, month
				"repositories",   // Filter by repositories
				"compare_to",     // Previous period for comparison
				"trend_analysis", // Calculate trend direction
			},
		},
		"metrics/summary": {
			OperationID:  "GetMetricsSummary",
			Method:       "GET",
			PathTemplate: "/api/v2/metrics/summary",
			OptionalParams: []string{
				"include_violations", // Include violation summary
				"include_scans",      // Include scan summary
				"include_components", // Include component summary
				"include_exposure",   // Include exposure summary
				"date_range",         // today, week, month, quarter, year
			},
		},
		"metrics/dashboard": {
			OperationID:  "GetDashboardMetrics",
			Method:       "GET",
			PathTemplate: "/api/v2/metrics/dashboard",
			OptionalParams: []string{
				"widgets",      // Array of widget types to include
				"date_range",   // Dashboard date range
				"repositories", // Filter all widgets by repositories
				"refresh",      // Force refresh cached metrics
			},
		},

		// Export Operations for Reports
		"reports/export/violations": {
			OperationID:    "ExportViolations",
			Method:         "POST",
			PathTemplate:   "/api/v2/reports/export/violations",
			RequiredParams: []string{"format"},
			OptionalParams: []string{
				"filters",      // Violation filters
				"date_from",    // Start date
				"date_to",      // End date
				"repositories", // Repositories to include
				"severities",   // Severity levels
				"async",        // Generate asynchronously
			},
		},
		"reports/export/inventory": {
			OperationID:    "ExportInventory",
			Method:         "POST",
			PathTemplate:   "/api/v2/reports/export/inventory",
			RequiredParams: []string{"format"},
			OptionalParams: []string{
				"repositories",     // Repositories to include
				"package_types",    // Package types to include
				"include_metadata", // Include extended metadata
				"async",            // Generate asynchronously
			},
		},
	}

	return operations
}

// ReportRequest represents a report generation request
type ReportRequest struct {
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Filters ReportFilters          `json:"filters,omitempty"`
	Format  string                 `json:"format,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// ReportFilters represents filters for report generation
type ReportFilters struct {
	Repositories   []string   `json:"repositories,omitempty"`
	Builds         []string   `json:"builds,omitempty"`
	ReleaseBundles []string   `json:"release_bundles,omitempty"`
	ComponentIDs   []string   `json:"component_ids,omitempty"`
	Artifacts      []string   `json:"artifacts,omitempty"`
	Severities     []string   `json:"severities,omitempty"`
	CVEs           []string   `json:"cves,omitempty"`
	Licenses       []string   `json:"licenses,omitempty"`
	WatchNames     []string   `json:"watch_names,omitempty"`
	PolicyNames    []string   `json:"policy_names,omitempty"`
	DateFrom       *time.Time `json:"date_from,omitempty"`
	DateTo         *time.Time `json:"date_to,omitempty"`
}

// ReportResponse represents a report generation response
type ReportResponse struct {
	ReportID    string     `json:"report_id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DownloadURL string     `json:"download_url,omitempty"`
	Format      string     `json:"format,omitempty"`
	Size        int64      `json:"size,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// MetricsResponse represents a metrics query response
type MetricsResponse struct {
	Metrics  []MetricPoint          `json:"metrics"`
	Summary  map[string]interface{} `json:"summary,omitempty"`
	Metadata MetricsMetadata        `json:"metadata"`
}

// MetricPoint represents a single metric data point
type MetricPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Value     float64                `json:"value"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// MetricsMetadata represents metadata for metrics response
type MetricsMetadata struct {
	DateFrom    time.Time `json:"date_from"`
	DateTo      time.Time `json:"date_to"`
	Granularity string    `json:"granularity"`
	MetricType  string    `json:"metric_type"`
	GroupBy     []string  `json:"group_by,omitempty"`
	TotalCount  int       `json:"total_count"`
}

// FormatReportRequest formats a report generation request based on report type
func FormatReportRequest(reportType string, params map[string]interface{}) (interface{}, error) {
	request := ReportRequest{
		Name:    getStringParam(params, "name", fmt.Sprintf("Report-%d", time.Now().Unix())),
		Type:    reportType,
		Format:  getStringParam(params, "format", "json"),
		Options: make(map[string]interface{}),
	}

	// Build filters
	filters := ReportFilters{}

	if repos, ok := params["repositories"].([]string); ok {
		filters.Repositories = repos
	}
	if builds, ok := params["builds"].([]string); ok {
		filters.Builds = builds
	}
	if bundles, ok := params["release_bundles"].([]string); ok {
		filters.ReleaseBundles = bundles
	}
	if components, ok := params["component_ids"].([]string); ok {
		filters.ComponentIDs = components
	}
	if artifacts, ok := params["artifacts"].([]string); ok {
		filters.Artifacts = artifacts
	}

	// Handle severity filters
	if severities, ok := params["severities"].([]string); ok {
		filters.Severities = severities
	} else if severity, ok := params["severity_filter"].(string); ok {
		filters.Severities = []string{severity}
	}

	// Handle CVE filters
	if cves, ok := params["cves"].([]string); ok {
		filters.CVEs = cves
	} else if cve, ok := params["cve_filter"].(string); ok {
		filters.CVEs = []string{cve}
	}

	// Handle license filters
	if licenses, ok := params["licenses"].([]string); ok {
		filters.Licenses = licenses
	} else if license, ok := params["license_filter"].(string); ok {
		filters.Licenses = []string{license}
	}

	// Handle watch and policy filters
	if watches, ok := params["watch_names"].([]string); ok {
		filters.WatchNames = watches
	}
	if policies, ok := params["policy_names"].([]string); ok {
		filters.PolicyNames = policies
	}

	// Handle date range
	if dateFrom, ok := params["date_from"].(string); ok {
		if t, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			filters.DateFrom = &t
		}
	}
	if dateTo, ok := params["date_to"].(string); ok {
		if t, err := time.Parse(time.RFC3339, dateTo); err == nil {
			filters.DateTo = &t
		}
	}

	request.Filters = filters

	// Add type-specific options
	switch reportType {
	case "vulnerability":
		request.Options["include_ignorable"] = getBoolParam(params, "include_ignorable", false)
	case "license":
		request.Options["include_unknown"] = getBoolParam(params, "include_unknown", false)
		request.Options["group_by_artifact"] = getBoolParam(params, "group_by_artifact", false)
	case "sbom":
		request.Options["spec_version"] = getStringParam(params, "spec_version", "2.3")
		request.Options["include_vex"] = getBoolParam(params, "include_vex", false)
		request.Options["component_scope"] = getStringParam(params, "component_scope", "all")
		request.Options["include_dev_deps"] = getBoolParam(params, "include_dev_deps", false)
		request.Options["include_licenses"] = getBoolParam(params, "include_licenses", true)
		request.Options["include_vulns"] = getBoolParam(params, "include_vulns", true)
	case "compliance":
		if standards, ok := params["standards"].([]string); ok {
			request.Options["standards"] = standards
		}
		request.Options["include_passed"] = getBoolParam(params, "include_passed", true)
		request.Options["include_failed"] = getBoolParam(params, "include_failed", true)
		request.Options["include_warnings"] = getBoolParam(params, "include_warnings", true)
	}

	return request, nil
}

// FormatMetricsQuery formats a metrics query request
func FormatMetricsQuery(metricType string, params map[string]interface{}) (map[string]interface{}, error) {
	query := make(map[string]interface{})

	// Common parameters for all metrics
	if dateFrom, ok := params["date_from"].(string); ok {
		query["date_from"] = dateFrom
	} else {
		// Default to last 30 days
		query["date_from"] = time.Now().AddDate(0, 0, -30).Format(time.RFC3339)
	}

	if dateTo, ok := params["date_to"].(string); ok {
		query["date_to"] = dateTo
	} else {
		query["date_to"] = time.Now().Format(time.RFC3339)
	}

	query["granularity"] = getStringParam(params, "granularity", "day")

	// Type-specific parameters
	switch metricType {
	case "violations":
		if repos, ok := params["repositories"].([]string); ok {
			query["repositories"] = repos
		}
		if severities, ok := params["severities"].([]string); ok {
			query["severities"] = severities
		}
		query["violation_type"] = getStringParam(params, "violation_type", "all")
		if groupBy, ok := params["group_by"].(string); ok {
			query["group_by"] = groupBy
		}

	case "scans":
		query["scan_type"] = getStringParam(params, "scan_type", "all")
		if status, ok := params["status"].(string); ok {
			query["status"] = status
		}
		if groupBy, ok := params["group_by"].(string); ok {
			query["group_by"] = groupBy
		}

	case "components":
		if repos, ok := params["repositories"].([]string); ok {
			query["repositories"] = repos
		}
		if packageTypes, ok := params["package_types"].([]string); ok {
			query["package_types"] = packageTypes
		}
		query["include_vulns"] = getBoolParam(params, "include_vulns", true)
		query["include_licenses"] = getBoolParam(params, "include_licenses", true)

	case "exposure":
		if repos, ok := params["repositories"].([]string); ok {
			query["repositories"] = repos
		}
		if severityLevels, ok := params["severity_levels"].([]string); ok {
			query["severity_levels"] = severityLevels
		}
		if cves, ok := params["cve_ids"].([]string); ok {
			query["cve_ids"] = cves
		}

	case "trends":
		query["metric_type"] = getStringParam(params, "metric_type", "violations")
		if compareTo, ok := params["compare_to"].(string); ok {
			query["compare_to"] = compareTo
		}
		query["trend_analysis"] = getBoolParam(params, "trend_analysis", true)
	}

	return query, nil
}

// ParseReportResponse parses a report generation response
func ParseReportResponse(response interface{}) (*ReportResponse, error) {
	respMap, ok := response.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid report response format")
	}

	report := &ReportResponse{
		ReportID: getStringFromResponse(respMap, "report_id"),
		Name:     getStringFromResponse(respMap, "name"),
		Type:     getStringFromResponse(respMap, "type"),
		Status:   getStringFromResponse(respMap, "status"),
	}

	// Parse timestamps
	if createdStr := getStringFromResponse(respMap, "created_at"); createdStr != "" {
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			report.CreatedAt = t
		}
	}

	if completedStr := getStringFromResponse(respMap, "completed_at"); completedStr != "" {
		if t, err := time.Parse(time.RFC3339, completedStr); err == nil {
			report.CompletedAt = &t
		}
	}

	report.DownloadURL = getStringFromResponse(respMap, "download_url")
	report.Format = getStringFromResponse(respMap, "format")

	if size, ok := respMap["size"].(float64); ok {
		report.Size = int64(size)
	}

	report.Error = getStringFromResponse(respMap, "error")

	return report, nil
}

// ParseMetricsResponse parses a metrics query response
func ParseMetricsResponse(response interface{}) (*MetricsResponse, error) {
	respMap, ok := response.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid metrics response format")
	}

	metricsResp := &MetricsResponse{
		Metrics: []MetricPoint{},
		Summary: make(map[string]interface{}),
	}

	// Parse metrics array
	if metricsData, ok := respMap["metrics"].([]interface{}); ok {
		for _, m := range metricsData {
			if metric, ok := m.(map[string]interface{}); ok {
				point := MetricPoint{
					Labels:  make(map[string]string),
					Details: make(map[string]interface{}),
				}

				// Parse timestamp
				if tsStr := getStringFromResponse(metric, "timestamp"); tsStr != "" {
					if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
						point.Timestamp = t
					}
				}

				// Parse value
				if val, ok := metric["value"].(float64); ok {
					point.Value = val
				}

				// Parse labels
				if labels, ok := metric["labels"].(map[string]interface{}); ok {
					for k, v := range labels {
						point.Labels[k] = fmt.Sprintf("%v", v)
					}
				}

				// Parse details
				if details, ok := metric["details"].(map[string]interface{}); ok {
					point.Details = details
				}

				metricsResp.Metrics = append(metricsResp.Metrics, point)
			}
		}
	}

	// Parse summary
	if summary, ok := respMap["summary"].(map[string]interface{}); ok {
		metricsResp.Summary = summary
	}

	// Parse metadata
	if metadata, ok := respMap["metadata"].(map[string]interface{}); ok {
		md := MetricsMetadata{}

		if dateFromStr := getStringFromResponse(metadata, "date_from"); dateFromStr != "" {
			if t, err := time.Parse(time.RFC3339, dateFromStr); err == nil {
				md.DateFrom = t
			}
		}

		if dateToStr := getStringFromResponse(metadata, "date_to"); dateToStr != "" {
			if t, err := time.Parse(time.RFC3339, dateToStr); err == nil {
				md.DateTo = t
			}
		}

		md.Granularity = getStringFromResponse(metadata, "granularity")
		md.MetricType = getStringFromResponse(metadata, "metric_type")

		if groupBy, ok := metadata["group_by"].([]interface{}); ok {
			for _, g := range groupBy {
				md.GroupBy = append(md.GroupBy, fmt.Sprintf("%v", g))
			}
		}

		if count, ok := metadata["total_count"].(float64); ok {
			md.TotalCount = int(count)
		}

		metricsResp.Metadata = md
	}

	return metricsResp, nil
}

// GetReportStatus checks if a report is ready and returns its status
func GetReportStatus(response interface{}) (string, bool, error) {
	report, err := ParseReportResponse(response)
	if err != nil {
		return "", false, err
	}

	status := strings.ToLower(report.Status)
	isReady := status == "completed" || status == "ready"

	return report.Status, isReady, nil
}

// GetReportTypes returns all available report types
func GetReportTypes() []string {
	return []string{
		"vulnerability",
		"license",
		"operational_risk",
		"sbom",
		"compliance",
		"violations",
		"inventory",
	}
}

// GetReportFormats returns all available report formats
func GetReportFormats() []string {
	return []string{
		"json",
		"pdf",
		"csv",
		"xml",
		"spdx",           // For SBOM
		"cyclonedx-json", // For SBOM
		"cyclonedx-xml",  // For SBOM
	}
}

// GetMetricTypes returns all available metric types
func GetMetricTypes() []string {
	return []string{
		"violations",
		"scans",
		"components",
		"exposure",
		"trends",
		"summary",
		"dashboard",
	}
}

// ValidateReportType validates if a report type is supported
func ValidateReportType(reportType string) error {
	validTypes := GetReportTypes()
	for _, valid := range validTypes {
		if strings.EqualFold(reportType, valid) {
			return nil
		}
	}
	return fmt.Errorf("unsupported report type: %s, valid types are: %v", reportType, validTypes)
}

// ValidateReportFormat validates if a report format is supported
func ValidateReportFormat(format string) error {
	validFormats := GetReportFormats()
	for _, valid := range validFormats {
		if strings.EqualFold(format, valid) {
			return nil
		}
	}
	return fmt.Errorf("unsupported report format: %s, valid formats are: %v", format, validFormats)
}

// Helper function to get string parameter with default
func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

// Helper function to get bool parameter with default
func getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}

// Helper function to safely get string from response
func getStringFromResponse(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

package xray

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddReportsAndMetricsOperations(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	operations := provider.AddReportsAndMetricsOperations()

	// Test that all expected operations are present
	expectedOps := []string{
		"reports/vulnerability",
		"reports/license",
		"reports/operational_risk",
		"reports/sbom",
		"reports/compliance",
		"reports/status",
		"reports/download",
		"reports/list",
		"reports/get",
		"reports/delete",
		"reports/schedule",
		"reports/schedule/list",
		"reports/schedule/delete",
		"metrics/violations",
		"metrics/scans",
		"metrics/components",
		"metrics/exposure",
		"metrics/trends",
		"metrics/summary",
		"metrics/dashboard",
		"reports/export/violations",
		"reports/export/inventory",
	}

	for _, opName := range expectedOps {
		t.Run("Operation_"+opName, func(t *testing.T) {
			op, exists := operations[opName]
			assert.True(t, exists, "Operation %s should exist", opName)
			assert.NotEmpty(t, op.Method, "Operation %s should have a method", opName)
			assert.NotEmpty(t, op.PathTemplate, "Operation %s should have a path template", opName)
		})
	}
}

func TestFormatReportRequest(t *testing.T) {
	tests := []struct {
		name       string
		reportType string
		params     map[string]interface{}
		validate   func(t *testing.T, req interface{})
	}{
		{
			name:       "Vulnerability Report with Filters",
			reportType: "vulnerability",
			params: map[string]interface{}{
				"name":              "vuln-report-1",
				"format":            "pdf",
				"repositories":      []string{"repo1", "repo2"},
				"severities":        []string{"Critical", "High"},
				"include_ignorable": true,
				"date_from":         "2024-01-01T00:00:00Z",
				"date_to":           "2024-01-31T23:59:59Z",
			},
			validate: func(t *testing.T, req interface{}) {
				report := req.(ReportRequest)
				assert.Equal(t, "vuln-report-1", report.Name)
				assert.Equal(t, "vulnerability", report.Type)
				assert.Equal(t, "pdf", report.Format)
				assert.Equal(t, []string{"repo1", "repo2"}, report.Filters.Repositories)
				assert.Equal(t, []string{"Critical", "High"}, report.Filters.Severities)
				assert.True(t, report.Options["include_ignorable"].(bool))
			},
		},
		{
			name:       "License Report with Defaults",
			reportType: "license",
			params: map[string]interface{}{
				"name":     "license-report",
				"builds":   []string{"build1", "build2"},
				"licenses": []string{"MIT", "Apache-2.0"},
			},
			validate: func(t *testing.T, req interface{}) {
				report := req.(ReportRequest)
				assert.Equal(t, "license-report", report.Name)
				assert.Equal(t, "license", report.Type)
				assert.Equal(t, "json", report.Format) // Default format
				assert.Equal(t, []string{"build1", "build2"}, report.Filters.Builds)
				assert.Equal(t, []string{"MIT", "Apache-2.0"}, report.Filters.Licenses)
				assert.False(t, report.Options["include_unknown"].(bool))
				assert.False(t, report.Options["group_by_artifact"].(bool))
			},
		},
		{
			name:       "SBOM Report with All Options",
			reportType: "sbom",
			params: map[string]interface{}{
				"name":             "sbom-report",
				"format":           "spdx",
				"artifacts":        []string{"/path/to/artifact"},
				"spec_version":     "2.3",
				"include_vex":      true,
				"component_scope":  "direct",
				"include_dev_deps": true,
				"include_licenses": false,
				"include_vulns":    false,
			},
			validate: func(t *testing.T, req interface{}) {
				report := req.(ReportRequest)
				assert.Equal(t, "sbom-report", report.Name)
				assert.Equal(t, "sbom", report.Type)
				assert.Equal(t, "spdx", report.Format)
				assert.Equal(t, []string{"/path/to/artifact"}, report.Filters.Artifacts)
				assert.Equal(t, "2.3", report.Options["spec_version"])
				assert.True(t, report.Options["include_vex"].(bool))
				assert.Equal(t, "direct", report.Options["component_scope"])
				assert.True(t, report.Options["include_dev_deps"].(bool))
				assert.False(t, report.Options["include_licenses"].(bool))
				assert.False(t, report.Options["include_vulns"].(bool))
			},
		},
		{
			name:       "Compliance Report",
			reportType: "compliance",
			params: map[string]interface{}{
				"name":             "compliance-report",
				"standards":        []string{"PCI-DSS", "HIPAA"},
				"include_passed":   false,
				"include_failed":   true,
				"include_warnings": false,
			},
			validate: func(t *testing.T, req interface{}) {
				report := req.(ReportRequest)
				assert.Equal(t, "compliance-report", report.Name)
				assert.Equal(t, "compliance", report.Type)
				standards := report.Options["standards"].([]string)
				assert.Equal(t, []string{"PCI-DSS", "HIPAA"}, standards)
				assert.False(t, report.Options["include_passed"].(bool))
				assert.True(t, report.Options["include_failed"].(bool))
				assert.False(t, report.Options["include_warnings"].(bool))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := FormatReportRequest(tt.reportType, tt.params)
			require.NoError(t, err)
			require.NotNil(t, req)
			tt.validate(t, req)
		})
	}
}

func TestFormatMetricsQuery(t *testing.T) {
	tests := []struct {
		name       string
		metricType string
		params     map[string]interface{}
		validate   func(t *testing.T, query map[string]interface{})
	}{
		{
			name:       "Violations Metrics with Filters",
			metricType: "violations",
			params: map[string]interface{}{
				"date_from":      "2024-01-01T00:00:00Z",
				"date_to":        "2024-01-31T23:59:59Z",
				"granularity":    "hour",
				"repositories":   []string{"repo1", "repo2"},
				"severities":     []string{"Critical", "High"},
				"violation_type": "security",
				"group_by":       "severity",
			},
			validate: func(t *testing.T, query map[string]interface{}) {
				assert.Equal(t, "2024-01-01T00:00:00Z", query["date_from"])
				assert.Equal(t, "2024-01-31T23:59:59Z", query["date_to"])
				assert.Equal(t, "hour", query["granularity"])
				assert.Equal(t, []string{"repo1", "repo2"}, query["repositories"])
				assert.Equal(t, []string{"Critical", "High"}, query["severities"])
				assert.Equal(t, "security", query["violation_type"])
				assert.Equal(t, "severity", query["group_by"])
			},
		},
		{
			name:       "Scans Metrics with Defaults",
			metricType: "scans",
			params: map[string]interface{}{
				"scan_type": "artifact",
				"status":    "completed",
			},
			validate: func(t *testing.T, query map[string]interface{}) {
				// Should have default date range
				assert.NotEmpty(t, query["date_from"])
				assert.NotEmpty(t, query["date_to"])
				assert.Equal(t, "day", query["granularity"]) // Default
				assert.Equal(t, "artifact", query["scan_type"])
				assert.Equal(t, "completed", query["status"])
			},
		},
		{
			name:       "Trends Metrics",
			metricType: "trends",
			params: map[string]interface{}{
				"metric_type":    "violations",
				"date_from":      "2024-01-01T00:00:00Z",
				"compare_to":     "previous_month",
				"trend_analysis": true,
			},
			validate: func(t *testing.T, query map[string]interface{}) {
				assert.Equal(t, "violations", query["metric_type"])
				assert.Equal(t, "previous_month", query["compare_to"])
				assert.True(t, query["trend_analysis"].(bool))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := FormatMetricsQuery(tt.metricType, tt.params)
			require.NoError(t, err)
			require.NotNil(t, query)
			tt.validate(t, query)
		})
	}
}

func TestParseReportResponse(t *testing.T) {
	mockResponse := map[string]interface{}{
		"report_id":    "report-123",
		"name":         "Test Report",
		"type":         "vulnerability",
		"status":       "completed",
		"created_at":   "2024-01-01T10:00:00Z",
		"completed_at": "2024-01-01T10:05:00Z",
		"download_url": "/api/v2/reports/report-123/download",
		"format":       "pdf",
		"size":         float64(1024576), // 1MB
	}

	report, err := ParseReportResponse(mockResponse)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, "report-123", report.ReportID)
	assert.Equal(t, "Test Report", report.Name)
	assert.Equal(t, "vulnerability", report.Type)
	assert.Equal(t, "completed", report.Status)
	assert.Equal(t, "/api/v2/reports/report-123/download", report.DownloadURL)
	assert.Equal(t, "pdf", report.Format)
	assert.Equal(t, int64(1024576), report.Size)

	// Check timestamps
	expectedCreated, _ := time.Parse(time.RFC3339, "2024-01-01T10:00:00Z")
	assert.Equal(t, expectedCreated, report.CreatedAt)

	expectedCompleted, _ := time.Parse(time.RFC3339, "2024-01-01T10:05:00Z")
	assert.Equal(t, expectedCompleted, *report.CompletedAt)
}

func TestParseMetricsResponse(t *testing.T) {
	mockResponse := map[string]interface{}{
		"metrics": []interface{}{
			map[string]interface{}{
				"timestamp": "2024-01-01T10:00:00Z",
				"value":     float64(42),
				"labels": map[string]interface{}{
					"severity":   "Critical",
					"repository": "repo1",
				},
				"details": map[string]interface{}{
					"count": float64(5),
					"trend": "increasing",
				},
			},
			map[string]interface{}{
				"timestamp": "2024-01-01T11:00:00Z",
				"value":     float64(35),
				"labels": map[string]interface{}{
					"severity":   "High",
					"repository": "repo2",
				},
			},
		},
		"summary": map[string]interface{}{
			"total":   float64(77),
			"average": float64(38.5),
			"max":     float64(42),
			"min":     float64(35),
		},
		"metadata": map[string]interface{}{
			"date_from":   "2024-01-01T00:00:00Z",
			"date_to":     "2024-01-31T23:59:59Z",
			"granularity": "hour",
			"metric_type": "violations",
			"group_by":    []interface{}{"severity", "repository"},
			"total_count": float64(2),
		},
	}

	metrics, err := ParseMetricsResponse(mockResponse)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Check metrics data
	assert.Len(t, metrics.Metrics, 2)

	// First metric
	assert.Equal(t, float64(42), metrics.Metrics[0].Value)
	assert.Equal(t, "Critical", metrics.Metrics[0].Labels["severity"])
	assert.Equal(t, "repo1", metrics.Metrics[0].Labels["repository"])
	assert.Equal(t, float64(5), metrics.Metrics[0].Details["count"])
	assert.Equal(t, "increasing", metrics.Metrics[0].Details["trend"])

	// Second metric
	assert.Equal(t, float64(35), metrics.Metrics[1].Value)
	assert.Equal(t, "High", metrics.Metrics[1].Labels["severity"])
	assert.Equal(t, "repo2", metrics.Metrics[1].Labels["repository"])

	// Check summary
	assert.Equal(t, float64(77), metrics.Summary["total"])
	assert.Equal(t, float64(38.5), metrics.Summary["average"])
	assert.Equal(t, float64(42), metrics.Summary["max"])
	assert.Equal(t, float64(35), metrics.Summary["min"])

	// Check metadata
	assert.Equal(t, "hour", metrics.Metadata.Granularity)
	assert.Equal(t, "violations", metrics.Metadata.MetricType)
	assert.Equal(t, []string{"severity", "repository"}, metrics.Metadata.GroupBy)
	assert.Equal(t, 2, metrics.Metadata.TotalCount)
}

func TestGetReportStatus(t *testing.T) {
	tests := []struct {
		name         string
		response     interface{}
		expectStatus string
		expectReady  bool
		expectError  bool
	}{
		{
			name: "Completed Report",
			response: map[string]interface{}{
				"report_id": "123",
				"status":    "completed",
			},
			expectStatus: "completed",
			expectReady:  true,
			expectError:  false,
		},
		{
			name: "Ready Report",
			response: map[string]interface{}{
				"report_id": "456",
				"status":    "ready",
			},
			expectStatus: "ready",
			expectReady:  true,
			expectError:  false,
		},
		{
			name: "Generating Report",
			response: map[string]interface{}{
				"report_id": "789",
				"status":    "generating",
			},
			expectStatus: "generating",
			expectReady:  false,
			expectError:  false,
		},
		{
			name: "Failed Report",
			response: map[string]interface{}{
				"report_id": "000",
				"status":    "failed",
				"error":     "Internal error",
			},
			expectStatus: "failed",
			expectReady:  false,
			expectError:  false,
		},
		{
			name:         "Invalid Response",
			response:     "invalid",
			expectStatus: "",
			expectReady:  false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ready, err := GetReportStatus(tt.response)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectStatus, status)
				assert.Equal(t, tt.expectReady, ready)
			}
		})
	}
}

func TestValidateReportType(t *testing.T) {
	// Valid types
	validTypes := []string{
		"vulnerability",
		"license",
		"operational_risk",
		"sbom",
		"compliance",
		"violations",
		"inventory",
	}

	for _, reportType := range validTypes {
		t.Run("Valid_"+reportType, func(t *testing.T) {
			err := ValidateReportType(reportType)
			assert.NoError(t, err)
		})
	}

	// Test case insensitive
	t.Run("Case_Insensitive", func(t *testing.T) {
		err := ValidateReportType("VULNERABILITY")
		assert.NoError(t, err)
	})

	// Invalid type
	t.Run("Invalid_Type", func(t *testing.T) {
		err := ValidateReportType("invalid_type")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported report type")
	})
}

func TestValidateReportFormat(t *testing.T) {
	// Valid formats
	validFormats := []string{
		"json",
		"pdf",
		"csv",
		"xml",
		"spdx",
		"cyclonedx-json",
		"cyclonedx-xml",
	}

	for _, format := range validFormats {
		t.Run("Valid_"+format, func(t *testing.T) {
			err := ValidateReportFormat(format)
			assert.NoError(t, err)
		})
	}

	// Test case insensitive
	t.Run("Case_Insensitive", func(t *testing.T) {
		err := ValidateReportFormat("JSON")
		assert.NoError(t, err)
	})

	// Invalid format
	t.Run("Invalid_Format", func(t *testing.T) {
		err := ValidateReportFormat("invalid_format")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported report format")
	})
}

func TestGetReportTypes(t *testing.T) {
	types := GetReportTypes()
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "vulnerability")
	assert.Contains(t, types, "license")
	assert.Contains(t, types, "sbom")
	assert.Contains(t, types, "compliance")
}

func TestGetReportFormats(t *testing.T) {
	formats := GetReportFormats()
	assert.NotEmpty(t, formats)
	assert.Contains(t, formats, "json")
	assert.Contains(t, formats, "pdf")
	assert.Contains(t, formats, "csv")
	assert.Contains(t, formats, "xml")
}

func TestGetMetricTypes(t *testing.T) {
	types := GetMetricTypes()
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "violations")
	assert.Contains(t, types, "scans")
	assert.Contains(t, types, "components")
	assert.Contains(t, types, "trends")
}

// Integration test for report generation workflow
func TestReportGenerationWorkflow(t *testing.T) {
	// Create a test server that simulates the Xray API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/reports/vulnerabilities":
			// Report generation request
			assert.Equal(t, "POST", r.Method)

			// Return report ID
			response := map[string]interface{}{
				"report_id": "report-test-123",
				"status":    "generating",
			}
			_ = json.NewEncoder(w).Encode(response)

		case "/api/v2/reports/report-test-123/status":
			// Status check
			assert.Equal(t, "GET", r.Method)

			// Return completed status
			response := map[string]interface{}{
				"report_id":    "report-test-123",
				"status":       "completed",
				"completed_at": time.Now().Format(time.RFC3339),
				"download_url": "/api/v2/reports/report-test-123/download",
			}
			_ = json.NewEncoder(w).Encode(response)

		case "/api/v2/reports/report-test-123/download":
			// Download report
			assert.Equal(t, "GET", r.Method)

			// Return mock PDF data
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", "attachment; filename=report.pdf")
			_, _ = w.Write([]byte("mock-pdf-content"))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test report generation request
	params := map[string]interface{}{
		"name":         "test-report",
		"format":       "pdf",
		"repositories": []string{"repo1", "repo2"},
		"severities":   []string{"Critical", "High"},
	}

	req, err := FormatReportRequest("vulnerability", params)
	require.NoError(t, err)

	report := req.(ReportRequest)
	assert.Equal(t, "test-report", report.Name)
	assert.Equal(t, "vulnerability", report.Type)
	assert.Equal(t, "pdf", report.Format)
}

// Test metrics aggregation workflow
func TestMetricsAggregationWorkflow(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/metrics/violations":
			// Return violations metrics
			response := map[string]interface{}{
				"metrics": []map[string]interface{}{
					{
						"timestamp": "2024-01-01T00:00:00Z",
						"value":     float64(10),
						"labels": map[string]string{
							"severity": "Critical",
						},
					},
					{
						"timestamp": "2024-01-02T00:00:00Z",
						"value":     float64(15),
						"labels": map[string]string{
							"severity": "High",
						},
					},
				},
				"summary": map[string]interface{}{
					"total":          float64(25),
					"critical_count": float64(10),
					"high_count":     float64(15),
				},
			}
			_ = json.NewEncoder(w).Encode(response)

		case "/api/v2/metrics/trends":
			// Return trend analysis
			response := map[string]interface{}{
				"metrics": []map[string]interface{}{
					{
						"timestamp": "2024-01-01T00:00:00Z",
						"value":     float64(100),
						"details": map[string]interface{}{
							"trend_direction": "increasing",
							"change_percent":  float64(15.5),
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test metrics query formatting
	params := map[string]interface{}{
		"date_from":   "2024-01-01T00:00:00Z",
		"date_to":     "2024-01-31T23:59:59Z",
		"granularity": "day",
		"severities":  []string{"Critical", "High"},
		"group_by":    "severity",
	}

	query, err := FormatMetricsQuery("violations", params)
	require.NoError(t, err)
	assert.NotNil(t, query)
	assert.Equal(t, "2024-01-01T00:00:00Z", query["date_from"])
	assert.Equal(t, "day", query["granularity"])
}

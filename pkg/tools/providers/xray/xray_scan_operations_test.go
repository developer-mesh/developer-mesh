package xray

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArtifactSummaryResponse(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
		validate    func(t *testing.T, response *ArtifactSummaryResponse)
	}{
		{
			name: "valid response with vulnerabilities",
			jsonData: `{
				"artifacts": [{
					"general": {
						"path": "docker-local/myimage:latest",
						"name": "myimage",
						"sha256": "abc123",
						"pkg_type": "Docker",
						"component_id": "docker://myimage:latest"
					},
					"issues": [{
						"issue_id": "XRAY-123",
						"summary": "Critical vulnerability in OpenSSL",
						"description": "Buffer overflow in OpenSSL",
						"issue_type": "security",
						"severity": "Critical",
						"provider": "JFrog",
						"cves": [{
							"cve": "CVE-2021-12345",
							"cvss_v3_score": 9.8
						}]
					}, {
						"issue_id": "XRAY-124",
						"summary": "Medium severity issue",
						"severity": "Medium",
						"issue_type": "security",
						"provider": "JFrog"
					}],
					"licenses": [{
						"license_key": "Apache-2.0",
						"name": "Apache License 2.0",
						"url": "https://www.apache.org/licenses/LICENSE-2.0"
					}]
				}]
			}`,
			expectError: false,
			validate: func(t *testing.T, response *ArtifactSummaryResponse) {
				assert.Len(t, response.Artifacts, 1)
				artifact := response.Artifacts[0]
				assert.Equal(t, "docker-local/myimage:latest", artifact.General.Path)
				assert.Equal(t, "Docker", artifact.General.PackageType)
				assert.Len(t, artifact.Issues, 2)
				assert.Equal(t, SeverityCritical, artifact.Issues[0].Severity)
				assert.Equal(t, "CVE-2021-12345", artifact.Issues[0].CVE[0].ID)
				assert.Equal(t, 9.8, artifact.Issues[0].CVE[0].CVSSV3Score)
				assert.Len(t, artifact.Licenses, 1)
			},
		},
		{
			name: "response with errors",
			jsonData: `{
				"artifacts": [],
				"errors": [{
					"error": "NOT_FOUND",
					"message": "Artifact not found",
					"path": "docker-local/nonexistent:latest"
				}]
			}`,
			expectError: false,
			validate: func(t *testing.T, response *ArtifactSummaryResponse) {
				assert.Empty(t, response.Artifacts)
				assert.Len(t, response.Errors, 1)
				assert.Equal(t, "NOT_FOUND", response.Errors[0].Error)
			},
		},
		{
			name:        "invalid JSON",
			jsonData:    `{invalid json}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := ParseArtifactSummaryResponse([]byte(tt.jsonData))

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, response)
				if tt.validate != nil {
					tt.validate(t, response)
				}
			}
		})
	}
}

func TestParseBuildSummaryResponse(t *testing.T) {
	jsonData := `{
		"build_name": "my-build",
		"build_number": "123",
		"issues": [{
			"issue_id": "XRAY-200",
			"summary": "High severity vulnerability",
			"severity": "High",
			"issue_type": "security",
			"provider": "JFrog"
		}],
		"licenses": [{
			"license_key": "MIT",
			"name": "MIT License"
		}],
		"summary": {
			"total_issues": 5,
			"critical": 1,
			"high": 2,
			"medium": 1,
			"low": 1,
			"unknown": 0
		}
	}`

	response, err := ParseBuildSummaryResponse([]byte(jsonData))
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "my-build", response.BuildName)
	assert.Equal(t, "123", response.BuildNumber)
	assert.Len(t, response.Issues, 1)
	assert.Equal(t, SeverityHigh, response.Issues[0].Severity)
	assert.Equal(t, 5, response.Summary.TotalIssues)
	assert.Equal(t, 2, response.Summary.High)
}

func TestParseScanResponse(t *testing.T) {
	jsonData := `{
		"scan_id": "scan-abc-123",
		"status": "initiated",
		"created": "2024-01-01T12:00:00Z"
	}`

	response, err := ParseScanResponse([]byte(jsonData))
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "scan-abc-123", response.ScanID)
	assert.Equal(t, "initiated", response.Status)
	assert.Equal(t, 2024, response.Created.Year())
}

func TestParseScanStatusResponse(t *testing.T) {
	jsonData := `{
		"status": "in_progress",
		"progress": 75,
		"message": "Scanning dependencies...",
		"created": "2024-01-01T12:00:00Z"
	}`

	response, err := ParseScanStatusResponse([]byte(jsonData))
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "in_progress", response.Status)
	assert.Equal(t, 75, response.Progress)
	assert.Equal(t, "Scanning dependencies...", response.Message)
}

func TestCategorizeBySeverity(t *testing.T) {
	issues := []Issue{
		{ID: "1", Severity: SeverityCritical},
		{ID: "2", Severity: SeverityCritical},
		{ID: "3", Severity: SeverityHigh},
		{ID: "4", Severity: SeverityMedium},
		{ID: "5", Severity: SeverityLow},
		{ID: "6", Severity: ""}, // Should become Unknown
	}

	categorized := CategorizeBySeverity(issues)

	assert.Len(t, categorized[SeverityCritical], 2)
	assert.Len(t, categorized[SeverityHigh], 1)
	assert.Len(t, categorized[SeverityMedium], 1)
	assert.Len(t, categorized[SeverityLow], 1)
	assert.Len(t, categorized[SeverityUnknown], 1)

	// Verify all severity levels are initialized even if empty
	assert.NotNil(t, categorized[SeverityCritical])
	assert.NotNil(t, categorized[SeverityHigh])
	assert.NotNil(t, categorized[SeverityMedium])
	assert.NotNil(t, categorized[SeverityLow])
	assert.NotNil(t, categorized[SeverityUnknown])
}

func TestGetSeveritySummary(t *testing.T) {
	issues := []Issue{
		{ID: "1", Severity: SeverityCritical},
		{ID: "2", Severity: SeverityCritical},
		{ID: "3", Severity: SeverityHigh},
		{ID: "4", Severity: SeverityMedium},
		{ID: "5", Severity: SeverityLow},
	}

	summary := GetSeveritySummary(issues)

	assert.Equal(t, 5, summary.TotalIssues)
	assert.Equal(t, 2, summary.Critical)
	assert.Equal(t, 1, summary.High)
	assert.Equal(t, 1, summary.Medium)
	assert.Equal(t, 1, summary.Low)
	assert.Equal(t, 0, summary.Unknown)

	// Check map format
	assert.Equal(t, 2, summary.BySeverity["Critical"])
	assert.Equal(t, 1, summary.BySeverity["High"])
}

func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected ScanSeverity
	}{
		{"Critical", SeverityCritical},
		{"critical", SeverityCritical},
		{"CRITICAL", SeverityCritical},
		{"1", SeverityCritical},
		{"High", SeverityHigh},
		{"high", SeverityHigh},
		{"2", SeverityHigh},
		{"Medium", SeverityMedium},
		{"medium", SeverityMedium},
		{"3", SeverityMedium},
		{"Low", SeverityLow},
		{"low", SeverityLow},
		{"4", SeverityLow},
		{"unknown", SeverityUnknown},
		{"", SeverityUnknown},
		{"invalid", SeverityUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeSeverity(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatScanRequest(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]interface{}
		expectError bool
		validate    func(t *testing.T, request *ArtifactScanRequest)
	}{
		{
			name: "valid request with component ID",
			params: map[string]interface{}{
				"componentId": "docker://myrepo/myimage:latest",
			},
			expectError: false,
			validate: func(t *testing.T, request *ArtifactScanRequest) {
				assert.Equal(t, "docker://myrepo/myimage:latest", request.ComponentID)
				assert.Empty(t, request.Watch)
			},
		},
		{
			name: "request with watch",
			params: map[string]interface{}{
				"componentId": "docker://myrepo/myimage:latest",
				"watch":       "production-watch",
			},
			expectError: false,
			validate: func(t *testing.T, request *ArtifactScanRequest) {
				assert.Equal(t, "docker://myrepo/myimage:latest", request.ComponentID)
				assert.Equal(t, "production-watch", request.Watch)
			},
		},
		{
			name:        "missing component ID",
			params:      map[string]interface{}{},
			expectError: true,
		},
		{
			name: "empty component ID",
			params: map[string]interface{}{
				"componentId": "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, err := FormatScanRequest(tt.params)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, request)
				if tt.validate != nil {
					tt.validate(t, request)
				}
			}
		})
	}
}

func TestFormatBuildScanRequest(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]interface{}
		expectError bool
		validate    func(t *testing.T, request *BuildScanRequest)
	}{
		{
			name: "valid build scan request",
			params: map[string]interface{}{
				"buildName":   "my-build",
				"buildNumber": "123",
			},
			expectError: false,
			validate: func(t *testing.T, request *BuildScanRequest) {
				assert.Equal(t, "my-build", request.BuildName)
				assert.Equal(t, "123", request.BuildNumber)
				assert.Empty(t, request.Context)
			},
		},
		{
			name: "request with context",
			params: map[string]interface{}{
				"buildName":   "my-build",
				"buildNumber": "123",
				"context":     "production",
			},
			expectError: false,
			validate: func(t *testing.T, request *BuildScanRequest) {
				assert.Equal(t, "my-build", request.BuildName)
				assert.Equal(t, "123", request.BuildNumber)
				assert.Equal(t, "production", request.Context)
			},
		},
		{
			name: "missing build name",
			params: map[string]interface{}{
				"buildNumber": "123",
			},
			expectError: true,
		},
		{
			name: "missing build number",
			params: map[string]interface{}{
				"buildName": "my-build",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, err := FormatBuildScanRequest(tt.params)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, request)
				if tt.validate != nil {
					tt.validate(t, request)
				}
			}
		})
	}
}

func TestFormatArtifactSummaryRequest(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]interface{}
		expectError bool
		validate    func(t *testing.T, request *ArtifactSummaryRequest)
	}{
		{
			name: "valid request with string slice",
			params: map[string]interface{}{
				"paths": []string{"repo/path1", "repo/path2"},
			},
			expectError: false,
			validate: func(t *testing.T, request *ArtifactSummaryRequest) {
				assert.Len(t, request.Paths, 2)
				assert.Contains(t, request.Paths, "repo/path1")
				assert.Contains(t, request.Paths, "repo/path2")
			},
		},
		{
			name: "valid request with interface slice",
			params: map[string]interface{}{
				"paths": []interface{}{"repo/path1", "repo/path2"},
			},
			expectError: false,
			validate: func(t *testing.T, request *ArtifactSummaryRequest) {
				assert.Len(t, request.Paths, 2)
			},
		},
		{
			name: "valid request with single path",
			params: map[string]interface{}{
				"path": "repo/single-path",
			},
			expectError: false,
			validate: func(t *testing.T, request *ArtifactSummaryRequest) {
				assert.Len(t, request.Paths, 1)
				assert.Equal(t, "repo/single-path", request.Paths[0])
			},
		},
		{
			name:        "missing paths",
			params:      map[string]interface{}{},
			expectError: true,
		},
		{
			name: "empty paths array",
			params: map[string]interface{}{
				"paths": []string{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, err := FormatArtifactSummaryRequest(tt.params)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, request)
				if tt.validate != nil {
					tt.validate(t, request)
				}
			}
		})
	}
}

func TestGetMostSevereIssue(t *testing.T) {
	tests := []struct {
		name     string
		issues   []Issue
		expected *Issue
	}{
		{
			name: "critical is most severe",
			issues: []Issue{
				{ID: "1", Severity: SeverityLow},
				{ID: "2", Severity: SeverityCritical},
				{ID: "3", Severity: SeverityMedium},
			},
			expected: &Issue{ID: "2", Severity: SeverityCritical},
		},
		{
			name: "high when no critical",
			issues: []Issue{
				{ID: "1", Severity: SeverityLow},
				{ID: "2", Severity: SeverityHigh},
				{ID: "3", Severity: SeverityMedium},
			},
			expected: &Issue{ID: "2", Severity: SeverityHigh},
		},
		{
			name:     "empty list",
			issues:   []Issue{},
			expected: nil,
		},
		{
			name: "unknown severity treated as lowest priority",
			issues: []Issue{
				{ID: "1", Severity: SeverityUnknown},
				{ID: "2", Severity: SeverityLow},
			},
			expected: &Issue{ID: "2", Severity: SeverityLow},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMostSevereIssue(tt.issues)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.Severity, result.Severity)
			}
		})
	}
}

func TestHasCriticalVulnerabilities(t *testing.T) {
	tests := []struct {
		name     string
		issues   []Issue
		expected bool
	}{
		{
			name: "has critical",
			issues: []Issue{
				{Severity: SeverityHigh},
				{Severity: SeverityCritical},
			},
			expected: true,
		},
		{
			name: "no critical",
			issues: []Issue{
				{Severity: SeverityHigh},
				{Severity: SeverityMedium},
			},
			expected: false,
		},
		{
			name:     "empty list",
			issues:   []Issue{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasCriticalVulnerabilities(tt.issues)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterIssuesBySeverity(t *testing.T) {
	issues := []Issue{
		{ID: "1", Severity: SeverityCritical},
		{ID: "2", Severity: SeverityHigh},
		{ID: "3", Severity: SeverityMedium},
		{ID: "4", Severity: SeverityLow},
		{ID: "5", Severity: SeverityUnknown},
	}

	tests := []struct {
		name        string
		minSeverity ScanSeverity
		expectedIDs []string
	}{
		{
			name:        "critical and above",
			minSeverity: SeverityCritical,
			expectedIDs: []string{"1"},
		},
		{
			name:        "high and above",
			minSeverity: SeverityHigh,
			expectedIDs: []string{"1", "2"},
		},
		{
			name:        "medium and above",
			minSeverity: SeverityMedium,
			expectedIDs: []string{"1", "2", "3"},
		},
		{
			name:        "low and above",
			minSeverity: SeverityLow,
			expectedIDs: []string{"1", "2", "3", "4"},
		},
		{
			name:        "all including unknown",
			minSeverity: SeverityUnknown,
			expectedIDs: []string{"1", "2", "3", "4", "5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterIssuesBySeverity(issues, tt.minSeverity)
			assert.Len(t, filtered, len(tt.expectedIDs))

			// Verify the correct issues are included
			filteredIDs := make([]string, len(filtered))
			for i, issue := range filtered {
				filteredIDs[i] = issue.ID
			}

			for _, expectedID := range tt.expectedIDs {
				assert.Contains(t, filteredIDs, expectedID)
			}
		})
	}
}

func TestXrayScanIntegration(t *testing.T) {
	// This test simulates a complete scan workflow
	t.Run("complete scan workflow", func(t *testing.T) {
		// Simulate scan request
		scanParams := map[string]interface{}{
			"componentId": "docker://myrepo/vulnerable-app:1.0",
			"watch":       "production",
		}

		scanRequest, err := FormatScanRequest(scanParams)
		require.NoError(t, err)
		assert.Equal(t, "docker://myrepo/vulnerable-app:1.0", scanRequest.ComponentID)

		// Simulate scan response
		scanResponseJSON := `{
			"scan_id": "scan-xyz-789",
			"status": "initiated",
			"created": "2024-01-01T10:00:00Z"
		}`

		scanResponse, err := ParseScanResponse([]byte(scanResponseJSON))
		require.NoError(t, err)
		assert.Equal(t, "scan-xyz-789", scanResponse.ScanID)

		// Simulate checking scan status
		statusJSON := `{
			"status": "completed",
			"progress": 100,
			"message": "Scan completed successfully",
			"completed": "2024-01-01T10:05:00Z"
		}`

		statusResponse, err := ParseScanStatusResponse([]byte(statusJSON))
		require.NoError(t, err)
		assert.Equal(t, "completed", statusResponse.Status)
		assert.Equal(t, 100, statusResponse.Progress)

		// Simulate getting artifact summary
		summaryParams := map[string]interface{}{
			"paths": []string{"docker-local/vulnerable-app:1.0"},
		}

		summaryRequest, err := FormatArtifactSummaryRequest(summaryParams)
		require.NoError(t, err)
		assert.NotNil(t, summaryRequest) // Use the variable to fix compilation
		assert.Len(t, summaryRequest.Paths, 1)

		summaryJSON := `{
			"artifacts": [{
				"general": {
					"path": "docker-local/vulnerable-app:1.0",
					"name": "vulnerable-app",
					"pkg_type": "Docker"
				},
				"issues": [
					{"issue_id": "XRAY-1", "severity": "Critical", "summary": "Critical CVE", "issue_type": "security"},
					{"issue_id": "XRAY-2", "severity": "High", "summary": "High CVE", "issue_type": "security"},
					{"issue_id": "XRAY-3", "severity": "Medium", "summary": "Medium CVE", "issue_type": "security"}
				]
			}]
		}`

		summaryResponse, err := ParseArtifactSummaryResponse([]byte(summaryJSON))
		require.NoError(t, err)

		// Analyze results
		artifact := summaryResponse.Artifacts[0]
		summary := GetSeveritySummary(artifact.Issues)
		assert.Equal(t, 3, summary.TotalIssues)
		assert.Equal(t, 1, summary.Critical)
		assert.Equal(t, 1, summary.High)
		assert.Equal(t, 1, summary.Medium)

		// Check for critical vulnerabilities
		hasCritical := HasCriticalVulnerabilities(artifact.Issues)
		assert.True(t, hasCritical)

		// Get most severe issue
		mostSevere := GetMostSevereIssue(artifact.Issues)
		require.NotNil(t, mostSevere)
		assert.Equal(t, SeverityCritical, mostSevere.Severity)

		// Filter for high and above
		highAndAbove := FilterIssuesBySeverity(artifact.Issues, SeverityHigh)
		assert.Len(t, highAndAbove, 2) // Critical and High
	})
}

func TestResponseParsingEdgeCases(t *testing.T) {
	t.Run("empty artifact list", func(t *testing.T) {
		jsonData := `{"artifacts": []}`
		response, err := ParseArtifactSummaryResponse([]byte(jsonData))
		require.NoError(t, err)
		assert.Empty(t, response.Artifacts)
		assert.Empty(t, response.Errors)
	})

	t.Run("artifact with no issues", func(t *testing.T) {
		jsonData := `{
			"artifacts": [{
				"general": {
					"path": "clean-repo/clean-image:latest",
					"name": "clean-image"
				},
				"issues": []
			}]
		}`
		response, err := ParseArtifactSummaryResponse([]byte(jsonData))
		require.NoError(t, err)
		assert.Len(t, response.Artifacts, 1)
		assert.Empty(t, response.Artifacts[0].Issues)

		summary := GetSeveritySummary(response.Artifacts[0].Issues)
		assert.Equal(t, 0, summary.TotalIssues)
	})

	t.Run("malformed severity values", func(t *testing.T) {
		// Test normalizing various severity formats
		assert.Equal(t, SeverityCritical, NormalizeSeverity("CRITICAL"))
		assert.Equal(t, SeverityHigh, NormalizeSeverity("high"))
		assert.Equal(t, SeverityUnknown, NormalizeSeverity("invalid"))
		assert.Equal(t, SeverityUnknown, NormalizeSeverity(""))
	})

	t.Run("scan response with minimal fields", func(t *testing.T) {
		jsonData := `{"scan_id": "minimal-scan", "status": "pending"}`
		response, err := ParseScanResponse([]byte(jsonData))
		require.NoError(t, err)
		assert.Equal(t, "minimal-scan", response.ScanID)
		assert.Equal(t, "pending", response.Status)
		assert.True(t, response.Created.IsZero())
		assert.True(t, response.Completed.IsZero())
	})

	t.Run("build summary with partial data", func(t *testing.T) {
		jsonData := `{
			"build_name": "partial-build",
			"build_number": "456",
			"issues": [],
			"licenses": []
		}`
		response, err := ParseBuildSummaryResponse([]byte(jsonData))
		require.NoError(t, err)
		assert.Equal(t, "partial-build", response.BuildName)
		assert.Empty(t, response.Issues)
		assert.Empty(t, response.Licenses)
		assert.Equal(t, 0, response.Summary.TotalIssues)
	})
}

func TestTimeHandling(t *testing.T) {
	// Test that time fields are properly parsed
	now := time.Now().UTC()
	scanResponse := ScanResponse{
		ScanID:    "time-test",
		Status:    "completed",
		Created:   now,
		Completed: now.Add(5 * time.Minute),
	}

	data, err := json.Marshal(scanResponse)
	require.NoError(t, err)

	parsed, err := ParseScanResponse(data)
	require.NoError(t, err)

	// Times should be preserved (within a reasonable margin due to JSON serialization)
	assert.WithinDuration(t, scanResponse.Created, parsed.Created, time.Second)
	assert.WithinDuration(t, scanResponse.Completed, parsed.Completed, time.Second)
}

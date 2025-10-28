package xray

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddComponentIntelligenceOperations(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	operations := provider.AddComponentIntelligenceOperations()

	// Check that all expected operations are present
	expectedOperations := []string{
		"components/searchByCves",
		"components/searchCvesByComponents",
		"components/findByName",
		"components/exportDetails",
		"graph/artifact",
		"graph/build",
		"graph/compareArtifacts",
		"graph/compareBuilds",
		"licenses/report",
		"licenses/summary",
		"vulnerabilities/componentSummary",
		"vulnerabilities/exportSBOM",
		"components/versions",
		"components/impact",
	}

	for _, opName := range expectedOperations {
		_, exists := operations[opName]
		assert.True(t, exists, "Operation %s should exist", opName)
	}

	// Verify a few operations have correct properties
	searchByCves := operations["components/searchByCves"]
	assert.Equal(t, "SearchComponentsByCVEs", searchByCves.OperationID)
	assert.Equal(t, "POST", searchByCves.Method)
	assert.Equal(t, "/api/v1/component/searchByCves", searchByCves.PathTemplate)
	assert.Equal(t, []string{"cves"}, searchByCves.RequiredParams)

	graphArtifact := operations["graph/artifact"]
	assert.Equal(t, "GetArtifactDependencyGraph", graphArtifact.OperationID)
	assert.Equal(t, "POST", graphArtifact.Method)
	assert.Equal(t, "/api/v1/dependencyGraph/artifact", graphArtifact.PathTemplate)
}

func TestSearchComponentsByCVEs(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/component/searchByCves", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Parse request body
		var requestBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		require.NoError(t, err)

		// Verify CVEs are present
		cves, ok := requestBody["cves"].([]interface{})
		assert.True(t, ok)
		assert.Greater(t, len(cves), 0)

		// Return mock response
		response := []CVESearchResult{
			{
				CVE: "CVE-2021-44228",
				Components: []ComponentMatch{
					{
						Name:        "org.apache.logging.log4j:log4j-core",
						PackageType: "Maven",
						Version:     "2.14.1",
						Repository:  "libs-release",
						Path:        "org/apache/logging/log4j/log4j-core/2.14.1",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Configure provider with test server
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Execute the operation with authentication context
	ctx := context.Background()
	// Add authentication to context using provider context
	ctx = providers.WithContext(ctx, &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			APIKey: "test-api-key",
		},
	})
	params := map[string]interface{}{
		"cves": []string{"CVE-2021-44228"},
	}

	result, err := provider.ExecuteOperation(ctx, "components/searchByCves", params)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Parse the response
	results, err := ParseCVESearchResponse(result)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "CVE-2021-44228", results[0].CVE)
	assert.Len(t, results[0].Components, 1)
	assert.Equal(t, "org.apache.logging.log4j:log4j-core", results[0].Components[0].Name)
}

func TestGetDependencyGraph(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/dependencyGraph/artifact", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Return mock dependency graph
		response := DependencyGraph{
			RootComponent: ComponentNode{
				ID:          "npm://express:4.17.1",
				Name:        "express",
				Version:     "4.17.1",
				PackageType: "npm",
				Depth:       0,
			},
			Nodes: []ComponentNode{
				{
					ID:          "npm://express:4.17.1",
					Name:        "express",
					Version:     "4.17.1",
					PackageType: "npm",
					Depth:       0,
				},
				{
					ID:          "npm://body-parser:1.19.0",
					Name:        "body-parser",
					Version:     "1.19.0",
					PackageType: "npm",
					Depth:       1,
				},
			},
			Edges: []DependencyEdge{
				{
					From:         "npm://express:4.17.1",
					To:           "npm://body-parser:1.19.0",
					Relationship: "depends_on",
				},
			},
			TotalNodes: 2,
			MaxDepth:   1,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Configure provider with test server
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Execute the operation with authentication context
	ctx := context.Background()
	// Add authentication to context using provider context
	ctx = providers.WithContext(ctx, &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			APIKey: "test-api-key",
		},
	})
	params := map[string]interface{}{
		"artifact_path": "npm-local/express/-/express-4.17.1.tgz",
	}

	result, err := provider.ExecuteOperation(ctx, "graph/artifact", params)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Parse the response
	graph, err := ParseDependencyGraphResponse(result)
	require.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, "express", graph.RootComponent.Name)
	assert.Equal(t, 2, graph.TotalNodes)
	assert.Equal(t, 1, graph.MaxDepth)
	assert.Len(t, graph.Edges, 1)
}

func TestExportComponentDetails(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/component/exportDetails", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Parse request body
		var requestBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		require.NoError(t, err)

		// Verify required fields
		assert.Equal(t, "commons-collections:commons-collections", requestBody["component_name"])
		assert.Equal(t, "maven", requestBody["package_type"])
		assert.Equal(t, "json", requestBody["output_format"])

		// Return a mock response (normally this would be a zip file)
		response := map[string]interface{}{
			"status":  "success",
			"message": "Export initiated",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Configure provider with test server
	config := provider.GetDefaultConfiguration()
	config.BaseURL = server.URL
	provider.SetConfiguration(config)

	// Execute the operation with authentication context
	ctx := context.Background()
	// Add authentication to context using provider context
	ctx = providers.WithContext(ctx, &providers.ProviderContext{
		Credentials: &providers.ProviderCredentials{
			APIKey: "test-api-key",
		},
	})
	params := map[string]interface{}{
		"component_name": "commons-collections:commons-collections",
		"package_type":   "maven",
		"output_format":  "json",
		"violations":     true,
		"license":        true,
		"security":       true,
	}

	result, err := provider.ExecuteOperation(ctx, "components/exportDetails", params)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestBuildComponentIdentifier(t *testing.T) {
	tests := []struct {
		name        string
		packageType string
		group       string
		compName    string
		version     string
		expected    string
	}{
		{
			name:        "Maven with group",
			packageType: "maven",
			group:       "org.apache.commons",
			compName:    "commons-lang3",
			version:     "3.12.0",
			expected:    "gav://org.apache.commons:commons-lang3:3.12.0",
		},
		{
			name:        "Docker with namespace",
			packageType: "docker",
			group:       "library",
			compName:    "nginx",
			version:     "1.21.0",
			expected:    "docker://library/nginx:1.21.0",
		},
		{
			name:        "NPM with scope",
			packageType: "npm",
			group:       "angular",
			compName:    "core",
			version:     "12.0.0",
			expected:    "npm://@angular/core:12.0.0",
		},
		{
			name:        "NPM without scope",
			packageType: "npm",
			group:       "",
			compName:    "express",
			version:     "4.17.1",
			expected:    "npm://express:4.17.1",
		},
		{
			name:        "PyPI package",
			packageType: "pypi",
			group:       "",
			compName:    "requests",
			version:     "2.28.0",
			expected:    "pypi://requests:2.28.0",
		},
		{
			name:        "Go module",
			packageType: "go",
			group:       "github.com/stretchr",
			compName:    "testify",
			version:     "v1.8.0",
			expected:    "go://github.com/stretchr/testify:v1.8.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildComponentIdentifier(tt.packageType, tt.group, tt.compName, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateComponentIdentifier(t *testing.T) {
	tests := []struct {
		name        string
		componentID string
		wantErr     bool
		errContains string
	}{
		{
			name:        "Valid Maven identifier",
			componentID: "gav://org.apache.commons:commons-lang3:3.12.0",
			wantErr:     false,
		},
		{
			name:        "Valid Docker identifier",
			componentID: "docker://library/nginx:1.21.0",
			wantErr:     false,
		},
		{
			name:        "Valid NPM identifier",
			componentID: "npm://express:4.17.1",
			wantErr:     false,
		},
		{
			name:        "Missing separator",
			componentID: "npmexpress:4.17.1",
			wantErr:     true,
			errContains: "missing '://' separator",
		},
		{
			name:        "Empty package type",
			componentID: "://express:4.17.1",
			wantErr:     true,
			errContains: "missing package type",
		},
		{
			name:        "Empty component path",
			componentID: "npm://",
			wantErr:     true,
			errContains: "missing component path",
		},
		{
			name:        "Unsupported package type",
			componentID: "unknown://package:1.0.0",
			wantErr:     true,
			errContains: "unsupported package type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateComponentIdentifier(tt.componentID)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFilterVulnerabilitiesBySeverity(t *testing.T) {
	issues := []VulnerabilityIssue{
		{IssueID: "XRAY-1", Severity: "Critical"},
		{IssueID: "XRAY-2", Severity: "High"},
		{IssueID: "XRAY-3", Severity: "Medium"},
		{IssueID: "XRAY-4", Severity: "Low"},
		{IssueID: "XRAY-5", Severity: "Unknown"},
	}

	tests := []struct {
		name        string
		minSeverity string
		expected    []string
	}{
		{
			name:        "Filter critical only",
			minSeverity: "critical",
			expected:    []string{"XRAY-1"},
		},
		{
			name:        "Filter high and above",
			minSeverity: "high",
			expected:    []string{"XRAY-1", "XRAY-2"},
		},
		{
			name:        "Filter medium and above",
			minSeverity: "medium",
			expected:    []string{"XRAY-1", "XRAY-2", "XRAY-3"},
		},
		{
			name:        "Filter all",
			minSeverity: "unknown",
			expected:    []string{"XRAY-1", "XRAY-2", "XRAY-3", "XRAY-4", "XRAY-5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterVulnerabilitiesBySeverity(issues, tt.minSeverity)
			var ids []string
			for _, issue := range filtered {
				ids = append(ids, issue.IssueID)
			}
			assert.Equal(t, tt.expected, ids)
		})
	}
}

func TestAnalyzeDependencyDepth(t *testing.T) {
	graph := &DependencyGraph{
		TotalNodes: 5,
		MaxDepth:   3,
		Nodes: []ComponentNode{
			{ID: "1", Depth: 0},
			{ID: "2", Depth: 1, Issues: []VulnerabilityIssue{{IssueID: "V1"}}},
			{ID: "3", Depth: 1, Licenses: []LicenseInfo{{Name: "MIT"}}},
			{ID: "4", Depth: 2, Issues: []VulnerabilityIssue{{IssueID: "V2"}}},
			{ID: "5", Depth: 3},
		},
	}

	analysis := AnalyzeDependencyDepth(graph)

	assert.Equal(t, 5, analysis["total_nodes"])
	assert.Equal(t, 3, analysis["max_depth"])
	assert.Equal(t, 2, analysis["direct_dependencies"])
	assert.Equal(t, 2, analysis["transitive_dependencies"])
	assert.Equal(t, 2, analysis["vulnerable_nodes"])
	assert.Equal(t, 1, analysis["licensed_nodes"])

	depthDist := analysis["depth_distribution"].(map[int]int)
	assert.Equal(t, 1, depthDist[0])
	assert.Equal(t, 2, depthDist[1])
	assert.Equal(t, 1, depthDist[2])
	assert.Equal(t, 1, depthDist[3])
}

func TestFormatRequestFunctions(t *testing.T) {
	t.Run("FormatCVESearchRequest", func(t *testing.T) {
		request := FormatCVESearchRequest([]string{"CVE-2021-44228", "CVE-2021-45046"})
		cves := request["cves"].([]string)
		assert.Len(t, cves, 2)
		assert.Contains(t, cves, "CVE-2021-44228")
		assert.Contains(t, cves, "CVE-2021-45046")
	})

	t.Run("FormatComponentSearchRequest", func(t *testing.T) {
		request := FormatComponentSearchRequest([]string{"gav://log4j:log4j:2.14.1"})
		components := request["components_id"].([]string)
		assert.Len(t, components, 1)
		assert.Equal(t, "gav://log4j:log4j:2.14.1", components[0])
	})

	t.Run("FormatDependencyGraphRequest", func(t *testing.T) {
		request := FormatDependencyGraphRequest("libs-release/my-app.jar", true)
		assert.Equal(t, "libs-release/my-app.jar", request["artifact_path"])
		assert.Equal(t, true, request["include_issues"])
	})

	t.Run("FormatBuildDependencyGraphRequest", func(t *testing.T) {
		request := FormatBuildDependencyGraphRequest("my-app", "123", "artifactory.example.com")
		assert.Equal(t, "my-app", request["build_name"])
		assert.Equal(t, "123", request["build_number"])
		assert.Equal(t, "artifactory.example.com", request["artifactory_instance"])
	})

	t.Run("FormatComponentExportRequest", func(t *testing.T) {
		options := map[string]bool{
			"violations": true,
			"license":    true,
			"security":   false,
		}
		request := FormatComponentExportRequest("log4j:log4j", "maven", "json", options)
		assert.Equal(t, "log4j:log4j", request["component_name"])
		assert.Equal(t, "maven", request["package_type"])
		assert.Equal(t, "json", request["output_format"])
		assert.Equal(t, true, request["violations"])
		assert.Equal(t, true, request["license"])
		assert.Equal(t, false, request["security"])
	})
}

func TestGetSupportedPackageTypes(t *testing.T) {
	packageTypes := GetSupportedPackageTypes()
	assert.Contains(t, packageTypes, "maven")
	assert.Contains(t, packageTypes, "npm")
	assert.Contains(t, packageTypes, "docker")
	assert.Contains(t, packageTypes, "pypi")
	assert.Contains(t, packageTypes, "go")
	assert.Contains(t, packageTypes, "nuget")
	assert.Greater(t, len(packageTypes), 10, "Should support many package types")
}

func TestGetCriticalPath(t *testing.T) {
	graph := &DependencyGraph{
		Nodes: []ComponentNode{
			{
				ID:   "1",
				Name: "safe-component",
			},
			{
				ID:   "2",
				Name: "vulnerable-component",
				Issues: []VulnerabilityIssue{
					{Severity: "Critical"},
					{Severity: "High"},
				},
			},
			{
				ID:   "3",
				Name: "low-risk-component",
				Issues: []VulnerabilityIssue{
					{Severity: "Low"},
				},
			},
		},
	}

	path := GetCriticalPath(graph)
	assert.Len(t, path, 1)
	assert.Equal(t, "vulnerable-component", path[0].Name)
}

func TestComponentIntelligenceIntegration(t *testing.T) {
	logger := &observability.NoopLogger{}
	provider := NewXrayProvider(logger)

	// Verify all operations are accessible (component intelligence operations are automatically included)
	allOps := provider.GetOperationMappings()

	// Check that component intelligence operations are included
	assert.Contains(t, allOps, "components/searchByCves")
	assert.Contains(t, allOps, "graph/artifact")
	assert.Contains(t, allOps, "licenses/report")
	assert.Contains(t, allOps, "vulnerabilities/componentSummary")

	// Verify operation count increased
	assert.Greater(t, len(allOps), 40, "Should have many operations including component intelligence")
}

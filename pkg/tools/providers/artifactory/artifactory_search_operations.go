package artifactory

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// getEnhancedSearchOperations returns enhanced search operation mappings for Artifactory
// This implements Epic 4, Story 4.1: Enhance Existing Search Operations
func (p *ArtifactoryProvider) getEnhancedSearchOperations() map[string]providers.OperationMapping {
	return map[string]providers.OperationMapping{
		// Enhanced artifact search with additional parameters
		"search/artifacts": {
			OperationID:    "searchArtifacts",
			Method:         "GET",
			PathTemplate:   "/api/search/artifact",
			RequiredParams: []string{},
			OptionalParams: []string{
				"name",           // Artifact name pattern
				"repos",          // Comma-separated list of repositories
				"includeRemote",  // Include remote repositories
				"type",           // Artifact type (file/folder)
				"size",           // Size in bytes
				"created",        // Created date range
				"modified",       // Modified date range
				"lastDownloaded", // Last downloaded date
			},
		},

		// AQL search - already enhanced in Story 1.1
		"search/aql": {
			OperationID:    "searchAQL",
			Method:         "POST",
			PathTemplate:   "/api/search/aql",
			RequiredParams: []string{"query"},
			OptionalParams: []string{
				"limit",  // Limit number of results
				"offset", // Offset for pagination
			},
		},

		// GAVC search with complete parameters
		"search/gavc": {
			OperationID:    "searchGAVC",
			Method:         "GET",
			PathTemplate:   "/api/search/gavc",
			RequiredParams: []string{},
			OptionalParams: []string{
				"g",         // Group ID
				"a",         // Artifact ID
				"v",         // Version
				"c",         // Classifier
				"repos",     // Comma-separated list of repositories
				"recursive", // Search recursively
				"limit",     // Limit results
			},
		},

		// Enhanced property search with actual property parameters
		"search/property": {
			OperationID:    "searchByProperty",
			Method:         "GET",
			PathTemplate:   "/api/search/prop",
			RequiredParams: []string{},
			OptionalParams: []string{
				"p",         // Property key=value pairs (e.g., key1=value1,key2=value2)
				"repos",     // Comma-separated list of repositories
				"recursive", // Search recursively
			},
		},

		// Checksum search - already complete
		"search/checksum": {
			OperationID:    "searchByChecksum",
			Method:         "GET",
			PathTemplate:   "/api/search/checksum",
			RequiredParams: []string{},
			OptionalParams: []string{
				"md5",    // MD5 checksum
				"sha1",   // SHA1 checksum
				"sha256", // SHA256 checksum
				"repos",  // Comma-separated list of repositories
			},
		},

		// Enhanced pattern search
		"search/pattern": {
			OperationID:    "searchByPattern",
			Method:         "GET",
			PathTemplate:   "/api/search/pattern",
			RequiredParams: []string{"pattern"},
			OptionalParams: []string{
				"repos",     // Comma-separated list of repositories
				"recursive", // Search recursively
			},
		},

		// NEW: Search by creation/modification dates
		"search/dates": {
			OperationID:    "searchByDates",
			Method:         "GET",
			PathTemplate:   "/api/search/dates",
			RequiredParams: []string{},
			OptionalParams: []string{
				"dateFields", // created/lastModified
				"from",       // From date (ISO 8601)
				"to",         // To date (ISO 8601)
				"repos",      // Comma-separated list of repositories
			},
		},

		// NEW: Search for build artifacts
		"search/buildArtifacts": {
			OperationID:    "searchBuildArtifacts",
			Method:         "GET",
			PathTemplate:   "/api/search/buildArtifacts",
			RequiredParams: []string{},
			OptionalParams: []string{
				"buildName",   // Build name
				"buildNumber", // Build number
				"project",     // Project key
				"repos",       // Comma-separated list of repositories
			},
		},

		// NEW: Search for artifact dependencies
		"search/dependency": {
			OperationID:    "searchByDependency",
			Method:         "GET",
			PathTemplate:   "/api/search/dependency",
			RequiredParams: []string{},
			OptionalParams: []string{
				"sha1",  // SHA1 of the artifact to find dependencies for
				"repos", // Comma-separated list of repositories
			},
		},

		// NEW: Search for unused artifacts
		"search/usage": {
			OperationID:    "searchByUsage",
			Method:         "GET",
			PathTemplate:   "/api/search/usage",
			RequiredParams: []string{},
			OptionalParams: []string{
				"notUsedSince",  // ISO 8601 date
				"createdBefore", // ISO 8601 date
				"repos",         // Comma-separated list of repositories
			},
		},

		// NEW: Search for latest version
		// Note: This endpoint returns plain text version string, not JSON
		"search/latestVersion": {
			OperationID:    "searchLatestVersion",
			Method:         "GET",
			PathTemplate:   "/api/search/latestVersion",
			RequiredParams: []string{},
			OptionalParams: []string{
				"g",         // Group ID
				"a",         // Artifact ID
				"v",         // Version pattern
				"repos",     // Comma-separated list of repositories
				"remote",    // Include remote repositories
				"listFiles", // List files in the version
			},
			// Note: Response is text/plain, not JSON
		},

		// NEW: Search with stats
		"search/stats": {
			OperationID:    "searchWithStats",
			Method:         "GET",
			PathTemplate:   "/api/search/stats",
			RequiredParams: []string{},
			OptionalParams: []string{
				"name",      // Artifact name pattern
				"repos",     // Comma-separated list of repositories
				"statsOnly", // Return only statistics
			},
		},

		// NEW: Bad checksum search
		"search/badChecksum": {
			OperationID:    "searchBadChecksum",
			Method:         "GET",
			PathTemplate:   "/api/search/badChecksum",
			RequiredParams: []string{},
			OptionalParams: []string{
				"type",  // Checksum type (md5/sha1/sha256)
				"repos", // Comma-separated list of repositories
			},
		},

		// NEW: License search
		"search/license": {
			OperationID:    "searchByLicense",
			Method:         "GET",
			PathTemplate:   "/api/search/license",
			RequiredParams: []string{},
			OptionalParams: []string{
				"license",  // License name or pattern
				"repos",    // Comma-separated list of repositories
				"approved", // true/false - filter by approval status
				"unknown",  // Include artifacts with unknown licenses
			},
		},

		// NEW: Metadata search
		"search/metadata": {
			OperationID:    "searchByMetadata",
			Method:         "GET",
			PathTemplate:   "/api/search/metadata",
			RequiredParams: []string{},
			OptionalParams: []string{
				"metadata",  // Metadata key=value pairs
				"repos",     // Comma-separated list of repositories
				"recursive", // Search recursively
			},
		},
	}
}

// validateSearchParameters validates search operation parameters
func (p *ArtifactoryProvider) validateSearchParameters(operation string, params map[string]interface{}) error {
	switch operation {
	case "search/artifacts":
		// At least one search criterion should be provided
		if len(params) == 0 {
			return fmt.Errorf("at least one search parameter required (name, type, size, etc.)")
		}

	case "search/property":
		// Property search needs either 'p' parameter or property key-value pairs
		if _, hasP := params["p"]; !hasP {
			// Check if we have any property parameters
			hasProps := false
			for key := range params {
				if key != "repos" && key != "recursive" {
					hasProps = true
					break
				}
			}
			if !hasProps {
				return fmt.Errorf("property search requires property parameters (e.g., p=key1=value1)")
			}
		}

	case "search/dates":
		// Dates search needs at least from or to
		if _, hasFrom := params["from"]; !hasFrom {
			if _, hasTo := params["to"]; !hasTo {
				return fmt.Errorf("dates search requires 'from' or 'to' parameter")
			}
		}
		// Validate dateFields if provided
		if dateFields, ok := params["dateFields"].(string); ok {
			validFields := map[string]bool{"created": true, "lastModified": true}
			for _, field := range strings.Split(dateFields, ",") {
				if !validFields[strings.TrimSpace(field)] {
					return fmt.Errorf("invalid dateFields value: %s (must be 'created' or 'lastModified')", field)
				}
			}
		}

	case "search/buildArtifacts":
		// Build artifacts search needs at least buildName
		if _, hasBuildName := params["buildName"]; !hasBuildName {
			return fmt.Errorf("buildArtifacts search requires 'buildName' parameter")
		}

	case "search/dependency":
		// Dependency search needs sha1
		if _, hasSha1 := params["sha1"]; !hasSha1 {
			return fmt.Errorf("dependency search requires 'sha1' parameter")
		}

	case "search/usage":
		// Usage search needs at least one date parameter
		if _, hasNotUsed := params["notUsedSince"]; !hasNotUsed {
			if _, hasCreated := params["createdBefore"]; !hasCreated {
				return fmt.Errorf("usage search requires 'notUsedSince' or 'createdBefore' parameter")
			}
		}

	case "search/latestVersion":
		// Latest version search needs group and artifact
		if _, hasG := params["g"]; !hasG {
			return fmt.Errorf("latestVersion search requires 'g' (group) parameter")
		}
		if _, hasA := params["a"]; !hasA {
			return fmt.Errorf("latestVersion search requires 'a' (artifact) parameter")
		}

	case "search/badChecksum":
		// Bad checksum search should have a type
		if checkType, ok := params["type"].(string); ok {
			validTypes := map[string]bool{"md5": true, "sha1": true, "sha256": true}
			if !validTypes[checkType] {
				return fmt.Errorf("invalid checksum type: %s (must be md5, sha1, or sha256)", checkType)
			}
		}

	case "search/license":
		// License search should have at least license parameter
		if _, hasLicense := params["license"]; !hasLicense {
			// Check for approved or unknown flags
			if _, hasApproved := params["approved"]; !hasApproved {
				if _, hasUnknown := params["unknown"]; !hasUnknown {
					return fmt.Errorf("license search requires 'license', 'approved', or 'unknown' parameter")
				}
			}
		}

	case "search/metadata":
		// Metadata search needs metadata parameter
		if _, hasMetadata := params["metadata"]; !hasMetadata {
			return fmt.Errorf("metadata search requires 'metadata' parameter")
		}
	}

	return nil
}

// formatSearchURL formats the search URL with query parameters
func (p *ArtifactoryProvider) formatSearchURL(baseURL string, params map[string]interface{}) string {
	if len(params) == 0 {
		return baseURL
	}

	values := url.Values{}
	for key, value := range params {
		// Skip empty values
		if value == nil || value == "" {
			continue
		}

		// Convert value to string
		switch v := value.(type) {
		case string:
			values.Add(key, v)
		case bool:
			values.Add(key, fmt.Sprintf("%t", v))
		case int:
			values.Add(key, fmt.Sprintf("%d", v))
		case float64:
			values.Add(key, fmt.Sprintf("%.0f", v))
		default:
			values.Add(key, fmt.Sprintf("%v", v))
		}
	}

	if len(values) == 0 {
		return baseURL
	}

	return fmt.Sprintf("%s?%s", baseURL, values.Encode())
}

// GetSearchExamples returns example usage for search operations
func GetSearchExamples() map[string][]map[string]interface{} {
	return map[string][]map[string]interface{}{
		"search/artifacts": {
			{
				"description": "Search for JAR files in libs-release repo",
				"params": map[string]interface{}{
					"name":  "*.jar",
					"repos": "libs-release-local",
					"type":  "file",
				},
			},
			{
				"description": "Search for recently modified artifacts",
				"params": map[string]interface{}{
					"name":     "*",
					"modified": "2024-01-01",
					"repos":    "libs-release-local,libs-snapshot-local",
				},
			},
		},
		"search/property": {
			{
				"description": "Search for artifacts with specific build properties",
				"params": map[string]interface{}{
					"p":     "build.name=my-app,build.status=stable",
					"repos": "libs-release-local",
				},
			},
		},
		"search/dates": {
			{
				"description": "Search for artifacts created in last 7 days",
				"params": map[string]interface{}{
					"dateFields": "created",
					"from":       "2024-01-01T00:00:00Z",
					"to":         "2024-01-08T00:00:00Z",
					"repos":      "libs-release-local",
				},
			},
		},
		"search/usage": {
			{
				"description": "Find unused artifacts older than 6 months",
				"params": map[string]interface{}{
					"notUsedSince":  "2023-07-01T00:00:00Z",
					"createdBefore": "2023-07-01T00:00:00Z",
					"repos":         "libs-release-local",
				},
			},
		},
		"search/buildArtifacts": {
			{
				"description": "Find all artifacts from a specific build",
				"params": map[string]interface{}{
					"buildName":   "my-app",
					"buildNumber": "42",
					"repos":       "libs-release-local",
				},
			},
		},
		"search/latestVersion": {
			{
				"description": "Find latest version of a Maven artifact",
				"params": map[string]interface{}{
					"g":     "com.example",
					"a":     "my-library",
					"repos": "libs-release-local",
				},
			},
		},
	}
}

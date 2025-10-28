package artifactory

import (
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// GetEnhancedAIOptimizedDefinitions returns comprehensive AI-friendly tool definitions
// This implements Story 0.2: Enhance Operation Definitions for AI Discovery
func (p *ArtifactoryProvider) GetEnhancedAIOptimizedDefinitions() []providers.AIOptimizedToolDefinition {
	if p == nil {
		return nil
	}

	return []providers.AIOptimizedToolDefinition{
		// Artifact Management
		{
			Name:        "artifactory_artifacts",
			DisplayName: "Artifact and Package Management",
			Category:    "artifact_management",
			Subcategory: "storage",
			Description: "Upload, download, copy, move artifacts (JAR, npm, Docker layers). Use when: publishing builds, fetching dependencies, promoting releases.",
			DetailedHelp: `Manage individual artifacts and entire directory structures in Artifactory:
- Upload artifacts with automatic checksums and metadata extraction
- Download artifacts with optional property-based filtering
- Copy/move artifacts between repositories with transaction safety
- Set custom properties for search and workflow automation
- Manage artifact metadata and statistics
- Support for all package types with format-specific metadata`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Upload a JAR file with properties",
					Input: map[string]interface{}{
						"action": "upload",
						"parameters": map[string]interface{}{
							"repoKey":  "libs-release-local",
							"itemPath": "com/mycompany/myapp/1.0.0/myapp-1.0.0.jar",
							"properties": map[string]string{
								"build.name":   "myapp",
								"build.number": "123",
								"vcs.revision": "abc123def",
							},
						},
					},
					Explanation: "Uploads artifact with build metadata as properties for traceability",
				},
				{
					Scenario: "Copy artifacts with pattern matching",
					Input: map[string]interface{}{
						"action": "copy",
						"parameters": map[string]interface{}{
							"srcRepoKey":      "libs-snapshot-local",
							"srcItemPath":     "com/mycompany/myapp/1.0.0-SNAPSHOT/",
							"targetRepoKey":   "libs-release-local",
							"targetItemPath":  "com/mycompany/myapp/1.0.0/",
							"dry":             false,
							"suppressLayouts": false,
							"failFast":        true,
						},
					},
					Explanation: "Promotes snapshot artifacts to release repository with path transformation",
				},
				{
					Scenario: "Set properties recursively on directory",
					Input: map[string]interface{}{
						"action": "properties/set",
						"parameters": map[string]interface{}{
							"repoKey":  "libs-release-local",
							"itemPath": "com/mycompany/",
							"properties": map[string]string{
								"retention.days": "365",
								"quality.gate":   "passed",
								"environment":    "production",
							},
							"recursive": true,
						},
					},
					Explanation: "Applies properties to all artifacts in a directory tree for policy enforcement",
				},
			},
			SemanticTags: []string{
				"artifact", "file", "package", "binary", "upload", "download",
				"deploy", "fetch", "copy", "move", "promote", "delete",
				"properties", "metadata", "checksum", "sha256", "md5",
				"storage", "repository", "path",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "The artifact operation to perform",
						Examples:    []interface{}{"upload", "download", "info", "copy", "move", "delete", "properties/set", "properties/delete"},
					},
					"parameters": {
						Type:        "object",
						Description: "Operation-specific parameters",
						Properties: map[string]providers.AIPropertySchema{
							"repoKey": {
								Type:        "string",
								Description: "Repository containing the artifact",
								Examples:    []interface{}{"libs-release-local", "npm-local", "docker-local"},
							},
							"itemPath": {
								Type:        "string",
								Description: "Path to artifact within repository",
								Examples:    []interface{}{"com/mycompany/myapp/1.0.0/myapp-1.0.0.jar", "mypackage/-/mypackage-1.0.0.tgz"},
								Template:    "package/path/version/filename",
							},
							"properties": {
								Type:        "object",
								Description: "Custom properties to set on artifact",
								Examples:    []interface{}{map[string]string{"build.number": "123", "quality": "verified"}},
							},
							"recursive": {
								Type:         "boolean",
								Description:  "Apply operation to all children (for directories)",
								SmartDefault: "false",
							},
							"dry": {
								Type:         "boolean",
								Description:  "Simulate operation without making changes",
								SmartDefault: "false",
							},
						},
					},
				},
			},
		},

		// Search Operations
		{
			Name:        "artifactory_search",
			DisplayName: "Advanced Artifact Search",
			Category:    "search",
			Subcategory: "query",
			Description: "Search artifacts by name, checksum, properties, or Maven coordinates. Use when: finding dependencies, troubleshooting missing artifacts.",
			DetailedHelp: `Search for artifacts using multiple methods:
- AQL (Artifactory Query Language): Most powerful, supports complex queries with multiple criteria
- GAVC: Search Maven artifacts by Group, Artifact, Version, Classifier
- Properties: Find artifacts by custom properties
- Checksum: Locate artifacts by MD5, SHA1, or SHA256
- Pattern: Simple wildcard searches
- Quick search: Fast searches by name patterns`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Complex AQL search for recent large artifacts",
					Input: map[string]interface{}{
						"action": "aql",
						"parameters": map[string]interface{}{
							"query": `items.find({
								"repo": {"$match": "libs-*"},
								"created": {"$gt": "2025-01-01"},
								"size": {"$gt": 10485760},
								"name": {"$match": "*.jar"}
							}).include("repo", "path", "name", "size", "created", "modified_by", "property")`,
						},
					},
					Explanation: "Find JAR files over 10MB created after Jan 1, 2025 in libs repositories with full metadata",
				},
				{
					Scenario: "Search Maven artifacts by coordinates",
					Input: map[string]interface{}{
						"action": "gavc",
						"parameters": map[string]interface{}{
							"g":     "com.mycompany",
							"a":     "myapp",
							"v":     "1.0.*",
							"c":     "sources",
							"repos": "libs-release-local,libs-snapshot-local",
						},
					},
					Explanation: "Find all 1.0.x versions of myapp with sources classifier in specified repositories",
				},
				{
					Scenario: "Find artifacts by build properties",
					Input: map[string]interface{}{
						"action": "property",
						"parameters": map[string]interface{}{
							"properties": map[string]string{
								"build.name":   "myapp",
								"build.number": "123",
							},
							"repos": "libs-release-local",
						},
					},
					Explanation: "Locate all artifacts from specific build for traceability",
				},
				{
					Scenario: "Find duplicate artifacts by checksum",
					Input: map[string]interface{}{
						"action": "checksum",
						"parameters": map[string]interface{}{
							"sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							"repos":  "*-local",
						},
					},
					Explanation: "Find all copies of an artifact across local repositories using SHA256",
				},
			},
			SemanticTags: []string{
				"search", "find", "query", "locate", "discover", "aql",
				"gavc", "maven", "coordinates", "checksum", "sha1", "sha256", "md5",
				"properties", "metadata", "pattern", "wildcard", "filter",
				"created", "modified", "size", "name",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Search method to use",
						Examples:    []interface{}{"aql", "artifacts", "gavc", "property", "checksum", "pattern"},
					},
					"parameters": {
						Type:        "object",
						Description: "Search-specific parameters",
						Properties: map[string]providers.AIPropertySchema{
							"query": {
								Type:        "string",
								Description: "AQL query string (for aql action)",
								Template:    "items.find({criteria}).include(fields)",
							},
							"name": {
								Type:        "string",
								Description: "Artifact name pattern (wildcards supported)",
								Examples:    []interface{}{"*.jar", "myapp-*.zip", "lib-*"},
							},
							"repos": {
								Type:        "string",
								Description: "Comma-separated list of repositories to search",
								Examples:    []interface{}{"*-local", "libs-release-local,libs-snapshot-local"},
							},
							"g": {
								Type:        "string",
								Description: "Maven group ID (for GAVC search)",
								Examples:    []interface{}{"com.mycompany", "org.springframework"},
							},
							"a": {
								Type:        "string",
								Description: "Maven artifact ID (for GAVC search)",
							},
							"v": {
								Type:        "string",
								Description: "Maven version (supports wildcards)",
								Examples:    []interface{}{"1.0.0", "1.0.*", "[1.0,2.0)"},
							},
							"properties": {
								Type:        "object",
								Description: "Properties to search by",
							},
							"md5": {
								Type:        "string",
								Description: "MD5 checksum",
								Template:    "^[a-fA-F0-9]{32}$",
							},
							"sha1": {
								Type:        "string",
								Description: "SHA1 checksum",
								Template:    "^[a-fA-F0-9]{40}$",
							},
							"sha256": {
								Type:        "string",
								Description: "SHA256 checksum",
								Template:    "^[a-fA-F0-9]{64}$",
							},
						},
					},
				},
				AIHints: &providers.AIParameterHints{
					ConditionalRequirements: []providers.ConditionalRequirement{
						{If: "action=aql", Then: "query is required"},
						{If: "action=gavc", Then: "at least one of g,a,v,c is required"},
						{If: "action=checksum", Then: "one of md5,sha1,sha256 is required"},
						{If: "action=property", Then: "properties is required"},
					},
				},
			},
		},

		// Build Management
		{
			Name:        "artifactory_builds",
			DisplayName: "CI/CD Build Management",
			Category:    "ci_cd",
			Subcategory: "build_info",
			Description: "Publish build metadata, promote builds between environments. Use when: CI/CD pipeline publishing, release promotion.",
			DetailedHelp: `Integrate with CI/CD pipelines to track build artifacts and dependencies:
- Store comprehensive build information including environment, tools, and dependencies
- Promote builds through environments (dev → staging → production)
- Track build artifacts and their properties
- Link builds to source control commits and issues
- Support for Jenkins, TeamCity, Bamboo, Azure DevOps, GitHub Actions, and more`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Upload build information from CI pipeline",
					Input: map[string]interface{}{
						"action": "upload",
						"parameters": map[string]interface{}{
							"buildInfo": map[string]interface{}{
								"name":    "myapp",
								"number":  "123",
								"started": "2025-01-28T10:00:00.000Z",
								"modules": []map[string]interface{}{
									{
										"id": "com.mycompany:myapp:1.0.0",
										"artifacts": []map[string]interface{}{
											{
												"type": "jar",
												"sha1": "abc123def456",
												"md5":  "123456789",
												"name": "myapp-1.0.0.jar",
											},
										},
									},
								},
								"buildAgent": map[string]string{
									"name":    "Jenkins",
									"version": "2.400",
								},
								"vcs": []map[string]string{
									{
										"revision": "abc123def",
										"url":      "https://github.com/mycompany/myapp.git",
									},
								},
							},
						},
					},
					Explanation: "Publishes comprehensive build information including artifacts, VCS info, and build environment",
				},
				{
					Scenario: "Promote build to production",
					Input: map[string]interface{}{
						"action": "promote",
						"parameters": map[string]interface{}{
							"buildName":    "myapp",
							"buildNumber":  "123",
							"targetRepo":   "libs-prod-local",
							"status":       "Released",
							"comment":      "Promoted to production after QA approval",
							"ciUser":       "jenkins",
							"copy":         true,
							"dependencies": true,
							"properties": map[string]string{
								"release.version": "1.0.0",
								"release.date":    "2025-01-28",
							},
						},
					},
					Explanation: "Promotes build artifacts to production repository with release metadata",
				},
			},
			SemanticTags: []string{
				"build", "ci", "cd", "pipeline", "jenkins", "teamcity",
				"promote", "promotion", "release", "deployment", "staging",
				"buildinfo", "artifacts", "dependencies", "modules",
				"vcs", "git", "commit", "revision",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Build operation to perform",
						Examples:    []interface{}{"list", "get", "runs", "upload", "promote", "delete"},
					},
					"parameters": {
						Type:        "object",
						Description: "Build-specific parameters",
						Properties: map[string]providers.AIPropertySchema{
							"buildName": {
								Type:        "string",
								Description: "Name of the build (typically project name)",
								Examples:    []interface{}{"myapp", "backend-service", "web-ui"},
							},
							"buildNumber": {
								Type:        "string",
								Description: "Build number (unique per build name)",
								Examples:    []interface{}{"123", "2025.01.28.1", "v1.0.0-build.456"},
							},
							"targetRepo": {
								Type:        "string",
								Description: "Target repository for promotion",
								Examples:    []interface{}{"libs-prod-local", "docker-prod-local"},
							},
							"status": {
								Type:        "string",
								Description: "Build or promotion status",
								Examples:    []interface{}{"Released", "Staged", "Rolled-back"},
							},
						},
					},
				},
			},
		},

		// Docker Registry Operations
		{
			Name:        "artifactory_docker",
			DisplayName: "Docker Registry Operations",
			Category:    "container_management",
			Subcategory: "docker",
			Description: "List Docker images and tags in registry. Use when: checking available images, finding image versions.",
			DetailedHelp: `Artifactory as a Docker Registry:
- Host private Docker images with enterprise security
- Proxy Docker Hub and other registries with intelligent caching
- Virtual registries for simplified access to multiple sources
- Support for manifest lists and multi-architecture images
- Integration with Docker CLI, Kubernetes, and CI/CD tools
- Vulnerability scanning with Xray integration`,
			UsageExamples: []providers.Example{
				{
					Scenario: "List Docker images in repository",
					Input: map[string]interface{}{
						"action": "repositories",
						"parameters": map[string]interface{}{
							"repoKey": "docker-local",
							"n":       100,
							"last":    "",
						},
					},
					Explanation: "Lists up to 100 Docker images in the docker-local repository",
				},
				{
					Scenario: "List tags for a Docker image",
					Input: map[string]interface{}{
						"action": "tags",
						"parameters": map[string]interface{}{
							"repoKey":   "docker-local",
							"imagePath": "mycompany/myapp",
							"n":         50,
						},
					},
					Explanation: "Lists up to 50 tags for the mycompany/myapp image",
				},
			},
			SemanticTags: []string{
				"docker", "container", "image", "registry", "tag", "layer",
				"manifest", "oci", "containerization", "kubernetes", "k8s",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Docker operation to perform",
						Examples:    []interface{}{"repositories", "tags"},
					},
					"parameters": {
						Type:        "object",
						Description: "Docker-specific parameters",
						Properties: map[string]providers.AIPropertySchema{
							"repoKey": {
								Type:        "string",
								Description: "Docker repository key",
								Examples:    []interface{}{"docker-local", "docker-hub-remote"},
							},
							"imagePath": {
								Type:        "string",
								Description: "Docker image path",
								Examples:    []interface{}{"library/nginx", "mycompany/myapp"},
							},
							"n": {
								Type:         "integer",
								Description:  "Number of results to return",
								SmartDefault: "100",
							},
						},
					},
				},
			},
		},

		// Internal Helper Operations
		{
			Name:        "artifactory_helpers",
			DisplayName: "AI-Optimized Helper Operations",
			Category:    "internal",
			Subcategory: "helpers",
			Description: "Get current user identity and available features. Use when: checking permissions.",
			DetailedHelp: `Helper operations that simplify complex Artifactory tasks:
- Get current user: Identifies the authenticated user (handles the 2-step API process)
- Check available features: Probes what JFrog components are installed and accessible
- These operations encapsulate complex logic that would be difficult for AI agents to orchestrate`,
			UsageExamples: []providers.Example{
				{
					Scenario: "Get current authenticated user details",
					Input: map[string]interface{}{
						"action": "internal/current-user",
					},
					Explanation: "Automatically handles the complex process of identifying the current user",
				},
				{
					Scenario: "Check what JFrog features are available",
					Input: map[string]interface{}{
						"action": "internal/available-features",
					},
					ExpectedOutput: `{
						"artifactory": {"available": true, "status": "active"},
						"xray": {"available": true, "status": "active"},
						"pipelines": {"available": false, "reason": "not installed"},
						"repository_types": {"local": {"supported": true, "count": 15}}
					}`,
					Explanation: "Probes various endpoints to determine what features are available",
				},
			},
			SemanticTags: []string{
				"helper", "internal", "user", "whoami", "identity", "features",
				"capabilities", "available", "probe", "check", "discover",
			},
			InputSchema: providers.AIParameterSchema{
				Type: "object",
				Properties: map[string]providers.AIPropertySchema{
					"action": {
						Type:        "string",
						Description: "Helper operation to perform",
						Examples:    []interface{}{"internal/current-user", "internal/available-features"},
					},
				},
				Required: []string{"action"},
			},
		},
	}
}

// GetAQLTemplates returns common AQL query templates for AI agents
// This helps AI agents construct valid AQL queries without syntax errors
// NOTE: Consider using AQLQueryBuilder for programmatic query construction
func GetAQLTemplates() map[string]string {
	return map[string]string{
		// Basic templates
		"find-by-name": `items.find({"name": {"$match": "%s"}})`,
		"find-by-repo": `items.find({"repo": "%s"})`,
		"find-by-path": `items.find({"path": {"$match": "%s"}})`,

		// Date-based templates
		"find-recent":     `items.find({"modified": {"$gt": "%s"}})`,
		"find-old":        `items.find({"created": {"$lt": "%s"}})`,
		"find-date-range": `items.find({"created": {"$gt": "%s", "$lt": "%s"}})`,

		// Size-based templates
		"find-large-files": `items.find({"size": {"$gt": %d}})`,
		"find-small-files": `items.find({"size": {"$lt": %d}})`,
		"find-size-range":  `items.find({"size": {"$gt": %d, "$lt": %d}})`,

		// Checksum templates
		"find-by-sha1":   `items.find({"actual_sha1": "%s"})`,
		"find-by-sha256": `items.find({"sha256": "%s"})`,
		"find-by-md5":    `items.find({"actual_md5": "%s"})`,

		// Property templates
		"find-by-property":     `items.find({"@%s": "%s"})`,
		"find-by-properties":   `items.find({"@%s": "%s", "@%s": "%s"})`,
		"find-property-exists": `items.find({"@%s": {"$match": "*"}})`,

		// Complex templates
		"find-maven-artifacts": `items.find({"repo": {"$match": "*maven*"}, "name": {"$match": "*.jar"}})`,
		"find-docker-images":   `items.find({"repo": {"$match": "*docker*"}, "name": "manifest.json"})`,
		"find-npm-packages":    `items.find({"repo": {"$match": "*npm*"}, "name": {"$match": "*.tgz"}})`,

		// With includes
		"find-with-properties": `items.find(%s).include("property")`,
		"find-with-stats":      `items.find(%s).include("stat")`,
		"find-with-all":        `items.find(%s).include("*")`,

		// Sorting and limiting
		"find-sorted":  `items.find(%s).sort({"$desc": ["size"]})`,
		"find-limited": `items.find(%s).limit(%d)`,
		"find-offset":  `items.find(%s).offset(%d).limit(%d)`,
	}
}

// GetAQLBuilderExamples returns examples of using the AQLQueryBuilder
// This helps AI agents understand how to programmatically build AQL queries
func GetAQLBuilderExamples() map[string]string {
	examples := make(map[string]string)

	// Get the common examples from the builder
	builderExamples := GetCommonAQLExamples()

	// Convert each example to its query string
	for name, builderFunc := range builderExamples {
		builder := builderFunc()
		query, _ := builder.Build()
		examples[name] = query
	}

	// Add descriptions for AI agents
	examples["_description"] = "Use AQLQueryBuilder for safe, programmatic query construction"
	examples["_usage"] = "builder := NewAQLQueryBuilder().FindItemsByRepo('libs-release').FindItemsByName('*.jar').Build()"

	return examples
}

// GetErrorResolutions returns common error patterns and their resolutions
// This helps AI agents provide actionable solutions when operations fail
func GetErrorResolutions() map[string]string {
	return map[string]string{
		"401 Unauthorized":               "Check that your API key or token is valid and not expired. Use 'internal/current-user' to verify authentication.",
		"403 Forbidden":                  "You don't have permission for this operation. Check your user permissions or use 'internal/available-features' to see what's accessible.",
		"404 Not Found":                  "The requested resource doesn't exist. Verify the repository key, artifact path, or username is correct.",
		"409 Conflict":                   "Resource already exists. Use a different name or delete the existing resource first.",
		"400 Bad Request":                "Invalid parameters. Check that all required parameters are provided and properly formatted.",
		"500 Internal Server Error":      "Artifactory server error. Check system health with 'system/ping' and retry the operation.",
		"Repository does not exist":      "The specified repository was not found. Use 'repos/list' to see available repositories.",
		"Package type cannot be changed": "Cannot change package type after repository creation. Delete and recreate the repository.",
		"Invalid repository key":         "Repository keys must be lowercase alphanumeric with hyphens and underscores only.",
		"Invalid AQL query":              "AQL syntax error. Use AQLQueryBuilder for safe query construction, or GetAQLTemplates() for templates. Call ValidateAQLQuery() to check syntax.",
		"Xray not available":             "Xray is not installed or not accessible. Use 'internal/available-features' to check what's available.",
		"Token expired":                  "Access token has expired. Generate a new token using 'tokens/create' operation.",
		"Insufficient storage":           "Server is running low on storage. Contact administrator or use 'system/storage' to check usage.",
	}
}

// GetCapabilityDescriptions returns detailed capability descriptions
// This helps AI agents understand what operations are possible and their limitations
func GetCapabilityDescriptions() map[string]providers.Capability {
	return map[string]providers.Capability{
		"repository_management": {
			Action:   "manage",
			Resource: "repositories",
			Constraints: []string{
				"Create, configure, and manage all repository types (local, remote, virtual, federated)",
			},
		},
		"artifact_operations": {
			Action:   "manage",
			Resource: "artifacts",
			Constraints: []string{
				"Upload, download, copy, move, delete artifacts with transactional safety",
			},
		},
		"property_management": {
			Action:   "manage",
			Resource: "properties",
			Constraints: []string{
				"Set and delete custom properties on artifacts for metadata and workflow automation",
			},
		},
		"advanced_search": {
			Action:   "search",
			Resource: "artifacts",
			Constraints: []string{
				"Search using AQL, GAVC, properties, checksums, and patterns across repositories",
			},
		},
		"build_integration": {
			Action:   "manage",
			Resource: "builds",
			Constraints: []string{
				"Store build info, promote builds, track artifacts and dependencies from CI/CD",
			},
		},
		"security_management": {
			Action:   "manage",
			Resource: "security",
			Constraints: []string{
				"Manage users, groups, permissions, and access tokens for RBAC",
			},
		},
		"docker_registry": {
			Action:   "manage",
			Resource: "docker_images",
			Constraints: []string{
				"Full Docker registry capabilities with image and tag management",
			},
		},
		"multi_protocol": {
			Action:   "support",
			Resource: "package_formats",
			Constraints: []string{
				"Support for Maven, npm, Docker, PyPI, NuGet, Helm, Go, and more package formats",
			},
		},
	}
}

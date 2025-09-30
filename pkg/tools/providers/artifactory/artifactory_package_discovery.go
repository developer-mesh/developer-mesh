package artifactory

import (
	"fmt"
	"path"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/tools/providers"
)

// getPackageDiscoveryOperations returns package discovery operation mappings for Artifactory
// This implements Epic 4, Story 4.2: Simplify Package Discovery
func (p *ArtifactoryProvider) getPackageDiscoveryOperations() map[string]providers.OperationMapping {
	return map[string]providers.OperationMapping{
		// Core package operations using storage API
		"packages/info": {
			OperationID:    "getPackageInfo",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{packagePath}",
			RequiredParams: []string{"repoKey", "packagePath"},
			OptionalParams: []string{"properties", "lastModified", "statsOnly"},
			// Returns package metadata including size, checksums, created, modified dates
		},

		"packages/versions": {
			OperationID:    "listPackageVersions",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{packagePath}",
			RequiredParams: []string{"repoKey", "packagePath"},
			OptionalParams: []string{"list", "deep", "includeRootPath", "depth", "listFolders", "mdTimestamps"},
			// Lists all versions of a package using deep directory listing
			// Note: Set list=true and deep=1 for version listing
		},

		"packages/latest": {
			OperationID:    "getLatestPackageVersion",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{packagePath}",
			RequiredParams: []string{"repoKey", "packagePath"},
			OptionalParams: []string{"list", "deep", "lastModified", "preRelease", "releaseStatus"},
			// Gets the latest version based on modification time
			// Note: Set list=true, deep=1, lastModified=true
		},

		"packages/stats": {
			OperationID:    "getPackageStatistics",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{packagePath}",
			RequiredParams: []string{"repoKey", "packagePath"},
			OptionalParams: []string{"stats"},
			// Returns download statistics for the package
			// Note: Set stats=true
		},

		"packages/properties": {
			OperationID:    "getPackageProperties",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{packagePath}",
			RequiredParams: []string{"repoKey", "packagePath"},
			OptionalParams: []string{"properties", "propertyName"},
			// Returns all properties set on the package
			// Note: Set properties=true or properties=[comma-separated names]
		},

		// Maven-specific package operations
		"packages/maven/info": {
			OperationID:    "getMavenPackageInfo",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{groupId}/{artifactId}",
			RequiredParams: []string{"repoKey", "groupId", "artifactId"},
			OptionalParams: []string{"version"},
			// Gets Maven package info using GAV coordinates
		},

		"packages/maven/versions": {
			OperationID:    "listMavenVersions",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{groupId}/{artifactId}",
			RequiredParams: []string{"repoKey", "groupId", "artifactId"},
			OptionalParams: []string{"list", "deep", "includeSnapshots", "includePrereleases"},
			// Lists all versions of a Maven artifact
		},

		"packages/maven/pom": {
			OperationID:    "getMavenPom",
			Method:         "GET",
			PathTemplate:   "/{repoKey}/{groupId}/{artifactId}/{version}/{artifactId}-{version}.pom",
			RequiredParams: []string{"repoKey", "groupId", "artifactId", "version"},
			// Downloads the POM file for a Maven artifact
		},

		// NPM-specific package operations
		"packages/npm/info": {
			OperationID:    "getNpmPackageInfo",
			Method:         "GET",
			PathTemplate:   "/api/npm/{repoKey}/{packageName}",
			RequiredParams: []string{"repoKey", "packageName"},
			// Gets NPM package metadata including all versions
		},

		"packages/npm/versions": {
			OperationID:    "listNpmVersions",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{scope}/{packageName}",
			RequiredParams: []string{"repoKey", "packageName"},
			OptionalParams: []string{"list", "deep", "scope"},
			// Lists all versions of an NPM package
		},

		"packages/npm/tarball": {
			OperationID:    "getNpmTarball",
			Method:         "GET",
			PathTemplate:   "/{repoKey}/{packageName}/-/{packageName}-{version}.tgz",
			RequiredParams: []string{"repoKey", "packageName", "version"},
			OptionalParams: []string{"scope"},
			// Downloads NPM package tarball
		},

		// Docker-specific package operations
		"packages/docker/info": {
			OperationID:    "getDockerImageInfo",
			Method:         "GET",
			PathTemplate:   "/api/docker/{repoKey}/v2/{imageName}/manifests/{tag}",
			RequiredParams: []string{"repoKey", "imageName", "tag"},
			// Gets Docker image manifest
		},

		"packages/docker/tags": {
			OperationID:    "listDockerTags",
			Method:         "GET",
			PathTemplate:   "/api/docker/{repoKey}/v2/{imageName}/tags/list",
			RequiredParams: []string{"repoKey", "imageName"},
			OptionalParams: []string{"n", "last"},
			// Lists all tags for a Docker image
		},

		"packages/docker/layers": {
			OperationID:    "getDockerLayers",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{imageName}/{tag}?list&deep=1",
			RequiredParams: []string{"repoKey", "imageName", "tag"},
			// Lists all layers of a Docker image
		},

		// PyPI-specific package operations
		"packages/pypi/info": {
			OperationID:    "getPypiPackageInfo",
			Method:         "GET",
			PathTemplate:   "/api/pypi/{repoKey}/simple/{packageName}",
			RequiredParams: []string{"repoKey", "packageName"},
			// Gets PyPI package information
		},

		"packages/pypi/versions": {
			OperationID:    "listPypiVersions",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{packageName}?list&deep=1",
			RequiredParams: []string{"repoKey", "packageName"},
			// Lists all versions of a PyPI package
		},

		// NuGet-specific package operations
		"packages/nuget/info": {
			OperationID:    "getNugetPackageInfo",
			Method:         "GET",
			PathTemplate:   "/api/nuget/{repoKey}/Packages(Id='{packageId}',Version='{version}')",
			RequiredParams: []string{"repoKey", "packageId"},
			OptionalParams: []string{"version"},
			// Gets NuGet package metadata
		},

		"packages/nuget/versions": {
			OperationID:    "listNugetVersions",
			Method:         "GET",
			PathTemplate:   "/api/nuget/{repoKey}/FindPackagesById()?id='{packageId}'",
			RequiredParams: []string{"repoKey", "packageId"},
			// Lists all versions of a NuGet package
		},

		// Generic package operations (for any package type)
		"packages/search": {
			OperationID:    "searchPackages",
			Method:         "GET",
			PathTemplate:   "/api/search/artifact",
			RequiredParams: []string{},
			OptionalParams: []string{"name", "repos", "type", "packageType"},
			// Search for packages across repositories
		},

		"packages/dependencies": {
			OperationID:    "getPackageDependencies",
			Method:         "GET",
			PathTemplate:   "/api/storage/{repoKey}/{packagePath}?properties=dependency.*",
			RequiredParams: []string{"repoKey", "packagePath"},
			// Get package dependencies from properties
		},

		"packages/dependents": {
			OperationID:    "getPackageDependents",
			Method:         "GET",
			PathTemplate:   "/api/search/dependency?sha1={sha1}",
			RequiredParams: []string{"sha1"},
			OptionalParams: []string{"repos"},
			// Find packages that depend on this package
		},
	}
}

// PackageInfo represents information about a package
type PackageInfo struct {
	URI          string                 `json:"uri"`
	DownloadURI  string                 `json:"downloadUri,omitempty"`
	Repo         string                 `json:"repo"`
	Path         string                 `json:"path"`
	Created      string                 `json:"created,omitempty"`
	CreatedBy    string                 `json:"createdBy,omitempty"`
	LastModified string                 `json:"lastModified,omitempty"`
	ModifiedBy   string                 `json:"modifiedBy,omitempty"`
	LastUpdated  string                 `json:"lastUpdated,omitempty"`
	Size         string                 `json:"size,omitempty"`
	MimeType     string                 `json:"mimeType,omitempty"`
	Checksums    map[string]string      `json:"checksums,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
	Children     []PackageChild         `json:"children,omitempty"`
}

// PackageChild represents a child item in a package listing
type PackageChild struct {
	URI    string `json:"uri"`
	Folder bool   `json:"folder"`
	Size   int64  `json:"size,omitempty"`
}

// PackageVersion represents a package version
type PackageVersion struct {
	Version      string `json:"version"`
	Released     string `json:"released,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Downloads    int    `json:"downloads,omitempty"`
	URI          string `json:"uri,omitempty"`
	IsPreRelease bool   `json:"isPreRelease,omitempty"`
	IsSnapshot   bool   `json:"isSnapshot,omitempty"`
}

// formatPackagePath formats a package path based on package type
func (p *ArtifactoryProvider) formatPackagePath(packageType, packageName string, options map[string]string) string {
	switch strings.ToLower(packageType) {
	case "maven":
		// Format: groupId/artifactId/version
		if groupId, ok := options["groupId"]; ok {
			groupId = strings.ReplaceAll(groupId, ".", "/")
			if artifactId, ok := options["artifactId"]; ok {
				path := fmt.Sprintf("%s/%s", groupId, artifactId)
				if version, ok := options["version"]; ok {
					path = fmt.Sprintf("%s/%s", path, version)
				}
				return path
			}
		}
		return packageName

	case "npm":
		// Format: @scope/package-name or package-name
		if scope, ok := options["scope"]; ok {
			return fmt.Sprintf("%s/%s", scope, packageName)
		}
		return packageName

	case "docker":
		// Format: imageName/tag
		if tag, ok := options["tag"]; ok {
			return fmt.Sprintf("%s/%s", packageName, tag)
		}
		return packageName

	case "pypi":
		// Format: package-name
		return strings.ToLower(strings.ReplaceAll(packageName, "_", "-"))

	case "nuget":
		// Format: packageId/version
		if version, ok := options["version"]; ok {
			return fmt.Sprintf("%s/%s", packageName, version)
		}
		return packageName

	case "go":
		// Format: module-path
		return packageName

	case "helm":
		// Format: chart-name
		return packageName

	default:
		// Generic format
		return packageName
	}
}

// parsePackageVersions parses version information from a storage listing response
func (p *ArtifactoryProvider) parsePackageVersions(response interface{}, packageType string) ([]PackageVersion, error) {
	var versions []PackageVersion

	// Parse response based on expected format
	if pkgInfo, ok := response.(*PackageInfo); ok {
		// Process children for version directories
		for _, child := range pkgInfo.Children {
			if child.Folder {
				version := extractVersionFromPath(child.URI, packageType)
				if version != "" {
					versions = append(versions, PackageVersion{
						Version: version,
						URI:     child.URI,
						Size:    child.Size,
					})
				}
			}
		}
	} else if responseMap, ok := response.(map[string]interface{}); ok {
		// Handle different response formats based on package type
		if children, exists := responseMap["children"]; exists {
			if childList, ok := children.([]interface{}); ok {
				for _, child := range childList {
					if childMap, ok := child.(map[string]interface{}); ok {
						if uri, hasURI := childMap["uri"].(string); hasURI {
							if folder, isFolder := childMap["folder"].(bool); !isFolder || folder {
								version := extractVersionFromPath(uri, packageType)
								if version != "" {
									pv := PackageVersion{
										Version: version,
										URI:     uri,
									}
									if size, hasSize := childMap["size"].(float64); hasSize {
										pv.Size = int64(size)
									}
									versions = append(versions, pv)
								}
							}
						}
					}
				}
			}
		}
	}

	// Sort versions by semantic versioning if applicable
	sortPackageVersions(versions, packageType)

	return versions, nil
}

// extractVersionFromPath extracts version from a URI path based on package type
func extractVersionFromPath(uri, packageType string) string {
	// Clean up URI
	uri = strings.TrimPrefix(uri, "/")
	uri = strings.TrimSuffix(uri, "/")

	switch strings.ToLower(packageType) {
	case "maven":
		// Extract version from Maven path
		parts := strings.Split(uri, "/")
		if len(parts) > 0 {
			// Version is typically the last directory
			version := parts[len(parts)-1]
			// Validate it looks like a version
			if isValidVersion(version) {
				return version
			}
		}

	case "npm":
		// NPM versions are in directories named like the version
		base := path.Base(uri)
		if isValidVersion(base) {
			return base
		}

	case "docker":
		// Docker tags are the version
		base := path.Base(uri)
		return base

	default:
		// Try to extract version from path
		base := path.Base(uri)
		if isValidVersion(base) {
			return base
		}
	}

	return ""
}

// isValidVersion checks if a string looks like a version
func isValidVersion(s string) bool {
	// Basic version validation
	if s == "" || s == "." || s == ".." {
		return false
	}

	// Check for common version patterns
	// Semantic versioning: X.Y.Z
	// Maven snapshots: X.Y.Z-SNAPSHOT
	// Pre-releases: X.Y.Z-alpha, X.Y.Z-beta, X.Y.Z-rc.1
	// Date versions: 20240101, 2024.01.01

	// Must contain at least one digit
	hasDigit := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			hasDigit = true
			break
		}
	}

	return hasDigit
}

// sortPackageVersions sorts versions based on package type conventions
func sortPackageVersions(versions []PackageVersion, packageType string) {
	// This is a simplified version sorter
	// In production, you'd want to use a proper semantic version library
	// For now, we'll leave the versions in the order they were returned
}

// getPackageTypeFromRepo attempts to determine package type from repository key
func (p *ArtifactoryProvider) getPackageTypeFromRepo(repoKey string) string {
	// Common repository naming patterns
	patterns := map[string][]string{
		"maven":   {"maven", "libs", "releases", "snapshots"},
		"npm":     {"npm", "node"},
		"docker":  {"docker", "containers"},
		"pypi":    {"pypi", "python"},
		"nuget":   {"nuget", "dotnet"},
		"go":      {"go", "golang"},
		"helm":    {"helm", "charts"},
		"generic": {"generic", "files"},
	}

	lowerRepo := strings.ToLower(repoKey)
	for packageType, keywords := range patterns {
		for _, keyword := range keywords {
			if strings.Contains(lowerRepo, keyword) {
				return packageType
			}
		}
	}

	return "generic"
}

// GetPackageDiscoveryExamples returns example usage for package discovery operations
func GetPackageDiscoveryExamples() map[string][]map[string]interface{} {
	return map[string][]map[string]interface{}{
		"packages/info": {
			{
				"description": "Get info for a Maven package",
				"params": map[string]interface{}{
					"repoKey":     "maven-central",
					"packagePath": "org/springframework/spring-core",
				},
			},
			{
				"description": "Get info for an NPM package",
				"params": map[string]interface{}{
					"repoKey":     "npm-local",
					"packagePath": "@angular/core",
				},
			},
		},
		"packages/versions": {
			{
				"description": "List all versions of a Maven artifact",
				"params": map[string]interface{}{
					"repoKey":     "libs-release",
					"packagePath": "com/example/my-library",
				},
			},
			{
				"description": "List Docker image tags",
				"params": map[string]interface{}{
					"repoKey":     "docker-local",
					"packagePath": "my-app",
				},
			},
		},
		"packages/maven/versions": {
			{
				"description": "List Maven artifact versions",
				"params": map[string]interface{}{
					"repoKey":    "maven-central",
					"groupId":    "org.springframework",
					"artifactId": "spring-core",
				},
			},
		},
		"packages/npm/info": {
			{
				"description": "Get NPM package metadata",
				"params": map[string]interface{}{
					"repoKey":     "npm-registry",
					"packageName": "express",
				},
			},
		},
		"packages/docker/tags": {
			{
				"description": "List Docker image tags",
				"params": map[string]interface{}{
					"repoKey":   "docker-hub",
					"imageName": "nginx",
				},
			},
		},
	}
}

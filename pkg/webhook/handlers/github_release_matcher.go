package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/google/uuid"
)

// GitHubReleaseMatcher matches Artifactory packages to GitHub releases
type GitHubReleaseMatcher struct {
	releaseRepo repository.PackageReleaseRepository
	logger      observability.Logger
}

// NewGitHubReleaseMatcher creates a new GitHub release matcher
func NewGitHubReleaseMatcher(releaseRepo repository.PackageReleaseRepository, logger observability.Logger) *GitHubReleaseMatcher {
	if logger == nil {
		logger = observability.NewLogger("github-release-matcher")
	}

	return &GitHubReleaseMatcher{
		releaseRepo: releaseRepo,
		logger:      logger,
	}
}

// FindMatchingRelease attempts to find a GitHub release that matches the Artifactory package
func (m *GitHubReleaseMatcher) FindMatchingRelease(ctx context.Context, tenantID uuid.UUID, packageInfo *PackageInfo) (*models.PackageRelease, error) {
	// Try exact version match first
	release, err := m.releaseRepo.GetByVersion(ctx, tenantID, packageInfo.Name, packageInfo.Version)
	if err == nil {
		m.logger.Debug("Found exact version match", map[string]interface{}{
			"package": packageInfo.Name,
			"version": packageInfo.Version,
		})
		return release, nil
	}

	// Try with 'v' prefix (common in Git tags)
	versionWithV := "v" + packageInfo.Version
	release, err = m.releaseRepo.GetByVersion(ctx, tenantID, packageInfo.Name, versionWithV)
	if err == nil {
		m.logger.Debug("Found version match with 'v' prefix", map[string]interface{}{
			"package": packageInfo.Name,
			"version": versionWithV,
		})
		return release, nil
	}

	// Try semantic version matching (1.0.0 could match v1.0.0 or release-1.0.0)
	releases, err := m.findReleasesBySemanticVersion(ctx, tenantID, packageInfo)
	if err == nil && len(releases) > 0 {
		m.logger.Debug("Found semantic version match", map[string]interface{}{
			"package": packageInfo.Name,
			"version": packageInfo.Version,
			"matches": len(releases),
		})
		// Return the most recent match
		return releases[0], nil
	}

	// Try matching by Maven coordinates for Maven packages
	if packageInfo.Type == models.PackageTypeMaven && packageInfo.GroupID != "" {
		artifactIDRelease, err := m.releaseRepo.GetByVersion(ctx, tenantID, packageInfo.ArtifactID, packageInfo.Version)
		if err == nil {
			m.logger.Debug("Found match by Maven artifact ID", map[string]interface{}{
				"artifact_id": packageInfo.ArtifactID,
				"version":     packageInfo.Version,
			})
			return artifactIDRelease, nil
		}
	}

	// Try fuzzy matching based on package name variations
	fuzzyRelease, err := m.findReleaseByFuzzyMatch(ctx, tenantID, packageInfo)
	if err == nil {
		m.logger.Debug("Found fuzzy match", map[string]interface{}{
			"package": packageInfo.Name,
			"version": packageInfo.Version,
		})
		return fuzzyRelease, nil
	}

	m.logger.Debug("No matching GitHub release found", map[string]interface{}{
		"package": packageInfo.Name,
		"version": packageInfo.Version,
	})

	return nil, fmt.Errorf("no matching GitHub release found for %s@%s", packageInfo.Name, packageInfo.Version)
}

// findReleasesBySemanticVersion finds releases by parsing semantic versions
func (m *GitHubReleaseMatcher) findReleasesBySemanticVersion(ctx context.Context, tenantID uuid.UUID, packageInfo *PackageInfo) ([]*models.PackageRelease, error) {
	// Parse the version
	version := parseVersion(packageInfo.Version)
	if version == nil {
		return nil, fmt.Errorf("invalid semantic version: %s", packageInfo.Version)
	}

	// Get all releases for the package
	releases, err := m.releaseRepo.GetByRepository(ctx, tenantID, packageInfo.Name, 100, 0)
	if err != nil {
		return nil, err
	}

	var matches []*models.PackageRelease
	for _, release := range releases {
		// Check if semantic versions match
		if release.VersionMajor != nil && *release.VersionMajor == version.Major &&
			release.VersionMinor != nil && *release.VersionMinor == version.Minor &&
			release.VersionPatch != nil && *release.VersionPatch == version.Patch {

			// Check prerelease match if applicable
			if version.Prerelease != nil {
				if release.Prerelease != nil && *release.Prerelease == *version.Prerelease {
					matches = append(matches, release)
				}
			} else if release.Prerelease == nil {
				// Both have no prerelease
				matches = append(matches, release)
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no semantic version matches found")
	}

	return matches, nil
}

// findReleaseByFuzzyMatch attempts to find a release using fuzzy matching
func (m *GitHubReleaseMatcher) findReleaseByFuzzyMatch(ctx context.Context, tenantID uuid.UUID, packageInfo *PackageInfo) (*models.PackageRelease, error) {
	// Generate variations of the package name
	variations := m.generateNameVariations(packageInfo.Name)

	// Try each variation
	for _, variation := range variations {
		// Try exact version
		release, err := m.releaseRepo.GetByVersion(ctx, tenantID, variation, packageInfo.Version)
		if err == nil {
			return release, nil
		}

		// Try with 'v' prefix
		versionWithV := "v" + packageInfo.Version
		release, err = m.releaseRepo.GetByVersion(ctx, tenantID, variation, versionWithV)
		if err == nil {
			return release, nil
		}
	}

	return nil, fmt.Errorf("no fuzzy match found")
}

// generateNameVariations generates common variations of a package name
func (m *GitHubReleaseMatcher) generateNameVariations(name string) []string {
	variations := []string{name}

	// Remove scope for NPM packages (@scope/package -> package)
	if strings.HasPrefix(name, "@") {
		parts := strings.Split(name, "/")
		if len(parts) == 2 {
			variations = append(variations, parts[1])
		}
	}

	// Maven coordinates (group:artifact -> artifact)
	if strings.Contains(name, ":") {
		parts := strings.Split(name, ":")
		if len(parts) == 2 {
			variations = append(variations, parts[1])
		}
	}

	// Convert hyphens to underscores and vice versa
	variations = append(variations, strings.ReplaceAll(name, "-", "_"))
	variations = append(variations, strings.ReplaceAll(name, "_", "-"))

	// Convert dots to hyphens (common in some package managers)
	variations = append(variations, strings.ReplaceAll(name, ".", "-"))

	// Lowercase variations
	lowerName := strings.ToLower(name)
	if lowerName != name {
		variations = append(variations, lowerName)
		variations = append(variations, strings.ReplaceAll(lowerName, "-", "_"))
		variations = append(variations, strings.ReplaceAll(lowerName, "_", "-"))
	}

	return variations
}

// parseVersion parses a semantic version string
func parseVersion(versionStr string) *models.VersionInfo {
	// Remove common prefixes
	versionStr = strings.TrimPrefix(versionStr, "v")
	versionStr = strings.TrimPrefix(versionStr, "version-")
	versionStr = strings.TrimPrefix(versionStr, "release-")

	// Parse semantic version using regex
	semverRegex := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-(.+))?`)
	matches := semverRegex.FindStringSubmatch(versionStr)

	version := &models.VersionInfo{
		Raw: versionStr,
	}

	if len(matches) >= 4 {
		version.Major, _ = strconv.Atoi(matches[1])
		version.Minor, _ = strconv.Atoi(matches[2])
		version.Patch, _ = strconv.Atoi(matches[3])

		if len(matches) > 4 && matches[4] != "" {
			version.Prerelease = &matches[4]
		}
	}

	return version
}

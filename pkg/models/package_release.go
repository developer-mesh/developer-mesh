package models

import (
	"time"

	"github.com/google/uuid"
)

// PackageType represents the type of package
type PackageType string

const (
	PackageTypeNPM     PackageType = "npm"
	PackageTypeMaven   PackageType = "maven"
	PackageTypePython  PackageType = "python"
	PackageTypeGo      PackageType = "go"
	PackageTypeDocker  PackageType = "docker"
	PackageTypeGeneric PackageType = "generic"
)

// PackageRelease represents a package release in the system
type PackageRelease struct {
	ID               uuid.UUID `json:"id" db:"id"`
	TenantID         uuid.UUID `json:"tenant_id" db:"tenant_id"`
	RepositoryName   string    `json:"repository_name" db:"repository_name"`
	PackageName      string    `json:"package_name" db:"package_name"`
	Version          string    `json:"version" db:"version"`
	VersionMajor     *int      `json:"version_major,omitempty" db:"version_major"`
	VersionMinor     *int      `json:"version_minor,omitempty" db:"version_minor"`
	VersionPatch     *int      `json:"version_patch,omitempty" db:"version_patch"`
	Prerelease       *string   `json:"prerelease,omitempty" db:"prerelease"`
	IsBreakingChange bool      `json:"is_breaking_change" db:"is_breaking_change"`
	ReleaseNotes     *string   `json:"release_notes,omitempty" db:"release_notes"`
	Changelog        *string   `json:"changelog,omitempty" db:"changelog"`
	PublishedAt      time.Time `json:"published_at" db:"published_at"`
	AuthorLogin      *string   `json:"author_login,omitempty" db:"author_login"`
	GitHubReleaseID  *int64    `json:"github_release_id,omitempty" db:"github_release_id"`
	ArtifactoryPath  *string   `json:"artifactory_path,omitempty" db:"artifactory_path"`
	PackageType      string    `json:"package_type" db:"package_type"`
	Description      *string   `json:"description,omitempty" db:"description"`
	License          *string   `json:"license,omitempty" db:"license"`
	Homepage         *string   `json:"homepage,omitempty" db:"homepage"`
	DocumentationURL *string   `json:"documentation_url,omitempty" db:"documentation_url"`
	Metadata         JSONMap   `json:"metadata" db:"metadata"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// PackageAsset represents an artifact associated with a release
type PackageAsset struct {
	ID             uuid.UUID `json:"id" db:"id"`
	ReleaseID      uuid.UUID `json:"release_id" db:"release_id"`
	Name           string    `json:"name" db:"name"`
	ContentType    *string   `json:"content_type,omitempty" db:"content_type"`
	SizeBytes      *int64    `json:"size_bytes,omitempty" db:"size_bytes"`
	DownloadURL    *string   `json:"download_url,omitempty" db:"download_url"`
	ArtifactoryURL *string   `json:"artifactory_url,omitempty" db:"artifactory_url"`
	SHA256Checksum *string   `json:"sha256_checksum,omitempty" db:"sha256_checksum"`
	SHA1Checksum   *string   `json:"sha1_checksum,omitempty" db:"sha1_checksum"`
	MD5Checksum    *string   `json:"md5_checksum,omitempty" db:"md5_checksum"`
	Metadata       JSONMap   `json:"metadata" db:"metadata"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// APIChangeType represents the type of API change
type APIChangeType string

const (
	APIChangeAdded      APIChangeType = "added"
	APIChangeModified   APIChangeType = "modified"
	APIChangeDeprecated APIChangeType = "deprecated"
	APIChangeRemoved    APIChangeType = "removed"
)

// PackageAPIChange tracks API/interface changes in a release
type PackageAPIChange struct {
	ID             uuid.UUID     `json:"id" db:"id"`
	ReleaseID      uuid.UUID     `json:"release_id" db:"release_id"`
	ChangeType     APIChangeType `json:"change_type" db:"change_type"`
	APISignature   string        `json:"api_signature" db:"api_signature"`
	Description    *string       `json:"description,omitempty" db:"description"`
	Breaking       bool          `json:"breaking" db:"breaking"`
	MigrationGuide *string       `json:"migration_guide,omitempty" db:"migration_guide"`
	FilePath       *string       `json:"file_path,omitempty" db:"file_path"`
	LineNumber     *int          `json:"line_number,omitempty" db:"line_number"`
	Metadata       JSONMap       `json:"metadata" db:"metadata"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
}

// DependencyType represents the type of dependency
type DependencyType string

const (
	DependencyTypeRuntime  DependencyType = "runtime"
	DependencyTypeDev      DependencyType = "dev"
	DependencyTypePeer     DependencyType = "peer"
	DependencyTypeOptional DependencyType = "optional"
	DependencyTypeBuild    DependencyType = "build"
)

// PackageDependency represents a dependency of a package release
type PackageDependency struct {
	ID                uuid.UUID       `json:"id" db:"id"`
	ReleaseID         uuid.UUID       `json:"release_id" db:"release_id"`
	DependencyName    string          `json:"dependency_name" db:"dependency_name"`
	VersionConstraint *string         `json:"version_constraint,omitempty" db:"version_constraint"`
	DependencyType    *DependencyType `json:"dependency_type,omitempty" db:"dependency_type"`
	RepositoryURL     *string         `json:"repository_url,omitempty" db:"repository_url"`
	ResolvedVersion   *string         `json:"resolved_version,omitempty" db:"resolved_version"`
	Metadata          JSONMap         `json:"metadata" db:"metadata"`
	CreatedAt         time.Time       `json:"created_at" db:"created_at"`
}

// PackageReleaseCreate represents the data needed to create a new package release
type PackageReleaseCreate struct {
	TenantID         uuid.UUID `json:"tenant_id" validate:"required"`
	RepositoryName   string    `json:"repository_name" validate:"required"`
	PackageName      string    `json:"package_name" validate:"required"`
	Version          string    `json:"version" validate:"required"`
	VersionMajor     *int      `json:"version_major,omitempty"`
	VersionMinor     *int      `json:"version_minor,omitempty"`
	VersionPatch     *int      `json:"version_patch,omitempty"`
	Prerelease       *string   `json:"prerelease,omitempty"`
	IsBreakingChange bool      `json:"is_breaking_change"`
	ReleaseNotes     *string   `json:"release_notes,omitempty"`
	Changelog        *string   `json:"changelog,omitempty"`
	PublishedAt      time.Time `json:"published_at" validate:"required"`
	AuthorLogin      *string   `json:"author_login,omitempty"`
	GitHubReleaseID  *int64    `json:"github_release_id,omitempty"`
	PackageType      string    `json:"package_type" validate:"required"`
	Description      *string   `json:"description,omitempty"`
	License          *string   `json:"license,omitempty"`
	Homepage         *string   `json:"homepage,omitempty"`
	DocumentationURL *string   `json:"documentation_url,omitempty"`
	Metadata         JSONMap   `json:"metadata,omitempty"`
}

// PackageReleaseWithDetails includes all related data for a release
type PackageReleaseWithDetails struct {
	Release      PackageRelease      `json:"release"`
	Assets       []PackageAsset      `json:"assets,omitempty"`
	APIChanges   []PackageAPIChange  `json:"api_changes,omitempty"`
	Dependencies []PackageDependency `json:"dependencies,omitempty"`
}

// VersionInfo represents parsed semantic version information
type VersionInfo struct {
	Raw        string  `json:"raw"`
	Major      int     `json:"major"`
	Minor      int     `json:"minor"`
	Patch      int     `json:"patch"`
	Prerelease *string `json:"prerelease,omitempty"`
}

// ReleaseNotesSection represents a parsed section of release notes
type ReleaseNotesSection struct {
	Title   string   `json:"title"`
	Content []string `json:"content"`
}

// ParsedReleaseNotes represents structured release notes
type ParsedReleaseNotes struct {
	RawNotes          string                 `json:"raw_notes"`
	HasBreakingChange bool                   `json:"has_breaking_changes"`
	BreakingChanges   []string               `json:"breaking_changes,omitempty"`
	NewFeatures       []string               `json:"new_features,omitempty"`
	BugFixes          []string               `json:"bug_fixes,omitempty"`
	MigrationGuide    *string                `json:"migration_guide,omitempty"`
	Sections          []ReleaseNotesSection  `json:"sections,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// PackageReleaseRepository defines the interface for package release storage
type PackageReleaseRepository interface {
	// Create stores a new package release
	Create(ctx context.Context, release *models.PackageRelease) error

	// GetByID retrieves a package release by ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.PackageRelease, error)

	// GetByVersion retrieves a release by package name and version
	GetByVersion(ctx context.Context, tenantID uuid.UUID, packageName, version string) (*models.PackageRelease, error)

	// GetByRepository retrieves all releases for a repository
	GetByRepository(ctx context.Context, tenantID uuid.UUID, repoName string, limit, offset int) ([]*models.PackageRelease, error)

	// GetLatestByPackage retrieves the latest release for a package
	GetLatestByPackage(ctx context.Context, tenantID uuid.UUID, packageName string) (*models.PackageRelease, error)

	// ListByTenant retrieves all releases for a tenant
	ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*models.PackageRelease, error)

	// Update updates a package release
	Update(ctx context.Context, release *models.PackageRelease) error

	// Delete deletes a package release
	Delete(ctx context.Context, id uuid.UUID) error

	// CreateAsset stores a package asset
	CreateAsset(ctx context.Context, asset *models.PackageAsset) error

	// GetAssetsByReleaseID retrieves all assets for a release
	GetAssetsByReleaseID(ctx context.Context, releaseID uuid.UUID) ([]*models.PackageAsset, error)

	// CreateAPIChange stores an API change record
	CreateAPIChange(ctx context.Context, change *models.PackageAPIChange) error

	// GetAPIChangesByReleaseID retrieves all API changes for a release
	GetAPIChangesByReleaseID(ctx context.Context, releaseID uuid.UUID) ([]*models.PackageAPIChange, error)

	// CreateDependency stores a package dependency
	CreateDependency(ctx context.Context, dep *models.PackageDependency) error

	// GetDependenciesByReleaseID retrieves all dependencies for a release
	GetDependenciesByReleaseID(ctx context.Context, releaseID uuid.UUID) ([]*models.PackageDependency, error)

	// GetWithDetails retrieves a release with all related data
	GetWithDetails(ctx context.Context, id uuid.UUID) (*models.PackageReleaseWithDetails, error)

	// SearchByName searches for releases by package name (fuzzy search)
	SearchByName(ctx context.Context, tenantID uuid.UUID, namePattern string, limit int) ([]*models.PackageRelease, error)

	// GetVersionHistory retrieves version history for a package
	GetVersionHistory(ctx context.Context, tenantID uuid.UUID, packageName string, limit int) ([]*models.PackageRelease, error)

	// FindByDependency finds releases that depend on a specific package
	FindByDependency(ctx context.Context, tenantID uuid.UUID, dependencyName string, limit int) ([]*models.PackageRelease, error)
}

// packageReleaseRepository is the SQL implementation
type packageReleaseRepository struct {
	db *sqlx.DB
}

// NewPackageReleaseRepository creates a new package release repository
func NewPackageReleaseRepository(db *sqlx.DB) PackageReleaseRepository {
	return &packageReleaseRepository{db: db}
}

// Create stores a new package release
func (r *packageReleaseRepository) Create(ctx context.Context, release *models.PackageRelease) error {
	if release.ID == uuid.Nil {
		release.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.package_releases (
			id, tenant_id, repository_name, package_name, version,
			version_major, version_minor, version_patch, prerelease,
			is_breaking_change, release_notes, changelog, published_at,
			author_login, github_release_id, artifactory_path, package_type,
			description, license, homepage, documentation_url, metadata
		) VALUES (
			:id, :tenant_id, :repository_name, :package_name, :version,
			:version_major, :version_minor, :version_patch, :prerelease,
			:is_breaking_change, :release_notes, :changelog, :published_at,
			:author_login, :github_release_id, :artifactory_path, :package_type,
			:description, :license, :homepage, :documentation_url, :metadata
		)
		ON CONFLICT (tenant_id, repository_name, version)
		DO UPDATE SET
			package_name = EXCLUDED.package_name,
			version_major = EXCLUDED.version_major,
			version_minor = EXCLUDED.version_minor,
			version_patch = EXCLUDED.version_patch,
			prerelease = EXCLUDED.prerelease,
			is_breaking_change = EXCLUDED.is_breaking_change,
			release_notes = EXCLUDED.release_notes,
			changelog = EXCLUDED.changelog,
			published_at = EXCLUDED.published_at,
			author_login = EXCLUDED.author_login,
			github_release_id = EXCLUDED.github_release_id,
			artifactory_path = EXCLUDED.artifactory_path,
			package_type = EXCLUDED.package_type,
			description = EXCLUDED.description,
			license = EXCLUDED.license,
			homepage = EXCLUDED.homepage,
			documentation_url = EXCLUDED.documentation_url,
			metadata = EXCLUDED.metadata,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id`

	var id uuid.UUID
	rows, err := r.db.NamedQueryContext(ctx, query, release)
	if err != nil {
		return fmt.Errorf("failed to insert package release: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			// Log but don't override the primary error
			fmt.Printf("warning: failed to close rows: %v\n", closeErr)
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan release ID: %w", err)
		}
		release.ID = id
	}

	return nil
}

// GetByID retrieves a package release by ID
func (r *packageReleaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PackageRelease, error) {
	var release models.PackageRelease
	query := `SELECT * FROM mcp.package_releases WHERE id = $1`

	err := r.db.GetContext(ctx, &release, query, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("package release not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get package release: %w", err)
	}

	return &release, nil
}

// GetByVersion retrieves a release by package name and version
func (r *packageReleaseRepository) GetByVersion(ctx context.Context, tenantID uuid.UUID, packageName, version string) (*models.PackageRelease, error) {
	var release models.PackageRelease
	query := `
		SELECT * FROM mcp.package_releases
		WHERE tenant_id = $1 AND package_name = $2 AND version = $3`

	err := r.db.GetContext(ctx, &release, query, tenantID, packageName, version)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("package release not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get package release: %w", err)
	}

	return &release, nil
}

// GetByRepository retrieves all releases for a repository
func (r *packageReleaseRepository) GetByRepository(ctx context.Context, tenantID uuid.UUID, repoName string, limit, offset int) ([]*models.PackageRelease, error) {
	var releases []*models.PackageRelease
	query := `
		SELECT * FROM mcp.package_releases
		WHERE tenant_id = $1 AND repository_name = $2
		ORDER BY published_at DESC
		LIMIT $3 OFFSET $4`

	err := r.db.SelectContext(ctx, &releases, query, tenantID, repoName, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases by repository: %w", err)
	}

	return releases, nil
}

// GetLatestByPackage retrieves the latest release for a package
func (r *packageReleaseRepository) GetLatestByPackage(ctx context.Context, tenantID uuid.UUID, packageName string) (*models.PackageRelease, error) {
	var release models.PackageRelease
	query := `
		SELECT * FROM mcp.package_releases
		WHERE tenant_id = $1 AND package_name = $2
		ORDER BY published_at DESC
		LIMIT 1`

	err := r.db.GetContext(ctx, &release, query, tenantID, packageName)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no releases found for package: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}

	return &release, nil
}

// ListByTenant retrieves all releases for a tenant
func (r *packageReleaseRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*models.PackageRelease, error) {
	var releases []*models.PackageRelease
	query := `
		SELECT * FROM mcp.package_releases
		WHERE tenant_id = $1
		ORDER BY published_at DESC
		LIMIT $2 OFFSET $3`

	err := r.db.SelectContext(ctx, &releases, query, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases by tenant: %w", err)
	}

	return releases, nil
}

// Update updates a package release
func (r *packageReleaseRepository) Update(ctx context.Context, release *models.PackageRelease) error {
	release.UpdatedAt = time.Now()

	query := `
		UPDATE mcp.package_releases SET
			repository_name = :repository_name,
			package_name = :package_name,
			version = :version,
			version_major = :version_major,
			version_minor = :version_minor,
			version_patch = :version_patch,
			prerelease = :prerelease,
			is_breaking_change = :is_breaking_change,
			release_notes = :release_notes,
			changelog = :changelog,
			published_at = :published_at,
			author_login = :author_login,
			github_release_id = :github_release_id,
			artifactory_path = :artifactory_path,
			package_type = :package_type,
			description = :description,
			license = :license,
			homepage = :homepage,
			documentation_url = :documentation_url,
			metadata = :metadata,
			updated_at = :updated_at
		WHERE id = :id`

	_, err := r.db.NamedExecContext(ctx, query, release)
	if err != nil {
		return fmt.Errorf("failed to update package release: %w", err)
	}

	return nil
}

// Delete deletes a package release
func (r *packageReleaseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM mcp.package_releases WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete package release: %w", err)
	}

	return nil
}

// CreateAsset stores a package asset
func (r *packageReleaseRepository) CreateAsset(ctx context.Context, asset *models.PackageAsset) error {
	if asset.ID == uuid.Nil {
		asset.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.package_assets (
			id, release_id, name, content_type, size_bytes,
			download_url, artifactory_url, sha256_checksum,
			sha1_checksum, md5_checksum, metadata
		) VALUES (
			:id, :release_id, :name, :content_type, :size_bytes,
			:download_url, :artifactory_url, :sha256_checksum,
			:sha1_checksum, :md5_checksum, :metadata
		)`

	_, err := r.db.NamedExecContext(ctx, query, asset)
	if err != nil {
		return fmt.Errorf("failed to insert package asset: %w", err)
	}

	return nil
}

// GetAssetsByReleaseID retrieves all assets for a release
func (r *packageReleaseRepository) GetAssetsByReleaseID(ctx context.Context, releaseID uuid.UUID) ([]*models.PackageAsset, error) {
	var assets []*models.PackageAsset
	query := `SELECT * FROM mcp.package_assets WHERE release_id = $1 ORDER BY created_at`

	err := r.db.SelectContext(ctx, &assets, query, releaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets: %w", err)
	}

	return assets, nil
}

// CreateAPIChange stores an API change record
func (r *packageReleaseRepository) CreateAPIChange(ctx context.Context, change *models.PackageAPIChange) error {
	if change.ID == uuid.Nil {
		change.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.package_api_changes (
			id, release_id, change_type, api_signature, description,
			breaking, migration_guide, file_path, line_number, metadata
		) VALUES (
			:id, :release_id, :change_type, :api_signature, :description,
			:breaking, :migration_guide, :file_path, :line_number, :metadata
		)`

	_, err := r.db.NamedExecContext(ctx, query, change)
	if err != nil {
		return fmt.Errorf("failed to insert API change: %w", err)
	}

	return nil
}

// GetAPIChangesByReleaseID retrieves all API changes for a release
func (r *packageReleaseRepository) GetAPIChangesByReleaseID(ctx context.Context, releaseID uuid.UUID) ([]*models.PackageAPIChange, error) {
	var changes []*models.PackageAPIChange
	query := `SELECT * FROM mcp.package_api_changes WHERE release_id = $1 ORDER BY created_at`

	err := r.db.SelectContext(ctx, &changes, query, releaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API changes: %w", err)
	}

	return changes, nil
}

// CreateDependency stores a package dependency
func (r *packageReleaseRepository) CreateDependency(ctx context.Context, dep *models.PackageDependency) error {
	if dep.ID == uuid.Nil {
		dep.ID = uuid.New()
	}

	query := `
		INSERT INTO mcp.package_dependencies (
			id, release_id, dependency_name, version_constraint,
			dependency_type, repository_url, resolved_version, metadata
		) VALUES (
			:id, :release_id, :dependency_name, :version_constraint,
			:dependency_type, :repository_url, :resolved_version, :metadata
		)`

	_, err := r.db.NamedExecContext(ctx, query, dep)
	if err != nil {
		return fmt.Errorf("failed to insert dependency: %w", err)
	}

	return nil
}

// GetDependenciesByReleaseID retrieves all dependencies for a release
func (r *packageReleaseRepository) GetDependenciesByReleaseID(ctx context.Context, releaseID uuid.UUID) ([]*models.PackageDependency, error) {
	var deps []*models.PackageDependency
	query := `SELECT * FROM mcp.package_dependencies WHERE release_id = $1 ORDER BY created_at`

	err := r.db.SelectContext(ctx, &deps, query, releaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}

	return deps, nil
}

// GetWithDetails retrieves a release with all related data
func (r *packageReleaseRepository) GetWithDetails(ctx context.Context, id uuid.UUID) (*models.PackageReleaseWithDetails, error) {
	// Get the release
	release, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get all related data
	assets, err := r.GetAssetsByReleaseID(ctx, id)
	if err != nil {
		return nil, err
	}

	apiChanges, err := r.GetAPIChangesByReleaseID(ctx, id)
	if err != nil {
		return nil, err
	}

	dependencies, err := r.GetDependenciesByReleaseID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &models.PackageReleaseWithDetails{
		Release:      *release,
		Assets:       derefSlice(assets),
		APIChanges:   derefSlice(apiChanges),
		Dependencies: derefSlice(dependencies),
	}, nil
}

// derefSlice converts a slice of pointers to a slice of values
func derefSlice[T any](slice []*T) []T {
	result := make([]T, len(slice))
	for i, item := range slice {
		if item != nil {
			result[i] = *item
		}
	}
	return result
}

// SearchByName searches for releases by package name (fuzzy search)
func (r *packageReleaseRepository) SearchByName(ctx context.Context, tenantID uuid.UUID, namePattern string, limit int) ([]*models.PackageRelease, error) {
	if limit <= 0 {
		limit = 20
	}

	var releases []*models.PackageRelease
	query := `
		SELECT * FROM mcp.package_releases
		WHERE tenant_id = $1
		AND (
			package_name ILIKE $2
			OR repository_name ILIKE $2
		)
		ORDER BY published_at DESC
		LIMIT $3`

	pattern := "%" + namePattern + "%"
	err := r.db.SelectContext(ctx, &releases, query, tenantID, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search releases by name: %w", err)
	}

	return releases, nil
}

// GetVersionHistory retrieves version history for a package
func (r *packageReleaseRepository) GetVersionHistory(ctx context.Context, tenantID uuid.UUID, packageName string, limit int) ([]*models.PackageRelease, error) {
	if limit <= 0 {
		limit = 50
	}

	var releases []*models.PackageRelease
	query := `
		SELECT * FROM mcp.package_releases
		WHERE tenant_id = $1 AND package_name = $2
		ORDER BY published_at DESC
		LIMIT $3`

	err := r.db.SelectContext(ctx, &releases, query, tenantID, packageName, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get version history: %w", err)
	}

	return releases, nil
}

// FindByDependency finds releases that depend on a specific package
func (r *packageReleaseRepository) FindByDependency(ctx context.Context, tenantID uuid.UUID, dependencyName string, limit int) ([]*models.PackageRelease, error) {
	if limit <= 0 {
		limit = 50
	}

	var releases []*models.PackageRelease
	query := `
		SELECT DISTINCT pr.* FROM mcp.package_releases pr
		JOIN mcp.package_dependencies pd ON pd.release_id = pr.id
		WHERE pr.tenant_id = $1
		AND pd.dependency_name = $2
		ORDER BY pr.published_at DESC
		LIMIT $3`

	err := r.db.SelectContext(ctx, &releases, query, tenantID, dependencyName, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find releases by dependency: %w", err)
	}

	return releases, nil
}

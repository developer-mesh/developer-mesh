// Package search provides package-aware semantic search capabilities
package search

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// PackageSearchService provides semantic search for package releases
type PackageSearchService struct {
	db               *sqlx.DB
	embeddingService *embedding.ServiceV2
	ranker           *RelevanceRanker
	logger           observability.Logger
	metrics          observability.MetricsClient
}

// NewPackageSearchService creates a new package search service
func NewPackageSearchService(
	db *sqlx.DB,
	embeddingService *embedding.ServiceV2,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *PackageSearchService {
	if logger == nil {
		logger = observability.NewLogger("package-search-service")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	return &PackageSearchService{
		db:               db,
		embeddingService: embeddingService,
		ranker:           NewRelevanceRanker(logger),
		logger:           logger,
		metrics:          metrics,
	}
}

// PackageSearchQuery represents a search query for packages
type PackageSearchQuery struct {
	Query           string
	PackageTypes    []string
	Repositories    []string
	VersionRange    string
	IncludeBreaking bool
	OnlyLatest      bool
	MinSimilarity   float64
	Limit           int
	Offset          int
	TenantID        uuid.UUID
	AgentID         string
}

// PackageSearchResult represents a search result
type PackageSearchResult struct {
	Release         *models.PackageRelease
	Distance        float64
	Similarity      float64
	Score           float64
	MatchedKeywords []string
	Highlights      []string
}

// Search performs semantic search for package releases
func (s *PackageSearchService) Search(ctx context.Context, query PackageSearchQuery) ([]*PackageSearchResult, error) {
	// Validate query
	if query.Query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}
	if query.TenantID == uuid.Nil {
		return nil, fmt.Errorf("tenant_id is required")
	}

	// Set defaults
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.MinSimilarity <= 0 {
		query.MinSimilarity = 0.5 // Default 50% similarity threshold
	}

	// Generate embedding for the query
	embeddingReq := embedding.GenerateEmbeddingRequest{
		Text:     query.Query,
		TenantID: query.TenantID,
		AgentID:  query.AgentID,
		Metadata: map[string]interface{}{
			"source": "package_search",
			"query":  query.Query,
		},
	}

	embeddingResp, err := s.embeddingService.GenerateEmbedding(ctx, embeddingReq)
	if err != nil {
		s.logger.Error("Failed to generate query embedding", map[string]interface{}{
			"error": err.Error(),
			"query": query.Query,
		})
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Build SQL query with filters using cosine distance operator
	sqlQuery := `
		SELECT
			pr.id,
			pr.tenant_id,
			pr.repository_name,
			pr.package_name,
			pr.version,
			pr.version_major,
			pr.version_minor,
			pr.version_patch,
			pr.prerelease,
			pr.is_breaking_change,
			pr.release_notes,
			pr.changelog,
			pr.published_at,
			pr.author_login,
			pr.github_release_id,
			pr.artifactory_path,
			pr.package_type,
			pr.description,
			pr.license,
			pr.homepage,
			pr.documentation_url,
			pr.metadata,
			pr.created_at,
			pr.updated_at,
			1 - (e.embedding <=> (SELECT embedding FROM mcp.embedding_vectors WHERE id = $1)) AS similarity
		FROM mcp.package_releases pr
		JOIN mcp.context_embeddings e ON e.context_id::text = pr.id::text
		WHERE pr.tenant_id = $2
	`

	params := []interface{}{embeddingResp.EmbeddingID, query.TenantID}
	paramIdx := 3

	// Add package type filter
	if len(query.PackageTypes) > 0 {
		sqlQuery += fmt.Sprintf(" AND pr.package_type = ANY($%d)", paramIdx)
		params = append(params, pq.Array(query.PackageTypes))
		paramIdx++
	}

	// Add repository filter
	if len(query.Repositories) > 0 {
		sqlQuery += fmt.Sprintf(" AND pr.repository_name = ANY($%d)", paramIdx)
		params = append(params, pq.Array(query.Repositories))
		paramIdx++
	}

	// Add breaking change filter
	if !query.IncludeBreaking {
		sqlQuery += " AND pr.is_breaking_change = false"
	}

	// Add latest version filter
	if query.OnlyLatest {
		sqlQuery += `
			AND pr.published_at = (
				SELECT MAX(pr2.published_at)
				FROM mcp.package_releases pr2
				WHERE pr2.tenant_id = pr.tenant_id
				AND pr2.package_name = pr.package_name
			)
		`
	}

	// Add similarity threshold
	sqlQuery += fmt.Sprintf(" AND (1 - (e.embedding <=> (SELECT embedding FROM mcp.embedding_vectors WHERE id = $1))) >= $%d", paramIdx)
	params = append(params, query.MinSimilarity)
	paramIdx++

	// Order by semantic similarity (descending - higher is more similar)
	sqlQuery += " ORDER BY similarity DESC"

	// Add pagination
	sqlQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", paramIdx, paramIdx+1)
	params = append(params, query.Limit, query.Offset)

	s.logger.Debug("Executing package search", map[string]interface{}{
		"query":          query.Query,
		"tenant_id":      query.TenantID,
		"limit":          query.Limit,
		"min_similarity": query.MinSimilarity,
		"model_used":     embeddingResp.ModelUsed,
	})

	// Execute search
	rows, err := s.db.QueryContext(ctx, sqlQuery, params...)
	if err != nil {
		s.logger.Error("Failed to execute package search", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to execute package search: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": closeErr.Error(),
			})
		}
	}()

	// Process results
	var results []*PackageSearchResult
	for rows.Next() {
		var release models.PackageRelease
		var similarity float64

		err := rows.Scan(
			&release.ID,
			&release.TenantID,
			&release.RepositoryName,
			&release.PackageName,
			&release.Version,
			&release.VersionMajor,
			&release.VersionMinor,
			&release.VersionPatch,
			&release.Prerelease,
			&release.IsBreakingChange,
			&release.ReleaseNotes,
			&release.Changelog,
			&release.PublishedAt,
			&release.AuthorLogin,
			&release.GitHubReleaseID,
			&release.ArtifactoryPath,
			&release.PackageType,
			&release.Description,
			&release.License,
			&release.Homepage,
			&release.DocumentationURL,
			&release.Metadata,
			&release.CreatedAt,
			&release.UpdatedAt,
			&similarity,
		)
		if err != nil {
			s.logger.Warn("Failed to scan search result", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		result := &PackageSearchResult{
			Release:    &release,
			Distance:   1.0 - similarity, // Convert similarity to distance
			Similarity: similarity,
		}

		// Calculate relevance score
		result.Score = s.ranker.Rank(query.Query, result)

		// Extract matched keywords and highlights
		result.MatchedKeywords = s.extractMatchedKeywords(query.Query, &release)
		result.Highlights = s.generateHighlights(query.Query, &release)

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	// Re-rank results by final score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Record metrics
	s.metrics.RecordHistogram("package_search_results_count", float64(len(results)), nil)

	s.logger.Info("Package search completed", map[string]interface{}{
		"query":       query.Query,
		"results":     len(results),
		"tenant_id":   query.TenantID,
		"model_used":  embeddingResp.ModelUsed,
		"dimensions":  embeddingResp.Dimensions,
		"tokens_used": embeddingResp.TokensUsed,
	})

	return results, nil
}

// GetPackageHistory retrieves version history for a package
func (s *PackageSearchService) GetPackageHistory(ctx context.Context, tenantID uuid.UUID, packageName string, limit int) ([]*PackageVersionInfo, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT
			id,
			version,
			published_at,
			is_breaking_change,
			release_notes,
			changelog,
			prerelease,
			package_type
		FROM mcp.package_releases
		WHERE tenant_id = $1 AND package_name = $2
		ORDER BY published_at DESC
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, query, tenantID, packageName, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get package history: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": closeErr.Error(),
			})
		}
	}()

	var versions []*PackageVersionInfo
	for rows.Next() {
		var version PackageVersionInfo
		err := rows.Scan(
			&version.ID,
			&version.Version,
			&version.PublishedAt,
			&version.IsBreakingChange,
			&version.ReleaseNotes,
			&version.Changelog,
			&version.Prerelease,
			&version.PackageType,
		)
		if err != nil {
			s.logger.Warn("Failed to scan version info", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		versions = append(versions, &version)
	}

	return versions, rows.Err()
}

// GetDependencyGraph builds a dependency graph for a package
func (s *PackageSearchService) GetDependencyGraph(ctx context.Context, tenantID uuid.UUID, packageName, version string, depth int) (*DependencyGraph, error) {
	if depth <= 0 {
		depth = 3 // Default depth
	}

	// Find the release
	var releaseID uuid.UUID
	query := `
		SELECT id FROM mcp.package_releases
		WHERE tenant_id = $1 AND package_name = $2 AND version = $3
	`
	err := s.db.QueryRowContext(ctx, query, tenantID, packageName, version).Scan(&releaseID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("package release not found: %s@%s", packageName, version)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	// Build the graph
	graph := &DependencyGraph{
		Root: &DependencyNode{
			PackageName: packageName,
			Version:     version,
			Children:    make([]*DependencyNode, 0),
		},
		Nodes: make(map[string]*DependencyNode),
	}

	graph.Nodes[packageName] = graph.Root

	// Recursively fetch dependencies
	if err := s.fetchDependenciesRecursive(ctx, tenantID, releaseID, graph.Root, graph.Nodes, depth, 0); err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	return graph, nil
}

// fetchDependenciesRecursive recursively fetches dependencies for a release
func (s *PackageSearchService) fetchDependenciesRecursive(
	ctx context.Context,
	tenantID uuid.UUID,
	releaseID uuid.UUID,
	node *DependencyNode,
	visited map[string]*DependencyNode,
	maxDepth, currentDepth int,
) error {
	if currentDepth >= maxDepth {
		return nil
	}

	// Fetch dependencies for this release
	query := `
		SELECT
			dependency_name,
			version_constraint,
			resolved_version,
			dependency_type
		FROM mcp.package_dependencies
		WHERE release_id = $1
	`

	rows, err := s.db.QueryContext(ctx, query, releaseID)
	if err != nil {
		return fmt.Errorf("failed to fetch dependencies: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": closeErr.Error(),
			})
		}
	}()

	for rows.Next() {
		var depName, versionConstraint string
		var resolvedVersion, depType *string

		if err := rows.Scan(&depName, &versionConstraint, &resolvedVersion, &depType); err != nil {
			continue
		}

		// Check if we've already visited this dependency
		if existingNode, exists := visited[depName]; exists {
			node.Children = append(node.Children, existingNode)
			continue
		}

		// Create new node
		depNode := &DependencyNode{
			PackageName:     depName,
			Version:         versionConstraint,
			ResolvedVersion: resolvedVersion,
			DependencyType:  depType,
			Children:        make([]*DependencyNode, 0),
		}

		visited[depName] = depNode
		node.Children = append(node.Children, depNode)

		// Find the release for this dependency
		var depReleaseID uuid.UUID
		depQuery := `
			SELECT id FROM mcp.package_releases
			WHERE tenant_id = $1 AND package_name = $2
			ORDER BY published_at DESC
			LIMIT 1
		`
		err := s.db.QueryRowContext(ctx, depQuery, tenantID, depName).Scan(&depReleaseID)
		if err != nil {
			// Dependency not found in our system, skip recursion
			continue
		}

		// Recursively fetch dependencies
		if err := s.fetchDependenciesRecursive(ctx, tenantID, depReleaseID, depNode, visited, maxDepth, currentDepth+1); err != nil {
			s.logger.Warn("Failed to fetch transitive dependencies", map[string]interface{}{
				"package": depName,
				"error":   err.Error(),
			})
		}
	}

	return rows.Err()
}

// extractMatchedKeywords extracts keywords from the query that match the release
func (s *PackageSearchService) extractMatchedKeywords(query string, release *models.PackageRelease) []string {
	queryWords := strings.Fields(strings.ToLower(query))
	matched := make(map[string]bool)

	// Check against package name
	packageWords := strings.FieldsFunc(strings.ToLower(release.PackageName), func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/'
	})
	for _, qw := range queryWords {
		for _, pw := range packageWords {
			if strings.Contains(pw, qw) || strings.Contains(qw, pw) {
				matched[qw] = true
			}
		}
	}

	// Check against description
	if release.Description != nil {
		descWords := strings.Fields(strings.ToLower(*release.Description))
		for _, qw := range queryWords {
			for _, dw := range descWords {
				if strings.Contains(dw, qw) {
					matched[qw] = true
				}
			}
		}
	}

	// Convert to slice
	result := make([]string, 0, len(matched))
	for keyword := range matched {
		result = append(result, keyword)
	}

	return result
}

// generateHighlights generates text highlights for the search result
func (s *PackageSearchService) generateHighlights(query string, release *models.PackageRelease) []string {
	var highlights []string

	// Add package name
	highlights = append(highlights, fmt.Sprintf("%s@%s", release.PackageName, release.Version))

	// Add description if available
	if release.Description != nil && *release.Description != "" {
		highlights = append(highlights, *release.Description)
	}

	// Add breaking change indicator
	if release.IsBreakingChange {
		highlights = append(highlights, "⚠️ BREAKING CHANGE")
	}

	return highlights
}

// PackageVersionInfo represents version history information
type PackageVersionInfo struct {
	ID               uuid.UUID
	Version          string
	PublishedAt      interface{}
	IsBreakingChange bool
	ReleaseNotes     *string
	Changelog        *string
	Prerelease       *string
	PackageType      string
}

// DependencyGraph represents a package dependency graph
type DependencyGraph struct {
	Root  *DependencyNode
	Nodes map[string]*DependencyNode
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
	PackageName     string
	Version         string
	ResolvedVersion *string
	DependencyType  *string
	Children        []*DependencyNode
}

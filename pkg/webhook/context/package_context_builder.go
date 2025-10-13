package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// PackageContextBuilder builds enriched, searchable context for package releases
type PackageContextBuilder struct {
	embeddingService *embedding.ServiceV2
	logger           observability.Logger
}

// NewPackageContextBuilder creates a new package context builder
func NewPackageContextBuilder(
	embeddingService *embedding.ServiceV2,
	logger observability.Logger,
) *PackageContextBuilder {
	if logger == nil {
		logger = observability.NewLogger("package-context-builder")
	}

	return &PackageContextBuilder{
		embeddingService: embeddingService,
		logger:           logger,
	}
}

// EnrichedPackageContext represents enriched context for a package release
type EnrichedPackageContext struct {
	// Core Information
	ReleaseID   uuid.UUID `json:"release_id"`
	PackageName string    `json:"package_name"`
	Version     string    `json:"version"`
	PackageType string    `json:"package_type"`
	ReleaseDate time.Time `json:"release_date"`
	Repository  string    `json:"repository"`

	// Release Information
	ReleaseNotes    *string  `json:"release_notes,omitempty"`
	Changelog       *string  `json:"changelog,omitempty"`
	BreakingChanges []string `json:"breaking_changes,omitempty"`
	NewFeatures     []string `json:"new_features,omitempty"`
	BugFixes        []string `json:"bug_fixes,omitempty"`
	MigrationGuide  *string  `json:"migration_guide,omitempty"`

	// Package Metadata
	Description   *string `json:"description,omitempty"`
	Author        *string `json:"author,omitempty"`
	License       *string `json:"license,omitempty"`
	Homepage      *string `json:"homepage,omitempty"`
	Documentation *string `json:"documentation,omitempty"`

	// Dependencies
	Dependencies    []DependencyInfo `json:"dependencies,omitempty"`
	DevDependencies []DependencyInfo `json:"dev_dependencies,omitempty"`

	// API Changes
	APIChanges []APIChangeInfo `json:"api_changes,omitempty"`

	// Artifactory Information
	ArtifactoryPath *string     `json:"artifactory_path,omitempty"`
	Assets          []AssetInfo `json:"assets,omitempty"`

	// Search Optimization
	SearchableText string   `json:"searchable_text"`
	Keywords       []string `json:"keywords"`
	Categories     []string `json:"categories"`

	// Embeddings
	Embedding      []float32 `json:"embedding,omitempty"`
	EmbeddingModel string    `json:"embedding_model,omitempty"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata"`
}

// DependencyInfo represents dependency information
type DependencyInfo struct {
	Name              string  `json:"name"`
	Version           string  `json:"version,omitempty"`
	VersionConstraint string  `json:"version_constraint,omitempty"`
	Type              string  `json:"type,omitempty"`
	RepositoryURL     *string `json:"repository_url,omitempty"`
}

// APIChangeInfo represents API change information
type APIChangeInfo struct {
	Type        string  `json:"type"`
	Signature   string  `json:"signature"`
	Description *string `json:"description,omitempty"`
	Breaking    bool    `json:"breaking"`
	FilePath    *string `json:"file_path,omitempty"`
	LineNumber  *int    `json:"line_number,omitempty"`
}

// AssetInfo represents asset information
type AssetInfo struct {
	Name           string            `json:"name"`
	Size           int64             `json:"size,omitempty"`
	ContentType    *string           `json:"content_type,omitempty"`
	DownloadURL    *string           `json:"download_url,omitempty"`
	ArtifactoryURL *string           `json:"artifactory_url,omitempty"`
	Checksums      map[string]string `json:"checksums,omitempty"`
}

// BuildReleaseContext builds enriched context for a GitHub or Artifactory release
func (b *PackageContextBuilder) BuildReleaseContext(
	ctx context.Context,
	release *models.PackageRelease,
	assets []models.PackageAsset,
	apiChanges []models.PackageAPIChange,
	dependencies []models.PackageDependency,
	releaseNotes *models.ParsedReleaseNotes,
) (*EnrichedPackageContext, error) {
	if release == nil {
		return nil, fmt.Errorf("release cannot be nil")
	}

	enrichedCtx := &EnrichedPackageContext{
		ReleaseID:   release.ID,
		PackageName: release.PackageName,
		Version:     release.Version,
		PackageType: release.PackageType,
		ReleaseDate: release.PublishedAt,
		Repository:  release.RepositoryName,

		// Package metadata
		Description:     release.Description,
		Author:          release.AuthorLogin,
		License:         release.License,
		Homepage:        release.Homepage,
		Documentation:   release.DocumentationURL,
		ArtifactoryPath: release.ArtifactoryPath,

		Metadata: make(map[string]interface{}),
	}

	// Add release notes information
	if releaseNotes != nil {
		enrichedCtx.ReleaseNotes = &releaseNotes.RawNotes
		enrichedCtx.BreakingChanges = releaseNotes.BreakingChanges
		enrichedCtx.NewFeatures = releaseNotes.NewFeatures
		enrichedCtx.BugFixes = releaseNotes.BugFixes
		enrichedCtx.MigrationGuide = releaseNotes.MigrationGuide
	} else if release.ReleaseNotes != nil {
		enrichedCtx.ReleaseNotes = release.ReleaseNotes
	}

	// Convert dependencies
	enrichedCtx.Dependencies = make([]DependencyInfo, 0, len(dependencies))
	enrichedCtx.DevDependencies = make([]DependencyInfo, 0)

	for _, dep := range dependencies {
		depInfo := DependencyInfo{
			Name:          dep.DependencyName,
			RepositoryURL: dep.RepositoryURL,
		}

		if dep.VersionConstraint != nil {
			depInfo.VersionConstraint = *dep.VersionConstraint
		}
		if dep.ResolvedVersion != nil {
			depInfo.Version = *dep.ResolvedVersion
		}

		// Separate by type
		if dep.DependencyType != nil && *dep.DependencyType == models.DependencyTypeDev {
			enrichedCtx.DevDependencies = append(enrichedCtx.DevDependencies, depInfo)
		} else {
			enrichedCtx.Dependencies = append(enrichedCtx.Dependencies, depInfo)
		}
	}

	// Convert API changes
	enrichedCtx.APIChanges = make([]APIChangeInfo, 0, len(apiChanges))
	for _, change := range apiChanges {
		changeInfo := APIChangeInfo{
			Type:        string(change.ChangeType),
			Signature:   change.APISignature,
			Description: change.Description,
			Breaking:    change.Breaking,
			FilePath:    change.FilePath,
			LineNumber:  change.LineNumber,
		}
		enrichedCtx.APIChanges = append(enrichedCtx.APIChanges, changeInfo)
	}

	// Convert assets
	enrichedCtx.Assets = make([]AssetInfo, 0, len(assets))
	for _, asset := range assets {
		assetInfo := AssetInfo{
			Name:           asset.Name,
			ContentType:    asset.ContentType,
			DownloadURL:    asset.DownloadURL,
			ArtifactoryURL: asset.ArtifactoryURL,
		}

		if asset.SizeBytes != nil {
			assetInfo.Size = *asset.SizeBytes
		}

		// Add checksums
		checksums := make(map[string]string)
		if asset.SHA256Checksum != nil {
			checksums["sha256"] = *asset.SHA256Checksum
		}
		if asset.SHA1Checksum != nil {
			checksums["sha1"] = *asset.SHA1Checksum
		}
		if asset.MD5Checksum != nil {
			checksums["md5"] = *asset.MD5Checksum
		}
		if len(checksums) > 0 {
			assetInfo.Checksums = checksums
		}

		enrichedCtx.Assets = append(enrichedCtx.Assets, assetInfo)
	}

	// Generate searchable text
	enrichedCtx.SearchableText = b.GenerateSearchableText(enrichedCtx)

	// Extract keywords
	enrichedCtx.Keywords = b.ExtractKeywords(enrichedCtx)

	// Categorize package
	enrichedCtx.Categories = b.CategorizePackage(enrichedCtx)

	// Add metadata from release
	if release.Metadata != nil {
		for k, v := range release.Metadata {
			enrichedCtx.Metadata[k] = v
		}
	}

	// Add context-specific metadata
	enrichedCtx.Metadata["has_breaking_changes"] = release.IsBreakingChange
	enrichedCtx.Metadata["asset_count"] = len(assets)
	enrichedCtx.Metadata["dependency_count"] = len(dependencies)
	enrichedCtx.Metadata["api_change_count"] = len(apiChanges)

	return enrichedCtx, nil
}

// GenerateSearchableText creates a comprehensive searchable text representation
func (b *PackageContextBuilder) GenerateSearchableText(ctx *EnrichedPackageContext) string {
	var parts []string

	// Package identification
	parts = append(parts, fmt.Sprintf("Package: %s version %s", ctx.PackageName, ctx.Version))
	parts = append(parts, fmt.Sprintf("Type: %s", ctx.PackageType))
	parts = append(parts, fmt.Sprintf("Repository: %s", ctx.Repository))

	// Description
	if ctx.Description != nil && *ctx.Description != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", *ctx.Description))
	}

	// Release notes
	if ctx.ReleaseNotes != nil && *ctx.ReleaseNotes != "" {
		parts = append(parts, "Release Notes:", *ctx.ReleaseNotes)
	}

	// Breaking changes (high priority for search)
	if len(ctx.BreakingChanges) > 0 {
		parts = append(parts, "BREAKING CHANGES:")
		for _, change := range ctx.BreakingChanges {
			parts = append(parts, "- "+change)
		}
	}

	// New features
	if len(ctx.NewFeatures) > 0 {
		parts = append(parts, "New Features:")
		for _, feature := range ctx.NewFeatures {
			parts = append(parts, "- "+feature)
		}
	}

	// Bug fixes
	if len(ctx.BugFixes) > 0 {
		parts = append(parts, "Bug Fixes:")
		for _, fix := range ctx.BugFixes {
			parts = append(parts, "- "+fix)
		}
	}

	// Migration guide
	if ctx.MigrationGuide != nil && *ctx.MigrationGuide != "" {
		parts = append(parts, "Migration Guide:", *ctx.MigrationGuide)
	}

	// API changes
	if len(ctx.APIChanges) > 0 {
		parts = append(parts, "API Changes:")
		for _, change := range ctx.APIChanges {
			changeDesc := fmt.Sprintf("- %s: %s", change.Type, change.Signature)
			if change.Breaking {
				changeDesc += " [BREAKING]"
			}
			if change.Description != nil {
				changeDesc += " - " + *change.Description
			}
			parts = append(parts, changeDesc)
		}
	}

	// Dependencies (for discovery)
	if len(ctx.Dependencies) > 0 {
		depNames := make([]string, 0, len(ctx.Dependencies))
		for _, dep := range ctx.Dependencies {
			depStr := dep.Name
			if dep.Version != "" {
				depStr += "@" + dep.Version
			} else if dep.VersionConstraint != "" {
				depStr += dep.VersionConstraint
			}
			depNames = append(depNames, depStr)
		}
		parts = append(parts, "Dependencies: "+strings.Join(depNames, ", "))
	}

	// License and author
	if ctx.License != nil && *ctx.License != "" {
		parts = append(parts, "License: "+*ctx.License)
	}
	if ctx.Author != nil && *ctx.Author != "" {
		parts = append(parts, "Author: "+*ctx.Author)
	}

	// Assets
	if len(ctx.Assets) > 0 {
		assetNames := make([]string, 0, len(ctx.Assets))
		for _, asset := range ctx.Assets {
			assetNames = append(assetNames, asset.Name)
		}
		parts = append(parts, "Assets: "+strings.Join(assetNames, ", "))
	}

	return strings.Join(parts, "\n\n")
}

// ExtractKeywords extracts relevant keywords for search optimization
func (b *PackageContextBuilder) ExtractKeywords(ctx *EnrichedPackageContext) []string {
	keywords := make(map[string]bool)

	// Add package name variations
	keywords[ctx.PackageName] = true
	keywords[strings.ToLower(ctx.PackageName)] = true

	// Extract from package name (e.g., "my-awesome-package" -> ["my", "awesome", "package"])
	nameParts := strings.FieldsFunc(ctx.PackageName, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/'
	})
	for _, part := range nameParts {
		if len(part) > 2 { // Skip very short words
			keywords[strings.ToLower(part)] = true
		}
	}

	// Add package type
	keywords[ctx.PackageType] = true

	// Add version keywords
	keywords[ctx.Version] = true
	if strings.HasPrefix(ctx.Version, "v") {
		keywords[strings.TrimPrefix(ctx.Version, "v")] = true
	}

	// Extract from description
	if ctx.Description != nil {
		descWords := strings.Fields(strings.ToLower(*ctx.Description))
		for _, word := range descWords {
			// Clean and add significant words (length > 3)
			word = strings.Trim(word, ".,;:!?()")
			if len(word) > 3 {
				keywords[word] = true
			}
		}
	}

	// Add license as keyword
	if ctx.License != nil {
		keywords[strings.ToLower(*ctx.License)] = true
	}

	// Add breaking change indicator
	if len(ctx.BreakingChanges) > 0 {
		keywords["breaking-change"] = true
		keywords["breaking"] = true
	}

	// Add dependency names
	for _, dep := range ctx.Dependencies {
		keywords[dep.Name] = true
	}

	// Convert map to slice
	result := make([]string, 0, len(keywords))
	for keyword := range keywords {
		result = append(result, keyword)
	}

	return result
}

// CategorizePackage assigns categories to the package for better organization
func (b *PackageContextBuilder) CategorizePackage(ctx *EnrichedPackageContext) []string {
	categories := make(map[string]bool)

	// Primary category: package type
	categories[ctx.PackageType] = true

	// Categorize by breaking changes
	if len(ctx.BreakingChanges) > 0 {
		categories["breaking-change"] = true
	}

	// Categorize by prerelease status
	if ctx.Metadata != nil {
		if isPrerelease, ok := ctx.Metadata["is_prerelease"].(bool); ok && isPrerelease {
			categories["prerelease"] = true
		}
	}

	// Categorize by source
	if ctx.Metadata != nil {
		if source, ok := ctx.Metadata["source"].(string); ok {
			categories[source] = true
		}
	}
	if ctx.ArtifactoryPath != nil {
		categories["artifactory"] = true
	}

	// Categorize by content
	lowerText := strings.ToLower(ctx.SearchableText)

	// Check for common categories
	if strings.Contains(lowerText, "security") || strings.Contains(lowerText, "vulnerability") || strings.Contains(lowerText, "cve") {
		categories["security"] = true
	}
	if strings.Contains(lowerText, "performance") || strings.Contains(lowerText, "optimization") {
		categories["performance"] = true
	}
	if strings.Contains(lowerText, "deprecat") {
		categories["deprecation"] = true
	}
	if len(ctx.NewFeatures) > 0 {
		categories["feature"] = true
	}
	if len(ctx.BugFixes) > 0 {
		categories["bugfix"] = true
	}
	if len(ctx.APIChanges) > 0 {
		categories["api-change"] = true
	}

	// Convert map to slice
	result := make([]string, 0, len(categories))
	for category := range categories {
		result = append(result, category)
	}

	return result
}

// GenerateEmbedding generates an embedding for the enriched context
func (b *PackageContextBuilder) GenerateEmbedding(
	ctx context.Context,
	enrichedCtx *EnrichedPackageContext,
	tenantID uuid.UUID,
	agentID string,
) error {
	// Generate embedding using the embedding service
	req := embedding.GenerateEmbeddingRequest{
		Text:      enrichedCtx.SearchableText,
		TenantID:  tenantID,
		AgentID:   agentID,
		ContextID: &enrichedCtx.ReleaseID,
		Metadata: map[string]interface{}{
			"package_name": enrichedCtx.PackageName,
			"version":      enrichedCtx.Version,
			"package_type": enrichedCtx.PackageType,
			"source":       "package_release",
		},
	}

	resp, err := b.embeddingService.GenerateEmbedding(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Note: The actual embedding vector is stored in the database by the service
	// We store the metadata about the embedding in our context
	enrichedCtx.EmbeddingModel = resp.ModelUsed

	// Store embedding ID in metadata
	if enrichedCtx.Metadata == nil {
		enrichedCtx.Metadata = make(map[string]interface{})
	}
	enrichedCtx.Metadata["embedding_id"] = resp.EmbeddingID.String()

	b.logger.Info("Generated embedding for package release", map[string]interface{}{
		"package":      enrichedCtx.PackageName,
		"version":      enrichedCtx.Version,
		"embedding_id": resp.EmbeddingID,
		"model":        resp.ModelUsed,
		"dimensions":   resp.Dimensions,
		"tokens_used":  resp.TokensUsed,
	})

	return nil
}

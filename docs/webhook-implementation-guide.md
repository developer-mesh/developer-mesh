# Webhook Enhancement Implementation Guide

This guide provides concrete implementation details for the webhook enhancement plan, designed for mid-level engineers.

## Prerequisites

### Local Development Setup
```bash
# Required tools
- Go 1.21+
- Docker & Docker Compose
- PostgreSQL client (psql)
- Redis client (redis-cli)
- Make

# Start local environment
make dev

# Run database migrations
make migrate-up

# Verify services
docker-compose -f docker-compose.local.yml ps
```

## Step-by-Step Implementation

### Step 1: Database Schema Setup

#### 1.1 Create Migration File
```bash
# Create new migration
make create-migration name=add_package_releases_tables

# This creates two files in /migrations:
# - TIMESTAMP_add_package_releases_tables.up.sql
# - TIMESTAMP_add_package_releases_tables.down.sql
```

#### 1.2 Migration Content
```sql
-- migrations/TIMESTAMP_add_package_releases_tables.up.sql

-- Use the mcp schema
SET search_path TO mcp;

-- Create package releases table
CREATE TABLE IF NOT EXISTS package_releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
    repository_name TEXT NOT NULL,
    package_name TEXT NOT NULL,
    version TEXT NOT NULL,
    version_major INTEGER,
    version_minor INTEGER,
    version_patch INTEGER,
    prerelease TEXT,
    is_breaking_change BOOLEAN DEFAULT FALSE,
    release_notes TEXT,
    changelog TEXT,
    published_at TIMESTAMP WITH TIME ZONE NOT NULL,
    author_login TEXT,
    github_release_id BIGINT,
    artifactory_path TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_package_release UNIQUE(tenant_id, repository_name, version)
);

-- Add remaining tables from the plan...
-- (See webhook-enhancement-plan.md for complete schema)

-- Create indexes
CREATE INDEX idx_package_releases_name_version ON package_releases(package_name, version);
CREATE INDEX idx_package_releases_published_at ON package_releases(published_at DESC);
```

```sql
-- migrations/TIMESTAMP_add_package_releases_tables.down.sql
SET search_path TO mcp;

DROP TABLE IF EXISTS package_dependencies CASCADE;
DROP TABLE IF EXISTS package_api_changes CASCADE;
DROP TABLE IF EXISTS package_assets CASCADE;
DROP TABLE IF EXISTS package_releases CASCADE;
```

#### 1.3 Run Migration
```bash
# Apply migration
make migrate-up

# Verify tables were created
psql -h localhost -U devmesh -d devmesh_development -c "\dt mcp.package_*"
```

### Step 2: Repository Structure

#### 2.1 Create Package Structure
```bash
# Create new package directories
mkdir -p pkg/webhook/handlers
mkdir -p pkg/webhook/extractors
mkdir -p pkg/webhook/enrichers
mkdir -p pkg/clients/artifactory
mkdir -p pkg/repository/package
```

#### 2.2 File Organization
```
pkg/
├── webhook/
│   ├── handlers/
│   │   ├── github_release_handler.go      # GitHub release webhook handler
│   │   ├── github_release_handler_test.go
│   │   ├── artifactory_handler.go         # Artifactory webhook handler
│   │   └── artifactory_handler_test.go
│   ├── extractors/
│   │   ├── package_extractor.go           # Package type detection & extraction
│   │   ├── package_extractor_test.go
│   │   ├── npm_extractor.go               # NPM-specific extraction
│   │   ├── maven_extractor.go             # Maven-specific extraction
│   │   └── version_parser.go              # Semantic version parsing
│   └── enrichers/
│       ├── context_builder.go             # Build enriched context
│       ├── api_analyzer.go                # Analyze API changes
│       └── dependency_resolver.go         # Resolve dependencies
├── clients/
│   └── artifactory/
│       ├── client.go                       # Artifactory REST client
│       ├── client_test.go
│       └── types.go                        # Artifactory types
└── repository/
    └── package/
        ├── release_repository.go           # Package release DB operations
        ├── release_repository_test.go
        └── models.go                       # Domain models
```

### Step 3: Core Components Implementation

#### 3.1 Package Release Model
```go
// pkg/repository/package/models.go
package package

import (
    "time"
    "github.com/google/uuid"
)

type PackageRelease struct {
    ID               uuid.UUID  `db:"id" json:"id"`
    TenantID         uuid.UUID  `db:"tenant_id" json:"tenant_id"`
    RepositoryName   string     `db:"repository_name" json:"repository_name"`
    PackageName      string     `db:"package_name" json:"package_name"`
    Version          string     `db:"version" json:"version"`
    VersionMajor     *int       `db:"version_major" json:"version_major"`
    VersionMinor     *int       `db:"version_minor" json:"version_minor"`
    VersionPatch     *int       `db:"version_patch" json:"version_patch"`
    Prerelease       *string    `db:"prerelease" json:"prerelease"`
    IsBreakingChange bool       `db:"is_breaking_change" json:"is_breaking_change"`
    ReleaseNotes     *string    `db:"release_notes" json:"release_notes"`
    Changelog        *string    `db:"changelog" json:"changelog"`
    PublishedAt      time.Time  `db:"published_at" json:"published_at"`
    AuthorLogin      *string    `db:"author_login" json:"author_login"`
    GithubReleaseID  *int64     `db:"github_release_id" json:"github_release_id"`
    ArtifactoryPath  *string    `db:"artifactory_path" json:"artifactory_path"`
    CreatedAt        time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}

type PackageAsset struct {
    ID              uuid.UUID  `db:"id" json:"id"`
    ReleaseID       uuid.UUID  `db:"release_id" json:"release_id"`
    Name            string     `db:"name" json:"name"`
    ContentType     *string    `db:"content_type" json:"content_type"`
    SizeBytes       *int64     `db:"size_bytes" json:"size_bytes"`
    DownloadURL     *string    `db:"download_url" json:"download_url"`
    ArtifactoryURL  *string    `db:"artifactory_url" json:"artifactory_url"`
    SHA256Checksum  *string    `db:"sha256_checksum" json:"sha256_checksum"`
    Metadata        JSONB      `db:"metadata" json:"metadata"`
    CreatedAt       time.Time  `db:"created_at" json:"created_at"`
}
```

#### 3.2 Repository Implementation
```go
// pkg/repository/package/release_repository.go
package package

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/jmoiron/sqlx"
    "github.com/google/uuid"
)

type ReleaseRepository struct {
    db     *sqlx.DB
    logger observability.Logger
}

func NewReleaseRepository(db *sqlx.DB, logger observability.Logger) *ReleaseRepository {
    return &ReleaseRepository{
        db:     db,
        logger: logger,
    }
}

func (r *ReleaseRepository) CreateRelease(ctx context.Context, release *PackageRelease) error {
    query := `
        INSERT INTO mcp.package_releases (
            tenant_id, repository_name, package_name, version,
            version_major, version_minor, version_patch, prerelease,
            is_breaking_change, release_notes, changelog,
            published_at, author_login, github_release_id, artifactory_path
        ) VALUES (
            :tenant_id, :repository_name, :package_name, :version,
            :version_major, :version_minor, :version_patch, :prerelease,
            :is_breaking_change, :release_notes, :changelog,
            :published_at, :author_login, :github_release_id, :artifactory_path
        )
        ON CONFLICT (tenant_id, repository_name, version)
        DO UPDATE SET
            updated_at = CURRENT_TIMESTAMP,
            artifactory_path = COALESCE(EXCLUDED.artifactory_path, package_releases.artifactory_path)
        RETURNING id, created_at, updated_at
    `

    rows, err := r.db.NamedQueryContext(ctx, query, release)
    if err != nil {
        return fmt.Errorf("failed to create release: %w", err)
    }
    defer rows.Close()

    if rows.Next() {
        err = rows.Scan(&release.ID, &release.CreatedAt, &release.UpdatedAt)
        if err != nil {
            return fmt.Errorf("failed to scan returning values: %w", err)
        }
    }

    r.logger.Info("Package release created", map[string]interface{}{
        "release_id":   release.ID,
        "package_name": release.PackageName,
        "version":      release.Version,
    })

    return nil
}

func (r *ReleaseRepository) GetReleaseByVersion(ctx context.Context, tenantID uuid.UUID, packageName, version string) (*PackageRelease, error) {
    query := `
        SELECT * FROM mcp.package_releases
        WHERE tenant_id = $1 AND package_name = $2 AND version = $3
    `

    var release PackageRelease
    err := r.db.GetContext(ctx, &release, query, tenantID, packageName, version)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, nil
        }
        return nil, fmt.Errorf("failed to get release: %w", err)
    }

    return &release, nil
}

// Add more repository methods as needed...
```

#### 3.3 GitHub Release Handler
```go
// pkg/webhook/handlers/github_release_handler.go
package handlers

import (
    "context"
    "encoding/json"
    "fmt"
    "regexp"
    "strings"
    "time"

    "github.com/google/uuid"
    pkgRepo "github.com/developer-mesh/developer-mesh/pkg/repository/package"
    "github.com/developer-mesh/developer-mesh/pkg/webhook/extractors"
    "github.com/developer-mesh/developer-mesh/pkg/webhook/enrichers"
)

type GitHubReleaseHandler struct {
    releaseRepo     *pkgRepo.ReleaseRepository
    extractor       *extractors.PackageExtractor
    contextBuilder  *enrichers.ContextBuilder
    lifecycleManager *webhook.ContextLifecycleManager
    logger          observability.Logger
}

func NewGitHubReleaseHandler(
    releaseRepo *pkgRepo.ReleaseRepository,
    extractor *extractors.PackageExtractor,
    contextBuilder *enrichers.ContextBuilder,
    lifecycleManager *webhook.ContextLifecycleManager,
    logger observability.Logger,
) *GitHubReleaseHandler {
    return &GitHubReleaseHandler{
        releaseRepo:     releaseRepo,
        extractor:       extractor,
        contextBuilder:  contextBuilder,
        lifecycleManager: lifecycleManager,
        logger:          logger,
    }
}

func (h *GitHubReleaseHandler) Handle(ctx context.Context, event *webhook.WebhookEvent) error {
    // Extract action from payload
    action, ok := event.Payload["action"].(string)
    if !ok || action != "published" {
        h.logger.Debug("Skipping non-published release event", map[string]interface{}{
            "action": action,
        })
        return nil
    }

    // Parse release data
    releaseData, ok := event.Payload["release"].(map[string]interface{})
    if !ok {
        return fmt.Errorf("invalid release payload structure")
    }

    repoData, ok := event.Payload["repository"].(map[string]interface{})
    if !ok {
        return fmt.Errorf("invalid repository payload structure")
    }

    // Extract key fields
    tagName, _ := releaseData["tag_name"].(string)
    releaseName, _ := releaseData["name"].(string)
    releaseBody, _ := releaseData["body"].(string)
    isDraft, _ := releaseData["draft"].(bool)
    isPrerelease, _ := releaseData["prerelease"].(bool)
    publishedAt, _ := releaseData["published_at"].(string)

    repoFullName, _ := repoData["full_name"].(string)
    repoName, _ := repoData["name"].(string)

    if isDraft {
        h.logger.Debug("Skipping draft release", nil)
        return nil
    }

    // Parse version
    version := h.parseVersion(tagName)

    // Detect package type and extract metadata
    packageInfo, err := h.extractor.DetectAndExtract(ctx, repoFullName, tagName)
    if err != nil {
        h.logger.Warn("Failed to extract package info", map[string]interface{}{
            "error": err.Error(),
            "repo":  repoFullName,
            "tag":   tagName,
        })
        // Continue processing even if extraction fails
        packageInfo = &extractors.PackageInfo{
            Name: repoName,
            Type: extractors.PackageTypeUnknown,
        }
    }

    // Parse release notes
    releaseAnalysis := h.analyzeReleaseNotes(releaseBody)

    // Create package release record
    publishTime, _ := time.Parse(time.RFC3339, publishedAt)

    release := &pkgRepo.PackageRelease{
        TenantID:         uuid.MustParse(event.TenantId), // Use webhook tenant ID
        RepositoryName:   repoFullName,
        PackageName:      packageInfo.Name,
        Version:          tagName,
        VersionMajor:     version.Major,
        VersionMinor:     version.Minor,
        VersionPatch:     version.Patch,
        Prerelease:       version.Prerelease,
        IsBreakingChange: releaseAnalysis.HasBreakingChanges,
        ReleaseNotes:     &releaseBody,
        Changelog:        releaseAnalysis.Changelog,
        PublishedAt:      publishTime,
    }

    // Store in database
    if err := h.releaseRepo.CreateRelease(ctx, release); err != nil {
        return fmt.Errorf("failed to store release: %w", err)
    }

    // Build enriched context for embedding
    contextData := h.contextBuilder.BuildFromRelease(
        release,
        packageInfo,
        releaseAnalysis,
    )

    // Store context for semantic search
    metadata := &webhook.ContextMetadata{
        TenantID:   event.TenantId,
        SourceType: "github_release",
        SourceID:   fmt.Sprintf("%s@%s", packageInfo.Name, tagName),
        Importance: h.calculateImportance(releaseAnalysis),
        Tags: []string{
            "release",
            string(packageInfo.Type),
            packageInfo.Name,
            tagName,
        },
    }

    if err := h.lifecycleManager.StoreContext(ctx, event.TenantId, contextData, metadata); err != nil {
        h.logger.Error("Failed to store context", map[string]interface{}{
            "error":   err.Error(),
            "release": release.ID,
        })
    }

    h.logger.Info("GitHub release processed successfully", map[string]interface{}{
        "package_name": packageInfo.Name,
        "version":      tagName,
        "release_id":   release.ID,
    })

    return nil
}

func (h *GitHubReleaseHandler) parseVersion(tag string) *extractors.Version {
    // Remove common prefixes
    tag = strings.TrimPrefix(tag, "v")
    tag = strings.TrimPrefix(tag, "release-")
    tag = strings.TrimPrefix(tag, "version-")

    // Parse semantic version
    // Pattern: MAJOR.MINOR.PATCH[-PRERELEASE][+METADATA]
    pattern := `^(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9\-\.]+))?(?:\+([a-zA-Z0-9\-\.]+))?$`
    re := regexp.MustCompile(pattern)

    matches := re.FindStringSubmatch(tag)
    if len(matches) >= 4 {
        major := h.parseInt(matches[1])
        minor := h.parseInt(matches[2])
        patch := h.parseInt(matches[3])

        var prerelease *string
        if len(matches) > 4 && matches[4] != "" {
            prerelease = &matches[4]
        }

        return &extractors.Version{
            Major:      &major,
            Minor:      &minor,
            Patch:      &patch,
            Prerelease: prerelease,
            Raw:        tag,
        }
    }

    // Non-semver version
    return &extractors.Version{
        Raw: tag,
    }
}

func (h *GitHubReleaseHandler) parseInt(s string) int {
    var i int
    fmt.Sscanf(s, "%d", &i)
    return i
}

func (h *GitHubReleaseHandler) analyzeReleaseNotes(body string) *ReleaseAnalysis {
    analysis := &ReleaseAnalysis{
        RawNotes: body,
    }

    lowerBody := strings.ToLower(body)

    // Check for breaking changes
    breakingPatterns := []string{
        "breaking change",
        "breaking:",
        "⚠️ breaking",
        "incompatible",
        "migration required",
    }

    for _, pattern := range breakingPatterns {
        if strings.Contains(lowerBody, pattern) {
            analysis.HasBreakingChanges = true
            break
        }
    }

    // Extract sections
    analysis.Features = h.extractSection(body, "features", "new features", "what's new", "added")
    analysis.Fixes = h.extractSection(body, "bug fixes", "fixes", "fixed", "resolved")
    analysis.Deprecated = h.extractSection(body, "deprecated", "deprecations")

    // Build changelog summary
    if len(analysis.Features) > 0 || len(analysis.Fixes) > 0 {
        changelog := ""
        if len(analysis.Features) > 0 {
            changelog += "Features:\n" + strings.Join(analysis.Features, "\n") + "\n\n"
        }
        if len(analysis.Fixes) > 0 {
            changelog += "Fixes:\n" + strings.Join(analysis.Fixes, "\n")
        }
        analysis.Changelog = &changelog
    }

    return analysis
}

func (h *GitHubReleaseHandler) extractSection(body string, markers ...string) []string {
    // Implementation to extract bullet points under section headers
    // This is a simplified version - you may want to use a markdown parser

    lines := strings.Split(body, "\n")
    var items []string
    inSection := false

    for _, line := range lines {
        lineLower := strings.ToLower(strings.TrimSpace(line))

        // Check if we're entering a section
        for _, marker := range markers {
            if strings.Contains(lineLower, marker) {
                inSection = true
                break
            }
        }

        // Check if we're leaving the section (new header)
        if inSection && strings.HasPrefix(line, "#") && !strings.Contains(lineLower, markers[0]) {
            inSection = false
        }

        // Extract items (lines starting with -, *, or numbers)
        if inSection {
            trimmed := strings.TrimSpace(line)
            if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
                item := strings.TrimPrefix(trimmed, "-")
                item = strings.TrimPrefix(item, "*")
                item = strings.TrimSpace(item)
                if item != "" {
                    items = append(items, item)
                }
            }
        }
    }

    return items
}

func (h *GitHubReleaseHandler) calculateImportance(analysis *ReleaseAnalysis) float64 {
    importance := 0.5 // Base importance

    if analysis.HasBreakingChanges {
        importance += 0.3
    }

    if len(analysis.Features) > 0 {
        importance += 0.1
    }

    if len(analysis.Fixes) > 5 {
        importance += 0.1
    }

    return min(importance, 1.0)
}

type ReleaseAnalysis struct {
    RawNotes           string
    HasBreakingChanges bool
    Features           []string
    Fixes              []string
    Deprecated         []string
    Changelog          *string
}
```

### Step 4: Wire Everything Together

#### 4.1 Update Worker Initialization
```go
// apps/worker/main.go - Add to the initialization section

import (
    pkgRepo "github.com/developer-mesh/developer-mesh/pkg/repository/package"
    "github.com/developer-mesh/developer-mesh/pkg/webhook/handlers"
    "github.com/developer-mesh/developer-mesh/pkg/webhook/extractors"
    "github.com/developer-mesh/developer-mesh/pkg/webhook/enrichers"
)

// In main() or initialization function:

// Create package release repository
releaseRepo := pkgRepo.NewReleaseRepository(db, logger)

// Create package extractor
packageExtractor := extractors.NewPackageExtractor(
    githubClient,
    redisClient,
    logger,
)

// Create context builder
contextBuilder := enrichers.NewContextBuilder(
    embeddingService,
    logger,
)

// Create GitHub release handler
githubReleaseHandler := handlers.NewGitHubReleaseHandler(
    releaseRepo,
    packageExtractor,
    contextBuilder,
    lifecycleManager,
    logger,
)

// Register the handler for GitHub release events
eventProcessor.RegisterHandler("github.release", githubReleaseHandler.Handle)
```

#### 4.2 Add Webhook Routes
```go
// apps/rest-api/internal/api/routes.go - Add webhook route

// In RegisterRoutes() function:

// GitHub webhooks
router.HandleFunc("/webhooks/github", server.handleGitHubWebhook).Methods("POST")

// Artifactory webhooks
router.HandleFunc("/webhooks/artifactory", server.handleArtifactoryWebhook).Methods("POST")
```

### Step 5: Testing

#### 5.1 Unit Test Example
```go
// pkg/webhook/handlers/github_release_handler_test.go
package handlers

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestGitHubReleaseHandler_Handle(t *testing.T) {
    // Setup
    mockRepo := new(MockReleaseRepository)
    mockExtractor := new(MockPackageExtractor)
    mockBuilder := new(MockContextBuilder)
    mockLifecycle := new(MockLifecycleManager)
    logger := observability.NewTestLogger()

    handler := NewGitHubReleaseHandler(
        mockRepo,
        mockExtractor,
        mockBuilder,
        mockLifecycle,
        logger,
    )

    // Test data
    event := &webhook.WebhookEvent{
        EventId:  "test-123",
        TenantId: "00000000-0000-0000-0000-000000000000",
        EventType: "release",
        Payload: map[string]interface{}{
            "action": "published",
            "release": map[string]interface{}{
                "tag_name": "v1.2.3",
                "name": "Release 1.2.3",
                "body": "## Features\n- New feature\n## Bug Fixes\n- Fixed bug",
                "draft": false,
                "prerelease": false,
                "published_at": "2024-01-15T10:00:00Z",
            },
            "repository": map[string]interface{}{
                "full_name": "org/repo",
                "name": "repo",
            },
        },
    }

    // Set expectations
    mockExtractor.On("DetectAndExtract", mock.Anything, "org/repo", "v1.2.3").
        Return(&extractors.PackageInfo{
            Name: "repo",
            Type: extractors.PackageTypeNPM,
        }, nil)

    mockRepo.On("CreateRelease", mock.Anything, mock.Anything).
        Return(nil)

    mockBuilder.On("BuildFromRelease", mock.Anything, mock.Anything, mock.Anything).
        Return(map[string]interface{}{
            "package": "repo",
            "version": "v1.2.3",
        })

    mockLifecycle.On("StoreContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
        Return(nil)

    // Execute
    err := handler.Handle(context.Background(), event)

    // Assert
    assert.NoError(t, err)
    mockRepo.AssertCalled(t, "CreateRelease", mock.Anything, mock.Anything)
    mockLifecycle.AssertCalled(t, "StoreContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
```

#### 5.2 Integration Test
```go
// test/integration/github_release_test.go
package integration

import (
    "testing"
    "bytes"
    "net/http"
    "encoding/json"
)

func TestGitHubReleaseWebhook(t *testing.T) {
    // Skip if not integration test
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Prepare webhook payload
    payload := map[string]interface{}{
        "action": "published",
        "release": map[string]interface{}{
            "tag_name": "v1.0.0",
            "body": "Test release",
        },
        "repository": map[string]interface{}{
            "full_name": "test/repo",
        },
    }

    body, _ := json.Marshal(payload)

    // Send webhook
    req, _ := http.NewRequest("POST", "http://localhost:8081/webhooks/github", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-GitHub-Event", "release")
    req.Header.Set("X-GitHub-Delivery", "test-delivery-123")

    client := &http.Client{}
    resp, err := client.Do(req)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    // Verify database
    // Check that release was created in database
    // Check that context was stored
}
```

### Step 6: Configuration

#### 6.1 Environment Configuration
```bash
# .env.development - Add these variables

# GitHub Configuration
GITHUB_TOKEN=ghp_xxxxxxxxxxxx
GITHUB_WEBHOOK_SECRET=your-webhook-secret

# Artifactory Configuration
ARTIFACTORY_URL=https://artifactory.company.com
ARTIFACTORY_API_KEY=AKCxxxxxxxxxx
ARTIFACTORY_WEBHOOK_TOKEN=webhook-token

# Feature Flags
ENABLE_PACKAGE_TRACKING=true
ENABLE_GITHUB_RELEASE_HANDLER=true
ENABLE_ARTIFACTORY_HANDLER=false  # Enable when ready
```

#### 6.2 Config Structure Updates
```go
// pkg/common/config/config.go - Add to existing config

type Config struct {
    // ... existing fields ...

    // Package tracking
    PackageTracking PackageTrackingConfig `mapstructure:"package_tracking"`

    // Artifactory
    Artifactory ArtifactoryConfig `mapstructure:"artifactory"`
}

type PackageTrackingConfig struct {
    Enabled                bool   `mapstructure:"enabled"`
    EnableGitHubReleases   bool   `mapstructure:"enable_github_releases"`
    EnableArtifactory      bool   `mapstructure:"enable_artifactory"`
    MaxPackageSize         int64  `mapstructure:"max_package_size"`
    CacheDuration          string `mapstructure:"cache_duration"`
}

type ArtifactoryConfig struct {
    URL           string `mapstructure:"url"`
    APIKey        string `mapstructure:"api_key"`
    WebhookToken  string `mapstructure:"webhook_token"`
    Timeout       string `mapstructure:"timeout"`
}
```

### Step 7: Monitoring & Debugging

#### 7.1 Add Metrics
```go
// In the handler
h.metricsClient.IncrementCounter("webhook.github.release.processed", 1.0)
h.metricsClient.RecordHistogram("webhook.github.release.duration", time.Since(start).Seconds())
```

#### 7.2 Debugging Commands
```bash
# Check if tables exist
psql -h localhost -U devmesh -d devmesh_development \
  -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'mcp' AND table_name LIKE 'package_%';"

# View recent releases
psql -h localhost -U devmesh -d devmesh_development \
  -c "SELECT package_name, version, published_at FROM mcp.package_releases ORDER BY published_at DESC LIMIT 10;"

# Check webhook processing logs
docker-compose logs -f worker | grep -i "release"

# Monitor Redis queue
redis-cli xinfo stream webhook-events

# Test webhook manually
curl -X POST http://localhost:8081/webhooks/github \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: release" \
  -H "X-GitHub-Delivery: test-123" \
  -d @test/fixtures/github_release_payload.json
```

### Step 8: Common Issues & Solutions

#### Issue 1: Database Migration Fails
```bash
# Check current migration status
make migrate-status

# Rollback if needed
make migrate-down

# Fix the migration file and retry
make migrate-up
```

#### Issue 2: Webhook Not Processing
```bash
# Check worker is running
docker-compose ps worker

# Check Redis stream
redis-cli xlen webhook-events

# Check for errors
docker-compose logs worker --tail=100 | grep ERROR
```

#### Issue 3: Context Not Being Stored
```go
// Add debug logging
h.logger.Debug("Storing context", map[string]interface{}{
    "tenant_id": event.TenantId,
    "context_size": len(contextData),
})
```

### Step 9: Validation Checklist

- [ ] Database migrations applied successfully
- [ ] Package release is stored in database
- [ ] Context is generated and stored
- [ ] Embedding is created
- [ ] Semantic search returns the release
- [ ] Metrics are recorded
- [ ] Tests pass
- [ ] No errors in logs

### Step 10: Deployment Notes

1. **Migration Order**: Always run database migrations before deploying new code
2. **Feature Flags**: Use feature flags to gradually roll out
3. **Monitoring**: Watch error rates and processing times after deployment
4. **Rollback Plan**: Keep previous version ready for quick rollback

## Additional Resources

- [GitHub Webhook Events Documentation](https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads#release)
- [Artifactory REST API](https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API)
- [Semantic Versioning Spec](https://semver.org/)
- [Go SQL Package Tutorial](https://go.dev/doc/tutorial/database-access)

## Quick Reference Commands

```bash
# Development workflow
make fmt                    # Format code
make lint                   # Run linter
make test                   # Run tests
make pre-commit            # Run all checks

# Database
make migrate-up            # Apply migrations
make migrate-down          # Rollback last migration
make migrate-status        # Check migration status

# Docker
docker-compose up -d       # Start services
docker-compose logs -f     # View logs
docker-compose restart worker  # Restart worker

# Testing
go test ./pkg/webhook/handlers/...  # Test handlers
go test -v -run TestGitHubRelease   # Run specific test
```
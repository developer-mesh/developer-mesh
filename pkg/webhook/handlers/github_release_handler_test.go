package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPackageReleaseRepository is a mock implementation
type MockPackageReleaseRepository struct {
	mock.Mock
}

func (m *MockPackageReleaseRepository) Create(ctx context.Context, release *models.PackageRelease) error {
	args := m.Called(ctx, release)
	return args.Error(0)
}

func (m *MockPackageReleaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PackageRelease, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PackageRelease), args.Error(1)
}

func (m *MockPackageReleaseRepository) GetByVersion(ctx context.Context, tenantID uuid.UUID, packageName, version string) (*models.PackageRelease, error) {
	args := m.Called(ctx, tenantID, packageName, version)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PackageRelease), args.Error(1)
}

func (m *MockPackageReleaseRepository) GetByRepository(ctx context.Context, tenantID uuid.UUID, repoName string, limit, offset int) ([]*models.PackageRelease, error) {
	args := m.Called(ctx, tenantID, repoName, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PackageRelease), args.Error(1)
}

func (m *MockPackageReleaseRepository) GetLatestByPackage(ctx context.Context, tenantID uuid.UUID, packageName string) (*models.PackageRelease, error) {
	args := m.Called(ctx, tenantID, packageName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PackageRelease), args.Error(1)
}

func (m *MockPackageReleaseRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*models.PackageRelease, error) {
	args := m.Called(ctx, tenantID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PackageRelease), args.Error(1)
}

func (m *MockPackageReleaseRepository) Update(ctx context.Context, release *models.PackageRelease) error {
	args := m.Called(ctx, release)
	return args.Error(0)
}

func (m *MockPackageReleaseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPackageReleaseRepository) CreateAsset(ctx context.Context, asset *models.PackageAsset) error {
	args := m.Called(ctx, asset)
	return args.Error(0)
}

func (m *MockPackageReleaseRepository) GetAssetsByReleaseID(ctx context.Context, releaseID uuid.UUID) ([]*models.PackageAsset, error) {
	args := m.Called(ctx, releaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PackageAsset), args.Error(1)
}

func (m *MockPackageReleaseRepository) CreateAPIChange(ctx context.Context, change *models.PackageAPIChange) error {
	args := m.Called(ctx, change)
	return args.Error(0)
}

func (m *MockPackageReleaseRepository) GetAPIChangesByReleaseID(ctx context.Context, releaseID uuid.UUID) ([]*models.PackageAPIChange, error) {
	args := m.Called(ctx, releaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PackageAPIChange), args.Error(1)
}

func (m *MockPackageReleaseRepository) CreateDependency(ctx context.Context, dep *models.PackageDependency) error {
	args := m.Called(ctx, dep)
	return args.Error(0)
}

func (m *MockPackageReleaseRepository) GetDependenciesByReleaseID(ctx context.Context, releaseID uuid.UUID) ([]*models.PackageDependency, error) {
	args := m.Called(ctx, releaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PackageDependency), args.Error(1)
}

func (m *MockPackageReleaseRepository) GetWithDetails(ctx context.Context, id uuid.UUID) (*models.PackageReleaseWithDetails, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PackageReleaseWithDetails), args.Error(1)
}

func (m *MockPackageReleaseRepository) SearchByName(ctx context.Context, tenantID uuid.UUID, namePattern string, limit int) ([]*models.PackageRelease, error) {
	args := m.Called(ctx, tenantID, namePattern, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PackageRelease), args.Error(1)
}

func (m *MockPackageReleaseRepository) GetVersionHistory(ctx context.Context, tenantID uuid.UUID, packageName string, limit int) ([]*models.PackageRelease, error) {
	args := m.Called(ctx, tenantID, packageName, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PackageRelease), args.Error(1)
}

func (m *MockPackageReleaseRepository) FindByDependency(ctx context.Context, tenantID uuid.UUID, dependencyName string, limit int) ([]*models.PackageRelease, error) {
	args := m.Called(ctx, tenantID, dependencyName, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.PackageRelease), args.Error(1)
}

func TestGitHubReleaseHandler_ParseVersion(t *testing.T) {
	handler := &GitHubReleaseHandler{}

	tests := []struct {
		name               string
		tag                string
		expectedMajor      int
		expectedMinor      int
		expectedPatch      int
		expectedPrerelease *string
	}{
		{
			name:          "simple version with v prefix",
			tag:           "v1.2.3",
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 3,
		},
		{
			name:          "version without prefix",
			tag:           "2.0.1",
			expectedMajor: 2,
			expectedMinor: 0,
			expectedPatch: 1,
		},
		{
			name:               "version with prerelease",
			tag:                "v3.1.4-alpha.1",
			expectedMajor:      3,
			expectedMinor:      1,
			expectedPatch:      4,
			expectedPrerelease: strPtr("alpha.1"),
		},
		{
			name:               "version with beta",
			tag:                "1.0.0-beta",
			expectedMajor:      1,
			expectedMinor:      0,
			expectedPatch:      0,
			expectedPrerelease: strPtr("beta"),
		},
		{
			name:          "version with release- prefix",
			tag:           "release-5.2.1",
			expectedMajor: 5,
			expectedMinor: 2,
			expectedPatch: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := handler.parseVersion(tt.tag)

			assert.Equal(t, tt.expectedMajor, version.Major)
			assert.Equal(t, tt.expectedMinor, version.Minor)
			assert.Equal(t, tt.expectedPatch, version.Patch)

			if tt.expectedPrerelease != nil {
				assert.NotNil(t, version.Prerelease)
				assert.Equal(t, *tt.expectedPrerelease, *version.Prerelease)
			} else {
				assert.Nil(t, version.Prerelease)
			}
		})
	}
}

func TestGitHubReleaseHandler_ParseReleaseNotes(t *testing.T) {
	handler := &GitHubReleaseHandler{}

	tests := []struct {
		name                     string
		body                     string
		expectedHasBreaking      bool
		expectedBreakingCount    int
		expectedNewFeaturesCount int
		expectedBugFixesCount    int
	}{
		{
			name: "release notes with breaking changes",
			body: `## Breaking Changes
- Removed deprecated API endpoint
- Changed authentication method

## Features
- Added new dashboard
- Improved performance

## Bug Fixes
- Fixed login issue
- Resolved memory leak`,
			expectedHasBreaking:      true,
			expectedBreakingCount:    2,
			expectedNewFeaturesCount: 2,
			expectedBugFixesCount:    2,
		},
		{
			name: "release notes without breaking changes",
			body: `## New Features
- Added dark mode
- New export feature

## Bug Fixes
- Fixed CSS bug
- Updated dependencies`,
			expectedHasBreaking:      false,
			expectedBreakingCount:    0,
			expectedNewFeaturesCount: 2,
			expectedBugFixesCount:    2,
		},
		{
			name:                     "empty release notes",
			body:                     "",
			expectedHasBreaking:      false,
			expectedBreakingCount:    0,
			expectedNewFeaturesCount: 0,
			expectedBugFixesCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notes := handler.parseReleaseNotes(tt.body)

			assert.Equal(t, tt.expectedHasBreaking, notes.HasBreakingChange)
			assert.Equal(t, tt.expectedBreakingCount, len(notes.BreakingChanges))
			assert.Equal(t, tt.expectedNewFeaturesCount, len(notes.NewFeatures))
			assert.Equal(t, tt.expectedBugFixesCount, len(notes.BugFixes))
		})
	}
}

func TestGitHubReleaseHandler_Handle_Published(t *testing.T) {
	mockRepo := new(MockPackageReleaseRepository)
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()

	handler := NewGitHubReleaseHandler(mockRepo, nil, logger, metrics)

	// Create a sample GitHub release payload
	payload := GitHubReleasePayload{
		Action: "published",
		Release: GitHubRelease{
			ID:          12345,
			TagName:     "v1.0.0",
			Name:        "First Release",
			Body:        "## Features\n- Initial release",
			Draft:       false,
			Prerelease:  false,
			PublishedAt: time.Now().Format(time.RFC3339),
			Author: GitHubUser{
				Login: "testuser",
			},
			Assets: []GitHubAsset{},
		},
		Repository: GitHubRepository{
			FullName:    "test/repo",
			Description: "Test repository",
		},
	}

	payloadBytes, _ := json.Marshal(payload)

	event := queue.Event{
		EventID:   "test-event-1",
		EventType: "release",
		Payload:   payloadBytes,
		AuthContext: &queue.EventAuthContext{
			TenantID: "00000000-0000-0000-0000-000000000001",
		},
		Timestamp: time.Now(),
	}

	// Mock the repository Create call
	mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *models.PackageRelease) bool {
		return r.PackageName == "First Release" && r.Version == "v1.0.0"
	})).Return(nil)

	// Execute the handler
	err := handler.Handle(context.Background(), event)

	// Assertions
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestGitHubReleaseHandler_Handle_SkipDraft(t *testing.T) {
	mockRepo := new(MockPackageReleaseRepository)
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()

	handler := NewGitHubReleaseHandler(mockRepo, nil, logger, metrics)

	payload := GitHubReleasePayload{
		Action: "published",
		Release: GitHubRelease{
			TagName: "v1.0.0",
			Draft:   true, // This is a draft
		},
	}

	payloadBytes, _ := json.Marshal(payload)

	event := queue.Event{
		EventID:   "test-event-2",
		EventType: "release",
		Payload:   payloadBytes,
		Timestamp: time.Now(),
	}

	// Execute - should not call repository
	err := handler.Handle(context.Background(), event)

	// Should return nil without calling repository
	assert.NoError(t, err)
	mockRepo.AssertNotCalled(t, "Create")
}

func TestGitHubReleaseHandler_ExtractSection(t *testing.T) {
	handler := &GitHubReleaseHandler{}

	body := `# Release v1.0.0

## Breaking Changes
- Removed old API
- Changed configuration format

## Features
- Added new feature A
- Improved feature B

## Bug Fixes
- Fixed issue #123
- Resolved crash on startup`

	tests := []struct {
		name          string
		headers       []string
		expectedCount int
		expectedFirst string
	}{
		{
			name:          "extract breaking changes",
			headers:       []string{"breaking changes", "breaking"},
			expectedCount: 2,
			expectedFirst: "Removed old API",
		},
		{
			name:          "extract features",
			headers:       []string{"features", "new features"},
			expectedCount: 2,
			expectedFirst: "Added new feature A",
		},
		{
			name:          "extract bug fixes",
			headers:       []string{"bug fixes", "fixes"},
			expectedCount: 2,
			expectedFirst: "Fixed issue #123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := handler.extractSection(body, tt.headers)

			assert.Equal(t, tt.expectedCount, len(items))
			if len(items) > 0 {
				assert.Equal(t, tt.expectedFirst, items[0])
			}
		})
	}
}

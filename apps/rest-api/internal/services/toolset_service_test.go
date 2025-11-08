package services

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDynamicToolRepository is a mock implementation of DynamicToolRepository
type MockDynamicToolRepository struct {
	mock.Mock
}

func (m *MockDynamicToolRepository) List(ctx context.Context, tenantID, status string) ([]*models.DynamicTool, error) {
	args := m.Called(ctx, tenantID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.DynamicTool), args.Error(1)
}

func (m *MockDynamicToolRepository) GetByID(ctx context.Context, id string) (*models.DynamicTool, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DynamicTool), args.Error(1)
}

func (m *MockDynamicToolRepository) Create(ctx context.Context, tool *models.DynamicTool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}

func (m *MockDynamicToolRepository) Update(ctx context.Context, tool *models.DynamicTool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}

func (m *MockDynamicToolRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDynamicToolRepository) UpdateStatus(ctx context.Context, id, status string) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockDynamicToolRepository) GetByToolName(ctx context.Context, tenantID, toolName string) (*models.DynamicTool, error) {
	args := m.Called(ctx, tenantID, toolName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DynamicTool), args.Error(1)
}

func (m *MockDynamicToolRepository) ListByTenantAndProvider(ctx context.Context, tenantID, provider string) ([]*models.DynamicTool, error) {
	args := m.Called(ctx, tenantID, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.DynamicTool), args.Error(1)
}

func TestParseToolName(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	mockRepo := new(MockDynamicToolRepository)
	service := NewToolsetService(mockRepo, logger)

	tests := []struct {
		name             string
		toolName         string
		expectedProvider string
		expectedResource string
		expectedAction   string
	}{
		{
			name:             "standard GitHub tool with action first",
			toolName:         "mcp__devmesh__github_get_repository",
			expectedProvider: "github",
			expectedResource: "repos",
			expectedAction:   "get",
		},
		{
			name:             "GitHub tool with resource first (no action detected)",
			toolName:         "mcp__devmesh__github_repos_list",
			expectedProvider: "github",
			expectedResource: "repos_list", // whole thing treated as resource since "repos" isn't an action verb
			expectedAction:   "",
		},
		{
			name:             "GitHub pull request tool",
			toolName:         "mcp__devmesh__github_create_pull_request",
			expectedProvider: "github",
			expectedResource: "pulls",
			expectedAction:   "create",
		},
		{
			name:             "GitHub list issues (action detected)",
			toolName:         "mcp__devmesh__github_list_issues",
			expectedProvider: "github",
			expectedResource: "issues",
			expectedAction:   "list",
		},
		{
			name:             "Harness execute pipeline (action detected)",
			toolName:         "mcp__devmesh__harness_execute_pipelines",
			expectedProvider: "harness",
			expectedResource: "pipelines",
			expectedAction:   "execute",
		},
		{
			name:             "GitHub run workflows (action detected)",
			toolName:         "mcp__devmesh__github_run_workflows",
			expectedProvider: "github",
			expectedResource: "workflows",
			expectedAction:   "run",
		},
		{
			name:             "GitHub list secrets (action detected)",
			toolName:         "mcp__devmesh__github_list_secrets",
			expectedProvider: "github",
			expectedResource: "security",
			expectedAction:   "list",
		},
		{
			name:             "GitHub files tool",
			toolName:         "mcp__devmesh__github_get_file_contents",
			expectedProvider: "github",
			expectedResource: "file_contents", // "file" normalizes to "code", but "file_contents" stays as is
			expectedAction:   "get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, resource, action := service.parseToolName(tt.toolName)
			assert.Equal(t, tt.expectedProvider, provider, "Provider mismatch")
			assert.Equal(t, tt.expectedResource, resource, "Resource mismatch")
			assert.Equal(t, tt.expectedAction, action, "Action mismatch")
		})
	}
}

func TestNormalizeResource(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	mockRepo := new(MockDynamicToolRepository)
	service := NewToolsetService(mockRepo, logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Repository variations
		{
			name:     "repository singular",
			input:    "repository",
			expected: "repos",
		},
		{
			name:     "repositories plural",
			input:    "repositories",
			expected: "repos",
		},
		{
			name:     "repos already normalized",
			input:    "repos",
			expected: "repos",
		},
		// Pull request variations
		{
			name:     "pull_request singular",
			input:    "pull_request",
			expected: "pulls",
		},
		{
			name:     "pull_requests plural",
			input:    "pull_requests",
			expected: "pulls",
		},
		{
			name:     "pulls already normalized",
			input:    "pulls",
			expected: "pulls",
		},
		// Workflow variations
		{
			name:     "pipeline singular",
			input:    "pipeline",
			expected: "pipelines",
		},
		{
			name:     "pipelines plural",
			input:    "pipelines",
			expected: "pipelines",
		},
		{
			name:     "workflow singular",
			input:    "workflow",
			expected: "workflows",
		},
		// File and code variations
		{
			name:     "file singular",
			input:    "file",
			expected: "code",
		},
		{
			name:     "files plural",
			input:    "files",
			expected: "code",
		},
		{
			name:     "content",
			input:    "content",
			expected: "code",
		},
		// Security variations
		{
			name:     "secret singular",
			input:    "secret",
			expected: "security",
		},
		{
			name:     "secrets plural",
			input:    "secrets",
			expected: "security",
		},
		// Pass through for unrecognized
		{
			name:     "unknown resource",
			input:    "unknown",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.normalizeResource(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAutoGenerateToolsets(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	mockRepo := new(MockDynamicToolRepository)
	service := NewToolsetService(mockRepo, logger)

	tests := []struct {
		name                string
		tools               []*models.DynamicTool
		expectedToolsetCount int
		expectedToolsets    map[string]int // toolset name -> expected tool count
	}{
		{
			name: "GitHub tools grouped correctly",
			tools: []*models.DynamicTool{
				{
					ID:          "1",
					ToolName:    "mcp__devmesh__github_get_repository",
					DisplayName: "Get Repository",
					Status:      "active",
				},
				{
					ID:          "2",
					ToolName:    "mcp__devmesh__github_list_repositories",
					DisplayName: "List Repositories",
					Status:      "active",
				},
				{
					ID:          "3",
					ToolName:    "mcp__devmesh__github_create_issue",
					DisplayName: "Create Issue",
					Status:      "active",
				},
				{
					ID:          "4",
					ToolName:    "mcp__devmesh__github_list_issues",
					DisplayName: "List Issues",
					Status:      "active",
				},
			},
			expectedToolsetCount: 2,
			expectedToolsets: map[string]int{
				"github_repos":  2,
				"github_issues": 2,
			},
		},
		{
			name: "Multiple providers grouped correctly",
			tools: []*models.DynamicTool{
				{
					ID:          "1",
					ToolName:    "mcp__devmesh__github_list_repositories",
					DisplayName: "List Repos",
					Status:      "active",
				},
				{
					ID:          "2",
					ToolName:    "mcp__devmesh__harness_execute_pipelines",
					DisplayName: "Execute Pipeline",
					Status:      "active",
				},
				{
					ID:          "3",
					ToolName:    "mcp__devmesh__github_create_issue",
					DisplayName: "Create Issue",
					Status:      "active",
				},
			},
			expectedToolsetCount: 3,
			expectedToolsets: map[string]int{
				"github_repos":      1,
				"github_issues":     1,
				"harness_pipelines": 1,
			},
		},
		{
			name: "Inactive tools filtered out",
			tools: []*models.DynamicTool{
				{
					ID:          "1",
					ToolName:    "mcp__devmesh__github_list_repositories",
					DisplayName: "List Repos",
					Status:      "active",
				},
				{
					ID:          "2",
					ToolName:    "mcp__devmesh__github_get_repository",
					DisplayName: "Get Repo",
					Status:      "inactive",
				},
				{
					ID:          "3",
					ToolName:    "mcp__devmesh__github_list_issues",
					DisplayName: "List Issues",
					Status:      "active",
				},
			},
			expectedToolsetCount: 2,
			expectedToolsets: map[string]int{
				"github_repos":  1, // Only active tool
				"github_issues": 1,
			},
		},
		{
			name:                 "Empty tool list",
			tools:                []*models.DynamicTool{},
			expectedToolsetCount: 0,
			expectedToolsets:     map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockRepo.On("List", mock.Anything, "tenant-1", "").Return(tt.tools, nil).Once()

			// Execute
			toolsets, err := service.AutoGenerateToolsets(context.Background(), "tenant-1")

			// Verify
			require.NoError(t, err)
			assert.Equal(t, tt.expectedToolsetCount, len(toolsets), "Toolset count mismatch")

			// Verify each expected toolset
			for toolsetName, expectedToolCount := range tt.expectedToolsets {
				found := false
				for _, ts := range toolsets {
					if ts.Toolset.Name == toolsetName {
						found = true
						assert.Equal(t, expectedToolCount, len(ts.Tools),
							"Tool count mismatch for toolset %s", toolsetName)
						assert.Equal(t, expectedToolCount, len(ts.Toolset.Tools),
							"Toolset.Tools length mismatch for toolset %s", toolsetName)
						break
					}
				}
				assert.True(t, found, "Expected toolset %s not found", toolsetName)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestAutoGenerateToolsets_RepositoryError(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	mockRepo := new(MockDynamicToolRepository)
	service := NewToolsetService(mockRepo, logger)

	// Setup mock to return error
	mockRepo.On("List", mock.Anything, "tenant-1", "").Return(nil, assert.AnError).Once()

	// Execute
	toolsets, err := service.AutoGenerateToolsets(context.Background(), "tenant-1")

	// Verify
	assert.Error(t, err)
	assert.Nil(t, toolsets)
	mockRepo.AssertExpectations(t)
}

func TestToolsetMetadata(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	mockRepo := new(MockDynamicToolRepository)
	service := NewToolsetService(mockRepo, logger)

	desc1 := "List all repositories"
	desc2 := "Get a specific repository"

	tools := []*models.DynamicTool{
		{
			ID:          "1",
			ToolName:    "mcp__devmesh__github_list_repositories",
			DisplayName: "List Repositories",
			Description: &desc1,
			Status:      "active",
		},
		{
			ID:          "2",
			ToolName:    "mcp__devmesh__github_get_repository",
			DisplayName: "Get Repository",
			Description: &desc2,
			Status:      "active",
		},
	}

	mockRepo.On("List", mock.Anything, "tenant-1", "").Return(tools, nil).Once()

	toolsets, err := service.AutoGenerateToolsets(context.Background(), "tenant-1")
	require.NoError(t, err)
	require.Equal(t, 1, len(toolsets))

	toolset := toolsets[0]

	// Verify toolset metadata
	assert.Equal(t, "github_repos", toolset.Toolset.Name)
	assert.Equal(t, "GitHub Repositories", toolset.Toolset.DisplayName) // Display name comes from generateDisplayName
	assert.NotEmpty(t, toolset.Toolset.Description)
	assert.Equal(t, "repository", toolset.Toolset.Category)
	assert.Equal(t, "github", toolset.Toolset.Provider)
	assert.Contains(t, toolset.Toolset.Tags, "github")
	assert.Equal(t, 2, len(toolset.Toolset.Tools))
	assert.Equal(t, 2, len(toolset.Tools))

	// Verify action counts
	assert.Equal(t, 1, toolset.ActionCounts["list"])
	assert.Equal(t, 1, toolset.ActionCounts["get"])

	mockRepo.AssertExpectations(t)
}

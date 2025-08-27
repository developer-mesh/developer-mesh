package adapters

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/feature"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/services"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockOrgToolRepository mocks the OrganizationToolRepository
type mockOrgToolRepository struct {
	mock.Mock
}

func (m *mockOrgToolRepository) Create(ctx context.Context, tool *models.OrganizationTool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}

func (m *mockOrgToolRepository) GetByID(ctx context.Context, id string) (*models.OrganizationTool, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OrganizationTool), args.Error(1)
}

func (m *mockOrgToolRepository) ListByOrganization(ctx context.Context, orgID string) ([]*models.OrganizationTool, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.OrganizationTool), args.Error(1)
}

func (m *mockOrgToolRepository) Update(ctx context.Context, tool *models.OrganizationTool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}

func (m *mockOrgToolRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockOrgToolRepository) GetByInstanceName(ctx context.Context, orgID, name string) (*models.OrganizationTool, error) {
	args := m.Called(ctx, orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OrganizationTool), args.Error(1)
}

func (m *mockOrgToolRepository) UpdateHealthStatus(ctx context.Context, id string, status json.RawMessage, message string) error {
	args := m.Called(ctx, id, status, message)
	return args.Error(0)
}

func (m *mockOrgToolRepository) ListByTenant(ctx context.Context, tenantID string) ([]*models.OrganizationTool, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.OrganizationTool), args.Error(1)
}

func (m *mockOrgToolRepository) UpdateStatus(ctx context.Context, id, status string) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockOrgToolRepository) UpdateHealth(ctx context.Context, id string, healthStatus json.RawMessage, healthMessage string) error {
	args := m.Called(ctx, id, healthStatus, healthMessage)
	return args.Error(0)
}

// mockTemplateRepository mocks the ToolTemplateRepository
type mockTemplateRepository struct {
	mock.Mock
}

func (m *mockTemplateRepository) Create(ctx context.Context, template *models.ToolTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *mockTemplateRepository) GetByID(ctx context.Context, id string) (*models.ToolTemplate, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ToolTemplate), args.Error(1)
}

func (m *mockTemplateRepository) GetByProviderName(ctx context.Context, provider string) (*models.ToolTemplate, error) {
	args := m.Called(ctx, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ToolTemplate), args.Error(1)
}

func (m *mockTemplateRepository) List(ctx context.Context) ([]*models.ToolTemplate, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.ToolTemplate), args.Error(1)
}

func (m *mockTemplateRepository) ListByCategory(ctx context.Context, category string) ([]*models.ToolTemplate, error) {
	args := m.Called(ctx, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.ToolTemplate), args.Error(1)
}

func (m *mockTemplateRepository) Upsert(ctx context.Context, template *models.ToolTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *mockTemplateRepository) Update(ctx context.Context, template *models.ToolTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *mockTemplateRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// TestOrganizationToolAdapter tests the OrganizationToolAdapter
func TestOrganizationToolAdapter_GetOrganizationTools(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	auditLogger := auth.NewAuditLogger(logger)

	// Enable features
	feature.SetEnabled(feature.EnableStandardTools, true)
	feature.SetEnabled(feature.EnablePermissionCaching, true)

	// Create mocks
	orgToolRepo := new(mockOrgToolRepository)
	templateRepo := new(mockTemplateRepository)

	// Create provider registry
	providerRegistry := services.NewProviderRegistry(logger)
	githubProvider := github.NewGitHubProvider(logger)
	providerRegistry.RegisterProvider("github", githubProvider)

	// Create permission cache
	memCache := cache.NewMemoryCache(100, 5*time.Minute)
	permissionCache := services.NewPermissionCacheService(memCache, nil, logger)

	// Create adapter
	config := DefaultOrganizationToolAdapterConfig()
	adapter := NewOrganizationToolAdapter(
		orgToolRepo,
		templateRepo,
		providerRegistry,
		permissionCache,
		auditLogger,
		logger,
		metrics,
		config,
	)

	// Test data
	orgID := "org-123"
	templateID := "template-github"

	orgTools := []*models.OrganizationTool{
		{
			ID:             "tool-1",
			OrganizationID: orgID,
			TenantID:       "tenant-1",
			TemplateID:     templateID,
			InstanceName:   "github-main",
			DisplayName:    "GitHub Main",
			Status:         "active",
			IsActive:       true,
		},
	}

	template := &models.ToolTemplate{
		ID:           templateID,
		ProviderName: "github",
		DisplayName:  "GitHub",
		Description:  "GitHub integration",
		Category:     "version_control",
	}

	// Setup mocks
	orgToolRepo.On("ListByOrganization", mock.Anything, orgID).Return(orgTools, nil)
	templateRepo.On("GetByID", mock.Anything, templateID).Return(template, nil)

	// Test without token
	tools, err := adapter.GetOrganizationTools(ctx, orgID, "")
	assert.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "tool-1", tools[0].ID)

	// Test with token
	tools, err = adapter.GetOrganizationTools(ctx, orgID, "test-token")
	assert.NoError(t, err)
	assert.Len(t, tools, 1)

	// Verify mocks
	orgToolRepo.AssertExpectations(t)
	templateRepo.AssertExpectations(t)
}

func TestOrganizationToolAdapter_ExecuteOperation(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	auditLogger := auth.NewAuditLogger(logger)

	// Create mocks
	orgToolRepo := new(mockOrgToolRepository)
	templateRepo := new(mockTemplateRepository)

	// Create provider registry
	providerRegistry := services.NewProviderRegistry(logger)

	// Create permission cache
	memCache := cache.NewMemoryCache(100, 5*time.Minute)
	permissionCache := services.NewPermissionCacheService(memCache, nil, logger)

	// Create adapter
	config := DefaultOrganizationToolAdapterConfig()
	adapter := NewOrganizationToolAdapter(
		orgToolRepo,
		templateRepo,
		providerRegistry,
		permissionCache,
		auditLogger,
		logger,
		metrics,
		config,
	)

	// Test data
	orgID := "org-123"
	toolID := "tool-1"
	templateID := "template-github"

	orgTool := &models.OrganizationTool{
		ID:             toolID,
		OrganizationID: orgID,
		TenantID:       "tenant-1",
		TemplateID:     templateID,
		InstanceName:   "github-main",
		Status:         "active",
		IsActive:       true,
	}

	template := &models.ToolTemplate{
		ID:           templateID,
		ProviderName: "github",
		DisplayName:  "GitHub",
		Description:  "GitHub integration",
	}

	// Setup mocks
	orgToolRepo.On("GetByID", mock.Anything, toolID).Return(orgTool, nil)
	templateRepo.On("GetByID", mock.Anything, templateID).Return(template, nil)

	// Execute operation (will fail without provider, but tests the flow)
	params := map[string]interface{}{
		"org": "test-org",
	}

	_, err := adapter.ExecuteOperation(ctx, orgID, toolID, "repos/list", params, "test-token")
	// We expect an error since provider is not registered
	assert.Error(t, err)

	// Verify mocks
	orgToolRepo.AssertExpectations(t)
	templateRepo.AssertExpectations(t)
}

func TestOrganizationToolAdapter_ExpandToMCPTools(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	auditLogger := auth.NewAuditLogger(logger)

	// Create mocks
	orgToolRepo := new(mockOrgToolRepository)
	templateRepo := new(mockTemplateRepository)

	// Create provider registry and register GitHub
	providerRegistry := services.NewProviderRegistry(logger)
	githubProvider := github.NewGitHubProvider(logger)
	providerRegistry.RegisterProvider("github", githubProvider)

	// Create permission cache
	memCache := cache.NewMemoryCache(100, 5*time.Minute)
	permissionCache := services.NewPermissionCacheService(memCache, nil, logger)

	// Create adapter
	config := DefaultOrganizationToolAdapterConfig()
	adapter := NewOrganizationToolAdapter(
		orgToolRepo,
		templateRepo,
		providerRegistry,
		permissionCache,
		auditLogger,
		logger,
		metrics,
		config,
	)

	// Test data
	orgID := "org-123"
	templateID := "template-github"

	orgTools := []*models.OrganizationTool{
		{
			ID:             "tool-1",
			OrganizationID: orgID,
			TenantID:       "tenant-1",
			TemplateID:     templateID,
			InstanceName:   "github-main",
			DisplayName:    "GitHub Main",
			Status:         "active",
			IsActive:       true,
		},
	}

	template := &models.ToolTemplate{
		ID:           templateID,
		ProviderName: "github",
		DisplayName:  "GitHub",
		Description:  "GitHub integration",
	}

	// Setup mocks
	templateRepo.On("GetByID", mock.Anything, templateID).Return(template, nil)

	// Expand to MCP tools
	mcpTools, err := adapter.ExpandToMCPTools(ctx, orgTools)
	assert.NoError(t, err)
	assert.NotEmpty(t, mcpTools)

	// Verify tool structure
	for _, tool := range mcpTools {
		assert.Contains(t, tool.Name, "github_")
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
		assert.NotNil(t, tool.Metadata)
		assert.Equal(t, orgID, tool.Metadata["organization_id"])
		assert.Equal(t, "tool-1", tool.Metadata["tool_id"])
	}

	// Verify mocks
	templateRepo.AssertExpectations(t)
}

func TestOrganizationToolAdapter_CircuitBreaker(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	auditLogger := auth.NewAuditLogger(logger)

	// Create mocks
	orgToolRepo := new(mockOrgToolRepository)
	templateRepo := new(mockTemplateRepository)

	// Create provider registry
	providerRegistry := services.NewProviderRegistry(logger)

	// Create permission cache
	memCache := cache.NewMemoryCache(100, 5*time.Minute)
	permissionCache := services.NewPermissionCacheService(memCache, nil, logger)

	// Create adapter with low circuit breaker threshold
	config := DefaultOrganizationToolAdapterConfig()
	config.CircuitBreakerMaxRequests = 2
	config.CircuitBreakerRatio = 0.5

	adapter := NewOrganizationToolAdapter(
		orgToolRepo,
		templateRepo,
		providerRegistry,
		permissionCache,
		auditLogger,
		logger,
		metrics,
		config,
	)

	// Test data
	orgID := "org-123"
	toolID := "tool-1"
	templateID := "template-github"

	orgTool := &models.OrganizationTool{
		ID:             toolID,
		OrganizationID: orgID,
		TenantID:       "tenant-1",
		TemplateID:     templateID,
		InstanceName:   "github-main",
		Status:         "active",
		IsActive:       true,
	}

	template := &models.ToolTemplate{
		ID:           templateID,
		ProviderName: "github",
		DisplayName:  "GitHub",
		Description:  "GitHub integration",
	}

	// Setup mocks
	orgToolRepo.On("GetByID", mock.Anything, toolID).Return(orgTool, nil)
	templateRepo.On("GetByID", mock.Anything, templateID).Return(template, nil)

	// Execute multiple failing operations
	params := map[string]interface{}{"test": "params"}

	for i := 0; i < 5; i++ {
		_, _ = adapter.ExecuteOperation(ctx, orgID, toolID, "test-op", params, "test-token")
	}

	// Check health status
	health := adapter.GetHealthStatus(ctx)
	assert.NotNil(t, health)

	// Should have a circuit breaker for github
	if githubHealth, ok := health["github"]; ok {
		assert.NotEmpty(t, githubHealth.CircuitState)
	}
}

func TestOrganizationToolAdapter_Bulkhead(t *testing.T) {
	// Setup
	_ = context.Background()
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()
	auditLogger := auth.NewAuditLogger(logger)

	// Create mocks
	orgToolRepo := new(mockOrgToolRepository)
	templateRepo := new(mockTemplateRepository)

	// Create provider registry
	providerRegistry := services.NewProviderRegistry(logger)

	// Create permission cache
	memCache := cache.NewMemoryCache(100, 5*time.Minute)
	permissionCache := services.NewPermissionCacheService(memCache, nil, logger)

	// Create adapter with small bulkhead
	config := DefaultOrganizationToolAdapterConfig()
	config.MaxConcurrentRequests = 2
	config.QueueSize = 1

	adapter := NewOrganizationToolAdapter(
		orgToolRepo,
		templateRepo,
		providerRegistry,
		permissionCache,
		auditLogger,
		logger,
		metrics,
		config,
	)

	// Test bulkhead stats
	stats := adapter.bulkhead.Stats()
	assert.Equal(t, 2, stats.MaxConcurrent)
	assert.Equal(t, 1, stats.MaxQueueSize)
	assert.Equal(t, 0, stats.ActiveRequests)
	assert.Equal(t, 0, stats.QueuedRequests)
	assert.False(t, stats.Closed)
}

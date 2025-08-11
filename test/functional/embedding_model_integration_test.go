//go:build integration
// +build integration

package functional

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/embedding_usage"
	"github.com/developer-mesh/developer-mesh/pkg/repository/model_catalog"
	"github.com/developer-mesh/developer-mesh/pkg/repository/tenant_models"
)

// EmbeddingModelIntegrationSuite tests the complete embedding model management flow
type EmbeddingModelIntegrationSuite struct {
	suite.Suite
	ctx              context.Context
	db               *sqlx.DB
	redisClient      *redis.Client
	cache            cache.Cache
	logger           observability.Logger
	metrics          observability.MetricsClient
	modelService     services.ModelManagementService
	catalogRepo      model_catalog.ModelCatalogRepository
	tenantModelsRepo tenant_models.TenantModelsRepository
	usageRepo        embedding_usage.EmbeddingUsageRepository
	testTenantID     uuid.UUID
	testModelID      uuid.UUID
}

// SetupSuite runs once before all tests
func (s *EmbeddingModelIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.logger = observability.NewStandardLogger("integration_test")
	s.metrics = observability.NewNoOpMetricsClient()
	s.testTenantID = uuid.New()
	s.testModelID = uuid.New()

	// Setup database connection
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://devmesh:devmesh@localhost:5432/devmesh_test?sslmode=disable"
	}

	db, err := database.Connect(dbURL)
	s.Require().NoError(err)
	s.db = db.GetDB()

	// Setup Redis connection
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	s.redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   1, // Use DB 1 for tests
	})

	// Verify Redis connection
	err = s.redisClient.Ping(s.ctx).Err()
	s.Require().NoError(err)

	// Create cache
	s.cache = cache.NewRedisCache(s.redisClient, 5*time.Minute)

	// Create repositories
	s.catalogRepo = model_catalog.NewModelCatalogRepository(s.db)
	s.tenantModelsRepo = tenant_models.NewTenantModelsRepository(s.db)
	s.usageRepo = embedding_usage.NewEmbeddingUsageRepository(s.db)

	// Create model management service
	s.modelService = services.NewModelManagementService(
		s.catalogRepo,
		s.tenantModelsRepo,
		s.usageRepo,
		s.cache,
		s.logger,
		s.metrics,
	)

	// Run migrations
	s.runMigrations()
}

// TearDownSuite runs once after all tests
func (s *EmbeddingModelIntegrationSuite) TearDownSuite() {
	// Clean up test data
	s.cleanupTestData()

	// Close connections
	if s.redisClient != nil {
		if err := s.redisClient.Close(); err != nil {
			s.T().Logf("Failed to close Redis client: %v", err)
		}
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.T().Logf("Failed to close database: %v", err)
		}
	}
}

// SetupTest runs before each test
func (s *EmbeddingModelIntegrationSuite) SetupTest() {
	// Clear Redis cache
	s.redisClient.FlushDB(s.ctx)

	// Insert test data
	s.insertTestData()
}

// TearDownTest runs after each test
func (s *EmbeddingModelIntegrationSuite) TearDownTest() {
	// Clean up test-specific data
	s.cleanupTestData()
}

// TestModelCatalogOperations tests CRUD operations on model catalog
func (s *EmbeddingModelIntegrationSuite) TestModelCatalogOperations() {
	// Create a new model
	model := &model_catalog.EmbeddingModel{
		ID:           uuid.New(),
		Provider:     "openai",
		ModelName:    "Test Model",
		ModelID:      "test-model-001",
		Dimensions:   1536,
		MaxTokens:    8192,
		IsAvailable:  true,
		IsDeprecated: false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Test Create
	err := s.catalogRepo.Create(s.ctx, model)
	s.NoError(err)

	// Test GetByID
	retrieved, err := s.catalogRepo.GetByID(s.ctx, model.ID)
	s.NoError(err)
	s.NotNil(retrieved)
	s.Equal(model.ModelID, retrieved.ModelID)

	// Test Update
	model.ModelName = "Updated Test Model"
	err = s.catalogRepo.Update(s.ctx, model)
	s.NoError(err)

	// Test ListAvailable
	filter := &model_catalog.ModelFilter{
		Provider: &model.Provider,
	}
	models, err := s.catalogRepo.ListAvailable(s.ctx, filter)
	s.NoError(err)
	s.NotEmpty(models)

	// Test Delete
	err = s.catalogRepo.Delete(s.ctx, model.ID)
	s.NoError(err)
}

// TestTenantModelConfiguration tests tenant-specific model configuration
func (s *EmbeddingModelIntegrationSuite) TestTenantModelConfiguration() {
	// Configure model for tenant
	config := &services.ConfigureTenantModelRequest{
		ModelID:      s.testModelID,
		Enabled:      true,
		IsDefault:    true,
		MonthlyQuota: 1000000,
		DailyQuota:   50000,
		RateLimitRPM: 100,
		Priority:     1,
		CustomSettings: map[string]interface{}{
			"temperature": 0.7,
		},
	}

	result, err := s.modelService.ConfigureTenantModel(s.ctx, s.testTenantID, config)
	s.NoError(err)
	s.NotNil(result)
	s.Equal(s.testModelID, result.ModelID)

	// Test model selection
	selectionReq := &services.ModelSelectionRequest{
		TenantID:        s.testTenantID,
		RequestedModel:  "",
		TaskType:        "general",
		EstimatedTokens: 1000,
	}

	selected, err := s.modelService.SelectModelForRequest(s.ctx, selectionReq)
	s.NoError(err)
	s.NotNil(selected)
	s.Equal(s.testModelID, selected.ModelID)

	// Test quota checking
	quotas, err := s.modelService.GetTenantQuotas(s.ctx, s.testTenantID)
	s.NoError(err)
	s.NotNil(quotas)
}

// TestUsageTracking tests usage tracking and quota enforcement
func (s *EmbeddingModelIntegrationSuite) TestUsageTracking() {
	// Track usage
	usage := &embedding_usage.UsageRecord{
		ID:              uuid.New(),
		TenantID:        s.testTenantID,
		ModelID:         s.testModelID,
		AgentID:         uuid.New(),
		RequestID:       uuid.New().String(),
		TokensUsed:      1500,
		CostUSD:         0.002,
		Provider:        "openai",
		ModelName:       "text-embedding-3-small",
		TaskType:        "search",
		RequestDuration: 150,
		Success:         true,
		CreatedAt:       time.Now(),
	}

	err := s.usageRepo.RecordUsage(s.ctx, usage)
	s.NoError(err)

	// Get usage stats
	stats, err := s.modelService.GetUsageStats(s.ctx, s.testTenantID, &s.testModelID, "day")
	s.NoError(err)
	s.NotNil(stats)
	s.Equal(int64(1500), stats.TotalTokens)
	s.Equal(0.002, stats.TotalCost)
}

// TestModelFailover tests automatic model failover
func (s *EmbeddingModelIntegrationSuite) TestModelFailover() {
	// Configure multiple models for tenant
	primaryModel := s.testModelID
	fallbackModel := uuid.New()

	// Insert fallback model
	fallback := &model_catalog.EmbeddingModel{
		ID:           fallbackModel,
		Provider:     "bedrock",
		ModelName:    "Fallback Model",
		ModelID:      "fallback-model",
		Dimensions:   1024,
		IsAvailable:  true,
		IsDeprecated: false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err := s.catalogRepo.Create(s.ctx, fallback)
	s.NoError(err)

	// Configure both models for tenant
	primaryConfig := &services.ConfigureTenantModelRequest{
		ModelID:      primaryModel,
		Enabled:      true,
		IsDefault:    true,
		Priority:     1,
		MonthlyQuota: 1000,
	}
	_, err = s.modelService.ConfigureTenantModel(s.ctx, s.testTenantID, primaryConfig)
	s.NoError(err)

	fallbackConfig := &services.ConfigureTenantModelRequest{
		ModelID:      fallbackModel,
		Enabled:      true,
		IsDefault:    false,
		Priority:     2,
		MonthlyQuota: 10000,
	}
	_, err = s.modelService.ConfigureTenantModel(s.ctx, s.testTenantID, fallbackConfig)
	s.NoError(err)

	// Simulate primary model quota exhaustion
	for i := 0; i < 10; i++ {
		usage := &embedding_usage.UsageRecord{
			ID:         uuid.New(),
			TenantID:   s.testTenantID,
			ModelID:    primaryModel,
			TokensUsed: 100,
			Success:    true,
			CreatedAt:  time.Now(),
		}
		s.usageRepo.RecordUsage(s.ctx, usage)
	}

	// Request should now select fallback model
	selectionReq := &services.ModelSelectionRequest{
		TenantID:        s.testTenantID,
		EstimatedTokens: 100,
	}

	selected, err := s.modelService.SelectModelForRequest(s.ctx, selectionReq)
	s.NoError(err)
	s.NotNil(selected)
	// Should select fallback when primary quota is exhausted
	s.Equal(fallbackModel, selected.ModelID)
}

// TestConcurrentRequests tests concurrent model selection and usage tracking
func (s *EmbeddingModelIntegrationSuite) TestConcurrentRequests() {
	concurrency := 10
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			// Perform model selection
			selectionReq := &services.ModelSelectionRequest{
				TenantID:        s.testTenantID,
				EstimatedTokens: 100,
			}

			selected, err := s.modelService.SelectModelForRequest(s.ctx, selectionReq)
			if err != nil {
				errors <- err
				return
			}

			// Track usage
			usage := &embedding_usage.UsageRecord{
				ID:         uuid.New(),
				TenantID:   s.testTenantID,
				ModelID:    selected.ModelID,
				TokensUsed: 100,
				Success:    true,
				CreatedAt:  time.Now(),
			}

			err = s.usageRepo.RecordUsage(s.ctx, usage)
			errors <- err
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		err := <-errors
		s.NoError(err)
	}

	// Verify usage was tracked correctly
	stats, err := s.modelService.GetUsageStats(s.ctx, s.testTenantID, nil, "day")
	s.NoError(err)
	s.Equal(int64(concurrency*100), stats.TotalTokens)
}

// Helper methods

func (s *EmbeddingModelIntegrationSuite) runMigrations() {
	// Run migration for embedding tables
	migrations := []string{
		`CREATE SCHEMA IF NOT EXISTS mcp`,
		`CREATE TABLE IF NOT EXISTS mcp.embedding_model_catalog (
			id UUID PRIMARY KEY,
			provider VARCHAR(50) NOT NULL,
			model_name VARCHAR(100) NOT NULL,
			model_id VARCHAR(100) UNIQUE NOT NULL,
			model_version VARCHAR(50),
			dimensions INT NOT NULL,
			max_tokens INT,
			supports_binary BOOLEAN DEFAULT FALSE,
			is_available BOOLEAN DEFAULT TRUE,
			is_deprecated BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS mcp.tenant_embedding_models (
			id UUID PRIMARY KEY,
			tenant_id UUID NOT NULL,
			model_id UUID NOT NULL REFERENCES mcp.embedding_model_catalog(id),
			enabled BOOLEAN DEFAULT TRUE,
			is_default BOOLEAN DEFAULT FALSE,
			monthly_quota_tokens BIGINT,
			daily_quota_tokens BIGINT,
			rate_limit_rpm INT,
			priority INT DEFAULT 100,
			custom_settings JSONB,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(tenant_id, model_id)
		)`,
		`CREATE TABLE IF NOT EXISTS mcp.embedding_usage_tracking (
			id UUID PRIMARY KEY,
			tenant_id UUID NOT NULL,
			model_id UUID NOT NULL,
			agent_id UUID,
			request_id VARCHAR(100),
			tokens_used INT NOT NULL,
			cost_usd DECIMAL(10,6),
			provider VARCHAR(50),
			model_name VARCHAR(100),
			task_type VARCHAR(50),
			request_duration_ms INT,
			success BOOLEAN DEFAULT TRUE,
			error_message TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
	}

	for _, migration := range migrations {
		_, err := s.db.Exec(migration)
		if err != nil {
			s.T().Logf("Migration error (may be ok if already exists): %v", err)
		}
	}
}

func (s *EmbeddingModelIntegrationSuite) insertTestData() {
	// Insert test model
	model := &model_catalog.EmbeddingModel{
		ID:           s.testModelID,
		Provider:     "openai",
		ModelName:    "Test Embedding Model",
		ModelID:      "test-embedding-model",
		Dimensions:   1536,
		MaxTokens:    8192,
		IsAvailable:  true,
		IsDeprecated: false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := s.catalogRepo.Create(s.ctx, model)
	if err != nil {
		s.T().Logf("Failed to insert test model: %v", err)
	}
}

func (s *EmbeddingModelIntegrationSuite) cleanupTestData() {
	// Clean up in reverse order of foreign key dependencies
	queries := []string{
		`DELETE FROM mcp.embedding_usage_tracking WHERE tenant_id = $1`,
		`DELETE FROM mcp.tenant_embedding_models WHERE tenant_id = $1`,
		`DELETE FROM mcp.embedding_model_catalog WHERE model_id LIKE 'test-%' OR model_id LIKE 'fallback-%'`,
	}

	for _, query := range queries {
		_, err := s.db.Exec(query, s.testTenantID)
		if err != nil {
			s.T().Logf("Cleanup error: %v", err)
		}
	}
}

// TestEmbeddingModelIntegrationSuite runs the test suite
func TestEmbeddingModelIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(EmbeddingModelIntegrationSuite))
}

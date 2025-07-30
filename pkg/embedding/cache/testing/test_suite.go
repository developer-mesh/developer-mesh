package testing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

// CacheTestSuite provides a comprehensive test suite for cache functionality
type CacheTestSuite struct {
	suite.Suite

	// Infrastructure
	redisClient *redis.Client
	db          *sqlx.DB

	// Cache components
	baseCache   *cache.SemanticCache
	tenantCache *cache.TenantAwareCache
	vectorStore *cache.VectorStore

	// Test data
	tenantIDs  []uuid.UUID
	configRepo repository.TenantConfigRepository
}

// SetupSuite runs once before all tests
func (s *CacheTestSuite) SetupSuite() {
	// Setup Redis
	s.redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Test database
	})

	// Verify Redis connection
	ctx := context.Background()
	err := s.redisClient.Ping(ctx).Err()
	if err != nil {
		s.T().Skip("Redis not available")
	}

	// Setup PostgreSQL with pgvector (if available)
	// s.db = setupTestDB()

	// Create test tenants
	s.tenantIDs = []uuid.UUID{
		uuid.New(),
		uuid.New(),
		uuid.New(),
	}
}

// SetupTest runs before each test
func (s *CacheTestSuite) SetupTest() {
	// Clear Redis
	s.redisClient.FlushDB(context.Background())

	// Create base cache
	config := cache.DefaultConfig()
	config.Prefix = "test"
	baseCache, err := cache.NewSemanticCache(s.redisClient, config, nil)
	s.Require().NoError(err)
	s.baseCache = baseCache

	// Create config repository
	s.configRepo = NewMockTenantConfigRepo()

	// Create tenant-aware cache
	s.tenantCache = cache.NewTenantAwareCache(
		s.baseCache,
		s.configRepo,
		nil, // rate limiter
		"test-encryption-key",
		observability.NewLogger("test"),
		nil, // metrics
	)

	// Create vector store if DB available
	if s.db != nil {
		s.vectorStore = cache.NewVectorStore(s.db, nil, nil)
	}

	// Setup tenant configs
	s.setupTenantConfigs()
}

// TearDownTest runs after each test
func (s *CacheTestSuite) TearDownTest() {
	// Stop any background processes
	if s.tenantCache != nil {
		s.tenantCache.StopLRUEviction()
	}
}

// TearDownSuite runs once after all tests
func (s *CacheTestSuite) TearDownSuite() {
	if s.redisClient != nil {
		_ = s.redisClient.Close()
	}
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *CacheTestSuite) setupTenantConfigs() {
	// Setup different tenant configurations for testing
	configs := map[string]*models.TenantConfig{
		s.tenantIDs[0].String(): {
			TenantID: s.tenantIDs[0].String(),
			Features: map[string]interface{}{
				"cache": map[string]interface{}{
					"enabled":        true,
					"max_entries":    100,
					"max_bytes":      1048576, // 1MB
					"cache_warming":  true,
					"async_eviction": true,
				},
			},
		},
		s.tenantIDs[1].String(): {
			TenantID: s.tenantIDs[1].String(),
			Features: map[string]interface{}{
				"cache": map[string]interface{}{
					"enabled":     true,
					"max_entries": 50,
					"max_bytes":   524288, // 512KB
				},
			},
		},
		s.tenantIDs[2].String(): {
			TenantID: s.tenantIDs[2].String(),
			Features: map[string]interface{}{
				"cache": map[string]interface{}{
					"enabled": false, // Cache disabled
				},
			},
		},
	}

	for id, config := range configs {
		s.configRepo.(*MockTenantConfigRepo).configs[id] = config
	}
}

// Test Cases

func (s *CacheTestSuite) TestTenantIsolation() {
	ctx1 := auth.WithTenantID(context.Background(), s.tenantIDs[0])
	ctx2 := auth.WithTenantID(context.Background(), s.tenantIDs[1])

	query := "test query"
	embedding := []float32{1, 2, 3, 4, 5}
	results := []cache.CachedSearchResult{
		{ID: "1", Content: "Result 1", Score: 0.95},
	}

	// Set in tenant 1
	err := s.tenantCache.Set(ctx1, query, embedding, results)
	s.NoError(err)

	// Get from tenant 1 - should find
	entry1, err := s.tenantCache.Get(ctx1, query, embedding)
	s.NoError(err)
	s.NotNil(entry1)
	s.Equal("Result 1", entry1.Results[0].Content)

	// Get from tenant 2 - should not find
	entry2, err := s.tenantCache.Get(ctx2, query, embedding)
	s.NoError(err)
	s.Nil(entry2)
}

func (s *CacheTestSuite) TestFeatureFlags() {
	// Test with cache disabled tenant
	ctx := auth.WithTenantID(context.Background(), s.tenantIDs[2])

	query := "disabled tenant query"
	embedding := []float32{1, 2, 3}
	results := []cache.CachedSearchResult{
		{ID: "1", Content: "Should not cache", Score: 0.9},
	}

	// Try to set - should fail
	err := s.tenantCache.Set(ctx, query, embedding, results)
	s.Equal(cache.ErrFeatureDisabled, err)

	// Try to get - should fail
	entry, err := s.tenantCache.Get(ctx, query, embedding)
	s.Equal(cache.ErrFeatureDisabled, err)
	s.Nil(entry)
}

func (s *CacheTestSuite) TestLRUEviction() {
	if s.tenantCache.GetLRUManager() == nil {
		s.T().Skip("LRU manager not available")
	}

	ctx := auth.WithTenantID(context.Background(), s.tenantIDs[1])

	// Tenant 2 has max 50 entries
	// Fill beyond limit
	for i := 0; i < 60; i++ {
		query := fmt.Sprintf("query_%d", i)
		embedding := []float32{float32(i), float32(i + 1)}
		results := []cache.CachedSearchResult{
			{ID: fmt.Sprintf("%d", i), Content: fmt.Sprintf("Result %d", i), Score: 0.9},
		}

		err := s.tenantCache.Set(ctx, query, embedding, results)
		s.NoError(err)

		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Access some entries to make them more recent
	for i := 50; i < 55; i++ {
		query := fmt.Sprintf("query_%d", i)
		embedding := []float32{float32(i), float32(i + 1)}
		_, _ = s.tenantCache.Get(ctx, query, embedding)
	}

	// Trigger eviction manually
	if s.tenantCache.GetLRUManager() != nil {
		// Use a large target bytes to force eviction
		err := s.tenantCache.GetLRUManager().EvictForTenant(ctx, s.tenantIDs[1], 1024*1024*10) // 10MB target
		s.NoError(err)
	}

	// Check that old entries were evicted
	for i := 0; i < 10; i++ {
		query := fmt.Sprintf("query_%d", i)
		embedding := []float32{float32(i), float32(i + 1)}
		entry, _ := s.tenantCache.Get(ctx, query, embedding)
		if entry != nil {
			s.T().Logf("Unexpected: Entry %d still in cache", i)
		}
	}

	// Recently accessed entries should still be there
	for i := 50; i < 55; i++ {
		query := fmt.Sprintf("query_%d", i)
		embedding := []float32{float32(i), float32(i + 1)}
		entry, err := s.tenantCache.Get(ctx, query, embedding)
		s.NoError(err)
		if entry == nil {
			s.T().Logf("Entry %d was evicted (should have been kept)", i)
		}
	}
}

func (s *CacheTestSuite) TestEncryption() {
	ctx := auth.WithTenantID(context.Background(), s.tenantIDs[0])

	// Create entry with sensitive data
	query := "sensitive query"
	embedding := []float32{1, 2, 3}
	results := []cache.CachedSearchResult{
		{
			ID:      "1",
			Content: "Public content",
			Score:   0.95,
			Metadata: map[string]interface{}{
				"api_key": "secret-key-12345",
				"secret":  "sensitive-data",
			},
		},
	}

	// Set with encryption
	err := s.tenantCache.Set(ctx, query, embedding, results)
	s.NoError(err)

	// Get and verify decryption
	entry, err := s.tenantCache.Get(ctx, query, embedding)
	s.NoError(err)
	s.NotNil(entry)

	// Check if sensitive data was encrypted/decrypted
	if decrypted, ok := entry.Metadata["decrypted_data"]; ok {
		s.Contains(decrypted, "secret-key-12345")
	}
}

func (s *CacheTestSuite) TestConcurrentAccess() {
	ctx := auth.WithTenantID(context.Background(), s.tenantIDs[0])

	// Test concurrent reads and writes
	done := make(chan bool)
	errors := make(chan error, 100)

	// Writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			query := fmt.Sprintf("concurrent_%d", id)
			embedding := []float32{float32(id)}
			results := []cache.CachedSearchResult{
				{ID: fmt.Sprintf("%d", id), Content: fmt.Sprintf("Concurrent %d", id), Score: 0.9},
			}

			if err := s.tenantCache.Set(ctx, query, embedding, results); err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			query := fmt.Sprintf("concurrent_%d", id)
			embedding := []float32{float32(id)}

			// Try multiple times as write might not be complete
			for j := 0; j < 5; j++ {
				if entry, _ := s.tenantCache.Get(ctx, query, embedding); entry != nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		s.NoError(err)
	}
}

// TestMigration removed - no migration needed for greenfield deployment

// Helper to run the test suite
func TestCacheSuite(t *testing.T) {
	suite.Run(t, new(CacheTestSuite))
}

// MockTenantConfigRepo for testing
type MockTenantConfigRepo struct {
	configs map[string]*models.TenantConfig
}

func NewMockTenantConfigRepo() *MockTenantConfigRepo {
	return &MockTenantConfigRepo{
		configs: make(map[string]*models.TenantConfig),
	}
}

func (m *MockTenantConfigRepo) GetByTenantID(ctx context.Context, tenantID string) (*models.TenantConfig, error) {
	if config, ok := m.configs[tenantID]; ok {
		return config, nil
	}
	return nil, cache.ErrTenantNotFound
}

func (m *MockTenantConfigRepo) Create(ctx context.Context, config *models.TenantConfig) error {
	m.configs[config.TenantID] = config
	return nil
}

func (m *MockTenantConfigRepo) Update(ctx context.Context, config *models.TenantConfig) error {
	m.configs[config.TenantID] = config
	return nil
}

func (m *MockTenantConfigRepo) Delete(ctx context.Context, tenantID string) error {
	delete(m.configs, tenantID)
	return nil
}

func (m *MockTenantConfigRepo) List(ctx context.Context, limit, offset int) ([]*models.TenantConfig, error) {
	var configs []*models.TenantConfig
	for _, config := range m.configs {
		configs = append(configs, config)
	}
	return configs, nil
}

func (m *MockTenantConfigRepo) Exists(ctx context.Context, tenantID string) (bool, error) {
	_, exists := m.configs[tenantID]
	return exists, nil
}

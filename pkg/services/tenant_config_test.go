package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	_ "github.com/S-Corkum/devops-mcp/pkg/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations
type mockTenantConfigRepository struct {
	mock.Mock
}

func (m *mockTenantConfigRepository) GetByTenantID(ctx context.Context, tenantID string) (*models.TenantConfig, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TenantConfig), args.Error(1)
}

func (m *mockTenantConfigRepository) Create(ctx context.Context, config *models.TenantConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockTenantConfigRepository) Update(ctx context.Context, config *models.TenantConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockTenantConfigRepository) Delete(ctx context.Context, tenantID string) error {
	args := m.Called(ctx, tenantID)
	return args.Error(0)
}

func (m *mockTenantConfigRepository) Exists(ctx context.Context, tenantID string) (bool, error) {
	args := m.Called(ctx, tenantID)
	return args.Bool(0), args.Error(1)
}

type mockCache struct {
	mock.Mock
}

func (m *mockCache) Get(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *mockCache) Flush(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockCache) Close() error {
	args := m.Called()
	return args.Error(0)
}

type mockEncryptionService struct {
	mock.Mock
}

func (m *mockEncryptionService) Encrypt(ctx context.Context, data []byte) ([]byte, error) {
	args := m.Called(ctx, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockEncryptionService) Decrypt(ctx context.Context, data []byte) ([]byte, error) {
	args := m.Called(ctx, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockEncryptionService) EncryptString(ctx context.Context, data string) (string, error) {
	args := m.Called(ctx, data)
	return args.String(0), args.Error(1)
}

func (m *mockEncryptionService) DecryptString(ctx context.Context, data string) (string, error) {
	args := m.Called(ctx, data)
	return args.String(0), args.Error(1)
}

func TestNewTenantConfigService(t *testing.T) {
	repo := &mockTenantConfigRepository{}
	cache := &mockCache{}
	encryption := &mockEncryptionService{}
	logger := observability.NewLogger("test")

	service := NewTenantConfigService(repo, cache, encryption, logger)
	assert.NotNil(t, service)
	assert.IsType(t, &tenantConfigService{}, service)
}

func TestTenantConfigService_GetConfig(t *testing.T) {
	ctx := context.Background()
	tenantID := "test-tenant-123"
	cacheKey := "tenant:config:" + tenantID

	testConfig := &models.TenantConfig{
		ID:       uuid.New().String(),
		TenantID: tenantID,
		RateLimitConfig: models.RateLimitConfig{
			DefaultRequestsPerMinute: 100,
		},
		EncryptedTokens: json.RawMessage(`{"github": "encrypted_token"}`),
		FeaturesJSON:    json.RawMessage(`{"feature1": true}`),
		AllowedOrigins:  pq.StringArray{"https://app.example.com"},
	}

	t.Run("get from cache", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := NewTenantConfigService(repo, cache, encryption, logger)

		// Setup cache hit
		cache.On("Get", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig")).
			Run(func(args mock.Arguments) {
				config := args.Get(2).(*models.TenantConfig)
				*config = *testConfig
			}).
			Return(nil)

		config, err := service.GetConfig(ctx, tenantID)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, tenantID, config.TenantID)

		cache.AssertExpectations(t)
		repo.AssertNotCalled(t, "GetByTenantID")
	})

	t.Run("get from repository with decryption", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := NewTenantConfigService(repo, cache, encryption, logger)

		// Setup cache miss
		cache.On("Get", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig")).
			Return(errors.New("cache miss"))

		// Setup repository response
		repo.On("GetByTenantID", ctx, tenantID).Return(testConfig, nil)

		// Setup decryption
		decryptedTokens := []byte(`{"github": "ghp_decrypted_token"}`)
		encryption.On("Decrypt", ctx, testConfig.EncryptedTokens).
			Return(decryptedTokens, nil)

		// Setup cache set
		cache.On("Set", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig"), 5*time.Minute).
			Return(nil)

		config, err := service.GetConfig(ctx, tenantID)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, tenantID, config.TenantID)
		assert.Equal(t, "ghp_decrypted_token", config.ServiceTokens["github"])
		assert.Equal(t, true, config.Features["feature1"])

		cache.AssertExpectations(t)
		repo.AssertExpectations(t)
		encryption.AssertExpectations(t)
	})

	t.Run("not found returns default config", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := NewTenantConfigService(repo, cache, encryption, logger)

		// Setup cache miss
		cache.On("Get", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig")).
			Return(errors.New("cache miss"))

		// Setup repository not found
		repo.On("GetByTenantID", ctx, tenantID).Return(nil, nil)

		config, err := service.GetConfig(ctx, tenantID)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, tenantID, config.TenantID)
		assert.Equal(t, 60, config.RateLimitConfig.DefaultRequestsPerMinute) // default values

		cache.AssertExpectations(t)
		repo.AssertExpectations(t)
	})

	t.Run("decryption error continues without tokens", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := NewTenantConfigService(repo, cache, encryption, logger)

		// Setup cache miss
		cache.On("Get", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig")).
			Return(errors.New("cache miss"))

		// Setup repository response
		repo.On("GetByTenantID", ctx, tenantID).Return(testConfig, nil)

		// Setup decryption failure
		encryption.On("Decrypt", ctx, testConfig.EncryptedTokens).
			Return(nil, errors.New("decryption failed"))

		// Setup cache set
		cache.On("Set", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig"), 5*time.Minute).
			Return(nil)

		config, err := service.GetConfig(ctx, tenantID)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Empty(t, config.ServiceTokens)

		cache.AssertExpectations(t)
		repo.AssertExpectations(t)
		encryption.AssertExpectations(t)
	})

	t.Run("no cache service", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := NewTenantConfigService(repo, nil, encryption, logger)

		// Setup repository response
		repo.On("GetByTenantID", ctx, tenantID).Return(testConfig, nil)

		// Setup decryption
		decryptedTokens := []byte(`{"github": "ghp_decrypted_token"}`)
		encryption.On("Decrypt", ctx, testConfig.EncryptedTokens).
			Return(decryptedTokens, nil)

		config, err := service.GetConfig(ctx, tenantID)
		assert.NoError(t, err)
		assert.NotNil(t, config)

		repo.AssertExpectations(t)
		encryption.AssertExpectations(t)
	})
}

func TestTenantConfigService_CreateConfig(t *testing.T) {
	ctx := context.Background()

	config := &models.TenantConfig{
		TenantID: "test-tenant-123",
		ServiceTokens: map[string]string{
			"github": "ghp_test_token",
		},
		Features: map[string]interface{}{
			"feature1": true,
		},
	}

	t.Run("successful creation with encryption", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := NewTenantConfigService(repo, cache, encryption, logger)

		// Setup encryption
		tokensJSON, _ := json.Marshal(config.ServiceTokens)
		encryptedTokens := []byte(`{"encrypted": "data"}`)
		encryption.On("Encrypt", ctx, tokensJSON).Return(encryptedTokens, nil)

		// Setup repository create
		repo.On("Create", ctx, config).Return(nil)

		// Setup cache invalidation
		cacheKey := "tenant:config:" + config.TenantID
		cache.On("Delete", ctx, cacheKey).Return(nil)

		err := service.CreateConfig(ctx, config)
		assert.NoError(t, err)
		assert.Equal(t, json.RawMessage(encryptedTokens), config.EncryptedTokens)

		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
		encryption.AssertExpectations(t)
	})

	t.Run("no encryption service", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		logger := observability.NewLogger("test")

		service := NewTenantConfigService(repo, cache, nil, logger)

		// Setup repository create
		repo.On("Create", ctx, config).Return(nil)

		// Setup cache invalidation
		cacheKey := "tenant:config:" + config.TenantID
		cache.On("Delete", ctx, cacheKey).Return(nil)

		err := service.CreateConfig(ctx, config)
		assert.NoError(t, err)

		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})

	t.Run("encryption error", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := NewTenantConfigService(repo, cache, encryption, logger)

		// Setup encryption failure
		tokensJSON, _ := json.Marshal(config.ServiceTokens)
		encryption.On("Encrypt", ctx, tokensJSON).Return(nil, errors.New("encryption failed"))

		err := service.CreateConfig(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to encrypt")

		encryption.AssertExpectations(t)
		repo.AssertNotCalled(t, "Create")
	})
}

func TestTenantConfigService_SetServiceToken(t *testing.T) {
	ctx := context.Background()
	tenantID := "test-tenant-123"
	provider := "github"
	token := "ghp_new_token"

	existingConfig := &models.TenantConfig{
		TenantID: tenantID,
		ServiceTokens: map[string]string{
			"gitlab": "glpat_existing",
		},
	}

	t.Run("successful token addition", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := &tenantConfigService{
			repo:       repo,
			cache:      cache,
			encryption: encryption,
			logger:     logger,
			cacheTTL:   5 * time.Minute,
		}

		// Mock GetConfig
		cacheKey := "tenant:config:" + tenantID
		cache.On("Get", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig")).
			Return(errors.New("cache miss"))
		repo.On("GetByTenantID", ctx, tenantID).Return(existingConfig, nil)
		encryption.On("Decrypt", ctx, mock.Anything).Return([]byte(`{}`), nil)
		cache.On("Set", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig"), 5*time.Minute).
			Return(nil)

		// Mock UpdateConfig
		encryption.On("Encrypt", ctx, mock.Anything).Return([]byte(`{"encrypted": "data"}`), nil)
		repo.On("Update", ctx, mock.AnythingOfType("*models.TenantConfig")).Return(nil)
		cache.On("Delete", ctx, cacheKey).Return(nil)

		err := service.SetServiceToken(ctx, tenantID, provider, token)
		assert.NoError(t, err)

		// Verify token was added
		assert.Equal(t, token, existingConfig.ServiceTokens[provider])
		assert.Equal(t, "glpat_existing", existingConfig.ServiceTokens["gitlab"])

		cache.AssertExpectations(t)
		repo.AssertExpectations(t)
		encryption.AssertExpectations(t)
	})
}

func TestTenantConfigService_SetFeature(t *testing.T) {
	ctx := context.Background()
	tenantID := "test-tenant-123"
	feature := "new_feature"
	value := true

	existingConfig := &models.TenantConfig{
		TenantID: tenantID,
		Features: map[string]interface{}{
			"existing_feature": "enabled",
		},
	}

	t.Run("successful feature addition", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := &tenantConfigService{
			repo:       repo,
			cache:      cache,
			encryption: encryption,
			logger:     logger,
			cacheTTL:   5 * time.Minute,
		}

		// Mock GetConfig
		cacheKey := "tenant:config:" + tenantID
		cache.On("Get", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig")).
			Return(errors.New("cache miss"))
		repo.On("GetByTenantID", ctx, tenantID).Return(existingConfig, nil)
		encryption.On("Decrypt", ctx, mock.Anything).Return([]byte(`{}`), nil)
		cache.On("Set", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig"), 5*time.Minute).
			Return(nil)

		// Mock UpdateConfig
		encryption.On("Encrypt", ctx, mock.Anything).Return([]byte(`{"encrypted": "data"}`), nil)
		repo.On("Update", ctx, mock.AnythingOfType("*models.TenantConfig")).Return(nil)
		cache.On("Delete", ctx, cacheKey).Return(nil)

		err := service.SetFeature(ctx, tenantID, feature, value)
		assert.NoError(t, err)

		// Verify feature was added
		assert.Equal(t, value, existingConfig.Features[feature])
		assert.Equal(t, "enabled", existingConfig.Features["existing_feature"])

		cache.AssertExpectations(t)
		repo.AssertExpectations(t)
		encryption.AssertExpectations(t)
	})
}

func TestTenantConfigService_IsFeatureEnabled(t *testing.T) {
	ctx := context.Background()
	tenantID := "test-tenant-123"

	config := &models.TenantConfig{
		TenantID: tenantID,
		Features: map[string]interface{}{
			"enabled_feature":  true,
			"disabled_feature": false,
			"string_feature":   "enabled",
		},
	}

	t.Run("check enabled feature", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := &tenantConfigService{
			repo:       repo,
			cache:      cache,
			encryption: encryption,
			logger:     logger,
			cacheTTL:   5 * time.Minute,
		}

		// Mock GetConfig
		cacheKey := "tenant:config:" + tenantID
		cache.On("Get", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig")).
			Run(func(args mock.Arguments) {
				c := args.Get(2).(*models.TenantConfig)
				*c = *config
			}).
			Return(nil)

		enabled, err := service.IsFeatureEnabled(ctx, tenantID, "enabled_feature")
		assert.NoError(t, err)
		assert.True(t, enabled)

		cache.AssertExpectations(t)
	})
}

func TestTenantConfigService_SetRateLimitForKeyType(t *testing.T) {
	ctx := context.Background()
	tenantID := "test-tenant-123"
	keyType := "admin"
	limit := models.KeyTypeRateLimit{
		RequestsPerMinute: 1000,
		RequestsPerHour:   50000,
		RequestsPerDay:    500000,
	}

	existingConfig := &models.TenantConfig{
		TenantID: tenantID,
		RateLimitConfig: models.RateLimitConfig{
			DefaultRequestsPerMinute: 60,
			KeyTypeOverrides:         make(map[string]models.KeyTypeRateLimit),
		},
	}

	t.Run("successful rate limit addition", func(t *testing.T) {
		repo := &mockTenantConfigRepository{}
		cache := &mockCache{}
		encryption := &mockEncryptionService{}
		logger := observability.NewLogger("test")

		service := &tenantConfigService{
			repo:       repo,
			cache:      cache,
			encryption: encryption,
			logger:     logger,
			cacheTTL:   5 * time.Minute,
		}

		// Mock GetConfig
		cacheKey := "tenant:config:" + tenantID
		cache.On("Get", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig")).
			Return(errors.New("cache miss"))
		repo.On("GetByTenantID", ctx, tenantID).Return(existingConfig, nil)
		encryption.On("Decrypt", ctx, mock.Anything).Return([]byte(`{}`), nil)
		cache.On("Set", ctx, cacheKey, mock.AnythingOfType("*models.TenantConfig"), 5*time.Minute).
			Return(nil)

		// Mock UpdateConfig
		encryption.On("Encrypt", ctx, mock.Anything).Return([]byte(`{"encrypted": "data"}`), nil)
		repo.On("Update", ctx, mock.AnythingOfType("*models.TenantConfig")).Return(nil)
		cache.On("Delete", ctx, cacheKey).Return(nil)

		err := service.SetRateLimitForKeyType(ctx, tenantID, keyType, limit)
		assert.NoError(t, err)

		// Verify rate limit was added
		assert.Equal(t, limit, existingConfig.RateLimitConfig.KeyTypeOverrides[keyType])

		cache.AssertExpectations(t)
		repo.AssertExpectations(t)
		encryption.AssertExpectations(t)
	})
}

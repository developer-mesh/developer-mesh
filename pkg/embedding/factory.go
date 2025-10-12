package embedding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/agents"
	"github.com/developer-mesh/developer-mesh/pkg/chunking"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/common/config"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
)

// EmbeddingFactoryConfig contains configuration for the embedding factory
type EmbeddingFactoryConfig struct {
	// Model configuration
	ModelType       ModelType `json:"model_type"`
	ModelName       string    `json:"model_name"`
	ModelAPIKey     string    `json:"model_api_key,omitempty"`
	ModelEndpoint   string    `json:"model_endpoint,omitempty"`
	ModelDimensions int       `json:"model_dimensions"`

	// Additional model parameters (used for provider-specific configurations)
	Parameters map[string]interface{} `json:"parameters,omitempty"`

	// Storage configuration
	DatabaseConnection *sql.DB `json:"-"`
	DatabaseSchema     string  `json:"database_schema"`

	// Pipeline configuration
	Concurrency     int  `json:"concurrency"`
	BatchSize       int  `json:"batch_size"`
	IncludeComments bool `json:"include_comments"`
	EnrichMetadata  bool `json:"enrich_metadata"`
}

// NewEmbeddingFactory creates a new embedding factory with the specified configuration
func NewEmbeddingFactory(config *EmbeddingFactoryConfig) (*EmbeddingFactory, error) {
	if config == nil {
		return nil, errors.New("configuration is required")
	}

	// Validate required configuration parameters
	if config.ModelType == "" {
		return nil, errors.New("model type is required")
	}

	// Check if the model type is supported
	if config.ModelType != ModelTypeOpenAI && config.ModelType != ModelTypeBedrock &&
		config.ModelType != ModelTypeHuggingFace && config.ModelType != ModelTypeAnthropic &&
		config.ModelType != ModelTypeCustom {
		return nil, fmt.Errorf("unsupported model type: %s", config.ModelType)
	}

	if config.ModelName == "" {
		return nil, errors.New("model name is required")
	}

	// Check for required API key for certain model types
	if config.ModelType == ModelTypeOpenAI && config.ModelAPIKey == "" {
		return nil, errors.New("model API key is required for OpenAI models")
	}

	if config.DatabaseConnection == nil {
		return nil, errors.New("database connection is required")
	}

	if config.DatabaseSchema == "" {
		return nil, errors.New("database schema is required")
	}

	// Set default values for optional parameters
	if config.Concurrency <= 0 {
		config.Concurrency = 4 // Default concurrency
	}

	if config.BatchSize <= 0 {
		config.BatchSize = 10 // Default batch size
	}

	return &EmbeddingFactory{
		config: config,
	}, nil
}

// EmbeddingFactory creates and configures embedding components
type EmbeddingFactory struct {
	config *EmbeddingFactoryConfig
}

// CreateEmbeddingService creates an embedding service based on the factory configuration
func (f *EmbeddingFactory) CreateEmbeddingService() (EmbeddingService, error) {
	switch f.config.ModelType {
	case ModelTypeAnthropic:
		// Extract Anthropic configuration
		endpoint := ""
		useMock := false

		// Check for endpoint in ModelEndpoint field
		if f.config.ModelEndpoint != "" {
			endpoint = f.config.ModelEndpoint
		}

		// Check for parameters
		if f.config.Parameters != nil {
			// Check for endpoint parameter
			if e, ok := f.config.Parameters["endpoint"].(string); ok && e != "" {
				endpoint = e
			}

			// Check for mock mode configuration
			if mockVal, ok := f.config.Parameters["use_mock_embeddings"].(bool); ok {
				useMock = mockVal
			} else if mockVal, ok := f.config.Parameters["use_mock"].(bool); ok {
				// Alternate naming for backward compatibility
				useMock = mockVal
			}
		}

		// Create Anthropic configuration
		config := &AnthropicConfig{
			APIKey:            f.config.ModelAPIKey,
			Endpoint:          endpoint,
			Model:             f.config.ModelName,
			UseMockEmbeddings: useMock,
		}

		// If use_mock is explicitly true, use the mock constructor directly
		if useMock {
			return NewMockAnthropicEmbeddingService(f.config.ModelName)
		}

		return NewAnthropicEmbeddingService(config)

	case ModelTypeOpenAI:
		return NewOpenAIEmbeddingService(
			f.config.ModelAPIKey,
			f.config.ModelName,
			f.config.ModelDimensions,
		)
	case ModelTypeBedrock:
		// Get AWS configuration parameters
		region := "us-west-2" // Default region
		accessKey := ""
		secretKey := ""
		sessionToken := ""
		useMock := false

		if f.config.ModelEndpoint != "" {
			region = f.config.ModelEndpoint // Using ModelEndpoint for region as a temporary solution
		}

		if f.config.Parameters != nil {
			// Check for region parameter
			if r, ok := f.config.Parameters["region"].(string); ok && r != "" {
				region = r
			}

			// Check for credentials with both naming conventions
			// 1. First check for the standard AWS SDK names
			if ak, ok := f.config.Parameters["access_key_id"].(string); ok {
				accessKey = ak
			} else if ak, ok := f.config.Parameters["access_key"].(string); ok {
				// Backward compatibility
				accessKey = ak
			}

			if sk, ok := f.config.Parameters["secret_access_key"].(string); ok {
				secretKey = sk
			} else if sk, ok := f.config.Parameters["secret_key"].(string); ok {
				// Backward compatibility
				secretKey = sk
			}

			if st, ok := f.config.Parameters["session_token"].(string); ok {
				sessionToken = st
			}

			// Check for mock mode configuration
			if mockVal, ok := f.config.Parameters["use_mock_embeddings"].(bool); ok {
				useMock = mockVal
			} else if mockVal, ok := f.config.Parameters["use_mock"].(bool); ok {
				// Alternate naming for backward compatibility
				useMock = mockVal
			}
		}

		// Create Bedrock configuration
		config := &BedrockConfig{
			Region:            region,
			AccessKeyID:       accessKey,
			SecretAccessKey:   secretKey,
			SessionToken:      sessionToken,
			ModelID:           f.config.ModelName,
			UseMockEmbeddings: useMock,
		}

		// If use_mock is explicitly true, use the mock constructor directly
		if useMock {
			return NewMockBedrockEmbeddingService(f.config.ModelName)
		}

		// Otherwise use standard constructor which will still fall back to mock if needed
		return NewBedrockEmbeddingService(config)
	case ModelTypeHuggingFace:
		// Not implemented yet
		return nil, errors.New("HuggingFace embedding service not implemented yet")
	case ModelTypeCustom:
		// Not implemented yet
		return nil, errors.New("custom embedding service not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported model type: %s", f.config.ModelType)
	}
}

// CreateEmbeddingStorage creates an embedding storage based on the factory configuration
func (f *EmbeddingFactory) CreateEmbeddingStorage() (EmbeddingStorage, error) {
	if f.config.DatabaseConnection == nil {
		return nil, errors.New("database connection is required")
	}

	return NewPgVectorStorage(f.config.DatabaseConnection, f.config.DatabaseSchema)
}

// CreateEmbeddingPipeline creates a complete embedding pipeline
func (f *EmbeddingFactory) CreateEmbeddingPipeline(
	chunkingService *chunking.ChunkingService,
	contentProvider GitHubContentProvider,
) (*DefaultEmbeddingPipeline, error) {
	// Validate required parameters
	if chunkingService == nil {
		return nil, errors.New("chunking service is required")
	}

	if contentProvider == nil {
		return nil, errors.New("content provider is required")
	}

	// Create the embedding service
	embeddingService, err := f.CreateEmbeddingService()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding service: %w", err)
	}

	// Create the embedding storage
	storage, err := f.CreateEmbeddingStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding storage: %w", err)
	}

	// Create the pipeline configuration
	pipelineConfig := &EmbeddingPipelineConfig{
		Concurrency:     f.config.Concurrency,
		BatchSize:       f.config.BatchSize,
		IncludeComments: f.config.IncludeComments,
		EnrichMetadata:  f.config.EnrichMetadata,
	}

	// Create the embedding pipeline
	pipeline, err := NewEmbeddingPipeline(
		embeddingService,
		storage,
		chunkingService,
		contentProvider,
		pipelineConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding pipeline: %w", err)
	}

	return pipeline, nil
}

// Initialize tests all components and returns a fully configured pipeline ready for use
func (f *EmbeddingFactory) Initialize(ctx context.Context, chunkingService *chunking.ChunkingService, contentProvider GitHubContentProvider) (*DefaultEmbeddingPipeline, error) {
	// Create the embedding pipeline
	pipeline, err := f.CreateEmbeddingPipeline(chunkingService, contentProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding pipeline: %w", err)
	}

	// Optionally test the embedding service with a simple ping
	// This is just to verify that the embedding service can be reached
	embeddingService, err := f.CreateEmbeddingService()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding service: %w", err)
	}

	// Test embedding generation with a simple text
	_, err = embeddingService.GenerateEmbedding(ctx, "This is a test", "test", "test-id")
	if err != nil {
		return nil, fmt.Errorf("embedding service test failed: %w", err)
	}

	return pipeline, nil
}

// CreateEmbeddingServiceV2 creates the multi-agent embedding service (ServiceV2)
// This is a standalone factory function used by REST API and Worker for ServiceV2
func CreateEmbeddingServiceV2(cfg *config.Config, db database.Database, cache cache.Cache) (*ServiceV2, error) {
	// Initialize providers map
	providerMap := make(map[string]providers.Provider)

	// Check if config exists
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}

	// Configure OpenAI if enabled
	if cfg.Embedding.Providers.OpenAI.Enabled && cfg.Embedding.Providers.OpenAI.APIKey != "" {
		openaiCfg := providers.ProviderConfig{
			APIKey:         cfg.Embedding.Providers.OpenAI.APIKey,
			Endpoint:       "https://api.openai.com/v1",
			MaxRetries:     3,
			RetryDelayBase: 100 * time.Millisecond,
		}

		openaiProvider, err := providers.NewOpenAIProvider(openaiCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenAI provider: %w", err)
		}
		providerMap["openai"] = openaiProvider
	}

	// Configure AWS Bedrock if enabled
	if cfg.Embedding.Providers.Bedrock.Enabled {
		bedrockCfg := providers.ProviderConfig{
			Region:   cfg.Embedding.Providers.Bedrock.Region,
			Endpoint: cfg.Embedding.Providers.Bedrock.Endpoint,
		}

		bedrockProvider, err := providers.NewBedrockProvider(bedrockCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create Bedrock provider: %w", err)
		}
		providerMap["bedrock"] = bedrockProvider
	}

	// Configure Google if enabled
	if cfg.Embedding.Providers.Google.Enabled && cfg.Embedding.Providers.Google.APIKey != "" {
		googleCfg := providers.ProviderConfig{
			APIKey:   cfg.Embedding.Providers.Google.APIKey,
			Endpoint: cfg.Embedding.Providers.Google.Endpoint,
		}

		googleProvider, err := providers.NewGoogleProvider(googleCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create Google provider: %w", err)
		}
		providerMap["google"] = googleProvider
	}

	// Require at least one provider
	if len(providerMap) == 0 {
		return nil, fmt.Errorf("at least one embedding provider must be configured (OpenAI, Bedrock, or Google)")
	}

	// Initialize repositories
	sqlxDB := db.DB()
	sqlDB := sqlxDB.DB // Get the underlying *sql.DB from *sqlx.DB

	// Create agent repository and service
	agentRepo := agents.NewPostgresRepository(sqlxDB, "mcp")
	agentService := agents.NewService(agentRepo)

	// Create embedding repository
	embeddingRepo := NewRepository(sqlDB)

	// For now, metrics repository can be nil - it's optional
	var metricsRepo MetricsRepository

	// Create embedding cache adapter
	embeddingCache := NewEmbeddingCacheAdapter(cache)

	// Create ServiceV2 - this is our ONLY embedding service
	return NewServiceV2(ServiceV2Config{
		Providers:    providerMap,
		AgentService: agentService,
		Repository:   embeddingRepo,
		MetricsRepo:  metricsRepo,
		Cache:        embeddingCache,
	})
}

// EmbeddingCacheAdapter adapts cache.Cache to EmbeddingCache
type EmbeddingCacheAdapter struct {
	cache cache.Cache
}

// NewEmbeddingCacheAdapter creates a new cache adapter
func NewEmbeddingCacheAdapter(cache cache.Cache) EmbeddingCache {
	return &EmbeddingCacheAdapter{cache: cache}
}

// Get retrieves a cached embedding
func (e *EmbeddingCacheAdapter) Get(ctx context.Context, key string) (*CachedEmbedding, error) {
	var cached CachedEmbedding
	err := e.cache.Get(ctx, key, &cached)
	if err != nil {
		return nil, err
	}
	return &cached, nil
}

// Set stores an embedding in cache
func (e *EmbeddingCacheAdapter) Set(ctx context.Context, key string, embedding *CachedEmbedding, ttl time.Duration) error {
	return e.cache.Set(ctx, key, embedding, ttl)
}

// Delete removes an embedding from cache
func (e *EmbeddingCacheAdapter) Delete(ctx context.Context, key string) error {
	return e.cache.Delete(ctx, key)
}

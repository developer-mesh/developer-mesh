package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/model_catalog"
)

// ModelDiscoveryService handles automatic discovery of embedding models from providers
type ModelDiscoveryService interface {
	// DiscoverBedrockModels discovers available models from AWS Bedrock
	DiscoverBedrockModels(ctx context.Context) ([]*model_catalog.EmbeddingModel, error)

	// DiscoverOpenAIModels discovers available models from OpenAI
	DiscoverOpenAIModels(ctx context.Context) ([]*model_catalog.EmbeddingModel, error)

	// DiscoverAllProviders discovers models from all configured providers
	DiscoverAllProviders(ctx context.Context) (map[string][]*model_catalog.EmbeddingModel, error)

	// ScheduleDiscovery schedules periodic model discovery
	ScheduleDiscovery(ctx context.Context, interval time.Duration) error

	// ProcessDiscoveryEvent processes a discovery event from Redis stream
	ProcessDiscoveryEvent(ctx context.Context, event *DiscoveryEvent) error
}

// DiscoveryEvent represents a model discovery event
type DiscoveryEvent struct {
	ID        string                 `json:"id"`
	Provider  string                 `json:"provider"`
	Timestamp time.Time              `json:"timestamp"`
	Models    []DiscoveredModel      `json:"models"`
	Errors    []string               `json:"errors,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// DiscoveredModel represents a discovered model
type DiscoveredModel struct {
	ModelID      string  `json:"model_id"`
	ModelName    string  `json:"model_name"`
	Provider     string  `json:"provider"`
	Version      string  `json:"version,omitempty"`
	Dimensions   int     `json:"dimensions,omitempty"`
	MaxTokens    int     `json:"max_tokens,omitempty"`
	CostPerToken float64 `json:"cost_per_token,omitempty"`
	IsEmbedding  bool    `json:"is_embedding"`
}

// ModelDiscoveryServiceImpl implements ModelDiscoveryService
type ModelDiscoveryServiceImpl struct {
	bedrockClient *bedrockruntime.Client
	redisClient   *redis.Client
	catalogRepo   model_catalog.ModelCatalogRepository
	logger        observability.Logger
	streamKey     string
}

// NewModelDiscoveryService creates a new ModelDiscoveryService
func NewModelDiscoveryService(
	awsConfig awsconfig.Config,
	redisClient *redis.Client,
	catalogRepo model_catalog.ModelCatalogRepository,
) (ModelDiscoveryService, error) {
	bedrockClient := bedrockruntime.NewFromConfig(awsConfig)

	return &ModelDiscoveryServiceImpl{
		bedrockClient: bedrockClient,
		redisClient:   redisClient,
		catalogRepo:   catalogRepo,
		logger:        observability.NewStandardLogger("model_discovery_service"),
		streamKey:     "model_discovery_events",
	}, nil
}

// DiscoverBedrockModels discovers available models from AWS Bedrock
func (s *ModelDiscoveryServiceImpl) DiscoverBedrockModels(ctx context.Context) ([]*model_catalog.EmbeddingModel, error) {
	s.logger.Info("Starting Bedrock model discovery", nil)

	// Since bedrockruntime doesn't have ListFoundationModels, we'll use a hardcoded list
	// In production, this could be updated via a separate bedrock management API call
	bedrockModels := []struct {
		ID         string
		Name       string
		Version    string
		Dimensions int
		MaxTokens  int
		Cost       float64
	}{
		{"amazon.titan-embed-text-v1", "Amazon Titan Embedding V1", "1.0", 1536, 8192, 0.10},
		{"amazon.titan-embed-text-v2:0", "Amazon Titan Embedding V2", "2.0", 1024, 8192, 0.02},
		{"amazon.titan-embed-image-v1", "Amazon Titan Image Embedding", "1.0", 1024, 0, 0.08},
		{"cohere.embed-english-v3", "Cohere Embed English V3", "3.0", 1024, 512, 0.10},
		{"cohere.embed-multilingual-v3", "Cohere Embed Multilingual V3", "3.0", 1024, 512, 0.10},
	}

	var embeddingModels []*model_catalog.EmbeddingModel

	for _, model := range bedrockModels {
		embeddingModel := &model_catalog.EmbeddingModel{
			ID:                   uuid.New(),
			Provider:             "bedrock",
			ModelName:            model.Name,
			ModelID:              model.ID,
			ModelVersion:         &model.Version,
			Dimensions:           model.Dimensions,
			MaxTokens:            &model.MaxTokens,
			CostPerMillionTokens: &model.Cost,
			ModelType:            "embedding",
			IsAvailable:          true,
			IsDeprecated:         false,
			RequiresAPIKey:       false,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}

		// Add capabilities
		capabilities := map[string]interface{}{
			"provider":       "aws",
			"supports_batch": true,
			"max_batch_size": 100,
		}
		embeddingModel.Capabilities, _ = json.Marshal(capabilities)

		embeddingModels = append(embeddingModels, embeddingModel)
	}

	s.logger.Info("Discovered Bedrock models", map[string]interface{}{
		"count": len(embeddingModels),
	})

	// Bulk upsert to catalog
	if len(embeddingModels) > 0 {
		err := s.catalogRepo.BulkUpsert(ctx, embeddingModels)
		if err != nil {
			return nil, fmt.Errorf("failed to upsert Bedrock models: %w", err)
		}
	}

	// Publish discovery event to Redis stream
	s.publishDiscoveryEvent(ctx, "bedrock", embeddingModels, nil)

	return embeddingModels, nil
}

// DiscoverOpenAIModels discovers available models from OpenAI
func (s *ModelDiscoveryServiceImpl) DiscoverOpenAIModels(ctx context.Context) ([]*model_catalog.EmbeddingModel, error) {
	s.logger.Info("Starting OpenAI model discovery", nil)

	// Hardcoded list for now since OpenAI API doesn't provide embedding model list
	// In production, this would query the OpenAI API
	openAIModels := []struct {
		ID         string
		Name       string
		Dimensions int
		MaxTokens  int
		Cost       float64
	}{
		{"text-embedding-3-small", "Text Embedding 3 Small", 1536, 8191, 0.02},
		{"text-embedding-3-large", "Text Embedding 3 Large", 3072, 8191, 0.13},
		{"text-embedding-ada-002", "Text Embedding Ada v2", 1536, 8191, 0.10},
	}

	var embeddingModels []*model_catalog.EmbeddingModel

	for _, model := range openAIModels {
		embeddingModel := &model_catalog.EmbeddingModel{
			ID:                   uuid.New(),
			Provider:             "openai",
			ModelName:            model.Name,
			ModelID:              model.ID,
			Dimensions:           model.Dimensions,
			MaxTokens:            &model.MaxTokens,
			CostPerMillionTokens: &model.Cost,
			ModelType:            "embedding",
			IsAvailable:          true,
			IsDeprecated:         model.ID == "text-embedding-ada-002", // Ada is deprecated
			RequiresAPIKey:       true,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}

		// Add capabilities
		capabilities := map[string]interface{}{
			"supports_batching": true,
			"max_batch_size":    2048,
			"encoding":          "cl100k_base",
		}
		embeddingModel.Capabilities, _ = json.Marshal(capabilities)

		embeddingModels = append(embeddingModels, embeddingModel)
	}

	s.logger.Info("Discovered OpenAI models", map[string]interface{}{
		"count": len(embeddingModels),
	})

	// Bulk upsert to catalog
	if len(embeddingModels) > 0 {
		err := s.catalogRepo.BulkUpsert(ctx, embeddingModels)
		if err != nil {
			return nil, fmt.Errorf("failed to upsert OpenAI models: %w", err)
		}
	}

	// Publish discovery event to Redis stream
	s.publishDiscoveryEvent(ctx, "openai", embeddingModels, nil)

	return embeddingModels, nil
}

// DiscoverAllProviders discovers models from all configured providers
func (s *ModelDiscoveryServiceImpl) DiscoverAllProviders(ctx context.Context) (map[string][]*model_catalog.EmbeddingModel, error) {
	results := make(map[string][]*model_catalog.EmbeddingModel)
	errors := []string{}

	// Discover Bedrock models
	bedrockModels, err := s.DiscoverBedrockModels(ctx)
	if err != nil {
		s.logger.Error("Failed to discover Bedrock models", map[string]interface{}{
			"error": err.Error(),
		})
		errors = append(errors, fmt.Sprintf("bedrock: %v", err))
	} else {
		results["bedrock"] = bedrockModels
	}

	// Discover OpenAI models
	openAIModels, err := s.DiscoverOpenAIModels(ctx)
	if err != nil {
		s.logger.Error("Failed to discover OpenAI models", map[string]interface{}{
			"error": err.Error(),
		})
		errors = append(errors, fmt.Sprintf("openai: %v", err))
	} else {
		results["openai"] = openAIModels
	}

	// Log summary
	totalModels := 0
	for _, models := range results {
		totalModels += len(models)
	}

	s.logger.Info("Completed model discovery", map[string]interface{}{
		"providers":    len(results),
		"total_models": totalModels,
		"errors":       len(errors),
	})

	if len(errors) > 0 && len(results) == 0 {
		return nil, fmt.Errorf("all providers failed: %v", strings.Join(errors, "; "))
	}

	return results, nil
}

// ScheduleDiscovery schedules periodic model discovery using Redis streams
func (s *ModelDiscoveryServiceImpl) ScheduleDiscovery(ctx context.Context, interval time.Duration) error {
	// Create a scheduled task in Redis stream
	event := map[string]interface{}{
		"type":     "schedule_discovery",
		"interval": interval.String(),
		"next_run": time.Now().Add(interval),
	}

	err := s.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: s.streamKey,
		Values: event,
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to schedule discovery: %w", err)
	}

	s.logger.Info("Scheduled model discovery", map[string]interface{}{
		"interval": interval.String(),
	})

	return nil
}

// ProcessDiscoveryEvent processes a discovery event from Redis stream
func (s *ModelDiscoveryServiceImpl) ProcessDiscoveryEvent(ctx context.Context, event *DiscoveryEvent) error {
	s.logger.Info("Processing discovery event", map[string]interface{}{
		"event_id": event.ID,
		"provider": event.Provider,
	})

	// Convert discovered models to catalog models
	var catalogModels []*model_catalog.EmbeddingModel

	for _, discovered := range event.Models {
		if !discovered.IsEmbedding {
			continue
		}

		catalogModel := &model_catalog.EmbeddingModel{
			ID:                   uuid.New(),
			Provider:             discovered.Provider,
			ModelName:            discovered.ModelName,
			ModelID:              discovered.ModelID,
			ModelVersion:         &discovered.Version,
			Dimensions:           discovered.Dimensions,
			MaxTokens:            &discovered.MaxTokens,
			CostPerMillionTokens: &discovered.CostPerToken,
			ModelType:            "embedding",
			IsAvailable:          true,
			IsDeprecated:         false,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}

		catalogModels = append(catalogModels, catalogModel)
	}

	// Bulk upsert to catalog
	if len(catalogModels) > 0 {
		err := s.catalogRepo.BulkUpsert(ctx, catalogModels)
		if err != nil {
			return fmt.Errorf("failed to upsert discovered models: %w", err)
		}
	}

	s.logger.Info("Processed discovery event", map[string]interface{}{
		"models_added": len(catalogModels),
	})

	return nil
}

// Helper methods

// TODO: Uncomment these helper methods when model metadata enrichment is implemented
// func (s *ModelDiscoveryServiceImpl) getModelDimensions(modelID string) int {
// 	// Map of known model dimensions
// 	dimensions := map[string]int{
// 		"amazon.titan-embed-text-v1":   1536,
// 		"amazon.titan-embed-text-v2:0": 1024,
// 		"amazon.titan-embed-image-v1":  1024,
// 		"cohere.embed-english-v3":      1024,
// 		"cohere.embed-multilingual-v3": 1024,
// 	}
//
// 	if dim, ok := dimensions[modelID]; ok {
// 		return dim
// 	}
//
// 	return 1536 // Default dimension
// }
//
// func (s *ModelDiscoveryServiceImpl) getModelMaxTokens(modelID string) int {
// 	// Map of known model token limits
// 	tokens := map[string]int{
// 		"amazon.titan-embed-text-v1":   8192,
// 		"amazon.titan-embed-text-v2:0": 8192,
// 		"amazon.titan-embed-image-v1":  0, // Image model
// 		"cohere.embed-english-v3":      512,
// 		"cohere.embed-multilingual-v3": 512,
// 	}
//
// 	if tok, ok := tokens[modelID]; ok {
// 		return tok
// 	}
//
// 	return 8192 // Default token limit
// }
//
// func (s *ModelDiscoveryServiceImpl) getModelCost(modelID string) float64 {
// 	// Map of known model costs per million tokens
// 	costs := map[string]float64{
// 		"amazon.titan-embed-text-v1":   0.10,
// 		"amazon.titan-embed-text-v2:0": 0.02,
// 		"amazon.titan-embed-image-v1":  0.08,
// 		"cohere.embed-english-v3":      0.10,
// 		"cohere.embed-multilingual-v3": 0.10,
// 	}
//
// 	if cost, ok := costs[modelID]; ok {
// 		return cost
// 	}
//
// 	return 0.10 // Default cost
// }

func (s *ModelDiscoveryServiceImpl) publishDiscoveryEvent(ctx context.Context, provider string, models []*model_catalog.EmbeddingModel, errors []string) {
	// Convert models to discovered format
	var discovered []DiscoveredModel
	for _, model := range models {
		var version string
		if model.ModelVersion != nil {
			version = *model.ModelVersion
		}

		var maxTokens int
		if model.MaxTokens != nil {
			maxTokens = *model.MaxTokens
		}

		var cost float64
		if model.CostPerMillionTokens != nil {
			cost = *model.CostPerMillionTokens
		}

		discovered = append(discovered, DiscoveredModel{
			ModelID:      model.ModelID,
			ModelName:    model.ModelName,
			Provider:     provider,
			Version:      version,
			Dimensions:   model.Dimensions,
			MaxTokens:    maxTokens,
			CostPerToken: cost,
			IsEmbedding:  true,
		})
	}

	// Create discovery event
	event := DiscoveryEvent{
		ID:        uuid.New().String(),
		Provider:  provider,
		Timestamp: time.Now(),
		Models:    discovered,
		Errors:    errors,
		Metadata: map[string]interface{}{
			"source": "automatic_discovery",
		},
	}

	// Publish to Redis stream
	eventJSON, _ := json.Marshal(event)
	err := s.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: s.streamKey,
		Values: map[string]interface{}{
			"event": string(eventJSON),
		},
	}).Err()

	if err != nil {
		s.logger.Error("Failed to publish discovery event", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/developer-mesh/developer-mesh/pkg/agents"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
	"github.com/google/uuid"
)

// AgentService defines the interface for agent configuration management
type AgentService interface {
	GetConfig(ctx context.Context, agentID string) (*agents.AgentConfig, error)
	GetModelsForAgent(ctx context.Context, agentID string, taskType agents.TaskType) (primary []string, fallback []string, err error)
	CreateConfig(ctx context.Context, config *agents.AgentConfig) error
	UpdateConfig(ctx context.Context, agentID string, update *agents.ConfigUpdateRequest) (*agents.AgentConfig, error)
}

// ServiceV2 is the enhanced embedding service with multi-agent support
type ServiceV2 struct {
	providers        map[string]providers.Provider
	agentService     AgentService
	repository       *Repository
	metricsRepo      MetricsRepository
	router           *SmartRouter
	dimensionAdapter *DimensionAdapter
	cache            EmbeddingCache
	modelSelector    ModelSelector
	progressFunc     func(float64) // Progress callback for batch operations
	mu               sync.RWMutex
}

// ServiceV2Config contains configuration for the service
type ServiceV2Config struct {
	Providers     map[string]providers.Provider
	AgentService  AgentService
	Repository    *Repository
	MetricsRepo   MetricsRepository
	Cache         EmbeddingCache
	ModelSelector ModelSelector
	RouterConfig  *RouterConfig
}

// EmbeddingCache defines the interface for caching embeddings
type EmbeddingCache interface {
	Get(ctx context.Context, key string) (*CachedEmbedding, error)
	Set(ctx context.Context, key string, embedding *CachedEmbedding, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// CachedEmbedding represents a cached embedding
type CachedEmbedding struct {
	Embedding  []float32              `json:"embedding"`
	Model      string                 `json:"model"`
	Provider   string                 `json:"provider"`
	Dimensions int                    `json:"dimensions"`
	Metadata   map[string]interface{} `json:"metadata"`
	CachedAt   time.Time              `json:"cached_at"`
}

// MetricsRepository stores embedding metrics
type MetricsRepository interface {
	RecordMetric(ctx context.Context, metric *EmbeddingMetric) error
	GetMetrics(ctx context.Context, filter MetricsFilter) ([]*EmbeddingMetric, error)
	GetAgentCosts(ctx context.Context, agentID string, period time.Duration) (*CostSummary, error)
}

// EmbeddingMetric represents a single metric entry
type EmbeddingMetric struct {
	ID                     uuid.UUID `json:"id" db:"id"`
	AgentID                string    `json:"agent_id" db:"agent_id"`
	ModelProvider          string    `json:"model_provider" db:"model_provider"`
	ModelName              string    `json:"model_name" db:"model_name"`
	ModelDimensions        int       `json:"model_dimensions" db:"model_dimensions"`
	RequestID              uuid.UUID `json:"request_id" db:"request_id"`
	TokenCount             int       `json:"token_count" db:"token_count"`
	TotalLatencyMs         int       `json:"total_latency_ms" db:"total_latency_ms"`
	ProviderLatencyMs      int       `json:"provider_latency_ms" db:"provider_latency_ms"`
	NormalizationLatencyMs int       `json:"normalization_latency_ms" db:"normalization_latency_ms"`
	CostUSD                float64   `json:"cost_usd" db:"cost_usd"`
	Status                 string    `json:"status" db:"status"`
	ErrorMessage           string    `json:"error_message" db:"error_message"`
	RetryCount             int       `json:"retry_count" db:"retry_count"`
	FinalProvider          string    `json:"final_provider" db:"final_provider"`
	TenantID               uuid.UUID `json:"tenant_id" db:"tenant_id"`
	Timestamp              time.Time `json:"timestamp" db:"timestamp"`
}

// GenerateEmbeddingRequest represents a request to generate an embedding
type GenerateEmbeddingRequest struct {
	AgentID   string                 `json:"agent_id" validate:"required"`
	Text      string                 `json:"text" validate:"required,max=50000"`
	Model     string                 `json:"model"` // Optional model specification (e.g., "bedrock:amazon.titan-embed-text-v2:0")
	TaskType  agents.TaskType        `json:"task_type"`
	Metadata  map[string]interface{} `json:"metadata"`
	RequestID string                 `json:"request_id"`
	TenantID  uuid.UUID              `json:"tenant_id"`
	ContextID *uuid.UUID             `json:"context_id,omitempty"` // Optional context reference
}

// GenerateEmbeddingResponse represents the response from generating an embedding
type GenerateEmbeddingResponse struct {
	EmbeddingID          uuid.UUID              `json:"embedding_id"`
	RequestID            string                 `json:"request_id"`
	ModelUsed            string                 `json:"model_used"`
	Provider             string                 `json:"provider"`
	Dimensions           int                    `json:"dimensions"`
	NormalizedDimensions int                    `json:"normalized_dimensions"`
	CostUSD              float64                `json:"cost_usd"`
	TokensUsed           int                    `json:"tokens_used"`
	GenerationTimeMs     int64                  `json:"generation_time_ms"`
	Cached               bool                   `json:"cached"`
	Metadata             map[string]interface{} `json:"metadata"`
}

// NewServiceV2 creates a new enhanced embedding service
func NewServiceV2(config ServiceV2Config) (*ServiceV2, error) {
	if len(config.Providers) == 0 {
		return nil, fmt.Errorf("at least one provider is required")
	}

	if config.AgentService == nil {
		return nil, fmt.Errorf("agent service is required")
	}

	if config.Repository == nil {
		return nil, fmt.Errorf("repository is required")
	}

	s := &ServiceV2{
		providers:     config.Providers,
		agentService:  config.AgentService,
		repository:    config.Repository,
		metricsRepo:   config.MetricsRepo,
		cache:         config.Cache,
		modelSelector: config.ModelSelector,
	}

	// Use default model selector if none provided
	if s.modelSelector == nil {
		s.modelSelector = NewDefaultModelSelector()
	}

	// Initialize router
	if config.RouterConfig == nil {
		config.RouterConfig = DefaultRouterConfig()
	}
	s.router = NewSmartRouter(config.RouterConfig, config.Providers)

	// Initialize dimension adapter
	s.dimensionAdapter = NewDimensionAdapter()

	return s, nil
}

// parseModelString parses a model string like "bedrock:amazon.titan-embed-text-v2:0" into provider and model
func (s *ServiceV2) parseModelString(modelStr string) (provider, model string) {
	if modelStr == "" {
		return "", ""
	}

	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	// If no provider specified, try to infer from model name
	if strings.Contains(modelStr, "titan") {
		return "bedrock", modelStr
	}
	if strings.Contains(modelStr, "ada") || strings.Contains(modelStr, "embedding") {
		return "openai", modelStr
	}
	if strings.Contains(modelStr, "gemini") || strings.Contains(modelStr, "gecko") {
		return "google", modelStr
	}

	// Default to bedrock
	return "bedrock", modelStr
}

// GenerateEmbedding generates an embedding for the given request
func (s *ServiceV2) GenerateEmbedding(ctx context.Context, req GenerateEmbeddingRequest) (*GenerateEmbeddingResponse, error) {
	// Input validation
	if err := s.validateEmbeddingRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	start := time.Now()
	requestID := req.RequestID
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Try to get agent configuration, but don't fail if not found
	var agentConfig *agents.AgentConfig
	if req.AgentID != "" {
		agentConfig, _ = s.agentService.GetConfig(ctx, req.AgentID)
		// Note: agent config not found, will use defaults
	}

	// Determine task type
	taskType := req.TaskType
	if taskType == "" {
		taskType = agents.TaskTypeGeneralQA
	}

	// First check if we already have this embedding in the database
	// This is more efficient than generating and then hitting a duplicate key error
	contentHash := calculateContentHash(req.Text)

	// Determine which model we'll use (needed for deduplication check)
	modelName := "amazon.titan-embed-text-v2:0" // Default
	if req.Model != "" {
		_, model := s.parseModelString(req.Model)
		if model != "" {
			modelName = model
		}
	}

	// Check for existing embedding
	existingEmbeddingID, checkErr := s.repository.GetExistingEmbedding(ctx, contentHash, modelName, req.TenantID)
	if checkErr != nil {
		// Log warning but continue with generation
		// Continue with generation even if check fails
	} else if existingEmbeddingID != nil {
		// We found an existing embedding - return it immediately

		return &GenerateEmbeddingResponse{
			EmbeddingID:          *existingEmbeddingID,
			RequestID:            requestID,
			ModelUsed:            modelName,
			Provider:             "cached", // Indicates this was retrieved, not generated
			Dimensions:           1024,     // Default dimensions for Titan v2
			NormalizedDimensions: StandardDimension,
			Cached:               true,
			Metadata: map[string]interface{}{
				"deduplicated": true,
				"content_hash": contentHash,
			},
			GenerationTimeMs: time.Since(start).Milliseconds(),
			CostUSD:          0, // No cost for deduplicated embeddings
			TokensUsed:       0, // No tokens used
		}, nil
	}

	// Check cache if enabled (for in-memory caching)
	if s.cache != nil {
		cacheKey := s.generateCacheKey(req)
		if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			return &GenerateEmbeddingResponse{
				EmbeddingID:          uuid.New(),
				RequestID:            requestID,
				ModelUsed:            cached.Model,
				Provider:             cached.Provider,
				Dimensions:           cached.Dimensions,
				NormalizedDimensions: StandardDimension,
				Cached:               true,
				Metadata:             cached.Metadata,
				GenerationTimeMs:     time.Since(start).Milliseconds(),
			}, nil
		}
	}

	// Select model and provider using router or use defaults from request
	var routingDecision *RoutingDecision
	var modelSelection *ModelSelectionResult // Store for usage tracking
	var err error

	if agentConfig != nil {
		// Use agent config if available
		routingDecision, err = s.router.SelectProvider(ctx, &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    taskType,
			RequestID:   requestID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to select provider: %w", err)
		}
	} else {
		// Use model selector for intelligent model selection
		var agentID *uuid.UUID
		if req.AgentID != "" {
			id, err := uuid.Parse(req.AgentID)
			if err != nil {
				// Invalid agent ID format, continue without agent context
				// This allows non-UUID agent identifiers to be used without failing
				agentID = nil
			} else {
				agentID = &id
			}
		}

		taskTypeStr := string(taskType)
		modelSelection, err = s.modelSelector.SelectModel(ctx, ModelSelectionRequest{
			TenantID:       req.TenantID,
			AgentID:        agentID,
			TaskType:       &taskTypeStr,
			RequestedModel: &req.Model,
			TokenEstimate:  len(req.Text) / 4, // Rough estimate: 1 token per 4 chars
		})

		if err != nil {
			// Fall back to defaults if model selection fails
			provider := "bedrock"
			model := "amazon.titan-embed-text-v2:0"

			// Optional: Allow override only if it's a valid embedding model
			if req.Model != "" && strings.Contains(req.Model, "embed") {
				reqProvider, reqModel := s.parseModelString(req.Model)
				if reqProvider != "" {
					provider = reqProvider
				}
				if reqModel != "" {
					model = reqModel
				}
			}

			routingDecision = &RoutingDecision{
				Strategy: "fallback",
				Candidates: []ProviderCandidate{
					{
						Provider: provider,
						Model:    model,
					},
				},
			}
		} else {
			// Use the selected model
			routingDecision = &RoutingDecision{
				Strategy: "model_selector",
				Candidates: []ProviderCandidate{
					{
						Provider: modelSelection.Provider,
						Model:    modelSelection.ModelIdentifier,
					},
				},
			}
		}
	}

	// Generate embedding with selected provider using exponential backoff
	var embeddingResp *providers.EmbeddingResponse
	var lastErr error
	retryCount := 0

	for _, candidate := range routingDecision.Candidates {
		provider := s.providers[candidate.Provider]
		if provider == nil {
			continue
		}

		// Create exponential backoff strategy
		b := backoff.NewExponentialBackOff()
		b.InitialInterval = 500 * time.Millisecond
		b.RandomizationFactor = 0.5
		b.Multiplier = 1.5
		b.MaxInterval = 5 * time.Second
		b.MaxElapsedTime = 30 * time.Second
		b.Reset()

		// Wrap provider call in retry logic
		operation := func() error {
			providerStart := time.Now()
			resp, err := provider.GenerateEmbedding(ctx, providers.GenerateEmbeddingRequest{
				Text:      req.Text,
				Model:     candidate.Model,
				Metadata:  req.Metadata,
				RequestID: requestID,
			})
			providerLatency := time.Since(providerStart)

			if err != nil {
				// Check if error is retryable
				if isRetryableError(err) {
					return err
				}
				// Non-retryable error, return as permanent
				return backoff.Permanent(err)
			}

			embeddingResp = resp

			// Success - record metrics
			s.recordMetric(ctx, &EmbeddingMetric{
				ID:                uuid.New(),
				AgentID:           req.AgentID,
				ModelProvider:     candidate.Provider,
				ModelName:         candidate.Model,
				ModelDimensions:   embeddingResp.Dimensions,
				RequestID:         uuid.MustParse(requestID),
				TokenCount:        embeddingResp.TokensUsed,
				TotalLatencyMs:    int(time.Since(start).Milliseconds()),
				ProviderLatencyMs: int(providerLatency.Milliseconds()),
				CostUSD:           calculateCost(embeddingResp.TokensUsed, candidate.Model),
				Status:            "success",
				RetryCount:        retryCount,
				FinalProvider:     candidate.Provider,
				TenantID:          req.TenantID,
				Timestamp:         time.Now(),
			})
			return nil
		}

		// Execute with backoff
		lastErr = backoff.Retry(operation, b)

		if lastErr == nil {
			// Success with this provider
			break
		}

		// Failure - record and try next provider
		s.recordMetric(ctx, &EmbeddingMetric{
			ID:            uuid.New(),
			AgentID:       req.AgentID,
			ModelProvider: candidate.Provider,
			ModelName:     candidate.Model,
			RequestID:     uuid.MustParse(requestID),
			Status:        "failure",
			ErrorMessage:  lastErr.Error(),
			RetryCount:    retryCount,
			TenantID:      req.TenantID,
			Timestamp:     time.Now(),
		})

		retryCount++
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed: %w", lastErr)
	}

	// Track usage asynchronously if we have a model selector and model selection
	if s.modelSelector != nil && embeddingResp != nil && modelSelection != nil {
		go func() {
			var agentID *uuid.UUID
			if req.AgentID != "" {
				id, err := uuid.Parse(req.AgentID)
				if err == nil {
					agentID = &id
				}
				// If not a valid UUID, agentID remains nil (anonymous usage)
			}
			taskTypeStr := string(taskType)
			s.modelSelector.TrackUsage(
				context.Background(),
				req.TenantID,
				modelSelection.ModelID,
				agentID,
				embeddingResp.TokensUsed,
				int(time.Since(start).Milliseconds()),
				&taskTypeStr,
			)
		}()
	}

	// Normalize embedding to standard dimension
	normStart := time.Now()
	normalizedEmbedding := s.dimensionAdapter.Normalize(
		embeddingResp.Embedding,
		embeddingResp.Dimensions,
		StandardDimension,
	)
	normLatency := time.Since(normStart)

	// Store embedding
	// Add agent-specific metadata
	metadata := make(map[string]interface{})
	if req.Metadata != nil {
		metadata = req.Metadata
	}
	metadata["agent_id"] = req.AgentID
	metadata["task_type"] = taskType
	metadata["normalized_embedding"] = normalizedEmbedding
	metadata["cost_usd"] = calculateCost(embeddingResp.TokensUsed, embeddingResp.Model)
	metadata["generation_time_ms"] = time.Since(start).Milliseconds()

	insertReq := InsertRequest{
		ContextID:            req.ContextID, // Now properly nullable
		Content:              req.Text,
		Embedding:            embeddingResp.Embedding,
		ModelName:            embeddingResp.Model,
		TenantID:             req.TenantID,
		Metadata:             json.RawMessage(mustMarshalJSON(metadata)),
		ConfiguredDimensions: nil, // Only set when actually using dimension reduction
	}

	embeddingID, err := s.repository.InsertEmbedding(ctx, insertReq)
	if err != nil {
		return nil, fmt.Errorf("failed to store embedding: %w", err)
	}

	// Update cache
	if s.cache != nil {
		cacheKey := s.generateCacheKey(req)
		_ = s.cache.Set(ctx, cacheKey, &CachedEmbedding{
			Embedding:  embeddingResp.Embedding,
			Model:      embeddingResp.Model,
			Provider:   embeddingResp.ProviderInfo.Provider,
			Dimensions: embeddingResp.Dimensions,
			Metadata:   metadata,
			CachedAt:   time.Now(),
		}, 1*time.Hour) // Cache errors are non-critical
	}

	// Update final metrics with normalization time
	s.recordMetric(ctx, &EmbeddingMetric{
		ID:                     uuid.New(),
		AgentID:                req.AgentID,
		ModelProvider:          embeddingResp.ProviderInfo.Provider,
		ModelName:              embeddingResp.Model,
		NormalizationLatencyMs: int(normLatency.Milliseconds()),
		TenantID:               req.TenantID,
		Timestamp:              time.Now(),
	})

	return &GenerateEmbeddingResponse{
		EmbeddingID:          embeddingID,
		RequestID:            requestID,
		ModelUsed:            embeddingResp.Model,
		Provider:             embeddingResp.ProviderInfo.Provider,
		Dimensions:           embeddingResp.Dimensions,
		NormalizedDimensions: StandardDimension,
		CostUSD:              calculateCost(embeddingResp.TokensUsed, embeddingResp.Model),
		TokensUsed:           embeddingResp.TokensUsed,
		GenerationTimeMs:     time.Since(start).Milliseconds(),
		Cached:               false,
		Metadata:             metadata,
	}, nil
}

// BatchGenerateEmbeddings generates embeddings for multiple texts
func (s *ServiceV2) BatchGenerateEmbeddings(ctx context.Context, reqs []GenerateEmbeddingRequest) ([]*GenerateEmbeddingResponse, error) {
	if len(reqs) == 0 {
		return []*GenerateEmbeddingResponse{}, nil
	}

	// Group requests by agent and task type for optimal batching
	type batchKey struct {
		agentID  string
		taskType agents.TaskType
	}
	batches := make(map[batchKey][]int)

	for i, req := range reqs {
		taskType := req.TaskType
		if taskType == "" {
			taskType = agents.TaskTypeGeneralQA
		}
		key := batchKey{
			agentID:  req.AgentID,
			taskType: taskType,
		}
		batches[key] = append(batches[key], i)
	}

	// Process each batch concurrently
	responses := make([]*GenerateEmbeddingResponse, len(reqs))
	var wg sync.WaitGroup
	errCh := make(chan error, len(batches))

	for key, indices := range batches {
		wg.Add(1)
		go func(bk batchKey, reqIndices []int) {
			defer wg.Done()

			// Get agent configuration
			agentConfig, err := s.agentService.GetConfig(ctx, bk.agentID)
			if err != nil {
				errCh <- fmt.Errorf("failed to get config for agent %s: %w", bk.agentID, err)
				return
			}

			// Get models for this agent and task type
			primaryModels, _, err := s.agentService.GetModelsForAgent(ctx, bk.agentID, bk.taskType)
			if err != nil {
				errCh <- fmt.Errorf("failed to get models for agent %s: %w", bk.agentID, err)
				return
			}

			if len(primaryModels) == 0 {
				errCh <- fmt.Errorf("no models configured for agent %s task %s", bk.agentID, bk.taskType)
				return
			}

			// Find a provider that supports the model
			modelName := primaryModels[0]
			var provider providers.Provider
			var providerName string

			for pName, p := range s.providers {
				models := p.GetSupportedModels()
				for _, m := range models {
					if m.Name == modelName {
						provider = p
						providerName = pName
						break
					}
				}
				if provider != nil {
					break
				}
			}

			if provider == nil {
				errCh <- fmt.Errorf("no provider found for model %s", modelName)
				return
			}

			// Extract texts for this batch
			texts := make([]string, len(reqIndices))
			for i, idx := range reqIndices {
				texts[i] = reqs[idx].Text
			}

			// Create batch request
			batchReq := providers.BatchGenerateEmbeddingRequest{
				Texts:     texts,
				Model:     modelName,
				Metadata:  agentConfig.Metadata,
				RequestID: uuid.New().String(),
			}

			// Generate embeddings
			start := time.Now()
			batchResp, err := provider.BatchGenerateEmbeddings(ctx, batchReq)
			if err != nil {
				// Fall back to sequential processing
				for _, idx := range reqIndices {
					resp, err := s.GenerateEmbedding(ctx, reqs[idx])
					if err != nil {
						errCh <- fmt.Errorf("failed to generate embedding for request %d: %w", idx, err)
						return
					}
					responses[idx] = resp
				}
				return
			}

			generationTime := time.Since(start).Milliseconds()

			// Process responses
			for i, idx := range reqIndices {
				if i >= len(batchResp.Embeddings) {
					errCh <- fmt.Errorf("missing embedding for request %d", idx)
					return
				}

				// Convert metadata to JSON
				metadataJSON, err := json.Marshal(reqs[idx].Metadata)
				if err != nil {
					errCh <- fmt.Errorf("failed to marshal metadata for request %d: %w", idx, err)
					return
				}

				// Store embedding
				dims := batchResp.Dimensions
				insertReq := InsertRequest{
					ContextID:            reqs[idx].ContextID,
					Content:              texts[i],
					Embedding:            batchResp.Embeddings[i],
					ModelName:            modelName,
					TenantID:             reqs[idx].TenantID,
					Metadata:             json.RawMessage(metadataJSON),
					ContentIndex:         idx,
					ChunkIndex:           0,
					ConfiguredDimensions: &dims,
				}

				embeddingID, err := s.repository.InsertEmbedding(ctx, insertReq)
				if err != nil {
					errCh <- fmt.Errorf("failed to store embedding %d: %w", idx, err)
					return
				}

				// Create response
				responses[idx] = &GenerateEmbeddingResponse{
					EmbeddingID:          embeddingID,
					RequestID:            reqs[idx].RequestID,
					ModelUsed:            modelName,
					Provider:             providerName,
					Dimensions:           batchResp.Dimensions,
					NormalizedDimensions: StandardDimension,
					GenerationTimeMs:     generationTime / int64(len(reqIndices)),
					Cached:               false,
					Metadata:             reqs[idx].Metadata,
				}
			}
		}(key, indices)
	}

	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("batch processing failed with %d errors: %v", len(errs), errs)
	}

	return responses, nil
}

// GenerateBatch generates embeddings for multiple texts with progress tracking
func (s *ServiceV2) GenerateBatch(ctx context.Context, texts []string, model string) ([][]float32, error) {
	const batchSize = 100

	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Use default model if not specified
	if model == "" {
		// Find first available model
		for _, provider := range s.providers {
			models := provider.GetSupportedModels()
			if len(models) > 0 && models[0].IsActive {
				model = models[0].Name
				break
			}
		}
		if model == "" {
			return nil, fmt.Errorf("no active models available")
		}
	}

	// Find provider that supports the model
	var provider providers.Provider
	var providerName string
	for name, p := range s.providers {
		models := p.GetSupportedModels()
		for _, m := range models {
			if m.Name == model && m.IsActive {
				provider = p
				providerName = name
				break
			}
		}
		if provider != nil {
			break
		}
	}

	if provider == nil {
		return nil, fmt.Errorf("no provider found for model %s", model)
	}

	// Process in batches
	var results [][]float32
	totalBatches := (len(texts) + batchSize - 1) / batchSize

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		// Create batch request
		batchReq := providers.BatchGenerateEmbeddingRequest{
			Texts:     batch,
			Model:     model,
			RequestID: uuid.New().String(),
		}

		// Generate embeddings with retry using circuit breaker pattern
		var embeddings [][]float32
		err := s.generateWithRetry(ctx, func() error {
			// Record metrics
			start := time.Now()
			defer func() {
				if s.metricsRepo != nil {
					s.recordMetric(ctx, &EmbeddingMetric{
						ID:                uuid.New(),
						ModelProvider:     providerName,
						ModelName:         model,
						TokenCount:        len(batch),
						TotalLatencyMs:    int(time.Since(start).Milliseconds()),
						ProviderLatencyMs: int(time.Since(start).Milliseconds()),
						Status:            "success",
						Timestamp:         time.Now(),
					})
				}
			}()

			// Call provider
			resp, err := provider.BatchGenerateEmbeddings(ctx, batchReq)
			if err != nil {
				// Record error metric
				if s.metricsRepo != nil {
					s.recordMetric(ctx, &EmbeddingMetric{
						ID:            uuid.New(),
						ModelProvider: providerName,
						ModelName:     model,
						TokenCount:    len(batch),
						Status:        "error",
						ErrorMessage:  err.Error(),
						Timestamp:     time.Now(),
					})
				}
				return err
			}

			embeddings = resp.Embeddings
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("batch %d/%d failed: %w", (i/batchSize)+1, totalBatches, err)
		}

		results = append(results, embeddings...)

		// Progress callback if available
		if s.progressFunc != nil {
			progress := float64(end) / float64(len(texts))
			s.progressFunc(progress)
		}

		// Add small delay between batches to avoid rate limiting
		if end < len(texts) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return results, nil
}

// generateWithRetry implements retry logic with exponential backoff
func (s *ServiceV2) generateWithRetry(ctx context.Context, fn func() error) error {
	const maxRetries = 3
	baseDelay := 1 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err

			// Check if error is retryable
			if provErr, ok := err.(*providers.ProviderError); ok && !provErr.IsRetryable {
				return err
			}

			// Calculate backoff delay
			delay := baseDelay * time.Duration(1<<uint(attempt))
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// SetProgressCallback sets the progress callback function
func (s *ServiceV2) SetProgressCallback(fn func(float64)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progressFunc = fn
}

// GetProviderHealth returns health status of all providers
func (s *ServiceV2) GetProviderHealth(ctx context.Context) map[string]ProviderHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	health := make(map[string]ProviderHealth)

	for name, provider := range s.providers {
		status := ProviderHealth{
			Name:   name,
			Status: "healthy",
		}

		if err := provider.HealthCheck(ctx); err != nil {
			status.Status = "unhealthy"
			status.Error = err.Error()
		}

		// Get circuit breaker status from router
		if cbStatus := s.router.GetCircuitBreakerStatus(name); cbStatus != nil {
			status.CircuitBreakerState = cbStatus.State
			status.FailureCount = cbStatus.FailureCount
		}

		health[name] = status
	}

	return health
}

// Private helper methods

func (s *ServiceV2) generateCacheKey(req GenerateEmbeddingRequest) string {
	// Create a deterministic cache key
	return fmt.Sprintf("embedding:%s:%s:%s", req.AgentID, req.TaskType, hashString(req.Text))
}

func (s *ServiceV2) recordMetric(ctx context.Context, metric *EmbeddingMetric) {
	if s.metricsRepo != nil {
		go func() {
			// In production, metrics errors should be logged but not fail the operation
			_ = s.metricsRepo.RecordMetric(context.Background(), metric)
		}()
	}
}

// validateEmbeddingRequest validates the embedding request for security
func (s *ServiceV2) validateEmbeddingRequest(req GenerateEmbeddingRequest) error {
	// Validate agent ID
	if req.AgentID == "" {
		return fmt.Errorf("agent ID is required")
	}
	if len(req.AgentID) > 255 {
		return fmt.Errorf("agent ID too long")
	}

	// Validate text
	if req.Text == "" {
		return fmt.Errorf("text is required")
	}
	const maxTextLength = 1_000_000 // 1MB max text size
	if len(req.Text) > maxTextLength {
		return fmt.Errorf("text exceeds maximum length of %d characters", maxTextLength)
	}

	// Validate tenant ID if provided
	// UUID validation is implicit in the type when TenantID != uuid.Nil

	// Validate metadata if provided
	if req.Metadata != nil {
		// Ensure metadata doesn't contain sensitive keys
		sensitiveKeys := []string{"password", "secret", "token", "apikey", "api_key"}
		for key := range req.Metadata {
			lowerKey := strings.ToLower(key)
			for _, sensitive := range sensitiveKeys {
				if strings.Contains(lowerKey, sensitive) {
					return fmt.Errorf("metadata contains potentially sensitive key: %s", key)
				}
			}
		}
	}

	return nil
}

func calculateCost(tokens int, model string) float64 {
	// Cost calculation based on model
	costPer1MTokens := map[string]float64{
		"text-embedding-3-small":       0.02,
		"text-embedding-3-large":       0.13,
		"text-embedding-ada-002":       0.10,
		"amazon.titan-embed-text-v2:0": 0.02,
		"voyage-code-2":                0.10,
	}

	if cost, ok := costPer1MTokens[model]; ok {
		return float64(tokens) * cost / 1_000_000
	}

	// Default cost
	return float64(tokens) * 0.05 / 1_000_000
}

func mustMarshalJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

func hashString(s string) string {
	// Simple hash for cache key
	// In production, use a proper hash function
	return fmt.Sprintf("%x", s[:min(len(s), 32)])
}

func calculateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable error patterns
	errStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"timeout",
		"connection",
		"temporary",
		"unavailable",
		"throttled",
		"rate limit",
		"too many requests",
		"429",
		"502",
		"503",
		"504",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Check if error implements a Temporary() method
	type temporary interface {
		Temporary() bool
	}
	if te, ok := err.(temporary); ok {
		return te.Temporary()
	}

	return false
}

// Types for health and metrics

type ProviderHealth struct {
	Name                string `json:"name"`
	Status              string `json:"status"`
	Error               string `json:"error,omitempty"`
	CircuitBreakerState string `json:"circuit_breaker_state,omitempty"`
	FailureCount        int    `json:"failure_count,omitempty"`
}

type MetricsFilter struct {
	AgentID   string
	Provider  string
	StartTime time.Time
	EndTime   time.Time
	Status    string
	Limit     int
}

type CostSummary struct {
	AgentID      string             `json:"agent_id"`
	Period       string             `json:"period"`
	TotalCostUSD float64            `json:"total_cost_usd"`
	ByProvider   map[string]float64 `json:"by_provider"`
	ByModel      map[string]float64 `json:"by_model"`
	RequestCount int                `json:"request_count"`
	TokensUsed   int                `json:"tokens_used"`
}

const StandardDimension = 1536 // OpenAI standard for cross-model compatibility

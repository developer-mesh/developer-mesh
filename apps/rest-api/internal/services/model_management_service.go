package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/embedding_usage"
	"github.com/developer-mesh/developer-mesh/pkg/repository/model_catalog"
	"github.com/developer-mesh/developer-mesh/pkg/repository/tenant_models"
)

// ModelSelectionRequest represents a request for model selection
type ModelSelectionRequest struct {
	TenantID       uuid.UUID  `json:"tenant_id"`
	AgentID        *uuid.UUID `json:"agent_id,omitempty"`
	TaskType       *string    `json:"task_type,omitempty"`
	RequestedModel *string    `json:"requested_model,omitempty"`
	TokenEstimate  int        `json:"token_estimate,omitempty"`
}

// ModelSelectionResponse represents the selected model and its details
type ModelSelectionResponse struct {
	ModelID              uuid.UUID              `json:"model_id"`
	ModelIdentifier      string                 `json:"model_identifier"`
	Provider             string                 `json:"provider"`
	Dimensions           int                    `json:"dimensions"`
	CostPerMillionTokens float64                `json:"cost_per_million_tokens"`
	EstimatedCost        float64                `json:"estimated_cost,omitempty"`
	IsWithinQuota        bool                   `json:"is_within_quota"`
	QuotaRemaining       *int64                 `json:"quota_remaining,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

// ModelFilter defines filtering options for model listing
type ModelFilter struct {
	Provider      *string
	ModelType     *string
	AvailableOnly bool
	Offset        int
	Limit         int
}

// CreateModelRequest for creating a new model
type CreateModelRequest struct {
	Provider             string                 `json:"provider"`
	ModelName            string                 `json:"model_name"`
	ModelID              string                 `json:"model_id"`
	ModelVersion         string                 `json:"model_version,omitempty"`
	Dimensions           int                    `json:"dimensions"`
	MaxTokens            int                    `json:"max_tokens,omitempty"`
	CostPerMillionTokens float64                `json:"cost_per_million_tokens,omitempty"`
	ModelType            string                 `json:"model_type,omitempty"`
	Capabilities         map[string]interface{} `json:"capabilities,omitempty"`
}

// UpdateModelRequest for updating a model
type UpdateModelRequest struct {
	IsAvailable          *bool                  `json:"is_available,omitempty"`
	IsDeprecated         *bool                  `json:"is_deprecated,omitempty"`
	CostPerMillionTokens *float64               `json:"cost_per_million_tokens,omitempty"`
	Capabilities         map[string]interface{} `json:"capabilities,omitempty"`
}

// ConfigureTenantModelRequest for configuring a model for a tenant
type ConfigureTenantModelRequest struct {
	ModelID             uuid.UUID `json:"model_id"`
	IsEnabled           bool      `json:"is_enabled"`
	IsDefault           bool      `json:"is_default"`
	MonthlyTokenLimit   *int64    `json:"monthly_token_limit,omitempty"`
	DailyTokenLimit     *int64    `json:"daily_token_limit,omitempty"`
	MonthlyRequestLimit *int      `json:"monthly_request_limit,omitempty"`
	Priority            int       `json:"priority"`
}

// UpdateTenantModelRequest for updating tenant model configuration
type UpdateTenantModelRequest struct {
	IsEnabled           *bool  `json:"is_enabled,omitempty"`
	MonthlyTokenLimit   *int64 `json:"monthly_token_limit,omitempty"`
	DailyTokenLimit     *int64 `json:"daily_token_limit,omitempty"`
	MonthlyRequestLimit *int   `json:"monthly_request_limit,omitempty"`
	Priority            *int   `json:"priority,omitempty"`
}

// ModelManagementService defines the interface for model management operations
type ModelManagementService interface {
	// Model catalog operations
	ListModels(ctx context.Context, filter *ModelFilter) ([]*model_catalog.EmbeddingModel, int, error)
	GetModel(ctx context.Context, id uuid.UUID) (*model_catalog.EmbeddingModel, error)
	CreateModel(ctx context.Context, req *CreateModelRequest) (*model_catalog.EmbeddingModel, error)
	UpdateModel(ctx context.Context, id uuid.UUID, req *UpdateModelRequest) (*model_catalog.EmbeddingModel, error)
	DeleteModel(ctx context.Context, id uuid.UUID) error
	ListProviders(ctx context.Context) ([]string, error)

	// Tenant model operations
	ListTenantModels(ctx context.Context, tenantID uuid.UUID, enabledOnly bool) ([]*tenant_models.TenantEmbeddingModel, error)
	ConfigureTenantModel(ctx context.Context, tenantID uuid.UUID, req *ConfigureTenantModelRequest) (*tenant_models.TenantEmbeddingModel, error)
	UpdateTenantModel(ctx context.Context, tenantID, modelID uuid.UUID, req *UpdateTenantModelRequest) (*tenant_models.TenantEmbeddingModel, error)
	RemoveTenantModel(ctx context.Context, tenantID, modelID uuid.UUID) error
	SetDefaultModel(ctx context.Context, tenantID, modelID uuid.UUID) error
	GetUsageStats(ctx context.Context, tenantID uuid.UUID, modelID *uuid.UUID, period string) (map[string]interface{}, error)
	GetTenantQuotas(ctx context.Context, tenantID uuid.UUID) (map[string]interface{}, error)

	// Model selection operations
	SelectModelForRequest(ctx context.Context, req *ModelSelectionRequest) (*ModelSelectionResponse, error)
	TrackUsage(ctx context.Context, tenantID, modelID uuid.UUID, agentID *uuid.UUID, tokens int, latencyMs int, taskType *string)
	CheckQuota(ctx context.Context, tenantID, modelID uuid.UUID, tokenEstimate int) (bool, error)

	// Legacy operations
	GetTenantModels(ctx context.Context, tenantID uuid.UUID) ([]*tenant_models.TenantEmbeddingModel, error)
	EnableModelForTenant(ctx context.Context, tenantID, modelID uuid.UUID) error
	GetUsageSummary(ctx context.Context, tenantID uuid.UUID, period string) (*embedding_usage.UsageSummary, error)
	RefreshModelCache(ctx context.Context) error
}

// ModelManagementServiceImpl implements ModelManagementService
type ModelManagementServiceImpl struct {
	db          *sqlx.DB
	redisClient *redis.Client
	cache       *cache.RedisCache
	catalogRepo model_catalog.ModelCatalogRepository
	tenantRepo  tenant_models.TenantModelsRepository
	usageRepo   embedding_usage.EmbeddingUsageRepository
	logger      observability.Logger
	cacheTTL    time.Duration
}

// NewModelManagementService creates a new ModelManagementService instance
func NewModelManagementService(
	db *sqlx.DB,
	redisClient *redis.Client,
	catalogRepo model_catalog.ModelCatalogRepository,
	tenantRepo tenant_models.TenantModelsRepository,
	usageRepo embedding_usage.EmbeddingUsageRepository,
) (ModelManagementService, error) {
	redisCache, err := cache.NewRedisCache(cache.RedisConfig{
		Address:  redisClient.Options().Addr,
		Password: redisClient.Options().Password,
		Database: redisClient.Options().DB,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create redis cache: %w", err)
	}

	return &ModelManagementServiceImpl{
		db:          db,
		redisClient: redisClient,
		cache:       redisCache,
		catalogRepo: catalogRepo,
		tenantRepo:  tenantRepo,
		usageRepo:   usageRepo,
		logger:      observability.NewStandardLogger("model_management_service"),
		cacheTTL:    5 * time.Minute,
	}, nil
}

// SelectModelForRequest selects the best available model for a request
func (s *ModelManagementServiceImpl) SelectModelForRequest(ctx context.Context, req *ModelSelectionRequest) (*ModelSelectionResponse, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("model_selection:%s:%v:%v:%v",
		req.TenantID, req.AgentID, req.TaskType, req.RequestedModel)

	var cachedResponse ModelSelectionResponse
	err := s.cache.Get(ctx, cacheKey, &cachedResponse)
	if err == nil {
		s.logger.Debug("Model selection cache hit", map[string]interface{}{
			"tenant_id": req.TenantID,
			"cache_key": cacheKey,
		})

		// Still check quota for cached response
		if req.TokenEstimate > 0 {
			isWithinQuota, err := s.CheckQuota(ctx, req.TenantID, cachedResponse.ModelID, req.TokenEstimate)
			if err != nil {
				s.logger.Warn("Failed to check quota", map[string]interface{}{
					"error": err.Error(),
				})
			}
			cachedResponse.IsWithinQuota = isWithinQuota
		}

		return &cachedResponse, nil
	}

	// Get model from database using the SQL function
	selection, err := s.tenantRepo.GetModelForRequest(ctx, req.TenantID, req.AgentID, req.TaskType, req.RequestedModel)
	if err != nil {
		s.logger.Error("Failed to select model", map[string]interface{}{
			"tenant_id": req.TenantID,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to select model: %w", err)
	}

	response := &ModelSelectionResponse{
		ModelID:              selection.ModelID,
		ModelIdentifier:      selection.ModelIdentifier,
		Provider:             selection.Provider,
		Dimensions:           selection.Dimensions,
		CostPerMillionTokens: selection.CostPerMillionTokens,
		IsWithinQuota:        true,
	}

	// Calculate estimated cost if token estimate provided
	if req.TokenEstimate > 0 {
		response.EstimatedCost = (float64(req.TokenEstimate) / 1000000) * selection.CostPerMillionTokens

		// Check quota
		isWithinQuota, err := s.CheckQuota(ctx, req.TenantID, selection.ModelID, req.TokenEstimate)
		if err != nil {
			s.logger.Warn("Failed to check quota", map[string]interface{}{
				"error": err.Error(),
			})
		}
		response.IsWithinQuota = isWithinQuota

		// Get remaining quota
		status, err := s.tenantRepo.CheckUsageLimits(ctx, req.TenantID, selection.ModelID)
		if err == nil && status.MonthlyTokenLimit > 0 {
			remaining := status.MonthlyTokenLimit - status.MonthlyTokensUsed
			response.QuotaRemaining = &remaining
		}
	}

	// Add metadata
	response.Metadata = map[string]interface{}{
		"is_default":  selection.IsDefault,
		"priority":    selection.Priority,
		"selected_at": time.Now().UTC(),
	}

	// Cache the response
	err = s.cache.Set(ctx, cacheKey, response, s.cacheTTL)
	if err != nil {
		s.logger.Warn("Failed to cache model selection", map[string]interface{}{
			"error": err.Error(),
		})
	}

	s.logger.Info("Model selected", map[string]interface{}{
		"tenant_id":        req.TenantID,
		"model_id":         response.ModelID,
		"model_identifier": response.ModelIdentifier,
	})

	return response, nil
}

// TrackUsage records embedding usage asynchronously
func (s *ModelManagementServiceImpl) TrackUsage(ctx context.Context, tenantID, modelID uuid.UUID, agentID *uuid.UUID, tokens int, latencyMs int, taskType *string) {
	// Run in goroutine for async tracking
	go func() {
		// Create new context with timeout for async operation
		trackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		record := &embedding_usage.UsageRecord{
			TenantID:            tenantID,
			AgentID:             agentID,
			ModelID:             modelID,
			TokensUsed:          tokens,
			EmbeddingsGenerated: 1,
			TaskType:            taskType,
			CreatedAt:           time.Now(),
		}

		if latencyMs > 0 {
			record.LatencyMs = &latencyMs
		}

		err := s.usageRepo.TrackUsage(trackCtx, record)
		if err != nil {
			s.logger.Error("Failed to track usage", map[string]interface{}{
				"tenant_id": tenantID,
				"model_id":  modelID,
				"error":     err.Error(),
			})
		} else {
			s.logger.Debug("Usage tracked", map[string]interface{}{
				"tenant_id": tenantID,
				"model_id":  modelID,
				"tokens":    tokens,
			})
		}

		// Invalidate quota cache
		quotaCacheKey := fmt.Sprintf("quota:%s:%s", tenantID, modelID)
		_ = s.cache.Delete(trackCtx, quotaCacheKey)
	}()
}

// CheckQuota verifies if tenant has quota available
func (s *ModelManagementServiceImpl) CheckQuota(ctx context.Context, tenantID, modelID uuid.UUID, tokenEstimate int) (bool, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("quota:%s:%s", tenantID, modelID)

	var status tenant_models.UsageStatus
	err := s.cache.Get(ctx, cacheKey, &status)
	if err == nil {
		// Check if estimate would exceed limits
		if status.MonthlyTokenLimit > 0 && status.MonthlyTokensUsed+int64(tokenEstimate) > status.MonthlyTokenLimit {
			return false, nil
		}
		if status.DailyTokenLimit > 0 && status.DailyTokensUsed+int64(tokenEstimate) > status.DailyTokenLimit {
			return false, nil
		}
		return true, nil
	}

	// Get from database
	statusPtr, err := s.tenantRepo.CheckUsageLimits(ctx, tenantID, modelID)
	if err != nil {
		// Log error but don't block - assume quota is available
		s.logger.Warn("Failed to check usage limits", map[string]interface{}{
			"tenant_id": tenantID,
			"model_id":  modelID,
			"error":     err.Error(),
		})
		return true, nil
	}

	// Cache the status
	_ = s.cache.Set(ctx, cacheKey, *statusPtr, 1*time.Minute)

	// Check if estimate would exceed limits
	if statusPtr.MonthlyTokenLimit > 0 && statusPtr.MonthlyTokensUsed+int64(tokenEstimate) > statusPtr.MonthlyTokenLimit {
		return false, nil
	}
	if statusPtr.DailyTokenLimit > 0 && statusPtr.DailyTokensUsed+int64(tokenEstimate) > statusPtr.DailyTokenLimit {
		return false, nil
	}

	return statusPtr.IsWithinLimits, nil
}

// GetTenantModels returns all models available to a tenant
func (s *ModelManagementServiceImpl) GetTenantModels(ctx context.Context, tenantID uuid.UUID) ([]*tenant_models.TenantEmbeddingModel, error) {
	// Check cache
	cacheKey := fmt.Sprintf("tenant_models:%s", tenantID)

	var cachedModels []*tenant_models.TenantEmbeddingModel
	cacheData, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		err = json.Unmarshal([]byte(cacheData), &cachedModels)
		if err == nil {
			return cachedModels, nil
		}
	}

	// Get from database
	models, err := s.tenantRepo.ListTenantModels(ctx, tenantID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant models: %w", err)
	}

	// Cache the result
	if data, err := json.Marshal(models); err == nil {
		_ = s.redisClient.Set(ctx, cacheKey, data, s.cacheTTL).Err()
	}

	return models, nil
}

// EnableModelForTenant enables a model for a tenant with default settings
func (s *ModelManagementServiceImpl) EnableModelForTenant(ctx context.Context, tenantID, modelID uuid.UUID) error {
	// Check if model exists in catalog
	model, err := s.catalogRepo.GetByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("failed to get model from catalog: %w", err)
	}

	if !model.IsAvailable {
		return fmt.Errorf("model %s is not available", model.ModelID)
	}

	// Check if already enabled
	existing, err := s.tenantRepo.GetTenantModel(ctx, tenantID, modelID)
	if err == nil && existing != nil {
		if existing.IsEnabled {
			return nil // Already enabled
		}
		// Just update to enable
		existing.IsEnabled = true
		return s.tenantRepo.UpdateTenantModel(ctx, existing)
	}

	// Create new tenant model configuration
	tenantModel := &tenant_models.TenantEmbeddingModel{
		TenantID:  tenantID,
		ModelID:   modelID,
		IsEnabled: true,
		IsDefault: false,
		Priority:  100,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.tenantRepo.CreateTenantModel(ctx, tenantModel)
	if err != nil {
		return fmt.Errorf("failed to enable model for tenant: %w", err)
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("tenant_models:%s", tenantID)
	_ = s.redisClient.Del(ctx, cacheKey).Err()

	s.logger.Info("Model enabled for tenant", map[string]interface{}{
		"tenant_id": tenantID,
		"model_id":  modelID,
	})

	return nil
}

// GetUsageSummary returns usage summary for a tenant
func (s *ModelManagementServiceImpl) GetUsageSummary(ctx context.Context, tenantID uuid.UUID, period string) (*embedding_usage.UsageSummary, error) {
	switch period {
	case "day", "daily":
		return s.usageRepo.GetCurrentDayUsage(ctx, tenantID)
	case "month", "monthly":
		return s.usageRepo.GetCurrentMonthUsage(ctx, tenantID)
	case "week", "weekly":
		now := time.Now()
		weekStart := now.AddDate(0, 0, -int(now.Weekday()))
		weekEnd := weekStart.AddDate(0, 0, 7)
		return s.usageRepo.GetUsageSummary(ctx, tenantID, weekStart, weekEnd)
	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}
}

// RefreshModelCache refreshes the model cache
func (s *ModelManagementServiceImpl) RefreshModelCache(ctx context.Context) error {
	// Clear all model-related cache keys
	pattern := "model_selection:*"
	iter := s.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		_ = s.redisClient.Del(ctx, iter.Val()).Err()
	}

	pattern = "tenant_models:*"
	iter = s.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		_ = s.redisClient.Del(ctx, iter.Val()).Err()
	}

	s.logger.Info("Model cache refreshed", nil)
	return nil
}

// ListModels lists models from the catalog
func (s *ModelManagementServiceImpl) ListModels(ctx context.Context, filter *ModelFilter) ([]*model_catalog.EmbeddingModel, int, error) {
	catalogFilter := &model_catalog.ModelFilter{
		Provider:    filter.Provider,
		ModelType:   filter.ModelType,
		IsAvailable: &filter.AvailableOnly,
	}

	models, err := s.catalogRepo.ListAll(ctx, catalogFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list models: %w", err)
	}

	total := len(models)

	// Apply pagination
	start := filter.Offset
	end := filter.Offset + filter.Limit
	if end > total {
		end = total
	}
	if start > total {
		start = total
	}

	return models[start:end], total, nil
}

// GetModel retrieves a model by ID
func (s *ModelManagementServiceImpl) GetModel(ctx context.Context, id uuid.UUID) (*model_catalog.EmbeddingModel, error) {
	return s.catalogRepo.GetByID(ctx, id)
}

// CreateModel creates a new model in the catalog
func (s *ModelManagementServiceImpl) CreateModel(ctx context.Context, req *CreateModelRequest) (*model_catalog.EmbeddingModel, error) {
	capabilities, _ := json.Marshal(req.Capabilities)

	model := &model_catalog.EmbeddingModel{
		Provider:             req.Provider,
		ModelName:            req.ModelName,
		ModelID:              req.ModelID,
		ModelVersion:         &req.ModelVersion,
		Dimensions:           req.Dimensions,
		MaxTokens:            &req.MaxTokens,
		CostPerMillionTokens: &req.CostPerMillionTokens,
		ModelType:            req.ModelType,
		Capabilities:         capabilities,
		IsAvailable:          true,
		IsDeprecated:         false,
	}

	err := s.catalogRepo.Create(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	return model, nil
}

// UpdateModel updates a model in the catalog
func (s *ModelManagementServiceImpl) UpdateModel(ctx context.Context, id uuid.UUID, req *UpdateModelRequest) (*model_catalog.EmbeddingModel, error) {
	model, err := s.catalogRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	if req.IsAvailable != nil {
		model.IsAvailable = *req.IsAvailable
	}
	if req.IsDeprecated != nil {
		model.IsDeprecated = *req.IsDeprecated
	}
	if req.CostPerMillionTokens != nil {
		model.CostPerMillionTokens = req.CostPerMillionTokens
	}
	if req.Capabilities != nil {
		capabilities, _ := json.Marshal(req.Capabilities)
		model.Capabilities = capabilities
	}

	err = s.catalogRepo.Update(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("failed to update model: %w", err)
	}

	return model, nil
}

// DeleteModel deletes a model from the catalog
func (s *ModelManagementServiceImpl) DeleteModel(ctx context.Context, id uuid.UUID) error {
	return s.catalogRepo.Delete(ctx, id)
}

// ListProviders lists available providers
func (s *ModelManagementServiceImpl) ListProviders(ctx context.Context) ([]string, error) {
	return s.catalogRepo.GetProviders(ctx)
}

// ListTenantModels lists models configured for a tenant
func (s *ModelManagementServiceImpl) ListTenantModels(ctx context.Context, tenantID uuid.UUID, enabledOnly bool) ([]*tenant_models.TenantEmbeddingModel, error) {
	return s.tenantRepo.ListTenantModels(ctx, tenantID, enabledOnly)
}

// ConfigureTenantModel configures a model for a tenant
func (s *ModelManagementServiceImpl) ConfigureTenantModel(ctx context.Context, tenantID uuid.UUID, req *ConfigureTenantModelRequest) (*tenant_models.TenantEmbeddingModel, error) {
	model := &tenant_models.TenantEmbeddingModel{
		TenantID:            tenantID,
		ModelID:             req.ModelID,
		IsEnabled:           req.IsEnabled,
		IsDefault:           req.IsDefault,
		MonthlyTokenLimit:   req.MonthlyTokenLimit,
		DailyTokenLimit:     req.DailyTokenLimit,
		MonthlyRequestLimit: req.MonthlyRequestLimit,
		Priority:            req.Priority,
		CreatedBy:           func() *uuid.UUID { u := uuid.New(); return &u }(), // Should be extracted from context
	}

	err := s.tenantRepo.CreateTenantModel(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("failed to configure tenant model: %w", err)
	}

	// Clear cache for this tenant
	s.clearTenantCache(ctx, tenantID)

	return model, nil
}

// UpdateTenantModel updates a tenant's model configuration
func (s *ModelManagementServiceImpl) UpdateTenantModel(ctx context.Context, tenantID, modelID uuid.UUID, req *UpdateTenantModelRequest) (*tenant_models.TenantEmbeddingModel, error) {
	model, err := s.tenantRepo.GetTenantModel(ctx, tenantID, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant model: %w", err)
	}

	if req.IsEnabled != nil {
		model.IsEnabled = *req.IsEnabled
	}
	if req.MonthlyTokenLimit != nil {
		model.MonthlyTokenLimit = req.MonthlyTokenLimit
	}
	if req.DailyTokenLimit != nil {
		model.DailyTokenLimit = req.DailyTokenLimit
	}
	if req.MonthlyRequestLimit != nil {
		model.MonthlyRequestLimit = req.MonthlyRequestLimit
	}
	if req.Priority != nil {
		model.Priority = *req.Priority
	}

	err = s.tenantRepo.UpdateTenantModel(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("failed to update tenant model: %w", err)
	}

	// Clear cache for this tenant
	s.clearTenantCache(ctx, tenantID)

	return model, nil
}

// RemoveTenantModel removes a model configuration for a tenant
func (s *ModelManagementServiceImpl) RemoveTenantModel(ctx context.Context, tenantID, modelID uuid.UUID) error {
	err := s.tenantRepo.DeleteTenantModel(ctx, tenantID, modelID)
	if err != nil {
		return fmt.Errorf("failed to remove tenant model: %w", err)
	}

	// Clear cache for this tenant
	s.clearTenantCache(ctx, tenantID)

	return nil
}

// SetDefaultModel sets a model as the default for a tenant
func (s *ModelManagementServiceImpl) SetDefaultModel(ctx context.Context, tenantID, modelID uuid.UUID) error {
	err := s.tenantRepo.SetDefaultModel(ctx, tenantID, modelID)
	if err != nil {
		return fmt.Errorf("failed to set default model: %w", err)
	}

	// Clear cache for this tenant
	s.clearTenantCache(ctx, tenantID)

	return nil
}

// GetUsageStats returns usage statistics for a tenant
func (s *ModelManagementServiceImpl) GetUsageStats(ctx context.Context, tenantID uuid.UUID, modelID *uuid.UUID, period string) (map[string]interface{}, error) {
	// Calculate time range based on period
	now := time.Now()
	var startTime time.Time

	switch period {
	case "day":
		startTime = now.Add(-24 * time.Hour)
	case "week":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "month":
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		startTime = now.Add(-30 * 24 * time.Hour)
	}

	// Get usage from repository
	summary, err := s.usageRepo.GetUsageSummary(ctx, tenantID, startTime, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	// Get model breakdown
	modelBreakdown, err := s.usageRepo.GetUsageByModel(ctx, tenantID, startTime, now)
	if err != nil {
		s.logger.Warn("Failed to get model breakdown", map[string]interface{}{
			"error": err.Error(),
		})
		modelBreakdown = []embedding_usage.ModelUsage{}
	}

	// Convert model breakdown to map
	modelUsage := make(map[string]map[string]interface{})
	for _, m := range modelBreakdown {
		if modelID == nil || m.ModelID == *modelID {
			modelIDStr := m.ModelID.String()
			modelUsage[modelIDStr] = map[string]interface{}{
				"tokens":   m.TokensUsed,
				"requests": m.RequestCount,
				"cost":     m.TotalCost,
			}
		}
	}

	return map[string]interface{}{
		"period":         period,
		"start_time":     startTime,
		"end_time":       now,
		"total_tokens":   summary.TotalTokens,
		"total_requests": summary.TotalRequests,
		"total_cost":     summary.TotalCost,
		"by_model":       modelUsage,
	}, nil
}

// GetTenantQuotas returns quota information for a tenant
func (s *ModelManagementServiceImpl) GetTenantQuotas(ctx context.Context, tenantID uuid.UUID) (map[string]interface{}, error) {
	models, err := s.tenantRepo.ListTenantModels(ctx, tenantID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant models: %w", err)
	}

	quotas := make([]map[string]interface{}, 0)

	for _, model := range models {
		status, err := s.tenantRepo.CheckUsageLimits(ctx, tenantID, model.ModelID)
		if err != nil {
			s.logger.Warn("Failed to check usage limits", map[string]interface{}{
				"tenant_id": tenantID,
				"model_id":  model.ModelID,
				"error":     err.Error(),
			})
			continue
		}

		quotas = append(quotas, map[string]interface{}{
			"model_id":              model.ModelID,
			"monthly_token_limit":   model.MonthlyTokenLimit,
			"monthly_tokens_used":   status.MonthlyTokensUsed,
			"daily_token_limit":     model.DailyTokenLimit,
			"daily_tokens_used":     status.DailyTokensUsed,
			"monthly_request_limit": model.MonthlyRequestLimit,
			"monthly_requests_used": status.MonthlyRequestsUsed,
			"is_within_limits":      status.IsWithinLimits,
		})
	}

	return map[string]interface{}{
		"tenant_id": tenantID,
		"quotas":    quotas,
	}, nil
}

// clearTenantCache clears cache entries for a specific tenant
func (s *ModelManagementServiceImpl) clearTenantCache(ctx context.Context, tenantID uuid.UUID) {
	pattern := fmt.Sprintf("model_selection:%s:*", tenantID)
	iter := s.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		_ = s.redisClient.Del(ctx, iter.Val()).Err()
	}
}

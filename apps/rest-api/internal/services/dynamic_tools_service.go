package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/storage"
	pkgcache "github.com/developer-mesh/developer-mesh/pkg/cache"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	pkgrepository "github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/developer-mesh/developer-mesh/pkg/tools/adapters"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// DynamicToolsServiceInterface defines the interface for dynamic tools operations
type DynamicToolsServiceInterface interface {
	ListTools(ctx context.Context, tenantID string, status string) ([]*models.DynamicTool, error)
	GetTool(ctx context.Context, tenantID, toolID string) (*models.DynamicTool, error)
	CreateTool(ctx context.Context, tenantID string, config tools.ToolConfig) (*models.DynamicTool, error)
	UpdateTool(ctx context.Context, tenantID, toolID string, config tools.ToolConfig) (*models.DynamicTool, error)
	DeleteTool(ctx context.Context, tenantID, toolID string) error

	// Discovery operations
	StartDiscovery(ctx context.Context, config tools.ToolConfig) (*models.DiscoverySession, error)
	GetDiscoverySession(ctx context.Context, sessionID string) (*models.DiscoverySession, error)
	ConfirmDiscovery(ctx context.Context, sessionID string, toolConfig tools.ToolConfig) (*models.DynamicTool, error)

	// Health check operations
	CheckToolHealth(ctx context.Context, tenantID, toolID string) (*models.ToolHealthStatus, error)
	RefreshToolHealth(ctx context.Context, tenantID, toolID string) (*models.ToolHealthStatus, error)

	// Action operations
	ListToolActions(ctx context.Context, tenantID, toolID string) ([]models.ToolAction, error)
	ExecuteToolAction(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (interface{}, error)
	ExecuteToolActionWithPassthrough(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}, passthroughAuth *models.PassthroughAuthBundle) (interface{}, error)

	// Credential operations
	UpdateToolCredentials(ctx context.Context, tenantID, toolID string, creds *models.TokenCredential) error

	// Multi-API Discovery operations
	DiscoverMultipleAPIs(ctx context.Context, portalURL string) (*adapters.MultiAPIDiscoveryResult, error)
	CreateToolsFromMultipleAPIs(ctx context.Context, tenantID string, result *adapters.MultiAPIDiscoveryResult, baseConfig tools.ToolConfig) ([]*models.DynamicTool, error)

	// Repository access
	GetDynamicToolRepository() pkgrepository.DynamicToolRepository
}

// DynamicToolsService implements the dynamic tools business logic
type DynamicToolsService struct {
	db                       *sqlx.DB
	logger                   observability.Logger
	metricsClient            observability.MetricsClient
	encryptionSvc            *security.EncryptionService
	discoveryService         DiscoveryServiceInterface
	healthCheckMgr           *tools.HealthCheckManager
	toolCache                map[string]*models.DynamicTool // Simple in-memory cache
	toolCacheMu              sync.RWMutex                   // Mutex for thread-safe cache access
	patternRepo              *storage.DiscoveryPatternRepository
	multiAPIDiscoveryService *adapters.MultiAPIDiscoveryService
	dynamicToolRepo          pkgrepository.DynamicToolRepository
	cacheService             *pkgcache.Service // Execution result cache
}

// NewDynamicToolsService creates a new dynamic tools service
func NewDynamicToolsService(
	db *sqlx.DB,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	encryptionSvc *security.EncryptionService,
	patternRepo *storage.DiscoveryPatternRepository,
	cacheService *pkgcache.Service,
) DynamicToolsServiceInterface {
	// Create discovery service with pattern repository
	discoveryService := NewEnhancedDiscoveryService(
		logger,
		metricsClient,
		patternRepo,
		storage.NewDiscoveryHintRepository(db.DB),
	)

	// Create health check manager with required dependencies
	cacheClient := cache.NewMemoryCache(1000, 24*time.Hour)
	openAPIHandler := adapters.NewOpenAPIAdapter(logger)
	healthCheckMgr := tools.NewHealthCheckManager(cacheClient, openAPIHandler, logger, metricsClient)

	// Create multi-API discovery service
	multiAPIDiscoveryService := adapters.NewMultiAPIDiscoveryService(logger)

	// Create dynamic tool repository
	dynamicToolRepo := pkgrepository.NewDynamicToolRepository(db)

	return &DynamicToolsService{
		db:                       db,
		logger:                   logger,
		metricsClient:            metricsClient,
		encryptionSvc:            encryptionSvc,
		discoveryService:         discoveryService,
		healthCheckMgr:           healthCheckMgr,
		toolCache:                make(map[string]*models.DynamicTool),
		patternRepo:              patternRepo,
		multiAPIDiscoveryService: multiAPIDiscoveryService,
		dynamicToolRepo:          dynamicToolRepo,
		cacheService:             cacheService,
	}
}

// ListTools lists all tools for a tenant
func (s *DynamicToolsService) ListTools(ctx context.Context, tenantID string, status string) ([]*models.DynamicTool, error) {
	query := `
		SELECT 
			id, tenant_id, tool_name, display_name, base_url,
			config, auth_type, retry_policy, status, 
			health_status, last_health_check,
			created_at, updated_at, provider, passthrough_config
		FROM mcp.tool_configurations
		WHERE tenant_id = $1
	`
	args := []interface{}{tenantID}

	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tools: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			s.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var tools []*models.DynamicTool
	for rows.Next() {
		tool, err := s.scanTool(rows)
		if err != nil {
			s.logger.Error("Failed to scan tool", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}
		tools = append(tools, tool)
	}

	return tools, nil
}

// GetTool gets a specific tool
func (s *DynamicToolsService) GetTool(ctx context.Context, tenantID, toolID string) (*models.DynamicTool, error) {
	// Check cache first with read lock
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	s.toolCacheMu.RLock()
	if cached, ok := s.toolCache[cacheKey]; ok {
		s.toolCacheMu.RUnlock()
		return cached, nil
	}
	s.toolCacheMu.RUnlock()

	// Use repository to get the tool (handles JSONB unmarshaling properly)
	tool, err := s.dynamicToolRepo.GetByID(ctx, toolID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool: %w", err)
	}

	// Verify tenant ownership
	if tool.TenantID != tenantID {
		return nil, fmt.Errorf("tool not found")
	}

	// Cache the result with write lock
	s.toolCacheMu.Lock()
	s.toolCache[cacheKey] = tool
	s.toolCacheMu.Unlock()

	return tool, nil
}

// CreateTool creates a new tool with discovery
func (s *DynamicToolsService) CreateTool(ctx context.Context, tenantID string, config tools.ToolConfig) (*models.DynamicTool, error) {
	// Perform discovery first
	result, err := s.discoveryService.DiscoverTool(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to discover tool: %w", err)
	}

	// Allow partial discovery if we at least found the OpenAPI spec URL
	if result.Status != tools.DiscoveryStatusSuccess && result.Status != tools.DiscoveryStatusPartial {
		return nil, fmt.Errorf("discovery failed with status: %s", result.Status)
	}

	// For partial discovery, ensure we have minimum required information
	if result.Status == tools.DiscoveryStatusPartial {
		if result.SpecURL == "" && config.OpenAPIURL == "" {
			return nil, fmt.Errorf("partial discovery without OpenAPI spec URL")
		}
		// Log warning about partial discovery
		s.logger.Warn("Creating tool with partial discovery results", map[string]interface{}{
			"tool_name":  config.Name,
			"status":     result.Status,
			"urls_found": len(result.DiscoveredURLs),
		})
	}

	// Extract tool information from discovery result
	toolID := uuid.New().String()
	now := time.Now()

	// Prepare config with discovered information
	if config.Config == nil {
		config.Config = make(map[string]interface{})
	}
	config.Config["discovery_result"] = result
	config.Config["spec_url"] = result.SpecURL
	config.Config["discovered_urls"] = result.DiscoveredURLs

	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Extract webhook configuration from discovery
	var webhookConfig *models.ToolWebhookConfig
	if result.WebhookConfig != nil {
		webhookConfig = result.WebhookConfig
		// Ensure the webhook path includes the tool ID
		if webhookConfig.EndpointPath == "" {
			webhookConfig.EndpointPath = fmt.Sprintf("/api/webhooks/tools/%s", toolID)
		}
	}

	// Encrypt credentials if provided
	var encryptedCreds []byte
	if config.Credential != nil {
		encryptedJSON, err := s.encryptionSvc.EncryptJSON(config.Credential, config.TenantID)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt credentials: %w", err)
		}
		encryptedCreds = []byte(encryptedJSON)
	}

	// Pre-cache the OpenAPI spec if available
	if specURL, ok := config.Config["spec_url"].(string); ok && specURL != "" {
		s.logger.Info("Pre-caching OpenAPI spec for tool", map[string]interface{}{
			"tool_name": config.Name,
			"spec_url":  specURL,
		})

		// Create cache repository
		cacheRepo := pkgrepository.NewOpenAPICacheRepository(s.db)

		// Attempt to fetch and cache the spec
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			if err := s.preCacheOpenAPISpec(ctx, specURL, cacheRepo); err != nil {
				s.logger.Error("Failed to pre-cache OpenAPI spec", map[string]interface{}{
					"tool_name": config.Name,
					"spec_url":  specURL,
					"error":     err.Error(),
				})
			} else {
				s.logger.Info("Successfully pre-cached OpenAPI spec", map[string]interface{}{
					"tool_name": config.Name,
					"spec_url":  specURL,
				})
			}
		}()
	}

	// Marshal webhook config if present
	var webhookConfigPtr *json.RawMessage
	if webhookConfig != nil {
		wcJSON, err := json.Marshal(webhookConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal webhook config: %w", err)
		}
		webhookConfigJSON := json.RawMessage(wcJSON)
		webhookConfigPtr = &webhookConfigJSON
	}

	// Insert tool into database
	query := `
		INSERT INTO mcp.tool_configurations (
			id, tenant_id, tool_name, tool_type, display_name, base_url, config,
			auth_type, credentials_encrypted, retry_policy, status,
			health_status, last_health_check, created_at, updated_at,
			provider, passthrough_config, webhook_config,
			documentation_url, openapi_url, openapi_spec_url, description, health_message
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
		)
	`

	// Sanitize tool name to comply with database constraint
	// Convert to lowercase, replace spaces and special chars with hyphens
	sanitizedName := sanitizeToolName(config.Name)

	// Check if a tool with this name already exists for the tenant
	existingQuery := `
		SELECT id FROM mcp.tool_configurations 
		WHERE tenant_id = $1 AND tool_name = $2
		LIMIT 1
	`
	var existingID string
	err = s.db.GetContext(ctx, &existingID, existingQuery, tenantID, sanitizedName)
	if err == nil {
		// Tool already exists
		return nil, fmt.Errorf("tool with name '%s' already exists for this tenant", sanitizedName)
	} else if err != sql.ErrNoRows {
		// Actual database error
		return nil, fmt.Errorf("failed to check for existing tool: %w", err)
	}

	// Determine tool type - default to "rest" for API tools
	toolType := "rest"
	if config.Provider == "graphql" {
		toolType = "graphql"
	}

	// Convert passthrough config to JSON if provided
	var passthroughConfigPtr *json.RawMessage
	if config.PassthroughConfig != nil {
		pcJSON, err := json.Marshal(config.PassthroughConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal passthrough config: %w", err)
		}
		passthroughConfigJSON := json.RawMessage(pcJSON)
		passthroughConfigPtr = &passthroughConfigJSON
	}

	// Handle nullable string fields
	var documentationURL *string
	if config.DocumentationURL != "" {
		documentationURL = &config.DocumentationURL
	}

	var openAPIURL *string
	if config.OpenAPIURL != "" {
		openAPIURL = &config.OpenAPIURL
	}

	// Extract OpenAPI spec URL from discovery result if available
	var openAPISpecURL *string
	if result != nil && result.SpecURL != "" {
		openAPISpecURL = &result.SpecURL
	}

	tool := &models.DynamicTool{
		ID:                   toolID,
		TenantID:             tenantID,
		ToolName:             sanitizedName,
		ToolType:             toolType,
		DisplayName:          config.Name, // Keep original name for display
		BaseURL:              config.BaseURL,
		DocumentationURL:     documentationURL,
		OpenAPIURL:           openAPIURL,
		OpenAPISpecURL:       openAPISpecURL,
		Config:               config.Config,
		AuthType:             "bearer", // Default, should be extracted from discovery
		CredentialsEncrypted: encryptedCreds,
		Status:               "active",
		IsActive:             true, // New tools are active by default
		HealthStatus:         nil,  // Will be set after first health check
		LastHealthCheck:      &now,
		CreatedAt:            now,
		UpdatedAt:            now,
		Provider:             config.Provider,
		WebhookConfig:        webhookConfigPtr,
		PassthroughConfig:    passthroughConfigPtr,
		Description:          nil, // Can be set later
		HealthMessage:        nil, // Will be set after health check
	}

	// Convert empty webhook config to nil for proper JSONB handling
	var webhookConfigParam interface{} = nil
	if webhookConfigPtr != nil {
		webhookConfigParam = webhookConfigPtr
	}

	// Convert empty passthrough config to nil for proper JSONB handling
	var passthroughConfigParam interface{} = nil
	if passthroughConfigPtr != nil {
		passthroughConfigParam = passthroughConfigPtr
	}

	// Handle health_status - pass nil interface{} for NULL in JSONB column
	var healthStatusParam interface{} = nil
	if tool.HealthStatus != nil {
		healthStatusParam = tool.HealthStatus
	}

	// Execute the insert
	_, err = s.db.ExecContext(ctx, query,
		toolID, tenantID, sanitizedName, toolType, config.Name, config.BaseURL, configJSON,
		tool.AuthType, encryptedCreds, nil, tool.Status,
		healthStatusParam, tool.LastHealthCheck, tool.CreatedAt, tool.UpdatedAt,
		tool.Provider, passthroughConfigParam, webhookConfigParam,
		documentationURL, openAPIURL, openAPISpecURL, nil, nil, // documentation_url, openapi_url, openapi_spec_url, description, health_message
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool: %w", err)
	}

	// Update cache with write lock
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	s.toolCacheMu.Lock()
	s.toolCache[cacheKey] = tool
	s.toolCacheMu.Unlock()

	return tool, nil
}

// UpdateTool updates an existing tool
func (s *DynamicToolsService) UpdateTool(ctx context.Context, tenantID, toolID string, config tools.ToolConfig) (*models.DynamicTool, error) {
	// Similar to CreateTool but with UPDATE query
	// Implementation would follow the same pattern
	return nil, fmt.Errorf("not implemented")
}

// DeleteTool deletes a tool
func (s *DynamicToolsService) DeleteTool(ctx context.Context, tenantID, toolID string) error {
	query := `
		DELETE FROM mcp.tool_configurations
		WHERE tenant_id = $1 AND id = $2
	`

	result, err := s.db.ExecContext(ctx, query, tenantID, toolID)
	if err != nil {
		return fmt.Errorf("failed to delete tool: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tool not found")
	}

	// Invalidate cache with write lock
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	s.toolCacheMu.Lock()
	delete(s.toolCache, cacheKey)
	s.toolCacheMu.Unlock()

	return nil
}

// StartDiscovery starts a discovery session
func (s *DynamicToolsService) StartDiscovery(ctx context.Context, config tools.ToolConfig) (*models.DiscoverySession, error) {
	// Generate session ID
	sessionID := uuid.New().String()
	tenantID := config.TenantID

	// Create discovery session in database
	session := &models.DiscoverySession{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		SessionID: sessionID,
		BaseURL:   config.BaseURL,
		Status:    "discovering",
		DiscoveryResult: &models.DiscoveryResult{
			Status:         "in_progress",
			DiscoveredURLs: []string{},
		},
		DiscoveryMetadata: map[string]interface{}{
			"started_at": time.Now(),
			"config":     config,
		},
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	// Insert into database
	query := `
		INSERT INTO tool_discovery_sessions (
			id, tenant_id, session_id, base_url, status,
			discovery_result, discovery_metadata, created_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`

	metadataJSON, _ := json.Marshal(session.DiscoveryMetadata)
	discoveryResultJSON, _ := json.Marshal(session.DiscoveryResult)

	_, err := s.db.ExecContext(ctx, query,
		session.ID, tenantID, sessionID, config.BaseURL,
		session.Status, discoveryResultJSON, metadataJSON,
		session.CreatedAt, session.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery session: %w", err)
	}

	// Start async discovery
	go s.performDiscovery(context.Background(), sessionID, config)

	return session, nil
}

// GetDynamicToolRepository returns the underlying repository for direct access
func (s *DynamicToolsService) GetDynamicToolRepository() pkgrepository.DynamicToolRepository {
	return s.dynamicToolRepo
}

// GetDiscoverySession gets a discovery session
func (s *DynamicToolsService) GetDiscoverySession(ctx context.Context, sessionID string) (*models.DiscoverySession, error) {
	query := `
		SELECT 
			id, tenant_id, session_id, base_url, status,
			discovery_result, discovery_metadata,
			created_at, expires_at
		FROM tool_discovery_sessions
		WHERE session_id = $1
	`

	var session models.DiscoverySession
	var discoveryResultJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID, &session.TenantID, &session.SessionID,
		&session.BaseURL, &session.Status, &discoveryResultJSON,
		&metadataJSON, &session.CreatedAt, &session.ExpiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("discovery session not found")
		}
		return nil, err
	}

	// Parse JSON fields
	if discoveryResultJSON != nil {
		var result models.DiscoveryResult
		if err := json.Unmarshal(discoveryResultJSON, &result); err == nil {
			session.DiscoveryResult = &result
		}
	}
	if err := json.Unmarshal(metadataJSON, &session.DiscoveryMetadata); err != nil {
		session.DiscoveryMetadata = map[string]interface{}{}
	}

	return &session, nil
}

// ConfirmDiscovery confirms a discovery session
func (s *DynamicToolsService) ConfirmDiscovery(ctx context.Context, sessionID string, toolConfig tools.ToolConfig) (*models.DynamicTool, error) {
	// Get the discovery session
	session, err := s.GetDiscoverySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Verify session status
	if session.Status != "discovered" {
		return nil, fmt.Errorf("session not ready for confirmation: %s", session.Status)
	}

	// Verify session hasn't expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("discovery session has expired")
	}

	// Use the discovered OpenAPI URL if available
	if session.DiscoveryResult != nil && session.DiscoveryResult.SpecURL != "" {
		toolConfig.OpenAPIURL = session.DiscoveryResult.SpecURL
	}

	// Create the tool with discovered configuration
	tool, err := s.CreateTool(ctx, session.TenantID, toolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool: %w", err)
	}

	// Update session status to confirmed
	updateQuery := `
		UPDATE tool_discovery_sessions
		SET status = 'confirmed', updated_at = NOW()
		WHERE session_id = $1
	`
	_, _ = s.db.ExecContext(ctx, updateQuery, sessionID)

	return tool, nil
}

// CheckToolHealth checks tool health
func (s *DynamicToolsService) CheckToolHealth(ctx context.Context, tenantID, toolID string) (*models.ToolHealthStatus, error) {
	tool, err := s.GetTool(ctx, tenantID, toolID)
	if err != nil {
		return nil, err
	}

	// Convert to tools.ToolConfig
	config := tools.ToolConfig{
		ID:       tool.ID,
		TenantID: tool.TenantID,
		Name:     tool.ToolName,
		BaseURL:  "",
		Config:   tool.Config,
	}

	// Extract base URL from config
	if baseURL, ok := tool.Config["base_url"].(string); ok {
		config.BaseURL = baseURL
	}

	// Use health check manager
	status, err := s.healthCheckMgr.CheckHealth(ctx, config, false)
	if err != nil {
		return nil, err
	}

	// Convert to models.ToolHealthStatus
	return &models.ToolHealthStatus{
		IsHealthy:    status.IsHealthy,
		LastChecked:  status.LastChecked,
		ResponseTime: status.ResponseTime,
		Error:        status.Error,
		Details:      status.Details,
	}, nil
}

// RefreshToolHealth refreshes tool health
func (s *DynamicToolsService) RefreshToolHealth(ctx context.Context, tenantID, toolID string) (*models.ToolHealthStatus, error) {
	// Force refresh by clearing cache
	return s.CheckToolHealth(ctx, tenantID, toolID)
}

// ListToolActions lists available actions for a tool
func (s *DynamicToolsService) ListToolActions(ctx context.Context, tenantID, toolID string) ([]models.ToolAction, error) {
	// Get the tool
	tool, err := s.GetTool(ctx, tenantID, toolID)
	if err != nil {
		return nil, err
	}

	// Create OpenAPI cache repository
	cacheRepo := pkgrepository.NewOpenAPICacheRepository(s.db)

	// Create adapter for the tool
	adapter, err := adapters.NewDynamicToolAdapter(tool, cacheRepo, s.encryptionSvc, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool adapter: %w", err)
	}

	// List actions from the adapter
	actions, err := adapter.ListActions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}

	// Record metric
	if s.metricsClient != nil {
		s.metricsClient.IncrementCounterWithLabels("tools.actions.list", 1, map[string]string{
			"tenant_id": tenantID,
			"tool_id":   toolID,
			"tool_name": tool.ToolName,
		})
	}

	return actions, nil
}

// ExecuteToolAction executes a tool action
func (s *DynamicToolsService) ExecuteToolAction(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (interface{}, error) {
	start := time.Now()

	// Extract cache TTL from params if provided
	cacheTTL := 3600 // Default 1 hour
	if ttl, ok := params["cache_ttl"].(int); ok {
		cacheTTL = ttl
		delete(params, "cache_ttl") // Remove from params before execution
	}

	// Remove explicit cache key from params if provided
	if _, ok := params["cache_key"].(string); ok {
		delete(params, "cache_key") // Remove from params before execution
	}

	// Use cache service for execution
	if s.cacheService != nil {
		req := &pkgcache.ExecutionRequest{
			TenantID:   tenantID,
			ToolID:     toolID,
			Action:     action,
			Parameters: params,
			TTLSeconds: cacheTTL,
		}

		result, err := s.cacheService.GetOrCompute(ctx, req, func(ctx context.Context) (interface{}, error) {
			// This function only runs on cache miss
			return s.executeToolActionInternal(ctx, tenantID, toolID, action, params)
		})

		if err != nil {
			s.logger.Error("Failed to execute tool action", map[string]interface{}{
				"tenant_id": tenantID,
				"tool_id":   toolID,
				"action":    action,
				"error":     err.Error(),
			})
			// Record failure metric
			if s.metricsClient != nil {
				s.metricsClient.IncrementCounterWithLabels("tools.actions.execute.error", 1, map[string]string{
					"tenant_id": tenantID,
					"tool_id":   toolID,
					"action":    action,
				})
			}
			return nil, err
		}

		// The cache service now properly handles ToolExecutionResponse types
		// and adds cache metadata directly to the struct

		// Record success metrics
		if s.metricsClient != nil {
			s.metricsClient.RecordHistogram("tools.actions.execute.duration", float64(time.Since(start).Milliseconds()), map[string]string{
				"tenant_id": tenantID,
				"tool_id":   toolID,
				"action":    action,
				"status":    "completed",
			})
		}

		return result, nil
	}

	// Fallback to direct execution if cache service not available
	return s.executeToolActionInternal(ctx, tenantID, toolID, action, params)
}

// executeToolActionInternal performs the actual tool execution (called by cache on miss)
func (s *DynamicToolsService) executeToolActionInternal(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (interface{}, error) {
	// Get the tool
	tool, err := s.GetTool(ctx, tenantID, toolID)
	if err != nil {
		return nil, err
	}

	// Check if tool is active
	if tool.Status != "active" {
		return nil, fmt.Errorf("tool is not active: %s", tool.Status)
	}

	// Create OpenAPI cache repository
	cacheRepo := pkgrepository.NewOpenAPICacheRepository(s.db)

	// Create adapter for the tool
	adapter, err := adapters.NewDynamicToolAdapter(tool, cacheRepo, s.encryptionSvc, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool adapter: %w", err)
	}

	// Execute the action
	result, err := adapter.ExecuteAction(ctx, action, params)
	if err != nil {
		return nil, err
	}

	// Log execution to database
	executionID := uuid.New().String()
	query := `
		INSERT INTO mcp.tool_executions (
			id, tool_id, tenant_id, action, input_data,
			status, output_data, duration_ms, executed_at, completed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	paramsJSON, _ := json.Marshal(params)
	resultJSON, _ := json.Marshal(result)
	status := "completed"
	if !result.Success {
		status = "failed"
	}
	now := time.Now()

	_, dbErr := s.db.ExecContext(ctx, query,
		executionID, toolID, tenantID, action, paramsJSON,
		status, resultJSON, result.Duration, result.ExecutedAt, now,
	)
	if dbErr != nil {
		s.logger.Warn("Failed to log tool execution", map[string]interface{}{
			"error": dbErr.Error(),
		})
	}

	return result, nil
}

// ExecuteToolActionWithPassthrough executes a tool action with passthrough authentication
func (s *DynamicToolsService) ExecuteToolActionWithPassthrough(
	ctx context.Context,
	tenantID, toolID, action string,
	params map[string]interface{},
	passthroughAuth *models.PassthroughAuthBundle,
) (interface{}, error) {
	start := time.Now()

	// Get the tool
	tool, err := s.GetTool(ctx, tenantID, toolID)
	if err != nil {
		return nil, err
	}

	// Check if tool is active
	if tool.Status != "active" {
		return nil, fmt.Errorf("tool is not active: %s", tool.Status)
	}

	// Parse passthrough config
	var passthroughConfig *models.EnhancedPassthroughConfig
	if tool.PassthroughConfig != nil {
		passthroughConfig = &models.EnhancedPassthroughConfig{}
		if err := json.Unmarshal(*tool.PassthroughConfig, passthroughConfig); err != nil {
			s.logger.Warn("Failed to parse passthrough config", map[string]interface{}{
				"tool_id": toolID,
				"error":   err.Error(),
			})
		}
	}

	// Check if passthrough is allowed
	if passthroughConfig != nil && passthroughConfig.Mode == "disabled" {
		return nil, fmt.Errorf("passthrough authentication is disabled for this tool")
	}

	// If passthrough is required but not provided
	if passthroughConfig != nil && passthroughConfig.Mode == "required" && passthroughAuth == nil {
		return nil, fmt.Errorf("passthrough authentication is required for this tool")
	}

	// Create OpenAPI cache repository
	cacheRepo := pkgrepository.NewOpenAPICacheRepository(s.db)

	// Create adapter for the tool
	adapter, err := adapters.NewDynamicToolAdapter(tool, cacheRepo, s.encryptionSvc, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool adapter: %w", err)
	}

	// Execute the action with passthrough auth
	execResult, err := adapter.ExecuteWithPassthrough(ctx, action, params, passthroughAuth, passthroughConfig)
	if err != nil {
		s.logger.Error("Failed to execute tool action with passthrough", map[string]interface{}{
			"tenant_id":       tenantID,
			"tool_id":         toolID,
			"action":          action,
			"has_passthrough": passthroughAuth != nil,
			"error":           err.Error(),
		})
		// Record failure metric
		if s.metricsClient != nil {
			s.metricsClient.IncrementCounterWithLabels("tools.actions.execute.passthrough.error", 1, map[string]string{
				"tenant_id": tenantID,
				"tool_id":   toolID,
				"action":    action,
			})
		}
		return nil, err
	}

	// Log execution to database with passthrough flag
	executionID := uuid.New().String()
	status := "completed"

	// Check if there was an error in the execution
	if execResult.Error != "" {
		status = "failed"
	}

	// Determine auth method used
	authMethod := "stored"
	if passthroughAuth != nil {
		authMethod = "passthrough"
	}

	query := `
		INSERT INTO mcp.tool_executions (
			id, tool_id, tenant_id, action, input_data,
			output_data, status, duration_ms, executed_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	responseJSON, _ := json.Marshal(execResult)
	paramsJSON, _ := json.Marshal(params)

	_, dbErr := s.db.ExecContext(ctx, query,
		executionID, toolID, tenantID, action, paramsJSON,
		responseJSON, status, time.Since(start).Milliseconds(),
		time.Now(), time.Now(),
	)
	if dbErr != nil {
		s.logger.Warn("Failed to log tool execution", map[string]interface{}{
			"error": dbErr.Error(),
		})
	}

	// Record success metrics
	if s.metricsClient != nil {
		s.metricsClient.RecordHistogram("tools.actions.execute.passthrough.duration", float64(time.Since(start).Milliseconds()), map[string]string{
			"tenant_id":   tenantID,
			"tool_id":     toolID,
			"action":      action,
			"status":      status,
			"auth_method": authMethod,
		})
	}

	return execResult, nil
}

// UpdateToolCredentials updates tool credentials
func (s *DynamicToolsService) UpdateToolCredentials(ctx context.Context, tenantID, toolID string, creds *models.TokenCredential) error {
	// Verify tool exists and belongs to tenant
	tool, err := s.GetTool(ctx, tenantID, toolID)
	if err != nil {
		return err
	}

	// Encrypt the new credentials
	encryptedJSON, err := s.encryptionSvc.EncryptJSON(creds, tenantID)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	// Update in database
	query := `
		UPDATE tool_configurations 
		SET 
			credentials_encrypted = $1,
			auth_type = $2,
			updated_at = NOW()
		WHERE id = $3 AND tenant_id = $4
	`

	// Determine auth type from credentials
	authType := "token"
	if creds.Type != "" {
		authType = creds.Type
	}

	result, err := s.db.ExecContext(ctx, query, []byte(encryptedJSON), authType, toolID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to update credentials: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tool not found or not owned by tenant")
	}

	// Invalidate cache with write lock
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	s.toolCacheMu.Lock()
	delete(s.toolCache, cacheKey)
	s.toolCacheMu.Unlock()

	// Log the update
	s.logger.Info("Tool credentials updated", map[string]interface{}{
		"tenant_id": tenantID,
		"tool_id":   toolID,
		"tool_name": tool.ToolName,
		"auth_type": authType,
	})

	// Optionally test the new credentials
	if tool.HealthStatus != nil {
		// Trigger a health check with new credentials
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, _ = s.RefreshToolHealth(ctx, tenantID, toolID)
		}()
	}

	return nil
}

// performDiscovery performs the actual discovery process asynchronously
func (s *DynamicToolsService) performDiscovery(ctx context.Context, sessionID string, config tools.ToolConfig) {
	// Update status to discovering
	updateQuery := `
		UPDATE tool_discovery_sessions
		SET status = $2, discovery_metadata = discovery_metadata || $3
		WHERE session_id = $1
	`

	startMetadata, _ := json.Marshal(map[string]interface{}{
		"discovery_started": time.Now(),
	})
	_, _ = s.db.ExecContext(ctx, updateQuery, sessionID, "discovering", startMetadata)

	// Perform discovery
	result, err := s.discoveryService.DiscoverTool(ctx, config)

	if err != nil {
		// Update with error
		errorQuery := `
			UPDATE tool_discovery_sessions
			SET status = 'failed', discovery_result = $2
			WHERE session_id = $1
		`
		errorResult := &models.DiscoveryResult{
			Status:         "failed",
			RequiresManual: true,
			Metadata: map[string]interface{}{
				"error": err.Error(),
			},
		}
		errorResultJSON, _ := json.Marshal(errorResult)
		_, _ = s.db.ExecContext(ctx, errorQuery, sessionID, errorResultJSON)
		return
	}

	// Update with results
	status := "discovered"
	if result.Status != tools.DiscoveryStatusSuccess {
		status = "partial"
	}

	discoveryResultJSON, _ := json.Marshal(result)
	metadataJSON, _ := json.Marshal(map[string]interface{}{
		"discovery_completed": time.Now(),
		"result":              result,
	})

	finalQuery := `
		UPDATE tool_discovery_sessions
		SET 
			status = $2,
			discovery_result = $3,
			discovery_metadata = discovery_metadata || $4
		WHERE session_id = $1
	`

	_, _ = s.db.ExecContext(ctx, finalQuery,
		sessionID, status, discoveryResultJSON, metadataJSON,
	)
}

// scanTool scans a tool from database row
func (s *DynamicToolsService) scanTool(rows *sql.Rows) (*models.DynamicTool, error) {
	var tool models.DynamicTool
	var configJSON, healthStatusJSON []byte
	var retryPolicyJSON, passthroughConfigJSON sql.NullString

	err := rows.Scan(
		&tool.ID,
		&tool.TenantID,
		&tool.ToolName,
		&tool.DisplayName,
		&tool.BaseURL,
		&configJSON,
		&tool.AuthType,
		&retryPolicyJSON,
		&tool.Status,
		&healthStatusJSON,
		&tool.LastHealthCheck,
		&tool.CreatedAt,
		&tool.UpdatedAt,
		&tool.Provider,
		&passthroughConfigJSON,
	)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(configJSON, &tool.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if len(healthStatusJSON) > 0 {
		rawMsg := json.RawMessage(healthStatusJSON)
		tool.HealthStatus = &rawMsg
	}

	if retryPolicyJSON.Valid {
		if err := json.Unmarshal([]byte(retryPolicyJSON.String), &tool.RetryPolicy); err != nil {
			s.logger.Warn("Failed to unmarshal retry policy", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	if passthroughConfigJSON.Valid {
		if err := json.Unmarshal([]byte(passthroughConfigJSON.String), &tool.PassthroughConfig); err != nil {
			s.logger.Warn("Failed to unmarshal passthrough config", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return &tool, nil
}

// DiscoverMultipleAPIs discovers all APIs from a portal URL
func (s *DynamicToolsService) DiscoverMultipleAPIs(ctx context.Context, portalURL string) (*adapters.MultiAPIDiscoveryResult, error) {
	s.logger.Info("Starting multi-API discovery", map[string]interface{}{
		"portal_url": portalURL,
	})

	result, err := s.multiAPIDiscoveryService.DiscoverMultipleAPIs(ctx, portalURL)
	if err != nil {
		s.logger.Error("Multi-API discovery failed", map[string]interface{}{
			"portal_url": portalURL,
			"error":      err.Error(),
		})
		return nil, err
	}

	s.logger.Info("Multi-API discovery completed", map[string]interface{}{
		"portal_url": portalURL,
		"apis_found": len(result.DiscoveredAPIs),
		"status":     result.Status,
		"errors":     len(result.Errors),
	})

	return result, nil
}

// CreateToolsFromMultipleAPIs creates multiple tools from discovery results
func (s *DynamicToolsService) CreateToolsFromMultipleAPIs(ctx context.Context, tenantID string, result *adapters.MultiAPIDiscoveryResult, baseConfig tools.ToolConfig) ([]*models.DynamicTool, error) {
	var createdTools []*models.DynamicTool
	var errors []error

	s.logger.Info("Creating tools from multi-API discovery", map[string]interface{}{
		"tenant_id":  tenantID,
		"apis_count": len(result.DiscoveredAPIs),
	})

	for _, api := range result.DiscoveredAPIs {
		// Create a unique tool config for each API
		toolConfig := baseConfig
		toolConfig.TenantID = tenantID
		toolConfig.Name = fmt.Sprintf("%s - %s", baseConfig.Name, api.Name)
		toolConfig.OpenAPIURL = api.SpecURL

		// Add metadata
		if toolConfig.Config == nil {
			toolConfig.Config = make(map[string]interface{})
		}
		toolConfig.Config["api_category"] = api.Category
		toolConfig.Config["api_version"] = api.Version
		toolConfig.Config["discovered_from"] = result.BaseURL
		toolConfig.Config["discovery_method"] = result.DiscoveryMethod

		// Create the tool
		tool, err := s.CreateTool(ctx, tenantID, toolConfig)
		if err != nil {
			s.logger.Error("Failed to create tool from discovered API", map[string]interface{}{
				"tenant_id": tenantID,
				"api_name":  api.Name,
				"spec_url":  api.SpecURL,
				"error":     err.Error(),
			})
			errors = append(errors, fmt.Errorf("failed to create tool for %s: %w", api.Name, err))
			continue
		}

		createdTools = append(createdTools, tool)
		s.logger.Info("Successfully created tool from discovered API", map[string]interface{}{
			"tenant_id": tenantID,
			"tool_id":   tool.ID,
			"api_name":  api.Name,
		})
	}

	// Record metrics
	if s.metricsClient != nil {
		s.metricsClient.IncrementCounterWithLabels("tools.multi_api.created", float64(len(createdTools)), map[string]string{
			"tenant_id": tenantID,
			"portal":    result.BaseURL,
		})
		if len(errors) > 0 {
			s.metricsClient.IncrementCounterWithLabels("tools.multi_api.failed", float64(len(errors)), map[string]string{
				"tenant_id": tenantID,
				"portal":    result.BaseURL,
			})
		}
	}

	if len(errors) > 0 && len(createdTools) == 0 {
		return nil, fmt.Errorf("failed to create any tools: %v", errors)
	}

	return createdTools, nil
}

// preCacheOpenAPISpec fetches and caches an OpenAPI spec
func (s *DynamicToolsService) preCacheOpenAPISpec(ctx context.Context, specURL string, cacheRepo pkgrepository.OpenAPICacheRepository) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Fetch the spec
	req, err := http.NewRequestWithContext(ctx, "GET", specURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch spec: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			s.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch spec: HTTP %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read spec body: %w", err)
	}

	// Parse the OpenAPI spec
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	// Use LoadFromData to parse the spec
	spec, err := loader.LoadFromData(body)
	if err != nil {
		s.logger.Warn("Failed to parse full OpenAPI spec, attempting minimal parse", map[string]interface{}{
			"spec_url": specURL,
			"error":    err.Error(),
		})
		// Even if full parsing fails, we can still cache the raw spec
		// The adapter will handle parsing errors gracefully
	}

	// Cache the spec with 24 hour TTL
	if spec != nil {
		if err := cacheRepo.Set(ctx, specURL, spec, 24*time.Hour); err != nil {
			return fmt.Errorf("failed to cache spec: %w", err)
		}

		// Log success with operation count
		operationCount := 0
		if spec.Paths != nil {
			for _, pathItem := range spec.Paths.Map() {
				operationCount += len(pathItem.Operations())
			}
		}

		s.logger.Info("Cached OpenAPI spec", map[string]interface{}{
			"spec_url":   specURL,
			"paths":      len(spec.Paths.Map()),
			"operations": operationCount,
			"title":      spec.Info.Title,
			"version":    spec.Info.Version,
		})
	}

	return nil
}

// sanitizeToolName converts a display name to a valid tool_name
// that matches the database constraint: ^[a-zA-Z0-9][a-zA-Z0-9_-]*$
func sanitizeToolName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace spaces and special characters with hyphens
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	name = re.ReplaceAllString(name, "-")

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Ensure it starts with alphanumeric
	if len(name) > 0 && !regexp.MustCompile(`^[a-zA-Z0-9]`).MatchString(name) {
		name = "tool-" + name
	}

	// If empty, generate a default name
	if name == "" {
		name = "tool-" + uuid.New().String()[:8]
	}

	return name
}

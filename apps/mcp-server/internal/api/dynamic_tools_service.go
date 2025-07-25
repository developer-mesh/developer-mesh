package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// DynamicToolService handles business logic for dynamic tools
type DynamicToolService struct {
	db            *sql.DB
	logger        observability.Logger
	encryptionSvc *security.EncryptionService
	toolCache     map[string]*Tool // Simple in-memory cache
}

// NewDynamicToolService creates a new dynamic tool service
func NewDynamicToolService(
	db *sql.DB,
	logger observability.Logger,
	encryptionSvc *security.EncryptionService,
) *DynamicToolService {
	return &DynamicToolService{
		db:            db,
		logger:        logger,
		encryptionSvc: encryptionSvc,
		toolCache:     make(map[string]*Tool),
	}
}

// ListTools lists all tools for a tenant
func (s *DynamicToolService) ListTools(ctx context.Context, tenantID string, status string) ([]*Tool, error) {
	query := `
		SELECT 
			id, tenant_id, tool_name, display_name, 
			config, auth_type, retry_policy, status, 
			health_status, last_health_check,
			created_at, updated_at
		FROM tool_configurations
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
	defer rows.Close()
	
	var tools []*Tool
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
func (s *DynamicToolService) GetTool(ctx context.Context, tenantID, toolID string) (*Tool, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	if cached, ok := s.toolCache[cacheKey]; ok {
		return cached, nil
	}
	
	query := `
		SELECT 
			id, tenant_id, tool_name, display_name, 
			config, credentials_encrypted, auth_type, 
			retry_policy, status, health_status, 
			last_health_check, created_at, updated_at
		FROM tool_configurations
		WHERE tenant_id = $1 AND id = $2
	`
	
	row := s.db.QueryRowContext(ctx, query, tenantID, toolID)
	tool, err := s.scanToolWithCredentials(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrToolNotFound
		}
		return nil, fmt.Errorf("failed to get tool: %w", err)
	}
	
	// Decrypt credentials
	if tool.Config.Credential != nil && tool.Config.Credential.Token != "" {
		decrypted, err := s.encryptionSvc.DecryptCredential([]byte(tool.Config.Credential.Token), tenantID)
		if err != nil {
			s.logger.Error("Failed to decrypt credentials", map[string]interface{}{
				"tool_id": toolID,
				"error":   err.Error(),
			})
		} else {
			tool.Config.Credential.Token = decrypted
		}
	}
	
	// Update cache
	s.toolCache[cacheKey] = tool
	
	return tool, nil
}

// CreateTool creates a new tool
func (s *DynamicToolService) CreateTool(ctx context.Context, config tools.ToolConfig) (*Tool, error) {
	// Prepare data
	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	
	retryPolicyJSON, err := json.Marshal(config.RetryPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retry policy: %w", err)
	}
	
	var credentialsEncrypted []byte
	authType := "token"
	if config.Credential != nil {
		credentialsEncrypted = []byte(config.Credential.Token) // Already encrypted
		authType = config.Credential.Type
	}
	
	// Insert tool
	query := `
		INSERT INTO tool_configurations (
			id, tenant_id, tool_name, display_name,
			config, credentials_encrypted, auth_type,
			retry_policy, status, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		) RETURNING created_at, updated_at
	`
	
	var createdAt, updatedAt time.Time
	err = s.db.QueryRowContext(
		ctx, query,
		config.ID, config.TenantID, config.Name, config.Name,
		configJSON, credentialsEncrypted, authType,
		retryPolicyJSON, "active", "api",
	).Scan(&createdAt, &updatedAt)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create tool: %w", err)
	}
	
	// Build response
	tool := &Tool{
		ID:               config.ID,
		TenantID:         config.TenantID,
		Name:             config.Name,
		DisplayName:      config.Name,
		BaseURL:          config.BaseURL,
		DocumentationURL: config.DocumentationURL,
		OpenAPIURL:       config.OpenAPIURL,
		AuthType:         authType,
		Config:           config.Config,
		RetryPolicy:      config.RetryPolicy,
		HealthConfig:     config.HealthConfig,
		Status:           "active",
		CreatedAt:        createdAt.Format(time.RFC3339),
		UpdatedAt:        updatedAt.Format(time.RFC3339),
		Config:           config,
	}
	
	// Update cache
	cacheKey := fmt.Sprintf("%s:%s", config.TenantID, config.ID)
	s.toolCache[cacheKey] = tool
	
	return tool, nil
}

// UpdateTool updates a tool configuration
func (s *DynamicToolService) UpdateTool(ctx context.Context, config tools.ToolConfig) (*Tool, error) {
	// Prepare data
	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	
	retryPolicyJSON, err := json.Marshal(config.RetryPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retry policy: %w", err)
	}
	
	// Update tool
	query := `
		UPDATE tool_configurations
		SET 
			tool_name = $3,
			display_name = $4,
			config = $5,
			retry_policy = $6,
			updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING updated_at
	`
	
	var updatedAt time.Time
	err = s.db.QueryRowContext(
		ctx, query,
		config.TenantID, config.ID, config.Name, config.Name,
		configJSON, retryPolicyJSON,
	).Scan(&updatedAt)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrToolNotFound
		}
		return nil, fmt.Errorf("failed to update tool: %w", err)
	}
	
	// Invalidate cache
	cacheKey := fmt.Sprintf("%s:%s", config.TenantID, config.ID)
	delete(s.toolCache, cacheKey)
	
	// Get updated tool
	return s.GetTool(ctx, config.TenantID, config.ID)
}

// DeleteTool deletes a tool
func (s *DynamicToolService) DeleteTool(ctx context.Context, tenantID, toolID string) error {
	query := `
		DELETE FROM tool_configurations
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
		return ErrToolNotFound
	}
	
	// Invalidate cache
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	delete(s.toolCache, cacheKey)
	
	return nil
}

// StartDiscovery starts a discovery session
func (s *DynamicToolService) StartDiscovery(ctx context.Context, config tools.ToolConfig) (*DiscoverySession, error) {
	sessionID := uuid.New().String()
	
	// Create discovery session
	query := `
		INSERT INTO tool_discovery_sessions (
			id, tenant_id, session_id, base_url,
			status, discovery_metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6
		) RETURNING created_at, expires_at
	`
	
	metadataJSON, _ := json.Marshal(config.Config)
	
	var createdAt, expiresAt time.Time
	err := s.db.QueryRowContext(
		ctx, query,
		uuid.New().String(), config.TenantID, sessionID, config.BaseURL,
		"pending", metadataJSON,
	).Scan(&createdAt, &expiresAt)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery session: %w", err)
	}
	
	return &DiscoverySession{
		ID:        sessionID,
		TenantID:  config.TenantID,
		SessionID: sessionID,
		BaseURL:   config.BaseURL,
		Status:    tools.DiscoveryStatusManualNeeded,
		CreatedAt: createdAt.Format(time.RFC3339),
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

// GetDiscoverySession gets a discovery session
func (s *DynamicToolService) GetDiscoverySession(ctx context.Context, sessionID string) (*DiscoverySession, error) {
	query := `
		SELECT 
			id, tenant_id, session_id, base_url,
			status, discovered_urls, selected_url,
			discovery_metadata, error_message,
			created_at, expires_at
		FROM tool_discovery_sessions
		WHERE session_id = $1
	`
	
	var session DiscoverySession
	var discoveredURLs pq.StringArray
	var metadata, selectedURL, errorMessage sql.NullString
	
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID, &session.TenantID, &session.SessionID, &session.BaseURL,
		&session.Status, &discoveredURLs, &selectedURL,
		&metadata, &errorMessage,
		&session.CreatedAt, &session.ExpiresAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get discovery session: %w", err)
	}
	
	session.DiscoveredURLs = []string(discoveredURLs)
	if selectedURL.Valid {
		session.SelectedURL = selectedURL.String
	}
	if errorMessage.Valid {
		session.ErrorMessage = errorMessage.String
	}
	if metadata.Valid {
		json.Unmarshal([]byte(metadata.String), &session.Metadata)
	}
	
	return &session, nil
}

// UpdateDiscoverySession updates a discovery session
func (s *DynamicToolService) UpdateDiscoverySession(ctx context.Context, sessionID string, status tools.DiscoveryStatus, result *tools.DiscoveryResult, err error) error {
	query := `
		UPDATE tool_discovery_sessions
		SET 
			status = $2,
			discovered_urls = $3,
			discovery_metadata = $4,
			error_message = $5
		WHERE session_id = $1
	`
	
	var discoveredURLs pq.StringArray
	var metadata []byte
	var errorMessage sql.NullString
	
	if result != nil {
		discoveredURLs = result.DiscoveredURLs
		metadata, _ = json.Marshal(result.Metadata)
	}
	
	if err != nil {
		errorMessage = sql.NullString{String: err.Error(), Valid: true}
	}
	
	_, execErr := s.db.ExecContext(
		ctx, query,
		sessionID, string(status), discoveredURLs, metadata, errorMessage,
	)
	
	if execErr != nil {
		return fmt.Errorf("failed to update discovery session: %w", execErr)
	}
	
	return nil
}

// CreateToolFromDiscovery creates a tool from a discovery session
func (s *DynamicToolService) CreateToolFromDiscovery(ctx context.Context, session *DiscoverySession, req ConfirmDiscoveryRequest) (*Tool, error) {
	// Build tool config
	config := tools.ToolConfig{
		ID:               uuid.New().String(),
		TenantID:         session.TenantID,
		Name:             req.Name,
		BaseURL:          session.BaseURL,
		DocumentationURL: "",
		OpenAPIURL:       req.SelectedURL,
		Config:           req.Config,
		RetryPolicy:      req.RetryPolicy,
		HealthConfig:     req.HealthConfig,
	}
	
	// Handle credentials
	if req.Credentials != nil {
		encrypted, err := s.encryptionSvc.EncryptCredential(
			req.Credentials.Token,
			session.TenantID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt credentials: %w", err)
		}
		
		config.Credential = &models.TokenCredential{
			Type:         req.AuthType,
			Token:        string(encrypted),
			HeaderName:   req.Credentials.HeaderName,
			HeaderPrefix: req.Credentials.HeaderPrefix,
			QueryParam:   req.Credentials.QueryParam,
			Username:     req.Credentials.Username,
			Password:     req.Credentials.Password,
		}
	}
	
	// Create tool
	tool, err := s.CreateTool(ctx, config)
	if err != nil {
		return nil, err
	}
	
	// Mark session as confirmed
	s.db.ExecContext(ctx, `
		UPDATE tool_discovery_sessions
		SET status = 'confirmed', selected_url = $2
		WHERE session_id = $1
	`, session.SessionID, req.SelectedURL)
	
	return tool, nil
}

// UpdateHealthStatus updates the health status of a tool
func (s *DynamicToolService) UpdateHealthStatus(ctx context.Context, tenantID, toolID string, status *tools.HealthStatus) error {
	healthStatus := "unknown"
	if status.IsHealthy {
		healthStatus = "healthy"
	} else if status.Error != "" {
		healthStatus = "unhealthy"
	}
	
	query := `
		UPDATE tool_configurations
		SET 
			health_status = $3,
			last_health_check = $4
		WHERE tenant_id = $1 AND id = $2
	`
	
	_, err := s.db.ExecContext(
		ctx, query,
		tenantID, toolID, healthStatus, status.LastChecked,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update health status: %w", err)
	}
	
	// Invalidate cache
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	delete(s.toolCache, cacheKey)
	
	return nil
}

// ExecuteAction executes a tool action
func (s *DynamicToolService) ExecuteAction(ctx context.Context, tool *Tool, action string, params map[string]interface{}) (*ExecutionResult, error) {
	// This is a placeholder - actual execution will be handled by the tool registry
	// and the generated tools from OpenAPI specs
	
	startTime := time.Now()
	
	// Record execution
	executionID := uuid.New().String()
	paramsJSON, _ := json.Marshal(params)
	
	query := `
		INSERT INTO tool_executions (
			id, tool_config_id, tenant_id, action,
			parameters, status, executed_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
	`
	
	_, err := s.db.ExecContext(
		ctx, query,
		executionID, tool.ID, tool.TenantID, action,
		paramsJSON, "running", "api",
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to record execution: %w", err)
	}
	
	// TODO: Actual execution through tool registry
	// For now, return a mock response
	result := &ExecutionResult{
		ToolID:       tool.ID,
		Action:       action,
		Status:       "success",
		Result:       map[string]interface{}{"message": "Action executed successfully"},
		ResponseTime: int(time.Since(startTime).Milliseconds()),
		RetryCount:   0,
		ExecutedAt:   startTime.Format(time.RFC3339),
	}
	
	// Update execution record
	resultJSON, _ := json.Marshal(result.Result)
	s.db.ExecContext(ctx, `
		UPDATE tool_executions
		SET 
			status = $2,
			result = $3,
			response_time_ms = $4,
			completed_at = NOW()
		WHERE id = $1
	`, executionID, result.Status, resultJSON, result.ResponseTime)
	
	return result, nil
}

// GetAvailableActions gets available actions for a tool
func (s *DynamicToolService) GetAvailableActions(ctx context.Context, tool *Tool) ([]ActionDefinition, error) {
	// TODO: Get actions from generated tools
	// For now, return mock actions
	actions := []ActionDefinition{
		{
			Name:        "list_items",
			Description: "List all items",
			Method:      "GET",
			Path:        "/items",
			Parameters:  map[string]interface{}{"limit": "integer", "offset": "integer"},
			Returns:     map[string]interface{}{"type": "array"},
		},
		{
			Name:        "create_item",
			Description: "Create a new item",
			Method:      "POST",
			Path:        "/items",
			Parameters:  map[string]interface{}{"name": "string", "description": "string"},
			Returns:     map[string]interface{}{"type": "object"},
		},
	}
	
	return actions, nil
}

// Helper methods

func (s *DynamicToolService) scanTool(rows *sql.Rows) (*Tool, error) {
	var tool Tool
	var configJSON, retryPolicyJSON []byte
	var healthStatus, lastHealthCheck sql.NullString
	var createdAt, updatedAt time.Time
	
	err := rows.Scan(
		&tool.ID, &tool.TenantID, &tool.Name, &tool.DisplayName,
		&configJSON, &tool.AuthType, &retryPolicyJSON, &tool.Status,
		&healthStatus, &lastHealthCheck,
		&createdAt, &updatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	// Parse JSON fields
	json.Unmarshal(configJSON, &tool.Config)
	json.Unmarshal(retryPolicyJSON, &tool.RetryPolicy)
	
	// Set timestamps
	tool.CreatedAt = createdAt.Format(time.RFC3339)
	tool.UpdatedAt = updatedAt.Format(time.RFC3339)
	
	// Build tool config
	tool.Config = tools.ToolConfig{
		ID:           tool.ID,
		TenantID:     tool.TenantID,
		Name:         tool.Name,
		Config:       tool.Config,
		RetryPolicy:  tool.RetryPolicy,
	}
	
	return &tool, nil
}

func (s *DynamicToolService) scanToolWithCredentials(row *sql.Row) (*Tool, error) {
	var tool Tool
	var configJSON, retryPolicyJSON, credentialsEncrypted []byte
	var healthStatus, lastHealthCheck sql.NullString
	var createdAt, updatedAt time.Time
	
	err := row.Scan(
		&tool.ID, &tool.TenantID, &tool.Name, &tool.DisplayName,
		&configJSON, &credentialsEncrypted, &tool.AuthType,
		&retryPolicyJSON, &tool.Status, &healthStatus,
		&lastHealthCheck, &createdAt, &updatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	// Parse JSON fields
	json.Unmarshal(configJSON, &tool.Config)
	json.Unmarshal(retryPolicyJSON, &tool.RetryPolicy)
	
	// Set timestamps
	tool.CreatedAt = createdAt.Format(time.RFC3339)
	tool.UpdatedAt = updatedAt.Format(time.RFC3339)
	
	// Build tool config
	tool.Config = tools.ToolConfig{
		ID:          tool.ID,
		TenantID:    tool.TenantID,
		Name:        tool.Name,
		Config:      tool.Config,
		RetryPolicy: tool.RetryPolicy,
	}
	
	// Handle credentials
	if len(credentialsEncrypted) > 0 {
		tool.Config.Credential = &models.TokenCredential{
			Type:  tool.AuthType,
			Token: string(credentialsEncrypted),
		}
	}
	
	return &tool, nil
}
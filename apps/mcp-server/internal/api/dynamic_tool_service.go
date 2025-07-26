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
	"github.com/jmoiron/sqlx"
)

var (
	// ErrToolNotFound is returned when a tool is not found
	ErrToolNotFound = errors.New("tool not found")
	// ErrSessionNotFound is returned when a discovery session is not found
	ErrSessionNotFound = errors.New("discovery session not found")
)

// DynamicToolService handles dynamic tool operations
type DynamicToolService struct {
	db            *sqlx.DB
	encryptionSvc *security.EncryptionService
	logger        observability.Logger
	// In-memory storage for discovery sessions (could be moved to cache/db)
	sessions map[string]*DiscoverySession
}

// DiscoverySession represents an ongoing tool discovery session
type DiscoverySession struct {
	ID        string                 `json:"id"`
	Status    tools.DiscoveryStatus  `json:"status"`
	StartedAt time.Time              `json:"started_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Result    *tools.DiscoveryResult `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Config    tools.ToolConfig       `json:"-"` // Don't expose config with credentials
}

// Tool represents a configured tool
type Tool struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	BaseURL      string              `json:"base_url"`
	Status       string              `json:"status"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	Config       tools.ToolConfig    `json:"-"` // Internal use only
	HealthStatus *tools.HealthStatus `json:"health_status,omitempty"`
}

// NewDynamicToolService creates a new dynamic tool service
func NewDynamicToolService(db *sqlx.DB, encryptionSvc *security.EncryptionService, logger observability.Logger) *DynamicToolService {
	return &DynamicToolService{
		db:            db,
		encryptionSvc: encryptionSvc,
		logger:        logger,
		sessions:      make(map[string]*DiscoverySession),
	}
}

// ListTools lists all tools for a tenant
func (s *DynamicToolService) ListTools(ctx context.Context, tenantID, status string) ([]*Tool, error) {
	query := `
		SELECT 
			id, tenant_id, tool_name, base_url, documentation_url, 
			openapi_url, status, created_at, updated_at
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
		var (
			tool             Tool
			documentationURL sql.NullString
			openAPIURL       sql.NullString
			tenantIDStr      string
		)

		err := rows.Scan(
			&tool.ID,
			&tenantIDStr,
			&tool.Name,
			&tool.BaseURL,
			&documentationURL,
			&openAPIURL,
			&tool.Status,
			&tool.CreatedAt,
			&tool.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool row: %w", err)
		}

		// Build config for internal use
		tool.Config = tools.ToolConfig{
			ID:       tool.ID,
			TenantID: tenantID,
			Name:     tool.Name,
			BaseURL:  tool.BaseURL,
		}

		if documentationURL.Valid {
			tool.Config.DocumentationURL = documentationURL.String
		}
		if openAPIURL.Valid {
			tool.Config.OpenAPIURL = openAPIURL.String
		}

		tools = append(tools, &tool)
	}

	return tools, nil
}

// GetTool retrieves a specific tool
func (s *DynamicToolService) GetTool(ctx context.Context, tenantID, toolID string) (*Tool, error) {
	query := `
		SELECT 
			id, tenant_id, tool_name, base_url, documentation_url, 
			openapi_url, auth_type, encrypted_credentials, config,
			retry_policy, health_config, status, created_at, updated_at
		FROM tool_configurations
		WHERE id = $1 AND tenant_id = $2
	`

	var (
		tool             Tool
		documentationURL sql.NullString
		openAPIURL       sql.NullString
		authType         sql.NullString
		encryptedCreds   sql.NullString
		configJSON       sql.NullString
		retryPolicyJSON  sql.NullString
		healthConfigJSON sql.NullString
		tenantIDStr      string
	)

	err := s.db.QueryRowContext(ctx, query, toolID, tenantID).Scan(
		&tool.ID,
		&tenantIDStr,
		&tool.Name,
		&tool.BaseURL,
		&documentationURL,
		&openAPIURL,
		&authType,
		&encryptedCreds,
		&configJSON,
		&retryPolicyJSON,
		&healthConfigJSON,
		&tool.Status,
		&tool.CreatedAt,
		&tool.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrToolNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tool: %w", err)
	}

	// Build full config
	tool.Config = tools.ToolConfig{
		ID:       tool.ID,
		TenantID: tenantID,
		Name:     tool.Name,
		BaseURL:  tool.BaseURL,
	}

	if documentationURL.Valid {
		tool.Config.DocumentationURL = documentationURL.String
	}
	if openAPIURL.Valid {
		tool.Config.OpenAPIURL = openAPIURL.String
	}

	// Parse JSON fields
	if configJSON.Valid {
		if err := json.Unmarshal([]byte(configJSON.String), &tool.Config.Config); err != nil {
			s.logger.Warn("Failed to parse tool config JSON", map[string]interface{}{
				"tool_id": toolID,
				"error":   err.Error(),
			})
		}
	}

	if retryPolicyJSON.Valid {
		var retryPolicy tools.RetryPolicy
		if err := json.Unmarshal([]byte(retryPolicyJSON.String), &retryPolicy); err == nil {
			tool.Config.RetryPolicy = &retryPolicy
		}
	}

	if healthConfigJSON.Valid {
		var healthConfig tools.HealthConfig
		if err := json.Unmarshal([]byte(healthConfigJSON.String), &healthConfig); err == nil {
			tool.Config.HealthConfig = &healthConfig
		}
	}

	// Decrypt credentials if present
	if encryptedCreds.Valid && authType.Valid {
		decrypted, err := s.encryptionSvc.DecryptCredential([]byte(encryptedCreds.String), tenantID)
		if err != nil {
			s.logger.Error("Failed to decrypt tool credentials", map[string]interface{}{
				"tool_id": toolID,
				"error":   err.Error(),
			})
		} else {
			tool.Config.Credential = &models.TokenCredential{
				Type:  authType.String,
				Token: decrypted,
			}
		}
	}

	return &tool, nil
}

// CreateTool creates a new tool configuration
func (s *DynamicToolService) CreateTool(ctx context.Context, config tools.ToolConfig) (*Tool, error) {
	// Marshal JSON fields
	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var retryPolicyJSON []byte
	if config.RetryPolicy != nil {
		retryPolicyJSON, err = json.Marshal(config.RetryPolicy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal retry policy: %w", err)
		}
	}

	var healthConfigJSON []byte
	if config.HealthConfig != nil {
		healthConfigJSON, err = json.Marshal(config.HealthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal health config: %w", err)
		}
	}

	// Insert into database
	query := `
		INSERT INTO tool_configurations (
			id, tenant_id, tool_name, base_url, documentation_url,
			openapi_url, auth_type, encrypted_credentials, config,
			retry_policy, health_config, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'active'
		)
		RETURNING created_at, updated_at
	`

	var (
		authType       sql.NullString
		encryptedCreds sql.NullString
	)

	if config.Credential != nil {
		authType = sql.NullString{String: config.Credential.Type, Valid: true}
		encryptedCreds = sql.NullString{String: config.Credential.Token, Valid: true}
	}

	tool := &Tool{
		ID:      config.ID,
		Name:    config.Name,
		BaseURL: config.BaseURL,
		Status:  "active",
		Config:  config,
	}

	err = s.db.QueryRowContext(
		ctx, query,
		config.ID,
		config.TenantID,
		config.Name,
		config.BaseURL,
		sql.NullString{String: config.DocumentationURL, Valid: config.DocumentationURL != ""},
		sql.NullString{String: config.OpenAPIURL, Valid: config.OpenAPIURL != ""},
		authType,
		encryptedCreds,
		string(configJSON),
		retryPolicyJSON,
		healthConfigJSON,
	).Scan(&tool.CreatedAt, &tool.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create tool: %w", err)
	}

	return tool, nil
}

// UpdateTool updates an existing tool configuration
func (s *DynamicToolService) UpdateTool(ctx context.Context, config tools.ToolConfig) (*Tool, error) {
	// Similar to CreateTool but with UPDATE query
	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		UPDATE tool_configurations
		SET 
			tool_name = $3,
			base_url = $4,
			documentation_url = $5,
			openapi_url = $6,
			config = $7,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND tenant_id = $2
		RETURNING created_at, updated_at
	`

	tool := &Tool{
		ID:      config.ID,
		Name:    config.Name,
		BaseURL: config.BaseURL,
		Status:  "active",
		Config:  config,
	}

	err = s.db.QueryRowContext(
		ctx, query,
		config.ID,
		config.TenantID,
		config.Name,
		config.BaseURL,
		sql.NullString{String: config.DocumentationURL, Valid: config.DocumentationURL != ""},
		sql.NullString{String: config.OpenAPIURL, Valid: config.OpenAPIURL != ""},
		string(configJSON),
	).Scan(&tool.CreatedAt, &tool.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to update tool: %w", err)
	}

	return tool, nil
}

// DeleteTool deletes a tool configuration
func (s *DynamicToolService) DeleteTool(ctx context.Context, tenantID, toolID string) error {
	query := `
		UPDATE tool_configurations
		SET status = 'deleted', updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND tenant_id = $2 AND status != 'deleted'
	`

	result, err := s.db.ExecContext(ctx, query, toolID, tenantID)
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

	return nil
}

// StartDiscovery starts a new discovery session
func (s *DynamicToolService) StartDiscovery(ctx context.Context, config tools.ToolConfig) (*DiscoverySession, error) {
	session := &DiscoverySession{
		ID:        uuid.New().String(),
		Status:    tools.DiscoveryStatusInProgress,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
		Config:    config,
	}

	s.sessions[session.ID] = session

	return session, nil
}

// GetDiscoverySession retrieves a discovery session
func (s *DynamicToolService) GetDiscoverySession(ctx context.Context, sessionID string) (*DiscoverySession, error) {
	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

// UpdateDiscoverySession updates a discovery session
func (s *DynamicToolService) UpdateDiscoverySession(ctx context.Context, sessionID string, status tools.DiscoveryStatus, result *tools.DiscoveryResult, err error) error {
	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	session.Status = status
	session.UpdatedAt = time.Now()
	session.Result = result
	if err != nil {
		session.Error = err.Error()
	}

	return nil
}

// CreateToolFromDiscovery creates a tool from a discovery session
func (s *DynamicToolService) CreateToolFromDiscovery(ctx context.Context, session *DiscoverySession, req ConfirmDiscoveryRequest) (*Tool, error) {
	// Build tool config from discovery result and request
	config := session.Config
	config.ID = uuid.New().String()
	config.Name = req.Name

	if session.Result != nil && session.Result.OpenAPISpec != nil {
		// Use discovered information
		if session.Result.SpecURL != "" {
			config.OpenAPIURL = session.Result.SpecURL
		}
	}

	// Apply any overrides from request
	if req.Config != nil {
		for k, v := range req.Config {
			config.Config[k] = v
		}
	}

	// Create the tool
	return s.CreateTool(ctx, config)
}

// UpdateHealthStatus updates the health status of a tool
func (s *DynamicToolService) UpdateHealthStatus(ctx context.Context, tenantID, toolID string, status *tools.HealthStatus) error {
	healthData, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal health status: %w", err)
	}

	query := `
		UPDATE tool_configurations
		SET 
			health_status = $1,
			last_health_check = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $3 AND tenant_id = $4
	`

	_, err = s.db.ExecContext(ctx, query, string(healthData), status.LastChecked, toolID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to update health status: %w", err)
	}

	return nil
}

// GetAvailableActions retrieves available actions for a tool
func (s *DynamicToolService) GetAvailableActions(ctx context.Context, tool *Tool) ([]map[string]interface{}, error) {
	// This would be implemented by querying the tool's OpenAPI spec
	// and extracting available operations
	// For now, return empty list
	return []map[string]interface{}{}, nil
}

// ExecuteAction executes a tool action
func (s *DynamicToolService) ExecuteAction(ctx context.Context, tool *Tool, action string, params map[string]interface{}) (map[string]interface{}, error) {
	// This would be implemented by:
	// 1. Looking up the action in the tool's OpenAPI spec
	// 2. Validating parameters
	// 3. Making the HTTP request
	// 4. Returning the response

	return map[string]interface{}{
		"status":  "not_implemented",
		"message": "Action execution not yet implemented",
	}, nil
}

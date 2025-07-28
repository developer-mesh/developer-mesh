package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/jmoiron/sqlx"
)

// DiscoverySession represents an active discovery session
type DiscoverySession struct {
	ID             string                `json:"id"`
	TenantID       string                `json:"tenant_id"`
	SessionID      string                `json:"session_id"`
	BaseURL        string                `json:"base_url"`
	Status         string                `json:"status"`
	Suggestions    []DiscoverySuggestion `json:"suggestions"`
	DiscoveredURLs []string              `json:"discovered_urls"`
	SelectedURL    string                `json:"selected_url,omitempty"`
	ExpiresAt      time.Time             `json:"expires_at"`
}

// DiscoverySuggestion represents a potential API endpoint
type DiscoverySuggestion struct {
	URL        string  `json:"url"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
}

// DiscoveryService manages tool discovery sessions
type DiscoveryService struct {
	db           *sqlx.DB
	toolRegistry *ToolRegistry
	logger       observability.Logger
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(db *sqlx.DB, toolRegistry *ToolRegistry, logger observability.Logger) *DiscoveryService {
	return &DiscoveryService{
		db:           db,
		toolRegistry: toolRegistry,
		logger:       logger,
	}
}

// StartDiscovery initiates a new discovery session
func (s *DiscoveryService) StartDiscovery(
	ctx context.Context,
	tenantID string,
	sessionID string,
	baseURL string,
	hints map[string]interface{},
) (*DiscoverySession, error) {
	// Create temporary tool config for discovery
	config := tool.ToolConfig{
		TenantID: tenantID,
		Type:     "discovery",
		BaseURL:  baseURL,
		Config:   hints,
	}

	// If hints contain OpenAPI URL, use it directly
	if openAPIURL, ok := hints["openapi_url"].(string); ok {
		config.OpenAPIURL = openAPIURL
	}

	// Attempt discovery
	result, err := s.toolRegistry.GetOpenAPIAdapter().DiscoverAPIs(ctx, config)
	if err != nil {
		s.logger.Error("Discovery failed", map[string]interface{}{
			"error":    err.Error(),
			"base_url": baseURL,
		})
	}

	// Build suggestions based on discovery results
	suggestions := s.buildSuggestions(baseURL, result, hints)

	// Create discovery session
	session := &DiscoverySession{
		TenantID:       tenantID,
		SessionID:      sessionID,
		BaseURL:        baseURL,
		Status:         "needs_confirmation",
		Suggestions:    suggestions,
		DiscoveredURLs: result.DiscoveredURLs,
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	// If only one high-confidence suggestion, auto-select it
	if len(suggestions) == 1 && suggestions[0].Confidence >= 0.9 {
		session.Status = "auto_discovered"
		session.SelectedURL = suggestions[0].URL
	}

	// Store session in database
	sessJSON, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	query := `
		INSERT INTO tool_discovery_sessions 
		(tenant_id, session_id, base_url, status, discovered_urls, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err = s.db.ExecContext(ctx, query,
		tenantID, sessionID, baseURL, session.Status,
		sessJSON, session.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store discovery session: %w", err)
	}

	return session, nil
}

// ConfirmDiscovery confirms a discovery selection and creates the tool
func (s *DiscoveryService) ConfirmDiscovery(
	ctx context.Context,
	tenantID string,
	sessionID string,
	selectedURL string,
	toolName string,
	authToken string,
	authConfig map[string]interface{},
) (*tool.ToolConfig, error) {
	// Retrieve session
	var dbSession struct {
		BaseURL        string          `db:"base_url"`
		Status         string          `db:"status"`
		DiscoveredURLs json.RawMessage `db:"discovered_urls"`
		ExpiresAt      time.Time       `db:"expires_at"`
	}

	query := `
		SELECT base_url, status, discovered_urls, expires_at
		FROM tool_discovery_sessions
		WHERE tenant_id = $1 AND session_id = $2`

	err := s.db.GetContext(ctx, &dbSession, query, tenantID, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("discovery session not found")
		}
		return nil, fmt.Errorf("failed to retrieve session: %w", err)
	}

	// Check if session is expired
	if time.Now().After(dbSession.ExpiresAt) {
		return nil, fmt.Errorf("discovery session expired")
	}

	// Build credential
	var credential *tool.TokenCredential
	if authConfig != nil {
		credential = s.buildCredentialFromConfig(authConfig)
	} else {
		// Default to bearer token
		credential = &tool.TokenCredential{
			Type:  "bearer",
			Token: authToken,
		}
	}

	// Create tool configuration
	config := &tool.ToolConfig{
		TenantID:    tenantID,
		Type:        "openapi",
		Name:        toolName,
		DisplayName: toolName,
		BaseURL:     dbSession.BaseURL,
		OpenAPIURL:  selectedURL,
		Config: map[string]interface{}{
			"base_url":    dbSession.BaseURL,
			"openapi_url": selectedURL,
		},
		Credential:   credential,
		Status:       "active",
		HealthStatus: "unknown",
	}

	// Register the tool
	_, err = s.toolRegistry.RegisterTool(ctx, tenantID, config, "discovery")
	if err != nil {
		return nil, fmt.Errorf("failed to register tool: %w", err)
	}

	// Update session status
	_, err = s.db.ExecContext(ctx,
		"UPDATE tool_discovery_sessions SET status = 'completed', selected_url = $1 WHERE session_id = $2",
		selectedURL, sessionID,
	)
	if err != nil {
		s.logger.Error("Failed to update discovery session", map[string]interface{}{
			"error":      err.Error(),
			"session_id": sessionID,
		})
	}

	return config, nil
}

// GetSession retrieves a discovery session
func (s *DiscoveryService) GetSession(ctx context.Context, tenantID, sessionID string) (*DiscoverySession, error) {
	var dbSession struct {
		BaseURL        string          `db:"base_url"`
		Status         string          `db:"status"`
		DiscoveredURLs json.RawMessage `db:"discovered_urls"`
		SelectedURL    sql.NullString  `db:"selected_url"`
		ExpiresAt      time.Time       `db:"expires_at"`
	}

	query := `
		SELECT base_url, status, discovered_urls, selected_url, expires_at
		FROM tool_discovery_sessions
		WHERE tenant_id = $1 AND session_id = $2`

	err := s.db.GetContext(ctx, &dbSession, query, tenantID, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, err
	}

	session := &DiscoverySession{
		TenantID:  tenantID,
		SessionID: sessionID,
		BaseURL:   dbSession.BaseURL,
		Status:    dbSession.Status,
		ExpiresAt: dbSession.ExpiresAt,
	}

	if dbSession.SelectedURL.Valid {
		session.SelectedURL = dbSession.SelectedURL.String
	}

	// Unmarshal discovered URLs
	if len(dbSession.DiscoveredURLs) > 0 {
		var urls []string
		if err := json.Unmarshal(dbSession.DiscoveredURLs, &urls); err == nil {
			session.DiscoveredURLs = urls
		}
	}

	return session, nil
}

// buildSuggestions creates discovery suggestions based on results
func (s *DiscoveryService) buildSuggestions(baseURL string, result *tool.DiscoveryResult, hints map[string]interface{}) []DiscoverySuggestion {
	suggestions := []DiscoverySuggestion{}

	// If we found OpenAPI specs, add them with high confidence
	for _, url := range result.DiscoveredURLs {
		suggestions = append(suggestions, DiscoverySuggestion{
			URL:        url,
			Type:       "openapi",
			Confidence: 0.9,
		})
	}

	// Add common API documentation patterns with lower confidence
	if len(suggestions) == 0 {
		// Try subdomain patterns
		if parsed, err := url.Parse(baseURL); err == nil {
			subdomains := []string{"api", "apidocs", "docs", "developer"}
			for _, subdomain := range subdomains {
				suggestion := DiscoverySuggestion{
					URL:        fmt.Sprintf("https://%s.%s", subdomain, parsed.Host),
					Type:       "documentation",
					Confidence: 0.7,
				}
				suggestions = append(suggestions, suggestion)
			}
		}

		// Try common paths
		commonPaths := []string{
			"/api-docs",
			"/documentation",
			"/developers",
			"/api/documentation",
		}
		for _, path := range commonPaths {
			suggestions = append(suggestions, DiscoverySuggestion{
				URL:        baseURL + path,
				Type:       "documentation",
				Confidence: 0.5,
			})
		}
	}

	// If hints contain suggestions, add them
	if hintedURLs, ok := hints["suggested_urls"].([]interface{}); ok {
		for _, url := range hintedURLs {
			if urlStr, ok := url.(string); ok {
				suggestions = append(suggestions, DiscoverySuggestion{
					URL:        urlStr,
					Type:       "user_hint",
					Confidence: 0.8,
				})
			}
		}
	}

	return suggestions
}

// buildCredentialFromConfig creates credential from auth config
func (s *DiscoveryService) buildCredentialFromConfig(authConfig map[string]interface{}) *tool.TokenCredential {
	authType, _ := authConfig["type"].(string)
	if authType == "" {
		authType = "bearer"
	}

	cred := &tool.TokenCredential{
		Type: authType,
	}

	// Extract fields based on type
	switch authType {
	case "bearer", "token":
		cred.Token, _ = authConfig["token"].(string)
		cred.HeaderName, _ = authConfig["header_name"].(string)
		cred.HeaderPrefix, _ = authConfig["header_prefix"].(string)
	case "api_key":
		cred.APIKey, _ = authConfig["api_key"].(string)
		cred.HeaderName, _ = authConfig["header_name"].(string)
	case "basic":
		cred.Username, _ = authConfig["username"].(string)
		cred.Password, _ = authConfig["password"].(string)
	}

	return cred
}

// CleanupExpiredSessions removes expired discovery sessions
func (s *DiscoveryService) CleanupExpiredSessions(ctx context.Context) error {
	query := "DELETE FROM tool_discovery_sessions WHERE expires_at < $1"
	result, err := s.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup sessions: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		s.logger.Info("Cleaned up expired discovery sessions", map[string]interface{}{
			"count": rows,
		})
	}

	return nil
}

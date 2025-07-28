package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/handlers"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAuthMiddleware provides a mock authentication for testing
type MockAuthMiddleware struct{}

func (m *MockAuthMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set mock claims
		claims := &auth.Claims{
			UserID:   "test-user",
			TenantID: "test-tenant",
			Email:    "test@example.com",
		}
		c.Set("claims", claims)
		c.Next()
	}
}

func setupTestDB(t *testing.T) *sqlx.DB {
	// Use in-memory SQLite for testing
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create test schema
	schema := `
	CREATE TABLE tool_configurations (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		tool_type TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		display_name TEXT,
		config TEXT NOT NULL,
		credentials_encrypted BLOB,
		auth_type TEXT NOT NULL DEFAULT 'token',
		retry_policy TEXT,
		status TEXT DEFAULT 'active',
		health_status TEXT DEFAULT 'unknown',
		last_health_check TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_by TEXT,
		UNIQUE(tenant_id, tool_name)
	);

	CREATE TABLE tool_discovery_sessions (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		session_id TEXT UNIQUE,
		tool_type TEXT,
		base_url TEXT,
		status TEXT,
		discovered_urls TEXT,
		selected_url TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP
	);

	CREATE TABLE tool_executions (
		id TEXT PRIMARY KEY,
		tool_config_id TEXT REFERENCES tool_configurations(id),
		tenant_id TEXT NOT NULL,
		action TEXT,
		parameters TEXT,
		status TEXT,
		retry_count INTEGER DEFAULT 0,
		error TEXT,
		response_time_ms INTEGER,
		executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		executed_by TEXT
	);
	`

	_, err = db.Exec(schema)
	require.NoError(t, err)

	return db
}

func setupTestAPI(t *testing.T, db *sqlx.DB) (*gin.Engine, *handlers.DynamicToolAPI) {
	gin.SetMode(gin.TestMode)

	// Create services
	logger := observability.NewStandardLogger("test")
	credentialManager, err := services.NewCredentialManager("test-encryption-key-32-characters-long")
	require.NoError(t, err)

	healthChecker := services.NewHealthChecker(logger)
	toolService := services.NewToolService(db, credentialManager, logger)
	toolRegistry := services.NewToolRegistry(toolService, healthChecker, logger)
	retryHandler := services.NewRetryHandler(logger)
	discoveryService := services.NewDiscoveryService(db, toolRegistry, logger)
	executionService := services.NewExecutionService(db, toolRegistry, retryHandler, logger)

	// Create API handler
	api := handlers.NewDynamicToolAPI(
		toolRegistry,
		discoveryService,
		executionService,
		logger,
	)

	// Setup router
	router := gin.New()
	router.Use(gin.Recovery())

	// Add mock auth middleware
	mockAuth := &MockAuthMiddleware{}
	router.Use(mockAuth.Middleware())

	// Register routes
	v1 := router.Group("/api/v1")
	api.RegisterRoutes(v1)

	return router, api
}

func TestDynamicToolAPI_RegisterTool(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	router, _ := setupTestAPI(t, db)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
		wantError  string
	}{
		{
			name: "successful registration",
			payload: map[string]interface{}{
				"name":         "test-github",
				"display_name": "Test GitHub",
				"base_url":     "https://api.github.com",
				"auth_config": map[string]interface{}{
					"type":  "bearer",
					"token": "test-token",
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing required fields",
			payload: map[string]interface{}{
				"name": "test-tool",
				// Missing base_url and auth_config
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid request format",
		},
		{
			name: "invalid auth config",
			payload: map[string]interface{}{
				"name":     "test-tool",
				"base_url": "https://api.example.com",
				"auth_config": map[string]interface{}{
					"type": "invalid",
				},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid authentication configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/v1/tools", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantError != "" {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantError)
			}

			if tt.wantStatus == http.StatusCreated {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotEmpty(t, resp["id"])
				assert.Equal(t, tt.payload["name"], resp["name"])
				assert.Equal(t, "active", resp["status"])
			}
		})
	}
}

func TestDynamicToolAPI_ListTools(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	router, api := setupTestAPI(t, db)

	// Register a test tool
	config := &tool.ToolConfig{
		TenantID:    "test-tenant",
		Type:        "github",
		Name:        "test-github",
		DisplayName: "Test GitHub",
		BaseURL:     "https://api.github.com",
		Config: map[string]interface{}{
			"base_url": "https://api.github.com",
		},
		Credential: &tool.TokenCredential{
			Type:  "bearer",
			Token: "test-token",
		},
		Status:       "active",
		HealthStatus: "unknown",
	}

	registry := api.GetToolRegistry()
	_, err := registry.RegisterTool(context.Background(), "test-tenant", config, "test-user")
	require.NoError(t, err)

	// Test listing tools
	req := httptest.NewRequest("GET", "/api/v1/tools", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	tools := resp["tools"].([]interface{})
	assert.Len(t, tools, 1)

	tool := tools[0].(map[string]interface{})
	assert.Equal(t, "test-github", tool["name"])
	assert.Equal(t, "github", tool["type"])
	assert.Equal(t, "active", tool["status"])
}

func TestDynamicToolAPI_ExecuteAction(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	router, _ := setupTestAPI(t, db)

	// This would require mocking the HTTP client for actual execution
	// For now, we test the endpoint structure
	payload := map[string]interface{}{
		"context_id": "test-context",
		"parameters": map[string]interface{}{
			"repository": "test/repo",
			"title":      "Test Issue",
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/tools/test-github/actions/create_issue", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Would return 404 since tool doesn't exist
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDynamicToolAPI_HealthCheck(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	router, api := setupTestAPI(t, db)

	// Register a test tool
	config := &tool.ToolConfig{
		TenantID:    "test-tenant",
		Type:        "test",
		Name:        "test-tool",
		DisplayName: "Test Tool",
		BaseURL:     "http://localhost:9999", // Non-existent URL
		Config: map[string]interface{}{
			"base_url": "http://localhost:9999",
		},
		Credential: &tool.TokenCredential{
			Type:  "bearer",
			Token: "test-token",
		},
		Status:       "active",
		HealthStatus: "unknown",
	}

	registry := api.GetToolRegistry()
	_, err := registry.RegisterTool(context.Background(), "test-tenant", config, "test-user")
	require.NoError(t, err)

	// Test health check
	req := httptest.NewRequest("POST", "/api/v1/tools/test-tool/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "unhealthy", resp["status"]) // Should be unhealthy since URL doesn't exist
	assert.False(t, resp["cached"].(bool))
	assert.NotEmpty(t, resp["error"])
}

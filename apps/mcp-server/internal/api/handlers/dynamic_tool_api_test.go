//go:build integration
// +build integration

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/handlers"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core/tool"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/testutil"
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
		// Set mock claims with proper UUIDs
		claims := &auth.Claims{
			UserID:   testutil.TestUserIDString(),
			TenantID: testutil.TestTenantIDString(),
			Email:    "test@example.com",
		}
		c.Set("claims", claims)
		c.Set("tenant_id", testutil.TestTenantIDString())
		c.Next()
	}
}

func setupTestDB(t *testing.T) *sqlx.DB {
	// Skip if not in integration test mode or if SKIP_DB_TESTS is set
	if testing.Short() || os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test")
	}

	// Use PostgreSQL test container - use 127.0.0.1 instead of localhost to avoid IPv6
	dbURL := "postgres://test:test@127.0.0.1:5433/test?sslmode=disable"
	if envURL := os.Getenv("TEST_DATABASE_URL"); envURL != "" {
		dbURL = envURL
	}

	db, err := sqlx.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("Skipping test, cannot connect to test database: %v", err)
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		t.Skipf("Skipping test, cannot ping test database: %v. Run 'docker-compose -f docker-compose.test.yml up -d' to start test database.", err)
	}

	// Clean up any existing tables (check both public and mcp schemas)
	_, _ = db.Exec("DROP TABLE IF EXISTS mcp.tool_executions CASCADE")
	_, _ = db.Exec("DROP TABLE IF EXISTS mcp.tool_discovery_sessions CASCADE")
	_, _ = db.Exec("DROP TABLE IF EXISTS mcp.tool_configurations CASCADE")
	_, _ = db.Exec("DROP TABLE IF EXISTS tool_executions CASCADE")
	_, _ = db.Exec("DROP TABLE IF EXISTS tool_discovery_sessions CASCADE")
	_, _ = db.Exec("DROP TABLE IF EXISTS tool_configurations CASCADE")

	// Create mcp schema if it doesn't exist
	_, _ = db.Exec("CREATE SCHEMA IF NOT EXISTS mcp")

	// Create test schema matching the actual migration
	schema := `
	CREATE TABLE mcp.tool_configurations (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		tool_type TEXT NOT NULL,
		display_name TEXT,
		base_url TEXT,
		config JSONB,
		auth_config JSONB DEFAULT '{}',
		credentials_encrypted BYTEA,
		auth_type TEXT NOT NULL DEFAULT 'token',
		retry_policy JSONB,
		status TEXT DEFAULT 'active',
		health_status TEXT DEFAULT 'unknown',
		last_health_check TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_by TEXT,
		UNIQUE(tenant_id, tool_name)
	);

	CREATE TABLE mcp.tool_discovery_sessions (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		session_id TEXT UNIQUE,
		tool_type TEXT,
		base_url TEXT,
		status TEXT,
		discovered_urls JSONB,
		selected_url TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP
	);

	CREATE TABLE mcp.tool_executions (
		id TEXT PRIMARY KEY,
		tool_config_id TEXT REFERENCES mcp.tool_configurations(id),
		tenant_id TEXT NOT NULL,
		action TEXT,
		parameters JSONB,
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

	router := gin.New()

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

	// Setup router with middleware
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

	// Start a mock server for API discovery
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle all common OpenAPI paths
		if strings.HasSuffix(r.URL.Path, "/openapi.json") ||
			strings.HasSuffix(r.URL.Path, "/swagger.json") ||
			strings.Contains(r.URL.Path, "/api") && strings.Contains(r.URL.Path, "openapi") {
			w.Header().Set("Content-Type", "application/json")
			spec := map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "Mock GitHub API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{
					"/user": map[string]interface{}{
						"get": map[string]interface{}{
							"operationId": "getUser",
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "Success",
								},
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(spec)
			return
		}
		// Default response
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

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
				"base_url":     mockServer.URL + "/mock-github",
				"auth_config": map[string]interface{}{
					"type":  "bearer",
					"token": "test-token",
				},
				"config": map[string]interface{}{
					"timeout": 30,
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

	// Start a mock server for API discovery
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openapi.json") {
			w.Header().Set("Content-Type", "application/json")
			spec := map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "Mock GitHub API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{},
			}
			_ = json.NewEncoder(w).Encode(spec)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	router, api := setupTestAPI(t, db)

	// Register a test tool
	config := &tool.ToolConfig{
		TenantID:    testutil.TestTenantIDString(),
		Type:        "github",
		Name:        "test-github",
		DisplayName: "Test GitHub",
		BaseURL:     mockServer.URL,
		Config: map[string]interface{}{
			"base_url": mockServer.URL,
		},
		Credential: &tool.TokenCredential{
			Type:  "bearer",
			Token: "test-token",
		},
		Status:       "active",
		HealthStatus: "unknown",
	}

	registry := api.GetToolRegistry()
	_, err := registry.RegisterTool(context.Background(), testutil.TestTenantIDString(), config, testutil.TestUserIDString())
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

	// Start a mock server for API discovery
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openapi.json") {
			w.Header().Set("Content-Type", "application/json")
			spec := map[string]interface{}{
				"openapi": "3.0.0",
				"info": map[string]interface{}{
					"title":   "Mock Test API",
					"version": "1.0.0",
				},
				"paths": map[string]interface{}{},
			}
			_ = json.NewEncoder(w).Encode(spec)
			return
		}
		// For health check - return error to simulate unhealthy tool
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer mockServer.Close()

	router, api := setupTestAPI(t, db)

	// Register a test tool
	config := &tool.ToolConfig{
		TenantID:    testutil.TestTenantIDString(),
		Type:        "test",
		Name:        "test-tool",
		DisplayName: "Test Tool",
		BaseURL:     mockServer.URL,
		Config: map[string]interface{}{
			"base_url": mockServer.URL,
		},
		Credential: &tool.TokenCredential{
			Type:  "bearer",
			Token: "test-token",
		},
		Status:       "active",
		HealthStatus: "unknown",
	}

	registry := api.GetToolRegistry()
	_, err := registry.RegisterTool(context.Background(), testutil.TestTenantIDString(), config, testutil.TestUserIDString())
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

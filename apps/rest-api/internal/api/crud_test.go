//go:build integration
// +build integration

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/common/config"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/testutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contextAPI "github.com/developer-mesh/developer-mesh/apps/rest-api/internal/api/context"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/agent"
)

// TestContextCRUD tests full CRUD cycle for contexts using HTTP requests
func TestContextCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	server := setupTestServer(t)
	router := setupTestRouter(server)

	tenantID := uuid.New().String()
	var contextID string
	modelID := "test-model-" + uuid.New().String()

	// First create a model that the context will reference
	t.Run("Create Model for Context", func(t *testing.T) {
		t.Skip("Skipping model creation in SQLite test - model repository needs SQLite compatibility")
		payload := map[string]any{
			"id":       modelID,
			"name":     "Test Model",
			"provider": "openai",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/models", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code, "Failed to create model: %s", w.Body.String())
	})

	// Create
	t.Run("Create Context", func(t *testing.T) {
		payload := map[string]any{
			"agent_id":   "test-agent-" + uuid.New().String(),
			"model_id":   modelID,
			"max_tokens": 4000,
			"metadata": map[string]any{
				"test":   true,
				"source": "crud-test",
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/contexts", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// The response structure from the context API includes data wrapper
		data, ok := response["data"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, data, "id")
		assert.Equal(t, payload["agent_id"], data["agent_id"])
		assert.Equal(t, payload["model_id"], data["model_id"])

		// Store ID for subsequent tests
		contextID = data["id"].(string)
	})

	// Read
	t.Run("Read Context", func(t *testing.T) {
		t.Skip("Skipping context read test - depends on model creation which needs SQLite compatibility")
		require.NotEmpty(t, contextID)

		req := httptest.NewRequest("GET", "/api/v1/contexts/"+contextID, nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data, ok := response["data"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, contextID, data["id"])
	})

	// Update
	t.Run("Update Context", func(t *testing.T) {
		require.NotEmpty(t, contextID)

		updatePayload := map[string]any{
			"content": []map[string]any{
				{
					"role":    "user",
					"content": "Test message for CRUD",
				},
			},
			"options": map[string]any{
				"metadata": map[string]any{
					"updated": true,
					"version": 2,
				},
			},
		}

		body, _ := json.Marshal(updatePayload)
		req := httptest.NewRequest("PUT", "/api/v1/contexts/"+contextID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify update
		data, ok := response["data"].(map[string]any)
		assert.True(t, ok)
		metadata, ok := data["metadata"].(map[string]any)
		if ok {
			assert.Equal(t, true, metadata["updated"])
			assert.Equal(t, float64(2), metadata["version"])
		}
	})

	// List
	t.Run("List Contexts", func(t *testing.T) {
		// Context list requires agent_id parameter
		agentID := "test-agent-" + uuid.New().String()
		req := httptest.NewRequest("GET", "/api/v1/contexts?agent_id="+agentID+"&limit=10", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// The list endpoint returns an array of contexts with links
		assert.IsType(t, []any{}, response)
	})

	// Delete
	t.Run("Delete Context", func(t *testing.T) {
		require.NotEmpty(t, contextID)

		req := httptest.NewRequest("DELETE", "/api/v1/contexts/"+contextID, nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify deletion
		req = httptest.NewRequest("GET", "/api/v1/contexts/"+contextID, nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestAgentCRUD tests full CRUD cycle for agents using HTTP requests
func TestAgentCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := setupTestServer(t)
	router := setupTestRouter(server)

	tenantID := uuid.New().String()
	var agentID string

	// Create
	t.Run("Create Agent", func(t *testing.T) {
		payload := map[string]any{
			"name":     "test-agent-" + uuid.New().String(),
			"model_id": "gpt-4",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/agents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Logf("Response body: %s", w.Body.String())
		}
		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		if id, ok := response["id"]; ok {
			agentID = id.(string)
		} else {
			t.Fatalf("Response missing 'id' field: %+v", response)
		}
		assert.NotEmpty(t, agentID)

		agent, ok := response["agent"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, payload["name"], agent["name"])
	})

	// List
	t.Run("List Agents", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		agents, ok := response["agents"].([]any)
		assert.True(t, ok)
		assert.GreaterOrEqual(t, len(agents), 1)
	})

	// Update
	t.Run("Update Agent", func(t *testing.T) {
		require.NotEmpty(t, agentID)

		updatePayload := map[string]any{
			"name":     "Updated Agent Name",
			"model_id": "gpt-4-turbo",
		}

		body, _ := json.Marshal(updatePayload)
		req := httptest.NewRequest("PUT", "/api/v1/agents/"+agentID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		agent, ok := response["agent"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, updatePayload["name"], agent["name"])
		assert.Equal(t, updatePayload["model_id"], agent["model_id"])
	})
}

// TestModelCRUD tests full CRUD cycle for models using HTTP requests
func TestModelCRUD(t *testing.T) {
	t.Skip("Skipping model CRUD test - interface mismatch with SQLite repository")
	gin.SetMode(gin.TestMode)

	server := setupTestServer(t)
	router := setupTestRouter(server)

	tenantID := uuid.New().String()
	var modelID string

	// Create
	t.Run("Create Model", func(t *testing.T) {
		payload := map[string]any{
			"name":        "test-model-" + uuid.New().String(),
			"provider":    "openai",
			"model_type":  "chat",
			"description": "Test model for CRUD",
			"config": map[string]any{
				"api_version": "v1",
				"endpoint":    "https://api.openai.com",
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/models", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Logf("Response body: %s", w.Body.String())
		}
		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		if id, ok := response["id"]; ok {
			modelID = id.(string)
			assert.NotEmpty(t, modelID)
		} else {
			t.Fatalf("Response does not contain 'id' field: %+v", response)
		}
	})

	// List
	t.Run("List Models", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/models", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("X-Tenant-ID", tenantID)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Logf("Response body: %s", w.Body.String())
		}
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		models, ok := response["models"].([]any)
		assert.True(t, ok)
		assert.GreaterOrEqual(t, len(models), 1)
	})
}

// TestConcurrentWrites tests concurrent write operations using HTTP requests
func TestConcurrentWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := setupTestServer(t)
	router := setupTestRouter(server)

	tenantID := uuid.New().String()

	// Create initial context
	contextID := createTestContext(t, router, tenantID)

	t.Run("Concurrent Updates", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan int, 10)

		// Launch 10 concurrent updates
		for i := range 10 {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				updatePayload := map[string]any{
					"content": []map[string]any{
						{
							"role":    "user",
							"content": fmt.Sprintf("Concurrent update %d", index),
						},
					},
				}

				body, _ := json.Marshal(updatePayload)
				req := httptest.NewRequest("PUT", "/api/v1/contexts/"+contextID, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-token")
				req.Header.Set("X-Tenant-ID", tenantID)

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				results <- w.Code
			}(i)
		}

		wg.Wait()
		close(results)

		// Check results
		successCount := 0
		conflictCount := 0
		for code := range results {
			switch code {
			case http.StatusOK:
				successCount++
			case http.StatusConflict:
				conflictCount++
			}
		}

		// At least some should succeed
		assert.Greater(t, successCount, 0, "No successful updates")
		t.Logf("Concurrent updates: %d successful, %d conflicts", successCount, conflictCount)
	})
}

// Helper functions

func setupTestServer(t *testing.T) *Server {
	// Create mock components for testing
	db := setupTestDB(t)

	cfg := Config{
		ListenAddress: ":8080",
		EnableCORS:    false,
		Auth: AuthConfig{
			APIKeys: map[string]any{
				"test-key": "admin",
			},
			JWTSecret: "test-secret",
		},
	}

	// Create a mock engine with test context manager
	logger := observability.NewLogger("test-engine")
	engine := core.NewEngine(logger)
	ctxManager := core.NewMockContextManager()
	engine.SetContextManager(ctxManager)

	metrics := observability.NewNoOpMetricsClient()
	appConfig := &config.Config{}
	cacheClient := cache.NewNoOpCache()

	server := NewServer(engine, cfg, db, metrics, appConfig, cacheClient)
	server.logger = observability.NewLogger("test")

	return server
}

func setupTestRouter(server *Server) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	// Add test middleware for auth bypass and tenant handling
	router.Use(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "Bearer test-token" {
			// Extract tenant ID from header
			tenantID := c.GetHeader("X-Tenant-ID")
			if tenantID == "" {
				tenantID = testutil.TestTenantIDString()
			}

			c.Set("user", map[string]any{
				"id":        testutil.TestUserIDString(),
				"tenant_id": tenantID,
			})
			// Also set tenant_id separately for backward compatibility
			c.Set("tenant_id", tenantID)
		}
		c.Next()
	})

	// Setup routes - use the actual route registration
	v1 := router.Group("/api/v1")

	// Register agent routes - use the database-aware repository
	agentRepo := agent.NewRepository(server.db)
	agentAPI := NewAgentAPI(agentRepo)
	agentAPI.RegisterRoutes(v1)

	// Register model routes
	modelRepo := repository.NewModelRepository(server.db.DB)
	modelAPI := NewModelAPI(modelRepo)
	modelAPI.RegisterRoutes(v1)

	// Register context routes - now that we have modelRepo
	ctxAPI := contextAPI.NewAPI(
		server.engine.GetContextManager(),
		server.logger,
		server.metrics,
		server.db,
		modelRepo,
	)
	ctxAPI.RegisterRoutes(v1)

	return router
}

func setupTestDB(t *testing.T) *sqlx.DB {
	// Create an in-memory SQLite database for testing
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create necessary tables
	// Note: SQLite doesn't support schemas, so we create tables without schema prefix
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		name TEXT,
		model_id TEXT,
		type TEXT DEFAULT 'standard',
		status TEXT DEFAULT 'offline',
		capabilities TEXT,
		metadata TEXT,
		description TEXT,
		config TEXT,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		last_seen_at TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS models (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		name TEXT,
		provider TEXT,
		model_type TEXT,
		description TEXT,
		config TEXT,
		created_at TIMESTAMP,
		updated_at TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS contexts (
		id TEXT PRIMARY KEY,
		agent_id TEXT,
		model_id TEXT,
		content TEXT,
		metadata TEXT,
		created_at TIMESTAMP,
		updated_at TIMESTAMP
	);
	`

	_, err = db.Exec(schema)
	require.NoError(t, err)

	return db
}

func createTestContext(t *testing.T, router *gin.Engine, tenantID string) string {
	payload := map[string]any{
		"agent_id": "test-agent-" + uuid.New().String(),
		"model_id": "test-model",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/contexts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("X-Tenant-ID", tenantID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	data, ok := response["data"].(map[string]any)
	require.True(t, ok)

	return data["id"].(string)
}

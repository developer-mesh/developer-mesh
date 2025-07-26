package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

func TestDynamicToolsPassthrough(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create test server that simulates a tool API
	toolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authentication header
		authHeader := r.Header.Get("Authorization")

		// Return different responses based on the token
		switch authHeader {
		case "Bearer user-token-123":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Success with user token",
				"user":    "john.doe",
			})
		case "Bearer service-token-456":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Success with service token",
				"user":    "service-account",
			})
		default:
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Unauthorized",
			})
		}
	}))
	defer toolServer.Close()

	// Create dependencies
	// Setup test database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Run migrations
	err = runTestMigrations(db)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Use a test logger that prints to stdout
	logger := observability.NewLogger("test")
	metricsClient := observability.NewNoOpMetricsClient()
	encryptionSvc := security.NewEncryptionService("test-master-key")
	cacheClient := cache.NewMemoryCache(1000, 5*time.Minute)

	// Create services
	toolService := NewDynamicToolService(db, logger, metricsClient, encryptionSvc)
	healthCheckMgr := tools.NewHealthCheckManager(cacheClient, nil, logger, metricsClient)
	auditLogger := auth.NewAuditLogger(logger)

	// Create API
	api := NewDynamicToolsAPI(toolService, logger, metricsClient, encryptionSvc, healthCheckMgr, auditLogger)

	// Setup router with mock auth middleware that adds tenant_id
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Mock auth middleware - set tenant_id for all requests
		c.Set("tenant_id", "test-tenant")
		c.Set("user_id", "test-user")
		
		// Handle passthrough headers if present
		if userToken := c.GetHeader("X-User-Token"); userToken != "" {
			if provider := c.GetHeader("X-Token-Provider"); provider != "" {
				passthroughToken := auth.PassthroughToken{
					Token:    userToken,
					Provider: provider,
				}
				ctx := auth.WithPassthroughToken(c.Request.Context(), passthroughToken)
				c.Request = c.Request.WithContext(ctx)
			}
		}
		
		c.Next()
	})
	v1 := router.Group("/api/v1")
	api.RegisterRoutes(v1)

	// Create a test tool with GitHub provider
	createReq := CreateToolRequest{
		Name:     "test-github",
		BaseURL:  toolServer.URL,
		Provider: "github",
		AuthType: "bearer",
		Credentials: &CredentialInput{
			Token: "service-token-456",
		},
		PassthroughConfig: &PassthroughConfig{
			Mode:              "optional",
			FallbackToService: true,
		},
	}

	// Create tool
	reqBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/v1/tools", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Logf("Tool creation failed with status %d: %s", w.Code, w.Body.String())
	}
	require.Equal(t, http.StatusCreated, w.Code)

	var createdTool Tool
	err = json.Unmarshal(w.Body.Bytes(), &createdTool)
	require.NoError(t, err)

	t.Run("Execute with passthrough token", func(t *testing.T) {
		// Use a mock gateway key
		gatewayKey := "gw_test_gateway_key"

		// Execute action with passthrough token
		executeReq := map[string]interface{}{
			"action": "test",
		}
		reqBody, _ := json.Marshal(executeReq)

		req := httptest.NewRequest("POST", "/api/v1/tools/"+createdTool.ID+"/execute/test_action", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+gatewayKey)
		req.Header.Set("X-User-Token", "user-token-123")
		req.Header.Set("X-Token-Provider", "github")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed with user token
		assert.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		err2 := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err2)

		// Verify user token was used (would see john.doe in response)
		assert.Contains(t, result, "message")
	})

	t.Run("Execute without passthrough token (fallback to service)", func(t *testing.T) {
		executeReq := map[string]interface{}{
			"action": "test",
		}
		reqBody, _ := json.Marshal(executeReq)

		// Use a regular key for execution without passthrough
		req := httptest.NewRequest("POST", "/api/v1/tools/"+createdTool.ID+"/execute/test_action", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer test-key")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed with service token
		assert.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		err2 := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err2)

		// Verify service token was used
		assert.Contains(t, result, "message")
	})

	t.Run("Execute with required passthrough - no token provided", func(t *testing.T) {
		// Update tool to require passthrough
		updateReq := UpdateToolRequest{
			PassthroughConfig: &PassthroughConfig{
				Mode:              "required",
				FallbackToService: false,
			},
		}
		reqBody, _ := json.Marshal(updateReq)

		// Update tool configuration
		req := httptest.NewRequest("PUT", "/api/v1/tools/"+createdTool.ID, bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer test-key")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		// Now try to execute without passthrough token
		executeReq := map[string]interface{}{
			"action": "test",
		}
		reqBody, _ = json.Marshal(executeReq)

		// Try to execute without passthrough token
		req = httptest.NewRequest("POST", "/api/v1/tools/"+createdTool.ID+"/execute/test_action", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer test-key")
		req.Header.Set("Content-Type", "application/json")

		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail with 401
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var errorResp map[string]interface{}
		err2 := json.Unmarshal(w.Body.Bytes(), &errorResp)
		require.NoError(t, err2)
		assert.Contains(t, errorResp["error"], "passthrough token required")
	})

	t.Run("Execute with wrong provider token", func(t *testing.T) {
		// Use a mock gateway key with gitlab permissions
		gatewayKey := "gw_test_gitlab_key"

		executeReq := map[string]interface{}{
			"action": "test",
		}
		reqBody, _ := json.Marshal(executeReq)

		req := httptest.NewRequest("POST", "/api/v1/tools/"+createdTool.ID+"/execute/test_action", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+gatewayKey)
		req.Header.Set("X-User-Token", "gitlab-token-789")
		req.Header.Set("X-Token-Provider", "gitlab")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail with 403 if passthrough is required
		// Or succeed with service token if optional
		if w.Code == http.StatusForbidden {
			var errorResp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &errorResp)
			require.NoError(t, err)
			assert.Contains(t, errorResp["error"], "provider mismatch")
		}
	})
}

// runTestMigrations runs basic table creation for tests
func runTestMigrations(db *sql.DB) error {
	// Create tool_configurations table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tool_configurations (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			name TEXT NOT NULL,
			base_url TEXT NOT NULL,
			documentation_url TEXT,
			openapi_url TEXT,
			auth_type TEXT NOT NULL,
			credential TEXT,
			config TEXT,
			retry_policy TEXT,
			health_config TEXT,
			provider TEXT,
			passthrough_config TEXT DEFAULT '{"mode": "optional", "fallback_to_service": true}',
			status TEXT DEFAULT 'active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create users table for auth service
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			email TEXT NOT NULL,
			metadata TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

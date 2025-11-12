//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/agents"
	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmbeddingIntegration tests the complete embedding integration including REST API and MCP endpoints
func TestEmbeddingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip if no API key is configured
	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("TEST_WITH_MOCKS") != "true" {
		t.Skip("Skipping integration test: OPENAI_API_KEY not set and TEST_WITH_MOCKS not true")
	}

	ctx := context.Background()

	// API endpoints
	restAPIURL := getEnvOrDefault("REST_API_URL", "http://localhost:8081")
	mcpServerURL := getEnvOrDefault("MCP_SERVER_URL", "http://localhost:8080")
	apiKey := getEnvOrDefault("API_KEY", "dev-api-key")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	t.Run("REST API Embedding Flow", func(t *testing.T) {
		agentID := fmt.Sprintf("test-agent-%d", time.Now().Unix())

		// Step 1: Check provider health
		t.Run("Provider Health Check", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, "GET",
				restAPIURL+"/api/embeddings/providers/health", nil)
			require.NoError(t, err)

			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var healthResp struct {
				Providers map[string]embedding.ProviderHealth `json:"providers"`
				Timestamp string                              `json:"timestamp"`
			}
			err = json.NewDecoder(resp.Body).Decode(&healthResp)
			require.NoError(t, err)

			// At least one provider should be configured
			assert.NotEmpty(t, healthResp.Providers)

			// Check if any provider is healthy
			hasHealthyProvider := false
			for _, health := range healthResp.Providers {
				if health.Status == "healthy" {
					hasHealthyProvider = true
					break
				}
			}
			assert.True(t, hasHealthyProvider, "At least one provider should be healthy")
		})

		// Step 2: Create agent configuration
		t.Run("Create Agent Configuration", func(t *testing.T) {
			agentConfig := agents.AgentConfig{
				AgentID:           agentID,
				EmbeddingStrategy: agents.StrategyQuality,
				ModelPreferences: []agents.ModelPreference{
					{
						TaskType:       agents.TaskTypeGeneralQA,
						PrimaryModels:  []string{"text-embedding-3-small"},
						FallbackModels: []string{"text-embedding-ada-002"},
					},
				},
				Constraints: agents.AgentConstraints{
					MaxCostPerMonthUSD: 100.0,
					RateLimits: agents.RateLimitConfig{
						RequestsPerMinute: 100,
					},
				},
			}

			body, err := json.Marshal(agentConfig)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				restAPIURL+"/api/embeddings/agents", bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusCreated, resp.StatusCode)

			var createdConfig agents.AgentConfig
			err = json.NewDecoder(resp.Body).Decode(&createdConfig)
			require.NoError(t, err)

			assert.Equal(t, agentID, createdConfig.AgentID)
			assert.True(t, createdConfig.IsActive)
		})

		// Step 3: Generate embedding through REST API
		var embeddingID uuid.UUID
		t.Run("Generate Embedding via REST", func(t *testing.T) {
			embReq := embedding.GenerateEmbeddingRequest{
				AgentID:  agentID,
				Text:     "This is a test embedding for integration testing",
				TaskType: agents.TaskTypeGeneralQA,
			}

			body, err := json.Marshal(embReq)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				restAPIURL+"/api/embeddings", bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var embResp embedding.GenerateEmbeddingResponse
			err = json.NewDecoder(resp.Body).Decode(&embResp)
			require.NoError(t, err)

			// Verify response
			assert.NotEmpty(t, embResp.EmbeddingID)
			assert.NotEmpty(t, embResp.ModelUsed)
			assert.NotEmpty(t, embResp.Provider)
			assert.Greater(t, embResp.Dimensions, 0)
			assert.Greater(t, embResp.GenerationTimeMs, int64(0))

			embeddingID = embResp.EmbeddingID
		})

		// Step 4: Test batch generation
		t.Run("Batch Generate Embeddings", func(t *testing.T) {
			batchReqs := []embedding.GenerateEmbeddingRequest{
				{
					AgentID:  agentID,
					Text:     "First batch embedding text",
					TaskType: agents.TaskTypeGeneralQA,
				},
				{
					AgentID:  agentID,
					Text:     "Second batch embedding text",
					TaskType: agents.TaskTypeGeneralQA,
				},
				{
					AgentID:  agentID,
					Text:     "Third batch embedding text",
					TaskType: agents.TaskTypeGeneralQA,
				},
			}

			body, err := json.Marshal(batchReqs)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				restAPIURL+"/api/embeddings/batch", bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var batchResp struct {
				Embeddings []embedding.GenerateEmbeddingResponse `json:"embeddings"`
				Count      int                                   `json:"count"`
			}
			err = json.NewDecoder(resp.Body).Decode(&batchResp)
			require.NoError(t, err)

			assert.Equal(t, 3, batchResp.Count)
			assert.Len(t, batchResp.Embeddings, 3)

			for _, emb := range batchResp.Embeddings {
				assert.NotEmpty(t, emb.EmbeddingID)
				assert.Equal(t, embResp.ModelUsed, emb.ModelUsed)
			}
		})

		// Step 5: Test search functionality (if implemented)
		t.Run("Search Embeddings", func(t *testing.T) {
			searchReq := embedding.SearchRequest{
				AgentID: agentID,
				Query:   "test integration",
				Limit:   10,
			}

			body, err := json.Marshal(searchReq)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				restAPIURL+"/api/embeddings/search", bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Search might not be implemented yet
			if resp.StatusCode == http.StatusNotImplemented {
				t.Skip("Search functionality not yet implemented")
			}

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var searchResp embedding.SearchResponse
			err = json.NewDecoder(resp.Body).Decode(&searchResp)
			require.NoError(t, err)

			// Should find at least the embeddings we just created
			assert.GreaterOrEqual(t, len(searchResp.Results), 1)
		})

		// Step 6: Get agent configuration
		t.Run("Get Agent Configuration", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, "GET",
				fmt.Sprintf("%s/api/embeddings/agents/%s", restAPIURL, agentID), nil)
			require.NoError(t, err)

			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var config agents.AgentConfig
			err = json.NewDecoder(resp.Body).Decode(&config)
			require.NoError(t, err)

			assert.Equal(t, agentID, config.AgentID)
			assert.Equal(t, agents.StrategyQuality, config.EmbeddingStrategy)
		})

		// Step 7: Update agent configuration
		t.Run("Update Agent Configuration", func(t *testing.T) {
			updateReq := agents.ConfigUpdateRequest{
				EmbeddingStrategy: agents.StrategyBalanced,
			}

			body, err := json.Marshal(updateReq)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "PUT",
				fmt.Sprintf("%s/api/embeddings/agents/%s", restAPIURL, agentID),
				bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var updatedConfig agents.AgentConfig
			err = json.NewDecoder(resp.Body).Decode(&updatedConfig)
			require.NoError(t, err)

			assert.Equal(t, agents.StrategyBalanced, updatedConfig.EmbeddingStrategy)
		})
	})

	t.Run("MCP Server Embedding Flow", func(t *testing.T) {
		agentID := fmt.Sprintf("mcp-test-agent-%d", time.Now().Unix())

		// First create agent config via REST API
		t.Run("Setup Agent for MCP", func(t *testing.T) {
			agentConfig := agents.AgentConfig{
				AgentID:           agentID,
				EmbeddingStrategy: agents.StrategySpeed,
				ModelPreferences: []agents.ModelPreference{
					{
						TaskType:      agents.TaskTypeGeneralQA,
						PrimaryModels: []string{"text-embedding-3-small"},
					},
				},
			}

			body, err := json.Marshal(agentConfig)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				restAPIURL+"/api/embeddings/agents", bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		})

		// Test MCP embedding generation
		t.Run("Generate Embedding via MCP", func(t *testing.T) {
			mcpReq := map[string]interface{}{
				"action":   "generate_embedding",
				"agent_id": agentID,
				"text":     "MCP test embedding generation",
			}

			body, err := json.Marshal(mcpReq)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				mcpServerURL+"/api/v1/request", bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// MCP server might not be running in test environment
			if resp.StatusCode == http.StatusServiceUnavailable ||
				resp.StatusCode == http.StatusNotFound {
				t.Skip("MCP server not available")
			}

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var mcpResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&mcpResp)
			require.NoError(t, err)

			assert.NotEmpty(t, mcpResp["embedding_id"])
			assert.NotEmpty(t, mcpResp["model_used"])
			assert.NotEmpty(t, mcpResp["provider"])
		})

		// Test MCP provider health
		t.Run("Provider Health via MCP", func(t *testing.T) {
			mcpReq := map[string]interface{}{
				"action": "provider_health",
			}

			body, err := json.Marshal(mcpReq)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				mcpServerURL+"/api/v1/request", bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusServiceUnavailable ||
				resp.StatusCode == http.StatusNotFound {
				t.Skip("MCP server not available")
			}

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var healthResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&healthResp)
			require.NoError(t, err)

			assert.NotEmpty(t, healthResp["providers"])
		})
	})

	// Test error scenarios
	t.Run("Error Handling", func(t *testing.T) {
		t.Run("Invalid Agent ID", func(t *testing.T) {
			embReq := embedding.GenerateEmbeddingRequest{
				AgentID: "non-existent-agent",
				Text:    "This should fail",
			}

			body, err := json.Marshal(embReq)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				restAPIURL+"/api/embeddings", bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return error for non-existent agent
			assert.GreaterOrEqual(t, resp.StatusCode, 400)
		})

		t.Run("Missing Required Fields", func(t *testing.T) {
			// Missing agent_id
			embReq := map[string]interface{}{
				"text": "Missing agent ID",
			}

			body, err := json.Marshal(embReq)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				restAPIURL+"/api/embeddings", bytes.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run("Unauthorized Access", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, "GET",
				restAPIURL+"/api/embeddings/providers/health", nil)
			require.NoError(t, err)

			// No API key

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	})
}

// Helper function to get environment variable with default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

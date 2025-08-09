//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/suite"
)

// EmbeddingE2ESuite tests the complete embedding flow from API to WebSocket
type EmbeddingE2ESuite struct {
	suite.Suite
	ctx          context.Context
	apiBaseURL   string
	wsBaseURL    string
	apiKey       string
	httpClient   *http.Client
	wsConn       *websocket.Conn
	testTenantID string
	testAgentID  string
}

// SetupSuite runs once before all tests
func (s *EmbeddingE2ESuite) SetupSuite() {
	s.ctx = context.Background()

	// Get endpoints from environment or use defaults
	s.apiBaseURL = os.Getenv("E2E_API_URL")
	if s.apiBaseURL == "" {
		s.apiBaseURL = "http://localhost:8081"
	}

	s.wsBaseURL = os.Getenv("E2E_WS_URL")
	if s.wsBaseURL == "" {
		s.wsBaseURL = "ws://localhost:8080/ws"
	}

	s.apiKey = os.Getenv("E2E_API_KEY")
	if s.apiKey == "" {
		s.apiKey = "test-api-key-123"
	}

	s.testTenantID = uuid.New().String()
	s.testAgentID = uuid.New().String()

	// Create HTTP client with timeout
	s.httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}
}

// TearDownSuite runs once after all tests
func (s *EmbeddingE2ESuite) TearDownSuite() {
	if s.wsConn != nil {
		s.wsConn.Close()
	}
}

// TestCompleteEmbeddingFlow tests the entire embedding workflow
func (s *EmbeddingE2ESuite) TestCompleteEmbeddingFlow() {
	// Step 1: List available models via REST API
	s.T().Log("Step 1: Listing available embedding models")
	models := s.listEmbeddingModels()
	s.NotEmpty(models, "Should have at least one embedding model available")

	// Select first available model
	var selectedModel map[string]interface{}
	for _, model := range models {
		if modelMap, ok := model.(map[string]interface{}); ok {
			if available, _ := modelMap["is_available"].(bool); available {
				selectedModel = modelMap
				break
			}
		}
	}
	s.NotNil(selectedModel, "Should find at least one available model")
	modelID := selectedModel["model_id"].(string)
	s.T().Logf("Selected model: %s", modelID)

	// Step 2: Configure model for tenant
	s.T().Log("Step 2: Configuring model for tenant")
	s.configureTenantModel(selectedModel["id"].(string))

	// Step 3: Connect via WebSocket
	s.T().Log("Step 3: Establishing WebSocket connection")
	s.connectWebSocket()

	// Step 4: Register agent via WebSocket
	s.T().Log("Step 4: Registering agent")
	s.registerAgent()

	// Step 5: Generate embedding via WebSocket
	s.T().Log("Step 5: Generating embedding")
	embeddingResult := s.generateEmbedding("This is a test text for embedding generation", modelID)
	s.NotNil(embeddingResult)
	s.True(embeddingResult["success"].(bool), "Embedding generation should succeed")

	// Step 6: Check usage via REST API
	s.T().Log("Step 6: Checking usage statistics")
	usage := s.getUsageStats()
	s.NotNil(usage)

	// Step 7: Test model switching
	s.T().Log("Step 7: Testing model switching")
	s.testModelSwitching()

	// Step 8: Test quota enforcement
	s.T().Log("Step 8: Testing quota enforcement")
	s.testQuotaEnforcement()
}

// TestConcurrentEmbeddings tests concurrent embedding requests
func (s *EmbeddingE2ESuite) TestConcurrentEmbeddings() {
	// Connect and register agent
	s.connectWebSocket()
	s.registerAgent()

	// Send multiple concurrent embedding requests
	concurrency := 5
	results := make(chan map[string]interface{}, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			text := fmt.Sprintf("Concurrent test text %d", idx)
			result := s.generateEmbedding(text, "")
			results <- result
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < concurrency; i++ {
		result := <-results
		if result != nil && result["success"].(bool) {
			successCount++
		}
	}

	s.Equal(concurrency, successCount, "All concurrent requests should succeed")
}

// TestErrorHandling tests error scenarios
func (s *EmbeddingE2ESuite) TestErrorHandling() {
	// Test with invalid API key
	s.T().Log("Testing invalid API key")
	invalidClient := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", s.apiBaseURL+"/api/v1/embedding-models/catalog", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")

	resp, err := invalidClient.Do(req)
	s.NoError(err)
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// Test with invalid model ID
	s.connectWebSocket()
	s.registerAgent()

	s.T().Log("Testing invalid model ID")
	result := s.generateEmbedding("Test text", "invalid-model-id")
	s.False(result["success"].(bool), "Should fail with invalid model")

	// Test with empty text
	s.T().Log("Testing empty text")
	result = s.generateEmbedding("", "")
	s.False(result["success"].(bool), "Should fail with empty text")
}

// Helper methods

func (s *EmbeddingE2ESuite) listEmbeddingModels() []interface{} {
	req, err := http.NewRequest("GET", s.apiBaseURL+"/api/v1/embedding-models/catalog", nil)
	s.NoError(err)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	s.NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	s.NoError(err)

	models, ok := result["models"].([]interface{})
	s.True(ok, "Response should contain models array")

	return models
}

func (s *EmbeddingE2ESuite) configureTenantModel(modelID string) {
	config := map[string]interface{}{
		"model_id":       modelID,
		"enabled":        true,
		"is_default":     true,
		"monthly_quota":  1000000,
		"daily_quota":    50000,
		"rate_limit_rpm": 100,
	}

	body, _ := json.Marshal(config)
	req, err := http.NewRequest("POST", s.apiBaseURL+"/api/v1/tenant-models", bytes.NewReader(body))
	s.NoError(err)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", s.testTenantID)

	resp, err := s.httpClient.Do(req)
	s.NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusCreated, resp.StatusCode)
}

func (s *EmbeddingE2ESuite) connectWebSocket() {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+s.apiKey)
	header.Set("X-Tenant-ID", s.testTenantID)

	var err error
	s.wsConn, _, err = websocket.DefaultDialer.Dial(s.wsBaseURL, header)
	s.NoError(err)
}

func (s *EmbeddingE2ESuite) registerAgent() {
	msg := map[string]interface{}{
		"id":     uuid.New().String(),
		"type":   0, // Request
		"method": "agent.register",
		"params": map[string]interface{}{
			"agent_id":     s.testAgentID,
			"name":         "E2E Test Agent",
			"capabilities": []string{"embedding", "search"},
		},
	}

	err := s.wsConn.WriteJSON(msg)
	s.NoError(err)

	// Read response
	var response map[string]interface{}
	err = s.wsConn.ReadJSON(&response)
	s.NoError(err)
	s.NotNil(response["result"])
}

func (s *EmbeddingE2ESuite) generateEmbedding(text, modelID string) map[string]interface{} {
	msg := map[string]interface{}{
		"id":     uuid.New().String(),
		"type":   0, // Request
		"method": "embedding.generate",
		"params": map[string]interface{}{
			"text":      text,
			"model":     modelID,
			"task_type": "search",
			"agent_id":  s.testAgentID,
		},
	}

	err := s.wsConn.WriteJSON(msg)
	s.NoError(err)

	// Read response with timeout
	s.wsConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var response map[string]interface{}
	err = s.wsConn.ReadJSON(&response)
	s.NoError(err)

	result, ok := response["result"].(map[string]interface{})
	s.True(ok, "Response should contain result")

	return result
}

func (s *EmbeddingE2ESuite) getUsageStats() map[string]interface{} {
	req, err := http.NewRequest("GET", s.apiBaseURL+"/api/v1/tenant-models/usage?period=day", nil)
	s.NoError(err)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("X-Tenant-ID", s.testTenantID)

	resp, err := s.httpClient.Do(req)
	s.NoError(err)
	defer resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	s.NoError(err)

	return result
}

func (s *EmbeddingE2ESuite) testModelSwitching() {
	// Configure a second model with higher priority
	models := s.listEmbeddingModels()
	if len(models) < 2 {
		s.T().Skip("Need at least 2 models for switching test")
	}

	// Find another available model
	for _, model := range models[1:] {
		if modelMap, ok := model.(map[string]interface{}); ok {
			if available, _ := modelMap["is_available"].(bool); available {
				s.configureTenantModel(modelMap["id"].(string))
				break
			}
		}
	}

	// Generate embedding and check which model was used
	result := s.generateEmbedding("Test model switching", "")
	s.NotNil(result["model"])
}

func (s *EmbeddingE2ESuite) testQuotaEnforcement() {
	// Set a very low quota
	config := map[string]interface{}{
		"daily_quota": 10, // Very low quota
	}

	body, _ := json.Marshal(config)
	req, err := http.NewRequest("PUT", s.apiBaseURL+"/api/v1/tenant-models/quota", bytes.NewReader(body))
	s.NoError(err)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", s.testTenantID)

	resp, err := s.httpClient.Do(req)
	s.NoError(err)
	resp.Body.Close()

	// Try to exceed quota
	successCount := 0
	failCount := 0

	for i := 0; i < 20; i++ {
		result := s.generateEmbedding(fmt.Sprintf("Quota test %d", i), "")
		if result["success"].(bool) {
			successCount++
		} else {
			failCount++
		}
	}

	s.Greater(failCount, 0, "Some requests should fail due to quota")
}

// TestEmbeddingE2ESuite runs the E2E test suite
func TestEmbeddingE2ESuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Check if services are running
	client := &http.Client{Timeout: 2 * time.Second}
	apiURL := os.Getenv("E2E_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8081"
	}

	resp, err := client.Get(apiURL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skip("Services not running, skipping E2E tests")
	}
	resp.Body.Close()

	suite.Run(t, new(EmbeddingE2ESuite))
}

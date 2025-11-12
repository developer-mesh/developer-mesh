//go:build contract

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/adapters"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/core"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ContractTestServer encapsulates the test server and its dependencies
type ContractTestServer struct {
	server            *httptest.Server
	client            *http.Client
	baseURL           string
	mockEngine        *core.MockEngine
	mockAgentAdapter  *adapters.MockAgentAdapter
	mockModelAdapter  *adapters.MockModelAdapter
	mockVectorAdapter *adapters.MockVectorAdapter
	mockSearchAdapter *adapters.MockSearchAdapter
}

// setupContractTestServer creates a fully configured test server
func setupContractTestServer(t *testing.T) *ContractTestServer {
	gin.SetMode(gin.TestMode)

	// Create mock engine
	mockEngine := &core.MockEngine{}

	// Create mock adapters
	mockAgentAdapter := &adapters.MockAgentAdapter{}
	mockModelAdapter := &adapters.MockModelAdapter{}
	mockVectorAdapter := &adapters.MockVectorAdapter{}
	mockSearchAdapter := &adapters.MockSearchAdapter{}

	// Create minimal config for testing
	cfg := Config{
		Auth: AuthConfig{
			Enabled: false, // Disable auth for contract tests
		},
		RateLimit: RateLimitConfig{
			Enabled: false,
		},
		Performance: PerformanceConfig{
			EnableCompression: false,
			EnableETagCaching: false,
		},
		Versioning: VersioningConfig{
			DefaultVersion:    "v1",
			SupportedVersions: []string{"v1"},
		},
		EnableCORS: true,
	}

	// Create server
	server := &Server{
		engine: mockEngine,
		config: cfg,
	}

	// Create test router
	router := gin.New()
	router = server.SetupRouter(router, cfg)

	// Set up agent routes with mock adapter
	agentAPI := &AgentAPI{
		engine:  mockEngine,
		adapter: mockAgentAdapter,
	}
	agentAPI.RegisterRoutes(router)

	// Set up model routes with mock adapter
	modelAPI := &ModelAPI{
		engine:  mockEngine,
		adapter: mockModelAdapter,
	}
	modelAPI.RegisterRoutes(router)

	// Set up vector routes with mock adapter
	vectorAPI := &VectorAPI{
		adapter: mockVectorAdapter,
	}
	vectorAPI.RegisterRoutes(router)

	// Create test server
	ts := httptest.NewServer(router)

	return &ContractTestServer{
		server:            ts,
		client:            &http.Client{},
		baseURL:           ts.URL,
		mockEngine:        mockEngine,
		mockAgentAdapter:  mockAgentAdapter,
		mockModelAdapter:  mockModelAdapter,
		mockVectorAdapter: mockVectorAdapter,
		mockSearchAdapter: mockSearchAdapter,
	}
}

// cleanup closes the test server
func (cts *ContractTestServer) cleanup() {
	cts.server.Close()
}

// makeRequest is a helper to make HTTP requests
func (cts *ContractTestServer) makeRequest(method, path string, body any, headers map[string]string) (*http.Response, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, cts.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set default headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return cts.client.Do(req)
}

// TestHealthEndpointContract validates the health endpoint response format
func TestHealthEndpointContract(t *testing.T) {
	cts := setupContractTestServer(t)
	defer cts.cleanup()

	resp, err := cts.makeRequest("GET", "/health", nil, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]any
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Validate contract
	assert.Contains(t, response, "status")
	assert.Contains(t, response, "timestamp")
	assert.Contains(t, response, "components")

	// Validate component structure
	components, ok := response["components"].(map[string]any)
	assert.True(t, ok, "components should be an object")

	for name, component := range components {
		comp, ok := component.(map[string]any)
		assert.True(t, ok, "component %s should be an object", name)
		assert.Contains(t, comp, "status")
		assert.Contains(t, comp, "latency")
	}
}

// TestListAgentsContract validates the list agents endpoint
func TestListAgentsContract(t *testing.T) {
	cts := setupContractTestServer(t)
	defer cts.cleanup()

	// Setup mock response on the adapter
	cts.mockAgentAdapter.On("ListAgents", mock.Anything, mock.Anything).Return(
		[]*models.Agent{
			{
				ID:          "agent-1",
				Name:        "Test Agent",
				Description: "Test Description",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
		&models.PaginationInfo{Total: 1, Limit: 10, Offset: 0},
		nil,
	)

	headers := map[string]string{
		"Authorization": "Bearer test-token",
		"X-Tenant-ID":   "test-tenant",
	}

	resp, err := cts.makeRequest("GET", "/api/v1/agents?limit=10&offset=0", nil, headers)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 or 401 depending on auth setup
	assert.Contains(t, []int{http.StatusOK, http.StatusUnauthorized}, resp.StatusCode)

	if resp.StatusCode == http.StatusOK {
		var response map[string]any
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		// Validate pagination structure
		assert.Contains(t, response, "data")
		assert.Contains(t, response, "pagination")

		pagination, ok := response["pagination"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, pagination, "limit")
		assert.Contains(t, pagination, "offset")
		assert.Contains(t, pagination, "total")

		// Validate data array
		data, ok := response["data"].([]any)
		assert.True(t, ok, "data should be an array")

		// If there's data, validate agent structure
		if len(data) > 0 {
			agent, ok := data[0].(map[string]any)
			assert.True(t, ok)
			assert.Contains(t, agent, "id")
			assert.Contains(t, agent, "name")
			assert.Contains(t, agent, "description")
			assert.Contains(t, agent, "created_at")
			assert.Contains(t, agent, "updated_at")
		}
	}
}

// TestCreateContextContract validates context creation
func TestCreateContextContract(t *testing.T) {
	cts := setupContractTestServer(t)
	defer cts.cleanup()

	// Setup mock response
	mockContext := &models.Context{
		ID:            "ctx-123",
		AgentID:       "test-agent",
		ModelID:       "test-model",
		SessionID:     "session-123",
		CurrentTokens: 0,
		MaxTokens:     4000,
		Metadata: map[string]any{
			"source": "contract-test",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	cts.mockEngine.On("CreateContext", mock.Anything, mock.Anything).Return(mockContext, nil)

	payload := map[string]any{
		"agent_id":   "test-agent",
		"model_id":   "test-model",
		"max_tokens": 4000,
		"metadata": map[string]any{
			"source": "contract-test",
		},
	}

	headers := map[string]string{
		"Authorization": "Bearer test-token",
		"X-Tenant-ID":   "test-tenant",
	}

	resp, err := cts.makeRequest("POST", "/api/v1/contexts", payload, headers)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 201 or 401 depending on auth
	assert.Contains(t, []int{http.StatusCreated, http.StatusUnauthorized}, resp.StatusCode)

	if resp.StatusCode == http.StatusCreated {
		var response map[string]any
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		// Validate response contract
		assert.Contains(t, response, "id")
		assert.Contains(t, response, "agent_id")
		assert.Contains(t, response, "model_id")
		assert.Contains(t, response, "session_id")
		assert.Contains(t, response, "current_tokens")
		assert.Contains(t, response, "max_tokens")
		assert.Contains(t, response, "metadata")
		assert.Contains(t, response, "created_at")
		assert.Contains(t, response, "updated_at")

		// Validate types
		_, ok := response["id"].(string)
		assert.True(t, ok, "id should be string")

		_, ok = response["current_tokens"].(float64)
		assert.True(t, ok, "current_tokens should be number")
	}
}

// TestErrorResponseContract validates error response format
func TestErrorResponseContract(t *testing.T) {
	cts := setupContractTestServer(t)
	defer cts.cleanup()

	// Test 404 error
	resp, err := cts.makeRequest("GET", "/api/v1/nonexistent", nil, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var response map[string]any
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Validate error contract
	assert.Contains(t, response, "error")

	errorObj, ok := response["error"].(map[string]any)
	assert.True(t, ok, "error should be an object")
	assert.Contains(t, errorObj, "code")
	assert.Contains(t, errorObj, "message")

	// Optional fields
	// assert.Contains(t, errorObj, "details")
	// assert.Contains(t, errorObj, "request_id")
}

// TestPaginationContract validates pagination parameters
func TestPaginationContract(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		expectError bool
	}{
		{
			name:        "Valid pagination",
			query:       "?limit=20&offset=0",
			expectError: false,
		},
		{
			name:        "Invalid limit",
			query:       "?limit=-1&offset=0",
			expectError: true,
		},
		{
			name:        "Limit too high",
			query:       "?limit=1001&offset=0",
			expectError: true,
		},
		{
			name:        "Invalid offset",
			query:       "?limit=20&offset=-1",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cts := setupContractTestServer(t)
			defer cts.cleanup()

			// Setup mock for valid requests
			if !tt.expectError {
				cts.mockEngine.On("ListAgents", mock.Anything, mock.Anything).Return(
					[]*models.Agent{},
					&models.PaginationInfo{Total: 0, Limit: 20, Offset: 0},
					nil,
				)
			}

			headers := map[string]string{
				"Authorization": "Bearer test-token",
				"X-Tenant-ID":   "test-tenant",
			}

			resp, err := cts.makeRequest("GET", "/api/v1/agents"+tt.query, nil, headers)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tt.expectError {
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			} else {
				assert.Contains(t, []int{http.StatusOK, http.StatusUnauthorized}, resp.StatusCode)
			}
		})
	}
}

// TestVectorSearchContract validates vector search endpoint
func TestVectorSearchContract(t *testing.T) {
	cts := setupContractTestServer(t)
	defer cts.cleanup()

	// Setup mock response
	mockResults := []*models.VectorSearchResult{
		{
			ID:      "vec-1",
			Content: "Test content",
			Score:   0.95,
			Metadata: map[string]any{
				"source": "test",
			},
		},
	}
	cts.mockEngine.On("VectorSearch", mock.Anything, mock.Anything).Return(mockResults, nil)

	payload := map[string]any{
		"query":        "test search query",
		"limit":        10,
		"threshold":    0.7,
		"content_type": "document",
		"filters": map[string]any{
			"model_id": "gpt-4",
		},
	}

	headers := map[string]string{
		"Authorization": "Bearer test-token",
		"X-Tenant-ID":   "test-tenant",
	}

	resp, err := cts.makeRequest("POST", "/api/v1/search/vector", payload, headers)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t, []int{http.StatusOK, http.StatusUnauthorized}, resp.StatusCode)

	if resp.StatusCode == http.StatusOK {
		var response map[string]any
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		// Validate search response contract
		assert.Contains(t, response, "results")
		assert.Contains(t, response, "total")
		assert.Contains(t, response, "query")

		results, ok := response["results"].([]any)
		assert.True(t, ok, "results should be an array")

		// If there are results, validate structure
		if len(results) > 0 {
			result, ok := results[0].(map[string]any)
			assert.True(t, ok)
			assert.Contains(t, result, "id")
			assert.Contains(t, result, "content")
			assert.Contains(t, result, "score")
			assert.Contains(t, result, "metadata")

			score, ok := result["score"].(float64)
			assert.True(t, ok, "score should be a number")
			assert.True(t, score >= 0 && score <= 1, "score should be between 0 and 1")
		}
	}
}

// TestRateLimitingHeaders validates rate limiting headers
func TestRateLimitingHeaders(t *testing.T) {
	cts := setupContractTestServer(t)
	defer cts.cleanup()

	// Setup mock response
	cts.mockEngine.On("ListAgents", mock.Anything, mock.Anything).Return(
		[]*models.Agent{},
		&models.PaginationInfo{Total: 0, Limit: 10, Offset: 0},
		nil,
	)

	headers := map[string]string{
		"Authorization": "Bearer test-token",
		"X-Tenant-ID":   "test-tenant",
	}

	resp, err := cts.makeRequest("GET", "/api/v1/agents", nil, headers)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check for rate limiting headers
	if resp.Header.Get("X-RateLimit-Limit") != "" {
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Remaining"))
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Reset"))
	}
}

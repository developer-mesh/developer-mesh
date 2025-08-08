package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIDEAgentGitHubIntegration tests the full flow of:
// 1. IDE agent registering with universal system
// 2. Agent discovering GitHub tool capability
// 3. Agent requesting to read code from GitHub (non-destructive)
func TestIDEAgentGitHubIntegration(t *testing.T) {
	// Skip if not in integration mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Test configuration
	wsURL := getTestWebSocketURL()
	apiURL := getTestAPIURL()
	apiKey := getTestAPIKey()

	t.Run("IDE_Agent_GitHub_Read_Code", func(t *testing.T) {
		// Step 1: Register IDE agent with universal system
		agentID := registerIDEAgent(t, ctx, wsURL, apiKey)

		// Step 2: Discover GitHub tool capability
		githubToolID := discoverGitHubTool(t, ctx, apiURL, apiKey)

		// Step 3: Make non-destructive GitHub API call to read code
		readGitHubCode(t, ctx, apiURL, apiKey, agentID, githubToolID)
	})
}

// registerIDEAgent registers an IDE agent using the universal registration system
func registerIDEAgent(t *testing.T, ctx context.Context, wsURL, apiKey string) string {
	// Set up WebSocket connection
	headers := http.Header{
		"Authorization": []string{"Bearer " + apiKey},
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// Connect with mcp.v1 subprotocol (required)
	conn, resp, err := dialer.Dial(wsURL+"?subprotocols=mcp.v1", headers)
	require.NoError(t, err, "Failed to connect to WebSocket")
	require.NotNil(t, resp)
	defer conn.Close()

	// Send universal agent registration
	registration := map[string]interface{}{
		"type":       "agent.universal.register",
		"name":       "Test IDE Agent",
		"agent_type": "ide",
		"capabilities": []string{
			"code_analysis",
			"code_completion",
			"github_integration",
			"file_reading",
		},
		"requirements": map[string]interface{}{
			"min_memory": "2GB",
			"apis":       []string{"github"},
		},
		"metadata": map[string]interface{}{
			"version":   "1.0.0",
			"ide":       "vscode",
			"test_mode": true,
		},
	}

	err = conn.WriteJSON(registration)
	require.NoError(t, err, "Failed to send registration")

	// Read registration response
	var response map[string]interface{}
	err = conn.ReadJSON(&response)
	require.NoError(t, err, "Failed to read registration response")

	// Verify registration success
	assert.Equal(t, "agent.registered", response["type"])
	agentID, ok := response["agent_id"].(string)
	assert.True(t, ok, "agent_id not found in response")
	assert.NotEmpty(t, agentID)

	manifestID, ok := response["manifest_id"].(string)
	assert.True(t, ok, "manifest_id not found in response")
	assert.NotEmpty(t, manifestID)

	t.Logf("IDE Agent registered successfully: ID=%s, Manifest=%s", agentID, manifestID)

	// Send a health check to confirm agent is active
	healthCheck := map[string]interface{}{
		"type":     "agent.universal.health",
		"agent_id": agentID,
		"status":   "healthy",
		"metrics": map[string]interface{}{
			"cpu_usage":    15.5,
			"memory_usage": 1024,
			"active_tasks": 0,
		},
	}

	err = conn.WriteJSON(healthCheck)
	require.NoError(t, err, "Failed to send health check")

	return agentID
}

// discoverGitHubTool discovers the GitHub tool using capability-based discovery
func discoverGitHubTool(t *testing.T, ctx context.Context, apiURL, apiKey string) string {
	// Create HTTP client
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Discover tools with GitHub capability
	req, err := http.NewRequestWithContext(ctx, "GET",
		apiURL+"/api/v1/tools?capability=github", nil)
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var tools []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&tools)
	require.NoError(t, err)

	// Find GitHub tool
	var githubToolID string
	for _, tool := range tools {
		if name, ok := tool["name"].(string); ok && name == "github" {
			if id, ok := tool["id"].(string); ok {
				githubToolID = id
				break
			}
		}
	}

	require.NotEmpty(t, githubToolID, "GitHub tool not found")
	t.Logf("GitHub tool discovered: ID=%s", githubToolID)

	return githubToolID
}

// readGitHubCode makes a non-destructive GitHub API call to read code
func readGitHubCode(t *testing.T, ctx context.Context, apiURL, apiKey, agentID, toolID string) {
	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Prepare request to read a public repository file (non-destructive)
	// We'll read the README from a well-known public repo
	toolRequest := map[string]interface{}{
		"agent_id": agentID,
		"tool_id":  toolID,
		"action":   "get_file_content",
		"parameters": map[string]interface{}{
			"owner": "golang",    // Public org
			"repo":  "go",        // Public repo
			"path":  "README.md", // Read README
			"ref":   "master",    // Branch
		},
		"metadata": map[string]interface{}{
			"request_id": uuid.New().String(),
			"purpose":    "test_read_only",
			"agent_type": "ide",
		},
	}

	reqBody, err := json.Marshal(toolRequest)
	require.NoError(t, err)

	// Make the tool execution request
	req, err := http.NewRequestWithContext(ctx, "POST",
		apiURL+"/api/v1/tools/"+toolID+"/execute",
		bytes.NewReader(reqBody))
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", agentID)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check response
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Tool execution should succeed")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Verify we got file content
	if content, ok := result["content"].(string); ok {
		assert.NotEmpty(t, content, "File content should not be empty")
		assert.Contains(t, content, "Go", "README should contain 'Go'")
		t.Logf("Successfully read GitHub file: %d bytes", len(content))
	} else if data, ok := result["data"].(map[string]interface{}); ok {
		// Alternative response structure
		if content, ok := data["content"].(string); ok {
			assert.NotEmpty(t, content, "File content should not be empty")
			t.Logf("Successfully read GitHub file via data field: %d bytes", len(content))
		}
	}

	// Check that this was logged as a read-only operation
	if metadata, ok := result["metadata"].(map[string]interface{}); ok {
		assert.Equal(t, "read", metadata["operation_type"], "Should be marked as read operation")
		assert.Equal(t, false, metadata["destructive"], "Should be marked as non-destructive")
	}

	t.Log("IDE agent successfully read code from GitHub using universal agent system")
}

// Alternative: Test reading code from a specific file in the DevOps MCP project itself
func TestIDEAgentReadLocalProjectCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	wsURL := getTestWebSocketURL()
	apiURL := getTestAPIURL()
	apiKey := getTestAPIKey()

	t.Run("IDE_Agent_Read_Project_Code", func(t *testing.T) {
		// Register IDE agent
		agentID := registerIDEAgent(t, ctx, wsURL, apiKey)

		// Create a request to analyze local project code
		analyzeRequest := map[string]interface{}{
			"agent_id": agentID,
			"action":   "analyze_code",
			"parameters": map[string]interface{}{
				"file_path":       "pkg/models/agent_manifest.go",
				"analysis_type":   "structure",
				"include_metrics": true,
			},
		}

		// Send via WebSocket for real-time processing
		headers := http.Header{
			"Authorization": []string{"Bearer " + apiKey},
		}

		dialer := websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		}

		conn, _, err := dialer.Dial(wsURL+"?subprotocols=mcp.v1", headers)
		require.NoError(t, err)
		defer conn.Close()

		// Send analysis request
		message := map[string]interface{}{
			"type":              "agent.universal.message",
			"source_agent":      agentID,
			"target_capability": "code_analysis",
			"message_type":      "analyze.request",
			"payload":           analyzeRequest,
		}

		err = conn.WriteJSON(message)
		require.NoError(t, err)

		// Read analysis response
		var response map[string]interface{}
		err = conn.ReadJSON(&response)
		require.NoError(t, err)

		t.Logf("Code analysis response: %+v", response)
	})
}

// Helper functions

func getTestWebSocketURL() string {
	if url := os.Getenv("TEST_WS_URL"); url != "" {
		return url
	}
	return "ws://localhost:8080/ws"
}

func getTestAPIURL() string {
	if url := os.Getenv("TEST_API_URL"); url != "" {
		return url
	}
	return "http://localhost:8081"
}

func getTestAPIKey() string {
	if key := os.Getenv("TEST_API_KEY"); key != "" {
		return key
	}
	// Default test API key (should be in database)
	return "test_api_key_for_integration"
}

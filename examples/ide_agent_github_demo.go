package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// IDEAgent represents an IDE agent that can interact with GitHub
type IDEAgent struct {
	ID         string
	Name       string
	conn       *websocket.Conn
	apiURL     string
	apiKey     string
	httpClient *http.Client
}

// NewIDEAgent creates a new IDE agent
func NewIDEAgent(name, wsURL, apiURL, apiKey string) (*IDEAgent, error) {
	agent := &IDEAgent{
		Name:   name,
		apiURL: apiURL,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Connect to WebSocket with required mcp.v1 subprotocol
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Subprotocols:     []string{"mcp.v1"},
	}

	headers := http.Header{
		"Authorization": []string{"Bearer " + apiKey},
	}

	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	agent.conn = conn

	// Register with universal system
	if err := agent.register(); err != nil {
		conn.Close()
		return nil, err
	}

	// Start message handler
	go agent.handleMessages()

	return agent, nil
}

// register registers the IDE agent with the universal system
func (a *IDEAgent) register() error {
	registration := map[string]interface{}{
		"type":       "agent.universal.register",
		"name":       a.Name,
		"agent_type": "ide",
		"capabilities": []string{
			"code_analysis",
			"code_completion",
			"github_integration",
			"debugging",
			"refactoring",
		},
		"requirements": map[string]interface{}{
			"min_memory": "2GB",
			"apis":       []string{"github", "jira"},
		},
		"metadata": map[string]interface{}{
			"version": "1.0.0",
			"ide":     "vscode",
			"model":   "gpt-4",
		},
	}

	if err := a.conn.WriteJSON(registration); err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}

	// Read registration response
	var response map[string]interface{}
	if err := a.conn.ReadJSON(&response); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if response["type"] != "agent.registered" {
		return fmt.Errorf("registration failed: %v", response)
	}

	a.ID = response["agent_id"].(string)
	log.Printf("✅ IDE Agent registered: ID=%s, Name=%s", a.ID, a.Name)

	if orgID, ok := response["organization_id"].(string); ok {
		log.Printf("📍 Organization: %s", orgID)
	}

	return nil
}

// handleMessages handles incoming WebSocket messages
func (a *IDEAgent) handleMessages() {
	for {
		var msg map[string]interface{}
		if err := a.conn.ReadJSON(&msg); err != nil {
			log.Printf("Read error: %v", err)
			return
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "ping":
			a.conn.WriteJSON(map[string]interface{}{"type": "pong"})
		case "task.execute":
			log.Printf("Received task: %v", msg)
		default:
			log.Printf("Received message: %s", msgType)
		}
	}
}

// ReadGitHubCode reads code from GitHub (non-destructive operation)
func (a *IDEAgent) ReadGitHubCode(owner, repo, path string) error {
	log.Printf("📖 Reading GitHub file: %s/%s/%s", owner, repo, path)

	// First, discover GitHub tool
	toolID, err := a.discoverGitHubTool()
	if err != nil {
		return err
	}

	// Execute tool to read code
	request := map[string]interface{}{
		"agent_id": a.ID,
		"action":   "get_file_content",
		"parameters": map[string]interface{}{
			"owner": owner,
			"repo":  repo,
			"path":  path,
			"ref":   "main", // or "master"
		},
		"metadata": map[string]interface{}{
			"request_id": uuid.New().String(),
			"purpose":    "code_analysis",
			"agent_type": "ide",
		},
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return err
	}

	// Make HTTP request to execute tool
	req, err := http.NewRequest("POST",
		a.apiURL+"/api/v1/tools/"+toolID+"/execute",
		bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", a.ID)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tool execution failed: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	// Extract content
	var content string
	if c, ok := result["content"].(string); ok {
		content = c
	} else if data, ok := result["data"].(map[string]interface{}); ok {
		if c, ok := data["content"].(string); ok {
			content = c
		}
	}

	if content != "" {
		log.Printf("✅ Successfully read file: %d bytes", len(content))

		// Show first 500 characters
		preview := content
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		log.Printf("📄 Content preview:\n%s", preview)
	}

	return nil
}

// discoverGitHubTool discovers the GitHub tool ID
func (a *IDEAgent) discoverGitHubTool() (string, error) {
	req, err := http.NewRequest("GET",
		a.apiURL+"/api/v1/tools?capability=github", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tools []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tools); err != nil {
		return "", err
	}

	// Find GitHub tool
	for _, tool := range tools {
		if name, ok := tool["name"].(string); ok && name == "github" {
			if id, ok := tool["id"].(string); ok {
				log.Printf("🔧 Found GitHub tool: %s", id)
				return id, nil
			}
		}
	}

	return "", fmt.Errorf("GitHub tool not found")
}

// AnalyzeCode sends a code analysis request
func (a *IDEAgent) AnalyzeCode(filePath string) error {
	log.Printf("🔍 Analyzing code: %s", filePath)

	message := map[string]interface{}{
		"type":              "agent.universal.message",
		"source_agent":      a.ID,
		"source_agent_type": "ide",
		"target_capability": "code_analysis",
		"message_type":      "analyze.request",
		"priority":          5,
		"payload": map[string]interface{}{
			"file_path":           filePath,
			"analysis_type":       "all",
			"include_suggestions": true,
		},
	}

	return a.conn.WriteJSON(message)
}

// Close closes the agent connection
func (a *IDEAgent) Close() error {
	if a.conn != nil {
		// Send disconnect message
		a.conn.WriteJSON(map[string]interface{}{
			"type":     "agent.disconnect",
			"agent_id": a.ID,
		})
		return a.conn.Close()
	}
	return nil
}

func main() {
	// Configuration
	wsURL := getEnvOrDefault("WS_URL", "ws://localhost:8080/ws")
	apiURL := getEnvOrDefault("API_URL", "http://localhost:8081")
	apiKey := getEnvOrDefault("API_KEY", "your-api-key")

	// Create IDE agent
	agent, err := NewIDEAgent("VS Code Agent Demo", wsURL, apiURL, apiKey)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	// Example 1: Read code from a public GitHub repository
	log.Println("\n=== Example 1: Reading Public Repository ===")
	if err := agent.ReadGitHubCode("golang", "go", "README.md"); err != nil {
		log.Printf("Error reading GitHub code: %v", err)
	}

	// Example 2: Read code from the DevOps MCP project (if accessible)
	log.Println("\n=== Example 2: Reading Project Code ===")
	if err := agent.ReadGitHubCode("developer-mesh", "developer-mesh", "pkg/models/agent_manifest.go"); err != nil {
		log.Printf("Error reading project code: %v", err)
	}

	// Example 3: Analyze local code
	log.Println("\n=== Example 3: Analyzing Local Code ===")
	if err := agent.AnalyzeCode("pkg/models/agent_manifest.go"); err != nil {
		log.Printf("Error analyzing code: %v", err)
	}

	// Keep agent running for a bit to receive messages
	log.Println("\n✅ IDE Agent is running. Press Ctrl+C to exit.")
	time.Sleep(10 * time.Second)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

//go:build e2e

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(".env"); err != nil {
		fmt.Printf("⚠️  Warning: Could not load .env file: %v\n", err)
	}

	// Get configuration
	mcpBaseURL := os.Getenv("MCP_BASE_URL")
	apiBaseURL := os.Getenv("API_BASE_URL")
	apiKey := os.Getenv("E2E_API_KEY")

	fmt.Println("🔍 E2E Test Connection Validation")
	fmt.Println("=================================")
	fmt.Printf("MCP Base URL: %s\n", mcpBaseURL)
	fmt.Printf("API Base URL: %s\n", apiBaseURL)
	if len(apiKey) > 16 {
		fmt.Printf("API Key: %s...%s (length: %d)\n", apiKey[:8], apiKey[len(apiKey)-8:], len(apiKey))
	} else if apiKey != "" {
		fmt.Printf("API Key: [REDACTED] (length: %d)\n", len(apiKey))
	} else {
		fmt.Printf("API Key: not set\n")
	}
	fmt.Println()

	// Test API health endpoint
	fmt.Println("Testing API Health Endpoint...")
	apiHealthURL := apiBaseURL + "/health"
	if !testEndpoint(apiHealthURL, apiKey) {
		fmt.Printf("❌ API health check failed at %s\n", apiHealthURL)
	} else {
		fmt.Printf("✅ API is healthy at %s\n", apiHealthURL)
	}

	// Test MCP health endpoint
	fmt.Println("\nTesting MCP Health Endpoint...")
	mcpHealthURL := mcpBaseURL + "/health"
	if !testEndpoint(mcpHealthURL, apiKey) {
		fmt.Printf("❌ MCP health check failed at %s\n", mcpHealthURL)
	} else {
		fmt.Printf("✅ MCP is healthy at %s\n", mcpHealthURL)
	}

	// Test WebSocket endpoint (just check if it responds)
	fmt.Println("\nTesting WebSocket Endpoint...")
	wsURL := mcpBaseURL + "/ws"
	if !testWebSocketEndpoint(wsURL, apiKey) {
		fmt.Printf("❌ WebSocket endpoint check failed at %s\n", wsURL)
	} else {
		fmt.Printf("✅ WebSocket endpoint is accessible at %s\n", wsURL)
	}

	fmt.Println("\n=================================")
	fmt.Println("Validation complete!")
}

func testEndpoint(url, apiKey string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Printf("  Error creating request: %v\n", err)
		return false
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	fmt.Printf("  Response: %d %s\n", resp.StatusCode, resp.Status)
	return resp.StatusCode == http.StatusOK
}

func testWebSocketEndpoint(url, apiKey string) bool {
	// For WebSocket, we just test if the endpoint exists
	// A proper WebSocket test would require upgrading the connection
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("  Error creating request: %v\n", err)
		return false
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGVzdCBrZXkgZm9yIHZhbGlkYXRpb24=")
	req.Header.Set("Sec-WebSocket-Protocol", "mcp.v1")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	fmt.Printf("  Response: %d %s\n", resp.StatusCode, resp.Status)
	// 101 = Switching Protocols (success), 401 = Unauthorized (but endpoint exists)
	return resp.StatusCode == 101 || resp.StatusCode == 401 || resp.StatusCode == 426
}

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/examples/common"
)

func main() {
	common.PrintSection("Authentication Error Handling Example")
	common.PrintInfo("Demonstrates handling authentication and permission errors")

	ctx := context.Background()

	// Example 1: Handle missing API key
	handleMissingAPIKey(ctx)

	// Example 2: Handle invalid API key
	handleInvalidAPIKey(ctx)

	// Example 3: Handle permission denied
	handlePermissionDenied(ctx)

	// Example 4: Verify passthrough authentication
	verifyPassthroughAuth(ctx)

	common.PrintSection("Authentication Handling Complete")
	common.PrintSuccess("Demonstrated authentication error recovery")
}

func handleMissingAPIKey(ctx context.Context) {
	common.PrintSubsection("Handling Missing API Key")

	// Temporarily remove API key
	originalKey := os.Getenv("EDGE_MCP_API_KEY")
	os.Setenv("EDGE_MCP_API_KEY", "")

	client, err := common.NewClient(nil)

	// Restore API key
	if originalKey != "" {
		os.Setenv("EDGE_MCP_API_KEY", originalKey)
	}

	if err != nil {
		code, message := common.ExtractErrorCode(err)
		if code == 401 || code == 403 || strings.Contains(message, "authentication") {
			common.PrintError("Authentication failed", err)
			fmt.Println("\nRecovery steps:")
			fmt.Println("  1. Set EDGE_MCP_API_KEY environment variable")
			fmt.Println("  2. Verify API key is correct")
			fmt.Println("  3. Check API key has required permissions")
			fmt.Println("\nExample:")
			fmt.Println("  export EDGE_MCP_API_KEY=devmesh_ab80cbb2438dbb43339c0e3317ab2fc6dd0e046f3b50360df06abb5bae31a210")
		}
	} else {
		if client != nil {
			client.Close()
		}
		common.PrintSuccess("Connection succeeded (API key was present)")
	}
}

func handleInvalidAPIKey(ctx context.Context) {
	common.PrintSubsection("Handling Invalid API Key")

	// Use invalid API key
	config := &common.Config{
		URL:            "ws://localhost:8085/ws",
		APIKey:         "invalid-api-key-12345",
		ConnectTimeout: 5 * time.Second,
	}

	client, err := common.NewClient(config)
	if err != nil {
		code, message := common.ExtractErrorCode(err)
		if code == 401 || strings.Contains(message, "authentication") || strings.Contains(message, "unauthorized") {
			common.PrintError("Invalid API key", err)
			fmt.Println("\nRecovery steps:")
			fmt.Println("  1. Verify API key format (should start with 'devmesh_')")
			fmt.Println("  2. Check API key is active in the system")
			fmt.Println("  3. Request new API key if needed")
			fmt.Println("\nValid API key format:")
			fmt.Println("  devmesh_<64-character-hex-string>")
		}
	} else {
		if client != nil {
			client.Close()
		}
		common.PrintSuccess("Connection succeeded (API key was valid)")
	}
}

func handlePermissionDenied(ctx context.Context) {
	common.PrintSubsection("Handling Permission Denied Errors")

	// Create client with valid API key
	client, err := common.NewClient(nil)
	if err != nil {
		common.PrintError("Failed to create client", err)
		return
	}
	defer client.Close()

	// Try to call a tool (this won't actually fail in dev mode)
	_, err = client.CallTool(ctx, "github_create_repository", map[string]interface{}{
		"name":        "test-repo",
		"description": "Test repository",
	})

	if err != nil {
		code, message := common.ExtractErrorCode(err)
		if code == 403 || strings.Contains(message, "permission") {
			common.PrintError("Permission denied", err)
			fmt.Println("\nRecovery steps:")
			fmt.Println("  1. Verify API key has required scopes")
			fmt.Println("  2. Check resource-level permissions")
			fmt.Println("  3. Request elevated permissions if needed")
			fmt.Println("\nRequired permissions:")
			fmt.Println("  • repository:write - For creating repositories")
			fmt.Println("  • issues:write - For creating issues")
			fmt.Println("  • pulls:write - For creating pull requests")
		} else {
			common.PrintInfo("No permission error (tool call succeeded or failed for other reason)")
		}
	} else {
		common.PrintSuccess("Tool executed successfully")
	}
}

func verifyPassthroughAuth(ctx context.Context) {
	common.PrintSubsection("Verifying Passthrough Authentication")

	// Check for passthrough auth credentials
	githubToken := os.Getenv("GITHUB_TOKEN")
	harnessKey := os.Getenv("HARNESS_API_KEY")

	if githubToken != "" {
		common.PrintSuccess("GitHub token configured for passthrough auth")
		fmt.Printf("  Token: %s...%s\n", githubToken[:7], githubToken[len(githubToken)-7:])
	} else {
		common.PrintInfo("GitHub token not configured")
		fmt.Println("  Set GITHUB_TOKEN for GitHub API passthrough authentication")
		fmt.Println("  Example: export GITHUB_TOKEN=ghp_yourtoken")
	}

	if harnessKey != "" {
		common.PrintSuccess("Harness API key configured for passthrough auth")
		fmt.Printf("  Key: %s...\n", harnessKey[:12])
	} else {
		common.PrintInfo("Harness API key not configured")
		fmt.Println("  Set HARNESS_API_KEY and HARNESS_ACCOUNT_ID for Harness passthrough")
	}

	// Try GitHub operation with passthrough auth
	client, err := common.NewClient(nil)
	if err != nil {
		common.PrintError("Failed to create client", err)
		return
	}
	defer client.Close()

	if githubToken != "" {
		common.PrintInfo("\nTesting GitHub passthrough authentication...")
		_, err := client.CallTool(ctx, "github_get_me", map[string]interface{}{})
		if err != nil {
			common.PrintError("GitHub authentication failed", err)
			fmt.Println("\nCheck:")
			fmt.Println("  • Token is valid and not expired")
			fmt.Println("  • Token has required scopes")
			fmt.Println("  • Passthrough auth headers are configured correctly")
		} else {
			common.PrintSuccess("GitHub passthrough authentication working")
		}
	}
}

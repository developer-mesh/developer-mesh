package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/executor"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ShellTool provides shell command execution with strict security
type ShellTool struct {
	executor       *executor.CommandExecutor
	logger         observability.Logger
	restrictedCmds map[string]bool // Commands that are NEVER allowed
}

// NewShellTool creates a new shell tool with security constraints
func NewShellTool(exec *executor.CommandExecutor, logger observability.Logger) *ShellTool {
	return &ShellTool{
		executor: exec,
		logger:   logger,
		restrictedCmds: map[string]bool{
			// CRITICAL: These commands are NEVER allowed
			"rm":       true,
			"sudo":     true,
			"su":       true,
			"chmod":    true,
			"chown":    true,
			"kill":     true,
			"killall":  true,
			"shutdown": true,
			"reboot":   true,
			"halt":     true,
			"poweroff": true,
			"passwd":   true,
			"useradd":  true,
			"userdel":  true,
			"groupadd": true,
			"groupdel": true,
			"mount":    true,
			"umount":   true,
			"mkfs":     true,
			"fdisk":    true,
			"dd":       true,
			"nc":       true, // netcat can be dangerous
			"ncat":     true,
			"socat":    true,
			"curl":     false, // Allow curl but monitor usage
			"wget":     false, // Allow wget but monitor usage
		},
	}
}

// GetDefinitions returns tool definitions
func (t *ShellTool) GetDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "shell.execute",
			Description: "Execute a shell command",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Command to execute",
					},
					"cwd": map[string]interface{}{
						"type":        "string",
						"description": "Working directory (optional)",
					},
				},
				"required": []string{"command"},
			},
			Handler: t.handleExecute,
		},
	}
}

func (t *ShellTool) handleExecute(ctx context.Context, args json.RawMessage) (interface{}, error) {
	// Parse arguments
	var params struct {
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
		Cwd     string   `json:"cwd,omitempty"`
		Env     []string `json:"env,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// SECURITY LAYER 1: Command validation
	if params.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Extract the base command (first word)
	baseCmd := params.Command
	if strings.Contains(params.Command, " ") {
		// If command contains spaces, parse it
		parts := strings.Fields(params.Command)
		baseCmd = parts[0]
		// Add remaining parts to args if not already provided
		if len(params.Args) == 0 && len(parts) > 1 {
			params.Args = parts[1:]
		}
	}

	// SECURITY LAYER 2: Check against restricted commands
	if t.restrictedCmds[baseCmd] {
		t.logger.Warn("Blocked restricted command", map[string]interface{}{
			"command": baseCmd,
			"args":    params.Args,
		})
		return nil, fmt.Errorf("command '%s' is not allowed for security reasons", baseCmd)
	}

	// SECURITY LAYER 3: Prevent shell injection - no shell interpretation
	// We NEVER use sh -c or bash -c
	if baseCmd == "sh" || baseCmd == "bash" || baseCmd == "zsh" {
		// If trying to use shell directly, only allow specific safe operations
		if len(params.Args) > 0 && (params.Args[0] == "-c" || params.Args[0] == "--command") {
			return nil, fmt.Errorf("shell command execution with -c is not allowed for security")
		}
	}

	// SECURITY LAYER 4: Path validation for working directory
	if params.Cwd != "" {
		cleanPath := filepath.Clean(params.Cwd)
		if !t.executor.IsPathSafe(cleanPath) {
			return nil, fmt.Errorf("invalid working directory: %s", params.Cwd)
		}
		// Set the working directory for this execution
		if err := t.executor.SetWorkDir(cleanPath); err != nil {
			return nil, fmt.Errorf("failed to set working directory: %w", err)
		}
	}

	// SECURITY LAYER 5: Environment variable filtering
	_ = t.filterEnvironment(params.Env) // TODO: Use filtered environment when executor supports it

	// SECURITY LAYER 6: Argument validation - prevent injection attempts
	for _, arg := range params.Args {
		if t.isDangerousArgument(arg) {
			return nil, fmt.Errorf("potentially dangerous argument detected: %s", arg)
		}
	}

	// Log the command execution attempt
	t.logger.Info("Shell command execution requested", map[string]interface{}{
		"command": baseCmd,
		"args":    params.Args,
		"cwd":     params.Cwd,
	})

	// Execute the command using our secure executor
	result, err := t.executor.Execute(ctx, baseCmd, params.Args)
	if err != nil {
		// Log failed execution
		t.logger.Warn("Shell command failed", map[string]interface{}{
			"command":   baseCmd,
			"args":      params.Args,
			"error":     err.Error(),
			"exit_code": result.ExitCode,
		})
		// Still return the output even if command failed
	} else {
		// Log successful execution
		t.logger.Info("Shell command succeeded", map[string]interface{}{
			"command":   baseCmd,
			"args":      params.Args,
			"exit_code": result.ExitCode,
			"duration":  result.Duration,
		})
	}

	return map[string]interface{}{
		"stdout":   result.Stdout,
		"stderr":   result.Stderr,
		"exitCode": result.ExitCode,
		"success":  err == nil,
	}, nil
}

// filterEnvironment filters environment variables to prevent sensitive data leakage
func (t *ShellTool) filterEnvironment(env []string) []string {
	if env == nil {
		return nil
	}

	filtered := []string{}
	blockedPrefixes := []string{
		"AWS_SECRET",
		"AWS_SESSION",
		"API_KEY",
		"TOKEN",
		"PASSWORD",
		"SECRET",
		"PRIVATE",
		"CREDENTIAL",
	}

	for _, e := range env {
		// Check if environment variable contains sensitive prefixes
		upper := strings.ToUpper(e)
		blocked := false
		for _, prefix := range blockedPrefixes {
			if strings.Contains(upper, prefix) {
				blocked = true
				break
			}
		}
		if !blocked {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

// isDangerousArgument checks if an argument could be used for injection or escalation
func (t *ShellTool) isDangerousArgument(arg string) bool {
	dangerous := []string{
		"../",  // Path traversal
		"..\\", // Path traversal Windows
		"$(",   // Command substitution
		"$((",  // Arithmetic substitution
		"`",    // Command substitution
		"&&",   // Command chaining
		"||",   // Command chaining
		"|",    // Piping (could be dangerous)
		";",    // Command separator
		">",    // Redirection
		">>",   // Append redirection
		"<",    // Input redirection
		"2>",   // Error redirection
	}

	for _, d := range dangerous {
		if strings.Contains(arg, d) {
			return true
		}
	}

	// Check for attempts to reference sensitive files
	sensitiveFiles := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/etc/sudoers",
		"~/.ssh/",
		".ssh/id_rsa",
		"/root/",
	}

	for _, f := range sensitiveFiles {
		if strings.Contains(arg, f) {
			return true
		}
	}

	return false
}

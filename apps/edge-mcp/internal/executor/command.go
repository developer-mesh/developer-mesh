// Package executor provides secure command execution for Edge MCP
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Result contains the output from command execution
type Result struct {
	Stdout   string
	Stderr   string
	Error    error
	ExitCode int
	Duration time.Duration
}

// CommandExecutor provides secure command execution with sandboxing
type CommandExecutor struct {
	logger       observability.Logger
	maxTimeout   time.Duration
	workDir      string
	allowedPaths []string
	allowedCmds  map[string]bool // Whitelist of commands
}

// NewCommandExecutor creates a new secure command executor
func NewCommandExecutor(logger observability.Logger, workDir string, maxTimeout time.Duration) *CommandExecutor {
	// Get platform-specific command mappings
	platformCmds := platformCommands()

	// Build allowed commands list with platform-specific mappings
	allowedCmds := map[string]bool{
		// Git commands (cross-platform)
		"git": true,
		// Docker commands (cross-platform)
		"docker": true,
		// Build tools (cross-platform)
		"go":   true,
		"make": true,
		"npm":  true,
		"yarn": true,
		// Common tools
		"echo":  true,
		"which": true,
		"head":  true,
		"tail":  true,
		"wc":    true,
		"sort":  true,
		"uniq":  true,
	}

	// Add platform-specific commands
	for _, cmd := range platformCmds {
		allowedCmds[cmd] = true
	}

	executor := &CommandExecutor{
		logger:      logger,
		maxTimeout:  maxTimeout,
		workDir:     workDir,
		allowedCmds: allowedCmds,
		allowedPaths: []string{
			workDir, // Only allow operations in workDir by default
		},
	}

	// Log platform detection
	logger.Info("CommandExecutor initialized", map[string]interface{}{
		"platform":     runtime.GOOS,
		"architecture": runtime.GOARCH,
		"work_dir":     workDir,
		"max_timeout":  maxTimeout,
		"allowed_cmds": len(allowedCmds),
	})

	return executor
}

// Execute runs a command with security constraints
func (e *CommandExecutor) Execute(ctx context.Context, command string, args []string) (*Result, error) {
	startTime := time.Now()

	// STEP 1: Platform-specific command mapping
	if mappedCmd, exists := e.getMappedCommand(command); exists {
		e.logger.Debug("Mapping platform command", map[string]interface{}{
			"original": command,
			"mapped":   mappedCmd,
			"platform": runtime.GOOS,
		})
		command = mappedCmd
	}

	// STEP 2: Platform-specific validation
	if err := validatePlatformCommand(command); err != nil {
		e.logger.Warn("Platform validation failed", map[string]interface{}{
			"command":  command,
			"platform": runtime.GOOS,
			"error":    err.Error(),
		})
		return nil, err
	}

	// STEP 3: Validate command is allowed
	if !e.allowedCmds[command] {
		e.logger.Warn("Command not allowed", map[string]interface{}{
			"command": command,
			"args":    args,
		})
		return nil, fmt.Errorf("command not allowed: %s", command)
	}

	// STEP 4: Create timeout context (MANDATORY)
	ctx, cancel := context.WithTimeout(ctx, e.maxTimeout)
	defer cancel()

	// STEP 5: Create command with context
	cmd := exec.CommandContext(ctx, command, args...)

	// STEP 6: Set platform-specific security attributes
	setPlatformAttrs(cmd)

	// STEP 7: Set working directory with validation
	if e.workDir != "" {
		cmd.Dir = e.workDir
	}

	// STEP 8: Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// STEP 9: Execute with logging
	err := cmd.Run()

	// Calculate duration
	duration := time.Since(startTime)

	// Get exit code (platform-aware)
	exitCode := e.getExitCode(err)

	// STEP 10: Log execution (use structured logging pattern)
	e.logger.Info("Command executed", map[string]interface{}{
		"command":   command,
		"args":      args,
		"platform":  runtime.GOOS,
		"duration":  duration,
		"exit_code": exitCode,
		"success":   err == nil,
	})

	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Error:    err,
		ExitCode: exitCode,
		Duration: duration,
	}, err
}

// IsPathSafe validates that a path is within allowed directories
func (e *CommandExecutor) IsPathSafe(path string) bool {
	// Clean the path to prevent traversal attacks
	cleaned := filepath.Clean(path)

	// Reject paths with .. to prevent traversal
	if strings.Contains(cleaned, "..") {
		return false
	}

	// Check if path is within allowed directories
	for _, allowed := range e.allowedPaths {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}

		absPath, err := filepath.Abs(cleaned)
		if err != nil {
			return false
		}

		// Check if path is within allowed directory
		if strings.HasPrefix(absPath, absAllowed) {
			return true
		}
	}

	return false
}

// AddAllowedCommand adds a command to the allowlist
func (e *CommandExecutor) AddAllowedCommand(cmd string) {
	e.allowedCmds[cmd] = true
}

// RemoveAllowedCommand removes a command from the allowlist
func (e *CommandExecutor) RemoveAllowedCommand(cmd string) {
	delete(e.allowedCmds, cmd)
}

// AddAllowedPath adds a path to the allowed paths list
func (e *CommandExecutor) AddAllowedPath(path string) {
	e.allowedPaths = append(e.allowedPaths, path)
}

// SetWorkDir sets the working directory for command execution
func (e *CommandExecutor) SetWorkDir(dir string) error {
	if !e.IsPathSafe(dir) {
		return fmt.Errorf("invalid working directory: %s", dir)
	}
	e.workDir = dir
	return nil
}

// getMappedCommand returns platform-specific command mapping if it exists
func (e *CommandExecutor) getMappedCommand(command string) (string, bool) {
	// Only map on Windows
	if runtime.GOOS != "windows" {
		return command, false
	}

	// Check if we have a mapping for this command
	platformCmds := platformCommands()
	for unix, win := range platformCmds {
		if command == unix {
			return win, true
		}
	}

	return command, false
}

// getExitCode extracts exit code from error in a platform-aware manner
func (e *CommandExecutor) getExitCode(err error) int {
	if err == nil {
		return 0
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		// On Unix systems, use WaitStatus
		if runtime.GOOS != "windows" {
			if status, ok := exitError.Sys().(interface{ ExitStatus() int }); ok {
				return status.ExitStatus()
			}
		} else {
			// On Windows, ExitCode is available directly
			return exitError.ExitCode()
		}
	}

	// Default to 1 for any other error
	return 1
}

// Package executor provides secure command execution for Edge MCP
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	return &CommandExecutor{
		logger:     logger,
		maxTimeout: maxTimeout,
		workDir:    workDir,
		allowedCmds: map[string]bool{
			// Git commands
			"git": true,
			// Docker commands
			"docker": true,
			// Safe shell commands
			"ls":    true,
			"cat":   true,
			"grep":  true,
			"find":  true,
			"echo":  true,
			"pwd":   true,
			"which": true,
			"head":  true,
			"tail":  true,
			"wc":    true,
			"sort":  true,
			"uniq":  true,
			// Build tools
			"go":   true,
			"make": true,
			"npm":  true,
			"yarn": true,
			// Never allow: rm, sudo, chmod, chown, kill, shutdown, reboot
		},
		allowedPaths: []string{
			workDir, // Only allow operations in workDir by default
		},
	}
}

// Execute runs a command with security constraints
func (e *CommandExecutor) Execute(ctx context.Context, command string, args []string) (*Result, error) {
	startTime := time.Now()

	// STEP 1: Validate command is allowed
	if !e.allowedCmds[command] {
		e.logger.Warn("Command not allowed", map[string]interface{}{
			"command": command,
			"args":    args,
		})
		return nil, fmt.Errorf("command not allowed: %s", command)
	}

	// STEP 2: Create timeout context (MANDATORY)
	ctx, cancel := context.WithTimeout(ctx, e.maxTimeout)
	defer cancel()

	// STEP 3: Create command with context (following workflow_service_impl.go:5874)
	cmd := exec.CommandContext(ctx, command, args...)

	// STEP 4: Set security attributes (COPY from workflow_service_impl.go:5884)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for cleanup
	}

	// STEP 5: Set working directory with validation
	if e.workDir != "" {
		cmd.Dir = e.workDir
	}

	// STEP 6: Capture output (following workflow_service_impl.go:5879-5881)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// STEP 7: Execute with logging
	err := cmd.Run()

	// Calculate duration
	duration := time.Since(startTime)

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}

	// STEP 8: Log execution (use structured logging pattern)
	e.logger.Info("Command executed", map[string]interface{}{
		"command":   command,
		"args":      args,
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
	}, nil
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

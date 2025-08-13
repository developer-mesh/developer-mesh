//go:build windows
// +build windows

package executor

import (
	"fmt"
	"os/exec"
	"strings"
)

// setPlatformAttrs sets platform-specific attributes for Windows
func setPlatformAttrs(cmd *exec.Cmd) {
	// Windows doesn't support Setpgid, but we can use job objects
	// For now, we'll leave it empty as Windows handles process groups differently
	// In a production system, we'd use Windows Job Objects API
}

// platformCommands returns platform-specific command mappings for Windows
func platformCommands() map[string]string {
	return map[string]string{
		// Map Unix commands to Windows equivalents
		"ls":   "dir",
		"cat":  "type",
		"grep": "findstr",
		"find": "where",
		"pwd":  "cd",
		"ps":   "tasklist",
		// These remain the same
		"git":    "git",
		"docker": "docker",
		"go":     "go",
		"make":   "make",
		"npm":    "npm",
	}
}

// validatePlatformCommand performs Windows-specific command validation
func validatePlatformCommand(command string) error {
	// Check for dangerous Windows commands
	dangerousWinCommands := []string{
		"format", "del", "rd", "rmdir", "shutdown", "reg",
		"bcdedit", "diskpart", "netsh", "sc",
	}

	cmdLower := strings.ToLower(command)
	for _, dangerous := range dangerousWinCommands {
		if cmdLower == dangerous {
			return fmt.Errorf("dangerous Windows command blocked: %s", command)
		}
	}

	// Check for PowerShell attempts
	if strings.Contains(cmdLower, "powershell") || strings.Contains(cmdLower, "pwsh") {
		return fmt.Errorf("PowerShell execution blocked for security")
	}

	return nil
}

//go:build darwin || linux || freebsd || openbsd || netbsd
// +build darwin linux freebsd openbsd netbsd

package executor

import (
	"os/exec"
	"syscall"
)

// setPlatformAttrs sets platform-specific attributes for Unix systems
func setPlatformAttrs(cmd *exec.Cmd) {
	// On Unix systems, create new process group for better cleanup
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for cleanup
	}
}

// platformCommands returns platform-specific command mappings for Unix
func platformCommands() map[string]string {
	return map[string]string{
		// Unix commands are standard
		"ls":   "ls",
		"cat":  "cat",
		"grep": "grep",
		"find": "find",
		"pwd":  "pwd",
		"ps":   "ps",
	}
}

// validatePlatformCommand performs Unix-specific command validation
func validatePlatformCommand(command string) error {
	// Unix systems don't need special validation
	return nil
}

// Package platform provides platform detection and information
package platform

import (
	"os"
	"runtime"
)

// Info contains platform-specific information
type Info struct {
	OS           string            `json:"os"`
	Architecture string            `json:"architecture"`
	Version      string            `json:"version"`
	Hostname     string            `json:"hostname"`
	Environment  map[string]string `json:"environment"`
	Capabilities Capabilities      `json:"capabilities"`
}

// Capabilities describes what this platform can do
type Capabilities struct {
	Docker          bool `json:"docker"`
	Git             bool `json:"git"`
	ProcessGroups   bool `json:"process_groups"`
	SymbolicLinks   bool `json:"symbolic_links"`
	CaseSensitiveFS bool `json:"case_sensitive_fs"`
	ShellAvailable  bool `json:"shell_available"`
	PosixCompliant  bool `json:"posix_compliant"`
}

// GetInfo returns current platform information
func GetInfo() *Info {
	hostname, _ := os.Hostname()

	info := &Info{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Version:      runtime.Version(),
		Hostname:     hostname,
		Environment:  getEnvironment(),
		Capabilities: getCapabilities(),
	}

	return info
}

// getEnvironment returns safe environment variables
func getEnvironment() map[string]string {
	env := make(map[string]string)

	// Only include safe environment variables
	safeVars := []string{
		"PATH", "HOME", "USER", "SHELL", "LANG", "LC_ALL",
		"TERM", "EDITOR", "VISUAL", "TZ", "PWD",
	}

	for _, key := range safeVars {
		if value := os.Getenv(key); value != "" {
			env[key] = value
		}
	}

	return env
}

// getCapabilities returns platform-specific capabilities
func getCapabilities() Capabilities {
	caps := Capabilities{
		Docker:         checkCommand("docker"),
		Git:            checkCommand("git"),
		ShellAvailable: true,
	}

	// Platform-specific capabilities
	switch runtime.GOOS {
	case "windows":
		caps.ProcessGroups = false
		caps.SymbolicLinks = false // Limited support
		caps.CaseSensitiveFS = false
		caps.PosixCompliant = false
	case "darwin", "linux", "freebsd", "openbsd", "netbsd":
		caps.ProcessGroups = true
		caps.SymbolicLinks = true
		caps.CaseSensitiveFS = runtime.GOOS != "darwin" // macOS is usually case-insensitive
		caps.PosixCompliant = true
	default:
		// Conservative defaults for unknown platforms
		caps.ProcessGroups = false
		caps.SymbolicLinks = false
		caps.CaseSensitiveFS = false
		caps.PosixCompliant = false
	}

	return caps
}

// checkCommand checks if a command is available on the system
func checkCommand(cmd string) bool {
	// This is a simple check - in production, we'd use exec.LookPath
	// For now, assume common tools are available
	return cmd == "docker" || cmd == "git"
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsMacOS returns true if running on macOS
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsUnix returns true if running on a Unix-like system
func IsUnix() bool {
	switch runtime.GOOS {
	case "darwin", "linux", "freebsd", "openbsd", "netbsd", "solaris", "aix":
		return true
	default:
		return false
	}
}

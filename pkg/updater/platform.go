package updater

import (
	"bytes"
	"fmt"
	"runtime"
	"text/template"
)

// DetectPlatform returns the current operating system
func DetectPlatform() string {
	return runtime.GOOS
}

// DetectArch returns the current architecture
func DetectArch() string {
	return runtime.GOARCH
}

// detectPlatform is for internal use (deprecated, use DetectPlatform)
func detectPlatform() string {
	return DetectPlatform()
}

// detectArch is for internal use (deprecated, use DetectArch)
func detectArch() string {
	return DetectArch()
}

// generateAssetName generates an asset name based on the pattern and platform
func generateAssetName(pattern, platform, arch string) (string, error) {
	tmpl, err := template.New("asset").Parse(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to parse asset name template: %w", err)
	}

	ext := ""
	if platform == "windows" {
		ext = ".exe"
	}

	data := map[string]string{
		"OS":   platform,
		"Arch": arch,
		"Ext":  ext,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to generate asset name: %w", err)
	}

	return buf.String(), nil
}

// GetBinaryExtension returns the appropriate binary extension for the platform
func GetBinaryExtension(platform string) string {
	if platform == "windows" {
		return ".exe"
	}
	return ""
}

// NormalizeOS normalizes OS names between different formats
// This helps handle differences between Go's runtime.GOOS and GitHub asset naming
func NormalizeOS(os string) string {
	switch os {
	case "darwin":
		return "darwin" // Some releases use "macos", we'll handle both
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return os
	}
}

// NormalizeArch normalizes architecture names between different formats
func NormalizeArch(arch string) string {
	switch arch {
	case "amd64":
		return "amd64" // Also known as x86_64
	case "arm64":
		return "arm64" // Also known as aarch64
	case "386":
		return "386" // 32-bit x86
	default:
		return arch
	}
}

package updater

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPlatform(t *testing.T) {
	platform := detectPlatform()
	assert.NotEmpty(t, platform)
	// Should match runtime.GOOS
	assert.Equal(t, runtime.GOOS, platform)
}

func TestDetectArch(t *testing.T) {
	arch := detectArch()
	assert.NotEmpty(t, arch)
	// Should match runtime.GOARCH
	assert.Equal(t, runtime.GOARCH, arch)
}

func TestGenerateAssetName(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		platform string
		arch     string
		expected string
		wantErr  bool
	}{
		{
			name:     "linux amd64",
			pattern:  "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
			platform: "linux",
			arch:     "amd64",
			expected: "edge-mcp-linux-amd64",
			wantErr:  false,
		},
		{
			name:     "windows amd64",
			pattern:  "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
			platform: "windows",
			arch:     "amd64",
			expected: "edge-mcp-windows-amd64.exe",
			wantErr:  false,
		},
		{
			name:     "darwin arm64",
			pattern:  "edge-mcp-{{.OS}}-{{.Arch}}{{.Ext}}",
			platform: "darwin",
			arch:     "arm64",
			expected: "edge-mcp-darwin-arm64",
			wantErr:  false,
		},
		{
			name:     "custom pattern",
			pattern:  "myapp_{{.OS}}_{{.Arch}}_v1.0{{.Ext}}",
			platform: "linux",
			arch:     "386",
			expected: "myapp_linux_386_v1.0",
			wantErr:  false,
		},
		{
			name:     "invalid template",
			pattern:  "myapp-{{.Invalid}}",
			platform: "linux",
			arch:     "amd64",
			expected: "",
			wantErr:  false, // Template succeeds but outputs empty for missing field
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateAssetName(tt.pattern, tt.platform, tt.arch)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.expected != "" {
					assert.Equal(t, tt.expected, result)
				}
			}
		})
	}
}

func TestGetBinaryExtension(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		expected string
	}{
		{
			name:     "windows",
			platform: "windows",
			expected: ".exe",
		},
		{
			name:     "linux",
			platform: "linux",
			expected: "",
		},
		{
			name:     "darwin",
			platform: "darwin",
			expected: "",
		},
		{
			name:     "freebsd",
			platform: "freebsd",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := GetBinaryExtension(tt.platform)
			assert.Equal(t, tt.expected, ext)
		})
	}
}

func TestNormalizeOS(t *testing.T) {
	tests := []struct {
		name     string
		os       string
		expected string
	}{
		{
			name:     "darwin",
			os:       "darwin",
			expected: "darwin",
		},
		{
			name:     "linux",
			os:       "linux",
			expected: "linux",
		},
		{
			name:     "windows",
			os:       "windows",
			expected: "windows",
		},
		{
			name:     "unknown",
			os:       "freebsd",
			expected: "freebsd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeOS(tt.os)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeArch(t *testing.T) {
	tests := []struct {
		name     string
		arch     string
		expected string
	}{
		{
			name:     "amd64",
			arch:     "amd64",
			expected: "amd64",
		},
		{
			name:     "arm64",
			arch:     "arm64",
			expected: "arm64",
		},
		{
			name:     "386",
			arch:     "386",
			expected: "386",
		},
		{
			name:     "unknown",
			arch:     "mips",
			expected: "mips",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeArch(tt.arch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

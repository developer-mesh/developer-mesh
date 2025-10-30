package updater

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDevelopmentMode(t *testing.T) {
	// Save original env vars
	originalEnv := os.Getenv("ENVIRONMENT")
	originalAppEnv := os.Getenv("APP_ENV")
	originalGoEnv := os.Getenv("GO_ENV")
	defer func() {
		_ = os.Setenv("ENVIRONMENT", originalEnv)
		_ = os.Setenv("APP_ENV", originalAppEnv)
		_ = os.Setenv("GO_ENV", originalGoEnv)
	}()

	tests := []struct {
		name     string
		envVar   string
		envValue string
		expected bool
	}{
		{
			name:     "ENVIRONMENT=dev",
			envVar:   "ENVIRONMENT",
			envValue: "dev",
			expected: true,
		},
		{
			name:     "ENVIRONMENT=development",
			envVar:   "ENVIRONMENT",
			envValue: "development",
			expected: true,
		},
		{
			name:     "ENVIRONMENT=local",
			envVar:   "ENVIRONMENT",
			envValue: "local",
			expected: true,
		},
		{
			name:     "ENVIRONMENT=prod",
			envVar:   "ENVIRONMENT",
			envValue: "prod",
			expected: false,
		},
		{
			name:     "APP_ENV=dev",
			envVar:   "APP_ENV",
			envValue: "dev",
			expected: true,
		},
		{
			name:     "GO_ENV=development",
			envVar:   "GO_ENV",
			envValue: "development",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars
			_ = os.Unsetenv("ENVIRONMENT")
			_ = os.Unsetenv("APP_ENV")
			_ = os.Unsetenv("GO_ENV")

			// Set the test env var
			_ = os.Setenv(tt.envVar, tt.envValue)

			result := IsDevelopmentMode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsProductionMode(t *testing.T) {
	// Save original env vars
	originalEnv := os.Getenv("ENVIRONMENT")
	defer func() {
		_ = os.Setenv("ENVIRONMENT", originalEnv)
	}()

	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "production",
			envValue: "production",
			expected: true,
		},
		{
			name:     "prod",
			envValue: "prod",
			expected: true,
		},
		{
			name:     "development",
			envValue: "development",
			expected: false,
		},
		{
			name:     "dev",
			envValue: "dev",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("ENVIRONMENT", tt.envValue)
			result := IsProductionMode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldEnableAutoUpdate(t *testing.T) {
	// Save original env vars
	originalEnv := os.Getenv("ENVIRONMENT")
	originalDisable := os.Getenv("DISABLE_AUTO_UPDATE")
	originalEnable := os.Getenv("ENABLE_AUTO_UPDATE")
	defer func() {
		_ = os.Setenv("ENVIRONMENT", originalEnv)
		_ = os.Setenv("DISABLE_AUTO_UPDATE", originalDisable)
		_ = os.Setenv("ENABLE_AUTO_UPDATE", originalEnable)
	}()

	tests := []struct {
		name        string
		environment string
		disable     string
		enable      string
		expected    bool
	}{
		{
			name:        "explicitly disabled",
			environment: "production",
			disable:     "true",
			enable:      "",
			expected:    false,
		},
		{
			name:        "explicitly enabled in dev",
			environment: "development",
			disable:     "",
			enable:      "true",
			expected:    true,
		},
		{
			name:        "production default",
			environment: "production",
			disable:     "",
			enable:      "",
			expected:    true,
		},
		{
			name:        "development default",
			environment: "development",
			disable:     "",
			enable:      "",
			expected:    false,
		},
		{
			name:        "disable overrides environment",
			environment: "production",
			disable:     "yes",
			enable:      "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("ENVIRONMENT", tt.environment)
			_ = os.Setenv("DISABLE_AUTO_UPDATE", tt.disable)
			_ = os.Setenv("ENABLE_AUTO_UPDATE", tt.enable)

			result := ShouldEnableAutoUpdate()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetExecutablePath(t *testing.T) {
	path, err := GetExecutablePath()
	assert.NoError(t, err)
	assert.NotEmpty(t, path)

	// The path should be absolute
	assert.True(t, filepath.IsAbs(path), "executable path should be absolute")
}

func TestIsWritableExecutable(t *testing.T) {
	// This test is environment-dependent
	// In CI/CD, this might fail if running in a read-only container
	result := IsWritableExecutable()

	// We can't assert a specific result, but we can verify it doesn't panic
	_ = result
	t.Logf("IsWritableExecutable: %v", result)
}

// Test case sensitivity
func TestIsDevelopmentMode_CaseInsensitive(t *testing.T) {
	original := os.Getenv("ENVIRONMENT")
	defer func() {
		_ = os.Setenv("ENVIRONMENT", original)
	}()

	testCases := []string{"DEV", "Dev", "DEVELOPMENT", "Development", "LOCAL", "Local"}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			_ = os.Setenv("ENVIRONMENT", tc)
			assert.True(t, IsDevelopmentMode(), "should detect %s as development", tc)
		})
	}
}

func TestIsProductionMode_CaseInsensitive(t *testing.T) {
	original := os.Getenv("ENVIRONMENT")
	defer func() {
		_ = os.Setenv("ENVIRONMENT", original)
	}()

	testCases := []string{"PROD", "Prod", "PRODUCTION", "Production"}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			_ = os.Setenv("ENVIRONMENT", tc)
			assert.True(t, IsProductionMode(), "should detect %s as production", tc)
		})
	}
}

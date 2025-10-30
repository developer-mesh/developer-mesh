package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *Version
		expectError bool
	}{
		{
			name:  "simple version",
			input: "1.2.3",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
			},
			expectError: false,
		},
		{
			name:  "version with v prefix",
			input: "v1.2.3",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
			},
			expectError: false,
		},
		{
			name:  "version with prerelease",
			input: "1.2.3-beta.1",
			expected: &Version{
				Major:      1,
				Minor:      2,
				Patch:      3,
				Prerelease: "beta.1",
			},
			expectError: false,
		},
		{
			name:  "version with build metadata",
			input: "1.2.3+build.123",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Build: "build.123",
			},
			expectError: false,
		},
		{
			name:  "version with prerelease and build",
			input: "1.2.3-rc.1+build.456",
			expected: &Version{
				Major:      1,
				Minor:      2,
				Patch:      3,
				Prerelease: "rc.1",
				Build:      "build.456",
			},
			expectError: false,
		},
		{
			name:  "major only",
			input: "1",
			expected: &Version{
				Major: 1,
				Minor: 0,
				Patch: 0,
			},
			expectError: false,
		},
		{
			name:  "major.minor only",
			input: "1.2",
			expected: &Version{
				Major: 1,
				Minor: 2,
				Patch: 0,
			},
			expectError: false,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid major",
			input:       "x.2.3",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid minor",
			input:       "1.x.3",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid patch",
			input:       "1.2.x",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseVersion(tt.input)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.Major, result.Major)
				assert.Equal(t, tt.expected.Minor, result.Minor)
				assert.Equal(t, tt.expected.Patch, result.Patch)
				assert.Equal(t, tt.expected.Prerelease, result.Prerelease)
				assert.Equal(t, tt.expected.Build, result.Build)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		name     string
		version  *Version
		expected string
	}{
		{
			name: "simple version",
			version: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
			},
			expected: "1.2.3",
		},
		{
			name: "version with prerelease",
			version: &Version{
				Major:      1,
				Minor:      2,
				Patch:      3,
				Prerelease: "beta.1",
			},
			expected: "1.2.3-beta.1",
		},
		{
			name: "version with build",
			version: &Version{
				Major: 1,
				Minor: 2,
				Patch: 3,
				Build: "build.123",
			},
			expected: "1.2.3+build.123",
		},
		{
			name: "version with prerelease and build",
			version: &Version{
				Major:      1,
				Minor:      2,
				Patch:      3,
				Prerelease: "rc.1",
				Build:      "build.456",
			},
			expected: "1.2.3-rc.1+build.456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.version.String())
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int // -1, 0, 1
	}{
		{
			name:     "equal versions",
			v1:       "1.2.3",
			v2:       "1.2.3",
			expected: 0,
		},
		{
			name:     "v1 > v2 (major)",
			v1:       "2.0.0",
			v2:       "1.9.9",
			expected: 1,
		},
		{
			name:     "v1 < v2 (major)",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: -1,
		},
		{
			name:     "v1 > v2 (minor)",
			v1:       "1.5.0",
			v2:       "1.4.9",
			expected: 1,
		},
		{
			name:     "v1 < v2 (minor)",
			v1:       "1.4.0",
			v2:       "1.5.0",
			expected: -1,
		},
		{
			name:     "v1 > v2 (patch)",
			v1:       "1.2.4",
			v2:       "1.2.3",
			expected: 1,
		},
		{
			name:     "v1 < v2 (patch)",
			v1:       "1.2.3",
			v2:       "1.2.4",
			expected: -1,
		},
		{
			name:     "stable > prerelease",
			v1:       "1.2.3",
			v2:       "1.2.3-beta.1",
			expected: 1,
		},
		{
			name:     "prerelease < stable",
			v1:       "1.2.3-beta.1",
			v2:       "1.2.3",
			expected: -1,
		},
		{
			name:     "prerelease comparison",
			v1:       "1.2.3-beta.2",
			v2:       "1.2.3-beta.1",
			expected: 1,
		},
		{
			name:     "build metadata ignored",
			v1:       "1.2.3+build.123",
			v2:       "1.2.3+build.456",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver1, err := ParseVersion(tt.v1)
			require.NoError(t, err)

			ver2, err := ParseVersion(tt.v2)
			require.NoError(t, err)

			result := ver1.Compare(ver2)
			assert.Equal(t, tt.expected, result, "expected %d, got %d", tt.expected, result)
		})
	}
}

func TestIsNewerThan(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{
			name:     "newer version",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: true,
		},
		{
			name:     "older version",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: false,
		},
		{
			name:     "equal version",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: false,
		},
		{
			name:     "stable newer than prerelease",
			v1:       "1.0.0",
			v2:       "1.0.0-beta.1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver1, err := ParseVersion(tt.v1)
			require.NoError(t, err)

			ver2, err := ParseVersion(tt.v2)
			require.NoError(t, err)

			result := ver1.IsNewerThan(ver2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsOlderThan(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{
			name:     "older version",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: true,
		},
		{
			name:     "newer version",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: false,
		},
		{
			name:     "equal version",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver1, err := ParseVersion(tt.v1)
			require.NoError(t, err)

			ver2, err := ParseVersion(tt.v2)
			require.NoError(t, err)

			result := ver1.IsOlderThan(ver2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEquals(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{
			name:     "equal versions",
			v1:       "1.2.3",
			v2:       "1.2.3",
			expected: true,
		},
		{
			name:     "different versions",
			v1:       "1.2.3",
			v2:       "1.2.4",
			expected: false,
		},
		{
			name:     "build metadata ignored",
			v1:       "1.2.3+build.123",
			v2:       "1.2.3+build.456",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver1, err := ParseVersion(tt.v1)
			require.NoError(t, err)

			ver2, err := ParseVersion(tt.v2)
			require.NoError(t, err)

			result := ver1.Equals(ver2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsStable(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{
			name:     "stable version",
			version:  "1.2.3",
			expected: true,
		},
		{
			name:     "prerelease version",
			version:  "1.2.3-beta.1",
			expected: false,
		},
		{
			name:     "stable with build",
			version:  "1.2.3+build.123",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, err := ParseVersion(tt.version)
			require.NoError(t, err)

			result := ver.IsStable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompareVersionStrings(t *testing.T) {
	tests := []struct {
		name        string
		v1          string
		v2          string
		expected    int
		expectError bool
	}{
		{
			name:        "v1 > v2",
			v1:          "2.0.0",
			v2:          "1.0.0",
			expected:    1,
			expectError: false,
		},
		{
			name:        "v1 < v2",
			v1:          "1.0.0",
			v2:          "2.0.0",
			expected:    -1,
			expectError: false,
		},
		{
			name:        "v1 == v2",
			v1:          "1.0.0",
			v2:          "1.0.0",
			expected:    0,
			expectError: false,
		},
		{
			name:        "invalid v1",
			v1:          "invalid",
			v2:          "1.0.0",
			expected:    0,
			expectError: true,
		},
		{
			name:        "invalid v2",
			v1:          "1.0.0",
			v2:          "invalid",
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompareVersionStrings(tt.v1, tt.v2)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name        string
		v1          string
		v2          string
		expected    bool
		expectError bool
	}{
		{
			name:        "newer version",
			v1:          "2.0.0",
			v2:          "1.0.0",
			expected:    true,
			expectError: false,
		},
		{
			name:        "older version",
			v1:          "1.0.0",
			v2:          "2.0.0",
			expected:    false,
			expectError: false,
		},
		{
			name:        "equal version",
			v1:          "1.0.0",
			v2:          "1.0.0",
			expected:    false,
			expectError: false,
		},
		{
			name:        "invalid v1",
			v1:          "invalid",
			v2:          "1.0.0",
			expected:    false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsNewerVersion(tt.v1, tt.v2)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

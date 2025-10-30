package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Version
		wantErr  bool
	}{
		{
			name:  "simple version",
			input: "1.2.3",
			expected: &Version{
				Major: 1, Minor: 2, Patch: 3,
				Raw: "1.2.3",
			},
		},
		{
			name:  "version with v prefix",
			input: "v1.2.3",
			expected: &Version{
				Major: 1, Minor: 2, Patch: 3,
				Raw: "v1.2.3",
			},
		},
		{
			name:  "version with prerelease",
			input: "1.0.0-beta.1",
			expected: &Version{
				Major: 1, Minor: 0, Patch: 0,
				Prerelease: "beta.1",
				Raw:        "1.0.0-beta.1",
			},
		},
		{
			name:  "version with build metadata",
			input: "1.0.0+build.123",
			expected: &Version{
				Major: 1, Minor: 0, Patch: 0,
				Build: "build.123",
				Raw:   "1.0.0+build.123",
			},
		},
		{
			name:  "git describe format",
			input: "0.0.9-1-gca935e4e",
			expected: &Version{
				Major: 0, Minor: 0, Patch: 9,
				Prerelease: "dev.1.gca935e4e",
				Raw:        "0.0.9-1-gca935e4e",
			},
		},
		{
			name:  "git describe format with dirty",
			input: "0.0.9-1-gca935e4e-dirty",
			expected: &Version{
				Major: 0, Minor: 0, Patch: 9,
				Prerelease: "dev.1.gca935e4e",
				Dirty:      true,
				Raw:        "0.0.9-1-gca935e4e-dirty",
			},
		},
		{
			name:  "dev version",
			input: "dev",
			expected: &Version{
				Major: 0, Minor: 0, Patch: 0,
				Prerelease: "dev",
				Raw:        "dev",
			},
		},
		{
			name:  "version with dirty flag",
			input: "1.0.0-dirty",
			expected: &Version{
				Major: 1, Minor: 0, Patch: 0,
				Dirty: true,
				Raw:   "1.0.0-dirty",
			},
		},
		{
			name:    "empty version",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid version",
			input:   "not-a-version",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseVersion(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int // -1: v1 < v2, 0: v1 == v2, 1: v1 > v2
	}{
		// Basic comparisons
		{"equal versions", "1.0.0", "1.0.0", 0},
		{"major version difference", "2.0.0", "1.0.0", 1},
		{"minor version difference", "1.1.0", "1.0.0", 1},
		{"patch version difference", "1.0.1", "1.0.0", 1},

		// Prerelease comparisons
		{"stable vs prerelease", "1.0.0", "1.0.0-beta", 1},
		{"prerelease vs stable", "1.0.0-beta", "1.0.0", -1},
		{"beta vs alpha", "1.0.0-beta", "1.0.0-alpha", 1},
		{"beta.1 vs beta.2", "1.0.0-beta.1", "1.0.0-beta.2", -1},

		// Dev versions
		{"dev vs stable", "dev", "1.0.0", -1},
		{"dev vs dev", "dev", "dev", 0},
		{"dev build vs stable", "0.0.9-1-gca935e4e", "1.0.0", -1},

		// Dirty versions
		{"dirty vs clean", "1.0.0-dirty", "1.0.0", -1},
		{"clean vs dirty", "1.0.0", "1.0.0-dirty", 1},
		{"dirty vs dirty", "1.0.0-dirty", "1.0.0-dirty", 0},

		// Git describe format
		{"git describe same base", "0.0.9-1-gabc", "0.0.9-2-gdef", -1},
		{"git describe vs release", "0.0.9-1-gabc", "0.0.9", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := ParseVersion(tt.v1)
			assert.NoError(t, err)

			v2, err := ParseVersion(tt.v2)
			assert.NoError(t, err)

			result := v1.Compare(v2)
			assert.Equal(t, tt.expected, result,
				"Expected %s compared to %s to return %d, got %d",
				tt.v1, tt.v2, tt.expected, result)

			// Test convenience methods
			if tt.expected > 0 {
				assert.True(t, v1.IsNewer(v2))
				assert.False(t, v1.IsOlder(v2))
				assert.False(t, v1.IsEqual(v2))
			} else if tt.expected < 0 {
				assert.False(t, v1.IsNewer(v2))
				assert.True(t, v1.IsOlder(v2))
				assert.False(t, v1.IsEqual(v2))
			} else {
				assert.False(t, v1.IsNewer(v2))
				assert.False(t, v1.IsOlder(v2))
				assert.True(t, v1.IsEqual(v2))
			}
		})
	}
}

func TestVersionFlags(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		isDev         bool
		isPrerelease  bool
		isStable      bool
	}{
		{"stable release", "1.0.0", false, false, true},
		{"beta release", "1.0.0-beta.1", false, true, false},
		{"dev version", "dev", true, true, false},
		{"dirty version", "1.0.0-dirty", true, false, false},
		{"git describe dev", "0.0.9-1-gabc", true, true, false},
		{"zero version", "0.0.0", true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVersion(tt.version)
			assert.NoError(t, err)

			assert.Equal(t, tt.isDev, v.IsDevelopment(),
				"IsDevelopment() for %s", tt.version)
			assert.Equal(t, tt.isPrerelease, v.IsPrerelease(),
				"IsPrerelease() for %s", tt.version)
			assert.Equal(t, tt.isStable, v.IsStable(),
				"IsStable() for %s", tt.version)
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
				Major: 1, Minor: 2, Patch: 3,
			},
			expected: "1.2.3",
		},
		{
			name: "version with prerelease",
			version: &Version{
				Major: 1, Minor: 0, Patch: 0,
				Prerelease: "beta.1",
			},
			expected: "1.0.0-beta.1",
		},
		{
			name: "version with build",
			version: &Version{
				Major: 1, Minor: 0, Patch: 0,
				Build: "build.123",
			},
			expected: "1.0.0+build.123",
		},
		{
			name: "version with dirty flag",
			version: &Version{
				Major: 1, Minor: 0, Patch: 0,
				Dirty: true,
			},
			expected: "1.0.0-dirty",
		},
		{
			name: "full version",
			version: &Version{
				Major: 1, Minor: 0, Patch: 0,
				Prerelease: "beta.1",
				Build:      "build.123",
				Dirty:      true,
			},
			expected: "1.0.0-beta.1+build.123-dirty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesConstraint(t *testing.T) {
	v, _ := ParseVersion("1.2.3")

	tests := []struct {
		constraint string
		matches    bool
	}{
		// Exact matches
		{"1.2.3", true},
		{"v1.2.3", true},
		{"1.2.4", false},

		// Greater than
		{">1.0.0", true},
		{">1.2.3", false},
		{">2.0.0", false},

		// Greater than or equal
		{">=1.0.0", true},
		{">=1.2.3", true},
		{">=2.0.0", false},

		// Less than
		{"<2.0.0", true},
		{"<1.2.3", false},
		{"<1.0.0", false},

		// Less than or equal
		{"<=2.0.0", true},
		{"<=1.2.3", true},
		{"<=1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			result := v.MatchesConstraint(tt.constraint)
			assert.Equal(t, tt.matches, result,
				"Version %s matching constraint %s", v.String(), tt.constraint)
		})
	}
}

func TestRealWorldVersions(t *testing.T) {
	// Test with actual version strings from the project
	tests := []struct {
		name        string
		version     string
		shouldParse bool
	}{
		{"current dev version", "0.0.9-1-gca935e4e-dirty", true},
		{"release version", "0.0.9", true},
		{"edge-mcp prefixed", "edge-mcp-v1.0.0", true},
		{"future version", "1.0.0-beta.1", true},
		{"nightly build", "nightly-20250101", false}, // This format would need special handling
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVersion(tt.version)

			if tt.shouldParse {
				assert.NoError(t, err)
				assert.NotNil(t, v)
				t.Logf("Parsed %s -> %+v", tt.version, v)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
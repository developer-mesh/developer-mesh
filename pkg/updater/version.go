package updater

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version (major.minor.patch).
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string // e.g., "beta.1", "rc.2"
	Build      string // e.g., "20230101.abc123"
}

// ParseVersion parses a version string into a Version struct.
// Supports formats like: "1.2.3", "v1.2.3", "1.2.3-beta.1", "1.2.3+build.123"
func ParseVersion(v string) (*Version, error) {
	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	if v == "" {
		return nil, fmt.Errorf("empty version string")
	}

	version := &Version{}

	// Split on '+' to separate build metadata
	parts := strings.SplitN(v, "+", 2)
	if len(parts) == 2 {
		version.Build = parts[1]
	}

	// Split on '-' to separate prerelease
	parts = strings.SplitN(parts[0], "-", 2)
	if len(parts) == 2 {
		version.Prerelease = parts[1]
	}

	// Parse major.minor.patch
	numbers := strings.Split(parts[0], ".")
	if len(numbers) < 1 {
		return nil, fmt.Errorf("invalid version format: %s", v)
	}

	// Parse major
	major, err := strconv.Atoi(numbers[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %w", err)
	}
	version.Major = major

	// Parse minor (default to 0 if not present)
	if len(numbers) > 1 {
		minor, err := strconv.Atoi(numbers[1])
		if err != nil {
			return nil, fmt.Errorf("invalid minor version: %w", err)
		}
		version.Minor = minor
	}

	// Parse patch (default to 0 if not present)
	if len(numbers) > 2 {
		patch, err := strconv.Atoi(numbers[2])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %w", err)
		}
		version.Patch = patch
	}

	return version, nil
}

// String returns the string representation of the version.
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare compares two versions.
// Returns:
//
//	-1 if v < other
//	 0 if v == other
//	 1 if v > other
func (v *Version) Compare(other *Version) int {
	// Compare major
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	// Compare minor
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	// Compare patch
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Compare prerelease
	// A version without prerelease is greater than one with prerelease
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		// Lexicographic comparison for prerelease
		if v.Prerelease < other.Prerelease {
			return -1
		}
		return 1
	}

	// Versions are equal (build metadata is ignored for comparison)
	return 0
}

// IsNewerThan returns true if v is newer than other.
func (v *Version) IsNewerThan(other *Version) bool {
	return v.Compare(other) > 0
}

// IsOlderThan returns true if v is older than other.
func (v *Version) IsOlderThan(other *Version) bool {
	return v.Compare(other) < 0
}

// Equals returns true if v equals other.
func (v *Version) Equals(other *Version) bool {
	return v.Compare(other) == 0
}

// IsStable returns true if the version is a stable release (no prerelease).
func (v *Version) IsStable() bool {
	return v.Prerelease == ""
}

// CompareVersionStrings is a convenience function to compare two version strings.
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
//	error if either version string is invalid
func CompareVersionStrings(v1, v2 string) (int, error) {
	ver1, err := ParseVersion(v1)
	if err != nil {
		return 0, fmt.Errorf("invalid version v1: %w", err)
	}

	ver2, err := ParseVersion(v2)
	if err != nil {
		return 0, fmt.Errorf("invalid version v2: %w", err)
	}

	return ver1.Compare(ver2), nil
}

// IsNewerVersion is a convenience function that returns true if v1 > v2.
func IsNewerVersion(v1, v2 string) (bool, error) {
	result, err := CompareVersionStrings(v1, v2)
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

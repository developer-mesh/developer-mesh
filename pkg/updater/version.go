package updater

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version with additional metadata
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string // beta.1, rc.2, etc.
	Build      string // Git commit, build number
	Dirty      bool   // Has uncommitted changes
	Raw        string // Original version string
}

// Common version patterns we need to handle:
// - 1.0.0
// - v1.0.0
// - 1.0.0-beta.1
// - 1.0.0-rc.2+build.123
// - 0.0.9-1-gca935e4e-dirty (git describe format)
// - dev

var (
	// Semantic version regex (simplified)
	semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9\.\-]+))?(?:\+([a-zA-Z0-9\.\-]+))?$`)

	// Git describe format: tag-commits-ghash[-dirty]
	gitDescribeRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)-(\d+)-g([a-f0-9]+)(?:-dirty)?$`)
)

// ParseVersion parses a version string into a Version struct
func ParseVersion(versionString string) (*Version, error) {
	if versionString == "" {
		return nil, fmt.Errorf("empty version string")
	}

	v := &Version{
		Raw: versionString,
	}

	// Check for dirty flag
	if strings.HasSuffix(versionString, "-dirty") {
		v.Dirty = true
		versionString = strings.TrimSuffix(versionString, "-dirty")
	}

	// Special case: dev version
	if versionString == "dev" || versionString == "development" {
		v.Major = 0
		v.Minor = 0
		v.Patch = 0
		v.Prerelease = "dev"
		return v, nil
	}

	// Try git describe format first (common in development)
	if matches := gitDescribeRegex.FindStringSubmatch(versionString); matches != nil {
		v.Major, _ = strconv.Atoi(matches[1])
		v.Minor, _ = strconv.Atoi(matches[2])
		v.Patch, _ = strconv.Atoi(matches[3])

		// Store git metadata as prerelease
		commits := matches[4]
		hash := matches[5]
		if commits != "0" {
			v.Prerelease = fmt.Sprintf("dev.%s.g%s", commits, hash)
		}
		return v, nil
	}

	// Try standard semver
	if matches := semverRegex.FindStringSubmatch(versionString); matches != nil {
		v.Major, _ = strconv.Atoi(matches[1])
		v.Minor, _ = strconv.Atoi(matches[2])
		v.Patch, _ = strconv.Atoi(matches[3])

		if matches[4] != "" {
			v.Prerelease = matches[4]
		}
		if matches[5] != "" {
			v.Build = matches[5]
		}
		return v, nil
	}

	// Fallback: try to extract any version numbers
	parts := strings.Split(versionString, ".")
	if len(parts) >= 3 {
		v.Major, _ = strconv.Atoi(strings.TrimPrefix(parts[0], "v"))
		v.Minor, _ = strconv.Atoi(parts[1])
		v.Patch, _ = strconv.Atoi(parts[2])
		return v, nil
	}

	return nil, fmt.Errorf("unable to parse version: %s", versionString)
}

// Compare compares two versions
// Returns: -1 if v < other, 0 if v == other, 1 if v > other
func (v *Version) Compare(other *Version) int {
	// Major version comparison
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	// Minor version comparison
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	// Patch version comparison
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Prerelease comparison (absence is higher than presence)
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1 // 1.0.0 > 1.0.0-beta
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1 // 1.0.0-beta < 1.0.0
	}
	if v.Prerelease != other.Prerelease {
		// Special handling for dev versions
		if v.Prerelease == "dev" {
			return -1 // dev is always older
		}
		if other.Prerelease == "dev" {
			return 1
		}

		// Lexical comparison for other prereleases
		if v.Prerelease < other.Prerelease {
			return -1
		}
		return 1
	}

	// Dirty flag comparison (dirty is considered older)
	if v.Dirty && !other.Dirty {
		return -1
	}
	if !v.Dirty && other.Dirty {
		return 1
	}

	return 0 // Versions are equal
}

// IsNewer returns true if v is newer than other
func (v *Version) IsNewer(other *Version) bool {
	return v.Compare(other) > 0
}

// IsOlder returns true if v is older than other
func (v *Version) IsOlder(other *Version) bool {
	return v.Compare(other) < 0
}

// IsEqual returns true if versions are equal
func (v *Version) IsEqual(other *Version) bool {
	return v.Compare(other) == 0
}

// IsDevelopment returns true if this is a development version
func (v *Version) IsDevelopment() bool {
	return v.Dirty ||
		v.Prerelease == "dev" ||
		strings.HasPrefix(v.Prerelease, "dev.") ||
		v.Major == 0 && v.Minor == 0 && v.Patch == 0
}

// IsPrerelease returns true if this is a prerelease version
func (v *Version) IsPrerelease() bool {
	return v.Prerelease != ""
}

// IsStable returns true if this is a stable release version
func (v *Version) IsStable() bool {
	return !v.IsDevelopment() && !v.IsPrerelease() && !v.Dirty
}

// String returns the version as a string
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)

	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}

	if v.Build != "" {
		s += "+" + v.Build
	}

	if v.Dirty {
		s += "-dirty"
	}

	return s
}

// MatchesConstraint checks if version matches a constraint
// Examples: ">1.0.0", ">=1.0.0", "<2.0.0", "~1.2.0", "^1.0.0"
func (v *Version) MatchesConstraint(constraint string) bool {
	// This is a simplified implementation
	// For production, consider using a library like github.com/Masterminds/semver

	constraint = strings.TrimSpace(constraint)

	// Handle exact version
	if !strings.ContainsAny(constraint, "<>=^~") {
		other, err := ParseVersion(constraint)
		if err != nil {
			return false
		}
		return v.IsEqual(other)
	}

	// Handle simple comparisons
	if strings.HasPrefix(constraint, ">=") {
		other, err := ParseVersion(constraint[2:])
		if err != nil {
			return false
		}
		return v.Compare(other) >= 0
	}

	if strings.HasPrefix(constraint, ">") {
		other, err := ParseVersion(constraint[1:])
		if err != nil {
			return false
		}
		return v.Compare(other) > 0
	}

	if strings.HasPrefix(constraint, "<=") {
		other, err := ParseVersion(constraint[2:])
		if err != nil {
			return false
		}
		return v.Compare(other) <= 0
	}

	if strings.HasPrefix(constraint, "<") {
		other, err := ParseVersion(constraint[1:])
		if err != nil {
			return false
		}
		return v.Compare(other) < 0
	}

	// For now, don't handle complex constraints like ~, ^
	return false
}
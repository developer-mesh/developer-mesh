# Release Pipeline Changelog Extraction - Fix Summary

## Overview
Fixed critical issues with changelog extraction in release pipelines that were preventing proper release notes generation from CHANGELOG.md.

## Issues Identified

### 1. release.yml (Main Release Pipeline)
**Location**: `.github/workflows/release.yml` lines 117-122

**Problems**:
- Fragile awk pattern that failed silently on extraction errors
- No validation that version exists before extraction
- Poor error reporting - silently fell back to git commits
- Inconsistent regex escaping that could break on edge cases
- No visibility into why extraction failed

**Impact**: Release notes didn't include proper changelog content, falling back to generic git commit messages.

### 2. edge-mcp-release.yml (Edge MCP Release Pipeline)
**Location**: `.github/workflows/edge-mcp-release.yml` lines 193-283

**Problems**:
- **NO changelog extraction at all** - only static template
- Only provided a link to CHANGELOG.md without including content
- Completely ignored version-specific release notes
- No fallback mechanism if changelog missing

**Impact**: Edge MCP releases had no changelog content, only installation instructions.

## Solutions Implemented

### 1. Enhanced release.yml (Main Releases)

**Changes**:
- Improved awk pattern using `index()` function for robust matching
- Added version validation with clear error messages
- Shows available versions in CHANGELOG when version not found
- Proper handling of both `## [VERSION]` and `## [VERSION] - DATE` formats
- Better fallback with git commits including commit hashes
- Clear success/failure indicators with checkmarks

**New Pattern**:
```bash
CHANGELOG=$(awk -v ver="${VERSION}" '
  BEGIN { found = 0 }
  /^## \[/ {
    # Check if this line contains our version
    if (index($0, "[" ver "]") > 0) {
      found = 1
      next
    }
    # If we were capturing and hit a new version header, stop
    if (found) {
      exit
    }
  }
  found { print }
' CHANGELOG.md 2>/dev/null | sed 's/^[[:space:]]*//')
```

**Validation Added**:
```bash
if ! grep -q "^## \[${VERSION}\]" CHANGELOG.md; then
  echo "⚠️  Warning: Version ${VERSION} not found in CHANGELOG.md"
  echo "Looking for pattern: ^## \[${VERSION}\]"
  echo ""
  echo "Available versions in CHANGELOG:"
  grep "^## \[" CHANGELOG.md | head -5
  echo ""
fi
```

### 2. Added Changelog Extraction to edge-mcp-release.yml

**Changes**:
- Implemented complete changelog extraction (previously missing)
- Supports both Edge MCP-specific CHANGELOG and main CHANGELOG
- Handles both `[VERSION]` and `[edge-mcp-VERSION]` formats
- Smart fallback to edge-mcp specific commits when no changelog
- Clear error messages and version validation
- Proper documentation links at the end

**New Extraction Logic**:
```bash
# Check for edge-mcp specific CHANGELOG first
if [ -f "apps/edge-mcp/CHANGELOG.md" ]; then
  CHANGELOG_FILE="apps/edge-mcp/CHANGELOG.md"
else
  CHANGELOG_FILE="CHANGELOG.md"
fi

# Extract with support for both version formats
CHANGELOG=$(awk -v ver="${VERSION}" -v edge_ver="edge-mcp-${VERSION}" '
  BEGIN { found = 0 }
  /^## \[/ {
    if (index($0, "[" ver "]") > 0 || index($0, "[" edge_ver "]") > 0) {
      found = 1
      next
    }
    if (found) {
      exit
    }
  }
  found { print }
' "${CHANGELOG_FILE}" 2>/dev/null | sed 's/^[[:space:]]*//')
```

## Key Improvements

### 1. Robustness
- Uses `index()` function instead of regex `~` for reliable substring matching
- Handles CHANGELOG format variations automatically
- No longer breaks on special characters or unexpected formats

### 2. Visibility
- Clear success/failure indicators (✅/⚠️)
- Shows available versions when target version not found
- Explains what pattern it's looking for
- Includes extraction statistics (lines extracted)

### 3. Validation
- Verifies version exists before extraction
- Checks multiple version formats (plain and prefixed)
- Validates changelog file exists
- Proper error handling with informative messages

### 4. Fallback Strategy
- Main releases: Use git commits with hashes
- Edge MCP: Filter commits for edge-mcp specific changes
- Both: Include link to full changelog
- Clear indication when using fallback

## Testing

Created comprehensive test script: `scripts/test-changelog-extraction.sh`

**Test Coverage**:
1. ✅ Main release with existing version (0.0.6)
2. ✅ Main release with existing version (0.0.5)
3. ✅ Main release with non-existent version (1.0.0) - proper error handling
4. ✅ Edge MCP release with existing version (0.0.6)
5. ✅ Edge MCP release with non-existent version (2.0.0) - proper error handling

**Test Results**:
```
All tests completed!
- Successfully extracts existing versions (282+ lines for v0.0.6)
- Properly handles missing versions with warnings
- Shows available versions when extraction fails
- Validates both pipelines work identically
```

## CHANGELOG Format Requirements

The fixes support the standard Keep a Changelog format:

```markdown
## [Unreleased]

### Added
- New features

## [1.0.0] - 2025-10-17

### Added
- Feature 1
- Feature 2

### Fixed
- Bug fix 1
```

**Supported Formats**:
- `## [VERSION]` - Plain version header
- `## [VERSION] - DATE` - Version with date
- `## [edge-mcp-VERSION]` - Prefixed version (Edge MCP)
- `## [edge-mcp-VERSION] - DATE` - Prefixed with date

## Files Modified

1. `.github/workflows/release.yml` - Enhanced changelog extraction (lines 55-160)
2. `.github/workflows/edge-mcp-release.yml` - Added changelog extraction (lines 193-338)
3. `scripts/test-changelog-extraction.sh` - New test script (150+ lines)

## Usage

### For Main Releases
```bash
# Create tag and push
git tag v1.0.0
git push origin v1.0.0

# GitHub Action will:
# 1. Extract changelog for version 1.0.0 from CHANGELOG.md
# 2. Generate release notes with changelog content
# 3. Fall back to git commits if version not in CHANGELOG
# 4. Show clear warnings if extraction fails
```

### For Edge MCP Releases
```bash
# Create tag and push
git tag edge-mcp-v1.0.0
git push origin edge-mcp-v1.0.0

# GitHub Action will:
# 1. Look for apps/edge-mcp/CHANGELOG.md or CHANGELOG.md
# 2. Extract version 1.0.0 or edge-mcp-1.0.0
# 3. Generate release notes with changelog content
# 4. Fall back to edge-mcp specific commits if needed
```

### Testing Locally
```bash
# Run test suite
./scripts/test-changelog-extraction.sh

# Test specific version
VERSION="0.0.6" ./scripts/test-changelog-extraction.sh
```

## Best Practices

### Before Creating a Release

1. **Update CHANGELOG.md first**:
   ```markdown
   ## [1.0.0] - 2025-10-17

   ### Added
   - New feature

   ### Fixed
   - Bug fix
   ```

2. **Verify version exists**:
   ```bash
   grep "^## \[1.0.0\]" CHANGELOG.md
   ```

3. **Test extraction locally**:
   ```bash
   ./scripts/test-changelog-extraction.sh
   ```

4. **Create and push tag**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

### Changelog Maintenance

- Keep changelog updated with each PR
- Use semantic versioning (MAJOR.MINOR.PATCH)
- Follow Keep a Changelog format
- Include dates in version headers
- Group changes by type (Added, Changed, Fixed, Security, etc.)

## Troubleshooting

### Version Not Found in CHANGELOG
**Symptom**: Warning message during release
```
⚠️  Warning: Version 1.0.0 not found in CHANGELOG.md
```

**Solution**: Add the version to CHANGELOG.md before creating the release tag.

### Empty Changelog Extracted
**Symptom**: Release notes only show git commits

**Cause**: Version header exists but no content between it and next version

**Solution**: Add content between version headers in CHANGELOG.md

### Wrong Version Extracted
**Symptom**: Changelog shows content from different version

**Cause**: Version format mismatch (e.g., looking for `1.0.0` but CHANGELOG has `v1.0.0`)

**Solution**: Use consistent version format without 'v' prefix in CHANGELOG.md

## Future Enhancements

Potential improvements for future releases:

1. **Automatic CHANGELOG Update**: Auto-commit changelog updates during release
2. **Validation Hook**: Pre-release hook to verify changelog is updated
3. **Format Validation**: Check changelog follows Keep a Changelog format
4. **Diff Links**: Auto-generate comparison links between versions
5. **Breaking Changes**: Highlight breaking changes prominently
6. **Migration Guides**: Extract and highlight migration sections

## References

- [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
- [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
- [GitHub Actions: softprops/action-gh-release](https://github.com/softprops/action-gh-release)

## Summary

✅ **Fixed**: Main release changelog extraction now works reliably
✅ **Added**: Edge MCP release changelog extraction (was completely missing)
✅ **Improved**: Error handling with clear warnings and fallbacks
✅ **Tested**: Comprehensive test suite validates all scenarios
✅ **Documented**: Best practices and troubleshooting guide

**Impact**: Release notes now properly include changelog content instead of generic git commits.

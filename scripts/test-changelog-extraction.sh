#!/bin/bash
# Test script to validate changelog extraction for release pipelines
# This simulates what the GitHub Actions workflows do

set -e

echo "======================================"
echo "Changelog Extraction Test"
echo "======================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test function for main release.yml pattern
test_main_release() {
    local VERSION=$1
    echo "Testing main release pattern for version: ${VERSION}"
    echo "--------------------------------------"

    # First, verify the version exists
    if ! grep -q "^## \[${VERSION}\]" CHANGELOG.md; then
        echo -e "${YELLOW}⚠️  Warning: Version ${VERSION} not found in CHANGELOG.md${NC}"
        echo "Looking for pattern: ^## \[${VERSION}\]"
        echo ""
        echo "Available versions in CHANGELOG:"
        grep "^## \[" CHANGELOG.md | head -5
        echo ""
    else
        echo -e "${GREEN}✅ Version ${VERSION} found in CHANGELOG${NC}"
    fi

    # Extract changelog content
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

    if [ -n "$CHANGELOG" ] && [ "$CHANGELOG" != "" ]; then
        echo -e "${GREEN}✅ Successfully extracted changelog for version ${VERSION}${NC}"
        echo ""
        echo "First 10 lines of extracted changelog:"
        echo "--------------------------------------"
        echo "$CHANGELOG" | head -10
        echo "--------------------------------------"
        echo "Total lines extracted: $(echo "$CHANGELOG" | wc -l)"
    else
        echo -e "${RED}❌ Failed to extract changelog for version ${VERSION}${NC}"
        return 1
    fi

    echo ""
}

# Test function for edge-mcp-release.yml pattern
test_edge_mcp_release() {
    local VERSION=$1
    echo "Testing edge-mcp release pattern for version: ${VERSION}"
    echo "--------------------------------------"

    # Check if there's an edge-mcp specific CHANGELOG
    if [ -f "apps/edge-mcp/CHANGELOG.md" ]; then
        CHANGELOG_FILE="apps/edge-mcp/CHANGELOG.md"
        echo "Using Edge MCP specific changelog: ${CHANGELOG_FILE}"
    else
        CHANGELOG_FILE="CHANGELOG.md"
        echo "Using main changelog: ${CHANGELOG_FILE}"
    fi

    # Verify version exists
    if ! grep -q "^## \[${VERSION}\]" "${CHANGELOG_FILE}" 2>/dev/null && \
       ! grep -q "^## \[edge-mcp-${VERSION}\]" "${CHANGELOG_FILE}" 2>/dev/null; then
        echo -e "${YELLOW}⚠️  Warning: Version ${VERSION} not found in ${CHANGELOG_FILE}${NC}"
        echo "Looking for patterns: ^## \[${VERSION}\] or ^## \[edge-mcp-${VERSION}\]"
        echo ""
        echo "Available versions:"
        grep "^## \[" "${CHANGELOG_FILE}" 2>/dev/null | head -5 || echo "No versions found"
        echo ""
    else
        echo -e "${GREEN}✅ Version found in ${CHANGELOG_FILE}${NC}"
    fi

    # Extract changelog - try both version formats
    CHANGELOG=$(awk -v ver="${VERSION}" -v edge_ver="edge-mcp-${VERSION}" '
        BEGIN { found = 0 }
        /^## \[/ {
            # Check for either plain version or edge-mcp prefixed version
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

    if [ -n "$CHANGELOG" ] && [ "$CHANGELOG" != "" ]; then
        echo -e "${GREEN}✅ Successfully extracted changelog for Edge MCP version ${VERSION}${NC}"
        echo ""
        echo "First 10 lines of extracted changelog:"
        echo "--------------------------------------"
        echo "$CHANGELOG" | head -10
        echo "--------------------------------------"
        echo "Total lines extracted: $(echo "$CHANGELOG" | wc -l)"
    else
        echo -e "${RED}❌ Failed to extract changelog for version ${VERSION}${NC}"
        return 1
    fi

    echo ""
}

# Run tests
echo "Test 1: Main release with existing version (0.0.6)"
echo "======================================"
test_main_release "0.0.6"

echo ""
echo "Test 2: Main release with existing version (0.0.5)"
echo "======================================"
test_main_release "0.0.5"

echo ""
echo "Test 3: Main release with non-existent version (1.0.0)"
echo "======================================"
if test_main_release "1.0.0"; then
    echo -e "${RED}❌ Test should have failed for non-existent version${NC}"
    exit 1
else
    echo -e "${GREEN}✅ Correctly handled non-existent version${NC}"
fi

echo ""
echo "Test 4: Edge MCP release with existing version (0.0.6)"
echo "======================================"
test_edge_mcp_release "0.0.6"

echo ""
echo "Test 5: Edge MCP release with non-existent version (2.0.0)"
echo "======================================"
if test_edge_mcp_release "2.0.0"; then
    echo -e "${RED}❌ Test should have failed for non-existent version${NC}"
    exit 1
else
    echo -e "${GREEN}✅ Correctly handled non-existent version${NC}"
fi

echo ""
echo "======================================"
echo -e "${GREEN}All tests completed!${NC}"
echo "======================================"

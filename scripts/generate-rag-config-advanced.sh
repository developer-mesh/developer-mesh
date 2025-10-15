#!/bin/bash
#
# ⚠️  LEGACY SCRIPT - NOT RECOMMENDED FOR NEW DEPLOYMENTS ⚠️
#
# This advanced script generates individual repository configurations with filtering.
# It is kept for reference and backward compatibility only.
#
# RECOMMENDED APPROACH:
# Use the native `github_org` source type instead, which supports all these features
# natively without needing to run scripts:
#
# Example (preferred):
#   sources:
#     - id: my_org
#       type: github_org
#       config:
#         org: your-organization
#         token: ${GITHUB_TOKEN}
#         include_archived: false  # Built-in filtering
#         include_forks: false     # Built-in filtering
#         repos:                   # Optional: specific repos only
#           - "repo1"
#           - "repo2"
#         include_patterns:        # File patterns work the same
#           - "**/*.go"
#           - "**/*.md"
#
# See: docs/guides/rag-loader-multi-org-setup.md
#
set -e

ORG=$1
TOKEN=${GITHUB_TOKEN}
SCHEDULE=${SCHEDULE:-"0 */6 * * *"}
INCLUDE_ARCHIVED=${INCLUDE_ARCHIVED:-false}
INCLUDE_FORKS=${INCLUDE_FORKS:-false}

if [ -z "$ORG" ]; then
    echo "Usage: $0 <organization>"
    echo ""
    echo "⚠️  NOTE: This script is LEGACY. Consider using 'github_org' source type instead."
    echo ""
    echo "Environment variables:"
    echo "  GITHUB_TOKEN        - GitHub personal access token (required)"
    echo "  SCHEDULE           - Cron schedule (default: 0 */6 * * *)"
    echo "  INCLUDE_ARCHIVED   - Include archived repos (default: false)"
    echo "  INCLUDE_FORKS      - Include forked repos (default: false)"
    echo "  LANGUAGE_FILTER    - Only include repos with language (e.g., Go)"
    echo "  REPO_PATTERN       - Only include repos matching pattern (e.g., 'api-*')"
    echo ""
    echo "See: docs/guides/rag-loader-multi-org-setup.md"
    exit 1
fi

if [ -z "$TOKEN" ]; then
    echo "Error: GITHUB_TOKEN environment variable not set"
    exit 1
fi

echo "# Auto-generated RAG loader configuration"
echo "# Organization: $ORG"
echo "# Generated: $(date)"
echo "# Include archived: $INCLUDE_ARCHIVED"
echo "# Include forks: $INCLUDE_FORKS"
echo ""
echo "sources:"

# Function to get all repos (handles pagination)
get_all_repos() {
    local page=1
    local repos=""

    while true; do
        local response=$(curl -s -H "Authorization: token $TOKEN" \
            "https://api.github.com/orgs/$ORG/repos?per_page=100&page=$page&type=all")

        # Check if we got any repos
        local count=$(echo "$response" | jq '. | length')
        if [ "$count" -eq 0 ]; then
            break
        fi

        repos="$repos $response"
        page=$((page + 1))
    done

    echo "$repos" | jq -s 'add'
}

# Get all repos
ALL_REPOS=$(get_all_repos)

# Apply filters
FILTERED_REPOS="$ALL_REPOS"

# Filter archived
if [ "$INCLUDE_ARCHIVED" = "false" ]; then
    FILTERED_REPOS=$(echo "$FILTERED_REPOS" | jq '[.[] | select(.archived == false)]')
fi

# Filter forks
if [ "$INCLUDE_FORKS" = "false" ]; then
    FILTERED_REPOS=$(echo "$FILTERED_REPOS" | jq '[.[] | select(.fork == false)]')
fi

# Filter by language
if [ -n "$LANGUAGE_FILTER" ]; then
    FILTERED_REPOS=$(echo "$FILTERED_REPOS" | jq --arg lang "$LANGUAGE_FILTER" '[.[] | select(.language == $lang)]')
fi

# Filter by name pattern
if [ -n "$REPO_PATTERN" ]; then
    FILTERED_REPOS=$(echo "$FILTERED_REPOS" | jq --arg pattern "$REPO_PATTERN" '[.[] | select(.name | test($pattern))]')
fi

# Generate config for each repo
echo "$FILTERED_REPOS" | jq -r '.[] | .name' | while read REPO; do
    # Get repo details for better configuration
    REPO_DATA=$(echo "$FILTERED_REPOS" | jq -r --arg name "$REPO" '.[] | select(.name == $name)')
    LANGUAGE=$(echo "$REPO_DATA" | jq -r '.language // "unknown"')
    DEFAULT_BRANCH=$(echo "$REPO_DATA" | jq -r '.default_branch')

    # Convert repo name to valid ID
    ID=$(echo "${ORG}_${REPO}" | tr '.-' '_')

    # Determine include patterns based on language
    case "$LANGUAGE" in
        Go)
            PATTERNS='        - "**/*.go"
        - "**/*.md"
        - "go.mod"
        - "go.sum"'
            EXCLUDES='        - "vendor/**"
        - "**/*_test.go"
        - "**/*.pb.go"'
            ;;
        JavaScript|TypeScript)
            PATTERNS='        - "**/*.{js,ts,jsx,tsx}"
        - "**/*.md"
        - "package.json"'
            EXCLUDES='        - "node_modules/**"
        - "dist/**"
        - "**/*.test.{js,ts}"
        - "**/*.min.js"'
            ;;
        Python)
            PATTERNS='        - "**/*.py"
        - "**/*.md"
        - "requirements.txt"
        - "setup.py"'
            EXCLUDES='        - "venv/**"
        - "**/test_*.py"
        - "**/*_test.py"'
            ;;
        *)
            PATTERNS='        - "**/*.md"
        - "**/*.yaml"
        - "**/*.yml"'
            EXCLUDES='        - "vendor/**"
        - "node_modules/**"'
            ;;
    esac

    cat <<EOF
  - id: ${ID}
    type: github
    enabled: true
    schedule: "${SCHEDULE}"
    config:
      owner: ${ORG}
      repo: ${REPO}
      branch: ${DEFAULT_BRANCH}
      token: \${GITHUB_TOKEN}
      include_patterns:
${PATTERNS}
      exclude_patterns:
${EXCLUDES}

EOF
done

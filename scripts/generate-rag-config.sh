#!/bin/bash
#
# ⚠️  LEGACY SCRIPT - NOT RECOMMENDED FOR NEW DEPLOYMENTS ⚠️
#
# This script generates individual repository configurations for the RAG loader.
# It is kept for reference and backward compatibility only.
#
# RECOMMENDED APPROACH:
# Use the native `github_org` source type instead, which automatically discovers
# repositories without needing to run scripts or generate configs.
#
# Example (preferred):
#   sources:
#     - id: my_org
#       type: github_org
#       config:
#         org: your-organization
#         token: ${GITHUB_TOKEN}
#         include_archived: false
#         include_forks: false
#
# See: docs/guides/rag-loader-multi-org-setup.md
#
set -e

ORG=$1
TOKEN=${GITHUB_TOKEN}

if [ -z "$ORG" ]; then
    echo "Usage: $0 <organization>"
    echo ""
    echo "⚠️  NOTE: This script is LEGACY. Consider using 'github_org' source type instead."
    echo "See: docs/guides/rag-loader-multi-org-setup.md"
    exit 1
fi

if [ -z "$TOKEN" ]; then
    echo "Error: GITHUB_TOKEN environment variable not set"
    exit 1
fi

echo "# Auto-generated configuration for organization: $ORG"
echo "# Generated: $(date)"
echo ""
echo "sources:"

# Get all repos for the organization
REPOS=$(curl -s -H "Authorization: token $TOKEN" \
    "https://api.github.com/orgs/$ORG/repos?per_page=100&type=all" | \
    jq -r '.[].name')

for REPO in $REPOS; do
    # Convert repo name to valid ID (replace special chars)
    ID=$(echo "${ORG}_${REPO}" | tr '.-' '_')

    cat <<EOF
  - id: ${ID}
    type: github
    enabled: true
    schedule: "0 */6 * * *"
    config:
      owner: ${ORG}
      repo: ${REPO}
      branch: main
      token: \${GITHUB_TOKEN}
      include_patterns:
        - "**/*.go"
        - "**/*.md"
        - "**/*.yaml"
        - "**/*.yml"
      exclude_patterns:
        - "vendor/**"
        - "node_modules/**"
        - "**/*_test.go"

EOF
done

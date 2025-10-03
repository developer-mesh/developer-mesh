#!/bin/bash
set -e

# Edge MCP SDK Generation Script
# Generates client SDKs from OpenAPI specification using Docker

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SPEC_FILE="${SCRIPT_DIR}/edge-mcp.yaml"
SDK_DIR="${SCRIPT_DIR}/sdks"

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is required but not installed."
    echo "Install Docker from: https://www.docker.com/get-started"
    exit 1
fi

# Check if spec file exists
if [ ! -f "$SPEC_FILE" ]; then
    echo "Error: OpenAPI specification not found at: $SPEC_FILE"
    exit 1
fi

# Parse arguments
LANGUAGE="${1:-all}"

# Function to generate SDK
generate_sdk() {
    local lang=$1
    local output_dir="${SDK_DIR}/${lang}"

    echo "========================================="
    echo "Generating ${lang} SDK..."
    echo "========================================="

    # Remove old SDK
    rm -rf "$output_dir"
    mkdir -p "$output_dir"

    # Generate SDK based on language
    case $lang in
        go)
            docker run --rm -v "${SCRIPT_DIR}:/local" openapitools/openapi-generator-cli generate \
                -i /local/edge-mcp.yaml \
                -g go \
                -o /local/sdks/go \
                --additional-properties=packageName=edgemcp,packageVersion=1.0.0
            ;;
        python)
            docker run --rm -v "${SCRIPT_DIR}:/local" openapitools/openapi-generator-cli generate \
                -i /local/edge-mcp.yaml \
                -g python \
                -o /local/sdks/python \
                --additional-properties=packageName=edgemcp,projectName=edge-mcp-client,packageVersion=1.0.0
            ;;
        typescript)
            docker run --rm -v "${SCRIPT_DIR}:/local" openapitools/openapi-generator-cli generate \
                -i /local/edge-mcp.yaml \
                -g typescript-fetch \
                -o /local/sdks/typescript \
                --additional-properties=npmName=edge-mcp-client,npmVersion=1.0.0
            ;;
        java)
            docker run --rm -v "${SCRIPT_DIR}:/local" openapitools/openapi-generator-cli generate \
                -i /local/edge-mcp.yaml \
                -g java \
                -o /local/sdks/java \
                --additional-properties=artifactId=edge-mcp-client,groupId=com.developer-mesh,artifactVersion=1.0.0
            ;;
        csharp)
            docker run --rm -v "${SCRIPT_DIR}:/local" openapitools/openapi-generator-cli generate \
                -i /local/edge-mcp.yaml \
                -g csharp-netcore \
                -o /local/sdks/csharp \
                --additional-properties=packageName=EdgeMcp.Client,packageVersion=1.0.0
            ;;
        ruby)
            docker run --rm -v "${SCRIPT_DIR}:/local" openapitools/openapi-generator-cli generate \
                -i /local/edge-mcp.yaml \
                -g ruby \
                -o /local/sdks/ruby \
                --additional-properties=gemName=edge_mcp_client,gemVersion=1.0.0
            ;;
        *)
            echo "Error: Unknown language: $lang"
            echo "Supported languages: go, python, typescript, java, csharp, ruby"
            return 1
            ;;
    esac

    echo "✓ ${lang} SDK generated at: ${output_dir}"
    echo ""
}

# Main execution
if [ "$LANGUAGE" = "all" ]; then
    echo "Generating SDKs for all supported languages..."
    echo ""

    for lang in go python typescript; do
        generate_sdk "$lang"
    done

    echo "========================================="
    echo "All SDKs generated successfully!"
    echo "========================================="
    echo ""
    echo "SDK locations:"
    echo "  - Go:         ${SDK_DIR}/go"
    echo "  - Python:     ${SDK_DIR}/python"
    echo "  - TypeScript: ${SDK_DIR}/typescript"
    echo ""
    echo "See examples/ for usage examples."
else
    generate_sdk "$LANGUAGE"
fi

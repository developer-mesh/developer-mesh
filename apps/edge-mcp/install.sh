#!/bin/bash
# Edge MCP Installation Script
# This script downloads and installs the Edge MCP binary for your platform

set -e

# Configuration
REPO="developer-mesh/developer-mesh"
INSTALL_DIR="${EDGE_MCP_INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="edge-mcp"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
print_error() {
    echo -e "${RED}Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_info() {
    echo -e "${YELLOW}$1${NC}"
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    # Map to Go's OS names
    case "$OS" in
        linux*)
            OS="linux"
            ;;
        darwin*)
            OS="darwin"
            ;;
        msys*|mingw*|cygwin*)
            OS="windows"
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac
    
    # Map to Go's architecture names
    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        i386|i686)
            ARCH="386"
            ;;
        armv7l)
            ARCH="arm"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    
    echo "$OS-$ARCH"
}

# Get the latest release version
get_latest_version() {
    local version
    version=$(curl -s "https://api.github.com/repos/${REPO}/releases" | \
              grep '"tag_name":' | \
              grep 'edge-mcp-v' | \
              head -1 | \
              sed -E 's/.*"edge-mcp-v([^"]+)".*/\1/')
    
    if [ -z "$version" ]; then
        # If no edge-mcp release found, use nightly
        echo "nightly"
    else
        echo "$version"
    fi
}

# Download and install Edge MCP
install_edge_mcp() {
    local platform=$1
    local version=$2
    
    print_info "Installing Edge MCP ${version} for ${platform}..."
    
    # Construct download URL
    local binary_file="edge-mcp-${platform}"
    local archive_file="${binary_file}.tar.gz"
    
    if [ "$version" = "nightly" ]; then
        local download_url="https://github.com/${REPO}/releases/download/edge-mcp-nightly/${binary_file}"
        # For nightly, download the binary directly
        print_info "Downloading nightly build..."
        curl -L -o "${BINARY_NAME}" "${download_url}" || {
            print_error "Failed to download Edge MCP"
            exit 1
        }
    else
        local download_url="https://github.com/${REPO}/releases/download/edge-mcp-v${version}/${archive_file}"
        # For releases, download and extract the archive
        print_info "Downloading from: ${download_url}"
        curl -L -o "${archive_file}" "${download_url}" || {
            print_error "Failed to download Edge MCP"
            exit 1
        }
        
        print_info "Extracting archive..."
        tar -xzf "${archive_file}" || {
            print_error "Failed to extract archive"
            exit 1
        }
        
        # Rename to standard binary name
        mv "${binary_file}" "${BINARY_NAME}"
        rm -f "${archive_file}"
    fi
    
    # Make executable
    chmod +x "${BINARY_NAME}"
    
    # Move to install directory
    if [ -w "$INSTALL_DIR" ]; then
        mv "${BINARY_NAME}" "${INSTALL_DIR}/"
    else
        print_info "Installing to ${INSTALL_DIR} requires sudo access..."
        sudo mv "${BINARY_NAME}" "${INSTALL_DIR}/"
    fi
    
    print_success "Edge MCP installed successfully!"
}

# Verify installation
verify_installation() {
    if command -v edge-mcp &> /dev/null; then
        local installed_version=$(edge-mcp --version 2>/dev/null || echo "unknown")
        print_success "Edge MCP is installed: ${installed_version}"
        print_info ""
        print_info "Quick start:"
        print_info "  edge-mcp --port 8082           # Run Edge MCP"
        print_info "  edge-mcp --help                # Show help"
        print_info "  edge-mcp --version             # Show version"
        print_info ""
        print_info "For IDE setup guides, visit:"
        print_info "  https://github.com/${REPO}/tree/main/apps/edge-mcp/docs/ide-setup"
    else
        print_error "Edge MCP installation failed or ${INSTALL_DIR} is not in your PATH"
        print_info "Add ${INSTALL_DIR} to your PATH:"
        print_info "  export PATH=\$PATH:${INSTALL_DIR}"
        exit 1
    fi
}

# Main installation flow
main() {
    print_info "Edge MCP Installer"
    print_info "=================="
    
    # Check for required tools
    for tool in curl tar; do
        if ! command -v $tool &> /dev/null; then
            print_error "$tool is required but not installed"
            exit 1
        fi
    done
    
    # Detect platform
    PLATFORM=$(detect_platform)
    print_info "Detected platform: ${PLATFORM}"
    
    # Get version (from argument or latest)
    VERSION="${1:-$(get_latest_version)}"
    print_info "Version to install: ${VERSION}"
    
    # Create temp directory for download
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    # Install Edge MCP
    install_edge_mcp "$PLATFORM" "$VERSION"
    
    # Cleanup
    cd - > /dev/null
    rm -rf "$TEMP_DIR"
    
    # Verify installation
    verify_installation
}

# Run main function
main "$@"
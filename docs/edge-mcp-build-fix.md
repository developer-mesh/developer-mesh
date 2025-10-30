# Edge MCP Build Version Injection Fix

## Problem Fixed
The Edge MCP binary was always showing version "1.0.0" and commit "unknown" because the ldflags weren't being properly injected during the build process.

## Root Cause
1. **Missing ldflags**: The Makefile's `build-edge-mcp` target didn't include any `-ldflags` to inject version information
2. **GitHub Actions**: Uses `-X main.version` which can fail in certain contexts without proper quoting
3. **Missing variable**: The `buildTime` variable wasn't defined in main.go

## Solution Applied

### 1. Updated Makefile
Added proper version injection to both `build-edge-mcp` and `build-edge-mcp-all` targets:
```makefile
@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
BUILD_TIME=$$(date -u '+%Y-%m-%d_%H:%M:%S'); \
cd apps/edge-mcp && go build \
    -ldflags="-s -w \
        -X 'main.version=$${VERSION}' \
        -X 'main.commit=$${COMMIT}' \
        -X 'main.buildTime=$${BUILD_TIME}'" \
    -o ../../bin/edge-mcp ./cmd/server
```

### 2. Updated main.go
Added the `buildTime` variable and updated version display:
```go
var (
    version   = "1.0.0"
    commit    = "unknown"
    buildTime = "unknown"
)

// Version display now shows:
fmt.Printf("Edge MCP v%s (commit: %s, built: %s)\n", version, commit, buildTime)
```

### 3. Created Build Script
Added `scripts/build-edge-mcp.sh` for convenient development builds with proper version injection.

## GitHub Actions Fix (TODO)
The GitHub Actions workflows should be updated to use quoted ldflags:

**Current (may fail):**
```yaml
-ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}"
```

**Recommended (more robust):**
```yaml
-ldflags="-s -w -X 'main.version=${VERSION}' -X 'main.commit=${COMMIT}' -X 'main.buildTime=${BUILD_TIME}'"
```

## Version Format Explanation

When you run `edge-mcp --version`, you'll now see:
```
Edge MCP v0.0.9-1-gca935e4e-dirty (commit: ca935e4e, built: 2025-10-30_13:35:50)
```

- `0.0.9-1-gca935e4e-dirty`: Git describe format
  - `0.0.9`: Last tag
  - `1`: Number of commits since tag
  - `gca935e4e`: Git commit hash prefix
  - `dirty`: Uncommitted changes present
- `commit: ca935e4e`: Short commit hash
- `built: 2025-10-30_13:35:50`: Build timestamp

## Development Builds

For development builds, you have three options:

1. **Use the Makefile:**
   ```bash
   make build-edge-mcp
   ```

2. **Use the build script:**
   ```bash
   ./scripts/build-edge-mcp.sh
   ```

3. **Direct go build (for custom builds):**
   ```bash
   cd apps/edge-mcp
   VERSION=$(git describe --tags --always --dirty)
   COMMIT=$(git rev-parse --short HEAD)
   BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
   go build -ldflags="-X 'main.version=${VERSION}' -X 'main.commit=${COMMIT}' -X 'main.buildTime=${BUILD_TIME}'" -o edge-mcp ./cmd/server
   ```

## Benefits
1. **Version tracking**: You can now see exactly which version/commit of Edge MCP is running
2. **Development clarity**: The "dirty" flag shows when running with uncommitted changes
3. **Build reproducibility**: Build time helps track when a binary was created
4. **Update readiness**: Proper version info is essential for the self-update mechanism
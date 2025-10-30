# Edge MCP Auto-Update: Quick Start Guide

## Current Status
✅ **Version injection fixed** - Binaries now properly show version/commit/build time
⏳ **Auto-update planned** - Comprehensive implementation plan created
🚀 **Ready to implement** - All prerequisites met

## Immediate Next Steps (Priority Order)

### 1. Create Package Structure (30 mins)
```bash
# Create updater package
mkdir -p pkg/updater
touch pkg/updater/{updater.go,version.go,channel.go,devmode.go,config.go}
touch pkg/updater/{github.go,platform.go,download.go,rollback.go}
touch pkg/updater/{replace_unix.go,replace_windows.go,replace_darwin.go}
touch pkg/updater/{updater_test.go,version_test.go}
```

### 2. Implement Version Comparison (1 hour)
Start with `pkg/updater/version.go`:
- Parse semantic versions
- Compare versions correctly
- Handle pre-releases and dev builds

### 3. Add Dev Mode Detection (30 mins)
Implement `pkg/updater/devmode.go`:
- Detect dirty builds
- Check environment variables
- Verify installation paths

### 4. Create GitHub Client (2 hours)
Build `pkg/updater/github.go`:
- Connect to GitHub Releases API
- Parse release information
- Select appropriate assets

### 5. Test Manually (1 hour)
```bash
# Build with version
make build-edge-mcp

# Test version detection
./bin/edge-mcp --version

# Set dev mode to prevent updates during development
export EDGE_MCP_DEV_MODE=true

# Test with different channels
export EDGE_MCP_UPDATE_CHANNEL=beta
```

## Quick Implementation Example

### Minimal Working Updater
```go
// pkg/updater/updater.go
package updater

import (
    "context"
    "fmt"
    "os"
)

type Updater struct {
    currentVersion string
    channel        UpdateChannel
    disabled       bool
}

func New(version string) *Updater {
    return &Updater{
        currentVersion: version,
        channel:        ChannelStable,
        disabled:       IsDevBuild(),
    }
}

func IsDevBuild() bool {
    // Quick implementation
    exe, _ := os.Executable()
    return os.Getenv("EDGE_MCP_DEV_MODE") == "true" ||
           strings.Contains(exe, "/bin/edge-mcp") || // Local build
           strings.Contains(version, "dirty")
}
```

### Integration in main.go
```go
// apps/edge-mcp/cmd/server/main.go
import "github.com/developer-mesh/developer-mesh/pkg/updater"

func main() {
    // After flag parsing...

    // Initialize updater (only if not dev mode)
    if !updater.IsDevBuild() {
        upd := updater.New(version)

        // Check for updates in background
        go func() {
            time.Sleep(30 * time.Second) // Don't block startup
            if release, err := upd.CheckForUpdate(context.Background()); err == nil && release != nil {
                logger.Info("Update available", map[string]interface{}{
                    "current": version,
                    "latest":  release.Version,
                })
            }
        }()
    }
}
```

## Configuration to Add

### configs/config.yaml
```yaml
# Add to existing config
updater:
  enabled: true
  channel: stable
  checkInterval: 24h
  autoUpdate: false
  githubRepo: developer-mesh/developer-mesh
```

### Environment Variables
```bash
# For development
export EDGE_MCP_DEV_MODE=true           # Disable all updates
export EDGE_MCP_UPDATE_DISABLE=true     # Alternative way to disable

# For testing updates
export EDGE_MCP_UPDATE_CHANNEL=beta     # Use beta channel
export EDGE_MCP_UPDATE_CHECK_NOW=true   # Force immediate check
```

## Testing Checklist

### Basic Functionality
- [ ] Version comparison works correctly
- [ ] Dev builds don't attempt updates
- [ ] GitHub API returns releases
- [ ] Correct asset selected for platform

### Update Process
- [ ] Download completes successfully
- [ ] Checksum verification works
- [ ] Binary replacement succeeds
- [ ] Permissions preserved
- [ ] Rollback works

### Edge Cases
- [ ] No network connection handled
- [ ] GitHub rate limiting handled
- [ ] Corrupt download detected
- [ ] Insufficient permissions handled
- [ ] Running process handled

## Development Tips

### 1. Use Test Mode
```go
// Add test mode to updater
type Updater struct {
    testMode bool  // Skip actual replacement
}

func (u *Updater) ApplyUpdate(release *Release) error {
    if u.testMode {
        log.Printf("TEST MODE: Would update from %s to %s", u.currentVersion, release.Version)
        return nil
    }
    // Actual update logic
}
```

### 2. Local Testing
```bash
# Create fake release locally
mkdir -p /tmp/edge-mcp-releases
cp bin/edge-mcp /tmp/edge-mcp-releases/edge-mcp-v2.0.0

# Test update process without GitHub
EDGE_MCP_UPDATE_URL=file:///tmp/edge-mcp-releases ./bin/edge-mcp update check
```

### 3. Debugging
```bash
# Enable debug logging
export EDGE_MCP_LOG_LEVEL=debug

# Force update check
./bin/edge-mcp update check --verbose

# Dry run (when implemented)
./bin/edge-mcp update apply --dry-run
```

## Common Pitfalls to Avoid

1. **Don't update in Docker** - Check for `/.dockerenv` file
2. **Don't update dev builds** - Multiple detection methods
3. **Don't corrupt running binary** - Use atomic operations
4. **Don't lose permissions** - Preserve file mode
5. **Don't block startup** - Update checks in background

## Resources

- [GitHub Releases API](https://docs.github.com/en/rest/releases)
- [Go version comparison](https://github.com/hashicorp/go-version)
- [Atomic file operations](https://github.com/google/renameio)
- Similar implementations:
  - [GitHub CLI updater](https://github.com/cli/cli/tree/trunk/pkg/cmd/upgrade)
  - [Terraform self-update](https://github.com/hashicorp/terraform)
  - [Docker CLI updates](https://github.com/docker/cli)

## Get Help

If you run into issues:
1. Check `docs/edge-mcp-auto-update-plan.md` for detailed design
2. Review the test cases in `pkg/updater/*_test.go`
3. Enable debug logging with `EDGE_MCP_LOG_LEVEL=debug`
4. Ask in the developer chat/issues
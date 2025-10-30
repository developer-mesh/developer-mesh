# Edge MCP Auto-Update Guide

Edge MCP includes a built-in auto-update system that keeps your installation up-to-date with the latest features, bug fixes, and security patches.

## Overview

The auto-update system:
- ✅ **Checks automatically** - Runs in the background at configurable intervals
- ✅ **Downloads updates** - Optionally downloads new versions automatically
- ✅ **Safe by default** - Requires manual restart to apply updates (unless configured otherwise)
- ✅ **Channel support** - Choose between stable, beta, or latest releases
- ✅ **Manual control** - Check for updates on-demand with `--check-update` flag
- ✅ **Fully configurable** - Control all behavior via environment variables or config file

## Quick Start

### Check for Updates Manually

```bash
edge-mcp --check-update
```

**Example Output:**
```
Edge MCP v0.0.9 - Checking for updates...

Current version: 0.0.9
Latest version:  0.0.10

✅ Update available!

To update:
  1. Download from: https://github.com/developer-mesh/developer-mesh/releases/tag/0.0.10
  2. Or enable auto-update: EDGE_MCP_UPDATE_AUTO_DOWNLOAD=true
```

### Enable Auto-Download

```bash
# Auto-download updates (but don't auto-apply)
export EDGE_MCP_UPDATE_AUTO_DOWNLOAD=true
edge-mcp
```

When an update is available, Edge MCP will download it automatically. You'll see a message:
```
[INFO] Update downloaded successfully
[INFO] Update ready to apply. Restart edge-mcp to apply the update.
```

Simply restart Edge MCP to use the new version.

## Configuration

### Environment Variables

All auto-update behavior can be controlled via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `EDGE_MCP_UPDATE_ENABLED` | `true` | Enable/disable auto-update system |
| `EDGE_MCP_UPDATE_CHECK_INTERVAL` | `24h` | How often to check for updates |
| `EDGE_MCP_UPDATE_CHANNEL` | `stable` | Which release channel to follow |
| `EDGE_MCP_UPDATE_AUTO_DOWNLOAD` | `true` | Auto-download new versions |
| `EDGE_MCP_UPDATE_AUTO_APPLY` | `false` | Auto-apply updates (requires restart) |
| `EDGE_MCP_UPDATE_GITHUB_OWNER` | `developer-mesh` | GitHub organization/user |
| `EDGE_MCP_UPDATE_GITHUB_REPO` | `developer-mesh` | GitHub repository name |

### Configuration File

You can also configure auto-update in `config.yaml`:

```yaml
updater:
  enabled: true
  check_interval: 24h
  channel: stable
  auto_download: true
  auto_apply: false
  github_owner: developer-mesh
  github_repo: developer-mesh
```

**Note:** Environment variables override config file settings.

## Update Channels

Edge MCP supports three update channels:

### Stable (Default)

```bash
export EDGE_MCP_UPDATE_CHANNEL=stable
```

- **Recommended for production**
- Only includes official releases (e.g., `0.0.9`, `1.0.0`)
- Excludes pre-release versions (beta, rc, alpha)
- Maximum stability and reliability

### Beta

```bash
export EDGE_MCP_UPDATE_CHANNEL=beta
```

- **For testing upcoming features**
- Includes beta and release candidate versions (e.g., `1.0.0-beta.1`, `1.0.0-rc.2`)
- More frequent updates than stable
- Generally stable but may have rough edges

### Latest

```bash
export EDGE_MCP_UPDATE_CHANNEL=latest
```

- **For development and testing**
- Includes all releases, even nightly builds
- Most frequent updates
- May include experimental features

## Common Scenarios

### Scenario 1: Production Deployment

**Recommended Settings:**
```bash
# Keep auto-update enabled but conservative
export EDGE_MCP_UPDATE_ENABLED=true
export EDGE_MCP_UPDATE_CHANNEL=stable
export EDGE_MCP_UPDATE_AUTO_DOWNLOAD=true
export EDGE_MCP_UPDATE_AUTO_APPLY=false  # Manual restart for control
export EDGE_MCP_UPDATE_CHECK_INTERVAL=24h
```

**Why:**
- Automatic checking keeps you informed
- Auto-download prepares updates in advance
- Manual restart gives you control over update timing
- 24-hour interval balances freshness with stability

### Scenario 2: Development Environment

**Recommended Settings:**
```bash
# Use latest features for development
export EDGE_MCP_UPDATE_CHANNEL=latest
export EDGE_MCP_UPDATE_AUTO_DOWNLOAD=true
export EDGE_MCP_UPDATE_CHECK_INTERVAL=12h
```

**Why:**
- Latest channel gives you newest features
- More frequent checks (12h) keep you current
- Still requires manual restart for safety

### Scenario 3: Air-Gapped/Offline Environment

**Recommended Settings:**
```bash
# Disable auto-update completely
export EDGE_MCP_UPDATE_ENABLED=false
edge-mcp
```

Or use the command-line flag:
```bash
edge-mcp --disable-auto-update
```

**Why:**
- No network calls to GitHub
- Full control over updates via manual installation
- Suitable for environments without internet access

### Scenario 4: CI/CD Pipeline

**Recommended Settings:**
```bash
# Disable auto-update in CI/CD
export EDGE_MCP_UPDATE_ENABLED=false
# Or set ENVIRONMENT=development (auto-detected)
export ENVIRONMENT=development
```

**Why:**
- CI/CD should use pinned versions
- Reproducible builds require version stability
- Development mode auto-disables updates

## Disabling Auto-Update

There are three ways to disable auto-update:

### 1. Environment Variable (Recommended)

```bash
export EDGE_MCP_UPDATE_ENABLED=false
edge-mcp
```

### 2. Command-Line Flag

```bash
edge-mcp --disable-auto-update
```

### 3. Development Mode Detection

Auto-update is automatically disabled when Edge MCP detects development mode:

```bash
# Any of these disable auto-update
export ENVIRONMENT=development
export ENVIRONMENT=dev
export APP_ENV=development
export APP_ENV=dev
```

## Understanding Update Checks

### When Updates Are Checked

1. **On Startup** - Initial check 30 seconds after Edge MCP starts
2. **Periodic Checks** - Based on `CHECK_INTERVAL` (default: every 24 hours)
3. **Manual Checks** - When you run `edge-mcp --check-update`

### What Happens During a Check

```
1. Edge MCP queries GitHub Releases API
   ↓
2. Finds latest release matching your channel
   ↓
3. Compares with your current version
   ↓
4. If newer version exists:
   a. Logs "Update available"
   b. (If auto-download enabled) Downloads the binary
   c. (If auto-apply enabled) Prepares for application
   ↓
5. Waits for restart to apply update
```

### Log Messages

**Update available:**
```
[INFO] Update available current_version=0.0.9 latest_version=0.0.10
```

**Auto-downloading:**
```
[INFO] Downloading update version=0.0.10
[INFO] Update downloaded successfully size_bytes=15728640
```

**Ready to apply:**
```
[INFO] Update ready to apply. Restart edge-mcp to apply the update.
```

**Already up-to-date:**
```
[DEBUG] No update available current_version=0.0.10 latest_version=0.0.10
```

## Manual Update Process

If you prefer complete manual control:

### 1. Check Current Version

```bash
edge-mcp --version
```

Output: `Edge MCP v0.0.9 (commit: a1b2c3d, built: 2025-10-30_15:30:00)`

### 2. Check for Available Updates

```bash
edge-mcp --check-update
```

### 3. Download New Version

Visit the GitHub release page and download the binary for your platform:

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/developer-mesh/developer-mesh/releases/download/0.0.10/edge-mcp-darwin-arm64.tar.gz | tar xz
chmod +x edge-mcp-darwin-arm64
sudo mv edge-mcp-darwin-arm64 /usr/local/bin/edge-mcp
```

**macOS (Intel):**
```bash
curl -L https://github.com/developer-mesh/developer-mesh/releases/download/0.0.10/edge-mcp-darwin-amd64.tar.gz | tar xz
chmod +x edge-mcp-darwin-amd64
sudo mv edge-mcp-darwin-amd64 /usr/local/bin/edge-mcp
```

**Linux (x64):**
```bash
curl -L https://github.com/developer-mesh/developer-mesh/releases/download/0.0.10/edge-mcp-linux-amd64.tar.gz | tar xz
chmod +x edge-mcp-linux-amd64
sudo mv edge-mcp-linux-amd64 /usr/local/bin/edge-mcp
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri "https://github.com/developer-mesh/developer-mesh/releases/download/0.0.10/edge-mcp-windows-amd64.exe.zip" -OutFile "edge-mcp.zip"
Expand-Archive -Path "edge-mcp.zip" -DestinationPath .
Move-Item edge-mcp-windows-amd64.exe "C:\Program Files\edge-mcp\edge-mcp.exe" -Force
```

### 4. Verify New Version

```bash
edge-mcp --version
```

## Security Considerations

### Checksum Verification

All releases include SHA-256 checksums. To verify your download:

```bash
# Download checksums file
curl -L https://github.com/developer-mesh/developer-mesh/releases/download/0.0.10/checksums.txt -o checksums.txt

# Verify your binary
sha256sum -c checksums.txt --ignore-missing
```

Expected output: `edge-mcp-darwin-arm64.tar.gz: OK`

### Auto-Apply Safety

**Why `AUTO_APPLY` is disabled by default:**
- Gives you control over when updates happen
- Allows testing in staging before production
- Prevents unexpected downtime
- Lets you review changelogs first

**When to enable `AUTO_APPLY`:**
- Development environments
- Non-critical deployments
- When using orchestration that handles restarts
- With monitoring for failed restarts

## Troubleshooting

### "Auto-update disabled" message

**Cause:** Edge MCP detected development mode or explicit disable

**Check:**
```bash
# Check environment variables
env | grep -E "EDGE_MCP_UPDATE|ENVIRONMENT|APP_ENV"

# Check if running with --disable-auto-update flag
ps aux | grep edge-mcp
```

**Fix:**
```bash
# Explicitly enable
export EDGE_MCP_UPDATE_ENABLED=true
unset ENVIRONMENT
unset APP_ENV
```

### Update check fails with network error

**Cause:** Cannot reach GitHub API

**Check:**
```bash
# Test GitHub API access
curl -s https://api.github.com/repos/developer-mesh/developer-mesh/releases/latest | jq .tag_name
```

**Common Issues:**
- Corporate firewall blocking GitHub
- No internet connection
- GitHub API rate limit exceeded (60/hour unauthenticated)
- DNS resolution issues

**Fix:**
- Use manual update process
- Disable auto-update with `EDGE_MCP_UPDATE_ENABLED=false`
- Check proxy settings

### "Download timeout" or "Apply timeout" errors

**Cause:** Timeouts set too low for slow connections

**Fix:** These are controlled by the updater's internal defaults. If you experience timeouts:
1. Check your internet connection speed
2. Try manual download instead
3. Report the issue on GitHub if persistent

### Version shows "dirty" or incorrect version

**Cause:** Built from source with uncommitted changes

**Check:**
```bash
git status  # Should be clean
git describe --tags --always --dirty
```

**Fix:**
```bash
# Commit or stash changes
git stash

# Rebuild
make build-edge-mcp

# Check version
./bin/edge-mcp --version
```

## Best Practices

### ✅ Do

- **Enable auto-update in production** with `AUTO_APPLY=false`
- **Use stable channel** for production deployments
- **Monitor update logs** to stay informed
- **Test updates in staging** before enabling `AUTO_APPLY`
- **Check changelogs** before updating critical systems
- **Verify checksums** for manual downloads

### ❌ Don't

- **Don't enable `AUTO_APPLY` in production** without testing
- **Don't use latest channel** in production
- **Don't ignore security updates** - keep updated
- **Don't disable updates** without a good reason
- **Don't skip version verification** for manual installs

## Monitoring

### Check Auto-Update Status

View current configuration and last check time:

```bash
# Run with debug logging
edge-mcp --log-level debug

# Look for these log messages:
# [INFO] Background update checker initialized
# [DEBUG] Checking for updates
# [INFO] No update available / Update available
```

### Metrics to Monitor

If you have logging/monitoring setup:

- **Update check frequency** - Should match `CHECK_INTERVAL`
- **Update check failures** - Network or API issues
- **Available updates detected** - New versions found
- **Download successes/failures** - Auto-download working
- **Version changes** - Track when updates applied

## FAQ

### Q: How do I know if auto-update is enabled?

**A:** Run with debug logging:
```bash
edge-mcp --log-level debug
```

Look for: `[INFO] Background update checker initialized`

### Q: Will auto-update restart Edge MCP automatically?

**A:** No, unless you explicitly enable `EDGE_MCP_UPDATE_AUTO_APPLY=true`. By default, you must manually restart.

### Q: Can I schedule updates for specific times?

**A:** Not directly. Auto-update checks periodically, but you control when restarts happen. For scheduled updates:
1. Enable auto-download
2. Use your orchestration tool (systemd, supervisor, etc.) to schedule restarts

### Q: What happens if a download fails?

**A:** Edge MCP logs the error and continues running. It will retry on the next check interval.

### Q: Does auto-update work in air-gapped environments?

**A:** No, auto-update requires internet access to GitHub. For air-gapped environments:
```bash
export EDGE_MCP_UPDATE_ENABLED=false
```

### Q: Can I use a private GitHub repository?

**A:** Yes, configure your repository:
```bash
export EDGE_MCP_UPDATE_GITHUB_OWNER=your-org
export EDGE_MCP_UPDATE_GITHUB_REPO=your-private-repo
```

Note: You may need to configure GitHub authentication for private repos.

### Q: How much bandwidth does auto-update use?

**A:**
- **Update checks**: ~1-2 KB per check (JSON API call)
- **Downloads**: ~15-30 MB per update (varies by platform)
- **With default settings**: Minimal impact (1 check/day)

## Support

### Getting Help

- **Documentation**: Check this guide and related docs
- **GitHub Issues**: Report bugs at https://github.com/developer-mesh/developer-mesh/issues
- **Logs**: Run with `--log-level debug` for detailed information

### Related Documentation

- [Edge MCP README](../../apps/edge-mcp/README.md) - Main documentation
- [Installation Guide](../getting-started/installation.md) - Initial setup
- [Configuration Reference](../reference/configuration.md) - All config options
- [Troubleshooting Guide](../troubleshooting/edge-mcp.md) - Common issues

## Version History

- **0.0.9** - Initial auto-update implementation
  - Background update checking
  - Auto-download support
  - Manual update checking
  - Multi-channel support (stable, beta, latest)

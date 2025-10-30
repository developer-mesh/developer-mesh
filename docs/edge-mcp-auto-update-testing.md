# Edge MCP Auto-Update Testing Guide

This guide provides step-by-step instructions to test the Edge MCP auto-update implementation (Phase 4).

## Prerequisites

- Go 1.24+ installed
- Git repository with commits and tags
- GitHub access for release testing
- Edge MCP source code

## Testing Workflow

### 1. ✅ Unit Tests (Automated)

Run all updater-related unit tests:

```bash
cd apps/edge-mcp

# Test all updater components
go test -v ./internal/config ./internal/updater ./cmd/server -run "TestUpdater|TestBackgroundChecker"

# Or run specific test suites
go test -v ./internal/config -run TestUpdaterConfig
go test -v ./internal/updater -run TestBackgroundChecker
go test -v ./cmd/server -run TestUpdater
```

**Expected Results:**
- All tests pass ✅
- Config parsing tests: 40+ test cases
- Background checker tests: 15+ test cases
- Integration tests: 12+ test cases

---

### 2. Build with Version Injection

Test that version information is properly embedded in the binary.

```bash
# Build using Makefile (recommended)
make build-edge-mcp

# Check version
./bin/edge-mcp --version
```

**Expected Output:**
```
Edge MCP v0.0.9-1-ga16af9ca (commit: a16af9ca, built: 2025-10-30_15:38:45)
```

**Version Format Explained:**
- `0.0.9-1-ga16af9ca`: Git describe output
  - `0.0.9`: Last Git tag
  - `1`: Number of commits since tag
  - `ga16af9ca`: Git commit hash
- `commit: a16af9ca`: Short commit hash
- `built: 2025-10-30_15:38:45`: Build timestamp (UTC)

**Test Cases:**
```bash
# 1. Clean build (no uncommitted changes)
git status  # Should be clean
make build-edge-mcp
./bin/edge-mcp --version  # Should NOT show "-dirty"

# 2. Dirty build (with uncommitted changes)
echo "test" >> test.txt
make build-edge-mcp
./bin/edge-mcp --version  # Should show "-dirty"
git checkout test.txt  # Clean up

# 3. Tagged version
git tag 0.1.0-test
make build-edge-mcp
./bin/edge-mcp --version  # Should show 0.1.0-test
git tag -d 0.1.0-test  # Clean up
```

---

### 3. CLI Flags Testing

Test command-line flags for manual update control.

#### 3.1 Manual Update Check

```bash
# Build the binary first
make build-edge-mcp

# Test manual update check (requires GitHub access)
./bin/edge-mcp --check-update
```

**Expected Output:**
```
Checking for updates...
Current version: 0.0.9-1-ga16af9ca
Latest version: 0.0.9
Already running latest version.
```

**Or if update available:**
```
Checking for updates...
Current version: 0.0.8
Latest version: 0.0.9
Update available! Download from: https://github.com/developer-mesh/developer-mesh/releases/tag/0.0.9
```

#### 3.2 Disable Auto-Update Flag

```bash
# Start with auto-update disabled
./bin/edge-mcp --disable-auto-update --log-level debug

# Check logs - should see:
# "Auto-update disabled via command-line flag"
```

#### 3.3 Combined Flags

```bash
# Help text
./bin/edge-mcp --help

# Version + other flags
./bin/edge-mcp --version --log-level info

# Disable updates in production
./bin/edge-mcp --disable-auto-update --port 8082
```

---

### 4. Environment Variable Configuration

Test configuration via environment variables.

```bash
# Test 1: Disable via environment
EDGE_MCP_UPDATE_ENABLED=false ./bin/edge-mcp --log-level debug
# Expected: "Auto-update disabled" in logs

# Test 2: Change check interval
EDGE_MCP_UPDATE_CHECK_INTERVAL=1h ./bin/edge-mcp --log-level debug
# Expected: "check_interval: 1h0m0s" in logs

# Test 3: Change channel
EDGE_MCP_UPDATE_CHANNEL=beta ./bin/edge-mcp --log-level debug
# Expected: "channel: beta" in logs

# Test 4: Enable auto-apply (dangerous!)
EDGE_MCP_UPDATE_AUTO_APPLY=true ./bin/edge-mcp --log-level debug
# Expected: "auto_apply: true" in logs

# Test 5: Custom GitHub repo
EDGE_MCP_UPDATE_GITHUB_OWNER=myorg \
EDGE_MCP_UPDATE_GITHUB_REPO=myrepo \
./bin/edge-mcp --log-level debug
# Expected: "repo: myorg/myrepo" in logs

# Test 6: All overrides
EDGE_MCP_UPDATE_ENABLED=true \
EDGE_MCP_UPDATE_CHECK_INTERVAL=30m \
EDGE_MCP_UPDATE_CHANNEL=latest \
EDGE_MCP_UPDATE_AUTO_DOWNLOAD=true \
EDGE_MCP_UPDATE_AUTO_APPLY=false \
./bin/edge-mcp --log-level debug
```

**Environment Variable Reference:**
| Variable | Default | Options |
|----------|---------|---------|
| `EDGE_MCP_UPDATE_ENABLED` | `true` | `true`, `false`, `yes`, `no`, `1`, `0` |
| `EDGE_MCP_UPDATE_CHECK_INTERVAL` | `24h` | Duration: `1h`, `30m`, `24h` |
| `EDGE_MCP_UPDATE_CHANNEL` | `stable` | `stable`, `beta`, `latest` |
| `EDGE_MCP_UPDATE_AUTO_DOWNLOAD` | `true` | Same as ENABLED |
| `EDGE_MCP_UPDATE_AUTO_APPLY` | `false` | Same as ENABLED |
| `EDGE_MCP_UPDATE_GITHUB_OWNER` | `developer-mesh` | Any GitHub org/user |
| `EDGE_MCP_UPDATE_GITHUB_REPO` | `developer-mesh` | Any GitHub repo |

---

### 5. Background Checker Lifecycle

Test the background update checker service.

#### 5.1 Start/Stop Testing

```bash
# Terminal 1: Run edge-mcp with short check interval
EDGE_MCP_UPDATE_CHECK_INTERVAL=30s ./bin/edge-mcp --log-level debug

# Expected logs (within 30 seconds):
# [INFO] Background update checker initialized
# [INFO] Background update checker started
# [DEBUG] Checking for updates
# [INFO] No update available (or Update available)

# Press Ctrl+C to stop

# Expected shutdown logs:
# [INFO] Stopping background update checker
# [INFO] Background checker stopped
```

#### 5.2 Development Mode Detection

```bash
# Should auto-disable in development mode
ENVIRONMENT=development ./bin/edge-mcp --log-level debug
# Expected: "Development mode detected, auto-update disabled"

# Also check with APP_ENV
APP_ENV=dev ./bin/edge-mcp --log-level debug
# Expected: "Development mode detected, auto-update disabled"

# Production mode (enables auto-update)
ENVIRONMENT=production ./bin/edge-mcp --log-level debug
# Expected: "Background update checker initialized"
```

---

### 6. GitHub Integration Testing

Test actual communication with GitHub releases API.

#### 6.1 Check for Real Updates

```bash
# Prerequisites: Internet connection + GitHub API access

# Run manual check
./bin/edge-mcp --check-update

# Expected behavior:
# 1. Connects to GitHub API
# 2. Fetches latest release from developer-mesh/developer-mesh
# 3. Compares with current version
# 4. Reports if update is available
```

#### 6.2 Test Different Channels

```bash
# Stable channel (default - only tags without pre-release markers)
EDGE_MCP_UPDATE_CHANNEL=stable ./bin/edge-mcp --check-update

# Beta channel (includes beta releases like 1.0.0-beta.1)
EDGE_MCP_UPDATE_CHANNEL=beta ./bin/edge-mcp --check-update

# Latest channel (any release including nightlies)
EDGE_MCP_UPDATE_CHANNEL=latest ./bin/edge-mcp --check-update
```

#### 6.3 Test Auto-Download (Safe)

```bash
# Enable auto-download but NOT auto-apply
EDGE_MCP_UPDATE_ENABLED=true \
EDGE_MCP_UPDATE_AUTO_DOWNLOAD=true \
EDGE_MCP_UPDATE_AUTO_APPLY=false \
EDGE_MCP_UPDATE_CHECK_INTERVAL=1m \
./bin/edge-mcp --log-level debug

# If update is available, watch logs for:
# [INFO] Update available
# [INFO] Downloading update version=X.X.X
# [INFO] Update downloaded successfully
# [INFO] Update ready to apply. Restart edge-mcp to apply the update.

# The binary won't auto-restart (safe)
```

#### 6.4 Test Rate Limiting

```bash
# Run multiple checks quickly
for i in {1..5}; do
  ./bin/edge-mcp --check-update
  sleep 2
done

# Expected: All checks should succeed (GitHub allows ~60/hour for unauthenticated)
# If rate-limited: Error message about rate limit
```

---

### 7. Integration Testing Scenarios

#### Scenario 1: Fresh Install Simulation

```bash
# Simulate a user installing edge-mcp for the first time
rm -rf ~/.edge-mcp  # Clean any cached data
./bin/edge-mcp --log-level debug

# Expected:
# 1. Starts successfully
# 2. Background checker initializes
# 3. Performs initial check after 30 seconds
# 4. Reports current version status
```

#### Scenario 2: Upgrade Simulation

```bash
# 1. Build an older version (if you have an old tag)
git checkout 0.0.8
make build-edge-mcp
mv bin/edge-mcp bin/edge-mcp-old

# 2. Return to current version
git checkout -

# 3. Run old version
./bin/edge-mcp-old --check-update

# Expected: Shows update available to current version
```

#### Scenario 3: Offline Mode

```bash
# Disconnect from internet or block GitHub
# Then run:
./bin/edge-mcp --check-update

# Expected: Error message about connection failure
# Should NOT crash or panic
```

#### Scenario 4: Invalid Configuration

```bash
# Test with invalid duration
EDGE_MCP_UPDATE_CHECK_INTERVAL=invalid ./bin/edge-mcp --log-level debug
# Expected: Falls back to default (24h)

# Test with invalid boolean
EDGE_MCP_UPDATE_ENABLED=maybe ./bin/edge-mcp --log-level debug
# Expected: Falls back to default (true)
```

---

### 8. Production Readiness Checklist

Before deploying to production, verify:

- [ ] Unit tests pass: `make test`
- [ ] Build succeeds: `make build-edge-mcp`
- [ ] Version is correct: `./bin/edge-mcp --version`
- [ ] Manual update check works: `./bin/edge-mcp --check-update`
- [ ] Background checker starts and stops cleanly
- [ ] Environment variables override defaults correctly
- [ ] Development mode disables auto-update
- [ ] GitHub API integration works
- [ ] Rate limiting is handled gracefully
- [ ] Errors don't cause crashes

---

### 9. Monitoring in Production

Once deployed, monitor these aspects:

```bash
# Check updater status via logs
journalctl -u edge-mcp -f | grep -i update

# Expected periodic logs:
# [DEBUG] Checking for updates
# [INFO] No update available

# When update is available:
# [INFO] Update available current_version=X.X.X latest_version=X.X.X
# [INFO] Downloading update (if auto-download enabled)
# [INFO] Update ready to apply
```

**Metrics to Monitor:**
- Update check frequency (should match check_interval)
- Update check failures (network issues, API errors)
- Available updates detected
- Updates downloaded successfully
- Update application success/failure

---

### 10. Troubleshooting

#### Problem: Version shows "1.0.0" instead of git version

**Solution:**
```bash
# Verify git describe works
git describe --tags --always --dirty

# Rebuild with Makefile (includes ldflags)
make build-edge-mcp

# Don't use plain `go build` - it won't inject version
```

#### Problem: "Auto-update disabled" when it should be enabled

**Check:**
```bash
# 1. Environment variable overriding config?
env | grep EDGE_MCP

# 2. Development mode detected?
echo $ENVIRONMENT
echo $APP_ENV

# 3. Command-line flag set?
ps aux | grep edge-mcp | grep disable-auto-update
```

#### Problem: Update check fails with network error

**Debug:**
```bash
# Test GitHub API manually
curl -s https://api.github.com/repos/developer-mesh/developer-mesh/releases/latest | jq .tag_name

# Check rate limit
curl -s https://api.github.com/rate_limit | jq .rate

# Run with debug logging
./bin/edge-mcp --check-update --log-level debug
```

#### Problem: Background checker doesn't run

**Check:**
```bash
# 1. Is it enabled?
EDGE_MCP_UPDATE_ENABLED=true ./bin/edge-mcp --log-level debug

# 2. Check interval too long?
EDGE_MCP_UPDATE_CHECK_INTERVAL=1m ./bin/edge-mcp --log-level debug

# 3. Look for initialization logs
# Should see: "Background update checker initialized"
```

---

## Quick Test Script

Save this as `test-auto-update.sh`:

```bash
#!/bin/bash
set -e

echo "=========================================="
echo "Edge MCP Auto-Update Testing Script"
echo "=========================================="

# 1. Build
echo ""
echo "1. Building edge-mcp..."
make build-edge-mcp

# 2. Version check
echo ""
echo "2. Checking version..."
./bin/edge-mcp --version

# 3. Manual update check
echo ""
echo "3. Running manual update check..."
timeout 10s ./bin/edge-mcp --check-update || true

# 4. Test with disabled auto-update
echo ""
echo "4. Testing with auto-update disabled..."
timeout 5s ./bin/edge-mcp --disable-auto-update --log-level debug || true

# 5. Test environment overrides
echo ""
echo "5. Testing environment variable overrides..."
EDGE_MCP_UPDATE_CHANNEL=beta \
EDGE_MCP_UPDATE_CHECK_INTERVAL=30s \
timeout 5s ./bin/edge-mcp --log-level debug || true

echo ""
echo "=========================================="
echo "All tests completed!"
echo "=========================================="
```

Run it:
```bash
chmod +x test-auto-update.sh
./test-auto-update.sh
```

---

## Summary

**Phase 4 auto-update testing covers:**
1. ✅ Unit tests (40+ test cases)
2. ✅ Version injection verification
3. ✅ CLI flags functionality
4. ✅ Environment variable configuration
5. ✅ Background checker lifecycle
6. ✅ GitHub API integration
7. ✅ Production readiness

**Next Steps:**
- Deploy to staging environment
- Monitor for 24-48 hours
- Verify update checks occur as scheduled
- Test actual update application (requires new release)
- Deploy to production

For questions or issues, see:
- `docs/edge-mcp-auto-update-revised-plan.md` - Original implementation plan
- `docs/edge-mcp-build-fix.md` - Version injection details
- `apps/edge-mcp/internal/updater/` - Implementation code

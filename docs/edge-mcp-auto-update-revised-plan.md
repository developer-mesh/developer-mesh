# Edge MCP Auto-Update: REVISED Implementation Plan
## Using Existing Project Infrastructure

## Key Principle: Reuse Before Creating
Following the project's core philosophy: **Always use existing packages and extend them before creating new ones**

## What We'll Reuse vs What's New

### ✅ Reuse Existing
1. **HTTP Client** - Use `pkg/clients/rest_api_client.go`
2. **Circuit Breaker** - Use `pkg/resilience/circuit_breaker.go`
3. **GitHub API** - Extend `pkg/tools/providers/github/`
4. **Retry Logic** - Use `pkg/utils/retry.go`
5. **Observability** - Use `pkg/observability/`
6. **Security** - Use `pkg/security/` (extend for checksums)

### ⭐ New (Minimal)
1. **Version Comparison** - Already created in `pkg/updater/version.go` ✅
2. **Update Orchestration** - New in `pkg/updater/updater.go`
3. **Checksum Verification** - Add to `pkg/security/checksum.go`
4. **Binary Replacement** - New in `pkg/updater/replace.go`

## Revised Architecture

### 1. Extend pkg/security for Checksums
```go
// pkg/security/checksum.go
package security

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "os"
)

// VerifyFileChecksum verifies a file matches expected SHA256
func VerifyFileChecksum(filepath, expectedSHA256 string) error {
    file, err := os.Open(filepath)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer file.Close()

    h := sha256.New()
    if _, err := io.Copy(h, file); err != nil {
        return fmt.Errorf("failed to compute checksum: %w", err)
    }

    computed := hex.EncodeToString(h.Sum(nil))
    if computed != expectedSHA256 {
        return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, computed)
    }

    return nil
}
```

### 2. Extend GitHub Provider for Releases
```go
// pkg/tools/providers/github/release_downloader.go
package github

import (
    "context"
    "github.com/google/go-github/v74/github"
)

// ReleaseDownloader handles GitHub release operations
type ReleaseDownloader struct {
    client *github.Client
    logger observability.Logger
}

func (r *ReleaseDownloader) GetLatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, error) {
    // Use existing GitHub client
}

func (r *ReleaseDownloader) DownloadAsset(ctx context.Context, owner, repo string, assetID int64) ([]byte, error) {
    // Reuse download logic from handlers_actions_extended.go
}
```

### 3. Create Minimal Updater Package
```go
// pkg/updater/updater.go
package updater

import (
    "github.com/developer-mesh/developer-mesh/pkg/clients"
    "github.com/developer-mesh/developer-mesh/pkg/resilience"
    "github.com/developer-mesh/developer-mesh/pkg/security"
    "github.com/developer-mesh/developer-mesh/pkg/tools/providers/github"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/developer-mesh/developer-mesh/pkg/utils"
)

type Updater struct {
    // Reuse existing components
    githubClient   *github.ReleaseDownloader
    circuitBreaker *resilience.CircuitBreaker
    logger         observability.Logger
    retrier        *utils.Retrier

    // Update-specific
    currentVersion *Version
    channel        UpdateChannel
}
```

### 4. Use Existing REST Client Pattern
```go
// Reuse the pattern from pkg/clients/rest_api_client.go
func (u *Updater) CheckForUpdate(ctx context.Context) (*Release, error) {
    // Use circuit breaker from resilience package
    return u.circuitBreaker.Execute(func() (interface{}, error) {
        // Use GitHub client with retry
        return u.retrier.RetryWithBackoff(func() error {
            return u.githubClient.GetLatestRelease(ctx, "developer-mesh", "developer-mesh")
        })
    })
}
```

## Implementation Steps (Revised)

### Phase 1: Extend Existing Packages (Week 1)
- [x] Version comparison in `pkg/updater/version.go`
- [ ] Add checksum verification to `pkg/security/checksum.go`
- [ ] Extend GitHub provider with release methods
- [ ] Add dev mode detection using existing patterns

### Phase 2: Wire Components Together (Week 2)
- [ ] Create updater orchestrator using existing clients
- [ ] Integrate with existing circuit breaker
- [ ] Use existing retry mechanisms
- [ ] Leverage existing observability

### Phase 3: Binary Management (Week 3)
- [ ] Implement atomic replacement (new code required)
- [ ] Add rollback using existing patterns
- [ ] Platform-specific handling

### Phase 4: Integration (Week 4)
- [ ] Add CLI commands to edge-mcp
- [ ] Use existing config patterns
- [ ] Background checker with existing goroutine patterns

## Key Differences from Original Plan

### ❌ What We're NOT Creating
- ~~New HTTP client~~ → Use `pkg/clients`
- ~~New circuit breaker~~ → Use `pkg/resilience`
- ~~New retry logic~~ → Use `pkg/utils/retry`
- ~~New GitHub client~~ → Extend `pkg/tools/providers/github`
- ~~New logging~~ → Use `pkg/observability`

### ✅ What We ARE Creating (Minimal)
- Version comparison logic (DONE)
- Update orchestration logic
- Binary replacement logic
- Checksum verification (extending security)

## Benefits of This Approach

1. **Consistency** - Uses established patterns in the codebase
2. **Less Code** - Reuses 80% of needed functionality
3. **Better Testing** - Leverages already-tested components
4. **Maintainability** - Follows project conventions
5. **Security** - Uses proven security patterns

## Example: Using Existing Patterns

### Before (Creating New)
```go
// ❌ Don't create new HTTP client
func downloadFile(url string) ([]byte, error) {
    client := &http.Client{Timeout: 30 * time.Second}
    // ... custom implementation
}
```

### After (Reusing Existing)
```go
// ✅ Use existing client with circuit breaker
func (u *Updater) downloadRelease(ctx context.Context, url string) ([]byte, error) {
    return u.circuitBreaker.Execute(func() (interface{}, error) {
        return u.httpClient.Get(ctx, url)
    })
}
```

## Configuration Integration

Use existing config patterns from `pkg/config`:
```yaml
# configs/config.yaml (following existing patterns)
updater:
  enabled: ${EDGE_MCP_UPDATE_ENABLED:true}
  channel: ${EDGE_MCP_UPDATE_CHANNEL:stable}
  checkInterval: ${EDGE_MCP_UPDATE_INTERVAL:24h}
  githubRepo: developer-mesh/developer-mesh
  # Integrate with existing resilience config
  circuitBreaker:
    maxFailures: 3
    timeout: 30s
    resetTimeout: 60s
```

## Testing Strategy (Using Existing Patterns)

1. **Unit Tests** - Follow patterns in `*_test.go` files
2. **Mocks** - Use existing mock patterns
3. **Integration** - Add to `test/functional/`
4. **E2E** - Add to `test/e2e/`

## Migration Path

1. Start with read-only operations (checking for updates)
2. Add download capability using existing clients
3. Implement replacement as final step
4. Each step uses existing infrastructure

## Assumptions We're NOT Making

1. **Not assuming new patterns** - Using existing ones
2. **Not assuming GitHub API structure** - Using go-github library already in project
3. **Not assuming network behavior** - Using existing circuit breakers
4. **Not assuming file permissions** - Checking explicitly
5. **Not assuming platform behavior** - Testing each platform

## Next Immediate Actions

1. ✅ Review existing `pkg/tools/providers/github/` for reusable code
2. ✅ Check `pkg/clients/` patterns for HTTP operations
3. ✅ Understand `pkg/resilience/` circuit breaker usage
4. Add checksum to `pkg/security/`
5. Extend GitHub provider with release methods
6. Wire components in minimal updater package
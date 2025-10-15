# RAG Loader: GitHub Integration Guide

> **Purpose**: Detailed guide for configuring GitHub as a data source for RAG ingestion
> **Audience**: Developers and platform administrators setting up GitHub crawling
> **Prerequisites**: GitHub account with repository access, RAG loader configured
> **Related**: [RAG Loader User Guide](./rag-loader-user-guide.md)

## Overview

The RAG loader can automatically ingest code, documentation, and configuration files from your GitHub repositories. This guide covers everything you need to know about configuring GitHub as a data source.

## Table of Contents

1. [GitHub Authentication Setup](#github-authentication-setup)
2. [Basic Configuration](#basic-configuration)
3. [Advanced Configuration](#advanced-configuration)
4. [Multiple Repositories](#multiple-repositories)
5. [File Filtering](#file-filtering)
6. [Performance Tuning](#performance-tuning)
7. [Troubleshooting](#troubleshooting)
8. [Best Practices](#best-practices)

## GitHub Authentication Setup

### Option 1: Personal Access Token (Classic) - Recommended

**Step 1: Generate Token**

1. Navigate to https://github.com/settings/tokens
2. Click "Generate new token" → "Generate new token (classic)"
3. Configure the token:
   - **Name**: `RAG Loader - Production` (or environment-specific)
   - **Expiration**: 90 days (recommended for security)
   - **Scopes**:
     - ✅ `repo` - Full control of private repositories
     - ✅ `read:org` - Read organization membership (if accessing org repos)

4. Click "Generate token"
5. **IMPORTANT**: Copy the token immediately (starts with `ghp_`)

**Step 2: Store Securely**

```bash
# For local development
export GITHUB_TOKEN=ghp_your_token_here

# For production (Kubernetes)
kubectl create secret generic rag-loader-secrets \
  --namespace=devmesh \
  --from-literal=github-token=ghp_your_token_here
```

### Option 2: GitHub App (Enterprise)

For enterprise environments, consider using a GitHub App:

1. Create GitHub App in your organization
2. Install app on required repositories
3. Generate and download private key
4. Configure RAG loader with app credentials

```yaml
sources:
  - id: org_repos
    type: github
    config:
      auth_type: app
      app_id: 12345
      installation_id: 67890
      private_key_path: /secrets/github-app-key.pem
```

### Option 3: Fine-Grained Personal Access Token (Preview)

For more granular permissions:

1. Navigate to Settings → Developer settings → Personal access tokens → Fine-grained tokens
2. Create new token with specific repository access
3. Set permissions:
   - Repository: Contents (read-only)
   - Repository: Metadata (read-only)

## Configuration Types

The RAG loader supports two types of GitHub sources:

1. **Organization Source** (`github_org`) - Automatically discover and scan all repositories in a GitHub organization
2. **Single Repository** (`github`) - Scan individual repositories

### Choosing the Right Type

| Use Case | Type | Why |
|----------|------|-----|
| Scan entire organization | `github_org` | Automatic discovery, no manual repo list |
| Scan specific repos in org | `github_org` with `repos` list | Filtered automatic discovery |
| Single repository | `github` | Simple, explicit control |
| Different schedules per repo | `github` | Fine-grained scheduling |
| Mix of org and specific repos | Both types | Flexibility |

## Basic Configuration

### Organization-Wide Scanning (Recommended for Multiple Repos)

Automatically scan all repositories in an organization:

```yaml
sources:
  - id: my_company
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: your-organization   # GitHub organization name
      token: ${GITHUB_TOKEN}   # From environment variable
      include_archived: false  # Skip archived repos (default: false)
      include_forks: false     # Skip forked repos (default: false)
      include_patterns:
        - "**/*.go"
        - "**/*.md"
      exclude_patterns:
        - "vendor/**"
        - "**/*_test.go"
```

**Scan specific repos only:**
```yaml
sources:
  - id: my_company_core
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: your-organization
      token: ${GITHUB_TOKEN}
      repos:
        - "backend-api"
        - "frontend-web"
        - "docs"
      include_patterns:
        - "**/*.go"
        - "**/*.md"
```

### Single Repository Scanning

For scanning individual repositories:

```yaml
sources:
  - id: my_repo
    type: github
    enabled: true
    schedule: "0 */6 * * *"
    config:
      owner: your-org          # GitHub organization or username
      repo: your-repo          # Repository name
      branch: main             # Branch to crawl
      token: ${GITHUB_TOKEN}   # From environment variable
```

### What Gets Crawled

By default, the RAG loader will:
- ✅ Crawl all files in the specified branch
- ✅ Respect `.gitignore` patterns
- ✅ Skip binary files automatically
- ✅ Process text-based files (code, markdown, yaml, etc.)

### Testing Your Configuration

```bash
# Start the RAG loader
docker-compose up rag-loader

# Watch logs for GitHub activity
docker-compose logs -f rag-loader | grep -i github

# You should see:
# INFO  Starting GitHub crawl for your-org/your-repo
# INFO  Downloaded file: README.md
# INFO  Downloaded file: src/main.go
# INFO  Completed GitHub crawl: 42 files processed
```

## Advanced Configuration

### Complete Configuration Example

```yaml
sources:
  - id: main_application
    type: github
    enabled: true
    schedule: "0 */4 * * *"  # Every 4 hours

    config:
      # Repository details
      owner: developer-mesh
      repo: developer-mesh
      branch: main

      # Authentication
      token: ${GITHUB_TOKEN}

      # File inclusion patterns (glob syntax)
      include_patterns:
        # Go code
        - "**/*.go"
        - "go.mod"
        - "go.sum"

        # Documentation
        - "**/*.md"
        - "docs/**/*.txt"

        # Configuration
        - "**/*.yaml"
        - "**/*.yml"
        - "**/*.json"
        - "**/*.toml"

        # SQL
        - "**/*.sql"
        - "migrations/**"

        # Scripts
        - "**/*.sh"
        - "scripts/**"

      # File exclusion patterns (takes precedence)
      exclude_patterns:
        # Dependencies
        - "vendor/**"
        - "node_modules/**"
        - "**/.venv/**"

        # Tests (if you don't want them)
        - "**/*_test.go"
        - "**/*.test.js"
        - "**/test_*.py"

        # Generated code
        - "**/*.gen.go"
        - "**/*.pb.go"
        - "**/*.generated.ts"

        # Build artifacts
        - "dist/**"
        - "build/**"
        - "target/**"

        # IDE and system files
        - ".git/**"
        - ".idea/**"
        - ".vscode/**"
        - "**/.DS_Store"

        # Large data files
        - "**/*.csv"
        - "**/*.parquet"
        - "data/**"

      # Performance settings
      max_file_size: 1048576        # 1MB - skip larger files
      concurrent_downloads: 5        # Download 5 files at once
      requests_per_second: 10        # GitHub API rate limit
      timeout: 60s                   # Request timeout

      # Advanced options
      follow_symlinks: false         # Don't follow symlinks
      include_binary_files: false    # Skip binary files
      respect_gitignore: true        # Honor .gitignore patterns
```

## Multiple Repositories

### Pattern 1: Organization-Wide Scanning (Recommended)

Use the `github_org` source type to automatically discover all repositories:

```yaml
sources:
  - id: company_org
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: your-org
      token: ${GITHUB_TOKEN}
      include_archived: false
      include_forks: false
      include_patterns:
        - "**/*.go"
        - "**/*.ts"
        - "**/*.tsx"
        - "**/*.md"
        - "**/*.tf"
        - "**/*.yaml"
        - "**/*.sh"
      exclude_patterns:
        - "vendor/**"
        - "node_modules/**"
        - "**/*_test.*"
```

### Pattern 2: Multiple Organizations

Scan multiple organizations with separate tokens:

```yaml
sources:
  - id: engineering_org
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: company-engineering
      token: ${GITHUB_TOKEN_ENG}
      include_patterns: ["**/*.go", "**/*.md"]

  - id: platform_org
    type: github_org
    enabled: true
    schedule: "30 */6 * * *"  # 30 min offset
    config:
      org: company-platform
      token: ${GITHUB_TOKEN_PLATFORM}
      include_patterns: ["**/*.py", "**/*.md"]
```

See [Multi-Organization Setup Guide](./rag-loader-multi-org-setup.md) for detailed patterns.

### Pattern 3: Individual Repository Sources (Legacy)

For fine-grained control, configure each repository individually:

```yaml
sources:
  # Backend services
  - id: backend_service
    type: github
    enabled: true
    schedule: "0 */4 * * *"
    config:
      owner: your-org
      repo: backend-service
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns: ["**/*.go", "**/*.md"]

  # Frontend application
  - id: frontend_app
    type: github
    enabled: true
    schedule: "0 */6 * * *"
    config:
      owner: your-org
      repo: frontend-app
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns: ["**/*.ts", "**/*.tsx", "**/*.md"]

  # Infrastructure code
  - id: infrastructure
    type: github
    enabled: true
    schedule: "0 0 * * *"  # Once daily
    config:
      owner: your-org
      repo: infrastructure
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns: ["**/*.tf", "**/*.yaml", "**/*.sh"]
```

### Pattern 4: Multiple Branches

For scanning different branches of the same repository:

```yaml
sources:
  # Main branch (production)
  - id: app_main
    type: github
    enabled: true
    schedule: "0 */4 * * *"
    config:
      owner: your-org
      repo: your-app
      branch: main
      token: ${GITHUB_TOKEN}

  # Development branch
  - id: app_develop
    type: github
    enabled: true
    schedule: "0 */12 * * *"  # Less frequent
    config:
      owner: your-org
      repo: your-app
      branch: develop
      token: ${GITHUB_TOKEN}
```

### Pattern 5: Different Schedules for Different Content

```yaml
sources:
  # Critical documentation - frequent updates
  - id: docs_site
    type: github
    enabled: true
    schedule: "0 */2 * * *"  # Every 2 hours
    config:
      owner: your-org
      repo: documentation
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns: ["**/*.md"]

  # Code repository - moderate updates
  - id: main_code
    type: github
    enabled: true
    schedule: "0 */6 * * *"  # Every 6 hours
    config:
      owner: your-org
      repo: application
      branch: main
      token: ${GITHUB_TOKEN}

  # Archive/reference repository - infrequent
  - id: archive
    type: github
    enabled: true
    schedule: "0 0 * * 0"  # Weekly on Sunday
    config:
      owner: your-org
      repo: archived-projects
      branch: main
      token: ${GITHUB_TOKEN}
```

## File Filtering

### Include Patterns

Use glob patterns to specify which files to crawl:

```yaml
include_patterns:
  # Exact file names
  - "README.md"
  - "LICENSE"

  # File extensions (anywhere in tree)
  - "**/*.go"
  - "**/*.md"

  # Specific directories
  - "docs/**/*.md"
  - "src/**/*.ts"
  - "pkg/**/*.go"

  # Multiple extensions
  - "**/*.{js,ts,jsx,tsx}"

  # Complex patterns
  - "**/api/**/*.yaml"
  - "configs/*.{yaml,yml,json}"
```

### Exclude Patterns (Higher Priority)

Exclude patterns override include patterns:

```yaml
exclude_patterns:
  # Always exclude
  - "vendor/**"
  - "node_modules/**"
  - ".git/**"

  # Test files
  - "**/*_test.go"
  - "**/test_*.py"
  - "**/*.spec.ts"

  # Generated code
  - "**/*.pb.go"        # Protocol buffers
  - "**/*.gen.go"       # Generated Go
  - "**/generated/**"   # Generated directory

  # Build artifacts
  - "dist/**"
  - "build/**"
  - "*.min.js"

  # Documentation you don't want
  - "**/node_modules/**/README.md"
  - "vendor/**/README.md"
```

### Pre-built Filter Sets

**For Go Projects:**
```yaml
include_patterns:
  - "**/*.go"
  - "go.mod"
  - "go.sum"
  - "**/*.md"
  - "**/*.yaml"

exclude_patterns:
  - "vendor/**"
  - "**/*_test.go"
  - "**/*.pb.go"
  - "**/*.gen.go"
```

**For JavaScript/TypeScript Projects:**
```yaml
include_patterns:
  - "**/*.{js,ts,jsx,tsx}"
  - "**/*.json"
  - "**/*.md"
  - "package.json"
  - "tsconfig.json"

exclude_patterns:
  - "node_modules/**"
  - "dist/**"
  - "build/**"
  - "**/*.test.{js,ts}"
  - "**/*.spec.{js,ts}"
  - "**/*.min.js"
```

**For Documentation Only:**
```yaml
include_patterns:
  - "**/*.md"
  - "**/*.txt"
  - "docs/**"

exclude_patterns:
  - "node_modules/**/README.md"
  - "vendor/**/README.md"
  - "**/CHANGELOG.md"
```

**For Infrastructure as Code:**
```yaml
include_patterns:
  - "**/*.tf"
  - "**/*.tfvars"
  - "**/*.yaml"
  - "**/*.yml"
  - "**/*.sh"
  - "**/Dockerfile*"

exclude_patterns:
  - ".terraform/**"
  - "*.tfstate"
  - "*.tfstate.backup"
```

## Performance Tuning

### Rate Limiting

GitHub API has rate limits:
- **Authenticated requests**: 5,000 per hour
- **Unauthenticated requests**: 60 per hour

Configure appropriate rate limits:

```yaml
config:
  # Conservative (ensures you never hit limits)
  requests_per_second: 5

  # Moderate (good for most use cases)
  requests_per_second: 10

  # Aggressive (only if you have few sources)
  requests_per_second: 20
```

### Concurrent Downloads

Balance between speed and resource usage:

```yaml
config:
  # Conservative (low memory, slower)
  concurrent_downloads: 3

  # Moderate (recommended)
  concurrent_downloads: 5

  # Aggressive (faster but more memory)
  concurrent_downloads: 10
```

### File Size Limits

Skip large files that don't add value:

```yaml
config:
  # Small files only
  max_file_size: 524288  # 512KB

  # Medium files (recommended)
  max_file_size: 1048576  # 1MB

  # Large files (use cautiously)
  max_file_size: 5242880  # 5MB
```

### Schedule Optimization

Choose appropriate schedules based on update frequency:

```yaml
# High-activity repository (main branch)
schedule: "0 */2 * * *"  # Every 2 hours

# Normal activity
schedule: "0 */6 * * *"  # Every 6 hours

# Low activity (documentation)
schedule: "0 0 * * *"    # Once daily

# Archive (rarely changes)
schedule: "0 0 * * 0"    # Once weekly
```

## Troubleshooting

### Issue: Rate Limit Errors

**Symptoms:**
```
ERROR GitHub API rate limit exceeded
ERROR 403 Forbidden: rate limit exceeded
```

**Solutions:**

1. Check current rate limit:
```bash
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/rate_limit
```

2. Reduce `requests_per_second`:
```yaml
config:
  requests_per_second: 5  # Lower value
```

3. Adjust schedules to spread load:
```yaml
# Instead of all at same time
schedule: "0 */6 * * *"

# Stagger them
schedule: "0 */6 * * *"   # Repo 1
schedule: "30 */6 * * *"  # Repo 2 (30 min offset)
```

### Issue: Authentication Failed

**Symptoms:**
```
ERROR GitHub authentication failed: 401 Unauthorized
ERROR Invalid credentials
```

**Solutions:**

1. Verify token is set:
```bash
echo $GITHUB_TOKEN | cut -c1-10
# Should show: ghp_...
```

2. Test token directly:
```bash
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/user
```

3. Check token scopes:
```bash
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/rate_limit | jq '.resources'
```

4. Regenerate token if needed:
- Go to GitHub → Settings → Developer settings → Personal access tokens
- Revoke old token
- Generate new one with correct scopes

### Issue: No Files Being Processed

**Symptoms:**
```
INFO  Starting GitHub crawl for org/repo
INFO  Completed GitHub crawl: 0 files processed
```

**Solutions:**

1. Check include/exclude patterns:
```bash
# Test patterns locally
find . -name "*.go" | grep -v vendor | grep -v "_test.go"
```

2. Verify branch exists:
```bash
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/org/repo/branches
```

3. Check file size limits:
```yaml
config:
  max_file_size: 10485760  # Increase to 10MB
```

4. Enable debug logging:
```yaml
service:
  log_level: debug
```

### Issue: Slow Ingestion

**Symptoms:**
- Crawl takes > 30 minutes
- High memory usage
- Timeouts

**Solutions:**

1. Increase concurrency:
```yaml
config:
  concurrent_downloads: 10
```

2. Reduce file size:
```yaml
config:
  max_file_size: 524288  # 512KB only
```

3. Be more selective:
```yaml
include_patterns:
  - "src/**/*.go"  # Only src directory
exclude_patterns:
  - "**/*_test.go"
  - "vendor/**"
```

4. Split into multiple sources:
```yaml
# Split large repo into logical parts
sources:
  - id: repo_backend
    config:
      include_patterns: ["backend/**"]

  - id: repo_frontend
    config:
      include_patterns: ["frontend/**"]
```

### Issue: Missing Repository Access

**Symptoms:**
```
ERROR 404 Not Found: repository not found
```

**Solutions:**

1. Verify repository exists:
```bash
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/org/repo
```

2. Check organization membership:
```bash
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/user/orgs
```

3. Verify token has `repo` scope for private repos

4. For organization repos, ensure token has `read:org` scope

## Best Practices

### 1. Token Management

- ✅ Use separate tokens per environment (dev, staging, prod)
- ✅ Set expiration dates and rotate regularly (90 days)
- ✅ Use minimal required scopes
- ✅ Store tokens in secrets management (Kubernetes secrets, AWS Secrets Manager)
- ✅ Never commit tokens to git
- ✅ Monitor token usage via GitHub audit log

### 2. File Selection

- ✅ Be explicit with include patterns
- ✅ Always exclude vendor/node_modules
- ✅ Skip test files unless needed for documentation
- ✅ Exclude generated code
- ✅ Set reasonable file size limits
- ✅ Review what's being indexed periodically

### 3. Scheduling

- ✅ Match schedule to repository activity
- ✅ Stagger multiple repositories
- ✅ Use cron expressions carefully (test with https://crontab.guru)
- ✅ Monitor for overlapping jobs
- ✅ Consider time zones

### 4. Performance

- ✅ Start conservative, increase gradually
- ✅ Monitor rate limit usage
- ✅ Watch memory consumption
- ✅ Enable caching
- ✅ Set appropriate timeouts

### 5. Cost Optimization

- ✅ Use caching to avoid re-processing
- ✅ Skip large/binary files
- ✅ Don't crawl too frequently
- ✅ Exclude test files (less embeddings)
- ✅ Monitor embedding costs

### 6. Security

- ✅ Use read-only tokens
- ✅ Limit token scope to required repositories
- ✅ Enable audit logging
- ✅ Review access periodically
- ✅ Rotate tokens regularly
- ✅ Use GitHub Apps for enterprise

## Example Configurations

### Monorepo Configuration

```yaml
sources:
  - id: monorepo_full
    type: github
    enabled: true
    schedule: "0 */6 * * *"
    config:
      owner: your-org
      repo: monorepo
      branch: main
      token: ${GITHUB_TOKEN}

      include_patterns:
        # Multiple language support
        - "**/*.{go,ts,tsx,py,java}"
        - "**/*.{md,yaml,yml,json}"
        - "**/Dockerfile*"

      exclude_patterns:
        # All test files
        - "**/*_test.*"
        - "**/test_*.*"
        - "**/*.test.*"
        - "**/*.spec.*"

        # All dependencies
        - "vendor/**"
        - "node_modules/**"
        - "**/venv/**"
        - "**/target/**"

        # All generated code
        - "**/*.gen.*"
        - "**/*.pb.*"
        - "**/generated/**"

      max_file_size: 1048576
      concurrent_downloads: 5
      requests_per_second: 10
```

### Multi-Repo Organization

```yaml
sources:
  # Core services (high priority)
  - id: core_api
    type: github
    enabled: true
    schedule: "0 */3 * * *"  # Every 3 hours
    config:
      owner: your-org
      repo: core-api
      branch: main
      token: ${GITHUB_TOKEN}
      requests_per_second: 15

  - id: auth_service
    type: github
    enabled: true
    schedule: "15 */3 * * *"  # Offset by 15 min
    config:
      owner: your-org
      repo: auth-service
      branch: main
      token: ${GITHUB_TOKEN}
      requests_per_second: 15

  # Documentation (medium priority)
  - id: main_docs
    type: github
    enabled: true
    schedule: "0 */6 * * *"  # Every 6 hours
    config:
      owner: your-org
      repo: documentation
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns: ["**/*.md"]

  # Infrastructure (low priority)
  - id: terraform
    type: github
    enabled: true
    schedule: "0 2 * * *"  # Daily at 2 AM
    config:
      owner: your-org
      repo: infrastructure
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns: ["**/*.tf", "**/*.yaml"]
```

## Monitoring

Check these metrics for GitHub sources:

```bash
# Documents processed from GitHub
curl http://localhost:9094/metrics | grep 'rag_loader_documents.*github'

# GitHub API rate limit usage
curl http://localhost:9094/metrics | grep 'rag_loader_github_rate_limit'

# GitHub crawl duration
curl http://localhost:9094/metrics | grep 'rag_loader_ingestion_duration.*github'

# Errors by source
curl http://localhost:9094/metrics | grep 'rag_loader_errors.*github'
```

## Next Steps

1. ✅ Create GitHub personal access token
2. ✅ Configure first repository
3. ✅ Test with small repository
4. ✅ Monitor logs and metrics
5. ✅ Optimize patterns and schedules
6. ✅ Add additional repositories
7. ✅ Set up alerting

## Additional Resources

- [GitHub API Documentation](https://docs.github.com/en/rest)
- [GitHub Rate Limiting](https://docs.github.com/en/rest/overview/resources-in-the-rest-api#rate-limiting)
- [RAG Loader User Guide](./rag-loader-user-guide.md)
- [Cron Expression Generator](https://crontab.guru)

# RAG Loader: Multi-Organization and Bulk Repository Configuration

> **Purpose**: Guide for scanning multiple repositories and organizations efficiently
> **Audience**: Platform administrators managing multiple GitHub organizations
> **Related**: [RAG Loader User Guide](./rag-loader-user-guide.md) | [GitHub Setup](./rag-loader-github-setup.md)

## Overview

The RAG loader supports native organization-level configuration, allowing you to scan entire GitHub organizations without listing individual repositories. This guide covers:

- **Organization-wide scanning**: Automatically discover and scan all repos in an org
- **Selective repository scanning**: Choose specific repos within an organization
- **Multiple organizations**: Manage multiple orgs with separate tokens
- **Token management**: Best practices for authentication across organizations

## Quick Start

### Scanning All Repos in One Organization

**New Approach (Recommended):** Use the `github_org` source type:

```yaml
sources:
  - id: my_org
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: your-organization
      token: ${GITHUB_TOKEN}
      include_archived: false  # Skip archived repos
      include_forks: false     # Skip forked repos
      include_patterns:
        - "**/*.go"
        - "**/*.md"
      exclude_patterns:
        - "vendor/**"
        - "node_modules/**"
```

**That's it!** The RAG loader will automatically:
- Discover all repositories in the organization
- Apply filters (archived, forks)
- Create crawlers for each repository
- Scan on the configured schedule

### Multiple Organizations

Use multiple `github_org` sources with the same or different tokens:

```yaml
sources:
  - id: engineering_org
    type: github_org
    config:
      org: company-engineering
      token: ${GITHUB_TOKEN_ENG}

  - id: platform_org
    type: github_org
    config:
      org: company-platform
      token: ${GITHUB_TOKEN_PLATFORM}
```

## Configuration Patterns

### Pattern 1: Single Organization - All Repositories

Scan every repository in an organization:

```yaml
sources:
  - id: my_company
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: my-company
      token: ${GITHUB_TOKEN}
      include_archived: false
      include_forks: false
      include_patterns:
        - "**/*.go"
        - "**/*.md"
        - "**/*.yaml"
      exclude_patterns:
        - "vendor/**"
        - "node_modules/**"
```

### Pattern 2: Single Organization - Specific Repositories

Only scan specific repositories within an organization:

```yaml
sources:
  - id: my_company_core
    type: github_org
    enabled: true
    schedule: "0 */4 * * *"
    config:
      org: my-company
      token: ${GITHUB_TOKEN}
      repos:
        - "backend-api"
        - "frontend-web"
        - "mobile-app"
        - "documentation"
      include_patterns:
        - "**/*.go"
        - "**/*.ts"
        - "**/*.tsx"
        - "**/*.md"
```

### Pattern 3: Multiple Organizations - Same Token

One token with access to multiple organizations:

```yaml
sources:
  - id: engineering_org
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: company-engineering
      token: ${GITHUB_TOKEN}
      include_patterns:
        - "**/*.go"
        - "**/*.md"

  - id: platform_org
    type: github_org
    enabled: true
    schedule: "30 */6 * * *"  # 30 min offset
    config:
      org: company-platform
      token: ${GITHUB_TOKEN}
      include_patterns:
        - "**/*.go"
        - "**/*.md"
```

### Pattern 4: Multiple Organizations - Different Tokens

Separate tokens for better security and isolation:

```yaml
sources:
  - id: engineering_org
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: company-engineering
      token: ${GITHUB_TOKEN_ENG}
      include_patterns:
        - "**/*.go"
        - "**/*.md"

  - id: platform_org
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: company-platform
      token: ${GITHUB_TOKEN_PLATFORM}
      include_patterns:
        - "**/*.py"
        - "**/*.md"

  - id: data_org
    type: github_org
    enabled: true
    schedule: "0 */12 * * *"  # Less frequent
    config:
      org: company-data
      token: ${GITHUB_TOKEN_DATA}
      include_patterns:
        - "**/*.sql"
        - "**/*.md"
```

### Pattern 5: Mixed Configuration

Combine organization-wide and single-repo sources:

```yaml
sources:
  # Scan entire engineering org
  - id: engineering_org
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"
    config:
      org: company-engineering
      token: ${GITHUB_TOKEN}

  # But also scan a critical external repo more frequently
  - id: critical_external_repo
    type: github
    enabled: true
    schedule: "0 */2 * * *"  # Every 2 hours
    config:
      owner: external-org
      repo: critical-dependency
      branch: main
      token: ${GITHUB_TOKEN_EXTERNAL}
```

## Environment Variable Configuration

### Using .env File

Create `.env` in your project root:

```bash
# GitHub Tokens
GITHUB_TOKEN=ghp_your_primary_token
GITHUB_TOKEN_ENG=ghp_engineering_token
GITHUB_TOKEN_PLATFORM=ghp_platform_token
GITHUB_TOKEN_DATA=ghp_data_token
GITHUB_TOKEN_EXTERNAL=ghp_external_token

# AWS Bedrock
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...

# Database
DATABASE_HOST=postgres
DATABASE_NAME=devmesh_development
DATABASE_USER=devmesh
DATABASE_PASSWORD=devmesh

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
```

### Docker Compose with .env

```yaml
# docker-compose.local.yml
services:
  rag-loader:
    env_file:
      - .env
    volumes:
      - ./configs/rag-loader.yaml:/app/configs/rag-loader.yaml
```

### Kubernetes with Multiple Tokens

```bash
# Create secret with all tokens
kubectl create secret generic rag-loader-secrets \
  --namespace=devmesh \
  --from-literal=github-token=ghp_main_token \
  --from-literal=github-token-eng=ghp_eng_token \
  --from-literal=github-token-platform=ghp_platform_token \
  --from-literal=github-token-data=ghp_data_token
```

Update deployment:

```yaml
# deployment.yaml
env:
- name: GITHUB_TOKEN
  valueFrom:
    secretKeyRef:
      name: rag-loader-secrets
      key: github-token
- name: GITHUB_TOKEN_ENG
  valueFrom:
    secretKeyRef:
      name: rag-loader-secrets
      key: github-token-eng
- name: GITHUB_TOKEN_PLATFORM
  valueFrom:
    secretKeyRef:
      name: rag-loader-secrets
      key: github-token-platform
```

## Advanced Configuration Options

### Repository Filtering

**Include/Exclude Archived Repositories:**
```yaml
config:
  org: my-company
  include_archived: true  # Default: false
```

**Include/Exclude Forked Repositories:**
```yaml
config:
  org: my-company
  include_forks: true  # Default: false
```

**Specific Repository List:**
```yaml
config:
  org: my-company
  repos:
    - "repo1"
    - "repo2"
    - "repo3"
  # If repos is empty or not specified, ALL repos are scanned
```

### File Pattern Filtering

Control which files are indexed in each repository:

**By Language:**
```yaml
# Go repositories
include_patterns:
  - "**/*.go"
  - "**/*.md"
  - "go.mod"
  - "go.sum"
exclude_patterns:
  - "vendor/**"
  - "**/*_test.go"
  - "**/*.pb.go"

# TypeScript/JavaScript repositories
include_patterns:
  - "**/*.ts"
  - "**/*.tsx"
  - "**/*.js"
  - "**/*.jsx"
  - "**/*.md"
exclude_patterns:
  - "node_modules/**"
  - "dist/**"
  - "build/**"
  - "**/*.test.ts"
  - "**/*.min.js"

# Python repositories
include_patterns:
  - "**/*.py"
  - "**/*.md"
  - "requirements.txt"
  - "setup.py"
exclude_patterns:
  - "venv/**"
  - "**/__pycache__/**"
  - "**/test_*.py"
```

### Schedule Optimization

Stagger schedules to avoid overloading systems:

```yaml
sources:
  - id: critical_org
    schedule: "0 */2 * * *"  # Every 2 hours
    config:
      org: critical-services

  - id: standard_org
    schedule: "0 */6 * * *"  # Every 6 hours
    config:
      org: standard-services

  - id: docs_org
    schedule: "0 */12 * * *"  # Every 12 hours
    config:
      org: documentation
```

## Complete Real-World Example

### Scenario

Company with:
- 3 GitHub organizations
- Engineering: ~30 repos (Go/TypeScript)
- Platform: ~20 repos (Go/Python)
- Data: ~10 repos (Python/SQL)
- Different teams managing different orgs

### Step 1: Create .env File

```bash
# .env
GITHUB_TOKEN_ENG=ghp_engineering_team_token
GITHUB_TOKEN_PLATFORM=ghp_platform_team_token
GITHUB_TOKEN_DATA=ghp_data_team_token

AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...

DATABASE_HOST=postgres.company.internal
DATABASE_NAME=devmesh_production
DATABASE_USER=devmesh
DATABASE_PASSWORD=secure_password

REDIS_ADDR=redis.company.internal:6379
REDIS_PASSWORD=redis_password
```

### Step 2: Create Configuration

Create `configs/rag-loader.yaml`:

```yaml
sources:
  # Engineering Organization
  - id: engineering_org
    type: github_org
    enabled: true
    schedule: "0 */4 * * *"  # Every 4 hours
    config:
      org: company-engineering
      token: ${GITHUB_TOKEN_ENG}
      include_archived: false
      include_forks: false
      include_patterns:
        - "**/*.go"
        - "**/*.ts"
        - "**/*.tsx"
        - "**/*.md"
      exclude_patterns:
        - "vendor/**"
        - "node_modules/**"
        - "**/*_test.go"
        - "**/*.test.ts"

  # Platform Organization
  - id: platform_org
    type: github_org
    enabled: true
    schedule: "30 */4 * * *"  # Every 4 hours, 30 min offset
    config:
      org: company-platform
      token: ${GITHUB_TOKEN_PLATFORM}
      include_archived: false
      include_forks: false
      include_patterns:
        - "**/*.go"
        - "**/*.py"
        - "**/*.md"
      exclude_patterns:
        - "vendor/**"
        - "venv/**"
        - "**/*_test.go"
        - "**/test_*.py"

  # Data Organization (less frequent)
  - id: data_org
    type: github_org
    enabled: true
    schedule: "0 */8 * * *"  # Every 8 hours
    config:
      org: company-data
      token: ${GITHUB_TOKEN_DATA}
      include_archived: false
      include_forks: false
      include_patterns:
        - "**/*.py"
        - "**/*.sql"
        - "**/*.md"
      exclude_patterns:
        - "venv/**"
        - "**/__pycache__/**"

# Service configuration
service:
  port: 8084
  metrics_port: 9094
  log_level: info

# Scheduler configuration
scheduler:
  default_schedule: "0 */6 * * *"
  job_timeout: 30m
  max_concurrent_jobs: 3
```

### Step 3: Deploy

**Docker Compose:**
```bash
docker-compose -f docker-compose.local.yml up -d rag-loader
docker-compose logs -f rag-loader
```

**Kubernetes:**
```bash
# Create secret
kubectl create secret generic rag-loader-secrets \
  --namespace=devmesh \
  --from-literal=github-token-eng=$GITHUB_TOKEN_ENG \
  --from-literal=github-token-platform=$GITHUB_TOKEN_PLATFORM \
  --from-literal=github-token-data=$GITHUB_TOKEN_DATA \
  --from-literal=aws-access-key-id=$AWS_ACCESS_KEY_ID \
  --from-literal=aws-secret-access-key=$AWS_SECRET_ACCESS_KEY \
  --from-literal=database-password=$DATABASE_PASSWORD \
  --from-literal=redis-password=$REDIS_PASSWORD

# Deploy
kubectl apply -f apps/rag-loader/k8s/
```

### Step 4: Verify

```bash
# Check health
curl http://localhost:9094/health

# Check metrics
curl http://localhost:9094/metrics | grep rag_loader_documents_processed_total

# Check logs
docker-compose logs rag-loader | grep "Organization fetch complete"
# or
kubectl logs -n devmesh -l app=rag-loader | grep "Organization fetch complete"
```

## Monitoring Multiple Organizations

### Prometheus Queries

```promql
# Documents processed by organization
sum by (org) (rag_loader_documents_processed_total{source="github_org"})

# Errors by organization
sum by (org) (rag_loader_errors_total{source="github_org"})

# Embedding costs by organization
sum by (org) (rag_loader_embedding_cost_total{source="github_org"})

# Repository count by organization
rag_loader_org_repository_count
```

### Grafana Dashboard Example

Create alerts for:
- High error rates per organization
- Unusual cost increases
- Failed crawls
- Slow ingestion times

## Token Management Best Practices

### Security

1. **Use separate tokens per organization** for isolation
2. **Set minimal scopes** (read-only repository access)
3. **Rotate tokens every 90 days**
4. **Use GitHub Apps** for enterprise (better audit logs)
5. **Never commit tokens** to version control
6. **Use secrets management** (Kubernetes secrets, AWS Secrets Manager, Vault)

### Token Permissions

Minimum required permissions:
- `repo` (for private repos) or `public_repo` (for public only)
- `read:org` (to list organization repositories)

### Token Organization

```bash
# .env structure
# ====================
# Engineering Org
GITHUB_TOKEN_ENG=ghp_xxx
# Permissions: repo, read:org
# Owner: engineering-team@company.com
# Expires: 2025-06-01

# Platform Org
GITHUB_TOKEN_PLATFORM=ghp_yyy
# Permissions: repo, read:org
# Owner: platform-team@company.com
# Expires: 2025-06-01
```

### Token Verification

Test each token before deploying:

```bash
# Test token access
for TOKEN in $GITHUB_TOKEN_ENG $GITHUB_TOKEN_PLATFORM $GITHUB_TOKEN_DATA; do
  echo "Testing token..."
  curl -s -H "Authorization: token $TOKEN" \
    https://api.github.com/user | jq '.login'
done

# Test organization access
curl -s -H "Authorization: token $GITHUB_TOKEN_ENG" \
  https://api.github.com/orgs/company-engineering/repos | jq '.[].name'
```

## Troubleshooting

### No Repositories Found

**Symptom:** Logs show "No repositories found for organization"

**Solutions:**
1. Verify token has access: `curl -H "Authorization: token $TOKEN" https://api.github.com/orgs/ORG/repos`
2. Check organization name is correct (case-sensitive)
3. Ensure token has `read:org` permission
4. Check if `include_archived` or `include_forks` filtering is too restrictive

### Rate Limiting

**Symptom:** "API rate limit exceeded" errors

**Solutions:**
1. Stagger schedules across organizations
2. Use authenticated requests (higher rate limit)
3. Reduce scan frequency for less critical orgs
4. Consider GitHub Apps (higher rate limits)

### High Costs

**Symptom:** Unexpected AWS Bedrock costs

**Solutions:**
1. Review `include_patterns` - are you indexing too many files?
2. Add more `exclude_patterns` (node_modules, test files, etc.)
3. Reduce scan frequency
4. Use `repos` list to scan only critical repositories
5. Monitor `rag_loader_embedding_cost_total` metric

### Memory Issues

**Symptom:** RAG loader OOM (out of memory)

**Solutions:**
1. Reduce `max_concurrent_jobs` in scheduler config
2. Increase resource limits in Kubernetes/Docker
3. Scan fewer repositories per source
4. Split large organizations into multiple sources

## Migration from Script-Based Approach

### Legacy Method (Not Recommended)

Previously, you had to use scripts to generate individual repository configurations. This required:

1. Running `scripts/generate-rag-config.sh` for each organization
2. Manually managing generated YAML files
3. Dealing with pagination and filtering in bash scripts
4. Updating configs whenever new repos were added

**Old approach:**
```bash
# Generate config for an org (legacy)
./scripts/generate-rag-config.sh your-org > configs/your-org.yaml
```

This generated individual `github` sources for each repository:
```yaml
sources:
  - id: org_repo1
    type: github
    config:
      owner: your-org
      repo: repo1
  - id: org_repo2
    type: github
    config:
      owner: your-org
      repo: repo2
  # ... one entry per repo
```

### New Native Approach (Recommended)

Now you simply use the `github_org` source type:

```yaml
sources:
  - id: your_org
    type: github_org
    config:
      org: your-org
      token: ${GITHUB_TOKEN}
```

**Benefits:**
- No scripts to run
- Automatic repository discovery
- Dynamic updates when repos are added/removed
- Cleaner configuration
- Built-in filtering support

### Migrating Your Configuration

**From:** Script-generated individual repo sources
```yaml
sources:
  - id: eng_backend
    type: github
    config:
      owner: company-eng
      repo: backend-service
  - id: eng_frontend
    type: github
    config:
      owner: company-eng
      repo: frontend-app
  # ... 30 more repos
```

**To:** Single organization source
```yaml
sources:
  - id: engineering_org
    type: github_org
    config:
      org: company-eng
      token: ${GITHUB_TOKEN_ENG}
      include_archived: false
      include_forks: false
```

### Note on Legacy Scripts

The script-based configuration generation tools (`scripts/generate-rag-config.sh`, etc.) are kept for reference but are no longer the recommended approach. They may be useful if you need the old behavior for specific use cases.

## Additional Resources

- [RAG Loader User Guide](./rag-loader-user-guide.md)
- [GitHub Setup Guide](./rag-loader-github-setup.md)
- [GitHub API Documentation](https://docs.github.com/en/rest/repos/repos)
- [Quick Start Guide](./rag-loader-quickstart.md)

# RAG Loader Quick Start Guide

> **Purpose**: Get the RAG loader up and running in 10 minutes
> **Audience**: Users who want to start quickly
> **Full Documentation**: [RAG Loader User Guide](./rag-loader-user-guide.md) | [GitHub Setup](./rag-loader-github-setup.md)

## Prerequisites Checklist

- [ ] PostgreSQL 14+ with pgvector extension
- [ ] Redis 7+
- [ ] GitHub personal access token
- [ ] AWS account with Bedrock access (us-east-1)
- [ ] Docker Compose OR Kubernetes cluster
- [ ] API key or JWT token (for REST API setup)

## Setup Methods

Choose the method that fits your architecture:

1. **REST API (Recommended)**: Multi-tenant, dynamic configuration via API calls
2. **Docker Compose**: Quick local development with static YAML config
3. **Kubernetes**: Production deployment with ConfigMaps

---

## Method 1: REST API Setup (Recommended - Multi-Tenant)

**Best for**: Production environments, multi-tenant systems, dynamic configuration

### Step 1: Get Your Credentials

1. **GitHub Token**:
   - Go to https://github.com/settings/tokens
   - Generate new token (classic)
   - Select `repo` scope
   - Copy the token (starts with `ghp_`)

2. **DevMesh API Key**:
   - Get from your tenant administrator
   - Format: `devmesh_<hash>`

### Step 2: Start the RAG Loader Service

```bash
# Set required environment variables
export RAG_MASTER_KEY=$(openssl rand -base64 32)
export JWT_SECRET="your-jwt-secret"
export AWS_REGION="us-east-1"
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"

# Start services
docker-compose -f docker-compose.local.yml up -d rag-loader

# Check health
curl http://localhost:9094/health
```

### Step 3: Configure Source via API

Create a JSON file with your source configuration:

```bash
cat > github-source.json << 'EOF'
{
  "source_id": "my-org-repos",
  "source_type": "github_org",
  "config": {
    "org": "your-github-org",
    "include_archived": false,
    "include_forks": false,
    "include_patterns": ["*.md", "*.go", "*.yaml", "*.yml", "*.json"],
    "exclude_patterns": ["vendor/*", "node_modules/*", ".git/*"]
  },
  "credentials": {
    "token": "ghp_your_github_token_here"
  },
  "schedule": "0 */6 * * *",
  "description": "My GitHub organization repositories"
}
EOF
```

### Step 4: Create the Source

```bash
# Create source
curl -X POST http://localhost:8084/api/v1/rag/sources \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d @github-source.json

# Response:
# {
#   "id": "uuid-here",
#   "source_id": "my-org-repos",
#   "message": "Source created successfully"
# }
```

### Step 5: Trigger Initial Sync

```bash
# Manually trigger sync
curl -X POST http://localhost:8084/api/v1/rag/sources/my-org-repos/sync \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY"

# Response:
# {
#   "job_id": "uuid-here",
#   "message": "Sync job queued successfully"
# }
```

### Step 6: Monitor Progress

```bash
# List all sources
curl http://localhost:8084/api/v1/rag/sources \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY"

# Get specific source
curl http://localhost:8084/api/v1/rag/sources/my-org-repos \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY"

# Check sync jobs
curl http://localhost:8084/api/v1/rag/sources/my-org-repos/jobs \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY"

# Check metrics
curl http://localhost:9094/metrics | grep rag_loader_documents_processed_total
```

### API Source Types

**GitHub Organization (all repos)**:
```json
{
  "source_type": "github_org",
  "config": {
    "org": "your-org",
    "include_archived": false,
    "include_forks": false,
    "repos": []  // Empty = all repos, or specify: ["repo1", "repo2"]
  }
}
```

**GitHub Single Repository**:
```json
{
  "source_type": "github_repo",
  "config": {
    "owner": "your-org",
    "repo": "your-repo",
    "branch": "main"
  }
}
```

### API Management Commands

```bash
# Update source configuration
curl -X PUT http://localhost:8084/api/v1/rag/sources/my-org-repos \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "schedule": "0 */12 * * *"
  }'

# Delete source
curl -X DELETE http://localhost:8084/api/v1/rag/sources/my-org-repos \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY"

# List sync jobs
curl http://localhost:8084/api/v1/rag/sources/my-org-repos/jobs?limit=10 \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY"
```

### Managing Sync Jobs

**Delete duplicate or unwanted sync jobs**:

If you accidentally triggered multiple sync jobs, you can delete the unwanted ones directly from the database:

```bash
# Step 1: List all sync jobs for a source
docker exec devops-mcp-database-1 psql -U devmesh -d devmesh_development -c "
SELECT id, source_id, job_type, status, created_at
FROM rag.tenant_sync_jobs
WHERE source_id = 'my-org-repos'
ORDER BY created_at DESC;"

# Step 2: Delete specific jobs by ID (keep the most recent one)
docker exec devops-mcp-database-1 psql -U devmesh -d devmesh_development -c "
DELETE FROM rag.tenant_sync_jobs
WHERE id IN (
  'job-id-1',
  'job-id-2'
);"

# Step 3: Verify only desired jobs remain
curl http://localhost:8084/api/v1/rag/sources/my-org-repos/jobs \
  -H "Authorization: Bearer devmesh_YOUR_API_KEY"
```

**Note**: A DELETE endpoint for jobs will be added in a future release. For now, direct database access is required.

### API Security

**Credentials are encrypted**:
- GitHub tokens encrypted with AES-256-GCM
- Tenant-specific encryption keys derived from master key
- Per-tenant isolation enforced at database level

**Authentication required**:
- All endpoints require `Authorization: Bearer <token>` header
- Supports API keys (from `mcp.api_keys` table) or JWT tokens
- Row-level security disabled for app-level tenant isolation

---

## Method 2: Docker Compose Setup (Static YAML Config)

### Step 1: Get Your GitHub Token

1. Go to https://github.com/settings/tokens
2. Generate new token (classic)
3. Select `repo` scope
4. Copy the token (starts with `ghp_`)

### Step 2: Create Environment File

Create `.env`:

```bash
# GitHub
GITHUB_TOKEN=ghp_your_token_here

# AWS Bedrock
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key

# Database
DATABASE_HOST=postgres
DATABASE_NAME=devmesh_development
DATABASE_USER=devmesh
DATABASE_PASSWORD=devmesh

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
```

### Step 3: Create Configuration

Create `configs/rag-loader.yaml`:

**Option A: Scan entire organization (all repos)**
```yaml
sources:
  - id: my_org
    type: github_org
    enabled: true
    schedule: "*/10 * * * *"  # Every 10 minutes for testing
    config:
      org: your-github-org
      token: ${GITHUB_TOKEN}
      include_archived: false  # Skip archived repos
      include_forks: false     # Skip forked repos
      include_patterns:
        - "**/*.go"
        - "**/*.md"
      exclude_patterns:
        - "vendor/**"
        - "**/*_test.go"
```

**Option B: Scan specific repositories**
```yaml
sources:
  - id: my_org_specific
    type: github_org
    enabled: true
    schedule: "*/10 * * * *"
    config:
      org: your-github-org
      token: ${GITHUB_TOKEN}
      repos:
        - "repo1"
        - "repo2"
      include_patterns:
        - "**/*.go"
        - "**/*.md"
      exclude_patterns:
        - "vendor/**"

```

**Option C: Single repository (original method)**
```yaml
sources:
  - id: my_repo
    type: github
    enabled: true
    schedule: "*/10 * * * *"
    config:
      owner: your-github-org
      repo: your-repo-name
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns:
        - "**/*.go"
        - "**/*.md"
      exclude_patterns:
        - "vendor/**"
```

### Step 4: Start Services

```bash
# Start all services
docker-compose -f docker-compose.local.yml up -d

# Check RAG loader logs
docker-compose logs -f rag-loader

# You should see:
# INFO Starting RAG Loader
# INFO Starting GitHub crawl for your-org/your-repo
# INFO Downloaded file: README.md
# INFO Completed GitHub crawl: XX files processed
```

### Step 5: Verify It's Working

```bash
# Check health
curl http://localhost:9094/health

# Check metrics
curl http://localhost:9094/metrics | grep rag_loader_documents_processed_total

# Check database (should have embeddings)
docker-compose exec postgres psql -U devmesh -d devmesh_development \
  -c "SELECT COUNT(*) FROM rag.documents;"
```

## 10-Minute Setup (Kubernetes)

### Step 1: Create Secrets

```bash
kubectl create namespace devmesh

kubectl create secret generic rag-loader-secrets \
  --namespace=devmesh \
  --from-literal=github-token=ghp_your_token \
  --from-literal=database-host=postgres.your-cluster.local \
  --from-literal=database-user=devmesh \
  --from-literal=database-password=your-password \
  --from-literal=redis-addr=redis.your-cluster.local:6379 \
  --from-literal=redis-password=your-redis-password \
  --from-literal=aws-access-key-id=AKIA... \
  --from-literal=aws-secret-access-key=your-secret
```

### Step 2: Configure Data Source

Edit `apps/rag-loader/k8s/configmap.yaml` and update the sources section:

```yaml
sources:
  - id: my_github_repo
    type: github
    enabled: true
    schedule: "0 */6 * * *"
    config:
      owner: your-org
      repo: your-repo
      branch: main
      include_patterns:
        - "**/*.go"
        - "**/*.md"
```

### Step 3: Deploy

```bash
# Apply manifests
kubectl apply -f apps/rag-loader/k8s/

# Check status
kubectl get pods -n devmesh -l app=rag-loader

# Check logs
kubectl logs -n devmesh -l app=rag-loader --tail=100
```

### Step 4: Verify

```bash
# Port forward
kubectl port-forward -n devmesh svc/rag-loader 9094:9094

# Check health
curl http://localhost:9094/health

# Check metrics
curl http://localhost:9094/metrics | grep rag_loader
```

## Common First-Time Issues

### Issue: "GitHub authentication failed"

```bash
# Verify your token
echo $GITHUB_TOKEN | cut -c1-10
# Should show: ghp_...

# Test it
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/user
```

**Fix**: Regenerate token with `repo` scope

### Issue: "No files processed"

**Fix**: Check your include/exclude patterns

```yaml
# Start simple
include_patterns:
  - "**/*.md"  # Just markdown files first
exclude_patterns:
  - "vendor/**"
  - "node_modules/**"
```

### Issue: "Database connection failed"

```bash
# Test database connection
psql $DATABASE_URL -c "SELECT version();"

# Check if pgvector is installed
psql $DATABASE_URL -c "SELECT * FROM pg_extension WHERE extname='vector';"
```

**Fix**: Install pgvector extension

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

### Issue: "AWS Bedrock errors"

```bash
# Test AWS credentials
aws bedrock list-foundation-models --region us-east-1

# Test specific model
aws bedrock invoke-model \
  --region us-east-1 \
  --model-id amazon.titan-embed-text-v2:0 \
  --body '{"inputText":"test"}' \
  output.json
```

**Fix**: Ensure Bedrock is enabled in us-east-1 and you have appropriate permissions

### Issue: "Multiple duplicate sync jobs queued"

If you accidentally triggered the sync endpoint multiple times, you'll have duplicate queued jobs.

**Fix**: Delete the older jobs, keeping only the most recent:

```bash
# List jobs
docker exec devops-mcp-database-1 psql -U devmesh -d devmesh_development -c "
SELECT id, source_id, status, created_at
FROM rag.tenant_sync_jobs
WHERE source_id = 'my-org-repos'
ORDER BY created_at DESC;"

# Delete older jobs (keep the newest)
docker exec devops-mcp-database-1 psql -U devmesh -d devmesh_development -c "
DELETE FROM rag.tenant_sync_jobs
WHERE id IN ('old-job-id-1', 'old-job-id-2');"
```

See [Managing Sync Jobs](#managing-sync-jobs) for more details.

## Configuration Templates

### Template 1: Single Go Repository

```yaml
sources:
  - id: go_project
    type: github
    enabled: true
    schedule: "0 */6 * * *"
    config:
      owner: your-org
      repo: go-service
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns:
        - "**/*.go"
        - "**/*.md"
        - "go.mod"
        - "go.sum"
      exclude_patterns:
        - "vendor/**"
        - "**/*_test.go"
        - "**/*.pb.go"
```

### Template 2: Documentation Repository

```yaml
sources:
  - id: docs
    type: github
    enabled: true
    schedule: "0 */12 * * *"
    config:
      owner: your-org
      repo: documentation
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns:
        - "**/*.md"
        - "**/*.txt"
```

### Template 3: Multiple Repositories

```yaml
sources:
  - id: backend
    type: github
    enabled: true
    schedule: "0 */4 * * *"
    config:
      owner: your-org
      repo: backend-service
      branch: main
      token: ${GITHUB_TOKEN}

  - id: frontend
    type: github
    enabled: true
    schedule: "30 */4 * * *"  # 30 min offset
    config:
      owner: your-org
      repo: frontend-app
      branch: main
      token: ${GITHUB_TOKEN}
```

## Monitoring Quick Check

```bash
# Documents processed
curl -s http://localhost:9094/metrics | grep rag_loader_documents_processed_total

# Errors
curl -s http://localhost:9094/metrics | grep rag_loader_errors_total

# Embeddings generated
curl -s http://localhost:9094/metrics | grep rag_loader_embeddings_generated_total

# Cost tracking
curl -s http://localhost:9094/metrics | grep rag_loader_embedding_cost_total
```

## Next Steps

Once you have it working:

1. **Optimize patterns**: Review what files are being indexed
2. **Adjust schedule**: Set appropriate crawl frequency
3. **Add more sources**: Configure additional repositories
4. **Set up monitoring**: Configure Prometheus and Grafana
5. **Configure alerting**: Set up alerts for errors and high costs

## Getting Help

- 📖 **Full documentation**: [RAG Loader User Guide](./rag-loader-user-guide.md)
- 🔧 **GitHub setup**: [GitHub Integration Guide](./rag-loader-github-setup.md)
- 🐛 **Issues**: Check logs with `docker-compose logs -f rag-loader`
- 📊 **Metrics**: Access Prometheus metrics at `http://localhost:9094/metrics`

## Useful Commands

```bash
# Check if RAG loader is running
docker-compose ps rag-loader
kubectl get pods -n devmesh -l app=rag-loader

# View logs
docker-compose logs -f rag-loader
kubectl logs -n devmesh -l app=rag-loader -f

# Restart service
docker-compose restart rag-loader
kubectl rollout restart deployment/rag-loader -n devmesh

# Check database
docker-compose exec postgres psql -U devmesh -d devmesh_development \
  -c "SELECT COUNT(*) FROM rag.documents;"

# Test GitHub token
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/rate_limit

# List sync jobs for a source
docker exec devops-mcp-database-1 psql -U devmesh -d devmesh_development \
  -c "SELECT id, source_id, status, created_at FROM rag.tenant_sync_jobs WHERE source_id = 'my-org-repos' ORDER BY created_at DESC;"

# Delete duplicate sync jobs
docker exec devops-mcp-database-1 psql -U devmesh -d devmesh_development \
  -c "DELETE FROM rag.tenant_sync_jobs WHERE id = 'job-id-to-delete';"
```

## Troubleshooting Checklist

- [ ] GitHub token has `repo` scope
- [ ] Token is set in environment or secrets
- [ ] PostgreSQL has pgvector extension installed
- [ ] Redis is running and accessible
- [ ] AWS credentials are valid
- [ ] Bedrock is enabled in us-east-1
- [ ] Database schema is migrated
- [ ] Include/exclude patterns are correct
- [ ] Schedule is in valid cron format
- [ ] Network connectivity to all services

## Configuration Validation

Before deploying, validate your configuration:

```bash
# Check YAML syntax
yamllint configs/rag-loader.yaml

# Verify environment variables
env | grep -E 'GITHUB|AWS|DATABASE|REDIS'

# Test GitHub access
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/your-org/your-repo

# Verify AWS Bedrock
aws bedrock list-foundation-models --region us-east-1

# Test database
psql $DATABASE_URL -c "SELECT version();"

# Test Redis (extract host from REDIS_ADDR)
redis-cli -h ${REDIS_ADDR%:*} ping
```

## Success Criteria

You know it's working when:

✅ Health endpoint returns "healthy"
✅ Logs show "Completed GitHub crawl: N files processed"
✅ Metrics show `rag_loader_documents_processed_total` increasing
✅ Database has rows in `rag.documents` table
✅ No errors in logs or metrics

## What's Next?

After successful setup:

1. **Read the full guide**: [RAG Loader User Guide](./rag-loader-user-guide.md)
2. **Configure GitHub properly**: [GitHub Integration Guide](./rag-loader-github-setup.md)
3. **Set up monitoring**: Configure Prometheus and Grafana dashboards
4. **Optimize costs**: Review embedding costs and adjust patterns
5. **Scale up**: Add more repositories and data sources

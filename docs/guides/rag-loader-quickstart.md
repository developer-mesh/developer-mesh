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

## 5-Minute Setup (Docker Compose)

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
DATABASE_USERNAME=devmesh
DATABASE_PASSWORD=devmesh

# Redis
REDIS_HOST=redis
REDIS_PORT=6379
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
  --from-literal=database-username=devmesh \
  --from-literal=database-password=your-password \
  --from-literal=redis-host=redis.your-cluster.local \
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

# Test Redis
redis-cli -h $REDIS_HOST ping
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

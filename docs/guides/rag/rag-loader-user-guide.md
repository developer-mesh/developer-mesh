# RAG Loader User Guide

> **Purpose**: Complete guide for configuring and operating the RAG loader service
> **Audience**: DevOps engineers, platform administrators, developers
> **Prerequisites**: Kubernetes cluster, PostgreSQL with pgvector, Redis, AWS Bedrock access
> **Status**: Production-ready as of Phase 5

## Overview

The RAG Loader is an automated ingestion service that:
- Crawls configured data sources (GitHub, web, S3, etc.)
- Processes and chunks documents intelligently
- Generates embeddings using AWS Bedrock
- Stores vector embeddings in PostgreSQL with pgvector
- Runs on a configurable schedule
- Provides comprehensive monitoring and observability

### Architecture

```
┌─────────────────┐
│   Data Sources  │  (GitHub, Web, S3, etc.)
└────────┬────────┘
         │ crawl
         ▼
┌─────────────────┐
│  RAG Loader     │  ← You are here
│  - Scheduler    │
│  - Processor    │
│  - Embedder     │
└────────┬────────┘
         │ store
         ▼
┌─────────────────┐
│  PostgreSQL +   │
│  pgvector       │
└─────────────────┘
```

## Quick Start

### 1. Prerequisites

**Required Infrastructure:**
- PostgreSQL 14+ with pgvector extension
- Redis 7+ for caching
- AWS account with Bedrock access
- Kubernetes cluster (or Docker Compose for local development)

**Required Credentials:**
- GitHub Personal Access Token (classic) with `repo` scope
- AWS credentials with Bedrock permissions
- Database credentials

### 2. Minimal Configuration

Create a configuration file (`rag-loader.yaml`):

```yaml
service:
  port: 8084
  metrics_port: 9094

# Database connection
database:
  host: postgres.example.com
  port: 5432
  database: devmesh_production
  username: ${DATABASE_USERNAME}
  password: ${DATABASE_PASSWORD}
  sslmode: require

# Redis connection
redis:
  host: redis.example.com
  port: 6379
  password: ${REDIS_PASSWORD}

# Configure your first data source
sources:
  - id: my_github_repo
    type: github
    enabled: true
    schedule: "0 */6 * * *"  # Every 6 hours
    config:
      owner: your-org
      repo: your-repo
      branch: main
      # GitHub token via environment variable
      token: ${GITHUB_TOKEN}
      include_patterns:
        - "*.go"
        - "*.md"
        - "*.yaml"
      exclude_patterns:
        - "vendor/**"
        - "**/*_test.go"
```

### 3. Environment Variables

Create `.env` file:

```bash
# Database
DATABASE_USER=devmesh
DATABASE_PASSWORD=your-secure-password

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=your-redis-password

# GitHub
GITHUB_TOKEN=ghp_your_personal_access_token_here

# AWS Bedrock
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
```

### 4. Run with Docker Compose

```bash
# Start the service
docker-compose -f docker-compose.local.yml up rag-loader

# Check logs
docker-compose logs -f rag-loader

# Verify health
curl http://localhost:9094/health
```

### 5. Verify Operation

```bash
# Check metrics endpoint
curl http://localhost:9094/metrics | grep rag_loader

# Check API (if enabled)
curl http://localhost:8084/api/v1/rag/sources

# View logs
docker-compose logs -f rag-loader | grep -E "processing|completed"
```

## Configuration Guide

### Service Configuration

```yaml
service:
  # API server port (optional, disable by setting scheduler.enable_api: false)
  port: 8084

  # Metrics and health check port
  metrics_port: 9094

  # Graceful shutdown timeout
  shutdown_timeout: 30s

  # Logging level (debug, info, warn, error)
  log_level: info
```

### Database Configuration

```yaml
database:
  host: postgres.example.com
  port: 5432
  database: devmesh_production
  username: ${DATABASE_USERNAME}
  password: ${DATABASE_PASSWORD}

  # SSL mode (disable, require, verify-ca, verify-full)
  sslmode: require

  # Schema search path
  search_path: "rag,mcp,public"

  # Connection pool settings
  max_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 300s
```

**Database Setup:**

The RAG loader requires the `rag` schema and tables. Run migrations:

```bash
# Apply migrations
make migrate-up

# Or manually
psql $DATABASE_URL -f migrations/rag/*.sql
```

### Redis Configuration

```yaml
redis:
  host: redis.example.com
  port: 6379
  password: ${REDIS_PASSWORD}
  db: 0

  # Connection pool settings
  pool_size: 10
  min_idle_conns: 5

  # Timeouts
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
```

Redis is used for:
- Document caching
- Embedding caching
- Distributed locking (for scheduled jobs)
- Bloom filters (deduplication)

### Processing Configuration

```yaml
processing:
  # Chunking strategy
  chunking:
    default_strategy: semantic  # or: fixed, markdown
    strategies:
      fixed:
        max_tokens: 512
        overlap_tokens: 50
      semantic:
        max_tokens: 1024
        similarity_threshold: 0.8
      markdown:
        respect_headers: true
        max_section_size: 2048

  # Embedding generation
  embedding:
    model: amazon.titan-embed-text-v2:0
    batch_size: 100
    max_retries: 3
    retry_delay: 1s
    timeout: 30s

  # Deduplication
  deduplication:
    enabled: true
    bloom_filter_size: 1000000
    false_positive_rate: 0.01
```

### Scheduler Configuration

```yaml
scheduler:
  # Enable REST API for manual job triggers
  enable_api: true

  # Default schedule if source doesn't specify one (cron format)
  default_schedule: "0 */6 * * *"

  # Maximum concurrent jobs
  max_concurrent_jobs: 3

  # Job timeout
  job_timeout: 3600s  # 1 hour
```

### Rate Limiting

```yaml
rate_limiting:
  enabled: true
  requests_per_second: 100
  burst_size: 50

  # Per-source limits
  per_source:
    github: 10      # Respect GitHub API limits
    web: 20         # Web crawling
    s3: 50          # S3 is generous
    confluence: 10  # API limits
    jira: 10        # API limits
```

### Circuit Breaker

```yaml
circuit_breaker:
  enabled: true
  max_failures: 5
  reset_timeout: 60s
  half_open_max_requests: 3
  failure_threshold: 0.5
  minimum_request_count: 10
```

### Caching

```yaml
cache:
  enabled: true
  default_ttl: 24h
  max_memory_mb: 1024
  eviction_policy: "allkeys-lru"
  key_prefix: "rag:"
```

### Monitoring

```yaml
monitoring:
  metrics_enabled: true
  tracing_enabled: true
  log_level: info
  alert_on_failure: true
  quality_tracking: true
```

## Data Source Configuration

### GitHub Source

The RAG loader supports two types of GitHub sources:
1. **Single Repository** (`github` type) - Scan individual repositories
2. **Organization** (`github_org` type) - Automatically discover and scan all repos in an organization

#### Organization Source (Recommended for Multiple Repos)

Automatically scan all repositories in a GitHub organization:

```yaml
sources:
  - id: my_org
    type: github_org
    enabled: true
    schedule: "0 */6 * * *"  # Every 6 hours
    config:
      org: your-organization
      token: ${GITHUB_TOKEN}
      include_archived: false  # Skip archived repos (default: false)
      include_forks: false     # Skip forked repos (default: false)
      include_patterns:
        - "**/*.go"
        - "**/*.md"
        - "**/*.yaml"
      exclude_patterns:
        - "vendor/**"
        - "**/*_test.go"
```

**Scan specific repos only:**
```yaml
sources:
  - id: my_org_specific
    type: github_org
    enabled: true
    schedule: "0 */4 * * *"
    config:
      org: your-organization
      token: ${GITHUB_TOKEN}
      repos:
        - "backend-service"
        - "frontend-app"
        - "docs"
      include_patterns:
        - "**/*.go"
        - "**/*.md"
```

**Multiple organizations:**
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
    schedule: "30 */6 * * *"  # 30 min offset
    config:
      org: company-platform
      token: ${GITHUB_TOKEN_PLATFORM}
      include_patterns:
        - "**/*.py"
        - "**/*.md"
```

See [Multi-Organization Setup Guide](./rag-loader-multi-org-setup.md) for detailed configuration patterns.

#### Single Repository Source

For scanning individual repositories:

```yaml
sources:
  - id: my_repo
    type: github
    enabled: true
    schedule: "0 */6 * * *"  # Every 6 hours
    config:
      owner: your-org
      repo: your-repo
      branch: main
      token: ${GITHUB_TOKEN}
```

#### Advanced GitHub Configuration

```yaml
sources:
  - id: monorepo_project
    type: github
    enabled: true
    schedule: "0 2 * * *"  # Daily at 2 AM
    config:
      # Repository details
      owner: your-org
      repo: monorepo
      branch: main

      # Authentication
      token: ${GITHUB_TOKEN}

      # File filtering
      include_patterns:
        - "apps/**/*.go"
        - "pkg/**/*.go"
        - "*.md"
        - "docs/**/*.md"
        - "*.yaml"

      exclude_patterns:
        - "vendor/**"
        - "node_modules/**"
        - "**/*_test.go"
        - "**/*.gen.go"
        - ".git/**"
        - "*.pb.go"

      # Performance settings
      max_file_size: 1048576  # 1MB
      concurrent_downloads: 5

      # Rate limiting
      requests_per_second: 10
```

#### Multiple Repository Configuration

**Option 1: Organization Source (Recommended)**

Instead of configuring each repository individually, use the `github_org` type to automatically discover all repos:

```yaml
sources:
  - id: my_company
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
        - "**/*.md"
        - "**/*.yaml"
        - "**/*.tf"
        - "**/*.sh"
```

**Option 2: Individual Repository Sources**

For finer control or different schedules per repository:

```yaml
sources:
  # Main application repository
  - id: main_app
    type: github
    enabled: true
    schedule: "0 */4 * * *"  # Every 4 hours
    config:
      owner: your-org
      repo: main-app
      branch: main
      token: ${GITHUB_TOKEN}

  # Documentation repository
  - id: docs
    type: github
    enabled: true
    schedule: "0 */12 * * *"  # Every 12 hours
    config:
      owner: your-org
      repo: docs
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns:
        - "**/*.md"

  # Infrastructure repository
  - id: infrastructure
    type: github
    enabled: true
    schedule: "0 0 * * *"  # Daily at midnight
    config:
      owner: your-org
      repo: infrastructure
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns:
        - "**/*.yaml"
        - "**/*.tf"
        - "**/*.sh"
```

### Web Source (Future)

```yaml
sources:
  - id: docs_site
    type: web
    enabled: false  # Not yet implemented
    schedule: "0 0 * * *"
    config:
      base_url: https://docs.example.com
      max_depth: 3
      follow_external: false
      respect_robots: true
```

### S3 Source (Future)

```yaml
sources:
  - id: s3_documents
    type: s3
    enabled: false  # Not yet implemented
    schedule: "0 */12 * * *"
    config:
      bucket: my-docs-bucket
      prefix: documents/
      region: us-east-1
```

## GitHub Setup Guide

### Step 1: Create GitHub Personal Access Token

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Give it a descriptive name: "RAG Loader - Production"
4. Select scopes:
   - ✅ `repo` (Full control of private repositories)
   - ✅ `read:org` (if accessing organization repositories)
5. Set expiration (recommend 90 days, use rotation)
6. Click "Generate token"
7. **Copy the token immediately** - you won't see it again!

### Step 2: Test Token Access

```bash
# Test the token
export GITHUB_TOKEN=ghp_your_token_here

# List repositories
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/user/repos

# Get repository info
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/your-org/your-repo
```

### Step 3: Configure Token in Kubernetes

Create a Kubernetes secret:

```bash
kubectl create secret generic rag-loader-secrets \
  --namespace=devmesh \
  --from-literal=github-token=ghp_your_token_here \
  --from-literal=database-password=your-db-password \
  --from-literal=redis-password=your-redis-password \
  --from-literal=aws-access-key-id=your-aws-key \
  --from-literal=aws-secret-access-key=your-aws-secret
```

### Step 4: Configure Data Source

Add to your configuration:

```yaml
sources:
  - id: my_org_repos
    type: github
    enabled: true
    schedule: "0 */6 * * *"
    config:
      owner: your-org
      repo: your-repo
      branch: main
      token: ${GITHUB_TOKEN}
      include_patterns:
        - "**/*.go"
        - "**/*.md"
      exclude_patterns:
        - "vendor/**"
        - "**/*_test.go"
```

### Step 5: Verify Ingestion

```bash
# Check logs for GitHub crawling
kubectl logs -n devmesh deployment/rag-loader | grep -i github

# Check metrics
kubectl port-forward -n devmesh svc/rag-loader 9094:9094
curl http://localhost:9094/metrics | grep rag_loader_github
```

## Deployment

### Kubernetes Deployment

The RAG loader includes production-ready Kubernetes manifests in `apps/rag-loader/k8s/`.

#### 1. Create Namespace

```bash
kubectl create namespace devmesh
```

#### 2. Create Secrets

```bash
kubectl create secret generic rag-loader-secrets \
  --namespace=devmesh \
  --from-literal=database-host=postgres.example.com \
  --from-literal=database-user=devmesh \
  --from-literal=database-password=your-password \
  --from-literal=redis-addr=redis.example.com:6379 \
  --from-literal=redis-password=your-redis-password \
  --from-literal=aws-access-key-id=AKIA... \
  --from-literal=aws-secret-access-key=your-secret \
  --from-literal=github-token=ghp_your_token
```

#### 3. Deploy

```bash
# Apply all manifests
kubectl apply -f apps/rag-loader/k8s/

# Or individually
kubectl apply -f apps/rag-loader/k8s/configmap.yaml
kubectl apply -f apps/rag-loader/k8s/deployment.yaml

# Check status
kubectl get pods -n devmesh -l app=rag-loader
kubectl get svc -n devmesh rag-loader
```

#### 4. Verify Deployment

```bash
# Check pod logs
kubectl logs -n devmesh -l app=rag-loader --tail=50

# Port forward for testing
kubectl port-forward -n devmesh svc/rag-loader 8084:8084 9094:9094

# Test health endpoint
curl http://localhost:9094/health

# Test metrics
curl http://localhost:9094/metrics
```

### Docker Compose (Local Development)

```yaml
# docker-compose.local.yml
services:
  rag-loader:
    build:
      context: .
      dockerfile: apps/rag-loader/Dockerfile
    environment:
      - DATABASE_HOST=postgres
      - DATABASE_NAME=devmesh_development
      - DATABASE_USER=devmesh
      - DATABASE_PASSWORD=devmesh
      - REDIS_ADDR=redis:6379
      - GITHUB_TOKEN=${GITHUB_TOKEN}
      - AWS_REGION=us-east-1
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
    ports:
      - "8084:8084"  # API
      - "9094:9094"  # Metrics
    depends_on:
      - postgres
      - redis
    volumes:
      - ./configs:/app/configs:ro
```

Run:

```bash
docker-compose -f docker-compose.local.yml up rag-loader
```

## Monitoring and Operations

### Health Checks

```bash
# Health endpoint
curl http://localhost:9094/health
# Returns: healthy

# Readiness endpoint
curl http://localhost:9094/ready
# Returns: ready

# Metrics endpoint (Prometheus format)
curl http://localhost:9094/metrics
```

### Key Metrics

**Ingestion Metrics:**
```
rag_loader_documents_processed_total{source="github"} 1234
rag_loader_chunks_created_total{source="github"} 5678
rag_loader_embeddings_generated_total{model="titan-v2"} 5678
```

**Performance Metrics:**
```
rag_loader_ingestion_duration_seconds{source="github",quantile="0.95"} 45.2
rag_loader_cache_hit_ratio{type="document"} 0.85
```

**Quality Metrics:**
```
rag_loader_chunk_quality_score{source="github"} 0.92
rag_loader_deduplication_rate{source="github"} 0.15
```

**Cost Tracking:**
```
rag_loader_embedding_cost_total{model="titan-v2"} 12.45
rag_loader_tokens_processed_total{model="titan-v2"} 1234567
```

### Grafana Dashboard

Import the provided dashboard:

```bash
# Dashboard JSON location (if exists)
# grafana/dashboards/rag-loader.json
```

Or create custom panels:
- Documents processed over time
- Embedding generation rate
- Cache hit ratio
- Cost tracking
- Error rate
- Job duration

### Alerting

**Recommended Alerts:**

1. **High Error Rate**
   ```promql
   rate(rag_loader_ingestion_errors_total[5m]) > 0.1
   ```

2. **Job Failures**
   ```promql
   rag_loader_jobs_failed_total > 0
   ```

3. **High Costs**
   ```promql
   rate(rag_loader_embedding_cost_total[1h]) > 10
   ```

4. **Circuit Breaker Open**
   ```promql
   rag_loader_circuit_breaker_state{state="open"} == 1
   ```

### Troubleshooting

#### Logs Not Showing Data Ingestion

```bash
# Check if scheduler is running
kubectl logs -n devmesh -l app=rag-loader | grep -i scheduler

# Check if sources are enabled
kubectl get cm -n devmesh rag-loader-config -o yaml | grep enabled

# Manually trigger a job (if API enabled)
curl -X POST http://localhost:8084/api/v1/rag/sources/my_repo/sync \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

#### GitHub Authentication Errors

```bash
# Verify token
echo $GITHUB_TOKEN | cut -c1-10
# Should show: ghp_...

# Test token directly
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/user

# Check logs for auth errors
kubectl logs -n devmesh -l app=rag-loader | grep -i "auth\|401\|403"
```

#### High Memory Usage

```bash
# Check memory limits
kubectl describe pod -n devmesh -l app=rag-loader | grep -i memory

# Adjust in deployment:
# resources.limits.memory: "2Gi"

# Reduce batch size in config
# processing.embedding.batch_size: 50
```

#### Slow Ingestion

```bash
# Check metrics
curl http://localhost:9094/metrics | grep duration

# Increase concurrency
# scheduler.max_concurrent_jobs: 5

# Enable caching
# cache.enabled: true

# Increase batch size
# processing.embedding.batch_size: 200
```

## Best Practices

### 1. Schedule Configuration

- **High-priority repos**: Every 4 hours
- **Documentation**: Every 12-24 hours
- **Infrastructure**: Daily
- **Archives**: Weekly or disable

### 2. Cost Optimization

- Use `amazon.titan-embed-text-v2:0` (cheaper)
- Enable caching
- Filter out test files
- Set max file size limits
- Monitor costs via metrics

### 3. Performance

- Configure appropriate batch sizes
- Use rate limiting to avoid API throttling
- Enable circuit breakers
- Set reasonable timeouts
- Monitor queue depths

### 4. Security

- Rotate GitHub tokens every 90 days
- Use Kubernetes secrets (never commit tokens)
- Enable SSL for database connections
- Restrict network access
- Monitor for unauthorized access

### 5. Reliability

- Configure health checks
- Set appropriate resource limits
- Enable retry logic
- Monitor error rates
- Set up alerting

## Next Steps

1. Configure your first data source (GitHub recommended)
2. Test with a small repository
3. Monitor metrics and costs
4. Scale to additional sources
5. Set up alerting
6. Configure backups

## Resources

- [RAG Loader Implementation Guide](../rag-loader-implementation-guide.md)
- [GitHub API Documentation](https://docs.github.com/en/rest)
- [AWS Bedrock Pricing](https://aws.amazon.com/bedrock/pricing/)
- [pgvector Documentation](https://github.com/pgvector/pgvector)

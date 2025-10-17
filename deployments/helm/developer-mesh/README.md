# Developer Mesh Helm Chart

Production-ready Helm chart for deploying the Developer Mesh AI Agent Orchestration Platform on Kubernetes.

## Overview

Developer Mesh is a comprehensive platform for orchestrating multiple AI agents in DevOps workflows. This Helm chart deploys all platform components:

- **REST API**: Dynamic tools integration and management
- **Edge MCP**: WebSocket server for real-time agent communication
- **Worker**: Async webhook and event processing
- **RAG Loader**: Document indexing and semantic search
- **PostgreSQL** (optional): Database with pgvector extension
- **Redis** (optional): Caching and message queuing

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+
- PV provisioner support (for persistent volumes)
- (Optional) Prometheus Operator for monitoring
- (Optional) cert-manager for TLS certificates
- (Optional) Ingress controller (nginx recommended)

## Quick Start

### Development Deployment

```bash
# Install with embedded databases (development)
helm install developer-mesh ./developer-mesh \
  --namespace developer-mesh \
  --create-namespace \
  --values values-dev.yaml \
  --set global.aws.accessKeyId=YOUR_AWS_KEY \
  --set global.aws.secretAccessKey=YOUR_AWS_SECRET

# Port forward to access services
kubectl port-forward -n developer-mesh svc/developer-mesh-rest-api 8080:8080
kubectl port-forward -n developer-mesh svc/developer-mesh-edge-mcp 8082:8082
```

### Production Deployment

```bash
# Create secrets first
kubectl create namespace developer-mesh

# Database credentials
kubectl create secret generic database-credentials \
  --namespace developer-mesh \
  --from-literal=password=YOUR_DB_PASSWORD

# Redis credentials
kubectl create secret generic redis-credentials \
  --namespace developer-mesh \
  --from-literal=password=YOUR_REDIS_PASSWORD

# Application secrets
kubectl create secret generic rest-api-secrets \
  --namespace developer-mesh \
  --from-literal=admin-api-key=YOUR_ADMIN_KEY \
  --from-literal=reader-api-key=YOUR_READER_KEY \
  --from-literal=mcp-api-key=YOUR_MCP_KEY \
  --from-literal=github-token=YOUR_GITHUB_TOKEN \
  --from-literal=github-webhook-secret=YOUR_WEBHOOK_SECRET

# Encryption and JWT secrets
kubectl create secret generic developer-mesh-encryption-secret \
  --namespace developer-mesh \
  --from-literal=master-key=YOUR_32_CHAR_ENCRYPTION_KEY

kubectl create secret generic developer-mesh-jwt-secret \
  --namespace developer-mesh \
  --from-literal=jwt-secret=YOUR_JWT_SECRET

# Install with production values
helm install developer-mesh ./developer-mesh \
  --namespace developer-mesh \
  --values values-prod.yaml \
  --set global.database.host=your-rds-endpoint.region.rds.amazonaws.com \
  --set global.redis.host=your-elasticache-endpoint.cache.amazonaws.com
```

## Configuration

### Global Configuration

All services share common configuration through `global` values:

```yaml
global:
  environment: production

  database:
    embedded: false  # Use external database
    host: postgres.example.com
    port: 5432
    name: devmesh_production
    username: devmesh
    existingSecret: "database-credentials"

  redis:
    embedded: false  # Use external Redis
    host: redis.example.com
    port: 6379

  aws:
    region: us-east-1
    useIRSA: true  # Use IAM Roles for Service Accounts
    roleArn: "arn:aws:iam::ACCOUNT:role/developer-mesh"

  security:
    existingSecret: "encryption-secret"
    jwt:
      existingSecret: "jwt-secret"
```

### Service-Specific Configuration

Each service can be configured independently:

```yaml
rest-api:
  enabled: true
  replicaCount: 3

  config:
    logLevel: info
    github:
      enabled: true
      owner: "your-org"
      repo: "your-repo"

  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: 2000m
      memory: 2Gi

  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 20
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Ingress Controller                    │
│              (nginx, Traefik, or ALB)                    │
└─────────────┬───────────────────────────┬────────────────┘
              │                           │
              ▼                           ▼
┌──────────────────────┐      ┌───────────────────────┐
│      REST API        │      │      RAG Loader       │
│   (3+ replicas)      │      │    (2+ replicas)      │
└──────────┬───────────┘      └───────────┬───────────┘
           │                               │
           │         ┌─────────────────┐   │
           ├────────►│   Edge MCP      │◄──┤
           │         │  (5+ replicas)  │   │
           │         └─────────────────┘   │
           │                               │
           ▼                               ▼
┌──────────────────────┐      ┌───────────────────────┐
│      Worker          │      │    PostgreSQL         │
│   (3+ replicas)      │◄────►│   with pgvector       │
└──────────┬───────────┘      └───────────────────────┘
           │
           ▼
┌──────────────────────┐
│      Redis           │
│  (Standalone/HA)     │
└──────────────────────┘
```

## Deployment Scenarios

### Scenario 1: Development (Local/Minikube)

Use embedded PostgreSQL and Redis:

```bash
helm install developer-mesh ./developer-mesh \
  -f values-dev.yaml \
  --set postgresql.enabled=true \
  --set redis.enabled=true
```

### Scenario 2: Staging (EKS with RDS)

Use managed AWS services:

```bash
helm install developer-mesh ./developer-mesh \
  -f values-prod.yaml \
  --set global.environment=staging \
  --set global.database.host=staging-rds.region.rds.amazonaws.com \
  --set global.redis.host=staging-elasticache.region.cache.amazonaws.com \
  --set global.aws.useIRSA=true \
  --set global.aws.roleArn=arn:aws:iam::ACCOUNT:role/staging-role
```

### Scenario 3: Production (Multi-Region)

Deploy with full HA configuration:

```bash
helm install developer-mesh ./developer-mesh \
  -f values-prod.yaml \
  --set rest-api.autoscaling.maxReplicas=50 \
  --set edge-mcp.autoscaling.maxReplicas=50 \
  --set worker.autoscaling.maxReplicas=30
```

## Monitoring

### Prometheus Integration

The chart includes ServiceMonitor resources for Prometheus Operator:

```yaml
global:
  monitoring:
    enabled: true
    prometheusLabel: prometheus
    interval: 30s
```

### Grafana Dashboards

Import provided dashboards from `monitoring/dashboards/`:
- `developer-mesh-overview.json` - System overview
- `rest-api-metrics.json` - REST API metrics
- `edge-mcp-metrics.json` - Edge MCP metrics
- `worker-metrics.json` - Worker metrics

### Metrics Endpoints

- REST API: `http://<service>:8080/metrics`
- Edge MCP: `http://<service>:8082/metrics`
- RAG Loader: `http://<service>:9094/metrics`

## Security

### Network Policies

Enable network policies for production:

```yaml
global:
  networkPolicy:
    enabled: true
    type: NetworkPolicy
```

### Pod Security

All pods run with:
- Non-root user (UID 1000)
- Read-only root filesystem
- Dropped capabilities
- seccomp profile (RuntimeDefault)

### Secrets Management

**Recommended approaches:**

1. **External Secrets Operator** (production):
```yaml
# Install External Secrets Operator first
# Then use SecretStore and ExternalSecret resources
```

2. **Sealed Secrets** (GitOps):
```bash
kubeseal --format=yaml < secret.yaml > sealed-secret.yaml
```

3. **Helm Secrets** (development):
```bash
helm secrets install developer-mesh ./developer-mesh -f secrets.yaml
```

## Maintenance

### Upgrading

```bash
# Update dependencies
helm dependency update ./developer-mesh

# Upgrade release
helm upgrade developer-mesh ./developer-mesh \
  --namespace developer-mesh \
  -f values-prod.yaml \
  --set image.tag=v1.1.0
```

### Database Migrations

Migrations run automatically via pre-install/pre-upgrade hooks:

```yaml
migration:
  enabled: true  # Disable if managing migrations externally
```

### Backup and Recovery

**Database backups:**
```bash
# Automated via AWS RDS snapshots
# Or manual with pg_dump
kubectl exec -n developer-mesh deployment/developer-mesh-rest-api -- \
  pg_dump -h $DB_HOST -U $DB_USER $DB_NAME > backup.sql
```

**Redis backups:**
```bash
# Automated via ElastiCache snapshots
# Or manual with BGSAVE
kubectl exec -n developer-mesh deployment/redis -- redis-cli BGSAVE
```

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -n developer-mesh
kubectl describe pod -n developer-mesh <pod-name>
kubectl logs -n developer-mesh <pod-name> --previous
```

### Check Service Health

```bash
# REST API health
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://developer-mesh-rest-api.developer-mesh:8080/health

# Edge MCP health
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://developer-mesh-edge-mcp.developer-mesh:8082/health/live
```

### Common Issues

**Pods not starting:**
```bash
# Check init containers
kubectl logs -n developer-mesh <pod-name> -c wait-for-database

# Check resource constraints
kubectl describe nodes | grep -A 5 "Allocated resources"
```

**Database connection issues:**
```bash
# Test connectivity
kubectl run -it --rm debug --image=postgres:17 --restart=Never -- \
  psql -h $DB_HOST -U $DB_USER -d $DB_NAME
```

**High memory usage:**
```bash
# Check memory metrics
kubectl top pods -n developer-mesh --containers

# Adjust resources
helm upgrade developer-mesh ./developer-mesh \
  --set rest-api.resources.limits.memory=4Gi
```

## Values Reference

### Complete Values Structure

See [`values.yaml`](./values.yaml) for all available options.

### Key Configuration Sections

| Section | Description | Required |
|---------|-------------|----------|
| `global.database` | Database connection settings | Yes |
| `global.redis` | Redis connection settings | Yes |
| `global.aws` | AWS credentials and S3 config | Yes |
| `global.security` | Encryption and JWT settings | Yes |
| `rest-api.config` | REST API configuration | Yes |
| `rest-api.secrets` | API keys and tokens | Yes |
| `edge-mcp.config` | Edge MCP configuration | Yes |
| `worker.config` | Worker settings | No |
| `rag-loader.config` | RAG loader settings | No |

## Advanced Topics

### Multi-Tenant Setup

Configure tenant isolation:
```yaml
rest-api:
  config:
    features:
      multiTenantEnabled: true
```

### Custom Image Registry

Use private registry:
```yaml
global:
  imageRegistry: "your-registry.example.com"
  imagePullSecrets:
    - name: regcred
```

### High Availability

For critical workloads:
```yaml
rest-api:
  replicaCount: 5
  autoscaling:
    minReplicas: 5
    maxReplicas: 50
  podDisruptionBudget:
    minAvailable: 3

# Use multi-AZ database
global:
  database:
    host: "multi-az-rds.region.rds.amazonaws.com"
```

## Support

- Documentation: https://docs.developer-mesh.com
- Issues: https://github.com/developer-mesh/developer-mesh/issues
- Slack: https://developer-mesh.slack.com

## License

See [LICENSE](../../../LICENSE) file.

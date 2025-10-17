# Developer Mesh Deployment Guide

Comprehensive guide for deploying Developer Mesh on Kubernetes using Helm.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Prerequisites](#prerequisites)
3. [Pre-Deployment Checklist](#pre-deployment-checklist)
4. [Deployment Steps](#deployment-steps)
5. [Post-Deployment Validation](#post-deployment-validation)
6. [Configuration Management](#configuration-management)
7. [Security Best Practices](#security-best-practices)
8. [Monitoring and Alerting](#monitoring-and-alerting)
9. [Troubleshooting](#troubleshooting)

## Architecture Overview

### Component Stack

```
Application Layer:
├── REST API (8080)          # Dynamic tools API & orchestration
├── Edge MCP (8082)          # WebSocket MCP server
├── Worker (background)      # Async webhook processing
└── RAG Loader (8084)        # Document indexing & search

Infrastructure Layer:
├── PostgreSQL + pgvector    # Primary data store
├── Redis                    # Cache & message queue
├── S3 / Compatible          # Context storage
└── AWS Bedrock              # Embedding models
```

### Network Flow

```
┌─────────────┐
│   Internet  │
└──────┬──────┘
       │
       ▼
┌──────────────────┐
│ Ingress/ALB      │  ← TLS termination, rate limiting
└──────┬──────────┘
       │
       ├─────────┬──────────────┬────────────┐
       ▼         ▼              ▼            ▼
  ┌────────┐ ┌─────────┐  ┌────────┐  ┌──────────┐
  │REST API│ │Edge MCP │  │RAG Ldr │  │ Worker   │
  └────┬───┘ └────┬────┘  └───┬────┘  └────┬─────┘
       │          │            │            │
       └──────────┴────────────┴────────────┘
                       │
         ┌─────────────┴──────────────┐
         ▼                            ▼
    ┌──────────┐              ┌────────────┐
    │PostgreSQL│              │   Redis    │
    └──────────┘              └────────────┘
```

## Prerequisites

### Required Tools

```bash
# Kubernetes cluster (1.25+)
kubectl version --client

# Helm 3.8+
helm version

# (Optional) kubectl plugins
kubectl krew install neat
kubectl krew install tree
```

### Cloud Provider Setup

#### AWS (EKS)

```bash
# Create EKS cluster
eksctl create cluster \
  --name developer-mesh-prod \
  --region us-east-1 \
  --node-type t3.xlarge \
  --nodes 3 \
  --nodes-min 3 \
  --nodes-max 10 \
  --managed

# Create RDS PostgreSQL instance
aws rds create-db-instance \
  --db-instance-identifier developer-mesh-prod-db \
  --db-instance-class db.r6g.xlarge \
  --engine postgres \
  --engine-version 17.2 \
  --master-username devmesh \
  --master-user-password 'SECURE_PASSWORD' \
  --allocated-storage 100 \
  --vpc-security-group-ids sg-xxxxx \
  --db-subnet-group-name developer-mesh-subnet-group \
  --backup-retention-period 30 \
  --multi-az

# Create ElastiCache Redis cluster
aws elasticache create-replication-group \
  --replication-group-id developer-mesh-prod-redis \
  --replication-group-description "Developer Mesh Redis" \
  --engine redis \
  --cache-node-type cache.r6g.large \
  --num-cache-clusters 2 \
  --cache-subnet-group-name developer-mesh-subnet-group \
  --security-group-ids sg-xxxxx \
  --automatic-failover-enabled

# Create S3 bucket
aws s3 mb s3://developer-mesh-contexts-prod
aws s3api put-bucket-encryption \
  --bucket developer-mesh-contexts-prod \
  --server-side-encryption-configuration '{
    "Rules": [{
      "ApplyServerSideEncryptionByDefault": {
        "SSEAlgorithm": "AES256"
      }
    }]
  }'

# Create IAM role for IRSA
eksctl create iamserviceaccount \
  --name developer-mesh-sa \
  --namespace developer-mesh \
  --cluster developer-mesh-prod \
  --attach-policy-arn arn:aws:iam::ACCOUNT:policy/DeveloperMeshPolicy \
  --approve
```

#### GCP (GKE)

```bash
# Create GKE cluster
gcloud container clusters create developer-mesh-prod \
  --region us-central1 \
  --machine-type n2-standard-4 \
  --num-nodes 3 \
  --enable-autoscaling \
  --min-nodes 3 \
  --max-nodes 10

# Create Cloud SQL PostgreSQL
gcloud sql instances create developer-mesh-prod-db \
  --database-version POSTGRES_17 \
  --tier db-custom-4-16384 \
  --region us-central1 \
  --availability-type REGIONAL \
  --backup-start-time 02:00

# Create Memorystore Redis
gcloud redis instances create developer-mesh-prod-redis \
  --size 5 \
  --region us-central1 \
  --tier standard \
  --redis-version redis_7_0
```

### Install Required Operators

```bash
# cert-manager (for TLS)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Prometheus Operator (for monitoring)
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace

# (Optional) External Secrets Operator
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets external-secrets/external-secrets \
  --namespace external-secrets-system \
  --create-namespace
```

## Pre-Deployment Checklist

### ✅ Infrastructure Ready

- [ ] Kubernetes cluster created and accessible
- [ ] PostgreSQL database created (RDS/CloudSQL)
- [ ] Redis cluster created (ElastiCache/Memorystore)
- [ ] S3 bucket created and configured
- [ ] Network connectivity verified (VPC peering, security groups)
- [ ] DNS records configured (if using ingress)
- [ ] TLS certificates ready (cert-manager or manual)

### ✅ Secrets Prepared

- [ ] Database password
- [ ] Redis password (if auth enabled)
- [ ] JWT secret (32+ characters)
- [ ] Encryption master key (32+ characters)
- [ ] Admin API key
- [ ] Reader API key
- [ ] MCP API key
- [ ] GitHub token (if using GitHub integration)
- [ ] GitHub webhook secret
- [ ] AWS credentials (if not using IRSA)

### ✅ Configuration Values

- [ ] Database connection details
- [ ] Redis connection details
- [ ] AWS region and S3 bucket name
- [ ] Ingress hostnames
- [ ] Resource limits appropriate for workload
- [ ] Autoscaling thresholds set
- [ ] Monitoring endpoints configured

## Deployment Steps

### Step 1: Prepare Secrets

```bash
# Create namespace
kubectl create namespace developer-mesh

# Generate secure secrets
DB_PASSWORD=$(openssl rand -base64 32)
JWT_SECRET=$(openssl rand -base64 48)
ENCRYPTION_KEY=$(openssl rand -base64 32)
ADMIN_API_KEY="devmesh_$(openssl rand -hex 32)"
READER_API_KEY="devmesh_readonly_$(openssl rand -hex 32)"
MCP_API_KEY="devmesh_mcp_$(openssl rand -hex 32)"

# Create database secret
kubectl create secret generic database-credentials \
  --namespace developer-mesh \
  --from-literal=password="$DB_PASSWORD"

# Create application secrets
kubectl create secret generic rest-api-secrets \
  --namespace developer-mesh \
  --from-literal=admin-api-key="$ADMIN_API_KEY" \
  --from-literal=reader-api-key="$READER_API_KEY" \
  --from-literal=mcp-api-key="$MCP_API_KEY" \
  --from-literal=github-token="$GITHUB_TOKEN" \
  --from-literal=github-webhook-secret="$GITHUB_WEBHOOK_SECRET"

# Create security secrets
kubectl create secret generic developer-mesh-encryption-secret \
  --namespace developer-mesh \
  --from-literal=master-key="$ENCRYPTION_KEY"

kubectl create secret generic developer-mesh-jwt-secret \
  --namespace developer-mesh \
  --from-literal=jwt-secret="$JWT_SECRET"

# Save secrets securely (use password manager or vault)
cat > secrets-backup.txt <<EOF
Database Password: $DB_PASSWORD
JWT Secret: $JWT_SECRET
Encryption Key: $ENCRYPTION_KEY
Admin API Key: $ADMIN_API_KEY
Reader API Key: $READER_API_KEY
MCP API Key: $MCP_API_KEY
EOF

# Encrypt the backup
gpg -c secrets-backup.txt
rm secrets-backup.txt
```

### Step 2: Customize Values

Create `custom-values.yaml`:

```yaml
global:
  environment: production

  database:
    host: "your-rds-endpoint.us-east-1.rds.amazonaws.com"
    name: "devmesh_production"
    existingSecret: "database-credentials"

  redis:
    host: "your-elasticache-endpoint.cache.amazonaws.com"

  aws:
    region: us-east-1
    useIRSA: true
    roleArn: "arn:aws:iam::ACCOUNT_ID:role/developer-mesh-role"
    s3:
      bucket: "developer-mesh-contexts-prod"

  security:
    existingSecret: "developer-mesh-encryption-secret"
    jwt:
      existingSecret: "developer-mesh-jwt-secret"

rest-api:
  config:
    github:
      owner: "your-org"
      repo: "your-repo"

  secrets:
    create: false  # Using existing secrets

  ingress:
    enabled: true
    hosts:
      - host: api.developer-mesh.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: api-tls
        hosts:
          - api.developer-mesh.com
```

### Step 3: Deploy Helm Chart

```bash
# Update dependencies
cd deployments/helm/developer-mesh
helm dependency update

# Dry run to validate
helm install developer-mesh . \
  --namespace developer-mesh \
  --values values-prod.yaml \
  --values custom-values.yaml \
  --dry-run --debug

# Deploy
helm install developer-mesh . \
  --namespace developer-mesh \
  --values values-prod.yaml \
  --values custom-values.yaml \
  --wait \
  --timeout 10m

# Check status
helm status developer-mesh -n developer-mesh
```

### Step 4: Verify Deployment

```bash
# Check pods
kubectl get pods -n developer-mesh

# Expected output:
# NAME                                   READY   STATUS    RESTARTS   AGE
# developer-mesh-rest-api-xxx            1/1     Running   0          2m
# developer-mesh-edge-mcp-xxx            1/1     Running   0          2m
# developer-mesh-worker-xxx              1/1     Running   0          2m
# developer-mesh-rag-loader-xxx          1/1     Running   0          2m

# Check services
kubectl get svc -n developer-mesh

# Test health endpoints
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -v http://developer-mesh-rest-api.developer-mesh:8080/health
```

## Post-Deployment Validation

### Functional Testing

```bash
# 1. Test REST API
export API_KEY="your-admin-api-key"

curl -H "Authorization: Bearer $API_KEY" \
  https://api.developer-mesh.com/api/v1/health

# 2. Test Edge MCP WebSocket
websocat --header="Authorization: Bearer $API_KEY" \
  wss://api.developer-mesh.com/ws

# 3. Test RAG Loader
curl -H "Authorization: Bearer $API_KEY" \
  https://rag.developer-mesh.com/api/v1/health

# 4. Verify database connectivity
kubectl exec -n developer-mesh deployment/developer-mesh-rest-api -- \
  sh -c 'echo "SELECT version();" | psql $DATABASE_DSN'

# 5. Verify Redis connectivity
kubectl exec -n developer-mesh deployment/developer-mesh-rest-api -- \
  sh -c 'redis-cli -h $REDIS_HOST ping'
```

### Performance Testing

```bash
# Load test REST API
k6 run - <<EOF
import http from 'k6/http';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 100 },
    { duration: '5m', target: 100 },
    { duration: '2m', target: 0 },
  ],
};

export default function () {
  const res = http.get('https://api.developer-mesh.com/health');
  check(res, { 'status is 200': (r) => r.status === 200 });
}
EOF
```

## Configuration Management

### GitOps Workflow

```bash
# 1. Store values in Git
git-crypt init
git-crypt add-gpg-user YOUR_GPG_KEY
echo "custom-values.yaml filter=git-crypt diff=git-crypt" >> .gitattributes

# 2. Use ArgoCD
kubectl create namespace argocd
kubectl apply -n argocd -f \
  https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# 3. Create ArgoCD application
kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: developer-mesh
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/infrastructure
    targetRevision: HEAD
    path: helm/developer-mesh
    helm:
      valueFiles:
        - values-prod.yaml
        - custom-values.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: developer-mesh
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
EOF
```

## Security Best Practices

### 1. Use IRSA (IAM Roles for Service Accounts)

```yaml
global:
  aws:
    useIRSA: true
    roleArn: "arn:aws:iam::ACCOUNT:role/developer-mesh-role"
```

### 2. Enable Network Policies

```yaml
global:
  networkPolicy:
    enabled: true
```

### 3. Implement Pod Security Standards

```bash
kubectl label namespace developer-mesh \
  pod-security.kubernetes.io/enforce=restricted \
  pod-security.kubernetes.io/audit=restricted \
  pod-security.kubernetes.io/warn=restricted
```

### 4. Use External Secrets Operator

```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: aws-secrets-manager
  namespace: developer-mesh
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-east-1
      auth:
        jwt:
          serviceAccountRef:
            name: developer-mesh-sa
---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: database-credentials
  namespace: developer-mesh
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: database-credentials
  data:
    - secretKey: password
      remoteRef:
        key: developer-mesh/database
        property: password
```

### 5. Rotate Secrets Regularly

```bash
# Create rotation script
cat > rotate-secrets.sh <<'EOF'
#!/bin/bash
set -euo pipefail

# Generate new secrets
NEW_PASSWORD=$(openssl rand -base64 32)

# Update in secrets manager
aws secretsmanager update-secret \
  --secret-id developer-mesh/database \
  --secret-string "{\"password\":\"$NEW_PASSWORD\"}"

# Trigger ESO refresh
kubectl annotate secret database-credentials \
  -n developer-mesh \
  force-sync="$(date +%s)" \
  --overwrite

# Rolling restart
kubectl rollout restart deployment/developer-mesh-rest-api -n developer-mesh
EOF

chmod +x rotate-secrets.sh
```

## Monitoring and Alerting

### Grafana Dashboards

Import dashboards from [`monitoring/dashboards/`](./monitoring/dashboards/).

### Prometheus Alerts

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: developer-mesh-alerts
  namespace: developer-mesh
spec:
  groups:
    - name: developer-mesh
      interval: 30s
      rules:
        - alert: HighErrorRate
          expr: |
            rate(http_requests_total{status=~"5.."}[5m]) > 0.05
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "High error rate detected"

        - alert: PodMemoryUsage
          expr: |
            container_memory_usage_bytes{namespace="developer-mesh"} /
            container_spec_memory_limit_bytes{namespace="developer-mesh"} > 0.9
          for: 5m
          labels:
            severity: critical
```

## Troubleshooting

### Common Issues

See [README.md](./developer-mesh/README.md#troubleshooting) for detailed troubleshooting steps.

### Emergency Procedures

**Rollback:**
```bash
helm rollback developer-mesh -n developer-mesh
```

**Scale down:**
```bash
kubectl scale deployment --all --replicas=0 -n developer-mesh
```

**Database emergency backup:**
```bash
kubectl exec -n developer-mesh deployment/developer-mesh-rest-api -- \
  pg_dump -Fc -h $DB_HOST -U $DB_USER $DB_NAME > emergency-backup.dump
```

---

For additional support, see [README.md](./developer-mesh/README.md) or contact the platform team.

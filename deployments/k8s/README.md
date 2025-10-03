# Edge MCP Kubernetes Deployment Guide

This directory contains Kubernetes manifests and Helm charts for deploying Edge MCP to Kubernetes clusters.

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Deployment Methods](#deployment-methods)
4. [Quick Start](#quick-start)
5. [Configuration](#configuration)
6. [Production Deployment](#production-deployment)
7. [Monitoring and Operations](#monitoring-and-operations)
8. [Troubleshooting](#troubleshooting)

## Overview

Edge MCP is a WebSocket-based MCP (Model Context Protocol) server designed to facilitate communication between AI agents and development tools. This deployment guide covers:

- **Base Manifests**: Raw Kubernetes YAML files for manual deployment
- **Helm Chart**: Production-ready Helm chart with extensive configuration options
- **Autoscaling**: Horizontal Pod Autoscaler (HPA) for automatic scaling
- **High Availability**: Pod Disruption Budgets (PDB) for reliable deployments
- **Monitoring**: Prometheus integration via ServiceMonitor

## Prerequisites

### Required

- Kubernetes cluster (v1.24+)
- `kubectl` configured to access your cluster
- At least 3 worker nodes (for HA deployment)

### Optional

- **Helm 3.x** (for Helm-based deployment)
- **Prometheus Operator** (for ServiceMonitor support)
- **Metrics Server** (for HPA to function)
- **Redis** (for distributed L2 cache - recommended for production)

### Verify Prerequisites

```bash
# Check Kubernetes version
kubectl version --short

# Check if Metrics Server is installed (required for HPA)
kubectl get deployment metrics-server -n kube-system

# Check if Prometheus Operator is installed (optional)
kubectl get crd servicemonitors.monitoring.coreos.com
```

## Deployment Methods

### Method 1: Helm Chart (Recommended)

The Helm chart provides the most flexible and production-ready deployment method.

**Advantages:**
- ✅ Templated configuration with values.yaml
- ✅ Easy upgrades and rollbacks
- ✅ Conditional resource deployment
- ✅ Built-in best practices

**Location:** `helm/edge-mcp/`

### Method 2: Base Manifests

Direct kubectl apply using raw Kubernetes manifests.

**Advantages:**
- ✅ Simple and straightforward
- ✅ No additional tools required
- ✅ Full control over resources

**Location:** `base/`

### Method 3: Kustomize

Use Kustomize for environment-specific overlays.

**Advantages:**
- ✅ Native kubectl integration
- ✅ Environment-specific customization
- ✅ No templating language

**Location:** `base/kustomization.yaml`

## Quick Start

### Using Helm (Recommended)

```bash
# 1. Generate a secure API key
export EDGE_MCP_API_KEY=$(openssl rand -base64 32)

# 2. Install Edge MCP with Helm
helm install edge-mcp ./helm/edge-mcp \
  --namespace edge-mcp \
  --create-namespace \
  --set edgeMcp.auth.apiKey="$EDGE_MCP_API_KEY"

# 3. Wait for pods to be ready
kubectl wait --for=condition=ready pod \
  -l app=edge-mcp \
  -n edge-mcp \
  --timeout=300s

# 4. Port-forward to access Edge MCP
kubectl port-forward -n edge-mcp svc/edge-mcp 8082:8082

# 5. Connect to Edge MCP
# WebSocket endpoint: ws://localhost:8082/ws
# Use the API key from step 1 for authentication
```

### Using kubectl with Base Manifests

```bash
# 1. Edit secret.yaml and replace REPLACE_WITH_ACTUAL_API_KEY
# Generate with: openssl rand -base64 32
vim base/secret.yaml

# 2. Apply all manifests
kubectl apply -f base/

# 3. Wait for deployment
kubectl rollout status deployment/edge-mcp -n edge-mcp

# 4. Port-forward
kubectl port-forward -n edge-mcp svc/edge-mcp 8082:8082
```

### Using Kustomize

```bash
# 1. Edit base/secret.yaml with your API key
vim base/secret.yaml

# 2. Apply with kustomize
kubectl apply -k base/

# 3. Wait for deployment
kubectl rollout status deployment/edge-mcp -n edge-mcp
```

## Configuration

### Helm Configuration

All configuration is done via `values.yaml`. Key sections:

#### Authentication

```yaml
edgeMcp:
  auth:
    apiKey: "your-secure-api-key-here"
```

#### Core Platform Integration (Optional)

```yaml
edgeMcp:
  corePlatform:
    enabled: true
    url: "https://core-platform.example.com"
    apiKey: "core-platform-api-key"
```

#### Redis Cache Configuration

```yaml
edgeMcp:
  cache:
    redis:
      enabled: true  # Enable distributed L2 cache

redis:
  enabled: true  # Deploy Redis alongside Edge MCP
  master:
    persistence:
      enabled: true
      size: 10Gi
```

#### Rate Limiting

```yaml
edgeMcp:
  rateLimit:
    globalRps: 1000      # Global requests per second
    tenantRps: 100       # Per-tenant requests per second
    toolRps: 50          # Per-tool requests per second
    enableQuotas: true   # Enable quota management
```

#### Resource Limits

```yaml
edgeMcp:
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 512Mi
```

#### Autoscaling

```yaml
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80
```

### Environment Variables

All configuration can be overridden via environment variables:

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `EDGE_MCP_API_KEY` | API key for authentication | - |
| `DEV_MESH_URL` | Core Platform URL | - |
| `DEV_MESH_API_KEY` | Core Platform API key | - |
| `EDGE_MCP_REDIS_ENABLED` | Enable Redis cache | `false` |
| `EDGE_MCP_REDIS_URL` | Redis connection URL | `redis://redis:6379/0` |
| `EDGE_MCP_GLOBAL_RPS` | Global rate limit | `1000` |
| `EDGE_MCP_TENANT_RPS` | Per-tenant rate limit | `100` |
| `TRACING_ENABLED` | Enable distributed tracing | `false` |
| `OTLP_ENDPOINT` | OTLP collector endpoint | `localhost:4317` |

## Production Deployment

### 1. Prepare Production Values

Create a production values file:

```bash
cat > values-production.yaml <<EOF
edgeMcp:
  image:
    repository: your-registry.example.com/edge-mcp
    tag: "1.0.0"
    pullPolicy: IfNotPresent

  replicaCount: 5

  auth:
    apiKey: "\${EDGE_MCP_API_KEY}"

  corePlatform:
    enabled: true
    url: "https://core-platform.production.example.com"
    apiKey: "\${CORE_PLATFORM_API_KEY}"

  cache:
    redis:
      enabled: true

  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 2000m
      memory: 1Gi

autoscaling:
  enabled: true
  minReplicas: 5
  maxReplicas: 20
  targetCPUUtilizationPercentage: 60
  targetMemoryUtilizationPercentage: 70

redis:
  enabled: true
  master:
    persistence:
      enabled: true
      size: 50Gi
      storageClass: fast-ssd
    resources:
      requests:
        cpu: 500m
        memory: 1Gi
      limits:
        cpu: 2000m
        memory: 2Gi

serviceMonitor:
  enabled: true
  prometheusLabel: kube-prometheus

podDisruptionBudget:
  enabled: true
  minAvailable: 3
EOF
```

### 2. Deploy to Production

```bash
# Set secrets via environment variables
export EDGE_MCP_API_KEY=$(openssl rand -base64 32)
export CORE_PLATFORM_API_KEY="your-core-platform-key"

# Install with production values
helm install edge-mcp ./helm/edge-mcp \
  --namespace edge-mcp \
  --create-namespace \
  --values values-production.yaml \
  --set edgeMcp.auth.apiKey="$EDGE_MCP_API_KEY" \
  --set edgeMcp.corePlatform.apiKey="$CORE_PLATFORM_API_KEY"
```

### 3. Verify Deployment

```bash
# Check pod status
kubectl get pods -n edge-mcp

# Check HPA status
kubectl get hpa -n edge-mcp

# Check PDB status
kubectl get pdb -n edge-mcp

# Check service endpoints
kubectl get svc -n edge-mcp

# View logs
kubectl logs -n edge-mcp -l app=edge-mcp --tail=100

# Check health endpoints
kubectl run curl --image=curlimages/curl -it --rm -- \
  curl http://edge-mcp.edge-mcp.svc.cluster.local:8082/health/ready
```

## Monitoring and Operations

### Health Checks

Edge MCP provides three health check endpoints:

- **Liveness**: `/health/live` - Basic service health
- **Readiness**: `/health/ready` - Service ready to accept traffic
- **Startup**: `/health/startup` - One-time startup validation

### Prometheus Metrics

Metrics are exposed at `/metrics`:

- **Tool Execution**: `edge_mcp_tool_execution_duration_seconds`
- **Active Connections**: `edge_mcp_active_connections`
- **Error Rate**: `edge_mcp_errors_total`
- **Cache Performance**: `edge_mcp_cache_hits_total`, `edge_mcp_cache_misses_total`
- **Request Rate**: `edge_mcp_requests_total`

### Viewing Metrics

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n edge-mcp svc/edge-mcp 8082:8082

# Fetch metrics
curl http://localhost:8082/metrics
```

### Scaling

```bash
# Manual scaling (if HPA is disabled)
kubectl scale deployment edge-mcp -n edge-mcp --replicas=5

# View HPA status
kubectl get hpa edge-mcp -n edge-mcp

# Describe HPA for details
kubectl describe hpa edge-mcp -n edge-mcp
```

### Upgrading

```bash
# Using Helm
helm upgrade edge-mcp ./helm/edge-mcp \
  --namespace edge-mcp \
  --values values-production.yaml \
  --set edgeMcp.image.tag="1.1.0"

# Using kubectl (base manifests)
kubectl set image deployment/edge-mcp \
  edge-mcp=edge-mcp:1.1.0 \
  -n edge-mcp

# Rollback if needed
helm rollback edge-mcp -n edge-mcp
```

### Draining and Maintenance

```bash
# Cordon a node (prevent new pods)
kubectl cordon <node-name>

# Drain a node (respects PDB)
kubectl drain <node-name> --ignore-daemonsets

# Uncordon when done
kubectl uncordon <node-name>
```

## Troubleshooting

### Pods Not Starting

```bash
# Check pod events
kubectl describe pod -n edge-mcp -l app=edge-mcp

# Check logs
kubectl logs -n edge-mcp -l app=edge-mcp --tail=100

# Common issues:
# - Missing API key in secret
# - Image pull errors (check imagePullSecrets)
# - Resource constraints (check node capacity)
```

### HPA Not Scaling

```bash
# Check metrics-server is running
kubectl get deployment metrics-server -n kube-system

# Check HPA can get metrics
kubectl get hpa edge-mcp -n edge-mcp -o yaml

# View HPA events
kubectl describe hpa edge-mcp -n edge-mcp

# Common issues:
# - Metrics server not installed
# - Resource requests not set
# - Metrics not available yet (wait 1-2 minutes)
```

### Redis Connection Issues

```bash
# Check Redis pod status
kubectl get pods -n edge-mcp -l app.kubernetes.io/name=redis

# Check Redis logs
kubectl logs -n edge-mcp -l app.kubernetes.io/name=redis

# Test Redis connectivity from Edge MCP pod
kubectl exec -n edge-mcp -it deployment/edge-mcp -- \
  sh -c "apk add redis && redis-cli -h redis ping"

# Common issues:
# - Redis pod not running
# - Network policies blocking traffic
# - Incorrect Redis URL in configuration
```

### ServiceMonitor Not Working

```bash
# Check if Prometheus Operator is installed
kubectl get crd servicemonitors.monitoring.coreos.com

# Check ServiceMonitor status
kubectl get servicemonitor -n edge-mcp

# Check if Prometheus is scraping
kubectl logs -n monitoring prometheus-<pod-name> | grep edge-mcp

# Common issues:
# - Prometheus Operator not installed
# - Label selector mismatch
# - Service port name mismatch
```

### Connection Refused

```bash
# Check service exists
kubectl get svc edge-mcp -n edge-mcp

# Check endpoints are populated
kubectl get endpoints edge-mcp -n edge-mcp

# Test connectivity
kubectl run curl --image=curlimages/curl -it --rm -- \
  curl -v http://edge-mcp.edge-mcp.svc.cluster.local:8082/health/live

# Common issues:
# - Service selector doesn't match pod labels
# - Pods not ready (check readiness probe)
# - Network policies blocking traffic
```

## Advanced Topics

### Custom Metrics Autoscaling

To scale based on custom metrics (e.g., active connections):

1. Install Prometheus Adapter
2. Configure custom metrics
3. Update HPA:

```yaml
autoscaling:
  customMetrics:
  - type: Pods
    pods:
      metric:
        name: edge_mcp_active_connections
      target:
        type: AverageValue
        averageValue: "100"
```

### Multi-Region Deployment

For multi-region deployments:

1. Deploy Edge MCP to each region
2. Use shared Redis cluster for L2 cache
3. Configure load balancer for failover

### Disaster Recovery

```bash
# Backup Helm values
helm get values edge-mcp -n edge-mcp > backup-values.yaml

# Backup Redis data (if using persistent storage)
kubectl exec -n edge-mcp redis-0 -- \
  redis-cli bgsave

# Export PVC
kubectl get pvc -n edge-mcp -o yaml > backup-pvc.yaml
```

## Security Considerations

1. **API Keys**: Always use strong, randomly generated API keys
2. **Network Policies**: Implement network policies to restrict traffic
3. **RBAC**: Use least-privilege service accounts
4. **Image Security**: Scan images for vulnerabilities
5. **Secrets Management**: Consider using external secret managers (e.g., Vault)

## Support

For issues and questions:

- **GitHub Issues**: https://github.com/developer-mesh/developer-mesh/issues
- **Documentation**: https://github.com/developer-mesh/developer-mesh/tree/main/docs
- **Community**: https://discord.gg/developer-mesh

## License

See LICENSE file in the repository root.

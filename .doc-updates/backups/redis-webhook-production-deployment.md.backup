# Redis Webhook System - Production Deployment Guide

## Overview

This guide covers the production deployment of the Redis-based webhook processing system that replaces AWS SQS. The system provides cloud-agnostic webhook event processing with advanced AI capabilities for context management.

## Architecture Summary

### Core Components

1. **Redis Streams** - Message queue replacement for SQS
2. **Webhook Consumer Service** - Distributed event processing
3. **Context Lifecycle Manager** - Three-tier storage (hot/warm/cold)
4. **AI Intelligence Layer** - Embeddings, summarization, and predictive pre-warming
5. **Security Layer** - Encryption, rate limiting, and audit logging

### Data Flow

```
Webhook → Security Validation → Deduplication → Redis Stream → Consumer Workers
                                                                    ↓
                                                          AI Processing
                                                                    ↓
                                                      Context Storage (Hot/Warm/Cold)
```

## Prerequisites

### System Requirements

- **Redis**: 6.2+ (required for Streams features)
- **Go**: 1.21+
- **S3-compatible storage**: For cold tier storage
- **PostgreSQL**: 13+ (for existing DevOps MCP data)

### Infrastructure Requirements

- **Redis Cluster**: 
  - Minimum 3 nodes for HA
  - Redis Sentinel for automatic failover
  - TLS enabled
  - 16GB+ RAM per node recommended

- **Application Servers**:
  - Minimum 3 instances for HA
  - 4 CPU cores, 8GB RAM per instance
  - Auto-scaling based on queue depth

- **Monitoring**:
  - Prometheus for metrics
  - OpenTelemetry collector for traces
  - Centralized logging (ELK/CloudWatch)

## Configuration

### Environment Variables

```bash
# Redis Configuration
REDIS_ADDRESSES=redis-node1:6379,redis-node2:6379,redis-node3:6379
REDIS_SENTINEL_ENABLED=true
REDIS_MASTER_NAME=webhook-master
REDIS_PASSWORD=${REDIS_PASSWORD}
REDIS_TLS_ENABLED=true
REDIS_TLS_CERT_FILE=/etc/redis/certs/client.crt
REDIS_TLS_KEY_FILE=/etc/redis/certs/client.key
REDIS_TLS_CA_FILE=/etc/redis/certs/ca.crt

# Security Configuration
WEBHOOK_SIGNATURE_VERIFICATION=true
WEBHOOK_RATE_LIMITING=true
WEBHOOK_ENCRYPTION_ENABLED=true
ENCRYPTION_MASTER_KEY=${ENCRYPTION_MASTER_KEY}  # 32+ character key

# AI Services
AI_EMBEDDING_PROVIDER=openai
AI_EMBEDDING_MODEL=text-embedding-3-small
AI_SUMMARIZATION_MODEL=gpt-4-turbo
OPENAI_API_KEY=${OPENAI_API_KEY}

# S3 Cold Storage
S3_ENDPOINT=https://s3.amazonaws.com
S3_BUCKET=devops-mcp-webhook-contexts
S3_REGION=us-east-1
AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}

# Observability
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
OTEL_SERVICE_NAME=webhook-processor
OTEL_TRACES_SAMPLER_ARG=0.1
PROMETHEUS_PUSHGATEWAY=http://prometheus-pushgateway:9091
```

### Application Configuration (config.yaml)

```yaml
webhook:
  redis:
    addresses:
      - ${REDIS_NODE1}
      - ${REDIS_NODE2}
      - ${REDIS_NODE3}
    sentinel_enabled: true
    master_name: webhook-master
    password: ${REDIS_PASSWORD}
    tls_enabled: true
    pool_size: 100
    min_idle_conns: 10
    max_conn_age: 1h
    
  consumer:
    consumer_group: webhook-processors
    num_workers: 10
    batch_size: 100
    block_timeout: 5s
    max_retries: 3
    dead_letter_stream: webhook-events-dlq
    
  deduplication:
    window_configs:
      github:
        duration: 5m
        max_size: 1
      jira:
        duration: 10m
        max_size: 3
    bloom_filter_size: 10000000
    bloom_filter_hash_funcs: 4
    
  context_lifecycle:
    hot_duration: 30m
    warm_duration: 24h
    hot_importance_threshold: 0.8
    warm_importance_threshold: 0.5
    compression_type: zstd
    compression_level: 6
    
  security:
    enable_signature_verification: true
    signature_header: X-Webhook-Signature
    timestamp_tolerance: 5m
    enable_rate_limiting: true
    global_rate_limit: 10000  # RPS
    per_tool_rate_limits:
      github: 5000
      jira: 2000
    max_payload_size: 10485760  # 10MB
    enable_audit_logging: true
    
  ai:
    embedding:
      provider: ${AI_EMBEDDING_PROVIDER}
      model: ${AI_EMBEDDING_MODEL}
      dimensions: 1536
      batch_size: 100
      cache_duration: 24h
    summarization:
      model: ${AI_SUMMARIZATION_MODEL}
      max_chunk_size: 4000
      cache_duration: 168h  # 7 days
    prewarming:
      max_predictions: 10
      confidence_threshold: 0.7
      lookback_window: 15m
      
observability:
  metrics:
    enabled: true
    type: prometheus
    push_gateway: ${PROMETHEUS_PUSHGATEWAY}
    push_interval: 30s
  tracing:
    enabled: true
    endpoint: ${OTEL_EXPORTER_OTLP_ENDPOINT}
    sampling_rate: 0.1
  logging:
    level: info
    format: json
```

## Deployment Steps

### 1. Infrastructure Setup

#### Redis Cluster with Sentinel

```bash
# Deploy Redis cluster with Sentinel
kubectl apply -f k8s/redis-cluster.yaml
kubectl apply -f k8s/redis-sentinel.yaml

# Verify cluster health
redis-cli --cluster check redis-node1:6379
```

#### TLS Certificates

```bash
# Generate certificates for Redis TLS
./scripts/generate-redis-certs.sh

# Create Kubernetes secrets
kubectl create secret generic redis-tls \
  --from-file=ca.crt=ca.crt \
  --from-file=client.crt=client.crt \
  --from-file=client.key=client.key
```

### 2. Database Migrations

```bash
# No new database tables required - uses existing DevOps MCP schema
# Redis handles all webhook-specific data
```

### 3. Application Deployment

#### Build and Push Docker Images

```bash
# Build webhook processor image
docker build -f Dockerfile.webhook -t devops-mcp-webhook:latest .
docker tag devops-mcp-webhook:latest your-registry/devops-mcp-webhook:v1.0.0
docker push your-registry/devops-mcp-webhook:v1.0.0
```

#### Deploy to Kubernetes

```bash
# Create namespace
kubectl create namespace webhook-system

# Create secrets
kubectl create secret generic webhook-secrets \
  --from-literal=redis-password=${REDIS_PASSWORD} \
  --from-literal=encryption-key=${ENCRYPTION_MASTER_KEY} \
  --from-literal=openai-api-key=${OPENAI_API_KEY} \
  -n webhook-system

# Deploy application
kubectl apply -f k8s/webhook-deployment.yaml -n webhook-system
kubectl apply -f k8s/webhook-service.yaml -n webhook-system
kubectl apply -f k8s/webhook-hpa.yaml -n webhook-system
```

### 4. Configure Webhook Sources

#### GitHub Webhook Configuration

```bash
# For each GitHub organization/repository:
curl -X POST https://api.github.com/repos/ORG/REPO/hooks \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  -d '{
    "name": "web",
    "active": true,
    "events": ["*"],
    "config": {
      "url": "https://api.your-domain.com/webhooks/github",
      "content_type": "json",
      "secret": "${WEBHOOK_SECRET}",
      "insecure_ssl": "0"
    }
  }'
```

#### Tool-Specific Secrets

```bash
# Add webhook secrets to configuration
kubectl patch configmap webhook-config -n webhook-system --type merge -p '
{
  "data": {
    "signature_secrets": {
      "github": "'${GITHUB_WEBHOOK_SECRET}'",
      "jira": "'${JIRA_WEBHOOK_SECRET}'",
      "gitlab": "'${GITLAB_WEBHOOK_SECRET}'"
    }
  }
}'
```

### 5. Monitoring Setup

#### Prometheus Configuration

```yaml
# prometheus-config.yaml
scrape_configs:
  - job_name: 'webhook-processor'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - webhook-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        regex: webhook-processor
        action: keep
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: instance
```

#### Grafana Dashboards

Import the provided dashboards:
- `dashboards/webhook-overview.json` - System overview
- `dashboards/webhook-performance.json` - Performance metrics
- `dashboards/webhook-ai-intelligence.json` - AI service metrics
- `dashboards/webhook-security.json` - Security monitoring

#### Alerting Rules

```yaml
# alerts.yaml
groups:
  - name: webhook_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(webhook_events_failed_total[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High webhook error rate
          
      - alert: ConsumerLag
        expr: webhook_consumer_lag_messages > 1000
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: Webhook consumer lag is high
          
      - alert: CircuitBreakerOpen
        expr: webhook_circuit_breaker_state == 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: Circuit breaker is open for {{ $labels.tool_id }}
```

### 6. Load Testing

```bash
# Run load tests before going live
./scripts/load-test-webhooks.sh \
  --target https://api.your-domain.com/webhooks \
  --rate 1000 \
  --duration 10m \
  --tool github
```

## Operations

### Health Checks

```bash
# Check system health
curl https://api.your-domain.com/health/webhook

# Check Redis connectivity
redis-cli -h redis-node1 ping

# Check consumer lag
redis-cli -h redis-node1 XINFO GROUPS webhook-events
```

### Scaling

```bash
# Scale webhook processors
kubectl scale deployment webhook-processor -n webhook-system --replicas=20

# Auto-scaling based on Redis stream depth
# HPA configuration handles this automatically
```

### Backup and Recovery

```bash
# Backup Redis data
redis-cli -h redis-node1 BGSAVE

# Backup S3 cold storage
aws s3 sync s3://devops-mcp-webhook-contexts s3://backup-bucket/webhook-contexts

# Disaster recovery - restore from S3
./scripts/restore-contexts-from-s3.sh
```

### Troubleshooting

#### Common Issues

1. **High Memory Usage**
   ```bash
   # Check Redis memory
   redis-cli -h redis-node1 INFO memory
   
   # Trigger context lifecycle transitions
   kubectl exec -it webhook-processor-xxx -n webhook-system -- \
     ./webhook-cli force-transition --tier warm
   ```

2. **Consumer Lag**
   ```bash
   # Check pending messages
   redis-cli -h redis-node1 XPENDING webhook-events webhook-processors
   
   # Claim stale messages
   kubectl exec -it webhook-processor-xxx -n webhook-system -- \
     ./webhook-cli claim-pending --idle-time 5m
   ```

3. **Circuit Breaker Issues**
   ```bash
   # Reset circuit breaker
   kubectl exec -it webhook-processor-xxx -n webhook-system -- \
     ./webhook-cli reset-circuit-breaker --tool github
   ```

### Performance Tuning

#### Redis Optimization

```bash
# Redis configuration for production
maxmemory 8gb
maxmemory-policy allkeys-lru
save ""  # Disable RDB snapshots, use AOF only
appendonly yes
appendfsync everysec
```

#### Application Tuning

```yaml
# Optimal worker configuration based on load testing
consumer:
  num_workers: ${CORES * 3}  # 3 workers per CPU core
  batch_size: 100            # Process 100 messages per batch
  prefetch_count: 1000       # Prefetch messages for better throughput
```

## Security Considerations

### Network Security

- All Redis connections use TLS 1.3
- Webhook endpoints behind WAF
- IP whitelisting for webhook sources
- VPC peering for internal communication

### Data Security

- Encryption at rest using AES-256-GCM
- Per-tenant key derivation
- Sensitive fields redacted in logs
- Audit trail for all operations

### Compliance

- GDPR: Data retention policies enforced
- SOC2: Audit logging enabled
- HIPAA: Encryption meets requirements

## Migration from SQS

### Parallel Running

```bash
# Enable dual-write mode
kubectl set env deployment/webhook-processor \
  DUAL_WRITE_MODE=true \
  SQS_QUEUE_URL=${OLD_SQS_QUEUE} \
  -n webhook-system

# Monitor both systems
./scripts/compare-sqs-redis-metrics.sh
```

### Cutover Steps

1. Enable dual-write mode (both SQS and Redis)
2. Deploy Redis consumers alongside SQS workers
3. Validate message processing parity
4. Gradually shift traffic to Redis
5. Disable SQS workers
6. Remove dual-write mode

### Rollback Plan

```bash
# Quick rollback to SQS
kubectl set env deployment/webhook-processor \
  WEBHOOK_BACKEND=sqs \
  SQS_QUEUE_URL=${OLD_SQS_QUEUE} \
  -n webhook-system

# Scale down Redis consumers
kubectl scale deployment webhook-processor --replicas=0 -n webhook-system
```

## Monitoring Dashboards

### Key Metrics to Monitor

1. **Throughput**: Events processed per second
2. **Latency**: P50, P95, P99 processing times
3. **Error Rate**: Failed events percentage
4. **Queue Depth**: Pending messages in Redis
5. **AI Performance**: Embedding/summarization latencies
6. **Resource Usage**: CPU, memory, Redis memory

### SLO Targets

- **Availability**: 99.9% uptime
- **Latency**: P99 < 1 second for event processing
- **Error Rate**: < 0.1% failed events
- **Queue Depth**: < 1000 messages backlog

## Cost Optimization

### Redis Costs

- Use Redis cluster with appropriate node sizes
- Enable compression for warm tier
- Aggressive TTLs for hot tier
- Move to S3 for cold storage

### AI Service Costs

- Cache embeddings aggressively (24h+)
- Batch embedding requests
- Use smaller models where appropriate
- Implement request deduplication

## Support and Maintenance

### Regular Maintenance

- **Daily**: Check metrics and alerts
- **Weekly**: Review error logs and performance
- **Monthly**: Capacity planning review
- **Quarterly**: Security audit

### Contact Information

- **On-call**: #webhook-oncall Slack channel
- **Escalation**: webhook-team@company.com
- **Documentation**: https://wiki.company.com/webhook-system

## Appendix

### A. Sample Kubernetes Manifests

See `/k8s/` directory for complete manifests.

### B. Monitoring Queries

```promql
# Average processing time
rate(webhook_event_processing_duration_seconds_sum[5m]) / rate(webhook_event_processing_duration_seconds_count[5m])

# Error rate
rate(webhook_events_failed_total[5m]) / rate(webhook_events_received_total[5m])

# Consumer lag
webhook_consumer_lag_messages
```

### C. Emergency Procedures

1. **Complete System Down**: Follow DR plan in `/docs/disaster-recovery.md`
2. **Redis Failure**: Sentinel will handle failover automatically
3. **High Error Rate**: Check circuit breakers and external service health
4. **Memory Pressure**: Force context transitions to cold storage

---

This production deployment guide provides comprehensive instructions for deploying and operating the Redis-based webhook processing system. Regular updates to this document should be made as the system evolves.
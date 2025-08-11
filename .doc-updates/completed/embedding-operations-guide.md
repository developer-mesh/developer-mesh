<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:30:51
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Multi-Agent Embedding System Operations Guide

This guide covers operational aspects of running the Multi-Agent Embedding System in production.

## Table of Contents

1. [Deployment](#deployment)
2. [Configuration Management](#configuration-management)
3. [Monitoring and Alerting](#monitoring-and-alerting)
4. [Performance Tuning](#performance-tuning)
5. [Troubleshooting](#troubleshooting)
6. [Maintenance Tasks](#maintenance-tasks)
7. [Disaster Recovery](#disaster-recovery)

## Deployment

### Production Deployment Checklist

- [ ] **Provider Configuration**
  - [ ] API keys stored in secrets manager
  - [ ] At least 2 providers configured for redundancy
  - [ ] Provider endpoints verified
  
- [ ] **Database Setup**
  - [ ] PostgreSQL with pgvector installed
  - [ ] Proper indexes created
  - [ ] Connection pooling configured
  - [ ] Read replicas for search workloads
  
- [ ] **Caching Layer**
  - [ ] Redis cluster deployed
  - [ ] Appropriate memory allocation
  - [ ] Persistence configured
  - [ ] Failover tested
  
- [ ] **Security**
  - [ ] API authentication enabled
  - [ ] Network isolation configured
  - [ ] Secrets rotation scheduled
  - [ ] Audit logging enabled
  
- [ ] **Monitoring**
  - [ ] Metrics collection enabled
  - [ ] Log aggregation configured
  - [ ] Alerts configured
  - [ ] Dashboards created

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: embedding-api
  namespace: developer-mesh
spec:
  replicas: 3
  selector:
    matchLabels:
      app: embedding-api
  template:
    metadata:
      labels:
        app: embedding-api
    spec:
      containers:
      - name: rest-api
        image: developer-mesh/rest-api:latest
        ports:
        - containerPort: 8081
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: embedding-secrets
              key: openai-api-key
        - name: BEDROCK_ENABLED
          value: "true"
        - name: AWS_REGION
          value: "us-east-1"
        resources:
          requests:
            cpu: 500m
            memory: 1Gi
          limits:
            cpu: 2000m
            memory: 4Gi
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8081
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8081
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: embedding-api
  namespace: developer-mesh
spec:
  selector:
    app: embedding-api
  ports:
  - port: 80
    targetPort: 8081
  type: LoadBalancer
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: embedding-api-hpa
  namespace: developer-mesh
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: embedding-api
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Docker Compose Production

```yaml
version: '3.8'

services:
  rest-api:
    image: ${DOCKER_REGISTRY}/developer-mesh/rest-api:${VERSION}
    deploy:
      replicas: 3
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3
      resources:
        limits:
          cpus: '2'
          memory: 4G
        reservations:
          cpus: '0.5'
          memory: 1G
    environment:
      - ENVIRONMENT=production
      - OPENAI_ENABLED=${OPENAI_ENABLED}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - BEDROCK_ENABLED=${BEDROCK_ENABLED}
      - AWS_REGION=${AWS_REGION}
      - DATABASE_HOST=${DATABASE_HOST}
      - DATABASE_SSL_MODE=require
      - REDIS_CLUSTER_ENDPOINTS=${REDIS_CLUSTER_ENDPOINTS}
    networks:
      - embedding-network
    depends_on:
      - postgres
      - redis
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8081/health/ready"]
      interval: 30s
      timeout: 10s
      retries: 3

  postgres:
    image: pgvector/pgvector:pg15
    environment:
      - POSTGRES_PASSWORD=${DB_PASSWORD}
      - POSTGRES_DB=embeddings
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - embedding-network

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data
    networks:
      - embedding-network

networks:
  embedding-network:
    driver: overlay
    encrypted: true

volumes:
  postgres-data:
  redis-data:
```

## Configuration Management

### Environment-Specific Configurations

```yaml
# production.yaml
embedding:
  providers:
    openai:
      rate_limit:
        requests_per_minute: 3000
        tokens_per_minute: 1000000
    bedrock:
      rate_limit:
        requests_per_minute: 1000
  
  circuit_breaker:
    failure_threshold: 5
    success_threshold: 3
    timeout: "30s"
  
  router:
    selection_strategy: "smart"
    cache_ttl: "15m"
  
  security:
    api_key_rotation:
      enabled: true
      rotation_period: "30d"
    
    pii_detection:
      enabled: true
      redact_pii: true
```

### Dynamic Configuration Updates

```bash
# Update agent configuration without restart
curl -X PUT http://api.example.com/api/v1/embeddings/agents/prod-agent \
  -H "X-API-Key: admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "embedding_strategy": "quality",
    "constraints": {
      "max_cost_per_day": 150.0
    }
  }'
```

## Monitoring and Alerting

### Key Metrics to Monitor

#### System Metrics
- **Request Rate**: Requests per second by endpoint
- **Latency**: P50, P95, P99 response times
- **Error Rate**: 4xx and 5xx responses
- **Throughput**: Embeddings generated per minute

#### Provider Metrics
- **Provider Health**: Availability percentage
- **Provider Latency**: Response time by provider
- **Circuit Breaker State**: Open/Closed/Half-Open
- **Failover Rate**: Provider switch frequency

#### Cost Metrics
- **Cost per Hour/Day**: By agent and provider
- **Token Usage**: Tokens consumed by model
- **Budget Utilization**: Percentage of daily budget used

#### Database Metrics
- **Query Performance**: Slow queries
- **Connection Pool**: Active/Idle connections
- **Index Usage**: Vector search performance
- **Storage Growth**: Embedding table size

### Prometheus Configuration

```yaml
# prometheus-rules.yaml
groups:
  - name: embedding_alerts
    interval: 30s
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} errors per second"
      
      - alert: ProviderDown
        expr: embedding_provider_health{status="unhealthy"} == 1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Embedding provider {{ $labels.provider }} is down"
      
      - alert: HighCostRate
        expr: rate(embedding_cost_total[1h]) > 100
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High embedding cost rate"
          description: "Cost rate is ${{ $value }} per hour"
      
      - alert: CircuitBreakerOpen
        expr: circuit_breaker_state{state="open"} == 1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Circuit breaker open for {{ $labels.provider }}"
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Embedding System Dashboard",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])"
          }
        ]
      },
      {
        "title": "Latency Percentiles",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, http_request_duration_seconds_bucket)"
          }
        ]
      },
      {
        "title": "Provider Health",
        "targets": [
          {
            "expr": "embedding_provider_health"
          }
        ]
      },
      {
        "title": "Cost by Agent",
        "targets": [
          {
            "expr": "sum by (agent_id) (increase(embedding_cost_total[1h]))"
          }
        ]
      }
    ]
  }
}
```

## Performance Tuning

### Database Optimization

```sql
-- Create optimized indexes
CREATE INDEX idx_embeddings_agent_created 
  ON embeddings(agent_id, created_at DESC);

CREATE INDEX idx_embeddings_vector_cosine 
  ON embeddings USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 1000);

-- For high-recall requirements
CREATE INDEX idx_embeddings_vector_hnsw 
  ON embeddings USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 200);

-- Partitioning for large datasets
CREATE TABLE embeddings_2024_01 PARTITION OF embeddings
  FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

-- Analyze tables regularly
ANALYZE embeddings;
```

### Caching Optimization

```yaml
# Redis configuration
redis:
  cache_strategies:
    embeddings:
      ttl: 900  # 15 minutes
      max_memory: 4gb
      eviction_policy: allkeys-lru
    
    provider_health:
      ttl: 30  # 30 seconds
      max_entries: 100
```

### Connection Pooling

```yaml
database:
  pool:
    max_open_conns: 100
    max_idle_conns: 25
    conn_max_lifetime: 30m
    conn_max_idle_time: 10m
  
  read_replica_pool:
    max_open_conns: 200
    max_idle_conns: 50
```

## Troubleshooting

### Common Issues and Solutions

#### High Latency
**Symptoms**: Response times > 1s
**Diagnosis**:
```bash
# Check provider latencies
curl http://localhost:8081/api/v1/embeddings/providers/health

# Check database slow queries
SELECT query, calls, mean_exec_time 
FROM pg_stat_statements 
WHERE mean_exec_time > 100 
ORDER BY mean_exec_time DESC;
```
**Solutions**:
- Scale up API servers
- Add database read replicas
- Increase cache TTL
- Use smaller embedding models

#### Provider Failures
**Symptoms**: Circuit breaker open, increased errors
**Diagnosis**:
```bash
# Check logs
kubectl logs -l app=embedding-api | grep ERROR

# Check circuit breaker state
curl http://localhost:8081/metrics | grep circuit_breaker
```
**Solutions**:
- Verify API keys are valid
- Check provider service status
- Increase circuit breaker thresholds
- Configure more fallback providers

#### Cost Overruns
**Symptoms**: Daily budget exceeded alerts
**Diagnosis**:
```sql
-- Check cost by agent
SELECT agent_id, 
       SUM(cost_usd) as total_cost,
       COUNT(*) as request_count
FROM embedding_metrics
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY agent_id
ORDER BY total_cost DESC;
```
**Solutions**:
- Adjust agent cost limits
- Switch to cheaper models
- Enable more aggressive caching
- Implement request throttling

### Debug Logging

```yaml
# Enable debug logging for specific components
logging:
  level: info
  components:
    embedding.router: debug
    embedding.provider.openai: debug
    embedding.circuit_breaker: debug
```

## Maintenance Tasks

### Daily Tasks
- [ ] Review cost reports
- [ ] Check provider health status
- [ ] Monitor error rates
- [ ] Verify backup completion

### Weekly Tasks
- [ ] Analyze slow queries
- [ ] Review circuit breaker events
- [ ] Update agent configurations
- [ ] Check index fragmentation

### Monthly Tasks
- [ ] Rotate API keys
- [ ] Update provider libraries
- [ ] Review and optimize indexes
- [ ] Capacity planning review

### Maintenance Scripts

```bash
#!/bin/bash
# maintenance.sh

# Vacuum analyze embedding tables
psql -h $DB_HOST -U $DB_USER -d embeddings << EOF
VACUUM ANALYZE embeddings;
REINDEX INDEX CONCURRENTLY idx_embeddings_vector_cosine;
EOF

# Clear old cache entries
redis-cli --scan --pattern "embedding:*" | \
  xargs -L 1000 redis-cli DEL

# Archive old embeddings
psql -h $DB_HOST -U $DB_USER -d embeddings << EOF
INSERT INTO embeddings_archive 
SELECT * FROM embeddings 
WHERE created_at < NOW() - INTERVAL '90 days';

DELETE FROM embeddings 
WHERE created_at < NOW() - INTERVAL '90 days';
EOF

# Generate cost report
curl -X GET http://localhost:8081/api/v1/reports/costs \
  -H "X-API-Key: $ADMIN_KEY" \
  -o daily-cost-report.json
```

## Disaster Recovery

### Backup Strategy

```yaml
backup:
  postgres:
    schedule: "0 2 * * *"  # 2 AM daily
    retention: 30  # days
    method: pg_dump
    storage: s3://backups/embeddings/
  
  redis:
    schedule: "0 * * * *"  # Hourly
    retention: 7  # days
    method: bgsave
    storage: s3://backups/redis/
```

### Recovery Procedures

#### Provider Failure Recovery
1. Circuit breaker automatically opens
2. Requests route to fallback providers
3. Monitor recovery attempts
4. Manual intervention if needed:
   ```bash
   # Force provider health check
   curl -X POST http://localhost:8081/api/v1/admin/providers/openai/health-check
   
   # Reset circuit breaker
   curl -X POST http://localhost:8081/api/v1/admin/circuit-breaker/reset
   ```

#### Database Recovery
```bash
# Restore from backup
pg_restore -h $DB_HOST -U $DB_USER -d embeddings \
  /backups/embeddings_20240115.dump

# Rebuild indexes
psql -h $DB_HOST -U $DB_USER -d embeddings << EOF
REINDEX DATABASE embeddings;
ANALYZE;
EOF
```

#### Cache Recovery
```bash
# Redis is ephemeral, just restart
docker-compose restart redis

# Warm cache with recent embeddings
curl -X POST http://localhost:8081/api/v1/admin/cache/warm
```

### Runbooks

#### High Cost Alert Runbook
1. **Identify** high-cost agents:
   ```bash
   curl http://localhost:8081/api/v1/metrics/costs/by-agent
   ```
2. **Analyze** usage patterns
3. **Adjust** agent configurations:
   - Switch to cheaper models
   - Reduce daily limits
   - Enable caching
4. **Monitor** for 1 hour
5. **Escalate** if costs continue rising

#### Performance Degradation Runbook
1. **Check** provider latencies
2. **Review** database performance
3. **Scale** if needed:
   ```bash
   kubectl scale deployment embedding-api --replicas=5
   ```
4. **Enable** emergency caching
5. **Notify** stakeholders

## Security Operations

### API Key Management
```bash
# Rotate API keys
./scripts/rotate-api-keys.sh

# Audit API key usage
SELECT api_key_id, 
       COUNT(*) as requests,
       MAX(created_at) as last_used
FROM api_requests
WHERE created_at > NOW() - INTERVAL '7 days'
GROUP BY api_key_id;
```

### PII Detection
```yaml
pii_detection:
  enabled: true
  patterns:
    - ssn: '\d{3}-\d{2}-\d{4}'
    - email: '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}'
    - phone: '\+?1?\d{10,14}'
  action: redact  # or reject
```

### Audit Logging
```sql
-- Review suspicious activity
SELECT user_id, 
       endpoint,
       COUNT(*) as request_count,
       SUM(cost_usd) as total_cost
FROM audit_logs
WHERE created_at > NOW() - INTERVAL '1 hour'
GROUP BY user_id, endpoint
HAVING COUNT(*) > 1000
   OR SUM(cost_usd) > 100;

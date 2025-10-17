<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:27:27
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Authentication Operations Guide

## Table of Contents

1. [Deployment](#deployment)
2. [Configuration Management](#configuration-management)
3. [Monitoring and Alerting](#monitoring-and-alerting)
4. [Troubleshooting](#troubleshooting)
5. [Performance Tuning](#performance-tuning)
6. [Security Operations](#security-operations)
7. [Maintenance Tasks](#maintenance-tasks)
8. [Disaster Recovery](#disaster-recovery)

## Deployment

### Prerequisites

- PostgreSQL 12+ with pgvector extension
- Redis 6+ for distributed rate limiting
- Prometheus for metrics collection
- Sufficient CPU/memory for auth processing

### Environment Variables

```bash
# Core Authentication
JWT_SECRET=your-production-secret-minimum-32-chars
JWT_EXPIRATION=24h
AUTH_CACHE_ENABLED=true
AUTH_CACHE_TTL=5m

# Rate Limiting
RATE_LIMIT_MAX_ATTEMPTS=100
RATE_LIMIT_WINDOW=1m
RATE_LIMIT_LOCKOUT=15m

# Database
DATABASE_URL=postgres://user:pass@host:5432/devops_mcp?sslmode=require
DATABASE_POOL_SIZE=20
DATABASE_MAX_IDLE_TIME=30m

# Redis
REDIS_URL=redis://host:6379/0
REDIS_POOL_SIZE=50
REDIS_TIMEOUT=5s

# Monitoring
METRICS_ENABLED=true
AUDIT_LOG_ENABLED=true
AUDIT_LOG_LEVEL=info
```

### Deployment Steps

1. **Database Migration**
   ```bash
   # Run migrations
   make migrate-up dsn="${DATABASE_URL}"
   
   # Verify API keys table
   psql "${DATABASE_URL}" -c "SELECT COUNT(*) FROM api_keys;"
   ```

2. **Configuration Validation**
   ```bash
   # Validate configuration
   ./edge-mcp validate-config --config=configs/auth.production.yaml
   
   # Test authentication
   curl -H "Authorization: Bearer ${TEST_API_KEY}" https://api.example.com/health
   ```

3. **Health Checks**
   ```yaml
   # Kubernetes health check
   livenessProbe:
     httpGet:
       path: /health
       port: 8080
     initialDelaySeconds: 30
     periodSeconds: 10
   
   readinessProbe:
     httpGet:
       path: /ready
       port: 8080
       httpHeaders:
       - name: Authorization
         value: Bearer ${HEALTH_CHECK_KEY}
     initialDelaySeconds: 5
     periodSeconds: 5
   ```

## Configuration Management

### Production Configuration

```yaml
# configs/auth.production.yaml
auth:
  # JWT Configuration
  jwt:
    secret: ${JWT_SECRET} # From environment/secrets
    expiration: 24h
    refresh_enabled: true
    refresh_expiration: 7d
  
  # API Key Configuration
  api_keys:
    header: X-API-Key
    enable_database: true
    cache_ttl: 5m
    rotation_period: 90d
  
  # Rate Limiting
  rate_limiting:
    default:
      max_attempts: 100
      window: 1m
      lockout: 15m
    per_tenant:
      premium:
        max_attempts: 1000
        window: 1m
      standard:
        max_attempts: 100
        window: 1m
      trial:
        max_attempts: 10
        window: 1m
  
  # Security
  security:
    max_failed_attempts: 5
    lockout_duration: 15m
    password_policy:
      min_length: 12
      require_uppercase: true
      require_numbers: true
      require_special: true
```

### Dynamic Configuration Updates

```go
// Hot reload configuration
func (s *Server) ReloadConfig(ctx context.Context) error {
    newConfig, err := LoadConfig("configs/auth.production.yaml")
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }
    
    // Validate before applying
    if err := newConfig.Validate(); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }
    
    // Apply new configuration
    s.authService.UpdateConfig(newConfig)
    s.logger.Info("Configuration reloaded", map[string]interface{}{
        "version": newConfig.Version,
    })
    
    return nil
}
```

## Monitoring and Alerting

### Key Metrics to Monitor

1. **Authentication Metrics**
   ```promql
   # Authentication success rate
   rate(auth_attempts_total{status="success"}[5m]) / 
   rate(auth_attempts_total[5m]) * 100
   
   # Authentication latency P95
   histogram_quantile(0.95, 
     rate(auth_duration_seconds_bucket[5m])
   )
   
   # Failed authentication by type
   sum by (type) (
     rate(auth_attempts_total{status="failure"}[5m])
   )
   ```

2. **Rate Limiting Metrics**
   ```promql
   # Rate limit violations
   rate(auth_rate_limit_exceeded_total[5m])
   
   # Clients approaching rate limit
   (rate_limit_tokens_remaining / rate_limit_tokens_total) < 0.2
   ```

3. **API Key Metrics**
   ```promql
   # Expired API keys
   api_keys_total{status="expired"}
   
   # API keys expiring soon (7 days)
   api_keys_expiring_soon{days="7"}
   ```

### Alert Configuration

```yaml
# prometheus/alerts/auth.yml
groups:
  - name: authentication
    interval: 30s
    rules:
      - alert: HighAuthFailureRate
        expr: |
          (rate(auth_attempts_total{status="failure"}[5m]) / 
           rate(auth_attempts_total[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High authentication failure rate: {{ $value | humanizePercentage }}"
          description: "Authentication failure rate above 10% for 5 minutes"
      
      - alert: AuthServiceDown
        expr: up{job="auth-service"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Authentication service is down"
      
      - alert: RateLimitingOverload
        expr: rate(auth_rate_limit_exceeded_total[5m]) > 100
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High rate of rate limit violations"
      
      - alert: AuthLatencyHigh
        expr: |
          histogram_quantile(0.95, 
            rate(auth_duration_seconds_bucket[5m])
          ) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Authentication latency P95 above 500ms"
```

### Grafana Dashboard

Key panels to include:

1. **Authentication Overview**
   - Success/failure rate gauge
   - Request rate graph
   - Latency histogram
   - Active sessions counter

2. **Rate Limiting**
   - Violations by tenant
   - Top rate limited IPs
   - Lockout events timeline

3. **API Key Management**
   - Total keys by status
   - Key usage heatmap
   - Expiration timeline

4. **Security Events**
   - Failed auth attempts map
   - Suspicious activity alerts
   - Audit log browser

## Troubleshooting

### Common Issues

#### 1. 401 Unauthorized Errors

**Symptoms:**
- All requests return 401
- Valid API keys rejected

**Diagnosis:**
```bash
# Check if auth service is running
curl http://localhost:8080/health

# Verify API key format
echo -n "your-api-key" | wc -c  # Should be >= 16

# Check auth service logs
kubectl logs -f deployment/edge-mcp -c auth | grep ERROR

# Test with curl
curl -v -H "Authorization: Bearer your-api-key" http://localhost:8080/api/v1/models
```

**Solutions:**
- Verify API key is loaded in configuration
- Check for clock skew (JWT validation)
- Ensure cache is accessible
- Verify database connectivity

#### 2. 429 Rate Limit Errors

**Symptoms:**
- Frequent 429 responses
- Retry-After headers present

**Diagnosis:**
```bash
# Check rate limit status
redis-cli GET "auth:ratelimit:ip:192.168.1.1:count"
redis-cli GET "auth:ratelimit:ip:192.168.1.1:lockout"

# Monitor rate limit metrics
curl http://localhost:9090/metrics | grep rate_limit

# Check configuration
grep -A5 "rate_limit" configs/auth.production.yaml
```

**Solutions:**
- Adjust rate limits for specific tenants
- Implement client-side retry logic
- Clear rate limit for specific IP:
  ```bash
  redis-cli DEL "auth:ratelimit:ip:192.168.1.1:count"
  redis-cli DEL "auth:ratelimit:ip:192.168.1.1:lockout"
  ```

#### 3. JWT Token Issues

**Symptoms:**
- "Token expired" errors
- "Invalid signature" errors

**Diagnosis:**
```bash
# Decode JWT token
echo "your.jwt.token" | cut -d. -f2 | base64 -d | jq .

# Check token expiration
jwt_exp=$(echo "your.jwt.token" | cut -d. -f2 | base64 -d | jq -r .exp)
current_time=$(date +%s)
if [ $jwt_exp -lt $current_time ]; then
  echo "Token expired"
fi

# Verify JWT secret
echo $JWT_SECRET | wc -c  # Should be >= 32
```

**Solutions:**
- Regenerate tokens with correct expiration
- Verify JWT_SECRET matches across services
- Implement token refresh mechanism

#### 4. Performance Issues

**Symptoms:**
- High authentication latency
- Timeouts on auth endpoints

**Diagnosis:**
```bash
# Check cache hit rate
redis-cli INFO stats | grep keyspace

# Database query performance
psql $DATABASE_URL -c "
  SELECT query, calls, mean_exec_time 
  FROM pg_stat_statements 
  WHERE query LIKE '%api_keys%' 
  ORDER BY mean_exec_time DESC 
  LIMIT 10;
"

# CPU/Memory usage
kubectl top pods -l app=edge-mcp
```

**Solutions:**
- Enable caching if disabled
- Add database indexes:
  ```sql
  CREATE INDEX idx_api_keys_key_active ON api_keys(key, active);
  CREATE INDEX idx_api_keys_tenant_active ON api_keys(tenant_id, active);
  ```
- Scale auth service horizontally

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
# Enable in configuration
logging:
  level: debug
  auth_debug: true
  rate_limit_debug: true

# Or via environment
LOG_LEVEL=debug
AUTH_DEBUG=true
RATE_LIMIT_DEBUG=true
```

Debug log examples:
```
[DEBUG] Auth validation started key_suffix=...7890 method=api_key
[DEBUG] Cache lookup key=auth:apikey:...7890 hit=false
[DEBUG] Database query key_suffix=...7890 found=true
[DEBUG] Scopes validation required=[read,write] user=[read,write,admin] allowed=true
[DEBUG] Auth validation completed user_id=user-123 tenant_id=tenant-456 duration=12ms
```

## Performance Tuning

### Cache Optimization

```yaml
# Optimal cache configuration
cache:
  # In-memory cache for hot keys
  local:
    size: 10000
    ttl: 30s
  
  # Redis for distributed cache
  redis:
    pool_size: 100
    timeout: 100ms
    ttl: 5m
    
  # Cache warming
  warming:
    enabled: true
    keys:
      - pattern: "auth:apikey:admin-*"
        ttl: 10m
      - pattern: "auth:tenant:premium-*"
        ttl: 10m
```

### Database Optimization

```sql
-- Optimize API key queries
CREATE INDEX CONCURRENTLY idx_api_keys_lookup 
ON api_keys(key, active, tenant_id) 
WHERE active = true;

-- Partial index for active keys only
CREATE INDEX CONCURRENTLY idx_api_keys_active 
ON api_keys(tenant_id, created_at) 
WHERE active = true;

-- Analyze tables regularly
ANALYZE api_keys;

-- Monitor slow queries
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

### Connection Pooling

```go
// Optimal database pool configuration
dbConfig := pgxpool.Config{
    MaxConns:        50,
    MinConns:        10,
    MaxConnLifetime: 30 * time.Minute,
    MaxConnIdleTime: 5 * time.Minute,
    HealthCheckPeriod: 1 * time.Minute,
}

// Redis pool configuration
redisOpts := &redis.Options{
    PoolSize:     100,
    MinIdleConns: 20,
    MaxRetries:   3,
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,
}
```

## Security Operations

### API Key Rotation

```bash
#!/bin/bash
# Rotate API keys script

# Generate new key
NEW_KEY=$(openssl rand -hex 32)

# Update in database
psql $DATABASE_URL <<SQL
BEGIN;
-- Deactivate old key
UPDATE api_keys 
SET active = false, 
    deactivated_at = NOW() 
WHERE key = '$OLD_KEY';

-- Insert new key
INSERT INTO api_keys (key, tenant_id, user_id, name, scopes, active)
SELECT '$NEW_KEY', tenant_id, user_id, name || ' (rotated)', scopes, true
FROM api_keys 
WHERE key = '$OLD_KEY';
COMMIT;
SQL

# Update in application config
kubectl create secret generic api-keys \
  --from-literal=admin-key=$NEW_KEY \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart pods to pick up new keys
kubectl rollout restart deployment/edge-mcp
```

### Security Audit

```sql
-- Find unused API keys
SELECT key, name, last_used, created_at
FROM api_keys
WHERE active = true
  AND (last_used IS NULL OR last_used < NOW() - INTERVAL '90 days')
ORDER BY created_at;

-- Check for weak keys
SELECT key, name
FROM api_keys
WHERE active = true
  AND LENGTH(key) < 32;

-- Audit failed authentications
SELECT 
  date_trunc('hour', created_at) as hour,
  COUNT(*) as failures,
  COUNT(DISTINCT ip_address) as unique_ips
FROM auth_audit_log
WHERE success = false
  AND created_at > NOW() - INTERVAL '24 hours'
GROUP BY 1
ORDER BY 1 DESC;
```

### Incident Response

1. **Suspected Compromise**
   ```bash
   # Immediately revoke suspected keys
   ./scripts/revoke-api-key.sh $COMPROMISED_KEY
   
   # Analyze usage patterns
   ./scripts/analyze-key-usage.sh $COMPROMISED_KEY --days 7
   
   # Generate incident report
   ./scripts/security-incident-report.sh --key $COMPROMISED_KEY
   ```

2. **Brute Force Attack**
   ```bash
   # Block attacking IPs
   ./scripts/block-ips.sh --threshold 100 --window 5m
   
   # Increase rate limits temporarily
   kubectl set env deployment/edge-mcp RATE_LIMIT_MAX_ATTEMPTS=10
   
   # Alert security team
   ./scripts/security-alert.sh --type brute-force --severity high
   ```

## Maintenance Tasks

### Daily Tasks

```bash
#!/bin/bash
# Daily auth maintenance

# Clean up expired sessions
psql $DATABASE_URL -c "DELETE FROM sessions WHERE expires_at < NOW();"

# Update API key usage stats
psql $DATABASE_URL -c "REFRESH MATERIALIZED VIEW api_key_usage_stats;"

# Check for expiring keys
./scripts/notify-expiring-keys.sh --days 7

# Backup auth configuration
kubectl get configmap auth-config -o yaml > backups/auth-config-$(date +%Y%m%d).yaml
```

### Weekly Tasks

```bash
# Analyze auth patterns
./scripts/auth-usage-report.sh --week

# Rotate log files
logrotate -f /etc/logrotate.d/auth

# Update rate limit policies based on usage
./scripts/optimize-rate-limits.sh

# Security scan
./scripts/security-scan.sh --component auth
```

### Monthly Tasks

```bash
# Full security audit
./scripts/monthly-security-audit.sh

# Performance review
./scripts/auth-performance-report.sh --month

# Clean old audit logs
psql $DATABASE_URL -c "
  DELETE FROM auth_audit_log 
  WHERE created_at < NOW() - INTERVAL '90 days';
"

# Update documentation
./scripts/generate-auth-docs.sh > docs/auth-operations-$(date +%Y%m).md
```

## Disaster Recovery

### Backup Procedures

```bash
#!/bin/bash
# Backup authentication data

# Database backup
pg_dump $DATABASE_URL \
  --table=api_keys \
  --table=auth_audit_log \
  --table=sessions \
  > backups/auth-db-$(date +%Y%m%d-%H%M%S).sql

# Configuration backup
kubectl get all -n auth -o yaml > backups/auth-k8s-$(date +%Y%m%d-%H%M%S).yaml

# Redis backup
redis-cli --rdb backups/auth-redis-$(date +%Y%m%d-%H%M%S).rdb

# Encrypt and upload to S3
tar czf - backups/ | \
  openssl enc -aes-256-cbc -pass pass:$BACKUP_PASSWORD | \
  aws s3 cp - s3://backups/auth/$(date +%Y%m%d)/auth-backup.tar.gz.enc
```

### Recovery Procedures

1. **Service Recovery**
   ```bash
   # Restore from backup
   ./scripts/restore-auth.sh --backup s3://backups/auth/20240101/
   
   # Verify service health
   ./scripts/verify-auth-health.sh

   # Run integration tests
   make test-integration
   ```

2. **Data Recovery**
   ```sql
   -- Restore API keys
   \i backups/auth-db-20240101-120000.sql
   
   -- Verify data integrity
   SELECT COUNT(*), 
          COUNT(DISTINCT tenant_id) as tenants,
          COUNT(DISTINCT user_id) as users
   FROM api_keys
   WHERE active = true;
   ```

3. **Configuration Recovery**
   ```bash
   # Restore Kubernetes resources
   kubectl apply -f backups/auth-k8s-20240101-120000.yaml
   
   # Restore secrets
   ./scripts/restore-secrets.sh --component auth
   
   # Validate configuration
   ./scripts/validate-auth-config.sh
   ```

### Business Continuity

1. **Failover Procedures**
   - Primary to secondary region: < 5 minutes
   - Automatic health checks and routing
   - Zero-downtime configuration updates

2. **SLA Targets**
   - Authentication availability: 99.99%
   - Authentication latency P95: < 100ms
   - Rate limit accuracy: 99.9%

3. **Regular Drills**
   - Monthly failover tests
   - Quarterly disaster recovery drills

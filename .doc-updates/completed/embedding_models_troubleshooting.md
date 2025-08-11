<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:40:31
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Embedding Model Management - Troubleshooting Guide

## Table of Contents
- [Common Issues](#common-issues)
- [Diagnostic Commands](#diagnostic-commands)
- [Error Messages](#error-messages)
- [Performance Issues](#performance-issues)
- [Integration Problems](#integration-problems)
- [Recovery Procedures](#recovery-procedures)

## Common Issues

### 1. Model Not Available

**Symptoms:**
- Error: "No embedding models available for tenant"
- 503 Service Unavailable responses
- WebSocket embedding requests failing <!-- Source: pkg/models/websocket/binary.go -->

**Causes:**
- No models configured for tenant
- All models have exceeded quotas
- Provider outage or circuit breaker open
- Models marked as unavailable in catalog

**Solutions:**
```bash
# Check tenant model configuration
psql -h localhost -U devmesh -d devmesh_development -c "
SELECT tem.*, emc.model_name, emc.is_available 
FROM tenant_embedding_models tem
JOIN embedding_model_catalog emc ON tem.model_id = emc.id
WHERE tem.tenant_id = 'YOUR_TENANT_ID';"

# Enable a model for tenant
curl -X POST http://localhost:8081/api/v1/tenant-models \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "MODEL_UUID",
    "enabled": true,
    "monthly_quota": 1000000
  }'

# Check circuit breaker status
redis-cli get "circuit:embedding:provider:openai"
```

### 2. Quota Exceeded Errors

**Symptoms:**
- 402 Payment Required responses
- "Quota exceeded" error messages
- Automatic fallback to lower-tier models

**Causes:**
- Daily or monthly token limits reached
- Rate limiting (RPM) exceeded
- Budget constraints hit

**Solutions:**
```bash
# Check current usage
curl http://localhost:8081/api/v1/tenant-models/usage \
  -H "Authorization: Bearer YOUR_API_KEY"

# View quota status
curl http://localhost:8081/api/v1/tenant-models/quotas \
  -H "Authorization: Bearer YOUR_API_KEY"

# Increase quotas
curl -X PUT http://localhost:8081/api/v1/tenant-models/MODEL_ID \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "monthly_quota": 5000000,
    "daily_quota": 200000
  }'

# Reset usage counters (development only)
redis-cli del "usage:tenant:YOUR_TENANT_ID:daily"
redis-cli del "usage:tenant:YOUR_TENANT_ID:monthly"
```

### 3. Model Selection Issues

**Symptoms:**
- Wrong model being selected
- Inconsistent model selection
- Performance degradation

**Causes:**
- Incorrect priority settings
- Cache inconsistency
- Model availability changes

**Solutions:**
```bash
# Clear model selection cache
redis-cli del "model:selection:tenant:YOUR_TENANT_ID"

# Check model priorities
psql -h localhost -U devmesh -d devmesh_development -c "
SELECT model_id, priority, enabled, is_default 
FROM tenant_embedding_models 
WHERE tenant_id = 'YOUR_TENANT_ID'
ORDER BY priority ASC;"

# Update model priority
curl -X PUT http://localhost:8081/api/v1/tenant-models/MODEL_ID \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{"priority": 10}'
```

### 4. Authentication Failures

**Symptoms:**
- 401 Unauthorized errors
- "Invalid API key" messages
- WebSocket connection rejected <!-- Source: pkg/models/websocket/binary.go -->

**Causes:**
- Expired or invalid API keys
- Missing tenant context
- Incorrect authentication headers

**Solutions:**
```bash
# Verify API key
curl http://localhost:8081/api/v1/embedding-models/catalog \
  -H "Authorization: Bearer YOUR_API_KEY" -v

# Check tenant association
psql -h localhost -U devmesh -d devmesh_development -c "
SELECT * FROM api_keys 
WHERE key_hash = encode(digest('YOUR_API_KEY', 'sha256'), 'hex');"

# Test WebSocket with auth <!-- Source: pkg/models/websocket/binary.go -->
wscat -c ws://localhost:8080/ws \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "X-Tenant-ID: YOUR_TENANT_ID"
```

## Diagnostic Commands

### Database Queries

```sql
-- Check model catalog health
SELECT provider, COUNT(*) as models, 
       SUM(CASE WHEN is_available THEN 1 ELSE 0 END) as available
FROM embedding_model_catalog
GROUP BY provider;

-- View recent usage
SELECT date_trunc('hour', created_at) as hour,
       model_id, COUNT(*) as requests, SUM(token_count) as tokens
FROM embedding_usage_tracking
WHERE tenant_id = 'YOUR_TENANT_ID'
  AND created_at > NOW() - INTERVAL '24 hours'
GROUP BY hour, model_id
ORDER BY hour DESC;

-- Find quota violations
SELECT tem.*, 
       COALESCE(daily.tokens, 0) as daily_used,
       COALESCE(monthly.tokens, 0) as monthly_used
FROM tenant_embedding_models tem
LEFT JOIN (
    SELECT tenant_id, model_id, SUM(token_count) as tokens
    FROM embedding_usage_tracking
    WHERE created_at > CURRENT_DATE
    GROUP BY tenant_id, model_id
) daily ON tem.tenant_id = daily.tenant_id AND tem.model_id = daily.model_id
LEFT JOIN (
    SELECT tenant_id, model_id, SUM(token_count) as tokens
    FROM embedding_usage_tracking
    WHERE created_at > DATE_TRUNC('month', CURRENT_DATE)
    GROUP BY tenant_id, model_id
) monthly ON tem.tenant_id = monthly.tenant_id AND tem.model_id = monthly.model_id
WHERE tem.tenant_id = 'YOUR_TENANT_ID';
```

### Redis Commands

```bash
# Monitor real-time activity
redis-cli monitor | grep embedding

# Check circuit breaker states
redis-cli keys "circuit:*" | xargs -I {} redis-cli get {}

# View cached selections
redis-cli keys "model:selection:*"

# Check rate limiting counters
redis-cli keys "rate:*" | head -20 | xargs -I {} redis-cli ttl {}
```

### Prometheus Queries

```promql
# Request rate by model
rate(embedding_requests_total[5m])

# Error rate
rate(embedding_requests_total{status="error"}[5m]) / rate(embedding_requests_total[5m])

# Cost tracking
increase(embedding_cost_usd_total[1h])

# Quota usage ratio
embedding_quota_usage_ratio

# Model availability
embedding_model_availability
```

## Error Messages

### HTTP Status Codes

| Status | Meaning | Action |
|--------|---------|--------|
| 400 | Bad Request | Check request format and parameters |
| 401 | Unauthorized | Verify API key and tenant context |
| 402 | Payment Required | Quota exceeded, check limits |
| 404 | Not Found | Model or resource doesn't exist |
| 429 | Too Many Requests | Rate limited, reduce request frequency |
| 503 | Service Unavailable | No models available, check configuration |

### Common Error Responses

```json
// Quota exceeded
{
  "error": "Quota exceeded",
  "quota_type": "daily",
  "limit": 50000,
  "used": 50123,
  "reset_time": "2024-01-15T00:00:00Z",
  "suggested_models": ["uuid-1", "uuid-2"]
}

// Model not found
{
  "error": "Model not found",
  "details": "Model abc-123 is not configured for tenant",
  "code": "MODEL_NOT_CONFIGURED"
}

// Circuit breaker open
{
  "error": "Provider unavailable",
  "details": "Circuit breaker is open for provider openai",
  "retry_after": 30
}
```

## Performance Issues

### Slow Embedding Generation

**Diagnosis:**
```bash
# Check latency metrics
curl http://localhost:9090/api/v1/query?query=embedding_latency_seconds

# Monitor provider response times
tail -f logs/rest-api.log | grep "embedding_duration"

# Check connection pool status
psql -h localhost -U devmesh -d devmesh_development -c "
SELECT state, COUNT(*) FROM pg_stat_activity 
GROUP BY state;"
```

**Solutions:**
- Enable request batching
- Increase connection pool size
- Use caching for repeated texts
- Switch to faster models for non-critical tasks

### High Memory Usage

**Diagnosis:**
```bash
# Check Redis memory
redis-cli info memory

# PostgreSQL cache hit ratio
psql -h localhost -U devmesh -d devmesh_development -c "
SELECT 
  sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as cache_hit_ratio
FROM pg_statio_user_tables;"
```

**Solutions:**
- Adjust Redis maxmemory settings
- Implement TTL on cached embeddings
- Reduce batch sizes
- Enable compression for large embeddings

## Integration Problems

### WebSocket Connection Issues <!-- Source: pkg/models/websocket/binary.go -->

```bash
# Test basic connectivity
wscat -c ws://localhost:8080/ws

# Check with authentication
wscat -c ws://localhost:8080/ws \
  -H "Authorization: Bearer YOUR_API_KEY"

# Monitor WebSocket traffic <!-- Source: pkg/models/websocket/binary.go -->
tcpdump -i lo -s 0 -A 'tcp port 8080'
```

### Provider API Failures

```bash
# Test OpenAI connectivity
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer YOUR_OPENAI_KEY"

# Test AWS Bedrock
aws bedrock list-foundation-models --region us-east-1

# Check provider credentials
grep -E "OPENAI_API_KEY|AWS_ACCESS_KEY" .env
```

## Recovery Procedures

### Reset Circuit Breakers

```bash
# Reset all circuit breakers
redis-cli keys "circuit:*" | xargs -I {} redis-cli del {}

# Reset specific provider
redis-cli del "circuit:embedding:provider:openai"
```

### Force Model Discovery

```bash
# Trigger manual discovery
curl -X POST http://localhost:8081/api/v1/admin/discover-models \
  -H "Authorization: Bearer ADMIN_KEY"

# Check discovery status
redis-cli xrange discovery_jobs - + COUNT 10
```

### Emergency Fallback

```bash
# Set emergency default model for all tenants
psql -h localhost -U devmesh -d devmesh_development << EOF
UPDATE tenant_embedding_models 
SET is_default = false;

UPDATE tenant_embedding_models 
SET is_default = true
WHERE model_id = (
  SELECT id FROM embedding_model_catalog 
  WHERE model_id = 'amazon.titan-embed-text-v1' 
  AND is_available = true
  LIMIT 1
);
EOF
```

### Database Maintenance

```bash
# Vacuum and analyze tables
psql -h localhost -U devmesh -d devmesh_development << EOF
VACUUM ANALYZE embedding_model_catalog;
VACUUM ANALYZE tenant_embedding_models;
VACUUM ANALYZE embedding_usage_tracking;
EOF

# Archive old usage data
psql -h localhost -U devmesh -d devmesh_development << EOF
INSERT INTO embedding_usage_archive 
SELECT * FROM embedding_usage_tracking 
WHERE created_at < NOW() - INTERVAL '90 days';

DELETE FROM embedding_usage_tracking 
WHERE created_at < NOW() - INTERVAL '90 days';
EOF
```

## Monitoring Checklist

### Daily Checks
- [ ] Review error rates in Grafana
- [ ] Check quota usage for large tenants
- [ ] Verify all providers are responsive
- [ ] Monitor cost trends

### Weekly Checks
- [ ] Review model performance metrics
- [ ] Check for deprecated models
- [ ] Analyze usage patterns
- [ ] Update model priorities based on performance

### Monthly Checks
- [ ] Archive old usage data
- [ ] Review and adjust quotas
- [ ] Update model catalog
- [ ] Cost optimization review

## Support Escalation

### Level 1: Application Logs
```bash
tail -f logs/rest-api.log | grep ERROR
tail -f logs/mcp-server.log | grep embedding
docker-compose logs -f rest-api --tail=100
```

### Level 2: Database Analysis
```sql
-- Check for locks
SELECT * FROM pg_locks WHERE NOT granted;

-- Long running queries
SELECT pid, now() - query_start as duration, query 
FROM pg_stat_activity 
WHERE state != 'idle' 
ORDER BY duration DESC;
```

### Level 3: Infrastructure
- Check AWS Bedrock service health
- Verify network connectivity to providers
- Review CloudWatch metrics
- Check for rate limiting at provider level

## Quick Reference

### Service Endpoints
- REST API: http://localhost:8081 (REST API)
- WebSocket: ws://localhost:8080 (MCP Server)/ws
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000

### Important Files
- Config: `configs/config.development.yaml`
- Logs: `logs/rest-api.log`, `logs/mcp-server.log`
- Migrations: `migrations/000015_tenant_embedding_models.up.sql`

### Key Environment Variables
```bash
DATABASE_URL=postgres://devmesh:devmesh@localhost:5432/devmesh_development
REDIS_ADDR=localhost:6379
OPENAI_API_KEY=sk-...
AWS_REGION=us-east-1
```

### Useful Make Commands
```bash
make test-embedding    # Run embedding tests
make migrate-up        # Apply migrations
make seed-models       # Seed model catalog
make logs             # Tail all service logs

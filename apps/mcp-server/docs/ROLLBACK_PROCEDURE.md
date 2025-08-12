# MCP Server Rollback Procedure

## Overview
This document outlines the rollback procedures for the MCP Server with Adaptive Protocol Intelligence Layer (APIL) in case of deployment issues or critical failures.

## Table of Contents
- [Automatic Rollback Triggers](#automatic-rollback-triggers)
- [Manual Rollback Procedures](#manual-rollback-procedures)
- [APIL-Specific Rollback](#apil-specific-rollback)
- [Data Recovery](#data-recovery)
- [Post-Rollback Verification](#post-rollback-verification)
- [Incident Response](#incident-response)

## Automatic Rollback Triggers

The system will automatically trigger a rollback when:

1. **Health Check Failures**
   - Service fails to respond to health checks after deployment
   - Critical components report unhealthy status
   - Circuit breakers remain open for extended periods

2. **APIL Migration Failures**
   - Migration progress stalls for more than 1 hour
   - Error rate exceeds 10% during migration
   - Protocol compatibility issues detected

3. **Performance Degradation**
   - Response time P95 > 5 seconds
   - Error rate > 5% for 5 minutes
   - Memory usage > 90% of available

## Manual Rollback Procedures

### Quick Rollback (< 5 minutes)

For immediate rollback to the previous version:

```bash
#!/bin/bash
# Quick rollback script

# 1. Stop the current service
sudo systemctl stop mcp-server

# 2. Restore previous binary from backup
LATEST_BACKUP=$(ls -t /opt/backups/mcp/ | head -1)
sudo cp /opt/backups/mcp/$LATEST_BACKUP/mcp-server /opt/mcp-server/

# 3. Restore configuration if needed
sudo cp -r /opt/backups/mcp/$LATEST_BACKUP/configs/* /opt/mcp-server/configs/

# 4. Restart service
sudo systemctl start mcp-server

# 5. Verify health
curl http://localhost:8080/health
```

### Standard Rollback (5-15 minutes)

For controlled rollback with verification:

```bash
#!/bin/bash
# Standard rollback procedure

set -e

echo "Starting MCP Server rollback..."

# 1. Create rollback checkpoint
ROLLBACK_DIR="/opt/backups/mcp/rollback_$(date +%Y%m%d_%H%M%S)"
mkdir -p $ROLLBACK_DIR

# 2. Save current state for analysis
sudo journalctl -u mcp-server -n 1000 > $ROLLBACK_DIR/service_logs.txt
curl -s http://localhost:8080/metrics > $ROLLBACK_DIR/metrics.json
curl -s http://localhost:8080/api/v1/apil/status > $ROLLBACK_DIR/apil_status.json

# 3. Identify target version
echo "Available backups:"
ls -la /opt/backups/mcp/
read -p "Enter backup directory name to restore: " BACKUP_VERSION

# 4. Perform graceful shutdown
echo "Gracefully shutting down current service..."
sudo systemctl stop mcp-server
sleep 5

# 5. Restore from backup
echo "Restoring from backup: $BACKUP_VERSION"
sudo cp /opt/backups/mcp/$BACKUP_VERSION/mcp-server /opt/mcp-server/
sudo cp -r /opt/backups/mcp/$BACKUP_VERSION/configs/* /opt/mcp-server/configs/

# 6. Clear cache if needed
redis-cli FLUSHDB

# 7. Start service
echo "Starting restored service..."
sudo systemctl start mcp-server

# 8. Wait for service to be ready
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s http://localhost:8080/health | grep -q "healthy"; then
        echo "Service is healthy"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo "Waiting for service... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

# 9. Verify functionality
echo "Running verification tests..."
./scripts/verify_deployment.sh

echo "Rollback completed successfully"
```

### Database Rollback

If database schema changes need to be reverted:

```bash
# 1. Check current migration version
make migrate-status

# 2. Rollback to specific version
make migrate-down VERSION=20240101000000

# 3. Verify schema
psql -h localhost -U devmesh -d devmesh_development -c "\dt mcp.*"
```

## APIL-Specific Rollback

### Disable APIL Without Full Rollback

To disable APIL while keeping the current version:

```bash
# 1. Update environment variables
sudo sed -i 's/ENABLE_APIL=true/ENABLE_APIL=false/' /etc/systemd/system/mcp-server.service

# 2. Reload and restart
sudo systemctl daemon-reload
sudo systemctl restart mcp-server

# 3. Verify APIL is disabled
curl http://localhost:8080/api/v1/apil/status
```

### Revert APIL Migration

To revert an in-progress APIL migration:

```bash
# 1. Set migration mode to rollback
export APIL_MIGRATION_MODE=rollback

# 2. Trigger rollback via API
curl -X POST http://localhost:8080/api/v1/apil/rollback \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"target_stage": "dual_protocol"}'

# 3. Monitor rollback progress
watch -n 5 'curl -s http://localhost:8080/api/v1/apil/status | jq .'
```

## Data Recovery

### Cache Recovery

If cache corruption is suspected:

```bash
# 1. Export current cache data (if possible)
redis-cli --rdb /tmp/redis_backup.rdb BGSAVE

# 2. Clear cache
redis-cli FLUSHDB

# 3. Restart service to rebuild cache
sudo systemctl restart mcp-server
```

### Context Recovery

For context data issues:

```sql
-- Connect to database
psql -h localhost -U devmesh -d devmesh_development

-- Check context integrity
SELECT COUNT(*), status FROM mcp.contexts GROUP BY status;

-- Restore contexts from backup if needed
\i /opt/backups/mcp/contexts_backup.sql
```

## Post-Rollback Verification

### Essential Checks

Run these checks after any rollback:

```bash
#!/bin/bash
# Post-rollback verification

echo "=== Post-Rollback Verification ==="

# 1. Service status
echo "1. Service Status:"
systemctl status mcp-server --no-pager | head -20

# 2. Health check
echo -e "\n2. Health Check:"
curl -s http://localhost:8080/health | jq '.'

# 3. WebSocket connectivity
echo -e "\n3. WebSocket Test:"
timeout 5 wscat -c ws://localhost:8080/ws \
  -H "Authorization: Bearer $API_KEY" \
  -x '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"1.0.0"},"id":"1"}' || echo "WebSocket test completed"

# 4. Tool availability
echo -e "\n4. Tool Availability:"
curl -s -X POST http://localhost:8080/ws \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":"2"}' | jq '.result.tools | length'

# 5. Database connectivity
echo -e "\n5. Database Check:"
psql -h localhost -U devmesh -d devmesh_development -c "SELECT version();" 2>/dev/null | head -3

# 6. Redis connectivity
echo -e "\n6. Redis Check:"
redis-cli ping

# 7. Error rate check
echo -e "\n7. Recent Errors (last 100 lines):"
sudo journalctl -u mcp-server -n 100 | grep -i error | tail -5 || echo "No recent errors"

# 8. Performance metrics
echo -e "\n8. Performance Metrics:"
curl -s http://localhost:8080/metrics | grep -E "mcp_request_duration_seconds|mcp_websocket_connections" | head -10
```

### Load Testing After Rollback

Run a quick load test to verify performance:

```bash
# Run k6 load test
k6 run test/load/simple_test.js

# Check results
cat summary.json | jq '.metrics | {
  "errors": .ws_errors.count,
  "connections": .ws_connections.count,
  "p95_latency": .init_latency_ms.p95
}'
```

## Incident Response

### Communication Protocol

1. **Immediate Actions**
   - Notify on-call team via PagerDuty/Slack
   - Create incident channel: `#incident-mcp-rollback-YYYYMMDD`
   - Post initial status in #engineering

2. **Status Updates**
   - Every 15 minutes during active incident
   - Include: Current state, ETA, impact assessment

3. **Post-Incident**
   - Schedule RCA meeting within 24 hours
   - Document lessons learned
   - Update rollback procedures if needed

### Rollback Decision Matrix

| Scenario | Automatic Rollback | Manual Rollback | Disable Feature |
|----------|-------------------|-----------------|-----------------|
| Health check failures (> 5 min) | ✓ | | |
| APIL migration stalled | | ✓ | |
| High error rate (> 10%) | ✓ | | |
| Performance degradation | | ✓ | |
| Circuit breaker issues | | | ✓ |
| Memory leak detected | | ✓ | |
| Protocol compatibility error | ✓ | | |
| Non-critical feature bug | | | ✓ |

### Emergency Contacts

- **On-Call Engineer**: Check PagerDuty rotation
- **Platform Team Lead**: via Slack #platform-team
- **Database Admin**: via Slack #database-team
- **Security Team**: security@company.com (for security-related rollbacks)

## Monitoring During Rollback

### Key Metrics to Watch

```bash
# Watch critical metrics during rollback
watch -n 5 'echo "=== MCP Server Metrics ===" && \
  curl -s http://localhost:8080/metrics | grep -E "up|error|latency" | head -10 && \
  echo -e "\n=== System Resources ===" && \
  top -bn1 | head -5 && \
  echo -e "\n=== Connection Count ===" && \
  netstat -an | grep :8080 | wc -l'
```

### Alert Silence

During rollback, silence non-critical alerts:

```bash
# Silence alerts for 1 hour
curl -X POST http://localhost:9093/api/v1/silences \
  -H "Content-Type: application/json" \
  -d '{
    "matchers": [{"name": "service", "value": "mcp-server"}],
    "startsAt": "2024-01-01T00:00:00.000Z",
    "endsAt": "2024-01-01T01:00:00.000Z",
    "createdBy": "rollback-procedure",
    "comment": "Silenced during rollback"
  }'
```

## Rollback Automation

### Automated Rollback Script

Save this as `/opt/mcp-server/rollback.sh`:

```bash
#!/bin/bash
# Automated rollback with safety checks

set -e

# Configuration
MAX_WAIT_TIME=300  # 5 minutes
HEALTH_CHECK_INTERVAL=10
LOG_FILE="/var/log/mcp-rollback-$(date +%Y%m%d_%H%M%S).log"

# Functions
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a $LOG_FILE
}

check_health() {
    curl -s http://localhost:8080/health | grep -q "healthy"
    return $?
}

perform_rollback() {
    local backup_dir=$1
    
    log "Starting rollback to: $backup_dir"
    
    # Stop service
    log "Stopping MCP server..."
    sudo systemctl stop mcp-server
    
    # Restore files
    log "Restoring from backup..."
    sudo cp $backup_dir/mcp-server /opt/mcp-server/
    sudo cp -r $backup_dir/configs/* /opt/mcp-server/configs/
    
    # Start service
    log "Starting MCP server..."
    sudo systemctl start mcp-server
    
    # Wait for health
    local wait_time=0
    while [ $wait_time -lt $MAX_WAIT_TIME ]; do
        if check_health; then
            log "Service is healthy after rollback"
            return 0
        fi
        sleep $HEALTH_CHECK_INTERVAL
        wait_time=$((wait_time + HEALTH_CHECK_INTERVAL))
        log "Waiting for service to be healthy... ($wait_time/$MAX_WAIT_TIME seconds)"
    done
    
    log "ERROR: Service did not become healthy after rollback"
    return 1
}

# Main execution
main() {
    log "=== MCP Server Automated Rollback ==="
    
    # Check if rollback is needed
    if check_health; then
        log "Service is healthy, no rollback needed"
        exit 0
    fi
    
    log "Service is unhealthy, initiating rollback"
    
    # Find latest backup
    LATEST_BACKUP=$(ls -t /opt/backups/mcp/ | head -1)
    if [ -z "$LATEST_BACKUP" ]; then
        log "ERROR: No backup found"
        exit 1
    fi
    
    # Perform rollback
    if perform_rollback "/opt/backups/mcp/$LATEST_BACKUP"; then
        log "Rollback completed successfully"
        
        # Send notification
        curl -X POST $SLACK_WEBHOOK_URL \
          -H "Content-Type: application/json" \
          -d "{\"text\": \"MCP Server rolled back successfully to $LATEST_BACKUP\"}"
        
        exit 0
    else
        log "ERROR: Rollback failed"
        
        # Send alert
        curl -X POST $PAGERDUTY_URL \
          -H "Content-Type: application/json" \
          -d "{\"event_action\": \"trigger\", \"payload\": {\"summary\": \"MCP Server rollback failed\", \"severity\": \"critical\"}}"
        
        exit 1
    fi
}

# Run main function
main
```

Make it executable and add to cron for automatic monitoring:

```bash
chmod +x /opt/mcp-server/rollback.sh

# Add to crontab for automatic checks every 5 minutes
echo "*/5 * * * * /opt/mcp-server/rollback.sh" | crontab -
```

## Testing Rollback Procedures

### Regular Rollback Drills

Schedule monthly rollback drills:

```bash
# Rollback drill script
#!/bin/bash

echo "=== MCP Rollback Drill ==="
echo "This is a TEST - no actual rollback will occur"

# 1. Simulate failure
echo "Simulating service failure..."
sudo systemctl stop mcp-server

# 2. Wait 10 seconds
sleep 10

# 3. Perform rollback
echo "Performing rollback..."
sudo systemctl start mcp-server

# 4. Verify
sleep 5
if curl -s http://localhost:8080/health | grep -q "healthy"; then
    echo "✓ Rollback drill successful"
else
    echo "✗ Rollback drill failed"
fi
```

## Summary

This rollback procedure ensures minimal downtime and data loss in case of deployment issues. Key points:

1. **Always backup before deployment** - Automated in deployment script
2. **Monitor key metrics** - Health, performance, and error rates
3. **Use gradual rollback** when possible - Disable features before full rollback
4. **Document incidents** - Learn from each rollback event
5. **Test procedures regularly** - Monthly drills recommended

For urgent assistance, contact the on-call engineer via PagerDuty.
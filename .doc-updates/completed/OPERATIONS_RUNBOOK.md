<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:33:07
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Operations Runbook

## Overview

This runbook provides operational procedures for managing Developer Mesh in production. It covers daily operations, incident response, maintenance tasks, and emergency procedures.

**Note**: Developer Mesh currently runs on Docker Compose on EC2 instances, not Kubernetes. Many procedures in this document represent future goals rather than current implementation.

## Table of Contents

1. [Daily Operations](#daily-operations)
2. [Deployment Procedures](#deployment-procedures)
3. [Backup and Restore](#backup-and-restore)
4. [Disaster Recovery](#disaster-recovery)
5. [Performance Tuning](#performance-tuning)
6. [Capacity Planning](#capacity-planning)
7. [Maintenance Procedures](#maintenance-procedures)
8. [Emergency Procedures](#emergency-procedures)
9. [Health Checks](#health-checks)

## Daily Operations

### Morning Checklist (Start of Day)

```bash
#!/bin/bash
# Daily operations checklist

echo "=== Developer Mesh Daily Operations Checklist ==="
echo "Date: $(date)"

# 1. Check system health
echo -n "1. System Health Check... "
if curl -s http://localhost:8080/health | jq -r '.status' | grep -q "healthy"; then
    echo "✓ PASS"
else
    echo "✗ FAIL - Investigate immediately"
fi

# 2. Check REST API health
echo -n "2. REST API Health... "
if curl -s http://localhost:8081/health | jq -r '.status' | grep -q "healthy"; then
    echo "✓ PASS"
else
    echo "✗ FAIL - Check REST API status"
fi

# 3. Check Redis connection
echo -n "3. Redis Connection... "
if redis-cli ping | grep -q "PONG"; then
    echo "✓ PASS"
else
    echo "✗ FAIL - Check Redis status"
fi

# 4. Check disk space
echo -n "4. Disk Space... "
DISK_USAGE=$(df -h / | awk 'NR==2 {print $5}' | sed 's/%//')
if [ $DISK_USAGE -lt 80 ]; then
    echo "✓ PASS (${DISK_USAGE}% used)"
else
    echo "✗ WARNING - Disk usage at ${DISK_USAGE}%"
fi

# 5. Check error rates
echo -n "5. Error Rate (last hour)... "
ERROR_RATE=$(curl -s http://localhost:9090/api/v1/query?query='rate(http_requests_total{status=~"5.."}[1h])' | jq -r '.data.result[0].value[1]')
if (( $(echo "$ERROR_RATE < 0.01" | bc -l) )); then
    echo "✓ PASS (${ERROR_RATE})"
else
    echo "✗ WARNING - Error rate: ${ERROR_RATE}"
fi

# 6. Check API key expirations
echo "6. API Key Expirations:"
psql -h localhost -U mcp_user -d mcp -c "
    SELECT name, expires_at 
    FROM api_keys 
    WHERE expires_at < NOW() + INTERVAL '7 days' 
    AND revoked_at IS NULL
    ORDER BY expires_at;
"

echo "=== Checklist Complete ==="
```

### Service Status Monitoring

```bash
# Check all services (Docker Compose)
docker-compose -f docker-compose.production.yml ps

# Check service logs
docker-compose -f docker-compose.production.yml logs --tail=100 mcp-server
docker-compose -f docker-compose.production.yml logs --tail=100 rest-api
docker-compose -f docker-compose.production.yml logs --tail=100 worker

# Check resource usage
docker stats

# Check container health
docker inspect mcp-server | jq '.[0].State.Health'
```

### Database Operations

```sql
-- Daily database health queries

-- Check connection count
SELECT datname, count(*) 
FROM pg_stat_activity 
GROUP BY datname;

-- Check long-running queries
SELECT pid, now() - pg_stat_activity.query_start AS duration, query 
FROM pg_stat_activity 
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';

-- Check table sizes
SELECT schemaname AS table_schema,
       tablename AS table_name,
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 20;

-- Check index usage
SELECT schemaname, tablename, indexname, idx_scan
FROM pg_stat_user_indexes
ORDER BY idx_scan
LIMIT 20;
```

## Deployment Procedures

### Current Deployment Method

Developer Mesh is deployed using Docker Compose on EC2 instances. The deployment uses GitHub Actions to SSH into the EC2 instance and update containers.

#### Deployment Process

1. Code is pushed to main branch
2. GitHub Actions builds Docker images
3. Images are pushed to GitHub Container Registry
4. GitHub Actions SSHs to EC2 and runs deployment script
5. Docker Compose pulls new images and restarts services

#### Available Images

- `developer-mesh-mcp-server` - MCP protocol server (built locally)
- `developer-mesh-rest-api` - REST API service (built locally)
- `developer-mesh-worker` - Event processing worker (built locally)

All images:
- Support multiple architectures (amd64, arm64)
- Are signed with Sigstore Cosign for security
- Include SBOMs (Software Bill of Materials)
- Follow semantic versioning

#### Initial Deployment

```bash
#!/bin/bash
# Deploy using pre-built images

# 1. Set GitHub username (replace with actual username)
export GITHUB_USERNAME=your-github-username

# 2. Pull latest images
./scripts/pull-images.sh

# 3. Verify image signatures (optional but recommended)
cosign verify ghcr.io/${GITHUB_USERNAME}/developer-mesh-mcp-server:latest
cosign verify ghcr.io/${GITHUB_USERNAME}/developer-mesh-rest-api:latest
cosign verify ghcr.io/${GITHUB_USERNAME}/developer-mesh-worker:latest

# 4. Deploy using production docker-compose
docker-compose -f docker-compose.prod.yml up -d

# 5. Verify deployment
docker-compose -f docker-compose.prod.yml ps
./scripts/health-check.sh
```

#### Version Update Procedure

```bash
#!/bin/bash
# Update to a specific version

VERSION=$1
if [ -z "$VERSION" ]; then
    echo "Usage: ./update-version.sh VERSION"
    echo "Example: ./update-version.sh v1.2.3"
    exit 1
fi

echo "Updating Developer Mesh to version ${VERSION}"

# 1. Pull new version
GITHUB_USERNAME=your-github-username ./scripts/pull-images.sh ${VERSION}

# 2. Verify new images
docker images | grep developer-mesh | grep ${VERSION}

# 3. Update docker-compose file
export VERSION=${VERSION}

# 4. Stop current services
docker-compose -f docker-compose.prod.yml down

# 5. Start with new version
docker-compose -f docker-compose.prod.yml up -d

# 6. Verify update
docker-compose -f docker-compose.prod.yml ps
docker-compose -f docker-compose.prod.yml logs --tail=100

# 7. Health check
./scripts/health-check.sh
```

#### Docker Compose Deployment (Current)

```bash
# SSH to EC2 instance
ssh -i your-key.pem ec2-user@your-ec2-instance

# Navigate to deployment directory
cd /home/ec2-user/developer-mesh

# Pull latest images
docker-compose -f docker-compose.production.yml pull

# Restart services with new images
docker-compose -f docker-compose.production.yml up -d

# Verify deployment
docker-compose -f docker-compose.production.yml ps

# Check logs
docker-compose -f docker-compose.production.yml logs --tail=50
```

#### Future: Kubernetes Deployment (Not Implemented)

**Note**: Kubernetes deployment is planned but not currently implemented. The YAML below represents future architecture:

```yaml
# FUTURE: kubernetes/deployments/mcp-server.yaml
# This is aspirational - not currently used

#### Image Verification

```bash
#!/bin/bash
# Verify image integrity and scan for vulnerabilities

IMAGE=$1
if [ -z "$IMAGE" ]; then
    echo "Usage: ./verify-image.sh IMAGE"
    exit 1
fi

echo "Verifying image: ${IMAGE}"

# 1. Check signature
echo "Checking signature..."
cosign verify ${IMAGE}

# 2. Scan for vulnerabilities
echo "Scanning for vulnerabilities..."
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
    aquasec/trivy image ${IMAGE}

# 3. Inspect metadata
echo "Image metadata:"
docker inspect ${IMAGE} | jq '.[0].Config.Labels'

# 4. Check SBOM
echo "Extracting SBOM..."
cosign download sbom ${IMAGE} > sbom.json
```

#### Rolling Updates

```bash
#!/bin/bash
# Perform zero-downtime rolling update

SERVICE=$1
VERSION=$2

if [ -z "$SERVICE" ] || [ -z "$VERSION" ]; then
    echo "Usage: ./rolling-update.sh SERVICE VERSION"
    exit 1
fi

echo "Rolling update of ${SERVICE} to ${VERSION}"

# For Docker Swarm
docker service update \
    --image ghcr.io/${GITHUB_USERNAME}/developer-mesh-${SERVICE}:${VERSION} \
    --update-parallelism 1 \
    --update-delay 30s \
    mcp_${SERVICE}

# For Kubernetes
kubectl set image deployment/${SERVICE} \
    ${SERVICE}=ghcr.io/${GITHUB_USERNAME}/developer-mesh-${SERVICE}:${VERSION} \
    -n mcp-prod

kubectl rollout status deployment/${SERVICE} -n mcp-prod
```

## Backup and Restore

### Manual Backup (Current Process)

**Note**: Automated backups are not currently implemented. Use these manual procedures:

```bash
# SSH to EC2 instance
ssh -i your-key.pem ec2-user@your-ec2-instance

# Create backup directory
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
mkdir -p /tmp/backup_${TIMESTAMP}

# Backup PostgreSQL (RDS)
PGPASSWORD=$DB_PASSWORD pg_dump \
  -h your-rds-endpoint.rds.amazonaws.com \
  -U $DB_USER \
  -d $DB_NAME \
  -f /tmp/backup_${TIMESTAMP}/database.sql

# Backup application configuration
cp -r /home/ec2-user/developer-mesh/.env* /tmp/backup_${TIMESTAMP}/
cp -r /home/ec2-user/developer-mesh/configs /tmp/backup_${TIMESTAMP}/

# Upload to S3
aws s3 cp /tmp/backup_${TIMESTAMP}/ \
  s3://your-backup-bucket/backups/${TIMESTAMP}/ \
  --recursive
```

### Future: Automated Backups (Not Implemented)

```yaml
# FUTURE: This CronJob does not exist yet

### Manual Backup Procedure

```bash
#!/bin/bash
# Manual backup script

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/backups/mcp_${TIMESTAMP}"

echo "Starting manual backup at ${TIMESTAMP}"

# 1. Create backup directory
mkdir -p ${BACKUP_DIR}

# 2. Backup PostgreSQL
echo "Backing up PostgreSQL..."
pg_dump -h localhost -U mcp_user -d mcp -F c -f ${BACKUP_DIR}/postgres_mcp.dump

# 3. Backup Redis
echo "Backing up Redis..."
redis-cli BGSAVE
sleep 5
cp /var/lib/redis/dump.rdb ${BACKUP_DIR}/redis.rdb

# 4. Backup configuration files
echo "Backing up configuration..."
tar -czf ${BACKUP_DIR}/configs.tar.gz /etc/mcp/

# 5. Backup vector embeddings
echo "Backing up vector data..."
pg_dump -h localhost -U mcp_user -d mcp -t vector_embeddings -F c -f ${BACKUP_DIR}/vectors.dump

# 6. Record image versions
echo "Recording deployed image versions..."
docker images | grep developer-mesh | grep -v "<none>" > ${BACKUP_DIR}/image_versions.txt
docker-compose -f docker-compose.prod.yml ps --format json > ${BACKUP_DIR}/running_services.json

# 7. Upload to S3
echo "Uploading to S3..."
aws s3 cp ${BACKUP_DIR} s3://mcp-backups-prod/${TIMESTAMP}/ --recursive

# 8. Verify backup
echo "Verifying backup..."
aws s3 ls s3://mcp-backups-prod/${TIMESTAMP}/ --recursive

echo "Backup completed successfully"
```

### Restore Procedure

```bash
#!/bin/bash
# Restore from backup

BACKUP_DATE=$1
if [ -z "$BACKUP_DATE" ]; then
    echo "Usage: ./restore.sh YYYYMMDD_HHMMSS"
    exit 1
fi

echo "WARNING: This will restore from backup ${BACKUP_DATE}"
echo "This operation will overwrite current data!"
read -p "Are you sure? (yes/no): " CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo "Restore cancelled"
    exit 0
fi

# 1. Download backup from S3
echo "Downloading backup..."
aws s3 cp s3://mcp-backups-prod/${BACKUP_DATE}/ /tmp/restore/ --recursive

# 2. Stop services
echo "Stopping services..."
kubectl scale deployment mcp-server rest-api worker --replicas=0 -n mcp-prod

# 3. Restore PostgreSQL
echo "Restoring PostgreSQL..."
pg_restore -h localhost -U mcp_user -d mcp_test -c /tmp/restore/postgres_mcp.dump

# 4. Restore Redis
echo "Restoring Redis..."
redis-cli FLUSHALL
redis-cli SHUTDOWN
cp /tmp/restore/redis.rdb /var/lib/redis/dump.rdb
redis-server --daemonize yes

# 5. Restore configurations
echo "Restoring configurations..."
tar -xzf /tmp/restore/configs.tar.gz -C /

# 6. Restart services
echo "Restarting services..."
kubectl scale deployment mcp-server rest-api worker --replicas=3 -n mcp-prod

# 7. Verify restore
echo "Verifying restore..."
./health-check.sh

echo "Restore completed"
```

## Disaster Recovery

### Current DR Capabilities

**Warning**: Comprehensive disaster recovery is not fully implemented. Current capabilities:

- RDS automated backups (7-day retention)
- S3 cross-region replication (if configured)
- Manual EC2 snapshots (if taken)
- No automated failover
- No multi-region deployment

### Realistic Recovery Targets (Current State)

| Component | RTO (Recovery Time) | RPO (Point) | Priority |
|-----------|-------------------|-------------|----------|
| API Services | 2-4 hours | 24 hours | Critical |
| PostgreSQL | 1-2 hours | 24 hours | Critical |
| Redis Cache | 30 minutes | Data loss likely | High |
| S3 Storage | N/A | 0 (replicated) | High |
| Worker Services | 2-4 hours | 24 hours | Medium |

### Disaster Recovery Procedures

#### 1. Total System Failure (Manual Process)

```bash
#!/bin/bash
# Manual disaster recovery procedure

echo "=== Developer Mesh Disaster Recovery ==="
echo "Starting recovery at $(date)"

# 1. Launch new EC2 instance (manual)
echo "Step 1: Launch new EC2 instance from AWS Console"
echo "  - Use AMI: ami-xxxxxxxxx (your base AMI)"
echo "  - Instance type: t3.xlarge or larger"
echo "  - Security group: Allow ports 22, 80, 443, 8080, 8081"
echo "  - Key pair: Use existing or create new"

# 2. Restore RDS from snapshot
echo "Step 2: Restore RDS from snapshot"
echo "  - Use AWS Console to restore latest RDS snapshot"
echo "  - Update new RDS endpoint in .env file"

# 3. Setup application on new EC2
echo "Step 3: Install application"
ssh -i your-key.pem ec2-user@new-instance-ip << 'EOF'
  # Install Docker and Docker Compose
  sudo yum update -y
  sudo yum install -y docker git
  sudo service docker start
  sudo usermod -a -G docker ec2-user
  
  # Install Docker Compose
  sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
  sudo chmod +x /usr/local/bin/docker-compose
  
  # Clone repository
  git clone https://github.com/your-org/developer-mesh.git
  cd developer-mesh
  
  # Copy .env file (you'll need to create this)
  # Copy configs
  
  # Start services
  docker-compose -f docker-compose.production.yml up -d
EOF

# 4. Restore Redis from backup
echo "Step 4: Restoring Redis..."
kubectl exec -it redis-0 -n mcp-prod -- redis-cli --rdb /backup/redis.rdb

# 5. Verify services
echo "Step 5: Verifying services..."
kubectl wait --for=condition=ready pod -l app=mcp-server -n mcp-prod --timeout=300s
kubectl wait --for=condition=ready pod -l app=rest-api -n mcp-prod --timeout=300s
kubectl wait --for=condition=ready pod -l app=worker -n mcp-prod --timeout=300s

# 6. Run health checks
echo "Step 6: Running health checks..."
./health-check.sh

# 7. Update DNS
echo "Step 7: Updating DNS..."
./update-dns.sh --target=disaster-recovery

echo "Disaster recovery completed at $(date)"
```

#### 2. Database Failure Recovery

```bash
#!/bin/bash
# PostgreSQL failure recovery

# 1. Check if primary is down
if ! pg_isready -h primary.db.mcp; then
    echo "Primary database is down, initiating failover..."
    
    # 2. Promote standby to primary
    kubectl exec -it postgres-standby-0 -n mcp-prod -- pg_ctl promote
    
    # 3. Update connection strings
    kubectl set env deployment/mcp-server DATABASE_URL=postgres://user:pass@standby.db.mcp:5432/mcp
    kubectl set env deployment/rest-api DATABASE_URL=postgres://user:pass@standby.db.mcp:5432/mcp
    
    # 4. Restart services
    kubectl rollout restart deployment/mcp-server deployment/rest-api -n mcp-prod
    
    # 5. Set up new standby (when possible)
    echo "Remember to provision new standby database"
fi
```

#### 3. Region Failure

```yaml
# Multi-region failover configuration
apiVersion: v1
kind: ConfigMap
metadata:
  name: region-failover
data:
  primary_region: "us-east-1"
  secondary_region: "us-west-2"
  failover_script: |
    #!/bin/bash
    # Automated region failover
    
    PRIMARY_HEALTH=$(curl -s https://api-east.mcp.example.com/health | jq -r '.status')
    
    if [ "$PRIMARY_HEALTH" != "healthy" ]; then
        echo "Primary region unhealthy, failing over to secondary..."
        
        # Update Route53
        aws route53 change-resource-record-sets \
          --hosted-zone-id Z123456789 \
          --change-batch file://failover-west.json
        
        # Scale up secondary region
        kubectl --context=west-cluster scale deployment mcp-server rest-api --replicas=6
        
        # Notify team
        ./notify-oncall.sh "Region failover initiated: east -> west"
    fi
```

## Performance Tuning

### Current Performance Considerations

**Note**: Developer Mesh runs on a single EC2 instance, limiting scaling options. Performance tuning focuses on optimizing the single instance.

### Database Optimization (RDS)

```sql
-- Check slow queries
SELECT 
    calls,
    total_time,
    mean_time,
    query
FROM pg_stat_statements
WHERE mean_time > 100
ORDER BY mean_time DESC
LIMIT 10;

-- Optimize vacuum settings
ALTER TABLE contexts SET (autovacuum_vacuum_scale_factor = 0.1);
ALTER TABLE vector_embeddings SET (autovacuum_vacuum_scale_factor = 0.05);

-- Update statistics
ANALYZE contexts;
ANALYZE vector_embeddings;
ANALYZE api_keys;
```

### Application Tuning

```bash
# Update docker-compose.production.yml environment variables
services:
  mcp-server:
    environment:
      # Connection pool settings
      DB_MAX_CONNECTIONS: 50  # Adjust based on RDS instance
      DB_MAX_IDLE_CONNECTIONS: 10
      
      # Redis settings
      REDIS_POOL_SIZE: 20
      REDIS_MAX_RETRIES: 3
      
      # Server settings
      SERVER_READ_TIMEOUT: 30s
      SERVER_WRITE_TIMEOUT: 30s
      
      # Worker settings
      WORKER_POOL_SIZE: 10  # Limited by single instance
      
  # Resource limits (prevent OOM)
  deploy:
    resources:
      limits:
        memory: 4G
        cpus: '2'
      reservations:
        memory: 2G
        cpus: '1'
```

### Redis Optimization

```bash
# Redis performance tuning
redis-cli CONFIG SET maxmemory 4gb
redis-cli CONFIG SET maxmemory-policy allkeys-lru
redis-cli CONFIG SET tcp-keepalive 60
redis-cli CONFIG SET timeout 300

# Monitor Redis performance
redis-cli --latency
redis-cli --latency-history
redis-cli --bigkeys
```

### Load Testing

```javascript
// k6 load test for performance validation
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '5m', target: 100 },  // Ramp up
    { duration: '10m', target: 100 }, // Stay at 100 users
    { duration: '5m', target: 200 },  // Ramp to 200
    { duration: '10m', target: 200 }, // Stay at 200
    { duration: '5m', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function() {
  // Test context creation
  let contextRes = http.post('http://api.mcp.example.com/api/v1/contexts', 
    JSON.stringify({
      name: `perf-test-${__VU}-${__ITER}`,
      content: 'Performance test context',
    }),
    { headers: { 'Content-Type': 'application/json', 'X-API-Key': API_KEY } }
  );
  
  check(contextRes, {
    'context created': (r) => r.status === 201,
    'response time OK': (r) => r.timings.duration < 500,
  });
  
  sleep(1);
}
```

## Capacity Planning

### Resource Monitoring (Current Single-Instance)

```bash
#!/bin/bash
# Capacity monitoring for Docker Compose deployment

echo "=== MCP Capacity Report ==="
echo "Generated: $(date)"

# System resources
echo -e "\n## System Resources"
echo "CPU Cores: $(nproc)"
echo "Total Memory: $(free -h | grep Mem | awk '{print $2}')"
echo "Disk Space: $(df -h / | tail -1 | awk '{print $4}' ) available"

# Docker resource usage
echo -e "\n## Container Resources"
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}"

# EC2 instance metrics
echo -e "\n## EC2 Metrics"
aws cloudwatch get-metric-statistics \
  --namespace AWS/EC2 \
  --metric-name CPUUtilization \
  --dimensions Name=InstanceId,Value=$(ec2-metadata --instance-id | cut -d' ' -f2) \
  --statistics Average \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300

# Database Size
echo -e "\n## Database Growth"
psql -h localhost -U mcp_user -d mcp -c "
    SELECT 
        pg_database_size('mcp')/1024/1024 as size_mb,
        (SELECT count(*) FROM contexts) as context_count,
        (SELECT count(*) FROM vector_embeddings) as vector_count
"

# Storage Usage
echo -e "\n## Storage Usage"
df -h | grep -E '(Filesystem|/mnt/data|/var/lib)'

# Projection
echo -e "\n## 30-Day Projection"
echo "Based on current growth rate:"
echo "- CPU: +15% expected"
echo "- Memory: +20% expected"
echo "- Storage: +50GB expected"
echo "- Database: +10GB expected"
```

### Scaling Options (Limited by Single Instance)

```bash
# Current scaling options:

1. Vertical Scaling (Resize EC2)
   - Stop instance
   - Change instance type (e.g., t3.xlarge -> t3.2xlarge)
   - Start instance
   - Downtime: ~5-10 minutes

2. Optimize Container Resources
   - Adjust memory limits in docker-compose
   - Tune worker pool sizes
   - Optimize database connections

3. Future: Horizontal Scaling
   - Requires migration to ECS/Kubernetes
   - Or manual load balancer + multiple EC2 instances
```

### Future: Auto-scaling (Not Implemented)

```yaml
# FUTURE: This HPA configuration is aspirational
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
  - type: Pods
    pods:
      metric:
        name: http_requests_per_second
      target:
        type: AverageValue
        averageValue: "1000"
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
```

### Capacity Planning Spreadsheet

| Component | Current | 3-Month Target | 6-Month Target | Notes |
|-----------|---------|----------------|----------------|-------|
| API Pods | 6 | 10 | 15 | Scale based on request rate |
| Worker Pods | 4 | 6 | 10 | Scale based on queue depth |
| PostgreSQL | 500GB | 750GB | 1TB | Plan migration to larger instance |
| Redis Memory | 16GB | 32GB | 32GB | Implement data expiration |
| S3 Storage | 2TB | 5TB | 10TB | No action needed (elastic) |
| Network Bandwidth | 1Gbps | 2Gbps | 5Gbps | Upgrade network tier |

## Maintenance Procedures

### Scheduled Maintenance Window

```bash
#!/bin/bash
# Maintenance mode script

MAINTENANCE_DURATION=${1:-"30"} # Default 30 minutes

echo "Entering maintenance mode for ${MAINTENANCE_DURATION} minutes"

# 1. Enable maintenance mode
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: maintenance-mode
  namespace: mcp-prod
data:
  enabled: "true"
  message: "Developer Mesh is undergoing scheduled maintenance. We'll be back shortly."
  start_time: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  duration: "${MAINTENANCE_DURATION}m"
EOF

# 2. Drain traffic from nodes being maintained
kubectl cordon node-1 node-2
kubectl drain node-1 node-2 --ignore-daemonsets --delete-emptydir-data

# 3. Perform maintenance tasks
echo "Performing maintenance tasks..."

# Update system packages
ansible nodes -m apt -a "update_cache=yes upgrade=dist"

# Database maintenance
psql -h localhost -U mcp_user -d mcp -c "VACUUM ANALYZE;"
psql -h localhost -U mcp_user -d mcp -c "REINDEX DATABASE mcp;"

# Clear old logs
find /var/log/mcp -name "*.log" -mtime +30 -delete

# 4. Uncordon nodes
kubectl uncordon node-1 node-2

# 5. Disable maintenance mode
kubectl delete configmap maintenance-mode -n mcp-prod

echo "Maintenance completed"
```

### Zero-Downtime Deployment

```bash
#!/bin/bash
# Rolling deployment script using published images

SERVICE=$1
VERSION=$2
GITHUB_USERNAME=${GITHUB_USERNAME:-your-github-username}

if [ -z "$SERVICE" ] || [ -z "$VERSION" ]; then
    echo "Usage: ./deploy.sh SERVICE VERSION"
    echo "Example: ./deploy.sh mcp-server v1.2.3"
    exit 1
fi

echo "Deploying ${SERVICE} version ${VERSION}"

# 1. Pull and verify new image
echo "Pulling new image..."
docker pull ghcr.io/${GITHUB_USERNAME}/developer-mesh-${SERVICE}:${VERSION}

# 2. Verify image signature
echo "Verifying image signature..."
cosign verify ghcr.io/${GITHUB_USERNAME}/developer-mesh-${SERVICE}:${VERSION}

# 3. Update image in Kubernetes
kubectl set image deployment/${SERVICE} ${SERVICE}=ghcr.io/${GITHUB_USERNAME}/developer-mesh-${SERVICE}:${VERSION} -n mcp-prod

# 4. Check rollout status
kubectl rollout status deployment/${SERVICE} -n mcp-prod

# 5. Verify health
sleep 10
HEALTH=$(curl -s http://${SERVICE}.mcp.svc.cluster.local:8080/health | jq -r '.status')

if [ "$HEALTH" != "healthy" ]; then
    echo "Health check failed, rolling back..."
    kubectl rollout undo deployment/${SERVICE} -n mcp-prod
    exit 1
fi

# 6. Record deployment
echo "${SERVICE}:${VERSION}" >> /var/log/deployments.log

echo "Deployment successful"
```

### Certificate Renewal

```bash
#!/bin/bash
# TLS certificate renewal

echo "Checking certificate expiration..."

# Check current certificate
CERT_EXPIRY=$(echo | openssl s_client -servername api.mcp.example.com -connect api.mcp.example.com:443 2>/dev/null | openssl x509 -noout -dates | grep notAfter | cut -d= -f2)
EXPIRY_EPOCH=$(date -d "${CERT_EXPIRY}" +%s)
CURRENT_EPOCH=$(date +%s)
DAYS_LEFT=$(( ($EXPIRY_EPOCH - $CURRENT_EPOCH) / 86400 ))

echo "Certificate expires in ${DAYS_LEFT} days"

if [ $DAYS_LEFT -lt 30 ]; then
    echo "Renewing certificate..."
    
    # Request new certificate
    certbot certonly --webroot -w /var/www/certbot \
        -d api.mcp.example.com \
        -d mcp.example.com \
        --non-interactive \
        --agree-tos \
        --email ops@example.com
    
    # Update Kubernetes secret
    kubectl create secret tls mcp-tls-new \
        --cert=/etc/letsencrypt/live/api.mcp.example.com/fullchain.pem \
        --key=/etc/letsencrypt/live/api.mcp.example.com/privkey.pem \
        -n mcp-prod
    
    # Update ingress
    kubectl patch ingress mcp-ingress -n mcp-prod \
        -p '{"spec":{"tls":[{"secretName":"mcp-tls-new","hosts":["api.mcp.example.com"]}]}}'
    
    # Clean up old secret
    kubectl delete secret mcp-tls -n mcp-prod
    kubectl label secret mcp-tls-new name=mcp-tls -n mcp-prod
fi
```

## Emergency Procedures

### Service Degradation (Docker Compose)

```bash
#!/bin/bash
# Emergency response for service degradation

ALERT_TYPE=$1

case $ALERT_TYPE in
    "high_latency")
        echo "Responding to high latency alert..."
        # Restart services (limited options on single instance)
        docker-compose -f docker-compose.production.yml restart mcp-server
        # Clear Redis cache
        docker exec redis redis-cli FLUSHALL
        # Check memory usage
        free -h
        docker system prune -f
        # Notify team (manual process)
        echo "ALERT: High latency detected, services restarted"
        ;;
        
    "high_error_rate")
        echo "Responding to high error rate..."
        # Check logs
        docker-compose -f docker-compose.production.yml logs --tail=1000 > /tmp/error-logs.txt
        # Roll back if recent deployment
        CURRENT_VERSION=$(docker inspect mcp-server | jq -r '.[0].Config.Labels.version')
        echo "Current version: $CURRENT_VERSION"
        echo "To rollback: docker-compose pull with previous version tags"
        # Restart with increased logging
        docker-compose -f docker-compose.production.yml down
        LOG_LEVEL=debug docker-compose -f docker-compose.production.yml up -d
        ;;
        
    "database_connection_pool_exhausted")
        echo "Responding to database connection exhaustion..."
        # Increase connection pool
        kubectl set env deployment/mcp-server DB_MAX_CONNECTIONS=200 -n mcp-prod
        # Kill long-running queries
        psql -h localhost -U mcp_user -d mcp -c "
            SELECT pg_terminate_backend(pid) 
            FROM pg_stat_activity 
            WHERE datname = 'mcp' 
            AND pid <> pg_backend_pid() 
            AND state = 'active' 
            AND query_start < now() - interval '5 minutes';
        "
        ;;
esac
```

### Security Incident Response

```bash
#!/bin/bash
# Security incident response

INCIDENT_TYPE=$1

echo "Security incident detected: ${INCIDENT_TYPE}"
echo "Time: $(date)"

# 1. Isolate affected systems
case $INCIDENT_TYPE in
    "suspicious_api_activity")
        # Block suspicious IPs
        kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: emergency-block
  namespace: mcp-prod
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: trusted
EOF
        ;;
        
    "api_key_compromise")
        # Revoke all API keys
        psql -h localhost -U mcp_user -d mcp -c "
            UPDATE api_keys 
            SET revoked_at = NOW(), 
                revoked_reason = 'Security incident - bulk revocation' 
            WHERE revoked_at IS NULL;
        "
        ;;
esac

# 2. Preserve evidence
mkdir -p /security/incidents/$(date +%Y%m%d_%H%M%S)
kubectl logs --all-containers=true --prefix=true -n mcp-prod > /security/incidents/$(date +%Y%m%d_%H%M%S)/all-logs.txt

# 3. Notify security team
./notify-security.sh "Security incident: ${INCIDENT_TYPE}"

# 4. Enable enhanced monitoring
kubectl set env deployment/mcp-server SECURITY_MODE=enhanced -n mcp-prod
```

### Data Corruption Recovery

```bash
#!/bin/bash
# Data corruption recovery procedure

echo "Data corruption detected, starting recovery..."

# 1. Stop writes
kubectl scale deployment mcp-server rest-api --replicas=0 -n mcp-prod

# 2. Identify corruption extent
psql -h localhost -U mcp_user -d mcp -c "
    SELECT schemaname, tablename 
    FROM pg_tables 
    WHERE schemaname = 'public'
" | while read schema table; do
    echo "Checking $table..."
    pg_dump -h localhost -U mcp_user -d mcp -t $table --data-only > /tmp/check_$table.sql 2>&1
    if [ $? -ne 0 ]; then
        echo "Corruption detected in $table"
    fi
done

# 3. Restore from backup
echo "Restoring from last known good backup..."
./restore.sh $(aws s3 ls s3://mcp-backups-prod/ | grep -E 'PRE' | sort | tail -2 | head -1 | awk '{print $2}' | sed 's/\///')

# 4. Replay recent transactions from WAL
pg_basebackup -h standby.db.mcp -D /tmp/wal_replay -X stream -P

# 5. Verify data integrity
psql -h localhost -U mcp_user -d mcp -c "SELECT COUNT(*) FROM contexts;"
psql -h localhost -U mcp_user -d mcp -c "SELECT COUNT(*) FROM vector_embeddings;"

# 6. Resume services
kubectl scale deployment mcp-server rest-api --replicas=3 -n mcp-prod
```

## Health Checks

### Comprehensive Health Check Script

```bash
#!/bin/bash
# Comprehensive health check

echo "=== Developer Mesh Health Check ==="
echo "Timestamp: $(date)"

# Function to check endpoint
check_endpoint() {
    local name=$1
    local url=$2
    local expected=$3
    
    response=$(curl -s -o /dev/null -w "%{http_code}" $url)
    if [ "$response" == "$expected" ]; then
        echo "✓ $name: OK"
        return 0
    else
        echo "✗ $name: FAILED (got $response, expected $expected)"
        return 1
    fi
}

# API Health Checks
echo -e "\n## API Health"
check_endpoint "MCP Server Health" "http://localhost:8080/health" "200"
check_endpoint "REST API Health" "http://localhost:8081/health" "200"
# Note: Worker service typically doesn't expose health endpoint

# Database Health
echo -e "\n## Database Health"
if pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
    echo "✓ PostgreSQL: OK"
    
    # Check replication lag
    LAG=$(psql -h localhost -U mcp_user -d mcp -t -c "
        SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))::INT
    " 2>/dev/null || echo "N/A")
    
    if [ "$LAG" != "N/A" ] && [ "$LAG" -lt 60 ]; then
        echo "✓ Replication Lag: ${LAG}s"
    else
        echo "✗ Replication Lag: ${LAG}s (WARNING)"
    fi
else
    echo "✗ PostgreSQL: FAILED"
fi

# Redis Health
echo -e "\n## Redis Health"
if redis-cli ping > /dev/null 2>&1; then
    echo "✓ Redis: OK"
    
    # Check memory usage
    USED_MEMORY=$(redis-cli info memory | grep used_memory_human | cut -d: -f2 | tr -d '\r')
    echo "  Memory Usage: $USED_MEMORY"
else
    echo "✗ Redis: FAILED"
fi

# Docker Service Health
echo -e "\n## Docker Services"
UNHEALTHY=$(docker-compose -f docker-compose.production.yml ps | grep -v "Up" | grep -v "State" | wc -l)
if [ $UNHEALTHY -eq 0 ]; then
    echo "✓ All containers running"
else
    echo "✗ Unhealthy containers:"
    docker-compose -f docker-compose.production.yml ps | grep -v "Up" | grep -v "State"
fi

# Metrics Health
echo -e "\n## Metrics"
ERROR_RATE=$(curl -s "http://localhost:9090/api/v1/query?query=rate(http_requests_total{status=~\"5..\"}[5m])" | jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")
if [ "$ERROR_RATE" != "N/A" ]; then
    echo "✓ Error Rate: ${ERROR_RATE}"
else
    echo "✗ Error Rate: Unable to fetch"
fi

# Summary
echo -e "\n=== Health Check Complete ==="
```

### Monitoring Status (If Configured)

```bash
# Check if monitoring stack is running
if docker-compose -f docker-compose.production.yml ps | grep -q prometheus; then
    echo "✓ Prometheus is running"
    # Verify Prometheus targets
    curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .job, health: .health}'
else
    echo "✗ Prometheus not configured"
fi

if docker-compose -f docker-compose.production.yml ps | grep -q grafana; then
    echo "✓ Grafana is running"
    # Verify Grafana
    curl -s http://localhost:3000/api/health | jq '.database'
else
    echo "✗ Grafana not configured"
fi

# Note: Jaeger is not implemented
```

## Runbook Maintenance

This runbook should be reviewed and updated:
- **Monthly**: Review and update procedures
- **After Incidents**: Add new procedures based on lessons learned
- **Before Major Changes**: Update affected procedures
- **Quarterly**: Full review with operations team
- **On New Releases**: Update deployment procedures and image versions

### Version History
- **v2.0.0**: Updated to reflect actual Docker Compose deployment (not Kubernetes)
- **v1.1.0**: Added Docker registry deployment procedures
- **v1.0.0**: Initial runbook creation

Last Updated: 2024-01-23
Version: 2.0.0


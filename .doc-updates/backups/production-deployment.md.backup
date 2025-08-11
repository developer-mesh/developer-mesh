# Production Deployment Guide

> **Purpose**: Guide for deploying Developer Mesh platform to AWS using EC2 and Docker Compose
> **Audience**: DevOps engineers and system administrators deploying to production
> **Scope**: EC2 deployment, Docker Compose setup, AWS service configuration
> **Status**: Current deployment method uses EC2 with Docker Compose

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Architecture Overview](#architecture-overview)
3. [AWS Infrastructure Setup](#aws-infrastructure-setup)
4. [Service Deployment](#service-deployment)
5. [Configuration Management](#configuration-management)
6. [Security Hardening](#security-hardening)
7. [Monitoring and Alerts](#monitoring-and-alerts)
8. [Scaling Strategy](#scaling-strategy)
9. [Disaster Recovery](#disaster-recovery)
10. [Maintenance Procedures](#maintenance-procedures)

## Prerequisites

### Required Tools

```bash
# AWS CLI v2
aws --version  # Should be 2.x or higher

# Docker and Docker Compose
docker --version     # 24.0+
docker-compose --version  # 2.20+

# SSH client for EC2 access
ssh -V

# Go (for building from source)
go version  # 1.24 required

# Note: No Terraform or Kubernetes required
# Deployment uses Docker Compose on EC2
```

### AWS Account Setup

```bash
# Configure AWS credentials
aws configure --profile production
# AWS Access Key ID: [your-key]
# AWS Secret Access Key: [your-secret]
# Default region: us-east-1
# Default output format: json

# Verify access
aws sts get-caller-identity --profile production
```

### Required AWS Services

- **EC2**: Application servers
- **RDS PostgreSQL**: Primary database with pgvector
- **ElastiCache Redis**: Caching and session storage
- **S3**: Object storage for contexts
- **SQS**: Message queuing
- **Bedrock**: AI model access
- **ALB**: Load balancing
- **Route 53**: DNS management
- **CloudWatch**: Monitoring
- **VPC**: Network isolation

## Architecture Overview

### Current Production Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Route 53 (Optional)                           │
│                    (dev-mesh.io or your domain)                  │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────┴────────────────────────────────────────┐
│                   Elastic IP (54.86.185.227)                     │
│                    (Static IP for EC2)                           │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────┴────────────────────────────────────────┐
│                    EC2 Instance (t3.large)                       │
│                       Docker Host                                │
├──────────────────────────────────────────────────────────────────┤
│  Docker Compose Services:                                        │
│  - mcp-server (port 8080)                                        │
│  - rest-api (port 8081)                                          │
│  - worker (background processing)                                │
│  - nginx (reverse proxy, if used)                               │
└──────┬───────────────────────────────────────────────────────────┘
       │
       ├─── RDS PostgreSQL (with pgvector)                        
       ├─── ElastiCache Redis (via SSH tunnel from bastion)       
       ├─── S3 Bucket (sean-mcp-dev-contexts)                     
       ├─── SQS Queue (sean-mcp-test)                             
       └─── AWS Bedrock (embedding models)                        
```

**Note**: Current deployment does not use CloudFront, ALB, or Auto Scaling Groups. It's a single EC2 instance running Docker Compose.
                    │
    ┌───────────────┴─────────────────────────┐
    │                                         │
┌───┴──────────┐  ┌──────────────┐  ┌───────┴──────┐
│RDS PostgreSQL│  │ ElastiCache  │  │     S3       │
│  (Multi-AZ)  │  │Redis Cluster │  │   Buckets    │
│  + pgvector  │  │  (Multi-AZ)  │  │              │
└──────────────┘  └──────────────┘  └──────────────┘
```

### Network Architecture

```yaml
VPC:
  CIDR: 10.0.0.0/16
  
  Public Subnets:
    - 10.0.1.0/24 (AZ-1) # ALB, NAT Gateway
    - 10.0.2.0/24 (AZ-2) # ALB, NAT Gateway
    
  Private Subnets:
    - 10.0.10.0/24 (AZ-1) # Application servers
    - 10.0.20.0/24 (AZ-2) # Application servers
    
  Database Subnets:
    - 10.0.100.0/24 (AZ-1) # RDS, ElastiCache
    - 10.0.200.0/24 (AZ-2) # RDS, ElastiCache
```

## AWS Infrastructure Setup

**Note**: Infrastructure is currently set up manually via AWS Console. No Terraform code exists in the project.

### 1. EC2 Instance Setup

```bash
# Current production EC2 instance
Instance Type: t3.large (or appropriate size)
AMI: Amazon Linux 2023 or Ubuntu 22.04
Security Group: Allow ports 8080, 8081, 22
Elastic IP: Assigned for stable access

# SSH access (from CLAUDE.md)
export SSH_KEY_PATH=/path/to/your/key.pem
./scripts/ssh-to-ec2.sh

# On the EC2 instance
cd /home/ec2-user/developer-mesh
docker-compose ps
```

### 2. Manual VPC Configuration

If setting up a new environment:

1. Create VPC with CIDR 10.0.0.0/16
2. Create public subnet for EC2
3. Create private subnets for RDS and ElastiCache
4. Set up Internet Gateway for public subnet
5. Configure security groups:
   - EC2: Allow 22 (SSH), 8080 (MCP), 8081 (API)
   - RDS: Allow 5432 from EC2 security group
   - ElastiCache: Allow 6379 from bastion/EC2

### 3. RDS PostgreSQL Setup (Manual)

Create RDS instance via AWS Console:

```
Engine: PostgreSQL 15.x
Instance class: db.t3.medium or larger
Storage: 100 GB gp3
Multi-AZ: Yes (for production)
Database name: mcp
Master username: mcp_admin

Security:
- VPC: Your production VPC
- Subnet group: Private subnets
- Security group: Allow 5432 from EC2

Backup:
- Retention: 7-30 days
- Backup window: 03:00-04:00 UTC
```

After creation, enable pgvector:

```sql
-- Connect to RDS instance
psql -h your-rds-endpoint.rds.amazonaws.com -U mcp_admin -d mcp

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Run migrations
cd /path/to/developer-mesh
make migrate-up
```
```

### 4. ElastiCache Redis Setup (Manual)

Create ElastiCache cluster via AWS Console:

```
Engine: Redis 7.x
Node type: cache.t3.micro or larger
Number of replicas: 1 (for Multi-AZ)
Subnet group: Private subnets
Security group: Allow 6379 from bastion

Security:
- Encryption at rest: Yes
- Encryption in transit: Yes (if needed)
- AUTH token: Optional

Backup:
- Retention: 1-7 days
- Backup window: 03:00-05:00 UTC
```

**Important**: ElastiCache requires SSH tunnel for access:

```bash
# Run this before starting services
./scripts/aws/connect-elasticache.sh

# This creates SSH tunnel through bastion
# Local port 6379 -> ElastiCache endpoint
```
```

### 5. S3 Bucket Setup (Manual)

Create S3 bucket via AWS Console:

```
Bucket name: your-mcp-contexts (must be globally unique)
Region: us-east-1 (or your preferred region)

Settings:
- Versioning: Enabled
- Encryption: SSE-S3 (default)
- Public access: Block all public access

Bucket policy (example for IP restriction):
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::YOUR-ACCOUNT:role/YOUR-EC2-ROLE"
      },
      "Action": "s3:*",
      "Resource": [
        "arn:aws:s3:::your-mcp-contexts/*",
        "arn:aws:s3:::your-mcp-contexts"
      ],
      "Condition": {
        "IpAddress": {
          "aws:SourceIp": ["YOUR-EC2-IP/32"]
        }
      }
    }
  ]
}
```
```

### 6. SQS Queue Setup (Manual)

Create SQS queue via AWS Console:

```
Queue name: mcp-production-tasks
Type: Standard queue

Configuration:
- Visibility timeout: 30 seconds
- Message retention: 4 days (default)
- Maximum message size: 256 KB
- Receive message wait time: 20 seconds (long polling)

Access policy:
- Allow your EC2 instance role to send/receive/delete messages
```

### 7. AWS Bedrock Access

Ensure your AWS account has access to Bedrock:

```bash
# Request access to models in AWS Console
# Go to Bedrock > Model access
# Request access to:
- Amazon Titan Embeddings V1
- Amazon Titan Embeddings V2
- Cohere Embed models (if needed)
  })
  
  kms_master_key_id = "alias/aws/sqs"
  
  tags = {
    Name = "mcp-production-task-queue"
    Environment = "production"
  }
}

resource "aws_sqs_queue" "dlq" {
  name = "mcp-production-tasks-dlq"
  
  message_retention_seconds = 1209600 # 14 days
  
  tags = {
    Name = "mcp-production-dlq"
    Environment = "production"
  }
}
```

## Service Deployment

### 1. Building and Deploying to EC2

```bash
# On your local machine
# Build Docker images
make docker-build

# Tag images for deployment
docker tag mcp-server:latest mcp-server:production
docker tag rest-api:latest rest-api:production
docker tag worker:latest worker:production

# Save images (if transferring manually)
docker save mcp-server:production | gzip > mcp-server.tar.gz
docker save rest-api:production | gzip > rest-api.tar.gz
docker save worker:production | gzip > worker.tar.gz

# Transfer to EC2 (if not using registry)
scp -i $SSH_KEY_PATH *.tar.gz ec2-user@54.86.185.227:/home/ec2-user/
```

### 2. Docker Compose Configuration

**Production docker-compose.yml**:

```yaml
version: '3.8'

services:
  mcp-server:
    image: mcp-server:production
    ports:
      - "8080:8080"
    environment:
      - ENVIRONMENT=production
      - DATABASE_HOST=${RDS_ENDPOINT}
      - DATABASE_PASSWORD=${DB_PASSWORD}
      - REDIS_ADDR=127.0.0.1:6379  # Via SSH tunnel
      - S3_BUCKET=${S3_BUCKET}
      - AWS_REGION=us-east-1
      - JWT_SECRET=${JWT_SECRET}
      - BEDROCK_ENABLED=true
    restart: unless-stopped
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"

  rest-api:
    image: rest-api:production
    ports:
      - "8081:8081"
    environment:
      # Similar to mcp-server
    restart: unless-stopped

  worker:
    image: worker:production
    environment:
      - SQS_QUEUE_URL=${SQS_QUEUE_URL}
      # Other env vars
    restart: unless-stopped
```

### 3. Deployment Process

```bash
# SSH to EC2 instance
./scripts/ssh-to-ec2.sh

# On EC2 instance
cd /home/ec2-user/developer-mesh

# Load images (if transferred manually)
docker load < mcp-server.tar.gz
docker load < rest-api.tar.gz
docker load < worker.tar.gz

# Create .env file with production values
cat > .env << EOF
# Database
DATABASE_HOST=your-rds-endpoint.rds.amazonaws.com
DATABASE_PASSWORD=your-secure-password

# Redis (via SSH tunnel)
REDIS_ADDR=127.0.0.1:6379

# AWS
S3_BUCKET=your-mcp-contexts
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/YOUR-ACCOUNT/your-queue

# Secrets
JWT_SECRET=your-jwt-secret
ADMIN_API_KEY=your-admin-key
EOF

# Start services
docker-compose up -d

# Check status
docker-compose ps
docker-compose logs -f
```

### 4. Health Checks and Monitoring

```bash
# Check service health
curl http://localhost:8080/health
curl http://localhost:8081/health

# Monitor logs
docker-compose logs -f mcp-server
docker-compose logs -f worker

# Check resource usage
docker stats

# Setup CloudWatch agent (optional)
sudo yum install -y amazon-cloudwatch-agent
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-config-wizard
```

### 5. Systemd Service (Optional)

For automatic startup on reboot:

```bash
# Create systemd service
sudo tee /etc/systemd/system/developer-mesh.service << EOF
[Unit]
Description=Developer Mesh Docker Compose
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/home/ec2-user/developer-mesh
ExecStart=/usr/local/bin/docker-compose up -d
ExecStop=/usr/local/bin/docker-compose down
User=ec2-user

[Install]
WantedBy=multi-user.target
EOF

# Enable service
sudo systemctl enable developer-mesh
sudo systemctl start developer-mesh
```

## Configuration Management

### 1. Environment Variables

```bash
# Production environment file (.env)
ENVIRONMENT=production
LOG_LEVEL=info
LOG_FORMAT=json

# Database
DATABASE_HOST=your-rds.rds.amazonaws.com
DATABASE_PORT=5432
DATABASE_NAME=mcp
DATABASE_USER=mcp_admin
DATABASE_PASSWORD=${DB_PASSWORD}
DATABASE_SSL_MODE=require

# Redis (via SSH tunnel)
REDIS_ADDR=127.0.0.1:6379
USE_SSH_TUNNEL_FOR_REDIS=true

# AWS Services
AWS_REGION=us-east-1
S3_BUCKET=your-mcp-contexts
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/YOUR-ACCOUNT/your-queue

# Bedrock Configuration
BEDROCK_ENABLED=true
BEDROCK_SESSION_LIMIT=0.10
GLOBAL_COST_LIMIT=10.0

# Security
JWT_SECRET=${JWT_SECRET}
ADMIN_API_KEY=${ADMIN_API_KEY}
READER_API_KEY=${READER_API_KEY}
```

### 2. Secrets Management

```bash
# Option 1: Environment file on EC2 (current method)
# Store .env file with restricted permissions
chmod 600 .env

# Option 2: AWS Secrets Manager (future enhancement)
# Store sensitive values in Secrets Manager
aws secretsmanager create-secret \
  --name mcp/production/db-password \
  --secret-string "your-secure-password"

# Retrieve in application
DB_PASSWORD=$(aws secretsmanager get-secret-value \
  --secret-id mcp/production/db-password \
  --query SecretString --output text)

aws secretsmanager create-secret \
  --name mcp/production/jwt-secret \
  --secret-string "$(openssl rand -hex 32)"

# Rotate secrets
aws secretsmanager rotate-secret \
  --secret-id mcp/production/db-password \
  --rotation-lambda-arn arn:aws:lambda:us-east-1:123456789012:function:SecretsManagerRotation
```

### 3. Configuration Files

```bash
# Use configs/config.production.yaml
cp configs/config.production.yaml.example configs/config.production.yaml

# Update with production values
vim configs/config.production.yaml

# Key configuration sections:
# - database connection details
# - redis connection (via SSH tunnel)
# - AWS service endpoints
# - authentication keys
```

## Security Hardening

### 1. EC2 Security Group Configuration

```bash
# Security group for EC2 instance
# Inbound rules:
- Port 22 (SSH): Your IP only
- Port 8080 (MCP Server): 0.0.0.0/0 or specific IPs
- Port 8081 (REST API): 0.0.0.0/0 or specific IPs

# AWS CLI commands
aws ec2 authorize-security-group-ingress \
  --group-id sg-xxx \
  --protocol tcp \
  --port 22 \
  --cidr YOUR-IP/32

aws ec2 authorize-security-group-ingress \
  --group-id sg-xxx \
  --protocol tcp \
  --port 8080-8081 \
  --cidr 0.0.0.0/0
```

### 2. IAM Role for EC2 Instance

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::your-mcp-contexts/*",
        "arn:aws:s3:::your-mcp-contexts"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "sqs:SendMessage",
        "sqs:ReceiveMessage",
        "sqs:DeleteMessage",
        "sqs:GetQueueAttributes"
      ],
      "Resource": "arn:aws:sqs:us-east-1:*:your-queue"
    },
    {
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel"
      ],
      "Resource": "*"
    }
  ]
}
```

### 3. SSL/TLS Configuration (Optional)

For HTTPS access, use a reverse proxy like nginx:

```bash
# Install nginx on EC2
sudo yum install -y nginx

# Configure nginx as reverse proxy
sudo tee /etc/nginx/conf.d/mcp.conf << EOF
server {
    listen 443 ssl;
    server_name your-domain.com;
    
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
    }
    
    location /api/ {
        proxy_pass http://localhost:8081/;
    }
}
EOF

# Use Let's Encrypt for free SSL
sudo certbot --nginx -d your-domain.com
```

### 4. Application Security

```bash
# Use environment variables for secrets
export JWT_SECRET=$(openssl rand -hex 32)
export ADMIN_API_KEY=$(uuidgen)
export READER_API_KEY=$(uuidgen)

# Secure file permissions
chmod 600 .env
chmod 600 configs/config.production.yaml

# Regular security updates
sudo yum update -y
docker system prune -a

# Monitor for vulnerabilities
docker scan mcp-server:production
```

## Monitoring and Alerts

### 1. CloudWatch Monitoring

```bash
# Install CloudWatch agent
wget https://s3.amazonaws.com/amazoncloudwatch-agent/amazon_linux/amd64/latest/amazon-cloudwatch-agent.rpm
sudo rpm -U ./amazon-cloudwatch-agent.rpm

# Configure to send Docker logs
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-config-wizard

# Basic CloudWatch dashboard for EC2
# Monitor these metrics:
- EC2 CPU Utilization
- EC2 Network In/Out
- EC2 Disk Usage
- RDS CPU and connections
- ElastiCache CPU and evictions
- SQS messages and age
```

### 2. CloudWatch Alarms

```bash
# Create basic alarms via CLI

# EC2 CPU alarm
aws cloudwatch put-metric-alarm \
  --alarm-name ec2-high-cpu \
  --alarm-description "Alert when EC2 CPU exceeds 80%" \
  --metric-name CPUUtilization \
  --namespace AWS/EC2 \
  --statistic Average \
  --period 300 \
  --threshold 80 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=InstanceId,Value=i-xxx \
  --evaluation-periods 2

# RDS CPU alarm
aws cloudwatch put-metric-alarm \
  --alarm-name rds-high-cpu \
  --alarm-description "Alert when RDS CPU exceeds 80%" \
  --metric-name CPUUtilization \
  --namespace AWS/RDS \
  --statistic Average \
  --period 300 \
  --threshold 80 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=DBInstanceIdentifier,Value=mcp-production-db
```

### 3. Application Monitoring

```bash
# Monitor application logs
docker-compose logs -f --tail=100

# Check application metrics
curl http://localhost:8080/metrics
curl http://localhost:8081/metrics

# Monitor WebSocket connections
docker exec mcp-server sh -c 'netstat -an | grep :8080 | grep ESTABLISHED | wc -l'

# Database connections
docker exec -it postgres psql -U mcp_admin -d mcp -c "SELECT count(*) FROM pg_stat_activity;"
```

### 4. Log Management

```bash
# Docker logs configuration
# In docker-compose.yml:
logging:
  driver: json-file
  options:
    max-size: "100m"
    max-file: "10"

# Log rotation on host
sudo tee /etc/logrotate.d/docker-mcp << EOF
/var/lib/docker/containers/*/*.log {
  rotate 7
  daily
  compress
  size=100M
  missingok
  delaycompress
  copytruncate
}
EOF

# Search logs
docker-compose logs --tail=1000 | grep ERROR
docker-compose logs --since=1h | grep "agent_id"
```

## Scaling Strategy

### 1. Vertical Scaling (Current Approach)

```bash
# Resize EC2 instance when needed
# Stop instance
aws ec2 stop-instances --instance-ids i-xxx

# Change instance type
aws ec2 modify-instance-attribute \
  --instance-id i-xxx \
  --instance-type '{"Value": "t3.xlarge"}'

# Start instance
aws ec2 start-instances --instance-ids i-xxx
```

### 2. Future Horizontal Scaling

For horizontal scaling, consider:
- Multiple EC2 instances behind ALB
- Docker Swarm or manual container distribution
- Shared Redis/RDS for state
- SQS for work distribution

### 3. Database Scaling

```bash
# Scale RDS instance
aws rds modify-db-instance \
  --db-instance-identifier mcp-production-db \
  --db-instance-class db.t3.large \
  --apply-immediately

# Add read replica for reporting
aws rds create-db-instance-read-replica \
  --db-instance-identifier mcp-production-read \
  --source-db-instance-identifier mcp-production-db

# Monitor connections
SELECT count(*) FROM pg_stat_activity;
```

### 4. Cache Scaling

```bash
# Scale ElastiCache
aws elasticache modify-cache-cluster \
  --cache-cluster-id mcp-production-redis \
  --cache-node-type cache.t3.medium \
  --apply-immediately

# Monitor cache performance
redis-cli INFO stats
redis-cli INFO memory
```

## Disaster Recovery

### 1. Backup Strategy

```bash
# RDS automated backups (configured in console)
# Retention: 7-30 days
# Backup window: 03:00-04:00 UTC

# Manual RDS snapshot
aws rds create-db-snapshot \
  --db-instance-identifier mcp-production-db \
  --db-snapshot-identifier mcp-manual-$(date +%Y%m%d)

# S3 bucket versioning (enable in console)
# Lifecycle rules for cost optimization

# ElastiCache backup
aws elasticache create-snapshot \
  --cache-cluster-id mcp-production-redis \
  --snapshot-name redis-$(date +%Y%m%d)

# EC2 AMI backup
aws ec2 create-image \
  --instance-id i-xxx \
  --name "mcp-backup-$(date +%Y%m%d)" \
  --no-reboot
```

### 2. Recovery Procedures

```bash
# RDS recovery
aws rds restore-db-instance-from-db-snapshot \
  --db-instance-identifier mcp-production-restored \
  --db-snapshot-identifier mcp-manual-20241225

# Update connection string in .env
vim .env
# Update DATABASE_HOST

# Redis recovery
aws elasticache create-cache-cluster \
  --cache-cluster-id mcp-redis-restored \
  --snapshot-name redis-20241225

# EC2 recovery from AMI
aws ec2 run-instances \
  --image-id ami-xxx \
  --instance-type t3.large \
  --key-name your-key \
  --security-group-ids sg-xxx \
  --subnet-id subnet-xxx
```

### 3. Backup Validation

```bash
# Test restore procedure quarterly
# 1. Create test RDS instance from snapshot
# 2. Verify data integrity
# 3. Test application connectivity
# 4. Document recovery time

# Backup checklist
- [ ] RDS automated backups enabled
- [ ] S3 versioning enabled
- [ ] ElastiCache snapshots scheduled
- [ ] EC2 AMI created monthly
- [ ] Configuration files backed up
- [ ] Recovery procedures documented
```

## Maintenance Procedures

### 1. Application Updates

```bash
# Build new images
make docker-build

# Tag for production
docker tag mcp-server:latest mcp-server:v1.2.0
docker tag rest-api:latest rest-api:v1.2.0
docker tag worker:latest worker:v1.2.0

# Backup current state
docker-compose exec postgres pg_dump -U mcp_admin mcp > backup-$(date +%Y%m%d).sql

# Update with minimal downtime
docker-compose pull
docker-compose up -d --no-deps mcp-server
docker-compose up -d --no-deps rest-api
docker-compose up -d --no-deps worker

# Verify
docker-compose ps
curl http://localhost:8080/health
```

### 2. Database Maintenance

```bash
# Connect to RDS
psql -h your-rds.amazonaws.com -U mcp_admin -d mcp

# Run maintenance
VACUUM ANALYZE;

# Check table sizes
SELECT 
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

# Monitor connections
SELECT count(*), state 
FROM pg_stat_activity 
GROUP BY state;
```

### 3. Security Updates

```bash
# System updates
sudo yum update -y
sudo reboot  # if kernel updated

# Docker updates
sudo yum update docker -y
sudo systemctl restart docker

# Rotate secrets
export JWT_SECRET=$(openssl rand -hex 32)
export ADMIN_API_KEY=$(uuidgen)
# Update .env file

# Update Go dependencies
cd /home/ec2-user/developer-mesh
go get -u ./...
go mod tidy
make docker-build
```

### 4. Performance Tuning

```bash
# Monitor resource usage
docker stats
htop
iostat -x 1

# Tune Docker
sudo tee /etc/docker/daemon.json << EOF
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "10"
  },
  "storage-driver": "overlay2"
}
EOF

sudo systemctl restart docker

# Database connection pooling in app config
# Adjust in configs/config.production.yaml
database:
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
```

## Troubleshooting

### Common Issues

1. **ElastiCache Connection Failed**
   ```bash
   # Ensure SSH tunnel is running
   ./scripts/aws/connect-elasticache.sh
   # Check: lsof -i :6379
   ```

2. **High Memory Usage**
   ```bash
   # Check container memory
   docker stats
   # Restart specific service
   docker-compose restart mcp-server
   ```

3. **Database Connection Errors**
   ```bash
   # Check connection count
   psql -h RDS-ENDPOINT -U mcp_admin -d mcp
   SELECT count(*) FROM pg_stat_activity;
   # Terminate idle connections if needed
   ```

4. **SQS Processing Issues**
   ```bash
   # Check queue metrics in AWS Console
   # Restart worker if stuck
   docker-compose restart worker
   ```

## Cost Optimization

### 1. Right-Sizing

```bash
# Monitor actual usage
# EC2: Check CloudWatch CPU/Memory
# RDS: Check performance insights
# ElastiCache: Check memory usage

# Downsize if overprovisioned
aws ec2 modify-instance-attribute \
  --instance-id i-xxx \
  --instance-type '{"Value": "t3.medium"}'
```

### 2. Cost Saving Tips

```bash
# 1. Use Reserved Instances for steady workloads
# 2. Enable S3 lifecycle policies
# 3. Set Bedrock cost limits in config
# 4. Use CloudWatch to identify unused resources
# 5. Schedule dev/test environments to stop outside hours

# Stop EC2 instance when not needed (dev/test)
aws ec2 stop-instances --instance-ids i-xxx
```

## Next Steps

1. Set up monitoring and alerting
2. Implement backup automation
3. Document your specific configuration
4. Plan for scaling needs
5. Review security posture regularly

## Resources

### Project-Specific
- [CLAUDE.md](../../CLAUDE.md) - Production deployment notes
- [Docker Compose Reference](https://docs.docker.com/compose/)
- SSH script: `./scripts/ssh-to-ec2.sh`
- ElastiCache script: `./scripts/aws/connect-elasticache.sh`

### AWS Documentation
- [EC2 User Guide](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/)
- [RDS Best Practices](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_BestPractices.html)
- [ElastiCache Best Practices](https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/BestPractices.html)
- [S3 Security Best Practices](https://docs.aws.amazon.com/AmazonS3/latest/userguide/security-best-practices.html)

## Summary

This guide covers deploying Developer Mesh to AWS using:
- Single EC2 instance with Docker Compose
- RDS PostgreSQL with pgvector
- ElastiCache Redis (via SSH tunnel)
- S3 for object storage
- SQS for task queuing
- AWS Bedrock for embeddings

For high availability and scaling, consider future migration to:
- Multiple EC2 instances with load balancing
- Container orchestration (ECS/EKS)
- Auto-scaling groups
- Multi-region deployment
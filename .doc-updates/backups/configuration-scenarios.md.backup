# Configuration Scenarios Guide

> **Purpose**: Practical configuration examples for different Developer Mesh deployment scenarios
> **Audience**: Platform operators, DevOps engineers, and architects
> **Scope**: Real-world configuration templates for various use cases

## Table of Contents

1. [Overview](#overview)
2. [Development Environment](#development-environment)
3. [Small Team Setup](#small-team-setup)
4. [Enterprise Deployment](#enterprise-deployment)
5. [High-Performance Configuration](#high-performance-configuration)
6. [Cost-Optimized Setup](#cost-optimized-setup)
7. [Multi-Region Configuration](#multi-region-configuration)
8. [Hybrid Cloud Setup](#hybrid-cloud-setup)
9. [Security-Hardened Configuration](#security-hardened-configuration)
10. [AI Research Platform](#ai-research-platform)
11. [Edge Computing Scenario](#edge-computing-scenario)
12. [Migration Scenarios](#migration-scenarios)

## Overview

This guide provides battle-tested configuration templates for various Developer Mesh deployment scenarios. Each configuration is optimized for specific requirements and constraints.

### Configuration Philosophy

1. **Start Simple**: Begin with minimal configuration and scale up
2. **Measure First**: Use metrics to guide configuration changes
3. **Iterate Quickly**: Small, frequent adjustments beat big changes
4. **Document Everything**: Track why each setting was chosen

## Development Environment

### Scenario
Single developer working locally with real AWS services for testing.

### Configuration Files

#### `.env.development`
```bash
# Development Environment Configuration
# =====================================

# Core Settings
ENVIRONMENT=development
LOG_LEVEL=debug
DEBUG_MODE=true

# AWS Configuration
AWS_REGION=us-east-1
AWS_PROFILE=dev-profile

# Service URLs (Local with AWS Backend)
MCP_SERVER_URL=ws://localhost:8080
REST_API_URL=http://localhost:8081
WORKER_URL=http://localhost:8082

# Database (Local PostgreSQL)
DATABASE_URL=postgresql://devuser:devpass@localhost:5432/mcp_dev
DB_POOL_SIZE=5
DB_MAX_CONNECTIONS=10

# Redis (Local via Docker)
REDIS_ADDR=127.0.0.1:6379
USE_SSH_TUNNEL_FOR_REDIS=false
REDIS_POOL_SIZE=10

# AI Models (Limited for Dev)
BEDROCK_ENABLED=true
BEDROCK_SESSION_LIMIT=0.05    # $0.05 per session
GLOBAL_COST_LIMIT=1.00        # $1.00 daily limit
DEFAULT_EMBEDDING_MODEL=amazon.titan-embed-text-v1

# Development Features
ENABLE_PROFILING=true
ENABLE_TRACE_SAMPLING=1.0     # 100% sampling in dev
ENABLE_METRICS=true
METRICS_PORT=9090

# Security (Relaxed for Dev)
JWT_SECRET=dev-secret-change-in-prod
API_KEY_VALIDATION=false
CORS_ALLOWED_ORIGINS=*
```

#### `docker-compose.dev.yml`
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: devuser
      POSTGRES_PASSWORD: devpass
      POSTGRES_DB: mcp_dev
    ports:
      - "5432:5432"
    volumes:
      - ./data/postgres:/var/lib/postgresql/data
    command: >
      postgres
      -c shared_preload_libraries=pg_stat_statements,vector
      -c pg_stat_statements.track=all
      -c log_statement=all

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: >
      redis-server
      --maxmemory 256mb
      --maxmemory-policy allkeys-lru
      --save ""

  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # UI
      - "14268:14268"  # Collector
    environment:
      COLLECTOR_ZIPKIN_HTTP_PORT: 9411
```

#### `Makefile.dev`
```makefile
# Development Makefile
.PHONY: dev dev-deps dev-clean

dev-deps:
	docker-compose -f docker-compose.dev.yml up -d
	@echo "Waiting for services..."
	@sleep 5
	@echo "Running migrations..."
	migrate -path ./migrations -database $${DATABASE_URL} up

dev: dev-deps
	@echo "Starting development servers..."
	@goreman start -f Procfile.dev

dev-clean:
	docker-compose -f docker-compose.dev.yml down -v
	rm -rf ./data
```

## Small Team Setup

### Scenario
5-10 developers, staging environment, moderate traffic.

### Configuration Files

#### `.env.staging`
```bash
# Small Team Staging Configuration
# =================================

# Core Settings
ENVIRONMENT=staging
LOG_LEVEL=info
DEBUG_MODE=false

# AWS Configuration
AWS_REGION=us-east-1
AWS_ACCOUNT_ID=123456789012

# Service Discovery
SERVICE_DISCOVERY_NAMESPACE=mcp-staging
USE_SERVICE_DISCOVERY=true

# Database (RDS)
DATABASE_URL=postgresql://mcp_user:${DB_PASSWORD}@mcp-staging.cluster-abc123.us-east-1.rds.amazonaws.com:5432/mcp_staging
DB_POOL_SIZE=20
DB_MAX_CONNECTIONS=50
DB_STATEMENT_TIMEOUT=30s

# Redis (ElastiCache)
REDIS_ADDR=mcp-staging.abc123.cache.amazonaws.com:6379
REDIS_CLUSTER_MODE=false
REDIS_POOL_SIZE=50
REDIS_TLS_ENABLED=true

# SQS Configuration
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789012/mcp-staging-tasks
SQS_DLQ_URL=https://sqs.us-east-1.amazonaws.com/123456789012/mcp-staging-tasks-dlq
SQS_VISIBILITY_TIMEOUT=300
SQS_MAX_MESSAGES=10

# AI Models (Controlled Costs)
BEDROCK_ENABLED=true
BEDROCK_SESSION_LIMIT=0.25
GLOBAL_COST_LIMIT=50.00
EMBEDDING_CACHE_ENABLED=true
MODEL_SELECTION_STRATEGY=cost_optimized

# Performance Settings
HTTP_TIMEOUT=30s
WORKER_CONCURRENCY=10
MAX_REQUEST_SIZE=10MB
ENABLE_GZIP=true

# Monitoring
ENABLE_METRICS=true
METRICS_INTERVAL=60s
ENABLE_TRACING=true
TRACE_SAMPLING_RATE=0.1  # 10% sampling

# Security
JWT_EXPIRY=24h
SESSION_TIMEOUT=8h
RATE_LIMIT_ENABLED=true
RATE_LIMIT_RPS=100
```

#### `terraform/staging/main.tf`
```hcl
# Small Team Infrastructure
module "vpc" {
  source = "../modules/vpc"
  
  environment = "staging"
  cidr_block  = "10.1.0.0/16"
  az_count    = 2
  
  enable_nat_gateway = true
  single_nat_gateway = true  # Cost savings
}

module "ecs_cluster" {
  source = "../modules/ecs"
  
  cluster_name = "mcp-staging"
  
  capacity_providers = ["FARGATE_SPOT", "FARGATE"]
  
  default_capacity_provider_strategy = [{
    capacity_provider = "FARGATE_SPOT"
    weight           = 80
    base             = 0
  }, {
    capacity_provider = "FARGATE"
    weight           = 20
    base             = 1
  }]
}

module "rds" {
  source = "../modules/rds"
  
  identifier     = "mcp-staging"
  engine_version = "15.3"
  instance_class = "db.t4g.medium"
  
  allocated_storage     = 100
  max_allocated_storage = 200
  
  backup_retention_period = 7
  backup_window          = "03:00-04:00"
  
  multi_az = false  # Cost savings for staging
}

module "elasticache" {
  source = "../modules/elasticache"
  
  cluster_id           = "mcp-staging"
  node_type           = "cache.t4g.small"
  num_cache_nodes     = 1
  engine_version      = "7.0"
  
  automatic_failover_enabled = false
  snapshot_retention_limit   = 3
}
```

## Enterprise Deployment

### Scenario
100+ developers, multiple teams, high availability requirements.

### Configuration Files

#### `.env.production`
```bash
# Enterprise Production Configuration
# ====================================

# Core Settings
ENVIRONMENT=production
LOG_LEVEL=warn
DEBUG_MODE=false
SERVICE_NAME=mcp-platform

# Multi-Region Setup
AWS_REGIONS=us-east-1,us-west-2,eu-west-1
PRIMARY_REGION=us-east-1
ENABLE_CROSS_REGION_REPLICATION=true

# Database (Multi-AZ RDS)
DATABASE_URL=postgresql://mcp_prod:${DB_PASSWORD}@mcp-prod.cluster-xyz789.us-east-1.rds.amazonaws.com:5432/mcp_production
DB_POOL_SIZE=100
DB_MAX_CONNECTIONS=500
DB_STATEMENT_TIMEOUT=10s
READ_REPLICA_URLS=postgresql://reader1.xyz789.us-east-1.rds.amazonaws.com,postgresql://reader2.xyz789.us-east-1.rds.amazonaws.com

# Redis (ElastiCache Cluster Mode)
REDIS_CLUSTER_ENDPOINTS=mcp-prod-001.abc.cache.amazonaws.com:6379,mcp-prod-002.abc.cache.amazonaws.com:6379,mcp-prod-003.abc.cache.amazonaws.com:6379
REDIS_CLUSTER_MODE=true
REDIS_POOL_SIZE=200
REDIS_TLS_ENABLED=true
REDIS_AUTH_TOKEN=${REDIS_AUTH_TOKEN}

# Message Queue (SQS FIFO)
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/987654321098/mcp-prod-tasks.fifo
SQS_DLQ_URL=https://sqs.us-east-1.amazonaws.com/987654321098/mcp-prod-tasks-dlq.fifo
SQS_VISIBILITY_TIMEOUT=600
SQS_MAX_MESSAGES=10
SQS_LONG_POLLING_WAIT=20

# AI Models (Enterprise Scale)
BEDROCK_ENABLED=true
BEDROCK_SESSION_LIMIT=10.00
GLOBAL_COST_LIMIT=5000.00
MODEL_SELECTION_STRATEGY=performance_optimized
EMBEDDING_CACHE_SIZE=10GB
EMBEDDING_PRECOMPUTE_ENABLED=true

# Performance Settings
HTTP_TIMEOUT=10s
WORKER_CONCURRENCY=50
MAX_REQUEST_SIZE=50MB
CONNECTION_POOL_SIZE=1000
ENABLE_HTTP2=true

# High Availability
HEALTH_CHECK_INTERVAL=10s
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_BREAKER_THRESHOLD=0.5
RETRY_MAX_ATTEMPTS=3
RETRY_BACKOFF_MULTIPLIER=2

# Security
JWT_ROTATION_ENABLED=true
JWT_KEY_ID=prod-2024-01
SESSION_STORE=redis_cluster
ENCRYPTION_AT_REST=true
WAF_ENABLED=true
DDoS_PROTECTION=true

# Monitoring
ENABLE_METRICS=true
METRICS_DETAILED=true
ENABLE_TRACING=true
TRACE_SAMPLING_RATE=0.01  # 1% sampling
ENABLE_PROFILING=false
LOG_AGGREGATION=cloudwatch
CUSTOM_METRICS_NAMESPACE=MCP/Production

# Compliance
ENABLE_AUDIT_LOG=true
AUDIT_LOG_RETENTION=2555  # 7 years
PII_DETECTION_ENABLED=true
DATA_CLASSIFICATION_ENABLED=true
```

#### `terraform/production/main.tf`
```hcl
# Enterprise Infrastructure
module "multi_region_vpc" {
  source = "../modules/multi-region-vpc"
  
  regions = var.aws_regions
  
  vpc_configs = {
    us-east-1 = {
      cidr_block = "10.0.0.0/16"
      az_count   = 3
    }
    us-west-2 = {
      cidr_block = "10.1.0.0/16"
      az_count   = 3
    }
    eu-west-1 = {
      cidr_block = "10.2.0.0/16"
      az_count   = 3
    }
  }
  
  enable_vpc_peering = true
  enable_transit_gateway = true
}

module "global_accelerator" {
  source = "../modules/global-accelerator"
  
  name            = "mcp-global"
  ip_address_type = "IPV4"
  
  listeners = [{
    port     = 443
    protocol = "TCP"
  }]
  
  endpoint_groups = [
    {
      region = "us-east-1"
      endpoints = module.alb_us_east_1.endpoints
    },
    {
      region = "us-west-2"
      endpoints = module.alb_us_west_2.endpoints
    },
    {
      region = "eu-west-1"
      endpoints = module.alb_eu_west_1.endpoints
    }
  ]
}

module "rds_global_cluster" {
  source = "../modules/rds-aurora-global"
  
  global_cluster_identifier = "mcp-global-db"
  engine                   = "aurora-postgresql"
  engine_version          = "15.2"
  
  primary_cluster = {
    region         = "us-east-1"
    instance_count = 3
    instance_class = "db.r6g.2xlarge"
  }
  
  secondary_clusters = [
    {
      region         = "us-west-2"
      instance_count = 2
      instance_class = "db.r6g.xlarge"
    },
    {
      region         = "eu-west-1"
      instance_count = 2
      instance_class = "db.r6g.xlarge"
    }
  ]
  
  backup_retention_period = 35
  enable_backtrack       = true
  backtrack_window       = 72
}
```

## High-Performance Configuration

### Scenario
Maximum performance, low latency requirements, cost is secondary.

### Configuration Files

#### `.env.performance`
```bash
# High-Performance Configuration
# ==============================

# Optimized for Speed
ENVIRONMENT=production-performance
GO_MAX_PROCS=0  # Use all CPUs
GOGC=100        # Aggressive GC

# Network Optimization
TCP_NODELAY=true
TCP_QUICKACK=true
KEEP_ALIVE_INTERVAL=30s
HTTP2_ENABLED=true
HTTP2_MAX_CONCURRENT_STREAMS=1000

# Database Performance
DB_POOL_SIZE=500
DB_MAX_CONNECTIONS=1000
DB_ACQUIRE_TIMEOUT=1s
DB_STATEMENT_CACHE_SIZE=1000
DB_PREPARED_STATEMENTS=true
CONNECTION_LIFETIME=1h

# Redis Performance
REDIS_POOL_SIZE=1000
REDIS_PIPELINE_WINDOW=100ms
REDIS_PIPELINE_LIMIT=1000
REDIS_READ_TIMEOUT=100ms
REDIS_WRITE_TIMEOUT=100ms

# Caching
L1_CACHE_SIZE=4GB
L1_CACHE_TTL=5m
L2_CACHE_ENABLED=true
L3_CACHE_ENABLED=true
CACHE_WARMING_ENABLED=true
CACHE_PRELOAD_POPULAR=true

# AI Model Performance
MODEL_SELECTION_STRATEGY=performance_only
ENABLE_MODEL_CACHING=true
MODEL_CACHE_SIZE=50GB
BATCH_SIZE_EMBEDDINGS=100
BATCH_SIZE_INFERENCE=50
PARALLEL_MODEL_CALLS=10

# Worker Performance
WORKER_POOL_SIZE=200
TASK_PREFETCH_COUNT=50
TASK_PROCESSING_TIMEOUT=30s
ENABLE_TASK_PRIORITIES=true

# WebSocket Performance
WS_BUFFER_SIZE=65536
WS_COMPRESSION_LEVEL=6
WS_MAX_CONNECTIONS=50000
WS_PING_INTERVAL=30s
```

#### `performance-tuning.yaml`
```yaml
# Kubernetes Performance Configuration
apiVersion: v1
kind: ConfigMap
metadata:
  name: performance-tuning
data:
  nginx.conf: |
    worker_processes auto;
    worker_rlimit_nofile 65535;
    
    events {
        worker_connections 4096;
        use epoll;
        multi_accept on;
    }
    
    http {
        sendfile on;
        tcp_nopush on;
        tcp_nodelay on;
        keepalive_timeout 65;
        keepalive_requests 100;
        
        gzip on;
        gzip_comp_level 6;
        gzip_types text/plain application/json;
        
        upstream backend {
            least_conn;
            keepalive 32;
            
            server mcp-server-1:8080 max_fails=3 fail_timeout=30s;
            server mcp-server-2:8080 max_fails=3 fail_timeout=30s;
            server mcp-server-3:8080 max_fails=3 fail_timeout=30s;
        }
    }

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-server-performance
spec:
  replicas: 10
  template:
    spec:
      nodeSelector:
        node.kubernetes.io/instance-type: c6i.8xlarge
      
      containers:
      - name: mcp-server
        resources:
          requests:
            memory: "16Gi"
            cpu: "8"
          limits:
            memory: "32Gi"
            cpu: "16"
        
        env:
        - name: GOMAXPROCS
          value: "16"
        - name: GOMEMLIMIT
          value: "30GiB"
```

## Cost-Optimized Setup

### Scenario
Minimize costs while maintaining acceptable performance.

### Configuration Files

#### `.env.cost-optimized`
```bash
# Cost-Optimized Configuration
# =============================

# Resource Limits
ENVIRONMENT=production-budget
CPU_LIMIT=0.5
MEMORY_LIMIT=512MB

# Spot Instances
USE_SPOT_INSTANCES=true
SPOT_INSTANCE_INTERRUPTION_BEHAVIOR=hibernate
SPOT_MAX_PRICE=0.05

# Database (Minimal)
DB_INSTANCE_CLASS=db.t4g.micro
DB_POOL_SIZE=5
DB_MAX_CONNECTIONS=20
DB_MULTI_AZ=false
READ_REPLICA_COUNT=0

# Redis (Minimal)
REDIS_NODE_TYPE=cache.t4g.micro
REDIS_CLUSTER_MODE=false
REDIS_SNAPSHOT_RETENTION=0

# S3 Storage Classes
S3_STORAGE_CLASS=STANDARD_IA
S3_LIFECYCLE_ENABLED=true
S3_EXPIRE_DAYS=90

# AI Models (Strict Limits)
BEDROCK_SESSION_LIMIT=0.01
GLOBAL_COST_LIMIT=10.00
MODEL_SELECTION_STRATEGY=cheapest_always
DEFAULT_MODEL=amazon.titan-embed-text-v1
DISABLE_EXPENSIVE_MODELS=claude-3-opus,gpt-4

# Scaling
AUTO_SCALING_ENABLED=true
MIN_INSTANCES=1
MAX_INSTANCES=3
SCALE_DOWN_COOLDOWN=300
SCALE_UP_COOLDOWN=60
TARGET_CPU_UTILIZATION=80

# Off-Hours Scaling
ENABLE_SCHEDULED_SCALING=true
BUSINESS_HOURS_MIN=2
OFF_HOURS_MIN=0
WEEKEND_MIN=0

# Caching (Aggressive)
CACHE_EVERYTHING=true
DEFAULT_CACHE_TTL=24h
CACHE_STATIC_CONTENT=true
CDN_ENABLED=true
```

#### `cost-optimization.tf`
```hcl
# Cost-Optimized Infrastructure
data "aws_ami" "amazon_linux_2" {
  most_recent = true
  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-arm64-gp2"]
  }
  owners = ["amazon"]
}

resource "aws_launch_template" "cost_optimized" {
  name_prefix   = "mcp-cost-opt-"
  image_id      = data.aws_ami.amazon_linux_2.id
  instance_type = "t4g.nano"
  
  instance_market_options {
    market_type = "spot"
    spot_options {
      max_price = "0.01"
      spot_instance_type = "persistent"
    }
  }
  
  user_data = base64encode(templatefile("userdata.sh", {
    environment = "cost-optimized"
  }))
}

resource "aws_autoscaling_schedule" "off_hours" {
  autoscaling_group_name = aws_autoscaling_group.mcp.name
  scheduled_action_name  = "off-hours"
  
  min_size         = 0
  max_size         = 1
  desired_capacity = 0
  
  recurrence = "0 22 * * MON-FRI"  # 10 PM weekdays
}

resource "aws_autoscaling_schedule" "business_hours" {
  autoscaling_group_name = aws_autoscaling_group.mcp.name
  scheduled_action_name  = "business-hours"
  
  min_size         = 1
  max_size         = 3
  desired_capacity = 2
  
  recurrence = "0 8 * * MON-FRI"  # 8 AM weekdays
}
```

## Multi-Region Configuration

### Scenario
Global deployment with data sovereignty requirements.

### Configuration Files

#### `.env.multi-region`
```bash
# Multi-Region Configuration
# ==========================

# Region Configuration
PRIMARY_REGION=us-east-1
SECONDARY_REGIONS=us-west-2,eu-west-1,ap-southeast-1
ENABLE_REGION_FAILOVER=true
REGION_HEALTH_CHECK_INTERVAL=30s

# Data Residency
ENABLE_DATA_RESIDENCY=true
EU_DATA_REGION=eu-west-1
APAC_DATA_REGION=ap-southeast-1
US_DATA_REGION=us-east-1

# Cross-Region Replication
ENABLE_DB_REPLICATION=true
REPLICATION_LAG_THRESHOLD=1000ms
ENABLE_S3_REPLICATION=true
ENABLE_CACHE_REPLICATION=false  # Too expensive

# Regional Endpoints
US_EAST_ENDPOINT=https://us-east-1.api.mcp.example.com
US_WEST_ENDPOINT=https://us-west-2.api.mcp.example.com
EU_ENDPOINT=https://eu-west-1.api.mcp.example.com
APAC_ENDPOINT=https://ap-southeast-1.api.mcp.example.com

# Traffic Routing
ROUTING_POLICY=geoproximity
ENABLE_LATENCY_ROUTING=true
HEALTH_CHECK_PATH=/health
HEALTH_CHECK_INTERVAL=10s

# Regional Model Selection
US_BEDROCK_MODELS=claude-3-sonnet,titan-embed-v2
EU_BEDROCK_MODELS=claude-3-haiku,titan-embed-v1
APAC_BEDROCK_MODELS=titan-embed-v1
```

## Hybrid Cloud Setup

### Scenario
Mix of on-premises and cloud resources.

### Configuration Files

#### `.env.hybrid`
```bash
# Hybrid Cloud Configuration
# ==========================

# On-Premises Integration
ON_PREM_DATABASE_URL=postgresql://onprem.internal:5432/mcp
ON_PREM_LDAP_URL=ldap://ldap.internal:389
VPN_GATEWAY_ID=vgw-abc123

# Cloud Services
CLOUD_PROVIDER=aws
BACKUP_PROVIDER=aws
AI_PROVIDER=bedrock

# Hybrid Networking
ENABLE_DIRECT_CONNECT=true
DIRECT_CONNECT_VLAN=100
BGP_ASN=65000

# Data Synchronization
SYNC_DIRECTION=bidirectional
SYNC_INTERVAL=5m
CONFLICT_RESOLUTION=cloud_wins

# Security
ENABLE_TRANSIT_ENCRYPTION=true
ENABLE_VPN_BACKUP=true
CERTIFICATE_AUTHORITY=internal-ca
```

## Security-Hardened Configuration

### Scenario
Maximum security, compliance requirements.

### Configuration Files

#### `.env.security-hardened`
```bash
# Security-Hardened Configuration
# ================================

# Encryption Everything
ENCRYPTION_AT_REST=true
ENCRYPTION_IN_TRANSIT=true
ENVELOPE_ENCRYPTION=true
KMS_KEY_ID=arn:aws:kms:us-east-1:123456789012:key/abc-def

# Authentication
MFA_REQUIRED=true
PASSWORD_MIN_LENGTH=16
PASSWORD_REQUIRE_SPECIAL=true
SESSION_TIMEOUT=15m
IDLE_TIMEOUT=5m

# Network Security
ENABLE_WAF=true
ENABLE_SHIELD=true
ENABLE_GUARD_DUTY=true
ALLOWED_IP_RANGES=10.0.0.0/8,172.16.0.0/12

# Audit Logging
AUDIT_LOG_EVERYTHING=true
AUDIT_LOG_ENCRYPTION=true
AUDIT_RETENTION_YEARS=7
ENABLE_CLOUDTRAIL=true

# Compliance
ENABLE_HIPAA_MODE=true
ENABLE_PCI_MODE=true
ENABLE_SOC2_MODE=true
DATA_RETENTION_DAYS=2555
```

## AI Research Platform

### Scenario
Experimental AI workloads, multiple models.

### Configuration Files

#### `.env.ai-research`
```bash
# AI Research Configuration
# =========================

# Model Variety
ENABLE_ALL_MODELS=true
MODEL_EXPERIMENTATION=true
A_B_TESTING_ENABLED=true

# High Limits
BEDROCK_SESSION_LIMIT=100.00
GLOBAL_COST_LIMIT=10000.00
NO_RATE_LIMITS=true

# Experiment Tracking
ENABLE_MLFLOW=true
MLFLOW_TRACKING_URI=http://mlflow:5000
EXPERIMENT_VERSIONING=true

# GPU Support
ENABLE_GPU_INSTANCES=true
GPU_INSTANCE_TYPE=p4d.24xlarge
CUDA_VISIBLE_DEVICES=all
```

## Edge Computing Scenario

### Scenario
Distributed edge locations with central coordination.

### Configuration Files

#### `.env.edge`
```bash
# Edge Computing Configuration
# ============================

# Edge Locations
EDGE_LOCATIONS=edge-nyc,edge-lax,edge-chi
EDGE_SYNC_INTERVAL=30s
EDGE_CACHE_SIZE=1GB

# Offline Capability
ENABLE_OFFLINE_MODE=true
OFFLINE_QUEUE_SIZE=10000
SYNC_ON_RECONNECT=true

# Lightweight Models
EDGE_MODELS=titan-embed-text-v1
MODEL_QUANTIZATION=int8
MODEL_CACHE_LOCAL=true

# Bandwidth Optimization
COMPRESS_ALL_TRAFFIC=true
DELTA_SYNC_ENABLED=true
BINARY_PROTOCOL_ONLY=true
```

## Migration Scenarios

### From Monolith to Microservices

#### Stage 1: Strangler Fig Pattern
```bash
# Gradual Migration Configuration
ENABLE_LEGACY_API=true
LEGACY_API_URL=http://legacy.internal:8080
ROUTE_PERCENTAGE_NEW=10  # Start with 10% traffic

# Feature Flags
FEATURE_NEW_AUTH=false
FEATURE_NEW_EMBEDDINGS=true
FEATURE_NEW_WORKERS=true
```

#### Stage 2: Parallel Run
```bash
# Parallel Validation
ENABLE_SHADOW_MODE=true
COMPARE_RESULTS=true
DIVERGENCE_THRESHOLD=0.01
LOG_DIVERGENCES=true
```

#### Stage 3: Full Migration
```bash
# Completed Migration
ENABLE_LEGACY_API=false
ROUTE_PERCENTAGE_NEW=100
DEPRECATE_OLD_ENDPOINTS=true
```

## Configuration Best Practices

### 1. Environment Separation
```bash
# Use separate files
.env.development
.env.staging
.env.production

# Never commit secrets
*.env
!.env.example
```

### 2. Configuration Validation
```go
func ValidateConfig(cfg Config) error {
    if cfg.DatabaseURL == "" {
        return errors.New("DATABASE_URL required")
    }
    
    if cfg.BedrockEnabled && cfg.SessionLimit <= 0 {
        return errors.New("BEDROCK_SESSION_LIMIT must be positive")
    }
    
    return nil
}
```

### 3. Dynamic Configuration
```go
// Support runtime updates
type DynamicConfig struct {
    mu      sync.RWMutex
    values  map[string]interface{}
    watcher *fsnotify.Watcher
}

func (d *DynamicConfig) Watch(configFile string) error {
    return d.watcher.Add(configFile)
}
```

### 4. Configuration Testing
```bash
# Test configuration loading
make test-config ENV=production

# Validate all scenarios
for env in dev staging prod; do
    CONFIG_FILE=.env.$env go run cmd/validate/main.go
done
```

## Monitoring Configuration

### Metrics to Track
1. **Configuration Drift**: Changes over time
2. **Performance Impact**: Before/after metrics
3. **Cost Impact**: Configuration cost analysis
4. **Error Rates**: Configuration-related errors

### Configuration Audit
```sql
-- Track configuration changes
CREATE TABLE config_audit (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP DEFAULT NOW(),
    environment VARCHAR(50),
    key VARCHAR(100),
    old_value TEXT,
    new_value TEXT,
    changed_by VARCHAR(100),
    reason TEXT
);
```

## Next Steps

1. Choose the scenario closest to your needs
2. Start with the base configuration
3. Monitor metrics for 1-2 weeks
4. Adjust based on actual usage
5. Document all changes and reasons

## Additional Resources

- [Configuration Guide](./configuration-guide.md) - Detailed configuration reference
- [Performance Tuning Guide](./performance-tuning-guide.md) - Performance optimization
- [Cost Optimization Guide](./cost-optimization-guide.md) - Cost reduction strategies
- [Security Best Practices](./security-best-practices.md) - Security configuration
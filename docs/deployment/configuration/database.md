# Database Configuration

## Overview

Developer Mesh uses PostgreSQL as its primary database with the pgvector extension for vector similarity search. This document covers database configuration for all environments.

## PostgreSQL Requirements

- **Version**: PostgreSQL 14 or higher
- **Extensions Required**:
  - `pgvector` - Vector similarity search
  - `uuid-ossp` - UUID generation
- **Schemas Used**:
  - `mcp` - Primary schema for all tables
  - `public` - PostgreSQL default schema

## Connection Configuration

### Environment Variables

```bash
# Primary connection settings
DATABASE_HOST=localhost          # Database hostname
DATABASE_PORT=5432               # Database port
DATABASE_NAME=devmesh_development  # Database name
DATABASE_USER=devmesh            # Database username
DATABASE_PASSWORD=secure_password   # Database password

# SSL/TLS settings
DATABASE_SSL_MODE=disable        # Options: disable, require, verify-ca, verify-full

# Search path configuration
DATABASE_SEARCH_PATH=mcp,public  # Schema search order

# Alternative: Full connection string
DATABASE_DSN=postgresql://devmesh:password@localhost:5432/devmesh_development?sslmode=disable
```

### Configuration File (config.yaml)

```yaml
database:
  host: ${DATABASE_HOST:-localhost}
  port: ${DATABASE_PORT:-5432}
  name: ${DATABASE_NAME:-devmesh_development}
  user: ${DATABASE_USER:-devmesh}
  password: ${DATABASE_PASSWORD}
  ssl_mode: ${DATABASE_SSL_MODE:-disable}
  search_path: ${DATABASE_SEARCH_PATH:-mcp,public}
  
  # Connection pool configuration
  pool:
    max_open: 100        # Maximum open connections
    max_idle: 10         # Maximum idle connections
    max_lifetime: 1h     # Maximum connection lifetime
    idle_timeout: 10m    # Idle connection timeout
    
  # Query configuration
  query:
    timeout: 30s         # Query timeout
    slow_threshold: 1s   # Log queries slower than this
```

## Connection Pool Settings

### Development Settings

```yaml
pool:
  max_open: 25
  max_idle: 5
  max_lifetime: 30m
```

### Production Settings

```yaml
pool:
  max_open: 100
  max_idle: 10
  max_lifetime: 1h
  
  # Additional production settings
  prepared_statements: true
  connection_timeout: 10s
```

## Database Initialization

### 1. Create Database

```sql
-- Create user
CREATE USER devmesh WITH PASSWORD 'your_password';

-- Create database
CREATE DATABASE devmesh_development OWNER devmesh;

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE devmesh_development TO devmesh;
```

### 2. Install Extensions

```sql
-- Connect to the database
\c devmesh_development

-- Install required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgvector";
```

### 3. Create Schema

```sql
-- Create MCP schema
CREATE SCHEMA IF NOT EXISTS mcp AUTHORIZATION devmesh;

-- Set search path
ALTER DATABASE devmesh_development SET search_path TO mcp, public;
```

### 4. Run Migrations

```bash
# Using make
make migrate-up

# Or directly
migrate -path migrations -database $DATABASE_DSN up
```

## SSL/TLS Configuration

### SSL Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `disable` | No SSL | Development only |
| `require` | SSL required, no verification | Basic encryption |
| `verify-ca` | SSL with CA verification | Production |
| `verify-full` | SSL with full verification | High security |

### Production SSL Setup

```bash
# Environment variables
DATABASE_SSL_MODE=verify-full
DATABASE_SSL_CERT=/path/to/client-cert.pem
DATABASE_SSL_KEY=/path/to/client-key.pem
DATABASE_SSL_ROOT_CERT=/path/to/ca-cert.pem

# Connection string with SSL
DATABASE_DSN="postgresql://user:pass@host:5432/db?sslmode=verify-full&sslcert=client-cert.pem&sslkey=client-key.pem&sslrootcert=ca-cert.pem"
```

## pgvector Configuration

### Vector Index Settings

```sql
-- Create IVFFlat index for similarity search
CREATE INDEX ON mcp.embeddings 
USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);

-- Create HNSW index (better performance, more memory)
CREATE INDEX ON mcp.embeddings 
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

### Vector Search Configuration

```yaml
vector:
  dimensions: 1536       # Embedding dimensions
  index_type: ivfflat   # Options: ivfflat, hnsw
  similarity: cosine    # Options: cosine, l2, inner_product
  
  # Index parameters
  ivfflat:
    lists: 100          # Number of clusters
    probes: 10          # Clusters to search
    
  hnsw:
    m: 16              # Max connections per layer
    ef_construction: 64 # Size of dynamic candidate list
    ef_search: 40      # Size of search candidate list
```

## Performance Tuning

### PostgreSQL Configuration (postgresql.conf)

```ini
# Memory settings
shared_buffers = 4GB              # 25% of RAM
effective_cache_size = 12GB       # 75% of RAM
work_mem = 32MB                   # Per operation
maintenance_work_mem = 512MB      # For maintenance

# Connection settings
max_connections = 200
superuser_reserved_connections = 3

# WAL settings
wal_level = replica
max_wal_size = 2GB
min_wal_size = 1GB

# Query planner
random_page_cost = 1.1            # For SSD
effective_io_concurrency = 200    # For SSD

# Logging
log_min_duration_statement = 100  # Log slow queries (ms)
log_connections = on
log_disconnections = on
```

### Application-Level Optimizations

```yaml
# Prepared statements
database:
  prepared_statements: true
  statement_cache_size: 100
  
# Query optimizations
  query:
    timeout: 30s
    max_retries: 3
    retry_delay: 100ms
```

## Monitoring

### Health Checks

```sql
-- Check database health
SELECT version();
SELECT current_database();
SELECT pg_database_size(current_database());

-- Check connections
SELECT count(*) FROM pg_stat_activity;

-- Check slow queries
SELECT query, mean_exec_time, calls 
FROM pg_stat_statements 
ORDER BY mean_exec_time DESC 
LIMIT 10;
```

### Connection Pool Metrics

```yaml
metrics:
  enabled: true
  collect_interval: 10s
  
  # Metrics collected
  - pool_open_connections
  - pool_in_use_connections
  - pool_idle_connections
  - pool_wait_count
  - pool_wait_duration
```

## Backup and Recovery

### Backup Configuration

```bash
# Backup script
pg_dump -h $DATABASE_HOST -U $DATABASE_USER -d $DATABASE_NAME \
  --schema=mcp \
  --format=custom \
  --compress=9 \
  --file=backup-$(date +%Y%m%d-%H%M%S).dump

# Backup with pgvector data
pg_dump --no-owner --no-acl \
  --extension=pgvector \
  $DATABASE_DSN > backup.sql
```

### Restore Configuration

```bash
# Restore from custom format
pg_restore -h $DATABASE_HOST -U $DATABASE_USER -d $DATABASE_NAME \
  --clean --if-exists \
  backup.dump

# Restore with pgvector
psql $DATABASE_DSN < backup.sql
```

## Multi-Environment Setup

### Development

```yaml
database:
  host: localhost
  name: devmesh_development
  ssl_mode: disable
  pool:
    max_open: 25
```

### Staging

```yaml
database:
  host: staging-db.example.com
  name: devmesh_staging
  ssl_mode: require
  pool:
    max_open: 50
```

### Production

```yaml
database:
  host: prod-db.region.rds.amazonaws.com
  name: devmesh_production
  ssl_mode: verify-full
  pool:
    max_open: 100
  read_replicas:
    - host: read-replica-1.region.rds.amazonaws.com
    - host: read-replica-2.region.rds.amazonaws.com
```

## AWS RDS Configuration

### RDS Parameter Group

```ini
# Custom parameter group settings
shared_preload_libraries = 'pgvector'
max_connections = 200
shared_buffers = {DBInstanceClassMemory/4}
effective_cache_size = {DBInstanceClassMemory*3/4}
```

### RDS Security Group

```yaml
# Inbound rules
- protocol: tcp
  port: 5432
  source: application_security_group
```

## Troubleshooting

### Connection Issues

```bash
# Test connection
psql $DATABASE_DSN -c "SELECT 1"

# Check connectivity
pg_isready -h $DATABASE_HOST -p $DATABASE_PORT

# Debug connection string
echo $DATABASE_DSN | sed 's/:[^@]*@/:***@/'
```

### Performance Issues

```sql
-- Check table sizes
SELECT schemaname, tablename, 
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables 
WHERE schemaname = 'mcp'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Check index usage
SELECT schemaname, tablename, indexname, idx_scan
FROM pg_stat_user_indexes
WHERE schemaname = 'mcp'
ORDER BY idx_scan;

-- Vacuum and analyze
VACUUM ANALYZE;
```

### Migration Issues

```bash
# Check migration status
migrate -path migrations -database $DATABASE_DSN version

# Force version
migrate -path migrations -database $DATABASE_DSN force VERSION

# Create new migration
migrate create -ext sql -dir migrations -seq migration_name
```

## Security Best Practices

1. **Use SSL/TLS** in production
2. **Rotate passwords** regularly
3. **Limit connection sources** via firewall/security groups
4. **Use read replicas** for read-heavy workloads
5. **Enable audit logging** for compliance
6. **Backup regularly** with encryption
7. **Monitor for slow queries** and optimize
8. **Use prepared statements** to prevent SQL injection
9. **Set appropriate timeouts** to prevent hanging connections
10. **Implement row-level security** for multi-tenant data

## Related Documentation

- [Environment Variables Reference](../ENVIRONMENT_VARIABLES.md)
- [Configuration Overview](configuration-overview.md)
- [Migration Guide](../guides/migration-guide.md)
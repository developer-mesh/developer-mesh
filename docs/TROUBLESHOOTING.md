# Developer Mesh Troubleshooting Guide

## Quick Diagnostics

Run these commands first to diagnose common issues:

```bash
# Check environment and services
make validate-env
make health
make validate-services

# Check Docker status
docker ps
docker-compose -f docker-compose.local.yml ps

# Check logs
make logs
```

## Common Issues and Solutions

### 1. Services Won't Start

#### Symptom: `make dev` fails or services exit immediately

**Diagnosis:**
```bash
# Check Docker daemon
docker info

# Check port conflicts
lsof -i :8080
lsof -i :8081
lsof -i :5432
lsof -i :6379

# Check Docker compose logs
docker-compose -f docker-compose.local.yml logs
```

**Solutions:**
- **Port conflicts**: Kill conflicting processes or change ports in `.env`
- **Docker not running**: Start Docker Desktop
- **Out of disk space**: `docker system prune -a`
- **Corrupted containers**: `make docker-reset`

### 2. E2E Tests Failing

#### Symptom: Tests fail with connection errors

**Diagnosis:**
```bash
# Check service health
make validate-services

# Check E2E configuration
cat test/e2e/.env.local

# Run single test with debug
cd test/e2e && E2E_DEBUG=true ginkgo -v --focus="should register agent" ./scenarios
```

**Solutions:**
- **Services not ready**: `make wait-for-healthy` before running tests
- **Wrong environment**: Check `E2E_ENVIRONMENT` in `.env.local`
- **API key issues**: Verify `E2E_API_KEY=dev-admin-key-1234567890`
- **URL issues**: Ensure URLs include `http://` protocol

### 3. Database Connection Issues

#### Symptom: "connection refused" or "no such host"

**Local Docker Environment:**
```bash
# Check database container
docker exec $(docker ps -q -f name=database) pg_isready -U dev

# Test connection
docker exec -it $(docker ps -q -f name=database) psql -U dev -d dev
```

**AWS Environment:**
```bash
# Check SSH tunnel
make tunnel-status

# Test RDS connection via tunnel
PGPASSWORD=$DATABASE_PASSWORD psql -h localhost -p 5432 -U $DATABASE_USER -d $DATABASE_NAME -c "SELECT 1"

# Restart tunnel if needed
make tunnel-kill
make tunnel-rds
```

**Solutions:**
- **Docker**: Ensure `DATABASE_HOST=database` in `.env.local`
- **AWS**: Ensure `DATABASE_HOST=localhost` when using tunnels
- **Migrations**: Run `make migrate-up`

### 4. Redis Connection Issues

#### Symptom: "WRONGPASS" or connection timeouts

**Diagnosis:**
```bash
# Docker environment
docker exec $(docker ps -q -f name=redis) redis-cli ping

# AWS environment (via tunnel)
redis-cli -h localhost -p 6379 ping
```

**Solutions:**
- **Docker**: Use `REDIS_HOST=redis` and no password
- **AWS**: Check tunnel is active with `make tunnel-status`
- **Wrong config**: Verify `REDIS_ADDR` format (`host:port`)

### 5. Multi-Agent Test Failures

#### Symptom: "constraint violation" errors in multi-agent tests

**Solution:**
The multi-agent workflow requires these fields:
```json
{
  "strategy": "sequential",
  "coordination_mode": "centralized",
  "decision_strategy": "majority"
}
```

Run the fix: `make fix-multiagent`

### 6. SSH Tunnel Issues (AWS)

#### Symptom: "Permission denied" or "Connection refused"

**Diagnosis:**
```bash
# Check SSH key permissions
ls -la $SSH_KEY_PATH

# Test SSH connection
ssh -i $SSH_KEY_PATH ec2-user@$NAT_INSTANCE_IP "echo 'SSH works'"

# Check environment variables
make env-check | grep -E "(SSH_KEY_PATH|NAT_INSTANCE_IP|RDS_ENDPOINT)"
```

**Solutions:**
- **Fix key permissions**: `chmod 600 $SSH_KEY_PATH`
- **Expand key path**: Use full path, not `~`
- **Check NAT instance**: Ensure it's running in AWS
- **Security groups**: Verify SSH (22) is allowed

### 7. Build Failures

#### Symptom: Import errors or module not found

**Solutions:**
```bash
# Sync workspace
go work sync

# Update dependencies
make deps

# Clean and rebuild
make clean
make build
```

### 8. Environment Switching Issues

#### Symptom: Using wrong database or services after switching

**Complete Environment Switch:**

From Docker to AWS:
```bash
make env-aws
make down               # Stop Docker services
make tunnel-all         # Start SSH tunnels
# Start services with go run
```

From AWS to Docker:
```bash
make env-local
make tunnel-kill        # Stop SSH tunnels
make local-docker       # Start Docker environment
```

### 9. Test Data Issues

#### Symptom: "tenant not found" or missing test data

**Solutions:**
```bash
# Docker environment
make seed-test-data

# Reset and reseed
make reset-test-data
make seed-test-data

# Verify data
docker exec -it $(docker ps -q -f name=database) psql -U dev -d dev -c "SELECT id, name FROM tenants"
```

### 10. Performance Issues

#### Symptom: Slow tests or timeouts

**Diagnosis:**
```bash
# Check resource usage
docker stats

# Run specific test with timeout
cd test/e2e && ginkgo -v --timeout=60s --focus="specific test" ./scenarios

# Profile services
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof -http=:8090 cpu.prof
```

**Solutions:**
- **Increase timeouts**: Set `TIMEOUT=60s` in test commands
- **Resource limits**: Check Docker Desktop memory settings
- **Parallel tests**: Reduce with `PARALLEL=1`

## Environment Variables Reference

### Required for Local Docker
```bash
ENVIRONMENT=local
DATABASE_HOST=database
DATABASE_PORT=5432
DATABASE_USER=dev
DATABASE_PASSWORD=dev
DATABASE_NAME=dev
DATABASE_SSL_MODE=disable
REDIS_HOST=redis
REDIS_PORT=6379
USE_LOCALSTACK=true
USE_REAL_AWS=false
```

### Required for AWS Integration
```bash
USE_REAL_AWS=true
DATABASE_HOST=localhost  # When using tunnel
SSH_KEY_PATH=/path/to/key.pem
NAT_INSTANCE_IP=x.x.x.x
RDS_ENDPOINT=instance.region.rds.amazonaws.com
ELASTICACHE_ENDPOINT=cluster.cache.amazonaws.com
AWS_ACCESS_KEY_ID=xxx
AWS_SECRET_ACCESS_KEY=xxx
AWS_REGION=us-west-2
```

## Getting Help

1. **Check logs first**: `make logs` or `docker-compose logs -f service-name`
2. **Run diagnostics**: `make validate-env`
3. **Enable debug mode**: Set `DEBUG=true` or `E2E_DEBUG=true`
4. **Check recent changes**: `git status` and `git diff`

## Emergency Recovery

If everything is broken:

```bash
# Nuclear option - reset everything
make down
make tunnel-kill
docker system prune -a --volumes
rm -rf test/e2e/.env.local
make local-docker
```

## Additional Resources

- [Local Development Guide](./LOCAL_DEVELOPMENT.md)
- [E2E Testing Guide](../test/e2e/README.md)
- [Architecture Overview](./ARCHITECTURE.md)
- [GitHub Issues](https://github.com/yourusername/developer-mesh/issues)
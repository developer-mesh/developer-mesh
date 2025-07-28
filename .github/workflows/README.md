# GitHub Actions Workflows

This directory contains the CI/CD pipeline configurations for the DevOps MCP project.

## Workflows

### CI (ci.yml)
Runs on every push and pull request to validate code quality and run tests.

**Required Environment Variables:**
- `REDIS_ADDR`: Redis server address (default: localhost:6379)
- `REDIS_PASSWORD`: Redis password (empty for local development)
- `REDIS_TLS_ENABLED`: Enable TLS for Redis (default: false)
- `BEDROCK_ENABLED`: Enable AWS Bedrock features (default: false)
- `LOG_LEVEL`: Logging level (default: debug)

### Deploy to Production (deploy-production-v2.yml)
Deploys the application to production after successful CI runs on the main branch.

**Required Secrets:**
- `EC2_INSTANCE_IP`: Production EC2 instance IP
- `AWS_ACCESS_KEY_ID`: AWS credentials for deployment
- `AWS_SECRET_ACCESS_KEY`: AWS secret key
- `DATABASE_HOST`: PostgreSQL host
- `DATABASE_PASSWORD`: PostgreSQL password
- `REDIS_ENDPOINT`: Redis cluster endpoint (e.g., redis-cluster.abc.cache.amazonaws.com:6379)
- `S3_BUCKET`: S3 bucket for storage
- `ADMIN_API_KEY`: Admin API authentication
- `E2E_API_KEY`: E2E test API key
- `MCP_API_KEY`: MCP server API key
- `MCP_WEBHOOK_SECRET`: GitHub webhook secret

### E2E Tests (e2e-tests.yml)
Runs end-to-end tests after deployment or on schedule.

**Required Secrets:**
- `E2E_API_KEY`: API key for E2E tests
- `E2E_TENANT_ID`: Tenant ID for E2E tests
- `SLACK_WEBHOOK`: Slack webhook for notifications (optional)

## Migration Notes

### SQS to Redis Migration
The workflows have been updated to use Redis instead of AWS SQS:

1. **Removed Variables:**
   - `SQS_QUEUE_URL` - No longer needed
   - AWS credentials are still required for S3 and other AWS services

2. **Added Variables:**
   - `REDIS_ADDR` / `REDIS_ENDPOINT` - Redis connection string
   - `REDIS_TLS_ENABLED` - Enable TLS for production Redis
   - `REDIS_STREAM_NAME` - Stream name for webhooks (default: webhooks)
   - `REDIS_CONSUMER_GROUP` - Consumer group name (default: webhook-workers)

3. **Local Development:**
   ```yaml
   env:
     REDIS_ADDR: localhost:6379
     REDIS_PASSWORD: ""
     REDIS_TLS_ENABLED: false
   ```

4. **Production:**
   ```yaml
   env:
     REDIS_ADDR: ${{ secrets.REDIS_ENDPOINT }}
     REDIS_TLS_ENABLED: true
   ```

## Adding New Workflows

When adding new workflows that interact with the queue system:

1. Use Redis environment variables instead of SQS
2. Ensure Redis connectivity in health checks
3. Add appropriate retry logic for Redis operations
4. Consider using Redis Sentinel endpoints for HA

## Security Notes

- Never commit Redis passwords or connection strings
- Use GitHub Secrets for all sensitive configuration
- Enable TLS for production Redis connections
- Rotate credentials regularly
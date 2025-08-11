<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:39:31
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Production Deployment Fixes Required

## Current Issues

The production deployment is failing because containers are not starting properly. The root causes are:

### 1. Container Name Conflicts
- Old containers with names `mcp-server`, `rest-api`, and `worker` are not being cleaned up
- Docker-compose tries to create new containers with the same names, causing conflicts
- Error: "The container name is already in use"

### 2. Missing Docker Registry Authentication
- The deployment script doesn't log into GitHub Container Registry
- This can cause image pull failures for private repositories

### 3. Variable Escaping Issues
- Variables like `$container` in the deployment script are not properly escaped
- They get evaluated during script creation instead of execution

### 4. Insufficient Error Handling
- The deployment continues even when containers fail to start
- Smoke tests pass even when services are down
- No proper exit codes when failures occur

### 5. Config File Issues
- MCP server expects `config.yaml` but only `config.production.yaml` exists
- Missing symlink creation in deployment

## Required Fixes

### Fix 1: Update Deploy Services Step

Replace the current "Deploy Services (Simple)" step in `.github/workflows/deploy-production-v2.yml` with the fixed version in `scripts/fixed-deploy-step.yml`.

Key changes:
- Add explicit container cleanup before docker-compose
- Add GitHub Container Registry login
- Fix variable escaping (use `${variable}` consistently)
- Add proper error checking and exit codes
- Increase wait time to 30 seconds for container startup

### Fix 2: Update Deployment File Setup

In the "Update Deployment Files on EC2" step, add:
```bash
'echo "Creating config.yaml symlink..."',
'cd configs && ln -sf config.production.yaml config.yaml && cd ..',
```

### Fix 3: Fix Smoke Tests

Update the smoke test to properly track and report failures:
- Use a `SMOKE_TEST_PASSED` variable to track overall status
- Exit with code 1 if any test fails
- Add diagnostic output when tests fail

### Fix 4: Add Container Health Checks

Add health check commands to the docker-compose.production.yml for each service:
```yaml
healthcheck:
  test: ["CMD", "/app/SERVICE_NAME", "-health-check"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

## Immediate Actions

1. **Manual Fix** (temporary):
   ```bash
   ssh -i ~/.ssh/nat-instance.pem ec2-user@54.86.185.227
   cd /home/ec2-user/developer-mesh
   docker stop mcp-server rest-api worker
   docker rm mcp-server rest-api worker
   cd configs && ln -sf config.production.yaml config.yaml && cd ..
   docker-compose up -d
   ```

2. **Permanent Fix**:
   - Update `.github/workflows/deploy-production-v2.yml` with the fixes
   - Test deployment in a staging environment first
   - Monitor deployment logs for any remaining issues

## Verification

After applying fixes, verify:
1. All containers start successfully
2. Health endpoints respond (https://mcp.dev-mesh.io/health, https://api.dev-mesh.io/health)
3. WebSocket connections work <!-- Source: pkg/models/websocket/binary.go -->
4. Deployment fails properly when containers don't start

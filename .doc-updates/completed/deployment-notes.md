<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:39:40
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Developer Mesh Deployment Configuration

## Service Ports
- MCP Server: 8080 (uses configs/config.yaml)
- REST API: 8081 (uses configs/config.rest-api.yaml)  
- Worker: No exposed ports (uses configs/config.yaml)

## Key Configuration Changes Made

### 1. REST API Port Configuration
- Created separate config file: configs/config.rest-api.yaml
- Set api.listen_address to :8081 (REST API)
- Added environment variables in docker-compose.yml:
  - PORT=8081
  - API_LISTEN_ADDRESS=:8081 (REST API)

### 2. Docker Compose Configuration
- REST API mounts its own config file
- Each service has proper health checks
- Memory limits: 200MB per container (150MB reserved)

### 3. Instance Requirements
- Minimum: t3.small (2 vCPU, 2GB RAM)
- t2.micro (1GB RAM) is insufficient for all 3 containers

## To Deploy Again:
1. Ensure configs/config.yaml has api.listen_address: ":8080 (MCP Server)"
2. Create configs/config.rest-api.yaml with api.listen_address: ":8081 (REST API)"
3. Use the docker-compose.yml that mounts the correct config for each service
4. Ensure all AWS resources (RDS, Redis, SQS) are accessible from the instance

## Working Endpoints
- MCP Server: https://mcp.dev-mesh.io/health (SSL/TLS enabled)
- REST API: https://api.dev-mesh.io/health (SSL/TLS enabled)
- Direct access (bypassing Nginx):
  - http://<instance-ip>:8080 (MCP Server)/health (MCP Server)
  - http://<instance-ip>:8081 (REST API)/health (REST API)

## Current Status
- MCP Server: ✅ Healthy
- REST API: ✅ Healthy  
- Worker: ✅ Healthy (SQS timeout issue fixed)

## Worker SQS Timeout Issue (RESOLVED)

### Root Cause
The worker had a bug in `/apps/worker/cmd/worker/main.go` where it reused a 5-second timeout context meant for Redis connectivity testing for the entire worker process.

### Fix Applied
- Fixed in commit: 4752b43
- Changed to use separate context for Redis ping test
- Worker now properly receives SQS messages with 10-second long polling

### Deployment Steps After Fix
1. Wait for CI/CD to build new images
2. Pull new worker image: `docker pull ghcr.io/s-corkum/developer-mesh-worker:latest`
3. Restart worker: `docker-compose up -d worker`

## TLS/SSL Setup with Route 53 and Let's Encrypt

### DNS Configuration
1. Created A records in Route 53:
   - `mcp.dev-mesh.io` → Instance IP
   - `api.dev-mesh.io` → Instance IP

### Nginx Reverse Proxy
1. Installed Nginx as reverse proxy
2. Configured:
   - `mcp.dev-mesh.io` → localhost:8080 (MCP Server) (MCP Server)
   - `api.dev-mesh.io` → localhost:8081 (REST API) (REST API)

### SSL Certificates
1. Installed Certbot with nginx plugin
2. Obtained Let's Encrypt certificates for both domains
3. Automatic renewal enabled via systemd timer

### Security Group Updates
- Port 80 (HTTP) - Open to 0.0.0.0/0
- Port 443 (HTTPS) - Open to 0.0.0.0/0
- Port 22 (SSH) - Restricted to your IP
- Port 8080/8081 - Not exposed publicly (accessed via Nginx)

## GitHub Actions CD Pipeline

### Available Workflows
1. **Production Deploy** (`deploy-production-v2.yml`)
   - Automatic on push to main (after CI completes)
   - Manual trigger available
   - Blue-green deployment
   - Database migrations
   - Automatic rollback on failure
   - Integrated with E2E tests

### Required GitHub Secrets
See `docs/GITHUB_ACTIONS_SETUP.md` for complete setup guide.

### Deployment Commands
```bash
# Manual deployment via GitHub CLI
gh workflow run deploy-production-v2.yml

# Monitor deployment
gh run watch

# View deployment history
gh run list --workflow=deploy-production-v2.yml

# View CI/CD pipeline status
gh run list --workflow=ci.yml

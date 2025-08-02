# Makefile Review and Recommendations

## Current State Analysis

### Root Makefile (`/Makefile`)

**Strengths:**
- Good organization with clear sections
- Comprehensive help system
- Docker Compose integration (`make dev`)
- Pre-commit workflow (`make pre-commit`)
- Database migration commands
- Health check command exists

**Critical Gaps:**
1. **No E2E test targets** - Missing integration with test/e2e
2. **No local environment setup** for E2E tests
3. **No environment validation** before running services
4. **Limited service orchestration** - can't easily run services with proper config
5. **No test data seeding** commands
6. **Missing local vs production** environment switching

### E2E Test Makefile (`/test/e2e/Makefile`)

**Strengths:**
- Comprehensive test targets (single, multi, performance)
- Local test target exists (`test-local`)
- Good reporting options
- Validation command

**Critical Gaps:**
1. **Hardcoded localhost URLs** without protocol in `test-local`
2. **No environment file loading** for local tests
3. **No service dependency checking**
4. **Missing integration** with root Makefile

## Recommended Makefile Improvements

### 1. Add E2E Test Targets to Root Makefile

```makefile
# ==============================================================================
# E2E Testing Commands
# ==============================================================================

.PHONY: test-e2e
test-e2e: ## Run E2E tests against running services
	@echo "Running E2E tests..."
	@cd test/e2e && $(MAKE) test

.PHONY: test-e2e-local
test-e2e-local: dev-wait ## Run E2E tests against local Docker services
	@echo "Running E2E tests against local environment..."
	@cd test/e2e && E2E_ENVIRONMENT=local $(MAKE) test-local

.PHONY: test-e2e-single
test-e2e-single: ## Run single agent E2E tests
	@cd test/e2e && $(MAKE) test-single

.PHONY: test-e2e-multi
test-e2e-multi: ## Run multi-agent E2E tests
	@cd test/e2e && $(MAKE) test-multi

.PHONY: test-e2e-setup
test-e2e-setup: ## Setup E2E test environment
	@echo "Setting up E2E test environment..."
	@if [ ! -f test/e2e/.env.local ]; then \
		cp test/e2e/.env.example test/e2e/.env.local; \
		sed -i '' 's/your-api-key-here/dev-admin-key-1234567890/g' test/e2e/.env.local; \
		sed -i '' 's|#MCP_BASE_URL=.*|MCP_BASE_URL=http://localhost:8080|g' test/e2e/.env.local; \
		sed -i '' 's|#API_BASE_URL=.*|API_BASE_URL=http://localhost:8081|g' test/e2e/.env.local; \
		sed -i '' 's|#E2E_TENANT_ID=.*|E2E_TENANT_ID=00000000-0000-0000-0000-000000000001|g' test/e2e/.env.local; \
		echo "✅ Created test/e2e/.env.local"; \
	else \
		echo "✅ test/e2e/.env.local already exists"; \
	fi
```

### 2. Add Local Development Workflow

```makefile
# ==============================================================================
# Local Development Workflow
# ==============================================================================

.PHONY: local
local: local-start local-wait local-seed ## Complete local development setup

.PHONY: local-start
local-start: dev-setup ## Start local development environment
	@echo "Starting local development environment..."
	$(DOCKER_COMPOSE) up -d
	
.PHONY: local-wait
local-wait: ## Wait for services to be healthy
	@echo "Waiting for services to be healthy..."
	@for i in $$(seq 1 30); do \
		if make health-check-silent 2>/dev/null; then \
			echo "✅ All services are healthy"; \
			break; \
		fi; \
		if [ $$i -eq 30 ]; then \
			echo "❌ Services failed to start"; \
			make logs; \
			exit 1; \
		fi; \
		echo "⏳ Waiting for services... ($$i/30)"; \
		sleep 2; \
	done

.PHONY: local-seed
local-seed: ## Seed test data for local development
	@echo "Seeding test data..."
	@./scripts/local/seed-test-data.sh || echo "⚠️  Seed script not found"

.PHONY: local-reset
local-reset: down ## Reset local environment
	@echo "Resetting local environment..."
	@docker volume prune -f
	@make local

.PHONY: local-test
local-test: test-e2e-setup test-e2e-local ## Run E2E tests in local environment
```

### 3. Enhanced Health Checking

```makefile
# ==============================================================================
# Health & Validation Commands
# ==============================================================================

.PHONY: health-check-silent
health-check-silent: ## Silent health check for scripts
	@curl -sf http://localhost:8080/health > /dev/null && \
	 curl -sf http://localhost:8081/health > /dev/null && \
	 docker exec $$(docker ps -q -f name=database) pg_isready -U dev > /dev/null 2>&1 && \
	 docker exec $$(docker ps -q -f name=redis) redis-cli ping > /dev/null 2>&1

.PHONY: validate-env
validate-env: ## Validate environment configuration
	@echo "Validating environment configuration..."
	@./scripts/local/validate-environment.sh

.PHONY: validate-services
validate-services: ## Validate all services are properly configured
	@echo "Validating service configuration..."
	@echo -n "MCP Server API Keys: "
	@curl -s -H "X-API-Key: dev-admin-key-1234567890" http://localhost:8080/health | jq -r '.status // "❌ Failed"'
	@echo -n "REST API Keys: "
	@curl -s -H "X-API-Key: dev-admin-key-1234567890" http://localhost:8081/health | jq -r '.status // "❌ Failed"'
```

### 4. Update Docker Commands Section

```makefile
.PHONY: dev
dev: dev-setup up dev-wait ## Start development environment with health checks
	@echo "✅ Development environment is ready!"
	@echo ""
	@echo "Services available at:"
	@echo "  MCP Server: http://localhost:8080"
	@echo "  REST API: http://localhost:8081"
	@echo "  Mock Server: http://localhost:8082"
	@echo ""
	@echo "Run 'make test-e2e-local' to test against local services"

.PHONY: dev-wait
dev-wait: ## Wait for Docker services to be ready
	@make local-wait
```

### 5. Update E2E Test Makefile for Better Local Support

**File: `/test/e2e/Makefile` updates:**

```makefile
# Load local environment if it exists
ifneq (,$(wildcard ./.env.local))
    include .env.local
    export
endif

# Run tests against local environment
test-local:
	@echo "Running tests against local environment..."
	@if [ ! -f .env.local ]; then \
		echo "❌ Error: .env.local not found. Run 'make test-e2e-setup' from root directory"; \
		exit 1; \
	fi
	@echo "Using configuration:"
	@echo "  MCP_BASE_URL: $${MCP_BASE_URL:-http://localhost:8080}"
	@echo "  API_BASE_URL: $${API_BASE_URL:-http://localhost:8081}"
	@echo "  E2E_API_KEY: $${E2E_API_KEY:0:20}..."
	E2E_ENVIRONMENT=local \
	MCP_BASE_URL=$${MCP_BASE_URL:-http://localhost:8080} \
	API_BASE_URL=$${API_BASE_URL:-http://localhost:8081} \
	E2E_DEBUG=true \
	$(MAKE) test
```

### 6. Create Helper Scripts

**File: `/scripts/local/validate-environment.sh`**
```bash
#!/bin/bash
set -e

echo "Checking environment variables..."

# Required for local development
REQUIRED_VARS=(
    "DATABASE_HOST"
    "DATABASE_PORT"
    "DATABASE_USER"
    "DATABASE_PASSWORD"
    "REDIS_HOST"
    "REDIS_PORT"
)

MISSING=0
for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        echo "❌ Missing: $var"
        MISSING=$((MISSING + 1))
    else
        echo "✅ Found: $var=${!var}"
    fi
done

if [ $MISSING -gt 0 ]; then
    echo ""
    echo "❌ $MISSING required environment variables are missing"
    echo "💡 Run 'make dev-setup' to create .env file"
    exit 1
fi

echo "✅ All required environment variables are set"
```

### 7. Add Quick Development Commands

```makefile
# ==============================================================================
# Quick Development Commands
# ==============================================================================

.PHONY: quick-test
quick-test: ## Quick test against local services (assumes they're running)
	@cd test/e2e && \
	E2E_ENVIRONMENT=local \
	E2E_API_KEY=dev-admin-key-1234567890 \
	MCP_BASE_URL=http://localhost:8080 \
	API_BASE_URL=http://localhost:8081 \
	ginkgo -v --focus="Single Agent.*Basic Operations.*should register agent and receive acknowledgment" ./scenarios

.PHONY: fix-multiagent
fix-multiagent: ## Test the multi-agent workflow fix
	@cd test/e2e && \
	E2E_ENVIRONMENT=local \
	E2E_API_KEY=dev-admin-key-1234567890 \
	MCP_BASE_URL=http://localhost:8080 \
	API_BASE_URL=http://localhost:8081 \
	ginkgo -v --focus="Code Review Workflow" ./scenarios
```

### 8. Environment Management Commands

Since you have secrets in your .env file, we need commands to manage different environments:

```makefile
# ==============================================================================
# Environment Management
# ==============================================================================

.PHONY: env-check
env-check: ## Check current environment configuration
	@echo "Current Environment Configuration:"
	@echo "================================="
	@echo "ENVIRONMENT: $${ENVIRONMENT:-not set}"
	@echo "MCP_SERVER_URL: $${MCP_SERVER_URL:-not set}"
	@echo "REST_API_URL: $${REST_API_URL:-not set}"
	@echo "DATABASE_HOST: $${DATABASE_HOST:-not set}"
	@echo "REDIS_ADDR: $${REDIS_ADDR:-not set}"
	@echo "AWS_REGION: $${AWS_REGION:-not set}"
	@echo "USE_REAL_AWS: $${USE_REAL_AWS:-not set}"
	@echo "USE_LOCALSTACK: $${USE_LOCALSTACK:-not set}"
	@echo ""
	@echo "API Keys (first 20 chars):"
	@echo "ADMIN_API_KEY: $${ADMIN_API_KEY:0:20}..."
	@echo "GITHUB_ACCESS_TOKEN: $${GITHUB_ACCESS_TOKEN:0:20}..."

.PHONY: env-local
env-local: ## Configure environment for local Docker development
	@echo "Configuring for local Docker environment..."
	@cp .env .env.backup
	@echo "# Local Docker Environment Overrides" > .env.local
	@echo "ENVIRONMENT=local" >> .env.local
	@echo "DATABASE_HOST=database" >> .env.local
	@echo "DATABASE_PORT=5432" >> .env.local
	@echo "DATABASE_NAME=dev" >> .env.local
	@echo "DATABASE_USER=dev" >> .env.local
	@echo "DATABASE_PASSWORD=dev" >> .env.local
	@echo "DATABASE_SSL_MODE=disable" >> .env.local
	@echo "REDIS_HOST=redis" >> .env.local
	@echo "REDIS_PORT=6379" >> .env.local
	@echo "REDIS_ADDR=redis:6379" >> .env.local
	@echo "USE_LOCALSTACK=true" >> .env.local
	@echo "USE_REAL_AWS=false" >> .env.local
	@echo "AWS_ENDPOINT_URL=http://localstack:4566" >> .env.local
	@echo "✅ Created .env.local for Docker environment"

.PHONY: env-aws
env-aws: ## Configure environment for AWS development (using your existing .env)
	@echo "Using AWS development environment from .env"
	@if [ -f .env.local ]; then rm .env.local; fi
	@echo "✅ Configured for AWS development"
```

### 9. SSH Tunnel Management (from your .env)

Since you're using RDS via SSH tunnel, add these commands:

```makefile
# ==============================================================================
# SSH Tunnel Management
# ==============================================================================

.PHONY: tunnel-rds
tunnel-rds: ## Create SSH tunnel to RDS
	@echo "Creating SSH tunnel to RDS..."
	@ssh -N -L 5432:$(RDS_ENDPOINT):5432 \
		-i $(SSH_KEY_PATH) \
		ec2-user@$(NAT_INSTANCE_IP) &
	@echo "✅ RDS tunnel created on localhost:5432"
	@echo "💡 Run 'make tunnel-status' to check tunnel"

.PHONY: tunnel-redis
tunnel-redis: ## Create SSH tunnel to ElastiCache
	@echo "Creating SSH tunnel to ElastiCache..."
	@ssh -N -L 6379:$(ELASTICACHE_ENDPOINT):6379 \
		-i $(SSH_KEY_PATH) \
		ec2-user@$(NAT_INSTANCE_IP) &
	@echo "✅ Redis tunnel created on localhost:6379"

.PHONY: tunnel-all
tunnel-all: tunnel-rds tunnel-redis ## Create all SSH tunnels

.PHONY: tunnel-status
tunnel-status: ## Check SSH tunnel status
	@echo "Active SSH tunnels:"
	@ps aux | grep -E "ssh.*-L" | grep -v grep || echo "No active tunnels"

.PHONY: tunnel-kill
tunnel-kill: ## Kill all SSH tunnels
	@pkill -f "ssh.*-L" || echo "No tunnels to kill"
	@echo "✅ All SSH tunnels terminated"
```

### 10. Complete Local Development Workflow

Integrating everything from LOCAL_DEVELOPMENT_ANALYSIS.md:

```makefile
# ==============================================================================
# Complete Local Development Workflow
# ==============================================================================

.PHONY: local-docker
local-docker: env-local docker-reset local-wait local-seed test-e2e-setup ## Full local Docker setup
	@echo "✅ Local Docker environment ready!"
	@echo ""
	@echo "Services:"
	@echo "  MCP Server: http://localhost:8080"
	@echo "  REST API: http://localhost:8081"
	@echo "  Mock Server: http://localhost:8082"
	@echo ""
	@echo "Next steps:"
	@echo "  make test-e2e-local    # Run E2E tests"
	@echo "  make logs              # View service logs"
	@echo "  make health            # Check service health"

.PHONY: local-aws
local-aws: env-aws tunnel-all local-aws-wait test-e2e-setup ## Local development with AWS services
	@echo "✅ Local AWS environment ready!"
	@echo ""
	@echo "Using:"
	@echo "  RDS: via SSH tunnel on localhost:5432"
	@echo "  ElastiCache: via SSH tunnel on localhost:6379"
	@echo "  S3: Direct AWS access"
	@echo ""
	@echo "Services to start manually:"
	@echo "  make run-mcp-server    # In terminal 1"
	@echo "  make run-rest-api      # In terminal 2"
	@echo "  make run-worker        # In terminal 3"

.PHONY: local-aws-wait
local-aws-wait: ## Wait for AWS services via tunnels
	@echo "Checking AWS service connectivity..."
	@for i in $$(seq 1 10); do \
		if pg_isready -h localhost -p 5432 -U $(DATABASE_USER) > /dev/null 2>&1; then \
			echo "✅ RDS is accessible"; \
			break; \
		fi; \
		echo "⏳ Waiting for RDS tunnel... ($$i/10)"; \
		sleep 2; \
	done
	@redis-cli -h localhost -p 6379 ping > /dev/null 2>&1 && echo "✅ Redis is accessible" || echo "❌ Redis not accessible"

.PHONY: docker-reset
docker-reset: ## Reset Docker environment completely
	$(DOCKER_COMPOSE) down -v
	docker system prune -f
	$(DOCKER_COMPOSE) up -d
```

### 11. Test Data Seeding

Based on the LOCAL_DEVELOPMENT_ANALYSIS.md, we need proper test data seeding:

```makefile
# ==============================================================================
# Test Data Management
# ==============================================================================

.PHONY: seed-test-data
seed-test-data: ## Seed test data for local development
	@echo "Seeding test data..."
	@if [ "$${USE_REAL_AWS}" = "true" ]; then \
		echo "Seeding data to AWS RDS..."; \
		PGPASSWORD=$${DATABASE_PASSWORD} psql -h localhost -p 5432 -U $${DATABASE_USER} -d $${DATABASE_NAME} -f scripts/db/seed-test-data.sql; \
	else \
		echo "Seeding data to Docker PostgreSQL..."; \
		docker exec -i $$(docker ps -q -f name=database) psql -U dev -d dev < scripts/db/seed-test-data.sql; \
	fi
	@echo "✅ Test data seeded successfully"

.PHONY: create-seed-script
create-seed-script: ## Create the seed data SQL script
	@mkdir -p scripts/db
	@cat > scripts/db/seed-test-data.sql << 'EOF'
-- Test Data Seeding Script
-- Insert test tenants
INSERT INTO tenants (id, name, created_at, updated_at) VALUES
    ('00000000-0000-0000-0000-000000000001', 'Test Tenant 1', NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000002', 'Test Tenant 2', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert test agents
INSERT INTO agents (id, tenant_id, name, type, capabilities, created_at, updated_at) VALUES
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'test-code-agent', 'code_analysis', '["code_analysis", "code_review"]', NOW(), NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'test-security-agent', 'security', '["security_scanning", "vulnerability_detection"]', NOW(), NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'test-devops-agent', 'devops', '["deployment", "monitoring"]', NOW(), NOW())
ON CONFLICT DO NOTHING;

-- Insert test models
INSERT INTO models (id, tenant_id, name, provider, type, created_at, updated_at) VALUES
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'claude-3-opus', 'anthropic', 'llm', NOW(), NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'gpt-4', 'openai', 'llm', NOW(), NOW())
ON CONFLICT DO NOTHING;

-- Insert test API configurations
INSERT INTO tool_configurations (id, tenant_id, name, type, base_url, auth_config, created_at, updated_at) VALUES
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'GitHub API', 'github', 'https://api.github.com', '{"type": "bearer", "token": "test-token"}', NOW(), NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'Test Tool', 'custom', 'http://localhost:8082', '{"type": "api_key", "key": "test-key"}', NOW(), NOW())
ON CONFLICT DO NOTHING;
EOF
	@echo "✅ Created scripts/db/seed-test-data.sql"

.PHONY: reset-test-data
reset-test-data: ## Reset test data to clean state
	@echo "Resetting test data..."
	@if [ "$${USE_REAL_AWS}" = "true" ]; then \
		PGPASSWORD=$${DATABASE_PASSWORD} psql -h localhost -p 5432 -U $${DATABASE_USER} -d $${DATABASE_NAME} -c "TRUNCATE TABLE tasks, workflows, workflow_executions, tool_executions CASCADE;"; \
	else \
		docker exec -i $$(docker ps -q -f name=database) psql -U dev -d dev -c "TRUNCATE TABLE tasks, workflows, workflow_executions, tool_executions CASCADE;"; \
	fi
	@echo "✅ Test data reset"
```

### 12. E2E Test Environment Configuration

Create proper E2E test configuration based on your .env:

```makefile
.PHONY: test-e2e-config
test-e2e-config: ## Configure E2E tests based on current environment
	@echo "Configuring E2E tests for environment: $${ENVIRONMENT:-development}"
	@mkdir -p test/e2e
	@if [ "$${ENVIRONMENT}" = "local" ] || [ -f .env.local ]; then \
		echo "E2E_ENVIRONMENT=local" > test/e2e/.env; \
		echo "E2E_API_KEY=$${ADMIN_API_KEY}" >> test/e2e/.env; \
		echo "MCP_BASE_URL=http://localhost:8080" >> test/e2e/.env; \
		echo "API_BASE_URL=http://localhost:8081" >> test/e2e/.env; \
		echo "E2E_TENANT_ID=00000000-0000-0000-0000-000000000001" >> test/e2e/.env; \
	else \
		echo "E2E_ENVIRONMENT=development" > test/e2e/.env; \
		echo "E2E_API_KEY=$${ADMIN_API_KEY}" >> test/e2e/.env; \
		echo "MCP_BASE_URL=$${MCP_SERVER_URL}" >> test/e2e/.env; \
		echo "API_BASE_URL=$${REST_API_URL}" >> test/e2e/.env; \
		echo "E2E_TENANT_ID=00000000-0000-0000-0000-000000000001" >> test/e2e/.env; \
	fi
	@echo "✅ E2E test configuration created"

.PHONY: test-e2e-aws
test-e2e-aws: test-e2e-config ## Run E2E tests against locally running services with AWS backends
	@cd test/e2e && \
	source .env && \
	$(MAKE) test
```

## Implementation Checklist

1. **Immediate Actions:**
   - [ ] Add E2E test targets to root Makefile
   - [ ] Create `test-e2e-setup` target for environment configuration
   - [ ] Add `local-wait` target for service health checking
   - [ ] Update `dev` target to include health checks
   - [ ] Add environment management commands (`env-check`, `env-local`, `env-aws`)
   - [ ] Add SSH tunnel management for AWS services

2. **Short Term:**
   - [ ] Create validation scripts
   - [ ] Add seed data commands
   - [ ] Improve error messages and guidance
   - [ ] Add quick test commands for common scenarios
   - [ ] Create complete workflows (`local-docker`, `local-aws`)

3. **Documentation:**
   - [ ] Update Makefile help text
   - [ ] Add examples in comments
   - [ ] Create troubleshooting section
   - [ ] Document environment switching

## Summary

The current Makefile structure is good but lacks integration between the root development workflow and E2E testing. With your existing .env file containing AWS credentials and configurations, we need to support two workflows:

1. **Local Docker Development**: `make local-docker`
   - Uses Docker Compose for all services
   - LocalStack for AWS services
   - No external dependencies

2. **Local with AWS Services**: `make local-aws`
   - Uses SSH tunnels to RDS and ElastiCache
   - Direct AWS S3 access
   - Runs services locally with `go run`

By adding these targets and scripts, developers will have:

1. **Single command local setup**: `make local-docker` or `make local-aws`
2. **Integrated E2E testing**: `make test-e2e-local` or `make test-e2e-aws`
3. **Proper health checking**: `make validate-services`
4. **Environment management**: `make env-check`, `make env-local`, `make env-aws`
5. **SSH tunnel management**: `make tunnel-all`, `make tunnel-status`
6. **Better error messages** and guidance

This will significantly improve the developer experience and reduce the time to get a working local environment while supporting both Docker-only and AWS-integrated development workflows.
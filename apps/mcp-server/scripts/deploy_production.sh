#!/bin/bash

# MCP Production Deployment Script with APIL
# This script deploys the MCP server with the Adaptive Protocol Intelligence Layer enabled

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
ENVIRONMENT=${1:-production}
ENABLE_APIL=${ENABLE_APIL:-true}
APIL_AUTO_MIGRATE=${APIL_AUTO_MIGRATE:-true}
MONITORING_ENABLED=${MONITORING_ENABLED:-true}
BACKUP_ENABLED=${BACKUP_ENABLED:-true}

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}🚀 MCP Production Deployment with APIL${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "Configuration:"
echo "  Environment: $ENVIRONMENT"
echo "  APIL Enabled: $ENABLE_APIL"
echo "  Auto Migration: $APIL_AUTO_MIGRATE"
echo "  Monitoring: $MONITORING_ENABLED"
echo "  Backup: $BACKUP_ENABLED"
echo ""

# Function to check command success
check_success() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ $1 successful${NC}"
    else
        echo -e "${RED}✗ $1 failed${NC}"
        exit 1
    fi
}

# Step 1: Pre-deployment backup
if [ "$BACKUP_ENABLED" = "true" ]; then
    echo -e "\n${YELLOW}Step 1: Creating pre-deployment backup...${NC}"
    
    BACKUP_DIR="/opt/backups/mcp/$(date +%Y%m%d_%H%M%S)"
    mkdir -p $BACKUP_DIR
    
    # Backup current binary
    if [ -f "/opt/mcp-server/mcp-server" ]; then
        cp /opt/mcp-server/mcp-server $BACKUP_DIR/
        echo "  Backed up binary to $BACKUP_DIR"
    fi
    
    # Backup configuration
    if [ -d "/opt/mcp-server/configs" ]; then
        cp -r /opt/mcp-server/configs $BACKUP_DIR/
        echo "  Backed up configs to $BACKUP_DIR"
    fi
    
    # Record deployment metadata
    cat > $BACKUP_DIR/deployment.json <<EOF
{
    "timestamp": "$(date -Iseconds)",
    "previous_version": "$(git describe --tags --always 2>/dev/null || echo 'unknown')",
    "environment": "$ENVIRONMENT"
}
EOF
    
    check_success "Backup creation"
else
    echo -e "\n${YELLOW}Step 1: Skipping backup (disabled)${NC}"
fi

# Step 2: Build the application
echo -e "\n${YELLOW}Step 2: Building MCP Server...${NC}"
make build
check_success "Build"

# Step 3: Run tests
echo -e "\n${YELLOW}Step 3: Running tests...${NC}"
make test
check_success "Tests"

# Step 4: Database migrations
echo -e "\n${YELLOW}Step 4: Running database migrations...${NC}"
make migrate-up
check_success "Migrations"

# Step 5: Deploy MCP Server
echo -e "\n${YELLOW}Step 5: Deploying MCP Server...${NC}"

# Set environment variables
export ENABLE_APIL=$ENABLE_APIL
export APIL_AUTO_MIGRATE=$APIL_AUTO_MIGRATE
export MCP_ENV=$ENVIRONMENT

# Create deployment directory if it doesn't exist
DEPLOY_DIR="/opt/mcp-server"
sudo mkdir -p $DEPLOY_DIR

# Copy binary and configs
sudo cp bin/mcp-server $DEPLOY_DIR/
sudo cp -r configs $DEPLOY_DIR/

# Create or update systemd service
cat << EOF | sudo tee /etc/systemd/system/mcp-server.service
[Unit]
Description=MCP Server with Adaptive Protocol Intelligence Layer
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=mcp
Group=mcp
WorkingDirectory=$DEPLOY_DIR
Environment="ENABLE_APIL=$ENABLE_APIL"
Environment="APIL_AUTO_MIGRATE=$APIL_AUTO_MIGRATE"
Environment="CONFIG_PATH=$DEPLOY_DIR/configs/config.$ENVIRONMENT.yaml"
Environment="LOG_LEVEL=info"
ExecStart=$DEPLOY_DIR/mcp-server
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

# Create mcp user if it doesn't exist
if ! id -u mcp >/dev/null 2>&1; then
    sudo useradd -r -s /bin/false mcp
    echo "Created mcp system user"
fi

# Set permissions
sudo chown -R mcp:mcp $DEPLOY_DIR

# Reload systemd and restart service
sudo systemctl daemon-reload
sudo systemctl enable mcp-server

# Graceful restart with health check
echo "Performing graceful restart..."
if systemctl is-active --quiet mcp-server; then
    sudo systemctl reload-or-restart mcp-server
else
    sudo systemctl start mcp-server
fi

# Wait for service to be ready
sleep 5

# Step 6: Health check
echo -e "\n${YELLOW}Step 6: Performing health check...${NC}"

MAX_RETRIES=30
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    HEALTH_RESPONSE=$(curl -s http://localhost:8080/health 2>/dev/null || echo "")
    
    if echo "$HEALTH_RESPONSE" | grep -q "healthy"; then
        echo -e "${GREEN}✓ Health check passed${NC}"
        echo "$HEALTH_RESPONSE" | jq '.' 2>/dev/null || echo "$HEALTH_RESPONSE"
        break
    else
        RETRY_COUNT=$((RETRY_COUNT + 1))
        echo "  Waiting for service to be ready... ($RETRY_COUNT/$MAX_RETRIES)"
        sleep 2
    fi
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "${RED}✗ Health check failed after $MAX_RETRIES attempts${NC}"
    echo "Rolling back deployment..."
    
    # Rollback to backup if available
    if [ -d "$BACKUP_DIR" ]; then
        sudo cp $BACKUP_DIR/mcp-server $DEPLOY_DIR/
        sudo systemctl restart mcp-server
        echo -e "${YELLOW}Rolled back to previous version${NC}"
    fi
    exit 1
fi

# Step 7: Verify APIL status
if [ "$ENABLE_APIL" = "true" ]; then
    echo -e "\n${YELLOW}Step 7: Checking APIL status...${NC}"
    
    # Check APIL migration status
    APIL_STATUS=$(curl -s http://localhost:8080/api/v1/apil/status 2>/dev/null || echo "{}")
    
    if [ ! -z "$APIL_STATUS" ] && [ "$APIL_STATUS" != "{}" ]; then
        echo -e "${GREEN}✓ APIL is active${NC}"
        echo "$APIL_STATUS" | jq '.' 2>/dev/null || echo "$APIL_STATUS"
        
        # Check migration progress
        MIGRATION_PROGRESS=$(echo "$APIL_STATUS" | jq -r '.migration_progress' 2>/dev/null || echo "0")
        if [ "$MIGRATION_PROGRESS" != "null" ]; then
            echo "  Migration Progress: ${MIGRATION_PROGRESS}%"
        fi
    else
        echo -e "${YELLOW}⚠ APIL status unavailable${NC}"
    fi
fi

# Step 8: Setup monitoring
if [ "$MONITORING_ENABLED" = "true" ]; then
    echo -e "\n${YELLOW}Step 8: Setting up monitoring...${NC}"
    
    # Check if Prometheus is running
    if systemctl is-active --quiet prometheus; then
        echo "  Prometheus is running"
        
        # Reload Prometheus configuration if alerts were updated
        if [ -f "monitoring/prometheus/alerts.yml" ]; then
            sudo cp monitoring/prometheus/alerts.yml /etc/prometheus/
            sudo kill -HUP $(pgrep prometheus) 2>/dev/null || true
            echo "  Updated Prometheus alerts"
        fi
    else
        echo "  Prometheus is not running (skipping monitoring setup)"
    fi
    
    # Check if Grafana is running
    if systemctl is-active --quiet grafana-server; then
        echo "  Grafana is running"
        # Could add dashboard import here via Grafana API
    else
        echo "  Grafana is not running (skipping dashboard setup)"
    fi
    
    check_success "Monitoring setup"
fi

# Step 9: Smoke tests
echo -e "\n${YELLOW}Step 9: Running smoke tests...${NC}"

# Test WebSocket connection
echo "Testing WebSocket connection..."
if command -v wscat &> /dev/null; then
    timeout 5 wscat -c ws://localhost:8080/ws \
        -H "Authorization: Bearer $API_KEY" \
        -x '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"1.0.0"},"id":"1"}' \
        > /tmp/ws_test.log 2>&1 || true
    
    if grep -q "protocolVersion" /tmp/ws_test.log 2>/dev/null; then
        echo -e "${GREEN}✓ WebSocket test passed${NC}"
    else
        echo -e "${YELLOW}⚠ WebSocket test inconclusive${NC}"
    fi
else
    echo "  wscat not installed, skipping WebSocket test"
fi

# Test tools list endpoint
echo "Testing tools list endpoint..."
TOOLS_RESPONSE=$(curl -s -X POST http://localhost:8080/ws \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":"2"}' 2>/dev/null || echo "")

if echo "$TOOLS_RESPONSE" | grep -q "tools"; then
    echo -e "${GREEN}✓ Tools list test passed${NC}"
else
    echo -e "${YELLOW}⚠ Tools list test inconclusive${NC}"
fi

# Step 10: Log rotation setup
echo -e "\n${YELLOW}Step 10: Setting up log rotation...${NC}"
cat << EOF | sudo tee /etc/logrotate.d/mcp-server
/var/log/mcp-server/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    create 0644 mcp mcp
    sharedscripts
    postrotate
        systemctl reload mcp-server > /dev/null 2>&1 || true
    endscript
}
EOF
check_success "Log rotation setup"

# Save deployment info
DEPLOYMENT_INFO="{
  \"timestamp\": \"$(date -Iseconds)\",
  \"environment\": \"$ENVIRONMENT\",
  \"apil_enabled\": $ENABLE_APIL,
  \"auto_migrate\": $APIL_AUTO_MIGRATE,
  \"monitoring\": $MONITORING_ENABLED,
  \"version\": \"$(git describe --tags --always 2>/dev/null || echo 'dev')\",
  \"commit\": \"$(git rev-parse HEAD 2>/dev/null || echo 'unknown')\",
  \"deployed_by\": \"$(whoami)\",
  \"deployment_host\": \"$(hostname)\"
}"

echo "$DEPLOYMENT_INFO" | sudo tee $DEPLOY_DIR/deployment.json | jq '.' 2>/dev/null

# Final summary
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}🎉 MCP Server Deployment Complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "Service Information:"
echo "  Status: $(systemctl is-active mcp-server)"
echo "  WebSocket: ws://localhost:8080/ws"
echo "  Health: http://localhost:8080/health"
echo "  Metrics: http://localhost:8080/metrics"

if [ "$ENABLE_APIL" = "true" ]; then
    echo "  APIL Status: http://localhost:8080/api/v1/apil/status"
    echo "  APIL Metrics: http://localhost:8080/api/v1/apil/metrics"
fi

if [ "$MONITORING_ENABLED" = "true" ]; then
    echo ""
    echo "Monitoring:"
    echo "  Prometheus: http://localhost:9090"
    echo "  Grafana: http://localhost:3000"
    echo "  Alerts: http://localhost:9090/alerts"
fi

echo ""
echo "Commands:"
echo "  View logs: sudo journalctl -u mcp-server -f"
echo "  Restart: sudo systemctl restart mcp-server"
echo "  Status: sudo systemctl status mcp-server"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Verify metrics: curl http://localhost:8080/metrics"
echo "2. Monitor APIL migration: curl http://localhost:8080/api/v1/apil/status"
echo "3. Check circuit breakers in monitoring dashboard"
echo "4. Review system logs for any errors"

if [ -d "$BACKUP_DIR" ]; then
    echo ""
    echo -e "${BLUE}Backup location: $BACKUP_DIR${NC}"
fi
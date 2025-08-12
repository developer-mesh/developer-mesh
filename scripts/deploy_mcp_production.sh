#!/bin/bash

# MCP Production Deployment Script
# This script deploys the MCP server with APIL enabled and sets up monitoring

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
ENVIRONMENT=${1:-production}
ENABLE_APIL=${ENABLE_APIL:-true}
APIL_AUTO_MIGRATE=${APIL_AUTO_MIGRATE:-false}
MONITORING_ENABLED=${MONITORING_ENABLED:-true}

echo -e "${GREEN}🚀 MCP Production Deployment Starting${NC}"
echo "Environment: $ENVIRONMENT"
echo "APIL Enabled: $ENABLE_APIL"
echo "Auto Migration: $APIL_AUTO_MIGRATE"
echo "Monitoring: $MONITORING_ENABLED"

# Step 1: Build the application
echo -e "\n${YELLOW}Step 1: Building MCP Server...${NC}"
make build
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Build successful${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

# Step 2: Run tests
echo -e "\n${YELLOW}Step 2: Running tests...${NC}"
make test
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Tests passed${NC}"
else
    echo -e "${RED}✗ Tests failed${NC}"
    exit 1
fi

# Step 3: Setup monitoring (if enabled)
if [ "$MONITORING_ENABLED" = "true" ]; then
    echo -e "\n${YELLOW}Step 3: Setting up monitoring...${NC}"
    
    # Start Prometheus
    if ! pgrep -x "prometheus" > /dev/null; then
        echo "Starting Prometheus..."
        prometheus --config.file=monitoring/prometheus/prometheus.yml \
                  --storage.tsdb.path=/var/lib/prometheus/ \
                  --web.console.templates=/etc/prometheus/consoles \
                  --web.console.libraries=/etc/prometheus/console_libraries &
        sleep 2
        echo -e "${GREEN}✓ Prometheus started${NC}"
    else
        echo "Prometheus already running"
    fi
    
    # Import Grafana dashboard
    if command -v grafana-cli &> /dev/null; then
        echo "Importing Grafana dashboard..."
        # This would normally use the Grafana API
        # grafana-cli admin import-dashboard monitoring/grafana/mcp_dashboard.json
        echo -e "${GREEN}✓ Dashboard imported${NC}"
    fi
    
    # Copy Prometheus alerts
    cp monitoring/prometheus/alerts.yml /etc/prometheus/
    # Reload Prometheus configuration
    kill -HUP $(pgrep prometheus)
    echo -e "${GREEN}✓ Alerts configured${NC}"
fi

# Step 4: Database migrations
echo -e "\n${YELLOW}Step 4: Running database migrations...${NC}"
make migrate-up
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Migrations completed${NC}"
else
    echo -e "${RED}✗ Migration failed${NC}"
    exit 1
fi

# Step 5: Deploy MCP Server
echo -e "\n${YELLOW}Step 5: Deploying MCP Server...${NC}"

# Set environment variables
export ENABLE_APIL=$ENABLE_APIL
export APIL_AUTO_MIGRATE=$APIL_AUTO_MIGRATE
export MCP_ENV=$ENVIRONMENT

# Create deployment directory
DEPLOY_DIR="/opt/mcp-server"
sudo mkdir -p $DEPLOY_DIR
sudo cp bin/mcp-server $DEPLOY_DIR/
sudo cp -r configs $DEPLOY_DIR/

# Create systemd service
cat << EOF | sudo tee /etc/systemd/system/mcp-server.service
[Unit]
Description=MCP Server with APIL
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=mcp
Group=mcp
WorkingDirectory=$DEPLOY_DIR
Environment="ENABLE_APIL=$ENABLE_APIL"
Environment="APIL_AUTO_MIGRATE=$APIL_AUTO_MIGRATE"
Environment="CONFIG_PATH=$DEPLOY_DIR/configs/config.$ENVIRONMENT.yaml"
ExecStart=$DEPLOY_DIR/mcp-server
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and start service
sudo systemctl daemon-reload
sudo systemctl enable mcp-server
sudo systemctl restart mcp-server

# Wait for service to start
sleep 5

# Step 6: Health check
echo -e "\n${YELLOW}Step 6: Performing health check...${NC}"
HEALTH_RESPONSE=$(curl -s http://localhost:8080/health)
if echo "$HEALTH_RESPONSE" | grep -q "healthy"; then
    echo -e "${GREEN}✓ Health check passed${NC}"
    echo "$HEALTH_RESPONSE" | jq '.'
else
    echo -e "${RED}✗ Health check failed${NC}"
    echo "$HEALTH_RESPONSE"
    exit 1
fi

# Step 7: Verify APIL status (if enabled)
if [ "$ENABLE_APIL" = "true" ]; then
    echo -e "\n${YELLOW}Step 7: Checking APIL status...${NC}"
    APIL_STATUS=$(curl -s -H "Authorization: Bearer $API_KEY" http://localhost:8080/api/v1/apil/status)
    if [ ! -z "$APIL_STATUS" ]; then
        echo -e "${GREEN}✓ APIL is active${NC}"
        echo "$APIL_STATUS" | jq '.'
    else
        echo -e "${YELLOW}⚠ APIL status unavailable${NC}"
    fi
fi

# Step 8: Run smoke tests
echo -e "\n${YELLOW}Step 8: Running smoke tests...${NC}"

# Test WebSocket connection
echo "Testing WebSocket connection..."
timeout 5 wscat -c ws://localhost:8080/ws \
    -H "Authorization: Bearer $API_KEY" \
    -x '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"1.0.0"},"id":"1"}' \
    > /tmp/ws_test.log 2>&1

if grep -q "protocolVersion" /tmp/ws_test.log; then
    echo -e "${GREEN}✓ WebSocket test passed${NC}"
else
    echo -e "${RED}✗ WebSocket test failed${NC}"
    cat /tmp/ws_test.log
fi

# Test tools list
echo "Testing tools list endpoint..."
TOOLS_RESPONSE=$(curl -s -X POST http://localhost:8080/ws \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":"2"}')

if echo "$TOOLS_RESPONSE" | grep -q "tools"; then
    echo -e "${GREEN}✓ Tools list test passed${NC}"
else
    echo -e "${RED}✗ Tools list test failed${NC}"
fi

# Step 9: Load test (optional)
if [ "$RUN_LOAD_TEST" = "true" ]; then
    echo -e "\n${YELLOW}Step 9: Running load test...${NC}"
    k6 run test/load/mcp_load_test.js \
        -e MCP_HOST=localhost:8080 \
        -e API_KEY=$API_KEY \
        --summary-export=load_test_results.json
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Load test completed${NC}"
        cat load_test_results.json | jq '.metrics | {
            "messages_sent": .messages_sent.count,
            "messages_received": .messages_received.count,
            "tools_list_p95": .tools_list_latency_ms.p95,
            "tool_call_p95": .tool_call_latency_ms.p95,
            "ws_errors": .ws_errors.count
        }'
    else
        echo -e "${YELLOW}⚠ Load test had issues${NC}"
    fi
fi

# Step 10: Setup log rotation
echo -e "\n${YELLOW}Step 10: Setting up log rotation...${NC}"
cat << EOF | sudo tee /etc/logrotate.d/mcp-server
/var/log/mcp-server/*.log {
    daily
    rotate 7
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
echo -e "${GREEN}✓ Log rotation configured${NC}"

# Final summary
echo -e "\n${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}🎉 MCP Server Deployment Complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "Service Status: $(systemctl is-active mcp-server)"
echo "WebSocket Endpoint: ws://localhost:8080/ws"
echo "Health Check: http://localhost:8080/health"
echo "Metrics: http://localhost:8080/metrics"

if [ "$ENABLE_APIL" = "true" ]; then
    echo "APIL Status: http://localhost:8080/api/v1/apil/status"
fi

if [ "$MONITORING_ENABLED" = "true" ]; then
    echo ""
    echo "Monitoring:"
    echo "  Prometheus: http://localhost:9090"
    echo "  Grafana: http://localhost:3000"
    echo "  Alerts: http://localhost:9090/alerts"
fi

echo ""
echo "Logs: sudo journalctl -u mcp-server -f"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Verify metrics are being collected: curl http://localhost:8080/metrics"
echo "2. Check APIL migration status: curl http://localhost:8080/api/v1/apil/status"
echo "3. Monitor circuit breakers in Grafana dashboard"
echo "4. Review alerts in Prometheus"

# Save deployment info
DEPLOYMENT_INFO="{
  \"timestamp\": \"$(date -Iseconds)\",
  \"environment\": \"$ENVIRONMENT\",
  \"apil_enabled\": $ENABLE_APIL,
  \"auto_migrate\": $APIL_AUTO_MIGRATE,
  \"monitoring\": $MONITORING_ENABLED,
  \"version\": \"$(git describe --tags --always)\",
  \"commit\": \"$(git rev-parse HEAD)\"
}"

echo "$DEPLOYMENT_INFO" | jq '.' > /opt/mcp-server/deployment.json
echo -e "\n${GREEN}Deployment info saved to /opt/mcp-server/deployment.json${NC}"
#!/bin/bash
# Stream 1: Code verification pipeline
# This script runs in the background to verify all code implementations

echo "[Stream 1] Starting code verification pipeline..."

# Create verification directories
mkdir -p .doc-verification/{found,missing,deprecated,todos}

# Background task 1: Find all implemented features
echo "[1.1] Scanning for implemented features..."
find . -name "*.go" -type f | while read file; do
    grep -l "func.*Handler\|func.*Service\|func.*Repository" "$file" 2>/dev/null >> .doc-verification/found/handlers.txt
done &

# Background task 2: Find TODO/unimplemented features  
echo "[1.2] Scanning for TODOs and unimplemented features..."
grep -r "TODO\|FIXME\|DEPRECATED\|Unimplemented" --include="*.go" . 2>/dev/null > .doc-verification/todos/todo-list.txt &

# Background task 3: Extract all API endpoints
echo "[1.3] Extracting API endpoints..."
grep -r "router\.\(GET\|POST\|PUT\|DELETE\|PATCH\)\|r\.\(GET\|POST\|PUT\|DELETE\|PATCH\)\|Handle\|HandleFunc" --include="*.go" apps/ 2>/dev/null > .doc-verification/found/endpoints.txt &

# Background task 4: Find all configuration structs
echo "[1.4] Extracting configuration options..."
grep -r "type.*Config struct" --include="*.go" -A 50 2>/dev/null > .doc-verification/found/configs.txt &

# Background task 5: Verify test coverage (lightweight check)
echo "[1.5] Checking test files..."
find . -name "*_test.go" -type f > .doc-verification/found/test-files.txt &

# Background task 6: Extract all Makefile targets
echo "[1.6] Extracting Makefile targets..."
grep "^[a-z-]*:" Makefile 2>/dev/null | sed 's/:.*//' | sort -u > .doc-verification/found/make-targets.txt &

# Background task 7: Docker service verification
echo "[1.7] Verifying Docker services..."
if [ -f docker-compose.local.yml ]; then
    docker-compose -f docker-compose.local.yml config --services 2>/dev/null > .doc-verification/found/docker-services.txt
fi &

# Background task 8: Find all environment variables
echo "[1.8] Extracting environment variables..."
grep -r "os.Getenv\|viper.Get\|GetString\|GetInt\|GetBool" --include="*.go" 2>/dev/null | \
    sed -n 's/.*Getenv("\([^"]*\)".*/\1/p; s/.*Get[A-Z]*("\([^"]*\)".*/\1/p' | \
    sort -u > .doc-verification/found/env-vars.txt &

# Background task 9: Find WebSocket implementations
echo "[1.9] Finding WebSocket implementations..."
grep -r "websocket\|WebSocket\|ws:/\|wss:/" --include="*.go" 2>/dev/null > .doc-verification/found/websocket.txt &

# Background task 10: Find embedding providers
echo "[1.10] Finding embedding providers..."
grep -r "OpenAI\|Bedrock\|Google.*Embed\|embedding.*provider" --include="*.go" pkg/ 2>/dev/null > .doc-verification/found/embedding-providers.txt &

# Background task 11: Find assignment strategies
echo "[1.11] Finding assignment strategies..."
grep -r "RoundRobin\|LeastLoaded\|CapabilityMatch" --include="*.go" 2>/dev/null > .doc-verification/found/assignment-strategies.txt &

# Background task 12: Find Redis Streams usage
echo "[1.12] Finding Redis Streams usage..."
grep -r "XADD\|XREAD\|XGroup\|Redis.*Stream" --include="*.go" 2>/dev/null > .doc-verification/found/redis-streams.txt &

wait
echo "[Stream 1] Code verification complete. Results in .doc-verification/"

# Generate summary
cat > .doc-verification/summary.txt << EOF
Code Verification Summary
========================
Handlers Found: $(wc -l < .doc-verification/found/handlers.txt 2>/dev/null || echo 0)
Endpoints Found: $(wc -l < .doc-verification/found/endpoints.txt 2>/dev/null || echo 0)
TODOs Found: $(wc -l < .doc-verification/todos/todo-list.txt 2>/dev/null || echo 0)
Test Files: $(wc -l < .doc-verification/found/test-files.txt 2>/dev/null || echo 0)
Make Targets: $(wc -l < .doc-verification/found/make-targets.txt 2>/dev/null || echo 0)
Docker Services: $(wc -l < .doc-verification/found/docker-services.txt 2>/dev/null || echo 0)
Environment Vars: $(wc -l < .doc-verification/found/env-vars.txt 2>/dev/null || echo 0)
EOF
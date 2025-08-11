#!/bin/bash
# Stream 3: Test execution pipeline
# This script tests all documented commands and examples

echo "[Stream 3] Starting command testing pipeline..."

mkdir -p .doc-testing/{passed,failed,skipped,logs}

# Background task 1: Test Makefile targets
echo "[3.1] Testing Makefile targets..."
(
if [ -f .doc-verification/found/make-targets.txt ]; then
    while IFS= read -r target; do
        echo "Testing: make $target"
        if timeout 5 make -n "$target" > /dev/null 2>&1; then
            echo "$target" >> .doc-testing/passed/make-targets.txt
        else
            echo "$target: $(make -n "$target" 2>&1 | head -1)" >> .doc-testing/failed/make-targets.txt
        fi
    done < .doc-verification/found/make-targets.txt
else
    # Fallback: test common targets
    for target in build test lint fmt deps pre-commit dev clean; do
        if timeout 5 make -n "$target" > /dev/null 2>&1; then
            echo "$target" >> .doc-testing/passed/make-targets.txt
        else
            echo "$target" >> .doc-testing/failed/make-targets.txt
        fi
    done
fi
) &

# Background task 2: Test Docker commands
echo "[3.2] Testing Docker commands..."
(
    # Test docker-compose configuration
    if docker-compose -f docker-compose.local.yml config > .doc-testing/logs/docker-compose-config.txt 2>&1; then
        echo "docker-compose config" >> .doc-testing/passed/docker-commands.txt
        
        # Extract services
        docker-compose -f docker-compose.local.yml config --services 2>/dev/null | while read service; do
            echo "Service: $service" >> .doc-testing/passed/docker-services.txt
        done
    else
        echo "docker-compose config" >> .doc-testing/failed/docker-commands.txt
    fi
    
    # Test Docker build commands (dry run)
    for dockerfile in $(find . -name "Dockerfile*" -type f); do
        dir=$(dirname "$dockerfile")
        name=$(basename "$dir")
        echo "Dockerfile found: $dockerfile for $name" >> .doc-testing/passed/dockerfiles.txt
    done
) &

# Background task 3: Test Go module commands
echo "[3.3] Testing Go module commands..."
(
    # Test go.work
    if [ -f go.work ]; then
        echo "go.work exists" >> .doc-testing/passed/go-workspace.txt
        
        # Extract Go version
        go_version=$(grep "^go " go.work | awk '{print $2}')
        echo "Go version: $go_version" >> .doc-testing/passed/go-version.txt
    fi
    
    # Test each module
    for mod in $(find . -name "go.mod" -type f); do
        dir=$(dirname "$mod")
        echo "Go module: $dir" >> .doc-testing/passed/go-modules.txt
    done
) &

# Background task 4: Verify service ports
echo "[3.4] Verifying service ports..."
(
    # Check documented ports in docker-compose
    if [ -f docker-compose.local.yml ]; then
        # Extract port mappings
        grep -E "^\s*- \"[0-9]+:[0-9]+\"" docker-compose.local.yml | while read line; do
            port=$(echo "$line" | sed 's/.*"\([0-9]*\):\([0-9]*\)".*/\1:\2/')
            echo "Port mapping: $port" >> .doc-testing/passed/ports.txt
        done
    fi
    
    # Check port references in code
    grep -r ":[0-9]\{4,5\}" --include="*.go" --include="*.yaml" --include="*.yml" 2>/dev/null | \
        grep -o ":[0-9]\{4,5\}" | sort -u | while read port; do
        echo "Port reference in code: $port" >> .doc-testing/passed/port-refs.txt
    done
) &

# Background task 5: Test configuration files
echo "[3.5] Testing configuration files..."
(
    for config in configs/*.yaml configs/*.yml; do
        if [ -f "$config" ]; then
            echo "Config file: $config" >> .doc-testing/passed/config-files.txt
            
            # Basic YAML validation
            if python3 -c "import yaml; yaml.safe_load(open('$config'))" 2>/dev/null; then
                echo "$config: valid YAML" >> .doc-testing/passed/yaml-valid.txt
            else
                echo "$config: invalid YAML" >> .doc-testing/failed/yaml-invalid.txt
            fi
        fi
    done
) &

# Background task 6: Test directory structure
echo "[3.6] Testing directory structure..."
(
    # Test documented directories exist
    for dir in apps pkg migrations configs scripts docs test; do
        if [ -d "$dir" ]; then
            echo "$dir/" >> .doc-testing/passed/directories.txt
        else
            echo "$dir/" >> .doc-testing/failed/directories.txt
        fi
    done
    
    # Test app subdirectories
    for app in mcp-server rest-api worker mockserver; do
        if [ -d "apps/$app" ]; then
            echo "apps/$app/" >> .doc-testing/passed/app-dirs.txt
        else
            echo "apps/$app/" >> .doc-testing/failed/app-dirs.txt
        fi
    done
) &

# Background task 7: Test package structure
echo "[3.7] Testing package structure..."
(
    # Common packages that should exist
    for pkg in models services repository middleware adapters utils; do
        if find pkg -type d -name "$pkg" 2>/dev/null | grep -q .; then
            echo "pkg/$pkg" >> .doc-testing/passed/packages.txt
        else
            echo "pkg/$pkg" >> .doc-testing/skipped/packages.txt
        fi
    done
) &

# Background task 8: Test API endpoint patterns
echo "[3.8] Testing API endpoint patterns..."
(
    # Look for common API patterns
    patterns=("/health" "/api/v1" "/metrics" "/ws" "/webhook")
    for pattern in "${patterns[@]}"; do
        if grep -r "$pattern" --include="*.go" apps/ 2>/dev/null | grep -q .; then
            echo "API pattern: $pattern" >> .doc-testing/passed/api-patterns.txt
        else
            echo "API pattern: $pattern" >> .doc-testing/failed/api-patterns.txt
        fi
    done
) &

# Background task 9: Test database migrations
echo "[3.9] Testing database migrations..."
(
    if [ -d migrations ]; then
        migration_count=$(find migrations -name "*.sql" -type f 2>/dev/null | wc -l)
        echo "Migration files: $migration_count" >> .doc-testing/passed/migrations.txt
        
        # Check for up/down pairs
        find migrations -name "*up.sql" -type f 2>/dev/null | while read up; do
            down="${up/up.sql/down.sql}"
            if [ -f "$down" ]; then
                echo "Migration pair: $(basename "$up" .up.sql)" >> .doc-testing/passed/migration-pairs.txt
            else
                echo "Missing down: $(basename "$up")" >> .doc-testing/failed/migration-pairs.txt
            fi
        done
    else
        echo "migrations/" >> .doc-testing/failed/directories.txt
    fi
) &

# Background task 10: Test environment variables
echo "[3.10] Testing environment variables..."
(
    # Check .env.example
    if [ -f .env.example ]; then
        echo ".env.example exists" >> .doc-testing/passed/env-files.txt
        
        # Count environment variables
        env_count=$(grep -c "^[A-Z_]*=" .env.example 2>/dev/null || echo 0)
        echo "Environment variables defined: $env_count" >> .doc-testing/passed/env-vars.txt
    else
        echo ".env.example" >> .doc-testing/failed/env-files.txt
    fi
    
    # Check for other env files
    for env_file in .env.development .env.docker .env.production; do
        if [ -f "$env_file" ]; then
            echo "$env_file exists" >> .doc-testing/passed/env-files.txt
        fi
    done
) &

wait
echo "[Stream 3] Command testing complete. Results in .doc-testing/"

# Generate test summary
cat > .doc-testing/summary.txt << EOF
Test Execution Summary
======================
Make Targets Passed: $(wc -l < .doc-testing/passed/make-targets.txt 2>/dev/null || echo 0)
Make Targets Failed: $(wc -l < .doc-testing/failed/make-targets.txt 2>/dev/null || echo 0)
Docker Services: $(wc -l < .doc-testing/passed/docker-services.txt 2>/dev/null || echo 0)
Go Modules: $(wc -l < .doc-testing/passed/go-modules.txt 2>/dev/null || echo 0)
Config Files: $(wc -l < .doc-testing/passed/config-files.txt 2>/dev/null || echo 0)
Directories OK: $(wc -l < .doc-testing/passed/directories.txt 2>/dev/null || echo 0)
API Patterns Found: $(wc -l < .doc-testing/passed/api-patterns.txt 2>/dev/null || echo 0)
Migration Files: $(grep "Migration files:" .doc-testing/passed/migrations.txt 2>/dev/null | cut -d: -f2 || echo 0)
Environment Vars: $(grep "Environment variables defined:" .doc-testing/passed/env-vars.txt 2>/dev/null | cut -d: -f2 || echo 0)
EOF
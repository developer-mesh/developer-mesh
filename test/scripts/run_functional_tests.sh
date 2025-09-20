#!/bin/bash

# run_functional_tests.sh - Script to run functional tests
# Usage: ./run_functional_tests.sh [options]
#   Options:
#     -v, --verbose     Show verbose output
#     -k, --keep-up     Keep containers up after tests finish
#     -f, --focus TEXT  Run only specs matching the TEXT
#     -b, --build       Force rebuild of Docker images
#
# Environment variables:
#   FORCE_BUILD=true  Force rebuild of Docker images

set -e  # Exit on error

# Parse command line arguments
VERBOSE=0
KEEP_UP=0
FOCUS=""
FORCE_BUILD=${FORCE_BUILD:-false}

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -v|--verbose)
      VERBOSE=1
      shift
      ;;
    -k|--keep-up)
      KEEP_UP=1
      shift
      ;;
    -f|--focus)
      FOCUS="$2"
      shift
      shift
      ;;
    -b|--build)
      FORCE_BUILD=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Get the script directory
SCRIPT_DIR="$(dirname "$0")"

# Navigate to project root (two directories up from script location)
cd "$SCRIPT_DIR/../.."
PROJECT_ROOT=$(pwd)

echo "Starting functional tests from $PROJECT_ROOT"

# Find Go and Ginkgo in PATH
GO_PATH=$(which go)
GINKGO_PATH=$(which ginkgo)

# Verify that Go is available
if [ -z "$GO_PATH" ]; then
    echo "Error: Go executable not found in PATH"
    echo "Please ensure Go is installed and in your PATH"
    exit 1
fi

# Verify that Ginkgo is available
if [ -z "$GINKGO_PATH" ]; then
    echo "Error: Ginkgo executable not found in PATH"
    echo "Installing Ginkgo..."
    go install github.com/onsi/ginkgo/v2/ginkgo@latest
    GINKGO_PATH=$(which ginkgo)
    if [ -z "$GINKGO_PATH" ]; then
        echo "Failed to install Ginkgo. Please install it manually:"
        echo "  go install github.com/onsi/ginkgo/v2/ginkgo@latest"
        exit 1
    fi
fi

echo "Using Go at: $GO_PATH"
echo "Using Ginkgo at: $GINKGO_PATH"

# Function to clean up resources
cleanup() {
    if [ $KEEP_UP -eq 0 ]; then
        echo "Cleaning up test environment..."
        docker-compose -f $PROJECT_ROOT/docker-compose.local.yml down -v
    else
        echo "Keeping containers up as requested. Use the following command to stop them:"
        echo "    docker-compose -f docker-compose.local.yml down -v"
    fi
}

# Register cleanup function to be called on script exit
trap cleanup EXIT

# Start the test environment
echo "Starting test environment with Docker Compose..."
docker-compose -f $PROJECT_ROOT/docker-compose.local.yml down -v --remove-orphans

# Only build if explicitly requested or images don't exist
if [ "${FORCE_BUILD}" = "true" ]; then
    echo "Building Docker images (forced)..."
    docker-compose -f $PROJECT_ROOT/docker-compose.local.yml build
elif ! docker images | grep -q "developer-mesh_edge-mcp\|developer-mesh_rest-api"; then
    echo "Docker images not found, building..."
    docker-compose -f $PROJECT_ROOT/docker-compose.local.yml build
else
    echo "Using existing Docker images (set FORCE_BUILD=true or use --build to rebuild)"
fi

docker-compose -f $PROJECT_ROOT/docker-compose.local.yml up -d

# Wait for all services to be healthy
echo "Waiting for all services to be healthy..."
sleep 5  # Initial wait for services to start

# Check PostgreSQL
echo "Checking PostgreSQL..."
max_attempts=20
attempt=1
while [ $attempt -le $max_attempts ]; do
    if docker-compose -f $PROJECT_ROOT/docker-compose.local.yml exec database pg_isready -U dev > /dev/null 2>&1; then
        echo "PostgreSQL is ready!"
        break
    fi
    echo "Waiting for PostgreSQL to be ready (attempt $attempt/$max_attempts)..."
    sleep 3
    attempt=$((attempt+1))
done

if [ $attempt -gt $max_attempts ]; then
    echo "Error: PostgreSQL is not ready after $max_attempts attempts"
    exit 1
fi

# Check MCP Server
echo "Checking MCP Server..."
max_attempts=30
attempt=1
while [ $attempt -le $max_attempts ]; do
    if curl -f -s http://localhost:8080/health > /dev/null 2>&1; then
        echo "MCP Server is ready!"
        break
    fi
    echo "Waiting for MCP Server to be ready (attempt $attempt/$max_attempts)..."
    sleep 3
    attempt=$((attempt+1))
done

if [ $attempt -gt $max_attempts ]; then
    echo "Error: Edge MCP Server is not ready after $max_attempts attempts"
    echo "Checking container logs..."
    docker-compose -f $PROJECT_ROOT/docker-compose.local.yml logs --tail=50 edge-mcp
    exit 1
fi

# Check REST API
echo "Checking REST API..."
max_attempts=30
attempt=1
while [ $attempt -le $max_attempts ]; do
    if curl -f -s http://localhost:8081/health > /dev/null 2>&1; then
        echo "REST API is ready!"
        break
    fi
    echo "Waiting for REST API to be ready (attempt $attempt/$max_attempts)..."
    sleep 3
    attempt=$((attempt+1))
done

if [ $attempt -gt $max_attempts ]; then
    echo "Error: REST API is not ready after $max_attempts attempts"
    echo "Checking container logs..."
    docker-compose -f $PROJECT_ROOT/docker-compose.local.yml logs --tail=50 rest-api
    exit 1
fi

# Set environment variables for the tests
# REST API configuration
export REST_API_URL="http://localhost:8081"
export API_KEY="dev-admin-key-1234567890"

# MCP Server configuration  
export MCP_SERVER_URL="http://localhost:8080"
export MCP_API_KEY="dev-admin-key-1234567890"

# Mock server for testing
export MOCKSERVER_URL="http://localhost:8082"

# Run the functional tests on the host machine instead of inside Docker
echo "Running functional tests using host Go installation..."
if [ -n "$FOCUS" ]; then
    echo "Focusing on tests matching: $FOCUS"
    focus_arg="--focus=$FOCUS"
else
    focus_arg=""
fi

# Navigate to functional test directory
cd $PROJECT_ROOT/test/functional

echo "Running Ginkgo functional tests..."
if [ $VERBOSE -eq 1 ]; then
    $GINKGO_PATH -r --randomize-all --race $focus_arg -v .
else
    $GINKGO_PATH -r --randomize-all --race $focus_arg .
fi

echo "Functional tests completed successfully!"

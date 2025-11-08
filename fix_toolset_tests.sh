#!/bin/bash

# Fix all test files to use observability.NewNoopLogger() when creating ToolsetManager

# Fix remote_toolset_provider_test.go
sed -i '' 's/NewToolsetManager(registry)/NewToolsetManager(registry, observability.NewNoopLogger())/g' apps/edge-mcp/internal/tools/builtin/remote_toolset_provider_test.go

# Fix toolset_manager_test.go
sed -i '' 's/NewToolsetManager(registry)/NewToolsetManager(registry, observability.NewNoopLogger())/g' apps/edge-mcp/internal/tools/toolset_manager_test.go

# Fix search_provider_test.go
sed -i '' 's/NewToolsetManager(registry)/NewToolsetManager(registry, observability.NewNoopLogger())/g' apps/edge-mcp/internal/tools/builtin/search_provider_test.go

echo "Fixed all test files"
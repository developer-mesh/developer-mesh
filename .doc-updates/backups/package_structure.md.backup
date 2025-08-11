# Package Structure Guidelines

## Top-level directories
- `cmd/` - Main applications for each service
- `internal/` - Private application code not meant to be imported
- `pkg/` - Shared libraries that can be used by external applications
- `apps/` - Application-specific code organized by service

## Package Placement Criteria

### When to use pkg/common/
Place code in `pkg/common/` when:
- It's shared utilities with no domain-specific logic
- It provides adapters to external libraries
- It's infrastructure code (AWS, database, etc.)

### When to use pkg/ directly
Place code in direct `pkg/` subdirectories when:
- It represents a domain concept (models, mcp, embedding)
- It's an API wrapper with domain-specific logic
- It's meant to be a primary API for consumers

### When to use apps/*/internal/
Place code in application-specific internal directories when:
- It's specific to a single application
- It shouldn't be imported by other applications
- It contains implementation details of the application

## Naming conventions
- Use singular nouns for package names (e.g., `model`, not `models`)
- Avoid abbreviations unless widely understood
- Follow Go community conventions for package names
- Use lowercase, single-word names

## Import Order
All files should follow this import order:
```go
import (
    // Standard library imports
    "context"
    "fmt"
    
    // External dependencies
    "github.com/gin-gonic/gin"
    
    // Internal packages - organized from most general to most specific
    "github.com/developer-mesh/developer-mesh/pkg/config"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/developer-mesh/developer-mesh/apps/rest-api/internal/core"
)
```

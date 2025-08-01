# Adapters Package

This package provides a clean, standardized way to integrate with external services following Go best practices.

## Architecture

### Clean Structure
```
pkg/adapters/
├── go.mod              # Single module for all adapters
├── interfaces.go       # Common interfaces and types
├── factory.go         # Factory pattern for creating adapters
├── setup.go           # Manager for adapter lifecycle
├── github/            # GitHub adapter implementation
│   ├── adapter_clean.go
│   ├── config.go
│   └── register.go
└── example_test.go    # Usage examples
```

### Key Interfaces

1. **SourceControlAdapter** - Standard interface for source control systems
   - Repository operations (Get, List)
   - Pull Request operations (Get, Create, List)
   - Issue operations (Get, Create, List)
   - Webhook handling
   - Health checks

2. **Factory** - Creates adapters based on provider type
   - Registers provider functions
   - Manages configurations
   - Creates adapter instances

3. **Manager** - High-level adapter management
   - Simplified API for getting adapters
   - Automatic health checks
   - Configuration management

## Usage

```go
// Create adapter manager
manager := adapters.NewManager(logger)

// Configure GitHub adapter
manager.SetConfig("github", adapters.Config{
    Timeout: 30 * time.Second,
    ProviderConfig: map[string]interface{}{
        "token": "your-github-token",
    },
})

// Get adapter
adapter, err := manager.GetAdapter(ctx, "github")
if err != nil {
    return err
}

// Use adapter
repos, err := adapter.ListRepositories(ctx, "owner")
```

## Design Principles

1. **Single Module** - One go.mod file for the entire adapters package
2. **Clear Interfaces** - Specific interfaces for different adapter types
3. **No Duplication** - Each provider has one implementation
4. **Factory Pattern** - Clean separation between creation and usage
5. **Provider Independence** - Each provider is self-contained

## Adding New Providers

1. Create a new directory (e.g., `gitlab/`)
2. Implement the `SourceControlAdapter` interface
3. Create a registration function
4. Register in `setup.go`

Example:
```go
// gitlab/adapter.go
type GitLabAdapter struct {
    adapters.BaseAdapter
    // ... fields
}

func New(ctx context.Context, config adapters.Config, logger observability.Logger) (adapters.SourceControlAdapter, error) {
    // Implementation
}

// gitlab/register.go
func Register(factory *adapters.Factory) error {
    return factory.RegisterProvider("gitlab", New)
}
```

## Benefits

1. **Simplicity** - Clean, easy-to-understand structure
2. **Testability** - Interfaces make mocking easy
3. **Extensibility** - Easy to add new providers
4. **Type Safety** - Strong typing throughout
5. **No Import Cycles** - Clear dependency flow
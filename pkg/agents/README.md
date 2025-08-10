# Agent Package - Three-Tier Architecture

## Overview

The `agents` package implements a three-tier architecture for managing AI agents in the Developer Mesh platform. This architecture provides multi-tenancy, multi-instance support, and flexible configuration management.

## Architecture

```
┌─────────────────┐
│ Agent Manifests │ ← System-wide agent type definitions
└────────┬────────┘
         │
┌────────▼────────┐
│ Configurations  │ ← Tenant-specific settings
└────────┬────────┘
         │
┌────────▼────────┐
│ Registrations   │ ← Active running instances
└─────────────────┘
```

## Core Components

### 1. Repository (`repository_three_tier.go`)
The main repository implementing the three-tier model with methods for:
- Managing agent manifests (blueprints)
- Configuring agents per tenant
- Registering and tracking active instances
- Finding available agents for work assignment
- Health monitoring and cleanup

### 2. Types (`types_three_tier.go`)
Data structures for the three-tier architecture:
- `AgentManifest`: Agent type definitions
- `AgentConfiguration`: Tenant-specific configurations
- `AgentRegistration`: Active instance tracking
- `AvailableAgent`: Agents ready for work
- `AgentMetrics`: Performance and health metrics

### 3. Service Layer (`service.go`)
Business logic for agent operations:
- Agent selection based on availability
- Workload management
- Health monitoring
- Event processing

## Usage Examples

### Creating an Agent Manifest

```go
repo := NewThreeTierRepository(db, "mcp")

manifest := &AgentManifest{
    AgentID:     "ide-agent",
    AgentType:   "ide",
    Name:        "IDE Agent",
    Description: "Provides IDE integration capabilities",
    Version:     "1.0.0",
    Capabilities: map[string]interface{}{
        "features": []string{"code_editing", "debugging", "testing"},
        "languages": []string{"go", "python", "javascript"},
    },
    Status: "active",
}

err := repo.CreateManifest(ctx, manifest)
```

### Configuring an Agent for a Tenant

```go
config := &AgentConfiguration{
    TenantID:   tenantID,
    ManifestID: manifestID,
    Name:       "Development IDE",
    Enabled:    true,
    Configuration: map[string]interface{}{
        "workspace": "/projects",
        "theme": "dark",
    },
    SystemPrompt: "You are a helpful coding assistant",
    Temperature:  0.7,
    MaxTokens:    4096,
    MaxWorkload:  5,
}

err := repo.CreateConfiguration(ctx, config)
```

### Registering an Agent Instance

```go
// This is called when an agent connects (e.g., IDE opens)
registration := &AgentRegistration{
    TenantID:   tenantID,
    AgentID:    "ide-agent",
    InstanceID: websocketConnID, // Unique per connection
    Name:       "VS Code Instance",
    ConnectionDetails: map[string]interface{}{
        "ip": "192.168.1.100",
        "version": "1.84.0",
    },
}

result, err := repo.RegisterInstance(ctx, registration)
// result.IsNew tells you if this is a new registration or reconnection
```

### Finding Available Agents

```go
// Get all available agents for a tenant
agents, err := repo.GetAvailableAgents(ctx, tenantID)

// Agents are returned sorted by availability score
for _, agent := range agents {
    fmt.Printf("Agent: %s, Score: %.2f, Workload: %d/%d\n",
        agent.ConfigName,
        agent.AvailabilityScore,
        agent.CurrentWorkload,
        agent.MaxWorkload,
    )
}
```

### Updating Agent Health

```go
// Regular health check from agent
err := repo.UpdateHealth(ctx, instanceID, HealthStatusHealthy)

// Clean up stale registrations
err := repo.Cleanup(ctx, 5*time.Minute)
```

## Database Tables

The package uses three main tables:

1. **mcp.agent_manifests** - Agent type definitions
2. **mcp.agent_configurations** - Tenant-specific configurations  
3. **mcp.agent_registrations** - Active instance tracking

See [AGENT_ARCHITECTURE.md](../../docs/AGENT_ARCHITECTURE.md) for complete schema.

## Key Features

### Idempotent Registration
- Reconnections don't create duplicate registrations
- Instance ID persists across reconnects
- Handles network interruptions gracefully

### Multi-Instance Support
- Multiple instances per configuration
- Load balancing across instances
- Health-based instance selection

### Workload Management
- Track current vs maximum workload
- Prevent overloading agents
- Automatic workload distribution

### Health Monitoring
- Per-instance health tracking
- Automatic stale instance cleanup
- Health-based availability scoring

## Testing

```bash
# Run unit tests
go test ./pkg/agents/...

# Run with coverage
go test -cover ./pkg/agents/...

# Run integration tests (requires database)
go test -tags=integration ./pkg/agents/...
```

## Best Practices

1. **Use Instance IDs**: Always provide a stable instance_id for registrations
2. **Regular Health Checks**: Update health status every 30-60 seconds
3. **Clean Up**: Run cleanup regularly to remove stale registrations
4. **Monitor Workload**: Check availability before assigning tasks
5. **Handle Errors**: Use exponential backoff for retries

## Directory Structure

```
pkg/agents/
├── repository_three_tier.go    # Three-tier repository implementation
├── types_three_tier.go         # Type definitions for three-tier model
├── service.go                  # Business logic layer
├── README.md                   # This file
└── tests/                      # Test files
```

## Related Documentation

- [Agent Architecture](../../docs/AGENT_ARCHITECTURE.md) - Detailed architecture documentation
- [API Documentation](../../apps/rest-api/README.md) - REST API endpoints
- [MCP Server](../../apps/mcp-server/README.md) - WebSocket server for agents

---

Package Version: 2.0.0
Last Updated: 2025-08-10

**Note**: This is the production three-tier architecture for Developer Mesh. There is no legacy system - this is the desired state implementation.
<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:27:11
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Agent Architecture - Three-Tier Model

## Overview

The Developer Mesh agent system uses a **three-tier architecture** that provides flexibility, scalability, and multi-tenancy support. This architecture separates agent definitions from tenant configurations and running instances.

## Architecture Tiers

### 1. Agent Manifests (Blueprints)
The top tier defines **what** agents are available in the system.

- **Purpose**: Define agent types and their capabilities
- **Scope**: System-wide, shared across all tenants
- **Examples**: IDE Agent, Slack Bot, GitHub Agent
- **Key Fields**:
  - `agent_id`: Unique identifier (e.g., "ide-agent")
  - `agent_type`: Category (e.g., "ide", "chat", "automation")
  - `capabilities`: What the agent can do
  - `requirements`: System requirements
  - `version`: Semantic version of the agent

### 2. Agent Configurations (Tenant Settings)
The middle tier defines **how** each tenant configures agents for their use.

- **Purpose**: Tenant-specific customization of agents
- **Scope**: Per-tenant, isolated configurations
- **Key Fields**:
  - `tenant_id`: Which tenant owns this configuration
  - `manifest_id`: Which agent type this configures
  - `configuration`: Tenant-specific settings
  - `system_prompt`: Custom LLM instructions
  - `model_id`: Which AI model to use
  - `max_workload`: Concurrency limits

### 3. Agent Registrations (Running Instances)
The bottom tier tracks **active instances** of agents.

- **Purpose**: Track running agent instances and their health
- **Scope**: Per-instance, supports multiple instances per configuration
- **Key Fields**:
  - `instance_id`: Unique instance identifier (e.g., WebSocket connection ID) <!-- Source: pkg/models/websocket/binary.go -->
  - `health_status`: Current health state (healthy, degraded, unknown)
  - `registration_status`: Active or inactive
  - `connection_details`: WebSocket ID, IP address, etc. <!-- Source: pkg/models/websocket/binary.go -->
  - `last_health_check`: When the instance last reported health

## Database Schema

```sql
-- Tier 1: Agent Manifests (Blueprints)
CREATE TABLE mcp.agent_manifests (
    id UUID PRIMARY KEY,
    agent_id TEXT UNIQUE NOT NULL,        -- "ide-agent", "slack-bot"
    agent_type TEXT NOT NULL,             -- "ide", "chat", "automation"
    name TEXT NOT NULL,
    description TEXT,
    version TEXT NOT NULL,
    capabilities JSONB,                   -- What the agent can do
    requirements TEXT,                    -- System requirements
    metadata JSONB,
    status TEXT,                          -- active, deprecated, beta
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- Tier 2: Agent Configurations (Tenant Settings)
CREATE TABLE mcp.agent_configurations (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    manifest_id UUID REFERENCES agent_manifests(id),
    name TEXT NOT NULL,
    enabled BOOLEAN DEFAULT true,
    configuration JSONB,                  -- Tenant-specific settings
    system_prompt TEXT,
    temperature NUMERIC,
    max_tokens INTEGER,
    model_id UUID,
    max_workload INTEGER DEFAULT 10,
    current_workload INTEGER DEFAULT 0,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    UNIQUE(tenant_id, manifest_id)
);

-- Tier 3: Agent Registrations (Active Instances)
CREATE TABLE mcp.agent_registrations (
    id UUID PRIMARY KEY,
    manifest_id UUID REFERENCES agent_manifests(id),
    tenant_id UUID NOT NULL,
    instance_id TEXT UNIQUE NOT NULL,     -- Unique instance identifier
    registration_status TEXT,              -- active, inactive
    health_status TEXT,                    -- healthy, degraded, unknown
    connection_details JSONB,             -- WebSocket details <!-- Source: pkg/models/websocket/binary.go -->
    runtime_config JSONB,
    activation_date TIMESTAMP,
    deactivation_date TIMESTAMP,
    last_health_check TIMESTAMP,
    failure_count INTEGER DEFAULT 0,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    UNIQUE(tenant_id, instance_id)
);
```

## Key Operations

### 1. Agent Registration (Idempotent)
When an agent connects (e.g., IDE opens), it registers itself:

```go
// The registration is idempotent - reconnections reuse existing registrations
result := repository.RegisterInstance(ctx, &AgentRegistration{
    TenantID:   tenantID,
    AgentID:    "ide-agent",
    InstanceID: websocketID,  // Unique per connection <!-- Source: pkg/models/websocket/binary.go -->
    Name:       "VS Code",
})
```

### 2. Finding Available Agents
To find agents ready to handle work:

```go
// Get all configured agents with healthy registrations
agents := repository.GetAvailableAgents(ctx, tenantID)
// Returns agents sorted by availability score
```

### 3. Health Management
Agents report health periodically:

```go
// Update health status for an instance
repository.UpdateHealth(ctx, instanceID, HealthStatusHealthy)

// Clean up stale registrations
repository.Cleanup(ctx, 5*time.Minute)
```

## Benefits of Three-Tier Architecture

### 1. **Multi-Instance Support**
- Multiple IDE windows can connect simultaneously
- Each gets its own registration with unique instance ID
- Load balancing across healthy instances

### 2. **Graceful Reconnections**
- Instance ID persists across reconnections
- WebSocket reconnects reuse existing registration <!-- Source: pkg/models/websocket/binary.go -->
- No duplicate registrations on network interruptions

### 3. **Multi-Tenancy**
- Single manifest serves all tenants
- Each tenant has isolated configurations
- Tenant-specific customization without code changes

### 4. **Health Tracking**
- Per-instance health monitoring
- Automatic failover to healthy instances
- Stale instance cleanup

### 5. **Scalability**
- Horizontal scaling with multiple instances
- Load distribution based on workload
- No single point of failure

## Example Flows

### IDE Agent Connection Flow
```
1. IDE Extension starts
   ↓
2. WebSocket connects to MCP server <!-- Source: pkg/models/websocket/binary.go -->
   ↓
3. RegisterInstance("ide-agent", websocket_id) <!-- Source: pkg/models/websocket/binary.go -->
   ↓
4. Creates/Updates: Manifest → Configuration → Registration
   ↓
5. Agent is now available for tasks
```

### Task Assignment Flow
```
1. Task needs IDE agent
   ↓
2. GetAvailableAgents(tenant_id)
   ↓
3. Filter: enabled configs + healthy registrations + available workload
   ↓
4. Select best agent by availability score
   ↓
5. Route task to selected instance
```

### Reconnection Flow
```
1. Network interruption
   ↓
2. WebSocket disconnects <!-- Source: pkg/models/websocket/binary.go -->
   ↓
3. Health status → degraded/unknown
   ↓
4. WebSocket reconnects with same instance_id <!-- Source: pkg/models/websocket/binary.go -->
   ↓
5. Registration updated (not duplicated)
   ↓
6. Health status → healthy
```

## Repository Interface

The `ThreeTierRepository` provides these key methods:

```go
// Manifest operations
CreateManifest(ctx, manifest)
GetManifest(ctx, agentID)
ListManifests(ctx, filter)

// Configuration operations
CreateConfiguration(ctx, config)
GetConfiguration(ctx, tenantID, configID)
ListConfigurations(ctx, tenantID, filter)
UpdateWorkload(ctx, configID, delta)

// Registration operations
RegisterInstance(ctx, registration)
GetActiveRegistrations(ctx, tenantID)
UpdateHealth(ctx, instanceID, status)
DeactivateRegistration(ctx, instanceID)

// Combined operations
GetAvailableAgents(ctx, tenantID)
GetAgentMetrics(ctx, configID, period)
Cleanup(ctx, staleThreshold)
```

## Configuration Example

```yaml
# Manifest (System-wide)
agent_id: "ide-agent"
agent_type: "ide"
capabilities:
  - code_editing
  - file_management
  - terminal_access
  
# Configuration (Tenant-specific)
tenant_id: "acme-corp"
manifest_id: "ide-agent-manifest-id"
configuration:
  allowed_directories: ["/workspace"]
  max_file_size: 10MB
system_prompt: "You are a helpful coding assistant for ACME Corp"
max_workload: 5

# Registration (Instance)
instance_id: "ws_conn_abc123"
health_status: "healthy"
connection_details:
  websocket_id: "ws_conn_abc123" <!-- Source: pkg/models/websocket/binary.go -->
  ip_address: "192.168.1.100"
  user_agent: "vscode/1.84.0"
```

## Health States

### Registration Status
- `active`: Instance is connected and available
- `inactive`: Instance has disconnected or been deactivated

### Health Status
- `healthy`: Operating normally, can accept work
- `degraded`: Operating with issues, use as fallback
- `unknown`: Haven't heard from instance recently
- `disconnected`: Instance has left

## Best Practices

1. **Always use instance_id for registration** - This ensures idempotency
2. **Regular health checks** - Update health every 30-60 seconds
3. **Clean up stale registrations** - Run cleanup periodically
4. **Monitor workload** - Respect max_workload limits
5. **Use availability score** - For intelligent agent selection

## Migration Notes


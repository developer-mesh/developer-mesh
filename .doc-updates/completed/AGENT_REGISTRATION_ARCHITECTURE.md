<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:27:38
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Agent Registration Architecture

## Overview

The DevOps MCP platform uses a three-tier agent architecture that properly separates concerns and enables idempotent registration, multiple instances, and graceful reconnections.

## Architecture Components

### 1. Agent Manifests (`mcp.agent_manifests`)
**Purpose**: Defines what types of agents exist in the system

- **agent_id**: Deterministic identifier (e.g., `ide-agent`, `slack-bot`, `k8s-monitor`)
- **agent_type**: Category of agent
- **capabilities**: What the agent can do
- **version**: Agent version

Example agents:
- IDE integration agents
- Slack bots
- Kubernetes monitors
- CI/CD pipeline agents

### 2. Agent Configurations (`mcp.agent_configurations`)
**Purpose**: Tenant-specific settings for each agent type

- **tenant_id + manifest_id**: Unique configuration per tenant per agent type
- **name**: Display name for this tenant's instance
- **model_id**: Which AI model to use
- **temperature**, **max_tokens**: AI parameters
- **system_prompt**: Custom prompts
- **max_workload**: Concurrency limits

### 3. Agent Registrations (`mcp.agent_registrations`)
**Purpose**: Tracks which instances are currently connected

- **instance_id**: Unique per connection (WebSocket ID, K8s pod ID) <!-- Source: pkg/models/websocket/binary.go -->
- **health_status**: Current health state
- **last_health_check**: Last heartbeat
- **connection_details**: How the agent is connected

## Key Features

### Idempotent Registration

The `register_agent_instance()` function handles all registration scenarios:

```sql
SELECT * FROM mcp.register_agent_instance(
    tenant_id,
    agent_id,      -- e.g., 'ide-agent'
    instance_id,   -- e.g., WebSocket connection ID <!-- Source: pkg/models/websocket/binary.go -->
    name,
    connection_details,
    runtime_config
);
```

This function:
1. Creates manifest if it doesn't exist
2. Creates configuration if it doesn't exist
3. Creates or updates registration based on instance_id
4. Returns registration details and whether it's new or reconnection

### Multiple Instance Support

Unlike the old architecture, multiple instances of the same agent can run simultaneously:

```
┌─────────────────────┐
│   IDE Agent Type    │
│   (manifest)        │
└──────────┬──────────┘
           │
    ┌──────┴──────┐
    │             │
┌───▼───┐    ┌───▼───┐
│Tenant1│    │Tenant2│
│Config │    │Config │
└───┬───┘    └───┬───┘
    │            │
┌───┼────────────┼───┐
│   │            │   │
│ Inst1  Inst2  Inst3│
│                    │
│  (3 developers)    │
└────────────────────┘
```

### Reconnection Handling

When an agent reconnects with the same instance_id:
- Registration is updated (not duplicated)
- Health status set to 'healthy'
- Last health check updated
- No duplicate key errors!

### Kubernetes Pod Cycling

When a pod restarts:
- New pod gets new instance_id
- New registration created
- Old registration marked stale after timeout
- No manual cleanup required

## Migration from Old Architecture

### Old Problems
- `UNIQUE(tenant_id, name)` constraint prevented reconnections
- Mixed concerns (identity, config, runtime state) in one table
- Couldn't run multiple instances
- Manual cleanup required for disconnections

### New Benefits
- ✅ Graceful reconnections
- ✅ Multiple instances per agent
- ✅ Kubernetes-friendly
- ✅ No duplicate key errors
- ✅ Automatic stale instance cleanup
- ✅ Clear separation of concerns

## Usage Examples

### Registering an Agent (Go)

```go
result, err := registry.RegisterAgentInstance(
    ctx,
    tenantID,
    "ide-agent",           // Agent type
    conn.ID,               // Instance ID (connection ID)
    connectionDetails,
    runtimeConfig,
)

if result.IsNew {
    log.Info("New agent instance registered")
} else {
    log.Info("Agent reconnected")
}
```

### Registering via WebSocket <!-- Source: pkg/models/websocket/binary.go -->

```json
{
    "type": 0,
    "id": "msg-123",
    "method": "agent.register",
    "params": {
        "agent_id": "ide-agent",
        "name": "VS Code Agent",
        "capabilities": ["code_completion", "debugging"],
        "auth": {
            "tenant_id": "00000000-0000-0000-0000-000000000001",
            "api_key": "dev-key"
        }
    }
}
```

### Getting Active Instances

```sql
-- Get all active instances of an agent for a tenant
SELECT 
    ar.instance_id,
    ar.health_status,
    ar.last_health_check,
    ac.name as config_name
FROM mcp.agent_registrations ar
JOIN mcp.agent_configurations ac ON ac.manifest_id = ar.manifest_id
WHERE ar.tenant_id = $1
  AND ar.registration_status = 'active'
  AND ac.manifest_id = (
      SELECT id FROM mcp.agent_manifests WHERE agent_id = $2
  );
```

## Backward Compatibility

A view named `mcp.agents` provides backward compatibility with existing code:
- Maps configurations to old agent structure
- Derives status from active registrations
- Supports INSERT/UPDATE/DELETE via triggers
- Allows gradual migration

## Best Practices

1. **Use deterministic agent_id**: Don't use UUIDs for agent_id, use meaningful identifiers
2. **Instance ID = Connection ID**: For WebSocket agents, use the connection ID <!-- Source: pkg/models/websocket/binary.go -->
3. **Regular heartbeats**: Send heartbeats to maintain health status
4. **Clean disconnection**: Call deregister on graceful shutdown
5. **Let stale instances timeout**: Don't manually clean up crashed instances

## Monitoring

Key metrics to track:
- Number of active registrations per agent type
- Registration success/failure rate
- Average reconnection frequency
- Stale instance count
- Health check latency

## Troubleshooting

### "Duplicate key" errors
- **Old system**: Required manual cleanup
- **New system**: Should never happen (idempotent registration)

### Multiple registrations for same agent
- **Expected**: Multiple developers or pods can connect
- **Check**: instance_id should be unique per connection

### Agent not appearing as active
- **Check**: Health status and last_health_check
- **Check**: Registration status is 'active'
- **Check**: Configuration is enabled

## Future Enhancements

1. **Load balancing**: Route requests to healthiest instance
2. **Affinity**: Prefer same instance for related requests
3. **Auto-scaling**: Spawn instances based on load
4. **Circuit breaking**: Disable unhealthy instances

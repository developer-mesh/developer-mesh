<!-- SOURCE VERIFICATION
Last Verified: 2025-10-17
Verification Method: Manual code and database schema review
Verified Against: migrations/sql/000006_agent_manifests.up.sql, pkg/models/agent_manifest.go
Status: Schema corrected to match actual implementation
-->

# Universal Agent Registration Architecture

## Overview

The Universal Agent Registration System enables any type of agent (IDE, Slack, monitoring, CI/CD, custom) to register with the Developer Mesh platform and collaborate seamlessly. This architecture provides flexible agent configuration, strict tenant isolation, and sophisticated rate limiting and circuit breaking capabilities.

## Core Components

### 1. Agent Manifest System

The agent manifest system provides flexible configuration for any agent type:

```sql
-- Core manifest table
CREATE TABLE mcp.agent_manifests (
    id UUID PRIMARY KEY,
    organization_id UUID REFERENCES organizations,
    agent_id VARCHAR(255) UNIQUE,
    agent_type VARCHAR(100),  -- ide, slack, monitoring, cicd, custom
    capabilities JSONB,       -- ["code_completion", "debugging", ...]
    requirements JSONB,       -- {"min_memory": "2GB", "apis": ["github"]}
    connection_config JSONB,  -- {"heartbeat": "30s", "timeout": "5m"}
    auth_config JSONB,        -- {"type": "oauth", "provider": "github"}
    metadata JSONB
);
```

### 2. Agent Registration Flow

```mermaid
graph TD
    A[Agent Connect] --> B[WebSocket Handshake] <!-- Source: pkg/models/websocket/binary.go -->
    B --> C[Authentication]
    C --> D[Organization Binding]
    D --> E[Manifest Creation/Update]
    E --> F[Capability Registration]
    F --> G[Health Monitoring]
    G --> H[Discovery Service Update]
```

### 3. Organization Isolation

#### Strict Tenant Separation

Every agent is automatically bound to its organization with no possibility of override:

```go
type Organization struct {
    ID               uuid.UUID
    Name             string
    StrictlyIsolated bool  // When true, NO cross-org communication
}

// Enforcement at discovery
func (s *ExtendedServer) discoverAgents(ctx context.Context, orgID uuid.UUID) []Agent {
    // ALWAYS filter by organization
    query := `SELECT * FROM agent_manifests WHERE organization_id = $1`
    // No way to bypass this filter
}
```

#### Cross-Organization Blocking

```go
func (s *ExtendedServer) canAccessAgent(ctx context.Context, source, target *Agent) bool {
    // Check organization match
    if source.OrganizationID != target.OrganizationID {
        // Check if strict isolation
        if source.Organization.StrictlyIsolated {
            s.logger.Warn("Cross-org access blocked", map[string]interface{}{
                "source_org": source.OrganizationID,
                "target_org": target.OrganizationID,
            })
            return false  // BLOCKED
        }
        // Check allow-list for partnerships
        if !s.isPartnerOrg(source.OrganizationID, target.OrganizationID) {
            return false  // BLOCKED
        }
    }
    return true
}
```

## Rate Limiting Architecture

### Multi-Level Rate Limiting

The system implements rate limiting at multiple levels to prevent abuse:

```go
type AgentRateLimiter struct {
    agentLimits      sync.Map  // Per-agent limits
    tenantLimits     sync.Map  // Per-tenant limits  
    capabilityLimits sync.Map  // Per-capability limits
}

// Configuration
type RateLimitConfig struct {
    DefaultAgentRPS      int     // 10 requests/second
    DefaultTenantRPS     int     // 100 requests/second
    DefaultCapabilityRPS int     // 50 requests/second
    BurstMultiplier      float64 // 1.5x for burst capacity
}
```

### Rate Limit Enforcement

```go
func (rl *AgentRateLimiter) CheckAgentLimit(agentID string) error {
    limit := rl.getAgentLimit(agentID)
    
    // Sliding window rate limiting
    if limit.CurrentRPS() > limit.MaxRPS {
        // Try burst capacity
        if limit.BurstAvailable() {
            limit.UseBurst()
            return nil
        }
        return ErrRateLimitExceeded
    }
    
    limit.RecordRequest()
    return nil
}
```

## Circuit Breaker System

### Failure Protection

Circuit breakers prevent cascading failures:

```go
type AgentCircuitBreaker struct {
    agentBreakers      sync.Map  // Per-agent breakers
    capabilityBreakers sync.Map  // Per-capability breakers
    tenantBreakers     sync.Map  // Per-tenant breakers
}

// Configuration
type CircuitBreakerConfig struct {
    FailureThreshold    int           // 3 failures to open
    ResetTimeout        time.Duration // 20 seconds
    SuccessThreshold    int           // 2 successes to close
    MaxRequestsHalfOpen int           // 3 requests in half-open
}
```

### Circuit Breaker States

```
CLOSED (normal) --> OPEN (blocked) --> HALF_OPEN (testing) --> CLOSED
         |                                      |
         +---------- failures ------------------+
```

### Health Marking

```go
func (cb *CircuitBreaker) markAgentUnhealthy(agentID string) {
    // Update database
    UPDATE agent_registrations 
    SET health_status = 'unhealthy',
        last_health_check = NOW()
    WHERE agent_id = $1;
    
    // Notify monitoring
    metrics.RecordCounter("agent_marked_unhealthy", 1, map[string]string{
        "agent_id": agentID,
    })
    
    // Remove from active pool
    s.agentPool.Remove(agentID)
}
```

## Message Routing

### Capability-Based Routing

Messages are routed based on capabilities, not agent types:

```go
type AgentMessage struct {
    ID               string
    SourceAgentID    string
    SourceAgentType  string  // ide, slack, monitoring
    TargetCapability string  // Route by capability
    TargetAgentID    string  // Or specific agent
    MessageType      string
    Priority         int     // 1-10 scale
    Payload          map[string]interface{}
}

func (broker *MessageBroker) RouteByCapability(capability string, msg *Message) error {
    // Find agents with capability
    agents := broker.findAgentsByCapability(capability)
    
    // Filter by organization
    agents = broker.filterByOrganization(agents, msg.SourceOrgID)
    
    // Apply load balancing
    target := broker.selectLeastLoaded(agents)
    
    // Route message
    return broker.sendToAgent(target, msg)
}
```

### Cross-Agent Communication Examples

#### IDE → Jira
```json
{
    "source_agent": "vscode-1",
    "target_capability": "issue_management",
    "message_type": "issue.create",
    "payload": {
        "title": "Bug in auth module",
        "description": "Login fails"
    }
}
```

#### Monitoring → Slack
```json
{
    "source_agent": "prometheus-1",
    "target_capability": "notifications",
    "message_type": "alert.critical",
    "priority": 10,
    "payload": {
        "metric": "cpu_usage",
        "value": 95.5,
        "host": "prod-api-01"
    }
}
```

## Database Schema

### Complete Schema (Verified from 000006_agent_manifests.up.sql)

```sql
-- Agent Manifests (defines agent types and capabilities)
CREATE TABLE mcp.agent_manifests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID REFERENCES mcp.organizations(id) ON DELETE CASCADE,
    agent_id VARCHAR(255) UNIQUE NOT NULL,
    agent_type VARCHAR(100) NOT NULL, -- ide, slack, monitoring, custom, etc.
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    description TEXT,
    capabilities JSONB DEFAULT '[]',        -- Agent capabilities array
    requirements JSONB DEFAULT '{}',        -- Runtime requirements
    connection_config JSONB DEFAULT '{}',  -- Connection settings
    auth_config JSONB DEFAULT '{}',        -- Authentication config
    metadata JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'inactive',
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    created_by UUID,
    updated_by UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Agent Registrations (active instances of agents)
CREATE TABLE mcp.agent_registrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    manifest_id UUID REFERENCES mcp.agent_manifests(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    instance_id VARCHAR(255) NOT NULL,
    registration_token TEXT,
    registration_status VARCHAR(50) DEFAULT 'pending',
    activation_date TIMESTAMP WITH TIME ZONE,
    expiration_date TIMESTAMP WITH TIME ZONE,
    runtime_config JSONB DEFAULT '{}',
    connection_details JSONB DEFAULT '{}',
    health_status VARCHAR(50) DEFAULT 'unknown',
    health_check_url TEXT,
    last_health_check TIMESTAMP WITH TIME ZONE,
    metrics JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, instance_id)
);

-- Agent Capabilities (normalized for efficient querying)
CREATE TABLE mcp.agent_capabilities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    manifest_id UUID REFERENCES mcp.agent_manifests(id) ON DELETE CASCADE,
    capability_type VARCHAR(100) NOT NULL,
    capability_name VARCHAR(255) NOT NULL,
    capability_config JSONB DEFAULT '{}',
    required BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(manifest_id, capability_type, capability_name)
);

-- Agent Communication Channels
CREATE TABLE mcp.agent_channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    registration_id UUID REFERENCES mcp.agent_registrations(id) ON DELETE CASCADE,
    channel_type VARCHAR(50) NOT NULL, -- websocket, http, grpc, redis, etc.
    channel_config JSONB NOT NULL,
    priority INTEGER DEFAULT 0,
    active BOOLEAN DEFAULT true,
    last_message_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(registration_id, channel_type)
);

-- Performance Indexes
CREATE INDEX idx_agent_manifests_org_id ON mcp.agent_manifests(organization_id);
CREATE INDEX idx_agent_manifests_agent_type ON mcp.agent_manifests(agent_type);
CREATE INDEX idx_agent_manifests_status ON mcp.agent_manifests(status);
CREATE INDEX idx_agent_manifests_capabilities ON mcp.agent_manifests USING gin(capabilities);
CREATE INDEX idx_agent_registrations_manifest_id ON mcp.agent_registrations(manifest_id);
CREATE INDEX idx_agent_registrations_tenant_id ON mcp.agent_registrations(tenant_id);
CREATE INDEX idx_agent_registrations_status ON mcp.agent_registrations(registration_status);
CREATE INDEX idx_agent_registrations_health ON mcp.agent_registrations(health_status);
CREATE INDEX idx_agent_capabilities_manifest_id ON mcp.agent_capabilities(manifest_id);
CREATE INDEX idx_agent_capabilities_type ON mcp.agent_capabilities(capability_type);
CREATE INDEX idx_agent_channels_registration_id ON mcp.agent_channels(registration_id);
CREATE INDEX idx_agent_channels_type ON mcp.agent_channels(channel_type);
CREATE INDEX idx_agent_channels_active ON mcp.agent_channels(active);
```

## Implementation Components

### Files Created

1. **Migration Files**:
   - `000006_agent_manifests.up.sql` - Database schema
   - `000006_agent_manifests.down.sql` - Rollback script

2. **Models**:
   - `pkg/models/agent_manifest.go` - Data structures

3. **Repository**:
   - `pkg/repository/agent_manifest_repository.go` - CRUD operations

4. **Extensions**:
   - `agent_registry_enhanced.go` - Enhanced registry with universal support
   - `agent_rate_limiter.go` - Multi-level rate limiting
   - `agent_circuit_breaker.go` - Failure protection

5. **Message Routing**:
   - `agent_message_broker.go` - Redis Streams broker <!-- Source: pkg/redis/streams_client.go -->
   - `agent_message_handlers.go` - Type-specific handlers

6. **WebSocket Integration**: <!-- Source: pkg/models/websocket/binary.go -->
   - `universal_agent_handlers_extended.go` - WebSocket handlers <!-- Source: pkg/models/websocket/binary.go -->
   - `server_extensions.go` - Extended server structure
   - `handlers_extended.go` - Handler registration

## Security Considerations

### Authentication
- All agents must authenticate via API key or JWT
- Organization binding happens at authentication time
- Cannot be overridden by agent

### Authorization
- Capability-based access control
- Organization-level isolation
- Admin override for special cases only

### Audit Logging
- All cross-organization attempts logged
- Message routing decisions recorded
- Rate limit violations tracked

## Monitoring and Metrics

### Key Metrics

```prometheus
# Agent metrics
mcp_agent_manifests_total{organization, type}
mcp_agent_registrations_active{organization, type}
mcp_agent_health_status{agent_id, status}
mcp_cross_agent_messages_total{source_type, target_type}

# Isolation metrics
mcp_cross_org_attempts_blocked{source_org, target_org}
mcp_tenant_isolation_mode{organization, mode}

# Rate limiting
mcp_rate_limit_exceeded{type, agent_id}
mcp_rate_limit_current_rps{type}

# Circuit breakers
mcp_circuit_breaker_state{agent_id, state}
mcp_circuit_breaker_trips{agent_id}
```

### Dashboards

Create Grafana dashboards for:
1. Agent registration and health
2. Cross-agent message flow
3. Rate limiting and circuit breaker status
4. Organization isolation violations

## Best Practices

### Agent Development

1. **Always specify agent type** in registration
2. **List all capabilities** the agent supports
3. **Define requirements** for proper routing
4. **Implement health checks** for monitoring
5. **Handle rate limits** gracefully with backoff
6. **Respect circuit breaker** signals

### Operations

1. **Monitor cross-org attempts** for security
2. **Tune rate limits** based on usage patterns
3. **Review circuit breaker trips** for issues
4. **Track agent health** for reliability
5. **Audit message routing** for compliance

## Troubleshooting

### Common Issues

1. **Agent Not Discovered**:
   - Check organization match
   - Verify capabilities registered
   - Ensure not in strict isolation

2. **Rate Limit Exceeded**:
   - Review request patterns
   - Consider increasing limits
   - Implement client-side throttling

3. **Circuit Breaker Open**:
   - Check agent health
   - Review failure logs
   - Wait for reset timeout

4. **Cross-Org Blocked**:
   - Verify organization settings
   - Check strict isolation mode
   - Review partnership configuration

## Future Enhancements

1. **Agent Hierarchies**: Parent-child agent relationships
2. **Capability Evolution**: Learning and adaptation
3. **Dynamic Routing**: ML-based optimal routing
4. **Federation**: Cross-platform agent communication

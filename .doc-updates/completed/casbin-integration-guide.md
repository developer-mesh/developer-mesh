<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:34:21
Verification Script: update-docs-parallel.sh
Batch: ac
-->

# Casbin RBAC Integration Guide

> **Status**: Future Implementation Guide
> **Complexity**: Medium
> **Estimated Effort**: 2-3 weeks
> **Dependencies**: Casbin v2, Database migrations

## Overview

This guide provides a comprehensive roadmap for integrating Casbin RBAC (Role-Based Access Control) into the Developer Mesh platform. Casbin is a powerful authorization library that supports various access control models including ACL, RBAC, ABAC, and more.

## Why Casbin?

### Current Limitations
```go
// Current simple implementation
if user.Role == "admin" {
    return true
}
```

### With Casbin
```go
// Complex policies with conditions
enforcer.AddPolicy("alice", "data1", "read")
enforcer.AddRoleForUser("alice", "admin")
enforcer.Enforce("alice", "data1", "read") // true
```

## Implementation Plan

### Phase 1: Basic Setup (Week 1)

#### 1.1 Add Dependencies
```bash
go get github.com/casbin/casbin/v2
go get github.com/casbin/gorm-adapter/v3
```

#### 1.2 Create Model Configuration
```ini
# configs/casbin/model.conf
[request_definition]
r = sub, tenant, obj, act

[policy_definition]
p = sub, tenant, obj, act, eft

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.sub, r.tenant) && r.tenant == p.tenant && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)
```

#### 1.3 Create Database Schema
```sql
-- migrations/add_casbin_rules.sql
CREATE TABLE IF NOT EXISTS casbin_rules (
    id SERIAL PRIMARY KEY,
    ptype VARCHAR(100) NOT NULL,
    v0 VARCHAR(100),
    v1 VARCHAR(100),
    v2 VARCHAR(100),
    v3 VARCHAR(100),
    v4 VARCHAR(100),
    v5 VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_casbin_rules ON casbin_rules(ptype, v0, v1, v2);
```

### Phase 2: Core Implementation (Week 1-2)

#### 2.1 Create Casbin Authorizer
```go
// pkg/auth/casbin_authorizer.go
package auth

import (
    "context"
    "fmt"
    
    "github.com/casbin/casbin/v2"
    gormadapter "github.com/casbin/gorm-adapter/v3"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

type CasbinAuthorizer struct {
    enforcer *casbin.Enforcer
    logger   observability.Logger
    tracer   observability.Tracer
}

func NewCasbinAuthorizer(db *gorm.DB, modelPath string) (*CasbinAuthorizer, error) {
    // Create adapter
    adapter, err := gormadapter.NewAdapterByDB(db)
    if err != nil {
        return nil, fmt.Errorf("failed to create adapter: %w", err)
    }
    
    // Create enforcer
    enforcer, err := casbin.NewEnforcer(modelPath, adapter)
    if err != nil {
        return nil, fmt.Errorf("failed to create enforcer: %w", err)
    }
    
    // Enable auto-save
    enforcer.EnableAutoSave(true)
    
    return &CasbinAuthorizer{
        enforcer: enforcer,
        logger:   observability.NewLogger("casbin"),
        tracer:   observability.NewTracer("casbin"),
    }, nil
}

func (c *CasbinAuthorizer) Authorize(ctx context.Context, req *AuthRequest) (bool, error) {
    ctx, span := c.tracer.Start(ctx, "CasbinAuthorizer.Authorize")
    defer span.End()
    
    // Extract tenant from context
    tenant := req.TenantID
    if tenant == "" {
        tenant = "default"
    }
    
    // Check permission
    allowed, err := c.enforcer.Enforce(req.Subject, tenant, req.Resource, req.Action)
    if err != nil {
        c.logger.Error("Authorization check failed", map[string]interface{}{
            "error":    err.Error(),
            "subject":  req.Subject,
            "resource": req.Resource,
            "action":   req.Action,
        })
        return false, err
    }
    
    // Log decision
    c.logger.Info("Authorization decision", map[string]interface{}{
        "allowed":  allowed,
        "subject":  req.Subject,
        "resource": req.Resource,
        "action":   req.Action,
        "tenant":   tenant,
    })
    
    return allowed, nil
}
```

#### 2.2 Policy Management Service
```go
// pkg/auth/policy_service.go
package auth

type PolicyService struct {
    enforcer *casbin.Enforcer
    logger   observability.Logger
}

func NewPolicyService(enforcer *casbin.Enforcer) *PolicyService {
    return &PolicyService{
        enforcer: enforcer,
        logger:   observability.NewLogger("policy"),
    }
}

// Add a policy
func (s *PolicyService) AddPolicy(tenant, subject, resource, action string, effect string) error {
    _, err := s.enforcer.AddPolicy(subject, tenant, resource, action, effect)
    if err != nil {
        s.logger.Error("Failed to add policy", map[string]interface{}{
            "error": err.Error(),
            "policy": map[string]string{
                "subject":  subject,
                "tenant":   tenant,
                "resource": resource,
                "action":   action,
                "effect":   effect,
            },
        })
    }
    return err
}

// Add role to user
func (s *PolicyService) AddRoleForUser(user, role, tenant string) error {
    _, err := s.enforcer.AddRoleForUser(user, role, tenant)
    return err
}

// Get roles for user
func (s *PolicyService) GetRolesForUser(user, tenant string) ([]string, error) {
    return s.enforcer.GetRolesForUser(user, tenant)
}

// Get permissions for user
func (s *PolicyService) GetPermissionsForUser(user, tenant string) ([][]string, error) {
    return s.enforcer.GetPermissionsForUser(user, tenant)
}
```

### Phase 3: Integration Points (Week 2)

#### 3.1 Update Authorization Manager
```go
// pkg/auth/manager.go updates
func NewManager(config *Config) (*Manager, error) {
    // ... existing code ...
    
    // Initialize Casbin if enabled
    if config.CasbinEnabled {
        casbinAuth, err := NewCasbinAuthorizer(
            config.Database,
            config.CasbinModelPath,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to create Casbin authorizer: %w", err)
        }
        m.authorizer = casbinAuth
    } else {
        // Fall back to existing authorizer
        m.authorizer = NewProductionAuthorizer(config)
    }
    
    return m, nil
}
```

#### 3.2 Add Policy API Endpoints
```go
// apps/rest-api/internal/api/policy_handlers.go
package api

type PolicyHandler struct {
    policyService *auth.PolicyService
}

// CreatePolicy godoc
// @Summary Create a new policy
// @Tags policies
// @Accept json
// @Produce json
// @Param policy body PolicyRequest true "Policy details"
// @Success 201 {object} PolicyResponse
// @Router /policies [post]
func (h *PolicyHandler) CreatePolicy(c *gin.Context) {
    var req PolicyRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Extract tenant from context
    tenant := c.GetString("tenant_id")
    
    err := h.policyService.AddPolicy(
        tenant,
        req.Subject,
        req.Resource,
        req.Action,
        req.Effect,
    )
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{
        "message": "Policy created successfully",
        "policy": req,
    })
}

// AssignRole assigns a role to a user
func (h *PolicyHandler) AssignRole(c *gin.Context) {
    var req RoleAssignmentRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    tenant := c.GetString("tenant_id")
    
    err := h.policyService.AddRoleForUser(req.User, req.Role, tenant)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "Role assigned successfully",
        "user": req.User,
        "role": req.Role,
    })
}
```

## Default Policies

### 3.3 Initial Policy Setup
```go
// pkg/auth/default_policies.go
func LoadDefaultPolicies(enforcer *casbin.Enforcer) error {
    // Default roles
    roles := [][]string{
        {"admin", "default", "*", "*", "allow"},
        {"user", "default", "contexts", "read", "allow"},
        {"user", "default", "contexts", "create", "allow"},
        {"user", "default", "contexts", "update", "allow"},
        {"viewer", "default", "*", "read", "allow"},
    }
    
    for _, role := range roles {
        _, err := enforcer.AddPolicy(role...)
        if err != nil {
            return err
        }
    }
    
    // Role inheritance
    enforcer.AddRoleForUser("admin", "user", "default")
    enforcer.AddRoleForUser("user", "viewer", "default")
    
    return nil
}
```

## Advanced Features

### 4.1 Attribute-Based Access Control (ABAC)
```ini
# Enhanced model for ABAC
[request_definition]
r = sub, tenant, obj, act, attrs

[policy_definition]
p = sub, tenant, obj, act, eft, condition

[matchers]
m = g(r.sub, p.sub, r.tenant) && r.tenant == p.tenant && \
    keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act) && \
    eval(p.condition)
```

### 4.2 Dynamic Policy Loading
```go
func (c *CasbinAuthorizer) ReloadPolicies() error {
    return c.enforcer.LoadPolicy()
}

// Watch for policy changes
func (c *CasbinAuthorizer) StartPolicyWatcher() {
    go func() {
        for {
            time.Sleep(30 * time.Second)
            if err := c.ReloadPolicies(); err != nil {
                c.logger.Error("Failed to reload policies", map[string]interface{}{
                    "error": err.Error(),
                })
            }
        }
    }()
}
```

### 4.3 Resource Hierarchy
```go
// Support for hierarchical resources
// /projects/123/contexts/456
func init() {
    casbin.AddFunction("resourceMatch", func(args ...interface{}) (interface{}, error) {
        key1 := args[0].(string)
        key2 := args[1].(string)
        
        // Check if key1 matches key2 or is a parent
        return strings.HasPrefix(key2, key1), nil
    })
}
```

## Testing Strategy

### 5.1 Unit Tests
```go
func TestCasbinAuthorizer(t *testing.T) {
    // Create test enforcer
    e, _ := casbin.NewEnforcer("../../configs/casbin/model.conf")
    
    // Add test policies
    e.AddPolicy("alice", "tenant1", "data1", "read", "allow")
    e.AddPolicy("bob", "tenant1", "data1", "write", "allow")
    
    // Test authorization
    tests := []struct {
        name     string
        subject  string
        tenant   string
        resource string
        action   string
        expected bool
    }{
        {
            name:     "alice can read",
            subject:  "alice",
            tenant:   "tenant1",
            resource: "data1",
            action:   "read",
            expected: true,
        },
        {
            name:     "alice cannot write",
            subject:  "alice",
            tenant:   "tenant1",
            resource: "data1",
            action:   "write",
            expected: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, _ := e.Enforce(tt.subject, tt.tenant, tt.resource, tt.action)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### 5.2 Integration Tests
```go
func TestPolicyAPI(t *testing.T) {
    // Test policy creation via API
    policy := PolicyRequest{
        Subject:  "test-user",
        Resource: "test-resource",
        Action:   "read",
        Effect:   "allow",
    }
    
    resp := postJSON("/api/v1/policies", policy)
    assert.Equal(t, 201, resp.StatusCode)
    
    // Verify authorization works
    token := generateTestToken("test-user", "tenant1")
    resp = getWithAuth("/api/v1/test-resource", token)
    assert.Equal(t, 200, resp.StatusCode)
}
```

## Migration Guide

### 6.1 Data Migration
```go
// Migrate existing policies to Casbin
func MigrateExistingPolicies(oldAuth Authorizer, newEnforcer *casbin.Enforcer) error {
    // Get all existing policies
    policies := oldAuth.GetAllPolicies()
    
    for _, p := range policies {
        _, err := newEnforcer.AddPolicy(
            p.Subject,
            p.TenantID,
            p.Resource,
            p.Action,
            "allow",
        )
        if err != nil {
            return err
        }
    }
    
    return newEnforcer.SavePolicy()
}
```

### 6.2 Gradual Rollout
```go
// Feature flag for gradual rollout
type HybridAuthorizer struct {
    oldAuth      Authorizer
    casbinAuth   *CasbinAuthorizer
    useCasbin    func(tenant string) bool
}

func (h *HybridAuthorizer) Authorize(ctx context.Context, req *AuthRequest) (bool, error) {
    if h.useCasbin(req.TenantID) {
        return h.casbinAuth.Authorize(ctx, req)
    }
    return h.oldAuth.Authorize(ctx, req)
}
```

## Performance Considerations

### 7.1 Caching
```go
// Enable policy caching
func NewCasbinAuthorizer(db *gorm.DB, modelPath string) (*CasbinAuthorizer, error) {
    // ... existing code ...
    
    // Enable caching
    enforcer.EnableCache(true)
    
    return &CasbinAuthorizer{
        enforcer: enforcer,
    }, nil
}
```

### 7.2 Batch Operations
```go
// Batch policy updates
func (s *PolicyService) AddPolicies(policies [][]string) error {
    return s.enforcer.AddPolicies(policies)
}

// Batch role assignments
func (s *PolicyService) AddRolesForUsers(assignments [][]string) error {
    return s.enforcer.AddGroupingPolicies(assignments)
}
```

## Monitoring and Debugging

### 8.1 Metrics
```go
// Add Casbin metrics
var (
    authorizationLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "casbin_authorization_duration_seconds",
            Help: "Time taken for authorization decisions",
        },
        []string{"tenant", "allowed"},
    )
    
    policyCount = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "casbin_policy_count",
            Help: "Number of policies by type",
        },
        []string{"type"},
    )
)
```

### 8.2 Debug Logging
```go
// Enable debug logging
func (c *CasbinAuthorizer) EnableDebug() {
    c.enforcer.EnableLog(true)
    
    // Custom logger
    c.enforcer.SetLogger(&CasbinLogger{
        logger: c.logger,
    })
}

type CasbinLogger struct {
    logger observability.Logger
}

func (l *CasbinLogger) LogPolicy(policy []string) {
    l.logger.Debug("Policy evaluated", map[string]interface{}{
        "policy": policy,
    })
}
```

## Security Best Practices

1. **Principle of Least Privilege**: Start with deny-all, explicitly allow
2. **Regular Audits**: Review policies periodically
3. **Policy Versioning**: Track policy changes
4. **Separation of Duties**: Different roles for policy management
5. **Multi-tenancy**: Ensure tenant isolation in policies

## Troubleshooting

### Common Issues

1. **Performance degradation**
   - Enable caching
   - Index database columns
   - Batch policy updates

2. **Policy conflicts**
   - Check policy order
   - Use explicit deny policies
   - Review matcher logic

3. **Migration issues**
   - Test with subset of data
   - Run in parallel mode
   - Monitor authorization metrics

## Next Steps

1. Review and approve implementation plan
2. Set up development environment
3. Create feature branch
4. Implement in phases
5. Extensive testing
6. Gradual production rollout
7. Monitor and optimize

## Resources

- [Casbin Documentation](https://casbin.org/)
- [Casbin Model Editor](https://casbin.org/editor/)
- [Casbin Examples](https://github.com/casbin/casbin/tree/master/examples)

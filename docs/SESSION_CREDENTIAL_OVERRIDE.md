# Session-Level Credential Override (Phase 1)

## Overview

The Edge MCP server now supports session-level credential override, enabling clients to pass credentials during MCP session initialization that take precedence over database-stored credentials.

## Implementation

### 1. MCP Initialize Enhancement

Clients can now pass credentials during the `initialize` method:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "clientInfo": {
      "name": "my-client",
      "version": "1.0.0"
    },
    "credentials": {
      "credentials": {
        "github": {
          "type": "bearer",
          "token": "ghp_your_token_here"
        }
      }
    }
  }
}
```

### 2. Session Storage

Credentials passed during initialization are stored in `session.PassthroughAuth` and persist for the duration of the session:

**File**: `apps/edge-mcp/internal/mcp/handler.go:738-745`

```go
// Store credentials in session if provided
if params.Credentials != nil {
    session.PassthroughAuth = params.Credentials
    h.logger.Debug("Stored passthrough credentials in session", map[string]interface{}{
        "session_id": sessionID,
        "num_credentials": len(params.Credentials.Credentials),
    })
}
```

### 3. Credential Precedence

The system automatically enforces credential precedence through the existing passthrough auth system:

**Priority Order**:
1. **Session Credentials** (passed during initialize) - Highest priority
2. **Database Credentials** (tenant_tool_credentials table) - Fallback

**File**: `pkg/tools/adapters/dynamic_tool_adapter.go:358-405`

The `applyAuthenticationWithPassthrough` method handles precedence:

```go
func (a *DynamicToolAdapter) applyAuthenticationWithPassthrough(
    req *http.Request,
    passthroughAuth *models.PassthroughAuthBundle,
    passthroughConfig *models.EnhancedPassthroughConfig,
) error {
    // Uses passthrough auth if available, falls back to database creds
    if usePassthrough && passthroughAuth != nil {
        // Apply passthrough authentication
        if err := passthroughAuthenticator.ApplyPassthroughAuth(...); err != nil {
            // Check if fallback is allowed
            if passthroughConfig != nil && passthroughConfig.FallbackToService {
                return a.applyAuthentication(req)  // Fallback to database
            }
            return err
        }
        return nil
    }

    // Use stored database credentials
    return a.applyAuthentication(req)
}
```

## Usage Example

### Claude Code Integration

When using Claude Code with `.claude.json`:

```json
{
  "mcpServers": {
    "devmesh": {
      "url": "ws://localhost:8080/ws",
      "headers": {
        "Authorization": "Bearer dev-admin-key-1234567890"
      },
      "credentials": {
        "github": {
          "type": "bearer",
          "token": "ghp_YOUR_UPDATED_TOKEN"
        }
      }
    }
  }
}
```

**Note**: Clients must implement credential passing during their MCP initialize call. The credentials in the client config are passed to the Edge MCP server during initialization, stored in the session, and used for all subsequent tool executions.

## Testing

Test coverage includes:

1. **Initialize with credentials**: `TestHandleInitialize_CredentialStorage` (handler_test.go:174-229)
   - Verifies credentials are properly stored in session.PassthroughAuth
   - Validates credential structure and content

2. **Existing passthrough auth tests**: `passthrough_auth_test.go`
   - Verifies credential precedence logic
   - Tests fallback behavior

## Security Considerations

- Session credentials are stored in memory only (not persisted to database)
- Credentials are scoped to a single MCP session
- Credentials are cleared when session terminates
- No additional encryption needed (credentials are in-memory only)

## Next Steps (Future Phases)

**Phase 2**: REST API endpoint for credential synchronization
- POST `/api/v1/auth/credentials/sync` to update database credentials
- Enables programmatic credential updates from clients

**Phase 3**: Client integration documentation
- Guide for Claude Code extension developers
- Guide for other MCP client implementations

## Related Files

- `apps/edge-mcp/internal/mcp/handler.go` - Session initialization and storage
- `apps/edge-mcp/internal/mcp/handler_test.go` - Test coverage
- `pkg/models/passthrough_auth.go` - Credential structures
- `pkg/tools/adapters/dynamic_tool_adapter.go` - Credential precedence logic

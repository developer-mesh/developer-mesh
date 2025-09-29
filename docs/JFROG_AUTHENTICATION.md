# JFrog Artifactory & Xray Authentication Guide

This document provides comprehensive authentication configuration for the Artifactory and Xray providers in Developer Mesh, including setup instructions, troubleshooting, and best practices.

## Overview

Both Artifactory and Xray providers support JFrog Platform's unified authentication system. The providers automatically detect the appropriate authentication method based on the credential format and apply the correct headers.

## Supported Authentication Methods

### 1. API Key Authentication (Legacy)

**Header:** `X-JFrog-Art-Api`
**Format:** Alphanumeric string (32-64 characters)
**Usage:** Legacy authentication method, still widely supported

```json
{
  "provider": "artifactory",
  "config": {
    "baseURL": "https://mycompany.jfrog.io/artifactory",
    "authType": "api_key",
    "credentials": {
      "apiKey": "AKCp5bBBN8FYuCdL3fjUEy2zJ8RnHzG5X2FqKwPv9mNcRdE7fUwN5sKrT4aB8mPqX3cYwZv"
    }
  }
}
```

**Detection Pattern:** Starts with `AKC` or matches `^[a-zA-Z0-9]{32,64}$`

### 2. Access Token Authentication (Recommended)

**Header:** `Authorization: Bearer`
**Format:** JWT token
**Usage:** Modern authentication method with scoped permissions

```json
{
  "provider": "artifactory",
  "config": {
    "baseURL": "https://mycompany.jfrog.io/artifactory",
    "authType": "bearer_token",
    "credentials": {
      "accessToken": "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsImtpZCI6IlJ6QTBOekJGTlRrNE16YzNOMEUzUVRJME9UUXhSVE15UTBSQ09ERTVOVEV4UkRJM1JUazNPQSJ9..."
    }
  }
}
```

**Detection Pattern:** JWT format (`eyJ*` prefix) or long alphanumeric token

### 3. Reference Token Authentication

**Header:** `Authorization: Bearer`
**Format:** Reference token that resolves to full access token
**Usage:** Secure token reference for distributed systems

```json
{
  "provider": "artifactory",
  "config": {
    "baseURL": "https://mycompany.jfrog.io/artifactory",
    "authType": "bearer_token",
    "credentials": {
      "accessToken": "cmVmOmFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6MTIzNDU2Nzg5MA=="
    }
  }
}
```

**Detection Pattern:** Base64 encoded reference token

## Automatic Authentication Detection

The providers automatically detect the authentication method using these rules:

### Detection Logic

```go
func (p *BaseProvider) detectJFrogAuthType(credential string) string {
    // 1. Check for JWT tokens (access tokens)
    if strings.HasPrefix(credential, "eyJ") || isJWTToken(credential) {
        return "bearer" // Use Authorization: Bearer header
    }

    // 2. Check for API keys starting with AKC
    if strings.HasPrefix(credential, "AKC") {
        return "api_key" // Use X-JFrog-Art-Api header
    }

    // 3. Check for traditional API key pattern
    if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{32,64}$`, credential); matched {
        return "api_key" // Use X-JFrog-Art-Api header
    }

    // 4. Check for reference tokens
    if isBase64Encoded(credential) && len(credential) > 20 {
        return "bearer" // Use Authorization: Bearer header
    }

    // 5. Default to bearer for unknown formats
    return "bearer"
}
```

### Applied Headers

| Credential Type | Header Applied | Example |
|----------------|----------------|---------|
| **API Key** | `X-JFrog-Art-Api: {credential}` | Traditional API keys |
| **Access Token** | `Authorization: Bearer {credential}` | JWT tokens |
| **Reference Token** | `Authorization: Bearer {credential}` | Reference tokens |

## Configuration Examples

### Environment Variables

```bash
# API Key authentication
export ARTIFACTORY_API_KEY="AKCp5bBBN8FYuCdL3fjUEy2zJ8RnHzG5X2FqKwPv9mNcRdE7fUwN5sKrT4aB8mPqX3cYwZv"
export XRAY_API_KEY="AKCp5bBBN8FYuCdL3fjUEy2zJ8RnHzG5X2FqKwPv9mNcRdE7fUwN5sKrT4aB8mPqX3cYwZv"

# Access token authentication
export ARTIFACTORY_ACCESS_TOKEN="eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiI..."
export XRAY_ACCESS_TOKEN="eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiI..."

# Base URLs
export ARTIFACTORY_BASE_URL="https://mycompany.jfrog.io/artifactory"
export XRAY_BASE_URL="https://mycompany.jfrog.io/xray"
```

### Provider Configuration

```json
{
  "providers": {
    "artifactory": {
      "baseURL": "https://mycompany.jfrog.io/artifactory",
      "authType": "auto", // Auto-detect from credential format
      "credentials": {
        "apiKey": "${ARTIFACTORY_API_KEY}" // Can be API key or access token
      },
      "healthEndpoint": "/api/system/ping",
      "requestTimeout": 30000
    },
    "xray": {
      "baseURL": "https://mycompany.jfrog.io/xray",
      "authType": "auto", // Auto-detect from credential format
      "credentials": {
        "apiKey": "${XRAY_API_KEY}" // Same credential as Artifactory
      },
      "healthEndpoint": "/api/v1/system/ping",
      "requestTimeout": 60000
    }
  }
}
```

### Context-Based Authentication

```json
{
  "context": {
    "jfrog_credentials": {
      "type": "access_token",
      "token": "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiI...",
      "scope": "applied-permissions/user",
      "expires_at": "2025-02-28T10:00:00Z"
    }
  }
}
```

## Unified Platform Authentication

### Single Sign-On (SSO)

JFrog Platform supports unified authentication across all services:

```json
{
  "platform_config": {
    "baseURL": "https://mycompany.jfrog.io",
    "credentials": {
      "accessToken": "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiI..."
    },
    "services": {
      "artifactory": {
        "endpoint": "/artifactory",
        "enabled": true
      },
      "xray": {
        "endpoint": "/xray",
        "enabled": true
      },
      "pipelines": {
        "endpoint": "/pipelines",
        "enabled": false
      }
    }
  }
}
```

### Token Scopes

Access tokens support different scopes for granular permissions:

| Scope | Description | Usage |
|-------|-------------|-------|
| `applied-permissions/user` | User's applied permissions | Standard user operations |
| `applied-permissions/groups:{group}` | Group-specific permissions | Group-scoped operations |
| `system:admin` | Full administrative access | Administrative operations |
| `api:*` | Full API access | Programmatic access |

## Authentication Validation

### Health Check with Authentication

```json
{
  "workflow": "auth_validation",
  "steps": [
    {
      "name": "Test Artifactory authentication",
      "provider": "artifactory",
      "action": "system/ping",
      "expected": "OK"
    },
    {
      "name": "Verify user identity",
      "provider": "artifactory",
      "action": "internal/current-user"
    },
    {
      "name": "Test Xray authentication",
      "provider": "xray",
      "action": "system/ping",
      "expected": "OK"
    },
    {
      "name": "Check available features",
      "provider": "artifactory",
      "action": "internal/available-features"
    }
  ]
}
```

### Permission Discovery

```json
{
  "workflow": "permission_discovery",
  "steps": [
    {
      "name": "Discover Artifactory permissions",
      "provider": "artifactory",
      "action": "security/permissions/list"
    },
    {
      "name": "List accessible repositories",
      "provider": "artifactory",
      "action": "repos/list"
    },
    {
      "name": "Check Xray watches",
      "provider": "xray",
      "action": "watches/list"
    },
    {
      "name": "List Xray policies",
      "provider": "xray",
      "action": "policies/list"
    }
  ]
}
```

## Troubleshooting Authentication

### Common Authentication Errors

#### 1. `401 Unauthorized`

**Possible Causes:**
- Invalid or expired credentials
- Incorrect authentication method
- Malformed credentials

**Resolution Steps:**
```json
{
  "troubleshooting": "401_unauthorized",
  "steps": [
    {
      "name": "Validate credential format",
      "action": "check_credential_pattern",
      "patterns": {
        "api_key": "^AKC[a-zA-Z0-9]{50,}$",
        "jwt_token": "^eyJ[a-zA-Z0-9_-]+\\.[a-zA-Z0-9_-]+\\.[a-zA-Z0-9_-]+$",
        "reference_token": "^[a-zA-Z0-9+/]+=*$"
      }
    },
    {
      "name": "Test basic connectivity",
      "provider": "artifactory",
      "action": "system/ping"
    },
    {
      "name": "Verify credential expiry",
      "action": "decode_jwt_expiry"
    }
  ]
}
```

#### 2. `403 Forbidden`

**Possible Causes:**
- Insufficient permissions
- Incorrect scope
- Operation not allowed for user type

**Resolution Steps:**
```json
{
  "troubleshooting": "403_forbidden",
  "steps": [
    {
      "name": "Check user permissions",
      "provider": "artifactory",
      "action": "internal/current-user"
    },
    {
      "name": "List accessible operations",
      "provider": "artifactory",
      "action": "internal/available-features"
    },
    {
      "name": "Verify repository access",
      "provider": "artifactory",
      "action": "repos/list"
    }
  ]
}
```

#### 3. `Authentication method not supported`

**Possible Causes:**
- Using wrong header format
- Credential type mismatch
- Server configuration issue

**Resolution Steps:**
```json
{
  "troubleshooting": "auth_method_not_supported",
  "steps": [
    {
      "name": "Force API key authentication",
      "config": {
        "authType": "api_key",
        "forceAuthHeader": "X-JFrog-Art-Api"
      }
    },
    {
      "name": "Force bearer token authentication",
      "config": {
        "authType": "bearer_token",
        "forceAuthHeader": "Authorization"
      }
    }
  ]
}
```

### Debug Authentication Headers

Enable debug logging to see applied headers:

```json
{
  "logging": {
    "level": "debug",
    "components": ["authentication", "http_client"]
  },
  "debug": {
    "logHeaders": true,
    "maskCredentials": true
  }
}
```

**Debug Output Example:**
```
DEBUG [auth] Detected credential type: jwt_token
DEBUG [auth] Applying header: Authorization: Bearer eyJ***[masked]
DEBUG [http] Request: GET /api/system/ping
DEBUG [http] Headers: {Authorization: Bearer [MASKED], User-Agent: DevMesh/1.0.0}
```

## Security Best Practices

### 1. Credential Management

- **Never hardcode credentials** in configuration files
- **Use environment variables** or secure secret management
- **Rotate tokens regularly** (recommended: every 90 days)
- **Use scoped tokens** with minimal required permissions
- **Monitor token usage** and audit access logs

### 2. Token Security

```json
{
  "security_config": {
    "token_rotation": {
      "enabled": true,
      "interval": "90d",
      "notification": "security@mycompany.com"
    },
    "scope_validation": {
      "enforce_minimum_scope": true,
      "allowed_scopes": ["applied-permissions/user", "api:read"]
    },
    "audit_logging": {
      "enabled": true,
      "log_level": "info",
      "include_headers": false
    }
  }
}
```

### 3. Network Security

- **Use HTTPS only** for all JFrog communications
- **Validate SSL certificates** in production environments
- **Configure proper timeouts** to prevent hanging connections
- **Use connection pooling** for better performance

### 4. Error Handling

```json
{
  "error_handling": {
    "authentication": {
      "retry_attempts": 3,
      "retry_delay": "1s",
      "backoff_multiplier": 2,
      "max_retry_delay": "30s"
    },
    "token_refresh": {
      "auto_refresh": true,
      "refresh_threshold": "300s",
      "fallback_credentials": "api_key"
    }
  }
}
```

## Integration Examples

### CI/CD Integration

```yaml
# GitHub Actions example
env:
  JFROG_ACCESS_TOKEN: ${{ secrets.JFROG_ACCESS_TOKEN }}
  JFROG_BASE_URL: "https://mycompany.jfrog.io"

steps:
  - name: Authenticate with JFrog
    run: |
      echo "Configuring JFrog authentication"
      export ARTIFACTORY_BASE_URL="${JFROG_BASE_URL}/artifactory"
      export XRAY_BASE_URL="${JFROG_BASE_URL}/xray"

  - name: Validate authentication
    run: |
      devmesh exec artifactory system/ping
      devmesh exec xray system/ping
```

### Kubernetes Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: jfrog-credentials
type: Opaque
data:
  access-token: <base64-encoded-access-token>
  api-key: <base64-encoded-api-key>
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: jfrog-config
data:
  artifactory-url: "https://mycompany.jfrog.io/artifactory"
  xray-url: "https://mycompany.jfrog.io/xray"
```

### Terraform Provider

```hcl
provider "artifactory" {
  url          = "https://mycompany.jfrog.io/artifactory"
  access_token = var.jfrog_access_token
}

provider "xray" {
  url          = "https://mycompany.jfrog.io/xray"
  access_token = var.jfrog_access_token
}
```

## Migration Guide

### From API Keys to Access Tokens

1. **Generate access token** in JFrog Platform UI
2. **Test token** with limited scope first
3. **Update configurations** gradually
4. **Monitor for issues** during transition
5. **Revoke old API keys** after successful migration

### Migration Script Example

```bash
#!/bin/bash
# Migration script from API key to access token

OLD_API_KEY="${JFROG_API_KEY}"
NEW_ACCESS_TOKEN="${JFROG_ACCESS_TOKEN}"

# Test new token
echo "Testing new access token..."
curl -H "Authorization: Bearer ${NEW_ACCESS_TOKEN}" \
     "${JFROG_BASE_URL}/artifactory/api/system/ping"

if [ $? -eq 0 ]; then
    echo "Access token validated successfully"
    # Update configuration
    sed -i "s/apiKey.*$/accessToken: ${NEW_ACCESS_TOKEN}/" config.yaml
    echo "Configuration updated"
else
    echo "Access token validation failed"
    exit 1
fi
```

## Advanced Configuration

### Multi-Instance Setup

```json
{
  "jfrog_instances": {
    "production": {
      "artifactory": {
        "baseURL": "https://prod.jfrog.mycompany.com/artifactory",
        "credentials": {
          "accessToken": "${PROD_JFROG_TOKEN}"
        }
      },
      "xray": {
        "baseURL": "https://prod.jfrog.mycompany.com/xray",
        "credentials": {
          "accessToken": "${PROD_JFROG_TOKEN}"
        }
      }
    },
    "staging": {
      "artifactory": {
        "baseURL": "https://staging.jfrog.mycompany.com/artifactory",
        "credentials": {
          "accessToken": "${STAGING_JFROG_TOKEN}"
        }
      },
      "xray": {
        "baseURL": "https://staging.jfrog.mycompany.com/xray",
        "credentials": {
          "accessToken": "${STAGING_JFROG_TOKEN}"
        }
      }
    }
  }
}
```

### Load Balancer Configuration

```json
{
  "load_balancer": {
    "enabled": true,
    "instances": [
      "https://jfrog1.mycompany.com",
      "https://jfrog2.mycompany.com",
      "https://jfrog3.mycompany.com"
    ],
    "strategy": "round_robin",
    "health_check": {
      "endpoint": "/api/system/ping",
      "interval": "30s",
      "timeout": "5s"
    }
  }
}
```
<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:38:33
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Encryption Key Configuration

## Overview

The DevOps MCP platform uses AES-256-GCM encryption with per-tenant key derivation to protect sensitive data such as API credentials, tokens, and other secrets. This document describes how to properly configure encryption keys for production deployments.

## Environment Variables

### Single Master Key (Recommended)

- **ENCRYPTION_MASTER_KEY**: Master encryption key for all services
  - Required for production deployments (has default in development)
  - Recommended to be at least 32 characters long
  - Used by both REST API and MCP Server for consistency
  - Encrypts tool credentials, authentication data, and secrets
  - Located in:
    - REST API: `apps/rest-api/internal/api/server.go:445`
    - MCP Server: `apps/mcp-server/internal/api/server.go:368`

### Legacy Support (Deprecated)

- **DEVMESH_ENCRYPTION_KEY**: Legacy REST API encryption key
  - **DEPRECATED** - Migrate to `ENCRYPTION_MASTER_KEY`
  - Falls back to `ENCRYPTION_MASTER_KEY` if not set
  - Will log deprecation warnings when used
  - Maintained for backward compatibility only

## How Encryption Works

### Technical Implementation

1. **Master Key Processing**: The provided key is hashed with SHA-256 to create a 256-bit key
2. **Per-Tenant Key Derivation**: For each encryption operation:
   - Generate a random 32-byte salt
   - Derive tenant-specific key: `PBKDF2(SHA256(masterKey) + tenantID, salt, 10000 iterations)`
   - This ensures each tenant has cryptographically isolated encryption
3. **Encryption**: AES-256-GCM (Galois/Counter Mode) provides:
   - Confidentiality through AES-256 encryption
   - Authentication through GCM mode (prevents tampering)
   - Forward secrecy through unique salt/nonce per operation
4. **Storage Format**: `[32-byte salt][12-byte nonce][ciphertext + 16-byte auth tag]`

### Security Guarantees

- **Tenant Isolation**: Each tenant's data is encrypted with a unique derived key
- **Forward Secrecy**: Each encryption uses a unique salt, so compromise of one value doesn't affect others
- **Authentication**: GCM mode detects any tampering with encrypted data
- **Cross-Tenant Protection**: One tenant cannot decrypt another tenant's data even with the same master key

## Best Practices

1. **Generate Strong Keys**: Use a cryptographically secure random generator
   ```bash
   # Generate a 32-character key (minimum recommended)
   openssl rand -base64 32
   
   # Generate a 64-character key (extra security)
   openssl rand -base64 48 | tr -d '\n' | cut -c1-64
   ```

2. **Store Securely**: 
   - Use a secrets management system (e.g., HashiCorp Vault, AWS Secrets Manager)
   - Never commit keys to version control
   - Rotate keys periodically

3. **Different Keys per Environment**:
   - Use different keys for development, staging, and production
   - Never reuse keys across environments

4. **Key Rotation**:
   - Plan for key rotation from the beginning
   - The system supports key rotation via the `RotateKey` method in `pkg/security/encryption.go`

## Development Mode

If no encryption key is provided, the system will:
1. Generate a random key automatically
2. Log a warning message
3. Continue operating (suitable for development only)

**WARNING**: Auto-generated keys are not persisted and will change on restart, making encrypted data unreadable.

## Production Deployment Example

```yaml
# docker-compose.yml
services:
  rest-api:
    environment:
      - ENCRYPTION_MASTER_KEY=${ENCRYPTION_MASTER_KEY}
  
  mcp-server:
    environment:
      - ENCRYPTION_MASTER_KEY=${ENCRYPTION_MASTER_KEY}
```

```bash
# .env (DO NOT COMMIT)
ENCRYPTION_MASTER_KEY=your-secure-32-character-key-here
```

## Kubernetes Deployment

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: encryption-keys
type: Opaque
data:
  encryption-master-key: <base64-encoded-key>
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rest-api
spec:
  template:
    spec:
      containers:
      - name: rest-api
        env:
        - name: ENCRYPTION_MASTER_KEY
          valueFrom:
            secretKeyRef:
              name: encryption-keys
              key: encryption-master-key
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-server
spec:
  template:
    spec:
      containers:
      - name: mcp-server
        env:
        - name: ENCRYPTION_MASTER_KEY
          valueFrom:
            secretKeyRef:
              name: encryption-keys
              key: encryption-master-key
```

## Troubleshooting

### Missing Encryption Key Warning

If you see warnings like:
```
ENCRYPTION_MASTER_KEY not set - using randomly generated key. This is not suitable for production!
```

This means the encryption key environment variable is not set. Set the `ENCRYPTION_MASTER_KEY` environment variable before starting the service.

### Migration from Multiple Keys

If you're migrating from the old multi-key setup:
1. Choose one of your existing keys to be the master key
2. Set `ENCRYPTION_MASTER_KEY` to that value
3. Remove `DEVMESH_ENCRYPTION_KEY` and `ENCRYPTION_KEY` from your configuration
4. The system will automatically use the master key for all services

### Encrypted Data Unreadable

If you cannot decrypt previously encrypted data:
- Ensure you're using the same encryption key that was used to encrypt the data
- Check that the key hasn't been accidentally modified or truncated

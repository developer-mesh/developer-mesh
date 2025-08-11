# Encryption Key Configuration

## Overview

The DevOps MCP platform uses encryption to protect sensitive data such as API credentials, tokens, and other secrets. This document describes how to properly configure encryption keys for production deployments.

## Environment Variables

### REST API Server

- **DEVMESH_ENCRYPTION_KEY**: Master encryption key for the REST API server
  - Required for production deployments
  - Must be at least 32 characters long
  - Used to encrypt tool credentials and other sensitive data

### MCP Server

- **ENCRYPTION_MASTER_KEY**: Master encryption key for the MCP server
  - Required for production deployments
  - Must be at least 32 characters long
  - Used to encrypt tool credentials and other sensitive data

## Best Practices

1. **Generate Strong Keys**: Use a cryptographically secure random generator
   ```bash
   # Generate a 32-byte key
   openssl rand -base64 32
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
   - The system supports key rotation via the `RotateKey` method

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
      - DEVMESH_ENCRYPTION_KEY=${DEVMESH_ENCRYPTION_KEY}
  
  mcp-server:
    environment:
      - ENCRYPTION_MASTER_KEY=${ENCRYPTION_MASTER_KEY}
```

```bash
# .env (DO NOT COMMIT)
DEVMESH_ENCRYPTION_KEY=your-secure-32-character-key-here
ENCRYPTION_MASTER_KEY=another-secure-32-character-key-here
```

## Kubernetes Deployment

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: encryption-keys
type: Opaque
data:
  devmesh-encryption-key: <base64-encoded-key>
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
        - name: DEVMESH_ENCRYPTION_KEY
          valueFrom:
            secretKeyRef:
              name: encryption-keys
              key: devmesh-encryption-key
```

## Troubleshooting

### Missing Encryption Key Warning

If you see warnings like:
```
DEVMESH_ENCRYPTION_KEY not set - using randomly generated key. This is not suitable for production!
```

This means the encryption key environment variable is not set. Set the appropriate environment variable before starting the service.

### Encrypted Data Unreadable

If you cannot decrypt previously encrypted data:
- Ensure you're using the same encryption key that was used to encrypt the data
- Check that the key hasn't been accidentally modified or truncated
- Verify the environment variable is being passed correctly to the container/process
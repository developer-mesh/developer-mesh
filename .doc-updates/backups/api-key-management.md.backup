# API Key Management Guide

This guide explains how to create, manage, and configure API keys for the Developer Mesh platform.

## Overview

API keys in Developer Mesh are stored in the PostgreSQL database in the `mcp.api_keys` table. In production, no static API keys are configured - all keys must be created in the database.

## Key Types

- **admin** (prefix: `adm_`) - Full administrative access
- **gateway** (prefix: `gw_`) - Gateway keys for service-to-service communication
- **agent** (prefix: `agt_`) - AI agent authentication
- **user** (prefix: `usr_`) - Standard user access

## Creating API Keys

### Method 1: Using the Management Script (Requires psql)

```bash
./scripts/manage-api-keys.sh create \
  -n "Production Admin" \
  -t tenant-123 \
  -T admin \
  -d  # Use database mode
```

### Method 2: Using the Key Generator

1. Generate a new API key:
```bash
go run scripts/generate-api-key/main.go admin
```

2. This will output:
   - The full API key (save this - it cannot be retrieved later)
   - The key hash for database storage
   - An SQL insert statement

3. Execute the SQL statement in your PostgreSQL database

### Method 3: Direct Database Insert

```sql
-- Generate a new admin API key
INSERT INTO mcp.api_keys (
    id, key_hash, key_prefix, tenant_id, user_id, name, key_type,
    scopes, is_active, rate_limit_requests, rate_limit_window_seconds,
    created_at, updated_at
) VALUES (
    uuid_generate_v4(), 
    'YOUR_KEY_HASH_HERE', 
    'YOUR_KEY_PREFIX_HERE', 
    'your-tenant-id', 
    NULL, 
    'Your Key Name', 
    'admin',
    ARRAY['read', 'write', 'admin'], 
    true, 
    1000, 
    60,
    CURRENT_TIMESTAMP, 
    CURRENT_TIMESTAMP
);
```

## API Key Format

API keys follow this format: `{prefix}_{base64_encoded_random_bytes}`

Example: `adm_dHg84VsfXagE4sd9jY-F5ePfOrgZnLXaBfRpxb_TW-0=`

## Security Considerations

1. **Storage**: Only the SHA-256 hash of the API key is stored in the database
2. **Transmission**: API keys should only be transmitted over HTTPS
3. **Rotation**: Implement regular key rotation policies
4. **Scopes**: Use the principle of least privilege when assigning scopes

## Using API Keys

### In HTTP Headers

```bash
# Using Authorization header
curl -H "Authorization: Bearer YOUR_API_KEY" https://api.dev-mesh.io/api/v1/...

# Using custom header (if configured)
curl -H "X-API-Key: YOUR_API_KEY" https://api.dev-mesh.io/api/v1/...
```

### In WebSocket Connections

```javascript
const ws = new WebSocket('wss://mcp.dev-mesh.io/ws', {
  headers: {
    'Authorization': 'Bearer YOUR_API_KEY'
  }
});
```

## Production Configuration

In production (`configs/config.production.yaml`):
```yaml
api:
  auth:
    api_keys:
      static_keys: {}  # No hardcoded keys
      source: "database"
```

This ensures all API keys must be created in the database.

## E2E Testing

For e2e tests against production:
1. Create a test API key in the production database
2. Store the key in GitHub secrets as `E2E_API_KEY`
3. The CI/CD pipeline will use this key for automated tests

## Troubleshooting

### 401 Unauthorized Errors

1. Check that the API key exists in the database:
```sql
SELECT key_prefix, name, key_type, is_active, expires_at 
FROM mcp.api_keys 
WHERE key_prefix = 'YOUR_KEY_PREFIX';
```

2. Verify the key is active and not expired
3. Check the debug logs (if enabled) for authentication details
4. Ensure the correct header is being used (Authorization or X-API-Key)

### Key Not Found

- Verify the key hash matches what's in the database
- Check that you're connecting to the correct database
- Ensure the key hasn't been revoked

## API Key Lifecycle

1. **Creation**: Generate key and store hash in database
2. **Distribution**: Securely share the full key with authorized users
3. **Usage**: Include key in API requests
4. **Monitoring**: Track usage via `api_key_usage` table
5. **Rotation**: Create new key, update clients, revoke old key
6. **Revocation**: Set `is_active = false` in database

## Best Practices

1. Never commit API keys to source control
2. Use environment variables or secure vaults for key storage
3. Implement key rotation every 90 days
4. Monitor key usage for anomalies
5. Use different keys for different environments
6. Implement IP whitelisting for sensitive keys
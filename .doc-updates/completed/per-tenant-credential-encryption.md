<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:38:03
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Per-Tenant Credential Encryption Documentation

## Overview

The DevOps MCP platform implements a sophisticated per-tenant credential encryption system to ensure that each tenant's sensitive data is isolated and protected using industry-standard encryption. This system is designed to meet enterprise security requirements while maintaining performance and scalability.

## Architecture

### Key Components

1. **Encryption Service** (`pkg/security/encryption.go`)
   - Provides AES-256-GCM encryption with authenticated encryption
   - Implements per-tenant key derivation using PBKDF2
   - Handles key rotation and secure token generation

2. **Master Key Management**
   - Single master key stored in environment variable `ENCRYPTION_KEY`
   - Used as base material for deriving tenant-specific keys
   - Never directly used for encryption

3. **Tenant-Specific Key Derivation**
   - Each tenant gets a unique encryption key derived from:
     - Master key
     - Tenant ID
     - Random salt (per encryption operation)
   - Uses PBKDF2 with SHA-256 and 10,000 iterations

## Implementation Details

### Encryption Process

```go
// 1. Generate random salt (32 bytes)
salt := make([]byte, 32)
io.ReadFull(rand.Reader, salt)

// 2. Derive tenant-specific key
key := pbkdf2.Key(
    append(masterKey, tenantID...), // Combine master key + tenant ID
    salt,                            // Random salt
    10000,                          // Iterations
    32,                             // Key size (256 bits)
    sha256.New                      // Hash function
)

// 3. Create AES-256-GCM cipher
block, _ := aes.NewCipher(key)
gcm, _ := cipher.NewGCM(block)

// 4. Generate nonce (12 bytes for GCM)
nonce := make([]byte, gcm.NonceSize())
io.ReadFull(rand.Reader, nonce)

// 5. Encrypt data with authentication
ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

// 6. Store: salt + nonce + ciphertext
```

### Storage Format

Encrypted credentials are stored as:
```
[32 bytes salt][12 bytes nonce][variable length ciphertext with 16 byte auth tag]
```

### Database Schema

```sql
CREATE TABLE tool_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    tool_id UUID NOT NULL REFERENCES dynamic_tools(id) ON DELETE CASCADE,
    encrypted_data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_tool_credentials_tool 
        FOREIGN KEY (tool_id) REFERENCES dynamic_tools(id) ON DELETE CASCADE
);

CREATE INDEX idx_tool_credentials_tenant_tool 
    ON tool_credentials(tenant_id, tool_id);
```

## Security Features

### 1. Tenant Isolation
- Each tenant's credentials are encrypted with a unique key
- Even if two tenants have the same credential value, the ciphertext will be different
- Compromise of one tenant's data doesn't affect others

### 1.1 Universal Agent Tenant Isolation
The platform extends tenant isolation to the universal agent registration system:

**Agent-Level Isolation:**
- Agents are automatically bound to their organization/tenant
- Cross-organization agent discovery is blocked by default
- Agent manifests include `organization_id` for enforcement
- All agent operations are filtered by organization

**Strict Isolation Mode:**
```go
type Organization struct {
    ID               uuid.UUID
    Name             string
    StrictlyIsolated bool  // When true, NO cross-org access allowed
}
```

**Message Routing Security:**
- Cross-organization messages blocked at broker level
- Explicit allow-lists for partner organizations
- All cross-org attempts are logged for audit
- Rate limiting per organization prevents abuse

**Discovery Filtering:**
```sql
-- Agent discovery automatically filtered by organization
SELECT * FROM agent_manifests 
WHERE organization_id = $1  -- User's org
  AND capability_name = $2;  -- Requested capability
```

### 2. Authenticated Encryption (AES-GCM)
- Provides both confidentiality and authenticity
- Detects any tampering with encrypted data
- Prevents padding oracle attacks

### 3. Key Derivation Security
- PBKDF2 with 10,000 iterations slows brute-force attacks
- Random salt prevents rainbow table attacks
- SHA-256 provides strong cryptographic hashing

### 4. Forward Secrecy
- Each encryption operation uses a unique salt
- Compromise of one encrypted value doesn't compromise others
- Different nonce for each encryption ensures uniqueness

## API Usage

### Creating a Tool with Credentials

```bash
curl -X POST https://api.example.com/api/v1/tools \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: tenant-123" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github-api",
    "base_url": "https://api.github.com",
    "credential": {
      "type": "bearer",
      "token": "ghp_xxxxxxxxxxxx"  # Plaintext - will be encrypted
    }
  }'
```

### Updating Credentials

```bash
curl -X PUT https://api.example.com/api/v1/tools/{toolId}/credentials \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: tenant-123" \
  -H "Content-Type: application/json" \
  -d '{
    "credential": {
      "type": "bearer",
      "token": "ghp_yyyyyyyyyyyy"  # New token - will be encrypted
    }
  }'
```

## Key Rotation

The system supports key rotation without service interruption:

```go
// Rotate encryption key for a tenant
func RotateKey(oldEncrypted []byte, tenantID string, newMasterKey string) ([]byte, error) {
    // 1. Decrypt with old key
    plaintext, err := DecryptCredential(oldEncrypted, tenantID)
    
    // 2. Create new encryption service
    newService := NewEncryptionService(newMasterKey)
    
    // 3. Encrypt with new key
    return newService.EncryptCredential(plaintext, tenantID)
}
```

### Key Rotation Process

1. Deploy new master key to environment
2. Run rotation job to re-encrypt all credentials
3. Verify all credentials can be decrypted
4. Remove old master key

## Configuration

### Environment Variables

```bash
# Required - Master encryption key (minimum 32 characters)
ENCRYPTION_KEY=your-very-secure-master-key-here

# Optional - Override default parameters
ENCRYPTION_SALT_SIZE=32        # Salt size in bytes
ENCRYPTION_KEY_ITERATIONS=10000 # PBKDF2 iterations
```

### Security Best Practices

1. **Master Key Generation**
   ```bash
   # Generate secure master key
   openssl rand -base64 32
   ```

2. **Key Storage**
   - Store master key in secure key management service (AWS KMS, HashiCorp Vault)
   - Never commit keys to source control
   - Rotate master key periodically (quarterly recommended)

3. **Access Control**
   - Limit access to encryption service to authorized services only
   - Audit all credential access
   - Monitor for unusual decryption patterns

## Monitoring and Alerts

### Metrics to Monitor

1. **Encryption Operations**
   - `encryption_operations_total{operation="encrypt|decrypt", status="success|failure"}`
   - `encryption_operation_duration_seconds`

2. **Key Derivation Performance**
   - `key_derivation_duration_seconds`
   - `key_cache_hit_rate`

3. **Security Events**
   - `decryption_failures_total`
   - `invalid_tenant_access_attempts`

### Alert Conditions

- High rate of decryption failures (possible attack)
- Unusual spike in encryption operations
- Access attempts with invalid tenant IDs
- Performance degradation in key derivation

## Compliance and Auditing

### Audit Log Format

```json
{
  "timestamp": "2024-01-26T10:30:00Z",
  "operation": "decrypt_credential",
  "tenant_id": "tenant-123",
  "tool_id": "tool-456",
  "user_id": "user-789",
  "ip_address": "192.0.2.1",
  "success": true,
  "duration_ms": 15
}
```

### Compliance Features

- **GDPR**: Supports right to erasure - credentials can be deleted per tenant
- **SOC 2**: Full audit trail of all credential operations
- **PCI DSS**: Strong encryption meets PCI requirements
- **HIPAA**: Encryption at rest for sensitive data

## Troubleshooting

### Common Issues

1. **Decryption Failures**
   - Check master key hasn't changed
   - Verify tenant ID matches encryption tenant ID
   - Ensure encrypted data hasn't been corrupted

2. **Performance Issues**
   - Consider caching derived keys (with appropriate TTL)
   - Monitor PBKDF2 iteration count vs security requirements
   - Check for excessive encryption operations

3. **Key Rotation Problems**
   - Ensure both old and new keys are available during rotation
   - Test rotation process in staging first
   - Have rollback plan ready

### Debug Mode

Enable debug logging for encryption operations:
```yaml
logging:
  encryption:
    level: debug
    include_tenant_id: true  # Be careful in production
```

## Testing

### Unit Test Example

```go
func TestPerTenantEncryption(t *testing.T) {
    svc := NewEncryptionService("test-master-key")
    
    // Test different tenants get different ciphertexts
    plaintext := "same-secret"
    
    encrypted1, _ := svc.EncryptCredential(plaintext, "tenant-1")
    encrypted2, _ := svc.EncryptCredential(plaintext, "tenant-2")
    
    // Same plaintext, different ciphertexts
    assert.NotEqual(t, encrypted1, encrypted2)
    
    // Both decrypt correctly
    decrypted1, _ := svc.DecryptCredential(encrypted1, "tenant-1")
    decrypted2, _ := svc.DecryptCredential(encrypted2, "tenant-2")
    
    assert.Equal(t, plaintext, decrypted1)
    assert.Equal(t, plaintext, decrypted2)
    
    // Cross-tenant decryption fails
    _, err := svc.DecryptCredential(encrypted1, "tenant-2")
    assert.Error(t, err)
}
```

## Performance Considerations

### Benchmarks

- Encryption: ~0.5ms per operation
- Decryption: ~0.3ms per operation
- Key derivation: ~10ms (can be cached)

### Optimization Strategies

1. **Key Caching**
   - Cache derived keys for 5 minutes
   - Use tenant ID as cache key
   - Clear cache on key rotation

2. **Batch Operations**
   - Process multiple credentials in parallel
   - Reuse derived keys within batch

3. **Hardware Acceleration**
   - Use AES-NI instructions when available
   - Consider HSM for high-security environments

## Migration Guide

### From Plaintext to Encrypted

```sql
-- Migration script example
BEGIN;

-- Add encrypted_data column
ALTER TABLE tool_credentials ADD COLUMN encrypted_data BYTEA;

-- Migrate existing data (run this in application)
-- SELECT id, tenant_id, plaintext_token FROM tool_credentials;
-- For each row: encrypt and update encrypted_data

-- Drop old column
ALTER TABLE tool_credentials DROP COLUMN plaintext_token;

COMMIT;
```

## Security Checklist

- [ ] Master key is at least 32 characters
- [ ] Master key stored securely (not in code)
- [ ] Key rotation process documented and tested
- [ ] Monitoring alerts configured
- [ ] Audit logging enabled
- [ ] Access controls implemented
- [ ] Regular security reviews scheduled
- [ ] Disaster recovery plan includes key backup
- [ ] Compliance requirements verified

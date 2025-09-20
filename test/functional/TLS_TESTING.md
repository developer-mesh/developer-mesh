# TLS Functional Testing Guide

This guide explains how to test the TLS configuration in the Developer Mesh platform.

## Overview

The platform supports industry-standard TLS configuration with:
- TLS 1.3 as default, TLS 1.2 as minimum
- Perfect Forward Secrecy (PFS) cipher suites only
- Optional TLS for development (disabled by default)
- Required TLS for production deployments

## Quick Start

### 1. Generate Test Certificates

```bash
# From project root
./scripts/certs/generate-dev-certs.sh
```

This creates:
- `certs/ca.crt` - Certificate Authority
- `certs/server.crt` - Server certificate
- `certs/server.key` - Server private key
- `certs/client.crt` - Client certificate (for mTLS)
- `certs/client.key` - Client private key

### 2. Start Services with TLS

```bash
# Export certificate paths
export TLS_CERT_FILE=./certs/server.crt
export TLS_KEY_FILE=./certs/server.key

# Start MCP Server with TLS
MCP_CONFIG_FILE=./test/functional/configs/config.tls.yaml ./apps/edge-mcp/edge-mcp

# Start REST API with TLS (in another terminal)
MCP_CONFIG_FILE=./test/functional/configs/config.tls.yaml ./apps/rest-api/api
```

### 3. Run TLS Tests

```bash
# Run all TLS tests
./test/functional/run-tls-tests.sh

# Or manually
TEST_TLS_ENABLED=true ginkgo ./test/functional/api -- --ginkgo.focus="TLS"
```

## Test Coverage

The TLS functional tests verify:

1. **Minimum TLS Version Enforcement**
   - Rejects TLS 1.1 and below
   - Accepts TLS 1.2 and 1.3

2. **Cipher Suite Security**
   - Rejects weak cipher suites
   - Only allows PFS cipher suites

3. **Protocol Preference**
   - Prefers TLS 1.3 when available
   - Falls back to TLS 1.2 when needed

4. **Certificate Handling**
   - Works with self-signed certs (dev)
   - Supports proper CA validation (prod)

## Configuration

### Development (TLS Optional)

```yaml
api:
  tls:
    enabled: false  # TLS disabled by default
```

### Testing (TLS Enabled)

```yaml
api:
  tls:
    enabled: true
    min_version: "1.3"
    cert_file: "${TLS_CERT_FILE}"
    key_file: "${TLS_KEY_FILE}"
    insecure_skip_verify: true  # For self-signed certs
```

### Production (TLS Required)

```yaml
api:
  tls:
    enabled: true
    min_version: "1.3"
    cert_file: "/etc/certs/tls.crt"  # From cert-manager
    key_file: "/etc/certs/tls.key"
    verify_certificates: true
    insecure_skip_verify: false
```

## Troubleshooting

### Connection Refused

If you get connection refused errors:
1. Ensure services are running with TLS config
2. Check certificate file paths are correct
3. Verify ports (HTTPS uses 8443, not 8080)

### Certificate Errors

For self-signed certificate errors:
1. Ensure `insecure_skip_verify: true` in test config
2. Use `-k` flag with curl for testing
3. Configure test HTTP clients to skip verification

### TLS Version Errors

If you see protocol version errors:
1. Check minimum TLS version in config
2. Ensure client supports required TLS version
3. Update client libraries if needed

## AWS ElastiCache with TLS

When using ElastiCache through SSH tunnel:

```yaml
cache:
  redis:
    tls:
      enabled: true
      insecure_skip_verify: true  # Required for SSH tunnel
```

## Best Practices

1. **Development**: Keep TLS disabled for simplicity
2. **Testing**: Use self-signed certificates
3. **Staging**: Use Let's Encrypt or internal CA
4. **Production**: Use cert-manager.io with proper CA

## Security Notes

- Never use `insecure_skip_verify: true` in production
- Always verify certificates in production
- Keep TLS 1.3 as default for best security
- Monitor for deprecated cipher suites
- Rotate certificates regularly
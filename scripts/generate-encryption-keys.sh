#!/bin/bash
# Script to generate secure encryption key for production use

set -euo pipefail

echo "=== DevOps MCP Encryption Key Generator ==="
echo ""
echo "This script generates a secure encryption key for production use."
echo "The key should be at least 32 characters long."
echo ""

# Function to generate a secure key
generate_key() {
    # Generate 32 bytes of random data and encode as base64
    # This produces a 44-character string suitable for encryption
    openssl rand -base64 32 | tr -d '\n'
}

# Generate the master key
MASTER_KEY=$(generate_key)

# Output in different formats
echo "=== Environment Variables ==="
echo "Add this to your .env file or set as environment variable:"
echo ""
echo "ENCRYPTION_MASTER_KEY=$MASTER_KEY"
echo ""

echo "=== Docker Compose ==="
echo "Add this to your docker-compose.yml environment section:"
echo ""
echo "      - ENCRYPTION_MASTER_KEY=$MASTER_KEY"
echo ""
echo "Or reference from .env file:"
echo "      - ENCRYPTION_MASTER_KEY=\${ENCRYPTION_MASTER_KEY}"
echo ""

echo "=== Kubernetes Secret ==="
echo "Create a Kubernetes secret with:"
echo ""
echo "kubectl create secret generic encryption-keys \\"
echo "  --from-literal=encryption-master-key=$MASTER_KEY"
echo ""

echo "=== AWS Systems Manager Parameter Store ==="
echo "Store in Parameter Store with:"
echo ""
echo "aws ssm put-parameter --name '/devops-mcp/encryption/master-key' --value '$MASTER_KEY' --type 'SecureString'"
echo ""

echo "=== IMPORTANT SECURITY NOTES ==="
echo "1. Store this key securely - it cannot be recovered if lost"
echo "2. Never commit this key to version control"
echo "3. Use different keys for each environment (dev, staging, prod)"
echo "4. Rotate keys periodically according to your security policy"
echo "5. Ensure proper access controls on wherever you store this key"
echo "6. This single key is used by all services for consistency"
echo ""
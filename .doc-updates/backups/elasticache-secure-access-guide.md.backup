# AWS ElastiCache Secure Access Guide for Local Development

> **Last Updated**: December 2024
> **Purpose**: Comprehensive guide for secure ElastiCache access during development
> **Focus**: SSH tunnel setup, security best practices, and troubleshooting

This guide provides comprehensive instructions for configuring secure access to AWS ElastiCache Redis from a local development machine using the Developer Mesh platform's SSH tunnel approach.

## Table of Contents
1. [Quick Start](#quick-start)
2. [Security Group Configuration](#security-group-configuration)
3. [VPC and Network Configuration](#vpc-and-network-configuration)
4. [SSH Tunnel Setup](#ssh-tunnel-setup)
5. [ElastiCache Security Best Practices](#elasticache-security-best-practices)
6. [Local Development Setup](#local-development-setup)
7. [Testing and Validation](#testing-and-validation)
8. [Troubleshooting](#troubleshooting)
9. [Production Considerations](#production-considerations)

## Prerequisites
- AWS CLI configured with appropriate credentials
- SSH client installed
- Redis CLI tools installed (`redis-cli`)
- VPC ID and subnet information for your ElastiCache cluster
- Bastion host SSH key file
- Environment variables configured in `.env`

## Quick Start

The Developer Mesh platform includes a ready-to-use SSH tunnel script:

```bash
# 1. Ensure your .env file contains:
BASTION_HOST_IP=<your-bastion-ip>
ELASTICACHE_ENDPOINT=<your-redis-endpoint>
BASTION_KEY_FILE=~/.ssh/dev-bastion-key.pem

# 2. Run the tunnel script (keep it running)
./scripts/aws/connect-elasticache.sh

# 3. In another terminal, test the connection
redis-cli -h localhost -p 6379 ping

# 4. Configure your application
REDIS_ADDR=127.0.0.1:6379  # Use 127.0.0.1, NOT localhost
USE_SSH_TUNNEL_FOR_REDIS=true
```

## Security Group Configuration

### Step 1: Identify Your Current IP Address

```bash
# Get your current public IP address
curl -s https://checkip.amazonaws.com
```

### Step 2: Create or Update Security Group

#### Using AWS CLI

```bash
# Create a new security group for ElastiCache access
aws ec2 create-security-group \
    --group-name elasticache-dev-access \
    --description "Security group for ElastiCache development access" \
    --vpc-id <your-vpc-id> \
    --region us-east-1

# Note the security group ID from the output
export SG_ID=<security-group-id>

# Add inbound rule for Redis port from your IP only
aws ec2 authorize-security-group-ingress \
    --group-id $SG_ID \
    --protocol tcp \
    --port 6379 \
    --cidr $(curl -s https://checkip.amazonaws.com)/32 \
    --region us-east-1

# Tag the security group for easy identification
aws ec2 create-tags \
    --resources $SG_ID \
    --tags Key=Name,Value=elasticache-dev-access \
    --region us-east-1
```

#### Using AWS Console

1. Navigate to EC2 → Security Groups
2. Click "Create security group"
3. Configure:
   - **Security group name**: `elasticache-dev-access`
   - **Description**: `Security group for ElastiCache development access`
   - **VPC**: Select the VPC containing your ElastiCache cluster
4. Add inbound rule:
   - **Type**: Custom TCP
   - **Port range**: 6379
   - **Source**: My IP (automatically detects your current IP)
   - **Description**: `Redis access from development machine`
5. Click "Create security group"

### Step 3: Attach Security Group to ElastiCache

```bash
# Modify ElastiCache cluster to use the security group
aws elasticache modify-replication-group \
    --replication-group-id <your-cluster-id> \
    --security-group-ids $SG_ID \
    --apply-immediately \
    --region us-east-1
```

## VPC and Network Configuration

### Understanding ElastiCache Network Requirements

ElastiCache clusters run within a VPC and are not directly accessible from the internet. You have three main options for secure access:

1. **SSH Tunnel through Bastion Host** (Recommended for development)
2. **AWS Client VPN** (Better for teams)
3. **Site-to-Site VPN** (Enterprise solution)

## SSH Tunnel Setup

The Developer Mesh platform uses SSH tunneling as the primary method for local ElastiCache access. This approach is secure, simple, and doesn't require VPN configuration.

### Using the Platform's SSH Tunnel Script

The platform includes `scripts/aws/connect-elasticache.sh` which handles tunnel management:

```bash
#!/bin/bash
# Key features of the script:
# - Reads configuration from .env file
# - Validates all required environment variables
# - Creates secure SSH tunnel with proper options
# - Provides clear connection information

# Required environment variables:
BASTION_HOST_IP=54.123.456.789         # Your bastion host's public IP
ELASTICACHE_ENDPOINT=redis.abc123.cache.amazonaws.com  # Redis endpoint
BASTION_KEY_FILE=~/.ssh/dev-bastion-key.pem          # Path to SSH key
REDIS_TUNNEL_PORT=6379                 # Local port (optional, defaults to 6379)
```

### Setting Up the Tunnel

1. **Configure Environment Variables**:
   ```bash
   # Add to your .env file
   BASTION_HOST_IP=54.123.456.789
   ELASTICACHE_ENDPOINT=sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com
   BASTION_KEY_FILE=$HOME/.ssh/dev-bastion-key.pem
   ```

2. **Ensure Key Permissions**:
   ```bash
   chmod 400 $HOME/.ssh/dev-bastion-key.pem
   ```

3. **Start the Tunnel**:
   ```bash
   ./scripts/aws/connect-elasticache.sh
   ```

4. **Keep the Tunnel Running**:
   - The script runs in the foreground
   - Keep the terminal window open
   - Press Ctrl+C to close the tunnel

### Advanced Tunnel Management

For background operation or automatic management:

```bash
# Run tunnel in background
nohup ./scripts/aws/connect-elasticache.sh > tunnel.log 2>&1 &

# Check if tunnel is running
ps aux | grep "[s]sh.*6379"

# Kill tunnel
pkill -f "ssh.*6379:.*cache.amazonaws.com"
```

### Creating a Systemd Service (Linux/macOS)

Create `/etc/systemd/system/elasticache-tunnel.service`:

```ini
[Unit]
Description=ElastiCache SSH Tunnel
After=network.target

[Service]
Type=simple
User=your-username
Environment="PATH=/usr/local/bin:/usr/bin:/bin"
WorkingDirectory=/path/to/developer-mesh
ExecStart=/path/to/developer-mesh/scripts/aws/connect-elasticache.sh
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable elasticache-tunnel
sudo systemctl start elasticache-tunnel
sudo systemctl status elasticache-tunnel
```

### Bastion Host Setup

If you need to create a bastion host:

```bash
# Create a key pair for the bastion host
aws ec2 create-key-pair \
    --key-name elasticache-bastion-key \
    --query 'KeyMaterial' \
    --output text > ~/.ssh/elasticache-bastion-key.pem

chmod 400 ~/.ssh/elasticache-bastion-key.pem

# Launch a minimal EC2 instance as bastion
aws ec2 run-instances \
    --image-id ami-0c02fb55956c7d316 \
    --instance-type t3.nano \
    --key-name elasticache-bastion-key \
    --security-group-ids <bastion-sg-id> \
    --subnet-id <public-subnet-id> \
    --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=elasticache-bastion}]' \
    --region us-east-1
```

#### Step 2: Configure Bastion Security Group

```bash
# Create bastion security group
aws ec2 create-security-group \
    --group-name elasticache-bastion-sg \
    --description "Security group for ElastiCache bastion host" \
    --vpc-id <your-vpc-id> \
    --region us-east-1

# Allow SSH from your IP only
aws ec2 authorize-security-group-ingress \
    --group-id <bastion-sg-id> \
    --protocol tcp \
    --port 22 \
    --cidr $(curl -s https://checkip.amazonaws.com)/32 \
    --region us-east-1

# Allow bastion to connect to ElastiCache
aws ec2 authorize-security-group-ingress \
    --group-id $SG_ID \
    --protocol tcp \
    --port 6379 \
    --source-group <bastion-sg-id> \
    --region us-east-1
```

### Option 2: AWS Client VPN

For a more permanent solution, set up AWS Client VPN:

```bash
# Generate server and client certificates
git clone https://github.com/OpenVPN/easy-rsa.git
cd easy-rsa/easyrsa3
./easyrsa init-pki
./easyrsa build-ca nopass
./easyrsa build-server-full server nopass
./easyrsa build-client-full client1.domain.tld nopass

# Import certificates to ACM
aws acm import-certificate \
    --certificate fileb://pki/issued/server.crt \
    --private-key fileb://pki/private/server.key \
    --certificate-chain fileb://pki/ca.crt \
    --region us-east-1

# Create Client VPN endpoint
aws ec2 create-client-vpn-endpoint \
    --client-cidr-block 10.0.0.0/22 \
    --server-certificate-arn <server-cert-arn> \
    --authentication-options Type=certificate-authentication,MutualAuthentication={ClientRootCertificateChainArn=<client-cert-arn>} \
    --connection-log-options Enabled=false \
    --region us-east-1
```

## ElastiCache Security Best Practices

### 1. Enable Encryption in Transit

```bash
# Create or modify replication group with encryption in transit
aws elasticache create-replication-group \
    --replication-group-id secure-redis-cluster \
    --replication-group-description "Secure Redis cluster with encryption" \
    --engine redis \
    --cache-node-type cache.t3.micro \
    --num-cache-clusters 1 \
    --transit-encryption-enabled \
    --region us-east-1
```

### 2. Enable Encryption at Rest

```bash
# Enable encryption at rest
aws elasticache create-replication-group \
    --replication-group-id secure-redis-cluster \
    --at-rest-encryption-enabled \
    --region us-east-1
```

### 3. Configure Redis AUTH

```bash
# Set AUTH token for Redis
aws elasticache modify-replication-group \
    --replication-group-id <your-cluster-id> \
    --auth-token <strong-password> \
    --auth-token-update-strategy ROTATE \
    --apply-immediately \
    --region us-east-1
```

### 4. Configure Parameter Groups

```bash
# Create custom parameter group
aws elasticache create-cache-parameter-group \
    --cache-parameter-group-family redis7 \
    --cache-parameter-group-name secure-redis-params \
    --description "Secure Redis parameter group" \
    --region us-east-1

# Configure secure parameters
aws elasticache modify-cache-parameter-group \
    --cache-parameter-group-name secure-redis-params \
    --parameter-name-values \
        ParameterName=maxmemory-policy,ParameterValue=allkeys-lru \
        ParameterName=timeout,ParameterValue=300 \
        ParameterName=tcp-keepalive,ParameterValue=60 \
    --region us-east-1
```

### 5. Enable Automatic Backups

```bash
# Configure automatic backups
aws elasticache modify-replication-group \
    --replication-group-id <your-cluster-id> \
    --snapshot-retention-limit 7 \
    --snapshot-window 03:00-05:00 \
    --apply-immediately \
    --region us-east-1
```

## Local Development Setup

### Platform Configuration

The Developer Mesh platform is pre-configured to work with ElastiCache through SSH tunnels. Here's how to set it up:

#### Step 1: Environment Configuration

Create or update your `.env` file:

```bash
# AWS Configuration
AWS_REGION=us-east-1
AWS_PROFILE=default  # Or your specific profile

# ElastiCache Configuration
REDIS_ADDR=127.0.0.1:6379              # IMPORTANT: Use 127.0.0.1, not localhost
USE_SSH_TUNNEL_FOR_REDIS=true          # Enable tunnel mode
ELASTICACHE_ENDPOINT=sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com
REDIS_PASSWORD=                        # Add if AUTH is enabled

# SSH Tunnel Configuration
BASTION_HOST_IP=54.123.456.789        # Your bastion's public IP
BASTION_KEY_FILE=~/.ssh/dev-bastion-key.pem
REDIS_TUNNEL_PORT=6379                # Local port for tunnel

# Cost Controls (for production)
BEDROCK_SESSION_LIMIT=0.10
GLOBAL_COST_LIMIT=10.0
```

#### Step 2: Application Configuration

The platform automatically detects tunnel mode from `USE_SSH_TUNNEL_FOR_REDIS`:

```go
// In pkg/common/aws_clients.go
if os.Getenv("USE_SSH_TUNNEL_FOR_REDIS") == "true" {
    // Uses REDIS_ADDR (127.0.0.1:6379) for tunneled connection
    // No TLS since tunnel handles encryption
} else {
    // Direct connection with TLS (for production)
}
```

#### Step 3: Running with SSH Tunnel

```bash
# Terminal 1: Start and keep the SSH tunnel running
./scripts/aws/connect-elasticache.sh

# Terminal 2: Run your application
make dev-native

# Or run specific services
go run apps/mcp-server/cmd/main.go
go run apps/rest-api/cmd/main.go
go run apps/worker/cmd/main.go
```

#### Step 4: Docker Development

For Docker-based development, ensure the tunnel is accessible:

```yaml
# docker-compose.yml adjustments
services:
  mcp-server:
    environment:
      - REDIS_ADDR=host.docker.internal:6379  # Access host's tunnel
      - USE_SSH_TUNNEL_FOR_REDIS=true
    extra_hosts:
      - "host.docker.internal:host-gateway"   # For Linux compatibility
```

### Automated Tunnel Script

Create `scripts/elasticache-tunnel.sh`:

```bash
#!/bin/bash

# ElastiCache tunnel management script
BASTION_IP="<bastion-public-ip>"
ELASTICACHE_ENDPOINT="sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com"
LOCAL_PORT=6379
REMOTE_PORT=6379

function start_tunnel() {
    echo "Starting ElastiCache tunnel..."
    ssh -N -f -L ${LOCAL_PORT}:${ELASTICACHE_ENDPOINT}:${REMOTE_PORT} \
        -i ~/.ssh/elasticache-bastion-key.pem \
        -o ServerAliveInterval=60 \
        -o ServerAliveCountMax=3 \
        -o ExitOnForwardFailure=yes \
        ec2-user@${BASTION_IP}
    
    if [ $? -eq 0 ]; then
        echo "Tunnel established on localhost:${LOCAL_PORT}"
    else
        echo "Failed to establish tunnel"
        exit 1
    fi
}

function stop_tunnel() {
    echo "Stopping ElastiCache tunnel..."
    pkill -f "ssh.*${LOCAL_PORT}:${ELASTICACHE_ENDPOINT}"
}

function status_tunnel() {
    if pgrep -f "ssh.*${LOCAL_PORT}:${ELASTICACHE_ENDPOINT}" > /dev/null; then
        echo "Tunnel is running"
    else
        echo "Tunnel is not running"
    fi
}

case "$1" in
    start)
        start_tunnel
        ;;
    stop)
        stop_tunnel
        ;;
    restart)
        stop_tunnel
        sleep 2
        start_tunnel
        ;;
    status)
        status_tunnel
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac
```

Make it executable:
```bash
chmod +x scripts/elasticache-tunnel.sh
```

## Testing and Validation

### Test Connectivity

```bash
# Test basic connectivity through tunnel
redis-cli -h localhost -p 6379 ping

# Test with AUTH if enabled
redis-cli -h localhost -p 6379 -a <your-auth-token> ping

# Test TLS connection (direct, not through tunnel)
redis-cli -h sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com \
    -p 6379 \
    --tls \
    --cacert /path/to/ca.crt \
    -a <your-auth-token> \
    ping
```

### Validate Security Configuration

```bash
# Check security group rules
aws ec2 describe-security-groups \
    --group-ids $SG_ID \
    --query 'SecurityGroups[0].IpPermissions' \
    --region us-east-1

# Verify encryption settings
aws elasticache describe-replication-groups \
    --replication-group-id <your-cluster-id> \
    --query 'ReplicationGroups[0].[TransitEncryptionEnabled,AtRestEncryptionEnabled,AuthTokenEnabled]' \
    --region us-east-1

# Test cluster info
redis-cli -h localhost -p 6379 -a <your-auth-token> INFO server
```

### Performance Testing

```bash
# Basic performance test
redis-benchmark -h localhost -p 6379 -a <your-auth-token> -c 10 -n 10000

# Test specific operations
redis-benchmark -h localhost -p 6379 -a <your-auth-token> -t get,set -n 100000
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Connection Timeout

```bash
# Check if tunnel is running
ps aux | grep ssh | grep 6379

# Check if bastion is reachable
nc -zv <bastion-ip> 22

# Test ElastiCache endpoint from bastion
ssh elasticache-tunnel "nc -zv sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com 6379"
```

#### 2. Authentication Errors

```bash
# Verify AUTH token
aws elasticache describe-cache-clusters \
    --cache-cluster-id <cluster-id> \
    --show-cache-node-info \
    --region us-east-1

# Test with correct AUTH
redis-cli -h localhost -p 6379 -a <your-auth-token> --no-auth-warning ping
```

#### 3. Security Group Issues

```bash
# List all security groups for the cluster
aws elasticache describe-cache-clusters \
    --cache-cluster-id <cluster-id> \
    --query 'CacheClusters[0].SecurityGroups' \
    --region us-east-1

# Check current IP address matches security group rule
echo "Current IP: $(curl -s https://checkip.amazonaws.com)"
aws ec2 describe-security-groups \
    --group-ids $SG_ID \
    --query 'SecurityGroups[0].IpPermissions[?FromPort==`6379`].IpRanges[].CidrIp' \
    --region us-east-1
```

#### 4. DNS Resolution Issues

```bash
# Test DNS resolution
nslookup sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com

# Use direct IP if DNS fails
dig +short sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com
```

### Monitoring and Logs

```bash
# View CloudWatch metrics
aws cloudwatch get-metric-statistics \
    --namespace AWS/ElastiCache \
    --metric-name CPUUtilization \
    --dimensions Name=CacheClusterId,Value=<cluster-id> \
    --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
    --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
    --period 300 \
    --statistics Average \
    --region us-east-1

# Check ElastiCache events
aws elasticache describe-events \
    --source-identifier <cluster-id> \
    --source-type cache-cluster \
    --max-records 20 \
    --region us-east-1
```

## Security Checklist

- [ ] Security group allows access only from specific IP addresses
- [ ] Encryption in transit is enabled
- [ ] Encryption at rest is enabled
- [ ] Redis AUTH is configured with a strong password
- [ ] Automatic backups are configured
- [ ] Network access is restricted to VPC only
- [ ] Bastion host has minimal security group rules
- [ ] SSH keys are properly secured (400 permissions)
- [ ] Environment variables are not committed to version control
- [ ] Regular security group audits are scheduled
- [ ] CloudWatch alarms are configured for security events

## Best Practices Summary

1. **Never expose ElastiCache directly to the internet**
2. **Always use encryption in transit and at rest**
3. **Implement Redis AUTH with strong passwords**
4. **Regularly rotate credentials and access keys**
5. **Monitor access logs and CloudWatch metrics**
6. **Use IAM roles for EC2 instances accessing ElastiCache**
7. **Implement network segmentation with security groups**
8. **Keep bastion hosts updated and minimal**
9. **Use VPN solutions for production environments**
10. **Document all access patterns and maintain audit trails**

## Production Considerations

### Moving Beyond SSH Tunnels

While SSH tunnels are perfect for development, production environments should use:

1. **Direct VPC Access**:
   ```bash
   # Production configuration
   USE_SSH_TUNNEL_FOR_REDIS=false
   REDIS_ADDR=redis-cluster.abc123.cache.amazonaws.com:6379
   REDIS_TLS_ENABLED=true
   ```

2. **AWS PrivateLink** for cross-VPC access
3. **VPC Peering** for multi-region setups
4. **Transit Gateway** for complex network topologies

### Production Security Checklist

- [ ] Remove all bastion hosts from production
- [ ] Enable AWS ElastiCache encryption at rest
- [ ] Enable TLS/SSL for all connections
- [ ] Configure Redis AUTH with AWS Secrets Manager rotation
- [ ] Set up CloudWatch alarms for suspicious activity
- [ ] Enable AWS GuardDuty for threat detection
- [ ] Configure VPC Flow Logs
- [ ] Implement least-privilege IAM policies
- [ ] Use AWS Systems Manager Session Manager instead of SSH
- [ ] Enable AWS Config rules for compliance

### Cost Optimization

1. **Right-size your ElastiCache nodes**:
   ```bash
   # Monitor usage patterns
   aws cloudwatch get-metric-statistics \
     --namespace AWS/ElastiCache \
     --metric-name CPUUtilization \
     --dimensions Name=CacheClusterId,Value=your-cluster-id \
     --statistics Average \
     --start-time 2024-12-01T00:00:00Z \
     --end-time 2024-12-25T00:00:00Z \
     --period 3600
   ```

2. **Use Reserved Instances** for predictable workloads
3. **Enable automatic backups** during low-traffic windows
4. **Consider ElastiCache Serverless** for variable workloads

## Platform-Specific Notes

The Developer Mesh platform is designed to work seamlessly with ElastiCache:

- **Automatic failover** handling in `pkg/common/aws_clients.go`
- **Connection pooling** for optimal performance
- **Retry logic** with exponential backoff
- **Metrics collection** for monitoring
- **Cost tracking** per operation

For platform-specific issues, check:
- `pkg/common/aws_clients.go` - Redis client configuration
- `pkg/cache/redis_cache.go` - Caching implementation
- `scripts/aws/connect-elasticache.sh` - SSH tunnel script

## Additional Resources

- [AWS ElastiCache Security Best Practices](https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/elasticache-security.html)
- [VPC Security Best Practices](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-security-best-practices.html)
- [Redis Security Guide](https://redis.io/topics/security)
- [AWS Well-Architected Framework - Security Pillar](https://docs.aws.amazon.com/wellarchitected/latest/security-pillar/welcome.html)
- [Developer Mesh Production Deployment Guide](./production-deployment.md)
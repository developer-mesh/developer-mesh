# AWS SQS Security Configuration Guide

## Current Configuration Summary

### Queue Details
- **Queue Name**: sean-mcp-test
- **Queue URL**: https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test
- **Region**: us-east-1
- **Account**: 594992249511

### Security Status
- ✅ **Server-Side Encryption (SSE)**: Enabled
- ⚠️ **Dead Letter Queue**: Not configured
- ⚠️ **IAM Access**: Currently using root credentials
- ✅ **Message Retention**: 4 days (345600 seconds)
- ✅ **Visibility Timeout**: 30 seconds

## Security Recommendations

### 1. IAM Best Practices

#### Create Dedicated IAM User/Role
Instead of using root credentials, create a dedicated IAM user or role for the application:

```bash
# Create IAM user
aws iam create-user --user-name mcp-sqs-worker

# Attach policy (use the policy in sqs-iam-policy.json)
aws iam put-user-policy \
  --user-name mcp-sqs-worker \
  --policy-name MCPSQSWorkerPolicy \
  --policy-document file://docs/operations/sqs-iam-policy.json

# Create access keys
aws iam create-access-key --user-name mcp-sqs-worker
```

#### Use IAM Roles for EC2/ECS/Lambda
If running on AWS infrastructure, use IAM roles instead of access keys:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "ec2.amazonaws.com",
          "ecs-tasks.amazonaws.com",
          "lambda.amazonaws.com"
        ]
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

### 2. Configure Dead Letter Queue

A Dead Letter Queue (DLQ) helps handle messages that fail processing:

```bash
# Create DLQ
aws sqs create-queue \
  --queue-name sean-mcp-test-dlq \
  --region us-east-1 \
  --attributes MessageRetentionPeriod=1209600

# Get DLQ ARN
DLQ_ARN=$(aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test-dlq \
  --attribute-names QueueArn \
  --region us-east-1 \
  --query 'Attributes.QueueArn' \
  --output text)

# Configure redrive policy
aws sqs set-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test \
  --attributes '{
    "RedrivePolicy": "{\"deadLetterTargetArn\":\"'$DLQ_ARN'\",\"maxReceiveCount\":3}"
  }'
```

### 3. Enhanced Encryption

While SSE is enabled, consider using Customer Master Keys (CMK) for additional control:

```bash
# Create CMK for SQS
aws kms create-key \
  --description "CMK for MCP SQS queues" \
  --key-usage ENCRYPT_DECRYPT \
  --origin AWS_KMS

# Update queue to use CMK
aws sqs set-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test \
  --attributes KmsMasterKeyId=alias/mcp-sqs-key
```

### 4. Queue Access Policy

Restrict queue access to specific AWS accounts or services:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowAccountAccess",
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::594992249511:user/mcp-sqs-worker"
      },
      "Action": [
        "sqs:SendMessage",
        "sqs:ReceiveMessage",
        "sqs:DeleteMessage",
        "sqs:GetQueueAttributes",
        "sqs:ChangeMessageVisibility"
      ],
      "Resource": "arn:aws:sqs:us-east-1:594992249511:sean-mcp-test"
    }
  ]
}
```

### 5. Monitoring and Alerting

Set up CloudWatch alarms for queue monitoring:

```bash
# Alarm for old messages
aws cloudwatch put-metric-alarm \
  --alarm-name mcp-sqs-old-messages \
  --alarm-description "Alert when messages are older than 1 hour" \
  --metric-name ApproximateAgeOfOldestMessage \
  --namespace AWS/SQS \
  --statistic Maximum \
  --period 300 \
  --threshold 3600 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=QueueName,Value=sean-mcp-test \
  --evaluation-periods 2

# Alarm for queue depth
aws cloudwatch put-metric-alarm \
  --alarm-name mcp-sqs-queue-depth \
  --alarm-description "Alert when queue has too many messages" \
  --metric-name ApproximateNumberOfMessagesVisible \
  --namespace AWS/SQS \
  --statistic Average \
  --period 300 \
  --threshold 1000 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=QueueName,Value=sean-mcp-test \
  --evaluation-periods 2
```

### 6. Application Configuration

Update your application configuration to use environment-specific settings:

```yaml
# Production configuration
worker:
  sqs:
    queue_url: "${SQS_QUEUE_URL}"
    region: "${AWS_REGION}"
    visibility_timeout: 300s  # 5 minutes for complex processing
    wait_time_seconds: 20     # Long polling
    max_messages: 10
    
    # Retry configuration
    retry:
      max_attempts: 3
      backoff_multiplier: 2
      initial_interval: 1s
      max_interval: 60s
```

### 7. Local Development Best Practices

For local development, use AWS profiles instead of environment variables:

```bash
# Configure AWS profile
aws configure --profile mcp-dev
AWS_ACCESS_KEY_ID=<dev-access-key>
AWS_SECRET_ACCESS_KEY=<dev-secret-key>
AWS_REGION=us-east-1

# Use profile in application
export AWS_PROFILE=mcp-dev
```

### 8. Security Checklist

- [ ] Replace root credentials with IAM user/role
- [ ] Configure Dead Letter Queue
- [ ] Set up CloudWatch monitoring
- [ ] Review and restrict queue access policy
- [ ] Enable CloudTrail logging for SQS API calls
- [ ] Regularly rotate access keys
- [ ] Use VPC endpoints for private connectivity
- [ ] Implement message validation in application
- [ ] Set appropriate visibility timeout
- [ ] Configure message retention based on requirements

## Testing Commands

```bash
# Run security check
./scripts/check-sqs-security.sh

# Test worker connection
./scripts/test-sqs-worker-connection.sh

# Monitor queue metrics
aws cloudwatch get-metric-statistics \
  --namespace AWS/SQS \
  --metric-name ApproximateNumberOfMessagesVisible \
  --dimensions Name=QueueName,Value=sean-mcp-test \
  --statistics Average \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300
```

## Environment Variables

Ensure these are set for the application:

```bash
export AWS_REGION=us-east-1
export SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test
export AWS_ACCESS_KEY_ID=<from-iam-user>
export AWS_SECRET_ACCESS_KEY=<from-iam-user>
```

## Next Steps

1. Create IAM user with minimal permissions
2. Configure Dead Letter Queue
3. Set up monitoring and alerting
4. Update application configuration
5. Test with new credentials
6. Document emergency procedures
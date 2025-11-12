# AWS IAM Permissions Reference

This document outlines the required AWS IAM permissions for Developer Mesh services in production environments.

## Table of Contents

- [AWS Bedrock Permissions](#aws-bedrock-permissions)
- [IAM Role Setup for Kubernetes (IRSA)](#iam-role-setup-for-kubernetes-irsa)
- [Testing Permissions](#testing-permissions)
- [Troubleshooting](#troubleshooting)

## AWS Bedrock Permissions

Developer Mesh uses AWS Bedrock for embedding generation. The following IAM permissions are required for the embedding service.

### Required Actions

The service requires permissions to invoke Bedrock foundation models:

- `bedrock:InvokeModel` - Invoke a foundation model
- `bedrock:InvokeModelWithResponseStream` - Invoke a foundation model with streaming response

### IAM Policy

Create an IAM policy with the following JSON:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "BedrockEmbeddingModels",
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream"
      ],
      "Resource": [
        "arn:aws:bedrock:*::foundation-model/amazon.titan-embed-text-v1",
        "arn:aws:bedrock:*::foundation-model/amazon.titan-embed-text-v2:0",
        "arn:aws:bedrock:*::foundation-model/cohere.embed-english-v3",
        "arn:aws:bedrock:*::foundation-model/cohere.embed-multilingual-v3"
      ]
    }
  ]
}
```

### Supported Models

Developer Mesh supports the following Bedrock embedding models:

| Model ID | Model Name | Dimensions | Use Case |
|----------|-----------|-----------|----------|
| `amazon.titan-embed-text-v1` | Amazon Titan Text Embeddings v1 | 1536 | General-purpose text embeddings |
| `amazon.titan-embed-text-v2:0` | Amazon Titan Text Embeddings v2 | 1024 | Enhanced general-purpose with dimension reduction support |
| `cohere.embed-english-v3` | Cohere Embed English v3 | 1024 | English text embeddings |
| `cohere.embed-multilingual-v3` | Cohere Embed Multilingual v3 | 1024 | Multilingual text embeddings |

### Regional Considerations

- **Wildcard Region (`*`)**: The policy above uses wildcard for regions to support multi-region deployments
- **Region-Specific**: If you want to restrict to specific regions, replace `*` with your region:
  ```json
  "arn:aws:bedrock:us-east-1::foundation-model/amazon.titan-embed-text-v1"
  ```
- **Model Availability**: Ensure your chosen models are available in your AWS region. Check the [AWS Bedrock documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/model-ids.html) for current availability.

### Cost Optimization

To reduce costs, you can restrict the policy to only the models you use:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "BedrockTitanOnly",
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel"
      ],
      "Resource": [
        "arn:aws:bedrock:us-east-1::foundation-model/amazon.titan-embed-text-v2:0"
      ]
    }
  ]
}
```

## IAM Role Setup for Kubernetes (IRSA)

### What is IRSA?

IAM Roles for Service Accounts (IRSA) allows Kubernetes pods to assume IAM roles without managing static AWS credentials. This is the recommended approach for production deployments.

### Prerequisites

- Amazon EKS cluster
- OIDC provider enabled on your cluster
- `eksctl` or AWS CLI configured

### Step 1: Create IAM Role with Trust Policy

Create an IAM role that can be assumed by your Kubernetes ServiceAccount:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/oidc.eks.REGION.amazonaws.com/id/OIDC_ID"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "oidc.eks.REGION.amazonaws.com/id/OIDC_ID:sub": "system:serviceaccount:NAMESPACE:SERVICE_ACCOUNT_NAME",
          "oidc.eks.REGION.amazonaws.com/id/OIDC_ID:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
```

Replace:
- `ACCOUNT_ID` - Your AWS account ID
- `REGION` - Your EKS cluster region (e.g., `us-east-1`)
- `OIDC_ID` - Your cluster's OIDC provider ID
- `NAMESPACE` - Kubernetes namespace (e.g., `devmesh`)
- `SERVICE_ACCOUNT_NAME` - ServiceAccount name (e.g., `devmesh-rest-api`)

### Step 2: Attach Bedrock Policy to Role

Attach the Bedrock IAM policy (from above) to this role:

```bash
aws iam attach-role-policy \
  --role-name devmesh-bedrock-role \
  --policy-arn arn:aws:iam::ACCOUNT_ID:policy/devmesh-bedrock-policy
```

Or use inline policy:

```bash
aws iam put-role-policy \
  --role-name devmesh-bedrock-role \
  --policy-name BedrockAccess \
  --policy-document file://bedrock-policy.json
```

### Step 3: Create Kubernetes ServiceAccount

Create a ServiceAccount with the IAM role annotation:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: devmesh-rest-api
  namespace: devmesh
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT_ID:role/devmesh-bedrock-role
```

### Step 4: Configure Deployment

Update your deployment to:
1. Use the ServiceAccount
2. Set the `BEDROCK_ROLE_ARN` environment variable

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: devmesh-rest-api
  namespace: devmesh
spec:
  template:
    spec:
      serviceAccountName: devmesh-rest-api
      containers:
      - name: rest-api
        image: devmesh/rest-api:latest
        env:
        - name: BEDROCK_ENABLED
          value: "true"
        - name: BEDROCK_REGION
          value: "us-east-1"
        - name: BEDROCK_ROLE_ARN
          value: "arn:aws:iam::ACCOUNT_ID:role/devmesh-bedrock-role"
        - name: AWS_REGION
          value: "us-east-1"
```

### Using eksctl (Automated Setup)

Create the role and ServiceAccount automatically with eksctl:

```bash
eksctl create iamserviceaccount \
  --name devmesh-rest-api \
  --namespace devmesh \
  --cluster your-cluster-name \
  --region us-east-1 \
  --attach-policy-arn arn:aws:iam::ACCOUNT_ID:policy/devmesh-bedrock-policy \
  --approve \
  --override-existing-serviceaccounts
```

## Testing Permissions

### Verify IAM Role Assumption

Check if the pod can assume the IAM role:

```bash
# Exec into the pod
kubectl exec -it deployment/devmesh-rest-api -n devmesh -- sh

# Check AWS identity
aws sts get-caller-identity

# Expected output shows the assumed role:
# {
#   "UserId": "AROA...:botocore-session-...",
#   "Account": "ACCOUNT_ID",
#   "Arn": "arn:aws:sts::ACCOUNT_ID:assumed-role/devmesh-bedrock-role/..."
# }
```

### Test Bedrock Access

Test Bedrock access from within the pod:

```bash
# List foundation models (requires additional permission if you want to enable this)
aws bedrock list-foundation-models --region us-east-1

# Invoke a model (requires InvokeModel permission)
aws bedrock-runtime invoke-model \
  --model-id amazon.titan-embed-text-v2:0 \
  --region us-east-1 \
  --body '{"inputText":"test"}' \
  output.json
```

### Check Application Logs

View logs for authentication issues:

```bash
kubectl logs -f deployment/devmesh-rest-api -n devmesh

# Look for:
# - "IRSA (IAM Roles for Service Accounts) is enabled"
# - "Assuming IAM role: arn:aws:iam::..."
# - "Bedrock provider initialized"
```

## Troubleshooting

### Common Issues

#### 1. AccessDeniedException

**Error**: `AccessDeniedException: User: arn:aws:sts::ACCOUNT:assumed-role/... is not authorized to perform: bedrock:InvokeModel`

**Solution**:
- Verify the IAM policy is attached to the role
- Check the resource ARNs match your model IDs
- Ensure the model is available in your region

#### 2. AssumeRole Failed

**Error**: `failed to load AWS config: operation error STS: AssumeRole`

**Solution**:
- Verify the trust policy allows your ServiceAccount
- Check OIDC provider is correctly configured
- Ensure the ServiceAccount annotation is correct

#### 3. Model Not Found

**Error**: `ValidationException: The provided model identifier is invalid`

**Solution**:
- Verify model availability in your region
- Check model ID spelling (case-sensitive)
- Review [AWS Bedrock Models by Region](https://docs.aws.amazon.com/bedrock/latest/userguide/models-regions.html)

#### 4. IRSA Not Detected

**Logs**: `IRSA not detected, will use standard AWS credential provider chain`

**Solution**:
- Verify ServiceAccount annotation exists
- Check OIDC provider is configured on the cluster
- Ensure AWS_WEB_IDENTITY_TOKEN_FILE and AWS_ROLE_ARN env vars are injected

### Debug Commands

```bash
# Check ServiceAccount annotations
kubectl get sa devmesh-rest-api -n devmesh -o yaml

# Check pod environment variables
kubectl exec deployment/devmesh-rest-api -n devmesh -- env | grep AWS

# Describe pod for events
kubectl describe pod -l app=devmesh-rest-api -n devmesh

# Check IAM role trust policy
aws iam get-role --role-name devmesh-bedrock-role --query 'Role.AssumeRolePolicyDocument'

# Check attached policies
aws iam list-attached-role-policies --role-name devmesh-bedrock-role
```

## Security Best Practices

1. **Principle of Least Privilege**: Only grant permissions for models you actually use
2. **Regional Restrictions**: Restrict to specific regions in production
3. **Audit Logging**: Enable CloudTrail logging for Bedrock API calls
4. **Role Separation**: Use separate IAM roles for different services (REST API, Worker, etc.)
5. **Cost Monitoring**: Set up CloudWatch alarms for unexpected Bedrock usage

## Additional Resources

- [AWS Bedrock Documentation](https://docs.aws.amazon.com/bedrock/)
- [EKS IRSA Documentation](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
- [AWS IAM Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
- [Developer Mesh Production Deployment Guide](../deployment/production-aws-usage.md)

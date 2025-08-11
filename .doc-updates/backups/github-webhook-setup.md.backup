# GitHub Organization Webhook Setup Guide

## Overview

This guide walks through setting up an organization-level webhook for the developer-mesh GitHub organization to integrate with the production Developer Mesh deployment.

## Prerequisites

- Admin access to the developer-mesh GitHub organization
- Access to GitHub repository secrets

## Webhook Configuration

### 1. Generate Webhook Secret

Generate a secure webhook secret using:
```bash
openssl rand -hex 32
```

Example output (DO NOT USE THIS - generate your own):
```
your-generated-webhook-secret-here-1234567890abcdef
```

### 2. Add Secret to GitHub Repository

1. Go to: https://github.com/developer-mesh/developer-mesh/settings/secrets/actions
2. Click "New repository secret"
3. Add the following secret:
   - **Name**: `MCP_WEBHOOK_SECRET`
   - **Value**: `<your-generated-webhook-secret>`

### 3. Configure Organization Webhook

1. Navigate to: https://github.com/organizations/developer-mesh/settings/hooks
2. Click "Add webhook"
3. Configure with these settings:

   **Payload URL**: 
   ```
   https://api.dev-mesh.io/api/webhooks/github
   ```

   **Content type**: 
   ```
   application/json
   ```

   **Secret**: 
   ```
   <your-generated-webhook-secret>
   ```

   **SSL verification**: 
   - ✅ Enable SSL verification (recommended)

   **Which events would you like to trigger this webhook?**
   - Select "Let me select individual events"
   - Check these events:
     - ✅ Issues
     - ✅ Issue comments
     - ✅ Pull requests
     - ✅ Pushes
     - ✅ Releases

   **Active**: 
   - ✅ Check this box

4. Click "Add webhook"

## Deployment

After adding the GitHub secret, deploy the updated configuration:

```bash
# Trigger deployment workflow
gh workflow run deploy-production-v2.yml

# Monitor deployment
gh run watch
```

## Verification

Once deployed, verify the webhook is working:

1. **Check webhook delivery in GitHub**:
   - Go to the webhook settings
   - Look for "Recent Deliveries" section
   - Green checkmark = successful delivery

2. **Test with a simple action**:
   - Create a test issue in any repository
   - Check webhook deliveries for success

3. **Monitor application logs**:
   ```bash
   # SSH to production
   ./scripts/ssh-to-ec2.sh

   # Check REST API logs
   docker-compose logs -f rest-api | grep webhook
   ```

## Webhook Security Features

The implementation includes:

1. **HMAC Signature Validation**: All requests are validated using SHA-256 HMAC
2. **IP Validation**: Optional validation against GitHub's IP ranges
3. **Event Type Filtering**: Only configured events are processed
4. **Async Processing**: Events are queued to SQS for reliable processing

## Troubleshooting

### Webhook returns 401 Unauthorized
- Verify the secret matches exactly
- Check that `GITHUB_WEBHOOK_SECRET` is deployed

### Webhook returns 400 Bad Request
- Check the payload format
- Verify event type is allowed

### No webhook deliveries showing
- Ensure webhook is marked as "Active"
- Check organization permissions

## Environment Variables

The following environment variables are configured in production:

- `GITHUB_WEBHOOK_SECRET`: The webhook secret (used for both webhook validation and GitHub adapter)
- `MCP_WEBHOOK_ENABLED`: Set to `true`
- `MCP_GITHUB_IP_VALIDATION`: Set to `true` for additional security
- `MCP_GITHUB_ALLOWED_EVENTS`: Comma-separated list of allowed events

## Related Documentation

- [Webhook API Reference](/docs/api-reference/webhook-api-reference.md)
- [Production Deployment Guide](/docs/guides/production-deployment.md)
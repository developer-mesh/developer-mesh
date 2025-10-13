# Webhook Setup Guide

## Overview
The package knowledge system **requires webhooks to be configured** in both GitHub and Artifactory. Without these webhooks, the system cannot capture release events or package deployments.

## Prerequisites
- Admin access to GitHub repositories (or organization)
- Admin access to JFrog Artifactory
- Public endpoint for webhook delivery (or use ngrok for testing)
- Webhook secrets for security

## Part 1: GitHub Webhook Setup

### Option A: Repository-Level Webhook (Per Repository)

1. **Navigate to Repository Settings**
   ```
   https://github.com/{owner}/{repo}/settings/hooks
   ```

2. **Click "Add webhook"**

3. **Configure Webhook Settings**
   ```
   Payload URL: https://your-domain.com/webhooks/github
   Content type: application/json
   Secret: your-github-webhook-secret
   ```

4. **Select Events**
   - Select "Let me select individual events"
   - Check these events:
     - ✅ Releases (REQUIRED)
     - ✅ Packages (if using GitHub Packages)
     - ✅ Push (optional - for tag tracking)
     - ✅ Registry packages (if using GitHub Container Registry)

5. **Save the webhook**

### Option B: Organization-Level Webhook (All Repositories)

1. **Navigate to Organization Settings**
   ```
   https://github.com/organizations/{org}/settings/hooks
   ```

2. **Click "Add webhook"**

3. **Configure Webhook Settings**
   ```
   Payload URL: https://your-domain.com/webhooks/github
   Content type: application/json
   Secret: your-github-webhook-secret
   ```

4. **Select Events** (same as repository level)

5. **Choose Repository Access**
   - "All repositories" - Recommended for full coverage
   - "Selected repositories" - If you only want specific repos

### GitHub Webhook Validation

Test the webhook is working:

```bash
# GitHub sends a ping event when you create the webhook
# Check your logs for the ping event
docker-compose logs -f rest-api | grep -i ping

# Manually trigger a test
# Create a release in GitHub and check logs
docker-compose logs -f worker | grep -i release
```

### GitHub Webhook Payload Example

When a release is published, GitHub sends:

```json
{
  "action": "published",
  "release": {
    "id": 123456789,
    "tag_name": "v1.2.3",
    "name": "Release 1.2.3",
    "body": "## What's New\n- Feature A\n- Feature B\n\n## Breaking Changes\n- API change X",
    "draft": false,
    "prerelease": false,
    "created_at": "2024-01-15T10:00:00Z",
    "published_at": "2024-01-15T10:05:00Z",
    "author": {
      "login": "developer"
    },
    "assets": [
      {
        "name": "package-1.2.3.jar",
        "content_type": "application/java-archive",
        "size": 1024000,
        "download_url": "https://github.com/..."
      }
    ]
  },
  "repository": {
    "id": 987654321,
    "name": "my-package",
    "full_name": "org/my-package",
    "private": true
  }
}
```

## Part 2: JFrog Artifactory Webhook Setup

### Step 1: Access Artifactory Admin

1. **Login to Artifactory**
   ```
   https://artifactory.company.com/ui/admin/
   ```

2. **Navigate to Webhooks**
   ```
   Admin → General → Webhooks
   ```

### Step 2: Create New Webhook

1. **Click "New Webhook"**

2. **Configure Basic Settings**
   ```
   Name: devmesh-package-tracker
   URL: https://your-domain.com/webhooks/artifactory
   Description: Sends package events to DevMesh
   ```

3. **Set Authentication** (if required)
   ```
   Custom HTTP Headers:
   X-Artifactory-Token: your-artifactory-webhook-token
   ```

### Step 3: Configure Event Types

Select the following events:

- ✅ **Artifact Deployed** (REQUIRED)
- ✅ **Artifact Deleted** (optional)
- ✅ **Build Promoted** (recommended)
- ✅ **Docker Tag Pushed** (if using Docker)

### Step 4: Configure Repositories

**Include Patterns:**
```
# All repositories
**/*

# Or specific patterns:
libs-release-local/**/*
libs-snapshot-local/**/*
docker-local/**/*
npm-local/**/*
```

**Exclude Patterns:**
```
# Exclude metadata files
**/*.sha1
**/*.sha256
**/*.md5
**/maven-metadata.xml

# Exclude temporary files
**/.tmp/**
**/_uploads/**
```

### Step 5: Test Configuration

1. **Use Test Button**
   - Artifactory has a "Test" button that sends a sample payload

2. **Verify Delivery**
   ```bash
   # Check logs for Artifactory webhook
   docker-compose logs -f rest-api | grep -i artifactory
   ```

### Artifactory Webhook Payload Example

When an artifact is deployed, Artifactory sends:

```json
{
  "domain": "artifact",
  "event_type": "deployed",
  "data": {
    "repo_path": "libs-release-local/com/company/my-service/1.2.3/my-service-1.2.3.jar",
    "repo_key": "libs-release-local",
    "path": "/com/company/my-service/1.2.3/my-service-1.2.3.jar",
    "name": "my-service-1.2.3.jar",
    "size": 15234567,
    "created": 1705315200000,
    "created_by": "jenkins",
    "modified": 1705315200000,
    "modified_by": "jenkins",
    "properties": {
      "build.name": ["my-service"],
      "build.number": ["123"],
      "vcs.revision": ["abc123def"],
      "version": ["1.2.3"]
    }
  }
}
```

## Part 3: Security Configuration

### GitHub Webhook Secret

1. **Generate a secure secret**
   ```bash
   openssl rand -hex 32
   # Output: 7d38b5c6a8e9f0123456789abcdef0123456789abcdef0123456789abcdef012
   ```

2. **Configure in GitHub webhook settings**

3. **Add to environment**
   ```bash
   # .env file
   GITHUB_WEBHOOK_SECRET=7d38b5c6a8e9f0123456789abcdef0123456789abcdef0123456789abcdef012
   ```

### Artifactory Webhook Token

1. **Generate a token**
   ```bash
   uuidgen | tr -d '-'
   # Output: a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
   ```

2. **Configure in Artifactory webhook headers**

3. **Add to environment**
   ```bash
   # .env file
   ARTIFACTORY_WEBHOOK_TOKEN=a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
   ```

## Part 4: Local Development Setup (Using ngrok)

For local development, use ngrok to expose your local endpoint:

### Step 1: Install ngrok
```bash
# macOS
brew install ngrok

# or download from https://ngrok.com/download
```

### Step 2: Start ngrok
```bash
# Expose your local REST API
ngrok http 8081

# Output:
# Forwarding https://abc123.ngrok.io -> http://localhost:8081
```

### Step 3: Configure Webhooks with ngrok URL
```
GitHub Webhook URL: https://abc123.ngrok.io/webhooks/github
Artifactory Webhook URL: https://abc123.ngrok.io/webhooks/artifactory
```

### Step 4: Monitor ngrok
```bash
# ngrok provides a web interface
open http://127.0.0.1:4040

# You can see all incoming requests and replay them
```

## Part 5: Troubleshooting

### Issue: GitHub Webhook Not Delivering

1. **Check Recent Deliveries**
   - Go to webhook settings
   - Click on the webhook
   - Check "Recent Deliveries" tab
   - Look for failed deliveries (red X)

2. **Common Issues**
   - SSL certificate problems
   - Firewall blocking GitHub IPs
   - Incorrect secret
   - Wrong content type

3. **GitHub IP Ranges**
   ```bash
   # GitHub publishes their webhook IPs
   curl https://api.github.com/meta | jq .hooks
   ```

### Issue: Artifactory Webhook Not Delivering

1. **Check Artifactory Logs**
   ```bash
   # In Artifactory
   Admin → System Logs → artifactory-request.log
   ```

2. **Test with curl**
   ```bash
   # Test your endpoint manually
   curl -X POST https://your-domain.com/webhooks/artifactory \
     -H "Content-Type: application/json" \
     -H "X-Artifactory-Token: your-token" \
     -d '{"event_type":"deployed","data":{}}'
   ```

### Issue: Events Not Processing

1. **Check REST API received the webhook**
   ```bash
   docker-compose logs rest-api | grep -i webhook
   ```

2. **Check Redis queue**
   ```bash
   # See if events are in queue
   redis-cli xlen webhook-events

   # Read events from stream
   redis-cli xread COUNT 10 STREAMS webhook-events 0
   ```

3. **Check Worker processing**
   ```bash
   docker-compose logs worker | grep -i "processing"
   ```

## Part 6: Monitoring Webhook Health

### Create Health Check Endpoint

```go
// GET /webhooks/health
{
  "github": {
    "last_received": "2024-01-15T10:30:00Z",
    "total_received": 1234,
    "last_processed": "2024-01-15T10:29:58Z",
    "pending": 2
  },
  "artifactory": {
    "last_received": "2024-01-15T10:25:00Z",
    "total_received": 567,
    "last_processed": "2024-01-15T10:24:55Z",
    "pending": 1
  }
}
```

### Set Up Alerts

```yaml
# Example Prometheus alert
- alert: WebhookNotReceiving
  expr: time() - webhook_last_received_timestamp > 3600
  annotations:
    summary: "No webhooks received in last hour"
```

## Part 7: Webhook Management Best Practices

### 1. **Version Your Webhooks**
```json
{
  "headers": {
    "X-Webhook-Version": "v1"
  }
}
```

### 2. **Implement Retry Logic**
- GitHub retries failed webhooks automatically
- Artifactory may need manual configuration
- Implement idempotency to handle retries

### 3. **Log All Webhook Events**
```go
// Log raw payload for debugging
logger.Info("Webhook received", map[string]interface{}{
    "source": "github",
    "event_type": eventType,
    "delivery_id": deliveryID,
    "payload_size": len(payload),
})
```

### 4. **Rate Limiting**
- GitHub: No rate limits on webhooks
- Artifactory: Check your instance's limits
- Implement queue-based processing to handle bursts

### 5. **Backup Webhook Configuration**
```bash
# Export GitHub webhook config
gh api repos/{owner}/{repo}/hooks > github-webhooks-backup.json

# Document Artifactory webhook settings
# Take screenshots or export via REST API
```

## Verification Checklist

After setup, verify everything works:

- [ ] GitHub webhook shows green checkmark in Recent Deliveries
- [ ] Artifactory webhook test succeeds
- [ ] REST API logs show webhook received
- [ ] Redis queue shows events
- [ ] Worker processes events
- [ ] Database contains package releases
- [ ] Context search returns packages

## Without Webhooks - Alternative Approaches

If webhooks cannot be configured, consider these alternatives:

### 1. **Polling Approach**
```go
// Periodically check for new releases
// Less efficient, higher latency
func PollGitHubReleases() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        releases := githubClient.GetLatestReleases()
        // Process new releases
    }
}
```

### 2. **Manual Trigger**
```bash
# API endpoint to manually register a release
POST /api/v1/packages/register
{
  "repository": "org/repo",
  "version": "v1.2.3",
  "artifactory_path": "libs-release/..."
}
```

### 3. **CI/CD Integration**
```yaml
# In Jenkins/GitHub Actions
- name: Notify DevMesh
  run: |
    curl -X POST $DEVMESH_URL/api/v1/releases \
      -d '{"version":"$VERSION","package":"$PACKAGE"}'
```

**However, webhooks are strongly recommended for:**
- Real-time updates
- Lower latency
- Reduced API calls
- Better scalability
- Automatic capture of all events

## Summary

**The system will NOT automatically capture package information without webhooks.** You must:

1. Configure GitHub webhook for release events
2. Configure Artifactory webhook for deployment events
3. Ensure webhooks can reach your endpoints
4. Verify webhook delivery and processing

Only after webhooks are properly configured will the system begin capturing and indexing package knowledge for AI assistants.
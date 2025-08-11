<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:39:51
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# GitHub Integration Testing Guide

This guide explains how to run integration and functional tests against the real GitHub API instead of mock servers.

## Prerequisites

1. **GitHub App Creation**
   - Create a GitHub App in your test organization
   - Generate a private key for the app
   - Install the app on your test repository
   - Note down the App ID and download the private key

2. **Personal Access Token (Alternative)**
   - If not using a GitHub App, create a Personal Access Token
   - Grant necessary permissions (repo, read:org, etc.)

3. **Test Repository**
   - Create a test repository in your organization
   - The tests will use this repository for API calls

## Configuration

### 1. Environment Variables

Create or update `.env.test` with your GitHub credentials:

```bash
# GitHub Authentication (choose one method)
# Option 1: Personal Access Token
GITHUB_TOKEN=your-github-token-here

# Option 2: GitHub App
GITHUB_APP_ID=your-app-id
GITHUB_APP_PRIVATE_KEY_PATH=/path/to/private-key.pem

# Common settings
GITHUB_WEBHOOK_SECRET=your-webhook-secret
GITHUB_TEST_ORG=your-test-org
GITHUB_TEST_REPO=your-test-repo
GITHUB_TEST_USER=your-username

# Disable mock server
USE_GITHUB_MOCK=false
GITHUB_API_URL=https://api.github.com
```

### 2. Load Environment

```bash
# Load environment variables
source .env.test

# Or export them individually
export GITHUB_TOKEN="your-token"
export GITHUB_TEST_ORG="your-org"
export GITHUB_TEST_REPO="your-repo"
export USE_GITHUB_MOCK=false
```

## Running Tests

### Integration Tests

1. **Run all integration tests with real GitHub:**
   ```bash
   # Using the provided script
   ./scripts/test-github-integration.sh
   
   # Or manually
   USE_GITHUB_MOCK=false go test -v -tags=integration ./pkg/tests/integration -run TestGitHub
   ```

2. **Run specific GitHub tests:**
   ```bash
   # Real API tests only
   go test -v -tags="integration,github_real" ./test/integration -run TestGitHubRealAPI
   
   # Enhanced tests (work with both mock and real)
   USE_GITHUB_MOCK=false go test -v -tags=integration ./pkg/tests/integration -run TestGitHubIntegrationEnhanced
   ```

### Functional Tests

1. **Start services with GitHub integration config:**
   ```bash
   # Use the GitHub integration configuration
   CONFIG_FILE=configs/config.github-integration.yaml make docker-compose-up
   ```

2. **Run functional tests:**
   ```bash
   # In another terminal
   cd test/functional
   USE_GITHUB_MOCK=false go test -v ./...
   ```

3. **Or use the all-in-one script:**
   ```bash
   ./scripts/test-github-integration.sh --with-functional
   ```

## Test Coverage

The integration tests cover:

- **Authentication**: Token and GitHub App authentication
- **Repository Operations**: List, get, create, update repositories
- **Pull Requests**: List, get, create, review pull requests
- **Issues**: List, get, create, update issues
- **Webhooks**: Signature validation and event processing
- **Rate Limiting**: Ensures proper rate limit handling
- **Error Handling**: API errors and edge cases

## Troubleshooting

### Common Issues

1. **401 Unauthorized**
   - Check your GitHub token is valid
   - Ensure token has required permissions
   - For GitHub Apps, verify installation

2. **404 Not Found**
   - Verify test organization and repository exist
   - Check you have access to the repository
   - Ensure correct spelling of org/repo names

3. **Rate Limit Exceeded**
   - GitHub API has rate limits (5000 req/hour for authenticated)
   - Tests include rate limiting logic
   - Wait or use a different token

4. **Test Timeouts**
   - Real API calls take longer than mocks
   - Increase test timeout: `go test -timeout 10m ...`

### Debug Mode

Enable detailed logging:

```bash
# Set log level
export LOG_LEVEL=debug

# Run with verbose output
go test -v -tags=integration ./... 2>&1 | tee test.log
```

## Safety Considerations

1. **Use a Dedicated Test Organization**
   - Never run tests against production repositories
   - Create a separate org for testing

2. **Clean Up Test Data**
   - Tests should clean up created resources
   - Periodically check for orphaned test data

3. **Rate Limiting**
   - Be mindful of GitHub's rate limits
   - Don't run tests in tight loops

4. **Secrets Management**
   - Never commit credentials to git
   - Use environment variables or secret managers
   - Rotate tokens regularly

## Switching Between Mock and Real API

The test suite supports both modes:

```bash
# Use mock server (default)
USE_GITHUB_MOCK=true go test ...

# Use real GitHub API
USE_GITHUB_MOCK=false go test ...
```

This allows you to:
- Develop quickly with mocks
- Validate against real API before deployment

<!-- SOURCE VERIFICATION
Last Verified: 2025-08-14
Manual Review: Verified against actual test files
Notes:
- test/github-live/github_live_test.go exists with github_live build tag
- test/github-live/verify_installation_test.go exists
- scripts/test-github-integration.sh exists and works
- Integration tests use GitHub App authentication
-->

# GitHub Integration Testing Guide

✅ **VERIFIED**: These tests and scripts actually exist and work as documented.

This guide explains how to run integration tests against the real GitHub API.

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

### Actual Test Files

1. **GitHub Live Tests** (`test/github-live/`):
   ```bash
   # These require the github_live build tag
   USE_GITHUB_MOCK=false go test -tags=github_live ./test/github-live -v
   
   # Using the provided script
   ./scripts/test-github-integration.sh
   ```

2. **Available Test Files**:
   - `test/github-live/github_live_test.go` - Main GitHub API tests
   - `test/github-live/verify_installation_test.go` - Installation verification
   - `test/integration/github_real_test.go` - Real API integration tests
   - `test/integration/ide_agent_github_test.go` - IDE agent GitHub tests

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

3. **Additional GitHub Test Scripts**:
   ```bash
   # Main integration test script
   ./scripts/test-github-integration.sh
   
   # GitHub webhook testing
   ./scripts/test-github-webhook.sh
   
   # IDE GitHub integration
   ./scripts/test-ide-github-integration.sh
   
   # Agent GitHub MCP testing
   ./scripts/test-agent-github-mcp.sh
   ```

## Actual Test Coverage

Based on `test/github-live/github_live_test.go`, the tests actually cover:

- **Authentication**: GitHub App authentication (NOT token auth)
- **Adapter Info**: Basic adapter functionality checks
- **Repository Operations**: Get repository info
- **Pull Requests**: List and get PR operations
- **Issues**: List and get issue operations  
- **Installation Verification**: App installation checks
- **Health Checks**: Adapter health verification

**Note**: Tests use GitHub App authentication, not Personal Access Tokens

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

## Test Implementation Details

### Required Build Tags
- `github_live` - For live GitHub API tests
- `integration` - For integration tests

### Environment Control
```bash
# Skip real API tests (default)
USE_GITHUB_MOCK=true go test ...

# Enable real API tests
USE_GITHUB_MOCK=false go test -tags=github_live ...
```

### Test Adapters
Tests use the `github.com/developer-mesh/developer-mesh/pkg/adapters/github` package for GitHub operations, not direct API calls.

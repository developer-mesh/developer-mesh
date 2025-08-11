# GitHub App Setup for Integration Testing

## Finding Your GitHub App Installation ID

After installing your GitHub App on a repository, you need the installation ID for authentication.

### Method 1: Via GitHub UI
1. Go to your GitHub App settings: https://github.com/settings/apps/YOUR_APP_NAME
2. Click on "Install App" in the sidebar
3. You'll see your installations listed
4. Click on the installation (e.g., your username or organization)
5. Look at the URL - it will be something like:
   `https://github.com/settings/installations/12345678`
   The number at the end (12345678) is your installation ID

### Method 2: Via API (if you have a Personal Access Token)
```bash
curl -H "Authorization: Bearer YOUR_PAT" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/app/installations
```

### Method 3: Via GitHub CLI
```bash
gh api /app/installations --jq '.[].id'
```

## Update Your .env.test

Once you have the installation ID, add it to your `.env.test`:

```bash
GITHUB_APP_INSTALLATION_ID=12345678
```

## Common Issues

### "Not Found (INVALID_CREDENTIALS)"
This error typically means:
1. The App ID is incorrect
2. The private key doesn't match the App
3. The App isn't installed on the repository
4. The installation ID is missing or incorrect

### Verifying Your Setup
1. Check that your App ID matches what's shown in GitHub
2. Ensure the private key file was downloaded from the same App
3. Verify the App is installed on your test repository
4. Confirm the installation ID is correct

## Testing Authentication
You can test your GitHub App authentication with:

```bash
# Using the test script
./scripts/test-github-integration.sh

# Or directly
cd test/github-live
go test -v -tags=github_live -run TestGitHubLiveAPI
```
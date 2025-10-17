# GitHub Tools Comprehensive Test Plan

## Overview
This document provides a comprehensive test plan for all 114 GitHub tools in the DevMesh platform.

## Tool Categories and Testing Strategy

### 1. Repository Operations (11 tools)
**Tools:**
- `get-repository` - Get repository details
- `list-repositories` - List repositories
- `create-repository` - Create new repository
- `update-repository` - Update repository settings
- `delete-repository` - Delete repository
- `fork-repository` - Fork a repository
- `watch-repository` - Watch a repository
- `unwatch-repository` - Unwatch a repository
- `repository-details-graphql` - Get detailed repository info via GraphQL
- `search-repositories` - Search for repositories
- `get-file-contents` - Get file contents from repository

**Test Strategy:**
1. Create a test repository
2. Update its settings (description, topics, visibility)
3. Fork the repository
4. Watch/unwatch operations
5. Search for the repository
6. Get file contents
7. Delete the test repository

**Required Test Data:**
- GitHub organization or user account
- Test repository name pattern: `devmesh-test-{timestamp}`

---

### 2. Issue Management (11 tools)
**Tools:**
- `create-issue` - Create new issue
- `get-issue` - Get issue details
- `update-issue` - Update issue
- `list-issues` - List issues
- `search-issues` - Search issues
- `lock-issue` - Lock issue for comments
- `unlock-issue` - Unlock issue
- `add-issue-comment` - Add comment to issue
- `get-issue-comments` - Get issue comments
- `get-issue-events` - Get issue events
- `get-issue-timeline` - Get issue timeline
- `issue-create-graphql` - Create issue via GraphQL
- `issues-list-graphql` - List issues via GraphQL

**Test Strategy:**
1. Create test issues with different labels and assignees
2. Update issue state (open/closed)
3. Add comments
4. Lock/unlock for testing
5. Search with various filters
6. Verify timeline and events

---

### 3. Pull Request Operations (14 tools)
**Tools:**
- `create-pull-request` - Create PR
- `get-pull-request` - Get PR details
- `update-pull-request` - Update PR
- `list-pull-requests` - List PRs
- `search-pull-requests` - Search PRs
- `merge-pull-request` - Merge PR
- `get-pull-request-diff` - Get PR diff
- `get-pull-request-files` - Get PR files
- `get-pull-request-reviews` - Get PR reviews
- `get-pull-request-review-comments` - Get review comments
- `add-pull-request-review-comment` - Add review comment
- `create-pull-request-review` - Create PR review
- `submit-pull-request-review` - Submit PR review
- `update-pull-request-branch` - Update PR branch
- `pull-request-create-graphql` - Create PR via GraphQL
- `pull-request-merge-graphql` - Merge PR via GraphQL
- `pull-request-review-add-graphql` - Add review via GraphQL

**Test Strategy:**
1. Create feature branch with changes
2. Create pull request
3. Add review comments
4. Request changes/approve
5. Update PR branch
6. Merge PR
7. Verify merge status

---

### 4. GitHub Actions & Workflows (11 tools)
**Tools:**
- `list-workflows` - List workflows
- `get-workflow-run` - Get workflow run details
- `list-workflow-runs` - List workflow runs
- `list-workflow-jobs` - List workflow jobs
- `run-workflow` - Trigger workflow run
- `cancel-workflow-run` - Cancel workflow run
- `rerun-workflow-run` - Rerun workflow
- `rerun-failed-jobs` - Rerun failed jobs
- `get-workflow-run-logs` - Get workflow logs
- `delete-workflow-run-logs` - Delete workflow logs
- `get-workflow-run-usage` - Get workflow usage
- `get-job-logs` - Get job logs
- `list-artifacts` - List workflow artifacts
- `download-artifact` - Download artifact

**Test Strategy:**
1. List available workflows
2. Trigger a test workflow
3. Monitor run status
4. Get logs and artifacts
5. Test rerun capabilities
6. Clean up logs

---

### 5. Security Operations (10 tools)
**Tools:**
- `list-code-scanning-alerts` - List code scanning alerts
- `get-code-scanning-alert` - Get specific alert
- `update-code-scanning-alert` - Update alert status
- `list-dependabot-alerts` - List Dependabot alerts
- `get-dependabot-alert` - Get Dependabot alert
- `update-dependabot-alert` - Update Dependabot alert
- `list-secret-scanning-alerts` - List secret scanning alerts
- `get-secret-scanning-alert` - Get secret alert
- `update-secret-scanning-alert` - Update secret alert
- `list-secret-scanning-locations` - List secret locations
- `list-security-advisories` - List security advisories
- `list-global-security-advisories` - List global advisories

**Test Strategy:**
1. Enable security features on test repo
2. List all security alerts
3. Update alert states
4. Verify remediation tracking

---

### 6. Git Operations (13 tools)
**Tools:**
- `get-commit` - Get commit details
- `list-commits` - List commits
- `create-commit` - Create commit
- `get-git-commit` - Get git commit object
- `create-blob` - Create blob
- `get-blob` - Get blob
- `create-tree` - Create tree
- `get-tree` - Get tree
- `create-ref` - Create reference
- `get-ref` - Get reference
- `update-ref` - Update reference
- `delete-ref` - Delete reference
- `list-refs` - List references
- `push-files` - Push multiple files

**Test Strategy:**
1. Create test branch
2. Create blobs and trees
3. Create commits programmatically
4. Update references
5. Push files
6. Verify commit history

---

### 7. Branch & Tag Management (7 tools)
**Tools:**
- `list-branches` - List branches
- `create-branch` - Create branch
- `get-tag` - Get tag details
- `list-tags` - List tags
- `create-release` - Create release
- `list-releases` - List releases
- `get-latest-release` - Get latest release
- `get-release-by-tag` - Get release by tag

**Test Strategy:**
1. Create feature branches
2. Create tags on commits
3. Create releases from tags
4. Verify release assets
5. Clean up test branches

---

### 8. File Operations (3 tools)
**Tools:**
- `create-or-update-file` - Create/update file
- `delete-file` - Delete file
- `get-file-contents` - Get file contents

**Test Strategy:**
1. Create test files
2. Update file contents
3. Delete files
4. Verify operations via API

---

### 9. Organization & Team Management (6 tools)
**Tools:**
- `get-organization` - Get org details
- `list-organizations` - List organizations
- `search-organizations` - Search organizations
- `get-teams` - Get teams
- `list-teams` - List teams
- `get-team-members` - Get team members

**Test Strategy:**
1. List user organizations
2. Get organization details
3. List teams and members
4. Verify permissions

---

### 10. User & Collaboration (9 tools)
**Tools:**
- `get-me` - Get authenticated user
- `search-users` - Search users
- `list-notifications` - List notifications
- `mark-notification-as-read` - Mark notification read
- `create-gist` - Create gist
- `get-gist` - Get gist
- `update-gist` - Update gist
- `delete-gist` - Delete gist
- `list-gists` - List gists
- `star-gist` - Star gist
- `unstar-gist` - Unstar gist

**Test Strategy:**
1. Get current user info
2. Create test gists
3. Star/unstar gists
4. Manage notifications
5. Clean up test gists

---

### 11. Search Operations (5 tools)
**Tools:**
- `search-code` - Search code
- `search-issues` - Search issues
- `search-pull-requests` - Search PRs
- `search-repositories` - Search repos
- `search-users` - Search users
- `search-organizations` - Search orgs
- `search-issues-prs-graphql` - Search via GraphQL

**Test Strategy:**
1. Search with various filters
2. Test pagination
3. Verify result accuracy
4. Test GraphQL vs REST

---

### 12. Discussions (4 tools)
**Tools:**
- `discussions-list` - List discussions
- `discussion-get` - Get discussion
- `discussion-comments-get` - Get discussion comments
- `discussion-categories-list` - List categories

**Test Strategy:**
1. List discussion categories
2. Get discussions in each category
3. Read discussion comments
4. Verify GraphQL responses

---

## Test Execution Plan

### Phase 1: Read-Only Operations (Safe Testing)
Test all GET/LIST operations that don't modify data:
1. Repository reading (get, list, search)
2. Issue reading (get, list, search)
3. Pull request reading
4. Workflow status checking
5. Security alert listing
6. Organization/team reading

### Phase 2: Isolated Write Operations
Test CREATE operations in isolated test repositories:
1. Create test repository
2. Create issues
3. Create branches
4. Create files
5. Create gists

### Phase 3: Modification Operations
Test UPDATE operations on test data:
1. Update issues
2. Update pull requests
3. Update files
4. Update repository settings

### Phase 4: Workflow Operations
Test GitHub Actions operations:
1. Trigger workflows
2. Monitor runs
3. Download artifacts
4. Rerun workflows

### Phase 5: Cleanup Operations
Test DELETE operations to clean up:
1. Delete test branches
2. Delete test files
3. Delete test gists
4. Delete test repository

## Test Data Requirements

### Required GitHub Resources:
- Personal access token with full scope
- Test organization (optional)
- Test repository prefix: `devmesh-test-`
- Test branch prefix: `test/`
- Test issue labels: `test`, `automated`

### Test Execution Environment:
```bash
export GITHUB_TOKEN="your-token"
export GITHUB_ORG="your-org"
export TEST_REPO="devmesh-test-$(date +%s)"
```

## Success Criteria

### Functional Requirements:
- All 114 tools execute without errors
- Correct parameter validation
- Proper error handling for invalid inputs
- Accurate response data

### Non-Functional Requirements:
- Response time < 5 seconds for standard operations
- Rate limiting handled gracefully
- Pagination works correctly
- Caching improves performance

## Risk Mitigation

### Safety Measures:
1. Use dedicated test repositories
2. Implement dry-run mode for destructive operations
3. Add confirmation prompts for deletions
4. Log all operations for audit trail
5. Implement rollback capabilities

### Rate Limiting Strategy:
1. Implement exponential backoff
2. Cache frequently accessed data
3. Batch operations where possible
4. Monitor API quota usage

## Automation Strategy

### Test Automation Framework:
```go
type GitHubToolTest struct {
    Tool        string
    Category    string
    Parameters  map[string]interface{}
    Expected    interface{}
    Cleanup     func()
}
```

### Continuous Testing:
1. Nightly test runs against test organization
2. Smoke tests on PR merges
3. Full regression weekly
4. Performance benchmarks monthly

## Reporting

### Test Report Format:
- Tool name and category
- Execution time
- Success/failure status
- Error messages (if any)
- API calls made
- Rate limit consumption

### Metrics to Track:
- Success rate per category
- Average execution time
- Error frequency
- Rate limit efficiency
- Cache hit ratio
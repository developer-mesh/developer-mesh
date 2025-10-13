# Phase 2 Testing Guide: Artifactory Integration

## Overview
This guide provides comprehensive testing procedures for the Phase 2 Artifactory webhook integration.

## Prerequisites

1. **Services Running:**
   ```bash
   docker-compose -f docker-compose.local.yml up -d
   ```

2. **Environment Variables (optional for testing):**
   ```bash
   export ARTIFACTORY_URL=https://artifactory.example.com
   export ARTIFACTORY_API_KEY=your-api-key-here
   ```

3. **Database Ready:**
   ```bash
   # Check migration status
   psql -h localhost -U devmesh -d devmesh_development -c \
     "SELECT version, dirty FROM mcp.schema_migrations ORDER BY version DESC LIMIT 1;"
   ```

## Test Scenarios

### Test 1: Maven Package Deployment

**Description:** Test Artifactory webhook for Maven artifact deployment

**Test Data:**
```json
{
  "domain": "artifact",
  "event_type": "deployed",
  "timestamp": 1701234567000,
  "data": {
    "repoPath": {
      "repoKey": "libs-release-local",
      "path": "com/example/test-app/1.0.0/test-app-1.0.0.jar"
    },
    "name": "test-app-1.0.0.jar",
    "path": "com/example/test-app/1.0.0/test-app-1.0.0.jar",
    "size": 1048576,
    "sha256": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
    "sha1": "1234567890abcdef1234567890abcdef12345678",
    "md5": "abcdef1234567890abcdef1234567890",
    "created": 1701234567000,
    "created_by": "jenkins-user",
    "modified": 1701234567000,
    "modified_by": "jenkins-user",
    "properties": {}
  }
}
```

**Send via Queue:**
```bash
# Using redis-cli
redis-cli XADD webhook_events * \
  event_id "test-maven-001" \
  event_type "artifactory.deployed" \
  payload '{"domain":"artifact","event_type":"deployed","timestamp":1701234567000,"data":{"repoPath":{"repoKey":"libs-release-local","path":"com/example/test-app/1.0.0/test-app-1.0.0.jar"},"name":"test-app-1.0.0.jar","path":"com/example/test-app/1.0.0/test-app-1.0.0.jar","size":1048576,"sha256":"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890","sha1":"1234567890abcdef1234567890abcdef12345678","md5":"abcdef1234567890abcdef1234567890","created":1701234567000,"created_by":"jenkins-user","modified":1701234567000,"modified_by":"jenkins-user"}}'
```

**Expected Results:**
1. Worker picks up event from queue
2. Parser extracts: `com.example:test-app:1.0.0`
3. Creates new package release record
4. Creates asset record with checksums
5. Package type: `maven`

**Verification:**
```sql
-- Check release was created
SELECT id, package_name, version, package_type, artifactory_path, metadata
FROM mcp.package_releases
WHERE package_name LIKE '%test-app%'
ORDER BY created_at DESC LIMIT 1;

-- Check asset was created
SELECT id, name, size_bytes, sha256_checksum, artifactory_url
FROM mcp.package_assets
WHERE release_id = (
  SELECT id FROM mcp.package_releases
  WHERE package_name LIKE '%test-app%'
  ORDER BY created_at DESC LIMIT 1
);
```

**Expected Output:**
```
Package Name: com.example:test-app (or test-app)
Version: 1.0.0
Package Type: maven
Artifactory Path: libs-release-local/com/example/test-app/1.0.0/test-app-1.0.0.jar
Metadata: Contains artifactory_repo, artifactory_deployed_by, maven_group_id, maven_artifact_id
```

### Test 2: NPM Package Deployment

**Test Data:**
```json
{
  "domain": "artifact",
  "event_type": "deployed",
  "timestamp": 1701234568000,
  "data": {
    "repoPath": {
      "repoKey": "npm-local",
      "path": "@myorg/my-package/-/my-package-2.1.5.tgz"
    },
    "name": "my-package-2.1.5.tgz",
    "path": "@myorg/my-package/-/my-package-2.1.5.tgz",
    "size": 524288,
    "sha256": "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
    "sha1": "fedcba0987654321fedcba0987654321fedcba09",
    "md5": "fedcba0987654321fedcba0987654321",
    "created": 1701234568000,
    "created_by": "npm-user",
    "modified": 1701234568000,
    "modified_by": "npm-user"
  }
}
```

**Send via Queue:**
```bash
redis-cli XADD webhook_events * \
  event_id "test-npm-001" \
  event_type "artifactory.deployed" \
  payload '{"domain":"artifact","event_type":"deployed","timestamp":1701234568000,"data":{"repoPath":{"repoKey":"npm-local","path":"@myorg/my-package/-/my-package-2.1.5.tgz"},"name":"my-package-2.1.5.tgz","path":"@myorg/my-package/-/my-package-2.1.5.tgz","size":524288,"sha256":"fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321","sha1":"fedcba0987654321fedcba0987654321fedcba09","md5":"fedcba0987654321fedcba0987654321","created":1701234568000,"created_by":"npm-user"}}'
```

**Expected Results:**
1. Parser extracts: `@myorg/my-package:2.1.5`
2. Creates package release
3. Package type: `npm`
4. Handles scoped package name correctly

**Verification:**
```sql
SELECT package_name, version, package_type
FROM mcp.package_releases
WHERE package_name LIKE '%my-package%'
ORDER BY created_at DESC LIMIT 1;
```

### Test 3: GitHub Release Matching

**Setup:**
First create a GitHub release, then deploy to Artifactory.

**Step 1: Create GitHub Release**
```bash
# Simulate GitHub release webhook
redis-cli XADD webhook_events * \
  event_id "test-github-release-001" \
  event_type "release" \
  payload '{"action":"published","release":{"id":12345,"tag_name":"v3.0.0","name":"Release 3.0.0","body":"Test release for matching","draft":false,"prerelease":false,"published_at":"2025-01-13T10:00:00Z","author":{"login":"developer"}},"repository":{"full_name":"myorg/matching-test","description":"Test repo","homepage":"https://example.com"}}'
```

**Step 2: Deploy to Artifactory**
```bash
redis-cli XADD webhook_events * \
  event_id "test-artifactory-match-001" \
  event_type "artifactory.deployed" \
  payload '{"domain":"artifact","event_type":"deployed","timestamp":1701234569000,"data":{"repoPath":{"repoKey":"libs-release-local","path":"com/myorg/matching-test/3.0.0/matching-test-3.0.0.jar"},"name":"matching-test-3.0.0.jar","path":"com/myorg/matching-test/3.0.0/matching-test-3.0.0.jar","size":2097152,"sha256":"123abc","sha1":"456def","md5":"789ghi","created_by":"jenkins"}}'
```

**Expected Results:**
1. GitHub release creates record with `github_release_id=12345`
2. Artifactory deployment matches to existing release
3. Release updated with `artifactory_path`
4. Both GitHub and Artifactory metadata present

**Verification:**
```sql
SELECT
  package_name,
  version,
  github_release_id,
  artifactory_path,
  metadata->'github_url' as github_url,
  metadata->'artifactory_repo' as artifactory_repo
FROM mcp.package_releases
WHERE package_name LIKE '%matching-test%'
ORDER BY created_at DESC LIMIT 1;
```

**Expected:** Single record with both GitHub and Artifactory data.

### Test 4: Python Package Deployment

**Test Data:**
```json
{
  "domain": "artifact",
  "event_type": "deployed",
  "timestamp": 1701234570000,
  "data": {
    "repoPath": {
      "repoKey": "pypi-local",
      "path": "my_python_lib-1.2.3-py3-none-any.whl"
    },
    "name": "my_python_lib-1.2.3-py3-none-any.whl",
    "path": "my_python_lib-1.2.3-py3-none-any.whl",
    "size": 131072,
    "sha256": "pythontest123",
    "created_by": "pip-user"
  }
}
```

**Send via Queue:**
```bash
redis-cli XADD webhook_events * \
  event_id "test-python-001" \
  event_type "artifactory.deployed" \
  payload '{"domain":"artifact","event_type":"deployed","timestamp":1701234570000,"data":{"repoPath":{"repoKey":"pypi-local","path":"my_python_lib-1.2.3-py3-none-any.whl"},"name":"my_python_lib-1.2.3-py3-none-any.whl","path":"my_python_lib-1.2.3-py3-none-any.whl","size":131072,"sha256":"pythontest123","created_by":"pip-user"}}'
```

**Expected Results:**
1. Parser extracts: `my_python_lib:1.2.3`
2. Package type: `python`
3. Version parsing handles Python wheel format

### Test 5: Error Handling

**Test 5a: Invalid JSON Payload**
```bash
redis-cli XADD webhook_events * \
  event_id "test-error-001" \
  event_type "artifactory.deployed" \
  payload 'invalid-json-{{'
```

**Expected:** Parse error logged, event not processed, no crash.

**Test 5b: Missing Required Fields**
```bash
redis-cli XADD webhook_events * \
  event_id "test-error-002" \
  event_type "artifactory.deployed" \
  payload '{"domain":"artifact"}'
```

**Expected:** Graceful handling, warning logged.

**Test 5c: Unknown Package Format**
```bash
redis-cli XADD webhook_events * \
  event_id "test-error-003" \
  event_type "artifactory.deployed" \
  payload '{"domain":"artifact","event_type":"deployed","data":{"repoPath":{"repoKey":"generic","path":"unknown/format"},"name":"weird.file"}}'
```

**Expected:** Falls back to generic type, creates record anyway.

## Monitoring During Tests

### 1. Watch Worker Logs
```bash
docker-compose -f docker-compose.local.yml logs -f worker | grep -i artifactory
```

### 2. Monitor Redis Queue
```bash
# Check queue length
redis-cli XLEN webhook_events

# Watch queue consumption
watch -n 1 'redis-cli XLEN webhook_events'
```

### 3. Check Metrics
```bash
# View Prometheus metrics
curl http://localhost:8088/metrics | grep artifactory
```

**Expected Metrics:**
```
webhook_artifactory_processed_total{repo_key="libs-release-local"} 1
webhook_artifactory_duration_seconds_bucket{le="0.5"} 1
```

## Validation Queries

### Query 1: Count Releases by Type
```sql
SELECT
  package_type,
  COUNT(*) as count,
  COUNT(CASE WHEN artifactory_path IS NOT NULL THEN 1 END) as with_artifactory
FROM mcp.package_releases
GROUP BY package_type;
```

### Query 2: Recent Artifactory Deployments
```sql
SELECT
  package_name,
  version,
  package_type,
  artifactory_path,
  metadata->'artifactory_deployed_by' as deployed_by,
  created_at
FROM mcp.package_releases
WHERE artifactory_path IS NOT NULL
ORDER BY created_at DESC
LIMIT 10;
```

### Query 3: Matched vs Unmatched Releases
```sql
SELECT
  CASE
    WHEN github_release_id IS NOT NULL AND artifactory_path IS NOT NULL
      THEN 'Both GitHub and Artifactory'
    WHEN github_release_id IS NOT NULL
      THEN 'GitHub Only'
    WHEN artifactory_path IS NOT NULL
      THEN 'Artifactory Only'
    ELSE 'Neither (shouldn''t happen)'
  END as source,
  COUNT(*) as count
FROM mcp.package_releases
GROUP BY source;
```

### Query 4: Assets with Checksums
```sql
SELECT
  pr.package_name,
  pr.version,
  pa.name as asset_name,
  pa.sha256_checksum,
  pa.artifactory_url
FROM mcp.package_assets pa
JOIN mcp.package_releases pr ON pa.release_id = pr.id
WHERE pa.artifactory_url IS NOT NULL
ORDER BY pa.created_at DESC
LIMIT 10;
```

## Performance Testing

### Load Test: Multiple Events
```bash
#!/bin/bash
# Send 10 Artifactory events rapidly
for i in {1..10}; do
  redis-cli XADD webhook_events * \
    event_id "load-test-$i" \
    event_type "artifactory.deployed" \
    payload "{\"domain\":\"artifact\",\"event_type\":\"deployed\",\"timestamp\":$(date +%s)000,\"data\":{\"repoPath\":{\"repoKey\":\"test-repo\",\"path\":\"com/example/app$i/1.0.0/app$i-1.0.0.jar\"},\"name\":\"app$i-1.0.0.jar\",\"size\":1024,\"created_by\":\"test\"}}"
  echo "Sent event $i"
done
```

**Measure:**
1. Time to process all 10 events
2. Check for any errors
3. Verify all 10 releases created

```sql
SELECT COUNT(*) FROM mcp.package_releases
WHERE package_name LIKE '%app%'
AND created_at > NOW() - INTERVAL '1 minute';
```

## Troubleshooting

### Issue: Events Not Processing

**Check 1: Worker Running**
```bash
docker-compose -f docker-compose.local.yml ps worker
```

**Check 2: Event in Queue**
```bash
redis-cli XLEN webhook_events
redis-cli XRANGE webhook_events - + COUNT 5
```

**Check 3: Worker Logs**
```bash
docker-compose -f docker-compose.local.yml logs --tail=100 worker | grep ERROR
```

### Issue: Releases Not Created

**Check 1: Parse Errors**
```bash
docker-compose -f docker-compose.local.yml logs worker | grep "Failed to parse"
```

**Check 2: Database Connection**
```bash
psql -h localhost -U devmesh -d devmesh_development -c "SELECT 1;"
```

**Check 3: Table Exists**
```sql
SELECT table_name FROM information_schema.tables
WHERE table_schema = 'mcp'
AND table_name = 'package_releases';
```

### Issue: GitHub Matching Not Working

**Check 1: Existing GitHub Release**
```sql
SELECT package_name, version, github_release_id
FROM mcp.package_releases
WHERE github_release_id IS NOT NULL;
```

**Check 2: Name Variations**
```bash
# Check logs for fuzzy matching attempts
docker-compose -f docker-compose.local.yml logs worker | grep "fuzzy match"
```

## Clean Up Test Data

```sql
-- Delete test releases
DELETE FROM mcp.package_releases
WHERE package_name LIKE '%test-%'
OR package_name LIKE '%example%';

-- Delete test assets (cascades automatically)

-- Verify cleanup
SELECT COUNT(*) FROM mcp.package_releases;
```

## Success Criteria

Phase 2 is successfully implemented if:

- ✅ Maven artifacts are parsed and stored correctly
- ✅ NPM packages (including scoped) are handled
- ✅ Python packages are recognized
- ✅ GitHub releases are matched when possible
- ✅ Artifactory-only packages create new releases
- ✅ Asset records include checksums
- ✅ Metadata includes deployment information
- ✅ No crashes on malformed data
- ✅ Metrics are recorded
- ✅ Logs are informative
- ✅ Performance is acceptable (<1s per event)

## Next Steps

After successful testing:
1. Configure Artifactory webhooks in production
2. Monitor initial deployments closely
3. Tune fuzzy matching if needed
4. Plan Phase 3 implementation

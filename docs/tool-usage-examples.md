# Tool Usage Examples Documentation

This document provides usage examples for tools in the DevOps MCP platform. These examples help AI agents understand how to properly use each tool with different parameter combinations and expected outputs.

## Example Structure

Each tool has 2-3 usage examples following this structure:
- **Simple**: Basic usage with minimal required parameters
- **Complex**: Advanced usage with multiple parameters and filters
- **Error Case**: Common error scenarios and how to handle them

Each example includes:
- `Name`: Type of example (simple/complex/error_case)
- `Description`: What the example demonstrates
- `Input`: Example parameters to pass to the tool
- `ExpectedOutput`: What a successful response looks like (for non-error cases)
- `ExpectedError`: Error details for error case examples
- `Notes`: Additional context and tips

## GitHub Provider Tools

### Issue Management Tools

#### get_issue
Retrieve detailed information about a specific GitHub issue.

**Simple Example:**
```json
{
  "owner": "facebook",
  "repo": "react",
  "issue_number": 1234
}
```
Returns basic issue data including title, state, and labels.

**Complex Example:**
```json
{
  "owner": "microsoft",
  "repo": "vscode",
  "issue_number": 98765
}
```
Returns comprehensive issue data with labels, assignees, and milestone information.

**Error Case:**
```json
{
  "owner": "github",
  "repo": "hub",
  "issue_number": 999999999
}
```
Returns 404 error - issue not found or lacks access permissions.

#### list_issues
Retrieve a paginated list of issues with filtering options.

**Simple Example:**
```json
{
  "owner": "golang",
  "repo": "go"
}
```
Returns all open issues sorted by creation date (default behavior).

**Complex Example:**
```json
{
  "owner": "kubernetes",
  "repo": "kubernetes",
  "state": "open",
  "labels": "bug,priority/P0",
  "assignee": "johndoe",
  "sort": "updated",
  "direction": "desc",
  "per_page": 50,
  "page": 2
}
```
Returns filtered issues with multiple criteria and custom pagination.

**Error Case:**
```json
{
  "owner": "torvalds",
  "repo": "linux",
  "state": "invalid_state"
}
```
Returns 422 error - invalid parameter value. The 'state' parameter only accepts 'open', 'closed', or 'all'.

#### create_issue
Create a new issue in a GitHub repository.

**Simple Example:**
```json
{
  "owner": "myorg",
  "repo": "myapp",
  "title": "Bug: Login button not working",
  "body": "The login button on the homepage is not responding to clicks.\n\n**Steps to reproduce:**\n1. Go to homepage\n2. Click login button\n3. Nothing happens"
}
```
Creates a basic issue with title and description.

**Complex Example:**
```json
{
  "owner": "kubernetes",
  "repo": "kubernetes",
  "title": "Feature: Add support for custom resource validation",
  "body": "## Description\nWe need to add validation for custom resources...\n\n## Acceptance Criteria\n- [ ] Schema validation\n- [ ] Webhook validation\n- [ ] Documentation",
  "labels": ["enhancement", "priority/P1", "sig/api-machinery"],
  "assignees": ["developer1", "developer2"],
  "milestone": 42
}
```
Creates an issue with labels, assignees, and milestone assignment.

**Error Case:**
```json
{
  "owner": "nodejs",
  "repo": "node",
  "title": "Test issue",
  "assignees": ["nonexistent-user-12345"]
}
```
Returns 422 error - validation failed. Assignees must be repository collaborators.

### Pull Request Tools

#### get_pull_request
Retrieve detailed information about a pull request.

**Simple Example:**
```json
{
  "owner": "golang",
  "repo": "go",
  "pull_number": 45678
}
```
Returns basic PR information including state and mergeability.

**Complex Example:**
```json
{
  "owner": "kubernetes",
  "repo": "kubernetes",
  "pull_number": 108234
}
```
Returns comprehensive PR data including review status, CI checks, and merge conflicts.

**Error Case:**
```json
{
  "owner": "torvalds",
  "repo": "linux",
  "pull_number": 999999999
}
```
Returns 404 error - PR not found (Linux kernel uses different workflow).

## Harness Provider Tools

### Pipeline Management

Harness pipelines already include comprehensive examples in their tool definitions:

- **Execute Pipeline**: Triggers production deployment with approval gates
- **List Pipelines**: Retrieves all CD pipelines in a project
- **Create Pipeline**: Creates new CI pipeline with stages and steps

### Project Management

- **Create Project**: Sets up new project for microservices
- **List Projects**: Shows all projects with specific modules enabled

### Connector Management

- **Create Connector**: Establishes GitHub connection with authentication
- **Validate Connector**: Tests cloud connector functionality

### GitOps Management

- **Sync Application**: Synchronizes GitOps app with Git source
- **Rollback Application**: Reverts to previous application version

## Best Practices for AI Agents

1. **Start Simple**: Use simple examples first to understand basic functionality
2. **Handle Errors**: Always implement error handling based on error case examples
3. **Validate Input**: Check required fields and parameter types before calling
4. **Use Notes**: Pay attention to notes for important context and constraints
5. **Test Incrementally**: Test with simple examples before moving to complex scenarios

## Validation

All examples are validated through automated tests that ensure:
- Required fields are present in non-error examples
- Field types match schema definitions
- Error examples have appropriate error codes
- Expected outputs are properly structured

## Contributing

When adding new tools or modifying existing ones:
1. Include at least 2 usage examples (simple + complex/error)
2. Ensure examples follow the established structure
3. Add corresponding tests in `*_test.go` files
4. Update this documentation with new examples

## Related Files

- `/pkg/tools/providers/github/enhanced_tool_definitions.go` - GitHub tool definitions with examples
- `/pkg/tools/providers/harness/ai_definitions.go` - Harness tool definitions with examples
- `/pkg/tools/providers/github/enhanced_tool_definitions_test.go` - Validation tests
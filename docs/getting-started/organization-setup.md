# Organization Setup and User Management

This guide walks you through registering your organization with DevMesh and managing your team members.

## Getting Started

DevMesh uses an organization-based model where all users belong to an organization. Each organization has its own isolated tenant with separate data, API keys, and resources.

## Step 1: Register Your Organization

To get started with DevMesh, you first need to register your organization. This creates your organization account along with an administrator user.

### Registration Requirements

Before registering, ensure you have:
- A unique organization slug (3-50 characters, lowercase letters, numbers, and hyphens only)
- A valid email address for the admin account
- A secure password that meets our requirements

### Password Requirements

Your password must:
- Be at least 8 characters long
- Contain at least one uppercase letter
- Contain at least one lowercase letter  
- Contain at least one number

### API Endpoint

```
POST /api/v1/auth/register/organization
```

### Request Format

```json
{
  "organization_name": "Your Company Name",
  "organization_slug": "your-company",
  "admin_email": "admin@yourcompany.com",
  "admin_name": "John Smith",
  "admin_password": "SecurePass123",
  "company_size": "10-50",        // Optional
  "industry": "Technology",        // Optional
  "use_case": "DevOps automation"  // Optional
}
```

### Example Registration

```bash
curl -X POST https://api.devmesh.io/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "Acme Corporation",
    "organization_slug": "acme-corp",
    "admin_email": "admin@acme.com",
    "admin_name": "Alice Johnson",
    "admin_password": "MySecure2024Pass"
  }'
```

### Response

On successful registration, you'll receive:

```json
{
  "organization": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "max_users": 5,
    "max_agents": 10
  },
  "user": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "email": "admin@acme.com",
    "name": "Alice Johnson",
    "role": "owner",
    "email_verified": false
  },
  "api_key": "devmesh_a1b2c3d4e5f6...",
  "message": "Organization registered successfully. Please check your email to verify your account."
}
```

**Important**: Save your API key securely. This is the only time it will be shown.

### Organization Slug Rules

The organization slug:
- Must start with a letter or number
- Can contain lowercase letters, numbers, and hyphens
- Must be 3-50 characters long
- Must be unique across all organizations
- Cannot be changed after registration

Examples of valid slugs:
- `acme-corp`
- `startup2024`
- `my-company-name`

Examples of invalid slugs:
- `Acme-Corp` (uppercase not allowed)
- `my_company` (underscores not allowed)
- `ab` (too short)
- `-company` (cannot start with hyphen)

## Step 2: Login to Your Account

Once registered, you can login to obtain JWT tokens for API access.

### API Endpoint

```
POST /api/v1/auth/login
```

### Request Format

```json
{
  "email": "admin@yourcompany.com",
  "password": "YourPassword123"
}
```

### Example Login

```bash
curl -X POST https://api.devmesh.io/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "MySecure2024Pass"
  }'
```

### Response

```json
{
  "user": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "email": "admin@acme.com",
    "name": "Alice Johnson",
    "role": "owner",
    "status": "active",
    "email_verified": true
  },
  "organization": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "max_users": 5,
    "max_agents": 10
  },
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "a1b2c3d4e5f6...",
  "expires_in": 86400
}
```

The `access_token` is valid for 24 hours. Use it in the Authorization header for API requests:

```bash
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

## Step 3: Invite Team Members

As an organization owner or admin, you can invite team members to join your organization.

### API Endpoint

```
POST /api/v1/users/invite
```

**Note**: This endpoint requires authentication. Include your access token in the Authorization header.

### Request Format

```json
{
  "email": "developer@yourcompany.com",
  "name": "Bob Smith",
  "role": "member"
}
```

### Available Roles

| Role | Description | Permissions |
|------|-------------|-------------|
| `admin` | Administrator | Can invite users, manage settings, configure agents |
| `member` | Team Member | Can use tools and agents, create resources |
| `readonly` | Read-Only User | Can view resources but cannot make changes |

**Note**: Only the organization owner can change user roles after invitation.

### Example Invitation

```bash
curl -X POST https://api.devmesh.io/api/v1/users/invite \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "developer@acme.com",
    "name": "Bob Developer",
    "role": "member"
  }'
```

### Response

```json
{
  "message": "Invitation sent successfully",
  "email": "developer@acme.com"
}
```

The invited user will receive an email with instructions to complete their registration.

## Step 4: Accept an Invitation

When a user receives an invitation, they can accept it to create their account.

### API Endpoint

```
POST /api/v1/auth/invitation/accept
```

### Request Format

```json
{
  "token": "invitation_token_from_email",
  "password": "NewUserPassword123"
}
```

The password must meet the same requirements as described in Step 1.

### Example Acceptance

```bash
curl -X POST https://api.devmesh.io/api/v1/auth/invitation/accept \
  -H "Content-Type: application/json" \
  -d '{
    "token": "abc123def456...",
    "password": "MySecurePass2024"
  }'
```

### Response

The response automatically logs in the new user:

```json
{
  "user": {
    "id": "770e8400-e29b-41d4-a716-446655440002",
    "email": "developer@acme.com",
    "name": "developer@acme.com",
    "role": "member"
  },
  "organization": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corporation",
    "slug": "acme-corp"
  },
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "xyz789...",
  "expires_in": 86400
}
```

## Account Security

### Failed Login Protection

- After 5 failed login attempts, the account is locked for 15 minutes
- Each failed attempt is logged for security auditing
- Successful login resets the failed attempt counter

### Session Management

- Access tokens expire after 24 hours
- Refresh tokens are valid for 30 days
- Each login creates a new session
- Sessions can be revoked if compromised

## Organization Limits

Currently, all organizations have the following default limits:
- Maximum users: 5
- Maximum agents: 10

These limits are hardcoded and cannot be changed at this time. A subscription system may be added in the future.

## Common Error Responses

### Registration Errors

| Error | Status Code | Meaning |
|-------|-------------|---------|
| "Organization slug already taken" | 409 | The slug is already in use |
| "Email already registered" | 409 | Email is already associated with an account |
| "Invalid organization slug format" | 400 | Slug doesn't meet requirements |
| "password must contain uppercase, lowercase, and numbers" | 400 | Password doesn't meet requirements |

### Login Errors

| Error | Status Code | Meaning |
|-------|-------------|---------|
| "Invalid email or password" | 401 | Credentials are incorrect |
| "account locked until..." | 401 | Too many failed attempts |
| "account is suspended" | 401 | Account has been suspended |

### Invitation Errors

| Error | Status Code | Meaning |
|-------|-------------|---------|
| "You don't have permission to invite users" | 403 | Only owners and admins can invite |
| "User with this email already exists" | 409 | Email is already registered |
| "Invitation already sent to this email" | 409 | Pending invitation exists |
| "Organization has reached maximum user limit" | 400 | Upgrade required to add more users |

## Using Your API Key

After registration, you receive an API key. Use it for service-to-service authentication:

```bash
# Using API Key
curl -H "X-API-Key: devmesh_a1b2c3d4e5f6..." \
     https://api.devmesh.io/api/v1/tools

# Or in Authorization header
curl -H "Authorization: Bearer devmesh_a1b2c3d4e5f6..." \
     https://api.devmesh.io/api/v1/agents
```

API keys:
- Never expire (unless manually revoked)
- Are tied to your organization's tenant
- Have admin-level permissions
- Should be kept secure and never shared

## Next Steps

After setting up your organization:

1. [Configure your first agent](../guides/agents/agent-registration-guide.md)
2. [Set up dynamic tools](../reference/api/dynamic_tools_api.md)
3. [Configure webhooks](../reference/api/webhook-api-reference.md)
4. [Explore the API](../reference/api/rest-api-reference.md)

## Getting Help

If you encounter issues:

1. Check the [Troubleshooting Guide](../troubleshooting/README.md)
2. Review [Common Issues](#common-error-responses) above
3. Contact support with your organization ID (never share your API key)

## Testing the Registration System

A test script is provided to verify the registration and authentication system:

```bash
# Run the test script
./scripts/test-organization-registration.sh

# Or test against a specific API URL
API_URL=https://api.devmesh.io ./scripts/test-organization-registration.sh
```

The script tests:
- Organization registration
- User login
- Invalid credentials handling
- User invitation
- API key generation
- Duplicate prevention
- Password validation
- Slug validation

## Notes for Developers

### Email Service Integration

The system expects an email service to be configured for:
- Welcome emails
- Email verification
- Invitation emails
- Password reset emails

If emails are not configured, the system will still work but users won't receive notifications.

### Placeholder Endpoints

The following endpoints are defined but not yet implemented:
- `POST /api/v1/auth/refresh` - Token refresh
- `POST /api/v1/auth/logout` - Session logout
- `POST /api/v1/auth/password/reset` - Password reset request
- `POST /api/v1/auth/email/verify` - Email verification
- `GET /api/v1/users` - List organization users
- `PUT /api/v1/users/:id/role` - Update user role
- `GET /api/v1/organization` - Get organization details
- `GET /api/v1/profile` - Get user profile

These will return `{"error": "Not implemented yet"}` with status 501.
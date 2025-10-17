# Organization & Authentication API Reference

## Overview

The Organization & Authentication API provides endpoints for organization registration, user authentication, and team management in DevMesh.

Base URL: `https://api.devmesh.io/api/v1`

## Authentication

Most endpoints require authentication using either:
- **JWT Token**: Bearer token obtained from login
- **API Key**: Organization API key for service-to-service calls

Include in request headers:
```
Authorization: Bearer <token>
```
or
```
X-API-Key: <api-key>
```

## Endpoints

### Organization Registration

#### Register Organization

Creates a new organization with an admin user account.

**Endpoint:** `POST /auth/register/organization`

**Authentication:** None (public endpoint)

**Request Body:**
```json
{
  "organization_name": "string",  // Required, 3-100 characters
  "organization_slug": "string",  // Required, 3-50 chars, lowercase, numbers, hyphens
  "admin_email": "string",        // Required, valid email
  "admin_name": "string",         // Required, 2-100 characters
  "admin_password": "string",     // Required, min 8 chars, must have uppercase, lowercase, number
  "company_size": "string",       // Optional: "1-10", "10-50", "50-200", "200+"
  "industry": "string",           // Optional
  "use_case": "string"            // Optional, description of intended use
}
```

**Response:** `201 Created`
```json
{
  "organization": {
    "id": "uuid",
    "name": "string",
    "slug": "string",
    "max_users": 5,
    "max_agents": 10,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  },
  "user": {
    "id": "uuid",
    "email": "string",
    "name": "string",
    "role": "owner",
    "email_verified": false
  },
  "api_key": "devmesh_xxx...",  // Save this! Only shown once
  "message": "Organization registered successfully. Please check your email to verify your account."
}
```

**Error Responses:**

| Status | Error | Description |
|--------|-------|-------------|
| 400 | Invalid organization slug format | Slug doesn't meet requirements |
| 400 | password validation failed | Password doesn't meet security requirements |
| 409 | Organization slug already taken | Slug is already in use |
| 409 | Email already registered | Email is already associated with an account |
| 500 | Failed to register organization | Server error during registration |

**Validation Rules:**
- Organization slug: `^[a-z0-9][a-z0-9-]{2,49}$`
- Password: Min 8 chars, requires uppercase, lowercase, and number
- Email: Valid email format
- Names: Min 2 chars for admin_name, min 3 for organization_name

---

### Authentication

#### Login

Authenticates a user and returns JWT tokens.

**Endpoint:** `POST /auth/login`

**Authentication:** None (public endpoint)

**Request Body:**
```json
{
  "email": "string",     // Required, valid email
  "password": "string"   // Required
}
```

**Response:** `200 OK`
```json
{
  "user": {
    "id": "uuid",
    "organization_id": "uuid",
    "tenant_id": "uuid",
    "email": "string",
    "name": "string",
    "role": "owner|admin|member|readonly",
    "status": "active|inactive|suspended",
    "email_verified": "boolean",
    "created_at": "2024-01-01T00:00:00Z"
  },
  "organization": {
    "id": "uuid",
    "name": "string",
    "slug": "string",
    "max_users": 5,
    "max_agents": 10
  },
  "access_token": "eyJhbGc...",  // JWT token, expires in 24 hours
  "refresh_token": "string",      // Use to get new access token
  "expires_in": 86400             // Seconds until access_token expires
}
```

**Error Responses:**

| Status | Error | Description |
|--------|-------|-------------|
| 400 | Invalid request | Malformed request body |
| 401 | Invalid email or password | Incorrect credentials |
| 401 | account locked until... | Too many failed attempts (5), locked for 15 minutes |
| 401 | account is [status] | Account is inactive or suspended |

**Security Notes:**
- Failed attempts are tracked per account
- Account locks after 5 failed attempts for 15 minutes
- All login attempts are logged for audit

---

### User Management

#### Invite User

Invites a new user to join the organization.

**Endpoint:** `POST /users/invite`

**Authentication:** Required (JWT or API Key)

**Required Role:** `owner` or `admin`

**Request Body:**
```json
{
  "email": "string",  // Required, valid email
  "name": "string",   // Required, 2-100 characters
  "role": "string"    // Required: "admin", "member", or "readonly"
}
```

**Response:** `200 OK`
```json
{
  "message": "Invitation sent successfully",
  "email": "string"
}
```

**Error Responses:**

| Status | Error | Description |
|--------|-------|-------------|
| 400 | Invalid request | Malformed request body |
| 400 | Organization has reached maximum user limit | Need to upgrade subscription |
| 401 | Unauthorized | No valid authentication provided |
| 403 | You don't have permission to invite users | Must be owner or admin |
| 409 | User with this email already exists | Email already registered |
| 409 | Invitation already sent to this email | Pending invitation exists |

**Notes:**
- Invitations expire after 7 days
- Users receive email with invitation link and token
- Organization's user limit is checked before sending

#### Accept Invitation

Accepts an invitation and creates a user account.

**Endpoint:** `POST /auth/invitation/accept`

**Authentication:** None (public endpoint)

**Request Body:**
```json
{
  "token": "string",    // Required, invitation token from email
  "password": "string"  // Required, min 8 chars, uppercase, lowercase, number
}
```

**Response:** `201 Created`
```json
{
  "user": {
    "id": "uuid",
    "organization_id": "uuid",
    "tenant_id": "uuid",
    "email": "string",
    "name": "string",
    "role": "admin|member|readonly",
    "status": "active",
    "email_verified": true
  },
  "organization": {
    "id": "uuid",
    "name": "string",
    "slug": "string",
    "subscription_tier": "string",
    "max_users": "integer",
    "max_agents": "integer"
  },
  "access_token": "eyJhbGc...",
  "refresh_token": "string",
  "expires_in": 86400
}
```

**Error Responses:**

| Status | Error | Description |
|--------|-------|-------------|
| 400 | Invalid request | Malformed request body |
| 400 | password validation failed | Password doesn't meet requirements |
| 400 | invalid or expired invitation | Token is invalid or expired |
| 400 | invitation already accepted | Invitation was already used |
| 400 | invitation has expired | Invitation older than 7 days |

**Notes:**
- Automatically logs in the user after successful acceptance
- Email is marked as verified since they received the invitation
- User gets the role specified in the invitation

---

## Not Yet Implemented

The following endpoints are defined but return `501 Not Implemented`:

### Authentication
- `POST /auth/refresh` - Refresh access token
- `POST /auth/logout` - Logout and invalidate session
- `POST /auth/password/reset` - Request password reset
- `POST /auth/password/reset/confirm` - Confirm password reset
- `POST /auth/email/verify` - Verify email address
- `POST /auth/email/resend` - Resend verification email
- `GET /auth/invitation/:token` - Get invitation details

### User Management  
- `GET /users` - List organization users
- `PUT /users/:id/role` - Update user role
- `DELETE /users/:id` - Remove user from organization

### Organization Management
- `GET /organization` - Get organization details
- `PUT /organization` - Update organization settings
- `GET /organization/usage` - Get usage statistics

### Profile Management
- `GET /profile` - Get current user profile
- `PUT /profile` - Update profile
- `POST /profile/password` - Change password

---

## Rate Limiting

API endpoints are rate limited per organization:
- Authentication endpoints: 10 requests per minute
- Management endpoints: 100 requests per minute
- Registration: 5 requests per hour

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1704067200
```

---

## Data Models

### Organization
```typescript
{
  id: string;                    // UUID
  name: string;                  // Display name
  slug: string;                  // Unique identifier
  owner_user_id?: string;        // UUID of owner
  max_users: number;             // User limit (currently fixed at 5)
  max_agents: number;            // Agent limit (currently fixed at 10)
  billing_email?: string;        // Billing contact
  settings: object;              // Custom settings
  created_at: string;            // ISO 8601
  updated_at: string;            // ISO 8601
}
```

### User
```typescript
{
  id: string;                    // UUID
  organization_id?: string;      // UUID
  tenant_id: string;             // UUID
  email: string;                 // Unique email
  name: string;                  // Display name
  role: "owner" | "admin" | "member" | "readonly";
  status: "active" | "inactive" | "suspended";
  email_verified: boolean;       
  email_verified_at?: string;    // ISO 8601
  last_login_at?: string;        // ISO 8601
  created_at: string;            // ISO 8601
  updated_at: string;            // ISO 8601
}
```

### User Roles

| Role | Can Invite Users | Can Manage Settings | Can Use Tools | Can View |
|------|------------------|---------------------|---------------|----------|
| owner | ✓ | ✓ | ✓ | ✓ |
| admin | ✓ | ✓ | ✓ | ✓ |
| member | ✗ | ✗ | ✓ | ✓ |
| readonly | ✗ | ✗ | ✗ | ✓ |

---

## Security Considerations

1. **Password Storage**: Passwords are hashed using bcrypt with cost factor 10
2. **Token Security**: JWT tokens use HS256 signing with minimum 32-character secret
3. **Session Management**: Refresh tokens stored in database, can be revoked
4. **Audit Logging**: All authentication events logged to `auth_audit_log` table
5. **Tenant Isolation**: Each organization has separate tenant_id for data isolation
6. **API Keys**: Generated using cryptographically secure random bytes

---

## Examples

### Complete Registration Flow

```bash
# 1. Register organization
curl -X POST https://api.devmesh.io/api/v1/auth/register/organization \
  -H "Content-Type: application/json" \
  -d '{
    "organization_name": "Tech Startup Inc",
    "organization_slug": "tech-startup",
    "admin_email": "ceo@techstartup.com",
    "admin_name": "Jane CEO",
    "admin_password": "SuperSecure2024!"
  }'

# Save the api_key from response!

# 2. Login as admin
curl -X POST https://api.devmesh.io/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "ceo@techstartup.com",
    "password": "SuperSecure2024!"
  }'

# Save the access_token!

# 3. Invite team member
curl -X POST https://api.devmesh.io/api/v1/users/invite \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "developer@techstartup.com",
    "name": "John Developer",
    "role": "member"
  }'

# 4. Team member accepts (with token from email)
curl -X POST https://api.devmesh.io/api/v1/auth/invitation/accept \
  -H "Content-Type: application/json" \
  -d '{
    "token": "INVITATION_TOKEN_FROM_EMAIL",
    "password": "DevPassword2024!"
  }'
```

### Using the API Key

```bash
# List tools using API key
curl -H "X-API-Key: devmesh_xxx..." \
     https://api.devmesh.io/api/v1/tools

# Create agent using API key
curl -X POST https://api.devmesh.io/api/v1/agents \
  -H "X-API-Key: devmesh_xxx..." \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Agent",
    "type": "code_review"
  }'
```

---

## Migration Notes

For existing deployments migrating to this system:

1. Run migration `000027_user_registration_auth.up.sql`
2. Existing API keys remain valid
3. Create organization records for existing tenants
4. Assign users to organizations
5. Set appropriate user roles

---

## Support

For issues or questions:
- Review [Organization Setup Guide](../getting-started/organization-setup.md)
- Check [Troubleshooting Guide](../troubleshooting/TROUBLESHOOTING.md)
- Contact support with organization ID (never share API keys or tokens)
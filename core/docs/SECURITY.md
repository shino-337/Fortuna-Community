# Security Guide

## Authentication

KSAM Core uses JWT (JSON Web Tokens) for authentication.

### Login

```bash
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "password"
}
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@example.com",
    "role": "admin"
  },
  "expiresAt": "2024-01-01T00:00:00Z"
}
```

### Using the Token

Include the token in the Authorization header:

```bash
Authorization: Bearer <token>
```

### Register New User (Admin Only)

```bash
POST /api/v1/auth/register
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "username": "newuser",
  "email": "user@example.com",
  "password": "securepassword",
  "role": "user"
}
```

## User Roles

- **admin**: Full access, can create users, manage all resources
- **user**: Can view and edit resources
- **viewer**: Read-only access

## Security Features

### Password Hashing

Passwords are hashed using bcrypt with cost factor 12.

### JWT Configuration

- Algorithm: HS256
- Token expiration: Configurable via `TOKEN_EXPIRATION_HOURS` (default: 24 hours)
- Secret key: Set via `JWT_SECRET` environment variable

### Security Headers

The following security headers are automatically added to all responses:

- `X-Frame-Options: DENY` - Prevents clickjacking
- `X-Content-Type-Options: nosniff` - Prevents MIME type sniffing
- `X-XSS-Protection: 1; mode=block` - XSS protection
- `Strict-Transport-Security` - HSTS
- `Content-Security-Policy` - CSP
- `Referrer-Policy` - Referrer policy
- `Permissions-Policy` - Permissions policy

### CORS

CORS is configured to allow requests from:
- `http://localhost:3000` (Dashboard)
- `http://localhost:8080` (Development)

Configure allowed origins in `internal/middleware/security.go`.

## Environment Variables

### Required

- `JWT_SECRET` - Secret key for JWT signing (change in production!)

### Optional

- `AUTH_ENABLED` - Enable/disable authentication (default: true)
- `TOKEN_EXPIRATION_HOURS` - Token expiration in hours (default: 24)
- `KSAM_ADMIN_USERNAME` - Default admin username
- `KSAM_ADMIN_PASSWORD` - Default admin password
- `KSAM_ADMIN_EMAIL` - Default admin email

## Best Practices

1. **Change JWT Secret**: Always change the default JWT secret in production
2. **Use Strong Passwords**: Enforce strong password policies
3. **HTTPS Only**: Always use HTTPS in production
4. **Token Expiration**: Set appropriate token expiration times
5. **Regular Audits**: Review audit logs regularly
6. **Least Privilege**: Assign minimum required roles to users
7. **Password Rotation**: Implement password rotation policies

## API Security

### Protected Endpoints

All `/api/v1/*` endpoints require authentication (except `/health`).

### Role-Based Access Control

- Admin: Full access
- User: Can view and edit
- Viewer: Read-only

### Audit Logging

All actions are logged in the `audit_logs` table with:
- User ID
- Action type
- Resource type
- IP address
- Timestamp

## Troubleshooting

### Token Expired

If you get a 401 Unauthorized error, your token may have expired. Login again to get a new token.

### Invalid Token

Check that:
1. Token is included in Authorization header
2. Token format is correct: `Bearer <token>`
3. Token hasn't been tampered with
4. JWT_SECRET matches between token generation and validation

### User Inactive

If a user account is marked as inactive, they cannot login even with correct credentials.


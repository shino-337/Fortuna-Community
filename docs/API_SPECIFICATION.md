# KSAM API Specification

**Version**: 1.0  
**Last Updated**: 2025-11-28  
**Base URL**: `https://ksam-core.example.com/api/v1`

---

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)
- [OpenAPI Specification](#openapi-specification)
- [API Endpoints](#api-endpoints)
  - [Authentication API](#authentication-api)
  - [Clusters API](#clusters-api)
  - [ServiceAccounts API](#serviceaccounts-api)
  - [Insights API](#insights-api)
  - [Graph API](#graph-api)
  - [Attack Simulation API](#attack-simulation-api)
  - [Policies API](#policies-api)
  - [Compliance API](#compliance-api)
  - [Audit Logs API](#audit-logs-api)
  - [Users API](#users-api)
- [WebSocket API](#websocket-api)
- [gRPC API](#grpc-api)
- [Examples](#examples)

---

## Overview

KSAM provides a RESTful API for managing Kubernetes security, RBAC analysis, risk insights, and policy automation.

**API Design Principles**:
- RESTful architecture
- JSON request/response
- JWT Bearer token authentication
- Consistent error responses
- Pagination for list endpoints
- Filtering and sorting support
- HATEOAS links where applicable

**API Versioning**:
- Current version: `v1`
- Version in URL path: `/api/v1/...`
- Breaking changes will increment major version
- Backward compatible changes within same version

---

## Authentication

All API requests (except `/auth/login` and `/auth/register`) require authentication.

### JWT Bearer Token

**Header**:
```
Authorization: Bearer <jwt_token>
```

**Token Structure**:
```json
{
  "user_id": "user-123",
  "email": "user@example.com",
  "role": "admin",
  "exp": 1732800000,
  "iat": 1732798200
}
```

**Token Lifetime**:
- Access token: 30 minutes
- Refresh token: 24 hours

**Example**:
```bash
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  https://ksam-core/api/v1/insights
```

---

## Error Handling

### Error Response Format

```json
{
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "ServiceAccount not found",
    "details": {
      "resource_type": "ServiceAccount",
      "resource_id": "sa-12345"
    },
    "timestamp": "2025-11-28T10:30:00Z",
    "trace_id": "trace-abc123"
  }
}
```

### HTTP Status Codes

| Code | Meaning | Usage |
|------|---------|-------|
| 200 | OK | Successful GET, PUT, PATCH |
| 201 | Created | Successful POST |
| 204 | No Content | Successful DELETE |
| 400 | Bad Request | Invalid request payload |
| 401 | Unauthorized | Missing or invalid token |
| 403 | Forbidden | Insufficient permissions |
| 404 | Not Found | Resource not found |
| 409 | Conflict | Resource already exists |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Server error |
| 503 | Service Unavailable | Service temporarily unavailable |

### Error Codes

| Code | Description |
|------|-------------|
| `INVALID_REQUEST` | Request validation failed |
| `UNAUTHORIZED` | Authentication required |
| `FORBIDDEN` | Insufficient permissions |
| `RESOURCE_NOT_FOUND` | Resource does not exist |
| `RESOURCE_ALREADY_EXISTS` | Resource with same identifier exists |
| `RATE_LIMIT_EXCEEDED` | Too many requests |
| `INTERNAL_ERROR` | Unexpected server error |

---

## Rate Limiting

**Limits**:
- Authenticated users: 1000 requests/hour
- Login endpoint: 5 requests/minute
- Anonymous: Not allowed (except login)

**Headers**:
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 950
X-RateLimit-Reset: 1732801200
```

**Rate Limit Exceeded Response**:
```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit exceeded. Try again in 3600 seconds.",
    "details": {
      "limit": 1000,
      "remaining": 0,
      "reset_at": "2025-11-28T12:00:00Z"
    }
  }
}
```

---

## OpenAPI Specification

### OpenAPI 3.0 YAML

```yaml
openapi: 3.0.3
info:
  title: KSAM API
  version: 1.0.0
  description: |
    KSAM (Kubernetes Service Account Manager) API for security management,
    RBAC analysis, risk insights, and policy automation.
  contact:
    name: KSAM Support
    email: support@ksam.io
    url: https://docs.ksam.io
  license:
    name: Apache 2.0
    url: https://www.apache.org/licenses/LICENSE-2.0.html

servers:
  - url: https://ksam-core.example.com/api/v1
    description: Production server
  - url: http://localhost:8080/api/v1
    description: Development server

security:
  - BearerAuth: []

components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT

  schemas:
    Error:
      type: object
      required:
        - code
        - message
      properties:
        code:
          type: string
          example: "RESOURCE_NOT_FOUND"
        message:
          type: string
          example: "Resource not found"
        details:
          type: object
        timestamp:
          type: string
          format: date-time
        trace_id:
          type: string

    PaginationMeta:
      type: object
      properties:
        total:
          type: integer
          example: 150
        page:
          type: integer
          example: 1
        per_page:
          type: integer
          example: 20
        total_pages:
          type: integer
          example: 8

    Cluster:
      type: object
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string
        config:
          type: object
        status:
          type: string
          enum: [active, inactive, error]
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time

    ServiceAccount:
      type: object
      properties:
        id:
          type: string
          format: uuid
        cluster_id:
          type: string
          format: uuid
        namespace:
          type: string
        name:
          type: string
        secrets:
          type: array
          items:
            type: string
        last_used:
          type: string
          format: date-time
          nullable: true
        risk_score:
          type: number
          format: float
          minimum: 0
          maximum: 10
        status:
          type: string
          enum: [active, orphaned, unused]
        created_at:
          type: string
          format: date-time

    Insight:
      type: object
      properties:
        id:
          type: string
          format: uuid
        rule_id:
          type: string
        cluster_id:
          type: string
          format: uuid
        resource_type:
          type: string
        resource_id:
          type: string
          format: uuid
        title:
          type: string
        description:
          type: string
        severity:
          type: string
          enum: [critical, high, medium, low, info]
        category:
          type: string
        risk_score:
          type: number
          format: float
        status:
          type: string
          enum: [active, resolved, ignored]
        cis_section:
          type: string
          nullable: true
        cis_level:
          type: integer
          nullable: true
        remediation:
          $ref: '#/components/schemas/Remediation'
        detected_at:
          type: string
          format: date-time
        resolved_at:
          type: string
          format: date-time
          nullable: true

    Remediation:
      type: object
      properties:
        description:
          type: string
        steps:
          type: array
          items:
            type: string
        auto_fix:
          type: boolean
        fix_script:
          type: string
          nullable: true

    AttackScenario:
      type: object
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string
        compromised_id:
          type: string
          format: uuid
        compromised_type:
          type: string
        objective:
          type: string
          enum: [access-secrets, privilege-escalation, network-access, persistence, data-exfiltration]
        blast_radius:
          $ref: '#/components/schemas/BlastRadius'
        attack_paths:
          type: array
          items:
            $ref: '#/components/schemas/AttackPath'
        lateral_moves:
          type: array
          items:
            $ref: '#/components/schemas/LateralMove'
        risk_score:
          type: number
          format: float
        simulated_at:
          type: string
          format: date-time

    BlastRadius:
      type: object
      properties:
        source_id:
          type: string
          format: uuid
        resources:
          type: array
          items:
            $ref: '#/components/schemas/ReachableResource'
        secrets:
          type: integer
        pods:
          type: integer
        services:
          type: integer
        high_risk_count:
          type: integer
        total_risk_score:
          type: number
          format: float

    ReachableResource:
      type: object
      properties:
        id:
          type: string
          format: uuid
        type:
          type: string
        name:
          type: string
        namespace:
          type: string
        risk_score:
          type: number
          format: float
        distance:
          type: integer

    AttackPath:
      type: object
      properties:
        nodes:
          type: array
          items:
            $ref: '#/components/schemas/PathNode'
        edges:
          type: array
          items:
            type: string
        total_risk:
          type: number
          format: float
        difficulty:
          type: number
          format: float
        impact:
          type: number
          format: float
        length:
          type: integer
        description:
          type: string

    PathNode:
      type: object
      properties:
        id:
          type: string
          format: uuid
        type:
          type: string
        name:
          type: string
        risk_score:
          type: number
          format: float
        vulnerable:
          type: boolean
        controls:
          type: array
          items:
            type: string

    LateralMove:
      type: object
      properties:
        from:
          type: string
          format: uuid
        to:
          type: string
          format: uuid
        method:
          type: string
        difficulty:
          type: number
          format: float
        prerequisites:
          type: array
          items:
            type: string
        description:
          type: string

    Policy:
      type: object
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string
        type:
          type: string
          enum: [kubearmor, networkpolicy, psp]
        target_resource_id:
          type: string
          format: uuid
        spec:
          type: object
        status:
          type: string
          enum: [draft, preview, active, disabled]
        confidence:
          type: number
          format: float
        generated_at:
          type: string
          format: date-time
        applied_at:
          type: string
          format: date-time
          nullable: true

    User:
      type: object
      properties:
        id:
          type: string
          format: uuid
        username:
          type: string
        email:
          type: string
          format: email
        role:
          type: string
          enum: [admin, user, viewer]
        created_at:
          type: string
          format: date-time
        last_login:
          type: string
          format: date-time
          nullable: true

paths:
  /auth/login:
    post:
      tags: [Authentication]
      summary: User login
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - username
                - password
              properties:
                username:
                  type: string
                password:
                  type: string
                  format: password
      responses:
        '200':
          description: Login successful
          content:
            application/json:
              schema:
                type: object
                properties:
                  access_token:
                    type: string
                  refresh_token:
                    type: string
                  expires_in:
                    type: integer
                  user:
                    $ref: '#/components/schemas/User'
        '401':
          description: Invalid credentials
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

  /auth/refresh:
    post:
      tags: [Authentication]
      summary: Refresh access token
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - refresh_token
              properties:
                refresh_token:
                  type: string
      responses:
        '200':
          description: Token refreshed
          content:
            application/json:
              schema:
                type: object
                properties:
                  access_token:
                    type: string
                  expires_in:
                    type: integer

  /clusters:
    get:
      tags: [Clusters]
      summary: List clusters
      parameters:
        - name: page
          in: query
          schema:
            type: integer
            default: 1
        - name: per_page
          in: query
          schema:
            type: integer
            default: 20
      responses:
        '200':
          description: List of clusters
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/Cluster'
                  meta:
                    $ref: '#/components/schemas/PaginationMeta'

  /serviceaccounts:
    get:
      tags: [ServiceAccounts]
      summary: List ServiceAccounts
      parameters:
        - name: cluster_id
          in: query
          schema:
            type: string
            format: uuid
        - name: namespace
          in: query
          schema:
            type: string
        - name: status
          in: query
          schema:
            type: string
            enum: [active, orphaned, unused]
        - name: min_risk_score
          in: query
          schema:
            type: number
            format: float
        - name: page
          in: query
          schema:
            type: integer
        - name: per_page
          in: query
          schema:
            type: integer
      responses:
        '200':
          description: List of ServiceAccounts
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/ServiceAccount'
                  meta:
                    $ref: '#/components/schemas/PaginationMeta'

  /serviceaccounts/{id}:
    get:
      tags: [ServiceAccounts]
      summary: Get ServiceAccount by ID
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: ServiceAccount details
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ServiceAccount'
        '404':
          description: ServiceAccount not found

  /insights:
    get:
      tags: [Insights]
      summary: List insights
      parameters:
        - name: cluster_id
          in: query
          schema:
            type: string
            format: uuid
        - name: severity
          in: query
          schema:
            type: string
            enum: [critical, high, medium, low, info]
        - name: status
          in: query
          schema:
            type: string
            enum: [active, resolved, ignored]
        - name: category
          in: query
          schema:
            type: string
        - name: page
          in: query
          schema:
            type: integer
        - name: per_page
          in: query
          schema:
            type: integer
      responses:
        '200':
          description: List of insights
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/Insight'
                  meta:
                    $ref: '#/components/schemas/PaginationMeta'

  /insights/{id}:
    get:
      tags: [Insights]
      summary: Get insight by ID
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: Insight details
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Insight'

    patch:
      tags: [Insights]
      summary: Update insight status
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                status:
                  type: string
                  enum: [resolved, ignored]
      responses:
        '200':
          description: Insight updated
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Insight'

  /graph/blast-radius/{resource_id}:
    get:
      tags: [Graph]
      summary: Get blast radius for a resource
      parameters:
        - name: resource_id
          in: path
          required: true
          schema:
            type: string
            format: uuid
        - name: max_depth
          in: query
          schema:
            type: integer
            default: 3
            minimum: 1
            maximum: 10
      responses:
        '200':
          description: Blast radius data
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/BlastRadius'

  /graph/shortest-path:
    get:
      tags: [Graph]
      summary: Find shortest path between resources
      parameters:
        - name: from
          in: query
          required: true
          schema:
            type: string
            format: uuid
        - name: to
          in: query
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: Shortest path
          content:
            application/json:
              schema:
                type: object
                properties:
                  path:
                    type: array
                    items:
                      type: string
                      format: uuid
                  length:
                    type: integer

  /attack-simulation/simulate:
    post:
      tags: [Attack Simulation]
      summary: Simulate attack scenario
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - compromised_id
                - objective
              properties:
                compromised_id:
                  type: string
                  format: uuid
                objective:
                  type: string
                  enum: [access-secrets, privilege-escalation, network-access]
      responses:
        '201':
          description: Simulation created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AttackScenario'

  /attack-simulation/scenarios:
    get:
      tags: [Attack Simulation]
      summary: List attack scenarios
      parameters:
        - name: page
          in: query
          schema:
            type: integer
        - name: per_page
          in: query
          schema:
            type: integer
      responses:
        '200':
          description: List of scenarios
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/AttackScenario'
                  meta:
                    $ref: '#/components/schemas/PaginationMeta'

  /policies:
    get:
      tags: [Policies]
      summary: List policies
      parameters:
        - name: type
          in: query
          schema:
            type: string
            enum: [kubearmor, networkpolicy, psp]
        - name: status
          in: query
          schema:
            type: string
            enum: [draft, preview, active, disabled]
        - name: page
          in: query
          schema:
            type: integer
        - name: per_page
          in: query
          schema:
            type: integer
      responses:
        '200':
          description: List of policies
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/Policy'
                  meta:
                    $ref: '#/components/schemas/PaginationMeta'

  /policies/{id}/apply:
    post:
      tags: [Policies]
      summary: Apply policy to cluster
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: Policy applied
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Policy'

  /users:
    get:
      tags: [Users]
      summary: List users (admin only)
      responses:
        '200':
          description: List of users
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/User'
```

---

## API Endpoints

### Authentication API

#### POST /auth/login

Login with username and password.

**Request**:
```json
{
  "username": "admin@example.com",
  "password": "SecurePassword123!"
}
```

**Response** (200):
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 1800,
  "user": {
    "id": "user-123",
    "username": "admin@example.com",
    "email": "admin@example.com",
    "role": "admin",
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

#### POST /auth/refresh

Refresh access token using refresh token.

**Request**:
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response** (200):
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 1800
}
```

#### POST /auth/logout

Logout and invalidate tokens.

**Request**: Empty body

**Response** (204): No content

---

### Clusters API

#### GET /clusters

List all clusters.

**Query Parameters**:
- `page` (integer): Page number (default: 1)
- `per_page` (integer): Items per page (default: 20, max: 100)

**Response** (200):
```json
{
  "data": [
    {
      "id": "cluster-123",
      "name": "production-us-east-1",
      "config": {
        "region": "us-east-1",
        "k8s_version": "1.28"
      },
      "status": "active",
      "created_at": "2025-01-01T00:00:00Z",
      "updated_at": "2025-11-28T10:00:00Z"
    }
  ],
  "meta": {
    "total": 5,
    "page": 1,
    "per_page": 20,
    "total_pages": 1
  }
}
```

#### GET /clusters/{id}

Get cluster details.

**Response** (200):
```json
{
  "id": "cluster-123",
  "name": "production-us-east-1",
  "config": {...},
  "status": "active",
  "statistics": {
    "pods": 1250,
    "service_accounts": 85,
    "namespaces": 42,
    "high_risk_count": 12
  },
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-11-28T10:00:00Z"
}
```

---

### ServiceAccounts API

#### GET /serviceaccounts

List ServiceAccounts with filtering and pagination.

**Query Parameters**:
- `cluster_id` (uuid): Filter by cluster
- `namespace` (string): Filter by namespace
- `status` (enum): Filter by status (active, orphaned, unused)
- `min_risk_score` (float): Minimum risk score
- `sort` (string): Sort field (default: risk_score desc)
- `page` (integer): Page number
- `per_page` (integer): Items per page

**Response** (200):
```json
{
  "data": [
    {
      "id": "sa-123",
      "cluster_id": "cluster-123",
      "namespace": "default",
      "name": "app-serviceaccount",
      "secrets": ["secret-1", "secret-2"],
      "last_used": "2025-11-28T09:30:00Z",
      "risk_score": 7.5,
      "status": "active",
      "permissions": {
        "roles": ["editor"],
        "cluster_roles": []
      },
      "created_at": "2025-01-01T00:00:00Z"
    }
  ],
  "meta": {
    "total": 85,
    "page": 1,
    "per_page": 20,
    "total_pages": 5
  }
}
```

#### GET /serviceaccounts/{id}

Get ServiceAccount details including permissions and usage.

**Response** (200):
```json
{
  "id": "sa-123",
  "cluster_id": "cluster-123",
  "namespace": "default",
  "name": "app-serviceaccount",
  "secrets": ["secret-1", "secret-2"],
  "last_used": "2025-11-28T09:30:00Z",
  "risk_score": 7.5,
  "status": "active",
  "permissions": {
    "roles": [
      {
        "name": "editor",
        "namespace": "default",
        "rules": [
          {
            "apiGroups": [""],
            "resources": ["pods"],
            "verbs": ["get", "list", "create"]
          }
        ]
      }
    ],
    "cluster_roles": []
  },
  "used_by_pods": [
    {
      "id": "pod-456",
      "name": "app-pod-1",
      "namespace": "default"
    }
  ],
  "insights": [
    {
      "id": "insight-789",
      "severity": "high",
      "title": "Overprivileged ServiceAccount"
    }
  ],
  "created_at": "2025-01-01T00:00:00Z"
}
```

#### GET /serviceaccounts/{id}/timeline

Get usage timeline for ServiceAccount.

**Query Parameters**:
- `start` (datetime): Start time (ISO 8601)
- `end` (datetime): End time (ISO 8601)
- `interval` (string): Time interval (1h, 1d, 1w)

**Response** (200):
```json
{
  "service_account_id": "sa-123",
  "timeline": [
    {
      "timestamp": "2025-11-28T09:00:00Z",
      "event_type": "api_access",
      "resource": "pods",
      "verb": "list",
      "pod": "app-pod-1"
    },
    {
      "timestamp": "2025-11-28T09:15:00Z",
      "event_type": "secret_access",
      "resource": "secrets/api-key",
      "verb": "get"
    }
  ]
}
```

---

### Insights API

#### GET /insights

List security insights with filtering.

**Query Parameters**:
- `cluster_id` (uuid): Filter by cluster
- `severity` (enum): critical, high, medium, low, info
- `status` (enum): active, resolved, ignored
- `category` (string): rbac, pod-security, network-policy, etc.
- `resource_type` (string): ServiceAccount, Pod, Role, etc.
- `resource_id` (uuid): Specific resource
- `sort` (string): Sort field (default: detected_at desc)
- `page`, `per_page`: Pagination

**Response** (200):
```json
{
  "data": [
    {
      "id": "insight-123",
      "rule_id": "cis-5.1.3",
      "cluster_id": "cluster-123",
      "resource_type": "ServiceAccount",
      "resource_id": "sa-123",
      "title": "ServiceAccount with cluster-admin binding",
      "description": "ServiceAccount 'default' in namespace 'kube-system' has cluster-admin permissions",
      "severity": "critical",
      "category": "rbac",
      "risk_score": 9.5,
      "status": "active",
      "cis_section": "5.1.3",
      "cis_level": 1,
      "remediation": {
        "description": "Remove cluster-admin binding and use least-privilege roles",
        "steps": [
          "1. Review required permissions",
          "2. Create custom Role with minimal permissions",
          "3. Delete cluster-admin binding",
          "4. Create new binding with custom role"
        ],
        "auto_fix": false
      },
      "detected_at": "2025-11-28T10:00:00Z",
      "resolved_at": null
    }
  ],
  "meta": {
    "total": 47,
    "page": 1,
    "per_page": 20,
    "total_pages": 3
  },
  "summary": {
    "by_severity": {
      "critical": 5,
      "high": 12,
      "medium": 20,
      "low": 10
    },
    "by_status": {
      "active": 42,
      "resolved": 3,
      "ignored": 2
    }
  }
}
```

#### GET /insights/{id}

Get insight details.

**Response** (200):
```json
{
  "id": "insight-123",
  "rule_id": "cis-5.1.3",
  "cluster_id": "cluster-123",
  "resource_type": "ServiceAccount",
  "resource_id": "sa-123",
  "resource": {
    "id": "sa-123",
    "name": "default",
    "namespace": "kube-system"
  },
  "title": "ServiceAccount with cluster-admin binding",
  "description": "...",
  "severity": "critical",
  "category": "rbac",
  "risk_score": 9.5,
  "status": "active",
  "cis_section": "5.1.3",
  "cis_level": 1,
  "remediation": {...},
  "evidence": {
    "binding_name": "default-cluster-admin",
    "role_name": "cluster-admin",
    "created_at": "2025-01-01T00:00:00Z"
  },
  "detected_at": "2025-11-28T10:00:00Z",
  "resolved_at": null
}
```

#### PATCH /insights/{id}

Update insight status (resolve or ignore).

**Request**:
```json
{
  "status": "resolved",
  "resolution_note": "Fixed by removing cluster-admin binding"
}
```

**Response** (200):
```json
{
  "id": "insight-123",
  "status": "resolved",
  "resolved_at": "2025-11-28T11:00:00Z",
  "resolved_by": "user-123"
}
```

---

### Graph API

#### GET /graph/blast-radius/{resource_id}

Calculate blast radius for a resource.

**Path Parameters**:
- `resource_id` (uuid): Resource ID

**Query Parameters**:
- `max_depth` (integer): Maximum traversal depth (default: 3, max: 10)

**Response** (200):
```json
{
  "source_id": "sa-123",
  "source": {
    "type": "ServiceAccount",
    "name": "app-sa",
    "namespace": "default"
  },
  "resources": [
    {
      "id": "pod-456",
      "type": "Pod",
      "name": "app-pod-1",
      "namespace": "default",
      "risk_score": 5.0,
      "distance": 1,
      "path": ["sa-123", "pod-456"]
    },
    {
      "id": "secret-789",
      "type": "Secret",
      "name": "api-key",
      "namespace": "default",
      "risk_score": 8.0,
      "distance": 2,
      "path": ["sa-123", "pod-456", "secret-789"]
    }
  ],
  "summary": {
    "total": 15,
    "by_type": {
      "Pod": 10,
      "Secret": 3,
      "Service": 2
    },
    "high_risk_count": 3,
    "critical_count": 1,
    "total_risk_score": 67.5
  }
}
```

#### GET /graph/shortest-path

Find shortest path between two resources.

**Query Parameters**:
- `from` (uuid): Source resource ID
- `to` (uuid): Target resource ID

**Response** (200):
```json
{
  "from": "sa-123",
  "to": "secret-789",
  "path": [
    {
      "id": "sa-123",
      "type": "ServiceAccount",
      "name": "app-sa"
    },
    {
      "id": "pod-456",
      "type": "Pod",
      "name": "app-pod-1",
      "edge": "USES"
    },
    {
      "id": "secret-789",
      "type": "Secret",
      "name": "api-key",
      "edge": "MOUNTS"
    }
  ],
  "length": 2,
  "risk_score": 7.5
}
```

#### GET /graph/neighborhood/{resource_id}

Get immediate neighbors of a resource.

**Query Parameters**:
- `depth` (integer): Neighborhood depth (default: 1)

**Response** (200):
```json
{
  "resource_id": "sa-123",
  "neighbors": [
    {
      "id": "pod-456",
      "type": "Pod",
      "name": "app-pod-1",
      "relationship": "USED_BY"
    },
    {
      "id": "role-789",
      "type": "Role",
      "name": "editor",
      "relationship": "HAS_PERMISSION"
    }
  ]
}
```

#### GET /graph/criticality-scores

Get PageRank-based criticality scores for all resources.

**Response** (200):
```json
{
  "scores": [
    {
      "resource_id": "sa-123",
      "resource_type": "ServiceAccount",
      "name": "kube-system/default",
      "criticality_score": 0.85
    },
    {
      "resource_id": "secret-456",
      "resource_type": "Secret",
      "name": "default/api-key",
      "criticality_score": 0.72
    }
  ]
}
```

---

### Attack Simulation API

#### POST /attack-simulation/simulate

Simulate an attack scenario.

**Request**:
```json
{
  "compromised_id": "sa-123",
  "objective": "access-secrets",
  "options": {
    "max_depth": 5,
    "include_lateral_moves": true
  }
}
```

**Response** (201):
```json
{
  "id": "scenario-abc123",
  "name": "ServiceAccount Compromise Simulation",
  "compromised_id": "sa-123",
  "compromised_type": "ServiceAccount",
  "compromised_name": "app-sa",
  "objective": "access-secrets",
  "blast_radius": {
    "source_id": "sa-123",
    "resources": [...],
    "secrets": 5,
    "pods": 12,
    "services": 3,
    "high_risk_count": 3,
    "total_risk_score": 45.5
  },
  "attack_paths": [
    {
      "nodes": [
        {
          "id": "sa-123",
          "type": "ServiceAccount",
          "name": "app-sa",
          "risk_score": 5.0
        },
        {
          "id": "pod-456",
          "type": "Pod",
          "name": "app-pod-1",
          "risk_score": 4.0
        },
        {
          "id": "secret-789",
          "type": "Secret",
          "name": "api-key",
          "risk_score": 8.0
        }
      ],
      "edges": ["USES", "MOUNTS"],
      "total_risk": 8.5,
      "difficulty": 0.2,
      "impact": 0.9,
      "length": 2,
      "description": "Attacker compromises ServiceAccount 'app-sa' → uses Pod 'app-pod-1' → mounts Secret 'api-key'"
    }
  ],
  "lateral_moves": [
    {
      "from": "pod-456",
      "to": "pod-789",
      "method": "Same namespace access",
      "difficulty": 0.3,
      "prerequisites": ["Network access"],
      "description": "Lateral move from app-pod-1 to db-pod-1 via service"
    }
  ],
  "risk_score": 8.5,
  "simulated_at": "2025-11-28T11:00:00Z"
}
```

#### GET /attack-simulation/scenarios

List attack scenarios.

**Query Parameters**:
- `cluster_id` (uuid): Filter by cluster
- `min_risk_score` (float): Minimum risk score
- `page`, `per_page`: Pagination

**Response** (200):
```json
{
  "data": [
    {
      "id": "scenario-abc123",
      "name": "ServiceAccount Compromise",
      "compromised_type": "ServiceAccount",
      "objective": "access-secrets",
      "risk_score": 8.5,
      "attack_paths_count": 3,
      "blast_radius_size": 20,
      "simulated_at": "2025-11-28T11:00:00Z"
    }
  ],
  "meta": {
    "total": 12,
    "page": 1,
    "per_page": 20,
    "total_pages": 1
  }
}
```

#### GET /attack-simulation/scenarios/{id}

Get scenario details (same as simulate response).

#### POST /attack-simulation/scenarios/{id}/playbook

Generate incident response playbook for a scenario.

**Response** (200):
```json
{
  "id": "playbook-xyz789",
  "scenario_id": "scenario-abc123",
  "title": "IR Playbook: ServiceAccount 'app-sa' Compromise",
  "detection": [
    {
      "order": 1,
      "action": "Monitor for unusual API calls",
      "description": "Watch for API calls from ServiceAccount 'app-sa'",
      "tools": ["KSAM Audit Logs", "K8s Audit Logs"]
    }
  ],
  "containment": [
    {
      "order": 1,
      "action": "Quarantine compromised pods",
      "description": "Add quarantine label",
      "command": "kubectl label pod app-pod-1 security.ksam.io/quarantine=true"
    }
  ],
  "eradication": [...],
  "recovery": [...],
  "lessons_learned": [
    "ServiceAccount 'app-sa' had excessive permissions",
    "Blast radius included 5 secrets"
  ],
  "created_at": "2025-11-28T11:05:00Z"
}
```

---

### Policies API

#### GET /policies

List policies.

**Query Parameters**:
- `type` (enum): kubearmor, networkpolicy, psp
- `status` (enum): draft, preview, active, disabled
- `target_resource_id` (uuid): Filter by target resource
- `min_confidence` (float): Minimum confidence score
- `page`, `per_page`: Pagination

**Response** (200):
```json
{
  "data": [
    {
      "id": "policy-123",
      "name": "nginx-security-policy",
      "type": "kubearmor",
      "target_resource_id": "pod-456",
      "target_resource": {
        "type": "Pod",
        "name": "nginx-pod",
        "namespace": "default"
      },
      "spec": {
        "apiVersion": "security.kubearmor.com/v1",
        "kind": "KubeArmorPolicy",
        ...
      },
      "status": "active",
      "confidence": 0.85,
      "generated_at": "2025-11-20T10:00:00Z",
      "applied_at": "2025-11-21T09:00:00Z"
    }
  ],
  "meta": {
    "total": 35,
    "page": 1,
    "per_page": 20,
    "total_pages": 2
  }
}
```

#### GET /policies/{id}

Get policy details.

**Response** (200):
```json
{
  "id": "policy-123",
  "name": "nginx-security-policy",
  "type": "kubearmor",
  "target_resource_id": "pod-456",
  "target_resource": {...},
  "spec": {
    "apiVersion": "security.kubearmor.com/v1",
    "kind": "KubeArmorPolicy",
    "metadata": {
      "name": "nginx-security-policy",
      "namespace": "default"
    },
    "spec": {
      "selector": {
        "matchLabels": {
          "app": "nginx"
        }
      },
      "process": {
        "matchPaths": [
          {"path": "/usr/sbin/nginx"},
          {"path": "/bin/sh"}
        ]
      },
      "file": {
        "matchPaths": [
          {"path": "/etc/nginx/", "readOnly": true}
        ]
      },
      "action": "Block"
    }
  },
  "status": "active",
  "confidence": 0.85,
  "baseline": {
    "observation_days": 14,
    "events_observed": 5420,
    "processes": ["/usr/sbin/nginx", "/bin/sh"],
    "files_accessed": ["/etc/nginx/nginx.conf", "/var/log/nginx/"]
  },
  "violations": [
    {
      "timestamp": "2025-11-28T09:00:00Z",
      "type": "blocked_process",
      "details": "Blocked execution of /usr/bin/curl"
    }
  ],
  "generated_at": "2025-11-20T10:00:00Z",
  "applied_at": "2025-11-21T09:00:00Z"
}
```

#### POST /policies/generate/{resource_id}

Generate policy for a resource.

**Path Parameters**:
- `resource_id` (uuid): Target resource ID

**Request**:
```json
{
  "type": "kubearmor",
  "options": {
    "min_confidence": 0.8,
    "observation_days": 14
  }
}
```

**Response** (201):
```json
{
  "id": "policy-456",
  "name": "auto-generated-policy-pod-456",
  "type": "kubearmor",
  "target_resource_id": "pod-456",
  "spec": {...},
  "status": "draft",
  "confidence": 0.85,
  "generated_at": "2025-11-28T12:00:00Z"
}
```

#### POST /policies/{id}/preview

Preview policy impact before applying.

**Response** (200):
```json
{
  "policy_id": "policy-123",
  "status": "preview",
  "confidence": 0.85,
  "predicted_impacts": {
    "blocked_processes": [
      {"process": "/usr/bin/curl", "frequency": 5, "last_seen": "2025-11-28T10:00:00Z"}
    ],
    "blocked_connections": [
      {"dest_ip": "1.2.3.4", "port": 443, "frequency": 10}
    ],
    "blocked_file_access": []
  },
  "false_positive_risk": "low",
  "recommendation": "safe_to_apply"
}
```

#### POST /policies/{id}/apply

Apply policy to cluster.

**Request**:
```json
{
  "dry_run": false,
  "monitor_mode": false
}
```

**Response** (200):
```json
{
  "id": "policy-123",
  "status": "active",
  "applied_at": "2025-11-28T12:30:00Z",
  "result": {
    "success": true,
    "message": "Policy applied successfully"
  }
}
```

#### DELETE /policies/{id}

Delete policy from cluster and database.

**Response** (204): No content

---

### Compliance API

#### GET /compliance/frameworks

List available compliance frameworks.

**Response** (200):
```json
{
  "frameworks": [
    {
      "id": "cis-1.8",
      "name": "CIS Kubernetes Benchmark v1.8",
      "version": "1.8",
      "sections": 5,
      "checks": 28
    },
    {
      "id": "soc2",
      "name": "SOC 2 Type II",
      "controls": 15
    }
  ]
}
```

#### GET /compliance/check

Run compliance check.

**Query Parameters**:
- `framework` (string): Framework ID (e.g., cis-1.8)
- `cluster_id` (uuid): Target cluster

**Response** (200):
```json
{
  "framework": "cis-1.8",
  "cluster_id": "cluster-123",
  "scan_date": "2025-11-28T12:00:00Z",
  "overall_score": 0.78,
  "passed": 22,
  "failed": 6,
  "total": 28,
  "results": [
    {
      "check_id": "cis-5.1.3",
      "section": "5.1.3",
      "title": "Ensure that service accounts are not granted cluster-admin",
      "level": 1,
      "status": "failed",
      "severity": "critical",
      "findings": [
        {
          "resource": "ServiceAccount:kube-system/default",
          "issue": "Has cluster-admin binding"
        }
      ],
      "remediation": "Remove cluster-admin binding"
    }
  ]
}
```

#### GET /compliance/report

Generate compliance report.

**Query Parameters**:
- `framework` (string): Framework ID
- `cluster_id` (uuid): Target cluster
- `format` (enum): json, pdf, html

**Response** (200):
```json
{
  "report_id": "report-abc123",
  "framework": "cis-1.8",
  "cluster_id": "cluster-123",
  "generated_at": "2025-11-28T13:00:00Z",
  "summary": {
    "overall_score": 0.78,
    "passed": 22,
    "failed": 6,
    "by_severity": {
      "critical": 2,
      "high": 3,
      "medium": 1
    }
  },
  "download_url": "/api/v1/compliance/reports/report-abc123/download"
}
```

---

### Audit Logs API

#### GET /audit-logs

List audit logs.

**Query Parameters**:
- `user_id` (uuid): Filter by user
- `action` (string): Filter by action
- `resource_type` (string): Filter by resource type
- `start` (datetime): Start time
- `end` (datetime): End time
- `page`, `per_page`: Pagination

**Response** (200):
```json
{
  "data": [
    {
      "id": "log-123",
      "timestamp": "2025-11-28T12:00:00Z",
      "level": "info",
      "event_type": "policy_applied",
      "user_id": "user-123",
      "user_email": "admin@example.com",
      "user_role": "admin",
      "action": "apply_policy",
      "resource_type": "Policy",
      "resource_id": "policy-123",
      "resource_name": "nginx-security-policy",
      "cluster_id": "cluster-123",
      "namespace": "default",
      "result": "success",
      "client_ip": "10.0.1.50",
      "duration_ms": 125
    }
  ],
  "meta": {
    "total": 1520,
    "page": 1,
    "per_page": 50,
    "total_pages": 31
  }
}
```

---

### Users API

#### GET /users

List users (admin only).

**Response** (200):
```json
{
  "data": [
    {
      "id": "user-123",
      "username": "admin@example.com",
      "email": "admin@example.com",
      "role": "admin",
      "created_at": "2025-01-01T00:00:00Z",
      "last_login": "2025-11-28T09:00:00Z"
    }
  ]
}
```

#### POST /users

Create user (admin only).

**Request**:
```json
{
  "username": "newuser@example.com",
  "email": "newuser@example.com",
  "password": "SecurePassword123!",
  "role": "user"
}
```

**Response** (201):
```json
{
  "id": "user-456",
  "username": "newuser@example.com",
  "email": "newuser@example.com",
  "role": "user",
  "created_at": "2025-11-28T13:00:00Z"
}
```

#### PATCH /users/{id}

Update user (admin or self).

**Request**:
```json
{
  "role": "admin"
}
```

**Response** (200):
```json
{
  "id": "user-456",
  "username": "newuser@example.com",
  "role": "admin",
  "updated_at": "2025-11-28T13:05:00Z"
}
```

#### DELETE /users/{id}

Delete user (admin only).

**Response** (204): No content

---

## WebSocket API

### Real-Time Event Stream

**Endpoint**: `wss://ksam-core.example.com/api/v1/events/stream`

**Authentication**: JWT token in query parameter or Sec-WebSocket-Protocol header

**Message Format**:
```json
{
  "type": "insight_created",
  "data": {
    "id": "insight-123",
    "severity": "critical",
    "title": "ServiceAccount with cluster-admin binding",
    "cluster_id": "cluster-123"
  },
  "timestamp": "2025-11-28T14:00:00Z"
}
```

**Event Types**:
- `insight_created`
- `insight_resolved`
- `policy_applied`
- `policy_violated`
- `anomaly_detected`
- `cluster_status_changed`

**Example (JavaScript)**:
```javascript
const ws = new WebSocket('wss://ksam-core/api/v1/events/stream?token=JWT_TOKEN');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event received:', data.type, data.data);
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};
```

---

## gRPC API

### Agent Communication Protocol

**Service Definition** (`fortuna_agent.proto`):

```protobuf
syntax = "proto3";

package fortuna_agent;

service AgentService {
  // Agent registration
  rpc Register(RegisterRequest) returns (RegisterResponse);
  
  // Stream inventory items
  rpc StreamInventory(stream InventoryItem) returns (stream Ack);
  
  // Stream runtime events
  rpc StreamEvents(stream Event) returns (stream Ack);
  
  // Heartbeat
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
}

message RegisterRequest {
  string agent_id = 1;
  string cluster_id = 2;
  string node_name = 3;
  string version = 4;
  map<string, string> metadata = 5;
}

message RegisterResponse {
  bool ok = 1;
  string message = 2;
  string agent_id = 3;
}

message InventoryItem {
  string item_id = 1;
  string item_type = 2;  // Pod, ServiceAccount, etc.
  string cluster_id = 3;
  string namespace = 4;
  bytes data = 5;  // JSON serialized
  int64 timestamp = 6;
}

message Event {
  string event_id = 1;
  string event_type = 2;  // execve, connect, open
  string cluster_id = 3;
  string pod_id = 4;
  bytes data = 5;  // JSON serialized
  int64 timestamp = 6;
}

message Ack {
  bool ok = 1;
  string error = 2;
}

message HeartbeatRequest {
  string agent_id = 1;
  AgentStatus status = 2;
}

message AgentStatus {
  int32 memory_mb = 1;
  float cpu_percent = 2;
  int32 events_collected = 3;
  int32 events_filtered = 4;
}

message HeartbeatResponse {
  bool ok = 1;
  float load = 2;  // 0.0-1.0
  float recommended_sampling_rate = 3;
  bool overloaded = 4;
}
```

**Connection**: `ksam-core.ksam.svc.cluster.local:9090`

**Security**: mTLS required

---

## Examples

### Example 1: Login and Get Insights

```bash
# Login
TOKEN=$(curl -s -X POST http://ksam-core:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin@example.com","password":"password"}' \
  | jq -r '.access_token')

# Get critical insights
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?severity=critical&status=active" \
  | jq
```

### Example 2: Run Attack Simulation

```bash
# Get ServiceAccount ID
SA_ID=$(curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/serviceaccounts?namespace=default&name=app-sa" \
  | jq -r '.data[0].id')

# Simulate attack
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://ksam-core:8080/api/v1/attack-simulation/simulate \
  -d "{\"compromised_id\":\"$SA_ID\",\"objective\":\"access-secrets\"}" \
  | jq
```

### Example 3: Generate and Apply Policy

```bash
# Generate policy for pod
POD_ID="pod-456"
POLICY=$(curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://ksam-core:8080/api/v1/policies/generate/$POD_ID \
  -d '{"type":"kubearmor"}')

POLICY_ID=$(echo $POLICY | jq -r '.id')

# Preview policy
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://ksam-core:8080/api/v1/policies/$POLICY_ID/preview \
  | jq

# Apply policy
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://ksam-core:8080/api/v1/policies/$POLICY_ID/apply \
  -d '{"dry_run":false}' \
  | jq
```

### Example 4: Get Blast Radius

```bash
# Get blast radius for ServiceAccount
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/graph/blast-radius/$SA_ID?max_depth=5" \
  | jq
```

### Example 5: Export Compliance Report

```bash
# Run compliance check
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/compliance/check?framework=cis-1.8&cluster_id=$CLUSTER_ID" \
  | jq

# Generate PDF report
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/compliance/report?framework=cis-1.8&cluster_id=$CLUSTER_ID&format=pdf" \
  -o compliance-report.pdf
```

---

**End of API Specification**

For questions or issues, contact: api-support@ksam.io

**Useful Tools**:
- Swagger UI: `https://ksam-core/swagger-ui/`
- Postman Collection: [Download](https://ksam.io/postman-collection.json)
- API Status: `https://status.ksam.io`

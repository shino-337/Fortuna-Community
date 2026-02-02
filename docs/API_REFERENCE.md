# Fortuna Platform - API Reference

**Last Updated**: 2026-01-06  
**Version**: 3.0  
**Status**: Production Ready

---

## Table of Contents

1. [Base URL & Authentication](#base-url--authentication)
2. [Health & Status Endpoints](#health--status-endpoints)
3. [Authentication](#authentication)
4. [Insights API](#insights-api)
5. [SBOM API](#sbom-api)
6. [CVE API](#cve-api)
7. [Risk API](#risk-api)
8. [Clusters API](#clusters-api)
9. [Pods API](#pods-api)
10. [Pod Capabilities API (PCE)](#pod-capabilities-api-pce)
11. [Service Accounts API](#service-accounts-api)
12. [Graph API](#graph-api)
13. [Audit API](#audit-api)
14. [Policy API](#policy-api)
15. [Metrics API](#metrics-api)
16. [Error Responses](#error-responses)

---

## Base URL & Authentication

### Base URL

```
http://fortuna-core.fortuna.svc.cluster.local:8080
```

For local development:
```
http://localhost:8080
```

### Authentication

The API supports JWT-based authentication. Authentication can be enabled/disabled via the `AUTH_ENABLED` environment variable.

**When Authentication is Enabled:**
- Most endpoints require a JWT token in the `Authorization` header
- Format: `Authorization: Bearer <token>`
- Token obtained via `/api/v1/auth/login`

**When Authentication is Disabled:**
- All endpoints are publicly accessible
- No authentication required

---

## Health & Status Endpoints

### Health Check

**GET** `/health`

**Description**: Check system health and database connectivity

**Authentication**: Not required

**Response**:
```json
{
  "status": "healthy",
  "timestamp": "2026-01-06T12:00:00.000Z",
  "checks": {
    "database": "ok"
  }
}
```

**Status Codes**:
- `200 OK`: System is healthy
- `503 Service Unavailable`: System is unhealthy

---

### Readiness Check

**GET** `/ready`

**Description**: Check if the service is ready to accept traffic

**Authentication**: Not required

**Response**:
```json
{
  "status": "ready",
  "timestamp": "2026-01-06T12:00:00.000Z"
}
```

**Status Codes**:
- `200 OK`: Service is ready
- `503 Service Unavailable`: Service is not ready

---

### Liveness Check

**GET** `/live`

**Description**: Check if the service is alive

**Authentication**: Not required

**Response**:
```json
{
  "status": "alive"
}
```

**Status Codes**:
- `200 OK`: Service is alive

---

### Prometheus Metrics

**GET** `/metrics`

**Description**: Prometheus metrics endpoint

**Authentication**: Not required

**Response**: Prometheus metrics format

---

## Authentication

### Login

**POST** `/api/v1/auth/login`

**Description**: Authenticate and receive JWT token

**Authentication**: Not required

**Request Body**:
```json
{
  "username": "admin",
  "password": "password"
}
```

**Response**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 3600,
  "user": {
    "id": 1,
    "username": "admin",
    "role": "admin"
  }
}
```

**Status Codes**:
- `200 OK`: Authentication successful
- `401 Unauthorized`: Invalid credentials

---

### Register

**POST** `/api/v1/auth/register`

**Description**: Register a new user

**Authentication**: 
- If `AUTH_ENABLED=true`: Requires admin token
- If `AUTH_ENABLED=false`: No authentication required

**Request Body**:
```json
{
  "username": "newuser",
  "password": "securepassword",
  "email": "user@example.com"
}
```

**Response**:
```json
{
  "id": 2,
  "username": "newuser",
  "email": "user@example.com",
  "role": "user",
  "createdAt": "2026-01-06T12:00:00.000Z"
}
```

**Status Codes**:
- `201 Created`: User created successfully
- `400 Bad Request`: Invalid input
- `403 Forbidden`: Admin access required (when auth enabled)

---

## Insights API

### Get Insights

**GET** `/api/v1/insights`

**Description**: Retrieve security insights with filtering and pagination

**Authentication**: Required if `AUTH_ENABLED=true`

**Query Parameters**:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `resource_uid` | string | No | - | Filter by resource UID |
| `resource_type` | string | No | - | Filter by resource type (Pod, Node, etc.) |
| `resource_namespace` | string | No | - | Filter by Kubernetes namespace |
| `resource_name` | string | No | - | Filter by resource name |
| `insight_type` | string | No | - | Filter by insight type (vulnerability, misconfiguration, etc.) |
| `type` | string | No | - | Alias for `insight_type` (legacy support) |
| `severity` | string | No | - | Filter by severity (critical, high, medium, low) |
| `status` | string | No | `active` | Filter by status (active, resolved, dismissed, all) |
| `sbom_id` | integer | No | - | Filter by SBOM ID |
| `cluster` | string | No | - | Filter by cluster ID |
| `page` | integer | No | `1` | Page number |
| `pageSize` | integer | No | `50` | Items per page (max 100) |

**Example Request**:
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/insights?resource_uid=0c12ecc6-68f5-4f88-8caf-a4c61648ca09&severity=high&status=active"
```

**Response**:
```json
{
  "insights": [
    {
      "id": 1,
      "resourceType": "Pod",
      "resourceNamespace": "fortuna",
      "resourceName": "test-pod",
      "resourceUid": "0c12ecc6-68f5-4f88-8caf-a4c61648ca09",
      "insightType": "vulnerability",
      "severity": "high",
      "title": "CVE-2021-3711 in OpenSSL",
      "description": "Vulnerability CVE-2021-3711 (HIGH) detected in package openssl@1.1.1f",
      "recommendation": "Update to OpenSSL 1.1.1l or later",
      "cveId": "CVE-2021-3711",
      "affectedComponent": "openssl",
      "affectedVersion": "1.1.1f",
      "fixedVersion": "1.1.1l",
      "cvss": 7.5,
      "status": "active",
      "detectedAt": "2026-01-06T12:00:00.000Z",
      "createdAt": "2026-01-06T12:00:00.000Z",
      "updatedAt": "2026-01-06T12:00:00.000Z"
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 50
}
```

**Status Codes**:
- `200 OK`: Success
- `400 Bad Request`: Invalid query parameters
- `500 Internal Server Error`: Server error

---

### Get Insight Summary

**GET** `/api/v1/insights/summary`

**Description**: Get summary statistics of insights

**Authentication**: Required if `AUTH_ENABLED=true`

**Response**:
```json
{
  "total": 150,
  "critical": 5,
  "high": 25,
  "medium": 50,
  "low": 70,
  "byType": {
    "vulnerability": 100,
    "misconfiguration": 30,
    "compliance": 20
  }
}
```

---

### Get Insight by ID

**GET** `/api/v1/insights/:id`

**Description**: Get a specific insight by ID

**Authentication**: Required if `AUTH_ENABLED=true`

**Response**:
```json
{
  "id": 1,
  "resourceType": "Pod",
  "resourceNamespace": "fortuna",
  "resourceName": "test-pod",
  "resourceUid": "0c12ecc6-68f5-4f88-8caf-a4c61648ca09",
  "insightType": "vulnerability",
  "severity": "high",
  "title": "CVE-2021-3711 in OpenSSL",
  "description": "Vulnerability CVE-2021-3711 (HIGH) detected",
  "recommendation": "Update to OpenSSL 1.1.1l or later",
  "cveId": "CVE-2021-3711",
  "affectedComponent": "openssl",
  "affectedVersion": "1.1.1f",
  "fixedVersion": "1.1.1l",
  "cvss": 7.5,
  "status": "active",
  "detectedAt": "2026-01-06T12:00:00.000Z",
  "createdAt": "2026-01-06T12:00:00.000Z",
  "updatedAt": "2026-01-06T12:00:00.000Z"
}
```

**Status Codes**:
- `200 OK`: Success
- `404 Not Found`: Insight not found

---

### Acknowledge Insight

**POST** `/api/v1/insights/:id/acknowledge`

**Description**: Acknowledge an insight

**Authentication**: Required if `AUTH_ENABLED=true`

**Response**:
```json
{
  "id": 1,
  "status": "acknowledged",
  "updatedAt": "2026-01-06T12:00:00.000Z"
}
```

---

### Resolve Insight

**POST** `/api/v1/insights/:id/resolve`

**Description**: Mark an insight as resolved

**Authentication**: Required if `AUTH_ENABLED=true`

**Response**:
```json
{
  "id": 1,
  "status": "resolved",
  "updatedAt": "2026-01-06T12:00:00.000Z"
}
```

---

### Dismiss Insight

**POST** `/api/v1/insights/:id/dismiss`

**Description**: Dismiss an insight

**Authentication**: Required if `AUTH_ENABLED=true`

**Response**:
```json
{
  "id": 1,
  "status": "dismissed",
  "updatedAt": "2026-01-06T12:00:00.000Z"
}
```

---

### Delete Insight

**DELETE** `/api/v1/insights/:id`

**Description**: Delete an insight (soft delete)

**Authentication**: Required if `AUTH_ENABLED=true`

**Status Codes**:
- `200 OK`: Success
- `404 Not Found`: Insight not found

---

## SBOM API

> **Note**: SBOM data is primarily ingested via gRPC from Agents. REST API endpoints for SBOM retrieval are available through the database queries.

### Get SBOMs (via Database Query)

**Note**: Direct SBOM REST endpoints are not currently exposed. SBOM data can be accessed via:
- Insights API (filtered by `sbom_id`)
- Database queries
- gRPC API (for Agent communication)

**Future Endpoints** (planned):
- `GET /api/v1/sboms` - List SBOMs
- `GET /api/v1/sboms/:id` - Get SBOM by ID
- `GET /api/v1/sboms/:id/components` - Get SBOM components
- `GET /api/v1/sboms/:id/cves` - Get CVE matches for SBOM

---

## CVE API

> **Note**: CVE data is stored in the database and can be queried. Direct REST endpoints for CVE data are planned.

**Future Endpoints** (planned):
- `GET /api/v1/cves` - List CVEs
- `GET /api/v1/cves/:id` - Get CVE by ID
- `GET /api/v1/cves/:id/packages` - Get affected packages
- `GET /api/v1/cve-matches` - Get CVE matches with filtering

---

## Risk API

### Get Risk Scores

**GET** `/api/v1/risk/scores`

**Description**: Get all risk scores

**Authentication**: Required if `AUTH_ENABLED=true`

**Query Parameters**:
- `page`: Page number (default: 1)
- `pageSize`: Items per page (default: 50)

**Response**:
```json
{
  "scores": [
    {
      "uid": "pod-uid-123",
      "score": 7.5,
      "factors": {
        "vulnerabilities": 5.0,
        "permissions": 2.5
      },
      "updatedAt": "2026-01-06T12:00:00.000Z"
    }
  ],
  "total": 100,
  "page": 1,
  "pageSize": 50
}
```

---

### Get Risk Score by UID

**GET** `/api/v1/risk/scores/:uid`

**Description**: Get risk score for a specific resource

**Authentication**: Required if `AUTH_ENABLED=true`

**Response**:
```json
{
  "uid": "pod-uid-123",
  "score": 7.5,
  "factors": {
    "vulnerabilities": 5.0,
    "permissions": 2.5
  },
  "updatedAt": "2026-01-06T12:00:00.000Z"
}
```

---

### Calculate Risk Score

**POST** `/api/v1/risk/scores/:uid/calculate`

**Description**: Trigger risk score calculation for a resource

**Authentication**: Required if `AUTH_ENABLED=true`

**Response**:
```json
{
  "uid": "pod-uid-123",
  "score": 7.5,
  "status": "calculated",
  "updatedAt": "2026-01-06T12:00:00.000Z"
}
```

---

### Get Risk Trends

**GET** `/api/v1/risk/trends`

**Description**: Get risk trends over time

**Authentication**: Required if `AUTH_ENABLED=true`

**Query Parameters**:
- `days`: Number of days (default: 7)

**Response**:
```json
{
  "trends": [
    {
      "date": "2026-01-01",
      "averageScore": 6.5,
      "count": 100
    }
  ]
}
```

---

### Get Top Risks

**GET** `/api/v1/risk/top`

**Description**: Get top risks by score

**Authentication**: Required if `AUTH_ENABLED=true`

**Query Parameters**:
- `limit`: Number of results (default: 10)

**Response**:
```json
{
  "risks": [
    {
      "uid": "pod-uid-123",
      "score": 9.5,
      "resourceType": "Pod",
      "resourceName": "high-risk-pod"
    }
  ]
}
```

---

### Get Risk Analytics

**GET** `/api/v1/risk/analytics/trends`

**Description**: Get risk trend analytics

**Authentication**: Required if `AUTH_ENABLED=true`

**GET** `/api/v1/risk/analytics/comparison`

**Description**: Get risk comparison analytics

**GET** `/api/v1/risk/analytics/correlation`

**Description**: Get risk correlation analytics

---

## Clusters API

### Get Clusters

**GET** `/api/v1/clusters`

**Description**: List all clusters

**Authentication**: Required if `AUTH_ENABLED=true`

**Response**:
```json
{
  "clusters": [
    {
      "id": 1,
      "name": "production",
      "endpoint": "https://k8s.example.com",
      "status": "connected",
      "lastSync": "2026-01-06T12:00:00.000Z"
    }
  ]
}
```

---

### Get Cluster Stats

**GET** `/api/v1/clusters/stats`

**Description**: Get cluster statistics

**Authentication**: Required if `AUTH_ENABLED=true`

**Response**:
```json
{
  "clusters": [
    {
      "id": 1,
      "name": "production",
      "serviceAccountCount": 100,
      "podCount": 500,
      "deploymentCount": 50,
      "connectionStatus": "connected"
    }
  ]
}
```

---

### Get Cluster by ID

**GET** `/api/v1/clusters/:id`

**Description**: Get a specific cluster

**Authentication**: Required if `AUTH_ENABLED=true`

---

## Pods API

### Get Pods

**GET** `/api/v1/pods`

**Description**: List pods with optional filters

**Authentication**: Required if `AUTH_ENABLED=true`

**Query Parameters**:
- `cluster`: Filter by cluster ID
- `namespace`: Filter by namespace
- `serviceAccount`: Filter by service account
- `page`: Page number (default: 1)
- `pageSize`: Items per page (default: 50)

**Response**:
```json
{
  "pods": [
    {
      "id": 1,
      "uid": "pod-uid-123",
      "name": "test-pod",
      "namespace": "default",
      "clusterId": 1
    }
  ],
  "total": 100,
  "page": 1,
  "pageSize": 50
}
```

---

## Pod Capabilities API (PCE)

### List Pod Capabilities

**GET** `/api/v1/pod-capabilities`

**Description**: List evaluated pod capabilities with filters and pagination

**Authentication**: Optional (depends on `AUTH_ENABLED`)

**Query Params**:
- `podUid` (optional): Filter by pod UID
- `clusterId` (optional): Filter by cluster ID
- `namespace` (optional): Filter by namespace
- `capabilityId` (optional): Filter by capability ID (e.g., `FS_HOST_RW`)
- `severity` (optional): Filter by severity (e.g., `CRITICAL`)
- `limit` (optional): Default `50`, max `200`
- `offset` (optional): Default `0`

**Response**:
```json
{
  "capabilities": [
    {
      "podUid": "a16e7c70-...",
      "namespace": "kube-system",
      "capabilityId": "FS_HOST_RW",
      "group": "FS",
      "severity": "CRITICAL",
      "evidence": {
        "hostPath": true
      },
      "mitreTechniques": ["T1611"],
      "createdAt": "2026-01-26T06:24:11Z",
      "updatedAt": "2026-01-26T06:24:11Z"
    }
  ],
  "total": 40,
  "limit": 50,
  "offset": 0
}
```

**Status Codes**:
- `200 OK`
- `500 Internal Server Error`

---

### Get Pod Capabilities (by Pod UID)

**GET** `/api/v1/pods/{podUid}/capabilities`

**Description**: Get all capabilities for a specific pod UID

**Authentication**: Optional (depends on `AUTH_ENABLED`)

**Response**:
```json
{
  "podUid": "24228d0a-...",
  "capabilities": [
    {
      "podUid": "24228d0a-...",
      "namespace": "kube-flannel",
      "capabilityId": "API_K8S_WRITE",
      "group": "API",
      "severity": "HIGH",
      "evidence": {
        "role": "flannel",
        "roleKind": "ClusterRole"
      },
      "mitreTechniques": ["T1609"],
      "createdAt": "2026-01-26T06:24:11Z",
      "updatedAt": "2026-01-26T06:24:11Z"
    }
  ],
  "total": 4
}
```

**Status Codes**:
- `200 OK`
- `400 Bad Request`
- `500 Internal Server Error`

---

### Pod Capabilities Summary (by Cluster/Namespace)

**GET** `/api/v1/pod-capabilities/summary`

**Description**: Aggregated counts by cluster, namespace, capability, and severity

**Authentication**: Optional (depends on `AUTH_ENABLED`)

**Query Params**:
- `clusterId` (optional)
- `namespace` (optional)
- `capabilityId` (optional)
- `severity` (optional)

**Response**:
```json
{
  "summary": [
    {
      "clusterId": "minikube",
      "namespace": "kube-system",
      "capabilityId": "NET_HOST_NETWORK",
      "severity": "MEDIUM",
      "count": 8
    }
  ],
  "total": 6
}
```

**Status Codes**:
- `200 OK`
- `500 Internal Server Error`

---

### Pod Capabilities Summary (by Cluster Only)

**GET** `/api/v1/pod-capabilities/summary/cluster`

**Description**: Aggregated counts by cluster only

**Authentication**: Optional (depends on `AUTH_ENABLED`)

**Query Params**:
- `clusterId` (optional)

**Response**:
```json
{
  "summary": [
    { "clusterId": "minikube", "count": 40 }
  ],
  "total": 1
}
```

**Status Codes**:
- `200 OK`
- `500 Internal Server Error`

---

### Pod Capabilities Summary (by Capability Only)

**GET** `/api/v1/pod-capabilities/summary/capability`

**Description**: Aggregated counts by capability and severity

**Authentication**: Optional (depends on `AUTH_ENABLED`)

**Query Params**:
- `capabilityId` (optional)
- `severity` (optional)

**Response**:
```json
{
  "summary": [
    { "capabilityId": "FS_HOST_RW", "severity": "CRITICAL", "count": 8 }
  ],
  "total": 6
}
```

**Status Codes**:
- `200 OK`
- `500 Internal Server Error`

---

### Pod Capabilities Summary (by Namespace)

**GET** `/api/v1/pod-capabilities/summary/namespace`

**Description**: Aggregated counts by namespace and severity

**Authentication**: Optional (depends on `AUTH_ENABLED`)

**Query Params**:
- `namespace` (optional)
- `severity` (optional)
- `capabilityId` (optional)

**Response**:
```json
{
  "summary": [
    { "namespace": "kube-system", "severity": "MEDIUM", "count": 8 }
  ],
  "total": 1
}
```

**Status Codes**:
- `200 OK`
- `500 Internal Server Error`

---

### Pod Capabilities Summary (by Severity Only)

**GET** `/api/v1/pod-capabilities/summary/severity`

**Description**: Aggregated counts by severity only

**Authentication**: Optional (depends on `AUTH_ENABLED`)

**Query Params**:
- `severity` (optional)
- `capabilityId` (optional)

**Response**:
```json
{
  "summary": [
    { "severity": "CRITICAL", "count": 10 }
  ],
  "total": 4
}
```

**Status Codes**:
- `200 OK`
- `500 Internal Server Error`

---

### Pod Capabilities Trend (Time Series)

**GET** `/api/v1/pod-capabilities/trends`

**Description**: Time-series counts by severity (daily)

**Authentication**: Optional (depends on `AUTH_ENABLED`)

**Query Params**:
- `days` (optional, default `7`, max `90`)
- `namespace` (optional)
- `capabilityId` (optional)
- `podUid` (optional)
- `clusterId` (optional)

**Response**:
```json
{
  "points": [
    { "date": "2026-01-26", "critical": 2, "high": 3, "medium": 5, "low": 1 }
  ],
  "total": 1
}
```

**Status Codes**:
- `200 OK`
- `500 Internal Server Error`

---

### Get Pod by ID

**GET** `/api/v1/pods/:id`

**Description**: Get a specific pod

**Authentication**: Required if `AUTH_ENABLED=true`

---

## Service Accounts API

### Get Service Accounts

**GET** `/api/v1/serviceaccounts`

**Description**: List service accounts with filters

**Authentication**: Required if `AUTH_ENABLED=true`

**Query Parameters**:
- `cluster`: Filter by cluster ID
- `namespace`: Filter by namespace
- `page`: Page number
- `pageSize`: Items per page

---

### Get Service Account by ID

**GET** `/api/v1/serviceaccounts/:id`

**Description**: Get a specific service account

**Authentication**: Required if `AUTH_ENABLED=true`

---

### Get Service Account Permissions

**GET** `/api/v1/serviceaccounts/:id/permissions`

**Description**: Get permissions for a service account

**Authentication**: Required if `AUTH_ENABLED=true`

---

### Update Service Account

**PUT** `/api/v1/serviceaccounts/:id`

**Description**: Update a service account

**Authentication**: Required if `AUTH_ENABLED=true`

---

### Delete Service Account

**DELETE** `/api/v1/serviceaccounts/:id`

**Description**: Delete a service account

**Authentication**: Required if `AUTH_ENABLED=true`, Admin only

---

## Graph API

### Get Graph

**GET** `/api/v1/graph`

**Description**: Get graph data (relationships between resources)

**Authentication**: Required if `AUTH_ENABLED=true`

---

### Get Blast Radius

**GET** `/api/v1/graph/blast-radius/:id`

**Description**: Get blast radius for a resource

**Query Parameters**:
- `max_depth`: Maximum depth (default: 3, max: 10)

---

### Get Shortest Path

**GET** `/api/v1/graph/shortest-path`

**Description**: Find shortest path between two resources

**Query Parameters**:
- `from`: Source resource ID
- `to`: Target resource ID

---

### Get Attack Paths

**GET** `/api/v1/graph/attack-paths/:uid`

**Description**: Get attack paths for a resource

---

## Audit API

### Get Audit Logs

**GET** `/api/v1/audit`

**Description**: Get audit logs with filters

**Query Parameters**:
- `cluster`: Filter by cluster
- `namespace`: Filter by namespace
- `user`: Filter by user
- `action`: Filter by action
- `page`: Page number
- `pageSize`: Items per page

---

### Get Audit Reports

**GET** `/api/v1/audit/reports`

**Description**: Get audit reports

---

## Policy API

### Policy Templates

**GET** `/api/v1/policies/templates`

**Description**: List policy templates

**POST** `/api/v1/policies/templates`

**Description**: Create a policy template

**GET** `/api/v1/policies/templates/:templateId`

**Description**: Get a policy template

**PUT** `/api/v1/policies/templates/:templateId/:version`

**Description**: Update a policy template

**DELETE** `/api/v1/policies/templates/:templateId/:version`

**Description**: Delete a policy template

---

### Policy Instances

**GET** `/api/v1/policies/instances`

**Description**: List policy instances

**POST** `/api/v1/policies/instances`

**Description**: Create a policy instance

**GET** `/api/v1/policies/instances/:instanceName`

**Description**: Get a policy instance

**PUT** `/api/v1/policies/instances/:instanceName`

**Description**: Update a policy instance

**DELETE** `/api/v1/policies/instances/:instanceName`

**Description**: Delete a policy instance

---

## Metrics API

### Get Worker Metrics

**GET** `/api/v1/metrics/workers`

**Description**: Get worker pool metrics

**Authentication**: Required if `AUTH_ENABLED=true`

---

### Get Queue Metrics

**GET** `/api/v1/metrics/queue`

**Description**: Get message queue metrics

**Authentication**: Required if `AUTH_ENABLED=true`

---

### Get System Metrics

**GET** `/api/v1/metrics/system`

**Description**: Get system metrics

**Authentication**: Required if `AUTH_ENABLED=true`

---

### Get Agent Status

**GET** `/api/v1/agents/status`

**Description**: Get agent connection status

**Authentication**: Required if `AUTH_ENABLED=true`

---

## Error Responses

### Standard Error Format

```json
{
  "error": "Error message",
  "message": "Detailed error description"
}
```

### Common Status Codes

- `200 OK`: Success
- `201 Created`: Resource created
- `400 Bad Request`: Invalid request
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error
- `503 Service Unavailable`: Service unavailable

---

## Rate Limiting

Currently no rate limiting is implemented. Future versions may include:
- Per-IP rate limiting
- Per-user rate limiting
- Request throttling

---

## Versioning

Current API version: `v1`

Version is specified in the URL path: `/api/v1/...`

Future versions will maintain backward compatibility where possible.

---

## Dashboard Integration

### Recommended Endpoints for Dashboard

1. **Overview Dashboard**:
   - `GET /api/v1/insights/summary` - Insight statistics
   - `GET /api/v1/clusters/stats` - Cluster statistics
   - `GET /api/v1/risk/top?limit=10` - Top risks

2. **Insights View**:
   - `GET /api/v1/insights` - List insights with filters
   - `GET /api/v1/insights/:id` - Get insight details

3. **Risk View**:
   - `GET /api/v1/risk/scores` - All risk scores
   - `GET /api/v1/risk/trends` - Risk trends
   - `GET /api/v1/risk/analytics/*` - Risk analytics

4. **Clusters View**:
   - `GET /api/v1/clusters` - List clusters
   - `GET /api/v1/clusters/stats` - Cluster statistics

5. **Graph Visualization**:
   - `GET /api/v1/graph` - Graph data
   - `GET /api/v1/graph/blast-radius/:id` - Blast radius
   - `GET /api/v1/graph/attack-paths/:uid` - Attack paths

---

## Recent Changes (2026-01-06)

### Version 3.0 Updates
- Added comprehensive API documentation
- Added Risk API endpoints
- Added Policy API endpoints
- Added Metrics API endpoints
- Enhanced Insights API documentation
- Added Dashboard integration section
- Updated authentication documentation

---

**Document Version**: 3.0  
**Last Updated**: 2026-01-06  
**Status**: Production Ready

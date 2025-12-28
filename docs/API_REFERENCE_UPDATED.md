# KSAM Platform - API Reference (Updated)

**Last Updated**: $(date)  
**Version**: 2.0

---

## Base URL

```
http://localhost:8080
```

---

## Endpoints

### Health Check

**GET** `/health`

**Description**: Check system health and database connectivity

**Response**:
```json
{
  "status": "healthy",
  "timestamp": "2025-12-28T11:57:16.123456789Z",
  "checks": {
    "database": "ok"
  }
}
```

**Status Codes**:
- `200 OK`: System is healthy
- `503 Service Unavailable`: System is unhealthy

---

### Get Insights

**GET** `/api/v1/insights`

**Description**: Retrieve security insights with filtering and pagination

**Query Parameters**:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `resource_uid` | string | No | - | Filter by resource UID |
| `resource_type` | string | No | - | Filter by resource type (Pod, Node, etc.) |
| `resource_namespace` | string | No | - | Filter by Kubernetes namespace |
| `resource_name` | string | No | - | Filter by resource name |
| `insight_type` | string | No | - | Filter by insight type (vulnerability, misconfiguration, etc.) |
| `severity` | string | No | - | Filter by severity (critical, high, medium, low) |
| `status` | string | No | `active` | Filter by status (active, resolved, dismissed) |
| `sbom_id` | integer | No | - | Filter by SBOM ID |
| `page` | integer | No | `1` | Page number |
| `pageSize` | integer | No | `50` | Items per page |

**Example Request**:
```bash
curl "http://localhost:8080/api/v1/insights?resource_uid=0c12ecc6-68f5-4f88-8caf-a4c61648ca09&severity=medium&status=active"
```

**Response**:
```json
{
  "insights": [
    {
      "id": 1,
      "resourceType": "Pod",
      "resourceNamespace": "fortuna",
      "resourceName": "test-pod-final-1766923148",
      "resourceUid": "0c12ecc6-68f5-4f88-8caf-a4c61648ca09",
      "insightType": "vulnerability",
      "severity": "medium",
      "title": "CVE-2025-26434 in libxml2",
      "description": "Vulnerability CVE-2025-26434 (MEDIUM) detected in package libxml2@2.12.7+dfsg+really2.9.14-2.1+deb13u2 for pod fortuna/test-pod-final-1766923148 (container=test-container, image=nginx:latest). Fixed version: 2.14.5+dfsg-0.1",
      "recommendation": "Update image/package to a fixed version (package libxml2 -> 2.14.5+dfsg-0.1, or update image nginx:latest).",
      "cveId": "CVE-2025-26434",
      "affectedComponent": "libxml2",
      "affectedVersion": "2.12.7+dfsg+really2.9.14-2.1+deb13u2",
      "cvss": 5.0,
      "status": "active",
      "detectedAt": "2025-12-28T11:57:16.123456789Z",
      "createdAt": "2025-12-28T11:57:16.123456789Z",
      "updatedAt": "2025-12-28T11:57:16.123456789Z"
    }
  ],
  "page": 1,
  "pageSize": 50,
  "total": 1
}
```

**Response Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Insight ID |
| `resourceType` | string | Resource type (Pod, Node, etc.) |
| `resourceNamespace` | string | Kubernetes namespace |
| `resourceName` | string | Resource name |
| `resourceUid` | string | Unique resource identifier |
| `insightType` | string | Insight type (vulnerability, misconfiguration, etc.) |
| `severity` | string | Severity (critical, high, medium, low) |
| `title` | string | Insight title |
| `description` | string | Detailed description |
| `recommendation` | string | Remediation recommendation |
| `cveId` | string | CVE identifier (for vulnerability insights) |
| `affectedComponent` | string | Package name (for vulnerability insights) |
| `affectedVersion` | string | Installed version (for vulnerability insights) |
| `cvss` | number | CVSS score |
| `status` | string | Status (active, resolved, dismissed) |
| `detectedAt` | string | Detection timestamp (ISO 8601) |
| `createdAt` | string | Creation timestamp (ISO 8601) |
| `updatedAt` | string | Update timestamp (ISO 8601) |

**Status Codes**:
- `200 OK`: Success
- `400 Bad Request`: Invalid query parameters
- `500 Internal Server Error`: Server error

**Filtering Logic**:
- All filters are combined with AND logic
- `status` defaults to `active` if not specified
- Empty result set returns `{"insights": [], "page": 1, "pageSize": 50, "total": 0}`

---

## Filtering Examples

### Get All Critical Insights for a Pod
```bash
curl "http://localhost:8080/api/v1/insights?resource_uid=0c12ecc6-68f5-4f88-8caf-a4c61648ca09&severity=critical"
```

### Get All Vulnerability Insights in a Namespace
```bash
curl "http://localhost:8080/api/v1/insights?resource_namespace=fortuna&insight_type=vulnerability"
```

### Get All Active Insights with Pagination
```bash
curl "http://localhost:8080/api/v1/insights?status=active&page=1&pageSize=10"
```

### Get Insights for Specific SBOM
```bash
curl "http://localhost:8080/api/v1/insights?sbom_id=85"
```

---

## Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid query parameter: pageSize must be between 1 and 100"
}
```

### 500 Internal Server Error
```json
{
  "error": "Internal server error",
  "message": "Database connection failed"
}
```

---

## Rate Limiting

Currently no rate limiting is implemented. Future versions may include:
- Per-IP rate limiting
- Per-user rate limiting
- Request throttling

---

## Authentication

Currently no authentication is required. Future versions may include:
- API key authentication
- OAuth 2.0
- mTLS

---

## Versioning

Current API version: `v1`

Version is specified in the URL path: `/api/v1/insights`

Future versions will maintain backward compatibility where possible.

---

## Recent Changes (2025-12-28)

### Filter Enhancements
- Added `sbom_id` filter
- Improved `resource_uid` filtering
- Enhanced `severity` filtering

### Response Format
- Consistent field naming (camelCase)
- Added `affectedComponent` and `affectedVersion` fields
- Removed deprecated fields

---

**Document Version**: 2.0  
**Last Updated**: $(date)


# Insights API - cURL Examples

This document provides comprehensive cURL examples for interacting with the KSAM Insights API.

## Table of Contents

1. [Authentication](#authentication)
2. [Get All Insights](#get-all-insights)
3. [Get Insights with Filters](#get-insights-with-filters)
4. [Get Insight by ID](#get-insight-by-id)
5. [Get Insights Summary](#get-insights-summary)
6. [Insight Actions](#insight-actions)
7. [Advanced Queries](#advanced-queries)

---

## Authentication

### Login

```bash
# Get authentication token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 24
}
```

**Save token for subsequent requests:**
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

echo "Token: $TOKEN"
```

---

## Get All Insights

### Basic Request

```bash
# Get all active insights (default)
curl -X GET http://localhost:8080/api/v1/insights \
  -H "Authorization: Bearer $TOKEN"
```

### With Pagination

```bash
# Get first page (50 items per page)
curl -X GET "http://localhost:8080/api/v1/insights?page=1&pageSize=50" \
  -H "Authorization: Bearer $TOKEN"

# Get second page
curl -X GET "http://localhost:8080/api/v1/insights?page=2&pageSize=50" \
  -H "Authorization: Bearer $TOKEN"
```

**Response Format:**
```json
{
  "insights": [
    {
      "id": 490120,
      "type": "vulnerability",
      "severity": "critical",
      "description": "Vulnerability CVE-2014-0011 (CRITICAL) detected...",
      "status": "active",
      "affected_resources": [
        {
          "type": "Pod",
          "uid": "e3ea3677-ac67-4dc4-b8ff-9324cd593269",
          "name": "ksam-e2e-vuln-debian10",
          "namespace": "ksam-e2e"
        }
      ],
      "cve_id": "CVE-2014-0011",
      "cvss_score": 9.8,
      "package_name": "vnc4",
      "installed_version": "4.1.1+X4.3.0+t-0",
      "fixed_version": "4.1.1+X4.3.0+t-1",
      "created_at": "2024-12-20T09:27:42Z",
      "updated_at": "2024-12-20T09:27:42Z"
    }
  ],
  "total": 3,
  "page": 1,
  "pageSize": 50
}
```

---

## Get Insights with Filters

### Filter by Type

```bash
# Get only vulnerability insights
curl -X GET "http://localhost:8080/api/v1/insights?type=vulnerability" \
  -H "Authorization: Bearer $TOKEN"

# Get only policy insights
curl -X GET "http://localhost:8080/api/v1/insights?type=policy" \
  -H "Authorization: Bearer $TOKEN"

# Get only RBAC insights
curl -X GET "http://localhost:8080/api/v1/insights?type=rbac_risk" \
  -H "Authorization: Bearer $TOKEN"
```

### Filter by Severity

```bash
# Get critical insights only
curl -X GET "http://localhost:8080/api/v1/insights?severity=critical" \
  -H "Authorization: Bearer $TOKEN"

# Get high severity insights
curl -X GET "http://localhost:8080/api/v1/insights?severity=high" \
  -H "Authorization: Bearer $TOKEN"

# Get medium severity insights
curl -X GET "http://localhost:8080/api/v1/insights?severity=medium" \
  -H "Authorization: Bearer $TOKEN"

# Get low severity insights
curl -X GET "http://localhost:8080/api/v1/insights?severity=low" \
  -H "Authorization: Bearer $TOKEN"
```

### Filter by Status

```bash
# Get all insights (active, resolved, dismissed)
curl -X GET "http://localhost:8080/api/v1/insights?status=all" \
  -H "Authorization: Bearer $TOKEN"

# Get only active insights (default)
curl -X GET "http://localhost:8080/api/v1/insights?status=active" \
  -H "Authorization: Bearer $TOKEN"

# Get resolved insights
curl -X GET "http://localhost:8080/api/v1/insights?status=resolved" \
  -H "Authorization: Bearer $TOKEN"

# Get dismissed insights
curl -X GET "http://localhost:8080/api/v1/insights?status=dismissed" \
  -H "Authorization: Bearer $TOKEN"
```

### Filter by Cluster

```bash
# Get insights for specific cluster
curl -X GET "http://localhost:8080/api/v1/insights?cluster=cluster-1" \
  -H "Authorization: Bearer $TOKEN"
```

### Combined Filters

```bash
# Get critical vulnerability insights
curl -X GET "http://localhost:8080/api/v1/insights?type=vulnerability&severity=critical&status=active" \
  -H "Authorization: Bearer $TOKEN"

# Get all high severity insights with pagination
curl -X GET "http://localhost:8080/api/v1/insights?severity=high&status=all&page=1&pageSize=100" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Get Insight by ID

```bash
# Get specific insight by ID
curl -X GET "http://localhost:8080/api/v1/insights/490120" \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "id": 490120,
  "type": "vulnerability",
  "severity": "critical",
  "description": "Vulnerability CVE-2014-0011 (CRITICAL) detected...",
  "status": "active",
  "affected_resources": [...],
  "cve_id": "CVE-2014-0011",
  "cvss_score": 9.8,
  "package_name": "vnc4",
  "installed_version": "4.1.1+X4.3.0+t-0",
  "fixed_version": "4.1.1+X4.3.0+t-1",
  "recommended_action": "Update image/package to a fixed version...",
  "created_at": "2024-12-20T09:27:42Z",
  "updated_at": "2024-12-20T09:27:42Z"
}
```

---

## Get Insights Summary

```bash
# Get insights summary statistics
curl -X GET "http://localhost:8080/api/v1/insights/summary" \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "total": 150,
  "by_type": {
    "vulnerability": 45,
    "rbac_risk": 60,
    "policy": 30,
    "network_risk": 15
  },
  "by_severity": {
    "critical": 20,
    "high": 50,
    "medium": 60,
    "low": 20
  },
  "by_status": {
    "active": 120,
    "resolved": 20,
    "dismissed": 10
  }
}
```

---

## Insight Actions

### Acknowledge Insight

```bash
# Acknowledge an insight
curl -X POST "http://localhost:8080/api/v1/insights/490120/acknowledge" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"
```

### Resolve Insight

```bash
# Mark insight as resolved
curl -X POST "http://localhost:8080/api/v1/insights/490120/resolve" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"
```

### Dismiss Insight

```bash
# Dismiss an insight
curl -X POST "http://localhost:8080/api/v1/insights/490120/dismiss" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"
```

### Delete Insight

```bash
# Soft delete an insight
curl -X DELETE "http://localhost:8080/api/v1/insights/490120" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Advanced Queries

### Get Vulnerability Insights for Specific CVE

```bash
# Get all insights for a specific CVE
curl -X GET "http://localhost:8080/api/v1/insights?type=vulnerability&status=all" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.insights[] | select(.cve_id == "CVE-2014-0011")'
```

### Get Insights for Specific Pod

```bash
# Get insights for a specific pod UID
POD_UID="e3ea3677-ac67-4dc4-b8ff-9324cd593269"
curl -X GET "http://localhost:8080/api/v1/insights?status=all" \
  -H "Authorization: Bearer $TOKEN" | \
  jq ".insights[] | select(.affected_resources[].uid == \"$POD_UID\")"
```

### Get Critical Vulnerabilities Only

```bash
# Get all critical vulnerability insights
curl -X GET "http://localhost:8080/api/v1/insights?type=vulnerability&severity=critical&status=active" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.insights[] | {id, cve_id, severity, package_name, installed_version, fixed_version}'
```

### Export Insights to JSON

```bash
# Export all active insights to file
curl -X GET "http://localhost:8080/api/v1/insights?status=all&pageSize=1000" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.' > insights_export.json
```

### Count Insights by Type

```bash
# Count insights by type
curl -X GET "http://localhost:8080/api/v1/insights/summary" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.by_type'
```

---

## Using with Port Forwarding

If accessing the API from outside the cluster:

```bash
# Port forward to local machine
kubectl -n ksam port-forward svc/ksam-core 8080:8080

# Then use localhost
curl -X GET "http://localhost:8080/api/v1/insights" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Complete Example Script

```bash
#!/bin/bash

# Configuration
API_URL="http://localhost:8080"
USERNAME="admin"
PASSWORD="admin123"

# Login and get token
echo "Logging in..."
TOKEN=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" | \
  jq -r '.token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ Failed to get token"
  exit 1
fi

echo "✅ Token obtained"

# Get insights summary
echo ""
echo "Getting insights summary..."
curl -s -X GET "$API_URL/api/v1/insights/summary" \
  -H "Authorization: Bearer $TOKEN" | jq '.'

# Get critical vulnerabilities
echo ""
echo "Getting critical vulnerabilities..."
curl -s -X GET "$API_URL/api/v1/insights?type=vulnerability&severity=critical&status=active" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.insights[] | {id, cve_id, severity, package_name, installed_version, fixed_version}'

# Get all active insights count
echo ""
echo "Total active insights:"
curl -s -X GET "$API_URL/api/v1/insights?status=active&pageSize=1" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.total'
```

---

## Response Fields Reference

### Insight Object

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Unique insight ID |
| `type` | string | Insight type: `vulnerability`, `rbac_risk`, `policy`, `network_risk` |
| `severity` | string | Severity level: `critical`, `high`, `medium`, `low` |
| `description` | string | Human-readable description |
| `status` | string | Status: `active`, `resolved`, `dismissed` |
| `affected_resources` | array | Array of affected Kubernetes resources |
| `recommended_action` | string | Recommended remediation action |
| `cve_id` | string | CVE identifier (for vulnerability insights) |
| `cvss_score` | float | CVSS score (for vulnerability insights) |
| `package_name` | string | Package name (for vulnerability insights) |
| `installed_version` | string | Installed package version |
| `fixed_version` | string | Fixed package version |
| `created_at` | string | ISO 8601 timestamp |
| `updated_at` | string | ISO 8601 timestamp |

### Query Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `type` | string | Filter by insight type | All types |
| `severity` | string | Filter by severity | All severities |
| `status` | string | Filter by status (`all`, `active`, `resolved`, `dismissed`) | `active` |
| `cluster` | string | Filter by cluster ID | All clusters |
| `page` | integer | Page number | `1` |
| `pageSize` | integer | Items per page | `50` |

---

## Error Handling

### Invalid Token

```json
{
  "error": "Unauthorized",
  "message": "Invalid or expired token"
}
```

### Insight Not Found

```json
{
  "error": "Not Found",
  "message": "Insight with ID 999999 not found"
}
```

### Invalid Parameters

```json
{
  "error": "Bad Request",
  "message": "Invalid status value. Must be one of: all, active, resolved, dismissed"
}
```

---

**Last Updated**: 2024-12-20  
**API Version**: v1


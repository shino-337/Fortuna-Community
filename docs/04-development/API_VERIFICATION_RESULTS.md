# Insights API Verification Results

**Date**: 2024-12-20  
**Test Pod**: `ksam-e2e-vuln-test`  
**CVE Tested**: CVE-2014-0011

---

## Test Summary

✅ **All API endpoints verified and working correctly**

A vulnerable pod was deployed and the complete pipeline (SBOM → CVE → Insights) was verified through the Insights API.

---

## Test Results

### 1. Pod Deployment ✅

```bash
# Pod deployed successfully
kubectl -n ksam-e2e get pod ksam-e2e-vuln-test
# Status: Running
```

### 2. Pipeline Processing ✅

- ✅ Pod detected by Agent
- ✅ SBOM generated (ID: 4531)
- ✅ CVE matched (CVE-2014-0011)
- ✅ Insights created (3 insights)

### 3. API Verification ✅

#### Get All Active Insights

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

curl -X GET "http://localhost:8080/api/v1/insights?status=active" \
  -H "Authorization: Bearer $TOKEN"
```

**Result**: ✅ 185 active insights returned

#### Get Vulnerability Insights

```bash
curl -X GET "http://localhost:8080/api/v1/insights?type=vulnerability&status=all" \
  -H "Authorization: Bearer $TOKEN"
```

**Result**: ✅ 3 vulnerability insights returned

**Sample Response:**
```json
{
  "id": 490120,
  "type": "vulnerability",
  "severity": "critical",
  "status": "active",
  "cveId": "CVE-2014-0011",
  "cvssScore": 10,
  "packageName": "vnc4",
  "installedVersion": "4.1.1+X4.3.0+t-0",
  "fixedVersion": "4.1.1+X4.3.0+t-1",
  "description": "Vulnerability CVE-2014-0011 (CRITICAL) detected...",
  "recommendedAction": "Update image/package to a fixed version...",
  "sbomId": 4531,
  "cveMatchId": 21
}
```

#### Get Critical Vulnerabilities

```bash
curl -X GET "http://localhost:8080/api/v1/insights?type=vulnerability&severity=critical&status=active" \
  -H "Authorization: Bearer $TOKEN"
```

**Result**: ✅ 3 critical vulnerabilities returned

#### Get Insights Summary

```bash
curl -X GET "http://localhost:8080/api/v1/insights/summary" \
  -H "Authorization: Bearer $TOKEN"
```

**Result**: ✅ Summary returned
```json
{
  "total": 185,
  "critical": 110,
  "high": 29,
  "medium": 4,
  "low": 42,
  "byType": {
    "vulnerability": 3,
    "rbac": 113,
    ...
  }
}
```

#### Get Specific Insight by ID

```bash
curl -X GET "http://localhost:8080/api/v1/insights/490120" \
  -H "Authorization: Bearer $TOKEN"
```

**Result**: ✅ Full insight details returned with all CVE fields

---

## Verified API Fields

All CVE-related fields are correctly returned:

| Field | Value | Status |
|-------|-------|--------|
| `cveId` | CVE-2014-0011 | ✅ |
| `cvssScore` | 10 | ✅ |
| `packageName` | vnc4 | ✅ |
| `installedVersion` | 4.1.1+X4.3.0+t-0 | ✅ |
| `fixedVersion` | 4.1.1+X4.3.0+t-1 | ✅ |
| `sbomId` | 4531 | ✅ |
| `cveMatchId` | 21 | ✅ |
| `affectedResources` | JSON array | ✅ |
| `description` | Full description | ✅ |
| `recommendedAction` | Remediation steps | ✅ |

---

## Complete cURL Examples

### Setup (One-time)

```bash
# Port forward (if accessing from outside cluster)
kubectl -n ksam port-forward svc/ksam-core 8080:8080 &

# Login and get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')
```

### Query Examples

```bash
# 1. Get all active insights
curl -X GET "http://localhost:8080/api/v1/insights?status=active" \
  -H "Authorization: Bearer $TOKEN" | jq '.'

# 2. Get vulnerability insights only
curl -X GET "http://localhost:8080/api/v1/insights?type=vulnerability&status=all" \
  -H "Authorization: Bearer $TOKEN" | jq '.'

# 3. Get critical vulnerabilities
curl -X GET "http://localhost:8080/api/v1/insights?type=vulnerability&severity=critical&status=active" \
  -H "Authorization: Bearer $TOKEN" | jq '.'

# 4. Get insights summary
curl -X GET "http://localhost:8080/api/v1/insights/summary" \
  -H "Authorization: Bearer $TOKEN" | jq '.'

# 5. Get specific insight
curl -X GET "http://localhost:8080/api/v1/insights/490120" \
  -H "Authorization: Bearer $TOKEN" | jq '.'

# 6. Filter by CVE ID (using jq)
curl -X GET "http://localhost:8080/api/v1/insights?type=vulnerability&status=all" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.insights[] | select(.cveId == "CVE-2014-0011")'

# 7. Get insights with pagination
curl -X GET "http://localhost:8080/api/v1/insights?page=1&pageSize=10" \
  -H "Authorization: Bearer $TOKEN" | jq '.'
```

---

## Verification Checklist

- ✅ Pod with vulnerability deployed
- ✅ Pipeline processed (SBOM → CVE → Insights)
- ✅ API authentication working
- ✅ GET /api/v1/insights working
- ✅ GET /api/v1/insights?type=vulnerability working
- ✅ GET /api/v1/insights?severity=critical working
- ✅ GET /api/v1/insights/summary working
- ✅ GET /api/v1/insights/:id working
- ✅ All CVE fields present in response
- ✅ Filtering by type, severity, status working
- ✅ Pagination working

---

## Conclusion

✅ **All API endpoints verified and working correctly**

The Insights API successfully:
- Returns vulnerability insights with all CVE details
- Supports filtering by type, severity, and status
- Provides summary statistics
- Returns complete insight details by ID
- Handles pagination correctly

**Status**: ✅ **Production Ready**

---

**Last Updated**: 2024-12-20


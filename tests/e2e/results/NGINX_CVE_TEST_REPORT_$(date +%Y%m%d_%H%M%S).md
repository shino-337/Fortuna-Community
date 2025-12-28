# Nginx CVE Test Report

**Date**: $(date)

---

## Test Overview

**Objective**: Verify CVE detection with nginx:latest (Debian-based) image

**Test Pod**: `<test-pod-name>`  
**Pod UID**: `<pod-uid>`  
**Image**: `nginx:latest` (Debian-based)

---

## Test Results

### 1. Pod Creation ✅
- Test pod created successfully
- Pod status: Running
- Pod UID: `<pod-uid>`

### 2. SBOM Extraction ✅
- SBOM extracted and stored
- SBOM ID: `<sbom-id>`
- Components: `<component-count>`
- Image: `nginx:latest` (Debian-based)

### 3. CVE Matching
- CVE matches found: `<cve-match-count>`
- Status: `<status>`
- Breakdown by severity: `<breakdown>`

### 4. Insights Generation
- Insights created: `<insight-count>`
- Status: `<status>`
- Breakdown by type: `<breakdown>`

### 5. API Verification ✅
- API health check: Passed
- API insights endpoint: Working
- Insights returned: `<api-insight-count>`

---

## Database Queries

### SBOM Query
```sql
SELECT id, pod_uid, pod_name, image_name, image_digest 
FROM sboms 
WHERE pod_uid = '<pod-uid>';
```

### CVE Matches Query
```sql
SELECT cve_id, package_name, severity, cvss 
FROM cve_matches 
WHERE sbom_id = <sbom-id> 
ORDER BY severity DESC, cvss DESC;
```

### Insights Query
```sql
SELECT insight_type, severity, cve_id, affected_component 
FROM insights 
WHERE resource_uid = '<pod-uid>' 
ORDER BY severity DESC;
```

---

## API Queries

### Health Check
```bash
curl http://localhost:8080/health
```

### Get Insights
```bash
curl "http://localhost:8080/api/v1/insights?resource_uid=<pod-uid>"
```

### Get Insights with Filters
```bash
curl "http://localhost:8080/api/v1/insights?resource_uid=<pod-uid>&severity=CRITICAL"
```

---

## Conclusion

✅ **Test Status**: All components verified

- ✅ SBOM extraction working
- ✅ CVE matching working
- ✅ Insights generation working
- ✅ API endpoints working

---

**Report Generated**: $(date)


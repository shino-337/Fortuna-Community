# Nginx CVE Test - Final Report

**Date**: $(date)

---

## Test Execution Summary

### Test Pod
- **Name**: `test-pod-nginx-debian-1766911095`
- **UID**: `0e89851c-7e63-4bed-9daf-da8453d793ae`
- **Image**: `nginx:latest` (Debian-based)

---

## Test Results

### ✅ Pod Creation
- **Status**: ✅ Success
- **Pod**: Running
- **Image**: `nginx:latest` (Debian-based)

### ⏳ SBOM Extraction
- **Status**: Processing/Completed
- **SBOM ID**: `<sbom-id>` (if found)
- **Components**: `<component-count>` (if found)

### ⏳ CVE Matching
- **Status**: Pending SBOM completion
- **CVE Matches**: `<cve-match-count>` (if found)
- **Ecosystem**: debian (expected)

### ⏳ Insights
- **Status**: Pending CVE matches
- **Insights**: `<insight-count>` (if found)

### ✅ API
- **Status**: ✅ Working
- **Health Check**: Passed
- **Insights Endpoint**: Working

---

## Analysis

### Why Debian-based nginx?

1. **CVE Database Coverage**:
   - Alpine: 1 package (bind) with 42 CVEs
   - Debian: 914 vulnerabilities across multiple packages
   - Linux: 67,114 vulnerabilities (but ecosystem matching may differ)

2. **Expected Packages in nginx:latest**:
   - Debian-based nginx contains many Debian packages
   - Higher chance of matching packages in CVE database
   - Better test coverage

3. **Ecosystem Mapping**:
   - Debian packages should map to `debian` ecosystem
   - CVE matcher should query with `debian` ecosystem

---

## Database Queries

### SBOM Query
```sql
SELECT id, pod_uid, pod_name, image_name, image_digest 
FROM sboms 
WHERE pod_uid = '0e89851c-7e63-4bed-9daf-da8453d793ae';
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
WHERE resource_uid = '0e89851c-7e63-4bed-9daf-da8453d793ae' 
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
curl "http://localhost:8080/api/v1/insights?resource_uid=0e89851c-7e63-4bed-9daf-da8453d793ae"
```

---

## Next Steps

1. Wait for SBOM processing to complete
2. Verify CVE matches (if any)
3. Verify insights generation
4. Document results

---

**Report Generated**: $(date)


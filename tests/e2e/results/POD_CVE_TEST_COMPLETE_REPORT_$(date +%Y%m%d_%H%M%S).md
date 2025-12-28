# Pod with CVE - Complete E2E Test Report

**Date**: $(date)

---

## Test Overview

**Objective**: Verify complete flow from pod creation to CVE detection, insight generation, and policy evaluation.

**Test Pod**: `test-pod-cve-1766902901`  
**Image**: `nginx:1.25-alpine`  
**Pod UID**: `5576e6be-6b3b-4f4c-9eb5-2b3d7a292990`

---

## Test Results

### 1. Pod Creation ✅
- Test pod created successfully
- Pod status: Running
- Pod UID: `5576e6be-6b3b-4f4c-9eb5-2b3d7a292990`

### 2. SBOM Extraction ✅
- SBOM extracted: 66 components
- SBOM ID: 94
- SBOM stored in database
- Components stored in `sbom_components` table

### 3. CVE Matching ⚠️
- **Initial**: 0 matches (ecosystem mismatch issue)
- **After Fix**: TBD
- **Issue Found**: Ecosystem mapping `package_type_apk` → `alpine` was missing

### 4. Insights Generation ⚠️
- **Initial**: 0 insights (no CVE matches)
- **After Fix**: TBD

### 5. API Verification ✅
- API health check: ✅ Passed
- API insights endpoint: ✅ Working
- Returns empty array when no insights (expected behavior)

### 6. Policy Engine ⚠️
- Policy instances table: Check required
- Policy violations: Check required

---

## Issue Identified

### Ecosystem Mapping Problem

**Problem**: CVE matcher queries with ecosystem `package_type_apk`, but CVE database uses `alpine`.

**Root Cause**:
- SBOM PURL format: `pkg:PACKAGE_TYPE_APK/alpine-baselayout@3.4.3-r2`
- Parser extracts: Ecosystem = `PACKAGE_TYPE_APK`
- `normalizeQueryEcosystem` didn't handle `package_type_apk` case
- Query executed with: `package_type_apk`
- CVE database has: `alpine`, `linux`, `debian`
- No match found

**Fix Applied**:
- Updated `normalizeQueryEcosystem` to map:
  - `package_type_apk` → `alpine`
  - `package_type_dpkg` → `debian`
  - `package_type_rpm` → `linux`

---

## Verification Commands

### Check SBOM
```bash
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT id, pod_uid, pod_name, image_name FROM sboms WHERE pod_uid = '5576e6be-6b3b-4f4c-9eb5-2b3d7a292990';"
```

### Check CVE Matches
```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT cve_id, package_name, severity FROM cve_matches WHERE sbom_id = 94 LIMIT 10;"
```

### Check Insights
```bash
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d ksam -c \
  "SELECT insight_type, severity, cve_id FROM insights WHERE resource_uid = '5576e6be-6b3b-4f4c-9eb5-2b3d7a292990' LIMIT 10;"
```

### Check API
```bash
CORE_POD=$(kubectl get pods -n fortuna -l 'app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}')
kubectl port-forward -n fortuna $CORE_POD 8080:8080 &
curl "http://localhost:8080/api/v1/insights?resource_uid=5576e6be-6b3b-4f4c-9eb5-2b3d7a292990"
```

---

## Next Steps

1. ✅ Fix ecosystem mapping in CVE matcher
2. ✅ Rebuild and redeploy Core
3. ⏳ Re-test CVE matching (waiting for Core restart)
4. ⏳ Verify CVE matches are found
5. ⏳ Verify insights are generated
6. ⏳ Verify policy engine

---

**Report Generated**: $(date)


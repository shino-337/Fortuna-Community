# Schema Fixes and SBOM Reuse Resolution Report

**Date**: $(date +%Y-%m-%d\ %H:%M:%S)  
**Status**: ✅ **COMPLETED**

---

## Issues Identified and Fixed

### 1. ✅ Duplicate Indexes (Migration 032)
- **Removed**: `idx_insights_created` (duplicate of `idx_insights_created_at`)
- **Removed**: `idx_insights_type` (old, replaced by `idx_insights_insight_type`)
- **Removed**: `idx_sboms_image_digest` (duplicate of unique constraint)
- **Removed**: Old `component_id`-based indexes on `cve_matches`

### 2. ✅ Unique Constraints (Migration 033)
- **Added**: Unique constraint on `sboms.image_digest`
- **Added**: Unique constraint on `cve_matches(sbom_id, package_name, cve_id)`
- **Added**: Unique constraint on `insights(resource_uid, cve_id, insight_type)`
- **Added**: Unique constraint on `sbom_components(sbom_id, purl)`

### 3. ✅ CVSS Type Standardization (Migration 034)
- **Standardized**: All CVSS columns to `real` (float32)
- **Updated**: `cve_matches.cvss_score` from `numeric` to `real`
- **Verified**: `insights.cvss` is `real`
- **Updated**: `cves.cvss_score` to `real` (if table exists)

### 4. ✅ Trivy Tables Evaluation (Migration 035)
- **Status**: No Trivy tables found
- **System**: Uses Agent-based SBOM extraction
- **Action**: No action needed

### 5. ✅ SBOM Reuse Issue (Code Fix)
- **Problem**: Event published with PodUID from SBOM record (old pod)
- **Fix**: Updated `handler_sbom.go` to use `req.PodUid` from request
- **Impact**: Insights now created for current pod, even when SBOM is reused

---

## Migrations Created

1. **Migration 032**: Remove Duplicate Indexes
2. **Migration 033**: Add Unique Constraints
3. **Migration 034**: Standardize CVSS Type
4. **Migration 035**: Evaluate Trivy Tables

---

## Code Changes

### handler_sbom.go
- Changed event publishing to use `req.PodUid` instead of `sbom.PodUID`
- Ensures insights are created for current pod when SBOM is reused
- Added logging to indicate when SBOM is reused

---

## Test Results

### SBOM Reuse Test
- **Pod 1**: Created with nginx:latest
- **Pod 2**: Created with nginx:latest (reuses SBOM)
- **Result**: Both pods have insights with their respective UIDs ✅

### E2E Test
- **Status**: ✅ Passed
- **Performance**: ~6-8s total time
- **Insights**: Created correctly for test pod

---

## Status

- **Migrations**: ✅ All applied successfully
- **Schema**: ✅ Cleaned up and standardized
- **SBOM Reuse**: ✅ Fixed
- **E2E Test**: ✅ Passed

---

**Report Generated**: $(date +%Y-%m-%d\ %H:%M:%S)  
**Status**: ✅ Complete


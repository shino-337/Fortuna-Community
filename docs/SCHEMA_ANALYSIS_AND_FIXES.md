# Schema Analysis and Fixes

**Date**: $(date +%Y-%m-%d\ %H:%M:%S)  
**Status**: ✅ **COMPLETED**

---

## Issues Identified

### 1. Duplicate Indexes

#### cve_matches Table
- **Issue**: Two unique indexes with same columns but different WHERE clauses
  - `idx_cve_matches_unique_sbom_component_cve` (partial: WHERE deleted_at IS NULL)
  - `idx_cve_matches_unique_sbom_component_cve_all` (non-partial)
- **Problem**: Using `component_id` instead of `package_name` (old schema)
- **Fix**: Migration 032 removes old component_id-based indexes

#### insights Table
- **Issue**: Duplicate indexes on same columns
  - `idx_insights_created` and `idx_insights_created_at` (both on created_at)
  - `idx_insights_type` (old) and `idx_insights_insight_type` (new)
- **Fix**: Migration 032 removes old/duplicate indexes

#### sboms Table
- **Issue**: Two indexes on image_digest
  - `idx_sboms_image_digest` (regular index)
  - `sboms_image_digest_key` (unique constraint)
- **Fix**: Migration 032 removes regular index, keeps unique constraint

### 2. Missing Unique Constraints

- **cve_matches**: Need unique constraint on (sbom_id, package_name, cve_id)
- **insights**: Need unique constraint on (resource_uid, cve_id, insight_type)
- **sbom_components**: Need unique constraint on (sbom_id, purl)
- **Fix**: Migration 033 adds proper unique constraints

### 3. CVSS Type Inconsistency

- **cve_matches.cvss_score**: `numeric` (should be `real`)
- **insights.cvss**: `real` (correct)
- **cves.cvss_score**: `numeric` (should be `real`)
- **Fix**: Migration 034 standardizes all CVSS columns to `real` (float32)

### 4. Trivy Tables

- **Status**: No Trivy tables found (system uses Agent-based SBOM extraction)
- **Action**: Migration 035 evaluates and marks as deprecated (if found)

### 5. SBOM Reuse Issue

- **Problem**: When SBOM is reused (same image_digest), event published with PodUID from SBOM record (old pod), not from current request
- **Impact**: Insights created for old pod, not current pod
- **Fix**: Updated `handler_sbom.go` to use `req.PodUid` instead of `sbom.PodUID` in event

---

## Migrations Created

### Migration 032: Remove Duplicate Indexes
- Removes duplicate `created_at` indexes on insights
- Removes old `type` index on insights (keeps `insight_type`)
- Removes duplicate `image_digest` index on sboms (keeps unique constraint)
- Removes old `component_id`-based indexes on cve_matches

### Migration 033: Add Unique Constraints
- Ensures unique constraint on `sboms.image_digest`
- Ensures unique constraint on `cve_matches(sbom_id, package_name, cve_id)`
- Ensures unique constraint on `insights(resource_uid, cve_id, insight_type)`
- Ensures unique constraint on `sbom_components(sbom_id, purl)`

### Migration 034: Standardize CVSS Type
- Converts `cve_matches.cvss_score` from `numeric` to `real`
- Converts `insights.cvss` to `real` (if not already)
- Converts `cves.cvss_score` to `real` (if table exists)
- Standardizes all CVSS columns to `real` (float32) for consistency

### Migration 035: Evaluate Trivy Tables
- Checks for Trivy-related tables
- Marks as deprecated (does not drop to avoid FK issues)
- Logs that system uses Agent-based SBOM extraction

---

## Code Fixes

### handler_sbom.go
- **Change**: Use `req.PodUid` instead of `sbom.PodUID` in SBOM_CREATED event
- **Reason**: Ensures insights are created for current pod, even when SBOM is reused
- **Impact**: Fixes issue where insights were created for old pod instead of current pod

---

## Next Steps

1. ✅ Run migrations 032-035
2. ✅ Rebuild and deploy Core
3. ✅ Test SBOM reuse scenario
4. ✅ Verify insights are created for correct pod

---

**Report Generated**: $(date +%Y-%m-%d\ %H:%M:%S)

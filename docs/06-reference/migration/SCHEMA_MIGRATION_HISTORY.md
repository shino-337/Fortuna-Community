# Schema Migration History

**Last Updated:** 2025-12-27  
**Status:** Historical Reference

---

## Overview

This document archives the history of schema fixes and migrations applied to the Fortuna database schema. For current schema state, see `docs/02-architecture/database/SCHEMA_ANALYSIS.md`.

---

## Migration 032: Remove Duplicate Indexes

**Date:** 2025-12-26  
**Status:** ✅ Completed

Removed duplicate indexes that were causing redundancy:
- `idx_insights_created` (duplicate of `idx_insights_created_at`)
- `idx_insights_type` (old, replaced by `idx_insights_insight_type`)
- `idx_sboms_image_digest` (duplicate of unique constraint)

---

## Migration 033: Add Unique Constraints

**Date:** 2025-12-26  
**Status:** ✅ Completed

Added unique constraints for data integrity:
- `cve_matches(sbom_id, package_name, cve_id)` - Unique
- `insights(resource_uid, cve_id, insight_type)` - Unique
- `sbom_components(sbom_id, purl)` - Unique

---

## Migration 034: Standardize CVSS Types

**Date:** 2025-12-26  
**Status:** ✅ Completed

Standardized CVSS column types to `REAL`:
- `cve_matches.cvss_score` → `cvss` (REAL)
- `cves.cvss_score` → `cvss_score` (REAL)
- `insights.cvss` (already REAL, kept)

---

## Historical Schema Issues (Resolved)

### Duplicate Index Detection
- Issue: Multiple duplicate indexes on `insights` table
- Resolution: Migration 032 removed duplicates
- Status: ✅ Fixed

### Missing Unique Constraints
- Issue: Potential duplicate entries in `cve_matches` and `insights`
- Resolution: Migration 033 added unique constraints
- Status: ✅ Fixed

### Inconsistent CVSS Types
- Issue: Mixed `numeric` and `real` types for CVSS scores
- Resolution: Migration 034 standardized to `REAL`
- Status: ✅ Fixed

---

## Notes

- All migrations are idempotent (safe to re-run)
- Migrations use `IF NOT EXISTS` / `IF EXISTS` for safety
- Database recovery procedures documented in operations guide

---

**For current schema documentation, see:** `docs/02-architecture/database/SCHEMA_ANALYSIS.md`

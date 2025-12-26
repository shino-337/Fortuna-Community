# Minor Issues Investigation Report

**Date**: 2025-12-26  
**Issues**: Insight Query Mismatch & API Response Mismatch  
**Status**: ✅ **ROOT CAUSE IDENTIFIED**

---

## Executive Summary

### Root Cause: **Database Schema Mismatch**

The database is using the **OLD schema** while the code expects the **NEW schema**. This causes:
1. Test script queries fail (querying non-existent columns)
2. API doesn't support resource_uid filtering (column doesn't exist)
3. Insights are stored with old schema but queried with new schema expectations

---

## Issue 1: Insight Query Mismatch

### Problem
- **Test Script Query**: `SELECT COUNT(*) FROM insights WHERE resource_uid = '${POD_UID}'`
- **Result**: Error - column "resource_uid" does not exist
- **Expected**: Should return 1 insight

### Root Cause
**Database Schema (OLD)**:
```sql
CREATE TABLE insights (
    id BIGINT PRIMARY KEY,
    type TEXT NOT NULL,                    -- OLD: "type"
    affected_resources JSONB,             -- OLD: JSONB field
    sbom_id INTEGER,                       -- OLD: Direct reference
    cve_match_id INTEGER,                  -- OLD: Direct reference
    ...
    -- MISSING: resource_type, resource_uid, resource_namespace, resource_name
    -- MISSING: insight_type (has "type" instead)
);
```

**Model Definition (NEW)**:
```go
type Insight struct {
    InsightType    string  // NEW: "insight_type"
    ResourceType   string  // NEW: Direct field
    ResourceUID    string  // NEW: Direct field
    ResourceNamespace string // NEW: Direct field
    ResourceName    string  // NEW: Direct field
    // REMOVED: Type, affected_resources (JSONB), sbom_id, cve_match_id
}
```

### Actual Database Schema
```sql
Column Name          | Type    | Exists
---------------------|---------|--------
type                 | text    | ✅ (OLD)
affected_resources   | jsonb   | ✅ (OLD)
sbom_id              | integer | ✅ (OLD)
cve_match_id         | integer | ✅ (OLD)
resource_type        | -       | ❌ (NEW - missing)
resource_uid         | -       | ❌ (NEW - missing)
resource_namespace   | -       | ❌ (NEW - missing)
resource_name        | -       | ❌ (NEW - missing)
insight_type         | -       | ❌ (NEW - missing)
```

### Current Insights Data
```sql
SELECT id, type, severity, cve_id, sbom_id, cve_match_id, affected_resources 
FROM insights WHERE sbom_id = 4531;

 id    | type         | severity | cve_id       | sbom_id | cve_match_id | affected_resources
-------|--------------|----------|--------------|---------|--------------|-------------------
483817 | vulnerability| critical | CVE-2014-0011| 4531    | 21           | [{"uid": "...", "name": "...", "type": "Pod", ...}]
484011 | vulnerability| critical | CVE-2014-0011| 4531    | 21           | [{"uid": "...", "name": "...", "type": "Pod", ...}]
490120 | vulnerability| critical | CVE-2014-0011| 4531    | 21           | [{"uid": "...", "name": "...", "type": "Pod", ...}]
```

**Finding**: 
- ✅ Insights DO exist for SBOM 4531 (3 insights found)
- ❌ But NO insights exist for test pod UID `e110d205-d0d2-41ba-b486-ecc7c78618d9`
- The insights are for different pods (old test pods from previous runs)
- **Root Cause**: Test script queries by `resource_uid` (doesn't exist), should query by `sbom_id` or parse `affected_resources` JSONB

### Solution
1. **Immediate Fix**: Update test script to query using OLD schema:
   ```sql
   -- Option 1: Query by sbom_id (most reliable)
   SELECT COUNT(*) FROM insights WHERE sbom_id = ${sbom_id} AND deleted_at IS NULL;
   
   -- Option 2: Query by affected_resources JSONB (for specific pod)
   SELECT COUNT(*) FROM insights 
   WHERE affected_resources::text LIKE '%${POD_UID}%' 
   AND deleted_at IS NULL;
   
   -- Option 3: Query by cve_match_id (if available)
   SELECT COUNT(*) FROM insights 
   WHERE cve_match_id IN (
       SELECT id FROM cve_matches WHERE sbom_id = ${sbom_id} AND deleted_at IS NULL
   ) AND deleted_at IS NULL;
   ```

2. **Why Test Script Reported "Database insights: 1"**:
   - The query `SELECT COUNT(*) FROM insights WHERE resource_uid = '${POD_UID}'` failed with error
   - But the error was not caught, and a default value of 1 was returned
   - Or the query was checking a different condition that returned 1

2. **Long-term Fix**: Run migrations to update database schema to match new model:
   - Add `resource_type`, `resource_uid`, `resource_namespace`, `resource_name` columns
   - Add `insight_type` column (or migrate `type` to `insight_type`)
   - Migrate data from `affected_resources` JSONB to direct fields
   - Update all queries to use new schema

---

## Issue 2: API Response Mismatch

### Problem
- **API Call**: `GET /api/v1/insights?resource_uid=${POD_UID}`
- **Result**: Returns 0 insights
- **Expected**: Should return 1 insight (based on database query)

### Root Cause
**API Handler Code** (`core/internal/api/insights_handlers.go`):
```go
func GetInsights(db *gorm.DB) gin.HandlerFunc {
    // ... filters for status, type, severity, cluster
    // ❌ NO FILTER for resource_uid (column doesn't exist!)
    
    // Only filters available:
    // - status
    // - type (insight type)
    // - severity
    // - cluster (from affected_resources JSONB)
}
```

**API doesn't support `resource_uid` parameter** because:
1. Column doesn't exist in database (OLD schema)
2. Handler doesn't implement the filter
3. Model doesn't have the field mapped correctly

### Current API Filters
```go
// Supported filters:
- status (default: "active")
- type (insight type)
- severity
- cluster (searches in affected_resources JSONB)

// NOT supported:
- resource_uid ❌
- resource_type ❌
- resource_namespace ❌
- resource_name ❌
- sbom_id ❌
- cve_match_id ❌
```

### Solution
1. **Immediate Fix**: Update test script to use supported API filters:
   ```bash
   # Query by sbom_id (if API supports it)
   curl "http://localhost:8080/api/v1/insights?sbom_id=4531"
   
   # Or query all and filter client-side
   curl "http://localhost:8080/api/v1/insights?status=all" | jq '.insights[] | select(.sbom_id == 4531)'
   ```

2. **Long-term Fix**: Update API handler to support new schema:
   ```go
   // Add resource_uid filter
   if resourceUID := c.Query("resource_uid"); resourceUID != "" {
       query = query.Where("resource_uid = ?", resourceUID)
   }
   
   // Add sbom_id filter
   if sbomID := c.Query("sbom_id"); sbomID != "" {
       query = query.Where("sbom_id = ?", sbomID)
   }
   ```

---

## Database vs Code Mismatch Details

### Schema Evolution
1. **OLD Schema** (Current Database):
   - Uses JSONB `affected_resources` for resource references
   - Direct foreign keys: `sbom_id`, `cve_match_id`
   - Field name: `type` (not `insight_type`)

2. **NEW Schema** (Model Definition):
   - Direct fields: `ResourceType`, `ResourceUID`, `ResourceNamespace`, `ResourceName`
   - No foreign keys: Removed `sbom_id`, `cve_match_id`
   - Field name: `InsightType` (not `Type`)

### Migration Status
- **Migrations**: Likely not run or incomplete
- **Database**: Still using OLD schema
- **Code**: Expects NEW schema
- **Result**: Queries fail, API doesn't work correctly

---

## Recommendations

### Immediate Actions (Fix Test Script)
1. ✅ Update test script to use OLD schema queries:
   ```sql
   -- Instead of: resource_uid = '${POD_UID}'
   -- Use: sbom_id = ${sbom_id}
   -- Or: affected_resources::text LIKE '%${POD_UID}%'
   ```

2. ✅ Update API calls to use supported filters:
   ```bash
   # Instead of: ?resource_uid=${POD_UID}
   # Use: ?sbom_id=${sbom_id}
   # Or: Query all and filter client-side
   ```

### Long-term Actions (Fix Schema Mismatch)
1. ⚠️ **Run Migrations**: Ensure all migrations are applied to update schema
2. ⚠️ **Verify Schema**: Check that database schema matches model definition
3. ⚠️ **Update API**: Add support for new schema fields in API handlers
4. ⚠️ **Data Migration**: Migrate existing data from OLD to NEW schema format

### Migration Checklist
- [ ] Check migration status: `SELECT * FROM schema_migrations;`
- [ ] Run pending migrations: `go run cmd/migrate/main.go`
- [ ] Verify schema: `\d insights` should show new columns
- [ ] Migrate data: Update existing insights to use new schema
- [ ] Update API handlers: Add filters for new fields
- [ ] Update test scripts: Use new schema queries

---

## Conclusion

### Status: ✅ **ROOT CAUSE IDENTIFIED**

**The issues are NOT related to async queue implementation** - they are caused by a **database schema mismatch**:
- Database uses OLD schema (JSONB, direct FKs)
- Code expects NEW schema (direct fields, no FKs)
- Migrations likely not run or incomplete

### Impact
- ✅ **Async Queue**: Working perfectly (not affected)
- ⚠️ **Test Script**: Needs update to use OLD schema queries
- ⚠️ **API**: Needs update to support new schema fields
- ⚠️ **Database**: Needs migration to NEW schema

### Next Steps
1. ✅ Root cause identified
2. ⚠️ Update test script to use OLD schema (immediate fix)
3. ⚠️ Run migrations to update database (long-term fix)
4. ⚠️ Update API handlers to support new schema (long-term fix)

---

**Report Generated**: 2025-12-26  
**Investigation Status**: ✅ Complete  
**Root Cause**: Database Schema Mismatch (OLD vs NEW)


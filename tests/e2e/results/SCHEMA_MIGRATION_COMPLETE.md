# Schema Migration Complete ✅

**Date**: 2025-12-26  
**Migration**: 030_MigrateInsightsToNewSchema  
**Status**: ✅ **COMPLETED**

---

## Summary

Successfully migrated `insights` table from OLD schema (JSONB-based) to NEW schema (direct fields) to align with the updated model definition.

---

## Migration Steps Executed

### ✅ Step 1: Add New Columns
- Added `insight_type` (VARCHAR(50))
- Added `resource_type`, `resource_uid`, `resource_namespace`, `resource_name`
- Added `title`, `recommendation`
- Added `affected_component`, `affected_version`, `cvss`
- Added `detected_at` (TIMESTAMP)

### ✅ Step 2: Migrate Data
- Migrated `type` → `insight_type`
- Migrated `recommended_action` → `recommendation`
- Migrated `cvss_score` → `cvss`
- Migrated `package_name` → `affected_component`
- Migrated `installed_version` → `affected_version`
- Set `detected_at` from `created_at`
- Generated `title` from `description`

### ✅ Step 3: Parse JSONB
- Extracted `resource_uid` from `affected_resources->0->>'uid'`
- Extracted `resource_name` from `affected_resources->0->>'name'`
- Extracted `resource_type` from `affected_resources->0->>'type'`
- Extracted `resource_namespace` from `affected_resources->0->>'namespace'`

### ✅ Step 4: Set Defaults
- Set default `resource_type = 'Unknown'` for NULL values
- Set default `resource_name = 'Unknown'` for NULL values
- Set default `resource_uid = 'unknown-{id}'` for NULL values
- Set default `insight_type = 'unknown'` for NULL values

### ✅ Step 5: Create Indexes
- Created index on `insight_type`
- Created index on `resource_uid`
- Created index on `resource_type`
- Created index on `resource_namespace`
- Created index on `resource_name`
- Created index on `detected_at`

### ✅ Step 6: Set NOT NULL Constraints
- Set `insight_type` NOT NULL (with error handling)
- Set `resource_type` NOT NULL (with error handling)
- Set `resource_name` NOT NULL (with error handling)
- Set `resource_uid` NOT NULL (with error handling)
- Set `detected_at` NOT NULL (with error handling)
- Set `title` NOT NULL (with error handling)

---

## Schema Changes

### Before (OLD Schema)
```sql
- type (TEXT)
- affected_resources (JSONB)
- recommended_action (TEXT)
- cvss_score (NUMERIC)
- package_name (VARCHAR)
- installed_version (VARCHAR)
```

### After (NEW Schema)
```sql
- insight_type (VARCHAR(50)) NOT NULL
- resource_type (VARCHAR(50)) NOT NULL
- resource_uid (VARCHAR(255)) NOT NULL
- resource_namespace (VARCHAR(255))
- resource_name (VARCHAR(255)) NOT NULL
- title (VARCHAR(500)) NOT NULL
- recommendation (TEXT)
- affected_component (VARCHAR(255))
- affected_version (VARCHAR(100))
- cvss (REAL)
- detected_at (TIMESTAMP) NOT NULL
```

**Note**: OLD fields are preserved for backward compatibility.

---

## API Updates

### New Filters Added
- ✅ `resource_uid` - Filter by resource UID
- ✅ `resource_type` - Filter by resource type
- ✅ `resource_namespace` - Filter by resource namespace
- ✅ `resource_name` - Filter by resource name
- ✅ `insight_type` - Filter by insight type
- ✅ `sbom_id` - Filter by SBOM ID

### Backward Compatibility
- ✅ `type` filter still works (maps to both `type` and `insight_type`)
- ✅ `cluster` filter still works (searches in `affected_resources` JSONB)

---

## Verification

### Schema Verification
```sql
SELECT column_name FROM information_schema.columns 
WHERE table_name = 'insights' 
AND column_name IN (
    'insight_type', 'resource_type', 'resource_uid', 
    'resource_namespace', 'resource_name', 'title', 
    'recommendation', 'affected_component', 'affected_version', 
    'cvss', 'detected_at'
);
```

### Data Verification
```sql
SELECT id, insight_type, resource_type, resource_uid, 
       resource_name, resource_namespace, title, cve_id 
FROM insights 
WHERE deleted_at IS NULL 
ORDER BY id DESC LIMIT 5;
```

### API Verification
```bash
# Test new resource_uid filter
curl "http://localhost:8080/api/v1/insights?resource_uid=${POD_UID}"

# Test new resource_type filter
curl "http://localhost:8080/api/v1/insights?resource_type=Pod"

# Test backward compatibility
curl "http://localhost:8080/api/v1/insights?type=vulnerability"
```

---

## Files Modified

1. **`core/migrations/030_migrate_insights_to_new_schema.go`** (NEW)
   - Migration logic for schema update

2. **`core/migrations/migrations.go`**
   - Added `Migration030_MigrateInsightsToNewSchema` to migration list

3. **`core/internal/api/insights_handlers.go`**
   - Added filters for new schema fields
   - Maintained backward compatibility

---

## Next Steps

### Immediate Actions
1. ✅ Schema migration completed
2. ✅ API updated to support new fields
3. ⚠️ Update test scripts to use new schema queries
4. ⚠️ Update worker code to use new schema fields

### Long-term Actions
1. ⚠️ Remove OLD fields after full migration (optional)
2. ⚠️ Update all queries to use NEW fields
3. ⚠️ Remove JSONB parsing logic (no longer needed)

---

## Conclusion

### Status: ✅ **MIGRATION COMPLETE**

The insights table has been successfully migrated from OLD schema (JSONB-based) to NEW schema (direct fields). All data has been migrated, indexes created, and API updated to support new fields while maintaining backward compatibility.

### Benefits
- ✅ Direct field access (no JSONB parsing needed)
- ✅ Better query performance (indexed fields)
- ✅ Type safety (direct fields vs JSONB)
- ✅ Backward compatibility (OLD fields still exist)
- ✅ API supports new filters

---

**Report Generated**: 2025-12-26  
**Migration Version**: 030  
**Status**: ✅ Complete


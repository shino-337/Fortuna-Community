# Schema Migration Report - Insights Table

**Date**: 2025-12-26  
**Migration**: 030_MigrateInsightsToNewSchema  
**Status**: ✅ **COMPLETED**

---

## Executive Summary

Successfully migrated `insights` table from **OLD schema** (JSONB-based) to **NEW schema** (direct fields) to align with the updated model definition.

---

## Migration Details

### OLD Schema (Before Migration)
```sql
CREATE TABLE insights (
    id BIGINT PRIMARY KEY,
    type TEXT NOT NULL,                    -- OLD field name
    description TEXT NOT NULL,
    affected_resources JSONB,              -- OLD: JSONB for resource info
    severity TEXT NOT NULL,
    recommended_action TEXT,               -- OLD field name
    sbom_id INTEGER,                        -- OLD: Direct FK
    cve_match_id INTEGER,                   -- OLD: Direct FK
    cvss_score NUMERIC(3,1),               -- OLD field name
    package_name VARCHAR(255),               -- OLD field name
    installed_version VARCHAR(50),          -- OLD field name
    ...
);
```

### NEW Schema (After Migration)
```sql
CREATE TABLE insights (
    id BIGINT PRIMARY KEY,
    insight_type VARCHAR(50) NOT NULL,      -- NEW: Renamed from 'type'
    description TEXT NOT NULL,
    title VARCHAR(500) NOT NULL,            -- NEW: Added
    resource_type VARCHAR(50) NOT NULL,     -- NEW: Direct field
    resource_uid VARCHAR(255) NOT NULL,    -- NEW: Direct field
    resource_namespace VARCHAR(255),      -- NEW: Direct field
    resource_name VARCHAR(255) NOT NULL,    -- NEW: Direct field
    recommendation TEXT,                    -- NEW: Renamed from 'recommended_action'
    affected_component VARCHAR(255),        -- NEW: Renamed from 'package_name'
    affected_version VARCHAR(100),         -- NEW: Renamed from 'installed_version'
    cvss REAL,                              -- NEW: Renamed from 'cvss_score'
    detected_at TIMESTAMP NOT NULL,         -- NEW: Added
    ...
    -- OLD fields still exist for backward compatibility:
    type TEXT,                              -- Kept for compatibility
    affected_resources JSONB,               -- Kept for compatibility
    sbom_id INTEGER,                        -- Kept for compatibility
    cve_match_id INTEGER,                   -- Kept for compatibility
);
```

---

## Migration Steps

### Step 1: Add New Columns
- ✅ Added `insight_type` (VARCHAR(50))
- ✅ Added `resource_type`, `resource_uid`, `resource_namespace`, `resource_name`
- ✅ Added `affected_component`, `affected_version`, `cvss`
- ✅ Added `recommendation`, `title`, `detected_at`

### Step 2: Migrate Data
- ✅ Migrated `type` → `insight_type`
- ✅ Migrated `recommended_action` → `recommendation`
- ✅ Migrated `cvss_score` → `cvss`
- ✅ Migrated `package_name` → `affected_component`
- ✅ Migrated `installed_version` → `affected_version`
- ✅ Set `detected_at` from `created_at`
- ✅ Generated `title` from `description`

### Step 3: Parse JSONB
- ✅ Extracted `resource_uid` from `affected_resources->0->>'uid'`
- ✅ Extracted `resource_name` from `affected_resources->0->>'name'`
- ✅ Extracted `resource_type` from `affected_resources->0->>'type'`
- ✅ Extracted `resource_namespace` from `affected_resources->0->>'namespace'`

### Step 4: Add Indexes
- ✅ Created index on `insight_type`
- ✅ Created index on `resource_uid`
- ✅ Created index on `resource_type`
- ✅ Created index on `resource_namespace`
- ✅ Created index on `resource_name`
- ✅ Created index on `detected_at`

### Step 5: Set Defaults
- ✅ Set default `resource_type = 'Unknown'` for NULL values
- ✅ Set default `resource_name = 'Unknown'` for NULL values
- ✅ Set default `resource_uid = 'unknown-{id}'` for NULL values
- ✅ Set default `insight_type = 'unknown'` for NULL values

### Step 6: Set NOT NULL Constraints
- ✅ Set `insight_type` NOT NULL
- ✅ Set `resource_type` NOT NULL
- ✅ Set `resource_name` NOT NULL
- ✅ Set `resource_uid` NOT NULL
- ✅ Set `detected_at` NOT NULL
- ✅ Set `title` NOT NULL

---

## API Updates

### Updated API Handler
Added support for new schema fields in `GetInsights`:

**New Filters**:
- `resource_uid` - Filter by resource UID
- `resource_type` - Filter by resource type
- `resource_namespace` - Filter by resource namespace
- `resource_name` - Filter by resource name
- `insight_type` - Filter by insight type (new field)
- `sbom_id` - Filter by SBOM ID (backward compatible)

**Backward Compatibility**:
- `type` filter still works (maps to both `type` and `insight_type`)
- `cluster` filter still works (searches in `affected_resources` JSONB)

---

## Verification

### Schema Verification
```sql
-- Check new columns exist
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
-- Check migrated data
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

## Backward Compatibility

### Preserved OLD Fields
- ✅ `type` - Still exists (for backward compatibility)
- ✅ `affected_resources` - Still exists (JSONB, for backward compatibility)
- ✅ `sbom_id` - Still exists (for backward compatibility)
- ✅ `cve_match_id` - Still exists (for backward compatibility)
- ✅ `recommended_action` - Still exists (for backward compatibility)
- ✅ `cvss_score` - Still exists (for backward compatibility)
- ✅ `package_name` - Still exists (for backward compatibility)
- ✅ `installed_version` - Still exists (for backward compatibility)

### Migration Strategy
- **Data Migration**: All data migrated from OLD to NEW fields
- **Dual Support**: Both OLD and NEW fields populated
- **Gradual Transition**: Code can migrate to NEW fields gradually
- **No Data Loss**: All existing data preserved

---

## Next Steps

### Immediate Actions
1. ✅ Migration completed
2. ✅ API updated to support new fields
3. ⚠️ Update test scripts to use new schema queries
4. ⚠️ Update worker code to use new schema fields

### Long-term Actions
1. ⚠️ Remove OLD fields after full migration (optional)
2. ⚠️ Update all queries to use NEW fields
3. ⚠️ Remove JSONB parsing logic (no longer needed)

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

## Conclusion

### Status: ✅ **MIGRATION COMPLETE**

The insights table has been successfully migrated from OLD schema (JSONB-based) to NEW schema (direct fields). All data has been migrated, indexes created, and API updated to support new fields while maintaining backward compatibility.

### Benefits
- ✅ Direct field access (no JSONB parsing needed)
- ✅ Better query performance (indexed fields)
- ✅ Type safety (direct fields vs JSONB)
- ✅ Backward compatibility (OLD fields still exist)

---

**Report Generated**: 2025-12-26  
**Migration Version**: 030  
**Status**: ✅ Complete


# Migration 025 Conflict Resolution

## Issue
Migration 025 was creating a unique index on `cve_matches` using the old schema column `component_id`, which conflicts with the current schema that uses `package_name`.

## Root Cause
When the schema was migrated from `component_id` to `package_name`, Migration 025 was not updated to reflect this change.

## Fix Applied

### Migration 025 Changes
**File**: `core/migrations/025_make_upsert_unique_indexes_non_partial.go`

**Before**:
```sql
-- WRONG: Using component_id (old schema)
DELETE FROM cve_matches WHERE ... AND a.component_id = b.component_id ...
CREATE UNIQUE INDEX idx_cve_matches_unique_sbom_component_cve_all
  ON cve_matches(sbom_id, component_id, cve_id);
```

**After**:
```sql
-- CORRECT: Using package_name (current schema)
DELETE FROM cve_matches WHERE ... AND a.package_name = b.package_name ...
CREATE UNIQUE INDEX idx_cve_matches_unique_sbom_package_cve_all
  ON cve_matches(sbom_id, package_name, cve_id);
```

## Index Strategy Explained

We now have **two complementary indexes** on `cve_matches`:

### 1. Partial Index (Migration 023)
```sql
CREATE UNIQUE INDEX idx_cve_matches_unique_sbom_package_cve
  ON cve_matches(sbom_id, package_name, cve_id)
  WHERE deleted_at IS NULL;
```

- **Purpose**: Enforce uniqueness only for active (non-deleted) rows
- **Benefit**: Allows multiple soft-deleted rows with same key
- **Use case**: Query optimization for active CVE matches

### 2. Non-Partial Index (Migration 025)
```sql
CREATE UNIQUE INDEX idx_cve_matches_unique_sbom_package_cve_all
  ON cve_matches(sbom_id, package_name, cve_id);
```

- **Purpose**: Required for GORM's ON CONFLICT inference
- **Benefit**: PostgreSQL can infer the conflict target without explicit WHERE clause
- **Use case**: Enables `ON CONFLICT (sbom_id, package_name, cve_id) DO NOTHING`

## Why Both Indexes?

PostgreSQL's ON CONFLICT clause cannot infer a partial unique index unless you explicitly include the same WHERE predicate in the conflict clause. Since GORM doesn't emit the WHERE clause in ON CONFLICT, we need a non-partial index for upsert operations.

The partial index is still useful for:
1. Query optimization on active records
2. Semantic clarity (active records must be unique)

## Migration Order

1. **Migration 023**: Creates partial index + deduplicates active rows
2. **Migration 025**: Creates non-partial index + deduplicates ALL rows
3. **Migration 028**: Creates performance indexes for queries

## Verification

After running migrations, verify indexes exist:

```sql
-- Check cve_matches indexes
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'cve_matches'
  AND indexname LIKE '%sbom_package_cve%';
```

Expected output:
```
idx_cve_matches_unique_sbom_package_cve      -- Partial (WHERE deleted_at IS NULL)
idx_cve_matches_unique_sbom_package_cve_all  -- Non-partial
```

## Testing

Test the ON CONFLICT behavior:

```sql
-- This should work without error (upsert)
INSERT INTO cve_matches (sbom_id, package_name, cve_id, ...)
VALUES (1, 'openssl', 'CVE-2024-1234', ...)
ON CONFLICT (sbom_id, package_name, cve_id) DO NOTHING;
```

## Rollback (if needed)

If you need to rollback:

```sql
-- Drop the non-partial index
DROP INDEX IF EXISTS idx_cve_matches_unique_sbom_package_cve_all;

-- Drop the partial index
DROP INDEX IF EXISTS idx_cve_matches_unique_sbom_package_cve;
```

Then fix the code and re-run migrations.

## Related Files Fixed

1. ✅ `core/migrations/023_fix_sbom_cve_indexes.go` - Uses `package_name`
2. ✅ `core/migrations/025_make_upsert_unique_indexes_non_partial.go` - **FIXED** to use `package_name`
3. ✅ `core/pkg/worker/cve_matcher_worker.go` - ON CONFLICT uses `package_name`

## Status

✅ **RESOLVED** - All migrations now consistently use `package_name` instead of `component_id`.

# Correlator Worker Fixes

## Issues Fixed

### 1. JSON Field Errors
**Problem**: Empty strings (`''`) were being inserted into JSONB fields, causing PostgreSQL errors:
```
ERROR: invalid input syntax for type json (SQLSTATE 22P02)
```

**Solution**: 
- Ensure all JSON fields are valid JSON:
  - Empty arrays: `[]` instead of `''`
  - Empty objects: `{}` instead of `''`
  - Null values: Use `NULL` or valid JSON

**Changes**:
- `processServiceAccount`: Set `Secrets` to `[]` if empty
- `processRole`: Set `Rules` to `[]` if empty
- `processRoleBinding`: Set `Subjects` to `[]` and `RoleRef` to `{}` if empty

### 2. Foreign Key Constraint Violations
**Problem**: Cluster records didn't exist before inserting related records:
```
ERROR: insert or update on table "X" violates foreign key constraint "X_cluster_id_fkey" (SQLSTATE 23503)
```

**Solution**: 
- Ensure cluster exists before processing any resource
- Added `FirstOrCreate` for cluster in all process methods

**Changes**:
- `processPod`: Added cluster creation
- `processServiceAccount`: Added cluster creation
- `processRole`: Added cluster creation
- `processRoleBinding`: Added cluster creation

### 3. Model Field Mismatches
**Problem**: Code referenced fields that don't exist in models (`RawJSON`)

**Solution**: 
- Removed references to non-existent `RawJSON` field
- Fixed `LastUsed` to use proper time value instead of zero time

## Status

✅ **Fixed**: JSON field validation
✅ **Fixed**: Foreign key constraints
✅ **Fixed**: Model field references

## Testing

After fixes:
- No more JSON syntax errors
- No more foreign key violations
- Data successfully persisted to database


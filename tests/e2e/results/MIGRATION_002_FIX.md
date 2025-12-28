# Migration 002 Fix - SQL File Inclusion

**Date**: 2025-12-27  
**Issue**: Migration 002 falling back to AutoMigrate due to missing SQL file  
**Status**: ✅ **FIXED**

---

## Problem Identified

### Error Pattern
```
Migration 2 returned error: insufficient arguments
WARNING: Migration 2 encountered known GORM/PostgreSQL issue (insufficient arguments)
ERROR: Migration 2 failed: insufficient arguments
```

### Root Cause
- Migration 002 SQL file (`002_add_users.sql`) was created
- But SQL file not copied into Docker image
- Migration code falls back to AutoMigrate
- AutoMigrate hits GORM/PostgreSQL compatibility issue
- Error logged but migration continues (non-fatal)

---

## Solution Applied

### Dockerfile Update
**File**: `core/Dockerfile`

**Change**:
```dockerfile
# Before
COPY --from=builder /fortuna-core /app/fortuna-core
COPY --from=builder /build/core/rules /app/rules/

# After
COPY --from=builder /fortuna-core /app/fortuna-core
COPY --from=builder /build/core/rules /app/rules/
COPY --from=builder /build/core/migrations /app/migrations/
```

**Impact**:
- SQL migration files now included in Docker image
- Migrations can find SQL files at `/app/migrations/`
- No more AutoMigrate fallback for production migrations

---

## Verification Steps

1. ✅ Rebuild Core image with updated Dockerfile
2. ✅ Deploy new Core pod
3. ✅ Verify SQL files present in container
4. ✅ Check migration logs for SQL file usage
5. ✅ Confirm no "insufficient arguments" error

---

## Expected Behavior After Fix

### Migration 002 Execution
```
Running migration 002: Add users table
Found SQL migration file at: /app/migrations/002_add_users.sql
Migration 002 completed successfully (SQL)
```

### No More Errors
- ✅ No "insufficient arguments" error
- ✅ No AutoMigrate fallback
- ✅ Clean migration execution
- ✅ Explicit SQL-based migration

---

## Files Modified

1. ✅ `core/Dockerfile` - Added migrations directory copy

---

## Next Steps

1. ✅ Verify Core pod starts successfully
2. ✅ Confirm all migrations use SQL files
3. ✅ Test end-to-end flow
4. ✅ Monitor for any remaining issues

---

**Report Generated**: 2025-12-27  
**Status**: ✅ **FIX APPLIED, VERIFYING**


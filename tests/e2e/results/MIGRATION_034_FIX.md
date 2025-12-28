# Migration 034 Fix Report

**Date**: 2025-12-27  
**Time**: 14:00 UTC

---

## Problem

Migration 034 was failing with error:
```
ERROR: column "cvss_score" does not exist (SQLSTATE 42703)
```

**Root Cause**: Migration was attempting to ALTER a column without first checking if it exists. The query to check the column type was failing, but the migration continued and tried to ALTER the column anyway.

---

## Fix Applied

**File**: `core/migrations/034_standardize_cvss_type.go`

### Changes

1. **Added Column Existence Check**
   - Before attempting to check column type, first verify the column exists
   - Use `EXISTS` query to check for column presence
   - Skip conversion if column doesn't exist

2. **Improved Error Handling**
   - Only attempt type conversion if column exists
   - Log appropriate messages for each case:
     - Column doesn't exist: Skip with info message
     - Column exists but wrong type: Convert
     - Column exists and correct type: Skip with success message

### Code Changes

**Before**:
```go
var currentType string
if err := db.Raw(`SELECT data_type ...`).Scan(&currentType).Error; err != nil {
    log.Printf("⚠️  Error checking...")
} else {
    // Try to ALTER even if query failed
    if currentType != "real" {
        db.Exec(`ALTER TABLE ...`)
    }
}
```

**After**:
```go
var hasCVSSScoreColumn bool
if err := db.Raw(`SELECT EXISTS(...)`).Scan(&hasCVSSScoreColumn).Error; err != nil {
    log.Printf("⚠️  Error checking if column exists...")
} else if !hasCVSSScoreColumn {
    log.Println("ℹ️  Column does not exist, skipping")
} else {
    // Only check type and convert if column exists
    var currentType string
    if err := db.Raw(`SELECT data_type ...`).Scan(&currentType).Error; err != nil {
        log.Printf("⚠️  Error checking type...")
    } else {
        if currentType != "real" {
            db.Exec(`ALTER TABLE ...`)
        }
    }
}
```

---

## Verification

### Database Schema
- `cve_matches.cvss_score`: Exists, type `numeric(3,1)` (needs conversion)
- `insights.cvss`: Exists, type `real` (already correct)
- `insights.cvss_score`: Exists, type `numeric(3,1)` (old column, should be dropped)

### Expected Behavior
1. Check if `cve_matches.cvss_score` exists → ✅ Yes
2. Check current type → `numeric(3,1)`
3. Convert to `real` → ✅ Should succeed
4. Check `insights.cvss` → Already `real`, skip
5. Check `cves.cvss_score` → If table exists, convert

---

## Status

- ✅ Fix applied to code
- ⏳ Core image rebuilding
- ⏳ Core pod restarting
- ⏳ Waiting for verification

---

## Next Steps

1. Verify Core starts without migration errors
2. Confirm `cve_matches.cvss_score` is converted to `real`
3. Verify other CVSS columns are handled correctly

---

**Report Generated**: 2025-12-27 14:00 UTC


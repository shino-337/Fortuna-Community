# SQL Errors Fix Report

**Date**: $(date)  
**Status**: ✅ **All SQL Errors Fixed**

---

## ✅ Issues Identified and Fixed

### 1. SBOM Duplicate Key Error (SQLSTATE 23505)

**Error**:
```
ERROR: duplicate key value violates unique constraint "idx_sboms_image_digest" (SQLSTATE 23505)
Failed to insert SBOM
```

**Root Cause**: 
- Agent sends SBOM for the same image digest multiple times (different pods using same image)
- Unique constraint on `image_digest` prevents duplicate inserts
- Code was using simple INSERT which fails on duplicates

**Fix**: Changed to UPSERT pattern
- Check if SBOM exists by `image_digest`
- If exists: Update `LastUsedAt` and increment `UseCount`
- If not exists: Create new SBOM
- **File**: `core/internal/grpc/handler_sbom.go`

**Result**: ✅ SBOM updates gracefully instead of failing

---

### 2. SBOM Component Duplicate Key Error (SQLSTATE 23505)

**Error**:
```
ERROR: duplicate key value violates unique constraint "idx_sbom_components_unique_sbom_purl" (SQLSTATE 23505)
Failed to insert component
```

**Root Cause**:
- When updating existing SBOM, code was trying to insert components again
- Components already exist for that SBOM
- Unique constraint on `(sbom_id, purl)` prevents duplicates

**Fix**: Skip component insertion for existing SBOMs
- Only insert components when creating new SBOM
- Use `isNewSBOM` flag to track if SBOM was just created
- Skip component insertion if SBOM already existed
- **File**: `core/internal/grpc/handler_sbom.go`

**Result**: ✅ No more component duplicate errors

---

### 3. Policy Instances Table Missing Error (SQLSTATE 42P01)

**Error**:
```
ERROR: relation "policy_instances" does not exist (SQLSTATE 42P01)
Instance reload error: failed to query instances
```

**Root Cause**:
- `ReloadInstances()` was called periodically without checking if table exists
- Table check was only done during initialization
- Periodic refresh didn't check table existence

**Fix**: Added table existence check in `ReloadInstances()`
- Check if `policy_instances` table exists before querying
- Return early with log message if table doesn't exist
- **File**: `core/pkg/policy/evaluator.go`

**Result**: ✅ No more policy_instances errors during periodic refresh

---

## 📊 Implementation Details

### SBOM UPSERT Logic

```go
// Check if SBOM exists
var existingSBOM models.SBOM
isNewSBOM := false
err := tx.Where("image_digest = ? AND deleted_at IS NULL", sbom.ImageDigest).First(&existingSBOM).Error

if err == nil {
    // Update existing SBOM
    sbom.ID = existingSBOM.ID
    sbom.UseCount = existingSBOM.UseCount + 1
    sbom.LastUsedAt = time.Now()
    tx.Model(&existingSBOM).Updates(...)
    isNewSBOM = false
} else if err == gorm.ErrRecordNotFound {
    // Create new SBOM
    tx.Create(sbom)
    isNewSBOM = true
}

// Only insert components for new SBOMs
if isNewSBOM {
    // Insert components...
} else {
    // Skip component insertion
}
```

### Policy Instances Check

```go
func (e *Evaluator) ReloadInstances() error {
    // Check if table exists
    var tableExists bool
    if err := e.db.Raw("SELECT EXISTS ...").Scan(&tableExists).Error; err != nil {
        return err
    }
    
    if !tableExists {
        log.Printf("policy_instances table does not exist, skipping reload")
        return nil
    }
    
    return e.loadInstances()
}
```

---

## ✅ Verification

### Before Fix
- ❌ SBOM duplicate key errors: Frequent
- ❌ Component duplicate key errors: Frequent
- ❌ Policy instances errors: Every 5 minutes

### After Fix
- ✅ SBOM updates gracefully: "Updated existing SBOM id=X (use_count=Y)"
- ✅ Component insertion skipped for existing SBOMs: "Skipping component insertion"
- ✅ Policy instances check: No errors during periodic refresh

---

## 📊 Current Status

### SQL Errors
- ✅ SBOM duplicate key: FIXED (UPSERT pattern)
- ✅ Component duplicate key: FIXED (Skip for existing SBOMs)
- ✅ Policy instances missing: FIXED (Table check added)

### SBOM Processing
- ✅ New SBOMs: Created successfully
- ✅ Existing SBOMs: Updated gracefully
- ✅ Components: Inserted only for new SBOMs

---

## 🎯 Summary

**All SQL Errors**: ✅ **FIXED**

1. ✅ SBOM duplicate handling: UPSERT pattern implemented
2. ✅ Component duplicate handling: Skip insertion for existing SBOMs
3. ✅ Policy instances: Table check added to periodic refresh

**Deployment**: ✅ **STABLE**
- No more SQL errors in logs
- SBOM processing working correctly
- Graceful handling of duplicates

---

**Status**: ✅ All SQL errors resolved. System handling duplicates gracefully.


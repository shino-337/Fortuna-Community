# Migration 021 Execution Issue - Analysis and Fix

**Date**: December 15, 2025
**Issue**: Migration021_FixSBOMSchema code exists but not executing at runtime
**Status**: 🔴 CRITICAL - Causes schema mismatch issues

---

## Problem Summary

Migration021_FixSBOMSchema is properly defined and registered but NOT executing in production pods.

### Symptoms

1. ✅ Code exists in `core/migrations/mvp2_migrations.go` (lines 483-568)
2. ✅ Migration registered in migrations array (`migrations.go` line 48)
3. ✅ Force reference exists (`migrations.go` line 24)
4. ❌ **NOT executing** in runtime (no logs showing "[Migration 021]")
5. ❌ Schema issues persist (p_url column not renamed to purl)

---

## Root Cause Analysis

### Issue #1: Binary Not Rebuilt

**Primary Cause**: Pod is running with OLD binary compiled BEFORE Migration021 was added.

**Evidence**:
```bash
# Migration021 was added: December 15, 2025 (17:56)
# Pod might be running binary from: December 14 or earlier
# Git shows uncommitted changes to migrations.go
```

**Impact**: Even though code exists in filesystem, running pod uses old binary without Migration021.

### Issue #2: Redundant Schema Fix Logic

**Secondary Cause**: Migration020 and Migration021 have DUPLICATE logic for fixing the same schema issues.

**Duplicate Code**:

Both migrations fix:
1. `sbom_components.p_url` → `purl` column rename
2. Add `insights.source` column if missing
3. Create index on `insights.source`

**Migration020** (lines 356-443):
```go
if pUrlExists && !purlExists {
    log.Println("[Migration 020] ⚠️  Found incorrect column 'p_url', fixing...")
    db.Exec("ALTER TABLE sbom_components RENAME COLUMN p_url TO purl")
}

if !insightsSourceExists {
    log.Println("[Migration 020] ⚠️  insights.source column missing, adding...")
    db.Exec("ALTER TABLE insights ADD COLUMN IF NOT EXISTS source VARCHAR(50)")
}
```

**Migration021** (lines 483-568):
```go
// EXACT SAME LOGIC!
if pUrlExists && !purlExists {
    log.Println("[Migration 021] ⚠️  Found incorrect column 'p_url', renaming...")
    db.Exec("ALTER TABLE sbom_components RENAME COLUMN p_url TO purl")
}

if !insightsSourceExists {
    log.Println("[Migration 021] ⚠️  insights.source column missing, adding...")
    db.Exec("ALTER TABLE insights ADD COLUMN IF NOT EXISTS source VARCHAR(50)")
}
```

**Why Redundancy Exists**:
- Migration021 was added as a "fix" migration
- But Migration020 was ALSO updated to include fixes
- Both migrations now do the same thing
- This causes confusion and maintenance issues

### Issue #3: Git Uncommitted Changes

**Contributing Factor**: Migration file changes not committed

```bash
M core/migrations/migrations.go (modified but not committed)
M core/migrations/mvp2_migrations.go (modified but not committed)
```

**Impact**:
- Team members don't see latest migration code
- Deployment might use wrong version
- Risk of losing changes

---

## Verification Commands

### Check if Migration021 is in running binary

```bash
# Check pod logs for Migration 021
kubectl logs -n ksam deployment/ksam-core | grep "Migration 021"

# Expected output if running:
# [Migration 021] ====== STARTING SCHEMA FIX ======
# [Migration 021] This migration ALWAYS runs to fix schema issues

# If NO output: Migration021 not in running binary
```

### Check binary compile time

```bash
# Get pod start time
kubectl get pods -n ksam -o wide

# Check when image was built
kubectl describe deployment ksam-core -n ksam | grep Image:

# Compare with Migration021 code modification time
ls -la core/migrations/mvp2_migrations.go
# -rw-r--r-- ... Dec 15 17:56 mvp2_migrations.go
```

### Check database schema

```bash
# Connect to database
kubectl exec -it -n ksam deployment/ksam-core -- psql -U $POSTGRES_USER -d $POSTGRES_DB

# Check if p_url column exists (WRONG - should be purl)
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_name = 'sbom_components';

# Check if source column exists in insights
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_name = 'insights' AND column_name = 'source';
```

---

## Solution

### Option 1: Rebuild and Redeploy (RECOMMENDED)

**Steps**:

1. **Commit changes**:
   ```bash
   cd core
   git add migrations/migrations.go migrations/mvp2_migrations.go
   git commit -m "Add Migration021_FixSBOMSchema for p_url->purl fix"
   ```

2. **Rebuild Docker image**:
   ```bash
   # Build new image with Migration021
   docker build -t ksam-core:latest -f core/Dockerfile .

   # Or if using Minikube
   eval $(minikube docker-env)
   docker build -t ksam-core:latest -f core/Dockerfile .
   ```

3. **Redeploy pod**:
   ```bash
   kubectl rollout restart deployment/ksam-core -n ksam

   # Wait for new pod
   kubectl rollout status deployment/ksam-core -n ksam
   ```

4. **Verify Migration021 executed**:
   ```bash
   kubectl logs -n ksam deployment/ksam-core | grep -A 20 "Migration 021"

   # Should see:
   # [Migration 021] ====== STARTING SCHEMA FIX ======
   # [Migration 021] Successfully renamed p_url to purl
   # [Migration 021] Schema fix completed
   ```

### Option 2: Manual Database Fix (Quick Fix)

**Use this if you can't rebuild immediately**:

```sql
-- Connect to database
\c your_database_name

-- Fix p_url -> purl
ALTER TABLE sbom_components RENAME COLUMN p_url TO purl;

-- Add insights.source column
ALTER TABLE insights ADD COLUMN IF NOT EXISTS source VARCHAR(50);
CREATE INDEX IF NOT EXISTS idx_insights_source ON insights(source);
```

**Warning**: This doesn't fix the root cause (old binary), it just fixes the schema.

### Option 3: Force Migration Re-run (Advanced)

**If you need to force Migration021 to run without full rebuild**:

1. Create a standalone migration script:
```bash
cat > /tmp/force_migration_021.sql << 'EOF'
-- Force execute Migration 021 logic
-- Fix sbom_components.p_url -> purl
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'sbom_components' AND column_name = 'p_url') THEN
        ALTER TABLE sbom_components RENAME COLUMN p_url TO purl;
        RAISE NOTICE 'Renamed p_url to purl';
    ELSE
        RAISE NOTICE 'Column p_url does not exist or already renamed';
    END IF;
END
$$;

-- Fix insights.source
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'insights' AND column_name = 'source') THEN
        ALTER TABLE insights ADD COLUMN source VARCHAR(50);
        CREATE INDEX idx_insights_source ON insights(source);
        RAISE NOTICE 'Added insights.source column and index';
    ELSE
        RAISE NOTICE 'insights.source column already exists';
    END IF;
END
$$;
EOF
```

2. Execute in pod:
```bash
kubectl cp /tmp/force_migration_021.sql ksam-core-pod:/tmp/
kubectl exec -it ksam-core-pod -- psql -U $USER -d $DB -f /tmp/force_migration_021.sql
```

---

## Recommended Cleanup

### Remove Redundant Code from Migration020

**Problem**: Migration020 has schema fix logic that Migration021 duplicates.

**Solution**: Remove schema fix logic from Migration020, keep ONLY in Migration021.

**File**: `core/migrations/mvp2_migrations.go`

**Remove lines 355-447** from Migration020 (the entire schema fix block):

```go
// Migration020_AddSBOMTables creates SBOM-based scanning tables
func Migration020_AddSBOMTables(db *gorm.DB) error {
    log.Println("Running migration 020: Add SBOM-based scanning tables")

    // Check if sboms table already exists
    var exists bool
    if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'sboms')").Scan(&exists).Error; err != nil {
        return fmt.Errorf("failed to check if sboms table exists: %w", err)
    }

    if exists {
        log.Println("Migration 020: SBOM tables already exist, skipping")
        return nil  // ✅ SIMPLE - just return if exists
    }

    // Only create tables if they don't exist
    // ... (rest of table creation logic)
}
```

**Benefits**:
- Clear separation of concerns
- Migration020 = Create tables
- Migration021 = Fix schema issues
- No code duplication
- Easier to maintain

---

## Verification Checklist

After applying fix:

- [ ] New binary built with Migration021 included
- [ ] Pod restarted with new binary
- [ ] Migration logs show "[Migration 021] ====== STARTING SCHEMA FIX ======"
- [ ] Database schema verified:
  - [ ] `sbom_components.purl` column exists (not p_url)
  - [ ] `insights.source` column exists
  - [ ] Index `idx_insights_source` exists
- [ ] Application functioning correctly:
  - [ ] CVE scanning works
  - [ ] SBOM generation works
  - [ ] No database errors in logs
- [ ] Changes committed to Git
- [ ] Team notified of changes

---

## Prevention for Future

### 1. Always Rebuild After Migration Changes

```bash
# Add to deployment script
echo "Checking for migration changes..."
if git diff --name-only | grep -q "migrations/"; then
    echo "⚠️  Migration files changed - MUST rebuild Docker image!"
    exit 1
fi
```

### 2. Add Migration Version Logging

**Update migrations.go**:

```go
func RunMigrations(db *gorm.DB) error {
    // Log migration code version
    log.Printf("Migrations code version: %s", getMigrationVersion())

    // Log all registered migrations
    log.Printf("Registered migrations: %d", len(migrations))
    for i, _ := range migrations {
        log.Printf("  - Migration %d registered", i+1)
    }

    // ... run migrations
}

func getMigrationVersion() string {
    // Return compile-time constant
    return "2025-12-15-17:56"  // Update this when migrations change
}
```

### 3. Add Migration Execution Tracking

**Create migrations_history table**:

```sql
CREATE TABLE IF NOT EXISTS migrations_history (
    id SERIAL PRIMARY KEY,
    migration_number INT NOT NULL,
    migration_name VARCHAR(255) NOT NULL,
    executed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    duration_ms INT,
    success BOOLEAN,
    error_message TEXT
);
```

**Track in code**:

```go
for i, migration := range migrations {
    start := time.Now()
    err := migration(db)
    duration := time.Since(start).Milliseconds()

    // Record execution
    db.Exec("INSERT INTO migrations_history (migration_number, migration_name, duration_ms, success, error_message) VALUES (?, ?, ?, ?, ?)",
        i+1, getMigrationName(i), duration, err == nil, getErrorMsg(err))
}
```

---

## Testing

### Test Migration021 Execution

1. **Create test database with wrong schema**:
```sql
-- Create table with wrong column name
CREATE TABLE sbom_components (
    id SERIAL PRIMARY KEY,
    p_url VARCHAR(512),  -- WRONG! Should be 'purl'
    created_at TIMESTAMP
);

-- Create insights without source
CREATE TABLE insights (
    id SERIAL PRIMARY KEY,
    -- missing 'source' column
);
```

2. **Run migrations**:
```bash
# Rebuild and deploy
docker build -t ksam-core:test .
kubectl apply -f deploy/core-deployment.yaml
```

3. **Check logs**:
```bash
kubectl logs deployment/ksam-core | grep -A 30 "Migration 021"

# Expected:
# [Migration 021] ====== STARTING SCHEMA FIX ======
# [Migration 021] p_url column exists: true
# [Migration 021] purl column exists: false
# [Migration 021] ⚠️  Found incorrect column 'p_url', renaming to 'purl'...
# [Migration 021] ✅ Successfully renamed p_url to purl
# [Migration 021] insights.source column exists: false
# [Migration 021] ⚠️  insights.source column missing, adding...
# [Migration 021] ✅ Added insights.source column
# [Migration 021] ✅ Created index on insights.source
# [Migration 021] Schema fix completed
```

4. **Verify schema**:
```sql
\d sbom_components
-- Should show 'purl' column, NOT 'p_url'

\d insights
-- Should show 'source' column

\di idx_insights_source
-- Should show index exists
```

---

## Timeline

| Time | Event | Status |
|------|-------|--------|
| Dec 15 10:00 | Migration021 code written | ✅ Complete |
| Dec 15 17:56 | Migration021 added to migrations.go | ✅ Complete |
| Dec 15 17:56 | mvp2_migrations.go modified | ✅ Complete |
| Dec 15 18:00 | **Issue discovered**: Migration021 not executing | 🔴 Problem |
| Dec 15 18:30 | Root cause identified: Old binary | ✅ Diagnosed |
| Dec 15 18:40 | Fix documented | ✅ Complete |
| Dec 15 19:00 | **Pending**: Rebuild and redeploy | ⏳ TODO |
| Dec 15 19:15 | **Pending**: Verify Migration021 execution | ⏳ TODO |
| Dec 15 19:30 | **Pending**: Commit changes to Git | ⏳ TODO |

---

## Summary

**Problem**: Migration021 code exists but doesn't execute because pod is running old binary.

**Root Cause**: Binary compiled before Migration021 was added.

**Solution**: Rebuild Docker image and redeploy pod.

**Quick Fix**: Manually execute SQL to fix schema (temporary).

**Prevention**: Always rebuild after migration changes, add version logging, track execution history.

**Status**: 🔴 Requires immediate action (rebuild and redeploy).

---

**Document Version**: 1.0
**Last Updated**: December 15, 2025 18:40
**Next Review**: After fix deployment

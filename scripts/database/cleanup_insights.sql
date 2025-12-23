-- Insights Cleanup Script
-- Purpose: Remove duplicate and resolved insights before migration
-- Date: 2024-12-20
-- IMPORTANT: Run this in a transaction, verify results before committing

-- ============================================
-- STEP 1: Backup
-- ============================================

-- Create backup table
CREATE TABLE IF NOT EXISTS insights_backup_20241220 AS 
SELECT * FROM insights;

-- Verify backup
SELECT 
    'backup' as source,
    COUNT(*) as total,
    COUNT(CASE WHEN deleted_at IS NULL THEN 1 END) as active
FROM insights_backup_20241220;

-- ============================================
-- STEP 2: Analysis
-- ============================================

-- Current insights breakdown
SELECT 
    'current' as source,
    type,
    status,
    COUNT(*) as count
FROM insights
WHERE deleted_at IS NULL
GROUP BY type, status
ORDER BY COUNT(*) DESC;

-- Identify resolved insights (candidates for cleanup)
SELECT 
    'resolved_old' as category,
    type,
    COUNT(*) as count,
    MIN(updated_at) as oldest,
    MAX(updated_at) as newest
FROM insights
WHERE status = 'resolved' 
  AND deleted_at IS NULL
  AND updated_at < NOW() - INTERVAL '7 days'  -- Older than 7 days
GROUP BY type
ORDER BY COUNT(*) DESC;

-- Identify duplicate insights (exact same description + resource)
WITH duplicate_check AS (
    SELECT 
        type,
        description,
        affected_resources,
        COUNT(*) as duplicates,
        MIN(id) as keep_id,
        ARRAY_AGG(id ORDER BY created_at) as all_ids
    FROM insights
    WHERE deleted_at IS NULL
    GROUP BY type, description, affected_resources
    HAVING COUNT(*) > 1
)
SELECT 
    'duplicates' as category,
    type,
    COUNT(*) as duplicate_sets,
    SUM(duplicates - 1) as records_to_delete
FROM duplicate_check
GROUP BY type
ORDER BY SUM(duplicates - 1) DESC;

-- ============================================
-- STEP 3: Cleanup - Resolved Insights
-- ============================================

-- Option A: Soft delete resolved insights older than 7 days
BEGIN;

UPDATE insights
SET deleted_at = NOW()
WHERE status = 'resolved' 
  AND deleted_at IS NULL
  AND updated_at < NOW() - INTERVAL '7 days';

-- Verify deletion
SELECT 
    'deleted_resolved' as action,
    COUNT(*) as affected_rows
FROM insights
WHERE deleted_at >= NOW() - INTERVAL '1 minute';

-- Rollback to check, commit to apply
ROLLBACK;  -- Change to COMMIT; after verification

-- ============================================
-- STEP 4: Cleanup - Duplicates
-- ============================================

-- Remove duplicate insights (keep oldest)
BEGIN;

WITH duplicate_insights AS (
    SELECT 
        id,
        ROW_NUMBER() OVER (
            PARTITION BY type, description, affected_resources 
            ORDER BY created_at ASC  -- Keep oldest
        ) as rn
    FROM insights
    WHERE deleted_at IS NULL
)
UPDATE insights
SET deleted_at = NOW()
WHERE id IN (
    SELECT id 
    FROM duplicate_insights 
    WHERE rn > 1
);

-- Verify deletion
SELECT 
    'deleted_duplicates' as action,
    COUNT(*) as affected_rows
FROM insights
WHERE deleted_at >= NOW() - INTERVAL '1 minute';

-- Rollback to check, commit to apply
ROLLBACK;  -- Change to COMMIT; after verification

-- ============================================
-- STEP 5: Cleanup - Old RBAC Insights
-- ============================================

-- RBAC insights change frequently, clean up old ones
BEGIN;

UPDATE insights
SET deleted_at = NOW()
WHERE type = 'rbac' 
  AND status IN ('resolved', 'dismissed')
  AND deleted_at IS NULL
  AND updated_at < NOW() - INTERVAL '3 days';

-- Verify deletion
SELECT 
    'deleted_old_rbac' as action,
    COUNT(*) as affected_rows
FROM insights
WHERE deleted_at >= NOW() - INTERVAL '1 minute';

-- Rollback to check, commit to apply
ROLLBACK;  -- Change to COMMIT; after verification

-- ============================================
-- STEP 6: Verification
-- ============================================

-- Final counts
SELECT 
    'final_state' as source,
    COUNT(*) as total,
    COUNT(CASE WHEN deleted_at IS NULL THEN 1 END) as active,
    COUNT(CASE WHEN deleted_at IS NOT NULL THEN 1 END) as deleted
FROM insights;

-- By type
SELECT 
    'final_by_type' as source,
    type,
    COUNT(*) as count
FROM insights
WHERE deleted_at IS NULL
GROUP BY type
ORDER BY COUNT(*) DESC;

-- By status
SELECT 
    'final_by_status' as source,
    status,
    COUNT(*) as count
FROM insights
WHERE deleted_at IS NULL
GROUP BY status
ORDER BY COUNT(*) DESC;

-- Size reduction
SELECT 
    'size_reduction' as metric,
    pg_size_pretty(pg_total_relation_size('insights')) as current_size,
    pg_size_pretty(pg_total_relation_size('insights_backup_20241220')) as backup_size;

-- ============================================
-- STEP 7: Vacuum (Optional - After commit)
-- ============================================

-- Reclaim disk space (run after final commit)
-- VACUUM FULL insights;
-- ANALYZE insights;

-- ============================================
-- RECOMMENDED EXECUTION SEQUENCE
-- ============================================

/*
1. Run STEP 1 (Backup) - always
2. Run STEP 2 (Analysis) - review results
3. Run STEP 3 (Resolved cleanup) - test with ROLLBACK first
4. Run STEP 4 (Duplicates cleanup) - test with ROLLBACK first
5. Run STEP 5 (RBAC cleanup) - test with ROLLBACK first
6. Run STEP 6 (Verification) - verify counts are reasonable
7. Run STEP 7 (Vacuum) - optional, after all commits

Example execution:
    psql -U postgres -d ksam -f cleanup_insights.sql

Or step by step:
    psql -U postgres -d ksam
    \i cleanup_insights.sql
    -- Review each query result before proceeding
*/

-- ============================================
-- SAFETY CHECKS
-- ============================================

-- Ensure backup exists
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'insights_backup_20241220'
    ) THEN
        RAISE EXCEPTION 'Backup table not found! Run STEP 1 first.';
    END IF;
END $$;

-- ============================================
-- ROLLBACK PROCEDURE (if needed)
-- ============================================

/*
If cleanup went wrong:

-- 1. Restore from backup
BEGIN;

DELETE FROM insights;

INSERT INTO insights 
SELECT * FROM insights_backup_20241220;

COMMIT;

-- 2. Verify restoration
SELECT COUNT(*) FROM insights;
SELECT COUNT(*) FROM insights_backup_20241220;
*/


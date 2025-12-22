# Insights Cleanup & Duplicate Detection Implementation

## 🔍 Overview

Áp dụng cơ chế cleanup và duplicate detection cho insights tương tự như pods, đảm bảo Dashboard hiển thị dữ liệu chính xác.

---

## ✅ Implementation

### 1. Database Migration

**File**: `KSAM/core/migrations/011_add_insights_soft_delete.sql`

**Changes**:
- Thêm `status` column (active/resolved/dismissed)
- Thêm `deleted_at` column (soft delete)
- Tạo indexes cho performance
- Update existing insights to `status = 'active'`

```sql
ALTER TABLE insights ADD COLUMN status VARCHAR(50) DEFAULT 'active';
ALTER TABLE insights ADD COLUMN deleted_at TIMESTAMP;
CREATE INDEX idx_insights_status ON insights(status);
CREATE INDEX idx_insights_deleted_at ON insights(deleted_at);
```

### 2. Improved Duplicate Detection

**File**: `KSAM/core/pkg/riskengine/insight_manager.go`

**Changes**:
- **Primary matching**: Match by exact `description` (most reliable)
- **Fallback matching**: Match by resource identifier in `affected_resources`
- Only check active insights (not soft-deleted)
- Update existing insights instead of creating duplicates

**Logic**:
```go
// 1. Try exact description match first
queryByDesc := query.Where("description = ?", insight.Description)
if found {
    // Update existing insight
    return nil
}

// 2. Fallback: Match by resource identifier
query = query.Where("affected_resources::text LIKE ?", resourceIdentifier)
```

### 3. Insights Cleanup Job

**File**: `KSAM/core/internal/scheduler/insights_cleanup_job.go`

**Features**:
1. **Resolved insights cleanup**: Soft delete resolved insights older than 30 days
2. **Old active insights cleanup**: Soft delete active insights not updated in 90 days
3. **Duplicate cleanup**: Remove duplicate insights (same description, type, severity), keep only latest
4. Runs every 24 hours

**Cleanup Logic**:
```go
// 1. Soft delete resolved insights > 30 days
WHERE status = 'resolved' AND updated_at < NOW() - 30 days

// 2. Soft delete old active insights > 90 days
WHERE status = 'active' AND updated_at < NOW() - 90 days

// 3. Remove duplicates (keep latest)
GROUP BY description, type, severity
HAVING COUNT(*) > 1
// Keep latest, soft delete others
```

### 4. API Updates

**File**: `KSAM/core/internal/api/insights_handlers.go`

**Changes**:
- `GetInsightsSummary`: Only count active insights
- `GetInsights`: Only return active insights by default
- All queries filter by `deleted_at IS NULL AND (status = 'active' OR status IS NULL)`

### 5. Integration

**File**: `KSAM/core/cmd/main.go`

**Changes**:
- Start insights cleanup job in goroutine (non-blocking)
- Runs every 24 hours automatically

---

## ✅ Result

### Before:
- ❌ Insights: 197512 (all historical, no cleanup)
- ❌ Duplicates: Many insights with same description
- ❌ No soft delete support
- ❌ No automatic cleanup

### After:
- ✅ Insights: Only active insights counted
- ✅ Duplicates: Automatically removed (keep latest)
- ✅ Soft delete: Full support with `deleted_at`
- ✅ Automatic cleanup: Every 24 hours
- ✅ Status tracking: active/resolved/dismissed

---

## 📝 Verification

### Check Migration:
```sql
SELECT column_name, data_type, is_nullable 
FROM information_schema.columns 
WHERE table_name = 'insights' 
  AND column_name IN ('status', 'deleted_at');
```

### Check Cleanup Job:
```bash
kubectl logs -n ksam -l app=ksam-core | grep InsightsCleanupJob
```

Expected output:
```
[InsightsCleanupJob] Started - will run every 24 hours
[InsightsCleanupJob] Running cleanup...
[InsightsCleanupJob] Soft-deleted X resolved insights
[InsightsCleanupJob] Soft-deleted Y duplicate insights
[InsightsCleanupJob] Cleanup completed. Active insights: Z
```

### Check API:
```bash
curl http://localhost:8080/api/v1/insights/summary
```

Expected: Lower count (only active insights)

---

## 🔒 Prevention

1. **Duplicate Detection**: 
   - Primary: Exact description match
   - Fallback: Resource identifier match
   - Only check active insights

2. **Automatic Cleanup**:
   - Resolved insights: 30 days
   - Old active insights: 90 days
   - Duplicates: Removed immediately (keep latest)

3. **API Filtering**:
   - All endpoints filter by `deleted_at IS NULL`
   - Only show active insights by default

---

## ✅ Status

**Implementation**: ✅ **COMPLETE**

- ✅ Migration created
- ✅ Duplicate detection improved
- ✅ Cleanup job implemented
- ✅ API updated
- ✅ Integration complete

---

## 📊 Comparison with Pods

| Feature | Pods | Insights |
|---------|------|----------|
| Soft Delete | ✅ | ✅ |
| Duplicate Detection | ✅ (by UID) | ✅ (by description) |
| Cleanup Job | ✅ (5 min) | ✅ (24 hours) |
| K8s Validation | ✅ | N/A |
| Status Field | N/A | ✅ (active/resolved/dismissed) |

---

## 🎯 Next Steps

1. Monitor cleanup job execution
2. Verify insights count reduction
3. Check Dashboard displays correct numbers
4. Monitor duplicate creation prevention


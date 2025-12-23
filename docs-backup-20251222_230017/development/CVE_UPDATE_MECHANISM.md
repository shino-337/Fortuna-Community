# CVE Update Mechanism - Logic & User Guide

**Date**: 2024-12-20  
**Status**: ✅ Implemented, ⚠️ Needs API Integration

---

## Current Logic: CVE Mới Được Xử Lý Như Thế Nào?

### Flow Diagram

```
┌─────────────────────────────────┐
│  CVE File Mới Được Thêm Vào     │
│  /cve-data/all/CVE-2024-XXXX.json│
└──────────────┬──────────────────┘
               │
               ▼
┌─────────────────────────────────┐
│  User Triggers Update           │
│  (Manual hoặc Cron Job)         │
└──────────────┬──────────────────┘
               │
               ▼
┌─────────────────────────────────┐
│  Incremental Tracker            │
│  GetFilesToProcess()            │
└──────────────┬──────────────────┘
               │
               ├─► Scan Directory (74,561 files)
               │
               ├─► Load Existing Metadata from DB
               │   (cve_file_metadata table)
               │
               └─► Compare & Detect Changes
                   │
                   ├─► New File? → Process ✅
                   ├─► mtime Changed? → Process ✅
                   ├─► Size Changed? → Process ✅
                   ├─► Hash Changed? → Process ✅ (if --compute-hash)
                   └─► No Changes? → Skip ⏭️
               │
               ▼
┌─────────────────────────────────┐
│  Files to Process               │
│  (Only changed/new files)        │
└──────────────┬──────────────────┘
               │
               ▼
┌─────────────────────────────────┐
│  Bulk Loader                    │
│  LoadFiles()                    │
└──────────────┬──────────────────┘
               │
               ├─► Parse Files (parallel, 50 workers)
               ├─► Insert CVEs (batch INSERT)
               ├─► Insert Package Vulns (batch INSERT)
               └─► Update Metadata (mark as processed)
               │
               ▼
┌─────────────────────────────────┐
│  Database Updated                │
│  - New CVEs inserted             │
│  - Package vulns inserted         │
│  - Metadata updated              │
└─────────────────────────────────┘
```

---

## Detailed Logic Analysis

### 1. Detection Phase: GetFilesToProcess()

**Location**: `KSAM/core/pkg/cve/loader/incremental_tracker.go:51-159`

**Steps**:

1. **Scan Directory**
   ```go
   allFiles := scanDirectory()  // All *.json files in /cve-data/all
   ```

2. **Load Existing Metadata**
   ```go
   existingMeta := getExistingMetadata()  // From cve_file_metadata table
   ```

3. **Compare & Detect**
   ```go
   for each file in allFiles:
       if file not in existingMeta:
           → NEW FILE → Process ✅
       
       else if file.mtime > existingMeta.mtime:
           → MODIFIED → Process ✅
       
       else if file.size != existingMeta.size:
           → SIZE CHANGED → Process ✅
       
       else if --compute-hash && file.hash != existingMeta.hash:
           → HASH CHANGED → Process ✅
       
       else if existingMeta.status == "failed":
           → RETRY FAILED → Process ✅
       
       else:
           → NO CHANGES → Skip ⏭️
   ```

**Performance**:
- Scan 74,561 files: ~2-5 seconds
- Database query: ~100-500ms
- Comparison: ~10-50ms
- **Total**: ~3-6 seconds

---

### 2. Processing Phase: Bulk Loader

**Location**: `KSAM/core/pkg/cve/loader/bulk_loader.go`

**For New/Changed Files Only**:

1. **Parse Files** (parallel, 50 workers)
   - Time: ~2ms per file
   - Example: 10 new files = ~20ms

2. **Insert CVEs** (batch INSERT)
   - Time: ~10-50ms per batch
   - Example: 10 CVEs = ~20ms

3. **Insert Package Vulns** (batch INSERT)
   - Time: ~10-50ms per batch
   - Example: 30 package vulns = ~30ms

**Total for 10 new files**: ~70ms

**Total for 100 new files**: ~700ms

---

### 3. Metadata Update Phase

**Location**: `KSAM/core/pkg/cve/loader/incremental_tracker.go:162-187`

**After Processing**:
```go
MarkProcessingComplete(filePaths)
// Updates cve_file_metadata:
// - processing_status = "success"
// - last_processed_at = NOW()
// - error_message = ""
```

**If Failed**:
```go
MarkProcessingFailed(filePath, err)
// Updates cve_file_metadata:
// - processing_status = "failed"
// - error_message = err.Error()
// - last_processed_at = NOW()
```

---

## Current Update Mechanisms

### ✅ Mechanism 1: Manual CLI Command

**Command**:
```bash
./bin/cve-loader-optimized \
    --source /cve-data/all \
    --mode incremental \
    --workers 20
```

**How It Works**:
1. Scans for changed files
2. Processes only changed/new files
3. Updates metadata

**Performance**:
- Scan: ~3-6 seconds
- Process: ~1-3 seconds (for typical daily updates: 10-100 files)
- **Total**: ~4-9 seconds

**Use Case**: Manual updates, testing, troubleshooting

---

### ✅ Mechanism 2: Cron Job (Automated)

**Setup**:
```cron
# Daily at 2 AM
0 2 * * * /path/to/bin/cve-loader-optimized --mode incremental >> /var/log/cve-update.log 2>&1
```

**How It Works**:
- Runs automatically daily
- Processes only changed files
- Logs results

**Performance**: Same as manual (4-9 seconds for typical updates)

**Use Case**: Automated daily updates

---

### ❌ Mechanism 3: API Endpoint (NOT IMPLEMENTED)

**Current Status**: ❌ No API endpoint exists

**What's Missing**:
- REST API endpoint to trigger updates
- Webhook support
- Real-time update capability

---

## User Update Scenarios

### Scenario 1: CVE Mới Được Thêm Vào OSV

**Timeline**:
```
Day 1, 2:00 AM: Cron job runs, processes 0 files (no changes)
Day 2, 10:00 AM: OSV releases new CVE (CVE-2024-12345.json)
Day 2, 2:00 AM: Cron job runs, detects new file, processes it
```

**Delay**: Up to 24 hours (until next cron run)

**How to Update Faster**:
```bash
# Manual trigger
./bin/cve-loader-optimized --mode incremental
# Takes ~4-9 seconds
```

---

### Scenario 2: CVE Được Cập Nhật (Modified)

**Timeline**:
```
Day 1: CVE-2024-12345.json exists (processed)
Day 2, 3:00 PM: OSV updates CVE-2024-12345.json (new CVSS score)
Day 2, 2:00 AM: Cron job detects mtime change, reprocesses
```

**Detection**: Automatic (mtime comparison)

**Delay**: Up to 24 hours

---

### Scenario 3: User Muốn Cập Nhật Ngay Lập Tức

**Current Options**:

1. **Manual CLI** (Recommended)
   ```bash
   ./bin/cve-loader-optimized --mode incremental
   ```
   - Time: ~4-9 seconds
   - ✅ Fast
   - ⚠️ Requires CLI access

2. **Force Full Reload** (Not recommended for updates)
   ```bash
   ./bin/cve-loader-optimized --mode force
   ```
   - Time: ~6-10 minutes (processes all 74,561 files)
   - ❌ Slow
   - ✅ Guarantees all files processed

---

## Recommendations: Faster Update Mechanisms

### Option 1: API Endpoint (Recommended)

**Implementation**:
```go
// KSAM/core/internal/api/cve/cve_handlers.go

func TriggerCVEUpdate(c *gin.Context) {
    // Run incremental update in background
    go func() {
        loader := loader.NewIncrementalTracker(db, sourceDir, false)
        files, _ := loader.GetFilesToProcess(ctx)
        if len(files) > 0 {
            bulkLoader := loader.NewBulkLoader(db, 20, 500, 1000)
            bulkLoader.LoadFiles(ctx, files)
            loader.MarkProcessingComplete(ctx, files)
        }
    }()
    
    c.JSON(200, gin.H{
        "status": "update_triggered",
        "message": "CVE update started in background",
    })
}
```

**Endpoint**: `POST /api/v1/cve/update`

**Usage**:
```bash
curl -X POST http://localhost:8080/api/v1/cve/update \
  -H "Authorization: Bearer $TOKEN"
```

**Benefits**:
- ✅ Can be triggered from UI
- ✅ Can be called from webhooks
- ✅ No CLI access needed
- ✅ Fast response (~4-9 seconds)

---

### Option 2: Webhook Support

**Implementation**:
```go
// Webhook endpoint for OSV.dev notifications
func HandleOSVWebhook(c *gin.Context) {
    var payload struct {
        CVEID string `json:"cve_id"`
        Action string `json:"action"` // "created", "updated"
    }
    
    // Process specific CVE file
    filePath := fmt.Sprintf("/cve-data/all/%s.json", payload.CVEID)
    // ... process file
}
```

**Benefits**:
- ✅ Real-time updates (immediate)
- ✅ Only processes changed CVEs
- ✅ No polling needed

---

### Option 3: File Watcher (Advanced)

**Implementation**:
```go
// Watch /cve-data/all directory for changes
watcher, _ := fsnotify.NewWatcher()
watcher.Add("/cve-data/all")

for event := range watcher.Events {
    if event.Op&fsnotify.Write == fsnotify.Write {
        // File modified, process immediately
        processFile(event.Name)
    }
}
```

**Benefits**:
- ✅ Real-time (immediate detection)
- ✅ Automatic (no manual trigger)
- ⚠️ More complex implementation

---

### Option 4: Scheduled More Frequently

**Current**: Daily at 2 AM

**Option**: Every 6 hours
```cron
0 */6 * * * /path/to/bin/cve-loader-optimized --mode incremental
```

**Benefits**:
- ✅ Faster updates (max 6 hour delay)
- ✅ Simple (just change cron)
- ⚠️ More frequent runs

---

## Performance Comparison

### Update Scenarios

| Scenario | Current Method | Time | Delay |
|----------|---------------|------|-------|
| **1 new CVE** | Cron (daily) | 4-9 sec | Up to 24h |
| **1 new CVE** | Manual CLI | 4-9 sec | Immediate |
| **10 new CVEs** | Cron (daily) | 4-9 sec | Up to 24h |
| **10 new CVEs** | Manual CLI | 4-9 sec | Immediate |
| **100 new CVEs** | Cron (daily) | 5-10 sec | Up to 24h |
| **100 new CVEs** | Manual CLI | 5-10 sec | Immediate |
| **Full reload** | Force mode | 6-10 min | Immediate |

**Key Insight**: Incremental updates are **very fast** (4-9 seconds) regardless of number of new files!

---

## Implementation Plan: API Endpoint

### Step 1: Create API Handler

**File**: `KSAM/core/internal/api/cve/cve_handlers.go` (new)

```go
package cve

import (
    "context"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/ksam/core/pkg/cve/loader"
    "gorm.io/gorm"
)

// TriggerUpdate triggers an incremental CVE update
func TriggerUpdate(c *gin.Context) {
    db := c.MustGet("db").(*gorm.DB)
    sourceDir := "/cve-data/all" // From config
    
    // Run in background goroutine
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
        defer cancel()
        
        tracker := loader.NewIncrementalTracker(db, sourceDir, false)
        files, err := tracker.GetFilesToProcess(ctx)
        if err != nil {
            log.Printf("Failed to get files to process: %v", err)
            return
        }
        
        if len(files) == 0 {
            log.Printf("No files need processing")
            return
        }
        
        bulkLoader := loader.NewBulkLoader(db, 20, 500, 1000)
        if err := bulkLoader.LoadFiles(ctx, files); err != nil {
            log.Printf("Bulk load failed: %v", err)
            return
        }
        
        tracker.MarkProcessingComplete(ctx, files)
        log.Printf("CVE update completed: %d files processed", len(files))
    }()
    
    c.JSON(http.StatusAccepted, gin.H{
        "status": "accepted",
        "message": "CVE update started in background",
    })
}

// GetUpdateStatus returns current update status
func GetUpdateStatus(c *gin.Context) {
    db := c.MustGet("db").(*gorm.DB)
    sourceDir := "/cve-data/all"
    
    tracker := loader.NewIncrementalTracker(db, sourceDir, false)
    stats, err := tracker.GetStats(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "total_files": stats.TotalFiles,
        "success_count": stats.SuccessCount,
        "failed_count": stats.FailedCount,
        "pending_count": stats.PendingCount,
        "last_processed_at": stats.LastProcessedAt,
    })
}
```

### Step 2: Add Routes

**File**: `KSAM/core/internal/api/routes.go`

```go
// Add CVE routes
cveGroup := v1.Group("/cve")
{
    cveGroup.POST("/update", cve.TriggerUpdate)
    cveGroup.GET("/status", cve.GetUpdateStatus)
}
```

### Step 3: Usage

**Trigger Update**:
```bash
curl -X POST http://localhost:8080/api/v1/cve/update \
  -H "Authorization: Bearer $TOKEN"
```

**Check Status**:
```bash
curl http://localhost:8080/api/v1/cve/status \
  -H "Authorization: Bearer $TOKEN"
```

---

## Quick Reference: Update Commands

### Manual Update (CLI)

```bash
# Incremental update (recommended)
./bin/cve-loader-optimized \
    --source /cve-data/all \
    --mode incremental \
    --workers 20

# With hash checking (more accurate, slower)
./bin/cve-loader-optimized \
    --mode incremental \
    --compute-hash

# Force full reload (if needed)
./bin/cve-loader-optimized \
    --mode force \
    --workers 50 \
    --batch-size 1000
```

### Automated Update (Cron)

```cron
# Daily at 2 AM
0 2 * * * /path/to/bin/cve-loader-optimized --mode incremental

# Every 6 hours
0 */6 * * * /path/to/bin/cve-loader-optimized --mode incremental

# Every hour (for critical environments)
0 * * * * /path/to/bin/cve-loader-optimized --mode incremental
```

---

## Summary

### Current State

| Mechanism | Status | Speed | Delay |
|-----------|--------|-------|-------|
| **Manual CLI** | ✅ Available | 4-9 sec | Immediate |
| **Cron Job** | ✅ Available | 4-9 sec | Up to 24h |
| **API Endpoint** | ❌ Not Implemented | N/A | N/A |
| **Webhook** | ❌ Not Implemented | N/A | N/A |
| **File Watcher** | ❌ Not Implemented | N/A | N/A |

### Recommendations

**For Fast Updates**:
1. ✅ **Use Manual CLI** for immediate updates (4-9 seconds)
2. ⚠️ **Implement API Endpoint** for UI/webhook integration
3. ⚠️ **Increase Cron Frequency** to every 6 hours (if needed)

**For Real-time Updates**:
1. ⚠️ **Implement Webhook** for OSV.dev notifications
2. ⚠️ **Implement File Watcher** for automatic detection

---

**Last Updated**: 2024-12-20  
**Status**: ✅ Incremental Updates Work, ⚠️ API Integration Needed


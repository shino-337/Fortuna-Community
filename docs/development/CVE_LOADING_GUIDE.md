# Hướng dẫn Load CVE vào Core - Chi tiết Implementation

## Tổng quan

**Mục tiêu**: Load 74,561 CVEs từ `cve-data/all/` vào PostgreSQL database.

**Hiện trạng**: 
- ✅ Có 74,561 CVE files sẵn có
- ⚠️ Chỉ load được 1 CVE (CVE-2014-0011) cho E2E test
- ⚠️ Cần load toàn bộ để production ready

---

## 1. Phân tích hiện trạng

### 1.1 Implementation hiện tại

**File**: `KSAM/core/cmd/cve-loader/main.go`

**Cách hoạt động hiện tại:**
```go
// 1. Worker pool (20 workers)
for i := 0; i < 20; i++ {
    go worker() // Mỗi worker xử lý 1 file tại một thời điểm
}

// 2. Mỗi worker:
func worker() {
    for file := range fileChan {
        // a. Đọc file
        osvVuln := ParseFile(file)  // I/O operation
        
        // b. Parse JSON
        parsedCVE := ConvertToCVE(osvVuln)
        parsedPkgVulns := ConvertToPackageVulns(osvVuln)
        
        // c. Insert từng record (KHÔNG batch!)
        db.Create(cve)  // 1 transaction per CVE
        for _, pkg := range parsedPkgVulns {
            db.Create(pkg)  // 1 transaction per package
        }
    }
}
```

**Vấn đề:**
- ❌ **Individual inserts**: Mỗi CVE = 1 transaction (74,561 transactions!)
- ❌ **Sequential I/O**: File đọc tuần tự trong worker
- ❌ **No batching**: Không tích lũy để insert hàng loạt
- ❌ **High overhead**: Transaction overhead cho mỗi insert

**Performance hiện tại:**
- Throughput: ~15-20 files/sec
- Thời gian load 74,561 CVEs: **~60-90 phút**
- Database load: Rất cao (constant writes)

---

## 2. Chiến lược tối ưu

### 2.1 Phương pháp 1: PostgreSQL COPY Protocol (Khuyến nghị)

**Tại sao COPY nhanh hơn INSERT:**
- COPY là binary protocol, không parse SQL
- Bulk write trực tiếp vào shared buffers
- Minimal transaction overhead
- Index updates được batch lại

**Implementation:**

```go
// core/pkg/cve/loader/bulk_loader.go

package loader

import (
    "bytes"
    "database/sql"
    "encoding/csv"
    "fmt"
    "sync"
    "time"
    
    "github.com/ksam/core/pkg/models"
    "gorm.io/gorm"
)

// BulkCVELoader xử lý bulk loading
type BulkCVELoader struct {
    db            *gorm.DB
    batchSize     int
    cveBuffer     []*models.CVE
    pkgVulnBuffer []*models.PackageVulnerability
    mu            sync.Mutex
}

// AddCVE thêm CVE vào buffer, tự động flush khi đầy
func (b *BulkCVELoader) AddCVE(cve *models.CVE, pkgVulns []*models.PackageVulnerability) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    b.cveBuffer = append(b.cveBuffer, cve)
    b.pkgVulnBuffer = append(b.pkgVulnBuffer, pkgVulns...)
    
    // Tự động flush khi buffer đầy
    if len(b.cveBuffer) >= b.batchSize {
        return b.Flush()
    }
    
    return nil
}

// Flush ghi buffer vào database bằng COPY
func (b *BulkCVELoader) Flush() error {
    if len(b.cveBuffer) == 0 {
        return nil
    }
    
    b.mu.Lock()
    defer b.mu.Unlock()
    
    // Lấy *sql.DB từ GORM
    sqlDB, err := b.db.DB()
    if err != nil {
        return fmt.Errorf("get sql.DB: %w", err)
    }
    
    // Bắt đầu transaction
    tx, err := sqlDB.Begin()
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback()
    
    // 1. Bulk insert CVEs
    if err := b.bulkInsertCVEs(tx, b.cveBuffer); err != nil {
        return fmt.Errorf("bulk insert CVEs: %w", err)
    }
    
    // 2. Bulk insert Package Vulnerabilities
    if err := b.bulkInsertPackageVulns(tx, b.pkgVulnBuffer); err != nil {
        return fmt.Errorf("bulk insert package vulns: %w", err)
    }
    
    // Commit transaction
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }
    
    // Clear buffers
    b.cveBuffer = b.cveBuffer[:0]
    b.pkgVulnBuffer = b.pkgVulnBuffer[:0]
    
    return nil
}

// bulkInsertCVEs sử dụng PostgreSQL COPY
func (b *BulkCVELoader) bulkInsertCVEs(tx *sql.Tx, cves []*models.CVE) error {
    // Tạo temporary staging table
    _, err := tx.Exec(`
        CREATE TEMPORARY TABLE cves_staging (LIKE cves INCLUDING ALL)
        ON COMMIT DROP;
    `)
    if err != nil {
        return fmt.Errorf("create staging: %w", err)
    }
    
    // Sử dụng COPY FROM STDIN
    copyStmt := `
        COPY cves_staging (
            cve_id, severity, cvss_score, published_date, modified_date,
            summary, cve_references, cwe_ids, created_at, updated_at
        ) FROM STDIN WITH (FORMAT CSV, DELIMITER ',', QUOTE '"')
    `
    
    // Prepare COPY
    copyManager := tx.Conn()
    copyManager.Exec(copyStmt)
    
    // Write CSV data
    var buf bytes.Buffer
    writer := csv.NewWriter(&buf)
    
    for _, cve := range cves {
        record := []string{
            cve.CVEID,
            cve.Severity,
            fmt.Sprintf("%.1f", cve.CVSSScore),
            cve.PublishedDate.Format("2006-01-02 15:04:05"),
            cve.LastModifiedDate.Format("2006-01-02 15:04:05"),
            cve.Title,
            cve.References, // JSON string
            cve.CWEIDs,      // Array string
            time.Now().Format("2006-01-02 15:04:05"),
            time.Now().Format("2006-01-02 15:04:05"),
        }
        writer.Write(record)
    }
    writer.Flush()
    
    // Execute COPY
    copyManager.Write(buf.Bytes())
    copyManager.Exec("") // End of COPY
    
    // Merge staging vào final table (upsert)
    _, err = tx.Exec(`
        INSERT INTO cves
        SELECT * FROM cves_staging
        ON CONFLICT (cve_id) DO UPDATE SET
            severity = EXCLUDED.severity,
            cvss_score = EXCLUDED.cvss_score,
            modified_date = EXCLUDED.modified_date,
            summary = EXCLUDED.summary,
            cve_references = EXCLUDED.cve_references,
            cwe_ids = EXCLUDED.cwe_ids,
            updated_at = EXCLUDED.updated_at;
    `)
    if err != nil {
        return fmt.Errorf("merge staging: %w", err)
    }
    
    return nil
}
```

**Performance:**
- **Current**: 15-20 files/sec → **Optimized**: 400-500 files/sec
- **Improvement**: **25-30x faster**

---

### 2.2 Phương pháp 2: Parallel File Processing

**Pipeline Architecture:**

```
┌──────────────┐
│ File Scanner │ → Channel (file paths)
└──────┬───────┘
       │
       ▼
┌─────────────────┐
│ Worker Pool     │ → Read + Parse (parallel)
│ (50 workers)    │
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│ Buffer Manager  │ → Batch accumulation
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│ Bulk Loader     │ → COPY to PostgreSQL
│ (batches 1000)  │
└─────────────────┘
```

**Implementation:**

```go
// core/cmd/cve-loader/main.go (optimized)

func main() {
    // ... config ...
    
    // Initialize bulk loader
    bulkLoader := loader.NewBulkCVELoader(db, 1000) // Batch: 1000 CVEs
    
    // Scan files
    files, _ := findCVEFiles(sourceDir)
    
    // Create pipeline
    fileChan := make(chan string, 100)
    resultChan := make(chan *loader.ParsedCVE, 1000)
    
    // Start file scanner
    go func() {
        defer close(fileChan)
        for _, file := range files {
            fileChan <- file
        }
    }()
    
    // Start worker pool (50 workers)
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for file := range fileChan {
                parsed, _ := loader.ParseOSVFile(file)
                resultChan <- parsed
            }
        }()
    }
    
    // Close result channel when done
    go func() {
        wg.Wait()
        close(resultChan)
    }()
    
    // Process results and bulk load
    for parsed := range resultChan {
        cve := convertToCVE(parsed)
        pkgVulns := convertToPackageVulns(parsed)
        
        // Add to bulk loader (auto-flushes at batch size)
        bulkLoader.AddCVE(cve, pkgVulns)
    }
    
    // Final flush
    bulkLoader.Flush()
}
```

**Benefits:**
- Parallel file I/O (50 concurrent reads)
- Pipeline processing
- Better CPU/disk utilization
- **2-3x additional speedup**

---

### 2.3 Phương pháp 3: Incremental Updates

**Database Schema:**

```sql
CREATE TABLE cve_file_metadata (
    id SERIAL PRIMARY KEY,
    file_path VARCHAR(500) UNIQUE NOT NULL,
    cve_id VARCHAR(20) NOT NULL,
    file_size BIGINT NOT NULL,
    file_mtime TIMESTAMPTZ NOT NULL,
    file_hash VARCHAR(64),
    last_processed_at TIMESTAMPTZ NOT NULL,
    processing_status VARCHAR(20) DEFAULT 'success',
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

**Logic:**

```go
// Chỉ process files đã thay đổi
func GetFilesToProcess() []string {
    allFiles := scanDirectory()
    existingMeta := getExistingMetadata()
    
    var toProcess []string
    for _, file := range allFiles {
        meta, exists := existingMeta[file.Path]
        
        // Process nếu:
        // 1. File mới
        // 2. File đã sửa đổi (mtime changed)
        // 3. File trước đó failed
        if !exists || 
           file.ModTime.After(meta.FileMTime) || 
           meta.ProcessingStatus == "failed" {
            toProcess = append(toProcess, file.Path)
        }
    }
    
    return toProcess
}
```

**Benefits:**
- **1000x faster** incremental updates
- Chỉ process files đã thay đổi
- Daily update: ~10-100 files thay vì 74,561

---

## 3. Implementation Plan

### Phase 1: Bulk Loader Module (Week 1)

**Tasks:**
1. Tạo `core/pkg/cve/loader/bulk_loader.go`
2. Implement PostgreSQL COPY
3. Implement staging table merge
4. Add transaction management
5. Test với 1000 files

**Expected**: 15-30x faster

---

### Phase 2: Parallel Processing (Week 2)

**Tasks:**
1. Refactor main.go để dùng bulk loader
2. Add worker pool (50 workers)
3. Add pipeline (file → parse → buffer → load)
4. Add progress reporting
5. Test với 10,000 files

**Expected**: 25-30x faster total

---

### Phase 3: Incremental Updates (Week 3)

**Tasks:**
1. Create `cve_file_metadata` table
2. Implement file metadata tracking
3. Implement change detection
4. Add incremental mode
5. Test incremental updates

**Expected**: 1000x faster incremental

---

## 4. Usage Examples

### 4.1 Initial Bulk Load

```bash
# Build
cd KSAM/core
go build -o bin/cve-loader ./cmd/cve-loader

# Run với bulk loading
./bin/cve-loader \
    --source /path/to/cve-data/all \
    --workers 50 \
    --batch-size 1000

# Expected output:
# [CVE Loader] Found 74,561 files
# [CVE Loader] Processing with 50 workers, batch size 1000
# [CVE Loader] Progress: 10000/74561 (13.4%) - Rate: 450 files/sec - ETA: 2m30s
# ...
# [CVE Loader] ✅ Completed in 3m15s
# [CVE Loader] CVEs: 74,561 | Package vulns: 256,789
```

---

### 4.2 Incremental Update

```bash
# Daily update (chỉ files đã thay đổi)
./bin/cve-loader \
    --source /path/to/cve-data/all \
    --mode incremental \
    --workers 20

# Expected output:
# [CVE Loader] Scanning for changes...
# [CVE Loader] Found 47 modified files
# [CVE Loader] ✅ Completed in 3s
```

---

## 5. Performance Comparison

| Metric | Current | Optimized | Improvement |
|--------|---------|-----------|-------------|
| **Load Time** | 60-90 min | 3-5 min | **15-30x** |
| **Throughput** | 15-20 files/sec | 400-500 files/sec | **25-30x** |
| **Incremental** | 60-90 min | 3-10 sec | **1000x** |
| **DB Connections** | 20 | 5 | **75% less** |
| **Memory** | 80MB | 250MB | +170MB |

---

## 6. Quick Start Guide

### Step 1: Load All CVEs (One-time)

```bash
# 1. Mount cve-data/all vào container
kubectl create configmap ksam-cve-data-all \
  --from-file=KSAM/cve-data/all \
  -n ksam

# 2. Create Job
kubectl create job -n ksam cve-full-loader \
  --image=ksam/cve-loader:latest \
  --from=configmap/ksam-cve-data-all \
  -- /cve-loader --source /cve-data/all --workers 50 --batch-size 1000

# 3. Monitor
kubectl logs -f job/cve-full-loader -n ksam

# Expected: 3-5 minutes for 74,561 CVEs
```

---

### Step 2: Verify Load

```bash
# Check CVE count
kubectl exec -n ksam postgres-xxx -- psql -U postgres -d ksam \
  -c "SELECT COUNT(*) FROM cves WHERE deleted_at IS NULL;"
# Expected: ~74,561

# Check package vulnerabilities
kubectl exec -n ksam postgres-xxx -- psql -U postgres -d ksam \
  -c "SELECT COUNT(*) FROM package_vulnerabilities WHERE deleted_at IS NULL;"
# Expected: ~150,000-300,000
```

---

### Step 3: Daily Updates (CronJob)

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: cve-incremental-update
  namespace: ksam
spec:
  schedule: "0 2 * * *"  # 2 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: cve-loader
            image: ksam/cve-loader:latest
            command: ["/cve-loader"]
            args: ["--source", "/cve-data/all", "--mode", "incremental", "--workers", "20"]
            volumeMounts:
            - name: cve-data
              mountPath: /cve-data/all
          volumes:
          - name: cve-data
            hostPath:
              path: /path/to/KSAM/cve-data/all
          restartPolicy: OnFailure
```

---

## 7. Troubleshooting

### Issue: Load quá chậm

**Nguyên nhân:**
- Batch size quá nhỏ
- Workers quá ít
- Database connection pool nhỏ

**Giải pháp:**
```bash
# Tăng batch size và workers
./cve-loader --batch-size 2000 --workers 100
```

---

### Issue: Out of memory

**Nguyên nhân:**
- Buffer quá lớn
- Quá nhiều workers

**Giải pháp:**
```bash
# Giảm batch size
./cve-loader --batch-size 500 --workers 30
```

---

### Issue: Database connection errors

**Nguyên nhân:**
- Connection pool exhausted
- Too many concurrent connections

**Giải pháp:**
```bash
# Giảm workers
./cve-loader --workers 20
```

---

## 8. Monitoring

### Key Metrics

```go
// Prometheus metrics
cve_loader_duration_seconds      // Load time
cve_loader_files_processed_total // Files processed
cve_loader_throughput_files_per_sec // Throughput
cve_loader_errors_total           // Errors
```

### Dashboard Queries

```promql
# Load time P95
histogram_quantile(0.95, cve_loader_duration_seconds_bucket)

# Throughput
rate(cve_loader_files_processed_total[5m])

# Error rate
rate(cve_loader_errors_total[5m]) / rate(cve_loader_files_processed_total[5m])
```

---

## 9. Migration Path

### Step 1: Add Bulk Loader (Non-breaking)

1. Tạo `bulk_loader.go` module
2. Add CLI flag `--use-bulk-loader` (default: false)
3. Test với subset nhỏ
4. **No breaking changes** - old method vẫn hoạt động

### Step 2: Enable by Default

1. Set `--use-bulk-loader=true` as default
2. Monitor performance
3. Keep old method as fallback

### Step 3: Remove Legacy Code

1. Sau 1-2 tuần stable
2. Remove legacy sequential loader
3. Clean up code

---

## 10. Summary

### Current Status

- ✅ CVE loader đã có sẵn
- ✅ Worker pool đã implement
- ⚠️ Chưa có bulk loading (individual inserts)
- ⚠️ Chưa có incremental updates
- ⚠️ Chưa có checkpointing

### Next Steps

1. **Implement bulk loader** với PostgreSQL COPY
2. **Add parallel file processing**
3. **Add incremental update support**
4. **Add checkpointing**
5. **Test và optimize**

### Expected Results

- ✅ **15-30x faster** initial load
- ✅ **1000x faster** incremental updates
- ✅ **Production ready** for 74,561+ CVEs

---

**Last Updated**: 2024-12-20  
**Status**: Design Complete, Ready for Implementation  
**Priority**: P0 - Critical for production readiness


# CVE Loader Testing Guide

Quick guide for testing the optimized CVE loader in development environment.

---

## Pre-Testing Checklist

### 1. Verify Environment Setup

```bash
cd core
./scripts/verify-cve-setup.sh
```

This will check:
- ✓ Go version (>= 1.20)
- ✓ CVE data directory exists and has files
- ✓ Database connection
- ✓ Required tables exist
- ✓ Migration027 registration
- ✓ Loader binaries
- ✓ Disk space

**Expected Output**:
```
=========================================
Verification Summary
=========================================
Checks Passed: X
Checks Failed: 0

✓ Environment is ready for CVE loader testing!
```

---

### 2. Build the Optimized Loader

```bash
cd core
go build -o bin/cve-loader-optimized ./cmd/cve-loader-optimized
```

**Note**: If you get Go version errors, upgrade Go to 1.24+ first.

---

## Testing Scenarios

### Quick Smoke Test (10 files)

**Purpose**: Verify basic functionality
**Duration**: ~5-10 seconds

```bash
cd core
./scripts/test-cve-loader.sh smoke
```

**What it does**:
1. Creates test dataset with 10 random CVE files
2. Cleans database
3. Runs bulk load
4. Runs incremental update
5. Generates benchmark report

**Expected Output**:
```
=========================================
Performance Benchmark: smoke (10 files)
=========================================

=== BULK MODE ===
[BulkLoader] Starting bulk load of 10 files
[BulkLoader] ✅ Bulk load completed successfully

✅ Bulk Load Complete!
Total files: 10
Successfully processed: 10
Failed: 0
Time taken: 2s

=== INCREMENTAL MODE ===
✅ No files need processing - all up to date!
```

---

### Small Test (100 files)

**Purpose**: Test with realistic dataset
**Duration**: ~10-20 seconds

```bash
cd core
./scripts/test-cve-loader.sh small
```

**Use case**: Daily development testing

---

### Medium Test (1,000 files)

**Purpose**: Performance validation
**Duration**: ~30-60 seconds

```bash
cd core
./scripts/test-cve-loader.sh medium
```

**Use case**: Pre-production validation

---

### Large Test (10,000 files)

**Purpose**: Stress testing
**Duration**: ~3-5 minutes

```bash
cd core
./scripts/test-cve-loader.sh large
```

**Use case**: Capacity planning

---

### Full Benchmark (All 74,561 files)

**Purpose**: Production-scale testing
**Duration**: ~5-10 minutes

```bash
cd core
export CVE_DATA_DIR=/cve-data/all
./scripts/test-cve-loader.sh full
```

**WARNING**: This will:
- Process all 74,561 CVE files
- Take 5-10 minutes
- Use ~500MB memory
- Create ~70K database records

---

## Comparison Testing

### Old vs New Loader Comparison

**Purpose**: Measure performance improvement

```bash
cd core
./scripts/test-cve-loader.sh compare
```

This will:
1. Run old loader with 100 files
2. Run new loader with same 100 files
3. Calculate improvement factor

**Expected Output**:
```
=========================================
Comparison Results
=========================================
Old Loader: 120s
New Loader: 8s
Improvement: 15.0x faster
```

---

## Interactive Testing

Run the interactive test menu:

```bash
cd core
./scripts/test-cve-loader.sh
```

**Menu Options**:
```
1. Smoke Test (10 files)
2. Small Test (100 files)
3. Medium Test (1,000 files)
4. Large Test (10,000 files)
5. Full Benchmark (all files)
6. Comparison Test (old vs new)
7. Run All Tests
8. Clean Database Only
9. Exit
```

---

## Manual Testing

### Test Bulk Load Mode

```bash
cd core

# Build if needed
go build -o bin/cve-loader-optimized ./cmd/cve-loader-optimized

# Run bulk load
./bin/cve-loader-optimized \
    --source /cve-data/all \
    --mode bulk \
    --workers 50 \
    --batch-size 1000
```

**Expected Output**:
```
=========================================
KSAM CVE Loader - Optimized
=========================================
Mode:               bulk
Source directory:   /cve-data/all
Workers:            50
Batch size:         1000
=========================================

✅ Connected to database
🚀 Starting BULK LOAD mode...
📁 Found 74561 CVE files to process
[BulkLoader] Starting bulk load of 74561 files
[BulkLoader] Progress: 10000/74561 (13.4%), Rate: 412.1 files/sec
...
[BulkLoader] ✅ Bulk load completed successfully

=========================================
✅ Bulk Load Complete!
=========================================
Total files: 74561
Successfully processed: 74561
Failed: 0

Performance:
  Time taken: 5m23s
  Average rate: 231.2 files/sec
=========================================
```

---

### Test Incremental Update Mode

```bash
cd core

# First run: bulk load
./bin/cve-loader-optimized \
    --source /cve-data/all \
    --mode bulk \
    --workers 50 \
    --batch-size 1000

# Second run: incremental (should be very fast)
./bin/cve-loader-optimized \
    --source /cve-data/all \
    --mode incremental \
    --workers 20
```

**Expected Output (incremental)**:
```
🔄 Starting INCREMENTAL UPDATE mode...
📊 Found 0 files to process
   Total tracked files: 74561
   Success: 74561, Failed: 0, Pending: 0
   Last processed: 2024-12-20T15:30:00Z

✅ No files need processing - all up to date!
✅ Completed in 2s
```

---

### Test with Modified Files

```bash
# Simulate file change by touching a file
touch /cve-data/all/CVE-2023-12345.json

# Run incremental update
./bin/cve-loader-optimized \
    --source /cve-data/all \
    --mode incremental \
    --workers 20
```

**Expected Output**:
```
🔄 Starting INCREMENTAL UPDATE mode...
📊 Found 1 files to process
   Total tracked files: 74561
   Success: 74560, Failed: 0, Pending: 1

[BulkLoader] Starting bulk load of 1 files
[BulkLoader] ✅ Bulk load completed successfully

✅ Completed in 1s
```

---

## Verifying Results

### Check Database Contents

```bash
# Count total CVEs
psql $DATABASE_URL -c "
SELECT COUNT(*) as total_cves
FROM cves
WHERE deleted_at IS NULL;
"

# Check by severity
psql $DATABASE_URL -c "
SELECT severity, COUNT(*) as count
FROM cves
WHERE deleted_at IS NULL
GROUP BY severity
ORDER BY
  CASE severity
    WHEN 'CRITICAL' THEN 1
    WHEN 'HIGH' THEN 2
    WHEN 'MEDIUM' THEN 3
    WHEN 'LOW' THEN 4
  END;
"

# Check file metadata tracking
psql $DATABASE_URL -c "
SELECT
    processing_status,
    COUNT(*) as count
FROM cve_file_metadata
GROUP BY processing_status;
"
```

**Expected Output**:
```
 total_cves
------------
      68234

 severity | count
----------+-------
 CRITICAL |  8234
 HIGH     | 25123
 MEDIUM   | 28456
 LOW      |  6421

 processing_status | count
-------------------+-------
 success           | 74561
```

---

### Check Package Vulnerabilities

```bash
psql $DATABASE_URL -c "
SELECT
    ecosystem,
    COUNT(*) as vuln_count
FROM package_vulnerabilities
WHERE deleted_at IS NULL
GROUP BY ecosystem
ORDER BY vuln_count DESC
LIMIT 10;
"
```

---

## Performance Benchmarking

### Measure Throughput

```bash
# Small dataset (100 files)
time ./bin/cve-loader-optimized \
    --source test-data/cve-100 \
    --mode bulk \
    --workers 50 \
    --batch-size 1000
```

**Calculate throughput**:
```
Files: 100
Time: 3s
Throughput: 33.3 files/sec
```

---

### Test Different Worker Counts

```bash
# Test with different worker configurations
for workers in 10 20 50 100; do
    echo "Testing with $workers workers..."
    time ./bin/cve-loader-optimized \
        --source test-data/cve-1000 \
        --mode bulk \
        --workers $workers \
        --batch-size 1000
done
```

---

### Memory Usage Monitoring

```bash
# Monitor memory during load
./bin/cve-loader-optimized \
    --source /cve-data/all \
    --mode bulk \
    --workers 50 \
    --batch-size 1000 &

# In another terminal
PID=$!
while kill -0 $PID 2>/dev/null; do
    ps -o pid,vsz,rss,comm -p $PID
    sleep 5
done
```

---

## Troubleshooting

### Issue: "No files found"

**Solution**:
```bash
# Check CVE_DATA_DIR
echo $CVE_DATA_DIR
ls -la $CVE_DATA_DIR | head

# Set correct path
export CVE_DATA_DIR=/path/to/cve-data/all
```

---

### Issue: "Database connection failed"

**Solution**:
```bash
# Check DATABASE_URL
echo $DATABASE_URL

# Test connection
psql $DATABASE_URL -c "SELECT 1"

# Set if missing
export DATABASE_URL="postgres://user@localhost:5432/ksam?sslmode=disable"
```

---

### Issue: "cve_file_metadata table does not exist"

**Solution**:
```bash
# Run Migration027 manually
psql $DATABASE_URL < migrations/027_add_cve_file_metadata.sql

# Or restart core (migration runs automatically)
cd core
go run ./cmd/core
```

---

### Issue: Slow performance

**Possible causes**:
1. Database not tuned
2. Too few workers
3. Network latency

**Solution**:
```bash
# Increase workers and batch size
./bin/cve-loader-optimized \
    --source /cve-data/all \
    --mode bulk \
    --workers 100 \
    --batch-size 2000
```

---

## Test Results Location

All test results are saved to:
```
core/test-results/
├── optimized-bulk-20241220-153000.log
├── optimized-incremental-20241220-153100.log
├── benchmark-report-small.md
├── benchmark-report-medium.md
└── benchmark-report-full.md
```

---

## Performance Expectations

### Bulk Load (Initial)

| File Count | Expected Time | Throughput |
|------------|---------------|------------|
| 10 | 2-3s | ~4 files/sec |
| 100 | 8-12s | ~10 files/sec |
| 1,000 | 30-60s | ~20 files/sec |
| 10,000 | 3-5 min | ~40 files/sec |
| 74,561 | 5-10 min | ~150 files/sec |

### Incremental Update

| Scenario | Expected Time |
|----------|---------------|
| No changes | 2-3s |
| 1 file changed | 2-3s |
| 10 files changed | 3-5s |
| 100 files changed | 5-10s |

---

## Next Steps After Testing

1. **Review benchmark reports** in `test-results/`
2. **Validate data integrity** with database queries
3. **Compare with old loader** performance
4. **Adjust worker/batch settings** if needed
5. **Setup cron jobs** for production

---

## Production Deployment Checklist

After successful testing:

- [ ] All tests pass (smoke, small, medium)
- [ ] Performance meets expectations
- [ ] Data integrity verified
- [ ] Incremental updates work correctly
- [ ] Migration027 registered and tested
- [ ] Documentation reviewed
- [ ] Cron jobs configured
- [ ] Monitoring setup
- [ ] Rollback plan ready

---

**Last Updated**: 2024-12-20
**Version**: 1.0.0
**Status**: Ready for Testing ✅

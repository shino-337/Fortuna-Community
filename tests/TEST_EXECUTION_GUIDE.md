# Test Execution Guide

## Post-Optimization Test Suite

This guide explains how to execute the complete test suite to verify all optimizations and measure performance improvements.

---

## Prerequisites

### Required Tools
```bash
- kubectl (Kubernetes CLI)
- curl (HTTP client)
- jq (JSON processor)
- psql (PostgreSQL client)
- bc (Calculator for bash)
```

### Installation (macOS/Linux)
```bash
# macOS
brew install kubectl curl jq postgresql bc

# Ubuntu/Debian
sudo apt-get install kubectl curl jq postgresql-client bc

# RHEL/CentOS
sudo yum install kubectl curl jq postgresql bc
```

### Environment Variables
```bash
# Export these before running tests
export TEST_NAMESPACE="ksam-test"
export CORE_API_URL="http://localhost:8080"
export DB_HOST="localhost"
export DB_PORT="5432"
export DB_NAME="ksam"
export DB_USER="ksam"
export DB_PASSWORD="your-password"
```

### Verify Prerequisites
```bash
# Run this script to verify all tools are installed
cat > verify-prereqs.sh <<'EOF'
#!/bin/bash
echo "Checking prerequisites..."
command -v kubectl >/dev/null 2>&1 || { echo "❌ kubectl not found"; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "❌ curl not found"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "❌ jq not found"; exit 1; }
command -v psql >/dev/null 2>&1 || { echo "❌ psql not found"; exit 1; }
command -v bc >/dev/null 2>&1 || { echo "❌ bc not found"; exit 1; }
echo "✅ All prerequisites installed"
EOF
chmod +x verify-prereqs.sh
./verify-prereqs.sh
```

---

## Test Suite Overview

### 1. Optimization Verification Tests
**Purpose**: Verify all optimization changes are correctly implemented
**Location**: `tests/e2e/scripts/verify-optimizations.sh`
**Duration**: ~2-3 minutes

**Tests Performed**:
- Schema consistency (package_name vs component_id)
- Database indexes exist and are used
- No duplicate data
- Batch processing configured
- Prometheus metrics available
- Query performance sampling

### 2. Performance Impact Tests
**Purpose**: Measure actual performance improvements
**Location**: `tests/performance/scripts/measure-optimization-impact.sh`
**Duration**: ~3-5 minutes

**Benchmarks**:
- CVE lookup performance (bulk vs individual)
- Insight query performance
- Batch processing efficiency
- Connection pool utilization
- System-wide performance metrics

### 3. Complete E2E Flow Test
**Purpose**: Verify end-to-end functionality
**Location**: `tests/e2e/scenarios/pod-to-insight-flow.sh`
**Duration**: ~5-10 minutes

**Workflow**:
- Create test pod
- Wait for SBOM extraction
- Wait for CVE matching
- Wait for insight generation
- Verify API responses
- Compare API with database

---

## Quick Start

### Run All Tests (Recommended)
```bash
cd tests

# 1. Verify optimizations are implemented
./e2e/scripts/verify-optimizations.sh

# 2. Measure performance impact
./performance/scripts/measure-optimization-impact.sh

# 3. Run complete E2E test
./e2e/scenarios/pod-to-insight-flow.sh
```

### View Results
```bash
# E2E results
ls -lh e2e/results/

# Performance results
ls -lh performance/results/

# View latest results
cat e2e/results/optimization_verification_*.txt | tail -50
cat performance/results/optimization_impact_*.txt | tail -50
```

---

## Detailed Test Execution

### Test 1: Optimization Verification

**What it does**: Verifies all optimization changes are properly implemented in the database and code.

**Execute**:
```bash
cd tests/e2e/scripts
./verify-optimizations.sh
```

**Expected Output**:
```
==========================================
Optimization Verification Tests
==========================================
Date: 2025-12-24 14:30:15
==========================================

[Category 1] Schema Consistency Tests
----------------------------------------
[TEST] CVE Matches uses package_name (not component_id)
[PASS] ✅ CVE Matches uses package_name (not component_id)

[Category 2] Database Index Tests
----------------------------------------
[TEST] Index: idx_package_vulnerabilities_ecosystem_package
[PASS] ✅ Index: idx_package_vulnerabilities_ecosystem_package
...

==========================================
Test Summary
==========================================
Total Tests: 15
Passed: 15
Failed: 0
==========================================
```

**Success Criteria**:
- All 15 tests pass
- No schema drift
- All indexes created
- No duplicate data

---

### Test 2: Performance Impact Measurement

**What it does**: Measures actual performance improvements from optimizations.

**Execute**:
```bash
cd tests/performance/scripts
./measure-optimization-impact.sh
```

**Expected Output**:
```
==========================================
Optimization Impact Analysis
==========================================

[Benchmark 1] CVE Lookup Performance
----------------------------------------
Testing individual lookups (N queries)...
  ✓ Individual lookups (20 queries): 0.89s
Testing bulk lookup (1 query)...
  ✓ Bulk lookup (1 query): 0.07s
  → Speedup with bulk lookup: 12.71x faster

[Benchmark 2] Insight Query Performance
----------------------------------------
  ✓ Indexed query time: 0.23s
  → ✅ Meets performance target (<1.0s)

[Benchmark 3] Batch Processing Efficiency
----------------------------------------
  ✓ Matching duration: 1.8s
  → Projected time for 200 packages: 1.93s
  → ✅ Meets performance target (<5.0s for 200 packages)

[Benchmark 4] Connection Pool Utilization
----------------------------------------
  ✓ Connections open: 15
  ✓ Connections in use: 8
  ✓ Utilization: 53.3%
  → ✅ Healthy utilization (<80%)
```

**Success Criteria**:
- Bulk lookup speedup > 5x
- Insight query < 1s
- CVE matching < 5s (200 packages)
- Connection utilization < 80%

---

### Test 3: Complete E2E Flow

**What it does**: Verifies complete workflow from pod creation to insights in API.

**Execute**:
```bash
cd tests/e2e/scenarios
./pod-to-insight-flow.sh
```

**Expected Output**:
```
==========================================
E2E Test: Pod-to-Insight Flow
==========================================
Pod Name: test-pod-1735041015
Namespace: ksam-test
Image: nginx:latest
==========================================

Phase 1: Creating test pod...
✅ Pod created in 8.23 seconds

Phase 2: Waiting for SBOM extraction...
✅ SBOM extracted in 12.45 seconds
SBOM ID: 1234, Components: 187

Phase 3: Waiting for CVE matching...
✅ CVE matching completed in 2.1 seconds
CVE Matches: 34

Phase 4: Waiting for insight generation...
✅ Insight generation completed in 0.3 seconds
Insights: 12

Phase 5: Verifying API response...
✅ API insights verified

==========================================
Test Summary
==========================================
Pod Creation: 8.23 seconds
SBOM Extraction: 12.45 seconds
CVE Matching: 2.1 seconds
Insight Generation: 0.3 seconds
API Verification: 0.5 seconds
Total E2E Time: 23.58 seconds
==========================================
```

**Success Criteria**:
- All phases complete successfully
- CVE matching < 5s
- Insight generation < 2s
- Total E2E < 60s
- API data matches database

---

## Troubleshooting

### Test Failures

#### "Database connection failed"
```bash
# Check PostgreSQL is running
pg_isready -h localhost -p 5432

# Test credentials
PGPASSWORD=ksam psql -h localhost -U ksam -d ksam -c "SELECT 1;"
```

#### "kubectl command not found"
```bash
# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/
```

#### "Metrics endpoint not accessible"
```bash
# Check Core service is running
curl -s http://localhost:8080/health

# Check if running on different port
netstat -tuln | grep 8080
```

#### "No SBOM found within timeout"
```bash
# Check agent is running
kubectl get pods -n ksam-system -l app=ksam-agent

# Check agent logs
kubectl logs -n ksam-system -l app=ksam-agent --tail=50

# Verify agent can access pods
kubectl get pods -n ksam-test
```

#### "CVE matches not found"
```bash
# This may be normal if image has no vulnerabilities
# Check CVE database is populated
PGPASSWORD=ksam psql -h localhost -U ksam -d ksam -c \
  "SELECT COUNT(*) FROM cves;"

# Should return > 0
# If 0, run CVE database loader
```

---

## Interpreting Results

### Performance Metrics

#### Good Performance (✅)
```
- CVE Matching (200 pkg): 1.5 - 3.0s
- Insight Generation (100 CVEs): 0.1 - 0.5s
- Insight Query: 0.1 - 0.5s
- Connection Utilization: 30-70%
- Bulk Lookup Speedup: > 10x
```

#### Acceptable Performance (⚠️)
```
- CVE Matching (200 pkg): 3.0 - 5.0s
- Insight Generation (100 CVEs): 0.5 - 2.0s
- Insight Query: 0.5 - 1.0s
- Connection Utilization: 70-80%
- Bulk Lookup Speedup: 5-10x
```

#### Poor Performance (❌)
```
- CVE Matching (200 pkg): > 5.0s
- Insight Generation (100 CVEs): > 2.0s
- Insight Query: > 1.0s
- Connection Utilization: > 80%
- Bulk Lookup Speedup: < 5x
```

### What to Do If Performance Is Poor

1. **Check Indexes**:
   ```sql
   SELECT schemaname, tablename, indexname
   FROM pg_indexes
   WHERE schemaname = 'public'
   ORDER BY tablename, indexname;
   ```

2. **Analyze Query Plans**:
   ```sql
   EXPLAIN ANALYZE
   SELECT * FROM package_vulnerabilities
   WHERE ecosystem = 'debian' AND package_name IN ('openssl');
   ```

3. **Check Connection Pool**:
   ```bash
   curl -s http://localhost:8080/metrics | grep ksam_db_connections
   ```

4. **Review Logs**:
   ```bash
   # Core logs
   kubectl logs -n ksam-system -l app=ksam-core --tail=100

   # Worker logs
   kubectl logs -n ksam-system -l app=ksam-core | grep CVEMatcherWorker
   ```

---

## Continuous Testing

### Daily Testing
```bash
# Add to cron
0 2 * * * cd /path/to/ksam/tests && ./run-daily-tests.sh
```

### CI/CD Integration
```yaml
# GitLab CI example
test:optimization:
  stage: test
  script:
    - export DB_HOST=postgres
    - cd tests
    - ./e2e/scripts/verify-optimizations.sh
    - ./performance/scripts/measure-optimization-impact.sh
  artifacts:
    paths:
      - tests/e2e/results/
      - tests/performance/results/
```

---

## Result Analysis

### Compare Results Over Time
```bash
# Compare performance trends
cd tests/performance/results
for file in optimization_impact_*.txt; do
    echo "=== $file ==="
    grep "Bulk lookup" "$file"
    grep "Indexed query time" "$file"
    echo ""
done
```

### Generate Performance Report
```bash
# Create performance summary
cat > generate-report.sh <<'EOF'
#!/bin/bash
echo "Performance Trend Report"
echo "========================"
echo ""

cd tests/performance/results
for file in $(ls -t optimization_impact_*.txt | head -5); do
    date=$(echo $file | grep -oP '\d{8}_\d{6}')
    bulk=$(grep "Bulk lookup" $file | grep -oP '\d+\.\d+s' | head -1)
    query=$(grep "Indexed query time" $file | grep -oP '\d+\.\d+s')

    echo "Date: $date"
    echo "  Bulk Lookup: $bulk"
    echo "  Query Time: $query"
    echo ""
done
EOF
chmod +x generate-report.sh
./generate-report.sh
```

---

## Summary

### Test Execution Checklist

- [ ] Prerequisites installed
- [ ] Environment variables set
- [ ] Database accessible
- [ ] Core service running
- [ ] Agent service running
- [ ] Kubernetes cluster accessible

### Run Tests

- [ ] Optimization verification tests
- [ ] Performance impact tests
- [ ] Complete E2E flow test

### Verify Results

- [ ] All optimization tests pass
- [ ] Performance meets targets
- [ ] E2E flow completes successfully
- [ ] No errors in logs
- [ ] Results saved to files

### Next Steps

- [ ] Review test results
- [ ] Address any failures
- [ ] Monitor metrics in production
- [ ] Schedule regular testing

---

**For questions or issues, refer to the test scripts' inline documentation or check the logs in the results directories.**

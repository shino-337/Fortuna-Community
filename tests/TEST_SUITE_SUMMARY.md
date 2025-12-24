# KSAM Test Suite - Complete Summary

## 📋 Overview

Comprehensive test suite for verifying KSAM platform functionality from Pod creation to Insight generation and API availability.

## 📁 Structure

```
tests/
├── README.md                          # Main test suite documentation
├── QUICK_START.md                     # Quick start guide
├── TEST_SUITE_SUMMARY.md              # This file
│
├── e2e/                               # End-to-End Tests
│   ├── scenarios/
│   │   ├── pod-to-insight-flow.sh     # Complete E2E flow test
│   │   └── TEST_SCENARIOS.md          # Test scenario documentation
│   ├── scripts/
│   │   ├── create-test-pod.sh         # Create test pod utility
│   │   ├── verify-sbom.sh             # Verify SBOM extraction
│   │   ├── verify-cve-matches.sh      # Verify CVE matching
│   │   ├── verify-insights.sh         # Verify insight generation
│   │   └── compare-api-db.sh          # Compare API with database
│   └── results/                       # E2E test results
│       └── README.md                  # Results documentation
│
├── performance/                       # Performance Tests
│   ├── scripts/
│   │   ├── measure-sbom-processing.sh      # SBOM processing benchmark
│   │   ├── measure-cve-matching.sh        # CVE matching benchmark
│   │   └── measure-insight-generation.sh   # Insight generation benchmark
│   └── results/                       # Performance test results
│       └── README.md                  # Results documentation
│
└── verification/                      # Verification Checklists
    └── checklists/
        ├── e2e-verification-checklist.md   # Complete E2E checklist
        └── performance-checklist.md       # Performance verification
```

## 🎯 Test Categories

### 1. End-to-End Tests (`e2e/`)

**Purpose:** Verify complete flow from Pod creation to Insight generation

**Main Script:** `e2e/scenarios/pod-to-insight-flow.sh`

**Flow:**
1. Create test pod
2. Wait for SBOM extraction
3. Verify SBOM in database
4. Wait for CVE matching
5. Verify CVE matches in database
6. Wait for insight generation
7. Verify insights in database
8. Verify insights via API
9. Compare API response with database
10. Measure timing for each phase

**Utility Scripts:**
- `create-test-pod.sh` - Create test pod
- `verify-sbom.sh` - Verify SBOM extraction
- `verify-cve-matches.sh` - Verify CVE matching
- `verify-insights.sh` - Verify insight generation
- `compare-api-db.sh` - Compare API with database

**Test Scenarios:**
- Basic Pod-to-Insight Flow
- Multi-Container Pod
- High Vulnerability Image
- No Vulnerability Image
- Pod Deletion and Cleanup
- Rapid Pod Creation
- API Consistency
- Performance Benchmark

### 2. Performance Tests (`performance/`)

**Purpose:** Measure processing times and system performance

**Scripts:**
- `measure-sbom-processing.sh` - SBOM processing time
- `measure-cve-matching.sh` - CVE matching time
- `measure-insight-generation.sh` - Insight generation time

**Metrics:**
- Average/Min/Max processing times
- Processing rates (items/second)
- Component/match/insight counts
- Performance target verification

**Targets:**
- SBOM Processing: < 30 seconds
- CVE Matching: < 5 seconds for 200 packages
- Insight Generation: < 2 seconds for 100 CVEs
- Total E2E Time: < 60 seconds

### 3. Verification Checklists (`verification/`)

**Purpose:** Manual verification of architecture, schema, and integration

**Checklists:**
- `e2e-verification-checklist.md` - Complete E2E verification
- `performance-checklist.md` - Performance verification

**Coverage:**
- Pod creation and registration
- SBOM extraction and storage
- CVE matching and storage
- Insight generation and storage
- API endpoints and responses
- Database schema consistency
- Performance benchmarks
- Error handling

## 🚀 Quick Start

### Prerequisites

1. Kubernetes cluster accessible via `kubectl`
2. Core service running
3. Agent service running and connected
4. Database (PostgreSQL) accessible
5. NATS running
6. Tools: `kubectl`, `curl`, `jq`, `psql`, `bc`

### Environment Setup

```bash
export TEST_NAMESPACE="ksam-test"
export CORE_API_URL="http://localhost:8080"
export DB_HOST="localhost"
export DB_PORT="5432"
export DB_NAME="ksam"
export DB_USER="ksam"
export DB_PASSWORD="your-password"
```

### Run Complete E2E Test

```bash
cd tests/e2e/scenarios
./pod-to-insight-flow.sh
```

### Run Individual Tests

```bash
# Create test pod
cd tests/e2e/scripts
./create-test-pod.sh

# Verify components (use outputs from previous steps)
./verify-sbom.sh "<pod-uid>"
./verify-cve-matches.sh "<sbom-id>"
./verify-insights.sh "<pod-uid>"
./compare-api-db.sh "<pod-uid>"
```

### Run Performance Tests

```bash
cd tests/performance/scripts
./measure-sbom-processing.sh
./measure-cve-matching.sh "<sbom-id>"
./measure-insight-generation.sh "<pod-uid>"
```

## 📊 Test Results

### Result Files

**E2E Tests:**
- `e2e_YYYYMMDD_HHMMSS.log` - Complete execution log
- `summary_YYYYMMDD_HHMMSS.txt` - Test summary with timing
- `api_response_YYYYMMDD_HHMMSS.json` - API response snapshot

**Performance Tests:**
- `sbom_performance_YYYYMMDD_HHMMSS.txt` - SBOM processing results
- `cve_matching_performance_YYYYMMDD_HHMMSS.txt` - CVE matching results
- `insight_generation_performance_YYYYMMDD_HHMMSS.txt` - Insight generation results

### Result Locations

- E2E results: `tests/e2e/results/`
- Performance results: `tests/performance/results/`

## ✅ Verification Checklist

### Phase 1: Pod Creation & Registration
- [ ] Pod created successfully
- [ ] Pod reaches Running state
- [ ] Agent detects pod
- [ ] Agent registers with Core

### Phase 2: SBOM Extraction
- [ ] SBOM extraction triggered
- [ ] SBOM data is valid
- [ ] SBOM stored in database
- [ ] Components stored correctly

### Phase 3: CVE Matching
- [ ] CVE matching triggered
- [ ] CVE database lookup works
- [ ] CVE matches found (if vulnerabilities exist)
- [ ] CVE matches stored in database

### Phase 4: Insight Generation
- [ ] Insight generation triggered
- [ ] Risk evaluation works
- [ ] Insights created correctly
- [ ] Insights stored in database

### Phase 5: API Verification
- [ ] API endpoints accessible
- [ ] API returns insights
- [ ] API response format correct
- [ ] API data matches database

### Phase 6: Performance
- [ ] SBOM processing < 30s
- [ ] CVE matching < 5s (200 packages)
- [ ] Insight generation < 2s (100 CVEs)
- [ ] Total E2E < 60s

## 🔍 Test Coverage

### Functional Coverage
- ✅ Pod lifecycle management
- ✅ SBOM extraction and storage
- ✅ CVE matching and scoring
- ✅ Insight generation and persistence
- ✅ API endpoints and responses
- ✅ Database schema consistency
- ✅ Error handling and edge cases

### Performance Coverage
- ✅ SBOM processing time
- ✅ CVE matching time
- ✅ Insight generation time
- ✅ API response time
- ✅ Database query performance
- ✅ Batch operation efficiency

### Data Integrity Coverage
- ✅ API response vs database
- ✅ Field-by-field comparison
- ✅ Timestamp accuracy
- ✅ Count consistency
- ✅ No data loss

## 📝 Test Execution

### Manual Execution

1. Review test scenarios in `TEST_SCENARIOS.md`
2. Set environment variables
3. Run E2E test: `./pod-to-insight-flow.sh`
4. Review results in `results/` directory
5. Verify using checklists

### Automated Execution

```bash
# Run all E2E scenarios
for scenario in tests/e2e/scenarios/*.sh; do
    echo "Running: $scenario"
    "$scenario"
done

# Run all performance tests
for perf in tests/performance/scripts/*.sh; do
    echo "Running: $perf"
    "$perf"
done
```

## 🐛 Troubleshooting

### Common Issues

**Pod Not Created:**
- Check Kubernetes access
- Verify namespace exists
- Check resource limits

**SBOM Not Extracted:**
- Check Agent is running
- Verify Agent can access K8s API
- Check image accessibility

**CVE Matches Not Found:**
- May be normal (no vulnerabilities)
- Check CVE database is populated
- Verify CVE matching worker

**Insights Not Generated:**
- May be normal (no CVEs)
- Check RiskWorker is running
- Verify insight generation logic

**API Not Responding:**
- Check Core service is running
- Verify API endpoint
- Check database connection

## 📈 Performance Targets

| Metric | Target | Status |
|--------|--------|--------|
| SBOM Processing | < 30s | ⏳ To be measured |
| CVE Matching (200 packages) | < 5s | ⏳ To be measured |
| Insight Generation (100 CVEs) | < 2s | ⏳ To be measured |
| Total E2E Time | < 60s | ⏳ To be measured |
| API Response Time | < 100ms | ⏳ To be measured |

## 📚 Documentation

- **README.md** - Main test suite documentation
- **QUICK_START.md** - Quick start guide
- **TEST_SCENARIOS.md** - Detailed test scenarios
- **e2e-verification-checklist.md** - Complete E2E checklist
- **performance-checklist.md** - Performance verification

## 🎯 Next Steps

1. **Run Tests:** Execute E2E test suite
2. **Review Results:** Analyze test outputs
3. **Verify Performance:** Compare with targets
4. **Check Consistency:** Verify API vs database
5. **Document Issues:** Record any problems found
6. **Iterate:** Fix issues and re-test

## 📞 Support

For issues or questions:
1. Review test logs in `results/` directory
2. Check Core and Agent logs
3. Verify environment configuration
4. Consult verification checklists

---

**Last Updated:** $(date)
**Test Suite Version:** 1.0.0


# KSAM Test Suite

Comprehensive test suite for verifying KSAM (Kubernetes Service Account Management) platform functionality.

## 📁 Structure

```
tests/
├── e2e/                    # End-to-end test scenarios
│   ├── scenarios/         # Complete E2E test scenarios
│   ├── scripts/           # Reusable test scripts
│   └── results/           # Test execution results
├── performance/           # Performance benchmarks
│   ├── scripts/           # Performance measurement scripts
│   └── results/           # Performance test results
└── verification/          # Verification checklists and reports
    ├── checklists/       # Verification checklists
    └── reports/          # Verification reports
```

## 🎯 Test Categories

### 1. End-to-End Tests (`e2e/`)

Complete flow verification from Pod creation to Insight generation:

- **Pod Creation** → **SBOM Extraction** → **CVE Matching** → **Insight Generation** → **API Response**

**Key Scenarios:**
- `pod-to-insight-flow.sh` - Complete E2E flow
- `sbom-extraction.sh` - SBOM extraction verification
- `cve-matching.sh` - CVE matching verification
- `insight-generation.sh` - Insight generation verification

### 2. Performance Tests (`performance/`)

Measure processing times and system performance:

- SBOM processing time
- CVE matching time
- Insight generation time
- API response time
- Database query performance

### 3. Verification (`verification/`)

Architecture, schema, and integration verification:

- Architecture compliance
- Database schema consistency
- Integration points verification

## 🚀 Quick Start

### Run Complete E2E Test

```bash
cd tests/e2e/scenarios
./pod-to-insight-flow.sh
```

### Run Performance Benchmarks

```bash
cd tests/performance/scripts
./measure-sbom-processing.sh
```

### Run Verification Checklist

```bash
cd tests/verification/checklists
# Review and check off items in architecture-verification.md
```

## 📊 Test Results

All test results are stored in respective `results/` folders with timestamps and detailed logs.

## 🔍 Test Coverage

- ✅ Pod lifecycle management
- ✅ SBOM extraction and storage
- ✅ CVE matching and scoring
- ✅ Insight generation and persistence
- ✅ API endpoints and responses
- ✅ Database schema consistency
- ✅ Performance benchmarks
- ✅ Error handling and edge cases


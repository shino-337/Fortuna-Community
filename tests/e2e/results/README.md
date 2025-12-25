# E2E Test Results

This directory contains results from E2E test executions.

---

## Latest Test Execution

**Date**: 2025-12-25  
**Status**: Analysis Complete  
**Tests Run**: 2  
**Tests Passed**: 0  
**Tests Failed**: 2

---

## Test Reports

### Summary Reports
- `E2E_TEST_SUMMARY_*.md` - High-level test execution summary

### Detailed Reports  
- `E2E_TEST_REPORT_*.md` - Detailed test execution reports with root cause analysis

### Execution Logs
- `e2e_execution_*.log` - Complete test execution logs
- `e2e_*.log` - Test phase logs

---

## Test Results Overview

### Test 1: Basic Pod-to-Insight Flow (test-pod-1766651359)
- **Status**: ❌ FAILED
- **Duration**: >300s (timeout)
- **Issue**: SBOM not found - Agent did not process pod

### Test 2: Basic Pod-to-Insight Flow (test-pod-1766651747)
- **Status**: ❌ FAILED  
- **Duration**: >300s (timeout)
- **Issue**: SBOM not found - Agent did not process pod

---

## Key Findings

1. ✅ **Pod Creation**: Working correctly (8-14 seconds)
2. ❌ **SBOM Extraction**: Failing - Agent not processing test pods
3. ⏸️ **CVE Matching**: Not reached (blocked by SBOM extraction)
4. ⏸️ **Insight Generation**: Not reached
5. ⏸️ **API Verification**: Not reached

---

## Root Cause

**Primary Issue**: Agent local pod watcher is not detecting new test pods immediately.

**Contributing Factors**:
- Test pods may be scheduled on different node than agent
- Agent watcher may have delay in detecting new pods
- No node selector in test script to ensure pod is on agent node

---

## Fixes Applied

1. ✅ Fixed `db_query()` function to use kubectl exec instead of psql
2. ✅ Added node selector to test script to ensure pod is on agent node
3. ✅ Improved error handling in wait functions

---

## Next Steps

1. ✅ Fix script with node selector (completed)
2. ⏳ Re-run E2E test with fixed script
3. ⏳ Verify SBOM extraction works
4. ⏳ Complete full E2E flow
5. ⏳ Generate final test report

---

## File Structure

```
results/
├── README.md                           # This file
├── E2E_TEST_SUMMARY_*.md              # Test execution summaries
├── E2E_TEST_REPORT_*.md               # Detailed test reports
├── e2e_execution_*.log                # Complete execution logs
├── e2e_*.log                          # Phase-specific logs
└── sample_optimization_results.md     # Sample optimization results
```

---

## How to View Results

1. **Quick Overview**: Read `E2E_TEST_SUMMARY_*.md` (latest timestamp)
2. **Detailed Analysis**: Read `E2E_TEST_REPORT_*.md` (latest timestamp)
3. **Full Logs**: Check `e2e_execution_*.log` files

---

**Last Updated**: 2025-12-25 15:43:00

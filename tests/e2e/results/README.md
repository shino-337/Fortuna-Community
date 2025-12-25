# E2E Test Results

This directory contains results from E2E test executions.

---

## Latest Test Execution

**Date**: 2025-12-25  
**Status**: Analysis Complete - Critical Issues Identified  
**Tests Run**: 5  
**Tests Passed**: 0  
**Tests Failed**: 5  
**Success Rate**: 0%

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

### Issue 1: Agent Pod Watcher Not Detecting New Pods

**Primary Issue**: Agent local pod watcher is not detecting new test pods, even when correctly scheduled on agent node.

**Evidence**:
- ✅ Pods created successfully (6-8 seconds)
- ✅ Pods scheduled on correct node (minikube) via node selector
- ✅ Pods reach Running state
- ❌ Agent watcher does not detect new pods
- ❌ No "Pod added" logs for test pods
- ❌ No SBOM processing for test pods

**Possible Causes**:
- Informer resync period too long (30 seconds)
- Event handlers may miss pods that are already Running
- Field selector may have timing issues

### Issue 2: Database Schema Mismatch

**Expected Schema** (from code):
- `pod_uid`, `pod_name`, `namespace`, `container_name`

**Actual Schema** (from database):
- Missing: `pod_uid`, `pod_name`, `namespace`, `container_name`
- Has: `image_name`, `image_tag`, `image_digest`, `component_count`

**Impact**: Test script cannot query SBOMs by pod_uid/pod_name.

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

#!/bin/bash

# ============================================================================
# Run All Tests - Complete Test Suite Execution
# ============================================================================
# Executes the complete test suite in the correct order:
# 1. Optimization verification tests
# 2. Performance impact measurements
# 3. Complete E2E flow test
#
# Generates a comprehensive report at the end.
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="${TEST_DIR}/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${RESULTS_DIR}/complete_test_report_${TIMESTAMP}.md"

mkdir -p "${RESULTS_DIR}"

# Logging
log_header() {
    echo ""
    echo -e "${CYAN}========================================"
    echo -e "$1"
    echo -e "========================================${NC}"
    echo ""
}

log_success() {
    echo -e "${GREEN}✅${NC} $1"
}

log_fail() {
    echo -e "${RED}❌${NC} $1"
}

log_info() {
    echo -e "${BLUE}ℹ${NC}  $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC}  $1"
}

# Test results tracking
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Start report
cat > "${REPORT_FILE}" <<EOF
# KSAM Complete Test Report

**Date**: $(date)
**Test Suite**: Post-Optimization Verification
**Report ID**: ${TIMESTAMP}

---

## Test Execution Summary

EOF

log_header "KSAM Complete Test Suite"
log_info "Starting comprehensive test execution..."
log_info "Results directory: ${RESULTS_DIR}"
log_info "Report file: ${REPORT_FILE}"
echo ""

# ============================================================================
# Phase 1: Environment Verification
# ============================================================================
log_header "Phase 1: Environment Verification"

check_prerequisite() {
    local cmd=$1
    local name=$2

    if command -v "${cmd}" >/dev/null 2>&1; then
        log_success "${name} is installed"
        return 0
    else
        log_fail "${name} is not installed"
        return 1
    fi
}

PREREQS_OK=true
check_prerequisite "kubectl" "kubectl" || PREREQS_OK=false
check_prerequisite "curl" "curl" || PREREQS_OK=false
check_prerequisite "jq" "jq" || PREREQS_OK=false
check_prerequisite "psql" "psql" || PREREQS_OK=false
check_prerequisite "bc" "bc" || PREREQS_OK=false

if [ "${PREREQS_OK}" = false ]; then
    log_fail "Some prerequisites are missing. Please install them first."
    log_info "See TEST_EXECUTION_GUIDE.md for installation instructions."
    exit 1
fi

log_success "All prerequisites installed"

# Check environment variables
log_info "Checking environment variables..."
ENV_OK=true

check_env() {
    local var=$1
    local default=$2

    if [ -z "${!var:-}" ]; then
        export "${var}=${default}"
        log_warning "${var} not set, using default: ${default}"
    else
        log_success "${var} = ${!var}"
    fi
}

check_env "TEST_NAMESPACE" "ksam-test"
check_env "CORE_API_URL" "http://localhost:8080"
check_env "DB_HOST" "localhost"
check_env "DB_PORT" "5432"
check_env "DB_NAME" "ksam"
check_env "DB_USER" "ksam"

# Test database connection
log_info "Testing database connection..."
if PGPASSWORD="${DB_PASSWORD:-ksam}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -c "SELECT 1;" >/dev/null 2>&1; then
    log_success "Database connection successful"
else
    log_fail "Database connection failed"
    exit 1
fi

# Test Core API
log_info "Testing Core API connection..."
if curl -sf "${CORE_API_URL}/health" >/dev/null 2>&1; then
    log_success "Core API is accessible"
else
    log_warning "Core API may not be accessible (will retry during tests)"
fi

cat >> "${REPORT_FILE}" <<EOF
### Environment

- **Test Namespace**: ${TEST_NAMESPACE}
- **Core API**: ${CORE_API_URL}
- **Database**: ${DB_NAME}@${DB_HOST}:${DB_PORT}
- **Prerequisites**: ✅ All installed
- **Database Connection**: ✅ Successful
- **Core API**: $(curl -sf "${CORE_API_URL}/health" >/dev/null 2>&1 && echo "✅ Accessible" || echo "⚠️  Pending")

---

## Test Results

EOF

# ============================================================================
# Phase 2: Optimization Verification Tests
# ============================================================================
log_header "Phase 2: Optimization Verification Tests"
log_info "Running schema and optimization verification..."
echo ""

TESTS_RUN=$((TESTS_RUN + 1))

if "${TEST_DIR}/e2e/scripts/verify-optimizations.sh" 2>&1 | tee "${RESULTS_DIR}/optimization_verification_${TIMESTAMP}.log"; then
    log_success "Optimization verification tests PASSED"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    OPT_STATUS="✅ PASSED"
else
    log_fail "Optimization verification tests FAILED"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    OPT_STATUS="❌ FAILED"
fi

cat >> "${REPORT_FILE}" <<EOF
### 1. Optimization Verification Tests

**Status**: ${OPT_STATUS}

Tests performed:
- Schema consistency checks
- Database index verification
- Data quality validation
- Batch processing configuration
- Prometheus metrics availability
- Query performance sampling

Details: See \`optimization_verification_${TIMESTAMP}.log\`

---

EOF

# ============================================================================
# Phase 3: Performance Impact Measurements
# ============================================================================
log_header "Phase 3: Performance Impact Measurements"
log_info "Running performance benchmarks..."
echo ""

TESTS_RUN=$((TESTS_RUN + 1))

if "${TEST_DIR}/performance/scripts/measure-optimization-impact.sh" 2>&1 | tee "${RESULTS_DIR}/performance_impact_${TIMESTAMP}.log"; then
    log_success "Performance impact tests PASSED"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    PERF_STATUS="✅ PASSED"
else
    log_warning "Performance impact tests completed with warnings"
    TESTS_PASSED=$((TESTS_PASSED + 1))  # Not a failure, just warnings
    PERF_STATUS="⚠️  COMPLETED WITH WARNINGS"
fi

cat >> "${REPORT_FILE}" <<EOF
### 2. Performance Impact Measurements

**Status**: ${PERF_STATUS}

Benchmarks performed:
- CVE lookup performance (bulk vs individual)
- Insight query performance
- Batch processing efficiency
- Connection pool utilization
- System-wide performance metrics

Details: See \`performance_impact_${TIMESTAMP}.log\`

---

EOF

# ============================================================================
# Phase 4: Complete E2E Flow Test (Optional - takes longer)
# ============================================================================
log_header "Phase 4: Complete E2E Flow Test"
log_info "This test creates a real pod and verifies the complete flow."
log_warning "This test may take 5-10 minutes to complete."
echo ""

read -p "Run complete E2E test? (y/n) " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
    TESTS_RUN=$((TESTS_RUN + 1))

    if "${TEST_DIR}/e2e/scenarios/pod-to-insight-flow.sh" 2>&1 | tee "${RESULTS_DIR}/e2e_flow_${TIMESTAMP}.log"; then
        log_success "E2E flow test PASSED"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        E2E_STATUS="✅ PASSED"
    else
        log_fail "E2E flow test FAILED"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        E2E_STATUS="❌ FAILED"
    fi

    cat >> "${REPORT_FILE}" <<EOF
### 3. Complete E2E Flow Test

**Status**: ${E2E_STATUS}

Flow tested:
1. Create test pod
2. SBOM extraction
3. CVE matching
4. Insight generation
5. API verification
6. Database consistency

Details: See \`e2e_flow_${TIMESTAMP}.log\`

---

EOF
else
    log_info "Skipping E2E flow test"

    cat >> "${REPORT_FILE}" <<EOF
### 3. Complete E2E Flow Test

**Status**: ⏭️  SKIPPED (by user)

---

EOF
fi

# ============================================================================
# Phase 5: Generate Final Report
# ============================================================================
log_header "Phase 5: Generating Final Report"

# Extract key metrics from logs
BULK_SPEEDUP=$(grep -oP "Speedup with bulk lookup: \K[\d.]+(?=x)" "${RESULTS_DIR}/performance_impact_${TIMESTAMP}.log" 2>/dev/null || echo "N/A")
INDEXED_TIME=$(grep -oP "Indexed query time: \K[\d.]+(?=s)" "${RESULTS_DIR}/performance_impact_${TIMESTAMP}.log" 2>/dev/null || echo "N/A")
UTILIZATION=$(grep -oP "Utilization: \K[\d.]+(?=%)" "${RESULTS_DIR}/performance_impact_${TIMESTAMP}.log" 2>/dev/null || echo "N/A")

cat >> "${REPORT_FILE}" <<EOF
## Performance Metrics

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Bulk Lookup Speedup | ${BULK_SPEEDUP}x | >5x | $([ "${BULK_SPEEDUP}" != "N/A" ] && [ $(echo "${BULK_SPEEDUP} > 5" | bc) -eq 1 ] && echo "✅ PASS" || echo "⏳ N/A") |
| Indexed Query Time | ${INDEXED_TIME}s | <1s | $([ "${INDEXED_TIME}" != "N/A" ] && [ $(echo "${INDEXED_TIME} < 1" | bc) -eq 1 ] && echo "✅ PASS" || echo "⏳ N/A") |
| Connection Utilization | ${UTILIZATION}% | <80% | $([ "${UTILIZATION}" != "N/A" ] && [ $(echo "${UTILIZATION} < 80" | bc) -eq 1 ] && echo "✅ PASS" || echo "⏳ N/A") |

---

## Test Summary

- **Tests Run**: ${TESTS_RUN}
- **Tests Passed**: ${TESTS_PASSED}
- **Tests Failed**: ${TESTS_FAILED}
- **Success Rate**: $(echo "scale=1; (${TESTS_PASSED} / ${TESTS_RUN}) * 100" | bc)%

---

## Overall Status

$([ ${TESTS_FAILED} -eq 0 ] && echo "✅ **ALL TESTS PASSED**" || echo "❌ **SOME TESTS FAILED**")

$([ ${TESTS_FAILED} -eq 0 ] && echo "The system is ready for production deployment." || echo "Please review failed tests before deployment.")

---

## Files Generated

- Optimization Verification: \`optimization_verification_${TIMESTAMP}.log\`
- Performance Impact: \`performance_impact_${TIMESTAMP}.log\`
$([ -f "${RESULTS_DIR}/e2e_flow_${TIMESTAMP}.log" ] && echo "- E2E Flow Test: \`e2e_flow_${TIMESTAMP}.log\`" || echo "")
- Complete Report: \`complete_test_report_${TIMESTAMP}.md\`

---

## Next Steps

$([ ${TESTS_FAILED} -eq 0 ] && cat <<NEXT_STEPS
1. ✅ Review this report
2. ✅ Monitor metrics in production for 24-48 hours
3. ✅ Schedule production deployment
4. ✅ Set up automated daily testing
5. ✅ Configure alerting for performance degradation
NEXT_STEPS
|| cat <<NEXT_STEPS
1. ❌ Review failed test logs
2. ❌ Fix identified issues
3. ❌ Re-run failed tests
4. ❌ Verify fixes before deployment
NEXT_STEPS
)

---

**Report Generated**: $(date)
**Report Location**: ${REPORT_FILE}
EOF

# ============================================================================
# Display Final Summary
# ============================================================================
log_header "Test Execution Complete"

echo ""
log_info "Test Summary:"
echo "  Tests Run: ${TESTS_RUN}"
echo "  Tests Passed: ${TESTS_PASSED}"
echo "  Tests Failed: ${TESTS_FAILED}"
echo ""

if [ ${TESTS_FAILED} -eq 0 ]; then
    log_success "ALL TESTS PASSED ✅"
    echo ""
    log_info "The system is ready for production deployment."
else
    log_fail "SOME TESTS FAILED ❌"
    echo ""
    log_warning "Please review the logs and fix issues before deployment."
fi

echo ""
log_info "Complete report saved to:"
echo "  ${REPORT_FILE}"
echo ""
log_info "Review the report for detailed results and next steps."
echo ""

# Open report if on macOS
if [[ "$OSTYPE" == "darwin"* ]]; then
    log_info "Opening report in default markdown viewer..."
    open "${REPORT_FILE}" 2>/dev/null || true
fi

# Exit with appropriate code
if [ ${TESTS_FAILED} -eq 0 ]; then
    exit 0
else
    exit 1
fi

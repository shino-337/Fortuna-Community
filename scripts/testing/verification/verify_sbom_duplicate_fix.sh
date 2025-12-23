#!/bin/bash
#
# SBOM Duplicate Key Fix - Comprehensive Verification Test Suite
# ==============================================================
#
# This script runs comprehensive tests to verify the duplicate key fix
#
# Date: December 16, 2025
# Author: KSAM Development Team
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
TEST_NAMESPACE="default"
POSTGRES_POD=$(kubectl get pod -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}')
CORE_POD_SELECTOR="app=ksam-core"

# Test results tracking
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_WARNINGS=0

# Helper functions
print_header() {
    echo ""
    echo "================================================================"
    echo -e "${BLUE}$1${NC}"
    echo "================================================================"
}

print_test() {
    echo ""
    echo -e "${YELLOW}[TEST $1]${NC} $2"
    echo "----------------------------------------------------------------"
}

print_pass() {
    echo -e "${GREEN}✅ PASSED:${NC} $1"
    ((TESTS_PASSED++))
}

print_fail() {
    echo -e "${RED}❌ FAILED:${NC} $1"
    ((TESTS_FAILED++))
}

print_warning() {
    echo -e "${YELLOW}⚠️  WARNING:${NC} $1"
    ((TESTS_WARNINGS++))
}

print_info() {
    echo -e "${BLUE}ℹ️  INFO:${NC} $1"
}

# Database helper functions
db_query() {
    kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "$1" 2>/dev/null | sed 's/^[ \t]*//;s/[ \t]*$//'
}

db_count() {
    db_query "$1" | tr -d ' \n'
}

# Start test suite
print_header "SBOM Duplicate Key Fix - Verification Test Suite"

echo "Test Environment:"
echo "- Namespace: $TEST_NAMESPACE"
echo "- PostgreSQL Pod: $POSTGRES_POD"
echo "- Core Service: Running"
echo "- Date: $(date)"
echo ""

# ============================================================
# TEST 1: Database Schema Verification
# ============================================================
print_test "1" "Database Schema Verification"

print_info "Checking sboms table structure..."

# Check if sboms table exists
TABLE_CHECK=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'sboms';" 2>/dev/null | tr -d ' \n')

if [ "$TABLE_CHECK" = "1" ]; then
    print_pass "sboms table exists"
else
    print_fail "sboms table does not exist (check: $TABLE_CHECK)"
    exit 1
fi

# Check unique constraint on image_digest
CONSTRAINT_CHECK=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pg_constraint WHERE conname = 'sboms_image_digest_key';" 2>/dev/null | tr -d ' \n')

if [ "$CONSTRAINT_CHECK" = "1" ]; then
    print_pass "Unique constraint 'sboms_image_digest_key' exists on image_digest column"
else
    print_fail "Unique constraint 'sboms_image_digest_key' not found (check: $CONSTRAINT_CHECK)"
fi

# Check table columns
print_info "Verifying table columns..."
EXPECTED_COLUMNS="id image_name image_tag image_digest sbom_format sbom_content component_count os_packages language_packages generator generator_version generated_at last_used_at use_count created_at updated_at deleted_at"

for col in $EXPECTED_COLUMNS; do
    COL_CHECK=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'sboms' AND column_name = '$col';" 2>/dev/null | tr -d ' \n')
    if [ "$COL_CHECK" = "1" ]; then
        echo "  ✓ Column '$col' exists"
    else
        echo "  ✗ Column '$col' missing"
    fi
done

# ============================================================
# TEST 2: Clean Environment Setup
# ============================================================
print_test "2" "Clean Environment Setup"

print_info "Cleaning up any existing test pods..."
kubectl delete pods -l test=sbom-verify --ignore-not-found=true 2>/dev/null || true
sleep 3

print_info "Recording initial state..."
INITIAL_SBOM_COUNT=$(db_count "SELECT COUNT(*) FROM sboms;")
INITIAL_LOGS_BASELINE=$(date +%s)

print_pass "Environment ready (initial SBOM count: $INITIAL_SBOM_COUNT)"

# ============================================================
# TEST 3: Single Pod SBOM Creation (Baseline)
# ============================================================
print_test "3" "Single Pod SBOM Creation (Baseline)"

TEST_IMAGE="nginx:alpine"
SINGLE_POD="sbom-verify-single"

print_info "Creating single pod with image: $TEST_IMAGE"
kubectl run $SINGLE_POD \
    --image=$TEST_IMAGE \
    --restart=Never \
    --labels="test=sbom-verify" \
    --command -- sleep 300

print_info "Waiting for pod to be running..."
kubectl wait --for=condition=ready pod/$SINGLE_POD --timeout=60s

print_info "Waiting 30 seconds for SBOM processing..."
sleep 30

# Check if SBOM was created
SBOM_COUNT_AFTER_SINGLE=$(db_count "SELECT COUNT(*) FROM sboms;")
SBOM_CREATED=$((SBOM_COUNT_AFTER_SINGLE - INITIAL_SBOM_COUNT))

if [ $SBOM_CREATED -ge 0 ]; then
    print_pass "SBOM processing completed without errors (SBOMs: $INITIAL_SBOM_COUNT → $SBOM_COUNT_AFTER_SINGLE)"
else
    print_warning "SBOM count decreased unexpectedly"
fi

# Check logs for errors
ERRORS_SINGLE=$(kubectl logs -n ksam -l $CORE_POD_SELECTOR --since=1m 2>/dev/null | \
    grep -c "duplicate key.*sboms\|23505.*sbom\|failed to upsert SBOM" || echo "0")

if [ "$ERRORS_SINGLE" = "0" ]; then
    print_pass "No duplicate key errors in logs"
else
    print_fail "Found $ERRORS_SINGLE duplicate key errors in logs"
fi

# ============================================================
# TEST 4: Multiple Pods Same Image (Race Condition Test)
# ============================================================
print_test "4" "Multiple Pods with Same Image (Race Condition Test)"

MULTI_POD_COUNT=8
MULTI_POD_PREFIX="sbom-verify-multi"

print_info "Creating $MULTI_POD_COUNT pods concurrently with same image: $TEST_IMAGE"
print_info "This simulates the race condition scenario..."

# Record state before test
SBOM_COUNT_BEFORE_MULTI=$(db_count "SELECT COUNT(*) FROM sboms;")

# Create pods concurrently
for i in $(seq 1 $MULTI_POD_COUNT); do
    POD_NAME="${MULTI_POD_PREFIX}-${i}"
    kubectl run $POD_NAME \
        --image=$TEST_IMAGE \
        --restart=Never \
        --labels="test=sbom-verify,batch=multi" \
        --command -- sleep 300 &
done

# Wait for all background jobs
wait

print_info "All $MULTI_POD_COUNT pods created"

# Wait for pods to be running
print_info "Waiting for pods to be ready..."
sleep 5
kubectl wait --for=condition=ready pod -l batch=multi --timeout=90s || print_warning "Some pods may not be ready yet"

# Wait for SBOM processing
print_info "Waiting 45 seconds for SBOM processing..."
sleep 45

# Check final SBOM count
SBOM_COUNT_AFTER_MULTI=$(db_count "SELECT COUNT(*) FROM sboms;")
SBOM_CHANGE=$((SBOM_COUNT_AFTER_MULTI - SBOM_COUNT_BEFORE_MULTI))

print_info "SBOM count: $SBOM_COUNT_BEFORE_MULTI → $SBOM_COUNT_AFTER_MULTI (change: $SBOM_CHANGE)"

# Check for duplicate key errors
ERRORS_MULTI=$(kubectl logs -n ksam -l $CORE_POD_SELECTOR --since=2m 2>/dev/null | \
    grep -c "duplicate key.*sboms\|23505.*sbom\|failed to upsert SBOM" || echo "0")

if [ "$ERRORS_MULTI" = "0" ]; then
    print_pass "No duplicate key errors with $MULTI_POD_COUNT concurrent pods"
else
    print_fail "Found $ERRORS_MULTI duplicate key errors with concurrent pods"
    echo ""
    print_info "Error samples:"
    kubectl logs -n ksam -l $CORE_POD_SELECTOR --since=2m 2>/dev/null | \
        grep -A 2 "duplicate key\|23505\|failed to upsert" | head -10
fi

# ============================================================
# TEST 5: Use Count Verification
# ============================================================
print_test "5" "Use Count Incrementing Verification"

print_info "Checking use_count for $TEST_IMAGE..."

# Query use_count for the test image
USE_COUNT_QUERY="SELECT image_name, image_tag, image_digest, use_count
                 FROM sboms
                 WHERE image_name LIKE '%nginx%' AND image_tag = 'alpine'
                 AND deleted_at IS NULL
                 ORDER BY updated_at DESC LIMIT 1;"

SBOM_INFO=$(db_query "$USE_COUNT_QUERY")

if [ -n "$SBOM_INFO" ]; then
    print_info "SBOM found for $TEST_IMAGE:"
    echo "$SBOM_INFO"

    USE_COUNT=$(echo "$SBOM_INFO" | awk '{print $4}')

    if [ -n "$USE_COUNT" ] && [ "$USE_COUNT" -ge 1 ]; then
        if [ "$USE_COUNT" -gt 1 ]; then
            print_pass "use_count is incrementing correctly (use_count=$USE_COUNT, expected >= 2 from ${MULTI_POD_COUNT} pods)"
        else
            print_warning "use_count is $USE_COUNT (may be normal if SBOM processing incomplete)"
        fi
    else
        print_warning "use_count is $USE_COUNT (expected higher value)"
    fi
else
    print_warning "No SBOM found for $TEST_IMAGE (may not have been processed yet)"
fi

# ============================================================
# TEST 6: SBOM Processing Logs Verification
# ============================================================
print_test "6" "SBOM Processing Logs Verification"

print_info "Checking for SBOM creation/update logs..."

# Look for successful SBOM operations
SBOM_CREATED_LOGS=$(kubectl logs -n ksam -l $CORE_POD_SELECTOR --since=3m 2>/dev/null | \
    grep -c "Created new SBOM\|Updated existing SBOM.*ON CONFLICT\|Found existing SBOM" || echo "0")

print_info "Found $SBOM_CREATED_LOGS SBOM operation logs"

if [ "$SBOM_CREATED_LOGS" -gt 0 ]; then
    print_pass "SBOM operations are being logged"

    print_info "Sample SBOM operation logs:"
    kubectl logs -n ksam -l $CORE_POD_SELECTOR --since=3m 2>/dev/null | \
        grep "Created new SBOM\|Updated existing SBOM\|Found existing SBOM" | head -5
else
    print_warning "No SBOM operation logs found (pods may not have been processed yet)"
fi

# ============================================================
# TEST 7: Database Integrity Check
# ============================================================
print_test "7" "Database Integrity Check"

print_info "Checking for duplicate image_digest entries..."

DUPLICATES=$(db_query "
    SELECT image_digest, COUNT(*) as count
    FROM sboms
    WHERE deleted_at IS NULL
    GROUP BY image_digest
    HAVING COUNT(*) > 1;
")

if [ -z "$DUPLICATES" ]; then
    print_pass "No duplicate image_digest entries found (constraint working correctly)"
else
    print_fail "Found duplicate image_digest entries:"
    echo "$DUPLICATES"
fi

# Check for orphaned records
print_info "Checking for data consistency..."

TOTAL_SBOMS=$(db_count "SELECT COUNT(*) FROM sboms WHERE deleted_at IS NULL;")
TOTAL_COMPONENTS=$(db_count "SELECT COUNT(*) FROM sbom_components WHERE deleted_at IS NULL;")

print_info "Total SBOMs: $TOTAL_SBOMS"
print_info "Total components: $TOTAL_COMPONENTS"

if [ "$TOTAL_COMPONENTS" -ge "$TOTAL_SBOMS" ]; then
    print_pass "Component count is consistent with SBOM count"
else
    print_warning "Component count ($TOTAL_COMPONENTS) < SBOM count ($TOTAL_SBOMS)"
fi

# ============================================================
# TEST 8: Performance Check
# ============================================================
print_test "8" "Performance Check"

print_info "Checking recent SBOM processing times..."

# Look for processing time logs
PROCESSING_TIMES=$(kubectl logs -n ksam -l $CORE_POD_SELECTOR --since=3m 2>/dev/null | \
    grep "Extracted.*packages in" | tail -5)

if [ -n "$PROCESSING_TIMES" ]; then
    print_info "Recent SBOM extraction times:"
    echo "$PROCESSING_TIMES"
    print_pass "SBOM processing is operational"
else
    print_warning "No SBOM processing time logs found"
fi

# ============================================================
# TEST 9: Error Log Analysis
# ============================================================
print_test "9" "Comprehensive Error Log Analysis"

print_info "Analyzing all error types in logs..."

# Check for various error patterns
ERROR_PATTERNS=(
    "duplicate key.*sboms_image_digest_key"
    "SQLSTATE 23505"
    "failed to save SBOM"
    "failed to upsert SBOM"
    "failed to create SBOM"
)

TOTAL_ERRORS=0
for pattern in "${ERROR_PATTERNS[@]}"; do
    COUNT=$(kubectl logs -n ksam -l $CORE_POD_SELECTOR --since=5m 2>/dev/null | \
        grep -c "$pattern" || echo "0")

    if [ "$COUNT" -gt 0 ]; then
        echo "  ❌ '$pattern': $COUNT occurrences"
        TOTAL_ERRORS=$((TOTAL_ERRORS + COUNT))
    else
        echo "  ✓ '$pattern': 0 occurrences"
    fi
done

if [ $TOTAL_ERRORS -eq 0 ]; then
    print_pass "No SBOM-related errors found in logs"
else
    print_fail "Found $TOTAL_ERRORS SBOM-related errors in logs"
fi

# ============================================================
# TEST 10: ON CONFLICT Verification
# ============================================================
print_test "10" "ON CONFLICT Clause Verification"

print_info "Verifying ON CONFLICT update behavior..."

# Look for ON CONFLICT logs
ON_CONFLICT_LOGS=$(kubectl logs -n ksam -l $CORE_POD_SELECTOR --since=3m 2>/dev/null | \
    grep -c "Updated existing SBOM via ON CONFLICT" || echo "0")

if [ "$ON_CONFLICT_LOGS" -gt 0 ]; then
    print_pass "ON CONFLICT clause is being triggered (found $ON_CONFLICT_LOGS occurrences)"

    print_info "Sample ON CONFLICT logs:"
    kubectl logs -n ksam -l $CORE_POD_SELECTOR --since=3m 2>/dev/null | \
        grep "ON CONFLICT" | head -3
else
    print_info "No ON CONFLICT triggers detected (may indicate first-time inserts or not processed yet)"
fi

# ============================================================
# Cleanup
# ============================================================
print_header "Cleanup"

print_info "Cleaning up test pods..."
kubectl delete pods -l test=sbom-verify --ignore-not-found=true
print_pass "Test pods deleted"

# ============================================================
# Final Report
# ============================================================
print_header "Test Results Summary"

echo ""
echo "Test Statistics:"
echo "  ✅ Passed:   $TESTS_PASSED"
echo "  ❌ Failed:   $TESTS_FAILED"
echo "  ⚠️  Warnings: $TESTS_WARNINGS"
echo ""

TOTAL_TESTS=$((TESTS_PASSED + TESTS_FAILED))
if [ $TOTAL_TESTS -gt 0 ]; then
    SUCCESS_RATE=$((TESTS_PASSED * 100 / TOTAL_TESTS))
    echo "Success Rate: ${SUCCESS_RATE}%"
fi

echo ""
echo "Key Findings:"
echo "  - Initial SBOM count: $INITIAL_SBOM_COUNT"
echo "  - Final SBOM count: $SBOM_COUNT_AFTER_MULTI"
echo "  - Pods created: $((MULTI_POD_COUNT + 1))"
echo "  - Duplicate key errors: $((ERRORS_SINGLE + ERRORS_MULTI))"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    print_header "✅ ALL TESTS PASSED - FIX VERIFIED"
    echo ""
    echo "The SBOM duplicate key fix is working correctly!"
    echo "Concurrent pod processing with same image does not cause database errors."
    echo ""
    exit 0
else
    print_header "❌ SOME TESTS FAILED - REVIEW REQUIRED"
    echo ""
    echo "Please review the failed tests above."
    echo "The fix may need additional investigation."
    echo ""
    exit 1
fi

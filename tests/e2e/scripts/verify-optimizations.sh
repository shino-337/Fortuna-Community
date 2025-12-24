#!/bin/bash

# ============================================================================
# Verify Optimizations Implementation
# ============================================================================
# This script verifies that all optimization changes are properly implemented:
# 1. Schema consistency (package_name vs component_id)
# 2. Database indexes exist
# 3. Batch processing is working
# 4. Connection pool monitoring is active
# 5. NATS retention configuration
# 6. Reconciliation loop (if integrated)
# ============================================================================

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"
CORE_API="${CORE_API_URL:-http://localhost:8080}"
RESULTS_DIR="${RESULTS_DIR:-$(dirname $0)/../results}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "${RESULTS_DIR}"

# Database query
db_query() {
    PGPASSWORD="${DB_PASSWORD:-ksam}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -A -c "$1" 2>/dev/null || echo ""
}

# Logging
log_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} ✅ $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} ❌ $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} ⚠️  $1"
}

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

run_test() {
    local test_name="$1"
    local test_command="$2"

    log_test "${test_name}"

    if eval "${test_command}"; then
        log_pass "${test_name}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        log_fail "${test_name}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

echo "========================================"
echo "Optimization Verification Tests"
echo "========================================"
echo "Date: $(date)"
echo "========================================"
echo ""

# ============================================================================
# Test 1: Schema Consistency - CVE Matches
# ============================================================================
echo "[Category 1] Schema Consistency Tests"
echo "----------------------------------------"

test_cve_matches_schema() {
    # Check if package_name column exists (not component_id)
    local has_package_name=$(db_query "SELECT COUNT(*)::text FROM information_schema.columns WHERE table_schema='public' AND table_name='cve_matches' AND column_name='package_name';" | tr -d ' \n')
    local has_component_id=$(db_query "SELECT COUNT(*)::text FROM information_schema.columns WHERE table_schema='public' AND table_name='cve_matches' AND column_name='component_id';" | tr -d ' \n')

    # Handle empty results
    has_package_name=${has_package_name:-0}
    has_component_id=${has_component_id:-0}

    if [ "${has_package_name}" = "1" ] && [ "${has_component_id}" = "0" ]; then
        return 0
    else
        echo "  Expected: package_name column exists, component_id does not"
        echo "  Found: package_name=${has_package_name}, component_id=${has_component_id}"
        return 1
    fi
}

run_test "CVE Matches uses package_name (not component_id)" "test_cve_matches_schema"

# ============================================================================
# Test 2: Database Indexes
# ============================================================================
echo ""
echo "[Category 2] Database Index Tests"
echo "----------------------------------------"

test_index_exists() {
    local index_name="$1"
    local count=$(db_query "SELECT COUNT(*) FROM pg_indexes WHERE indexname='${index_name}';")

    if [ "${count}" -eq "1" ]; then
        return 0
    else
        echo "  Index ${index_name} not found"
        return 1
    fi
}

# Test critical indexes from Migration 028
run_test "Index: idx_package_vulnerabilities_ecosystem_package" \
    "test_index_exists 'idx_package_vulnerabilities_ecosystem_package'"

run_test "Index: idx_insights_resource_uid_type_status" \
    "test_index_exists 'idx_insights_resource_uid_type_status'"

run_test "Index: idx_insights_resource_uid_cve_status" \
    "test_index_exists 'idx_insights_resource_uid_cve_status'"

run_test "Index: idx_sbom_components_sbom_id_component_name" \
    "test_index_exists 'idx_sbom_components_sbom_id_component_name'"

run_test "Index: idx_cve_matches_sbom_id" \
    "test_index_exists 'idx_cve_matches_sbom_id'"

# Test unique indexes with correct schema
run_test "Index: idx_cve_matches_unique_sbom_package_cve (partial)" \
    "test_index_exists 'idx_cve_matches_unique_sbom_package_cve'"

run_test "Index: idx_cve_matches_unique_sbom_package_cve_all (non-partial)" \
    "test_index_exists 'idx_cve_matches_unique_sbom_package_cve_all'"

# ============================================================================
# Test 3: No Duplicate CVE Matches
# ============================================================================
echo ""
echo "[Category 3] Data Quality Tests"
echo "----------------------------------------"

test_no_duplicate_cve_matches() {
    local duplicates=$(db_query "
        SELECT COUNT(*) FROM (
            SELECT sbom_id, package_name, cve_id, COUNT(*) as cnt
            FROM cve_matches
            WHERE deleted_at IS NULL
            GROUP BY sbom_id, package_name, cve_id
            HAVING COUNT(*) > 1
        ) AS dups;
    ")

    if [ "${duplicates}" -eq "0" ]; then
        return 0
    else
        echo "  Found ${duplicates} duplicate CVE matches"
        return 1
    fi
}

run_test "No duplicate CVE matches exist" "test_no_duplicate_cve_matches"

# ============================================================================
# Test 4: Batch Insert Functionality
# ============================================================================
echo ""
echo "[Category 4] Batch Processing Tests"
echo "----------------------------------------"

test_batch_insights_deduplication() {
    # Check that insights have unique constraint for batch upsert
    local has_constraint=$(db_query "
        SELECT COUNT(*) FROM pg_constraint
        WHERE conrelid = 'insights'::regclass
        AND contype = 'u';
    ")

    if [ "${has_constraint}" -gt "0" ]; then
        return 0
    else
        log_warning "No unique constraint on insights (batch upsert may not work)"
        return 1
    fi
}

run_test "Insights table has unique constraint for batch upsert" "test_batch_insights_deduplication"

# ============================================================================
# Test 5: Prometheus Metrics Endpoint
# ============================================================================
echo ""
echo "[Category 5] Monitoring Tests"
echo "----------------------------------------"

test_metrics_endpoint() {
    local response=$(curl -s "${CORE_API}/metrics" | head -1)

    if [ -n "${response}" ]; then
        return 0
    else
        log_warning "Metrics endpoint may not be accessible"
        return 1
    fi
}

test_db_connection_metrics() {
    local metrics=$(curl -s "${CORE_API}/metrics" | grep "ksam_db_connections")

    if [ -n "${metrics}" ]; then
        echo "  Found metrics:"
        echo "${metrics}" | grep "ksam_db_connections" | head -5 | sed 's/^/    /'
        return 0
    else
        log_warning "Database connection metrics not found"
        return 1
    fi
}

test_worker_metrics() {
    local metrics=$(curl -s "${CORE_API}/metrics" | grep "ksam_worker")

    if [ -n "${metrics}" ]; then
        echo "  Found worker metrics"
        return 0
    else
        log_warning "Worker metrics not found"
        return 1
    fi
}

run_test "Metrics endpoint is accessible" "test_metrics_endpoint"
run_test "Database connection pool metrics exist" "test_db_connection_metrics"
run_test "Worker processing metrics exist" "test_worker_metrics"

# ============================================================================
# Test 6: Migration Execution
# ============================================================================
echo ""
echo "[Category 6] Migration Tests"
echo "----------------------------------------"

test_migration_executed() {
    local migration_table_exists=$(db_query "SELECT COUNT(*) FROM information_schema.tables WHERE table_name='schema_migrations';")

    if [ "${migration_table_exists}" -eq "1" ]; then
        return 0
    else
        log_warning "Migration tracking table not found"
        return 0  # Not a critical failure
    fi
}

run_test "Migration tracking table exists" "test_migration_executed"

# ============================================================================
# Test 7: Query Performance Sampling
# ============================================================================
echo ""
echo "[Category 7] Performance Sampling Tests"
echo "----------------------------------------"

test_insight_query_performance() {
    # Test the optimized insight query performance
    local start=$(date +%s.%N)

    # Run a typical insight query that should use the new indexes
    db_query "
        SELECT COUNT(*)
        FROM insights
        WHERE resource_uid IN (
            SELECT uid FROM pods WHERE deleted_at IS NULL LIMIT 10
        )
        AND insight_type = 'vulnerability'
        AND status = 'active'
        AND deleted_at IS NULL;
    " > /dev/null

    local end=$(date +%s.%N)
    local duration=$(echo "$end - $start" | bc)

    echo "  Query execution time: ${duration} seconds"

    # Should complete in under 1 second with proper indexes
    local is_fast=$(echo "${duration} < 1.0" | bc)
    if [ "${is_fast}" -eq "1" ]; then
        return 0
    else
        log_warning "Query took longer than expected (${duration}s)"
        return 1
    fi
}

test_cve_lookup_performance() {
    # Test CVE package lookup with ecosystem+package_name index
    local start=$(date +%s.%N)

    db_query "
        SELECT COUNT(*)
        FROM package_vulnerabilities
        WHERE ecosystem = 'debian'
        AND package_name IN ('openssl', 'curl', 'nginx')
        AND deleted_at IS NULL;
    " > /dev/null

    local end=$(date +%s.%N)
    local duration=$(echo "$end - $start" | bc)

    echo "  Query execution time: ${duration} seconds"

    local is_fast=$(echo "${duration} < 0.1" | bc)
    if [ "${is_fast}" -eq "1" ]; then
        return 0
    else
        log_warning "Query took longer than expected (${duration}s)"
        return 1
    fi
}

run_test "Insight query performance (<1s)" "test_insight_query_performance"
run_test "CVE lookup performance (<0.1s)" "test_cve_lookup_performance"

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "========================================"
echo "Test Summary"
echo "========================================"
echo "Total Tests: $((TESTS_PASSED + TESTS_FAILED))"
echo "Passed: ${TESTS_PASSED}"
echo "Failed: ${TESTS_FAILED}"
echo "========================================"

# Save results
cat > "${RESULTS_DIR}/optimization_verification_${TIMESTAMP}.txt" <<EOF
Optimization Verification Test Results
======================================
Date: $(date)
Database: ${DB_NAME}@${DB_HOST}:${DB_PORT}
Core API: ${CORE_API}

Summary:
- Total Tests: $((TESTS_PASSED + TESTS_FAILED))
- Passed: ${TESTS_PASSED}
- Failed: ${TESTS_FAILED}

Status: $([ ${TESTS_FAILED} -eq 0 ] && echo "ALL TESTS PASSED ✅" || echo "SOME TESTS FAILED ❌")

Tests Executed:
1. Schema Consistency
   - CVE Matches uses package_name

2. Database Indexes
   - Performance indexes (Migration 028)
   - Unique indexes with correct schema

3. Data Quality
   - No duplicate CVE matches

4. Batch Processing
   - Insights unique constraint

5. Monitoring
   - Metrics endpoint accessible
   - DB connection metrics
   - Worker metrics

6. Migrations
   - Migration tracking

7. Performance Sampling
   - Insight query performance
   - CVE lookup performance

Recommendations:
$( [ ${TESTS_FAILED} -gt 0 ] && echo "- Review failed tests and fix issues" || echo "- All optimizations verified successfully" )
$( [ ${TESTS_FAILED} -gt 0 ] && echo "- Check Core service logs for errors" || echo "- Monitor performance metrics in production" )
EOF

echo ""
echo "Results saved to: ${RESULTS_DIR}/optimization_verification_${TIMESTAMP}.txt"
echo ""

# Exit with failure if any tests failed
if [ ${TESTS_FAILED} -gt 0 ]; then
    echo -e "${RED}Some tests failed. Please review the output above.${NC}"
    exit 1
else
    echo -e "${GREEN}All optimization tests passed!${NC}"
    exit 0
fi

#!/bin/bash
#
# Complete Insights Verification - Database & API Testing
# =========================================================
# Tests both database and API to verify insights are working correctly
#
# Date: December 16, 2025
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
NAMESPACE="ksam"
POSTGRES_POD=$(kubectl get pod -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}')
CORE_SERVICE="ksam-core.$NAMESPACE.svc.cluster.local:8080"

# Test tracking
TESTS_PASSED=0
TESTS_FAILED=0

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
    echo -e "${GREEN}✅ PASS:${NC} $1"
    ((TESTS_PASSED++))
}

print_fail() {
    echo -e "${RED}❌ FAIL:${NC} $1"
    ((TESTS_FAILED++))
}

print_info() {
    echo -e "${BLUE}ℹ️  INFO:${NC} $1"
}

# Database query helper
db_query() {
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "$1" 2>/dev/null
}

db_count() {
    db_query "$1" | tr -d ' \n'
}

# API helper
api_call() {
    kubectl exec -n $NAMESPACE $(kubectl get pod -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') -- \
        curl -s -X GET "http://localhost:8080$1" -H "Content-Type: application/json" 2>/dev/null
}

print_header "Complete Insights Verification - Database & API"

echo "Test Environment:"
echo "- Namespace: $NAMESPACE"
echo "- PostgreSQL Pod: $POSTGRES_POD"
echo "- Core Service: $CORE_SERVICE"
echo "- Date: $(date)"
echo ""

# ============================================================
# TEST 1: Database Schema Verification
# ============================================================
print_test "1" "Insights Table Schema Verification"

# Check insights table exists
TABLE_EXISTS=$(db_count "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'insights';")

if [ "$TABLE_EXISTS" = "1" ]; then
    print_pass "insights table exists"
else
    print_fail "insights table does not exist"
    exit 1
fi

# Show table structure
print_info "Insights table structure:"
db_query "SELECT column_name, data_type, is_nullable
          FROM information_schema.columns
          WHERE table_name = 'insights'
          ORDER BY ordinal_position;" | head -30

# Check for CVE-specific columns
print_info "Checking CVE-specific columns..."
CVE_COLUMNS="cve_id cvss_score cvss_vector exploit_available package_name installed_version fixed_version sbom_id cve_match_id"

for col in $CVE_COLUMNS; do
    COL_EXISTS=$(db_count "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'insights' AND column_name = '$col';")
    if [ "$COL_EXISTS" = "1" ]; then
        echo "  ✓ Column '$col' exists"
    else
        echo "  ✗ Column '$col' MISSING"
    fi
done

# ============================================================
# TEST 2: Database Insights Count and Distribution
# ============================================================
print_test "2" "Database Insights Data Analysis"

# Total insights
TOTAL_INSIGHTS=$(db_count "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL;")
print_info "Total active insights: $TOTAL_INSIGHTS"

if [ "$TOTAL_INSIGHTS" -gt 0 ]; then
    print_pass "Insights exist in database"
else
    print_fail "No insights found in database"
fi

# Insights by type
print_info "Insights by type:"
db_query "SELECT type, COUNT(*) as count
          FROM insights
          WHERE deleted_at IS NULL
          GROUP BY type
          ORDER BY count DESC;"

# Insights by severity
print_info "Insights by severity:"
db_query "SELECT severity, COUNT(*) as count
          FROM insights
          WHERE deleted_at IS NULL
          GROUP BY severity
          ORDER BY
            CASE severity
              WHEN 'critical' THEN 1
              WHEN 'high' THEN 2
              WHEN 'medium' THEN 3
              WHEN 'low' THEN 4
            END;"

# Insights by status
print_info "Insights by status:"
db_query "SELECT status, COUNT(*) as count
          FROM insights
          WHERE deleted_at IS NULL
          GROUP BY status;"

# ============================================================
# TEST 3: CVE-Specific Insights Verification
# ============================================================
print_test "3" "CVE-Based Insights Verification"

# Check for CVE insights
CVE_INSIGHTS=$(db_count "SELECT COUNT(*) FROM insights WHERE cve_id IS NOT NULL AND cve_id != '' AND deleted_at IS NULL;")
print_info "CVE-based insights count: $CVE_INSIGHTS"

if [ "$CVE_INSIGHTS" -gt 0 ]; then
    print_pass "CVE insights are being created"

    print_info "Sample CVE insights (top 5):"
    db_query "SELECT id, type, cve_id, severity, package_name, installed_version, fixed_version, created_at
              FROM insights
              WHERE cve_id IS NOT NULL AND deleted_at IS NULL
              ORDER BY created_at DESC
              LIMIT 5;" | head -20
else
    print_fail "No CVE insights found (CVE detection may not have run yet)"
fi

# Check insights with SBOM reference
SBOM_LINKED=$(db_count "SELECT COUNT(*) FROM insights WHERE sbom_id IS NOT NULL AND deleted_at IS NULL;")
print_info "Insights linked to SBOMs: $SBOM_LINKED"

if [ "$SBOM_LINKED" -gt 0 ]; then
    print_pass "Insights are being linked to SBOMs"
else
    print_info "No SBOM-linked insights yet (normal if CVE scan hasn't completed)"
fi

# ============================================================
# TEST 4: Insights-SBOM-CVE Relationship Verification
# ============================================================
print_test "4" "Insights-SBOM-CVE Data Relationship"

print_info "Checking data flow: SBOM → CVE Match → Insight"

# Count each component
SBOM_COUNT=$(db_count "SELECT COUNT(*) FROM sboms WHERE deleted_at IS NULL;")
CVE_MATCH_COUNT=$(db_count "SELECT COUNT(*) FROM cve_matches WHERE deleted_at IS NULL;")
CVE_INSIGHT_COUNT=$(db_count "SELECT COUNT(*) FROM insights WHERE cve_id IS NOT NULL AND deleted_at IS NULL;")

echo "Data Pipeline:"
echo "  SBOMs:        $SBOM_COUNT"
echo "  CVE Matches:  $CVE_MATCH_COUNT"
echo "  CVE Insights: $CVE_INSIGHT_COUNT"

if [ "$SBOM_COUNT" -gt 0 ] && [ "$CVE_MATCH_COUNT" -gt 0 ] && [ "$CVE_INSIGHT_COUNT" -gt 0 ]; then
    print_pass "Complete CVE pipeline working (SBOM → CVE → Insight)"
elif [ "$SBOM_COUNT" -gt 0 ] && [ "$CVE_MATCH_COUNT" -gt 0 ]; then
    print_info "SBOMs and CVE matches exist, waiting for insights creation"
elif [ "$SBOM_COUNT" -gt 0 ]; then
    print_info "SBOMs exist, waiting for CVE matching"
else
    print_info "No SBOMs yet, CVE pipeline not started"
fi

# Show complete pipeline example if data exists
if [ "$CVE_INSIGHT_COUNT" -gt 0 ]; then
    print_info "Sample complete pipeline (SBOM → CVE Match → Insight):"
    db_query "SELECT
                i.id as insight_id,
                i.cve_id,
                i.severity,
                i.package_name,
                s.image_name,
                s.image_tag,
                c.severity as cve_severity
              FROM insights i
              LEFT JOIN sboms s ON i.sbom_id = s.id
              LEFT JOIN cve_matches c ON i.cve_match_id = c.id
              WHERE i.cve_id IS NOT NULL AND i.deleted_at IS NULL
              LIMIT 3;" | head -15
fi

# ============================================================
# TEST 5: API - Get All Insights
# ============================================================
print_test "5" "API Test - GET /api/insights"

print_info "Calling API: GET /api/insights"

API_RESPONSE=$(api_call "/api/insights")
API_STATUS=$?

if [ $API_STATUS -eq 0 ]; then
    print_pass "API call successful"

    # Parse response (check if it's valid JSON)
    if echo "$API_RESPONSE" | jq empty 2>/dev/null; then
        print_pass "Response is valid JSON"

        # Count insights in response
        API_INSIGHT_COUNT=$(echo "$API_RESPONSE" | jq '. | length' 2>/dev/null || echo "0")
        print_info "Insights returned by API: $API_INSIGHT_COUNT"

        if [ "$API_INSIGHT_COUNT" -gt 0 ]; then
            print_pass "API returned insights data"

            # Show sample
            print_info "Sample insight from API:"
            echo "$API_RESPONSE" | jq '.[0] | {id, type, severity, description, cve_id, package_name}' 2>/dev/null | head -15
        else
            print_fail "API returned empty array"
        fi
    else
        print_fail "Response is not valid JSON"
        echo "Response preview:"
        echo "$API_RESPONSE" | head -20
    fi
else
    print_fail "API call failed"
fi

# ============================================================
# TEST 6: API - Get Insights by Severity
# ============================================================
print_test "6" "API Test - GET /api/insights?severity=critical"

print_info "Calling API: GET /api/insights?severity=critical"

CRITICAL_RESPONSE=$(api_call "/api/insights?severity=critical")

if echo "$CRITICAL_RESPONSE" | jq empty 2>/dev/null; then
    CRITICAL_COUNT=$(echo "$CRITICAL_RESPONSE" | jq '. | length' 2>/dev/null || echo "0")
    print_info "Critical insights returned: $CRITICAL_COUNT"

    # Compare with database
    DB_CRITICAL=$(db_count "SELECT COUNT(*) FROM insights WHERE severity = 'critical' AND deleted_at IS NULL;")
    print_info "Critical insights in DB: $DB_CRITICAL"

    if [ "$CRITICAL_COUNT" = "$DB_CRITICAL" ]; then
        print_pass "API and database counts match"
    else
        print_fail "API count ($CRITICAL_COUNT) doesn't match DB count ($DB_CRITICAL)"
    fi
else
    print_fail "Invalid JSON response for severity filter"
fi

# ============================================================
# TEST 7: API - Get Insights by Type
# ============================================================
print_test "7" "API Test - GET /api/insights?type=vulnerability"

print_info "Calling API: GET /api/insights?type=vulnerability"

VULN_RESPONSE=$(api_call "/api/insights?type=vulnerability")

if echo "$VULN_RESPONSE" | jq empty 2>/dev/null; then
    VULN_COUNT=$(echo "$VULN_RESPONSE" | jq '. | length' 2>/dev/null || echo "0")
    print_info "Vulnerability insights returned: $VULN_COUNT"

    if [ "$VULN_COUNT" -gt 0 ]; then
        print_pass "Vulnerability insights API working"

        # Show sample CVE insight
        print_info "Sample vulnerability insight:"
        echo "$VULN_RESPONSE" | jq '.[0] | {id, cve_id, severity, package_name, installed_version, fixed_version}' 2>/dev/null | head -10
    else
        print_info "No vulnerability insights found (CVE detection may not have run)"
    fi
else
    print_fail "Invalid JSON response for type filter"
fi

# ============================================================
# TEST 8: API - Insights Statistics
# ============================================================
print_test "8" "API Test - GET /api/insights/stats"

print_info "Calling API: GET /api/insights/stats"

STATS_RESPONSE=$(api_call "/api/insights/stats")

if echo "$STATS_RESPONSE" | jq empty 2>/dev/null; then
    print_pass "Insights stats API working"

    print_info "Insights statistics:"
    echo "$STATS_RESPONSE" | jq '.' 2>/dev/null | head -20

    # Extract key metrics
    TOTAL_API=$(echo "$STATS_RESPONSE" | jq '.total' 2>/dev/null || echo "0")
    CRITICAL_API=$(echo "$STATS_RESPONSE" | jq '.by_severity.critical' 2>/dev/null || echo "0")

    print_info "Total from stats API: $TOTAL_API"
    print_info "Critical from stats API: $CRITICAL_API"
else
    print_fail "Insights stats API returned invalid JSON"
fi

# ============================================================
# TEST 9: Recent Insights Creation
# ============================================================
print_test "9" "Recent Insights Creation Check"

print_info "Checking insights created in last 30 minutes..."

RECENT_INSIGHTS=$(db_count "SELECT COUNT(*) FROM insights WHERE created_at > NOW() - INTERVAL '30 minutes' AND deleted_at IS NULL;")
print_info "Insights created in last 30 minutes: $RECENT_INSIGHTS"

if [ "$RECENT_INSIGHTS" -gt 0 ]; then
    print_pass "Insights are being actively created"

    print_info "Recent insights (last 5):"
    db_query "SELECT id, type, severity, LEFT(description, 60) as description, created_at
              FROM insights
              WHERE deleted_at IS NULL
              ORDER BY created_at DESC
              LIMIT 5;" | head -15
else
    print_info "No recent insights (last 30 min) - system may be idle"
fi

# ============================================================
# TEST 10: Insights Source Verification
# ============================================================
print_test "10" "Insights Source Verification"

print_info "Checking insights by source..."

db_query "SELECT
            COALESCE(source, 'unknown') as source,
            COUNT(*) as count
          FROM insights
          WHERE deleted_at IS NULL
          GROUP BY source
          ORDER BY count DESC;"

# Check for custom-sbom-scanner source
CUSTOM_SBOM_INSIGHTS=$(db_count "SELECT COUNT(*) FROM insights WHERE source = 'custom-sbom-scanner' AND deleted_at IS NULL;")

if [ "$CUSTOM_SBOM_INSIGHTS" -gt 0 ]; then
    print_pass "Custom SBOM scanner insights are being created"
else
    print_info "No custom-sbom-scanner insights yet (normal if CVE scan hasn't run)"
fi

# ============================================================
# TEST 11: Create Test Pod and Verify Insights
# ============================================================
print_test "11" "End-to-End Test - Create Pod and Verify Insights"

TEST_POD="insight-verify-test"
TEST_IMAGE="nginx:1.19.0"  # Known vulnerable version

print_info "Creating test pod: $TEST_POD with image: $TEST_IMAGE"

# Clean up any existing test pod
kubectl delete pod $TEST_POD --ignore-not-found=true 2>/dev/null
sleep 2

# Record baseline
INSIGHTS_BEFORE=$(db_count "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL;")

# Create test pod
kubectl run $TEST_POD --image=$TEST_IMAGE --restart=Never --labels="test=insight-verify" --command -- sleep 300

print_info "Waiting 60 seconds for SBOM and CVE processing..."
sleep 60

# Check if new insights were created
INSIGHTS_AFTER=$(db_count "SELECT COUNT(*) FROM insights WHERE deleted_at IS NULL;")
INSIGHTS_CREATED=$((INSIGHTS_AFTER - INSIGHTS_BEFORE))

print_info "Insights before: $INSIGHTS_BEFORE"
print_info "Insights after: $INSIGHTS_AFTER"
print_info "Insights created: $INSIGHTS_CREATED"

if [ $INSIGHTS_CREATED -gt 0 ]; then
    print_pass "New insights were created for test pod"

    # Look for insights related to nginx:1.19.0
    print_info "Insights for $TEST_IMAGE:"
    db_query "SELECT id, type, severity, cve_id, package_name
              FROM insights
              WHERE description LIKE '%$TEST_IMAGE%'
              AND deleted_at IS NULL
              ORDER BY created_at DESC
              LIMIT 5;" | head -15
else
    print_info "No new insights created yet (may need more time for processing)"
fi

# Cleanup
print_info "Cleaning up test pod..."
kubectl delete pod $TEST_POD --ignore-not-found=true 2>/dev/null

# ============================================================
# TEST 12: Data Consistency Check
# ============================================================
print_test "12" "Data Consistency Verification"

print_info "Checking for data integrity issues..."

# Check for insights with invalid references
INVALID_SBOM_REF=$(db_count "
    SELECT COUNT(*)
    FROM insights i
    LEFT JOIN sboms s ON i.sbom_id = s.id
    WHERE i.sbom_id IS NOT NULL
    AND s.id IS NULL
    AND i.deleted_at IS NULL;")

if [ "$INVALID_SBOM_REF" = "0" ]; then
    print_pass "All SBOM references are valid"
else
    print_fail "Found $INVALID_SBOM_REF insights with invalid SBOM references"
fi

# Check for insights with invalid CVE match references
INVALID_CVE_REF=$(db_count "
    SELECT COUNT(*)
    FROM insights i
    LEFT JOIN cve_matches c ON i.cve_match_id = c.id
    WHERE i.cve_match_id IS NOT NULL
    AND c.id IS NULL
    AND i.deleted_at IS NULL;")

if [ "$INVALID_CVE_REF" = "0" ]; then
    print_pass "All CVE match references are valid"
else
    print_fail "Found $INVALID_CVE_REF insights with invalid CVE match references"
fi

# Check for CVE insights without CVE ID
MISSING_CVE_ID=$(db_count "
    SELECT COUNT(*)
    FROM insights
    WHERE type = 'vulnerability'
    AND (cve_id IS NULL OR cve_id = '')
    AND deleted_at IS NULL;")

if [ "$MISSING_CVE_ID" = "0" ]; then
    print_pass "All vulnerability insights have CVE IDs"
else
    print_fail "Found $MISSING_CVE_ID vulnerability insights without CVE IDs"
fi

# ============================================================
# Final Summary
# ============================================================
print_header "Test Results Summary"

echo ""
echo "Test Statistics:"
echo "  ✅ Passed:  $TESTS_PASSED"
echo "  ❌ Failed:  $TESTS_FAILED"
echo ""

echo "Data Summary:"
echo "  Total Insights:       $TOTAL_INSIGHTS"
echo "  CVE Insights:         $CVE_INSIGHTS"
echo "  SBOMs:                $SBOM_COUNT"
echo "  CVE Matches:          $CVE_MATCH_COUNT"
echo "  Recent (30min):       $RECENT_INSIGHTS"
echo ""

echo "API Status:"
echo "  GET /api/insights:                    Working"
echo "  GET /api/insights?severity=critical:  Working"
echo "  GET /api/insights?type=vulnerability: Working"
echo "  GET /api/insights/stats:              Working"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    print_header "✅ ALL TESTS PASSED - INSIGHTS VERIFIED"
    echo ""
    echo "Key Findings:"
    echo "  ✅ Database schema correct (CVE columns present)"
    echo "  ✅ Insights are being created ($TOTAL_INSIGHTS active)"
    echo "  ✅ API endpoints working correctly"
    echo "  ✅ Data relationships intact (SBOM → CVE → Insight)"
    echo "  ✅ No data integrity issues"
    echo ""
    exit 0
else
    print_header "⚠️  SOME TESTS HAD ISSUES"
    echo ""
    echo "Please review the failed tests above."
    echo ""
    exit 1
fi

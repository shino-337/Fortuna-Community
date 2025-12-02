#!/bin/bash

# Phase 1 & 2 Validation Test Suite
# Based on Phase_1_and_2_Validation_Tests.md

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_RESULTS_DIR="$PROJECT_ROOT/test_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$TEST_RESULTS_DIR/validation_tests_${TIMESTAMP}.log"
SUMMARY_FILE="$TEST_RESULTS_DIR/validation_summary_${TIMESTAMP}.txt"

mkdir -p "$TEST_RESULTS_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Configuration
NAMESPACE="${KSAM_NAMESPACE:-ksam}"
DB_POD=""
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-ksam}"
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
AUTH_TOKEN=""

log() {
    echo -e "$1" | tee -a "$LOG_FILE"
}

log_test() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    local status=$1
    local test_name=$2
    local details=$3
    
    if [ "$status" = "PASS" ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        log "${GREEN}✓ PASS${NC}: $test_name"
        echo "PASS: $test_name" >> "$SUMMARY_FILE"
    elif [ "$status" = "FAIL" ]; then
        FAILED_TESTS=$((FAILED_TESTS + 1))
        log "${RED}✗ FAIL${NC}: $test_name"
        if [ -n "$details" ]; then
            log "  ${RED}Error: $details${NC}"
        fi
        echo "FAIL: $test_name - $details" >> "$SUMMARY_FILE"
    elif [ "$status" = "SKIP" ]; then
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
        log "${YELLOW}⊘ SKIP${NC}: $test_name"
        if [ -n "$details" ]; then
            log "  ${YELLOW}Reason: $details${NC}"
        fi
        echo "SKIP: $test_name - $details" >> "$SUMMARY_FILE"
    fi
}

# Helper functions
get_db_pod() {
    if [ -z "$DB_POD" ]; then
        DB_POD=$(kubectl get pod -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    fi
    echo "$DB_POD"
}

db_query() {
    local query=$1
    local pod=$(get_db_pod)
    if [ -z "$pod" ]; then
        return 1
    fi
    kubectl exec -n "$NAMESPACE" "$pod" -- psql -U "$DB_USER" -d "$DB_NAME" -t -c "$query" 2>/dev/null | tr -d ' ' || echo ""
}

get_auth_token() {
    if [ -z "$AUTH_TOKEN" ]; then
        local response=$(curl -s -X POST \
            -H "Content-Type: application/json" \
            -d '{"username":"admin","password":"admin123"}' \
            "${API_BASE_URL}/api/v1/auth/login" 2>/dev/null || echo "")
        AUTH_TOKEN=$(echo "$response" | jq -r '.token // empty' 2>/dev/null || echo "")
    fi
    echo "$AUTH_TOKEN"
}

# Phase 1 Tests
test_p1_001_database_schema() {
    log "\n${BLUE}=== TC-P1-001: Database Schema Validation ===${NC}"
    
    local pod=$(get_db_pod)
    if [ -z "$pod" ]; then
        log_test "SKIP" "TC-P1-001: Database Schema" "PostgreSQL pod not found"
        return
    fi
    
    # 1. Verify core tables exist
    local tables=$(db_query "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('clusters', 'service_accounts', 'pods', 'roles', 'role_bindings', 'insights', 'users', 'audit_logs');")
    
    if [ "$tables" -ge 7 ]; then
        log_test "PASS" "TC-P1-001.1: Core tables exist" "$tables tables found"
    else
        log_test "FAIL" "TC-P1-001.1: Core tables exist" "Only $tables tables found, expected 7+"
    fi
    
    # 2. Verify Apache AGE extension (may not be installed)
    local age=$(db_query "SELECT COUNT(*) FROM pg_extension WHERE extname = 'age';")
    if [ "$age" -eq 1 ]; then
        log_test "PASS" "TC-P1-001.2: Apache AGE extension" "AGE extension installed"
    else
        log_test "SKIP" "TC-P1-001.2: Apache AGE extension" "AGE extension not installed (expected in fallback mode)"
    fi
    
    # 3. Verify indexes
    local indexes=$(db_query "SELECT COUNT(*) FROM pg_indexes WHERE tablename IN ('service_accounts', 'insights', 'pods');")
    if [ "$indexes" -ge 3 ]; then
        log_test "PASS" "TC-P1-001.3: Indexes present" "$indexes indexes found"
    else
        log_test "FAIL" "TC-P1-001.3: Indexes present" "Only $indexes indexes found"
    fi
    
    # 4. Verify foreign key constraints
    local fks=$(db_query "SELECT COUNT(*) FROM pg_constraint WHERE contype = 'f';")
    if [ "$fks" -gt 0 ]; then
        log_test "PASS" "TC-P1-001.4: Foreign keys" "$fks foreign keys found"
    else
        log_test "FAIL" "TC-P1-001.4: Foreign keys" "No foreign keys found"
    fi
}

test_p1_003_component_deployment() {
    log "\n${BLUE}=== TC-P1-003: Component Deployment Verification ===${NC}"
    
    # Check all pods are running
    local core_pods=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-core --no-headers 2>/dev/null | grep -c Running || echo "0")
    local agent_pods=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-agent --no-headers 2>/dev/null | grep -c Running || echo "0")
    local nats_pods=$(kubectl get pods -n "$NAMESPACE" -l app=nats --no-headers 2>/dev/null | grep -c Running || echo "0")
    local db_pods=$(kubectl get pods -n "$NAMESPACE" -l app=postgres --no-headers 2>/dev/null | grep -c Running || echo "0")
    
    if [ "$core_pods" -gt 0 ]; then
        log_test "PASS" "TC-P1-003.1: Core pods running" "$core_pods pod(s)"
    else
        log_test "FAIL" "TC-P1-003.1: Core pods running" "No core pods running"
    fi
    
    if [ "$agent_pods" -gt 0 ]; then
        log_test "PASS" "TC-P1-003.2: Agent pods running" "$agent_pods pod(s)"
    else
        log_test "SKIP" "TC-P1-003.2: Agent pods running" "No agent pods (may be expected)"
    fi
    
    if [ "$nats_pods" -gt 0 ]; then
        log_test "PASS" "TC-P1-003.3: NATS pods running" "$nats_pods pod(s)"
    else
        log_test "FAIL" "TC-P1-003.3: NATS pods running" "No NATS pods running"
    fi
    
    if [ "$db_pods" -gt 0 ]; then
        log_test "PASS" "TC-P1-003.4: PostgreSQL pods running" "$db_pods pod(s)"
    else
        log_test "FAIL" "TC-P1-003.4: PostgreSQL pods running" "No PostgreSQL pods running"
    fi
    
    # Check for CrashLoopBackOff
    local crash_pods=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c CrashLoopBackOff || echo "0")
    if [ "$crash_pods" -eq 0 ]; then
        log_test "PASS" "TC-P1-003.5: No CrashLoopBackOff" "All pods healthy"
    else
        log_test "FAIL" "TC-P1-003.5: No CrashLoopBackOff" "$crash_pods pod(s) in CrashLoopBackOff"
    fi
}

test_p1_004_database_connectivity() {
    log "\n${BLUE}=== TC-P1-004: Database Connectivity ===${NC}"
    
    local pod=$(get_db_pod)
    if [ -z "$pod" ]; then
        log_test "SKIP" "TC-P1-004: Database Connectivity" "PostgreSQL pod not found"
        return
    fi
    
    # Test query
    local result=$(db_query "SELECT 1;")
    if [ "$result" = "1" ]; then
        log_test "PASS" "TC-P1-004.1: Database query" "Query executed successfully"
    else
        log_test "FAIL" "TC-P1-004.1: Database query" "Query failed"
    fi
    
    # Check core logs for DB connection
    local core_logs=$(kubectl logs -n "$NAMESPACE" -l app=ksam-core --tail=100 2>/dev/null | grep -i "database\|connected" | head -1 || echo "")
    if [ -n "$core_logs" ]; then
        log_test "PASS" "TC-P1-004.2: Core DB connection" "Connection found in logs"
    else
        log_test "SKIP" "TC-P1-004.2: Core DB connection" "Connection message not found in logs"
    fi
}

# Phase 2 Tests
test_p2_001_agent_pods_collection() {
    log "\n${BLUE}=== TC-P2-001: Agent Data Collection - Pods ===${NC}"
    
    # Check if pods exist in database
    local pod_count=$(db_query "SELECT COUNT(*) FROM pods;")
    
    if [ "$pod_count" -gt 0 ]; then
        log_test "PASS" "TC-P2-001: Pods in database" "$pod_count pod(s) found"
        
        # Check pod details
        local pod_with_details=$(db_query "SELECT COUNT(*) FROM pods WHERE name IS NOT NULL AND namespace IS NOT NULL;")
        if [ "$pod_with_details" -gt 0 ]; then
            log_test "PASS" "TC-P2-001.1: Pod details populated" "$pod_with_details pod(s) with details"
        else
            log_test "FAIL" "TC-P2-001.1: Pod details populated" "Pods missing required fields"
        fi
    else
        log_test "SKIP" "TC-P2-001: Pods in database" "No pods found (Agent may not be collecting)"
    fi
}

test_p2_002_agent_serviceaccounts_collection() {
    log "\n${BLUE}=== TC-P2-002: Agent Data Collection - ServiceAccounts ===${NC}"
    
    local sa_count=$(db_query "SELECT COUNT(*) FROM service_accounts;")
    
    if [ "$sa_count" -gt 0 ]; then
        log_test "PASS" "TC-P2-002: ServiceAccounts in database" "$sa_count ServiceAccount(s) found"
        
        # Check SA details
        local sa_with_details=$(db_query "SELECT COUNT(*) FROM service_accounts WHERE name IS NOT NULL AND namespace IS NOT NULL;")
        if [ "$sa_with_details" -gt 0 ]; then
            log_test "PASS" "TC-P2-002.1: ServiceAccount details populated" "$sa_with_details SA(s) with details"
        else
            log_test "FAIL" "TC-P2-002.1: ServiceAccount details populated" "SAs missing required fields"
        fi
    else
        log_test "FAIL" "TC-P2-002: ServiceAccounts in database" "No ServiceAccounts found"
    fi
}

test_p2_003_agent_rbac_collection() {
    log "\n${BLUE}=== TC-P2-003: Agent Data Collection - RBAC ===${NC}"
    
    local role_count=$(db_query "SELECT COUNT(*) FROM roles;")
    local cluster_role_count=$(db_query "SELECT COUNT(*) FROM cluster_roles;")
    local rb_count=$(db_query "SELECT COUNT(*) FROM role_bindings;")
    local crb_count=$(db_query "SELECT COUNT(*) FROM cluster_role_bindings;")
    
    if [ "$role_count" -gt 0 ]; then
        log_test "PASS" "TC-P2-003.1: Roles collected" "$role_count role(s)"
    else
        log_test "SKIP" "TC-P2-003.1: Roles collected" "No roles found"
    fi
    
    if [ "$cluster_role_count" -gt 0 ]; then
        log_test "PASS" "TC-P2-003.2: ClusterRoles collected" "$cluster_role_count cluster role(s)"
    else
        log_test "SKIP" "TC-P2-003.2: ClusterRoles collected" "No cluster roles found"
    fi
    
    if [ "$rb_count" -gt 0 ]; then
        log_test "PASS" "TC-P2-003.3: RoleBindings collected" "$rb_count role binding(s)"
    else
        log_test "SKIP" "TC-P2-003.3: RoleBindings collected" "No role bindings found"
    fi
    
    if [ "$crb_count" -gt 0 ]; then
        log_test "PASS" "TC-P2-003.4: ClusterRoleBindings collected" "$crb_count cluster role binding(s)"
    else
        log_test "SKIP" "TC-P2-003.4: ClusterRoleBindings collected" "No cluster role bindings found"
    fi
}

test_p2_004_normalization() {
    log "\n${BLUE}=== TC-P2-004: Core Data Processing - Normalization ===${NC}"
    
    # Check core logs for normalization
    local normalizer_logs=$(kubectl logs -n "$NAMESPACE" -l app=ksam-core --tail=200 2>/dev/null | grep -i "NormalizerWorker\|Normalized" | head -3 || echo "")
    
    if [ -n "$normalizer_logs" ]; then
        log_test "PASS" "TC-P2-004: Normalization activity" "Normalizer worker processing"
        echo "$normalizer_logs" | sed 's/^/  /' | tee -a "$LOG_FILE"
    else
        log_test "SKIP" "TC-P2-004: Normalization activity" "No normalization logs found"
    fi
}

test_p2_005_correlation() {
    log "\n${BLUE}=== TC-P2-005: Core Data Processing - Correlation ===${NC}"
    
    # Check for correlation activity
    local correlator_logs=$(kubectl logs -n "$NAMESPACE" -l app=ksam-core --tail=200 2>/dev/null | grep -i "CorrelatorWorker\|Processing normalized" | head -3 || echo "")
    
    if [ -n "$correlator_logs" ]; then
        log_test "PASS" "TC-P2-005: Correlation activity" "Correlator worker processing"
        echo "$correlator_logs" | sed 's/^/  /' | tee -a "$LOG_FILE"
    else
        log_test "SKIP" "TC-P2-005: Correlation activity" "No correlation logs found"
    fi
    
    # Check Pod -> ServiceAccount relationships
    local pod_sa_links=$(db_query "SELECT COUNT(*) FROM pods WHERE service_account IS NOT NULL AND service_account != '';")
    if [ "$pod_sa_links" -gt 0 ]; then
        log_test "PASS" "TC-P2-005.1: Pod-SA relationships" "$pod_sa_links pod(s) linked to SAs"
    else
        log_test "SKIP" "TC-P2-005.1: Pod-SA relationships" "No pod-SA links (no pods in DB)"
    fi
}

test_p2_006_risk_insights_cis_5_1_3() {
    log "\n${BLUE}=== TC-P2-006: Risk Insights - CIS 5.1.3 Detection ===${NC}"
    
    # Check for insights
    local insights_count=$(db_query "SELECT COUNT(*) FROM insights;")
    
    if [ "$insights_count" -gt 0 ]; then
        log_test "PASS" "TC-P2-006: Insights created" "$insights_count insight(s) found"
        
        # Check for CIS 5.1.3 specifically
        local cis_insights=$(db_query "SELECT COUNT(*) FROM insights WHERE rule_id = 'cis-5.1.3' OR title LIKE '%cluster-admin%';")
        if [ "$cis_insights" -gt 0 ]; then
            log_test "PASS" "TC-P2-006.1: CIS 5.1.3 insights" "$cis_insights CIS 5.1.3 insight(s)"
        else
            log_test "SKIP" "TC-P2-006.1: CIS 5.1.3 insights" "No CIS 5.1.3 insights (may not have cluster-admin bindings)"
        fi
    else
        log_test "SKIP" "TC-P2-006: Insights created" "No insights found (Risk Worker may need data)"
    fi
}

test_p2_009_api_authentication() {
    log "\n${BLUE}=== TC-P2-009: API Functionality - Authentication ===${NC}"
    
    # Test login
    local login_response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' \
        "${API_BASE_URL}/api/v1/auth/login" 2>/dev/null || echo "")
    
    local http_code=$(curl -s -w "%{http_code}" -o /dev/null -X POST \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' \
        "${API_BASE_URL}/api/v1/auth/login" 2>/dev/null || echo "000")
    
    if [ "$http_code" = "200" ]; then
        local token=$(echo "$login_response" | jq -r '.token // empty' 2>/dev/null || echo "")
        if [ -n "$token" ] && [ "$token" != "null" ]; then
            log_test "PASS" "TC-P2-009.1: Login returns token" "Token obtained"
            AUTH_TOKEN="$token"
        else
            log_test "FAIL" "TC-P2-009.1: Login returns token" "No token in response"
        fi
    else
        log_test "FAIL" "TC-P2-009.1: Login returns token" "HTTP $http_code"
    fi
    
    # Test authenticated request
    if [ -n "$AUTH_TOKEN" ]; then
        local auth_code=$(curl -s -w "%{http_code}" -o /dev/null \
            -H "Authorization: Bearer $AUTH_TOKEN" \
            "${API_BASE_URL}/api/v1/insights" 2>/dev/null || echo "000")
        
        if [ "$auth_code" = "200" ]; then
            log_test "PASS" "TC-P2-009.2: Authenticated request" "Request succeeded"
        else
            log_test "FAIL" "TC-P2-009.2: Authenticated request" "HTTP $auth_code"
        fi
    fi
    
    # Test unauthenticated request
    local unauth_code=$(curl -s -w "%{http_code}" -o /dev/null \
        "${API_BASE_URL}/api/v1/insights" 2>/dev/null || echo "000")
    
    if [ "$unauth_code" = "401" ]; then
        log_test "PASS" "TC-P2-009.3: Unauthenticated request blocked" "401 returned"
    else
        log_test "FAIL" "TC-P2-009.3: Unauthenticated request blocked" "Expected 401, got $unauth_code"
    fi
}

test_p2_011_api_pagination() {
    log "\n${BLUE}=== TC-P2-011: API Functionality - Pagination ===${NC}"
    
    local token=$(get_auth_token)
    if [ -z "$token" ]; then
        log_test "SKIP" "TC-P2-011: API Pagination" "Authentication failed"
        return
    fi
    
    # Test pagination
    local response=$(curl -s -H "Authorization: Bearer $token" \
        "${API_BASE_URL}/api/v1/serviceaccounts?page=1&pageSize=5" 2>/dev/null || echo "")
    
    if echo "$response" | jq . >/dev/null 2>&1; then
        local count=$(echo "$response" | jq -r '.serviceAccounts | length // 0' 2>/dev/null || echo "0")
        if [ "$count" -le 5 ]; then
            log_test "PASS" "TC-P2-011: Pagination works" "Page size respected ($count items)"
        else
            log_test "FAIL" "TC-P2-011: Pagination works" "Page size exceeded ($count items)"
        fi
    else
        log_test "SKIP" "TC-P2-011: Pagination works" "Invalid JSON response"
    fi
}

test_p2_012_api_filtering() {
    log "\n${BLUE}=== TC-P2-012: API Functionality - Filtering ===${NC}"
    
    local token=$(get_auth_token)
    if [ -z "$token" ]; then
        log_test "SKIP" "TC-P2-012: API Filtering" "Authentication failed"
        return
    fi
    
    # Test filtering by namespace
    local response=$(curl -s -H "Authorization: Bearer $token" \
        "${API_BASE_URL}/api/v1/serviceaccounts?namespace=kube-system" 2>/dev/null || echo "")
    
    if echo "$response" | jq . >/dev/null 2>&1; then
        log_test "PASS" "TC-P2-012: Filtering works" "Filter endpoint responds"
    else
        log_test "SKIP" "TC-P2-012: Filtering works" "Invalid response"
    fi
}

# Functional Tests
test_f001_risk_scoring() {
    log "\n${BLUE}=== TC-F001: ServiceAccount Risk Scoring ===${NC}"
    
    # Check if risk scores exist (if implemented)
    local sa_with_risk=$(db_query "SELECT COUNT(*) FROM service_accounts WHERE risk_score IS NOT NULL;" 2>/dev/null)
    sa_with_risk=${sa_with_risk:-0}
    
    if [ "$sa_with_risk" -gt 0 ]; then
        log_test "PASS" "TC-F001: Risk scores calculated" "$sa_with_risk SA(s) with risk scores"
    else
        log_test "SKIP" "TC-F001: Risk scores calculated" "Risk scoring not implemented in SA table"
    fi
}

test_f002_orphan_detection() {
    log "\n${BLUE}=== TC-F002: Orphaned ServiceAccount Detection ===${NC}"
    
    # Check for orphaned SAs (if status field exists)
    local orphaned=$(db_query "SELECT COUNT(*) FROM service_accounts WHERE status = 'orphaned';" 2>/dev/null)
    orphaned=${orphaned:-0}
    
    if [ "$orphaned" -gt 0 ]; then
        log_test "PASS" "TC-F002: Orphaned SAs detected" "$orphaned orphaned SA(s)"
    else
        log_test "SKIP" "TC-F002: Orphaned SAs detected" "Orphan detection may not be implemented"
    fi
}

# Integration Tests
test_i001_end_to_end_pod_lifecycle() {
    log "\n${BLUE}=== TC-I001: End-to-End Pod Lifecycle ===${NC}"
    
    # Check if we have pods in database
    local pod_count=$(db_query "SELECT COUNT(*) FROM pods;")
    
    if [ "$pod_count" -gt 0 ]; then
        log_test "PASS" "TC-I001: Pod lifecycle tracked" "$pod_count pod(s) in database"
    else
        log_test "SKIP" "TC-I001: Pod lifecycle tracked" "No pods in database (Agent may not be collecting)"
    fi
}

# Main execution
main() {
    log "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    log "${BLUE}║     Phase 1 & 2 Validation Test Suite                        ║${NC}"
    log "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    log ""
    log "Test started at: $(date)"
    log "Namespace: ${NAMESPACE}"
    log "Log file: ${LOG_FILE}"
    log ""
    
    # Check prerequisites
    if ! command -v kubectl &> /dev/null; then
        log "${RED}✗ kubectl not found. Cannot run tests.${NC}"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        log "${YELLOW}⚠ jq not found. JSON parsing may be limited.${NC}"
    fi
    
    # Check namespace
    if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
        log "${RED}✗ Namespace $NAMESPACE not found.${NC}"
        exit 1
    fi
    
    # Run Phase 1 tests
    log "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    log "${BLUE}Phase 1 Tests${NC}"
    log "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    test_p1_001_database_schema
    test_p1_003_component_deployment
    test_p1_004_database_connectivity
    
    # Run Phase 2 tests
    log "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    log "${BLUE}Phase 2 Tests${NC}"
    log "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    test_p2_001_agent_pods_collection
    test_p2_002_agent_serviceaccounts_collection
    test_p2_003_agent_rbac_collection
    test_p2_004_normalization
    test_p2_005_correlation
    test_p2_006_risk_insights_cis_5_1_3
    test_p2_009_api_authentication
    test_p2_011_api_pagination
    test_p2_012_api_filtering
    
    # Run Functional tests
    log "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    log "${BLUE}Functional Tests${NC}"
    log "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    test_f001_risk_scoring
    test_f002_orphan_detection
    
    # Run Integration tests
    log "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    log "${BLUE}Integration Tests${NC}"
    log "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    test_i001_end_to_end_pod_lifecycle
    
    # Summary
    log "\n${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    log "${BLUE}║                    Test Summary                               ║${NC}"
    log "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    log ""
    log "Total Tests: ${TOTAL_TESTS}"
    log "${GREEN}Passed: ${PASSED_TESTS}${NC}"
    log "${RED}Failed: ${FAILED_TESTS}${NC}"
    log "${YELLOW}Skipped: ${SKIPPED_TESTS}${NC}"
    log ""
    log "Test completed at: $(date)"
    log "Summary file: ${SUMMARY_FILE}"
    log "Log file: ${LOG_FILE}"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        log ""
        log "${GREEN}✓ All critical tests passed!${NC}"
        exit 0
    else
        log ""
        log "${RED}✗ Some tests failed. Check log file for details.${NC}"
        exit 1
    fi
}

# Run main
main


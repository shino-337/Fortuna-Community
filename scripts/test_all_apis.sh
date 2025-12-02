#!/bin/bash

# Comprehensive API Test Suite for KSAM
# Tests all API endpoints and data flow

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_RESULTS_DIR="$PROJECT_ROOT/test_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$TEST_RESULTS_DIR/test_all_apis_${TIMESTAMP}.log"

mkdir -p "$TEST_RESULTS_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# API Configuration
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
API_VERSION="v1"
FULL_API_URL="${API_BASE_URL}/api/${API_VERSION}"

# Authentication
AUTH_TOKEN=""
ADMIN_USERNAME="${KSAM_ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${KSAM_ADMIN_PASSWORD:-admin123}"

log() {
    echo -e "$1" | tee -a "$LOG_FILE"
}

log_test() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if [ $1 -eq 0 ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        log "${GREEN}✓ PASS${NC}: $2"
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        log "${RED}✗ FAIL${NC}: $2"
        if [ -n "$3" ]; then
            log "  Error: $3"
        fi
    fi
}

# Test helper functions
test_endpoint() {
    local method=$1
    local endpoint=$2
    local expected_status=$3
    local description=$4
    local data=$5
    local headers=$6
    
    local url="${FULL_API_URL}${endpoint}"
    local cmd="curl -s -w '\n%{http_code}' -X ${method}"
    
    if [ -n "$AUTH_TOKEN" ]; then
        cmd="$cmd -H 'Authorization: Bearer ${AUTH_TOKEN}'"
    fi
    
    if [ -n "$headers" ]; then
        cmd="$cmd $headers"
    fi
    
    if [ -n "$data" ]; then
        cmd="$cmd -H 'Content-Type: application/json' -d '$data'"
    fi
    
    cmd="$cmd '${url}'"
    
    local response=$(eval $cmd)
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" -eq "$expected_status" ]; then
        log_test 0 "$description"
        echo "$body" | jq . 2>/dev/null || echo "$body" | tee -a "$LOG_FILE"
        return 0
    else
        log_test 1 "$description" "Expected $expected_status, got $http_code. Response: $body"
        return 1
    fi
}

# Check if API is accessible
check_api_health() {
    log "\n${BLUE}=== API Health Check ===${NC}"
    
    local health=$(curl -s "${API_BASE_URL}/ready" || echo "FAILED")
    if [ "$health" != "FAILED" ]; then
        log_test 0 "API health check"
    else
        log_test 1 "API health check" "API not accessible at ${API_BASE_URL}"
        log "${RED}API is not accessible. Please ensure Core service is running.${NC}"
        exit 1
    fi
}

# Authentication Tests
test_authentication() {
    log "\n${BLUE}=== Authentication Tests ===${NC}"
    
    # Test login
    local login_data="{\"username\":\"${ADMIN_USERNAME}\",\"password\":\"${ADMIN_PASSWORD}\"}"
    local login_response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$login_data" \
        "${FULL_API_URL}/auth/login")
    
    local http_code=$(curl -s -w "%{http_code}" -o /dev/null -X POST \
        -H "Content-Type: application/json" \
        -d "$login_data" \
        "${FULL_API_URL}/auth/login")
    
    if [ "$http_code" -eq 200 ]; then
        AUTH_TOKEN=$(echo "$login_response" | jq -r '.token // empty' 2>/dev/null)
        if [ -n "$AUTH_TOKEN" ] && [ "$AUTH_TOKEN" != "null" ]; then
            log_test 0 "Login with valid credentials"
            log "  Token obtained: ${AUTH_TOKEN:0:20}..."
        else
            log_test 1 "Login with valid credentials" "No token in response"
        fi
    else
        log_test 1 "Login with valid credentials" "HTTP $http_code"
    fi
    
    # Test login with invalid credentials
    local invalid_data="{\"username\":\"invalid\",\"password\":\"invalid\"}"
    local invalid_code=$(curl -s -w "%{http_code}" -o /dev/null -X POST \
        -H "Content-Type: application/json" \
        -d "$invalid_data" \
        "${FULL_API_URL}/auth/login")
    
    if [ "$invalid_code" -eq 401 ] || [ "$invalid_code" -eq 403 ]; then
        log_test 0 "Login with invalid credentials (should fail)"
    else
        log_test 1 "Login with invalid credentials" "Expected 401/403, got $invalid_code"
    fi
    
    # Test protected endpoint without token
    local protected_code=$(curl -s -w "%{http_code}" -o /dev/null \
        "${FULL_API_URL}/insights")
    
    if [ "$protected_code" -eq 401 ]; then
        log_test 0 "Protected endpoint without token (should fail)"
    else
        log_test 1 "Protected endpoint without token" "Expected 401, got $protected_code"
    fi
}

# Clusters API Tests
test_clusters_api() {
    log "\n${BLUE}=== Clusters API Tests ===${NC}"
    
    test_endpoint "GET" "/clusters" 200 "Get all clusters"
    test_endpoint "GET" "/clusters/stats" 200 "Get clusters statistics"
    test_endpoint "GET" "/clusters/default" 200 "Get specific cluster" || \
    test_endpoint "GET" "/clusters/default" 404 "Get non-existent cluster (expected 404)"
}

# ServiceAccounts API Tests
test_serviceaccounts_api() {
    log "\n${BLUE}=== ServiceAccounts API Tests ===${NC}"
    
    test_endpoint "GET" "/serviceaccounts" 200 "Get all service accounts"
    test_endpoint "GET" "/serviceaccounts?cluster=default" 200 "Get service accounts by cluster"
    test_endpoint "GET" "/serviceaccounts?namespace=kube-system" 200 "Get service accounts by namespace"
    test_endpoint "GET" "/serviceaccounts?page=1&pageSize=10" 200 "Get service accounts with pagination"
    
    # Get first service account ID for detail test
    local sa_list=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
        "${FULL_API_URL}/serviceaccounts?pageSize=1")
    local sa_id=$(echo "$sa_list" | jq -r '.serviceAccounts[0].id // empty' 2>/dev/null)
    
    if [ -n "$sa_id" ] && [ "$sa_id" != "null" ]; then
        test_endpoint "GET" "/serviceaccounts/${sa_id}" 200 "Get specific service account"
        test_endpoint "GET" "/serviceaccounts/${sa_id}/permissions" 200 "Get service account permissions"
    else
        log "${YELLOW}⚠ No service accounts found for detail test${NC}"
    fi
}

# Insights API Tests
test_insights_api() {
    log "\n${BLUE}=== Insights API Tests ===${NC}"
    
    test_endpoint "GET" "/insights" 200 "Get all insights"
    test_endpoint "GET" "/insights/summary" 200 "Get insights summary"
    test_endpoint "GET" "/insights?severity=high" 200 "Get insights by severity"
    test_endpoint "GET" "/insights?type=cluster-admin" 200 "Get insights by type"
    test_endpoint "GET" "/insights?cluster=default" 200 "Get insights by cluster"
    test_endpoint "GET" "/insights?page=1&pageSize=10" 200 "Get insights with pagination"
    
    # Trigger risk evaluation
    test_endpoint "POST" "/insights/evaluate" 200 "Trigger risk evaluation"
    
    # Get first insight ID for detail test
    local insights_list=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
        "${FULL_API_URL}/insights?pageSize=1")
    local insight_id=$(echo "$insights_list" | jq -r '.insights[0].id // empty' 2>/dev/null)
    
    if [ -n "$insight_id" ] && [ "$insight_id" != "null" ]; then
        test_endpoint "GET" "/insights/${insight_id}" 200 "Get specific insight"
    else
        log "${YELLOW}⚠ No insights found for detail test${NC}"
    fi
}

# Graph API Tests
test_graph_api() {
    log "\n${BLUE}=== Graph API Tests ===${NC}"
    
    test_endpoint "GET" "/graph" 200 "Get graph data"
    test_endpoint "GET" "/graph?cluster=default" 200 "Get graph data by cluster"
    test_endpoint "GET" "/graph?namespace=kube-system" 200 "Get graph data by namespace"
    
    # Test blast radius (may fail if no resources)
    test_endpoint "GET" "/graph/blast-radius/test-id?max_depth=3" 200 "Get blast radius" || \
    test_endpoint "GET" "/graph/blast-radius/test-id?max_depth=3" 404 "Get blast radius (no resource)" || \
    test_endpoint "GET" "/graph/blast-radius/test-id?max_depth=3" 503 "Get blast radius (AGE not available)"
    
    # Test shortest path
    test_endpoint "GET" "/graph/shortest-path?from=test1&to=test2" 200 "Get shortest path" || \
    test_endpoint "GET" "/graph/shortest-path?from=test1&to=test2" 400 "Get shortest path (invalid params)" || \
    test_endpoint "GET" "/graph/shortest-path?from=test1&to=test2" 503 "Get shortest path (AGE not available)"
    
    # Test accessible resources
    test_endpoint "GET" "/graph/accessible/test-id?type=secrets" 200 "Get accessible resources" || \
    test_endpoint "GET" "/graph/accessible/test-id?type=secrets" 503 "Get accessible resources (AGE not available)"
    
    # Test custom Cypher query
    local query_data='{"query":"MATCH (n) RETURN n LIMIT 10","params":{}}'
    test_endpoint "POST" "/graph/query" 200 "Execute custom Cypher query" "$query_data" || \
    test_endpoint "POST" "/graph/query" 503 "Execute custom Cypher query (AGE not available)" "$query_data"
}

# Audit API Tests
test_audit_api() {
    log "\n${BLUE}=== Audit API Tests ===${NC}"
    
    test_endpoint "GET" "/audit" 200 "Get audit logs"
    test_endpoint "GET" "/audit?cluster=default" 200 "Get audit logs by cluster"
    test_endpoint "GET" "/audit?resource=serviceaccount" 200 "Get audit logs by resource"
    test_endpoint "GET" "/audit?action=create" 200 "Get audit logs by action"
    test_endpoint "GET" "/audit?page=1&pageSize=10" 200 "Get audit logs with pagination"
    test_endpoint "GET" "/audit/reports" 200 "Get audit reports"
}

# Deployments API Tests
test_deployments_api() {
    log "\n${BLUE}=== Deployments API Tests ===${NC}"
    
    test_endpoint "GET" "/deployments" 200 "Get all deployments"
    test_endpoint "GET" "/deployments?cluster=default" 200 "Get deployments by cluster"
    test_endpoint "GET" "/deployments?namespace=kube-system" 200 "Get deployments by namespace"
    
    # Get first deployment ID for detail test
    local deployments_list=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
        "${FULL_API_URL}/deployments?pageSize=1")
    local deployment_id=$(echo "$deployments_list" | jq -r '.deployments[0].id // empty' 2>/dev/null)
    
    if [ -n "$deployment_id" ] && [ "$deployment_id" != "null" ]; then
        test_endpoint "GET" "/deployments/${deployment_id}" 200 "Get specific deployment"
    else
        log "${YELLOW}⚠ No deployments found for detail test${NC}"
    fi
}

# Data Flow Tests
test_data_flow() {
    log "\n${BLUE}=== Data Flow Tests ===${NC}"
    
    # Check if Agent is sending data
    log "Checking Agent status..."
    local agent_pods=$(kubectl get pods -n ksam -l app=ksam-agent --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$agent_pods" -gt 0 ]; then
        log_test 0 "Agent pods running ($agent_pods pods)"
    else
        log_test 1 "Agent pods running" "No agent pods found"
    fi
    
    # Check NATS streams
    log "Checking NATS streams..."
    local nats_pods=$(kubectl get pods -n ksam -l app=nats --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$nats_pods" -gt 0 ]; then
        log_test 0 "NATS pods running ($nats_pods pods)"
    else
        log_test 1 "NATS pods running" "No NATS pods found"
    fi
    
    # Check database connectivity
    log "Checking database..."
    local db_pods=$(kubectl get pods -n ksam -l app=postgres --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$db_pods" -gt 0 ]; then
        log_test 0 "PostgreSQL pods running ($db_pods pods)"
    else
        log_test 1 "PostgreSQL pods running" "No PostgreSQL pods found"
    fi
    
    # Check Core workers
    log "Checking Core workers..."
    local core_logs=$(kubectl logs -n ksam -l app=ksam-core --tail=100 2>/dev/null | grep -i "WorkerPool\|RiskWorker\|CorrelatorWorker" | head -5 || echo "")
    if [ -n "$core_logs" ]; then
        log_test 0 "Core workers active"
        echo "$core_logs" | head -3 | sed 's/^/  /' | tee -a "$LOG_FILE"
    else
        log_test 1 "Core workers active" "No worker activity found in logs"
    fi
}

# Risk Engine Flow Tests
test_risk_engine_flow() {
    log "\n${BLUE}=== Risk Engine Flow Tests ===${NC}"
    
    # Check if Risk Worker is processing
    log "Checking Risk Worker activity..."
    local risk_logs=$(kubectl logs -n ksam -l app=ksam-core --tail=200 2>/dev/null | \
        grep -i "RiskWorker\|Evaluating risks" | head -5 || echo "")
    
    if [ -n "$risk_logs" ]; then
        log_test 0 "Risk Worker processing messages"
        echo "$risk_logs" | head -3 | sed 's/^/  /' | tee -a "$LOG_FILE"
    else
        log_test 1 "Risk Worker processing messages" "No risk evaluation activity found"
    fi
    
    # Check insights creation
    log "Checking insights in database..."
    local insights_count=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
        "${FULL_API_URL}/insights/summary" | jq -r '.total // 0' 2>/dev/null || echo "0")
    
    if [ "$insights_count" -gt 0 ]; then
        log_test 0 "Insights created ($insights_count total)"
    else
        log_test 1 "Insights created" "No insights found in database"
    fi
}

# Main execution
main() {
    log "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    log "${BLUE}║         KSAM Comprehensive API Test Suite                    ║${NC}"
    log "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    log ""
    log "Test started at: $(date)"
    log "API Base URL: ${API_BASE_URL}"
    log "Log file: ${LOG_FILE}"
    log ""
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        log "${YELLOW}⚠ kubectl not found. Some tests will be skipped.${NC}"
    fi
    
    # Check if jq is available
    if ! command -v jq &> /dev/null; then
        log "${YELLOW}⚠ jq not found. JSON parsing may be limited.${NC}"
    fi
    
    # Run tests
    check_api_health
    test_authentication
    
    if [ -z "$AUTH_TOKEN" ]; then
        log "${RED}⚠ Authentication failed. Some tests will be skipped.${NC}"
    else
        test_clusters_api
        test_serviceaccounts_api
        test_insights_api
        test_graph_api
        test_audit_api
        test_deployments_api
        test_data_flow
        test_risk_engine_flow
    fi
    
    # Summary
    log "\n${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    log "${BLUE}║                    Test Summary                               ║${NC}"
    log "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    log ""
    log "Total Tests: ${TOTAL_TESTS}"
    log "${GREEN}Passed: ${PASSED_TESTS}${NC}"
    log "${RED}Failed: ${FAILED_TESTS}${NC}"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        log ""
        log "${GREEN}✓ All tests passed!${NC}"
        exit 0
    else
        log ""
        log "${RED}✗ Some tests failed. Check log file for details.${NC}"
        exit 1
    fi
}

# Run main
main


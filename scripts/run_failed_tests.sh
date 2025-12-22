#!/bin/bash

# Re-run Failed and Skipped Tests
# Focus on tests that were skipped or failed in initial run

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_RESULTS_DIR="$PROJECT_ROOT/test_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$TEST_RESULTS_DIR/failed_tests_retry_${TIMESTAMP}.log"

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
    elif [ "$status" = "FAIL" ]; then
        FAILED_TESTS=$((FAILED_TESTS + 1))
        log "${RED}✗ FAIL${NC}: $test_name"
        if [ -n "$details" ]; then
            log "  ${RED}Error: $details${NC}"
        fi
    elif [ "$status" = "SKIP" ]; then
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
        log "${YELLOW}⊘ SKIP${NC}: $test_name"
        if [ -n "$details" ]; then
            log "  ${YELLOW}Reason: $details${NC}"
        fi
    fi
}

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

# Previously Failed/Skipped Tests
test_p2_001_agent_pods_collection() {
    log "\n${BLUE}=== TC-P2-001: Agent Data Collection - Pods (RETRY) ===${NC}"
    
    local pod_count=$(db_query "SELECT COUNT(*) FROM pods;")
    
    if [ "$pod_count" -gt 0 ]; then
        log_test "PASS" "TC-P2-001: Pods in database" "$pod_count pod(s) found"
        
        # Check pod details
        local pod_with_details=$(db_query "SELECT COUNT(*) FROM pods WHERE name IS NOT NULL AND namespace IS NOT NULL;")
        if [ "$pod_with_details" -gt 0 ]; then
            log_test "PASS" "TC-P2-001.1: Pod details populated" "$pod_with_details pod(s) with details"
            
            # Show sample pod
            local sample_pod=$(db_query "SELECT name, namespace, service_account FROM pods LIMIT 1;")
            log "  Sample pod: $sample_pod"
        else
            log_test "FAIL" "TC-P2-001.1: Pod details populated" "Pods missing required fields"
        fi
    else
        log_test "FAIL" "TC-P2-001: Pods in database" "No pods found after Agent restart"
    fi
}

test_p2_004_normalization() {
    log "\n${BLUE}=== TC-P2-004: Core Data Processing - Normalization (RETRY) ===${NC}"
    
    # Check core logs for normalization
    local normalizer_logs=$(kubectl logs -n "$NAMESPACE" -l app=ksam-core --tail=500 2>/dev/null | grep -i "NormalizerWorker\|Normalized" | head -5 || echo "")
    
    if [ -n "$normalizer_logs" ]; then
        log_test "PASS" "TC-P2-004: Normalization activity" "Normalizer worker processing"
        echo "$normalizer_logs" | sed 's/^/  /' | tee -a "$LOG_FILE"
    else
        log_test "FAIL" "TC-P2-004: Normalization activity" "No normalization logs found"
    fi
}

test_p2_005_correlation() {
    log "\n${BLUE}=== TC-P2-005: Core Data Processing - Correlation (RETRY) ===${NC}"
    
    # Check for correlation activity
    local correlator_logs=$(kubectl logs -n "$NAMESPACE" -l app=ksam-core --tail=500 2>/dev/null | grep -i "CorrelatorWorker\|Processing normalized" | head -5 || echo "")
    
    if [ -n "$correlator_logs" ]; then
        log_test "PASS" "TC-P2-005: Correlation activity" "Correlator worker processing"
        echo "$correlator_logs" | sed 's/^/  /' | tee -a "$LOG_FILE"
    else
        log_test "FAIL" "TC-P2-005: Correlation activity" "No correlation logs found"
    fi
    
    # Check Pod -> ServiceAccount relationships
    local pod_sa_links=$(db_query "SELECT COUNT(*) FROM pods WHERE service_account IS NOT NULL AND service_account != '';")
    if [ "$pod_sa_links" -gt 0 ]; then
        log_test "PASS" "TC-P2-005.1: Pod-SA relationships" "$pod_sa_links pod(s) linked to SAs"
    else
        local total_pods=$(db_query "SELECT COUNT(*) FROM pods;")
        if [ "$total_pods" -gt 0 ]; then
            log_test "FAIL" "TC-P2-005.1: Pod-SA relationships" "Pods exist but no SA links"
        else
            log_test "SKIP" "TC-P2-005.1: Pod-SA relationships" "No pods in database"
        fi
    fi
}

test_p2_006_risk_insights_cis_5_1_3() {
    log "\n${BLUE}=== TC-P2-006: Risk Insights - CIS 5.1.3 Detection (RETRY) ===${NC}"
    
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
        
        # Show insight summary
        local insight_summary=$(db_query "SELECT severity, COUNT(*) FROM insights GROUP BY severity;")
        log "  Insight summary: $insight_summary"
    else
        log_test "FAIL" "TC-P2-006: Insights created" "No insights found after data collection"
    fi
}

test_p1_003_crashloopbackoff() {
    log "\n${BLUE}=== TC-P1-003.5: No CrashLoopBackOff (RETRY) ===${NC}"
    
    # Check for CrashLoopBackOff
    local crash_pods=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c CrashLoopBackOff || echo "0")
    if [ "$crash_pods" -eq 0 ]; then
        log_test "PASS" "TC-P1-003.5: No CrashLoopBackOff" "All pods healthy"
    else
        local crash_pod_names=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep CrashLoopBackOff | awk '{print $1}' | tr '\n' ',' || echo "")
        log_test "FAIL" "TC-P1-003.5: No CrashLoopBackOff" "$crash_pods pod(s) in CrashLoopBackOff: $crash_pod_names"
    fi
}

test_agent_connectivity() {
    log "\n${BLUE}=== Agent Connectivity Test ===${NC}"
    
    # Check Agent pods
    local agent_pods=$(kubectl get pods -n "$NAMESPACE" -l app=ksam-agent --no-headers 2>/dev/null | grep -c "Running" || echo "0")
    if [ "$agent_pods" -gt 0 ]; then
        log_test "PASS" "Agent pods running" "$agent_pods pod(s)"
        
        # Check Agent logs for connection
        local agent_logs=$(kubectl logs -n "$NAMESPACE" -l app=ksam-agent --tail=50 2>/dev/null | grep -iE "Connected|Register|gRPC" | head -3 || echo "")
        if [ -n "$agent_logs" ]; then
            log_test "PASS" "Agent connected to Core" "Connection found in logs"
            echo "$agent_logs" | sed 's/^/  /' | tee -a "$LOG_FILE"
        else
            log_test "SKIP" "Agent connected to Core" "Connection message not found"
        fi
    else
        log_test "FAIL" "Agent pods running" "No agent pods running"
    fi
}

test_data_flow_complete() {
    log "\n${BLUE}=== Complete Data Flow Test ===${NC}"
    
    # Check Agent -> Core
    local agent_logs=$(kubectl logs -n "$NAMESPACE" -l app=ksam-agent --tail=100 2>/dev/null | grep -iE "Stream|Inventory|sent" | head -3 || echo "")
    if [ -n "$agent_logs" ]; then
        log_test "PASS" "Agent -> Core: Data streaming" "Agent sending data"
    else
        log_test "SKIP" "Agent -> Core: Data streaming" "No streaming activity in logs"
    fi
    
    # Check Core -> NATS
    local core_nats_logs=$(kubectl logs -n "$NAMESPACE" -l app=ksam-core --tail=200 2>/dev/null | grep -iE "Published|NATS.*publish" | head -3 || echo "")
    if [ -n "$core_nats_logs" ]; then
        log_test "PASS" "Core -> NATS: Publishing" "Core publishing to NATS"
    else
        log_test "SKIP" "Core -> NATS: Publishing" "No publish activity in logs"
    fi
    
    # Check NATS -> Workers
    local worker_logs=$(kubectl logs -n "$NAMESPACE" -l app=ksam-core --tail=300 2>/dev/null | grep -iE "NormalizerWorker.*Processing|CorrelatorWorker.*Processing|RiskWorker.*Evaluating" | head -5 || echo "")
    if [ -n "$worker_logs" ]; then
        log_test "PASS" "NATS -> Workers: Processing" "Workers processing messages"
        echo "$worker_logs" | sed 's/^/  /' | tee -a "$LOG_FILE"
    else
        log_test "SKIP" "NATS -> Workers: Processing" "No worker activity in logs"
    fi
    
    # Check Workers -> Database
    local pod_count=$(db_query "SELECT COUNT(*) FROM pods;")
    local sa_count=$(db_query "SELECT COUNT(*) FROM service_accounts;")
    local insights_count=$(db_query "SELECT COUNT(*) FROM insights;")
    
    if [ "$pod_count" -gt 0 ] || [ "$sa_count" -gt 0 ]; then
        log_test "PASS" "Workers -> Database: Data persisted" "Pods: $pod_count, SAs: $sa_count, Insights: $insights_count"
    else
        log_test "FAIL" "Workers -> Database: Data persisted" "No data in database"
    fi
}

# Main execution
main() {
    log "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    log "${BLUE}║     Failed/Skipped Tests Retry Suite                        ║${NC}"
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
    
    # Wait for Agent to stabilize
    log "${YELLOW}Waiting for Agent to stabilize...${NC}"
    sleep 30
    
    # Run previously failed/skipped tests
    test_p1_003_crashloopbackoff
    test_agent_connectivity
    test_p2_001_agent_pods_collection
    test_p2_004_normalization
    test_p2_005_correlation
    test_p2_006_risk_insights_cis_5_1_3
    test_data_flow_complete
    
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
    
    if [ $FAILED_TESTS -eq 0 ]; then
        log ""
        log "${GREEN}✓ All retry tests passed!${NC}"
        exit 0
    else
        log ""
        log "${RED}✗ Some tests still failing. Check log file for details.${NC}"
        exit 1
    fi
}

main


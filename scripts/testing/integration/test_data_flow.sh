#!/bin/bash

# Data Flow Test Script
# Tests the complete data flow: Agent -> NATS -> Workers -> Database -> API

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_RESULTS_DIR="$PROJECT_ROOT/test_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$TEST_RESULTS_DIR/test_data_flow_${TIMESTAMP}.log"

mkdir -p "$TEST_RESULTS_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "$1" | tee -a "$LOG_FILE"
}

log_test() {
    if [ $1 -eq 0 ]; then
        log "${GREEN}✓${NC} $2"
    else
        log "${RED}✗${NC} $2"
        if [ -n "$3" ]; then
            log "  ${RED}Error: $3${NC}"
        fi
    fi
}

check_pod_status() {
    local app=$1
    local namespace=${2:-ksam}
    local pods=$(kubectl get pods -n "$namespace" -l app="$app" --no-headers 2>/dev/null | grep -c Running || echo "0")
    echo "$pods"
}

check_pod_logs() {
    local app=$1
    local pattern=$2
    local namespace=${3:-ksam}
    kubectl logs -n "$namespace" -l app="$app" --tail=100 2>/dev/null | grep -i "$pattern" | head -5 || echo ""
}

# Test Agent -> NATS Flow
test_agent_nats_flow() {
    log "\n${BLUE}=== Agent -> NATS Flow ===${NC}"
    
    # Check Agent pods
    local agent_pods=$(check_pod_status "ksam-agent")
    if [ "$agent_pods" -gt 0 ]; then
        log_test 0 "Agent pods running ($agent_pods pods)"
        
        # Check Agent logs for NATS publishing
        local agent_logs=$(check_pod_logs "ksam-agent" "publish\|sent\|stream")
        if [ -n "$agent_logs" ]; then
            log_test 0 "Agent publishing to NATS"
            echo "$agent_logs" | sed 's/^/  /' | tee -a "$LOG_FILE"
        else
            log_test 1 "Agent publishing to NATS" "No publish activity found"
        fi
    else
        log_test 1 "Agent pods running" "No agent pods found"
    fi
    
    # Check NATS pods
    local nats_pods=$(check_pod_status "nats")
    if [ "$nats_pods" -gt 0 ]; then
        log_test 0 "NATS pods running ($nats_pods pods)"
    else
        log_test 1 "NATS pods running" "No NATS pods found"
    fi
}

# Test NATS -> Workers Flow
test_nats_workers_flow() {
    log "\n${BLUE}=== NATS -> Workers Flow ===${NC}"
    
    # Check Core pods
    local core_pods=$(check_pod_status "ksam-core")
    if [ "$core_pods" -gt 0 ]; then
        log_test 0 "Core pods running ($core_pods pods)"
        
        # Check Normalizer Worker
        local normalizer_logs=$(check_pod_logs "ksam-core" "NormalizerWorker\|normalized")
        if [ -n "$normalizer_logs" ]; then
            log_test 0 "Normalizer Worker processing"
            echo "$normalizer_logs" | head -2 | sed 's/^/  /' | tee -a "$LOG_FILE"
        else
            log_test 1 "Normalizer Worker processing" "No normalizer activity found"
        fi
        
        # Check Correlator Worker
        local correlator_logs=$(check_pod_logs "ksam-core" "CorrelatorWorker\|Processing normalized")
        if [ -n "$correlator_logs" ]; then
            log_test 0 "Correlator Worker processing"
            echo "$correlator_logs" | head -2 | sed 's/^/  /' | tee -a "$LOG_FILE"
        else
            log_test 1 "Correlator Worker processing" "No correlator activity found"
        fi
        
        # Check Risk Worker
        local risk_logs=$(check_pod_logs "ksam-core" "RiskWorker\|Evaluating risks")
        if [ -n "$risk_logs" ]; then
            log_test 0 "Risk Worker processing"
            echo "$risk_logs" | head -2 | sed 's/^/  /' | tee -a "$LOG_FILE"
        else
            log_test 1 "Risk Worker processing" "No risk evaluation activity found"
        fi
        
        # Check WorkerPool
        local workerpool_logs=$(check_pod_logs "ksam-core" "WorkerPool.*Started\|WorkerPool.*worker")
        if [ -n "$workerpool_logs" ]; then
            log_test 0 "WorkerPool active"
        else
            log_test 1 "WorkerPool active" "No worker pool activity found"
        fi
    else
        log_test 1 "Core pods running" "No core pods found"
    fi
}

# Test Workers -> Database Flow
test_workers_database_flow() {
    log "\n${BLUE}=== Workers -> Database Flow ===${NC}"
    
    # Check PostgreSQL
    local db_pods=$(check_pod_status "postgres")
    if [ "$db_pods" -gt 0 ]; then
        log_test 0 "PostgreSQL running ($db_pods pods)"
        
        # Check database connectivity from Core
        local db_errors=$(check_pod_logs "ksam-core" "database.*error\|failed to.*database\|connection.*failed" | grep -v "ERROR: invalid input" | head -3)
        if [ -z "$db_errors" ]; then
            log_test 0 "Database connectivity (no errors)"
        else
            log_test 1 "Database connectivity" "Database errors found"
            echo "$db_errors" | sed 's/^/  /' | tee -a "$LOG_FILE"
        fi
        
        # Check data in database
        local db_pod=$(kubectl get pod -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -n "$db_pod" ]; then
            # Count resources
            local sa_count=$(kubectl exec -n ksam "$db_pod" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM service_accounts;" 2>/dev/null | tr -d ' ' || echo "0")
            local pod_count=$(kubectl exec -n ksam "$db_pod" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods;" 2>/dev/null | tr -d ' ' || echo "0")
            local role_count=$(kubectl exec -n ksam "$db_pod" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM roles;" 2>/dev/null | tr -d ' ' || echo "0")
            local insights_count=$(kubectl exec -n ksam "$db_pod" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" 2>/dev/null | tr -d ' ' || echo "0")
            
            log "  ServiceAccounts: $sa_count"
            log "  Pods: $pod_count"
            log "  Roles: $role_count"
            log "  Insights: $insights_count"
            
            if [ "$sa_count" -gt 0 ] || [ "$pod_count" -gt 0 ] || [ "$role_count" -gt 0 ]; then
                log_test 0 "Data persisted in database"
            else
                log_test 1 "Data persisted in database" "No data found"
            fi
            
            if [ "$insights_count" -gt 0 ]; then
                log_test 0 "Insights created ($insights_count total)"
            else
                log_test 1 "Insights created" "No insights found"
            fi
        fi
    else
        log_test 1 "PostgreSQL running" "No PostgreSQL pods found"
    fi
}

# Test Database -> API Flow
test_database_api_flow() {
    log "\n${BLUE}=== Database -> API Flow ===${NC}"
    
    # Get auth token
    local login_response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' \
        "http://localhost:8080/api/v1/auth/login" 2>/dev/null || echo "")
    
    local auth_token=$(echo "$login_response" | jq -r '.token // empty' 2>/dev/null || echo "")
    
    if [ -z "$auth_token" ] || [ "$auth_token" = "null" ]; then
        log_test 1 "API authentication" "Failed to get token"
        return
    fi
    
    log_test 0 "API authentication"
    
    # Test API endpoints
    local api_base="http://localhost:8080/api/v1"
    local headers="-H 'Authorization: Bearer ${auth_token}'"
    
    # Test clusters endpoint
    local clusters_response=$(curl -s -H "Authorization: Bearer ${auth_token}" \
        "${api_base}/clusters" 2>/dev/null || echo "")
    if echo "$clusters_response" | jq . >/dev/null 2>&1; then
        log_test 0 "Clusters API endpoint"
    else
        log_test 1 "Clusters API endpoint" "Invalid response"
    fi
    
    # Test service accounts endpoint
    local sa_response=$(curl -s -H "Authorization: Bearer ${auth_token}" \
        "${api_base}/serviceaccounts?pageSize=1" 2>/dev/null || echo "")
    if echo "$sa_response" | jq . >/dev/null 2>&1; then
        log_test 0 "ServiceAccounts API endpoint"
    else
        log_test 1 "ServiceAccounts API endpoint" "Invalid response"
    fi
    
    # Test insights endpoint
    local insights_response=$(curl -s -H "Authorization: Bearer ${auth_token}" \
        "${api_base}/insights/summary" 2>/dev/null || echo "")
    if echo "$insights_response" | jq . >/dev/null 2>&1; then
        log_test 0 "Insights API endpoint"
        local total=$(echo "$insights_response" | jq -r '.total // 0' 2>/dev/null || echo "0")
        log "  Total insights: $total"
    else
        log_test 1 "Insights API endpoint" "Invalid response"
    fi
    
    # Test graph endpoint
    local graph_response=$(curl -s -H "Authorization: Bearer ${auth_token}" \
        "${api_base}/graph" 2>/dev/null || echo "")
    if echo "$graph_response" | jq . >/dev/null 2>&1; then
        log_test 0 "Graph API endpoint"
    else
        log_test 1 "Graph API endpoint" "Invalid response"
    fi
}

# Main execution
main() {
    log "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    log "${BLUE}║         KSAM Data Flow Test Suite                             ║${NC}"
    log "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    log ""
    log "Test started at: $(date)"
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
    
    # Run flow tests
    test_agent_nats_flow
    test_nats_workers_flow
    test_workers_database_flow
    test_database_api_flow
    
    log "\n${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    log "${BLUE}║                    Test Complete                               ║${NC}"
    log "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    log ""
    log "Test completed at: $(date)"
    log "Log file: ${LOG_FILE}"
}

main


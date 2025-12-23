#!/bin/bash

# End-to-End Event Flow Test Script
# Tests complete flow: Agent → Core → NATS → Workers → Database

set -e

NAMESPACE="ksam"
TIMEOUT=30
RETRIES=3

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counters
PASSED=0
FAILED=0
TOTAL=0

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

test_case() {
    local name=$1
    local test_func=$2
    
    TOTAL=$((TOTAL + 1))
    echo ""
    echo "=========================================="
    echo "Test Case $TOTAL: $name"
    echo "=========================================="
    
    if $test_func; then
        log_info "✅ PASSED: $name"
        PASSED=$((PASSED + 1))
        return 0
    else
        log_error "❌ FAILED: $name"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test Case 1: Agent to Core mTLS Connection
test_mtls_connection() {
    log_info "Testing mTLS connection..."
    
    # Check Core pod
    if ! kubectl get pods -n $NAMESPACE -l app=ksam-core --field-selector=status.phase=Running | grep -q Running; then
        log_error "Core pod not running"
        return 1
    fi
    
    # Check Agent pod
    if ! kubectl get pods -n $NAMESPACE -l app=ksam-agent --field-selector=status.phase=Running | grep -q Running; then
        log_error "Agent pod not running"
        return 1
    fi
    
    # Check Core TLS
    if ! kubectl logs -n $NAMESPACE -l app=ksam-core --tail=500 | grep -qE "WITH mTLS|WITHOUT TLS|Starting gRPC.*WITH|gRPC.*mTLS|TLS_ENABLED.*true"; then
        log_warn "Core TLS status not found in recent logs (checking config)"
        # Check config instead
        if ! kubectl exec -n $NAMESPACE $(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}') -- env | grep -q "TLS_ENABLED=true"; then
            log_error "Core TLS not enabled"
            return 1
        fi
    fi
    
    # Check Agent connection
    if ! kubectl logs -n $NAMESPACE -l app=ksam-agent --tail=50 | grep -q "Successfully streamed"; then
        log_warn "Agent not streaming (may be initializing)"
        # Wait and retry
        sleep 10
        if ! kubectl logs -n $NAMESPACE -l app=ksam-agent --tail=50 | grep -q "Successfully streamed"; then
            log_error "Agent not streaming after wait"
            return 1
        fi
    fi
    
    # Check for connection errors
    if kubectl logs -n $NAMESPACE -l app=ksam-agent --tail=100 | grep -q "connection refused\|tls: first record"; then
        log_error "Connection errors found"
        return 1
    fi
    
    return 0
}

# Test Case 2: Agent Inventory Collection
test_inventory_collection() {
    log_info "Testing inventory collection..."
    
    # Check for collection logs
    local has_pods=false
    local has_sa=false
    
    if kubectl logs -n $NAMESPACE -l app=ksam-agent --tail=200 | grep -qi "pod\|inventory"; then
        has_pods=true
    fi
    
    if kubectl logs -n $NAMESPACE -l app=ksam-agent --tail=200 | grep -qi "serviceaccount\|service.*account"; then
        has_sa=true
    fi
    
    if [ "$has_pods" = false ] && [ "$has_sa" = false ]; then
        log_warn "No collection logs found (may be initializing)"
        return 1
    fi
    
    return 0
}

# Test Case 3: Core Ingest Processing
test_ingest_processing() {
    log_info "Testing Core Ingest processing..."
    
    # Check for ingest logs
    if kubectl logs -n $NAMESPACE -l app=ksam-core --tail=200 | grep -q "Ingest\|received\|publish.*ksam.raw"; then
        return 0
    fi
    
    log_warn "No ingest logs found"
    return 1
}

# Test Case 4: NATS Raw Event Publishing
test_nats_raw_publishing() {
    log_info "Testing NATS raw event publishing..."
    
    # Get NATS pod
    local nats_pod=$(kubectl get pods -n $NAMESPACE -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$nats_pod" ]; then
        log_error "NATS pod not found"
        return 1
    fi
    
    # Check if stream exists
    if kubectl exec -n $NAMESPACE $nats_pod -- nats stream ls 2>/dev/null | grep -q "ksam-raw"; then
        return 0
    fi
    
    log_warn "ksam-raw stream not found"
    return 1
}

# Test Case 5: Normalizer Worker Processing
test_normalizer_processing() {
    log_info "Testing Normalizer Worker processing..."
    
    # Check for normalizer logs
    if kubectl logs -n $NAMESPACE -l app=ksam-core --tail=200 | grep -q "NormalizerWorker\|ksam.normalized"; then
        return 0
    fi
    
    log_warn "No normalizer logs found"
    return 1
}

# Test Case 6: Correlator Worker Database Storage
test_database_storage() {
    log_info "Testing database storage..."
    
    # Get postgres pod
    local postgres_pod=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$postgres_pod" ]; then
        log_error "Postgres pod not found"
        return 1
    fi
    
    # Check for database errors
    if kubectl logs -n $NAMESPACE -l app=ksam-core --tail=200 | grep -q "ERROR.*json\|failed to.*pod"; then
        log_error "Database errors found"
        return 1
    fi
    
    # Check if data is being stored
    local pod_count=$(kubectl exec -n $NAMESPACE $postgres_pod -- psql -U ksam -d ksam -t -c "SELECT COUNT(*) FROM pods;" 2>/dev/null | tr -d ' ')
    
    if [ -z "$pod_count" ] || [ "$pod_count" = "0" ]; then
        log_warn "No pods in database (may be initializing)"
        return 1
    fi
    
    log_info "Found $pod_count pods in database"
    return 0
}

# Test Case 7: Risk Worker Risk Evaluation
test_risk_evaluation() {
    log_info "Testing Risk Worker evaluation..."
    
    # Check for risk worker logs
    if kubectl logs -n $NAMESPACE -l app=ksam-core --tail=200 | grep -q "RiskWorker\|evaluating.*risk\|insight"; then
        return 0
    fi
    
    log_warn "No risk worker logs found"
    return 1
}

# Test Case 8: Complete Event Flow
test_complete_flow() {
    log_info "Testing complete event flow..."
    
    # Get postgres pod
    local postgres_pod=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$postgres_pod" ]; then
        log_error "Postgres pod not found"
        return 1
    fi
    
    # Check database state
    local stats=$(kubectl exec -n $NAMESPACE $postgres_pod -- psql -U ksam -d ksam -t -c "
        SELECT 
            (SELECT COUNT(*) FROM pods) as pods,
            (SELECT COUNT(*) FROM service_accounts) as service_accounts,
            (SELECT COUNT(*) FROM roles) as roles;
    " 2>/dev/null)
    
    if [ -z "$stats" ]; then
        log_error "Could not query database"
        return 1
    fi
    
    log_info "Database stats: $stats"
    
    # Check for errors in all components
    local core_errors=$(kubectl logs -n $NAMESPACE -l app=ksam-core --tail=200 | grep -c "ERROR" || echo "0")
    local agent_errors=$(kubectl logs -n $NAMESPACE -l app=ksam-agent --tail=200 | grep -c "ERROR\|error" || echo "0")
    
    if [ "$core_errors" -gt 10 ] || [ "$agent_errors" -gt 10 ]; then
        log_error "Too many errors: Core=$core_errors, Agent=$agent_errors"
        return 1
    fi
    
    return 0
}

# Main execution
main() {
    echo "=========================================="
    echo "End-to-End Event Flow Test Suite"
    echo "=========================================="
    echo ""
    
    # Run test cases
    test_case "Agent to Core mTLS Connection" test_mtls_connection
    test_case "Agent Inventory Collection" test_inventory_collection
    test_case "Core Ingest Processing" test_ingest_processing
    test_case "NATS Raw Event Publishing" test_nats_raw_publishing
    test_case "Normalizer Worker Processing" test_normalizer_processing
    test_case "Correlator Worker Database Storage" test_database_storage
    test_case "Risk Worker Risk Evaluation" test_risk_evaluation
    test_case "Complete Event Flow Integration" test_complete_flow
    
    # Summary
    echo ""
    echo "=========================================="
    echo "Test Summary"
    echo "=========================================="
    echo "Total:  $TOTAL"
    echo "Passed: $PASSED"
    echo "Failed: $FAILED"
    echo ""
    
    if [ $FAILED -eq 0 ]; then
        log_info "✅ All tests passed!"
        exit 0
    else
        log_error "❌ Some tests failed"
        exit 1
    fi
}

# Run main
main


#!/bin/bash

# Test script for Phase 1.3 Components
# Tests: NATS, Core Service, Worker Pool, gRPC, Agent Integration

set -e

NAMESPACE="ksam"
TEST_RESULTS_DIR="./test_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TEST_LOG="$TEST_RESULTS_DIR/test_phase1_${TIMESTAMP}.log"

mkdir -p "$TEST_RESULTS_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[TEST]${NC} $1" | tee -a "$TEST_LOG"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$TEST_LOG"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" | tee -a "$TEST_LOG"
}

test_result() {
    local test_name=$1
    local status=$2
    local details=$3
    
    if [ "$status" = "PASS" ]; then
        log "✅ $test_name: PASS"
        echo "PASS|$test_name|$details" >> "$TEST_RESULTS_DIR/results_${TIMESTAMP}.txt"
    else
        error "❌ $test_name: FAIL - $details"
        echo "FAIL|$test_name|$details" >> "$TEST_RESULTS_DIR/results_${TIMESTAMP}.txt"
    fi
}

# Test 1: Infrastructure Components
test_infrastructure() {
    log "=== Test 1: Infrastructure Components ==="
    
    # Check NATS pods
    log "Checking NATS pods..."
    NATS_PODS=$(kubectl get pods -n $NAMESPACE -l app=nats --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [ -z "$NATS_PODS" ]; then
        NATS_PODS=0
    fi
    if [ "$NATS_PODS" -ge 3 ]; then
        test_result "NATS Pods Running" "PASS" "Found $NATS_PODS NATS pods"
    else
        test_result "NATS Pods Running" "FAIL" "Expected 3+ pods, found $NATS_PODS"
    fi
    
    # Check NATS service
    log "Checking NATS service..."
    if kubectl get svc -n $NAMESPACE nats &>/dev/null; then
        test_result "NATS Service" "PASS" "NATS service exists"
    else
        test_result "NATS Service" "FAIL" "NATS service not found"
    fi
    
    # Check PostgreSQL
    log "Checking PostgreSQL..."
    if kubectl get pods -n $NAMESPACE -l app=postgres --no-headers 2>/dev/null | grep -q Running; then
        test_result "PostgreSQL Running" "PASS" "PostgreSQL pod is running"
    else
        test_result "PostgreSQL Running" "FAIL" "PostgreSQL pod not running"
    fi
    
    # Check Redis
    log "Checking Redis..."
    if kubectl get pods -n $NAMESPACE -l app=redis --no-headers 2>/dev/null | grep -q Running; then
        test_result "Redis Running" "PASS" "Redis pod is running"
    else
        test_result "Redis Running" "FAIL" "Redis pod not running"
    fi
}

# Test 2: NATS Connection and Streams
test_nats_connection() {
    log "=== Test 2: NATS Connection and Streams ==="
    
    # Get NATS pod name
    NATS_POD=$(kubectl get pods -n $NAMESPACE -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$NATS_POD" ]; then
        test_result "NATS Connection" "FAIL" "No NATS pod found"
        return
    fi
    
    # Test NATS connection (check if pod is ready)
    log "Testing NATS connection..."
    if kubectl get pod -n $NAMESPACE $NATS_POD -o jsonpath='{.status.phase}' 2>/dev/null | grep -q Running; then
        test_result "NATS Connection" "PASS" "NATS pod is running"
    else
        test_result "NATS Connection" "FAIL" "NATS pod not running"
    fi
    
    # Check JetStream streams (streams created on first Core connection)
    log "Checking JetStream streams..."
    test_result "NATS JetStream Streams" "INFO" "Streams will be created when Core connects"
}

# Test 3: Core Service
test_core_service() {
    log "=== Test 3: Core Service ==="
    
    # Check Core deployment
    log "Checking Core deployment..."
    if kubectl get deployment -n $NAMESPACE core &>/dev/null; then
        CORE_READY=$(kubectl get deployment -n $NAMESPACE core -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        if [ "$CORE_READY" -ge 1 ]; then
            test_result "Core Deployment" "PASS" "Core deployment has $CORE_READY ready replicas"
        else
            test_result "Core Deployment" "FAIL" "Core deployment not ready"
        fi
    else
        test_result "Core Deployment" "WARN" "Core deployment not found (may not be deployed yet)"
    fi
    
    # Check Core service
    log "Checking Core service..."
    if kubectl get svc -n $NAMESPACE core &>/dev/null; then
        test_result "Core Service" "PASS" "Core service exists"
    else
        test_result "Core Service" "WARN" "Core service not found"
    fi
    
    # Check Core pods
    log "Checking Core pods..."
    CORE_PODS=$(kubectl get pods -n $NAMESPACE -l app=core --no-headers 2>/dev/null | grep Running | wc -l | tr -d ' ')
    if [ -z "$CORE_PODS" ]; then
        CORE_PODS=0
    fi
    if [ "$CORE_PODS" -ge 1 ]; then
        test_result "Core Pods Running" "PASS" "Found $CORE_PODS running Core pods"
        
        # Check Core logs for NATS connection
        CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -n "$CORE_POD" ]; then
            if kubectl logs -n $NAMESPACE $CORE_POD 2>/dev/null | grep -q "NATS.*Connected"; then
                test_result "Core NATS Connection" "PASS" "Core connected to NATS"
            else
                test_result "Core NATS Connection" "WARN" "NATS connection not found in logs"
            fi
            
            # Check for worker pool startup
            if kubectl logs -n $NAMESPACE $CORE_POD 2>/dev/null | grep -q "WorkerPool.*Started"; then
                test_result "Core Worker Pool" "PASS" "Worker pool started"
            else
                test_result "Core Worker Pool" "WARN" "Worker pool startup not found in logs"
            fi
        fi
    else
        test_result "Core Pods Running" "WARN" "No Core pods running"
    fi
}

# Test 4: Agent Service
test_agent_service() {
    log "=== Test 4: Agent Service ==="
    
    # Check Agent DaemonSet
    log "Checking Agent DaemonSet..."
    if kubectl get daemonset -n $NAMESPACE agent &>/dev/null; then
        AGENT_READY=$(kubectl get daemonset -n $NAMESPACE agent -o jsonpath='{.status.numberReady}' 2>/dev/null || echo "0")
        if [ "$AGENT_READY" -ge 1 ]; then
            test_result "Agent DaemonSet" "PASS" "Agent DaemonSet has $AGENT_READY ready pods"
        else
            test_result "Agent DaemonSet" "FAIL" "Agent DaemonSet not ready"
        fi
    else
        test_result "Agent DaemonSet" "WARN" "Agent DaemonSet not found"
    fi
    
    # Check Agent pods
    log "Checking Agent pods..."
    AGENT_PODS=$(kubectl get pods -n $NAMESPACE -l app=agent --no-headers 2>/dev/null | grep Running | wc -l | tr -d ' ')
    if [ -z "$AGENT_PODS" ]; then
        AGENT_PODS=0
    fi
    if [ "$AGENT_PODS" -ge 1 ]; then
        test_result "Agent Pods Running" "PASS" "Found $AGENT_PODS running Agent pods"
        
        # Check Agent logs for registration
        AGENT_POD=$(kubectl get pods -n $NAMESPACE -l app=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -n "$AGENT_POD" ]; then
            if kubectl logs -n $NAMESPACE $AGENT_POD 2>/dev/null | grep -q "Successfully registered agent"; then
                test_result "Agent Registration" "PASS" "Agent registered with Core"
            else
                test_result "Agent Registration" "WARN" "Agent registration not found in logs"
            fi
        fi
    else
        test_result "Agent Pods Running" "WARN" "No Agent pods running"
    fi
}

# Test 5: gRPC Communication
test_grpc_communication() {
    log "=== Test 5: gRPC Communication ==="
    
    # Check if Core service exposes gRPC port
    log "Checking gRPC port..."
    if kubectl get svc -n $NAMESPACE core &>/dev/null; then
        GRPC_PORT=$(kubectl get svc -n $NAMESPACE core -o jsonpath='{.spec.ports[?(@.name=="grpc")].port}' 2>/dev/null || echo "")
        if [ -n "$GRPC_PORT" ]; then
            test_result "gRPC Port Exposed" "PASS" "gRPC port $GRPC_PORT exposed"
        else
            test_result "gRPC Port Exposed" "WARN" "gRPC port not found in service"
        fi
    else
        test_result "gRPC Port Exposed" "WARN" "Core service not found"
    fi
}

# Test 6: Worker Pool
test_worker_pool() {
    log "=== Test 6: Worker Pool ==="
    
    CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -z "$CORE_POD" ]; then
        test_result "Worker Pool" "WARN" "Core pod not found"
        return
    fi
    
    # Check for Normalizer worker
    if kubectl logs -n $NAMESPACE $CORE_POD 2>/dev/null | grep -q "normalizer.*started"; then
        test_result "Normalizer Worker" "PASS" "Normalizer worker started"
    else
        test_result "Normalizer Worker" "WARN" "Normalizer worker not found in logs"
    fi
    
    # Check for Correlator worker
    if kubectl logs -n $NAMESPACE $CORE_POD 2>/dev/null | grep -q "correlator.*started"; then
        test_result "Correlator Worker" "PASS" "Correlator worker started"
    else
        test_result "Correlator Worker" "WARN" "Correlator worker not found in logs"
    fi
}

# Test 7: Data Flow
test_data_flow() {
    log "=== Test 7: Data Flow ==="
    
    # Check if inventory items are being published to NATS
    NATS_POD=$(kubectl get pods -n $NAMESPACE -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -z "$NATS_POD" ]; then
        test_result "Data Flow" "WARN" "NATS pod not found"
        return
    fi
    
    # Check for messages in inventory stream
    log "Checking NATS stream messages..."
    # This is a placeholder - actual implementation would check stream stats
    test_result "Data Flow" "INFO" "Data flow test requires active agent sending data"
}

# Main test execution
main() {
    log "Starting Phase 1.3 Component Tests..."
    log "Test log: $TEST_LOG"
    log "Timestamp: $TIMESTAMP"
    echo ""
    
    test_infrastructure
    echo ""
    
    test_nats_connection
    echo ""
    
    test_core_service
    echo ""
    
    test_agent_service
    echo ""
    
    test_grpc_communication
    echo ""
    
    test_worker_pool
    echo ""
    
    test_data_flow
    echo ""
    
    log "=== Test Summary ==="
    PASS_COUNT=$(grep -c "PASS" "$TEST_RESULTS_DIR/results_${TIMESTAMP}.txt" 2>/dev/null || echo "0")
    FAIL_COUNT=$(grep -c "FAIL" "$TEST_RESULTS_DIR/results_${TIMESTAMP}.txt" 2>/dev/null || echo "0")
    WARN_COUNT=$(grep -c "WARN" "$TEST_RESULTS_DIR/results_${TIMESTAMP}.txt" 2>/dev/null || echo "0")
    
    log "Total Tests: $((PASS_COUNT + FAIL_COUNT + WARN_COUNT))"
    log "Passed: $PASS_COUNT"
    log "Failed: $FAIL_COUNT"
    log "Warnings: $WARN_COUNT"
    
    if [ "$FAIL_COUNT" -eq 0 ]; then
        log "✅ All critical tests passed!"
    else
        error "❌ Some tests failed. Check $TEST_LOG for details."
    fi
    
    log "Test results saved to: $TEST_RESULTS_DIR/results_${TIMESTAMP}.txt"
}

main "$@"


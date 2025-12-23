#!/bin/bash

# End-to-End Test Script for Phase 1.3
# Tests: Agent → Core → NATS → Workers

set -e

NAMESPACE="ksam"
TEST_RESULTS_DIR="./test_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TEST_LOG="$TEST_RESULTS_DIR/test_e2e_${TIMESTAMP}.log"

mkdir -p "$TEST_RESULTS_DIR"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

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
        log "✅ $test_name: PASS - $details"
        echo "PASS|$test_name|$details" >> "$TEST_RESULTS_DIR/e2e_results_${TIMESTAMP}.txt"
    else
        error "❌ $test_name: FAIL - $details"
        echo "FAIL|$test_name|$details" >> "$TEST_RESULTS_DIR/e2e_results_${TIMESTAMP}.txt"
    fi
}

# Test 1: Core Service Health
test_core_health() {
    log "=== Test 1: Core Service Health ==="
    
    CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -z "$CORE_POD" ]; then
        test_result "Core Pod Exists" "FAIL" "Core pod not found"
        return
    fi
    
    test_result "Core Pod Exists" "PASS" "Found pod: $CORE_POD"
    
    # Check if pod is ready
    READY=$(kubectl get pod -n $NAMESPACE $CORE_POD -o jsonpath='{.status.containerStatuses[0].ready}' 2>/dev/null)
    if [ "$READY" = "true" ]; then
        test_result "Core Pod Ready" "PASS" "Pod is ready"
    else
        test_result "Core Pod Ready" "FAIL" "Pod not ready"
    fi
    
    # Check HTTP health endpoint
    if kubectl exec -n $NAMESPACE $CORE_POD -- wget -q -O- http://localhost:8080/health 2>/dev/null | grep -q "ok"; then
        test_result "Core HTTP Health" "PASS" "Health endpoint responding"
    else
        test_result "Core HTTP Health" "WARN" "Health endpoint check failed"
    fi
    
    # Check for NATS connection in logs
    if kubectl logs -n $NAMESPACE $CORE_POD 2>&1 | grep -q "NATS.*Connected"; then
        test_result "Core NATS Connection" "PASS" "NATS connected"
    else
        test_result "Core NATS Connection" "WARN" "NATS connection not found in logs"
    fi
    
    # Check for worker pool startup
    WORKER_COUNT=$(kubectl logs -n $NAMESPACE $CORE_POD 2>&1 | grep -c "Worker.*started" || echo "0")
    if [ "$WORKER_COUNT" -ge 2 ]; then
        test_result "Core Worker Pool" "PASS" "Found $WORKER_COUNT workers started"
    else
        test_result "Core Worker Pool" "WARN" "Only $WORKER_COUNT workers found"
    fi
}

# Test 2: Agent Service
test_agent_service() {
    log "=== Test 2: Agent Service ==="
    
    AGENT_PODS=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [ -z "$AGENT_PODS" ]; then
        AGENT_PODS=0
    fi
    
    if [ "$AGENT_PODS" -ge 1 ]; then
        test_result "Agent Pods Running" "PASS" "Found $AGENT_PODS agent pods"
        
        AGENT_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -n "$AGENT_POD" ]; then
            # Check for registration
            if kubectl logs -n $NAMESPACE $AGENT_POD 2>&1 | grep -q "Successfully registered agent"; then
                test_result "Agent Registration" "PASS" "Agent registered with Core"
            else
                test_result "Agent Registration" "WARN" "Registration not found in logs"
            fi
        fi
    else
        test_result "Agent Pods Running" "FAIL" "No agent pods found"
    fi
}

# Test 3: NATS Streams
test_nats_streams() {
    log "=== Test 3: NATS Streams ==="
    
    NATS_POD=$(kubectl get pods -n $NAMESPACE -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -z "$NATS_POD" ]; then
        test_result "NATS Pod" "FAIL" "NATS pod not found"
        return
    fi
    
    # Check if streams exist (they should be created by Core)
    STREAMS=$(kubectl exec -n $NAMESPACE $NATS_POD -- nats --server=nats://localhost:4222 stream ls 2>/dev/null | grep -c "ksam-" || echo "0")
    if [ "$STREAMS" -ge 1 ]; then
        test_result "NATS Streams Created" "PASS" "Found $STREAMS streams"
    else
        test_result "NATS Streams Created" "WARN" "No streams found (may need Core to connect first)"
    fi
}

# Test 4: gRPC Communication
test_grpc_communication() {
    log "=== Test 4: gRPC Communication ==="
    
    CORE_SVC=$(kubectl get svc -n $NAMESPACE core -o jsonpath='{.spec.ports[?(@.name=="grpc")].port}' 2>/dev/null)
    if [ -n "$CORE_SVC" ]; then
        test_result "gRPC Port Exposed" "PASS" "gRPC port $CORE_SVC exposed"
    else
        test_result "gRPC Port Exposed" "FAIL" "gRPC port not found"
    fi
    
    # Check if Core is listening on gRPC port
    CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$CORE_POD" ]; then
        if kubectl logs -n $NAMESPACE $CORE_POD 2>&1 | grep -q "Starting gRPC server"; then
            test_result "gRPC Server Running" "PASS" "gRPC server started"
        else
            test_result "gRPC Server Running" "WARN" "gRPC server startup not found"
        fi
    fi
}

# Test 5: Data Flow
test_data_flow() {
    log "=== Test 5: Data Flow ==="
    
    # Check if Agent is sending data
    AGENT_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -z "$AGENT_POD" ]; then
        test_result "Data Flow" "WARN" "Agent pod not found"
        return
    fi
    
    # Check for inventory streaming
    if kubectl logs -n $NAMESPACE $AGENT_POD 2>&1 | grep -q "inventory\|stream"; then
        test_result "Agent Data Collection" "PASS" "Agent is collecting/streaming data"
    else
        test_result "Agent Data Collection" "INFO" "No data collection logs found yet"
    fi
    
    # Check Core for received data
    CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$CORE_POD" ]; then
        if kubectl logs -n $NAMESPACE $CORE_POD 2>&1 | grep -q "Published inventory\|Published event"; then
            test_result "Core Data Processing" "PASS" "Core is processing data"
        else
            test_result "Core Data Processing" "INFO" "No data processing logs found yet"
        fi
    fi
}

# Main
main() {
    log "Starting End-to-End Tests..."
    log "Test log: $TEST_LOG"
    log "Timestamp: $TIMESTAMP"
    echo ""
    
    test_core_health
    echo ""
    
    test_agent_service
    echo ""
    
    test_nats_streams
    echo ""
    
    test_grpc_communication
    echo ""
    
    test_data_flow
    echo ""
    
    log "=== Test Summary ==="
    PASS_COUNT=$(grep -c "PASS" "$TEST_RESULTS_DIR/e2e_results_${TIMESTAMP}.txt" 2>/dev/null || echo "0")
    FAIL_COUNT=$(grep -c "FAIL" "$TEST_RESULTS_DIR/e2e_results_${TIMESTAMP}.txt" 2>/dev/null || echo "0")
    WARN_COUNT=$(grep -c "WARN" "$TEST_RESULTS_DIR/e2e_results_${TIMESTAMP}.txt" 2>/dev/null || echo "0")
    
    log "Total Tests: $((PASS_COUNT + FAIL_COUNT + WARN_COUNT))"
    log "Passed: $PASS_COUNT"
    log "Failed: $FAIL_COUNT"
    log "Warnings: $WARN_COUNT"
    
    if [ "$FAIL_COUNT" -eq 0 ]; then
        log "✅ All critical tests passed!"
    else
        error "❌ Some tests failed. Check $TEST_LOG for details."
    fi
}

main "$@"

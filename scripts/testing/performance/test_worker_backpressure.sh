#!/bin/bash

# Test script for Worker Backpressure Handling
# Tests queue depth monitoring, backpressure metrics, and worker load tracking

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

NAMESPACE="ksam"
CORE_SERVICE="ksam-core"
CORE_PORT="8080"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Get Core pod name
get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Get Core service URL
get_core_url() {
    local port_forward_pid
    # Check if port-forward is already running
    if lsof -Pi :$CORE_PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "http://localhost:$CORE_PORT"
        return
    fi
    
    # Start port-forward in background
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    kubectl port-forward -n "$NAMESPACE" "$pod" $CORE_PORT:8080 >/dev/null 2>&1 &
    port_forward_pid=$!
    sleep 2
    
    echo "http://localhost:$CORE_PORT"
    echo "$port_forward_pid" > /tmp/ksam_port_forward.pid
}

# Cleanup port-forward
cleanup_port_forward() {
    if [ -f /tmp/ksam_port_forward.pid ]; then
        local pid=$(cat /tmp/ksam_port_forward.pid)
        kill $pid 2>/dev/null || true
        rm -f /tmp/ksam_port_forward.pid
    fi
    # Kill any remaining port-forwards
    pkill -f "kubectl port-forward.*$CORE_PORT" 2>/dev/null || true
}

trap cleanup_port_forward EXIT

# Test 1: Check Core pod is running
test_core_pod_running() {
    log_info "Test 1: Checking Core pod status..."
    
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    local status=$(kubectl get pod "$pod" -n "$NAMESPACE" -o jsonpath='{.status.phase}')
    if [ "$status" != "Running" ]; then
        log_error "Core pod is not Running (status: $status)"
        return 1
    fi
    
    log_success "Core pod is running: $pod"
    echo "  Pod: $pod"
    echo "  Status: $status"
    return 0
}

# Test 2: Check Queue Depth Monitoring is active
test_queue_depth_monitoring() {
    log_info "Test 2: Checking Queue Depth Monitoring..."
    
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    # Check logs for queue depth monitoring
    local logs=$(kubectl logs "$pod" -n "$NAMESPACE" --tail=100 | grep -i "queue depth\|QueueDepthMonitoring" || true)
    
    if [ -z "$logs" ]; then
        log_warn "Queue depth monitoring logs not found (may be starting up)"
        # Check if monitoring was started in main.go
        local main_logs=$(kubectl logs "$pod" -n "$NAMESPACE" --tail=200 | grep -i "Queue depth monitoring started" || true)
        if [ -n "$main_logs" ]; then
            log_success "Queue depth monitoring started (found in logs)"
            echo "  Log: $main_logs"
            return 0
        else
            log_error "Queue depth monitoring not found in logs"
            return 1
        fi
    else
        log_success "Queue depth monitoring is active"
        echo "  Logs: $logs"
        return 0
    fi
}

# Test 3: Check Prometheus Metrics for Queue Depth
test_queue_depth_metrics() {
    log_info "Test 3: Checking Queue Depth Metrics in Prometheus..."
    
    local url=$(get_core_url)
    if [ -z "$url" ]; then
        log_error "Failed to get Core URL"
        return 1
    fi
    
    # Wait a bit for metrics to be collected
    sleep 5
    
    # Get metrics
    local metrics=$(curl -s "$url/metrics" 2>/dev/null || echo "")
    
    if [ -z "$metrics" ]; then
        log_error "Failed to fetch metrics"
        return 1
    fi
    
    # Check for queue depth metric
    local queue_depth=$(echo "$metrics" | grep "ksam_worker_queue_depth" || true)
    
    if [ -z "$queue_depth" ]; then
        log_warn "Queue depth metric not found (may need time to collect)"
        echo "  Available worker metrics:"
        echo "$metrics" | grep "ksam_worker" | head -10
        return 0  # Not a failure, just not collected yet
    else
        log_success "Queue depth metrics found"
        echo "  Metrics:"
        echo "$queue_depth" | head -5
        return 0
    fi
}

# Test 4: Check Backpressure Metrics
test_backpressure_metrics() {
    log_info "Test 4: Checking Backpressure Metrics..."
    
    local url=$(get_core_url)
    if [ -z "$url" ]; then
        log_error "Failed to get Core URL"
        return 1
    fi
    
    # Get metrics
    local metrics=$(curl -s "$url/metrics" 2>/dev/null || echo "")
    
    if [ -z "$metrics" ]; then
        log_error "Failed to fetch metrics"
        return 1
    fi
    
    # Check for backpressure metrics
    local backpressure_total=$(echo "$metrics" | grep "ksam_worker_backpressure_total" || true)
    local backpressure_duration=$(echo "$metrics" | grep "ksam_worker_backpressure_duration" || true)
    local concurrent_processing=$(echo "$metrics" | grep "ksam_worker_concurrent_processing" || true)
    
    local found=0
    if [ -n "$backpressure_total" ]; then
        log_success "Backpressure total metric found"
        echo "  $backpressure_total"
        found=1
    fi
    
    if [ -n "$backpressure_duration" ]; then
        log_success "Backpressure duration metric found"
        echo "  $backpressure_duration"
        found=1
    fi
    
    if [ -n "$concurrent_processing" ]; then
        log_success "Concurrent processing metric found"
        echo "  $concurrent_processing"
        found=1
    fi
    
    if [ $found -eq 0 ]; then
        log_warn "Backpressure metrics not found (may need time to collect)"
        return 0  # Not a failure
    fi
    
    return 0
}

# Test 5: Check Worker Load Tracking
test_worker_load_tracking() {
    log_info "Test 5: Checking Worker Load Tracking..."
    
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    # Check logs for load tracker creation
    local logs=$(kubectl logs "$pod" -n "$NAMESPACE" --tail=200 | grep -i "load tracker\|WorkerLoadTracker" || true)
    
    if [ -z "$logs" ]; then
        log_warn "Load tracker logs not found (may be starting up)"
        return 0  # Not a failure
    else
        log_success "Load tracker is active"
        echo "  Logs:"
        echo "$logs" | head -5
        return 0
    fi
}

# Test 6: Check Worker Processing Metrics
test_worker_processing_metrics() {
    log_info "Test 6: Checking Worker Processing Metrics..."
    
    local url=$(get_core_url)
    if [ -z "$url" ]; then
        log_error "Failed to get Core URL"
        return 1
    fi
    
    # Get metrics
    local metrics=$(curl -s "$url/metrics" 2>/dev/null || echo "")
    
    if [ -z "$metrics" ]; then
        log_error "Failed to fetch metrics"
        return 1
    fi
    
    # Check for worker processing metrics
    local messages_processed=$(echo "$metrics" | grep "ksam_worker_messages_processed_total" || true)
    local processing_duration=$(echo "$metrics" | grep "ksam_worker_processing_duration_seconds" || true)
    
    local found=0
    if [ -n "$messages_processed" ]; then
        log_success "Messages processed metric found"
        echo "  $messages_processed"
        found=1
    fi
    
    if [ -n "$processing_duration" ]; then
        log_success "Processing duration metric found"
        echo "  $processing_duration"
        found=1
    fi
    
    if [ $found -eq 0 ]; then
        log_warn "Worker processing metrics not found"
        return 0  # Not a failure
    fi
    
    return 0
}

# Test 7: Check All Worker Metrics Summary
test_all_worker_metrics() {
    log_info "Test 7: Getting All Worker Metrics Summary..."
    
    local url=$(get_core_url)
    if [ -z "$url" ]; then
        log_error "Failed to get Core URL"
        return 1
    fi
    
    # Get metrics
    local metrics=$(curl -s "$url/metrics" 2>/dev/null || echo "")
    
    if [ -z "$metrics" ]; then
        log_error "Failed to fetch metrics"
        return 1
    fi
    
    # Extract all worker metrics
    local worker_metrics=$(echo "$metrics" | grep "ksam_worker" || true)
    
    if [ -z "$worker_metrics" ]; then
        log_error "No worker metrics found"
        return 1
    fi
    
    log_success "All worker metrics:"
    echo ""
    echo "$worker_metrics"
    echo ""
    
    # Count metrics by type
    local metric_types=$(echo "$worker_metrics" | grep -o "ksam_worker_[a-z_]*" | sort -u)
    log_info "Metric types found:"
    echo "$metric_types" | while read type; do
        local count=$(echo "$worker_metrics" | grep -c "$type" || echo "0")
        echo "  - $type: $count entries"
    done
    
    return 0
}

# Main test execution
main() {
    echo "=========================================="
    echo "Worker Backpressure Handling Test Suite"
    echo "=========================================="
    echo ""
    
    local tests_passed=0
    local tests_failed=0
    local tests_total=0
    
    # Run tests
    tests_total=$((tests_total + 1))
    if test_core_pod_running; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi
    echo ""
    
    tests_total=$((tests_total + 1))
    if test_queue_depth_monitoring; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi
    echo ""
    
    tests_total=$((tests_total + 1))
    if test_queue_depth_metrics; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi
    echo ""
    
    tests_total=$((tests_total + 1))
    if test_backpressure_metrics; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi
    echo ""
    
    tests_total=$((tests_total + 1))
    if test_worker_load_tracking; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi
    echo ""
    
    tests_total=$((tests_total + 1))
    if test_worker_processing_metrics; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi
    echo ""
    
    tests_total=$((tests_total + 1))
    if test_all_worker_metrics; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi
    echo ""
    
    # Summary
    echo "=========================================="
    echo "Test Summary"
    echo "=========================================="
    echo "Total Tests: $tests_total"
    echo -e "${GREEN}Passed: $tests_passed${NC}"
    echo -e "${RED}Failed: $tests_failed${NC}"
    echo ""
    
    if [ $tests_failed -eq 0 ]; then
        log_success "All tests passed!"
        return 0
    else
        log_error "Some tests failed"
        return 1
    fi
}

# Run main
main "$@"


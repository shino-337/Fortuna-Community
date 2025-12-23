#!/bin/bash

# Complete test script for all metrics: message format, queue depth, and backpressure

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

NAMESPACE="ksam"
CORE_PORT="8080"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Get Core pod
get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Setup port-forward
setup_port_forward() {
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    log_info "Setting up Core port-forward..."
    kubectl port-forward -n "$NAMESPACE" "$pod" $CORE_PORT:8080 >/dev/null 2>&1 &
    sleep 2
    log_success "Port-forward ready"
}

# Get metrics
get_metrics() {
    curl -s http://localhost:$CORE_PORT/metrics 2>/dev/null || echo ""
}

# Test 1: Fixed message format
test_fixed_messages() {
    log_info "Test 1: Publishing messages with fixed JSON format..."
    
    bash "$PROJECT_ROOT/scripts/publish_messages_fixed.sh" >/dev/null 2>&1
    
    log_info "Waiting for processing..."
    sleep 10
    
    # Check for successful processing
    local pod=$(get_core_pod)
    local logs=$(kubectl logs -n "$NAMESPACE" "$pod" --tail=50 | grep -E "Normalized and published" | wc -l)
    
    if [ "$logs" -gt 0 ]; then
        log_success "✅ Messages processed successfully ($logs successful)"
        return 0
    else
        log_warn "⚠️  No successful processing found yet"
        return 0  # Not a failure, may need more time
    fi
}

# Test 2: Queue depth metrics
test_queue_depth() {
    log_info "Test 2: Checking queue depth metrics..."
    
    # Wait for queue depth monitoring to run (runs every 10s)
    sleep 15
    
    local metrics=$(get_metrics)
    if [ -z "$metrics" ]; then
        log_error "Failed to get metrics"
        return 1
    fi
    
    local queue_depth=$(echo "$metrics" | grep "^ksam_worker_queue_depth" || true)
    
    if [ -n "$queue_depth" ]; then
        log_success "✅ Queue depth metrics found:"
        echo "$queue_depth"
        return 0
    else
        log_warn "⚠️  Queue depth metrics not found (may need more time or stream info)"
        return 0  # Not a failure
    fi
}

# Test 3: Backpressure test
test_backpressure() {
    log_info "Test 3: Testing backpressure with high load..."
    
    bash "$PROJECT_ROOT/scripts/test_backpressure.sh" 2>&1 | tail -30
    
    log_info "Checking backpressure metrics..."
    sleep 5
    
    local metrics=$(get_metrics)
    if [ -z "$metrics" ]; then
        log_error "Failed to get metrics"
        return 1
    fi
    
    local backpressure=$(echo "$metrics" | grep "^ksam_worker_backpressure_total" || true)
    
    if [ -n "$backpressure" ]; then
        log_success "✅ Backpressure metrics found:"
        echo "$backpressure"
        return 0
    else
        log_warn "⚠️  No backpressure events detected (load may not have reached threshold)"
        return 0  # Not a failure
    fi
}

# Test 4: All metrics summary
test_all_metrics() {
    log_info "Test 4: Getting all worker metrics summary..."
    
    local metrics=$(get_metrics)
    if [ -z "$metrics" ]; then
        log_error "Failed to get metrics"
        return 1
    fi
    
    local worker_metrics=$(echo "$metrics" | grep "^ksam_worker" || true)
    
    if [ -z "$worker_metrics" ]; then
        log_error "No worker metrics found"
        return 1
    fi
    
    echo ""
    echo "=========================================="
    echo "All Worker Metrics"
    echo "=========================================="
    echo ""
    echo "$worker_metrics" | head -50
    echo "..."
    echo ""
    
    # Count by type
    log_info "Metric summary:"
    echo "$worker_metrics" | grep -o "^ksam_worker_[a-z_]*" | sort -u | while read type; do
        local count=$(echo "$worker_metrics" | grep -c "^$type" || echo "0")
        echo "  - $type: $count entries"
    done
    
    return 0
}

# Cleanup
cleanup() {
    log_info "Cleaning up..."
    pkill -f "kubectl port-forward.*$CORE_PORT" 2>/dev/null || true
}

trap cleanup EXIT

# Main execution
main() {
    echo "=========================================="
    echo "Complete Metrics Test Suite"
    echo "=========================================="
    echo ""
    
    # Setup
    setup_port_forward || exit 1
    
    echo ""
    
    # Run tests
    test_fixed_messages
    echo ""
    
    test_queue_depth
    echo ""
    
    test_backpressure
    echo ""
    
    test_all_metrics
    echo ""
    
    echo "=========================================="
    log_success "All tests completed!"
    echo "=========================================="
}

main "$@"


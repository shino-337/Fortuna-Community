#!/bin/bash

# Complete script to publish test messages and verify metrics
# Uses Go script to publish messages via port-forward

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

NAMESPACE="ksam"
NATS_PORT="4222"
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

# Cleanup function
cleanup() {
    log_info "Cleaning up port-forwards..."
    pkill -f "kubectl port-forward.*$NATS_PORT" 2>/dev/null || true
    pkill -f "kubectl port-forward.*$CORE_PORT" 2>/dev/null || true
}

trap cleanup EXIT

# Get pod names
get_nats_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Setup port-forward for NATS
setup_nats_port_forward() {
    local pod=$(get_nats_pod)
    if [ -z "$pod" ]; then
        log_error "NATS pod not found"
        return 1
    fi
    
    log_info "Setting up NATS port-forward..."
    kubectl port-forward -n "$NAMESPACE" "$pod" $NATS_PORT:4222 >/dev/null 2>&1 &
    sleep 2
    
    # Test connection
    if ! nc -z localhost $NATS_PORT 2>/dev/null; then
        log_error "Failed to connect to NATS via port-forward"
        return 1
    fi
    
    log_success "NATS port-forward ready"
    return 0
}

# Setup port-forward for Core
setup_core_port_forward() {
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    log_info "Setting up Core port-forward..."
    kubectl port-forward -n "$NAMESPACE" "$pod" $CORE_PORT:8080 >/dev/null 2>&1 &
    sleep 2
    
    log_success "Core port-forward ready"
    return 0
}

# Publish test messages using test pod
publish_messages() {
    log_info "Publishing test messages to NATS..."
    
    # Use test pod script
    bash "$PROJECT_ROOT/scripts/publish_messages_via_testpod.sh"
    
    if [ $? -eq 0 ]; then
        log_success "Test messages published successfully"
        return 0
    else
        log_error "Failed to publish messages"
        return 1
    fi
}

# Get metrics from Core
get_metrics() {
    curl -s http://localhost:$CORE_PORT/metrics 2>/dev/null || echo ""
}

# Test metrics
test_metrics() {
    log_info "Waiting for messages to be processed..."
    sleep 10
    
    log_info "Checking metrics..."
    
    local metrics=$(get_metrics)
    if [ -z "$metrics" ]; then
        log_error "Failed to get metrics"
        return 1
    fi
    
    echo ""
    echo "=========================================="
    echo "Worker Metrics Results"
    echo "=========================================="
    echo ""
    
    # Queue Depth
    log_info "Queue Depth Metrics:"
    local queue_depth=$(echo "$metrics" | grep "^ksam_worker_queue_depth" || true)
    if [ -n "$queue_depth" ]; then
        echo "$queue_depth"
        log_success "✅ Queue depth metrics found"
    else
        log_warn "⚠️  Queue depth metrics not found"
    fi
    echo ""
    
    # Messages Processed
    log_info "Messages Processed Metrics:"
    local messages_processed=$(echo "$metrics" | grep "^ksam_worker_messages_processed_total" || true)
    if [ -n "$messages_processed" ]; then
        echo "$messages_processed" | head -10
        log_success "✅ Messages processed metrics found"
    else
        log_warn "⚠️  Messages processed metrics not found"
    fi
    echo ""
    
    # Processing Duration
    log_info "Processing Duration Metrics:"
    local processing_duration=$(echo "$metrics" | grep "^ksam_worker_processing_duration_seconds" || true)
    if [ -n "$processing_duration" ]; then
        echo "$processing_duration" | head -5
        log_success "✅ Processing duration metrics found"
    else
        log_warn "⚠️  Processing duration metrics not found"
    fi
    echo ""
    
    # Concurrent Processing
    log_info "Concurrent Processing Metrics:"
    local concurrent=$(echo "$metrics" | grep "^ksam_worker_concurrent_processing" || true)
    if [ -n "$concurrent" ]; then
        echo "$concurrent"
        log_success "✅ Concurrent processing metrics found"
    else
        log_warn "⚠️  Concurrent processing metrics not found"
    fi
    echo ""
    
    # Backpressure
    log_info "Backpressure Metrics:"
    local backpressure=$(echo "$metrics" | grep "^ksam_worker_backpressure" || true)
    if [ -n "$backpressure" ]; then
        echo "$backpressure"
        log_success "✅ Backpressure metrics found"
    else
        log_warn "⚠️  Backpressure metrics not found (no backpressure events)"
    fi
    echo ""
    
    # All Worker Metrics Summary
    log_info "All Worker Metrics Summary:"
    local all_worker=$(echo "$metrics" | grep "^ksam_worker" || true)
    if [ -n "$all_worker" ]; then
        echo "$all_worker" | head -30
        echo "..."
        local count=$(echo "$all_worker" | wc -l)
        log_success "✅ Found $count worker metric entries"
    else
        log_error "❌ No worker metrics found"
    fi
    echo ""
}

# Check worker logs
check_worker_logs() {
    log_info "Checking worker logs for processing..."
    
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    local logs=$(kubectl logs -n "$NAMESPACE" "$pod" --tail=50 | grep -E "Processing item|Normalized and published|processed message" || true)
    
    if [ -n "$logs" ]; then
        log_success "Processing logs found:"
        echo "$logs" | head -20
    else
        log_warn "No processing logs found"
    fi
    echo ""
}

# Main execution
main() {
    echo "=========================================="
    echo "Publish Test Messages & Verify Metrics"
    echo "=========================================="
    echo ""
    
    # Setup port-forwards
    setup_nats_port_forward || exit 1
    setup_core_port_forward || exit 1
    
    echo ""
    
    # Publish messages
    publish_messages || exit 1
    
    echo ""
    
    # Test metrics
    test_metrics
    
    # Check logs
    check_worker_logs
    
    echo "=========================================="
    log_success "Test completed!"
    echo "=========================================="
}

main "$@"


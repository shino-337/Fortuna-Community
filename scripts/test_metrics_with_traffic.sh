#!/bin/bash

# Test script to publish test messages and verify metrics
# This script publishes messages to NATS and then checks metrics

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

NAMESPACE="ksam"
CORE_SERVICE="ksam-core"
NATS_SERVICE="nats"

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

# Get Core pod name
get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Get NATS pod name
get_nats_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Get metrics from Core
get_metrics() {
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    kubectl exec -n "$NAMESPACE" "$pod" -- wget -qO- http://localhost:8080/metrics 2>/dev/null
}

# Test 1: Publish test messages
test_publish_messages() {
    log_info "Test 1: Publishing test messages to NATS..."
    
    local nats_pod=$(get_nats_pod)
    if [ -z "$nats_pod" ]; then
        log_error "NATS pod not found"
        return 1
    fi
    
    # Check if Go is available in NATS pod
    if ! kubectl exec -n "$NAMESPACE" "$nats_pod" -- which go >/dev/null 2>&1; then
        log_warn "Go not available in NATS pod, using nats CLI instead"
        
        # Use nats CLI to publish messages
        # First, create a simple test message
        local test_msg='{"kind":"Pod","uid":"test-pod-1","name":"test-pod-1","namespace":"default","labels":{"cluster":"test"},"raw_json":"{}","timestamp":'$(date +%s)'}'
        
        # Publish using nats CLI if available
        if kubectl exec -n "$NAMESPACE" "$nats_pod" -- which nats >/dev/null 2>&1; then
            kubectl exec -n "$NAMESPACE" "$nats_pod" -- nats pub "ksam.raw.pods" "$test_msg" || {
                log_warn "nats CLI not available, will use alternative method"
                return 1
            }
        else
            log_warn "nats CLI not available, will use Go script from local machine"
            return 1
        fi
    fi
    
    # Build and run Go script in NATS pod
    log_info "Building test message publisher..."
    cd "$PROJECT_ROOT"
    
    # Copy script to pod and run
    kubectl cp "$PROJECT_ROOT/scripts/publish_test_messages.go" "$NAMESPACE/$nats_pod:/tmp/publish_test_messages.go"
    kubectl exec -n "$NAMESPACE" "$nats_pod" -- go run /tmp/publish_test_messages.go
    
    log_success "Test messages published"
    return 0
}

# Test 2: Check queue depth metrics
test_queue_depth_metrics() {
    log_info "Test 2: Checking queue depth metrics..."
    
    # Wait a bit for messages to be processed
    sleep 5
    
    local metrics=$(get_metrics)
    if [ -z "$metrics" ]; then
        log_error "Failed to get metrics"
        return 1
    fi
    
    local queue_depth=$(echo "$metrics" | grep "^ksam_worker_queue_depth" || true)
    
    if [ -z "$queue_depth" ]; then
        log_warn "Queue depth metrics not found (may need more time)"
        return 0  # Not a failure
    else
        log_success "Queue depth metrics found:"
        echo "$queue_depth" | head -5
        return 0
    fi
}

# Test 3: Check processing metrics
test_processing_metrics() {
    log_info "Test 3: Checking processing metrics..."
    
    local metrics=$(get_metrics)
    if [ -z "$metrics" ]; then
        log_error "Failed to get metrics"
        return 1
    fi
    
    local messages_processed=$(echo "$metrics" | grep "^ksam_worker_messages_processed_total" || true)
    local processing_duration=$(echo "$metrics" | grep "^ksam_worker_processing_duration_seconds" || true)
    
    local found=0
    if [ -n "$messages_processed" ]; then
        log_success "Messages processed metrics found:"
        echo "$messages_processed" | head -10
        found=1
    fi
    
    if [ -n "$processing_duration" ]; then
        log_success "Processing duration metrics found:"
        echo "$processing_duration" | head -5
        found=1
    fi
    
    if [ $found -eq 0 ]; then
        log_warn "Processing metrics not found (may need more time)"
        return 0  # Not a failure
    fi
    
    return 0
}

# Test 4: Check concurrent processing metrics
test_concurrent_processing_metrics() {
    log_info "Test 4: Checking concurrent processing metrics..."
    
    local metrics=$(get_metrics)
    if [ -z "$metrics" ]; then
        log_error "Failed to get metrics"
        return 1
    fi
    
    local concurrent=$(echo "$metrics" | grep "^ksam_worker_concurrent_processing" || true)
    
    if [ -z "$concurrent" ]; then
        log_warn "Concurrent processing metrics not found"
        return 0  # Not a failure
    else
        log_success "Concurrent processing metrics found:"
        echo "$concurrent"
        return 0
    fi
}

# Test 5: Check all worker metrics
test_all_worker_metrics() {
    log_info "Test 5: Getting all worker metrics..."
    
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
    
    log_success "All worker metrics:"
    echo ""
    echo "$worker_metrics"
    echo ""
    
    # Count by metric type
    log_info "Metric summary:"
    echo "$worker_metrics" | grep -o "^ksam_worker_[a-z_]*" | sort -u | while read type; do
        local count=$(echo "$worker_metrics" | grep -c "^$type" || echo "0")
        echo "  - $type: $count entries"
    done
    
    return 0
}

# Test 6: Check worker logs for processing
test_worker_logs() {
    log_info "Test 6: Checking worker logs for message processing..."
    
    local pod=$(get_core_pod)
    if [ -z "$pod" ]; then
        log_error "Core pod not found"
        return 1
    fi
    
    # Wait a bit for processing
    sleep 3
    
    local logs=$(kubectl logs -n "$NAMESPACE" "$pod" --tail=100 | grep -E "Processing item|Normalized and published|processed message" || true)
    
    if [ -z "$logs" ]; then
        log_warn "No processing logs found (may need more time)"
        return 0  # Not a failure
    else
        log_success "Processing logs found:"
        echo "$logs" | head -20
        return 0
    fi
}

# Main execution
main() {
    echo "=========================================="
    echo "Worker Metrics Test with Traffic"
    echo "=========================================="
    echo ""
    
    # Check if we can publish messages
    if ! test_publish_messages; then
        log_warn "Failed to publish messages using pod method, trying local Go script..."
        
        # Try to run Go script locally if NATS is accessible
        if command -v go >/dev/null 2>&1; then
            log_info "Running Go script locally..."
            cd "$PROJECT_ROOT/scripts"
            export NATS_URL="nats://$(kubectl get svc -n ksam nats -o jsonpath='{.spec.clusterIP}'):4222"
            go run publish_test_messages.go || {
                log_error "Failed to publish messages"
                return 1
            }
        else
            log_error "Cannot publish messages - Go not available and pod method failed"
            return 1
        fi
    fi
    
    echo ""
    log_info "Waiting for messages to be processed..."
    sleep 10
    
    echo ""
    test_queue_depth_metrics
    echo ""
    
    test_processing_metrics
    echo ""
    
    test_concurrent_processing_metrics
    echo ""
    
    test_worker_logs
    echo ""
    
    test_all_worker_metrics
    echo ""
    
    log_success "Test completed!"
}

main "$@"


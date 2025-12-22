#!/bin/bash

# Test script for backpressure with high load
# Publishes >80 messages to trigger backpressure threshold

set -e

NAMESPACE="ksam"
TEST_POD="nats-backpressure-test"
NATS_SVC="nats.ksam.svc.cluster.local"
CORE_PORT="8080"

log_info() {
    echo "[INFO] $1"
}

log_success() {
    echo "[SUCCESS] $1"
}

log_error() {
    echo "[ERROR] $1"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test pod..."
    kubectl delete pod -n "$NAMESPACE" "$TEST_POD" --ignore-not-found=true 2>/dev/null || true
    pkill -f "kubectl port-forward.*$CORE_PORT" 2>/dev/null || true
}

trap cleanup EXIT

# Get Core pod
get_core_pod() {
    kubectl get pods -n "$NAMESPACE" -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

# Setup Core port-forward
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
}

# Get metrics
get_metrics() {
    curl -s http://localhost:$CORE_PORT/metrics 2>/dev/null || echo ""
}

# Check backpressure metrics
check_backpressure() {
    log_info "Checking backpressure metrics..."
    
    local metrics=$(get_metrics)
    if [ -z "$metrics" ]; then
        log_error "Failed to get metrics"
        return 1
    fi
    
    local backpressure=$(echo "$metrics" | grep "^ksam_worker_backpressure_total" || true)
    local concurrent=$(echo "$metrics" | grep "^ksam_worker_concurrent_processing" || true)
    
    echo ""
    echo "=========================================="
    echo "Backpressure Metrics"
    echo "=========================================="
    
    if [ -n "$backpressure" ]; then
        log_success "Backpressure events detected:"
        echo "$backpressure"
    else
        log_warn "No backpressure events yet"
    fi
    
    if [ -n "$concurrent" ]; then
        log_info "Current concurrent processing:"
        echo "$concurrent"
    fi
    
    echo ""
}

# Main execution
main() {
    echo "=========================================="
    echo "Backpressure Test - High Load"
    echo "=========================================="
    echo ""
    
    # Setup port-forward
    setup_core_port_forward || exit 1
    
    # Create test pod
    log_info "Creating test pod with nats-box..."
    kubectl run -n "$NAMESPACE" "$TEST_POD" \
        --image=natsio/nats-box:latest \
        --restart=Never \
        --command -- sleep 3600
    
    kubectl wait --for=condition=ready pod -n "$NAMESPACE" "$TEST_POD" --timeout=30s
    
    # Generate timestamp
    TIMESTAMP=$(date +%s)
    
    # Create temp directory
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT
    
    # Publish >80 messages rapidly to trigger backpressure
    log_info "Publishing 100 messages rapidly to trigger backpressure..."
    log_info "Backpressure threshold: 80 messages (80% of 100 max concurrent)"
    
    for i in {1..100}; do
        cat > "$TEMP_DIR/pod-$i.json" <<EOF
{
  "kind": "Pod",
  "uid": "backpressure-pod-$i",
  "name": "backpressure-pod-$i",
  "namespace": "default",
  "labels": {
    "cluster": "test",
    "backpressure-test": "true"
  },
  "raw_json": "{\"kind\":\"Pod\",\"metadata\":{\"name\":\"backpressure-pod-$i\",\"namespace\":\"default\",\"uid\":\"backpressure-pod-$i\"},\"spec\":{\"serviceAccountName\":\"default\"}}",
  "timestamp": $TIMESTAMP
}
EOF
        kubectl cp "$TEMP_DIR/pod-$i.json" "$NAMESPACE/$TEST_POD:/tmp/message.json" >/dev/null 2>&1
        kubectl exec -n "$NAMESPACE" "$TEST_POD" -- sh -c "cat /tmp/message.json | nats pub -s $NATS_SVC:4222 'ksam.raw.pods'" 2>&1 | grep -v "If you don't see" || true
        
        # Check metrics every 20 messages
        if [ $((i % 20)) -eq 0 ]; then
            log_info "Published $i messages, checking metrics..."
            check_backpressure
        fi
    done
    
    log_success "✅ Published 100 messages"
    
    # Wait for processing
    log_info "Waiting for processing and backpressure events..."
    sleep 15
    
    # Final check
    check_backpressure
    
    # Check worker logs for backpressure
    log_info "Checking worker logs for backpressure events..."
    local pod=$(get_core_pod)
    if [ -n "$pod" ]; then
        local logs=$(kubectl logs -n "$NAMESPACE" "$pod" --tail=50 | grep -i "backpressure\|NAKed\|entered backpressure" || true)
        if [ -n "$logs" ]; then
            log_success "Backpressure events in logs:"
            echo "$logs" | head -10
        else
            log_warn "No backpressure events in logs"
        fi
    fi
    
    echo ""
    log_success "Backpressure test completed!"
}

main "$@"


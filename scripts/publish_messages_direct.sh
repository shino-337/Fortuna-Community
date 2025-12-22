#!/bin/bash

# Publish messages directly using nats CLI with proper JSON encoding
# Uses printf to avoid shell escaping issues

set -e

NAMESPACE="ksam"
TEST_POD="nats-test-publisher"
NATS_SVC="nats.ksam.svc.cluster.local"

log_info() {
    echo "[INFO] $1"
}

log_success() {
    echo "[SUCCESS] $1"
}

# Cleanup
cleanup() {
    kubectl delete pod -n "$NAMESPACE" "$TEST_POD" --ignore-not-found=true 2>/dev/null || true
}

trap cleanup EXIT

# Create test pod
log_info "Creating test pod..."
kubectl run -n "$NAMESPACE" "$TEST_POD" \
    --image=natsio/nats-box:latest \
    --restart=Never \
    --command -- sleep 3600

kubectl wait --for=condition=ready pod -n "$NAMESPACE" "$TEST_POD" --timeout=30s

TIMESTAMP=$(date +%s)

# Function to publish with proper JSON
publish_json() {
    local subject=$1
    local json_data=$2
    
    # Use printf to avoid shell escaping, pipe to nats pub
    kubectl exec -n "$NAMESPACE" "$TEST_POD" -- sh -c "printf '%s' '$json_data' | nats pub -s $NATS_SVC:4222 '$subject'" 2>&1 | grep -v "If you don't see" || true
}

log_info "Publishing test messages..."

# Publish Pod messages
for i in {1..10}; do
    JSON='{"kind":"Pod","uid":"pod-uid-'$i'","name":"test-pod-'$i'","namespace":"default","labels":{"cluster":"test"},"raw_json":"{\"kind\":\"Pod\",\"metadata\":{\"name\":\"test-pod-'$i'\",\"namespace\":\"default\",\"uid\":\"pod-uid-'$i'\"},\"spec\":{\"serviceAccountName\":\"default\"}}","timestamp":'$TIMESTAMP'}'
    publish_json "ksam.raw.pods" "$JSON"
    sleep 0.1
done

# Publish ServiceAccount messages
for i in {1..5}; do
    JSON='{"kind":"ServiceAccount","uid":"sa-uid-'$i'","name":"test-sa-'$i'","namespace":"default","labels":{"cluster":"test"},"raw_json":"{\"kind\":\"ServiceAccount\",\"metadata\":{\"name\":\"test-sa-'$i'\",\"namespace\":\"default\",\"uid\":\"sa-uid-'$i'\"}}","timestamp":'$TIMESTAMP'}'
    publish_json "ksam.raw.serviceaccounts" "$JSON"
    sleep 0.1
done

log_success "✅ Published test messages"


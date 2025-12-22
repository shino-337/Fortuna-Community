#!/bin/bash

# Publish test messages using a test pod with nats-box
# This creates a single test pod and publishes all messages from it

set -e

NAMESPACE="ksam"
TEST_POD="nats-test-publisher"

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
}

trap cleanup EXIT

# Get NATS service
NATS_SVC="nats.ksam.svc.cluster.local"

log_info "Creating test pod with nats-box..."
kubectl run -n "$NAMESPACE" "$TEST_POD" \
    --image=natsio/nats-box:latest \
    --restart=Never \
    --command -- sleep 3600

# Wait for pod to be ready
log_info "Waiting for test pod to be ready..."
kubectl wait --for=condition=ready pod -n "$NAMESPACE" "$TEST_POD" --timeout=30s

# Generate test messages
TIMESTAMP=$(date +%s)

log_info "Publishing test messages..."

# Function to publish via test pod
publish_via_pod() {
    local subject=$1
    local message=$2
    
    kubectl exec -n "$NAMESPACE" "$TEST_POD" -- sh -c "echo '$message' | nats pub -s $NATS_SVC:4222 '$subject'" 2>&1 | grep -v "If you don't see" || true
}

# Publish Pod messages
log_info "Publishing 10 Pod messages..."
for i in {1..10}; do
    MSG="{\"kind\":\"Pod\",\"uid\":\"pod-uid-$i\",\"name\":\"test-pod-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_via_pod "ksam.raw.pods" "$MSG"
done

# Publish ServiceAccount messages
log_info "Publishing 5 ServiceAccount messages..."
for i in {1..5}; do
    MSG="{\"kind\":\"ServiceAccount\",\"uid\":\"sa-uid-$i\",\"name\":\"test-sa-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_via_pod "ksam.raw.serviceaccounts" "$MSG"
done

# Publish Role messages
log_info "Publishing 3 Role messages..."
for i in {1..3}; do
    MSG="{\"kind\":\"Role\",\"uid\":\"role-uid-$i\",\"name\":\"test-role-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_via_pod "ksam.raw.roles" "$MSG"
done

# Publish RoleBinding messages
log_info "Publishing 3 RoleBinding messages..."
for i in {1..3}; do
    MSG="{\"kind\":\"RoleBinding\",\"uid\":\"rb-uid-$i\",\"name\":\"test-rb-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_via_pod "ksam.raw.rolebindings" "$MSG"
done

# Publish more for metrics
log_info "Publishing 20 additional Pod messages for metrics..."
for i in {1..20}; do
    MSG="{\"kind\":\"Pod\",\"uid\":\"pod-uid-bulk-$i\",\"name\":\"test-pod-bulk-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_via_pod "ksam.raw.pods" "$MSG"
    sleep 0.05
done

log_success "✅ Published ~41 test messages"
echo "Waiting for processing..."


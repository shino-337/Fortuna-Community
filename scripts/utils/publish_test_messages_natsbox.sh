#!/bin/bash

# Publish test messages using nats-box image
# This script uses kubectl run with nats-box to publish messages

set -e

NAMESPACE="ksam"

log_info() {
    echo "[INFO] $1"
}

log_success() {
    echo "[SUCCESS] $1"
}

log_error() {
    echo "[ERROR] $1"
}

# Use NATS service name directly
NATS_SVC="nats.ksam.svc.cluster.local"

log_info "NATS service: $NATS_SVC"
log_info "Publishing test messages..."

# Generate timestamp
TIMESTAMP=$(date +%s)

# Get NATS pod
NATS_POD=$(kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$NATS_POD" ]; then
    log_error "NATS pod not found"
    exit 1
fi

log_info "NATS pod: $NATS_POD"

# Function to publish a message using exec into NATS pod
publish_message() {
    local subject=$1
    local message=$2
    local name=$3
    
    # Use kubectl exec to run nats CLI in NATS pod
    kubectl exec -n "$NAMESPACE" "$NATS_POD" -- sh -c "echo '$message' | nats pub '$subject'" 2>&1 | grep -v "If you don't see" || true
}

# Publish Pod messages
log_info "Publishing Pod messages..."
for i in {1..10}; do
    MSG="{\"kind\":\"Pod\",\"uid\":\"pod-uid-$i\",\"name\":\"test-pod-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_message "ksam.raw.pods" "$MSG" "pod-$i"
    sleep 0.1
done

# Publish ServiceAccount messages
log_info "Publishing ServiceAccount messages..."
for i in {1..5}; do
    MSG="{\"kind\":\"ServiceAccount\",\"uid\":\"sa-uid-$i\",\"name\":\"test-sa-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_message "ksam.raw.serviceaccounts" "$MSG" "sa-$i"
    sleep 0.1
done

# Publish Role messages
log_info "Publishing Role messages..."
for i in {1..3}; do
    MSG="{\"kind\":\"Role\",\"uid\":\"role-uid-$i\",\"name\":\"test-role-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_message "ksam.raw.roles" "$MSG" "role-$i"
    sleep 0.1
done

# Publish RoleBinding messages
log_info "Publishing RoleBinding messages..."
for i in {1..3}; do
    MSG="{\"kind\":\"RoleBinding\",\"uid\":\"rb-uid-$i\",\"name\":\"test-rb-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_message "ksam.raw.rolebindings" "$MSG" "rb-$i"
    sleep 0.1
done

# Publish more messages for backpressure test
log_info "Publishing additional messages for metrics..."
for i in {1..20}; do
    MSG="{\"kind\":\"Pod\",\"uid\":\"pod-uid-bulk-$i\",\"name\":\"test-pod-bulk-$i\",\"namespace\":\"default\",\"labels\":{\"cluster\":\"test\"},\"raw_json\":\"{}\",\"timestamp\":$TIMESTAMP}"
    publish_message "ksam.raw.pods" "$MSG" "pod-bulk-$i"
    sleep 0.05
done

log_success "✅ Published test messages"
echo ""
echo "Total messages published: ~41"
echo "Waiting for processing..."


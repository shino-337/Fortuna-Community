#!/bin/bash

# Fixed script to publish test messages with proper JSON format
# Uses file-based approach to avoid JSON escaping issues

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

# Generate timestamp
TIMESTAMP=$(date +%s)

# Function to publish a message using file-based approach
publish_message() {
    local subject=$1
    local json_file=$2
    
    # Copy JSON file to pod and publish
    kubectl cp "$json_file" "$NAMESPACE/$TEST_POD:/tmp/message.json" >/dev/null 2>&1
    kubectl exec -n "$NAMESPACE" "$TEST_POD" -- sh -c "cat /tmp/message.json | nats pub -s $NATS_SVC:4222 '$subject'" 2>&1 | grep -v "If you don't see" || true
}

# Create temp directory for JSON files
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

log_info "Publishing test messages with proper JSON format..."

# Publish Pod messages
log_info "Publishing 10 Pod messages..."
for i in {1..10}; do
    cat > "$TEMP_DIR/pod-$i.json" <<EOF
{
  "kind": "Pod",
  "uid": "pod-uid-$i",
  "name": "test-pod-$i",
  "namespace": "default",
  "labels": {
    "cluster": "test",
    "app": "test-app"
  },
  "raw_json": "{\"kind\":\"Pod\",\"metadata\":{\"name\":\"test-pod-$i\",\"namespace\":\"default\",\"uid\":\"pod-uid-$i\",\"labels\":{\"cluster\":\"test\"}},\"spec\":{\"serviceAccountName\":\"default\"}}",
  "timestamp": $TIMESTAMP
}
EOF
    publish_message "ksam.raw.pods" "$TEMP_DIR/pod-$i.json"
    sleep 0.1
done

# Publish ServiceAccount messages
log_info "Publishing 5 ServiceAccount messages..."
for i in {1..5}; do
    cat > "$TEMP_DIR/sa-$i.json" <<EOF
{
  "kind": "ServiceAccount",
  "uid": "sa-uid-$i",
  "name": "test-sa-$i",
  "namespace": "default",
  "labels": {
    "cluster": "test"
  },
  "raw_json": "{\"kind\":\"ServiceAccount\",\"metadata\":{\"name\":\"test-sa-$i\",\"namespace\":\"default\",\"uid\":\"sa-uid-$i\"}}",
  "timestamp": $TIMESTAMP
}
EOF
    publish_message "ksam.raw.serviceaccounts" "$TEMP_DIR/sa-$i.json"
    sleep 0.1
done

# Publish Role messages
log_info "Publishing 3 Role messages..."
for i in {1..3}; do
    cat > "$TEMP_DIR/role-$i.json" <<EOF
{
  "kind": "Role",
  "uid": "role-uid-$i",
  "name": "test-role-$i",
  "namespace": "default",
  "labels": {
    "cluster": "test"
  },
  "raw_json": "{\"kind\":\"Role\",\"metadata\":{\"name\":\"test-role-$i\",\"namespace\":\"default\",\"uid\":\"role-uid-$i\"},\"rules\":[]}",
  "timestamp": $TIMESTAMP
}
EOF
    publish_message "ksam.raw.roles" "$TEMP_DIR/role-$i.json"
    sleep 0.1
done

log_success "✅ Published test messages with proper JSON format"
echo "Waiting for processing..."


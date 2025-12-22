#!/bin/bash

# Publish vulnerable pod data directly to NATS to test risk engine
# This bypasses the agent and tests the risk evaluation directly

set -e

NAMESPACE="ksam"
TEST_NAMESPACE="vulnerable-pod-test"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Get NATS pod
NATS_POD=$(kubectl get pods -n "$NAMESPACE" -l app=nats -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$NATS_POD" ]; then
    log_warn "NATS pod not found, trying to use nats-box"
    # Try to create a nats-box pod
    kubectl run nats-box-test --image=natsio/nats-box --rm -i --restart=Never -n "$NAMESPACE" -- \
        sh -c "nats pub --server=nats://nats:4222 'ksam.raw.clusterrolebindings' '{\"kind\":\"ClusterRoleBinding\",\"name\":\"vulnerable-sa-cluster-admin\",\"namespace\":\"\",\"uid\":\"test-uid-123\",\"cluster_id\":\"minikube\",\"raw_json\":\"{\\\"metadata\\\":{\\\"name\\\":\\\"vulnerable-sa-cluster-admin\\\",\\\"namespace\\\":\\\"\\\"},\\\"roleRef\\\":{\\\"apiGroup\\\":\\\"rbac.authorization.k8s.io\\\",\\\"kind\\\":\\\"ClusterRole\\\",\\\"name\\\":\\\"cluster-admin\\\"},\\\"subjects\\\":[{\\\"kind\\\":\\\"ServiceAccount\\\",\\\"name\\\":\\\"vulnerable-sa\\\",\\\"namespace\\\":\\\"$TEST_NAMESPACE\\\"}]}\"}'" 2>&1 || log_warn "Failed to publish via nats-box"
    exit 0
fi

log_info "Using NATS pod: $NATS_POD"

# Publish ClusterRoleBinding data
log_info "Publishing ClusterRoleBinding data to NATS..."

CLUSTER_ROLE_BINDING_JSON='{
  "kind": "ClusterRoleBinding",
  "name": "vulnerable-sa-cluster-admin",
  "namespace": "",
  "uid": "test-uid-clusterrolebinding-123",
  "cluster_id": "minikube",
  "raw_json": "{\"metadata\":{\"name\":\"vulnerable-sa-cluster-admin\",\"namespace\":\"\"},\"roleRef\":{\"apiGroup\":\"rbac.authorization.k8s.io\",\"kind\":\"ClusterRole\",\"name\":\"cluster-admin\"},\"subjects\":[{\"kind\":\"ServiceAccount\",\"name\":\"vulnerable-sa\",\"namespace\":\"vulnerable-pod-test\"}]}"
}'

# Publish to ksam.raw.clusterrolebindings
kubectl exec -n "$NAMESPACE" "$NATS_POD" -- \
    nats pub --server=nats://localhost:4222 \
    'ksam.raw.clusterrolebindings' \
    "$CLUSTER_ROLE_BINDING_JSON" 2>&1 || {
    log_warn "Failed to publish via NATS pod, trying alternative method..."
    
    # Alternative: Use nats-box pod
    kubectl run nats-box-$(date +%s) --image=natsio/nats-box --rm -i --restart=Never -n "$NAMESPACE" -- \
        sh -c "nats pub --server=nats://nats:4222 'ksam.raw.clusterrolebindings' '$CLUSTER_ROLE_BINDING_JSON'" 2>&1 || log_warn "Failed to publish"
}

log_success "✅ Published ClusterRoleBinding data"

# Publish Pod data
log_info "Publishing Pod data to NATS..."

POD_JSON='{
  "kind": "Pod",
  "name": "vulnerable-pod",
  "namespace": "vulnerable-pod-test",
  "uid": "test-uid-pod-123",
  "cluster_id": "minikube",
  "raw_json": "{\"metadata\":{\"name\":\"vulnerable-pod\",\"namespace\":\"vulnerable-pod-test\"},\"spec\":{\"serviceAccountName\":\"vulnerable-sa\"}}"
}'

kubectl run nats-box-pod-$(date +%s) --image=natsio/nats-box --rm -i --restart=Never -n "$NAMESPACE" -- \
    sh -c "nats pub --server=nats://nats:4222 'ksam.raw.pods' '$POD_JSON'" 2>&1 || log_warn "Failed to publish Pod data"

log_success "✅ Published Pod data"

# Publish ServiceAccount data
log_info "Publishing ServiceAccount data to NATS..."

SERVICE_ACCOUNT_JSON='{
  "kind": "ServiceAccount",
  "name": "vulnerable-sa",
  "namespace": "vulnerable-pod-test",
  "uid": "test-uid-sa-123",
  "cluster_id": "minikube",
  "raw_json": "{\"metadata\":{\"name\":\"vulnerable-sa\",\"namespace\":\"vulnerable-pod-test\"}}"
}'

kubectl run nats-box-sa-$(date +%s) --image=natsio/nats-box --rm -i --restart=Never -n "$NAMESPACE" -- \
    sh -c "nats pub --server=nats://nats:4222 'ksam.raw.serviceaccounts' '$SERVICE_ACCOUNT_JSON'" 2>&1 || log_warn "Failed to publish ServiceAccount data"

log_success "✅ Published ServiceAccount data"

log_success "✅ All data published to NATS"
log_info "Waiting 10 seconds for workers to process..."
sleep 10

echo ""
log_info "Data has been published. Workers should process and create insights."
echo ""


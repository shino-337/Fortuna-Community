#!/bin/bash

# Generate Test Traffic for mTLS Testing
# Creates test resources to trigger Agent streaming

set -e

NAMESPACE="ksam"
TEST_PREFIX="ksam-mtls-test-$(date +%s)"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

cleanup() {
    log_info "Cleaning up test resources..."
    kubectl delete pod -n $NAMESPACE -l app=ksam-mtls-test --ignore-not-found=true >/dev/null 2>&1
    kubectl delete serviceaccount -n $NAMESPACE -l app=ksam-mtls-test --ignore-not-found=true >/dev/null 2>&1
    kubectl delete role -n $NAMESPACE -l app=ksam-mtls-test --ignore-not-found=true >/dev/null 2>&1
    kubectl delete rolebinding -n $NAMESPACE -l app=ksam-mtls-test --ignore-not-found=true >/dev/null 2>&1
}

trap cleanup EXIT

echo "=========================================="
echo "Generate Test Traffic for mTLS Testing"
echo "=========================================="
echo ""

# Test 1: Create test Pod
log_test "Test 1: Create test Pod"
cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: ${TEST_PREFIX}-pod
  namespace: $NAMESPACE
  labels:
    app: ksam-mtls-test
spec:
  containers:
  - name: test
    image: busybox:latest
    command: ["sleep", "60"]
  restartPolicy: Never
EOF

log_info "✅ Test pod created: ${TEST_PREFIX}-pod"
sleep 2
echo ""

# Test 2: Create test ServiceAccount
log_test "Test 2: Create test ServiceAccount"
cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${TEST_PREFIX}-sa
  namespace: $NAMESPACE
  labels:
    app: ksam-mtls-test
EOF

log_info "✅ Test ServiceAccount created: ${TEST_PREFIX}-sa"
sleep 2
echo ""

# Test 3: Create test Role
log_test "Test 3: Create test Role"
cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ${TEST_PREFIX}-role
  namespace: $NAMESPACE
  labels:
    app: ksam-mtls-test
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
EOF

log_info "✅ Test Role created: ${TEST_PREFIX}-role"
sleep 2
echo ""

# Test 4: Create test RoleBinding
log_test "Test 4: Create test RoleBinding"
cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ${TEST_PREFIX}-binding
  namespace: $NAMESPACE
  labels:
    app: ksam-mtls-test
subjects:
- kind: ServiceAccount
  name: ${TEST_PREFIX}-sa
  namespace: $NAMESPACE
roleRef:
  kind: Role
  name: ${TEST_PREFIX}-role
  apiGroup: rbac.authorization.k8s.io/v1
EOF

log_info "✅ Test RoleBinding created: ${TEST_PREFIX}-binding"
sleep 2
echo ""

# Test 5: Verify Agent is streaming
log_test "Test 5: Verify Agent is streaming"
log_info "Waiting for Agent to detect and stream test resources..."
sleep 5

AGENT_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$AGENT_POD" ]; then
    STREAMING_COUNT=$(kubectl logs -n $NAMESPACE $AGENT_POD --tail=50 --since=10s 2>/dev/null | grep -c "Successfully streamed" || echo "0")
    if [ "$STREAMING_COUNT" -gt 0 ] 2>/dev/null; then
        log_info "✅ Agent is streaming (detected $STREAMING_COUNT messages in last 10s)"
        kubectl logs -n $NAMESPACE $AGENT_POD --tail=10 --since=10s 2>/dev/null | grep "Successfully streamed" | head -3
    else
        log_warn "⚠️  No streaming detected yet (may take a few more seconds)"
        log_info "   Agent logs (last 20 lines):"
        kubectl logs -n $NAMESPACE $AGENT_POD --tail=20 2>/dev/null | tail -5
    fi
else
    log_warn "⚠️  Agent pod not found"
fi
echo ""

# Test 6: Monitor traffic for a period
log_test "Test 6: Monitor traffic generation"
log_info "Monitoring for 10 seconds to ensure traffic is generated..."
for i in {1..10}; do
    STREAMING=$(kubectl logs -n $NAMESPACE $AGENT_POD --tail=5 --since=2s 2>/dev/null | grep -c "Successfully streamed" || echo "0")
    if [ "$STREAMING" -gt 0 ] 2>/dev/null; then
        echo -n "."
    else
        echo -n "_"
    fi
    sleep 1
done
echo ""

FINAL_COUNT=$(kubectl logs -n $NAMESPACE $AGENT_POD --tail=100 --since=15s 2>/dev/null | grep -c "Successfully streamed" || echo "0")
if [ "$FINAL_COUNT" -gt 0 ] 2>/dev/null; then
    log_info "✅ Traffic generated: $FINAL_COUNT messages in last 15s"
else
    log_warn "⚠️  Limited traffic detected"
fi
echo ""

echo "=========================================="
log_info "✅ Test traffic generation completed"
echo "=========================================="
log_info "Test resources created:"
log_info "  - Pod: ${TEST_PREFIX}-pod"
log_info "  - ServiceAccount: ${TEST_PREFIX}-sa"
log_info "  - Role: ${TEST_PREFIX}-role"
log_info "  - RoleBinding: ${TEST_PREFIX}-binding"
echo ""
log_info "These resources will trigger Agent to stream data to Core"
log_info "Use this script before running packet capture tests"
echo ""



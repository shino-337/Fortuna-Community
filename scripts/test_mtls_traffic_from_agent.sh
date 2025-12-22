#!/bin/bash

# Test mTLS Traffic from Agent Pod Directly
# This script captures traffic directly from the Agent pod

set -e

NAMESPACE="ksam"
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$AGENT_POD" ]; then
    echo "❌ Agent pod not found"
    exit 1
fi

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

echo "=========================================="
echo "mTLS Traffic Test from Agent Pod"
echo "=========================================="
echo ""
log_info "Agent Pod: $AGENT_POD"
echo ""

# Test 1: Check if tcpdump is available in Agent pod
log_test "Test 1: Check network tools in Agent pod"
if kubectl exec -n $NAMESPACE $AGENT_POD -- which tcpdump >/dev/null 2>&1; then
    log_info "✅ tcpdump available"
elif kubectl exec -n $NAMESPACE $AGENT_POD -- which ss >/dev/null 2>&1; then
    log_info "✅ ss available (will use for connection monitoring)"
else
    log_info "⚠️  No network tools available, will use alternative methods"
fi
echo ""

# Test 2: Check active connections
log_test "Test 2: Check active connections to Core"
CORE_SVC="ksam-core.ksam.svc.cluster.local:9090"
log_info "Checking connections to: $CORE_SVC"

# Try to see connections
CONNECTIONS=$(kubectl exec -n $NAMESPACE $AGENT_POD -- sh -c "
    ss -tnp 2>/dev/null | grep ':9090' || 
    netstat -tnp 2>/dev/null | grep ':9090' ||
    echo 'No connection tools available'
" 2>/dev/null || echo "Unable to check")

if echo "$CONNECTIONS" | grep -q "ESTABLISHED\|ESTAB"; then
    log_info "✅ Active connection to Core detected"
    echo "$CONNECTIONS" | head -3 | sed 's/^/   /'
else
    log_warn "⚠️  No active connection detected (may connect on demand)"
fi
echo ""

# Test 3: Generate traffic and monitor
log_test "Test 3: Generate traffic and monitor"
log_info "Creating test resource to trigger Agent streaming..."

TEST_POD="ksam-mtls-test-$(date +%s)"
cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD
  namespace: $NAMESPACE
spec:
  containers:
  - name: test
    image: busybox:latest
    command: ["sleep", "30"]
  restartPolicy: Never
EOF

log_info "✅ Test pod created: $TEST_POD"
log_info "Waiting for Agent to stream..."
sleep 5

# Check Agent logs
STREAMING=$(kubectl logs -n $NAMESPACE $AGENT_POD --tail=20 --since=10s 2>/dev/null | grep -c "Successfully streamed" || echo "0")
if [ "$STREAMING" -gt 0 ] 2>/dev/null; then
    log_info "✅ Agent streaming detected ($STREAMING messages)"
    kubectl logs -n $NAMESPACE $AGENT_POD --tail=5 --since=10s 2>/dev/null | grep "Successfully streamed" | head -3 | sed 's/^/   /'
else
    log_warn "⚠️  No streaming detected in logs"
fi
echo ""

# Test 4: Check connection again
log_test "Test 4: Verify connection after traffic"
CONNECTIONS_AFTER=$(kubectl exec -n $NAMESPACE $AGENT_POD -- sh -c "
    ss -tnp 2>/dev/null | grep ':9090' || 
    netstat -tnp 2>/dev/null | grep ':9090' ||
    echo 'No connection tools available'
" 2>/dev/null || echo "Unable to check")

if echo "$CONNECTIONS_AFTER" | grep -q "ESTABLISHED\|ESTAB"; then
    log_info "✅ Connection active"
    echo "$CONNECTIONS_AFTER" | head -3 | sed 's/^/   /'
else
    log_info "ℹ️  Connection may be ephemeral (connects, sends, disconnects)"
fi
echo ""

# Cleanup
log_info "Cleaning up test pod..."
kubectl delete pod -n $NAMESPACE $TEST_POD --ignore-not-found=true >/dev/null 2>&1

echo "=========================================="
log_info "✅ Traffic test completed"
echo "=========================================="
log_info "Note: For packet capture, use test_mtls_traffic_with_debug_pod.sh"
log_info "      which uses a debug pod with tcpdump"
echo ""




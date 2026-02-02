#!/bin/bash
# Comprehensive test script for pod risk flow
# Tests: Agent sync -> Core processing -> Database -> API -> Dashboard

set -e

POD_NAME="${1:-test-pod-risk-$(date +%s)}"
NAMESPACE="${2:-default}"
TIMEOUT="${TIMEOUT:-300}"  # 5 minutes

echo "=========================================="
echo "Pod Risk Flow Test"
echo "=========================================="
echo "Pod Name: $POD_NAME"
echo "Namespace: $NAMESPACE"
echo "Timeout: ${TIMEOUT}s"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} ✅ $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} ❌ $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} ⚠️  $1"; }

# Step 1: Create test pod
log_info "Step 1: Creating test pod..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $NAMESPACE
  labels:
    app: test-pod-risk
    test: "true"
spec:
  containers:
  - name: test-container
    image: busybox:latest
    command: ['sh', '-c', 'sleep 3600']
    securityContext:
      privileged: false
      capabilities:
        add:
        - SYS_ADMIN
        - NET_ADMIN
  restartPolicy: Never
EOF

if kubectl wait --for=condition=Ready pod/$POD_NAME -n $NAMESPACE --timeout=60s 2>/dev/null; then
  log_success "Pod created and running"
  POD_UID=$(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.metadata.uid}')
  echo "Pod UID: $POD_UID"
else
  log_error "Pod failed to start"
  exit 1
fi

echo ""

# Step 2: Wait for agent sync
log_info "Step 2: Waiting for agent sync (max ${TIMEOUT}s)..."
AGENT_POD=$(kubectl get pods -n fortuna -l app=fortuna-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$AGENT_POD" ]; then
  log_warning "No agent pod found, skipping agent sync check"
else
  log_info "Agent pod: $AGENT_POD"
  echo "Checking agent logs for sync..."
  kubectl logs -n fortuna $AGENT_POD --tail=50 | grep -i "sync\|$POD_NAME" || log_warning "No sync logs found yet"
fi

echo ""

# Step 3: Check database
log_info "Step 3: Checking database..."
sleep 10  # Wait for sync

# Setup port-forward if needed
if ! pgrep -f "port-forward.*postgres" > /dev/null; then
  log_info "Starting PostgreSQL port-forward..."
  kubectl port-forward -n fortuna svc/postgres 5432:5432 > /tmp/postgres-pf.log 2>&1 &
  sleep 3
fi

echo "--- Pod in database ---"
kubectl exec -n fortuna $(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d fortuna -c "SELECT name, uid, namespace, cluster_id, created_at FROM pods WHERE name = '$POD_NAME' OR uid = '$POD_UID' LIMIT 1;" 2>/dev/null || \
  log_warning "Cannot query pods table"

echo ""
echo "--- Pod Capabilities ---"
kubectl exec -n fortuna $(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d fortuna -c "SELECT pod_name, pod_uid, capability_id, severity, state, created_at FROM pod_capabilities WHERE pod_name = '$POD_NAME' OR pod_uid = '$POD_UID' ORDER BY created_at DESC LIMIT 5;" 2>/dev/null || \
  log_warning "Cannot query pod_capabilities table"

echo ""
echo "--- Runtime Signals ---"
kubectl exec -n fortuna $(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- \
  psql -U postgres -d fortuna -c "SELECT pod_name, pod_uid, signal_type, severity, detected_at FROM runtime_signals WHERE pod_name = '$POD_NAME' OR pod_uid = '$POD_UID' ORDER BY detected_at DESC LIMIT 5;" 2>/dev/null || \
  log_warning "Cannot query runtime_signals table"

echo ""

# Step 4: Check Core API
log_info "Step 4: Checking Core API..."

CORE_SVC="http://localhost:8080"
if ! curl -s "$CORE_SVC/healthz" > /dev/null 2>&1; then
  log_warning "Core API not accessible on localhost:8080, trying to port-forward..."
  kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 > /tmp/core-pf.log 2>&1 &
  sleep 3
fi

echo "--- Pod Capabilities API ---"
curl -s "$CORE_SVC/api/v1/pod-capabilities?podUid=$POD_UID" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30 || log_warning "API error"

echo ""
echo "--- Pod Capabilities API (by podName) ---"
curl -s "$CORE_SVC/api/v1/pod-capabilities?podName=$POD_NAME" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30 || log_warning "API error"

echo ""
echo "--- Runtime Signals API ---"
curl -s "$CORE_SVC/api/v1/runtime-signals?podUid=$POD_UID" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30 || log_warning "API error"

echo ""

# Step 5: Check Core logs
log_info "Step 5: Checking Core logs..."
CORE_POD=$(kubectl get pods -n fortuna -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$CORE_POD" ]; then
  log_info "Core pod: $CORE_POD"
  echo "Recent Core logs (last 30 lines):"
  kubectl logs -n fortuna $CORE_POD --tail=30 | grep -i "pod\|capability\|signal\|$POD_NAME" || log_warning "No relevant logs found"
else
  log_warning "No Core pod found"
fi

echo ""

# Step 6: Check Agent logs
log_info "Step 6: Checking Agent logs..."
if [ -n "$AGENT_POD" ]; then
  echo "Recent Agent logs (last 30 lines):"
  kubectl logs -n fortuna $AGENT_POD --tail=30 | grep -i "sync\|pod\|$POD_NAME" || log_warning "No relevant logs found"
else
  log_warning "No Agent pod found"
fi

echo ""

# Step 7: Summary
log_info "Step 7: Summary"
echo "=========================================="
echo "Test Pod Information:"
echo "  Name: $POD_NAME"
echo "  UID:  $POD_UID"
echo "  Namespace: $NAMESPACE"
echo ""
echo "To check manually:"
echo "  kubectl get pod $POD_NAME -n $NAMESPACE"
echo "  ./scripts/check-pod-risk.sh $POD_NAME"
echo "  curl -s $CORE_SVC/api/v1/pod-capabilities?podName=$POD_NAME | jq"
echo ""
echo "To cleanup:"
echo "  kubectl delete pod $POD_NAME -n $NAMESPACE"
echo "=========================================="

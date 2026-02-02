#!/bin/bash
# Test complete flow: Pod creation -> Agent sync -> Core PCE -> Dashboard
# Usage: ./scripts/test-pod-sync-flow.sh [pod-name]

set -e

POD_NAME="${1:-test-sync-$(date +%s)}"
NAMESPACE="${2:-default}"

echo "=========================================="
echo "Pod Sync Flow Test"
echo "=========================================="
echo "Pod Name: $POD_NAME"
echo "Namespace: $NAMESPACE"
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

# Step 1: Create pod
log_info "Step 1: Creating pod..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $NAMESPACE
  labels:
    app: test-sync
    test: "true"
spec:
  containers:
  - name: test-container
    image: busybox:latest
    command: ['sh', '-c', 'sleep 3600']
    securityContext:
      privileged: true
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
log_info "Step 2: Waiting for agent sync (30s)..."
sleep 30

AGENT_POD=$(kubectl get pods -n fortuna -l app=fortuna-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$AGENT_POD" ]; then
  log_info "Checking agent logs for sync..."
  kubectl logs -n fortuna $AGENT_POD --tail=20 | grep -i "$POD_NAME\|sync" || log_warning "No sync logs found"
fi

echo ""

# Step 3: Check database
log_info "Step 3: Checking database..."
POSTGRES_POD=$(kubectl get pods -n fortuna -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$POSTGRES_POD" ]; then
  log_error "PostgreSQL pod not found"
  exit 1
fi

echo "--- Pod in database ---"
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "
  SELECT name, uid, namespace, cluster_id, created_at 
  FROM pods 
  WHERE name = '$POD_NAME' OR uid = '$POD_UID' 
  LIMIT 1;
" 2>/dev/null || log_warning "Pod not in database yet"

echo ""
echo "--- Pod Capabilities (should appear after PCE) ---"
sleep 10  # Wait for PCE evaluation
kubectl exec -n fortuna $POSTGRES_POD -- psql -U postgres -d fortuna -c "
  SELECT pc.pod_uid, pc.capability_id, pc.severity, p.name as pod_name
  FROM pod_capabilities pc
  LEFT JOIN pods p ON p.uid = pc.pod_uid AND p.deleted_at IS NULL
  WHERE pc.pod_uid = '$POD_UID' OR p.name = '$POD_NAME'
  ORDER BY pc.created_at DESC 
  LIMIT 10;
" 2>/dev/null || log_warning "No capabilities found yet"

echo ""

# Step 4: Check Core API
log_info "Step 4: Checking Core API..."
if ! pgrep -f "port-forward.*fortuna-core.*8080" > /dev/null; then
  kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 > /tmp/core-pf.log 2>&1 &
  sleep 3
fi

CORE_API="http://localhost:8080"

echo "--- Pod Capabilities API (by podUid) ---"
curl -s "$CORE_API/api/v1/pod-capabilities?podUid=$POD_UID" 2>/dev/null | \
  python3 -c "import sys, json; d=json.load(sys.stdin); caps=d.get('capabilities',[]); print(f'Total: {len(caps)}'); [print(f\"  podName: {c.get('podName','N/A')} | podUid: {c.get('podUid','')[:20]}... | capability: {c.get('capabilityId','')} | severity: {c.get('severity','')}\") for c in caps[:5]]" 2>/dev/null || \
  log_warning "API error"

echo ""
echo "--- Pod Capabilities API (by podName) ---"
curl -s "$CORE_API/api/v1/pod-capabilities?podName=$POD_NAME" 2>/dev/null | \
  python3 -c "import sys, json; d=json.load(sys.stdin); caps=d.get('capabilities',[]); matches=[c for c in caps if c.get('podName','')== '$POD_NAME' or c.get('podUid','')=='$POD_UID']; print(f'Total: {len(caps)}, Matches: {len(matches)}'); [print(f\"  podName: {c.get('podName','N/A')} | capability: {c.get('capabilityId','')}\") for c in matches[:5]]" 2>/dev/null || \
  log_warning "API error"

echo ""

# Step 5: Check Core logs
log_info "Step 5: Checking Core logs..."
CORE_POD=$(kubectl get pods -n fortuna -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$CORE_POD" ]; then
  log_info "Core pod: $CORE_POD"
  echo "Recent Core logs (PCE evaluation):"
  kubectl logs -n fortuna $CORE_POD --tail=50 | grep -i "pce\|capability\|$POD_NAME\|evaluate" | tail -10 || log_warning "No PCE logs found"
else
  log_warning "No Core pod found"
fi

echo ""

# Step 6: Summary
log_info "Step 6: Summary"
echo "=========================================="
echo "Test Results:"
echo "  Pod Name: $POD_NAME"
echo "  Pod UID:  $POD_UID"
echo ""
echo "To verify manually:"
echo "  kubectl get pod $POD_NAME -n $NAMESPACE"
echo "  ./scripts/verify-pod-data.sh $POD_NAME"
echo "  curl -s $CORE_API/api/v1/pod-capabilities?podName=$POD_NAME | jq"
echo ""
echo "To cleanup:"
echo "  kubectl delete pod $POD_NAME -n $NAMESPACE"
echo "=========================================="

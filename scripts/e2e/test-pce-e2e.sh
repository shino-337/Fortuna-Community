#!/usr/bin/env bash

# E2E Test for PCE + Runtime Signals
# Tests capability evaluation, state transitions, and runtime signal processing

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
TEST_POD_NAME="pce-test-pod-$(date +%s)"
PCE_SYNC_TIMEOUT_SECONDS="${PCE_SYNC_TIMEOUT_SECONDS:-420}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "PCE + Runtime Signals E2E Test"
echo "=========================================="
echo ""

# Step 1: Create test pod with privileged container
echo "Step 1: Creating test pod with privileged container..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${TEST_POD_NAME}
  namespace: ${NAMESPACE}
spec:
  containers:
  - name: test
    image: alpine:latest
    command: ["sleep", "3600"]
    securityContext:
      privileged: true
  hostPID: true
  hostNetwork: true
EOF

echo "✅ Test pod created: ${TEST_POD_NAME}"
echo ""

# Step 2: Wait for pod to be ready
echo "Step 2: Waiting for pod to be ready..."
kubectl -n "${NAMESPACE}" wait --for=condition=Ready "pod/${TEST_POD_NAME}" --timeout=60s || true
POD_UID=$(kubectl -n "${NAMESPACE}" get pod "${TEST_POD_NAME}" -o jsonpath='{.metadata.uid}')
echo "✅ Pod UID: ${POD_UID}"
echo ""

# Step 2b: Wait for pod to appear in Core (agent sync) so PCE can run
echo "Step 2b: Waiting for pod in Core (agent sync, up to ${PCE_SYNC_TIMEOUT_SECONDS}s)..."
DB_POD="$(require_postgres_pod)"
if WAIT_POD="$(wait_for_pod_in_db "$POD_UID" "$PCE_SYNC_TIMEOUT_SECONDS" "$DB_POD")"; then
  echo "✅ Pod found in Core after ${WAIT_POD}s"
else
  echo "❌ Pod ${POD_UID} not found in DB after ${PCE_SYNC_TIMEOUT_SECONDS}s"
  kubectl -n "${NAMESPACE}" delete pod "${TEST_POD_NAME}" --wait=false >/dev/null 2>&1 || true
  exit 1
fi
echo ""

# Step 3: Wait for PCE evaluation (capabilities written after sync)
echo "Step 3: Waiting for PCE evaluation (up to 90s)..."
CAP_COUNT=0
for _ in $(seq 1 18); do
  CAP_COUNT=$(kubectl -n "${NAMESPACE}" exec "${DB_POD}" -- psql -U postgres -d fortuna -t -A -c \
    "SELECT COUNT(*) FROM pod_capabilities WHERE pod_uid = '${POD_UID}';" 2>/dev/null | tr -d ' ')
  CAP_COUNT=${CAP_COUNT:-0}
  if [ "$CAP_COUNT" -gt 0 ] 2>/dev/null; then
    break
  fi
  sleep 5
done

# Step 4: Check capabilities in database
echo "Step 4: Checking capabilities in database..."
CAPS=$(kubectl -n "${NAMESPACE}" exec "${DB_POD}" -- psql -U postgres -d fortuna -t -A -F '|' -c "
  SELECT capability_id, state, severity 
  FROM pod_capabilities 
  WHERE pod_uid = '${POD_UID}' 
  ORDER BY capability_id;
" 2>&1)

if [ "${CAP_COUNT:-0}" -eq 0 ] 2>/dev/null; then
  echo "❌ No capabilities found for pod ${POD_UID}"
  kubectl -n "${NAMESPACE}" delete pod "${TEST_POD_NAME}" --wait=false >/dev/null 2>&1 || true
  exit 1
fi

echo "✅ Capabilities found:"
echo "$CAPS" | grep -v "^$"
echo ""

# Step 5: Verify expected capabilities
echo "Step 5: Verifying expected capabilities..."
EXPECTED_CAPS=("ESC_PRIV_POD" "ESC_HOSTPID_POD" "NET_HOSTNETWORK" "ID_TOKEN_POD")
for cap in "${EXPECTED_CAPS[@]}"; do
  if echo "$CAPS" | grep -q "$cap"; then
    echo "✅ Found capability: $cap"
  else
    echo "⚠️  Missing capability: $cap"
  fi
done
echo ""

# Step 6: Send runtime event (simulate proc root pivot)
echo "Step 6: Sending runtime event (PROC_ROOT_PIVOT)..."
CORE_POD="$(require_core_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

RUNTIME_EVENT=$(cat <<EOF
[{
  "pod_uid": "${POD_UID}",
  "namespace": "${NAMESPACE}",
  "syscall": "open",
  "target_path": "/proc/1/root",
  "capability": "",
  "timestamp": $(date +%s)
}]
EOF
)

RESPONSE=$(kubectl -n "${NAMESPACE}" exec "${CORE_POD}" -- curl -s -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "${RUNTIME_EVENT}" \
  http://localhost:8080/api/v1/runtime-events 2>&1)

if echo "$RESPONSE" | grep -q "processed"; then
  echo "✅ Runtime event sent successfully"
  echo "Response: $RESPONSE"
else
  echo "⚠️  Runtime event may have failed: $RESPONSE"
fi
echo ""

# Step 7: Wait for signal processing
echo "Step 7: Waiting for signal processing (5s)..."
sleep 5

# Step 8: Check runtime signals
echo "Step 8: Checking runtime signals..."
SIGNALS=$(kubectl -n "${NAMESPACE}" exec "${DB_POD}" -- psql -U postgres -d fortuna -t -A -F '|' -c "
  SELECT signal_type, category, confidence 
  FROM runtime_signals 
  WHERE pod_uid = '${POD_UID}' 
  ORDER BY created_at DESC 
  LIMIT 5;
" 2>&1)

if [ -n "$SIGNALS" ] && [ "$SIGNALS" != "" ]; then
  echo "✅ Runtime signals found:"
  echo "$SIGNALS" | grep -v "^$"
else
  echo "⚠️  No runtime signals found"
fi
echo ""

# Step 9: Check capability state transitions
echo "Step 9: Checking capability state transitions..."
UPDATED_CAPS=$(kubectl -n "${NAMESPACE}" exec "${DB_POD}" -- psql -U postgres -d fortuna -t -A -F '|' -c "
  SELECT capability_id, state, confidence, last_seen_at 
  FROM pod_capabilities 
  WHERE pod_uid = '${POD_UID}' 
  AND state IN ('confirmed', 'exploited')
  ORDER BY capability_id;
" 2>&1)

if [ -n "$UPDATED_CAPS" ] && [ "$UPDATED_CAPS" != "" ]; then
  echo "✅ Capabilities with state transitions:"
  echo "$UPDATED_CAPS" | grep -v "^$"
else
  echo "ℹ️  No state transitions yet (may need more signals)"
fi
echo ""

# Step 10: Check pod risk profile
echo "Step 10: Checking pod risk profile..."
PROFILE=$(kubectl -n "${NAMESPACE}" exec "${DB_POD}" -- psql -U postgres -d fortuna -t -A -F '|' -c "
  SELECT static_risk, runtime_score, capabilities 
  FROM pod_risk_profiles 
  WHERE pod_uid = '${POD_UID}';
" 2>&1)

if [ -n "$PROFILE" ] && [ "$PROFILE" != "" ]; then
  echo "✅ Pod risk profile:"
  echo "$PROFILE" | grep -v "^$"
else
  echo "⚠️  No risk profile found"
fi
echo ""

# Step 11: Cleanup
echo "Step 11: Cleaning up test pod..."
kubectl -n "${NAMESPACE}" delete pod "${TEST_POD_NAME}" --wait=false || true
echo "✅ Test pod deleted"
echo ""

echo "=========================================="
echo "E2E Test Complete"
echo "=========================================="

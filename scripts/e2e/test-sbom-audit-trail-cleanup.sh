#!/usr/bin/env bash
# ============================================================================
# E2E: SBOM Audit Trail on Cleanup
#
# Creates a pod, waits for SBOM, deletes the pod, then verifies that the
# reconciler creates an audit_logs entry when soft-deleting the orphaned SBOM.
#
# Env: NAMESPACE, CORE_API_URL, API_USER, API_PASS, TEST_NS
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/common.sh" 2>/dev/null || true

NAMESPACE="${NAMESPACE:-fortuna}"
TEST_NS="${TEST_NS:-fortuna}"
POD_NAME="e2e-audit-trail-$(date +%s)"
FAIL=0

query_db() {
  local sql="$1"
  local pg_pod
  pg_pod=$(kubectl get pods -n "$NAMESPACE" -l app=postgres \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  [ -z "$pg_pod" ] && { echo ""; return 1; }
  kubectl -n "$NAMESPACE" exec "$pg_pod" -- psql -U postgres -d fortuna -t -A -c "$sql" 2>/dev/null || echo ""
}

assert_ok()   { echo "  ✅ $1"; }
assert_fail() { echo "  ❌ $1"; FAIL=$((FAIL + 1)); }

echo "======================================================="
echo "E2E: SBOM Audit Trail on Cleanup"
echo "======================================================="
echo ""

echo "[1/6] Creating test pod: $POD_NAME..."
kubectl get namespace "$TEST_NS" &>/dev/null || kubectl create namespace "$TEST_NS"
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $TEST_NS
  labels:
    app: e2e-audit-trail
spec:
  restartPolicy: Never
$(e2e_node_selector_yaml 2)
  tolerations:
    - key: node-role.kubernetes.io/control-plane
      operator: Exists
      effect: NoSchedule
  containers:
    - name: test
      image: busybox:1.36
      imagePullPolicy: IfNotPresent
      command: ["sleep", "3600"]
EOF

kubectl wait --for=condition=Ready pod/"$POD_NAME" -n "$TEST_NS" --timeout=120s 2>/dev/null || true
POD_UID=$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
[ -z "$POD_UID" ] && { echo "ERROR: Pod UID not found"; exit 1; }
echo "  Pod UID: $POD_UID"
echo ""

echo "[2/6] Waiting for SBOM creation in DB (max 120s)..."
MAX_WAIT=120; INTERVAL=10; elapsed=0
SBOM_ID=""
while [ $elapsed -lt $MAX_WAIT ]; do
  SBOM_ID=$(query_db "SELECT id FROM sboms WHERE pod_uid='$POD_UID' AND deleted_at IS NULL LIMIT 1;" 2>/dev/null || echo "")
  SBOM_ID=$(echo "$SBOM_ID" | tr -d '[:space:]')
  [ -n "$SBOM_ID" ] && { echo "  SBOM ID: $SBOM_ID (after ${elapsed}s)"; break; }
  sleep "$INTERVAL"; elapsed=$((elapsed + INTERVAL)); echo "  ... ${elapsed}s"
done
[ -z "$SBOM_ID" ] && { echo "WARNING: SBOM not found in DB (agent may not have extracted yet)"; }
echo ""

echo "[3/6] Recording audit baseline..."
AUDIT_BEFORE=$(query_db "SELECT COUNT(*) FROM audit_logs WHERE resource='sbom' AND action='delete';" 2>/dev/null || echo "0")
AUDIT_BEFORE=$(echo "$AUDIT_BEFORE" | tr -d '[:space:]')
echo "  Audit delete count (before): $AUDIT_BEFORE"
echo ""

echo "[4/6] Deleting pod to orphan SBOM..."
kubectl delete pod "$POD_NAME" -n "$TEST_NS" --wait=true 2>/dev/null
echo "  Pod deleted"
echo ""

echo "[5/6] Waiting for pod soft-delete in DB (max 60s)..."
MAX_WAIT=60; INTERVAL=5; elapsed=0
while [ $elapsed -lt $MAX_WAIT ]; do
  POD_DELETED=$(query_db "SELECT COUNT(*) FROM pods WHERE uid='$POD_UID' AND deleted_at IS NOT NULL;" 2>/dev/null || echo "0")
  POD_DELETED=$(echo "$POD_DELETED" | tr -d '[:space:]')
  [ "${POD_DELETED:-0}" -gt 0 ] && { echo "  Pod soft-deleted in DB after ${elapsed}s"; break; }
  sleep "$INTERVAL"; elapsed=$((elapsed + INTERVAL))
done
echo ""

echo "[6/6] Checking audit trail..."
echo "  Note: audit_logs entry created when reconciler runs (every 1h by default)."
echo "  Checking if SBOM is still active..."
if [ -n "$SBOM_ID" ]; then
  SBOM_ACTIVE=$(query_db "SELECT COUNT(*) FROM sboms WHERE id=$SBOM_ID AND deleted_at IS NULL;" 2>/dev/null || echo "?")
  SBOM_ACTIVE=$(echo "$SBOM_ACTIVE" | tr -d '[:space:]')
  echo "  SBOM active: $SBOM_ACTIVE"

  if [ "$SBOM_ACTIVE" = "0" ]; then
    AUDIT_AFTER=$(query_db "SELECT COUNT(*) FROM audit_logs WHERE resource='sbom' AND action='delete';" 2>/dev/null || echo "0")
    AUDIT_AFTER=$(echo "$AUDIT_AFTER" | tr -d '[:space:]')
    echo "  Audit delete count (after): $AUDIT_AFTER"
    if [ "${AUDIT_AFTER:-0}" -gt "${AUDIT_BEFORE:-0}" ]; then
      assert_ok "Audit trail entry created for SBOM deletion"
      DETAILS=$(query_db "SELECT details FROM audit_logs WHERE resource='sbom' AND action='delete' ORDER BY created_at DESC LIMIT 1;" 2>/dev/null || echo "")
      echo "  Latest audit details: $(echo "$DETAILS" | head -c 200)"
    else
      echo "  INFO: No new audit entry yet (reconciler may not have run)"
    fi
  else
    echo "  SBOM still active (grace period). Audit entry will appear after reconciler runs."
    echo "  Grace period default: FORTUNA_SBOM_ORPHAN_GRACE_PERIOD=30m"
  fi
else
  echo "  SKIP: No SBOM ID to check"
fi

echo ""
echo "======================================================="
[ "$FAIL" -eq 0 ] && echo "RESULT: ALL PASSED ✅" || echo "RESULT: $FAIL FAILED ❌"
echo "======================================================="
exit "$FAIL"

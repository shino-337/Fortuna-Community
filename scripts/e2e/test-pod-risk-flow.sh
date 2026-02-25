#!/usr/bin/env bash
# Pod risk flow test:
# K8s pod -> agent sync -> DB -> authenticated API checks.

set -euo pipefail

POD_NAME="${1:-test-pod-risk-$(date +%s)}"
TEST_NS="${2:-default}"
TIMEOUT="${TIMEOUT:-300}"
NAMESPACE="${NAMESPACE:-fortuna}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "Pod Risk Flow Test"
echo "=========================================="
echo "Pod Name: $POD_NAME"
echo "Namespace: $TEST_NS"
echo "Timeout: ${TIMEOUT}s"
echo ""

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $TEST_NS
  labels:
    app: test-pod-risk
    test: "true"
spec:
  containers:
  - name: test-container
    image: busybox:latest
    command: ["sh", "-c", "sleep 3600"]
    securityContext:
      capabilities:
        add: ["SYS_ADMIN","NET_ADMIN"]
  restartPolicy: Never
EOF

kubectl wait --for=condition=Ready "pod/$POD_NAME" -n "$TEST_NS" --timeout=90s >/dev/null
POD_UID="$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}')"
echo "✅ Pod created: $POD_UID"

CORE_POD="$(require_core_pod)"
PG_POD="$(require_postgres_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

echo "⏳ Waiting pod to appear in DB..."
if WAITED="$(wait_for_pod_in_db "$POD_UID" "$TIMEOUT" "$PG_POD")"; then
  echo "✅ Pod present in DB after ${WAITED}s"
else
  echo "❌ Pod not found in DB after ${TIMEOUT}s"
  exit 1
fi

echo ""
echo "--- DB pod record ---"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT name, uid, namespace, cluster_id, node_name, created_at FROM pods WHERE uid = '$POD_UID' AND deleted_at IS NULL;"

echo ""
echo "--- DB capabilities (wait up to 60s) ---"
CAPS=0
for _ in $(seq 1 12); do
  CAPS="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
    "SELECT COUNT(*) FROM pod_capabilities WHERE pod_uid='$POD_UID';" | tr -d ' ')"
  CAPS="${CAPS:-0}"
  [ "$CAPS" -gt 0 ] 2>/dev/null && break
  sleep 5
done
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT pod_uid, capability_id, severity, state, updated_at FROM pod_capabilities WHERE pod_uid = '$POD_UID' ORDER BY updated_at DESC LIMIT 10;"

echo ""
echo "--- API pod-capabilities by podUid ---"
core_api_get "pod-capabilities?podUid=$POD_UID&pageSize=50" "$TOKEN" "$CORE_POD" | python3 -m json.tool | head -40

echo ""
echo "--- API runtime-signals (latest 10) ---"
core_api_get "runtime-signals?page=1&pageSize=10" "$TOKEN" "$CORE_POD" | python3 -m json.tool | head -40

if [ "${CAPS:-0}" -eq 0 ] 2>/dev/null; then
  echo "❌ No capabilities found for pod $POD_UID"
  exit 1
fi

echo ""
echo "✅ Pod Risk Flow PASS"
echo "Cleanup: kubectl delete pod $POD_NAME -n $TEST_NS"

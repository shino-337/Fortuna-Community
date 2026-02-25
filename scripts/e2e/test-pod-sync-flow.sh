#!/usr/bin/env bash
# Pod sync flow:
# create pod -> wait DB sync -> verify capabilities via DB + API.

set -euo pipefail

POD_NAME="${1:-test-sync-$(date +%s)}"
TEST_NS="${2:-default}"
NAMESPACE="${NAMESPACE:-fortuna}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "Pod Sync Flow Test"
echo "=========================================="
echo "Pod Name: $POD_NAME"
echo "Namespace: $TEST_NS"
echo ""

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $TEST_NS
  labels:
    app: test-sync
    test: "true"
spec:
  containers:
  - name: test-container
    image: busybox:latest
    command: ["sh", "-c", "sleep 3600"]
    securityContext:
      privileged: true
  restartPolicy: Never
EOF

kubectl wait --for=condition=Ready "pod/$POD_NAME" -n "$TEST_NS" --timeout=90s >/dev/null
POD_UID="$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}')"
echo "✅ Pod created: $POD_UID"

CORE_POD="$(require_core_pod)"
PG_POD="$(require_postgres_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

echo "⏳ Waiting pod to appear in DB..."
if WAITED="$(wait_for_pod_in_db "$POD_UID" 420 "$PG_POD")"; then
  echo "✅ Pod present in DB after ${WAITED}s"
else
  echo "❌ Pod not found in DB after 420s"
  exit 1
fi

echo ""
echo "--- DB pod row ---"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT name, uid, namespace, cluster_id, node_name FROM pods WHERE uid='$POD_UID' AND deleted_at IS NULL;"

echo ""
echo "--- DB capability count (wait up to 120s) ---"
CAPS=0
for _ in $(seq 1 24); do
  CAPS="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
    "SELECT COUNT(*) FROM pod_capabilities WHERE pod_uid='$POD_UID';" | tr -d ' ')"
  CAPS="${CAPS:-0}"
  [ "$CAPS" -gt 0 ] 2>/dev/null && break
  sleep 5
done
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT pod_uid, capability_id, severity, state FROM pod_capabilities WHERE pod_uid='$POD_UID' ORDER BY updated_at DESC LIMIT 10;"

echo ""
echo "--- API pod-capabilities by UID ---"
API_JSON="$(core_api_get "pod-capabilities?podUid=$POD_UID&pageSize=100" "$TOKEN" "$CORE_POD")"
python3 - <<'PY' "$API_JSON"
import json,sys
d=json.loads(sys.argv[1])
caps=d.get("capabilities",[])
print(f"total_capabilities={len(caps)}")
for c in caps[:8]:
    print(f"- {c.get('capabilityId')} severity={c.get('severity')} state={c.get('state')}")
PY

if [ "${CAPS:-0}" -eq 0 ] 2>/dev/null; then
  echo "❌ No capabilities evaluated for pod $POD_UID"
  exit 1
fi

echo ""
echo "✅ Pod Sync Flow PASS"
echo "Cleanup: kubectl delete pod $POD_NAME -n $TEST_NS"

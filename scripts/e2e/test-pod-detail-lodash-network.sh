#!/usr/bin/env bash
# =============================================================================
# Pod Detail network connection test using the lodash website pod.
# - Pod runs "serve" on TCP 3000 → LISTEN socket visible to ss -tunap.
# - Deploy website-vuln-lodash if not present, wait for reporter, verify
#   GET /runtime/pods/:uid/network returns >= 1 item (LISTEN on 3000).
# =============================================================================
# Prerequisites:
#   - Image built: nerdctl build -t website-vuln-lodash:latest -f scripts/e2e/fixtures/deploy-e2e/images/website-vuln-lodash/Dockerfile scripts/e2e/fixtures/deploy-e2e/images/website-vuln-lodash
#   - Agent can observe the node where the pod is scheduled. Set E2E_NODE_SELECTOR_HOST if a fixed node is required.
# =============================================================================

set -euo pipefail

POD_NAME="${1:-website-vuln-lodash}"
TEST_NS="${2:-fortuna-e2e}"
TIMEOUT="${TIMEOUT:-300}"
NAMESPACE="${NAMESPACE:-fortuna}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "Pod Detail Lodash Network Test"
echo "=========================================="
echo "Pod Name: $POD_NAME"
echo "Namespace: $TEST_NS"
echo "Timeout: ${TIMEOUT}s"
echo ""

# Ensure namespace exists
kubectl get namespace "$TEST_NS" &>/dev/null || kubectl create namespace "$TEST_NS"

# Deploy lodash pod if not present (do not delete existing – may be used for SBOM/CVE)
if ! kubectl get pod "$POD_NAME" -n "$TEST_NS" &>/dev/null; then
  echo "Deploying lodash website pod from scripts/e2e/fixtures/deploy-e2e/website-vuln-lodash-pod.yaml ..."
  kubectl apply -f "${PROJECT_ROOT}/scripts/e2e/fixtures/deploy-e2e/website-vuln-lodash-pod.yaml"
else
  echo "Pod $POD_NAME already exists in $TEST_NS."
fi

echo "Waiting for pod Ready..."
if ! kubectl wait --for=condition=Ready "pod/$POD_NAME" -n "$TEST_NS" --timeout=120s 2>/dev/null; then
  echo "⚠️  Pod not Ready (image website-vuln-lodash:latest may be missing). Build with:"
  echo "   nerdctl --namespace k8s.io build -t website-vuln-lodash:latest -f scripts/e2e/fixtures/deploy-e2e/images/website-vuln-lodash/Dockerfile scripts/e2e/fixtures/deploy-e2e/images/website-vuln-lodash"
  kubectl get pod "$POD_NAME" -n "$TEST_NS" 2>/dev/null || true
  exit 1
fi

POD_UID="$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}')"
echo "✅ Pod Ready: $POD_UID"

CORE_POD="$(require_core_pod)"
PG_POD="$(require_postgres_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

echo "⏳ Waiting for pod to appear in DB (pods table)..."
if ! WAITED="$(wait_for_pod_in_db "$POD_UID" "$TIMEOUT" "$PG_POD")"; then
  echo "❌ Pod not found in DB after ${TIMEOUT}s"
  exit 1
fi
echo "✅ Pod present in DB after ${WAITED}s"

echo "⏳ Waiting for Pod Detail reporter (interval 2m); waiting 150s..."
SLEEP_SECS="${POD_DETAIL_WAIT_SECS:-150}"
sleep "$SLEEP_SECS"

# Poll for network connections (agent runs ss -tunap; serve listens on 3000)
EXTRA_WAIT="${POD_DETAIL_EXTRA_WAIT:-120}"
for _ in $(seq 1 "$((EXTRA_WAIT / 30))"); do
  NC_COUNT=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
    "SELECT COUNT(*) FROM pod_network_connections WHERE pod_uid = '$POD_UID';" 2>/dev/null | tr -d ' ' || echo "0")
  if [ "${NC_COUNT:-0}" -gt 0 ] 2>/dev/null; then
    echo "✅ Found $NC_COUNT network connection row(s) for pod."
    break
  fi
  echo "   No network connections yet; waiting 30s more..."
  sleep 30
done

echo ""
echo "--- DB pod_network_connections ---"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT pod_uid, container_name, source_ip, source_port, dest_ip, dest_port, protocol, state, observed_at FROM pod_network_connections WHERE pod_uid = '$POD_UID' ORDER BY observed_at DESC LIMIT 10;"

echo ""
echo "--- API: GET /runtime/pods/:uid/network ---"
NETWORK_RESPONSE="$(core_api_get "runtime/pods/${POD_UID}/network" "$TOKEN" "$CORE_POD")"
echo "$NETWORK_RESPONSE" | python3 -m json.tool | head -50

# Assert: response has items (array) and podUid
if ! echo "$NETWORK_RESPONSE" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    if 'items' not in d or not isinstance(d.get('items'), list):
        sys.exit(1)
    if (d.get('podUid') or '') != '''$POD_UID''':
        sys.exit(2)
except (json.JSONDecodeError, Exception):
    sys.exit(3)
" 2>/dev/null; then
  echo "❌ FAIL: runtime/pods/:uid/network API must return JSON with 'items' and 'podUid'."
  exit 1
fi

# Assert: at least one connection (lodash pod has serve LISTEN on 3000)
ITEM_COUNT=$(echo "$NETWORK_RESPONSE" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(len(d.get('items') or []))
" 2>/dev/null || echo "0")
if [ "${ITEM_COUNT:-0}" -lt 1 ] 2>/dev/null; then
  echo "❌ FAIL: expected at least 1 network connection (serve listens on TCP 3000). Got $ITEM_COUNT. Ensure Agent is on same node as pod and Pod Detail reporter has run."
  exit 1
fi

# Optional: at least one item has state LISTEN and port 3000
echo "$NETWORK_RESPONSE" | python3 -c "
import sys, json
d = json.load(sys.stdin)
items = d.get('items') or []
listen_3000 = [i for i in items if (i.get('state') or '').upper() == 'LISTEN' and (i.get('sourcePort') == 3000 or i.get('destPort') == 3000)]
if listen_3000:
    print('✅ At least one connection: LISTEN on port 3000 (serve).')
else:
    print('⚠️  No LISTEN on 3000 in response (state/port may differ); items count OK.')
" 2>/dev/null || true

echo ""
echo "✅ Pod Detail Lodash Network test finished: API returns $ITEM_COUNT connection(s); Dashboard Network tab can display data."
echo "Cleanup (optional): kubectl delete pod $POD_NAME -n $TEST_NS"

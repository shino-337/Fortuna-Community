#!/usr/bin/env bash
# Pod Detail ping flow test:
# - Create a pod that pings 8.8.8.8 every 5s (real process + network).
# - Wait for Agent to collect runtime-metrics, processes, network-connections.
# - Verify DB rows and Core Pod Detail APIs.

set -euo pipefail

POD_NAME="${1:-fortuna-e2e-poddetail-ping}"
TEST_NS="${2:-fortuna-e2e}"
TIMEOUT="${TIMEOUT:-300}"
NAMESPACE="${NAMESPACE:-fortuna}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "Pod Detail Ping Flow Test"
echo "=========================================="
echo "Pod Name: $POD_NAME"
echo "Namespace: $TEST_NS"
echo "Timeout: ${TIMEOUT}s"
echo ""

echo "Removing existing pod if present (K8s forbids changing pod spec command/image in-place)..."
kubectl delete pod "$POD_NAME" -n "$TEST_NS" --ignore-not-found --wait=false 2>/dev/null || true
sleep 2
echo "Applying ping pod manifest..."
kubectl apply -f "${PROJECT_ROOT}/scripts/e2e/fixtures/deploy-e2e/fortuna-e2e-poddetail-ping-pod.yaml"

echo "Waiting for pod Ready..."
kubectl wait --for=condition=Ready "pod/$POD_NAME" -n "$TEST_NS" --timeout=120s >/dev/null
POD_UID="$(kubectl get pod "$POD_NAME" -n "$TEST_NS" -o jsonpath='{.metadata.uid}')"
echo "✅ Pod Ready: $POD_UID"

CORE_POD="$(require_core_pod)"
PG_POD="$(require_postgres_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

echo "⏳ Waiting for pod to appear in DB (pods table)..."
if WAITED="$(wait_for_pod_in_db "$POD_UID" "$TIMEOUT" "$PG_POD")"; then
  echo "✅ Pod present in DB after ${WAITED}s"
else
  echo "❌ Pod not found in DB after ${TIMEOUT}s"
  exit 1
fi

echo "⏳ Waiting for Pod Detail reporter (interval 2m); waiting 150s..."
SLEEP_SECS="${POD_DETAIL_WAIT_SECS:-150}"
sleep "$SLEEP_SECS"

# Optional: if still no metrics, poll up to 2 more minutes (reporter may run every 2 min)
EXTRA_WAIT="${POD_DETAIL_EXTRA_WAIT:-120}"
for _ in $(seq 1 "$((EXTRA_WAIT / 30))"); do
  COUNT=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
    "SELECT COUNT(*) FROM pod_runtime_metrics WHERE pod_uid = '$POD_UID';" 2>/dev/null | tr -d ' ' || echo "0")
  if [ "${COUNT:-0}" -gt 0 ] 2>/dev/null; then
    echo "✅ Found $COUNT runtime_metrics row(s) for pod."
    break
  fi
  echo "   No metrics yet; waiting 30s more..."
  sleep 30
done

echo ""
echo "--- DB pod_runtime_metrics ---"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT pod_uid, container_name, cpu_usage_millicore, memory_usage_bytes, last_observed_at FROM pod_runtime_metrics WHERE pod_uid = '$POD_UID' ORDER BY last_observed_at DESC LIMIT 5;"

echo ""
echo "--- DB pod_processes (expect ping process) ---"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT pod_uid, container_name, p_id, pp_id, user_name, cpu_percent, memory_percent, LEFT(command, 60) AS command, observed_at FROM pod_processes WHERE pod_uid = '$POD_UID' ORDER BY observed_at DESC LIMIT 10;"

echo ""
echo "--- DB pod_network_connections ---"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT pod_uid, container_name, source_ip, source_port, dest_ip, dest_port, protocol, state, observed_at FROM pod_network_connections WHERE pod_uid = '$POD_UID' ORDER BY observed_at DESC LIMIT 10;"

echo ""
echo "--- API: GET /runtime/pods/:uid/metrics ---"
core_api_get "runtime/pods/${POD_UID}/metrics" "$TOKEN" "$CORE_POD" | python3 -m json.tool | head -40

echo ""
echo "--- API: GET /runtime/pods/:uid/processes ---"
core_api_get "runtime/pods/${POD_UID}/processes" "$TOKEN" "$CORE_POD" | python3 -m json.tool | head -40

echo ""
echo "--- API: GET /runtime/pods/:uid/network (Dashboard Network tab uses this) ---"
NETWORK_RESPONSE="$(core_api_get "runtime/pods/${POD_UID}/network" "$TOKEN" "$CORE_POD")"
echo "$NETWORK_RESPONSE" | python3 -m json.tool | head -40

# Assert network-connections API returns valid JSON with items array (dashboard expects this)
if ! echo "$NETWORK_RESPONSE" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    if 'items' not in d or not isinstance(d.get('items'), list):
        sys.exit(1)
    # podUid should match (dashboard uses this to show pod-scoped data)
    uid = d.get('podUid') or ''
    expected = '''$POD_UID'''
    if uid != expected:
        sys.exit(2)
except (json.JSONDecodeError, Exception):
    sys.exit(3)
" 2>/dev/null; then
  echo "❌ FAIL: GET /runtime/pods/:uid/network must return JSON with 'items' (array) and 'podUid'; dashboard Network tab depends on it."
  exit 1
fi
echo "✅ Network-connections API: valid response (items + podUid); Dashboard Network tab can display data."

echo ""
echo "✅ Pod Detail Ping Flow test finished (check DB/API output above)."
METRICS_ROWS=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "SELECT COUNT(*) FROM pod_runtime_metrics WHERE pod_uid = '$POD_UID';" 2>/dev/null | tr -d ' ' || echo "0")
if [ "${METRICS_ROWS:-0}" -eq 0 ] 2>/dev/null; then
  echo ""
  echo "⚠️  No pod_runtime_metrics rows. Check: (1) Pod and Agent on same node: kubectl get pod $POD_NAME -n $TEST_NS -o jsonpath='{.spec.nodeName}'; kubectl -n $NAMESPACE get pods -l app.kubernetes.io/component=agent -o wide. (2) Agent logs: kubectl -n $NAMESPACE logs -l app.kubernetes.io/component=agent --tail=200 | grep -E 'PodDetail|send metrics|send processes'"
fi
echo "Cleanup (optional): kubectl delete pod $POD_NAME -n $TEST_NS"


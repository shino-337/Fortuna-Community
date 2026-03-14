#!/usr/bin/env bash

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "Test Promotion Flow"
echo "=========================================="

CORE_POD="$(require_core_pod)"
PG_POD="$(require_postgres_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

POD_UID=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
  "SELECT pod_uid FROM runtime_signals ORDER BY created_at DESC LIMIT 1;" | tr -d ' ')
SIGNAL_TYPE=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
  "SELECT signal_type FROM runtime_signals WHERE pod_uid='${POD_UID}' ORDER BY created_at DESC LIMIT 1;" | tr -d ' ')

if [ -z "$POD_UID" ] || [ -z "$SIGNAL_TYPE" ]; then
  echo "❌ No runtime_signals data found to test promotion flow"
  exit 1
fi

echo "Core pod: ${CORE_POD}"
echo ""

echo "=== 1. Check Pod Capabilities ==="
echo "Pod UID: ${POD_UID}"
core_api_get "pods/${POD_UID}/capabilities" "$TOKEN" "$CORE_POD" | python3 -m json.tool 2>/dev/null | head -40 || true
echo ""

echo "=== 2. Check Runtime Signals for Pod ==="
core_api_get "runtime/pods/${POD_UID}/signals" "$TOKEN" "$CORE_POD" | python3 -m json.tool 2>/dev/null | head -40 || true
echo ""

echo "=== 3. Check Promotion Rules for Signal ==="
core_api_get "promotion-rules/signal/${SIGNAL_TYPE}" "$TOKEN" "$CORE_POD" | python3 -m json.tool 2>/dev/null | head -60 || true
echo ""

echo "=== 4. Database Check ==="
echo "Capabilities for pod:"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT capability_id, state, confidence FROM pod_capabilities WHERE pod_uid='${POD_UID}' ORDER BY capability_id;"
echo ""

echo "Runtime signals count for pod:"
kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -c \
  "SELECT COUNT(*) as signal_count FROM runtime_signals WHERE pod_uid='${POD_UID}' AND signal_type='${SIGNAL_TYPE}';"
echo ""

echo "=========================================="
echo "Test Complete"
echo "=========================================="

#!/usr/bin/env bash
# Validate consistency among Kubernetes, DB and dashboard APIs.

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
SYNC_WAIT_SECONDS="${SYNC_WAIT_SECONDS:-75}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "Dashboard Consistency E2E"
echo "=========================================="
echo "Namespace: $NAMESPACE"
echo "Sync wait: ${SYNC_WAIT_SECONDS}s"
echo ""

CORE_POD="$(require_core_pod)"
PG_POD="$(require_postgres_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

echo "Step 1: Wait one sync window for fresh data..."
sleep "${SYNC_WAIT_SECONDS}"

echo "Step 2: Collect Kubernetes live counts..."
K8S_RUNNING_PODS="$(kubectl get pods -A --field-selector=status.phase=Running --no-headers | wc -l | tr -d ' ')"
K8S_RUNNING_PODS_FORTUNA="$(kubectl get pods -n "$NAMESPACE" --field-selector=status.phase=Running --no-headers | wc -l | tr -d ' ')"
K8S_NODES="$(kubectl get nodes --no-headers | wc -l | tr -d ' ')"
echo "  K8s running pods (all namespaces): $K8S_RUNNING_PODS"
echo "  K8s running pods (fortuna): $K8S_RUNNING_PODS_FORTUNA"
echo "  K8s nodes: $K8S_NODES"

echo "Step 3: Collect DB counts..."
DB_PODS="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
  "SELECT COUNT(DISTINCT uid) FROM pods WHERE deleted_at IS NULL;" | tr -d ' ')"
DB_CLUSTERS="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
  "SELECT COUNT(*) FROM clusters WHERE deleted_at IS NULL;" | tr -d ' ')"
DB_AGENTS="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
  "SELECT COUNT(*) FROM agents WHERE deleted_at IS NULL;" | tr -d ' ')"
echo "  DB pods: $DB_PODS"
echo "  DB clusters: $DB_CLUSTERS"
echo "  DB agents: $DB_AGENTS"

echo "Step 4: Collect Dashboard API counts..."
STATS_JSON="$(core_api_get "dashboard/stats" "$TOKEN" "$CORE_POD")"
CLUSTERS_JSON="$(core_api_get "clusters" "$TOKEN" "$CORE_POD")"

API_RUNNING_PODS="$(python3 - <<'PY' "$STATS_JSON"
import json,sys
print(json.loads(sys.argv[1]).get("runningPods", -1))
PY
)"
API_ACTIVE_AGENTS="$(python3 - <<'PY' "$STATS_JSON"
import json,sys
print(json.loads(sys.argv[1]).get("activeAgents", -1))
PY
)"
API_TOTAL_CLUSTERS="$(python3 - <<'PY' "$STATS_JSON"
import json,sys
print(json.loads(sys.argv[1]).get("totalClusters", -1))
PY
)"
API_CLUSTERS_LEN="$(python3 - <<'PY' "$CLUSTERS_JSON"
import json,sys
print(len(json.loads(sys.argv[1]).get("clusters", [])))
PY
)"
echo "  API stats.runningPods: $API_RUNNING_PODS"
echo "  API stats.activeAgents: $API_ACTIVE_AGENTS"
echo "  API stats.totalClusters: $API_TOTAL_CLUSTERS"
echo "  API /clusters length: $API_CLUSTERS_LEN"

echo ""
echo "Step 5: Assertions..."
FAIL=0

if [ "$API_RUNNING_PODS" != "$DB_PODS" ]; then
  echo "❌ Mismatch: API runningPods=$API_RUNNING_PODS != DB pods=$DB_PODS"
  FAIL=1
else
  echo "✅ runningPods matches DB"
fi

if [ "$API_ACTIVE_AGENTS" != "$DB_AGENTS" ]; then
  echo "❌ Mismatch: API activeAgents=$API_ACTIVE_AGENTS != DB agents=$DB_AGENTS"
  FAIL=1
else
  echo "✅ activeAgents matches DB"
fi

if [ "$API_TOTAL_CLUSTERS" != "$DB_CLUSTERS" ] || [ "$API_TOTAL_CLUSTERS" != "$API_CLUSTERS_LEN" ]; then
  echo "❌ Mismatch: API totalClusters=$API_TOTAL_CLUSTERS, DB clusters=$DB_CLUSTERS, /clusters len=$API_CLUSTERS_LEN"
  FAIL=1
else
  echo "✅ totalClusters matches DB and /clusters"
fi

if [ "$K8S_RUNNING_PODS" != "$DB_PODS" ]; then
  echo "⚠️  K8s running pods ($K8S_RUNNING_PODS) differs from DB pods ($DB_PODS) - may indicate sync scope/delay"
else
  echo "✅ K8s running pods matches DB pods"
fi

echo ""
if [ "$FAIL" -ne 0 ]; then
  echo "❌ Dashboard Consistency E2E FAILED"
  exit 1
fi

echo "✅ Dashboard Consistency E2E PASSED"

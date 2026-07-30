#!/usr/bin/env bash
# =============================================================================
# Verify Pod Detail empty response: DB counts (metrics, processes, connections,
# events), pod_uid consistency with pods table, and bad rows (pod_uid '' or '0').
# Usage: NAMESPACE=fortuna ./scripts/verify/verify-pod-detail-empty-response.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
DB_NAME="${DB_NAME:-fortuna}"
DB_USER="${DB_USER:-postgres}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }

echo ""
echo "=========================================="
echo "Pod Detail empty response diagnostic"
echo "=========================================="

PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$PG_POD" ]; then
  fail "Postgres pod not found in namespace $NAMESPACE"
  exit 1
fi

run_sql() {
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U "$DB_USER" -d "$DB_NAME" -t -A "$@" 2>/dev/null || true
}

echo ""
echo "=== 1. Row counts (Pod Detail tables) ==="
for table in pod_runtime_metrics pod_processes pod_network_connections k8s_events; do
  if run_sql -c "SELECT COUNT(*) FROM $table;" >/dev/null 2>&1; then
    N=$(run_sql -c "SELECT COUNT(*) FROM $table;")
    info "$table: $N"
  else
    warn "$table: table missing or error"
  fi
done

echo ""
echo "=== 2. Pods count (pods) ==="
PODS_N=$(run_sql -c "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;")
info "pods (deleted_at IS NULL): $PODS_N"

echo ""
echo "=== 3. Metrics/processes/connections per pod_uid (sample) ==="
run_sql -c "SELECT pod_uid, COUNT(*) FROM pod_runtime_metrics GROUP BY pod_uid ORDER BY COUNT(*) DESC LIMIT 5;" || true
run_sql -c "SELECT pod_uid, COUNT(*) FROM pod_processes GROUP BY pod_uid ORDER BY COUNT(*) DESC LIMIT 5;" || true

echo ""
echo "=== 4. pod_uid in detail tables present in pods? (sample) ==="
run_sql -c "
SELECT DISTINCT m.pod_uid FROM pod_runtime_metrics m
LEFT JOIN pods p ON p.uid = m.pod_uid AND p.deleted_at IS NULL
WHERE p.uid IS NULL
LIMIT 5;
" || true

echo ""
echo "=== 5. Bad pod_uid (empty or '0') ==="
BAD_M=$(run_sql -c "SELECT COUNT(*) FROM pod_runtime_metrics WHERE pod_uid = '' OR pod_uid = '0';")
BAD_P=$(run_sql -c "SELECT COUNT(*) FROM pod_processes WHERE pod_uid = '' OR pod_uid = '0';")
if [ "${BAD_M:-0}" -gt 0 ] || [ "${BAD_P:-0}" -gt 0 ]; then
  warn "pod_runtime_metrics with pod_uid '' or '0': $BAD_M"
  warn "pod_processes with pod_uid '' or '0': $BAD_P"
  echo "  To clean: DELETE FROM pod_runtime_metrics WHERE pod_uid = '' OR pod_uid = '0';"
  echo "           DELETE FROM pod_processes WHERE pod_uid = '' OR pod_uid = '0';"
else
  info "No bad pod_uid rows in metrics/processes"
fi

echo ""
echo "=========================================="
echo "Done. See docs/03-components/COMPONENTS.md#pod-detail for ingest and API checklist."
echo "=========================================="

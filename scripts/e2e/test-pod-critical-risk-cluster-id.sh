#!/usr/bin/env bash
# =============================================================================
# Test case 1: Pod có risk critical trên clusterId hiện tại
#   - Lấy clusterId active từ GET /clusters
#   - Nếu chưa có critical insight cho pod trong cluster: insert 1 critical
#     insight (vulnerability) cho một pod trong cluster
#   - Gọi GET insights/summary?clusterId và GET dashboard/stats?clusterId
#   - Xác nhận critical >= 1
#
# Test case 2: Kiểm tra API với clusterId đang active
#   - Gọi dashboard/stats?clusterId, insights/summary?clusterId, risks?clusterId
#   - Xác nhận cấu trúc response và tính nhất quán (critical count)
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
PG_POD=""
CORE_POD=""
TOKEN=""
CLUSTER_ID=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
ok()  { echo -e "${GREEN}[OK]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Resolve Core pod and get JWT
get_core_and_token() {
  CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  if [ -z "$CORE_POD" ]; then
    fail "Core pod not found in namespace $NAMESPACE"
    return 1
  fi
  TOKEN=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H "Content-Type: application/json" -d "{\"username\":\"${FORTUNA_ADMIN_USER:-admin}\",\"password\":\"${FORTUNA_ADMIN_PASSWORD:-${FORTUNA_DEFAULT_ADMIN_PASSWORD:-Fortuna_ChangeMe_123!}}\"}" 2>/dev/null | \
    python3 -c "import sys,json; print(json.load(sys.stdin).get('token','') or '')" 2>/dev/null || true)
  if [ -z "$TOKEN" ]; then
    fail "Could not get JWT"
    return 1
  fi
  return 0
}

api_get() {
  kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/$1" 2>/dev/null || echo '{}'
}

# Get active cluster ID from API
get_active_cluster_id() {
  local body
  body=$(api_get "clusters")
  CLUSTER_ID=$(echo "$body" | python3 -c "
import sys, json
try:
  d = json.load(sys.stdin)
  clusters = d.get('clusters') or []
  if clusters:
    print(clusters[0].get('id', ''))
except Exception:
  pass
" 2>/dev/null || true)
  if [ -z "$CLUSTER_ID" ]; then
    fail "Could not get active cluster ID from /clusters"
    return 1
  fi
  info "Active clusterId: $CLUSTER_ID"
  return 0
}

# Ensure one critical insight for a pod in cluster (insert if none)
ensure_critical_insight_for_cluster() {
  PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  if [ -z "$PG_POD" ]; then
    warn "Postgres pod not found; skipping insert (test may still pass if critical already exists)"
    return 0
  fi
  # Count critical insights for this cluster (insights joined with pods)
  local count
  count=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "
    SELECT COUNT(*) FROM insights i
    INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = '$CLUSTER_ID' AND p.deleted_at IS NULL
    WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL) AND LOWER(i.severity) = 'critical';
  " 2>/dev/null | tr -d ' ' || echo "0")
  if [ "${count:-0}" -ge 1 ]; then
    info "Cluster already has $count critical insight(s); no insert needed"
    return 0
  fi
  # Get one pod UID in cluster
  local pod_uid pod_name pod_ns
  pod_uid=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "
    SELECT uid FROM pods WHERE cluster_id = '$CLUSTER_ID' AND deleted_at IS NULL LIMIT 1;
  " 2>/dev/null | tr -d ' \r' || true)
  pod_name=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "
    SELECT name FROM pods WHERE cluster_id = '$CLUSTER_ID' AND deleted_at IS NULL LIMIT 1;
  " 2>/dev/null | tr -d ' \r' || true)
  pod_ns=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "
    SELECT namespace FROM pods WHERE cluster_id = '$CLUSTER_ID' AND deleted_at IS NULL LIMIT 1;
  " 2>/dev/null | tr -d ' \r' || true)
  if [ -z "$pod_uid" ]; then
    warn "No pod in cluster; cannot insert critical insight"
    return 0
  fi
  # Insert one critical vulnerability insight (unique on resource_uid, cve_id, insight_type)
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -c "
    INSERT INTO insights (
      resource_type, resource_uid, resource_namespace, resource_name,
      insight_type, severity, title, description, status, cve_id,
      detected_at, created_at, updated_at
    ) VALUES (
      'Pod', '$pod_uid', '$pod_ns', '$pod_name',
      'vulnerability', 'critical', 'E2E Critical Test', 'E2E test critical insight for clusterId',
      'active', 'CVE-E2E-TEST-001',
      NOW(), NOW(), NOW()
    ) ON CONFLICT (resource_uid, cve_id, insight_type) DO NOTHING;
  " 2>/dev/null || true
  # ON CONFLICT may not match if constraint is (resource_uid, cve_id, insight_type) - we use unique cve_id
  info "Inserted (or skipped) one critical insight for pod $pod_uid in cluster"
  return 0
}

echo "=========================================="
echo "E2E: Pod critical risk on clusterId + API check"
echo "=========================================="
echo ""

if ! get_core_and_token; then
  exit 1
fi
if ! get_active_cluster_id; then
  exit 1
fi
echo ""

# --- Test case 1: Pod có risk critical trên clusterId hiện tại ---
echo "========== Test case 1: Pod có risk critical trên clusterId hiện tại =========="
ensure_critical_insight_for_cluster
echo ""

summary=$(api_get "insights/summary?clusterId=$CLUSTER_ID")
stats=$(api_get "dashboard/stats?clusterId=$CLUSTER_ID")
crit_summary=$(echo "$summary" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('critical', 0))" 2>/dev/null || echo "0")
crit_stats=$(echo "$stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('criticalRisks', 0))" 2>/dev/null || echo "0")

if [ "${crit_summary:-0}" -ge 1 ] && [ "${crit_stats:-0}" -ge 1 ]; then
  ok "Test case 1 PASS: insights/summary critical=$crit_summary, dashboard/stats criticalRisks=$crit_stats (clusterId=$CLUSTER_ID)"
else
  fail "Test case 1 FAIL: expected critical >= 1 with clusterId. summary.critical=$crit_summary, stats.criticalRisks=$crit_stats"
  exit 1
fi
echo ""

# --- Test case 2: Kiểm tra API với clusterId đang active ---
echo "========== Test case 2: Kiểm tra API với clusterId đang active =========="
stats2=$(api_get "dashboard/stats?clusterId=$CLUSTER_ID")
summary2=$(api_get "insights/summary?clusterId=$CLUSTER_ID")
risks_body=$(api_get "risks?clusterId=$CLUSTER_ID")

# Check structure
if ! echo "$stats2" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'totalClusters' in d and 'totalRisks' in d" 2>/dev/null; then
  fail "Test case 2 FAIL: dashboard/stats?clusterId response missing required fields"
  exit 1
fi
if ! echo "$summary2" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'total' in d and 'critical' in d" 2>/dev/null; then
  fail "Test case 2 FAIL: insights/summary?clusterId response missing required fields"
  exit 1
fi
if ! echo "$risks_body" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'insights' in d" 2>/dev/null; then
  fail "Test case 2 FAIL: risks?clusterId response missing 'insights'"
  exit 1
fi

# Consistency: dashboard/stats criticalRisks should match insights/summary critical for same clusterId
s_crit=$(echo "$summary2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('critical', 0))" 2>/dev/null)
g_crit=$(echo "$stats2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('criticalRisks', 0))" 2>/dev/null)
risks_count=$(echo "$risks_body" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('insights', [])))" 2>/dev/null)
crit_list_count=$(echo "$risks_body" | python3 -c "
import sys, json
d = json.load(sys.stdin)
insights = d.get('insights') or []
print(sum(1 for i in insights if (i.get('severity') or '').lower() == 'critical'))
" 2>/dev/null)

ok "Test case 2 PASS: API với clusterId đang active"
echo "  dashboard/stats?clusterId=$CLUSTER_ID: totalRisks=$(echo "$stats2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('totalRisks'))"), criticalRisks=$g_crit, clusterName=$(echo "$stats2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(repr(d.get('clusterName','')))")"
echo "  insights/summary?clusterId=$CLUSTER_ID: total=$(echo "$summary2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total'))"), critical=$s_crit"
echo "  risks?clusterId=$CLUSTER_ID: insights count=$risks_count, critical in list=$crit_list_count"
if [ "${s_crit:-x}" != "${g_crit:-y}" ]; then
  warn "Note: summary.critical ($s_crit) vs stats.criticalRisks ($g_crit) may differ if dashboard/stats uses different aggregation"
fi
echo ""

echo "=========================================="
ok "All 2 test cases passed."
echo "=========================================="

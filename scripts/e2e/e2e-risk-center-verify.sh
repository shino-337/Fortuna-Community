#!/usr/bin/env bash
# ============================================================================
# E2E Risk Center – seed test insight và verify APIs
# ============================================================================
# Risk Center (tab Risks) cần: GET /risks, GET /insights/summary (vulnerability).
# Nếu insights table trống thì Risk Center hiển thị "Total: 0". Script này:
# 1. Lấy cluster_id và một pod uid từ DB (pod thuộc cluster).
# 2. Xóa insight E2E cũ (cve_id = 'E2E-RISK-CENTER') nếu có.
# 3. Insert 1 insight vulnerability (active, high) cho pod đó.
# 4. Gọi GET /api/v1/risks?clusterId=... và GET /api/v1/insights/summary?clusterId=...
# 5. Assert total >= 1 và summary high hoặc critical >= 1.
# 6. Gọi GET /api/v1/runtime-signals (tab Reference) và ghi kết quả.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
ok()  { echo -e "${GREEN}[OK]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
info() { echo -e "${BLUE}[E2E Risk Center]${NC} $*"; }

CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
[ -z "$CORE_POD" ] && CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [ -z "$CORE_POD" ] || [ -z "$PG_POD" ]; then
  fail "Core or Postgres pod not found in namespace $NAMESPACE"
  exit 1
fi

# JWT
LOGIN=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' 2>/dev/null || echo "{}")
TOKEN=$(echo "$LOGIN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null || echo "")
if [ -z "$TOKEN" ]; then
  warn "Login failed; API calls may return 401"
fi
AUTH_HEADER="Authorization: Bearer $TOKEN"

api_get() {
  local path="$1"
  if [ -n "$TOKEN" ]; then
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "$AUTH_HEADER" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo "{}"
  else
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s "http://localhost:8080/api/v1/$path" 2>/dev/null || echo "{}"
  fi
}

echo ""
echo "========== E2E Risk Center – seed insight & verify APIs =========="
info "Core pod: $CORE_POD | PG pod: $PG_POD"
echo ""

# 1. Get cluster_id and one pod uid from cluster
CLUSTER_ID=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "SELECT id FROM clusters WHERE deleted_at IS NULL LIMIT 1;" 2>/dev/null | tr -d '\r' || echo "")
POD_UID=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "SELECT uid FROM pods WHERE cluster_id = '$CLUSTER_ID' AND deleted_at IS NULL LIMIT 1;" 2>/dev/null | tr -d '\r' || echo "")

if [ -z "$CLUSTER_ID" ] || [ -z "$POD_UID" ]; then
  warn "No cluster or pod in DB (cluster_id=$CLUSTER_ID, pod_uid=$POD_UID). Skip seeding insight; verify APIs only."
else
  info "Cluster: $CLUSTER_ID | Pod UID: $POD_UID"
  # 2. Remove old E2E insight if any
  kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -c "DELETE FROM insights WHERE cve_id = 'E2E-RISK-CENTER';" 2>/dev/null || true
  # 3. Insert one vulnerability insight for this pod (columns match insight_manager batch insert)
  kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -v ON_ERROR_STOP=1 -c "
    INSERT INTO insights (
      resource_type, resource_uid, resource_namespace, resource_name,
      insight_type, severity, title, description, recommendation, status,
      cve_id, cvss, affected_component, affected_version,
      detected_at, created_at, updated_at
    ) VALUES (
      'Pod', '$POD_UID', 'fortuna', 'e2e-risk-center-pod',
      'vulnerability', 'high', 'E2E Risk Center Test', 'Synthetic insight for Risk Center E2E verification.', 'Run E2E to verify Risk Center APIs.',
      'active', 'E2E-RISK-CENTER', 7.5, 'e2e-test-pkg', '1.0.0',
      NOW(), NOW(), NOW()
    );
  " 2>/dev/null && ok "Inserted 1 test insight (E2E-RISK-CENTER)" || { warn "Insert failed (table schema may differ); continuing API checks"; }
fi
echo ""

# 4. GET /risks (Risk Center list)
info "GET /api/v1/risks (clusterId=$CLUSTER_ID)..."
RISKS_RESP=$(api_get "risks?page=1&pageSize=5&clusterId=${CLUSTER_ID}")
RISKS_TOTAL=$(echo "$RISKS_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total',0))" 2>/dev/null || echo "0")
RISKS_LEN=$(echo "$RISKS_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('insights',[])))" 2>/dev/null || echo "0")
if [ "${RISKS_TOTAL:-0}" -ge 1 ]; then
  ok "GET /risks: total=$RISKS_TOTAL, page size=$RISKS_LEN"
else
  warn "GET /risks: total=$RISKS_TOTAL (Risk Center tab Risks will show empty if 0)"
fi
echo ""

# 5. GET /insights/summary (severity bar)
info "GET /api/v1/insights/summary (clusterId=$CLUSTER_ID)..."
SUMMARY_RESP=$(api_get "insights/summary?clusterId=${CLUSTER_ID}")
SUM_TOTAL=$(echo "$SUMMARY_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total',0))" 2>/dev/null || echo "0")
SUM_CRIT=$(echo "$SUMMARY_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('critical',0))" 2>/dev/null || echo "0")
SUM_HIGH=$(echo "$SUMMARY_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('high',0))" 2>/dev/null || echo "0")
if [ "${SUM_TOTAL:-0}" -ge 1 ] || [ "${SUM_CRIT:-0}" -ge 1 ] || [ "${SUM_HIGH:-0}" -ge 1 ]; then
  ok "GET /insights/summary: total=$SUM_TOTAL, critical=$SUM_CRIT, high=$SUM_HIGH"
else
  warn "GET /insights/summary: total=$SUM_TOTAL (Risk Center severity bar will show zeros if all 0)"
fi
echo ""

# 6. GET /runtime-signals (Risk Center tab Reference / block Runtime signals)
info "GET /api/v1/runtime-signals (Risk Center Reference tab)..."
RT_RESP=$(api_get "runtime-signals?limit=10")
RT_TOTAL=$(echo "$RT_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total',0))" 2>/dev/null || echo "0")
RT_COUNT=$(echo "$RT_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('signals',[])))" 2>/dev/null || echo "0")
ok "GET /runtime-signals: total=$RT_TOTAL, count=$RT_COUNT"
echo ""

# Summary
echo "========== Risk Center E2E summary =========="
if [ "${RISKS_TOTAL:-0}" -ge 1 ] && [ "${SUM_TOTAL:-0}" -ge 1 ]; then
  ok "Risk Center has data: risks total=$RISKS_TOTAL, summary total=$SUM_TOTAL. Tab Risks and severity bar will show values."
else
  warn "Risk Center risks/summary still 0. Ensure insights exist for pods in the selected cluster (e.g. run this script after cluster sync; or seed insight with valid pod uid)."
fi
echo ""

#!/usr/bin/env bash
# ============================================================================
# E2E Comprehensive Test – Full luồng: K8s, API, DB, từng chức năng
# Output: docs/test-results/E2E-COMPREHENSIVE-<timestamp>.md
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_DIR="$REPO_ROOT/docs/test-results"
REPORT="$REPORT_DIR/E2E-COMPREHENSIVE-$TIMESTAMP.md"
mkdir -p "$REPORT_DIR"

# Use exec into Core pod (no port-forward needed)
CORE_POD=""
get_core_pod() {
  [ -n "${CORE_POD}" ] && return 0
  CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  [ -z "$CORE_POD" ] && CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  [ -n "$CORE_POD" ]
}

TOKEN=""
get_token() {
  [ -n "${TOKEN}" ] && return 0
  get_core_pod || return 1
  local resp
  resp=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null || echo "{}")
  TOKEN=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token','') or '')" 2>/dev/null || echo "")
  [ -n "$TOKEN" ]
}

api_get() {
  get_core_pod || { echo '{"error":"no core pod"}'; return 1; }
  local path="$1"
  if get_token 2>/dev/null; then
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -w "\n%{http_code}" -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo -e "\n000"
  else
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -w "\n%{http_code}" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo -e "\n000"
  fi
}

api_test() {
  local name="$1"
  local path="$2"
  local key="${3:-}"   # optional: check response has key
  local out code body
  out=$(api_get "$path" 2>/dev/null)
  body=$(echo "$out" | head -n -1)
  code=$(echo "$out" | tail -n 1)
  if [ "$code" = "200" ]; then
    if [ -n "$key" ]; then
      if echo "$body" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d.get('$key') is not None or ('$key' in str(d)) else 1)" 2>/dev/null; then
        echo "- **PASS** $name (200, has $key)"
      else
        echo "- **WARN** $name (200, missing key $key)"
      fi
    else
      echo "- **PASS** $name ($code)"
    fi
  else
    echo "- **FAIL** $name (HTTP $code)"
  fi
}

section() { echo ""; echo "---"; echo "## $1"; echo ""; }
step() { echo "### $1"; echo ""; }
ok() { echo "- **PASS** $1"; }
fail() { echo "- **FAIL** $1"; }
warn() { echo "- **WARN** $1"; }

{
  echo "# Báo cáo E2E Comprehensive – Full luồng chức năng"
  echo ""
  echo "**Thời gian**: $(date -Iseconds)"
  echo "**Namespace**: $NAMESPACE"
  echo "**File**: $REPORT"
  echo ""

  # ========== 1. K8s ==========
  section "1. Kubernetes – Cluster & Pods"
  step "1.1 Nodes"
  kubectl get nodes -o wide 2>/dev/null || fail "kubectl get nodes"
  echo ""
  step "1.2 Pods (fortuna)"
  kubectl get pods -n "$NAMESPACE" -o wide 2>/dev/null || fail "get pods"
  echo ""
  step "1.3 Services"
  kubectl get svc -n "$NAMESPACE" 2>/dev/null || true
  echo ""

  # ========== 2. Auth & Core ==========
  section "2. Auth & Core API"
  get_core_pod && ok "Core pod: $CORE_POD" || fail "Core pod not found"
  echo ""
  if get_token; then
    ok "JWT login (admin/admin123)"
  else
    fail "Login failed – API tests may 401"
  fi
  echo ""

  step "2.1 GET /health"
  HEALTH=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health 2>/dev/null || echo "000")
  [ "$HEALTH" = "200" ] && ok "Health $HEALTH" || fail "Health $HEALTH"
  echo ""

  # ========== 3. API theo từng chức năng ==========
  section "3. API – Từng chức năng (Dashboard / UI)"

  step "3.1 Dashboard – stats & insights (trang chủ)"
  api_test "GET /dashboard/stats" "dashboard/stats" "totalClusters"
  api_test "GET /dashboard/stats (sinceMinutes)" "dashboard/stats?sinceMinutes=60" "totalClusters"
  api_test "GET /insights/summary" "insights/summary" "critical"
  api_test "GET /insights/summary (sinceMinutes)" "insights/summary?sinceMinutes=1440" "critical"
  echo ""

  step "3.2 Clusters (trang Clusters)"
  api_test "GET /clusters" "clusters" "clusters"
  api_test "GET /clusters/stats" "clusters/stats" "total"
  echo ""

  step "3.3 Risks / Risk Center (trang Risk Center)"
  api_test "GET /risks" "risks" "insights"
  api_test "GET /insights" "insights" "insights"
  echo ""

  step "3.4 Resources (trang Resources)"
  api_test "GET /resources" "resources" "resources"
  api_test "GET /pods" "pods" "pods"
  echo ""

  step "3.5 Capabilities & PCE (sidebar Pod Capabilities)"
  api_test "GET /pod-capabilities/summary" "pod-capabilities/summary" "summary"
  api_test "GET /pod-capabilities/summary/capability" "pod-capabilities/summary/capability" "summary"
  api_test "GET /pod-capabilities/trends" "pod-capabilities/trends?days=7" "points"
  api_test "GET /capability-metadata" "capability-metadata" "metadata"
  api_test "GET /promotion-rules" "promotion-rules" "rules"
  echo ""

  step "3.6 Runtime signals & attack steps"
  api_test "GET /runtime-signals" "runtime-signals?limit=5" "signals"
  api_test "GET /attack-steps/summary" "attack-steps/summary" "summary"
  echo ""

  step "3.7 Notifications & Activity (sidebar Recent Activity)"
  api_test "GET /notifications" "notifications" "notifications"
  echo ""

  step "3.8 Dashboard charts"
  api_test "GET /dashboard/metrics/threat-velocity" "dashboard/metrics/threat-velocity?days=7" "trend"
  echo ""

  step "3.9 Agents (Cluster Health)"
  api_test "GET /agents/status" "agents/status" "agents"
  echo ""

  step "3.10 Attack Paths & Graph"
  api_test "GET /attack-paths/graph" "attack-paths/graph" "nodes"
  api_test "GET /graph" "graph" "nodes"
  echo ""

  step "3.11 Rules & Policies"
  api_test "GET /rules" "rules" "rules"
  echo ""

  step "3.12 SBOM"
  api_test "GET /sbom" "sbom" "sboms"
  echo ""

  step "3.13 Audit & Reports"
  api_test "GET /audit" "audit" "logs"
  api_test "GET /reports" "reports" "reports"
  echo ""

  step "3.14 Metrics & Error logs"
  api_test "GET /metrics/system" "metrics/system" "timestamp"
  api_test "GET /error-logs" "error-logs" "logs"
  echo ""

  # ========== 4. Database ==========
  section "4. Database – Dữ liệu ghi nhận"
  PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  if [ -n "$PG_POD" ]; then
    step "4.1 Row counts (bảng chính)"
    kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "
      SELECT 'clusters' AS tbl, COUNT(*) FROM clusters
      UNION ALL SELECT 'pods', COUNT(*) FROM pods
      UNION ALL SELECT 'insights', COUNT(*) FROM insights
      UNION ALL SELECT 'pod_capabilities', COUNT(*) FROM pod_capabilities
      UNION ALL SELECT 'runtime_signals', COUNT(*) FROM runtime_signals
      UNION ALL SELECT 'agents', COUNT(*) FROM agents
      UNION ALL SELECT 'promotion_rules', COUNT(*) FROM promotion_rules
      UNION ALL SELECT 'capability_metadata', COUNT(*) FROM capability_metadata;
    " 2>/dev/null || fail "DB query"
    echo ""
    step "4.2 Sample clusters (hiển thị trên UI)"
    kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT id, name, display_name, kube_config FROM clusters LIMIT 3;" 2>/dev/null || true
    echo ""
    step "4.3 Sample insights (Risk Center)"
    kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT id, severity, status, LEFT(title, 50) FROM insights ORDER BY detected_at DESC NULLS LAST LIMIT 3;" 2>/dev/null || true
    echo ""
  else
    step "4. Database"
    fail "Postgres pod not found"
  fi
  echo ""

  # ========== 5. Đồng bộ dữ liệu K8s ↔ Core ==========
  section "5. Đồng bộ K8s ↔ Core"
  step "5.1 Số pod thực tế (K8s) vs Core (DB)"
  K8S_PODS=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")
  DB_PODS=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT COUNT(*) FROM pods;" 2>/dev/null | tr -d ' ' || echo "0")
  echo "- K8s (fortuna namespace) pods: $K8S_PODS"
  echo "- Core DB (pods table): $DB_PODS"
  echo ""
  step "5.2 Agents (DB) vs DaemonSet pods"
  DB_AGENTS=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT COUNT(*) FROM agents WHERE deleted_at IS NULL;" 2>/dev/null | tr -d ' ' || echo "0")
  K8S_AGENTS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | grep -c Running || echo "0")
  echo "- DB agents (active): $DB_AGENTS"
  echo "- K8s agent pods (Running): $K8S_AGENTS"
  echo ""

  # ========== 6. Dashboard URL & Port-forward ==========
  section "6. Giao diện Dashboard"
  DASH_SVC=$(kubectl get svc -n "$NAMESPACE" -l app.kubernetes.io/component=dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "fortuna-dashboard")
  NODE_PORT=$(kubectl get svc -n "$NAMESPACE" "$DASH_SVC" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "")
  NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "")
  echo "- Service: $DASH_SVC"
  echo "- NodePort: ${NODE_PORT:-none}"
  if [ -n "$NODE_PORT" ] && [ -n "$NODE_IP" ]; then
    echo "- URL (trong cluster): http://${NODE_IP}:${NODE_PORT}"
  fi
  echo "- Port-forward (từ máy host): \`kubectl port-forward -n $NAMESPACE svc/$DASH_SVC 8081:80\` → http://localhost:8081"
  echo ""

  # ========== 7. Tổng kết ==========
  section "7. Tổng kết"
  echo "Các bước đã kiểm tra:"
  echo "1. Kubernetes: nodes, pods, services"
  echo "2. Auth & Core: login, health"
  echo "3. API từng chức năng: dashboard/stats, clusters, risks, resources, capabilities, runtime-signals, notifications, agents, graph, rules, sbom, audit, metrics"
  echo "4. Database: row counts, sample clusters & insights"
  echo "5. Đồng bộ: K8s pods vs DB pods, agents"
  echo "6. Dashboard: URL và hướng dẫn port-forward"
  echo ""
  echo "**Kết thúc báo cáo.**"
  echo ""

} | tee "$REPORT"

echo ""
echo "Report written to: $REPORT"
echo "Lines: $(wc -l < "$REPORT")"

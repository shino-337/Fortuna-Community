#!/usr/bin/env bash
# ============================================================================
# E2E Report with Capability Rules & Cases – Step-by-step, full capability tests
# Output: docs/test-results/E2E-WITH-CAPABILITY-<timestamp>.md
# ============================================================================
# Runs: K8s, Auth, Core API (all sections), DB, Sync, Dashboard URL,
#       then Capability-specific: metadata, promotion rules, pod capabilities,
#       runtime signals, attack steps, and scripts test-pce-e2e, test-promotion-flow,
#       test-runtime-signals-e2e. Writes one report with step-by-step results.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
REPO_ROOT="$PROJECT_ROOT"
NAMESPACE="${NAMESPACE:-fortuna}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_DIR="$REPO_ROOT/docs/test-results"
REPORT="$REPORT_DIR/E2E-WITH-CAPABILITY-$TIMESTAMP.md"
mkdir -p "$REPORT_DIR"

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
  local key="${3:-}"
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
  echo "# Báo cáo E2E – Full luồng + Capability rules & cases"
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
  if get_token; then ok "JWT login (admin/admin123)"; else fail "Login failed"; fi
  echo ""
  step "2.1 GET /health"
  HEALTH=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health 2>/dev/null || echo "000")
  [ "$HEALTH" = "200" ] && ok "Health $HEALTH" || fail "Health $HEALTH"
  echo ""

  # ========== 3. API theo từng chức năng ==========
  section "3. API – Từng chức năng (Dashboard / UI)"
  step "3.1 Dashboard – stats & insights"
  api_test "GET /dashboard/stats" "dashboard/stats" "totalClusters"
  api_test "GET /insights/summary" "insights/summary" "critical"
  echo ""
  step "3.2 Clusters"
  api_test "GET /clusters" "clusters" "clusters"
  api_test "GET /clusters/stats" "clusters/stats" "total"
  echo ""
  step "3.3 Risks / Risk Center"
  api_test "GET /risks" "risks" "insights"
  echo ""
  step "3.4 Resources"
  api_test "GET /resources" "resources" "resources"
  api_test "GET /pods" "pods" "pods"
  echo ""
  step "3.5 Capabilities & PCE"
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
  step "3.7 Notifications, Agents, Rules, SBOM, Audit, Metrics"
  api_test "GET /notifications" "notifications" "notifications"
  api_test "GET /agents/status" "agents/status" "agents"
  api_test "GET /rules" "rules" "rules"
  api_test "GET /sbom" "sbom" "sboms"
  api_test "GET /audit" "audit" "logs"
  api_test "GET /metrics/system" "metrics/system" "timestamp"
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
      UNION ALL SELECT 'capability_metadata', COUNT(*) FROM capability_metadata
      UNION ALL SELECT 'pod_attack_steps', COUNT(*) FROM pod_attack_steps
      UNION ALL SELECT 'pod_risk_profiles', COUNT(*) FROM pod_risk_profiles;
    " 2>/dev/null || fail "DB query"
    echo ""
    step "4.2 Sample capability_metadata (capability catalog)"
    kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT capability_id, domain, category, severity_base FROM capability_metadata ORDER BY capability_id LIMIT 15;" 2>/dev/null || true
    echo ""
    step "4.3 Sample promotion_rules (signal → state)"
    kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT capability_id, signal_type, min_occurrences, promote_to FROM promotion_rules ORDER BY capability_id, signal_type LIMIT 15;" 2>/dev/null || true
    echo ""
    step "4.4 Sample pod_capabilities (state, severity)"
    kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT pod_uid, capability_id, state, severity FROM pod_capabilities ORDER BY updated_at DESC NULLS LAST LIMIT 15;" 2>/dev/null || true
    echo ""
    step "4.5 Sample runtime_signals"
    kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT pod_uid, signal_type, category, confidence FROM runtime_signals ORDER BY created_at DESC LIMIT 10;" 2>/dev/null || true
    echo ""
  else
    step "4. Database"; fail "Postgres pod not found"
  fi
  echo ""

  # ========== 5. Capability rules & cases – API chi tiết ==========
  section "5. Capability rules & cases – API chi tiết"
  step "5.1 GET /capability-metadata (list) – kiểm tra metadata catalog"
  api_test "GET /capability-metadata" "capability-metadata" "metadata"
  echo ""
  step "5.2 GET /capability-metadata/:id – từng capability (ESC_PRIV_POD, ESC_HOSTPATH_NODE)"
  api_test "GET /capability-metadata/ESC_PRIV_POD" "capability-metadata/ESC_PRIV_POD" "capabilityId"
  api_test "GET /capability-metadata/ESC_HOSTPATH_NODE" "capability-metadata/ESC_HOSTPATH_NODE" "capabilityId"
  echo ""
  step "5.3 GET /promotion-rules (list) – quy tắc promotion"
  api_test "GET /promotion-rules" "promotion-rules" "rules"
  echo ""
  step "5.4 GET /promotion-rules/capability/:id – rules theo capability"
  api_test "GET /promotion-rules/capability/ESC_HOSTPATH_NODE" "promotion-rules/capability/ESC_HOSTPATH_NODE" "rules"
  api_test "GET /promotion-rules/signal/PROC_ROOT_PIVOT" "promotion-rules/signal/PROC_ROOT_PIVOT" "rules"
  echo ""
  step "5.5 GET /pod-capabilities (list + filter) – danh sách capability theo pod/cluster"
  api_test "GET /pod-capabilities" "pod-capabilities?limit=5" "capabilities"
  api_test "GET /pod-capabilities/summary/severity" "pod-capabilities/summary/severity" "summary"
  echo ""
  step "5.6 GET /runtime-signals, /runtime-signals/pods/:uid"
  api_test "GET /runtime-signals" "runtime-signals?limit=5" "signals"
  echo ""
  step "5.7 GET /attack-steps/summary, /attack-steps/pods/:uid"
  api_test "GET /attack-steps/summary" "attack-steps/summary" "summary"
  echo ""

  # ========== 6. Đồng bộ K8s ↔ Core ==========
  section "6. Đồng bộ K8s ↔ Core"
  step "6.1 Pods (K8s vs DB)"
  K8S_PODS=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")
  DB_PODS=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;" 2>/dev/null | tr -d ' ' || echo "0")
  echo "- K8s (fortuna) pods: $K8S_PODS | DB pods: $DB_PODS"
  echo ""
  step "6.2 Agents (DB vs DaemonSet)"
  DB_AGENTS=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT COUNT(*) FROM agents WHERE deleted_at IS NULL;" 2>/dev/null | tr -d ' ' || echo "0")
  K8S_AGENTS=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-agent --no-headers 2>/dev/null | grep -c Running || echo "0")
  echo "- DB agents: $DB_AGENTS | K8s agent pods (Running): $K8S_AGENTS"
  echo ""

  # ========== 7. Scripts E2E capability (PCE, promotion, runtime signals) ==========
  section "7. E2E scripts – Capability (PCE, promotion, runtime signals)"
  step "7.1 test-pce-e2e.sh – Pod privileged → PCE evaluation → capabilities DB → runtime event → signals → state"
  if [ -x "$SCRIPTS/e2e/test-pce-e2e.sh" ]; then
    echo '```'
    NAMESPACE="$NAMESPACE" bash "$SCRIPTS/e2e/test-pce-e2e.sh" 2>&1 || true
    echo '```'
  else
    echo "- Script not found or not executable: test-pce-e2e.sh"
  fi
  echo ""
  step "7.2 test-promotion-flow.sh – Pod capabilities, runtime signals, promotion rules"
  if [ -x "$SCRIPTS/e2e/test-promotion-flow.sh" ]; then
    echo '```'
    bash "$SCRIPTS/e2e/test-promotion-flow.sh" 2>&1 || true
    echo '```'
  else
    echo "- Script not found or not executable: test-promotion-flow.sh"
  fi
  echo ""
  step "7.3 test-runtime-signals-e2e.sh – Runtime signals E2E"
  if [ -x "$SCRIPTS/e2e/test-runtime-signals-e2e.sh" ]; then
    echo '```'
    NAMESPACE="$NAMESPACE" bash "$SCRIPTS/e2e/test-runtime-signals-e2e.sh" 2>&1 || true
    echo '```'
  else
    echo "- Script not found or not executable: test-runtime-signals-e2e.sh"
  fi
  echo ""

  # ========== 8. Dashboard URL ==========
  section "8. Giao diện Dashboard"
  DASH_SVC=$(kubectl get svc -n "$NAMESPACE" -l app=fortuna-dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "fortuna-dashboard")
  NODE_PORT=$(kubectl get svc -n "$NAMESPACE" "$DASH_SVC" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "")
  NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "")
  echo "- Service: $DASH_SVC"
  echo "- NodePort: ${NODE_PORT:-none}"
  if [ -n "$NODE_PORT" ] && [ -n "$NODE_IP" ]; then
    echo "- URL (trong cluster): http://${NODE_IP}:${NODE_PORT}"
  fi
  echo "- Port-forward: \`kubectl port-forward -n $NAMESPACE svc/$DASH_SVC 8081:80\` → http://localhost:8081"
  echo ""

  # ========== 9. Tổng kết ==========
  section "9. Tổng kết"
  echo "Các bước đã kiểm tra:"
  echo "1. Kubernetes: nodes, pods, services"
  echo "2. Auth & Core: login, health"
  echo "3. API: dashboard, clusters, risks, resources, **capabilities**, **promotion-rules**, **runtime-signals**, **attack-steps**, notifications, agents, rules, sbom, audit, metrics"
  echo "4. Database: row counts, **capability_metadata**, **promotion_rules**, **pod_capabilities**, **runtime_signals**"
  echo "5. **Capability rules & cases**: metadata by id, promotion rules by capability/signal, pod-capabilities, runtime-signals, attack-steps"
  echo "6. Đồng bộ: K8s pods vs DB pods, agents"
  echo "7. **E2E scripts**: test-pce-e2e.sh, test-promotion-flow.sh, test-runtime-signals-e2e.sh"
  echo "8. Dashboard: URL và port-forward"
  echo ""
  echo "**Kết thúc báo cáo.**"
  echo ""

} | tee "$REPORT"

echo ""
echo "Report written to: $REPORT"
echo "Lines: $(wc -l < "$REPORT")"

#!/usr/bin/env bash
# ============================================================================
# E2E đầy đủ + monitor Agent/Core + ghi nhận kết quả API
# ============================================================================
# 1. Snapshot log Agent/Core (trước)
# 2. Chạy run-e2e-full.sh → E2E-FULL-<ts>.md
# 3. Chạy test-priority1-apis.sh, test-runtime-signals-e2e.sh, e2e-dashboard-data.sh
# 4. Snapshot log Agent/Core (sau)
# 5. Gọi API: login, clusters, dashboard/stats, agents/status, insights/summary → ghi vào report
# 6. Ghi report tổng hợp: docs/test-results/E2E-COMPLETE-WITH-MONITOR-<ts>.md
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_DIR="$PROJECT_ROOT/docs/test-results"
REPORT="$REPORT_DIR/E2E-COMPLETE-WITH-MONITOR-$TIMESTAMP.md"
LOG_SNAPSHOT_BEFORE="$REPORT_DIR/e2e-logs-before-$TIMESTAMP.txt"
LOG_SNAPSHOT_AFTER="$REPORT_DIR/e2e-logs-after-$TIMESTAMP.txt"
API_RESULTS="$REPORT_DIR/e2e-api-results-$TIMESTAMP.txt"

mkdir -p "$REPORT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()  { echo -e "${BLUE}[E2E]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }

echo "=========================================="
echo "E2E Complete + Monitor Agent/Core + API"
echo "=========================================="
echo "  Report: $REPORT"
echo ""

# ----- 1. Log snapshot BEFORE -----
log_info "1. Snapshot Agent/Core logs (before)..."
{
  echo "=== Core (last 40 lines) ==="
  kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=core --tail=40 2>/dev/null || true
  echo ""
  echo "=== Agent (last 50 lines, sync/heartbeat) ==="
  kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=agent --tail=50 2>/dev/null | grep -E 'Syncer|sync|Heartbeat|Register|Connected|Error|failed|OK' || kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=agent --tail=30 2>/dev/null || true
} > "$LOG_SNAPSHOT_BEFORE"
log_ok "Saved $LOG_SNAPSHOT_BEFORE"

# ----- 2. Run E2E full report -----
log_info "2. Running run-e2e-full.sh..."
if [ -x "$SCRIPTS/e2e/run-e2e-full.sh" ]; then
  NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/run-e2e-full.sh" 2>&1 || log_warn "run-e2e-full.sh had errors"
  E2E_FULL=$(ls -t "$REPORT_DIR"/E2E-FULL-*.md 2>/dev/null | head -1)
  [ -n "$E2E_FULL" ] && log_ok "E2E full report: $E2E_FULL"
else
  log_warn "run-e2e-full.sh not found or not executable"
fi
echo ""

# ----- 3. Priority 1 APIs -----
log_info "3. Running test-priority1-apis.sh..."
if [ -x "$SCRIPTS/e2e/test-priority1-apis.sh" ]; then
  NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/test-priority1-apis.sh" 2>&1 | tee -a "$API_RESULTS" || true
else
  echo "test-priority1-apis.sh not found" >> "$API_RESULTS"
fi
echo "" >> "$API_RESULTS"
echo ""

# ----- 4. Runtime signals E2E -----
log_info "4. Running test-runtime-signals-e2e.sh..."
if [ -x "$SCRIPTS/e2e/test-runtime-signals-e2e.sh" ]; then
  NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/test-runtime-signals-e2e.sh" 2>&1 | tee -a "$API_RESULTS" || true
else
  echo "test-runtime-signals-e2e.sh not found" >> "$API_RESULTS"
fi
echo "" >> "$API_RESULTS"
echo ""

# ----- 5. Dashboard data E2E (optional, can be slow) -----
log_info "5. Running e2e-dashboard-data.sh..."
if [ -x "$SCRIPTS/e2e/e2e-dashboard-data.sh" ]; then
  NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/e2e-dashboard-data.sh" 2>&1 | tee -a "$API_RESULTS" || log_warn "e2e-dashboard-data.sh had errors"
else
  echo "e2e-dashboard-data.sh not found" >> "$API_RESULTS"
fi
echo "" >> "$API_RESULTS"
echo ""

# ----- 5b. Risk Center E2E (seed insight + verify /risks, /insights/summary, /runtime-signals) -----
log_info "5b. Running e2e-risk-center-verify.sh (Risk Center)..."
if [ -x "$SCRIPTS/e2e/e2e-risk-center-verify.sh" ]; then
  NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/e2e-risk-center-verify.sh" 2>&1 | tee -a "$API_RESULTS" || log_warn "e2e-risk-center-verify.sh had errors"
else
  echo "e2e-risk-center-verify.sh not found" >> "$API_RESULTS"
fi
echo "" >> "$API_RESULTS"
echo ""

# ----- 6. Log snapshot AFTER -----
log_info "6. Snapshot Agent/Core logs (after)..."
{
  echo "=== Core (last 50 lines) ==="
  kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=core --tail=50 2>/dev/null || true
  echo ""
  echo "=== Agent (last 60 lines, sync/heartbeat) ==="
  kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=agent --tail=60 2>/dev/null | grep -E 'Syncer|sync|Heartbeat|Register|Connected|Error|failed|OK' || kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/component=agent --tail=40 2>/dev/null || true
} > "$LOG_SNAPSHOT_AFTER"
log_ok "Saved $LOG_SNAPSHOT_AFTER"

# ----- 7. API results via port-forward (if available) or exec -----
log_info "7. Recording API results (clusters, dashboard/stats, agents/status, insights/summary)..."
CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$CORE_POD" ]; then
  CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
fi
if [ -n "$CORE_POD" ]; then
  LOGIN=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' 2>/dev/null || echo "{}")
  TOKEN=$(echo "$LOGIN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null || echo "")
  if [ -n "$TOKEN" ]; then
    echo "=== GET /api/v1/clusters ===" >> "$API_RESULTS"
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/clusters 2>/dev/null | python3 -m json.tool 2>/dev/null >> "$API_RESULTS" || true
    echo "" >> "$API_RESULTS"
    echo "=== GET /api/v1/dashboard/stats ===" >> "$API_RESULTS"
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/dashboard/stats 2>/dev/null | python3 -m json.tool 2>/dev/null >> "$API_RESULTS" || true
    echo "" >> "$API_RESULTS"
    echo "=== GET /api/v1/agents/status ===" >> "$API_RESULTS"
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/agents/status 2>/dev/null | python3 -m json.tool 2>/dev/null >> "$API_RESULTS" || true
    echo "" >> "$API_RESULTS"
    echo "=== GET /api/v1/insights/summary ===" >> "$API_RESULTS"
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/insights/summary 2>/dev/null | python3 -m json.tool 2>/dev/null >> "$API_RESULTS" || true
    log_ok "API results written to $API_RESULTS"
  else
    echo "Login failed; API results not recorded" >> "$API_RESULTS"
  fi
else
  echo "Core pod not found; API results not recorded" >> "$API_RESULTS"
fi

# ----- 8. Write consolidated report -----
log_info "8. Writing consolidated report..."
{
  echo "# E2E Complete – Monitor Agent/Core & API kết quả thực tế"
  echo ""
  echo "**Thời gian:** $(date -Iseconds)"
  echo "**Namespace:** $NAMESPACE"
  echo ""
  echo "---"
  echo "## 1. Test scripts đã chạy"
  echo ""
  echo "| Script | Mô tả |"
  echo "|--------|--------|"
  echo "| run-e2e-full.sh | E2E full (cluster, API, DB, dashboard) |"
  echo "| test-priority1-apis.sh | Promotion rules, runtime-signals API |"
  echo "| test-runtime-signals-e2e.sh | POST runtime-events, DB, GET runtime-signals |"
  echo "| e2e-dashboard-data.sh | Dashboard chart data (Threat Velocity, PCE Trend) |"
  echo "| e2e-risk-center-verify.sh | Risk Center: seed insight, /risks, /insights/summary, /runtime-signals |"
  echo ""
  echo "---"
  echo "## 2. Kết quả API thực tế (clusters, dashboard/stats, agents/status, insights/summary, Risk Center)"
  echo ""
  echo "\`\`\`json"
  cat "$API_RESULTS" 2>/dev/null | tail -100
  echo "\`\`\`"
  echo ""
  echo "---"
  echo "## 3. Agent/Core logs (trước khi chạy E2E)"
  echo ""
  echo "\`\`\`"
  cat "$LOG_SNAPSHOT_BEFORE" 2>/dev/null
  echo "\`\`\`"
  echo ""
  echo "---"
  echo "## 4. Agent/Core logs (sau khi chạy E2E)"
  echo ""
  echo "\`\`\`"
  cat "$LOG_SNAPSHOT_AFTER" 2>/dev/null
  echo "\`\`\`"
  echo ""
  echo "---"
  echo "## 5. File báo cáo chi tiết"
  echo ""
  echo "- E2E full: \`docs/test-results/E2E-FULL-*.md\` (mới nhất)"
  echo "- API raw: \`$API_RESULTS\`"
  echo "- Log before: \`$LOG_SNAPSHOT_BEFORE\`"
  echo "- Log after: \`$LOG_SNAPSHOT_AFTER\`"
  echo ""
} > "$REPORT"
log_ok "Report written: $REPORT"
echo ""
echo "=========================================="
echo "E2E Complete + Monitor finished"
echo "=========================================="
echo "  Report: $REPORT"
echo "  API results: $API_RESULTS"
echo "  Logs before: $LOG_SNAPSHOT_BEFORE"
echo "  Logs after:  $LOG_SNAPSHOT_AFTER"
echo ""

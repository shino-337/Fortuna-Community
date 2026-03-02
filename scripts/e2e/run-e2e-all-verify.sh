#!/usr/bin/env bash
# ============================================================================
# E2E toàn bộ testcase – verify và báo cáo kết quả chi tiết
# ============================================================================
# Thực thi: check-full-deployment → run-e2e-full → test-priority1-apis →
#           test-runtime-signals-e2e → e2e-dashboard-data → e2e-risk-center-verify →
#           (tùy chọn) run-e2e-with-capability-report
# Báo cáo: docs/test-results/E2E-ALL-VERIFY-<timestamp>.md (tóm tắt + link chi tiết)
# ============================================================================
# Usage: ./scripts/e2e/run-e2e-all-verify.sh [--skip-capability-report]
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_DIR="$PROJECT_ROOT/docs/test-results"
REPORT="$REPORT_DIR/E2E-ALL-VERIFY-$TIMESTAMP.md"
SUMMARY_TMP="$REPORT_DIR/.e2e-summary-$TIMESTAMP.txt"
mkdir -p "$REPORT_DIR"

SKIP_CAPABILITY_REPORT=false
[ "${1:-}" = "--skip-capability-report" ] && SKIP_CAPABILITY_REPORT=true

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()  { echo -e "${BLUE}[E2E]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_fail()  { echo -e "${RED}[FAIL]${NC} $1"; }

run_step() {
  local name="$1"
  local cmd="$2"
  local out
  if out=$(eval "$cmd" 2>&1); then
    echo "| $name | PASS |" >> "$SUMMARY_TMP"
    log_ok "$name"
    return 0
  else
    echo "| $name | FAIL |" >> "$SUMMARY_TMP"
    log_fail "$name"
    echo "$out" | tail -20
    return 1
  fi
}

echo "=========================================="
echo "E2E Toàn bộ testcase – Verify & Báo cáo"
echo "=========================================="
echo "  Report: $REPORT"
echo "  Namespace: $NAMESPACE"
echo ""

{
  echo "## Tóm tắt E2E (run tại $(date -Iseconds))"
  echo ""
  echo "| Bước | Kết quả |"
  echo "|------|--------|"
} > "$SUMMARY_TMP"

# ----- 1. Full deployment check -----
log_info "1. check-full-deployment.sh..."
run_step "check-full-deployment" "NAMESPACE=$NAMESPACE $SCRIPTS/verify/check-full-deployment.sh" || true
echo ""

# ----- 2. E2E full (cluster, API, DB, dashboard, test-pce-api, test-priority1, unit) -----
log_info "2. run-e2e-full.sh (ghi E2E-FULL-*.md)..."
E2E_FULL_OUT="$REPORT_DIR/.e2e-full-$TIMESTAMP.log"
if NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/run-e2e-full.sh" > "$E2E_FULL_OUT" 2>&1; then
  echo "| run-e2e-full.sh | PASS |" >> "$SUMMARY_TMP"
  log_ok "run-e2e-full.sh"
else
  echo "| run-e2e-full.sh | FAIL |" >> "$SUMMARY_TMP"
  log_fail "run-e2e-full.sh"
  tail -30 "$E2E_FULL_OUT" || true
fi
E2E_FULL_LATEST=$(ls -t "$REPORT_DIR"/E2E-FULL-*.md 2>/dev/null | head -1 || echo "")
echo ""

# ----- 3. Priority 1 APIs -----
log_info "3. test-priority1-apis.sh..."
run_step "test-priority1-apis" "NAMESPACE=$NAMESPACE $SCRIPTS/e2e/test-priority1-apis.sh" || true
echo ""

# ----- 4. Runtime signals E2E -----
log_info "4. test-runtime-signals-e2e.sh..."
run_step "test-runtime-signals-e2e" "NAMESPACE=$NAMESPACE $SCRIPTS/e2e/test-runtime-signals-e2e.sh" || true
echo ""

# ----- 5. Dashboard data E2E -----
log_info "5. e2e-dashboard-data.sh..."
run_step "e2e-dashboard-data" "NAMESPACE=$NAMESPACE $SCRIPTS/e2e/e2e-dashboard-data.sh" || true
echo ""

# ----- 6. Risk Center E2E -----
log_info "6. e2e-risk-center-verify.sh..."
run_step "e2e-risk-center-verify" "NAMESPACE=$NAMESPACE $SCRIPTS/e2e/e2e-risk-center-verify.sh" || true
echo ""

# ----- 7. Capability report (optional) -----
if [ "$SKIP_CAPABILITY_REPORT" = false ] && [ -x "$SCRIPTS/e2e/run-e2e-with-capability-report.sh" ]; then
  log_info "7. run-e2e-with-capability-report.sh..."
  if NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/run-e2e-with-capability-report.sh" 2>&1; then
    echo "| run-e2e-with-capability-report.sh | PASS |" >> "$SUMMARY_TMP"
    log_ok "run-e2e-with-capability-report.sh"
  else
    echo "| run-e2e-with-capability-report.sh | FAIL |" >> "$SUMMARY_TMP"
    log_fail "run-e2e-with-capability-report.sh"
  fi
  echo ""
else
  [ "$SKIP_CAPABILITY_REPORT" = true ] && echo "| run-e2e-with-capability-report.sh | SKIP |" >> "$SUMMARY_TMP"
fi

# ----- Write consolidated report -----
log_info "Writing report $REPORT..."
{
  echo "# E2E Toàn bộ – Báo cáo kết quả verify"
  echo ""
  echo "**Thời gian:** $(date -Iseconds)"
  echo "**Namespace:** $NAMESPACE"
  echo ""
  echo "---"
  echo ""
  cat "$SUMMARY_TMP"
  echo ""
  echo "---"
  echo "## Chi tiết từng bước"
  echo ""
  echo "### 1. Full deployment check"
  echo "Lệnh: \`./scripts/verify/check-full-deployment.sh\`"
  echo ""
  echo "### 2. E2E full (cluster, Core API, DB, dashboard, PCE API, Priority1 APIs, unit tests)"
  echo "Script: \`./scripts/e2e/run-e2e-full.sh\`"
  if [ -n "$E2E_FULL_LATEST" ]; then
    echo "Báo cáo chi tiết: [E2E-FULL]($(basename "$E2E_FULL_LATEST"))"
  fi
  echo ""
  echo "### 3. Priority 1 APIs"
  echo "Script: \`./scripts/e2e/test-priority1-apis.sh\`"
  echo ""
  echo "### 4. Runtime signals E2E"
  echo "Script: \`./scripts/e2e/test-runtime-signals-e2e.sh\`"
  echo ""
  echo "### 5. Dashboard data E2E"
  echo "Script: \`./scripts/e2e/e2e-dashboard-data.sh\`"
  echo ""
  echo "### 6. Risk Center E2E"
  echo "Script: \`./scripts/e2e/e2e-risk-center-verify.sh\`"
  echo ""
  echo "### 7. E2E with capability report (optional)"
  echo "Script: \`./scripts/e2e/run-e2e-with-capability-report.sh\`"
  echo "Báo cáo: \`docs/test-results/E2E-WITH-CAPABILITY-*.md\`"
  echo ""
  echo "---"
  echo "## File báo cáo trong thư mục"
  echo ""
  echo "\`\`\`"
  ls -la "$REPORT_DIR"/*.md 2>/dev/null | tail -15 || ls "$REPORT_DIR" | tail -15
  echo "\`\`\`"
  echo ""
  echo "---"
  echo "**Kết thúc báo cáo.**"
} > "$REPORT"

rm -f "$SUMMARY_TMP"
log_ok "Report written: $REPORT"
echo ""
echo "=========================================="
echo "E2E All Verify finished"
echo "=========================================="
echo "  Report: $REPORT"
echo ""

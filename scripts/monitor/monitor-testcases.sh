#!/usr/bin/env bash
# ============================================================================
# Monitor chi tiết các testcase – chạy lần lượt và in kết quả từng bước
# ============================================================================
# Usage:
#   ./scripts/monitor/monitor-testcases.sh              # in ra stdout
#   ./scripts/monitor/monitor-testcases.sh --report F   # in ra stdout và ghi vào file F
# Env: NAMESPACE=fortuna (mặc định), SKIP_DEPLOYMENT=1, SKIP_PRIORITY1=1, ...
#      để bỏ qua từng nhóm test (bất kỳ giá trị không rỗng = skip).
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"
REPORT_FILE=""

[ "${1:-}" = "--report" ] && [ -n "${2:-}" ] && REPORT_FILE="$2"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
section() { echo -e "\n${BLUE}========== $1 ==========${NC}"; }
run_script() {
  local name="$1"
  local path="$2"
  local ec=0
  if [ ! -x "$path" ]; then
    echo -e "${YELLOW}[SKIP]${NC} $name (not found or not executable: $path)"
    return 0
  fi
  section "$name"
  if [ -n "$REPORT_FILE" ]; then
    NAMESPACE="$NAMESPACE" "$path" 2>&1 | tee -a "$REPORT_FILE"
    ec=${PIPESTATUS[0]}
  else
    NAMESPACE="$NAMESPACE" "$path" 2>&1
    ec=$?
  fi
  if [ "$ec" -eq 0 ]; then
    echo -e "${GREEN}[PASS]${NC} $name"
    return 0
  else
    echo -e "${RED}[FAIL]${NC} $name (exit $ec)"
    return 1
  fi
}

echo "=========================================="
echo "Monitor testcase – $(date -Iseconds)"
echo "Namespace: $NAMESPACE"
echo "=========================================="

[ -n "$REPORT_FILE" ] && { : > "$REPORT_FILE"; echo "Report file: $REPORT_FILE"; }

FAIL_COUNT=0

# 1. Full deployment check
if [ -z "${SKIP_DEPLOYMENT:-}" ]; then
  run_script "check-full-deployment" "$SCRIPTS/verify/check-full-deployment.sh" || FAIL_COUNT=$((FAIL_COUNT+1))
else
  echo -e "\n${YELLOW}[SKIP]${NC} check-full-deployment (SKIP_DEPLOYMENT set)"
fi

# 2. Priority 1 APIs
if [ -z "${SKIP_PRIORITY1:-}" ]; then
  run_script "test-priority1-apis" "$SCRIPTS/e2e/test-priority1-apis.sh" || FAIL_COUNT=$((FAIL_COUNT+1))
else
  echo -e "\n${YELLOW}[SKIP]${NC} test-priority1-apis (SKIP_PRIORITY1 set)"
fi

# 3. Runtime signals E2E
if [ -z "${SKIP_RUNTIME_SIGNALS:-}" ]; then
  run_script "test-runtime-signals-e2e" "$SCRIPTS/e2e/test-runtime-signals-e2e.sh" || FAIL_COUNT=$((FAIL_COUNT+1))
else
  echo -e "\n${YELLOW}[SKIP]${NC} test-runtime-signals-e2e (SKIP_RUNTIME_SIGNALS set)"
fi

# 4. Risk Center E2E
if [ -z "${SKIP_RISK_CENTER:-}" ]; then
  run_script "e2e-risk-center-verify" "$SCRIPTS/e2e/e2e-risk-center-verify.sh" || FAIL_COUNT=$((FAIL_COUNT+1))
else
  echo -e "\n${YELLOW}[SKIP]${NC} e2e-risk-center-verify (SKIP_RISK_CENTER set)"
fi

# 5. Dashboard API verify
if [ -z "${SKIP_DASHBOARD_API:-}" ] && [ -x "$SCRIPTS/verify/verify-dashboard-api.sh" ]; then
  run_script "verify-dashboard-api" "$SCRIPTS/verify/verify-dashboard-api.sh" || FAIL_COUNT=$((FAIL_COUNT+1))
fi

# 6. Agent–Core connectivity
if [ -z "${SKIP_AGENT_CORE:-}" ] && [ -x "$SCRIPTS/verify/verify-agent-core-connectivity.sh" ]; then
  run_script "verify-agent-core-connectivity" "$SCRIPTS/verify/verify-agent-core-connectivity.sh" || FAIL_COUNT=$((FAIL_COUNT+1))
fi

section "Kết thúc"
echo "Chi tiết tất cả testcase: docs/TESTCASE_MONITOR.md"
echo "Chạy toàn bộ E2E + báo cáo: ./scripts/e2e/run-e2e-all-verify.sh"
if [ -n "$REPORT_FILE" ]; then
  echo "Đã ghi: $REPORT_FILE"
fi
[ "$FAIL_COUNT" -gt 0 ] && echo -e "${RED}Tổng số bước FAIL: $FAIL_COUNT${NC}" && exit 1
echo -e "${GREEN}Tất cả bước đã chạy (có thể có FAIL ở trên).${NC}"
exit 0

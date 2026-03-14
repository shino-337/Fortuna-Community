#!/usr/bin/env bash
# ============================================================================
# E2E Runner – Entry point duy nhất cho E2E
# ============================================================================
# Chạy theo suite: risk-center | full | priority1 | runtime | pce | dashboard |
#                  sbom | full-report
# Chi tiết test case và luồng code: docs/e2e/E2E-TestCases-And-Runner.md
# ============================================================================
# Usage:
#   ./scripts/e2e/run-e2e.sh                  # default: full
#   ./scripts/e2e/run-e2e.sh --suite=risk-center
#   NAMESPACE=my-ns ./scripts/e2e/run-e2e.sh --suite=priority1
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"
SUITE="${SUITE:-full}"

# Parse --suite=...
for arg in "$@"; do
  if [[ "$arg" =~ ^--suite=(.+)$ ]]; then
    SUITE="${BASH_REMATCH[1]}"
    break
  fi
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info() { echo -e "${BLUE}[E2E]${NC} $1"; }
log_ok()   { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; }

echo "=========================================="
echo " E2E Runner – suite=$SUITE"
echo "=========================================="
echo "  Namespace: $NAMESPACE"
echo "  Doc: docs/e2e/E2E-TestCases-And-Runner.md"
echo ""

run_script() {
  local name="$1"
  local path="$2"
  if [ -x "$path" ]; then
    log_info "Running $name..."
    if NAMESPACE="$NAMESPACE" "$path"; then
      log_ok "$name"
      return 0
    else
      log_fail "$name"
      return 1
    fi
  else
    log_warn "Script not found or not executable: $path"
    return 1
  fi
}

case "$SUITE" in
  risk-center)
    run_script "e2e-risk-center-full.sh (17 TCs)" "$SCRIPTS/e2e/e2e-risk-center-full.sh"
    ;;
  priority1)
    run_script "test-priority1-apis.sh" "$SCRIPTS/e2e/test-priority1-apis.sh"
    ;;
  runtime)
    run_script "test-runtime-signals-e2e.sh" "$SCRIPTS/e2e/test-runtime-signals-e2e.sh"
    ;;
  pce)
    run_script "test-pce-e2e.sh" "$SCRIPTS/e2e/test-pce-e2e.sh"
    run_script "test-promotion-flow.sh" "$SCRIPTS/e2e/test-promotion-flow.sh"
    ;;
  dashboard)
    run_script "e2e-dashboard-data.sh" "$SCRIPTS/e2e/e2e-dashboard-data.sh"
    ;;
  sbom)
    run_script "e2e-sbom-verify.sh" "$SCRIPTS/e2e/e2e-sbom-verify.sh"
    ;;
  full-report)
    if [ -x "$SCRIPTS/e2e/run-e2e-with-capability-report.sh" ]; then
      log_info "Running run-e2e-with-capability-report.sh..."
      NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/run-e2e-with-capability-report.sh" && log_ok "run-e2e-with-capability-report.sh" || log_fail "run-e2e-with-capability-report.sh"
    else
      run_script "run-e2e-full.sh" "$SCRIPTS/e2e/run-e2e-full.sh"
    fi
    ;;
  full)
    # Full: deployment check (optional) + run-e2e-full + Risk Center 17 TCs + priority1
    FAIL=0
    if [ -x "$SCRIPTS/verify/check-full-deployment.sh" ]; then
      log_info "1. check-full-deployment.sh..."
      NAMESPACE="$NAMESPACE" "$SCRIPTS/verify/check-full-deployment.sh" 2>/dev/null || true
    fi
    log_info "2. run-e2e-full.sh (report E2E-FULL-*.md)..."
    NAMESPACE="$NAMESPACE" "$SCRIPTS/e2e/run-e2e-full.sh" 2>/dev/null || FAIL=$((FAIL+1))
    log_info "3. e2e-risk-center-full.sh (17 TCs, report risk-center-e2e-*.md)..."
    run_script "e2e-risk-center-full.sh" "$SCRIPTS/e2e/e2e-risk-center-full.sh" || FAIL=$((FAIL+1))
    log_info "4. test-priority1-apis.sh..."
    run_script "test-priority1-apis.sh" "$SCRIPTS/e2e/test-priority1-apis.sh" || true
    echo ""
    if [ "$FAIL" -gt 0 ]; then
      log_fail "Full suite finished with $FAIL failed step(s)."
      exit 1
    fi
    log_ok "Full suite done."
    ;;
  *)
    echo "Unknown suite: $SUITE"
    echo "  Valid: risk-center | full | priority1 | runtime | pce | dashboard | sbom | full-report"
    exit 1
    ;;
esac

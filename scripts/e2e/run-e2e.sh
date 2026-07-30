#!/usr/bin/env bash
# ============================================================================
# E2E Runner – Entry point duy nhất cho E2E
# ============================================================================
# Chạy theo suite: full | full-report | risk-center | priority1 | runtime |
#                  runtime-gap | falco-runtime | pce | dashboard |
#                  consistency | pod-detail | pod-detail-live |
#                  sbom | sbom-full | sbom-quality
# SBOM-related suites: see scripts/README.md (E2E section).
# ============================================================================
# Usage:
#   ./scripts/e2e/run-e2e.sh                  # default: full
#   ./scripts/e2e/run-e2e.sh --list
#   ./scripts/e2e/run-e2e.sh --suite=risk-center
#   NAMESPACE=my-ns ./scripts/e2e/run-e2e.sh --suite=priority1
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"
SUITE="${SUITE:-full}"

print_suites() {
  cat <<'EOF'
Available E2E suites:
  full             Deployment sanity + full API report + risk-center + priority1
  full-report      Long-form capability report under test-results/
  risk-center      Risk Center matrix and API coverage
  priority1        Priority API smoke checks
  runtime          Runtime signal API flow
  runtime-gap      Runtime coverage gaps, matrix, and toxic-combo checks
  falco-runtime    Falco event ingestion flow
  pce              Pod Capability Engine and promotion flow
  dashboard        Dashboard data population and API verification
  consistency      K8s/DB/API consistency and pod-delete cleanup checks
  pod-detail       Unit/DB/API pod-detail verification report
  pod-detail-live  Live pod-detail process/network workload checks
  sbom             Default SBOM API verification
  sbom-full        Busybox + distroless + CoreDNS SBOM flow
  sbom-quality     SBOM confidence, component quality, and audit checks
EOF
}

# Parse --suite=...
for arg in "$@"; do
  if [[ "$arg" == "--list" || "$arg" == "-h" || "$arg" == "--help" ]]; then
    print_suites
    exit 0
  fi
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
echo "  See: scripts/README.md (E2E & integration scripts)"
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

CORE_PORT_FORWARD_PID=""
stop_core_port_forward() {
  if [ -n "${CORE_PORT_FORWARD_PID:-}" ]; then
    kill "$CORE_PORT_FORWARD_PID" 2>/dev/null || true
  fi
}

start_core_port_forward() {
  # SBOM scripts use http://localhost:8080 by default.
  # Create a port-forward to make localhost reachable from the runner host.
  pkill -f "kubectl.*port-forward.*svc/fortuna-core" 2>/dev/null || true

  log_info "Starting port-forward: svc/fortuna-core 8080:8080 -> localhost:8080..."
  : > /tmp/e2e-core-port-forward.log 2>/dev/null || true
  kubectl -n "$NAMESPACE" port-forward "svc/fortuna-core" 8080:8080 \
    > /tmp/e2e-core-port-forward.log 2>&1 &
  CORE_PORT_FORWARD_PID=$!

  # /healthz has no auth; wait until it answers (or time out quickly).
  local code="000"
  for _ in {1..30}; do
    code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/healthz" 2>/dev/null || echo "000")
    if [ "$code" != "000" ]; then
      log_ok "Core reachable via localhost:8080 (GET /healthz => $code)"
      return 0
    fi
    sleep 1
  done

  log_warn "Core not reachable via localhost:8080/healthz within ~30s (last http_code=$code). See /tmp/e2e-core-port-forward.log"
  return 1
}

trap stop_core_port_forward EXIT

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
  runtime-gap)
    FAIL=0
    run_script "test-runtime-gap-e2e.sh" "$SCRIPTS/e2e/test-runtime-gap-e2e.sh" || FAIL=$((FAIL+1))
    run_script "test-toxic-combo-hostns-escape-runtime.sh" "$SCRIPTS/e2e/test-toxic-combo-hostns-escape-runtime.sh" || FAIL=$((FAIL+1))
    if [ "$FAIL" -gt 0 ]; then
      log_fail "runtime-gap: $FAIL step(s) failed."
      exit 1
    fi
    ;;
  falco-runtime)
    run_script "test-falco-runtime-e2e.sh" "$SCRIPTS/e2e/test-falco-runtime-e2e.sh"
    ;;
  pce)
    run_script "test-pce-e2e.sh" "$SCRIPTS/e2e/test-pce-e2e.sh"
    run_script "test-promotion-flow.sh" "$SCRIPTS/e2e/test-promotion-flow.sh"
    ;;
  dashboard)
    run_script "e2e-dashboard-data.sh" "$SCRIPTS/e2e/e2e-dashboard-data.sh"
    ;;
  consistency)
    FAIL=0
    run_script "test-dashboard-consistency-e2e.sh" "$SCRIPTS/e2e/test-dashboard-consistency-e2e.sh" || FAIL=$((FAIL+1))
    run_script "e2e-pod-delete-cleanup-verify.sh" "$SCRIPTS/e2e/e2e-pod-delete-cleanup-verify.sh" || FAIL=$((FAIL+1))
    if [ "$FAIL" -gt 0 ]; then
      log_fail "consistency: $FAIL step(s) failed."
      exit 1
    fi
    ;;
  pod-detail)
    run_script "run-pod-detail-test-suite.sh" "$SCRIPTS/e2e/run-pod-detail-test-suite.sh"
    ;;
  pod-detail-live)
    FAIL=0
    run_script "test-pod-detail-ping-flow.sh" "$SCRIPTS/e2e/test-pod-detail-ping-flow.sh" || FAIL=$((FAIL+1))
    run_script "test-pod-detail-lodash-network.sh" "$SCRIPTS/e2e/test-pod-detail-lodash-network.sh" || FAIL=$((FAIL+1))
    if [ "$FAIL" -gt 0 ]; then
      log_fail "pod-detail-live: $FAIL step(s) failed."
      exit 1
    fi
    ;;
  sbom)
    export CORE_API_URL="http://localhost:8080"
    start_core_port_forward || true
    run_script "e2e-sbom-verify.sh (default pod)" "$SCRIPTS/e2e/e2e-sbom-verify.sh"
    ;;
  sbom-full)
    # SBOM luồng hiện tại: busybox + distroless + CoreDNS (kiểu control-plane)
    export CORE_API_URL="http://localhost:8080"
    start_core_port_forward || true
    FAIL=0
    run_script "test-sbom-pod-flow.sh (busybox)" "$SCRIPTS/e2e/test-sbom-pod-flow.sh" || FAIL=$((FAIL+1))
    run_script "test-sbom-distroless-hello.sh" "$SCRIPTS/e2e/test-sbom-distroless-hello.sh" || FAIL=$((FAIL+1))
    run_script "test-sbom-control-plane-coredns.sh" "$SCRIPTS/e2e/test-sbom-control-plane-coredns.sh" || FAIL=$((FAIL+1))
    if [ "$FAIL" -gt 0 ]; then
      log_fail "sbom-full: $FAIL step(s) failed."
      exit 1
    fi
    log_ok "sbom-full: all SBOM E2E scripts passed."
    ;;
  sbom-quality)
    export CORE_API_URL="http://localhost:8080"
    start_core_port_forward || true
    FAIL=0
    run_script "test-sbom-confidence-levels.sh" "$SCRIPTS/e2e/test-sbom-confidence-levels.sh" || FAIL=$((FAIL+1))
    run_script "test-sbom-distroless-components.sh" "$SCRIPTS/e2e/test-sbom-distroless-components.sh" || FAIL=$((FAIL+1))
    run_script "test-sbom-audit-trail-cleanup.sh" "$SCRIPTS/e2e/test-sbom-audit-trail-cleanup.sh" || FAIL=$((FAIL+1))
    run_script "test-sbom-control-plane-coredns-audit.sh" "$SCRIPTS/e2e/test-sbom-control-plane-coredns-audit.sh" || FAIL=$((FAIL+1))
    if [ "$FAIL" -gt 0 ]; then
      log_fail "sbom-quality: $FAIL step(s) failed."
      exit 1
    fi
    log_ok "sbom-quality: all SBOM quality scripts passed."
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
    print_suites
    exit 1
    ;;
esac

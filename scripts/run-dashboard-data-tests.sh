#!/usr/bin/env bash
# ============================================================================
# Run test cases that populate data for Dashboard charts, then verify APIs.
# Use after deploy so Threat Velocity and PCE Trend charts show real data.
# ============================================================================
# 1. Optionally load CVE data (if cve-data/ exists) for vulnerability insights.
# 2. E2E dashboard data: create pod, wait for Core sync, POST runtime-events,
#    trigger insights/evaluate/historical, verify threat-velocity & PCE trends.
# 3. PCE E2E test (privileged pod + wait for Core sync + capabilities + runtime).
# 4. Optionally SBOM pod flow (pod + SBOM → CVE match → insights).
# 5. Verify dashboard APIs; print reminder: log in, refresh dashboard.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_ok()      { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }

echo "=========================================="
echo "Dashboard chart data: run tests + verify"
echo "=========================================="
echo ""

# 1. Load CVE data (optional)
if [ -d "${CVE_DATA_DIR:-$PROJECT_ROOT/cve-data}/all" ] && [ -x "$SCRIPT_DIR/load-cve-data.sh" ]; then
  log_info "Step 1: Loading CVE data (for vulnerability insights / Threat Velocity)..."
  if "$SCRIPT_DIR/load-cve-data.sh" 2>/dev/null; then
    log_ok "CVE data loaded"
  else
    log_warn "CVE load skipped or failed (Threat Velocity may still show existing insights)"
  fi
else
  log_info "Step 1: CVE data dir not found or load script missing; skip (Threat Velocity uses existing insights)"
fi
echo ""

# 1b. Deploy E2E vuln pod (optional – more SBOM/CVE data for Threat Velocity)
if [ -f "$PROJECT_ROOT/deploy/e2e/ksam-e2e-vuln-pod.yaml" ]; then
  log_info "Step 1b: Deploying E2E vuln pod (debian:10 for CVE insights)..."
  kubectl apply -f "$PROJECT_ROOT/deploy/e2e/ksam-e2e-vuln-pod.yaml" 2>/dev/null || true
  sleep 30
  log_ok "E2E vuln pod applied (agent will sync and extract SBOM)"
else
  log_info "Step 1b: deploy/e2e/ksam-e2e-vuln-pod.yaml not found; skip"
fi
echo ""

# 2. E2E dashboard data (pod + Core sync wait + runtime-events + insights evaluate + verify)
log_info "Step 2: Running E2E dashboard data (pod, runtime-events, insights evaluate, verify)..."
if [ -x "$SCRIPT_DIR/e2e-dashboard-data.sh" ]; then
  if "$SCRIPT_DIR/e2e-dashboard-data.sh" 2>/dev/null; then
    log_ok "E2E dashboard data done"
  else
    log_warn "E2E dashboard data had errors (charts may still show existing data)"
  fi
else
  log_warn "e2e-dashboard-data.sh not found or not executable"
fi
echo ""

# 3. PCE E2E (pod_capabilities + runtime_signals → PCE Trend chart)
log_info "Step 3: Running PCE E2E test (privileged pod → wait for Core sync → capabilities + runtime)..."
if [ -x "$SCRIPT_DIR/test-pce-e2e.sh" ]; then
  if "$SCRIPT_DIR/test-pce-e2e.sh" 2>/dev/null; then
    log_ok "PCE E2E done (pod_capabilities + runtime_signals populated)"
  else
    log_warn "PCE E2E had errors (PCE Trend may still show existing data)"
  fi
else
  log_warn "test-pce-e2e.sh not found or not executable"
fi
echo ""

# 4. SBOM pod flow (optional – creates pod, SBOM, can trigger CVE insights)
log_info "Step 4: Running SBOM pod flow (optional – new pod + SBOM for insights)..."
if [ -x "$SCRIPT_DIR/test-sbom-pod-flow.sh" ]; then
  if "$SCRIPT_DIR/test-sbom-pod-flow.sh" 2>/dev/null; then
    log_ok "SBOM pod flow done"
  else
    log_warn "SBOM pod flow skipped or failed"
  fi
else
  log_info "Step 3: test-sbom-pod-flow.sh not found; skip"
fi
echo ""

# 5. Verify dashboard APIs
log_info "Step 5: Verifying dashboard APIs..."
if [ -x "$SCRIPT_DIR/verify-dashboard-apis.sh" ]; then
  "$SCRIPT_DIR/verify-dashboard-apis.sh" 2>&1 || true
else
  log_warn "verify-dashboard-apis.sh not found"
fi
echo ""

echo "=========================================="
log_ok "Dashboard data test run finished."
echo "=========================================="
echo ""
echo "Charts (Threat Velocity, PCE Trend) use:"
echo "  - GET /api/v1/dashboard/metrics/threat-velocity?days=7  (insights by date)"
echo "  - GET /api/v1/pod-capabilities/trends?days=7             (pod_capabilities by date)"
echo ""
echo "To see data on Dashboard:"
echo "  1. Port-forward: kubectl port-forward -n $NAMESPACE svc/fortuna-dashboard 8081:80"
echo "  2. Open: http://localhost:8081"
echo "  3. Log in: admin / admin123"
echo "  4. Refresh the page; charts should show 7 days (non-zero if data exists)."
echo ""
echo "Monitor Core/Agent (optional): ./scripts/monitor-agent-core.sh --logs 25"
echo ""

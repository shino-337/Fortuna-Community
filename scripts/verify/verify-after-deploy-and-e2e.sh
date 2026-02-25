#!/usr/bin/env bash
# ============================================================================
# Verify after clean/rebuild/deploy: migrations (built-in seed), APIs, E2E, actual data.
# Run after: scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset
# Usage: NAMESPACE=fortuna REPORT_DIR=docs/test-results bash scripts/verify/verify-after-deploy-and-e2e.sh
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
REPORT_DIR="${REPORT_DIR:-$PROJECT_ROOT/docs/test-results}"
REPORT_FILE="$REPORT_DIR/verify-after-deploy-$(date +%Y%m%d-%H%M%S).md"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()  { echo -e "${BLUE}[VERIFY]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_fail()  { echo -e "${RED}[FAIL]${NC} $1"; }

mkdir -p "$REPORT_DIR"

echo "=========================================="
echo "Verify after deploy: migrations, APIs, data, E2E"
echo "=========================================="
echo ""

# --- 1. Pods status ---
log_info "1. Pods status (namespace=$NAMESPACE)"
kubectl get pods -n "$NAMESPACE" -o wide 2>/dev/null || true
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || kubectl get pods -n "$NAMESPACE" -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$CORE_POD" ]; then
  log_fail "Core pod not found. Deploy first."
  exit 1
fi
log_ok "Core pod: $CORE_POD"
echo ""

# --- 2. Core logs: migrations & built-in seed ---
log_info "2. Core logs (migrations, built-in seed 050/051/061)"
kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=800 2>/dev/null | grep -E "Migration 050|Migration 051|Migration 061|built-in|Seed capability|Seed promotion" || log_warn "No migration/seed lines in last 800 lines"
echo ""

# --- 3. API: capability-metadata (Capability Catalog) - no auth for simple check ---
log_info "3. API GET /api/v1/capability-metadata (Capability Catalog)"
CAP_JSON=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -s "http://localhost:8080/api/v1/capability-metadata" 2>/dev/null || echo "{}")
CAP_COUNT=$(echo "$CAP_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('metadata',[])))" 2>/dev/null || echo "0")
if [ "${CAP_COUNT:-0}" -gt 0 ]; then
  log_ok "Capability Catalog has $CAP_COUNT entries (built-in seed ran)"
else
  log_warn "Capability Catalog count: $CAP_COUNT (expected >0 after built-in seed)"
fi
echo ""

# --- 4. API: health/dashboard-data-integrity ---
log_info "4. API GET /api/v1/health/dashboard-data-integrity"
INTEGRITY=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -s "http://localhost:8080/api/v1/health/dashboard-data-integrity" 2>/dev/null || echo "{}")
if echo "$INTEGRITY" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  log_ok "data-integrity returns valid JSON"
  echo "$INTEGRITY" | python3 -c "
import sys,json
d=json.load(sys.stdin)
cc=d.get('crossChecks',{})
print('  PodsCount:', cc.get('podsCount'), '| InsightsCount:', cc.get('insightsCount'), '| ActiveAgentsCount:', cc.get('activeAgentsCount'), '| CVEsCount:', cc.get('cvesCount'))
" 2>/dev/null || true
else
  log_warn "data-integrity non-JSON or error"
fi
echo ""

# --- 5. E2E pod-delete cleanup ---
log_info "5. E2E: pod-delete cleanup verify"
if [ -x "$PROJECT_ROOT/scripts/e2e/e2e-pod-delete-cleanup-verify.sh" ]; then
  NAMESPACE="$NAMESPACE" bash "$PROJECT_ROOT/scripts/e2e/e2e-pod-delete-cleanup-verify.sh" 2>&1 || log_warn "E2E script had failures"
else
  log_warn "e2e-pod-delete-cleanup-verify.sh not found or not executable"
fi
echo ""

# --- Report ---
{
  echo "# Verify after deploy – $(date -Iseconds)"
  echo ""
  echo "## 1. Pods"
  echo '```'
  kubectl get pods -n "$NAMESPACE" 2>/dev/null || true
  echo '```'
  echo ""
  echo "## 2. Capability Catalog"
  echo "- capability_metadata entries: $CAP_COUNT"
  echo ""
  echo "## 3. Data integrity (crossChecks)"
  echo "$INTEGRITY" | python3 -m json.tool 2>/dev/null || echo "$INTEGRITY"
} > "$REPORT_FILE" 2>/dev/null

log_ok "Report written: $REPORT_FILE"
echo "=========================================="
echo "Verify done."
echo "=========================================="

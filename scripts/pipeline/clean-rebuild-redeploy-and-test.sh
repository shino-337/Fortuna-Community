#!/usr/bin/env bash
# ============================================================================
# Clean → Rebuild → Redeploy → Run all test cases → Monitor Agent & Core
# Delegates clean/rebuild/deploy to full-clean-database-rebuild-deploy.sh.
# Core runs DB migrations on startup; use --db or --db-reset if agent sync 500
# (e.g. clusters.kubeconfig missing — migration 062). See that script for --db-reset.
# Usage:
#   ./scripts/pipeline/clean-rebuild-redeploy-and-test.sh              # full run
#   ./scripts/pipeline/clean-rebuild-redeploy-and-test.sh --db        # + DB clean (DELETE data)
#   ./scripts/pipeline/clean-rebuild-redeploy-and-test.sh --skip-rebuild --skip-deploy  # tests + monitor only
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRIPTS="$PROJECT_ROOT/scripts"
NAMESPACE="${NAMESPACE:-fortuna}"
REPORT_DIR="${PROJECT_ROOT}/docs/test-results"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT_FILE="${REPORT_DIR}/FULL-RUN-${TIMESTAMP}.md"

SKIP_CLEAN=false
SKIP_REBUILD=false
SKIP_DEPLOY=false
CLEAN_DB=false
for arg in "$@"; do
  case "$arg" in
    --skip-clean)   SKIP_CLEAN=true ;;
    --skip-rebuild) SKIP_REBUILD=true ;;
    --skip-deploy)  SKIP_DEPLOY=true ;;
    --db)           CLEAN_DB=true ;;
  esac
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
section() { echo -e "\n${BLUE}========== $1 ==========${NC}"; }
ok() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }

mkdir -p "$REPORT_DIR"
{
  echo "# Full run: clean, rebuild, redeploy, tests, monitor"
  echo "Started: $(date -Iseconds)"
  echo ""
} | tee -a "$REPORT_FILE" >/dev/null

cd "$PROJECT_ROOT"

# Phase 1: Clean + Rebuild + Redeploy
section "Phase 1: Clean / Rebuild / Redeploy"
EXTRA=""
[ "$CLEAN_DB" = true ] && EXTRA="--db"
[ "$SKIP_CLEAN" = true ] && EXTRA="$EXTRA --skip-clean"
[ "$SKIP_REBUILD" = true ] && EXTRA="$EXTRA --skip-rebuild"
[ "$SKIP_DEPLOY" = true ] && EXTRA="$EXTRA --skip-deploy"

if [ "$SKIP_CLEAN" = false ] || [ "$SKIP_REBUILD" = false ] || [ "$SKIP_DEPLOY" = false ]; then
  if ! "$SCRIPTS/pipeline/full-clean-database-rebuild-deploy.sh" $EXTRA 2>&1 | tee -a "$REPORT_FILE"; then
    fail "Clean/rebuild/redeploy failed"
    exit 1
  fi
  ok "Phase 1 done"
else
  ok "Phase 1 skipped (--skip-*)"
fi

# Wait for pods
section "Waiting for Core and Agent pods..."
kubectl wait --for=condition=ready pod -n "$NAMESPACE" -l app=fortuna-core --timeout=120s 2>/dev/null || warn "Core not ready in 120s"
kubectl wait --for=condition=ready pod -n "$NAMESPACE" -l app.kubernetes.io/component=agent --timeout=60s 2>/dev/null || warn "Some agents not ready in 60s"
sleep 5

# Phase 2: Monitor Agent & Core (snapshot)
section "Phase 2: Monitor Agent & Core (snapshot)"
"$SCRIPTS/monitor/monitor-agent-core.sh" --logs 25 2>&1 | tee -a "$REPORT_FILE"
ok "Phase 2 done"

# Phase 3: Test cases
section "Phase 3: Test cases"

run_test() {
  local name="$1"
  local cmd="$2"
  echo "" >> "$REPORT_FILE"
  echo "### $name" >> "$REPORT_FILE"
  echo '```' >> "$REPORT_FILE"
  if eval "$cmd" 2>&1 | tee -a "$REPORT_FILE"; then
    ok "$name"
    echo '```' >> "$REPORT_FILE"
    echo "Result: PASS" >> "$REPORT_FILE"
    return 0
  else
    fail "$name"
    echo '```' >> "$REPORT_FILE"
    echo "Result: FAIL" >> "$REPORT_FILE"
    return 1
  fi
}

PASS=0
FAIL=0

if run_test "check-full-deployment" "$SCRIPTS/verify/check-full-deployment.sh"; then ((PASS++)); else ((FAIL++)); fi
if run_test "test-priority1-apis" "$SCRIPTS/e2e/test-priority1-apis.sh"; then ((PASS++)); else ((FAIL++)); fi
if run_test "test-runtime-signals-e2e" "$SCRIPTS/e2e/test-runtime-signals-e2e.sh"; then ((PASS++)); else ((FAIL++)); fi
if run_test "verify-dashboard-api" "CORE_URL= bash -c 'CORE_POD=\$(kubectl get pods -n fortuna -l app=fortuna-core -o jsonpath={.items[0].metadata.name}); CORE_URL=http://localhost:8080; kubectl port-forward -n fortuna svc/fortuna-core 8080:8080 &'; sleep 3; $SCRIPTS/verify/verify-dashboard-api.sh; pkill -f 'port-forward.*fortuna-core' 2>/dev/null || true"; then ((PASS++)); else ((FAIL++)); fi

# Verify dashboard API via exec (no port-forward)
run_test "verify-dashboard-api (via exec)" "CORE_POD=\$(kubectl get pods -n fortuna -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}'); TOKEN=\$(kubectl -n fortuna exec \$CORE_POD -- curl -s -X POST http://localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"admin123\"}' | python3 -c 'import sys,json; print(json.load(sys.stdin).get(\"token\",\"\") or \"\")'); kubectl -n fortuna exec \$CORE_POD -- curl -s -H \"Authorization: Bearer \$TOKEN\" http://localhost:8080/api/v1/dashboard/stats | grep -q totalClusters && echo OK || echo FAIL" 2>&1 | tee -a "$REPORT_FILE"
[ $? -eq 0 ] && ((PASS++)) || ((FAIL++))

# Pod có risk critical trên clusterId hiện tại + kiểm tra API với clusterId active
if run_test "test-pod-critical-risk-cluster-id" "$SCRIPTS/e2e/test-pod-critical-risk-cluster-id.sh"; then ((PASS++)); else ((FAIL++)); fi

section "Test summary"
echo "Passed: $PASS | Failed: $FAIL" | tee -a "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "Finished: $(date -Iseconds)" >> "$REPORT_FILE"
echo "Report: $REPORT_FILE"

if [ "$FAIL" -gt 0 ]; then
  fail "Some tests failed ($FAIL)"
  exit 1
fi
ok "All tests passed"

#!/usr/bin/env bash
# =============================================================================
# Verify after clean/rebuild/redeploy: DB (migration 071, pods), Core, Agent,
# Pod Detail test suite, and real-data API checks.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
REPORT_DIR="${PROJECT_ROOT}/docs/test-results}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT="${REPORT_DIR}/verify-after-deploy-${TIMESTAMP}.md"

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
echo "# Verify after clean/rebuild/redeploy" > "$REPORT"
echo "**Time:** $(date -Iseconds)" >> "$REPORT"
echo "" >> "$REPORT"

# --- 1. Pods (Core, Agent, Dashboard, Postgres) ---
section "1. Pods (Core, Agent, Dashboard, Postgres)"
kubectl get pods -n "$NAMESPACE" -o wide 2>/dev/null | tee -a "$REPORT" || true
CORE_READY=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --no-headers 2>/dev/null | grep -c Running || echo "0")
AGENT_READY=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | grep -c Running || echo "0")
if [ "${CORE_READY:-0}" -lt 1 ]; then fail "Core not running"; else ok "Core: $CORE_READY running"; fi
if [ "${AGENT_READY:-0}" -lt 1 ]; then warn "Agent: $AGENT_READY running"; else ok "Agent: $AGENT_READY running"; fi
echo "" >> "$REPORT"
echo "Core running: $CORE_READY | Agent running: $AGENT_READY" >> "$REPORT"
echo "" >> "$REPORT"

# --- 2. Database: tables (migration 071), counts ---
section "2. Database (Postgres)"
PG_POD=$(kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$PG_POD" ]; then
  warn "Postgres pod not found"
  echo "Postgres: not found" >> "$REPORT"
else
  ok "Postgres pod: $PG_POD"
  echo "### Tables (including 071)" >> "$REPORT"
  kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -c "\dt" 2>/dev/null | tee -a "$REPORT" || true
  for table in pods pod_runtime_metrics pod_processes pod_network_connections k8s_events runtime_events runtime_signals pod_capabilities; do
    N=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "SELECT COUNT(*) FROM $table;" 2>/dev/null | tr -d ' \r' || echo "?")
    echo "  $table: $N" >> "$REPORT"
    [ "$table" = "pods" ] && POD_COUNT="$N"
  done
  echo "" >> "$REPORT"
  echo "Sample pods (id, name, uid, spec_hash):" >> "$REPORT"
  kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -c "SELECT id, name, uid, LEFT(spec_hash,12) FROM pods WHERE deleted_at IS NULL ORDER BY updated_at DESC LIMIT 5;" 2>/dev/null >> "$REPORT" || true
fi
echo "" >> "$REPORT"

# --- 3. Core process / logs ---
section "3. Core (process / logs)"
CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$CORE_POD" ]; then
  echo "### Core last 30 lines" >> "$REPORT"
  kubectl -n "$NAMESPACE" logs "$CORE_POD" --tail=30 2>/dev/null >> "$REPORT" || true
  if kubectl -n "$NAMESPACE" logs "$CORE_POD" --tail=100 2>/dev/null | grep -qi "migration 071\|listening\|started"; then
    ok "Core started and migrations (071) present or listening"
  else
    warn "Check Core logs for migration 071 / listening"
  fi
else
  warn "Core pod not found"
fi
echo "" >> "$REPORT"

# --- 4. Agent process / logs ---
section "4. Agent (process / logs)"
AGENT_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$AGENT_POD" ]; then
  echo "### Agent last 25 lines" >> "$REPORT"
  kubectl -n "$NAMESPACE" logs "$AGENT_POD" --tail=25 2>/dev/null >> "$REPORT" || true
  if kubectl -n "$NAMESPACE" logs "$AGENT_POD" --tail=200 2>/dev/null | grep -q "Sync completed\|sync completed\|Processing.*pods"; then
    ok "Agent sync activity seen"
  else
    warn "Agent sync not yet seen (wait 1–2 min)"
  fi
else
  warn "Agent pod not found"
fi
echo "" >> "$REPORT"

# --- 5. Pod Detail test suite (unit + DB) ---
section "5. Pod Detail test suite (unit + DB)"
if [ -x "$PROJECT_ROOT/scripts/e2e/run-pod-detail-test-suite.sh" ]; then
  if "$PROJECT_ROOT/scripts/e2e/run-pod-detail-test-suite.sh" 2>&1 | tee -a "$REPORT"; then
    ok "Pod detail test suite passed"
  else
    fail "Pod detail test suite had failures"
  fi
else
  (cd "$PROJECT_ROOT/core" && go test -v -count=1 ./internal/api/... -run 'TestGetPod|TestPostPod|PodRuntimeMetrics|PodProcesses|PodNetwork|K8sEvents' 2>&1) | tee -a "$REPORT"
  ok "Unit tests run (see report)"
fi
echo "" >> "$REPORT"

# --- 6. Real-data API: login, pods, pod detail, new endpoints ---
section "6. Real-data API (Core)"
if [ -n "$CORE_POD" ]; then
  TOKEN=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('token','') or '')" 2>/dev/null || echo "")
  if [ -n "$TOKEN" ]; then
    ok "Login OK"
    echo "### API checks" >> "$REPORT"
    # GET pods (domain: inventory/pods)
    PODS_JSON=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/inventory/pods?limit=5" 2>/dev/null || echo "{}")
    POD_UID=$(echo "$PODS_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); p=d.get('pods',[]); print(p[0].get('uid', p[0].get('id',''))) if p else print('')" 2>/dev/null || echo "")
    if [ -n "$POD_UID" ]; then
      ok "GET /inventory/pods: got pod uid=$POD_UID"
      echo "- GET /inventory/pods: pod uid=$POD_UID" >> "$REPORT"
      # GET pod detail (inventory) and runtime sub-resources
      kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/inventory/pods/$POD_UID" 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print('podIP:', d.get('podIP'), '| startTime:', d.get('startTime'), '| riskCount:', d.get('riskCount'))" 2>/dev/null >> "$REPORT" || true
      for path in "metrics" "processes" "network" "events"; do
        CODE=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/runtime/pods/$POD_UID/$path" 2>/dev/null || echo "000")
        if [ "$CODE" = "200" ]; then ok "GET /runtime/pods/$POD_UID/$path -> 200"; echo "- GET /runtime/pods/$POD_UID/$path: $CODE" >> "$REPORT"; else echo "- GET /runtime/pods/$POD_UID/$path: $CODE" >> "$REPORT"; warn "GET /runtime/pods/$POD_UID/$path -> $CODE"; fi
      done
    else
      warn "No pods in API (agent sync may not have run yet)"
      echo "- GET /inventory/pods: no pods" >> "$REPORT"
    fi
  else
    warn "Login failed (check Core auth)"
    echo "Login: failed" >> "$REPORT"
  fi
else
  warn "Core pod not found; skip API checks"
fi
echo "" >> "$REPORT"

section "Done"
echo "Report: $REPORT"
ok "Report written to ${REPORT_DIR}/verify-after-deploy-${TIMESTAMP}.md"

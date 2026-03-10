#!/usr/bin/env bash
# =============================================================================
# Verify deployment via real APIs only (no unit tests).
# Uses: DB check, Core/Agent pods & logs, then full API smoke test with JWT.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
REPORT_DIR="${PROJECT_ROOT}/docs/test-results}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
REPORT="${REPORT_DIR}/verify-via-api-${TIMESTAMP}.md"

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
echo "# Verify via API only (no unit tests)" > "$REPORT"
echo "**Time:** $(date -Iseconds)" >> "$REPORT"
echo "" >> "$REPORT"

# --- 1. Pods ---
section "1. Pods (Core, Agent, Dashboard, Postgres)"
kubectl get pods -n "$NAMESPACE" -o wide 2>/dev/null | tee -a "$REPORT" || true
CORE_READY=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core --no-headers 2>/dev/null | grep -c Running || echo "0")
AGENT_READY=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --no-headers 2>/dev/null | grep -c Running || echo "0")
if [ "${CORE_READY:-0}" -lt 1 ]; then fail "Core not running"; else ok "Core: $CORE_READY running"; fi
if [ "${AGENT_READY:-0}" -lt 1 ]; then warn "Agent: $AGENT_READY running"; else ok "Agent: $AGENT_READY running"; fi
echo "" >> "$REPORT"

# --- 2. Database ---
section "2. Database"
PG_POD=$(kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$PG_POD" ]; then
  warn "Postgres pod not found"
else
  ok "Postgres: $PG_POD"
  echo "### Table counts" >> "$REPORT"
  for table in pods agents sboms sbom_components package_vulnerabilities pod_runtime_metrics pod_processes pod_network_connections k8s_events runtime_events runtime_signals pod_capabilities pod_risk_profiles; do
    N=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "SELECT COUNT(*) FROM $table;" 2>/dev/null | tr -d ' \r' || echo "?")
    printf "  %s: %s\n" "$table" "$N" >> "$REPORT"
  done
  echo "" >> "$REPORT"
fi

# --- 3. Core / Agent logs (short) ---
section "3. Core & Agent logs"
CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
AGENT_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$CORE_POD" ]; then
  echo "### Core last 15 lines" >> "$REPORT"
  kubectl -n "$NAMESPACE" logs "$CORE_POD" --tail=15 2>/dev/null >> "$REPORT" || true
  ok "Core pod: $CORE_POD"
fi
if [ -n "$AGENT_POD" ]; then
  echo "### Agent last 10 lines" >> "$REPORT"
  kubectl -n "$NAMESPACE" logs "$AGENT_POD" --tail=10 2>/dev/null >> "$REPORT" || true
  ok "Agent pod: $AGENT_POD"
fi
echo "" >> "$REPORT"

# --- 4. API verification (all via real HTTP) ---
section "4. API verification (real endpoints)"
if [ -z "$CORE_POD" ]; then
  fail "Core pod not found; skip API checks"
  echo "API: skipped (no Core)" >> "$REPORT"
else
  BASE="http://localhost:8080"
  TOKEN=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST "$BASE/api/v1/auth/login" -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('token','') or '')" 2>/dev/null || echo "")
  if [ -z "$TOKEN" ]; then
    fail "Login failed"
    echo "API: Login failed" >> "$REPORT"
  else
    ok "Login OK"
    echo "### API results" >> "$REPORT"

    api_get() {
      local path="$1"
      local code
      code=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "$BASE$path" 2>/dev/null || echo "000")
      if [ "$code" = "200" ]; then ok "GET $path -> 200"; echo "- GET $path: $code" >> "$REPORT"; return 0; else echo "- GET $path: $code" >> "$REPORT"; warn "GET $path -> $code"; return 1; fi
    }

    FAILS=0
    api_get "/api/v1/me" || ((FAILS+=1))
    api_get "/api/v1/clusters" || ((FAILS+=1))
    api_get "/api/v1/clusters/stats" || ((FAILS+=1))
    api_get "/api/v1/pods?limit=10" || ((FAILS+=1))

    # First pod id for detail endpoints
    PODS_JSON=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/v1/pods?limit=5" 2>/dev/null || echo "{}")
    POD_ID=$(echo "$PODS_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); p=d.get('pods',[]); print(str(p[0].get('id',''))) if p else print('')" 2>/dev/null || echo "")
    POD_UID=$(echo "$PODS_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); p=d.get('pods',[]); print(p[0].get('uid','') or '') if p else print('')" 2>/dev/null || echo "")

    if [ -n "$POD_ID" ]; then
      api_get "/api/v1/pods/$POD_ID" || ((FAILS+=1))
      api_get "/api/v1/pods/$POD_ID/capabilities" || ((FAILS+=1))
      api_get "/api/v1/pods/$POD_ID/runtime-metrics" || ((FAILS+=1))
      api_get "/api/v1/pods/$POD_ID/processes" || ((FAILS+=1))
      api_get "/api/v1/pods/$POD_ID/network-connections" || ((FAILS+=1))
      api_get "/api/v1/pods/$POD_ID/events" || ((FAILS+=1))
      if [ -n "$POD_UID" ]; then
        api_get "/api/v1/pods/by-uid/$POD_UID" || ((FAILS+=1))
        api_get "/api/v1/pods/by-uid/$POD_UID/runtime-metrics" || ((FAILS+=1))
        api_get "/api/v1/pods/by-uid/$POD_UID/processes" || ((FAILS+=1))
        api_get "/api/v1/pods/by-uid/$POD_UID/network-connections" || ((FAILS+=1))
        api_get "/api/v1/pods/by-uid/$POD_UID/events" || ((FAILS+=1))
      fi
    else
      warn "No pods in API; skipping pod-detail endpoints"
      echo "- No pod id; pod-detail endpoints skipped" >> "$REPORT"
    fi

    api_get "/api/v1/dashboard/stats" || ((FAILS+=1))
    api_get "/api/v1/insights?limit=5" || ((FAILS+=1))
    api_get "/api/v1/sbom?limit=5" || ((FAILS+=1))
    api_get "/api/v1/health/dashboard-data-integrity" || ((FAILS+=1))
    api_get "/api/v1/agents/status" || api_get "/api/v1/monitoring/agents" || ((FAILS+=1))
    api_get "/api/v1/runtime-signals?limit=5" || ((FAILS+=1))
    api_get "/api/v1/pod-capabilities/summary" || ((FAILS+=1))
    api_get "/api/v1/risks?limit=5" || ((FAILS+=1))
    api_get "/api/v1/deployments?limit=5" || ((FAILS+=1))
    api_get "/api/v1/resources" || ((FAILS+=1))

    if [ "${FAILS:-0}" -gt 0 ]; then
      fail "API failures: $FAILS endpoints non-200"
    else
      ok "All API checks returned 200"
    fi
    echo "" >> "$REPORT"
  fi
fi

section "Done"
echo "Report: $REPORT"
ok "Report: $REPORT"

#!/usr/bin/env bash
# ============================================================================
# Verify Agent Availability: compare Dashboard (API/DB) with reality (pods)
# ============================================================================
# 1. Reality: fortuna-agent pod count (Running)
# 2. DB: agents table (all, and within ACTIVE_AGENT_CUTOFF = 15 min)
# 3. API: GET /api/v1/agents/status and /api/v1/dashboard/stats (if port-forward + JWT)
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
CORE_PORT="${CORE_PORT:-8080}"
CUTOFF_MIN="${ACTIVE_AGENT_CUTOFF_MINUTES:-15}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Agent Availability: Dashboard vs Reality"
echo "=========================================="
echo ""

# ---- 1. Reality: agent pods ----
echo -e "${BLUE}[1] Reality – Agent pods (Running)${NC}"
POD_COUNT=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent --field-selector=status.phase=Running -o name 2>/dev/null | wc -l)
echo "  Pod count (Running): $POD_COUNT"
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o wide 2>/dev/null | head -10
echo ""

# ---- 2. DB: agents table ----
echo -e "${BLUE}[2] DB – agents table${NC}"
PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
ACTIVE_COUNT=""
if [ -z "$PG_POD" ]; then
  echo "  Postgres pod not found; skip DB check."
else
  echo "  All agents (deleted_at IS NULL):"
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c \
    "SELECT agent_id, node_name, status, last_seen_at, now() - last_seen_at AS age FROM agents WHERE deleted_at IS NULL ORDER BY last_seen_at DESC;" 2>/dev/null || true
  echo ""
  echo "  Count of all ready agents (what API/Dashboard uses – no cutoff):"
  ACTIVE_COUNT=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c \
    "SELECT count(*) FROM agents WHERE deleted_at IS NULL AND (status = 'ready' OR status IS NULL);" 2>/dev/null | tr -d ' ' || echo "0")
  echo "  activeAgents (DB): $ACTIVE_COUNT"
fi
echo ""

# ---- 3. API (optional: need port-forward + JWT) ----
echo -e "${BLUE}[3] API – Core (localhost:${CORE_PORT})${NC}"
STATS_JSON=""
AGENTS_JSON=""
if command -v curl >/dev/null 2>&1; then
  if [ -n "${JWT:-}" ]; then
    STATS_JSON=$(curl -s -H "Authorization: Bearer $JWT" "http://127.0.0.1:${CORE_PORT}/api/v1/dashboard/stats" 2>/dev/null || true)
    AGENTS_JSON=$(curl -s -H "Authorization: Bearer $JWT" "http://127.0.0.1:${CORE_PORT}/api/v1/agents/status" 2>/dev/null || true)
  else
    STATS_JSON=$(curl -s "http://127.0.0.1:${CORE_PORT}/api/v1/dashboard/stats" 2>/dev/null || true)
    AGENTS_JSON=$(curl -s "http://127.0.0.1:${CORE_PORT}/api/v1/agents/status" 2>/dev/null || true)
  fi
fi

if echo "$STATS_JSON" | grep -q "activeAgents\|ActiveAgents"; then
  API_AGENTS=$(echo "$STATS_JSON" | sed -n 's/.*"activeAgents"[^:]*: *\([0-9]*\).*/\1/p' | head -1)
  [ -z "$API_AGENTS" ] && API_AGENTS=$(echo "$STATS_JSON" | sed -n 's/.*"ActiveAgents"[^:]*: *\([0-9]*\).*/\1/p' | head -1)
  echo "  GET /api/v1/dashboard/stats → activeAgents: $API_AGENTS"
else
  if echo "$STATS_JSON" | grep -q "401\|error\|Authorization"; then
    echo "  GET /api/v1/dashboard/stats → 401 or error (set JWT env and port-forward Core to ${CORE_PORT})"
  else
    echo "  GET /api/v1/dashboard/stats → no response (port-forward? kubectl port-forward -n $NAMESPACE svc/fortuna-core ${CORE_PORT}:8080)"
  fi
fi

if echo "$AGENTS_JSON" | grep -q '"agents"'; then
  API_TOTAL=$(echo "$AGENTS_JSON" | sed -n 's/.*"total"[^:]*: *\([0-9]*\).*/\1/p' | head -1)
  echo "  GET /api/v1/agents/status  → total: $API_TOTAL"
else
  if echo "$AGENTS_JSON" | grep -q "401\|error\|Authorization"; then
    echo "  GET /api/v1/agents/status  → 401 or error (set JWT env)"
  else
    echo "  GET /api/v1/agents/status  → no response"
  fi
fi
echo ""

# ---- Summary ----
echo "=========================================="
echo -e "${GREEN}Summary${NC}"
echo "=========================================="
echo "  Reality (Running pods): $POD_COUNT"
echo "  DB agents (all ready): ${ACTIVE_COUNT:-N/A}"
echo ""
if [ "${POD_COUNT:-0}" -gt 0 ] && [ "${ACTIVE_COUNT:-0}" -eq 0 ]; then
  echo -e "${YELLOW}Mismatch: pods are Running but no agents in DB. RegisterAgent or Ping (with agent_id) should create/update rows.${NC}"
  echo "  → Ensure Agent sends Ping with agent_id; Core updates agents.last_seen_at (raw Exec)."
  echo "  → Rebuild Agent + Core image and restart so pods use new code."
fi
if [ "${POD_COUNT:-0}" -eq "${ACTIVE_COUNT:-0}" ] && [ "${POD_COUNT:-0}" -gt 0 ]; then
  echo -e "${GREEN}Match: Dashboard agent count matches running pods.${NC}"
fi
echo ""

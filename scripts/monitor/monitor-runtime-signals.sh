#!/usr/bin/env bash
# =============================================================================
# Monitor Runtime Signals – Agent (reader), Core (ingest/REP), DB (counts), API
# Usage:
#   ./scripts/monitor/monitor-runtime-signals.sh           # one-shot
#   ./scripts/monitor/monitor-runtime-signals.sh --follow  # tail agent + core logs
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
FOLLOW=false
LINES="${LINES:-30}"

for arg in "$@"; do
  case "$arg" in
    --follow|-f) FOLLOW=true ;;
    --lines=*)   LINES="${arg#--lines=}" ;;
  esac
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
section() { echo -e "\n${BLUE}========== $1 ==========${NC}"; }
ok() { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

cd "$PROJECT_ROOT"

CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
[ -z "$CORE_POD" ] && CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
AGENT_PODS=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=agent --no-headers -o custom-columns=":metadata.name" 2>/dev/null || true)
PG_POD=$(kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
[ -z "$PG_POD" ] && PG_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/name=postgresql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

section "Runtime Signals – One-shot snapshot"
echo "Core: ${CORE_POD:-none} | Agents: $(echo "$AGENT_PODS" | wc -w) | Postgres: ${PG_POD:-none}"
echo ""

# --- DB counts ---
section "DB: runtime_events / runtime_signals"
if [ -n "$PG_POD" ]; then
  EVENTS=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "SELECT COUNT(*) FROM runtime_events;" 2>/dev/null || echo "?")
  SIGNALS=$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "SELECT COUNT(*) FROM runtime_signals;" 2>/dev/null || echo "?")
  echo "runtime_events: $EVENTS"
  echo "runtime_signals: $SIGNALS"
  if [ "${SIGNALS:-0}" -gt 0 ] 2>/dev/null; then
    echo "Latest 3 signals:"
    kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -c "
      SELECT id, pod_uid, signal_type, category, confidence, created_at
      FROM runtime_signals ORDER BY created_at DESC LIMIT 3;
    " 2>/dev/null || true
  fi
else
  warn "Postgres pod not found"
fi
echo ""

# --- API quick check ---
section "API: GET /runtime/signals?limit=3"
if [ -n "$CORE_POD" ]; then
  TOKEN=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")
  if [ -n "$TOKEN" ]; then
    RESP=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/runtime/signals?limit=3" 2>/dev/null || echo "{}")
    if echo "$RESP" | grep -q '"signals"'; then
      TOTAL=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total', 0))" 2>/dev/null || echo "?")
      ok "API total signals: $TOTAL"
    else
      warn "API response: $(echo "$RESP" | head -c 80)"
    fi
  else
    warn "Login failed; API check skipped"
  fi
else
  warn "Core pod not found"
fi
echo ""

# --- Agent logs (runtime events reader) ---
section "Agent logs (Runtime Events reader) – last ${LINES} lines"
for AGENT in $AGENT_PODS; do
  echo "--- $AGENT ---"
  kubectl -n "$NAMESPACE" logs "$AGENT" --tail="$LINES" 2>/dev/null | grep -E "Runtime|runtime|runtime-events|runtime_events" || echo "(no runtime events lines)"
done
echo ""

# --- Core logs (RuntimeEvent / REP) ---
section "Core logs (RuntimeEvent / REP) – last ${LINES} lines"
if [ -n "$CORE_POD" ]; then
  kubectl -n "$NAMESPACE" logs "$CORE_POD" --tail="$LINES" 2>/dev/null | grep -E "RuntimeEvent|REP|runtime.events|runtime_signals" || echo "(no RuntimeEvent lines)"
fi
echo ""

if [ "$FOLLOW" = true ]; then
  section "Follow mode: Agent + Core (Ctrl+C to stop)"
  kubectl -n "$NAMESPACE" logs -f -l app.kubernetes.io/component=agent --all-containers=true --max-log-requests=4 2>/dev/null &
  PID_AGENT=$!
  if [ -n "$CORE_POD" ]; then
    kubectl -n "$NAMESPACE" logs -f "$CORE_POD" 2>/dev/null &
    PID_CORE=$!
  else
    PID_CORE=""
  fi
  trap "kill $PID_AGENT $PID_CORE 2>/dev/null; exit 0" INT TERM
  wait
fi

echo ""
echo "Done. Use --follow to tail agent and core logs."

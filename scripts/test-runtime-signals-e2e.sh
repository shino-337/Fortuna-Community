#!/usr/bin/env bash
# =============================================================================
# Runtime Signals E2E – test cases thực tế
# 1. POST /api/v1/runtime-events (ingest event)
# 2. Kiểm tra DB: runtime_events, runtime_signals
# 3. GET /api/v1/runtime-signals
# 4. GET /api/v1/runtime-signals/pods/:podUid
# Chạy từ repo root; cần kubectl, namespace fortuna, Core + Postgres chạy.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
ok()  { echo -e "${GREEN}[OK]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

cd "$PROJECT_ROOT"

CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$CORE_POD" ]; then
  CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
fi
if [ -z "$CORE_POD" ]; then
  fail "Core pod not found in namespace $NAMESPACE"
  exit 1
fi

PG_POD=$(kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$PG_POD" ]; then
  PG_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/name=postgresql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
fi
if [ -z "$PG_POD" ]; then
  warn "Postgres pod not found; DB checks will be skipped"
fi

E2E_USER="${FORTUNA_E2E_USER:-admin}"
E2E_PASS="${FORTUNA_E2E_PASSWORD:-admin123}"
LOGIN_RESP=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$E2E_USER\",\"password\":\"$E2E_PASS\"}" 2>/dev/null || echo "{}")
TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null || echo "")
if [ -z "$TOKEN" ]; then
  warn "Login failed (user=$E2E_USER). POST/GET may return 401."
  AUTH_HEADER=""
else
  ok "JWT obtained for $E2E_USER"
  AUTH_HEADER="Authorization: Bearer $TOKEN"
fi

api_get() {
  local url="$1"
  if [ -n "$AUTH_HEADER" ]; then
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -H "$AUTH_HEADER" "$url" 2>/dev/null || echo "ERROR"
  else
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s "$url" 2>/dev/null || echo "ERROR"
  fi
}

api_post() {
  local url="$1"
  local body="$2"
  if [ -n "$AUTH_HEADER" ]; then
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST -H "Content-Type: application/json" -H "$AUTH_HEADER" -d "$body" "$url" 2>/dev/null || echo "ERROR"
  else
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST -H "Content-Type: application/json" -d "$body" "$url" 2>/dev/null || echo "ERROR"
  fi
}

db_query() {
  [ -z "$PG_POD" ] && return 1
  kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c "$1" 2>/dev/null || return 1
}

echo ""
echo "========== Runtime Signals E2E =========="
echo "Core pod: $CORE_POD | PG pod: ${PG_POD:-none}"
echo ""

# --- Test 1: POST /api/v1/runtime-events ---
echo "--- Test 1: POST /api/v1/runtime-events ---"
TEST_POD_UID="e2e-test-pod-$(date +%s)"
TS=$(date +%s)
PAYLOAD="[{\"event_type\":\"escape_attempt\",\"mitre_technique\":\"T1611.001\",\"signal\":\"PROC_ROOT_PIVOT\",\"severity\":\"high\",\"pod\":{\"name\":\"e2e-pod\",\"namespace\":\"default\",\"uid\":\"$TEST_POD_UID\"},\"syscall\":\"openat\",\"target\":\"/proc/1/root\",\"timestamp\":$TS}]"
RESP=$(api_post "http://localhost:8080/api/v1/runtime-events" "$PAYLOAD")
if echo "$RESP" | grep -q "processed"; then
  PROCESSED=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('processed', 0))" 2>/dev/null || echo "0")
  ok "POST /runtime-events returned processed=$PROCESSED"
else
  fail "POST /runtime-events failed: $RESP"
fi
echo ""

# --- Test 2: DB runtime_events / runtime_signals ---
echo "--- Test 2: DB runtime_events / runtime_signals ---"
if [ -n "$PG_POD" ]; then
  EVENTS_CNT=$(db_query "SELECT COUNT(*) FROM runtime_events;" 2>/dev/null || echo "0")
  SIGNALS_CNT=$(db_query "SELECT COUNT(*) FROM runtime_signals;" 2>/dev/null || echo "0")
  ok "runtime_events count: $EVENTS_CNT"
  ok "runtime_signals count: $SIGNALS_CNT"
  if [ "${SIGNALS_CNT:-0}" -gt 0 ]; then
    db_query "SELECT id, pod_uid, signal_type, category, confidence, created_at FROM runtime_signals ORDER BY created_at DESC LIMIT 3;" 2>/dev/null | while read -r line; do echo "  $line"; done
  fi
else
  warn "Skipped (no Postgres pod)"
fi
echo ""

# --- Test 3: GET /api/v1/runtime-signals ---
echo "--- Test 3: GET /api/v1/runtime-signals?limit=5 ---"
RESP=$(api_get "http://localhost:8080/api/v1/runtime-signals?limit=5")
if echo "$RESP" | grep -q '"signals"'; then
  TOTAL=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total', 0))" 2>/dev/null || echo "0")
  ok "GET /runtime-signals total=$TOTAL"
else
  fail "GET /runtime-signals failed: $(echo "$RESP" | head -c 120)"
fi
echo ""

# --- Test 4: GET /api/v1/runtime-signals/pods/:podUid ---
echo "--- Test 4: GET /api/v1/runtime-signals/pods/:podUid ---"
POD_UID_FROM_API=$(echo "$RESP" | python3 -c "
import sys, json
try:
  d = json.load(sys.stdin)
  sigs = d.get('signals') or []
  if sigs:
    print(sigs[0].get('podUid', ''))
except: pass
" 2>/dev/null || echo "")
if [ -n "$POD_UID_FROM_API" ]; then
  RESP2=$(api_get "http://localhost:8080/api/v1/runtime-signals/pods/$POD_UID_FROM_API")
  if echo "$RESP2" | grep -q '"signals"'; then
    CNT=$(echo "$RESP2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('signals', [])))" 2>/dev/null || echo "0")
    ok "GET /runtime-signals/pods/$POD_UID_FROM_API signals count: $CNT"
  else
    fail "GET /runtime-signals/pods/:uid failed"
  fi
else
  warn "No pod_uid from API; skipped Test 4 (try Test 1 first)"
fi
echo ""

# --- Test 5: GET by test pod UID (we just posted) ---
echo "--- Test 5: GET /runtime-signals/pods/$TEST_POD_UID ---"
RESP3=$(api_get "http://localhost:8080/api/v1/runtime-signals/pods/$TEST_POD_UID")
if echo "$RESP3" | grep -q '"signals"'; then
  CNT3=$(echo "$RESP3" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('signals', [])))" 2>/dev/null || echo "0")
  ok "Signals for test pod: $CNT3"
else
  warn "No signals yet for test pod (dedup or delay): $(echo "$RESP3" | head -c 80)"
fi
echo ""

echo "========== Runtime Signals E2E done =========="

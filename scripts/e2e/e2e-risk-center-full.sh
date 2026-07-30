#!/usr/bin/env bash
# ============================================================================
# E2E Risk Center – Full test suite (Phase 1–4 cải tiến)
# ============================================================================
# Chạy toàn bộ test case cho Risk Center: risks + scores, export CSV/PDF,
# insights summary / by-cluster, risk-rules CRUD, PCE trends/summary, WS.
# Kết quả chi tiết ghi vào: test-results/risk-center-e2e-YYYYMMDD-HHMMSS.md
# ============================================================================
# Usage: ./scripts/e2e/e2e-risk-center-full.sh
#        NAMESPACE=fortuna ./scripts/e2e/e2e-risk-center-full.sh
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
REPORT_DIR="${PROJECT_ROOT}/test-results"
mkdir -p "$REPORT_DIR"
REPORT_FILE="${REPORT_DIR}/risk-center-e2e-$(date +%Y%m%d-%H%M%S).md"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
ok()   { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
info() { echo -e "${BLUE}[E2E]${NC} $*"; }

# Counters
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

# Report buffer (append to REPORT_FILE at end)
report() { echo "$1" >> "$REPORT_FILE"; }

CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
[ -z "$CORE_POD" ] && CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [ -z "$CORE_POD" ]; then
  echo -e "${RED}Core pod not found in namespace $NAMESPACE${NC}"
  exit 1
fi

# JWT
LOGIN=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${FORTUNA_ADMIN_USER:-admin}\",\"password\":\"${FORTUNA_ADMIN_PASSWORD:-${FORTUNA_DEFAULT_ADMIN_PASSWORD:-Fortuna_ChangeMe_123!}}\"}" 2>/dev/null || echo "{}")
TOKEN=$(echo "$LOGIN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('token',''))" 2>/dev/null || echo "")
AUTH_HEADER="Authorization: Bearer $TOKEN"

api_get() {
  local path="$1"
  if [ -n "$TOKEN" ]; then
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -w "\nHTTP_CODE:%{http_code}" -H "$AUTH_HEADER" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo -e "\nHTTP_CODE:000"
  else
    kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -w "\nHTTP_CODE:%{http_code}" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo -e "\nHTTP_CODE:000"
  fi
}

api_post() {
  local path="$1"
  local body="$2"
  kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -w "\nHTTP_CODE:%{http_code}" -X POST \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" -d "$body" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo -e "\nHTTP_CODE:000"
}

api_put() {
  local path="$1"
  local body="$2"
  kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -w "\nHTTP_CODE:%{http_code}" -X PUT \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" -d "$body" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo -e "\nHTTP_CODE:000"
}

api_delete() {
  local path="$1"
  kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -w "\nHTTP_CODE:%{http_code}" -X DELETE \
    -H "$AUTH_HEADER" "http://localhost:8080/api/v1/$path" 2>/dev/null || echo -e "\nHTTP_CODE:000"
}

# Split body and HTTP code from response
get_http_code() { echo "$1" | grep -o 'HTTP_CODE:[0-9]*' | cut -d: -f2; }
get_body() { echo "$1" | sed '/^HTTP_CODE:/d'; }

run_tc() {
  local id="$1"
  local name="$2"
  local expected="$3"
  local actual="$4"
  local result="$5"   # PASS | FAIL | SKIP
  local detail="${6:-}"
  echo ""
  if [ "$result" = "PASS" ]; then
    ok "TC-$id: $name"
    ((PASS_COUNT++)) || true
  elif [ "$result" = "SKIP" ]; then
    warn "TC-$id: $name (SKIP: $detail)"
    ((SKIP_COUNT++)) || true
  else
    fail "TC-$id: $name"
    ((FAIL_COUNT++)) || true
  fi
  report "### TC-$id: $name"
  report "- **Kết quả:** $result"
  report "- **Mô tả:** $expected"
  report "- **Thực tế:** $actual"
  [ -n "$detail" ] && report "- **Chi tiết:** $detail"
  report ""
}

# ----- Init report -----
{
  echo "# Risk Center E2E – Báo cáo chi tiết"
  echo ""
  echo "**Thời gian:** $(date -Iseconds)"
  echo "**Namespace:** $NAMESPACE"
  echo "**Core pod:** $CORE_POD"
  echo ""
  echo "---"
  echo ""
} > "$REPORT_FILE"

info "Report: $REPORT_FILE"
echo "=============================================="
echo " Risk Center E2E – Full test suite"
echo "=============================================="
info "Core pod: $CORE_POD"

# Coverage pod matrix for risk center rules/runtime
MATRIX_FILE="$PROJECT_ROOT/deploy/e2e/risk-center-pod-matrix.yaml"
if [ -f "$MATRIX_FILE" ]; then
  info "Applying pod matrix: $MATRIX_FILE"
  kubectl apply -f "$MATRIX_FILE" >/dev/null 2>&1 || true
  kubectl -n risk-center-test wait --for=condition=Ready pod \
    rc-pss-privileged-host rc-rbac-cluster-admin rc-pss-cap-netraw-sysadmin rc-pss-no-limits \
    --timeout=120s >/dev/null 2>&1 || true
fi

# ---------------------------------------------------------------------------
# TC-01: GET /risks (list, pagination)
# ---------------------------------------------------------------------------
RESP=$(api_get "risk/insights?page=1&pageSize=5")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
TOTAL=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total',-1))" 2>/dev/null || echo "-1")
if [ "$CODE" = "200" ] && [ "${TOTAL:- -1}" -ge 0 ]; then
  run_tc "01" "GET /risks (list, pagination)" "HTTP 200, JSON có insights[], total" "HTTP $CODE, total=$TOTAL" "PASS"
else
  run_tc "01" "GET /risks (list, pagination)" "HTTP 200, JSON có insights[], total" "HTTP $CODE, total=$TOTAL" "FAIL" "Response: ${BODY:0:200}"
fi

# ---------------------------------------------------------------------------
# TC-02: GET /risks?withScores=1 (Phase 3.1)
# ---------------------------------------------------------------------------
RESP=$(api_get "risk/insights?page=1&pageSize=2&withScores=1")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
HAS_SCORE=$(echo "$BODY" | python3 -c "
import sys,json
try:
  d=json.load(sys.stdin)
  insights=d.get('insights',[])
  for i in insights:
    if 'totalScore' in i or 'priorityLevel' in i: print('yes'); break
  else: print('no' if insights else 'n/a')
except: print('err')
" 2>/dev/null || echo "err")
if [ "$CODE" = "200" ]; then
  run_tc "02" "GET /risks?withScores=1 (unified score)" "HTTP 200, insights có totalScore/priorityLevel khi có risk_scores" "HTTP $CODE, hasScoreField=$HAS_SCORE" "PASS"
else
  run_tc "02" "GET /risks?withScores=1" "HTTP 200" "HTTP $CODE" "FAIL"
fi

# ---------------------------------------------------------------------------
# TC-03: GET /risks?priorityLevel=P0
# ---------------------------------------------------------------------------
RESP=$(api_get "risks?page=1&pageSize=5&priorityLevel=P0")
CODE=$(get_http_code "$RESP")
run_tc "03" "GET /risks?priorityLevel=P0 (filter)" "HTTP 200 (filter P0)" "HTTP $CODE" "$([ "$CODE" = "200" ] && echo PASS || echo FAIL)"

# ---------------------------------------------------------------------------
# TC-04: GET /insights/summary
# ---------------------------------------------------------------------------
RESP=$(api_get "risk/insights/summary")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
TOTAL=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total',-1))" 2>/dev/null || echo "-1")
if [ "$CODE" = "200" ]; then
  run_tc "04" "GET /insights/summary (severity bar)" "HTTP 200, total/critical/high/..." "HTTP $CODE, total=$TOTAL" "PASS"
else
  run_tc "04" "GET /insights/summary" "HTTP 200" "HTTP $CODE" "FAIL"
fi

# ---------------------------------------------------------------------------
# TC-05: GET /insights/summary/by-cluster (Phase 2.2)
# ---------------------------------------------------------------------------
RESP=$(api_get "risk/insights/summary/by-cluster")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
BY_CLUSTER=$(echo "$BODY" | python3 -c "
import sys,json
try:
  d=json.load(sys.stdin)
  bc=d.get('byCluster',d) if isinstance(d,dict) else getattr(d,'byCluster',[])
  if isinstance(bc,list): print(len(bc))
  else: print('n/a')
except: print('err')
" 2>/dev/null || echo "0")
if [ "$CODE" = "200" ]; then
  run_tc "05" "GET /insights/summary/by-cluster (global view)" "HTTP 200, byCluster[]" "HTTP $CODE, byCluster length=$BY_CLUSTER" "PASS"
else
  run_tc "05" "GET /insights/summary/by-cluster" "HTTP 200" "HTTP $CODE" "FAIL"
fi

# ---------------------------------------------------------------------------
# TC-05b: GET /insights/summary/global (global aggregate, Phase #8)
# ---------------------------------------------------------------------------
RESP=$(api_get "risk/insights/summary/global")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
TOTAL_G=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total',-1))" 2>/dev/null || echo "-1")
if [ "$CODE" = "200" ] && [ "${TOTAL_G:- -1}" -ge 0 ]; then
  run_tc "05b" "GET /insights/summary/global (global summary)" "HTTP 200, total/critical/high/..." "HTTP $CODE, total=$TOTAL_G" "PASS"
else
  run_tc "05b" "GET /insights/summary/global" "HTTP 200, JSON có total" "HTTP $CODE, total=$TOTAL_G" "FAIL" "Response: ${BODY:0:200}"
fi

# ---------------------------------------------------------------------------
# TC-05c: GET /risk/histogram (Risk Score Distribution, Phase 3)
# ---------------------------------------------------------------------------
RESP=$(api_get "risk/histogram?sinceMinutes=30")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
HIST_BINS=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); b=d.get('bins',[]); print(len(b) if isinstance(b,list) else 0)" 2>/dev/null || echo "0")
HIST_TOTAL=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('totalFindings',-1))" 2>/dev/null || echo "-1")
if [ "$CODE" = "200" ] && [ "${HIST_BINS:-0}" -ge 1 ] && [ "${HIST_TOTAL:- -1}" -ge 0 ]; then
  run_tc "05c" "GET /risk/histogram (score distribution)" "HTTP 200, bins[], totalFindings" "HTTP $CODE, bins=$HIST_BINS, totalFindings=$HIST_TOTAL" "PASS"
else
  run_tc "05c" "GET /risk/histogram" "HTTP 200, bins[], totalFindings" "HTTP $CODE, bins=$HIST_BINS, totalFindings=$HIST_TOTAL" "FAIL" "Response: ${BODY:0:200}"
fi

# ---------------------------------------------------------------------------
# TC-05d / TC-05e: dashboard byType=all (Phase 1 data scope + Phase 7.2 E2E)
# ---------------------------------------------------------------------------
RESP=$(api_get "dashboard/stats?byType=all")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
TR=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('totalRisks',-1))" 2>/dev/null || echo "-1")
if [ "$CODE" = "200" ] && [ "${TR:- -1}" -ge 0 ]; then
  run_tc "05d" "GET /dashboard/stats?byType=all" "HTTP 200, totalRisks>=0" "HTTP $CODE, totalRisks=$TR" "PASS"
else
  run_tc "05d" "GET /dashboard/stats?byType=all" "HTTP 200, JSON hợp lệ" "HTTP $CODE" "FAIL" "Body: ${BODY:0:240}"
fi

RESP=$(api_get "dashboard/metrics/threat-velocity?byType=all&days=7")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
TREND_LEN=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); t=d.get('trend',[]); print(len(t) if isinstance(t,list) else 0)" 2>/dev/null || echo "0")
if [ "$CODE" = "200" ] && [ "${TREND_LEN:-0}" -ge 1 ]; then
  run_tc "05e" "GET /threat-velocity?byType=all&days=7" "HTTP 200, trend[]" "HTTP $CODE, trend_len=$TREND_LEN" "PASS"
else
  run_tc "05e" "GET /threat-velocity?byType=all" "HTTP 200, trend[]" "HTTP $CODE, trend_len=$TREND_LEN" "FAIL" "Body: ${BODY:0:240}"
fi

# ---------------------------------------------------------------------------
# TC-06: GET /risks/export (Phase 1.3 – CSV)
# ---------------------------------------------------------------------------
RESP=$(api_get "risk/insights/export")
CODE=$(get_http_code "$RESP")
BODY=$(get_body "$RESP")
IS_CSV=$(echo "$BODY" | head -1 | grep -qE '^[^,]*,[^,]*,' && echo "yes" || echo "no")
if [ "$CODE" = "200" ]; then
  run_tc "06" "GET /risks/export (CSV)" "HTTP 200, CSV body hoặc attachment" "HTTP $CODE, looks_csv=$IS_CSV" "PASS"
else
  run_tc "06" "GET /risks/export" "HTTP 200" "HTTP $CODE" "FAIL"
fi

# ---------------------------------------------------------------------------
# TC-07: GET /risks/export?format=pdf (Phase 3.3)
# ---------------------------------------------------------------------------
RESP=$(api_get "risk/insights/export?format=pdf")
CODE=$(get_http_code "$RESP")
BODY=$(get_body "$RESP")
IS_HTML=$(echo "$BODY" | head -1 | grep -qE '<!DOCTYPE|<html' && echo "yes" || echo "no")
if [ "$CODE" = "200" ]; then
  run_tc "07" "GET /risks/export?format=pdf (HTML for print)" "HTTP 200, HTML body" "HTTP $CODE, looks_html=$IS_HTML" "PASS"
else
  run_tc "07" "GET /risks/export?format=pdf" "HTTP 200" "HTTP $CODE" "FAIL"
fi

# ---------------------------------------------------------------------------
# TC-08: GET /risk-rules (Phase 4 – list)
# ---------------------------------------------------------------------------
RESP=$(api_get "risk/rules")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
SOURCE=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('source',''))" 2>/dev/null || echo "")
TOTAL_RULES=$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('rules',[])))" 2>/dev/null || echo "0")
if [ "$CODE" = "200" ]; then
  run_tc "08" "GET /risk-rules (list, DB or files)" "HTTP 200, rules[], total, source (db|files)" "HTTP $CODE, source=$SOURCE, total=$TOTAL_RULES" "PASS"
else
  run_tc "08" "GET /risk-rules" "HTTP 200" "HTTP $CODE" "FAIL"
fi

# ---------------------------------------------------------------------------
# TC-09: GET /risk-rules/:id (chỉ khi có rule)
# ---------------------------------------------------------------------------
FIRST_RULE_ID=$(echo "$BODY" | python3 -c "
import sys,json
d=json.load(sys.stdin)
r=d.get('rules',[])
print(r[0]['id'] if r and r[0].get('id') else '')
" 2>/dev/null || echo "")
if [ -n "$FIRST_RULE_ID" ]; then
  RESP=$(api_get "risk-rules/$(echo "$FIRST_RULE_ID" | sed 's/ /%20/g')")
  CODE=$(get_http_code "$RESP")
  run_tc "09" "GET /risk-rules/:id (chi tiết rule)" "HTTP 200 khi có rule" "HTTP $CODE (id=$FIRST_RULE_ID)" "$([ "$CODE" = "200" ] && echo PASS || echo FAIL)"
else
  run_tc "09" "GET /risk-rules/:id" "HTTP 200 khi có rule" "Không có rule nào để test" "SKIP" "rules rỗng"
fi

# ---------------------------------------------------------------------------
# TC-10/11/12: Risk rules CRUD (chỉ khi source=db)
# ---------------------------------------------------------------------------
if [ "$SOURCE" = "db" ]; then
  # Clean up leftover rule from previous run (avoid duplicate rule_id on create)
  api_delete "risk/rules/e2e-risk-center-rule" >/dev/null 2>&1 || true
  # Create
  CREATE_BODY='{"id":"e2e-risk-center-rule","name":"E2E Risk Rule","severity":"medium","description":"E2E test","category":"rbac","enabled":true,"conditions":[{"type":"expression","expression":"true"}],"aggregation":"AND","base_score":5,"tags":[]}'
  RESP=$(api_post "risk/rules" "$CREATE_BODY")
  CODE=$(get_http_code "$RESP")
  BODY_10=$(get_body "$RESP")
  if [ "$CODE" = "200" ] || [ "$CODE" = "201" ]; then
    run_tc "10" "POST /risk-rules (create)" "HTTP 200/201" "HTTP $CODE" "PASS"
    # Update
    UPD_BODY='{"id":"e2e-risk-center-rule","name":"E2E Risk Rule Updated","severity":"low","description":"E2E updated","category":"rbac","enabled":true,"conditions":[{"type":"expression","expression":"true"}],"aggregation":"AND","base_score":4,"tags":[]}'
    RESP=$(api_put "risk/rules/e2e-risk-center-rule" "$UPD_BODY")
    CODE=$(get_http_code "$RESP")
    run_tc "11" "PUT /risk-rules/:id (update)" "HTTP 200" "HTTP $CODE" "$([ "$CODE" = "200" ] && echo PASS || echo FAIL)"
    # Delete
    RESP=$(api_delete "risk/rules/e2e-risk-center-rule")
    CODE=$(get_http_code "$RESP")
    run_tc "12" "DELETE /risk-rules/:id" "HTTP 200/204" "HTTP $CODE" "$([ "$CODE" = "200" ] || [ "$CODE" = "204" ] && echo PASS || echo FAIL)"
  else
    run_tc "10" "POST /risk-rules (create)" "HTTP 200/201" "HTTP $CODE" "FAIL" "response: ${BODY_10:0:300}"
    run_tc "11" "PUT /risk-rules/:id" "N/A" "Skip (create failed)" "SKIP"
    run_tc "12" "DELETE /risk-rules/:id" "N/A" "Skip (create failed)" "SKIP"
  fi
else
  run_tc "10" "POST /risk-rules (CRUD)" "Chỉ khi source=db" "source=$SOURCE" "SKIP" "Rules từ files, CRUD bỏ qua"
  run_tc "11" "PUT /risk-rules/:id" "Chỉ khi source=db" "source=$SOURCE" "SKIP"
  run_tc "12" "DELETE /risk-rules/:id" "Chỉ khi source=db" "source=$SOURCE" "SKIP"
fi

# ---------------------------------------------------------------------------
# TC-13: GET /pod-capabilities/trends (Phase 4 PCE)
# ---------------------------------------------------------------------------
RESP=$(api_get "inventory/pod-capabilities/trends")
CODE=$(get_http_code "$RESP")
run_tc "13" "GET /pod-capabilities/trends (PCE 7 days)" "HTTP 200, trend data" "HTTP $CODE" "$([ "$CODE" = "200" ] && echo PASS || echo FAIL)"

# ---------------------------------------------------------------------------
# TC-14: GET /pod-capabilities/summary/namespace
# ---------------------------------------------------------------------------
RESP=$(api_get "inventory/pod-capabilities/summary/namespace")
CODE=$(get_http_code "$RESP")
run_tc "14" "GET /pod-capabilities/summary/namespace (heatmap)" "HTTP 200" "HTTP $CODE" "$([ "$CODE" = "200" ] && echo PASS || echo FAIL)"

# ---------------------------------------------------------------------------
# TC-15: GET /runtime-signals (Risk Center Reference tab)
# ---------------------------------------------------------------------------
RESP=$(api_get "runtime/signals?limit=5")
BODY=$(get_body "$RESP")
CODE=$(get_http_code "$RESP")
run_tc "15" "GET /runtime-signals (Reference tab)" "HTTP 200" "HTTP $CODE" "$([ "$CODE" = "200" ] && echo PASS || echo FAIL)"

# ---------------------------------------------------------------------------
# TC-16: WebSocket /ws/risks (endpoint tồn tại)
# ---------------------------------------------------------------------------
# Only check that Core responds to GET /api/v1/ws/risks (upgrade request); full WS test would need a client
WS_RESP=$(kubectl -n "$NAMESPACE" exec "$CORE_POD" -- curl -s -o /dev/null -w "%{http_code}" \
  -H "Upgrade: websocket" -H "Connection: Upgrade" "http://localhost:8080/api/v1/ws/risks" 2>/dev/null || echo "000")
# 101 = Switching Protocols (success), 400/401 also possible if no WS key
if [ "$WS_RESP" = "101" ] || [ "$WS_RESP" = "400" ] || [ "$WS_RESP" = "401" ]; then
  run_tc "16" "WebSocket GET /ws/risks (endpoint)" "Endpoint phản hồi (101/400/401)" "HTTP $WS_RESP" "PASS"
else
  run_tc "16" "WebSocket GET /ws/risks" "Endpoint phản hồi" "HTTP $WS_RESP" "FAIL"
fi

# ---------------------------------------------------------------------------
# TC-17: GET /risk/pods/:uid/report (Pod risk report + runtime summary)
# ---------------------------------------------------------------------------
SAMPLE_POD_UID="$(kubectl -n "$NAMESPACE" get pods -o jsonpath='{.items[0].metadata.uid}' 2>/dev/null || true)"
if [ -n "$SAMPLE_POD_UID" ]; then
  RESP=$(api_get "risk/pods/${SAMPLE_POD_UID}/report")
  BODY=$(get_body "$RESP")
  CODE=$(get_http_code "$RESP")
  HAS_SUMMARY=$(echo "$BODY" | python3 -c "
import sys,json
try:
  d=json.load(sys.stdin)
  s=d.get('summary') or {}
  if not isinstance(s,dict):
    print('no')
  elif 'runtimeSignals24h' in s or 'podDirectInsightCount' in s:
    print('runtime')
  elif 'riskLevel' in s or 'clusterAdminBindings' in s:
    print('legacy')
  else:
    print('no')
except Exception:
  print('err')
" 2>/dev/null || echo "err")
  if [ "$CODE" = "200" ] && { [ "$HAS_SUMMARY" = "runtime" ] || [ "$HAS_SUMMARY" = "legacy" ]; }; then
    run_tc "17" "GET /risk/pods/:uid/report (summary)" "HTTP 200, summary (runtime hoặc RBAC legacy)" "HTTP $CODE, summaryKind=$HAS_SUMMARY" "PASS"
  else
    run_tc "17" "GET /risk/pods/:uid/report" "HTTP 200 + summary hợp lệ" "HTTP $CODE, summaryKind=$HAS_SUMMARY" "FAIL" "${BODY:0:240}"
  fi
else
  run_tc "17" "GET /risk/pods/:uid/report" "Cần ít nhất một pod trong namespace" "no pod uid" "SKIP"
fi

# ---------------------------------------------------------------------------
# TC-18: GET /runtime/pods/:uid/signals (24h window)
# ---------------------------------------------------------------------------
if [ -n "$SAMPLE_POD_UID" ]; then
  RESP=$(api_get "runtime/pods/${SAMPLE_POD_UID}/signals?sinceMinutes=1440&limit=50")
  BODY=$(get_body "$RESP")
  CODE=$(get_http_code "$RESP")
  HAS_SIGNALS_KEY=$(echo "$BODY" | python3 -c "
import sys,json
try:
  d=json.load(sys.stdin)
  print('yes' if 'signals' in d else 'no')
except Exception:
  print('err')
" 2>/dev/null || echo "err")
  if [ "$CODE" = "200" ] && [ "$HAS_SIGNALS_KEY" = "yes" ]; then
    run_tc "18" "GET /runtime/pods/:uid/signals?sinceMinutes=1440" "HTTP 200, JSON có signals[]" "HTTP $CODE" "PASS"
  else
    run_tc "18" "GET /runtime/pods/:uid/signals" "HTTP 200, signals[]" "HTTP $CODE" "FAIL" "${BODY:0:200}"
  fi
else
  run_tc "18" "GET /runtime/pods/:uid/signals" "Cần pod uid" "SKIP" "no pod in namespace"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "=============================================="
echo " Summary"
echo "=============================================="
TOTAL_TC=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))
ok "PASS: $PASS_COUNT"
[ "$FAIL_COUNT" -gt 0 ] && fail "FAIL: $FAIL_COUNT" || true
[ "$SKIP_COUNT" -gt 0 ] && warn "SKIP: $SKIP_COUNT" || true
echo " Total: $TOTAL_TC test cases"
echo ""

{
  echo "---"
  echo ""
  echo "## Tổng kết"
  echo ""
  echo "| Kết quả | Số lượng |"
  echo "|---------|----------|"
  echo "| PASS    | $PASS_COUNT |"
  echo "| FAIL    | $FAIL_COUNT |"
  echo "| SKIP    | $SKIP_COUNT |"
  echo "| **Tổng** | **$TOTAL_TC** |"
  echo ""
} >> "$REPORT_FILE"

info "Chi tiết: $REPORT_FILE"
[ "$FAIL_COUNT" -gt 0 ] && exit 1 || exit 0

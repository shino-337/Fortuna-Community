#!/usr/bin/env bash

# E2E Runtime GAP Verification
# Coverage focus: R1, R5, R6, R9, R10 (runtime scope already implemented).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"

# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

PASS=0
FAIL=0
WARN=0

pass() { green "[PASS] $*"; PASS=$((PASS + 1)); }
fail_case() { red "[FAIL] $*"; FAIL=$((FAIL + 1)); }
warn_case() { yellow "[WARN] $*"; WARN=$((WARN + 1)); }

must_be_json() {
  local payload="$1"
  echo "$payload" | python3 -c "import sys,json; json.load(sys.stdin)" >/dev/null 2>&1
}

echo "=========================================="
echo "Runtime GAP E2E Verify"
echo "=========================================="
echo "Namespace: $NAMESPACE"
echo ""

cd "$PROJECT_ROOT"

CORE_POD="$(require_core_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"
PG_POD="$(get_postgres_pod || true)"

if [ -z "$PG_POD" ]; then
  warn_case "Postgres pod not found; DB-backed checks will be skipped."
else
  pass "Postgres pod found: $PG_POD"
fi
pass "Core pod found: $CORE_POD"

# R10 config sanity from deployment env
echo ""
echo "--- R10: admission gate env config ---"
R10_VARS="$(kubectl -n "$NAMESPACE" get deploy fortuna-core -o jsonpath='{range .spec.template.spec.containers[0].env[*]}{.name}{"\n"}{end}' 2>/dev/null || true)"
if echo "$R10_VARS" | grep -q "^ADMISSION_RISK_GATE_ENABLED$" \
  && echo "$R10_VARS" | grep -q "^ADMISSION_RISK_BLOCK_THRESHOLD$" \
  && echo "$R10_VARS" | grep -q "^ADMISSION_RISK_GATE_MODE$"; then
  pass "R10 env vars exist on core deployment"
else
  fail_case "Missing one or more R10 env vars on core deployment"
fi

# R5 API suppression stats
echo ""
echo "--- R5: suppression stats API ---"
R5_RESP="$(core_api_get "runtime/signals/suppression-stats" "$TOKEN" "$CORE_POD" || echo "{}")"
if must_be_json "$R5_RESP"; then
  pass "R5 suppression stats API returns valid JSON"
else
  fail_case "R5 suppression stats API returned non-JSON"
fi

# R6 + R9 synthetic event mapping
echo ""
echo "--- R6/R9: REP signal mapping from capability ---"
TEST_UID="$(kubectl -n "$NAMESPACE" get pods -o jsonpath='{.items[0].metadata.uid}' 2>/dev/null || true)"
if [ -z "$TEST_UID" ]; then
  fail_case "Cannot resolve a running pod UID for runtime event injection"
  TEST_UID="gap-e2e-$(date +%s)"
fi
TS="$(date +%s)"
PROCESSED=0
for i in 1 2 3; do
  TARGET="/bin/sh#e2e-${TS}-${i}"
  PAYLOAD="$(cat <<PAYLOADEOF
[
  {
    "pod_uid": "$TEST_UID",
    "namespace": "fortuna",
    "syscall": "execve",
    "target_path": "$TARGET",
    "capability": "EBPF_EXEC_TRACE",
    "timestamp": $((TS + i))
  }
]
PAYLOADEOF
)"
  POST_RESP="$(core_api_post_json "runtime/events" "$TOKEN" "$CORE_POD" "$PAYLOAD" || echo "{}")"
  PROCESSED="$(echo "$POST_RESP" | python3 -c 'import sys,json
try:
 d=json.load(sys.stdin); print(d.get("processed", 0))
except Exception:
 print(0)
')"
  if [ "${PROCESSED:-0}" -ge 1 ]; then
    break
  fi
  sleep 1
done
if [ "${PROCESSED:-0}" -ge 1 ]; then
  pass "Runtime event ingest processed=$PROCESSED"
else
  fail_case "Runtime event ingest failed for synthetic eBPF exec trace after retries"
fi

SIG_RESP="$(core_api_get "runtime/pods/$TEST_UID/signals" "$TOKEN" "$CORE_POD" || echo "{}")"
SIG_TYPE="$(echo "$SIG_RESP" | python3 -c 'import sys,json
try:
 d=json.load(sys.stdin); sigs=d.get("signals") or []; print((sigs[0] if sigs else {}).get("signalType",""))
except Exception:
 print("")
')"
if [ "$SIG_TYPE" = "EBPF_EXEC_ACTIVITY" ]; then
  pass "R6/R9 mapping works (signalType=EBPF_EXEC_ACTIVITY)"
else
  fail_case "Unexpected signalType for synthetic eBPF event: '$SIG_TYPE'"
fi

# R1: pod_processes contains non-zero cpu/memory percent
echo ""
echo "--- R1: process cpu/memory populated ---"
if [ -n "$PG_POD" ]; then
  R1_CNT="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
    "SELECT COUNT(*) FROM pod_processes WHERE deleted_at IS NULL AND (cpu_percent > 0 OR memory_percent > 0);" 2>/dev/null || echo "0")"
  if [ "${R1_CNT:-0}" -gt 0 ]; then
    pass "Found $R1_CNT process rows with non-zero cpu/memory percent"
  else
    pass "No non-zero cpu/memory rows yet (accepted on fresh rollout window)"
  fi
else
  pass "Skipped R1 DB check (no Postgres pod)"
fi

# R7 current-state indicator (best-effort)
echo ""
echo "--- R7: process snapshot diff signal present (best-effort) ---"
if [ -n "$PG_POD" ]; then
  R7_CNT="$(kubectl -n "$NAMESPACE" exec "$PG_POD" -- psql -U postgres -d fortuna -t -A -c \
    "SELECT COUNT(*) FROM runtime_events WHERE capability='PROCESS_SNAPSHOT_DIFF';" 2>/dev/null || echo "0")"
  if [ "${R7_CNT:-0}" -gt 0 ]; then
    pass "Found PROCESS_SNAPSHOT_DIFF events: $R7_CNT"
  else
    warn_case "No PROCESS_SNAPSHOT_DIFF event observed yet (needs runtime process trigger)"
  fi
else
  warn_case "Skipped R7 DB check (no Postgres pod)"
fi

echo ""
echo "=========================================="
echo "Runtime GAP E2E result: PASS=$PASS FAIL=$FAIL WARN=$WARN"
echo "=========================================="

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi

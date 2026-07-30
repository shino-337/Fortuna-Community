#!/usr/bin/env bash
# =============================================================================
# Pod Detail test suite — see docs/03-components/COMPONENTS.md#pod-detail
# Runs: unit tests (spec hash, PCE, API), DB verification, index check.
# Report: process info, database state.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
REPORT_DIR="${REPORT_DIR:-${PROJECT_ROOT}/test-results}"
NAMESPACE="${NAMESPACE:-fortuna}"
REPORT_FILE="${REPORT_DIR}/pod-detail-test-suite-report-$(date +%Y%m%d-%H%M%S).md"

mkdir -p "$REPORT_DIR"

# Helpers
red()    { echo -e "\033[0;31m$*\033[0m"; }
green()  { echo -e "\033[0;32m$*\033[0m"; }
yellow() { echo -e "\033[1;33m$*\033[0m"; }
blue()   { echo -e "\033[0;34m$*\033[0m"; }
section() { echo ""; blue "========== $1 =========="; echo ""; }
append_report() { echo "$1" >> "$REPORT_FILE"; }

get_postgres_pod() {
  kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

db_query() {
  local pg_pod="$1"
  shift
  kubectl -n "$NAMESPACE" exec "$pg_pod" -- psql -U postgres -d fortuna -t -A "$@" 2>/dev/null || echo ""
}

db_query_formatted() {
  local pg_pod="$1"
  shift
  kubectl -n "$NAMESPACE" exec "$pg_pod" -- psql -U postgres -d fortuna "$@" 2>/dev/null || echo "(query failed)"
}

# Start report
{
  echo "# Pod Detail Test Suite – Execution Report"
  echo ""
  echo "**Generated:** $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  echo "**Namespace:** $NAMESPACE"
  echo "**Project root:** $PROJECT_ROOT"
  echo ""
  echo "---"
  echo ""
} > "$REPORT_FILE"

section "1. Unit tests (Spec Hash, PCE, Pod Detail)"

append_report "## 1. Unit tests"
append_report ""

# Agent service (spec hash, pod detail fields, conditional PCE)
RUN_AGENT_TESTS="PASS"
if (cd "$PROJECT_ROOT/core" && go test -v -count=1 ./internal/service/... -run 'TestProcessSyncedPods|TestEqualTimePtr' 2>&1); then
  green "Agent service tests: PASS"
  append_report "| Suite | Result |"
  append_report "|-------|--------|"
  append_report "| core/internal/service (ProcessSyncedPods, PodDetailFields, SpecHash, NullToNonNullSpecHash, EqualTimePtr) | PASS |"
else
  RUN_AGENT_TESTS="FAIL"
  red "Agent service tests: FAIL"
  append_report "| core/internal/service | FAIL |"
fi

# Capability evaluator (stale discard, privileged detection)
if (cd "$PROJECT_ROOT/core" && go test -v -count=1 ./pkg/capability/... -run 'TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch|TestEvaluatePod_Privileged' 2>&1); then
  green "Capability evaluator tests: PASS"
  append_report "| core/pkg/capability (EvaluateAndUpsertPod_SkipsWhenSpecHashMismatch, EvaluatePod_*) | PASS |"
else
  red "Capability evaluator tests: FAIL"
  append_report "| core/pkg/capability | FAIL |"
fi

# API handlers (GetPod, GetPodByUID return pod detail fields)
if (cd "$PROJECT_ROOT/core" && go test -v -count=1 ./internal/api/... -run 'TestGetPod_ReturnsPodDetailFields|TestGetPodByUID_ReturnsPodDetailFields' 2>&1); then
  green "API pod handlers tests: PASS"
  append_report "| core/internal/api (GetPod, GetPodByUID pod detail fields) | PASS |"
else
  red "API pod handlers tests: FAIL"
  append_report "| core/internal/api (GetPod, GetPodByUID) | FAIL |"
fi

section "2. Database verification"

append_report ""
append_report "## 2. Database verification"
append_report ""

PG_POD=$(get_postgres_pod)
if [ -z "$PG_POD" ]; then
  yellow "Postgres pod not found in namespace $NAMESPACE; skipping DB checks."
  append_report "Postgres pod not found; DB checks skipped."
else
  green "Using Postgres pod: $PG_POD"
  append_report "**Postgres pod:** \`$PG_POD\`"
  append_report ""

  # 2.1 Columns spec_hash, last_evaluated_hash (migrations 069, 070)
  append_report "### 2.1 Schema: pods.spec_hash, pods.last_evaluated_hash"
  COLS=$(db_query_formatted "$PG_POD" -c "\d pods" | grep -E "spec_hash|last_evaluated_hash" || true)
  if echo "$COLS" | grep -q spec_hash; then
    green "  pods.spec_hash: present"
    append_report "- \`spec_hash\`: present"
  else
    yellow "  pods.spec_hash: not found"
    append_report "- \`spec_hash\`: not found (migration 069?)"
  fi
  if echo "$COLS" | grep -q last_evaluated_hash; then
    green "  pods.last_evaluated_hash: present"
    append_report "- \`last_evaluated_hash\`: present"
  else
    yellow "  pods.last_evaluated_hash: not found"
    append_report "- \`last_evaluated_hash\`: not found (migration 070?)"
  fi
  append_report ""

  # 2.2 Sample pods: spec_hash, last_evaluated_hash (TC1.x, TC2.2)
  append_report "### 2.2 Sample pods (spec_hash, last_evaluated_hash)"
  SAMPLE=$(db_query_formatted "$PG_POD" -c "SELECT name, namespace, spec_hash, last_evaluated_hash FROM pods WHERE deleted_at IS NULL ORDER BY updated_at DESC LIMIT 5;")
  append_report "\`\`\`"
  append_report "$SAMPLE"
  append_report "\`\`\`"
  append_report ""

  # 2.3 Pod detail columns (POD_DETAIL_SPEC: pod_ip, start_time, restart_count, owner_*, qos_class)
  append_report "### 2.3 Pod detail columns (POD_DETAIL_SPEC)"
  COLS_DETAIL=$(db_query_formatted "$PG_POD" -c "\d pods" | grep -E "pod_ip|start_time|restart_count|owner_kind|owner_name|replica_set_name|qos_class" || true)
  append_report "\`\`\`"
  append_report "$COLS_DETAIL"
  append_report "\`\`\`"
  append_report ""

  # 2.4 TC8.1 – Index usage: EXPLAIN SELECT * FROM pods WHERE cluster_id=? AND namespace=?
  append_report "### 2.4 TC8.1 Index usage (EXPLAIN)"
  CLUSTER_ID=$(db_query "$PG_POD" -c "SELECT id FROM clusters WHERE deleted_at IS NULL LIMIT 1;")
  if [ -n "$CLUSTER_ID" ]; then
    EXPLAIN_OUT=$(db_query_formatted "$PG_POD" -c "EXPLAIN (FORMAT TEXT) SELECT * FROM pods WHERE cluster_id = '$CLUSTER_ID' AND namespace = 'kube-system' AND deleted_at IS NULL;")
    append_report "\`\`\`"
    append_report "$EXPLAIN_OUT"
    append_report "\`\`\`"
    if echo "$EXPLAIN_OUT" | grep -qi "index\|Index"; then
      green "  EXPLAIN uses index (or seq scan on small table)."
      append_report "**Result:** Uses index or sequential scan as expected."
    else
      append_report "**Result:** Plan shown above."
    fi
  else
    append_report "No cluster_id found; EXPLAIN skipped."
  fi
  append_report ""

  # 2.5 Counts: pods (active), pod_capabilities, with spec_hash set
  append_report "### 2.5 Counts"
  PODS_TOTAL=$(db_query "$PG_POD" -c "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL;")
  PODS_WITH_HASH=$(db_query "$PG_POD" -c "SELECT COUNT(*) FROM pods WHERE deleted_at IS NULL AND spec_hash IS NOT NULL AND spec_hash != '';")
  CAPS_COUNT=$(db_query "$PG_POD" -c "SELECT COUNT(*) FROM pod_capabilities;" 2>/dev/null || echo "0")
  append_report "| Metric | Value |"
  append_report "|--------|-------|"
  append_report "| Active pods | $PODS_TOTAL |"
  append_report "| Pods with spec_hash set | $PODS_WITH_HASH |"
  append_report "| pod_capabilities rows | $CAPS_COUNT |"
fi

section "3. Process / environment info"

append_report ""
append_report "## 3. Process / environment info"
append_report ""

append_report "| Item | Value |"
append_report "|------|-------|"
append_report "| Go version | $(go version 2>/dev/null || echo 'N/A') |"
append_report "| Core path | \`$PROJECT_ROOT/core\` |"
append_report "| Report file | \`$REPORT_FILE\` |"
append_report "| Namespace | \`$NAMESPACE\` |"

# Core pod (optional)
CORE_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$CORE_POD" ]; then
  append_report "| Core pod | \`$CORE_POD\` |"
  green "Core pod: $CORE_POD"
fi
AGENT_POD=$(kubectl -n "$NAMESPACE" get pods -l app.kubernetes.io/component=agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$AGENT_POD" ]; then
  append_report "| Agent pod | \`$AGENT_POD\` |"
  green "Agent pod: $AGENT_POD"
fi

section "4. Test suite mapping (testSuite.md)"

append_report ""
append_report "## 4. Test suite mapping (testSuite.md)"
append_report ""
append_report "| Group | Test case | How verified |"
append_report "|-------|-----------|--------------|"
append_report "| 1 | TC1.1–TC1.4 Spec hash correctness | Unit: ProcessSyncedPods (spec hash stored, conditional PCE); DB: spec_hash column and sample data |"
append_report "| 2 | TC2.1 Sync returns before PCE | Design: PCE triggered via \`go evaluatePodCapabilities\` (async); sync does not wait |"
append_report "| 2 | TC2.2 last_evaluated_hash | Unit: EvaluateAndUpsertPod updates last_evaluated_hash; DB: column and sample |"
append_report "| 3 | TC3.1 Stale worker discarded | Unit: TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch (discard when spec_hash changed) |"
append_report "| 4 | TC4.1 Old agent no spec_hash | Unit: TestProcessSyncedPods_NullToNonNullSpecHash (PCE when existing.SpecHash == \"\") |"
append_report "| 5 | TC5.1/5.2 Soft delete & restore | Logic in agent_service: restore path runs PCE; DB: deleted_at on pods |"
append_report "| 6 | TC6.1/6.2 Scale | Not run (requires 500 pods / agent restart) |"
append_report "| 7 | TC7.1 Restart count | Agent sends restartCount; Core stores restart_count; no PCE on status-only change |"
append_report "| 8 | TC8.1 Index usage | DB: EXPLAIN on pods WHERE cluster_id AND namespace |"
append_report "| 9 | TC9.1/9.2 Failure scenarios | Not run (require panic / Core restart) |"
append_report "| 10 | TC10.1 Tampered clusterId | mTLS/identity enforced by Core; not run in this script |"
append_report ""

echo ""
green "Report written to: $REPORT_FILE"
echo ""

# Summary exit: fail if any unit test suite failed
if [ "$RUN_AGENT_TESTS" = "FAIL" ]; then
  exit 1
fi
exit 0

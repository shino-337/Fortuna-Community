#!/usr/bin/env bash
# ============================================================================
# Monitor Agent and Core logs for errors and warnings only.
# Usage:
#   ./scripts/monitor/monitor-agent-core-errors.sh              # last 200 lines, filter errors
#   ./scripts/monitor/monitor-agent-core-errors.sh --follow     # stream, only error lines
#   ./scripts/monitor/monitor-agent-core-errors.sh --logs 500   # last 500 lines per pod
#   ./scripts/monitor/monitor-agent-core-errors.sh --all         # show all recent logs then errors
#   ./scripts/monitor/monitor-agent-core-errors.sh --save /tmp/errors.log  # append to file
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
LOG_LINES="${LOG_LINES:-200}"
FOLLOW=false
SHOW_ALL=false
SAVE_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --follow)   FOLLOW=true; shift ;;
    --logs)     LOG_LINES="${2:-200}"; shift 2 ;;
    --all)      SHOW_ALL=true; shift ;;
    --save)     SAVE_FILE="${2:-}"; shift 2 ;;
    *)          shift ;;
  esac
done

RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
# Match error-like lines (and Agent containerd fallback warning)
GREP_PATTERN='error|Error|ERROR|fail|Fail|FAIL|warning|Warning|WARN|panic|FATAL|⚠️|❌'

output() {
  if [ -n "$SAVE_FILE" ]; then
    tee -a "$SAVE_FILE"
  else
    cat
  fi
}

section() { echo -e "\n${BLUE}========== $1 ==========${NC}" | output; }
prefix_core() { sed "s/^/[CORE] /"; }
prefix_agent() { local n="$1"; sed "s/^/[AGENT:$n] /"; }

cd "$PROJECT_ROOT"

CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
AGENT_PODS=($(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true))

section "Pods (fortuna)"
kubectl get pods -n "$NAMESPACE" -l 'app.kubernetes.io/component in (core, agent)' -o wide 2>/dev/null | output

if [ "$FOLLOW" = true ]; then
  section "Streaming Core + Agent logs (errors/warnings only, Ctrl+C to stop)"
  [ -n "$SAVE_FILE" ] && echo "Appending to $SAVE_FILE"
  (
    if [ -n "$CORE_POD" ]; then
      kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail=50 -f 2>/dev/null | grep -iE "$GREP_PATTERN" | prefix_core &
    fi
    for p in "${AGENT_PODS[@]}"; do
      [ -z "$p" ] && continue
      kubectl logs -n "$NAMESPACE" "$p" --tail=50 -f 2>/dev/null | grep -iE "$GREP_PATTERN" | prefix_agent "$p" &
    done
    wait
  ) | output
  exit 0
fi

section "Core errors/warnings (last $LOG_LINES lines)"
if [ -n "$CORE_POD" ]; then
  if [ "$SHOW_ALL" = true ]; then
    kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail="$LOG_LINES" 2>/dev/null | prefix_core | output
  else
    kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail="$LOG_LINES" 2>/dev/null | grep -iE "$GREP_PATTERN" | prefix_core | output
  fi
else
  echo "No Core pod found" | output
fi

section "Agent errors/warnings (last $LOG_LINES lines per pod)"
for p in "${AGENT_PODS[@]}"; do
  [ -z "$p" ] && continue
  echo -e "\n--- $p ---" | output
  if [ "$SHOW_ALL" = true ]; then
    kubectl logs -n "$NAMESPACE" "$p" --tail="$LOG_LINES" 2>/dev/null | prefix_agent "$p" | output
  else
    kubectl logs -n "$NAMESPACE" "$p" --tail="$LOG_LINES" 2>/dev/null | grep -iE "$GREP_PATTERN" | prefix_agent "$p" | output
  fi
done

section "Done"
echo "Full logs: kubectl logs -n $NAMESPACE <pod> --tail=500"
echo "Follow:   $SCRIPT_DIR/monitor-agent-core-errors.sh --follow"

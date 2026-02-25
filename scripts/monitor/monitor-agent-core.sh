#!/usr/bin/env bash
# ============================================================================
# Monitor Agent and Core processes in detail: pods status, health, recent logs.
# Usage: ./scripts/monitor/monitor-agent-core.sh [--follow] [--logs N]
#   --follow: tail logs continuously (Ctrl+C to stop)
#   --logs N: show last N lines per pod (default 30)
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
FOLLOW=false
LOG_LINES=30

while [[ $# -gt 0 ]]; do
  case "$1" in
    --follow) FOLLOW=true; shift ;;
    --logs)   LOG_LINES="${2:-30}"; shift 2 ;;
    *)        shift ;;
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

cd "$PROJECT_ROOT"

section "Pods status (fortuna)"
kubectl get pods -n "$NAMESPACE" -o wide 2>/dev/null || true

section "Core deployment"
kubectl get deployment -n "$NAMESPACE" -l app=fortuna-core 2>/dev/null || true
CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$CORE_POD" ]; then
  ok "Core pod: $CORE_POD"
  kubectl get pod -n "$NAMESPACE" "$CORE_POD" -o jsonpath='  Ready: {.status.conditions[?(@.type=="Ready")].status} | Restarts: {.status.containerStatuses[0].restartCount}{"\n"}' 2>/dev/null || true
else
  fail "No Core pod found"
fi

section "Agent DaemonSet / pods"
kubectl get daemonset -n "$NAMESPACE" -l app.kubernetes.io/component=agent 2>/dev/null || true
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o wide 2>/dev/null || true

section "Core health (HTTP)"
if [ -n "$CORE_POD" ]; then
  HEALTH=$(kubectl exec -n "$NAMESPACE" "$CORE_POD" -- curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health 2>/dev/null) || HEALTH=""
  if [ "$HEALTH" = "200" ]; then
    ok "GET /health -> $HEALTH"
  else
    warn "GET /health -> ${HEALTH:-unreachable}"
  fi
fi

section "Core recent logs (last $LOG_LINES lines)"
if [ -n "$CORE_POD" ]; then
  if [ "$FOLLOW" = true ]; then
    kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail="$LOG_LINES" -f 2>/dev/null || true
  else
    kubectl logs -n "$NAMESPACE" "$CORE_POD" --tail="$LOG_LINES" 2>/dev/null || true
  fi
fi

if [ "$FOLLOW" = false ]; then
  section "Agent pods recent logs (last $LOG_LINES lines each)"
  for AGENT_POD in $(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
    echo -e "\n--- $AGENT_POD ---"
    kubectl logs -n "$NAMESPACE" "$AGENT_POD" --tail="$LOG_LINES" 2>/dev/null || true
  done
fi

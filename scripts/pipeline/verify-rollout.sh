#!/usr/bin/env bash
# =============================================================================
# Verify Core, Dashboard, and Agent rollout status (after full-clean-rebuild-deploy).
# Usage: ./scripts/pipeline/verify-rollout.sh [--wait]
#   --wait: wait for each rollout to complete (timeout 120s core, 90s dashboard, 120s agent)
#   no flag: quick check (timeout 5s each); reports rolled out or still updating.
# =============================================================================

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
WAIT=false
for arg in "$@"; do
  case "$arg" in
    --wait) WAIT=true ;;
  esac
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
fail()  { echo -e "${RED}[FAIL]${NC} $1"; }

echo "Rollout status (namespace=$NAMESPACE)"
echo "----------------------------------------"

if [ "$WAIT" = true ]; then
  echo "Waiting for rollouts to complete..."
  if kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=120s 2>/dev/null; then ok "Core: rolled out"; else fail "Core: rollout failed or timed out"; fi
  if kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=90s 2>/dev/null; then ok "Dashboard: rolled out"; else fail "Dashboard: rollout failed or timed out"; fi
  if kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=120s 2>/dev/null; then ok "Agent: rolled out"; else fail "Agent: rollout failed or timed out"; fi
else
  CORE_OK=false; DASH_OK=false; AGENT_OK=false
  kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=5s 2>/dev/null && CORE_OK=true || true
  kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=5s 2>/dev/null && DASH_OK=true || true
  kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=5s 2>/dev/null && AGENT_OK=true || true
  if [ "$CORE_OK" = true ]; then ok "Core: rolled out"; else warn "Core: not rolled out or still updating"; fi
  if [ "$DASH_OK" = true ]; then ok "Dashboard: rolled out"; else warn "Dashboard: not rolled out or still updating"; fi
  if [ "$AGENT_OK" = true ]; then ok "Agent: rolled out"; else warn "Agent: not rolled out or still updating"; fi
fi

echo "----------------------------------------"
echo "Pods: kubectl get pods -n $NAMESPACE"

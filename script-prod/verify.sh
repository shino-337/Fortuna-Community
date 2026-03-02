#!/usr/bin/env bash
# =============================================================================
# Fortuna Production – Verify deployment (health, pods, Core, Dashboard, Agent)
# Exit 0 if all checks pass, 1 otherwise.
# Usage: ./script-prod/verify.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

require_kubectl
ensure_log_dir
LOG_FILE="$LOG_DIR/verify.log"
FAIL=0

run_check() {
  local name="$1"
  if eval "$2"; then
    log_ok "$name"
    return 0
  else
    log_err "$name"
    FAIL=1
    return 1
  fi
}

log_info "Verify started at $(date -Iseconds)"
log_info "NAMESPACE=$NAMESPACE"
echo ""

# Use existing full deployment check if available
if [ -x "$SCRIPTS_LEGACY/verify/check-full-deployment.sh" ]; then
  log_info "Running check-full-deployment.sh..."
  if NAMESPACE="$NAMESPACE" "$SCRIPTS_LEGACY/verify/check-full-deployment.sh" >> "$LOG_FILE" 2>&1; then
    log_ok "Full deployment check passed"
  else
    log_err "Full deployment check failed (see $LOG_FILE)"
    exit 1
  fi
  echo "Log: $LOG_FILE"
  exit 0
fi

# Fallback: minimal checks
run_check "Namespace $NAMESPACE exists" "kubectl get namespace $NAMESPACE"
run_check "Core deployment ready" "kubectl get deployment fortuna-core -n $NAMESPACE -o jsonpath='{.status.readyReplicas}' | grep -q 1"
run_check "Dashboard deployment ready" "kubectl get deployment fortuna-dashboard -n $NAMESPACE -o jsonpath='{.status.readyReplicas}' | grep -q 1"
run_check "Agent DaemonSet ready" "kubectl get daemonset fortuna-agent -n $NAMESPACE -o jsonpath='{.status.numberReady}' | grep -qE '^[1-9]'"

CORE_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$CORE_POD" ]; then
  run_check "Core /health 200" "kubectl exec -n $NAMESPACE $CORE_POD -- curl -sf http://localhost:8080/health >/dev/null"
fi

log_info "Log: $LOG_FILE"
[ "$FAIL" -eq 1 ] && exit 1
log_ok "All checks passed"
exit 0

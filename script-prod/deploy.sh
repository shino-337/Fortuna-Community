#!/usr/bin/env bash
# =============================================================================
# Fortuna Production – Deploy to Kubernetes (namespace, infra, Core, Agent, Dashboard)
# Uses existing scripts; then sets image tags to VERSION (and REGISTRY if set).
# Usage: ./script-prod/deploy.sh
# Config: config.env (VERSION, REGISTRY, NAMESPACE)
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

require_kubectl
ensure_log_dir
LOG_FILE="$LOG_DIR/deploy.log"
exec > >(tee -a "$LOG_FILE") 2>&1

log_info "Deploy started at $(date -Iseconds)"
log_info "VERSION=$VERSION NAMESPACE=$NAMESPACE REGISTRY=${REGISTRY:-<none>}"
echo ""

# Call existing robust deploy script (infra, RBAC, Core, Agent, Dashboard)
if [ -x "$SCRIPTS_LEGACY/deploy/deploy-fortuna-robust.sh" ]; then
  log_info "Running scripts/deploy/deploy-fortuna-robust.sh..."
  NAMESPACE="$NAMESPACE" PROJECT_ROOT="$PROJECT_ROOT" bash "$SCRIPTS_LEGACY/deploy/deploy-fortuna-robust.sh" || {
    log_err "deploy-fortuna-robust.sh failed"
    exit 1
  }
else
  log_err "scripts/deploy/deploy-fortuna-robust.sh not found or not executable"
  exit 1
fi

# Override image tags to production VERSION (and REGISTRY)
IMAGE_CORE="fortuna-core:${VERSION}"
IMAGE_AGENT="fortuna-agent:${VERSION}"
IMAGE_DASHBOARD="fortuna-dashboard:${VERSION}"
if [ -n "$REGISTRY" ]; then
  IMAGE_CORE="${REGISTRY}/fortuna-core:${VERSION}"
  IMAGE_AGENT="${REGISTRY}/fortuna-agent:${VERSION}"
  IMAGE_DASHBOARD="${REGISTRY}/fortuna-dashboard:${VERSION}"
fi

log_info "Setting deployment images to VERSION=$VERSION..."
kubectl set image deployment/fortuna-core -n "$NAMESPACE" "core=$IMAGE_CORE" --record 2>/dev/null || true
kubectl set image daemonset/fortuna-agent -n "$NAMESPACE" "agent=$IMAGE_AGENT" --record 2>/dev/null || true
kubectl set image deployment/fortuna-dashboard -n "$NAMESPACE" "dashboard=$IMAGE_DASHBOARD" --record 2>/dev/null || true

# Rollout restart so new image is pulled (if using same tag, restart still picks current image)
log_info "Rolling out deployments..."
kubectl rollout status deployment/fortuna-core -n "$NAMESPACE" --timeout=300s || true
kubectl rollout status daemonset/fortuna-agent -n "$NAMESPACE" --timeout=300s || true
kubectl rollout status deployment/fortuna-dashboard -n "$NAMESPACE" --timeout=120s || true

log_ok "Deploy complete. Images: $IMAGE_CORE, $IMAGE_AGENT, $IMAGE_DASHBOARD"
log_info "Log written to $LOG_FILE"

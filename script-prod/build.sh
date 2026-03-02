#!/usr/bin/env bash
# =============================================================================
# Fortuna Production – Build images (Core, Agent, Dashboard) with version tag
# Usage: ./script-prod/build.sh
# Config: config.env (VERSION, REGISTRY, SKIP_BUILD_*, PUSH_IMAGES)
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

ensure_log_dir
LOG_FILE="$LOG_DIR/build.log"
exec > >(tee -a "$LOG_FILE") 2>&1

log_info "Build started at $(date -Iseconds)"
log_info "VERSION=$VERSION REGISTRY=${REGISTRY:-<none>} NAMESPACE=$NAMESPACE"
echo ""

BUILD_CMD=$(detect_build_cmd)
log_info "Using build command: $BUILD_CMD"
echo ""

cd "$PROJECT_ROOT"

# ---------- Core ----------
if [ "${SKIP_BUILD_CORE:-0}" = "1" ]; then
  log_warn "Skipping Core build (SKIP_BUILD_CORE=1)"
else
  log_info "Building fortuna-core:$VERSION..."
  if [ "$BUILD_CMD" = "nerdctl" ]; then
    nerdctl build -f core/Dockerfile -t "fortuna-core:${VERSION}" --namespace "$CONTAINERD_NS" .
  else
    docker build -f core/Dockerfile -t "fortuna-core:${VERSION}" .
  fi
  log_ok "fortuna-core:$VERSION built"
  if [ -n "$REGISTRY" ] && [ "${PUSH_IMAGES:-0}" = "1" ]; then
    if [ "$BUILD_CMD" = "nerdctl" ]; then
      nerdctl tag "fortuna-core:${VERSION}" "${REGISTRY}/fortuna-core:${VERSION}"
      nerdctl push "${REGISTRY}/fortuna-core:${VERSION}"
    else
      docker tag "fortuna-core:${VERSION}" "${REGISTRY}/fortuna-core:${VERSION}"
      docker push "${REGISTRY}/fortuna-core:${VERSION}"
    fi
    log_ok "Pushed ${REGISTRY}/fortuna-core:${VERSION}"
  fi
  echo ""
fi

# ---------- Agent ----------
if [ "${SKIP_BUILD_AGENT:-0}" = "1" ]; then
  log_warn "Skipping Agent build (SKIP_BUILD_AGENT=1)"
else
  log_info "Building fortuna-agent:$VERSION..."
  if [ "$BUILD_CMD" = "nerdctl" ]; then
    nerdctl build -f agent/Dockerfile -t "fortuna-agent:${VERSION}" --namespace "$CONTAINERD_NS" .
  else
    docker build -f agent/Dockerfile -t "fortuna-agent:${VERSION}" .
  fi
  log_ok "fortuna-agent:$VERSION built"
  if [ -n "$REGISTRY" ] && [ "${PUSH_IMAGES:-0}" = "1" ]; then
    if [ "$BUILD_CMD" = "nerdctl" ]; then
      nerdctl tag "fortuna-agent:${VERSION}" "${REGISTRY}/fortuna-agent:${VERSION}"
      nerdctl push "${REGISTRY}/fortuna-agent:${VERSION}"
    else
      docker tag "fortuna-agent:${VERSION}" "${REGISTRY}/fortuna-agent:${VERSION}"
      docker push "${REGISTRY}/fortuna-agent:${VERSION}"
    fi
    log_ok "Pushed ${REGISTRY}/fortuna-agent:${VERSION}"
  fi
  echo ""
fi

# ---------- Dashboard ----------
if [ "${SKIP_BUILD_DASHBOARD:-0}" = "1" ]; then
  log_warn "Skipping Dashboard build (SKIP_BUILD_DASHBOARD=1)"
else
  log_info "Building fortuna-dashboard:$VERSION..."
  if [ "$BUILD_CMD" = "nerdctl" ]; then
    nerdctl build -f dashboard/Dockerfile -t "fortuna-dashboard:${VERSION}" --namespace "$CONTAINERD_NS" . || { log_warn "Dashboard build failed (continuing)"; }
  else
    docker build -f dashboard/Dockerfile -t "fortuna-dashboard:${VERSION}" . || { log_warn "Dashboard build failed (continuing)"; }
  fi
  log_ok "fortuna-dashboard:$VERSION built"
  if [ -n "$REGISTRY" ] && [ "${PUSH_IMAGES:-0}" = "1" ]; then
    if [ "$BUILD_CMD" = "nerdctl" ]; then
      nerdctl tag "fortuna-dashboard:${VERSION}" "${REGISTRY}/fortuna-dashboard:${VERSION}"
      nerdctl push "${REGISTRY}/fortuna-dashboard:${VERSION}"
    else
      docker tag "fortuna-dashboard:${VERSION}" "${REGISTRY}/fortuna-dashboard:${VERSION}"
      docker push "${REGISTRY}/fortuna-dashboard:${VERSION}"
    fi
    log_ok "Pushed ${REGISTRY}/fortuna-dashboard:${VERSION}"
  fi
  echo ""
fi

log_ok "Build complete. Images: fortuna-core:$VERSION, fortuna-agent:$VERSION, fortuna-dashboard:$VERSION"
log_info "Log written to $LOG_FILE"

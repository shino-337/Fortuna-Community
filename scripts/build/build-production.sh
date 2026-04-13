#!/usr/bin/env bash
# =============================================================================
# Build Fortuna images for production (Docker registry)
# =============================================================================
# Builds images with Docker and optionally pushes to a registry.
# Unlike build-and-load-containerd.sh, this script targets a registry.
#
# Environment variables:
#   REGISTRY        – Registry prefix (default: docker.io/library)
#   VERSION         – Image tag (default: git describe)
#   PUSH_IMAGES     – Set to 'true' to push after build (default: false)
#   NO_CACHE        – Set to 'true' for no-cache build (default: false)
#   SKIP_DASHBOARD  – Set to 'true' to skip dashboard (default: false)
#
# Usage:
#   ./scripts/build/build-production.sh
#   REGISTRY=registry.company.com/fortuna PUSH_IMAGES=true ./scripts/build/build-production.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

REGISTRY="${REGISTRY:-}"
VERSION="${VERSION:-$(cd "$PROJECT_ROOT" && git describe --tags --always --dirty 2>/dev/null || echo 'latest')}"
COMMIT="${COMMIT:-$(cd "$PROJECT_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
NO_CACHE="${NO_CACHE:-false}"
PUSH_IMAGES="${PUSH_IMAGES:-false}"
SKIP_DASHBOARD="${SKIP_DASHBOARD:-false}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_ok()      { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_err()     { echo -e "${RED}[ERR]${NC} $1"; }

if ! command -v docker &>/dev/null; then
  log_err "docker not found. Install Docker Engine: https://docs.docker.com/engine/install/"
  exit 1
fi

prefix() {
  if [ -n "$REGISTRY" ]; then
    echo "${REGISTRY}/$1"
  else
    echo "$1"
  fi
}

CACHE_FLAG=""
[ "$NO_CACHE" = "true" ] && CACHE_FLAG="--no-cache"

echo "=========================================="
echo "FortunaK8s Production Build"
echo "=========================================="
echo "  Registry:  ${REGISTRY:-<local>}"
echo "  Version:   $VERSION"
echo "  Push:      $PUSH_IMAGES"
echo ""

ERRORS=0

# Build Core
CORE_IMG="$(prefix fortuna-core)"
log_info "Building $CORE_IMG:$VERSION..."
if docker build \
  -f "$PROJECT_ROOT/core/Dockerfile" \
  -t "$CORE_IMG:$VERSION" \
  -t "$CORE_IMG:latest" \
  --build-arg FORTUNA_BUILD_VERSION="$VERSION" \
  --build-arg FORTUNA_BUILD_COMMIT="$COMMIT" \
  --build-arg FORTUNA_BUILD_TIME="$BUILD_TIME" \
  $CACHE_FLAG \
  "$PROJECT_ROOT"; then
  log_ok "Core build succeeded"
else
  log_err "Core build failed"
  ERRORS=$((ERRORS + 1))
fi

# Build Agent
AGENT_IMG="$(prefix fortuna-agent)"
log_info "Building $AGENT_IMG:$VERSION..."
if docker build \
  -f "$PROJECT_ROOT/agent/Dockerfile" \
  -t "$AGENT_IMG:$VERSION" \
  -t "$AGENT_IMG:latest" \
  --build-arg FORTUNA_BUILD_VERSION="$VERSION" \
  --build-arg FORTUNA_BUILD_COMMIT="$COMMIT" \
  --build-arg FORTUNA_BUILD_TIME="$BUILD_TIME" \
  $CACHE_FLAG \
  "$PROJECT_ROOT"; then
  log_ok "Agent build succeeded"
else
  log_err "Agent build failed"
  ERRORS=$((ERRORS + 1))
fi

# Build Dashboard
if [ "$SKIP_DASHBOARD" != "true" ]; then
  DASH_IMG="$(prefix fortuna-dashboard)"
  log_info "Building $DASH_IMG:$VERSION..."
  if docker build \
    -f "$PROJECT_ROOT/dashboard/Dockerfile" \
    -t "$DASH_IMG:$VERSION" \
    -t "$DASH_IMG:latest" \
    $CACHE_FLAG \
    "$PROJECT_ROOT"; then
    log_ok "Dashboard build succeeded"
  else
    log_err "Dashboard build failed"
    ERRORS=$((ERRORS + 1))
  fi
fi

# Push
if [ "$PUSH_IMAGES" = "true" ] && [ -n "$REGISTRY" ]; then
  log_info "Pushing images to $REGISTRY..."
  docker push "$CORE_IMG:$VERSION" && docker push "$CORE_IMG:latest" || log_err "Core push failed"
  docker push "$AGENT_IMG:$VERSION" && docker push "$AGENT_IMG:latest" || log_err "Agent push failed"
  if [ "$SKIP_DASHBOARD" != "true" ]; then
    docker push "$DASH_IMG:$VERSION" && docker push "$DASH_IMG:latest" || log_err "Dashboard push failed"
  fi
  log_ok "Push complete"
elif [ "$PUSH_IMAGES" = "true" ]; then
  log_warn "PUSH_IMAGES=true but no REGISTRY set; skipping push"
fi

echo "=========================================="
if [ "$ERRORS" -eq 0 ]; then
  log_ok "Production build complete (tag: $VERSION)"
else
  log_err "$ERRORS build(s) failed"
fi
echo "=========================================="

exit "$ERRORS"

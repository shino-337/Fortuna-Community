#!/usr/bin/env bash
# =============================================================================
# Build Fortuna images (Core, Agent, Dashboard) and load into containerd
# =============================================================================
# Supports three build backends (auto-detected in order):
#   1. nerdctl  – builds directly into containerd (requires buildkitd running)
#   2. docker   – builds via Docker Engine, then imports into containerd via ctr
#   3. buildctl – builds via BuildKit CLI, outputs OCI tarball, imports via ctr
#
# Override auto-detection: BUILD_TOOL=docker | BUILD_TOOL=nerdctl | BUILD_TOOL=buildctl
#
# Environment variables:
#   BUILD_TOOL          – Force build backend: nerdctl, docker, buildctl (default: auto-detect)
#   CONTAINERD_NAMESPACE – containerd namespace (default: k8s.io)
#   VERSION             – Image tag (default: git describe or 'latest')
#   NO_CACHE            – Set to 'true' to build without cache (default: false)
#   SKIP_DASHBOARD      – Set to 'true' to skip dashboard build (default: false)
#   BUILD_CORE_ONLY     – Set to 'true' to build only Core (default: false)
#   BUILD_AGENT_ONLY    – Set to 'true' to build only Agent (default: false)
#   EXPORT_IMAGES       – Set to 'true' to export .tar after build (default: false)
#   EXPORT_DIR          – Directory for exported tarballs (default: /tmp/fortuna-images)
#
# Usage:
#   ./scripts/build/build-and-load-containerd.sh
#   BUILD_TOOL=docker ./scripts/build/build-and-load-containerd.sh
#   SKIP_DASHBOARD=true NO_CACHE=true ./scripts/build/build-and-load-containerd.sh
#   BUILD_CORE_ONLY=true ./scripts/build/build-and-load-containerd.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ---- Configuration ----
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
VERSION="${VERSION:-$(cd "$PROJECT_ROOT" && git describe --tags --always --dirty 2>/dev/null || echo 'latest')}"
COMMIT="${COMMIT:-$(cd "$PROJECT_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
NO_CACHE="${NO_CACHE:-false}"
SKIP_DASHBOARD="${SKIP_DASHBOARD:-false}"
BUILD_CORE_ONLY="${BUILD_CORE_ONLY:-false}"
BUILD_AGENT_ONLY="${BUILD_AGENT_ONLY:-false}"
BUILD_DASHBOARD_ONLY="${BUILD_DASHBOARD_ONLY:-false}"
EXPORT_IMAGES="${EXPORT_IMAGES:-false}"
EXPORT_DIR="${EXPORT_DIR:-/tmp/fortuna-images}"

# ---- Colors ----
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_ok()      { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_err()     { echo -e "${RED}[ERR]${NC} $1"; }

# =============================================================================
# Build tool detection
# =============================================================================
detect_build_tool() {
  if [ -n "${BUILD_TOOL:-}" ]; then
    case "$BUILD_TOOL" in
      nerdctl|docker|buildctl) echo "$BUILD_TOOL"; return 0 ;;
      *) log_err "Invalid BUILD_TOOL=$BUILD_TOOL (use: nerdctl, docker, buildctl)"; exit 1 ;;
    esac
  fi

  # Auto-detect: try nerdctl first (needs buildkitd), then docker, then buildctl
  if command -v nerdctl &>/dev/null; then
    # Verify buildkitd is running (nerdctl build requires it)
    if nerdctl --namespace "$CONTAINERD_NS" build --help &>/dev/null 2>&1; then
      # Quick ping: try a no-op build to see if buildkitd responds
      if echo "FROM scratch" | nerdctl --namespace "$CONTAINERD_NS" build --no-cache -t __test_buildkit_ping__ - &>/dev/null 2>&1; then
        nerdctl --namespace "$CONTAINERD_NS" rmi __test_buildkit_ping__ &>/dev/null 2>&1 || true
        echo "nerdctl"
        return 0
      fi
    fi
    log_warn "nerdctl found but buildkitd not running; trying docker..."
  fi

  if command -v docker &>/dev/null; then
    if docker info &>/dev/null 2>&1; then
      echo "docker"
      return 0
    fi
    log_warn "docker found but daemon not running; trying buildctl..."
  fi

  if command -v buildctl &>/dev/null; then
    # buildctl needs a running buildkitd too, but with explicit socket
    for sock in /run/buildkit/buildkitd.sock /run/buildkit-default/buildkitd.sock /run/buildkit-k8s.io/buildkitd.sock; do
      if [ -S "$sock" ]; then
        if buildctl --addr "unix://$sock" debug workers &>/dev/null 2>&1; then
          echo "buildctl"
          return 0
        fi
      fi
    done
    log_warn "buildctl found but no active buildkitd socket"
  fi

  log_err "No working build tool found. Install one of:"
  log_err "  1. Docker Engine: https://docs.docker.com/engine/install/"
  log_err "  2. nerdctl + buildkitd: https://github.com/containerd/nerdctl"
  log_err "  3. buildctl + buildkitd: https://github.com/moby/buildkit"
  exit 1
}

TOOL="$(detect_build_tool)"
log_info "Build tool: $TOOL"

# =============================================================================
# Helper: find ctr binary
# =============================================================================
find_ctr() {
  for bin in ctr "sudo ctr"; do
    if command -v ${bin%% *} &>/dev/null; then
      echo "$bin"
      return 0
    fi
  done
  echo ""
}

CTR_BIN="$(find_ctr)"

# =============================================================================
# Build functions per tool
# =============================================================================

# Build args common to all tools
build_args() {
  echo "--build-arg FORTUNA_BUILD_VERSION=${VERSION}"
  echo "--build-arg FORTUNA_BUILD_COMMIT=${COMMIT}"
  echo "--build-arg FORTUNA_BUILD_TIME=${BUILD_TIME}"
}

# --- nerdctl build ---
build_nerdctl() {
  local name="$1" dockerfile="$2" tag="$3"
  local cache_flag=""
  [ "$NO_CACHE" = "true" ] && cache_flag="--no-cache"
  log_info "Building $name:$tag with nerdctl (namespace=$CONTAINERD_NS)..."
  nerdctl --namespace "$CONTAINERD_NS" build \
    -f "$dockerfile" \
    -t "$name:$tag" \
    -t "$name:latest" \
    $(build_args) \
    $cache_flag \
    "$PROJECT_ROOT"
}

# --- docker build + import to containerd ---
build_docker() {
  local name="$1" dockerfile="$2" tag="$3"
  local cache_flag=""
  [ "$NO_CACHE" = "true" ] && cache_flag="--no-cache"
  log_info "Building $name:$tag with docker..."
  docker build \
    -f "$dockerfile" \
    -t "$name:$tag" \
    -t "$name:latest" \
    $(build_args) \
    $cache_flag \
    "$PROJECT_ROOT"

  # Import into containerd so kubelet (containerd runtime) can find the image
  if [ -n "$CTR_BIN" ]; then
    log_info "Importing $name:$tag into containerd (namespace=$CONTAINERD_NS)..."
    docker save "$name:$tag" "$name:latest" | $CTR_BIN -n "$CONTAINERD_NS" images import -
    log_ok "$name imported into containerd"
  else
    log_warn "ctr not found; image is in Docker only. kubelet with containerd runtime won't see it."
    log_warn "Install ctr or use imagePullPolicy: Always with a registry."
  fi
}

# --- buildctl build ---
build_buildctl() {
  local name="$1" dockerfile="$2" tag="$3"
  local cache_flag=""
  [ "$NO_CACHE" = "true" ] && cache_flag="--no-cache"

  # Find working buildkitd socket
  local addr=""
  for sock in /run/buildkit/buildkitd.sock /run/buildkit-default/buildkitd.sock /run/buildkit-k8s.io/buildkitd.sock; do
    if [ -S "$sock" ]; then
      addr="unix://$sock"
      break
    fi
  done
  if [ -z "$addr" ]; then
    log_err "No buildkitd socket found for buildctl"
    return 1
  fi

  log_info "Building $name:$tag with buildctl (addr=$addr)..."
  mkdir -p "$EXPORT_DIR"
  local tarball="$EXPORT_DIR/${name//\//_}-${tag}.tar"

  buildctl --addr "$addr" build \
    --frontend dockerfile.v0 \
    --local context="$PROJECT_ROOT" \
    --local dockerfile="$(dirname "$dockerfile")" \
    --opt filename="$(basename "$dockerfile")" \
    --opt build-arg:FORTUNA_BUILD_VERSION="$VERSION" \
    --opt build-arg:FORTUNA_BUILD_COMMIT="$COMMIT" \
    --opt build-arg:FORTUNA_BUILD_TIME="$BUILD_TIME" \
    --output type=oci,name="$name:$tag" > "$tarball"

  # Import into containerd
  if [ -n "$CTR_BIN" ]; then
    log_info "Importing $name:$tag into containerd from tarball..."
    $CTR_BIN -n "$CONTAINERD_NS" images import "$tarball"
    # Also tag as :latest
    $CTR_BIN -n "$CONTAINERD_NS" images tag "docker.io/library/$name:$tag" "docker.io/library/$name:latest" 2>/dev/null || \
      $CTR_BIN -n "$CONTAINERD_NS" images tag "$name:$tag" "$name:latest" 2>/dev/null || true
    log_ok "$name imported into containerd"
  else
    log_warn "ctr not found; tarball saved at $tarball but not imported"
  fi
}

# =============================================================================
# Unified build dispatcher
# =============================================================================
build_image() {
  local name="$1" dockerfile="$2" tag="$3"
  case "$TOOL" in
    nerdctl)  build_nerdctl  "$name" "$dockerfile" "$tag" ;;
    docker)   build_docker   "$name" "$dockerfile" "$tag" ;;
    buildctl) build_buildctl "$name" "$dockerfile" "$tag" ;;
  esac
}

# =============================================================================
# Main
# =============================================================================
echo "=========================================="
echo "FortunaK8s Image Builder"
echo "=========================================="
echo "  Tool:      $TOOL"
echo "  Version:   $VERSION"
echo "  Commit:    $COMMIT"
echo "  No cache:  $NO_CACHE"
echo "  Skip dash: $SKIP_DASHBOARD"
echo "  Core only: $BUILD_CORE_ONLY"
echo "  Agent only: $BUILD_AGENT_ONLY"
echo ""

ERRORS=0

# ---- Build Core ----
if [ "$BUILD_AGENT_ONLY" != "true" ] && [ "$BUILD_DASHBOARD_ONLY" != "true" ]; then
  log_info "Building fortuna-core:${VERSION} (commit=${COMMIT})..."
  if build_image "fortuna-core" "$PROJECT_ROOT/core/Dockerfile" "$VERSION"; then
    log_ok "fortuna-core build succeeded"
  else
    log_err "fortuna-core build failed"
    ERRORS=$((ERRORS + 1))
  fi
  echo ""
fi

# ---- Build Agent ----
if [ "$BUILD_CORE_ONLY" != "true" ] && [ "$BUILD_DASHBOARD_ONLY" != "true" ]; then
  log_info "Building fortuna-agent:${VERSION} (commit=${COMMIT})..."
  if build_image "fortuna-agent" "$PROJECT_ROOT/agent/Dockerfile" "$VERSION"; then
    log_ok "fortuna-agent build succeeded"
  else
    log_err "fortuna-agent build failed"
    ERRORS=$((ERRORS + 1))
  fi
  echo ""
fi

# ---- Build Dashboard ----
if [ "$SKIP_DASHBOARD" != "true" ] && [ "$BUILD_CORE_ONLY" != "true" ] && [ "$BUILD_AGENT_ONLY" != "true" ] || [ "$BUILD_DASHBOARD_ONLY" = "true" ]; then
  log_info "Building fortuna-dashboard:${VERSION}..."
  if build_image "fortuna-dashboard" "$PROJECT_ROOT/dashboard/Dockerfile" "$VERSION"; then
    log_ok "fortuna-dashboard build succeeded"
  else
    log_err "fortuna-dashboard build failed"
    ERRORS=$((ERRORS + 1))
  fi
  echo ""
fi

# ---- Export tarballs (optional) ----
if [ "$EXPORT_IMAGES" = "true" ]; then
  mkdir -p "$EXPORT_DIR"
  log_info "Exporting images to $EXPORT_DIR..."
  case "$TOOL" in
    nerdctl)
      [ "$BUILD_AGENT_ONLY" != "true" ] && nerdctl --namespace "$CONTAINERD_NS" save -o "$EXPORT_DIR/fortuna-core-${VERSION}.tar" "fortuna-core:${VERSION}" 2>/dev/null || true
      [ "$BUILD_CORE_ONLY" != "true" ] && nerdctl --namespace "$CONTAINERD_NS" save -o "$EXPORT_DIR/fortuna-agent-${VERSION}.tar" "fortuna-agent:${VERSION}" 2>/dev/null || true
      [ "$SKIP_DASHBOARD" != "true" ] && [ "$BUILD_CORE_ONLY" != "true" ] && [ "$BUILD_AGENT_ONLY" != "true" ] && \
        nerdctl --namespace "$CONTAINERD_NS" save -o "$EXPORT_DIR/fortuna-dashboard-${VERSION}.tar" "fortuna-dashboard:${VERSION}" 2>/dev/null || true
      ;;
    docker)
      [ "$BUILD_AGENT_ONLY" != "true" ] && docker save -o "$EXPORT_DIR/fortuna-core-${VERSION}.tar" "fortuna-core:${VERSION}" 2>/dev/null || true
      [ "$BUILD_CORE_ONLY" != "true" ] && docker save -o "$EXPORT_DIR/fortuna-agent-${VERSION}.tar" "fortuna-agent:${VERSION}" 2>/dev/null || true
      [ "$SKIP_DASHBOARD" != "true" ] && [ "$BUILD_CORE_ONLY" != "true" ] && [ "$BUILD_AGENT_ONLY" != "true" ] && \
        docker save -o "$EXPORT_DIR/fortuna-dashboard-${VERSION}.tar" "fortuna-dashboard:${VERSION}" 2>/dev/null || true
      ;;
    buildctl)
      log_info "buildctl images already exported during build phase"
      ;;
  esac
  log_ok "Export done: $EXPORT_DIR"
fi

# ---- Summary ----
echo "=========================================="
if [ "$ERRORS" -eq 0 ]; then
  log_ok "All builds succeeded (tag: $VERSION)"
else
  log_err "$ERRORS build(s) failed"
fi

# List built images
log_info "Images in containerd (namespace=$CONTAINERD_NS):"
if [ "$TOOL" = "nerdctl" ]; then
  nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep -E "fortuna-(core|agent|dashboard)" || true
elif [ -n "$CTR_BIN" ]; then
  $CTR_BIN -n "$CONTAINERD_NS" images list 2>/dev/null | grep -E "fortuna-(core|agent|dashboard)" || true
fi
echo "=========================================="

exit "$ERRORS"

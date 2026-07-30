#!/usr/bin/env bash
# =============================================================================
# Build Fortuna images (Core, Agent, Dashboard) and load into containerd
# =============================================================================
# Supports three build backends (auto-detected in order):
#   1. nerdctl  – builds directly into containerd (requires buildkitd running)
#   2. buildctl – talks to buildkitd (unix socket, implicit default, or after AUTO_START_BUILDKIT)
#   3. docker   – requires docker-buildx-plugin (DOCKER_BUILDKIT=1); else skipped in auto-detect
#
# Override auto-detection: BUILD_TOOL=docker | BUILD_TOOL=nerdctl | BUILD_TOOL=buildctl
#
# Environment variables:
#   BUILD_TOOL          – Force build backend: nerdctl, docker, buildctl (default: auto-detect)
#   CONTAINERD_NAMESPACE – containerd namespace (default: k8s.io)
#   VERSION             – Image tag (default: git describe without dirty suffix, or commit/latest)
#   NO_CACHE            – Set to 'true' to build without cache (default: false)
#   SKIP_DASHBOARD      – Set to 'true' to skip dashboard build (default: false)
#   BUILD_CORE_ONLY     – Set to 'true' to build only Core (default: false)
#   BUILD_AGENT_ONLY    – Set to 'true' to build only Agent (default: false)
#   BUILD_DASHBOARD_ONLY – Set to 'true' to build only Dashboard (default: false)
#   EXPORT_IMAGES       – Set to 'true' to export .tar after build (default: false)
#   EXPORT_DIR          – Directory for exported tarballs (default: /tmp/fortuna-images)
#   REGISTRY            – Optional registry prefix for push mode, e.g. ghcr.io/org/repo
#   PUSH_IMAGES         – Set to 'true' to tag and push built images to REGISTRY (Docker or nerdctl backend)
#   ALLOW_BUILD_TOOL_FALLBACK – If true (default), BUILD_TOOL=nerdctl with no buildkit tries buildctl then docker+buildx.
#   PARALLEL_FORTUNA_BUILDS – Set to 1 to build fortuna-core and fortuna-agent concurrently (default: 0).
#   AUTO_START_BUILDKIT – If 1 (default), try systemctl start buildkit when buildctl has no socket yet.
#   GOPROXY / GOSUMDB   – Inherited by `go` inside Docker/nerdctl builds (default from builder image / host).
#
# Usage:
#   ./scripts/build/build-and-load-containerd.sh
#   BUILD_TOOL=docker ./scripts/build/build-and-load-containerd.sh
#   SKIP_DASHBOARD=true NO_CACHE=true ./scripts/build/build-and-load-containerd.sh
#   BUILD_CORE_ONLY=true ./scripts/build/build-and-load-containerd.sh
#   BUILD_DASHBOARD_ONLY=true ./scripts/build/build-and-load-containerd.sh
#   BUILD_TOOL=docker REGISTRY=ghcr.io/org/repo PUSH_IMAGES=true ./scripts/build/build-and-load-containerd.sh
# =============================================================================

set -euo pipefail

# Menu/cron/nohup sometimes start with a tiny PATH; ctr/nerdctl usually live under /usr/bin.
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"

# Docker Engine: BuildKit improves layer parallelism and cache metadata (Dockerfiles set BUILDKIT_INLINE_CACHE).
# Override with DOCKER_BUILDKIT=0 if an old daemon breaks on BuildKit.
export DOCKER_BUILDKIT="${DOCKER_BUILDKIT:-1}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ---- Configuration ----
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
VERSION="${VERSION:-$(cd "$PROJECT_ROOT" && git describe --tags --always 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo 'latest')}"
COMMIT="${COMMIT:-$(cd "$PROJECT_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
NO_CACHE="${NO_CACHE:-false}"
SKIP_DASHBOARD="${SKIP_DASHBOARD:-false}"
BUILD_CORE_ONLY="${BUILD_CORE_ONLY:-false}"
BUILD_AGENT_ONLY="${BUILD_AGENT_ONLY:-false}"
BUILD_DASHBOARD_ONLY="${BUILD_DASHBOARD_ONLY:-false}"
EXPORT_IMAGES="${EXPORT_IMAGES:-false}"
EXPORT_DIR="${EXPORT_DIR:-/tmp/fortuna-images}"
SKIP_CONTAINERD_IMPORT="${SKIP_CONTAINERD_IMPORT:-false}"
REGISTRY="${REGISTRY:-}"
PUSH_IMAGES="${PUSH_IMAGES:-false}"
ALLOW_BUILD_TOOL_FALLBACK="${ALLOW_BUILD_TOOL_FALLBACK:-true}"
PARALLEL_FORTUNA_BUILDS="${PARALLEL_FORTUNA_BUILDS:-0}"
AUTO_START_BUILDKIT="${AUTO_START_BUILDKIT:-1}"

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

# nerdctl build requires a running buildkitd (see moby/buildkit).
_nerdctl_bin_for_probe() {
  if command -v nerdctl &>/dev/null; then
    echo "nerdctl"
    return 0
  fi
  for p in /usr/bin/nerdctl /usr/local/bin/nerdctl /opt/nerdctl/nerdctl; do
    if [ -x "$p" ]; then
      echo "$p"
      return 0
    fi
  done
  echo ""
}

# True if BuildKit is listening (fast — avoids a full "FROM scratch" nerdctl build on every pipeline run).
_buildkit_workers_ok() {
  command -v buildctl &>/dev/null || return 1
  local sock
  for sock in /run/buildkit/buildkitd.sock /run/buildkit-default/buildkitd.sock /run/buildkit-k8s.io/buildkitd.sock; do
    if [ -S "$sock" ] && buildctl --addr "unix://$sock" debug workers &>/dev/null 2>&1; then
      return 0
    fi
  done
  return 1
}

nerdctl_buildkit_available() {
  local nc ns="${CONTAINERD_NS:-${CONTAINERD_NAMESPACE:-k8s.io}}"
  nc="$(_nerdctl_bin_for_probe)"
  [ -z "$nc" ] && return 1
  "$nc" --namespace "$ns" build --help &>/dev/null 2>&1 || return 1
  if _buildkit_workers_ok; then
    return 0
  fi
  # No buildctl / no visible socket: verify nerdctl can reach buildkit (slow path; cap when GNU timeout exists).
  if command -v timeout &>/dev/null; then
    if timeout 55 env "NERDCTL_PROBE=$nc" "NS=$ns" bash -c 'echo "FROM scratch" | "$NERDCTL_PROBE" --namespace "$NS" build --no-cache -t __fortuna_buildkit_probe__ -' &>/dev/null 2>&1; then
      "$nc" --namespace "$ns" rmi __fortuna_buildkit_probe__ &>/dev/null 2>&1 || true
      return 0
    fi
    return 1
  fi
  if ! echo "FROM scratch" | "$nc" --namespace "$ns" build --no-cache -t __fortuna_buildkit_probe__ - &>/dev/null 2>&1; then
    return 1
  fi
  "$nc" --namespace "$ns" rmi __fortuna_buildkit_probe__ &>/dev/null 2>&1 || true
  return 0
}

# Resolve how buildctl can reach buildkitd. Prints unix:// path or "default" (buildctl's implicit connection).
find_buildctl_connection() {
  command -v buildctl &>/dev/null || return 1
  local sock
  for sock in /run/buildkit/buildkitd.sock /run/buildkit-default/buildkitd.sock /run/buildkit-k8s.io/buildkitd.sock /var/run/buildkit/buildkitd.sock; do
    if [ -S "$sock" ] && buildctl --addr "unix://$sock" debug workers &>/dev/null 2>&1; then
      echo "unix://$sock"
      return 0
    fi
  done
  if buildctl debug workers &>/dev/null 2>&1; then
    echo "default"
    return 0
  fi
  return 1
}

# Best-effort start of root buildkitd (common on lab nodes) so buildctl can run without docker-buildx.
_maybe_start_buildkit_service() {
  [ "${AUTO_START_BUILDKIT:-1}" != "1" ] && return 0
  find_buildctl_connection &>/dev/null && return 0
  command -v systemctl &>/dev/null || return 0
  if [ "$(id -u)" = "0" ]; then
    systemctl start buildkit 2>/dev/null || true
  elif command -v sudo &>/dev/null && sudo -n true 2>/dev/null; then
    sudo -n systemctl start buildkit 2>/dev/null || true
  fi
  sleep 1
}

# =============================================================================
# Build tool detection
# =============================================================================
detect_build_tool() {
  if [ -n "${BUILD_TOOL:-}" ]; then
    case "$BUILD_TOOL" in
      docker)
        if docker info &>/dev/null 2>&1 && docker buildx version &>/dev/null 2>&1; then
          echo "docker"
          return 0
        fi
        log_err "BUILD_TOOL=docker but Docker is unreachable or docker-buildx-plugin is missing (docker buildx). Install buildx or use BUILD_TOOL=buildctl." >&2
        exit 1
        ;;
      buildctl)
        _maybe_start_buildkit_service
        if find_buildctl_connection &>/dev/null; then
          echo "buildctl"
          return 0
        fi
        log_err "BUILD_TOOL=buildctl but buildctl cannot reach buildkitd (try: systemctl start buildkit)." >&2
        exit 1
        ;;
      nerdctl)
        if nerdctl_buildkit_available; then
          echo "nerdctl"
          return 0
        fi
        log_warn "BUILD_TOOL=nerdctl but buildkitd is not reachable (no working unix socket)." >&2
        if [ "$ALLOW_BUILD_TOOL_FALLBACK" = "true" ]; then
          _maybe_start_buildkit_service
          if find_buildctl_connection &>/dev/null; then
            log_warn "Falling back to buildctl." >&2
            echo "buildctl"
            return 0
          fi
          if command -v docker &>/dev/null && docker info &>/dev/null 2>&1 && docker buildx version &>/dev/null 2>&1; then
            log_warn "Falling back to docker for this run." >&2
            echo "docker"
            return 0
          fi
        fi
        log_err "Install/start buildkitd (nerdctl needs it), or use Docker with buildx, or BUILD_TOOL=buildctl ..." >&2
        log_err "See https://github.com/moby/buildkit and https://github.com/containerd/nerdctl/blob/main/docs/build.md" >&2
        exit 1
        ;;
      *)
        log_err "Invalid BUILD_TOOL=$BUILD_TOOL (use: nerdctl, docker, buildctl)"
        exit 1
        ;;
    esac
  fi

  # Proactively start root buildkitd when idle (speeds nerdctl probe and enables buildctl fallback).
  _maybe_start_buildkit_service

  # Auto-detect: nerdctl → buildctl (buildkit) → docker (requires buildx for our Dockerfiles)
  if nerdctl_buildkit_available; then
    echo "nerdctl"
    return 0
  fi
  if command -v nerdctl &>/dev/null || [ -x /usr/bin/nerdctl ] || [ -x /usr/local/bin/nerdctl ]; then
    log_warn "nerdctl found but buildkitd not running; trying buildctl / docker..." >&2
  fi

  if command -v buildctl &>/dev/null && find_buildctl_connection &>/dev/null; then
    echo "buildctl"
    return 0
  fi

  if command -v docker &>/dev/null && docker info &>/dev/null 2>&1; then
    if docker buildx version &>/dev/null 2>&1; then
      echo "docker"
      return 0
    fi
    log_warn "docker is running but docker buildx is missing or broken; install docker-buildx-plugin (https://docs.docker.com/go/buildx/)." >&2
  elif command -v docker &>/dev/null; then
    log_warn "docker found but daemon not running" >&2
  fi

  log_err "No working build tool found. Install one of:" >&2
  log_err "  1. buildkitd + buildctl (e.g. apt install buildkit; systemctl start buildkit)" >&2
  log_err "  2. Docker Engine + docker-buildx-plugin (BuildKit features in Dockerfiles)" >&2
  log_err "  3. nerdctl + buildkitd: https://github.com/containerd/nerdctl" >&2
  exit 1
}

TOOL="$(detect_build_tool)"

# =============================================================================
# Helper: find ctr binary
# =============================================================================
find_ctr() {
  if command -v ctr &>/dev/null; then
    echo "ctr"
    return 0
  fi
  for p in /usr/bin/ctr /usr/local/bin/ctr /opt/containerd/bin/ctr; do
    if [ -x "$p" ]; then
      echo "$p"
      return 0
    fi
  done
  echo ""
}

CTR_BIN="$(find_ctr)"

find_nerdctl() {
  if command -v nerdctl &>/dev/null; then
    echo "nerdctl"
    return 0
  fi
  for p in /usr/bin/nerdctl /usr/local/bin/nerdctl /opt/nerdctl/nerdctl; do
    if [ -x "$p" ]; then
      echo "$p"
      return 0
    fi
  done
  echo ""
}

NERDCTL_BIN="$(find_nerdctl)"

# detect_build_tool already validated nerdctl + buildkit; do not re-run an expensive probe here.

log_info "Build tool: $TOOL"

# Lines from ctr + nerdctl for namespace (same view pipeline should use for checks).
fortuna_image_list_text() {
  local ns="${1:-$CONTAINERD_NS}"
  local out="" c n
  c="$(find_ctr)"
  n="$(find_nerdctl)"
  if [ -n "$c" ]; then
    out="$("$c" -n "$ns" images list 2>/dev/null || true)"
  fi
  if [ -n "$n" ]; then
    out="${out}
$("$n" --namespace "$ns" images 2>/dev/null || true)"
  fi
  printf '%s\n' "$out"
}

verify_image_listed_in_containerd() {
  local name="$1" ns="${2:-$CONTAINERD_NS}" i
  # ctr metadata can lag briefly after import; avoid false failures.
  for i in $(seq 1 40); do
    if fortuna_image_list_text "$ns" | grep -Fq -- "$name"; then
      return 0
    fi
    sleep 0.15
  done
  return 1
}

# =============================================================================
# Build functions per tool
# =============================================================================

# Build args common to all tools
build_args() {
  echo "--build-arg FORTUNA_BUILD_VERSION=${VERSION}"
  echo "--build-arg FORTUNA_BUILD_COMMIT=${COMMIT}"
  echo "--build-arg FORTUNA_BUILD_TIME=${BUILD_TIME}"
  echo "--build-arg BUILDKIT_INLINE_CACHE=1"
}

# --- nerdctl build ---
build_nerdctl() {
  local name="$1" dockerfile="$2" tag="$3"
  local cache_flag=""
  [ "$NO_CACHE" = "true" ] && cache_flag="--no-cache"
  log_info "Building $name:$tag with nerdctl (namespace=$CONTAINERD_NS)..."
  local nerdcli="${NERDCTL_BIN:-nerdctl}"
  "$nerdcli" --namespace "$CONTAINERD_NS" build \
    -f "$dockerfile" \
    -t "$name:$tag" \
    -t "$name:latest" \
    $(build_args) \
    $cache_flag \
    "$PROJECT_ROOT"

  if ! verify_image_listed_in_containerd "$name"; then
    log_err "nerdctl build finished but $name not listed in containerd (namespace=$CONTAINERD_NS)."
    return 1
  fi
}

# --- docker build + import to containerd ---
build_docker() {
  local name="$1" dockerfile="$2" tag="$3"
  local cache_flag=""
  [ "$NO_CACHE" = "true" ] && cache_flag="--no-cache"
  log_info "Building $name:$tag with docker..."
  if ! docker build \
    -f "$dockerfile" \
    -t "$name:$tag" \
    -t "$name:latest" \
    $(build_args) \
    $cache_flag \
    "$PROJECT_ROOT"; then
    log_err "docker build failed for $name (see errors above)."
    return 1
  fi

  # Import into containerd so kubelet (containerd runtime) can find the image
  if [ -n "$CTR_BIN" ]; then
    log_info "Importing $name:$tag into containerd (namespace=$CONTAINERD_NS)..."
    docker save "$name:$tag" "$name:latest" | $CTR_BIN -n "$CONTAINERD_NS" images import -
    log_ok "$name imported into containerd"
  elif [ -n "$NERDCTL_BIN" ]; then
    # Docker graph → same containerd namespace kubelet uses (no usable ctr)
    log_info "Importing $name:$tag via nerdctl load (namespace=$CONTAINERD_NS)..."
    docker save "$name:$tag" "$name:latest" | "$NERDCTL_BIN" --namespace "$CONTAINERD_NS" load
    log_ok "$name loaded into containerd via nerdctl"
  elif [ "$SKIP_CONTAINERD_IMPORT" = "true" ]; then
    log_warn "containerd import skipped (SKIP_CONTAINERD_IMPORT=true) — image is Docker-only (not loaded into $CONTAINERD_NS)."
    return 0
  else
    log_err "Neither ctr nor nerdctl found; cannot load $name into containerd namespace $CONTAINERD_NS."
    log_err "Install ctr, install nerdctl, or use BUILD_TOOL=nerdctl with buildkitd."
    return 1
  fi

  if ! verify_image_listed_in_containerd "$name"; then
    if sudo -n true 2>/dev/null && { [ -x /usr/bin/ctr ] || [ -x /usr/local/bin/ctr ]; }; then
      local sctr=""
      for sctr in /usr/bin/ctr /usr/local/bin/ctr; do
        [ -x "$sctr" ] || continue
        log_warn "$name not visible after import; retrying with sudo $sctr import..."
        docker save "$name:$tag" "$name:latest" | sudo -n "$sctr" -n "$CONTAINERD_NS" images import - || true
        break
      done
    fi
  fi
  if ! verify_image_listed_in_containerd "$name"; then
    log_err "$name is not visible in containerd (namespace=$CONTAINERD_NS) after docker build + import."
    log_err "Check: $(find_ctr 2>/dev/null || true) -n $CONTAINERD_NS images list | grep $name  ;  $(find_nerdctl 2>/dev/null || true) --namespace $CONTAINERD_NS images | grep $name"
    log_err "If import used a different containerd than kubelet, align CONTAINERD_NAMESPACE or use BUILD_TOOL=nerdctl."
    return 1
  fi
}

# --- buildctl build ---
build_buildctl() {
  local name="$1" dockerfile="$2" tag="$3"

  local addr
  addr="$(find_buildctl_connection)" || {
    log_err "No buildkitd reachable for buildctl (try: systemctl start buildkit; or install docker-buildx-plugin)."
    return 1
  }

  if [ "$addr" = "default" ]; then
    log_info "Building $name:$tag with buildctl (implicit BuildKit address)..."
  else
    log_info "Building $name:$tag with buildctl (addr=$addr)..."
  fi
  mkdir -p "$EXPORT_DIR"
  local tarball="$EXPORT_DIR/${name//\//_}-${tag}.tar"

  local bcmd=(buildctl)
  [ "$addr" != "default" ] && bcmd+=(--addr "$addr")
  [ "$NO_CACHE" = "true" ] && bcmd+=(--no-cache)
  bcmd+=(build
    --frontend dockerfile.v0
    --local context="$PROJECT_ROOT"
    --local dockerfile="$(dirname "$dockerfile")"
    --opt filename="$(basename "$dockerfile")"
    --opt build-arg:FORTUNA_BUILD_VERSION="$VERSION"
    --opt build-arg:FORTUNA_BUILD_COMMIT="$COMMIT"
    --opt build-arg:FORTUNA_BUILD_TIME="$BUILD_TIME"
    --opt build-arg:BUILDKIT_INLINE_CACHE=1
    --output type=oci,name="$name:$tag")
  "${bcmd[@]}" > "$tarball"

  # Import into containerd
  if [ -n "$CTR_BIN" ]; then
    log_info "Importing $name:$tag into containerd from tarball..."
    $CTR_BIN -n "$CONTAINERD_NS" images import "$tarball"
    # Also tag as :latest
    $CTR_BIN -n "$CONTAINERD_NS" images tag "docker.io/library/$name:$tag" "docker.io/library/$name:latest" 2>/dev/null || \
      $CTR_BIN -n "$CONTAINERD_NS" images tag "$name:$tag" "$name:latest" 2>/dev/null || true
    log_ok "$name imported into containerd"
  elif [ "$SKIP_CONTAINERD_IMPORT" = "true" ]; then
    log_warn "ctr not found; tarball saved at $tarball (not imported; SKIP_CONTAINERD_IMPORT=true)."
    return 0
  else
    log_err "ctr not found; cannot import $tarball into containerd namespace $CONTAINERD_NS."
    log_err "Install containerd's ctr or use BUILD_TOOL=nerdctl with buildkitd."
    return 1
  fi

  if ! verify_image_listed_in_containerd "$name"; then
    log_err "$name not listed in containerd ($CONTAINERD_NS) after buildctl import."
    return 1
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
    *)
      log_err "Unknown TOOL='$TOOL' (expected nerdctl|docker|buildctl). Fix detect_build_tool / BUILD_TOOL."
      return 1
      ;;
  esac
}

component_selected() {
  local name="$1"
  case "$name" in
    fortuna-core)
      [ "$BUILD_AGENT_ONLY" != "true" ] && [ "$BUILD_DASHBOARD_ONLY" != "true" ]
      ;;
    fortuna-agent)
      [ "$BUILD_CORE_ONLY" != "true" ] && [ "$BUILD_DASHBOARD_ONLY" != "true" ]
      ;;
    fortuna-dashboard)
      { [ "$SKIP_DASHBOARD" != "true" ] && [ "$BUILD_CORE_ONLY" != "true" ] && [ "$BUILD_AGENT_ONLY" != "true" ]; } || [ "$BUILD_DASHBOARD_ONLY" = "true" ]
      ;;
    *)
      return 1
      ;;
  esac
}

push_image() {
  local name="$1"
  [ "$PUSH_IMAGES" = "true" ] || return 0
  if [ -z "$REGISTRY" ]; then
    log_err "PUSH_IMAGES=true requires REGISTRY, e.g. REGISTRY=ghcr.io/org/repo."
    return 1
  fi
  if ! component_selected "$name"; then
    return 0
  fi

  local remote="${REGISTRY%/}/$name"
  case "$TOOL" in
    docker)
      log_info "Tagging and pushing $remote:$VERSION with docker..."
      docker tag "$name:$VERSION" "$remote:$VERSION"
      docker tag "$name:latest" "$remote:latest"
      docker push "$remote:$VERSION"
      docker push "$remote:latest"
      ;;
    nerdctl)
      local nerdcli="${NERDCTL_BIN:-nerdctl}"
      log_info "Tagging and pushing $remote:$VERSION with nerdctl (namespace=$CONTAINERD_NS)..."
      "$nerdcli" --namespace "$CONTAINERD_NS" tag "$name:$VERSION" "$remote:$VERSION"
      "$nerdcli" --namespace "$CONTAINERD_NS" tag "$name:latest" "$remote:latest"
      "$nerdcli" --namespace "$CONTAINERD_NS" push "$remote:$VERSION"
      "$nerdcli" --namespace "$CONTAINERD_NS" push "$remote:latest"
      ;;
    buildctl)
      log_err "PUSH_IMAGES=true is not supported with BUILD_TOOL=buildctl in this script. Use BUILD_TOOL=docker or BUILD_TOOL=nerdctl."
      return 1
      ;;
    *)
      log_err "Unknown build tool for push: $TOOL"
      return 1
      ;;
  esac
}

# =============================================================================
# Main
# =============================================================================
echo "=========================================="
echo "Fortuna Image Builder"
echo "=========================================="
echo "  Tool:      $TOOL"
echo "  Version:   $VERSION"
echo "  Commit:    $COMMIT"
echo "  No cache:  $NO_CACHE"
echo "  Skip dash: $SKIP_DASHBOARD"
echo "  Core only: $BUILD_CORE_ONLY"
echo "  Agent only: $BUILD_AGENT_ONLY"
echo "  Dashboard only: $BUILD_DASHBOARD_ONLY"
echo "  Registry:  ${REGISTRY:-<none>}"
echo "  Push:      $PUSH_IMAGES"
echo "  Parallel Go images: $PARALLEL_FORTUNA_BUILDS (1=core+agent concurrent)"
echo ""

ERRORS=0

# ---- Build Core + Agent (optional parallel) ----
_go_parallel=false
if [ "$PARALLEL_FORTUNA_BUILDS" = "1" ] && [ "$BUILD_CORE_ONLY" != "true" ] && [ "$BUILD_AGENT_ONLY" != "true" ] && [ "$BUILD_DASHBOARD_ONLY" != "true" ]; then
  _go_parallel=true
fi

if [ "$_go_parallel" = true ]; then
  log_info "Building fortuna-core + fortuna-agent in parallel (PARALLEL_FORTUNA_BUILDS=1)..."
  _rc_core=0
  _rc_agent=0
  build_image "fortuna-core" "$PROJECT_ROOT/core/Dockerfile" "$VERSION" &
  _pid_core=$!
  build_image "fortuna-agent" "$PROJECT_ROOT/agent/Dockerfile" "$VERSION" &
  _pid_agent=$!
  wait "$_pid_core" || _rc_core=$?
  wait "$_pid_agent" || _rc_agent=$?
  if [ "$_rc_core" -eq 0 ]; then
    log_ok "fortuna-core build succeeded"
  else
    log_err "fortuna-core build failed"
    ERRORS=$((ERRORS + 1))
  fi
  if [ "$_rc_agent" -eq 0 ]; then
    log_ok "fortuna-agent build succeeded"
  else
    log_err "fortuna-agent build failed"
    ERRORS=$((ERRORS + 1))
  fi
  echo ""
else
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
fi

# ---- Build Dashboard ----
if { [ "$SKIP_DASHBOARD" != "true" ] && [ "$BUILD_CORE_ONLY" != "true" ] && [ "$BUILD_AGENT_ONLY" != "true" ]; } || [ "$BUILD_DASHBOARD_ONLY" = "true" ]; then
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
      component_selected fortuna-core && nerdctl --namespace "$CONTAINERD_NS" save -o "$EXPORT_DIR/fortuna-core-${VERSION}.tar" "fortuna-core:${VERSION}" 2>/dev/null || true
      component_selected fortuna-agent && nerdctl --namespace "$CONTAINERD_NS" save -o "$EXPORT_DIR/fortuna-agent-${VERSION}.tar" "fortuna-agent:${VERSION}" 2>/dev/null || true
      component_selected fortuna-dashboard && nerdctl --namespace "$CONTAINERD_NS" save -o "$EXPORT_DIR/fortuna-dashboard-${VERSION}.tar" "fortuna-dashboard:${VERSION}" 2>/dev/null || true
      ;;
    docker)
      component_selected fortuna-core && docker save -o "$EXPORT_DIR/fortuna-core-${VERSION}.tar" "fortuna-core:${VERSION}" 2>/dev/null || true
      component_selected fortuna-agent && docker save -o "$EXPORT_DIR/fortuna-agent-${VERSION}.tar" "fortuna-agent:${VERSION}" 2>/dev/null || true
      component_selected fortuna-dashboard && docker save -o "$EXPORT_DIR/fortuna-dashboard-${VERSION}.tar" "fortuna-dashboard:${VERSION}" 2>/dev/null || true
      ;;
    buildctl)
      log_info "buildctl images already exported during build phase"
      ;;
  esac
  log_ok "Export done: $EXPORT_DIR"
fi

# ---- Registry push (optional) ----
if [ "$PUSH_IMAGES" = "true" ] && [ "$ERRORS" -eq 0 ]; then
  for image in fortuna-core fortuna-agent fortuna-dashboard; do
    component_selected "$image" || continue
    if push_image "$image"; then
      log_ok "Push complete for $image"
    else
      log_err "Push failed for $image"
      ERRORS=$((ERRORS + 1))
    fi
  done
elif [ "$PUSH_IMAGES" = "true" ]; then
  log_warn "Skipping registry push because one or more builds failed."
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
fortuna_image_list_text "$CONTAINERD_NS" | grep -E "fortuna-(core|agent|dashboard)" || true
echo "=========================================="

exit "$ERRORS"

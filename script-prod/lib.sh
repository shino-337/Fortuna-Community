#!/usr/bin/env bash
# =============================================================================
# Fortuna script-prod – common functions and defaults
# Source this in other scripts: source "$SCRIPT_DIR/lib.sh"
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
DEPLOY_DIR="$PROJECT_ROOT/deploy"
SCRIPTS_LEGACY="$PROJECT_ROOT/scripts"

# Load config if present
if [ -f "$SCRIPT_DIR/config.env" ]; then
  # shellcheck source=config.env.example
  set -a
  source "$SCRIPT_DIR/config.env"
  set +a
fi

VERSION="${VERSION:-latest}"
NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
REGISTRY="${REGISTRY:-}"
LOG_DIR="${LOG_DIR:-$SCRIPT_DIR/logs}"
PUSH_IMAGES="${PUSH_IMAGES:-0}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
log_ok()      { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_err()     { echo -e "${RED}[ERR]${NC} $*"; }

# Ensure kubectl available and cluster reachable
require_kubectl() {
  if ! command -v kubectl &>/dev/null; then
    log_err "kubectl not found. Install kubectl and ensure it is in PATH."
    exit 1
  fi
  if ! kubectl cluster-info &>/dev/null; then
    log_err "kubectl cannot reach cluster. Check KUBECONFIG."
    exit 1
  fi
}

# Ensure log directory exists
ensure_log_dir() {
  mkdir -p "$LOG_DIR"
}

# Resolve image name (with optional registry)
image_name() {
  local component="$1"
  if [ -n "$REGISTRY" ]; then
    echo "${REGISTRY}/${component}:${VERSION}"
  else
    echo "${component}:${VERSION}"
  fi
}

# Detect container runtime for build (nerdctl preferred for k8s.io)
detect_build_cmd() {
  if command -v nerdctl &>/dev/null; then
    echo "nerdctl"
    return
  fi
  if command -v docker &>/dev/null; then
    echo "docker"
    return
  fi
  log_err "Neither nerdctl nor docker found. Install one for building images."
  exit 1
}

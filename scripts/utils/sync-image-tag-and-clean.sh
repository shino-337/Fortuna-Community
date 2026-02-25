#!/usr/bin/env bash
# ============================================================================
# Sync Fortuna image tag in deploy YAMLs and clean unused images
# ============================================================================
# 1. Detect tag: BUILD_TAG, or git describe --tags --always --dirty, or from
#    deploy/fortuna-core-deployment.yaml, or "latest".
# 2. Update deploy/*.yaml so all use image: fortuna-*:TAG.
# 3. List fortuna images in containerd.
# 4. Clean: remove fortuna images with tag != TAG (keep only TAG); prune.
#
# Usage:
#   ./scripts/utils/sync-image-tag-and-clean.sh              # sync tag + clean
#   ./scripts/utils/sync-image-tag-and-clean.sh --sync-only   # only update YAMLs
#   ./scripts/utils/sync-image-tag-and-clean.sh --clean-only  # only clean (use tag from YAML)
#   BUILD_TAG=v1.2.3 ./scripts/utils/sync-image-tag-and-clean.sh
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"

SYNC_ONLY=false
CLEAN_ONLY=false
for arg in "$@"; do
  case "$arg" in
    --sync-only)  SYNC_ONLY=true ;;
    --clean-only) CLEAN_ONLY=true ;;
  esac
done

# Tag: BUILD_TAG > git describe > from deploy YAML > latest
get_tag_from_yaml() {
  [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ] && \
    grep -E '^\s+image:\s+fortuna-core:' "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" | \
    sed -E 's/.*image:\s+fortuna-core://' | tr -d ' \r' | head -1
}

TAG="${BUILD_TAG:-}"
if [ -z "$TAG" ]; then
  TAG=$(cd "$PROJECT_ROOT" && git describe --tags --always --dirty 2>/dev/null) || true
fi
if [ -z "$TAG" ]; then
  TAG=$(get_tag_from_yaml) || true
fi
TAG="${TAG:-latest}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_ok()      { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }

echo "=========================================="
echo "Sync image tag & clean"
echo "=========================================="
echo "  Tag: $TAG"
echo "  Sync only: $SYNC_ONLY | Clean only: $CLEAN_ONLY"
echo ""

# ---- Sync tag to deploy YAMLs ----
if [ "$CLEAN_ONLY" = false ]; then
  log_info "Syncing image tag ($TAG) in deploy YAMLs..."
  # Use @ as delimiter so tag with hyphens/digits does not break sed
  [ -f "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" ] && \
    sed -i "s@image: fortuna-core:[^[:space:]]*@image: fortuna-core:${TAG}@g" "$PROJECT_ROOT/deploy/fortuna-core-deployment.yaml" && \
    log_ok "fortuna-core-deployment.yaml -> fortuna-core:$TAG"
  [ -f "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" ] && \
    sed -i "s@image: fortuna-agent:[^[:space:]]*@image: fortuna-agent:${TAG}@g" "$PROJECT_ROOT/deploy/fortuna-agent-daemonset.yaml" && \
    log_ok "fortuna-agent-daemonset.yaml -> fortuna-agent:$TAG"
  [ -f "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" ] && \
    sed -i "s@image: fortuna-dashboard:[^[:space:]]*@image: fortuna-dashboard:${TAG}@g" "$PROJECT_ROOT/deploy/dashboard-deployment.yaml" && \
    log_ok "dashboard-deployment.yaml -> fortuna-dashboard:$TAG"
  echo ""
fi

# ---- List current fortuna images ----
log_info "Fortuna images in containerd (namespace=$CONTAINERD_NS):"
nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep -E 'fortuna|REPOSITORY' || echo "  (none or nerdctl not available)"
echo ""

if [ "$SYNC_ONLY" = true ]; then
  log_ok "Sync only done."
  exit 0
fi

# ---- Clean: remove fortuna images with tag != TAG, then prune ----
if [ "$CLEAN_ONLY" = true ]; then
  TAG=$(get_tag_from_yaml) || TAG="${BUILD_TAG:-latest}"
  [ -z "$TAG" ] && TAG="latest"
  log_info "Clean only: using tag from YAML or env: $TAG"
fi

if ! command -v nerdctl &>/dev/null; then
  log_warn "nerdctl not found; skip image clean"
  exit 0
fi

log_info "Removing fortuna images that do NOT match tag=$TAG..."
REMOVED=0
while read -r line; do
  name=$(echo "$line" | awk '{print $1}')
  tag=$(echo "$line" | awk '{print $2}')
  id=$(echo "$line" | awk '{print $3}')
  if [ -z "$id" ] || [ "$id" = "IMAGE" ]; then continue; fi
  if [ "$name" = "fortuna-core" ] || [ "$name" = "fortuna-agent" ] || [ "$name" = "fortuna-dashboard" ]; then
    if [ "$tag" != "$TAG" ]; then
      nerdctl --namespace "$CONTAINERD_NS" rmi --force "${name}:${tag}" 2>/dev/null && { log_ok "Removed ${name}:${tag}"; REMOVED=$((REMOVED+1)); } || true
    fi
  fi
done < <(nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna || true)

log_info "Pruning build cache and unused resources..."
nerdctl --namespace "$CONTAINERD_NS" system prune -f 2>/dev/null || true
nerdctl builder prune --namespace "$CONTAINERD_NS" -a -f 2>/dev/null || true

log_ok "Clean done. Kept fortuna-*:$TAG only."
echo ""
log_info "Current fortuna images:"
nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep -E 'fortuna|REPOSITORY' || true
echo "=========================================="

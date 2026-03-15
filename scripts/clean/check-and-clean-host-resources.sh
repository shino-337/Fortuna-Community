#!/usr/bin/env bash
# =============================================================================
# Check and clean host resources: old images, build cache, images in containerd
# =============================================================================
# - Mặc định: chỉ KIỂM TRA (disk, images trong ctr/nerdctl, cache, temp).
# - Với --clean: xóa fortuna images, prune system + builder, temp dirs.
# - Dùng nerdctl nếu có (namespace k8s.io), fallback ctr.
# =============================================================================
#
# Usage:
#   ./scripts/clean/check-and-clean-host-resources.sh              # check only
#   ./scripts/clean/check-and-clean-host-resources.sh --clean       # check then clean (confirm)
#   ./scripts/clean/check-and-clean-host-resources.sh --clean -y    # check then clean (no confirm)
#   ./scripts/clean/check-and-clean-host-resources.sh --check       # same as default
#
# =============================================================================

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
IMAGE_PREFIX="${IMAGE_PREFIX:-fortuna}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

DO_CLEAN=false
SKIP_CONFIRM=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --clean)   DO_CLEAN=true; shift ;;
    -y|--yes)  SKIP_CONFIRM=true; shift ;;
    --check)   shift ;; # no-op, default is check
    -h|--help)
      head -35 "$SCRIPT_DIR/check-and-clean-host-resources.sh" | tail -n +2
      exit 0
      ;;
    *) echo -e "${RED}Unknown option: $1${NC}" >&2; exit 1 ;;
  esac
done

# ---- Helpers ----
info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERR]${NC} $*"; }

# Detect nerdctl or ctr
NERDCTL=""
if command -v nerdctl &>/dev/null; then
  NERDCTL="nerdctl --namespace $CONTAINERD_NS"
fi
CTR=""
if command -v ctr &>/dev/null && [ -S /run/containerd/containerd.sock ] 2>/dev/null || [ -S /var/run/containerd/containerd.sock ] 2>/dev/null; then
  CTR="ctr -n $CONTAINERD_NS"
fi

# =============================================================================
# 1. CHECK: Disk usage
# =============================================================================
section_check_disk() {
  echo ""
  echo "=========================================="
  echo "1. Disk usage"
  echo "=========================================="
  df -h / 2>/dev/null || df -h . 2>/dev/null
  if [ -d /tmp ]; then
    echo "  /tmp: $(du -sh /tmp 2>/dev/null | cut -f1 || echo '?')"
  fi
  if [ -d /var/lib/containerd ]; then
    echo "  /var/lib/containerd: $(du -sh /var/lib/containerd 2>/dev/null | cut -f1 || echo '?')"
  fi
  for d in /tmp/fortuna-images /var/tmp/fortuna-images "$PROJECT_ROOT/.nerdctl"; do
    [ -d "$d" ] && echo "  $d: $(du -sh "$d" 2>/dev/null | cut -f1 || echo '?')"
  done
}

# =============================================================================
# 2. CHECK: Images in containerd (ctr)
# =============================================================================
section_check_ctr() {
  echo ""
  echo "=========================================="
  echo "2. Images in containerd (namespace=$CONTAINERD_NS)"
  echo "=========================================="
  if [ -n "$CTR" ]; then
    $CTR images ls 2>/dev/null | head -80 || true
    COUNT=$($CTR images ls -q 2>/dev/null | wc -l | tr -d ' ') || true
    COUNT=${COUNT:-0}
    echo "  Total image refs: $COUNT"
    if [ "$COUNT" -gt 0 ] 2>/dev/null; then
      echo "  Fortuna/KSAM refs:"
      $CTR images ls -q 2>/dev/null | grep -E "${IMAGE_PREFIX}|ksam" || true
    fi
  else
    warn "ctr not available or containerd socket not found; skip ctr list"
  fi
}

# =============================================================================
# 3. CHECK: Images via nerdctl (if available)
# =============================================================================
section_check_nerdctl() {
  echo ""
  echo "=========================================="
  echo "3. Images via nerdctl (namespace=$CONTAINERD_NS)"
  echo "=========================================="
  if [ -n "$NERDCTL" ]; then
    $NERDCTL images 2>/dev/null | grep -E "REPOSITORY|${IMAGE_PREFIX}|ksam" || echo "  (none or no nerdctl output)"
    echo ""
    echo "  Build cache: run with --clean to prune (nerdctl builder prune -a -f)"
    $NERDCTL builder prune --namespace "$CONTAINERD_NS" -a --dry-run 2>/dev/null || true
  else
    warn "nerdctl not found; skip nerdctl list"
  fi
}

# =============================================================================
# 4. CHECK: Temp / junk paths
# =============================================================================
section_check_temp() {
  echo ""
  echo "=========================================="
  echo "4. Temp / junk paths"
  echo "=========================================="
  for d in /tmp/fortuna-images /var/tmp/fortuna-images; do
    if [ -d "$d" ]; then
      echo "  $d exists ($(du -sh "$d" 2>/dev/null | cut -f1 || echo '?'))"
    else
      echo "  $d (absent)"
    fi
  done
  [ -d "$PROJECT_ROOT/.nerdctl" ] && echo "  $PROJECT_ROOT/.nerdctl exists" || echo "  .nerdctl (absent)"
}

# =============================================================================
# 5. CLEAN: Remove fortuna images + prune
# =============================================================================
do_clean() {
  if [ "$SKIP_CONFIRM" != "true" ]; then
    read -p "Remove fortuna images, prune system+builder cache, and temp dirs? (y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
      info "Aborted."
      return 1
    fi
  fi

  REMOVED=0

  # 5a. Remove by nerdctl (by tag then by ID)
  if [ -n "$NERDCTL" ]; then
    info "Removing fortuna images via nerdctl (by tag)..."
    for img in fortuna-core:latest fortuna-agent:latest fortuna-dashboard:latest; do
      if $NERDCTL rmi --force "$img" 2>/dev/null; then REMOVED=$((REMOVED+1)); fi
    done
    info "Removing remaining fortuna images by image ID..."
    while read -r id; do
      [ -z "$id" ] || [ "$id" = "ID" ] && continue
      if $NERDCTL rmi --force "$id" 2>/dev/null; then REMOVED=$((REMOVED+1)); fi
    done < <($NERDCTL images 2>/dev/null | grep -E 'fortuna-(core|agent|dashboard)|ksam' | awk '{print $3}' | sort -u)
    info "System prune (dangling, unused)..."
    $NERDCTL system prune -f 2>/dev/null || true
    info "Builder prune (build cache)..."
    $NERDCTL builder prune --namespace "$CONTAINERD_NS" -a -f 2>/dev/null || true
  fi

  # 5b. Remove by ctr (if nerdctl didn't remove all or nerdctl not present)
  if [ -n "$CTR" ]; then
    info "Removing fortuna/ksam image refs via ctr..."
    while read -r ref; do
      [ -z "$ref" ] && continue
      if $CTR images rm "$ref" 2>/dev/null; then REMOVED=$((REMOVED+1)); fi
    done < <($CTR images ls -q 2>/dev/null | grep -E "${IMAGE_PREFIX}|ksam" || true)
  fi

  # 5c. Temp dirs
  info "Removing temp dirs..."
  for d in /tmp/fortuna-images /var/tmp/fortuna-images; do
    if [ -d "$d" ]; then
      rm -rf "$d" && ok "Removed $d" && REMOVED=$((REMOVED+1))
    fi
  done

  ok "Clean done (removed/cleaned $REMOVED items)"
  return 0
}

# =============================================================================
# Main (check sections do not fail script on missing ctr/nerdctl or permission)
# =============================================================================
set +e
echo "=========================================="
echo "Host resources: check and clean"
echo "=========================================="
echo "  Mode:    $([ "$DO_CLEAN" = true ] && echo 'CHECK + CLEAN' || echo 'CHECK ONLY')"
echo "  Namespace: $CONTAINERD_NS"
echo "  Prefix:  $IMAGE_PREFIX"
echo ""

section_check_disk
section_check_ctr
section_check_nerdctl
section_check_temp

if [ "$DO_CLEAN" = true ]; then
  set -e
  echo ""
  echo "=========================================="
  echo "5. Clean"
  echo "=========================================="
  do_clean || exit 1
  echo ""
  info "Re-run without --clean to see state after clean."
fi

echo ""
ok "Done."

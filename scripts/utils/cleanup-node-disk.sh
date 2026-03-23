#!/usr/bin/env bash
# ============================================================================
# Kiểm tra & dọn dẹp đĩa trên node Kubernetes (tránh DiskPressure → Evicted pods)
# ============================================================================
# Chạy TRÊN NODE (SSH vào k8s-master / worker), có quyền sudo.
# Không xóa volume PVC; chỉ: log journal, prune buildkit/nerdctl (tùy chọn).
#
# Usage:
#   ./scripts/utils/cleanup-node-disk.sh --check-only
#   ./scripts/utils/cleanup-node-disk.sh --prune-nerdctl   # nerdctl -n k8s.io system prune -f
#   ./scripts/utils/cleanup-node-disk.sh --journal-vacuum # journalctl --vacuum-time=7d
#   ./scripts/utils/cleanup-node-disk.sh --all            # check + journal + prune (không -a)
#
# Env: NERDCTL_NS=k8s.io (default), PRUNE_ALL=1 để prune -a (nguy hiểm hơn)
# ============================================================================
set -euo pipefail

CHECK_ONLY=false
PRUNE_NERDCTL=false
JOURNAL_VACUUM=false
DO_ALL=false

# Không tham số = chỉ kiểm tra (df/du)
if [ $# -eq 0 ]; then
  CHECK_ONLY=true
fi

for a in "$@"; do
  case "$a" in
    --check-only) CHECK_ONLY=true ;;
    --prune-nerdctl) PRUNE_NERDCTL=true ;;
    --journal-vacuum) JOURNAL_VACUUM=true ;;
    --all) DO_ALL=true ;;
    *) echo "Unknown option: $a"; exit 1 ;;
  esac
done

if [ "$DO_ALL" = true ]; then
  CHECK_ONLY=false
  PRUNE_NERDCTL=true
  JOURNAL_VACUUM=true
fi

if [ "$CHECK_ONLY" = false ] && [ "$PRUNE_NERDCTL" = false ] && [ "$JOURNAL_VACUUM" = false ] && [ "$DO_ALL" = false ]; then
  echo "Usage: $0 [--check-only | --prune-nerdctl | --journal-vacuum | --all]  (mặc định: --check-only nếu không tham số)"
  exit 1
fi

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Disk check (node: $(hostname)) ===${NC}"
df -h / /var 2>/dev/null || df -h

echo ""
echo -e "${BLUE}=== Top disk usage (may be slow) ===${NC}"
if command -v du >/dev/null 2>&1; then
  sudo du -sh /var/lib/containerd /var/lib/kubelet/pods /var/log 2>/dev/null || true
fi

if [ "$CHECK_ONLY" = true ] && [ "$DO_ALL" = false ] && [ "$PRUNE_NERDCTL" = false ] && [ "$JOURNAL_VACUUM" = false ]; then
  echo -e "${GREEN}Done (--check-only).${NC}"
  exit 0
fi

if [ "$JOURNAL_VACUUM" = true ]; then
  echo -e "${YELLOW}Vacuuming systemd journal (keep 7 days)...${NC}"
  sudo journalctl --vacuum-time=7d 2>/dev/null || true
  echo -e "${GREEN}Journal vacuum done.${NC}"
fi

if [ "$PRUNE_NERDCTL" = true ]; then
  NERDCTL_NS="${NERDCTL_NS:-k8s.io}"
  if ! command -v nerdctl >/dev/null 2>&1; then
    echo -e "${RED}nerdctl not found; skip prune.${NC}"
  else
    echo -e "${YELLOW}nerdctl system prune (namespace=$NERDCTL_NS)...${NC}"
    if [ "${PRUNE_ALL:-0}" = "1" ]; then
      echo -e "${RED}PRUNE_ALL=1: running prune -a (removes unused images entirely)${NC}"
      sudo nerdctl --namespace "$NERDCTL_NS" system prune -a -f || true
    else
      sudo nerdctl --namespace "$NERDCTL_NS" system prune -f || true
    fi
    echo -e "${GREEN}nerdctl prune done.${NC}"
  fi
fi

echo ""
df -h / | tail -1
echo -e "${GREEN}Done. Kiểm tra node: kubectl describe node $(hostname) | grep -i pressure${NC}"

#!/usr/bin/env bash
# =============================================================================
# Kiểm tra nội dung có thể xóa (repo + images) và image outdated.
# Chỉ báo cáo, không xóa. Xóa thủ công hoặc dùng check-and-clean-host-resources.sh --clean
# =============================================================================
# Usage: ./scripts/clean/check-removable-content.sh
# =============================================================================

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
ok()   { echo -e "${GREEN}✅${NC} $*"; }
warn() { echo -e "${YELLOW}⚠️${NC}  $*"; }
info() { echo -e "${BLUE}[INFO]${NC} $*"; }

echo "=========================================="
echo "Nội dung có thể xóa / image outdated"
echo "=========================================="
echo ""

REMOVABLE=0

# --- 1. Repo: binary, tar, test-results ---
echo "=== 1. Trong repo ==="
if [ -f "$PROJECT_ROOT/fortuna-core" ]; then
  SIZE=$(du -sh "$PROJECT_ROOT/fortuna-core" 2>/dev/null | cut -f1)
  warn "Binary cũ: $PROJECT_ROOT/fortuna-core ($SIZE) — xóa: rm -f $PROJECT_ROOT/fortuna-core"
  REMOVABLE=$((REMOVABLE + 1))
fi
if [ -f "$PROJECT_ROOT/fortuna-agent-latest.tar" ]; then
  SIZE=$(du -sh "$PROJECT_ROOT/fortuna-agent-latest.tar" 2>/dev/null | cut -f1)
  warn "Tar (có thể rỗng): $PROJECT_ROOT/fortuna-agent-latest.tar ($SIZE) — xóa: rm -f $PROJECT_ROOT/fortuna-agent-latest.tar"
  REMOVABLE=$((REMOVABLE + 1))
fi
if [ -d "$PROJECT_ROOT/test-results" ]; then
  SIZE=$(du -sh "$PROJECT_ROOT/test-results" 2>/dev/null | cut -f1)
  warn "Test results (root): $PROJECT_ROOT/test-results ($SIZE) — xóa: rm -rf $PROJECT_ROOT/test-results"
  REMOVABLE=$((REMOVABLE + 1))
fi
for d in "$PROJECT_ROOT"/backup-session-*; do
  [ -d "$d" ] || continue
  SIZE=$(du -sh "$d" 2>/dev/null | cut -f1)
  warn "Backup session: $d ($SIZE) — xóa: rm -rf $d"
  REMOVABLE=$((REMOVABLE + 1))
done
COUNT_MD=$(find "${PROJECT_ROOT}/docs/test-results" -maxdepth 1 -name "*.md" ! -name "README.md" 2>/dev/null | wc -l | tr -d ' ')
if [ "${COUNT_MD:-0}" -gt 2 ]; then
  warn "docs/test-results: $COUNT_MD file .md (giữ 1–2 mới nhất, còn lại archive/xóa) — xem: ls docs/test-results/"
  REMOVABLE=$((REMOVABLE + 1))
fi
[ $REMOVABLE -eq 0 ] && info "Không phát hiện file repo dư thừa rõ ràng."
echo ""

# --- 2. Images: dangling + fortuna duplicate tags ---
echo "=== 2. Images (nerdctl namespace=$CONTAINERD_NS) ==="
if ! command -v nerdctl &>/dev/null; then
  warn "nerdctl không có; bỏ qua phần images."
else
  DANGLING=$(nerdctl --namespace "$CONTAINERD_NS" images -f "dangling=true" -q 2>/dev/null | wc -l | tr -d ' ')
  if [ "${DANGLING:-0}" -gt 0 ]; then
    warn "Dangling images: $DANGLING — dọn: nerdctl --namespace $CONTAINERD_NS system prune -f"
    REMOVABLE=$((REMOVABLE + 1))
  fi
  # Fortuna refs with same tag, multiple IDs (outdated)
  FORTUNA_LATEST=$(nerdctl --namespace "$CONTAINERD_NS" images --format '{{.Repository}} {{.Tag}} {{.ID}}' 2>/dev/null | grep -E 'fortuna-(core|agent|dashboard)\s+latest\s+' | awk '{print $3}' | sort -u | wc -l | tr -d ' ')
  if [ "${FORTUNA_LATEST:-0}" -gt 1 ]; then
    warn "fortuna-*:latest có nhiều image ID (cũ + mới) — dọn: ./scripts/clean/check-and-clean-host-resources.sh --clean -y hoặc clean-containerd-images.sh"
    REMOVABLE=$((REMOVABLE + 1))
  fi
  # Build cache
  info "Build cache: nerdctl builder prune -a --dry-run (xem dung lượng có thể giải phóng)"
  nerdctl --namespace "$CONTAINERD_NS" builder prune -a --dry-run 2>/dev/null | tail -3 || true
fi
echo ""

# --- 3. Disk ---
echo "=== 3. Disk ==="
df -h "$PROJECT_ROOT" 2>/dev/null | awk 'NR==1 || NR==2'
echo ""

echo "=========================================="
if [ $REMOVABLE -gt 0 ]; then
  echo -e "${YELLOW}Có $REMOVABLE mục có thể dọn. Chi tiết: scripts/clean/README-CLEANUP-CANDIDATES.md${NC}"
  echo "Clean host (images + cache): ./scripts/clean/check-and-clean-host-resources.sh --clean -y"
else
  ok "Không phát hiện mục dư thừa rõ ràng."
fi
echo ""

#!/usr/bin/env bash
# =============================================================================
# Kiểm tra môi trường để rebuild và deploy toàn bộ image (Fortuna).
# Chạy: ./scripts/verify/check-env-rebuild-deploy.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
ok()  { echo -e "${GREEN}✅${NC} $1"; }
warn() { echo -e "${YELLOW}⚠️${NC}  $1"; }
err()  { echo -e "${RED}❌${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

echo "=========================================="
echo "Kiểm tra môi trường – Rebuild & Deploy"
echo "=========================================="
echo ""

FAIL=0

# --- Prerequisites (build) ---
echo "=== Công cụ build & runtime ==="
command -v kubectl >/dev/null 2>&1 && ok "kubectl" || { err "kubectl"; FAIL=1; }
command -v nerdctl >/dev/null 2>&1 && ok "nerdctl" || { err "nerdctl (cần cho build vào containerd)"; FAIL=1; }
command -v ctr >/dev/null 2>&1 && ok "ctr (containerd)" || { err "ctr"; FAIL=1; }
echo ""

# --- Kubernetes cluster ---
echo "=== Kubernetes cluster ==="
if kubectl cluster-info &>/dev/null; then
  ok "Cluster accessible"
  kubectl get nodes --no-headers 2>/dev/null | wc -l | xargs -I{} info "Số node: {}"
else
  err "Không kết nối được cluster (kubectl cluster-info thất bại)"
  warn "Bật cluster (kind/kubeadm/minikube) rồi chạy lại. Deploy chỉ chạy khi cluster đã sẵn sàng."
  FAIL=1
fi
echo ""

# --- Repo: Dockerfiles ---
echo "=== Dockerfiles ==="
for f in core/Dockerfile agent/Dockerfile dashboard/Dockerfile; do
  [ -f "$PROJECT_ROOT/$f" ] && ok "$f" || { err "Thiếu $f"; FAIL=1; }
done
echo ""

# --- Repo: Build script ---
echo "=== Script build ==="
[ -x "$PROJECT_ROOT/scripts/build/build-and-load-containerd.sh" ] && ok "build-and-load-containerd.sh" || { err "Thiếu hoặc không executable: scripts/build/build-and-load-containerd.sh"; FAIL=1; }
echo ""

# --- Repo: Deploy YAMLs ---
echo "=== Deploy YAMLs ==="
for f in deploy/fortuna-core-deployment.yaml deploy/fortuna-agent-daemonset.yaml deploy/fortuna-rbac.yaml deploy/dashboard-deployment.yaml; do
  [ -f "$PROJECT_ROOT/$f" ] && ok "$f" || { err "Thiếu $f"; FAIL=1; }
done
echo ""

# --- Pipeline ---
echo "=== Pipeline ==="
[ -x "$PROJECT_ROOT/scripts/pipeline/full-clean-database-rebuild-deploy.sh" ] && ok "full-clean-database-rebuild-deploy.sh" || warn "Pipeline script không executable"
if [ ! -x "$PROJECT_ROOT/scripts/deploy/deploy-fortuna-robust.sh" ]; then
  warn "deploy-fortuna-robust.sh không tồn tại; pipeline sẽ apply trực tiếp deploy/*.yaml"
fi
echo ""

# --- Disk / RAM (gợi ý) ---
echo "=== Tài nguyên host (gợi ý) ==="
free -h | head -2
df -h / 2>/dev/null | tail -1
echo ""

echo "=========================================="
if [ $FAIL -eq 0 ]; then
  echo -e "${GREEN}Kết luận: Môi trường đủ để rebuild & deploy (khi cluster đã bật).${NC}"
  echo ""
  echo "Lệnh rebuild + deploy toàn bộ:"
  echo "  ./scripts/pipeline/full-clean-database-rebuild-deploy.sh"
  echo ""
  echo "Chỉ rebuild (không deploy):"
  echo "  ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --skip-deploy"
  echo ""
  echo "Chạy pipeline nền (tránh timeout, log /tmp/clean-rebuild-deploy.log):"
  echo "  RUN_ASYNC=1 ./scripts/pipeline/full-clean-database-rebuild-deploy.sh"
  exit 0
else
  echo -e "${RED}Một số mục chưa đạt; sửa lỗi trên rồi chạy lại.${NC}"
  exit 1
fi

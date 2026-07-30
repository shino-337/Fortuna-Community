#!/usr/bin/env bash
# ============================================================================
# Dọn image test E2E (vuln alpine, debian9, ubuntu18, ...)
# ============================================================================
# Xóa pod đang dùng image → xóa image. Chạy khi cần giải phóng dung lượng hoặc
# trước khi build lại image test.
# Usage: ./scripts/clean/clean-e2e-test-images.sh [--pods-only]
#   --pods-only: chỉ xóa pod, không xóa image
# ============================================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NAMESPACE="${NAMESPACE:-fortuna}"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"
PODS_ONLY=false
for arg in "$@"; do
  case "$arg" in
    --pods-only) PODS_ONLY=true ;;
  esac
done

echo "=== E2E test images cleanup ==="
echo ""

# 1. Xóa pod test vuln (để image có thể bị xóa)
echo "[1] Deleting E2E vuln pods (fortuna-e2e namespace)..."
for name in fortuna-e2e-vuln-alpine fortuna-e2e-vuln-debian9 fortuna-e2e-vuln-debian10 fortuna-e2e-vuln-ubuntu18; do
  kubectl delete pod -n fortuna-e2e "$name" --ignore-not-found=true 2>/dev/null && echo "  deleted pod $name" || true
done
echo ""

if [ "$PODS_ONLY" = true ]; then
  echo "Done (pods only)."
  exit 0
fi

# 2. Xóa image test
echo "[2] Removing E2E vuln images (nerdctl)..."
for img in fortuna-e2e-vuln-alpine:latest fortuna-e2e-vuln-debian9:latest fortuna-e2e-vuln-ubuntu18:latest; do
  if nerdctl --namespace "$CONTAINERD_NS" rmi "$img" --force 2>/dev/null; then
    echo "  removed $img"
  fi
done
echo ""
echo "Done."

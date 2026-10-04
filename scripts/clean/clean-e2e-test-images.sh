#!/usr/bin/env bash
# ============================================================================
# Remove E2E test images (vuln alpine, debian9, ubuntu18, ...)
# ============================================================================
# Deletes pods using the images, then the images. Run to free disk space or
# before rebuilding the test images.
# Usage: ./scripts/clean/clean-e2e-test-images.sh [--pods-only]
#   --pods-only: delete pods only, keep images
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

# 1. Delete vulnerable test pods (so their images can be removed)
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

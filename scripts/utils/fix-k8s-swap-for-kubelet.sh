#!/usr/bin/env bash
# =============================================================================
# Khắc phục: kubelet không chạy vì "running with swap on is not supported"
# Chạy trên từng node (master + worker): tắt swap, vô hiệu hóa swap sau reboot, restart kubelet.
#
# Usage:
#   sudo ./scripts/utils/fix-k8s-swap-for-kubelet.sh
#   ssh worker-node 'sudo bash -s' < scripts/utils/fix-k8s-swap-for-kubelet.sh
# =============================================================================

set -euo pipefail

echo "=== Fix Kubernetes: disable swap for kubelet ==="
echo ""

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root (sudo)."
  exit 1
fi

echo "[1/3] Current swap:"
cat /proc/swaps
echo ""

echo "[2/3] Disabling swap..."
swapoff -a || true
if [ -s /proc/swaps ]; then
  echo "Warning: swap still active after swapoff -a"
  cat /proc/swaps
else
  echo "  Swap off OK"
fi
echo ""

echo "[3/3] Disable swap after reboot (comment in /etc/fstab)..."
if grep -qE '^[^#].*\bswap\b' /etc/fstab 2>/dev/null; then
  sed -i.bak '/\bswap\b/s/^/# commented for k8s kubelet: /' /etc/fstab
  echo "  Commented swap line(s) in /etc/fstab (backup: /etc/fstab.bak)"
else
  echo "  No active swap entry in fstab (already commented or absent)"
fi
grep -E "swap|commented" /etc/fstab 2>/dev/null || true
echo ""

echo "Restarting kubelet..."
systemctl restart kubelet
sleep 2
if systemctl is-active --quiet kubelet; then
  echo "  kubelet is active."
else
  echo "  Warning: kubelet may still be starting. Check: systemctl status kubelet"
fi
echo ""
echo "Done. Verify: kubectl get nodes (from master)."

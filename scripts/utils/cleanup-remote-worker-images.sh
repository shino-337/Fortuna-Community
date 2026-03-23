#!/usr/bin/env bash
# ============================================================================
# SSH vào một worker: xóa image fortuna/ksam cũ trong containerd (k8s.io) + prune
# ============================================================================
# Giải phóng đĩa trước khi push-images-to-workers.sh import agent/core.
#
# Usage:
#   bash scripts/utils/cleanup-remote-worker-images.sh user@k8s-worker01
#   bash scripts/utils/cleanup-remote-worker-images.sh k8s@192.168.1.50
#
# Env: SSH_OPTS="-o StrictHostKeyChecking=no" (tùy chọn)
# ============================================================================
set -euo pipefail

if [ $# -lt 1 ] || [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  echo "Usage: $0 user@worker-host"
  exit 1
fi

TARGET="$1"
SSH_OPTS="${SSH_OPTS:-}"

REMOTE_SCRIPT=$(cat <<'EOS'
set -e
echo "=== $(hostname) df ==="
df -h / /var 2>/dev/null || df -h
echo "=== Removing fortuna|ksam images (ctr k8s.io) ==="
sudo ctr -n k8s.io images ls -q 2>/dev/null | grep -E 'fortuna|ksam' | while read -r ref; do
  [ -z "$ref" ] && continue
  echo "  rm $ref"
  sudo ctr -n k8s.io images rm "$ref" 2>/dev/null || true
done
echo "=== nerdctl system prune -f ==="
if command -v nerdctl >/dev/null 2>&1; then
  sudo nerdctl --namespace k8s.io system prune -f || true
else
  echo "nerdctl not found; skip prune"
fi
echo "=== Done df ==="
df -h / | tail -1
EOS
)

# shellcheck disable=SC2086
ssh $SSH_OPTS "$TARGET" "bash -s" <<< "$REMOTE_SCRIPT"
echo "Remote cleanup finished. Next: build on build host, then push-images-to-workers.sh --clean-remote"

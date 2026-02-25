#!/usr/bin/env bash
# =============================================================================
# Chạy script này TRÊN NODE worker01 (sau khi SSH hoặc console vào worker01).
# Cách dùng: SSH vào worker01 rồi chạy:
#   curl -sL <url> | sudo bash
#   hoặc copy nội dung vào worker01 rồi: sudo bash expand-disk-worker01-remote.sh
# =============================================================================
set -euo pipefail

echo "=== Hostname ==="
hostname

echo ""
echo "=== Block devices (lsblk) ==="
lsblk

echo ""
echo "=== Disk usage (df -h) ==="
df -h

echo ""
echo "=== Root mount ==="
ROOT_SOURCE=$(findmnt -n -o SOURCE / 2>/dev/null || true)
echo "Root SOURCE: $ROOT_SOURCE"

# Detect: LVM (e.g. /dev/mapper/ubuntu--vg-ubuntu--lv) vs partition (e.g. /dev/sda1)
if echo "$ROOT_SOURCE" | grep -q mapper; then
  echo ""
  echo "=== LVM detected - extending PV and LV ==="
  # Find PV used by root's VG
  ROOT_DEV=$(readlink -f "$ROOT_SOURCE" 2>/dev/null | head -1)
  PV_DEV=$(pvs -o pv_name --noheadings 2>/dev/null | tr -d ' ' | head -1)
  if [ -n "$PV_DEV" ] && [ -b "$PV_DEV" ]; then
    echo "Resizing PV: $PV_DEV"
    pvresize "$PV_DEV" || true
    echo "Extending LV to use free space..."
    LV_PATH=$(lvs -o lv_path --noheadings 2>/dev/null | tr -d ' ' | head -1)
    if [ -n "$LV_PATH" ]; then
      lvextend -l +100%FREE "$LV_PATH" || true
      echo "Resizing filesystem on $LV_PATH..."
      if type resize2fs &>/dev/null; then
        resize2fs "$LV_PATH" || true
      elif type xfs_growfs &>/dev/null; then
        xfs_growfs / || true
      fi
    fi
  fi
else
  echo ""
  echo "=== Partition layout - trying growpart + resize2fs ==="
  # Root is typically /dev/sda1 or /dev/vda1
  DISK_DEV=$(echo "$ROOT_SOURCE" | sed 's/[0-9]*$//')
  PART_NUM=$(echo "$ROOT_SOURCE" | grep -o '[0-9]*$')
  if [ -n "$DISK_DEV" ] && [ -b "$DISK_DEV" ] && [ -n "$PART_NUM" ]; then
    echo "Disk: $DISK_DEV, Partition: $PART_NUM"
    if type growpart &>/dev/null; then
      growpart "$DISK_DEV" "$PART_NUM" || true
    fi
    if type resize2fs &>/dev/null; then
      resize2fs "$ROOT_SOURCE" || true
    elif type xfs_growfs &>/dev/null; then
      xfs_growfs / || true
    fi
  else
    echo "Could not auto-detect disk/partition. Run manually:"
    echo "  sudo growpart /dev/sda 1   # or /dev/vda 1 - check lsblk"
    echo "  sudo resize2fs /dev/sda1  # or /dev/vda1"
  fi
fi

echo ""
echo "=== After resize (df -h) ==="
df -h /

echo ""
echo "=== Restart kubelet to refresh node capacity ==="
echo "Run: sudo systemctl restart kubelet"
echo "Then from master: kubectl describe node k8s-worker01 | grep -A2 Capacity"

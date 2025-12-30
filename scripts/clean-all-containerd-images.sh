#!/bin/bash

# ============================================================================
# Clean All Containerd Images (Fortuna)
# ============================================================================
# Quick script to remove all Fortuna/KSAM images from containerd
# ============================================================================

set -euo pipefail

NAMESPACE="${CONTAINERD_NAMESPACE:-k8s.io}"

echo "=========================================="
echo "Clean All Fortuna Images from Containerd"
echo "=========================================="
echo ""

# Check ctr
if ! command -v ctr >/dev/null 2>&1; then
    echo "❌ ctr not found"
    exit 1
fi

# List all fortuna/ksam images
echo "Finding Fortuna images..."
IMAGES=$(ctr -n "${NAMESPACE}" images ls -q 2>/dev/null | grep -E "fortuna|ksam" || echo "")

if [ -z "$IMAGES" ]; then
    echo "✅ No Fortuna images found"
    exit 0
fi

echo ""
echo "Images to delete:"
echo "$IMAGES" | while read -r img; do
    [ -n "$img" ] && echo "  - $img"
done
echo ""

# Delete all
echo "Deleting images..."
echo "$IMAGES" | while read -r img; do
    if [ -n "$img" ]; then
        echo -n "  Deleting $img... "
        if ctr -n "${NAMESPACE}" images rm "$img" 2>/dev/null; then
            echo "✅"
        else
            echo "❌"
        fi
    fi
done

echo ""
echo "✅ Cleanup complete"
echo ""
echo "Remaining images:"
ctr -n "${NAMESPACE}" images ls | grep -E "fortuna|ksam" || echo "  None"


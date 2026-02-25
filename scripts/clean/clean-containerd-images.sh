#!/bin/bash

# ============================================================================
# Clean Containerd Images
# ============================================================================
# Removes old Fortuna images from containerd k8s.io namespace
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
NAMESPACE="${CONTAINERD_NAMESPACE:-k8s.io}"
IMAGE_PREFIX="${IMAGE_PREFIX:-fortuna}"
DRY_RUN="${DRY_RUN:-false}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Clean old Fortuna images from containerd k8s.io namespace.

Options:
    -n, --namespace NAMESPACE    Containerd namespace (default: k8s.io)
    -p, --prefix PREFIX          Image name prefix (default: fortuna)
    -d, --dry-run               Show what would be deleted without deleting
    -a, --all                   Delete ALL images (not just fortuna)
    -h, --help                  Show this help

Examples:
    # Dry run (show what would be deleted)
    $0 --dry-run

    # Clean all fortuna images
    $0

    # Clean with custom prefix
    $0 --prefix ksam

    # Clean all images (dangerous!)
    $0 --all
EOF
}

DELETE_ALL=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -p|--prefix)
            IMAGE_PREFIX="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -a|--all)
            DELETE_ALL=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            exit 1
            ;;
    esac
done

echo "=========================================="
echo "Clean Containerd Images"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  Namespace:  ${NAMESPACE}"
echo "  Prefix:      ${IMAGE_PREFIX}"
echo "  Dry Run:     ${DRY_RUN}"
echo "  Delete All:  ${DELETE_ALL}"
echo ""

# Check prerequisites
if ! command -v ctr >/dev/null 2>&1; then
    echo -e "${RED}❌ ctr not found${NC}"
    exit 1
fi

# Check containerd socket
if [ ! -S /run/containerd/containerd.sock ] && [ ! -S /var/run/containerd/containerd.sock ]; then
    echo -e "${RED}❌ containerd socket not found${NC}"
    exit 1
fi

# List images
echo -e "${BLUE}Listing images in namespace ${NAMESPACE}...${NC}"
echo ""

if [ "$DELETE_ALL" = "true" ]; then
    echo -e "${YELLOW}⚠️  WARNING: Will delete ALL images in namespace ${NAMESPACE}${NC}"
    IMAGES=$(ctr -n "${NAMESPACE}" images ls -q 2>/dev/null || echo "")
else
    IMAGES=$(ctr -n "${NAMESPACE}" images ls -q 2>/dev/null | grep -E "${IMAGE_PREFIX}|ksam" || echo "")
fi

if [ -z "$IMAGES" ]; then
    echo -e "${GREEN}✅ No images found to delete${NC}"
    exit 0
fi

# Show images to be deleted
echo "Images to be deleted:"
echo "$IMAGES" | while read -r image; do
    if [ -n "$image" ]; then
        echo "  - $image"
    fi
done
echo ""

# Count images
IMAGE_COUNT=$(echo "$IMAGES" | grep -v '^$' | wc -l | tr -d ' ')
echo "Total images to delete: $IMAGE_COUNT"
echo ""

# Confirm deletion
if [ "$DRY_RUN" = "false" ]; then
    if [ "$DELETE_ALL" = "true" ]; then
        echo -e "${RED}⚠️  DANGER: This will delete ALL images!${NC}"
        read -p "Are you sure? Type 'yes' to continue: " confirm
        if [ "$confirm" != "yes" ]; then
            echo "Aborted."
            exit 0
        fi
    else
        read -p "Delete these images? (y/N): " confirm
        if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
            echo "Aborted."
            exit 0
        fi
    fi
fi

# Delete images
echo ""
if [ "$DRY_RUN" = "true" ]; then
    echo -e "${YELLOW}[DRY RUN] Would delete the following images:${NC}"
    echo "$IMAGES" | while read -r image; do
        if [ -n "$image" ]; then
            echo "  ctr -n ${NAMESPACE} images rm $image"
        fi
    done
    echo ""
    echo -e "${YELLOW}Run without --dry-run to actually delete${NC}"
else
    echo -e "${BLUE}Deleting images...${NC}"
    DELETED=0
    FAILED=0
    
    echo "$IMAGES" | while read -r image; do
        if [ -n "$image" ]; then
            echo -n "Deleting $image... "
            if ctr -n "${NAMESPACE}" images rm "$image" 2>/dev/null; then
                echo -e "${GREEN}✅${NC}"
                ((DELETED++))
            else
                echo -e "${RED}❌${NC}"
                ((FAILED++))
            fi
        fi
    done
    
    echo ""
    echo "=========================================="
    echo "Cleanup Summary"
    echo "=========================================="
    echo -e "${GREEN}✅ Deleted: ${DELETED}${NC}"
    if [ $FAILED -gt 0 ]; then
        echo -e "${RED}❌ Failed: ${FAILED}${NC}"
    fi
    echo ""
    
    # Verify cleanup
    echo "Remaining images:"
    REMAINING=$(ctr -n "${NAMESPACE}" images ls -q 2>/dev/null | grep -E "${IMAGE_PREFIX}|ksam" || echo "")
    if [ -z "$REMAINING" ]; then
        echo -e "${GREEN}✅ No ${IMAGE_PREFIX} images remaining${NC}"
    else
        echo -e "${YELLOW}⚠️  Still have images:${NC}"
        echo "$REMAINING" | while read -r image; do
            if [ -n "$image" ]; then
                echo "  - $image"
            fi
        done
    fi
fi

echo ""


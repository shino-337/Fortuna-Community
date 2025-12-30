#!/bin/bash

# ============================================================================
# Build and Import Fortuna Images to Containerd
# ============================================================================
# Complete workflow: Build with nerdctl → Import to containerd → Verify
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Configuration
IMAGE_PREFIX="${IMAGE_PREFIX:-fortuna}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')}"
BUILD_COMMIT="${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_TIME="${BUILD_TIME:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
NAMESPACE="${CONTAINERD_NAMESPACE:-k8s.io}"
EXPORT_FOR_DISTRIBUTION="${EXPORT_FOR_DISTRIBUTION:-false}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Fortuna Build and Import (Containerd)"
echo "=========================================="
echo ""

# Step 1: Build images
echo -e "${BLUE}Step 1: Building images with nerdctl...${NC}"
if [ -f "${SCRIPT_DIR}/build-with-containerd.sh" ]; then
    if [ "$EXPORT_FOR_DISTRIBUTION" = "true" ]; then
        EXPORT_IMAGES=true bash "${SCRIPT_DIR}/build-with-containerd.sh"
    else
        bash "${SCRIPT_DIR}/build-with-containerd.sh"
    fi
else
    echo -e "${RED}❌${NC} build-with-containerd.sh not found"
    exit 1
fi

# Step 2: Verify images in containerd
echo ""
echo -e "${BLUE}Step 2: Verifying images in containerd...${NC}"

verify_images() {
    local found=0
    
    if ctr -n "${NAMESPACE}" images ls | grep -q "${IMAGE_PREFIX}-core:${VERSION}"; then
        echo -e "${GREEN}✅${NC} Core image found: ${IMAGE_PREFIX}-core:${VERSION}"
        ((found++))
    else
        echo -e "${RED}❌${NC} Core image not found"
    fi
    
    if ctr -n "${NAMESPACE}" images ls | grep -q "${IMAGE_PREFIX}-agent:${VERSION}"; then
        echo -e "${GREEN}✅${NC} Agent image found: ${IMAGE_PREFIX}-agent:${VERSION}"
        ((found++))
    else
        echo -e "${RED}❌${NC} Agent image not found"
    fi
    
    return $found
}

if verify_images; then
    echo -e "${GREEN}✅${NC} All images verified in containerd"
else
    echo -e "${YELLOW}⚠️${NC}  Some images not found"
fi

# Step 3: Show image details
echo ""
echo -e "${BLUE}Step 3: Image details...${NC}"
echo ""
echo "Core images:"
ctr -n "${NAMESPACE}" images ls | grep "${IMAGE_PREFIX}-core" || echo "  None found"
echo ""
echo "Agent images:"
ctr -n "${NAMESPACE}" images ls | grep "${IMAGE_PREFIX}-agent" || echo "  None found"
echo ""

# Step 4: Export images if requested
if [ "$EXPORT_FOR_DISTRIBUTION" = "true" ]; then
    echo -e "${BLUE}Step 4: Exporting images for distribution...${NC}"
    
    EXPORT_DIR="/tmp/fortuna-images-${VERSION}"
    mkdir -p "$EXPORT_DIR"
    
    # Export Core
    CORE_EXPORT="${EXPORT_DIR}/${IMAGE_PREFIX}-core-${VERSION}.tar"
    echo "Exporting Core image..."
    nerdctl --namespace "${NAMESPACE}" save -o "$CORE_EXPORT" "${IMAGE_PREFIX}-core:${VERSION}"
    if [ -f "$CORE_EXPORT" ]; then
        echo -e "${GREEN}✅${NC} Core exported: $CORE_EXPORT ($(du -h "$CORE_EXPORT" | cut -f1))"
    fi
    
    # Export Agent
    AGENT_EXPORT="${EXPORT_DIR}/${IMAGE_PREFIX}-agent-${VERSION}.tar"
    echo "Exporting Agent image..."
    nerdctl --namespace "${NAMESPACE}" save -o "$AGENT_EXPORT" "${IMAGE_PREFIX}-agent:${VERSION}"
    if [ -f "$AGENT_EXPORT" ]; then
        echo -e "${GREEN}✅${NC} Agent exported: $AGENT_EXPORT ($(du -h "$AGENT_EXPORT" | cut -f1))"
    fi
    
    echo ""
    echo "Exported images are in: $EXPORT_DIR"
    echo "To import on another node:"
    echo "  ./scripts/import-to-containerd.sh ${CORE_EXPORT}"
    echo "  ./scripts/import-to-containerd.sh ${AGENT_EXPORT}"
    echo ""
fi

# Summary
echo "=========================================="
echo -e "${GREEN}✅ Build and Import Complete!${NC}"
echo "=========================================="
echo ""
echo "Images are ready in containerd:"
echo "  ${IMAGE_PREFIX}-core:${VERSION}"
echo "  ${IMAGE_PREFIX}-agent:${VERSION}"
echo ""
echo "To use in Kubernetes:"
echo "  Update deployment manifests with:"
echo "    image: ${IMAGE_PREFIX}-core:${VERSION}"
echo "    imagePullPolicy: IfNotPresent  # or Never for local images"
echo ""
echo "To verify images:"
echo "  ctr -n ${NAMESPACE} images ls | grep ${IMAGE_PREFIX}"
echo ""


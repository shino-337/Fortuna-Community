#!/bin/bash

# ============================================================================
# Build Fortuna Images with nerdctl and Import to Containerd
# ============================================================================
# Builds images using nerdctl and imports them into containerd for K8s
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Configuration
IMAGE_PREFIX="${IMAGE_PREFIX:-fortuna}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')}"
BUILD_COMMIT="${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_TIME="${BUILD_TIME:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
NAMESPACE="${CONTAINERD_NAMESPACE:-k8s.io}"  # Default containerd namespace for K8s

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Fortuna Build with Containerd (nerdctl)"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  Image Prefix: ${IMAGE_PREFIX}"
echo "  Version:     ${VERSION}"
echo "  Commit:      ${BUILD_COMMIT}"
echo "  Build Time:  ${BUILD_TIME}"
echo "  Namespace:   ${NAMESPACE}"
echo ""

# Check prerequisites
check_prerequisites() {
    echo -e "${BLUE}Checking prerequisites...${NC}"
    
    if ! command -v nerdctl >/dev/null 2>&1; then
        echo -e "${RED}❌ nerdctl not found${NC}"
        echo "Install nerdctl: https://github.com/containerd/nerdctl"
        exit 1
    fi
    echo -e "${GREEN}✅${NC} nerdctl found"
    
    if ! command -v ctr >/dev/null 2>&1; then
        echo -e "${RED}❌ ctr not found${NC}"
        echo "ctr is part of containerd. Ensure containerd is installed."
        exit 1
    fi
    echo -e "${GREEN}✅${NC} ctr found"
    
    # Check containerd socket
    if [ ! -S /run/containerd/containerd.sock ]; then
        echo -e "${YELLOW}⚠️${NC}  containerd.sock not found at /run/containerd/containerd.sock"
        echo "Trying alternative locations..."
        if [ -S /var/run/containerd/containerd.sock ]; then
            export CONTAINERD_ADDRESS=/var/run/containerd/containerd.sock
            echo -e "${GREEN}✅${NC} Found containerd.sock at /var/run/containerd/containerd.sock"
        else
            echo -e "${RED}❌${NC} Cannot find containerd socket"
            exit 1
        fi
    else
        echo -e "${GREEN}✅${NC} containerd.sock found"
    fi
    
    echo ""
}

# Function to build image with nerdctl
build_image() {
    local component=$1
    local dockerfile=$2
    local image_name="${IMAGE_PREFIX}-${component}:${VERSION}"
    local image_latest="${IMAGE_PREFIX}-${component}:latest"
    
    echo -e "${BLUE}Building ${component} with nerdctl...${NC}"
    echo "  Image: ${image_name}"
    echo "  Dockerfile: ${dockerfile}"
    echo ""
    
    cd "${PROJECT_ROOT}"
    
    # Build with nerdctl
    nerdctl build \
        -f "${dockerfile}" \
        -t "${image_name}" \
        -t "${image_latest}" \
        --build-arg FORTUNA_BUILD_VERSION="${VERSION}" \
        --build-arg FORTUNA_BUILD_COMMIT="${BUILD_COMMIT}" \
        --build-arg FORTUNA_BUILD_TIME="${BUILD_TIME}" \
        --namespace "${NAMESPACE}" \
        --progress=plain \
        .
    
    echo ""
    echo -e "${GREEN}✅ ${component} built successfully${NC}"
    echo "  ${image_name}"
    echo "  ${image_latest}"
    echo ""
}

# Function to verify image in containerd
verify_image() {
    local component=$1
    local image_name="${IMAGE_PREFIX}-${component}:${VERSION}"
    
    echo -e "${BLUE}Verifying ${component} image in containerd...${NC}"
    
    # List images using ctr
    if ctr -n "${NAMESPACE}" images ls | grep -q "${image_name}"; then
        echo -e "${GREEN}✅${NC} Image ${image_name} found in containerd"
        
        # Show image details
        echo "Image details:"
        ctr -n "${NAMESPACE}" images ls | grep "${image_name}" || true
        echo ""
    else
        echo -e "${RED}❌${NC} Image ${image_name} not found in containerd"
        return 1
    fi
}

# Function to export and import image (for multi-node clusters)
export_import_image() {
    local component=$1
    local image_name="${IMAGE_PREFIX}-${component}:${VERSION}"
    local export_file="/tmp/${IMAGE_PREFIX}-${component}-${VERSION}.tar"
    
    echo -e "${BLUE}Exporting ${component} image...${NC}"
    
    # Export image using nerdctl
    nerdctl --namespace "${NAMESPACE}" save -o "${export_file}" "${image_name}"
    
    if [ -f "${export_file}" ]; then
        echo -e "${GREEN}✅${NC} Image exported to ${export_file}"
        echo "  File size: $(du -h "${export_file}" | cut -f1)"
        echo ""
        echo "To import on another node:"
        echo "  nerdctl --namespace ${NAMESPACE} load -i ${export_file}"
        echo "  # Or using ctr:"
        echo "  ctr -n ${NAMESPACE} images import ${export_file}"
        echo ""
    else
        echo -e "${RED}❌${NC} Failed to export image"
        return 1
    fi
}

# Main build process
main() {
    check_prerequisites
    
    # Build Core
    build_image "core" "core/Dockerfile"
    verify_image "core"
    
    # Build Agent
    build_image "agent" "agent/Dockerfile"
    verify_image "agent"
    
    # Export images (optional, for multi-node)
    if [ "${EXPORT_IMAGES:-false}" = "true" ]; then
        echo -e "${BLUE}Exporting images for distribution...${NC}"
        export_import_image "core"
        export_import_image "agent"
    fi
    
    # Summary
    echo "=========================================="
    echo -e "${GREEN}✅ Build Complete!${NC}"
    echo "=========================================="
    echo ""
    echo "Images built and available in containerd:"
    echo "  ${IMAGE_PREFIX}-core:${VERSION}"
    echo "  ${IMAGE_PREFIX}-core:latest"
    echo "  ${IMAGE_PREFIX}-agent:${VERSION}"
    echo "  ${IMAGE_PREFIX}-agent:latest"
    echo ""
    echo "List all images:"
    echo "  ctr -n ${NAMESPACE} images ls | grep ${IMAGE_PREFIX}"
    echo ""
    echo "List with nerdctl:"
    echo "  nerdctl --namespace ${NAMESPACE} images ls | grep ${IMAGE_PREFIX}"
    echo ""
    echo "To use in Kubernetes:"
    echo "  Update deployment manifests with:"
    echo "    image: ${IMAGE_PREFIX}-core:${VERSION}"
    echo "    imagePullPolicy: IfNotPresent  # or Never for local images"
    echo ""
    echo "To export images for other nodes:"
    echo "  EXPORT_IMAGES=true ${0}"
    echo ""
}

# Run main
main


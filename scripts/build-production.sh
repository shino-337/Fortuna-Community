#!/bin/bash

# ============================================================================
# Production Build Script for Fortuna
# ============================================================================
# Builds Docker images with versioning for production K8s deployment
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Configuration
REGISTRY="${DOCKER_REGISTRY:-localhost:5000}"  # Default: local registry
IMAGE_PREFIX="${IMAGE_PREFIX:-fortuna}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')}"
BUILD_COMMIT="${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_TIME="${BUILD_TIME:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Fortuna Production Build"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  Registry:    ${REGISTRY}"
echo "  Image Prefix: ${IMAGE_PREFIX}"
echo "  Version:     ${VERSION}"
echo "  Commit:      ${BUILD_COMMIT}"
echo "  Build Time:  ${BUILD_TIME}"
echo ""

# Function to build image
build_image() {
    local component=$1
    local dockerfile=$2
    local image_name="${REGISTRY}/${IMAGE_PREFIX}-${component}:${VERSION}"
    local image_latest="${REGISTRY}/${IMAGE_PREFIX}-${component}:latest"
    
    echo -e "${GREEN}Building ${component}...${NC}"
    echo "  Image: ${image_name}"
    echo "  Dockerfile: ${dockerfile}"
    echo ""
    
    cd "${PROJECT_ROOT}"
    
    docker build \
        -f "${dockerfile}" \
        -t "${image_name}" \
        -t "${image_latest}" \
        --build-arg FORTUNA_BUILD_VERSION="${VERSION}" \
        --build-arg FORTUNA_BUILD_COMMIT="${BUILD_COMMIT}" \
        --build-arg FORTUNA_BUILD_TIME="${BUILD_TIME}" \
        --build-arg BUILDKIT_INLINE_CACHE=1 \
        --progress=plain \
        .
    
    echo ""
    echo -e "${GREEN}✅ ${component} built successfully${NC}"
    echo "  ${image_name}"
    echo "  ${image_latest}"
    echo ""
}

# Function to push image (optional)
push_image() {
    local component=$1
    local image_name="${REGISTRY}/${IMAGE_PREFIX}-${component}:${VERSION}"
    local image_latest="${REGISTRY}/${IMAGE_PREFIX}-${component}:latest"
    
    if [ "${PUSH_IMAGES:-false}" = "true" ]; then
        echo -e "${YELLOW}Pushing ${component}...${NC}"
        docker push "${image_name}"
        docker push "${image_latest}"
        echo -e "${GREEN}✅ ${component} pushed successfully${NC}"
        echo ""
    else
        echo -e "${YELLOW}Skipping push (set PUSH_IMAGES=true to push)${NC}"
        echo ""
    fi
}

# Main build process
main() {
    # Build Core
    build_image "core" "core/Dockerfile"
    push_image "core"
    
    # Build Agent
    build_image "agent" "agent/Dockerfile"
    push_image "agent"
    
    # Summary
    echo "=========================================="
    echo -e "${GREEN}✅ Build Complete!${NC}"
    echo "=========================================="
    echo ""
    echo "Images built:"
    echo "  ${REGISTRY}/${IMAGE_PREFIX}-core:${VERSION}"
    echo "  ${REGISTRY}/${IMAGE_PREFIX}-core:latest"
    echo "  ${REGISTRY}/${IMAGE_PREFIX}-agent:${VERSION}"
    echo "  ${REGISTRY}/${IMAGE_PREFIX}-agent:latest"
    echo ""
    echo "To push images to registry:"
    echo "  PUSH_IMAGES=true ${0}"
    echo ""
    echo "To use in Kubernetes:"
    echo "  Update deployment manifests with:"
    echo "    image: ${REGISTRY}/${IMAGE_PREFIX}-core:${VERSION}"
    echo "    imagePullPolicy: IfNotPresent  # or Always"
    echo ""
}

# Run main
main


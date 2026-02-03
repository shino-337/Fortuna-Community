#!/bin/bash

# ============================================================================
# Build and Load Dashboard Image to Containerd
# ============================================================================
# Dashboard is built ONLY inside the container (Dockerfile). Host needs nerdctl
# and containerd only. Do NOT require npm or Node.js on host.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Configuration
IMAGE_PREFIX="${IMAGE_PREFIX:-fortuna}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'latest')}"
BUILD_COMMIT="${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_TIME="${BUILD_TIME:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
NAMESPACE="${CONTAINERD_NAMESPACE:-k8s.io}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} ✅ $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} ❌ $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} ⚠️  $1"; }

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing=0
    
    if ! command -v nerdctl >/dev/null 2>&1; then
        log_error "nerdctl is not installed"
        missing=1
    else
        log_success "nerdctl found: $(nerdctl --version 2>&1 | head -1)"
    fi
    
    if ! command -v ctr >/dev/null 2>&1; then
        log_error "ctr is not installed (part of containerd)"
        missing=1
    else
        log_success "ctr found"
    fi
    
    # No npm/node required on host; build runs inside container (Dockerfile)
    
    if [ $missing -eq 1 ]; then
        exit 1
    fi
    
    echo ""
}

# Build dashboard image
build_dashboard() {
    local image_name="${IMAGE_PREFIX}-dashboard:${VERSION}"
    local image_latest="${IMAGE_PREFIX}-dashboard:latest"
    
    log_info "Building dashboard..."
    echo "  Image: ${image_name}"
    echo "  Dockerfile: dashboard/Dockerfile"
    echo "  Namespace: ${NAMESPACE}"
    echo ""
    
    cd "${PROJECT_ROOT}"
    
    if nerdctl build \
        -f "dashboard/Dockerfile" \
        -t "${image_name}" \
        -t "${image_latest}" \
        --namespace "${NAMESPACE}" \
        --progress=plain \
        .; then
        log_success "Dashboard built successfully"
        
        local size=$(ctr -n "${NAMESPACE}" images ls | grep "${image_name}" | awk '{print $3}' || echo "unknown")
        echo "  Size: ${size}"
        echo ""
    else
        log_error "Failed to build dashboard"
        exit 1
    fi
}

# Verify image (ctr may list as docker.io/library/IMAGE:TAG)
verify_dashboard() {
    local image_name="${IMAGE_PREFIX}-dashboard:${VERSION}"
    
    log_info "Verifying dashboard image in containerd..."
    
    local list
    list=$(ctr -n "${NAMESPACE}" images ls 2>/dev/null) || true
    if echo "${list}" | grep -q "${image_name}"; then
        log_success "Image ${image_name} found in containerd"
        echo "${list}" | grep "${image_name}" | head -1
        echo ""
        return 0
    fi
    if echo "${list}" | grep -q "${IMAGE_PREFIX}-dashboard"; then
        log_success "Image ${IMAGE_PREFIX}-dashboard found in containerd"
        echo "${list}" | grep "${IMAGE_PREFIX}-dashboard" | head -2
        echo ""
        return 0
    fi
    log_error "Image ${image_name} not found in containerd"
    return 1
}

# Main
main() {
    echo "=========================================="
    echo "Fortuna Dashboard Build (Containerd)"
    echo "=========================================="
    echo ""
    echo "Configuration:"
    echo "  Image Prefix: ${IMAGE_PREFIX}"
    echo "  Version:      ${VERSION}"
    echo "  Commit:       ${BUILD_COMMIT}"
    echo "  Build Time:   ${BUILD_TIME}"
    echo "  Namespace:    ${NAMESPACE}"
    echo ""
    
    check_prerequisites
    
    build_dashboard
    verify_dashboard
    
    echo "=========================================="
    log_success "Build Complete!"
    echo "=========================================="
    echo ""
    echo "Image available in containerd:"
    ctr -n "${NAMESPACE}" images ls | grep "${IMAGE_PREFIX}-dashboard" | head -2
    echo ""
    echo "To use in Kubernetes deployment:"
    echo "  image: ${IMAGE_PREFIX}-dashboard:${VERSION}"
    echo "  imagePullPolicy: Never  # for local images"
    echo ""
}

main

#!/bin/bash

# ============================================================================
# Build and Load Images to Containerd
# ============================================================================
# Builds core (go), agent (go), dashboard (build inside Dockerfile via node/npm).
# Host: nerdctl + containerd + go only. Do NOT require npm or Node.js on host.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Configuration
IMAGE_PREFIX="${IMAGE_PREFIX:-fortuna}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'latest')}"
BUILD_COMMIT="${BUILD_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_TIME="${BUILD_TIME:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
NAMESPACE="${CONTAINERD_NAMESPACE:-k8s.io}"  # containerd namespace for K8s
NO_CACHE="${NO_CACHE:-false}"  # set to true for clean rebuild (no cache)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} ✅ $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} ❌ $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} ⚠️  $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing=0
    
    if ! command -v nerdctl >/dev/null 2>&1; then
        log_error "nerdctl is not installed"
        log_info "Install from: https://github.com/containerd/nerdctl"
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
    
    if ! command -v go >/dev/null 2>&1; then
        log_error "go is not installed (required for building)"
        missing=1
    else
        log_success "go found: $(go version | awk '{print $3}')"
    fi
    
    # Check containerd socket
    if [ ! -S /run/containerd/containerd.sock ] && [ ! -S /var/run/containerd/containerd.sock ]; then
        log_error "containerd socket not found"
        log_info "Expected locations: /run/containerd/containerd.sock or /var/run/containerd/containerd.sock"
        missing=1
    else
        log_success "containerd socket found"
    fi
    
    if [ $missing -eq 1 ]; then
        exit 1
    fi
    
    echo ""
}

# Build image with nerdctl
build_image() {
    local component=$1
    local dockerfile=$2
    local image_name="${IMAGE_PREFIX}-${component}:${VERSION}"
    local image_latest="${IMAGE_PREFIX}-${component}:latest"
    
    log_info "Building ${component}..."
    echo "  Image: ${image_name}"
    echo "  Dockerfile: ${dockerfile}"
    echo "  Namespace: ${NAMESPACE}"
    echo ""
    
    cd "${PROJECT_ROOT}"
    
    # Build args - dashboard doesn't need FORTUNA build args (npm runs in Dockerfile)
    local build_args=""
    if [ "${component}" != "dashboard" ]; then
        build_args="--build-arg FORTUNA_BUILD_VERSION=${VERSION} --build-arg FORTUNA_BUILD_COMMIT=${BUILD_COMMIT} --build-arg FORTUNA_BUILD_TIME=${BUILD_TIME}"
    fi
    
    # Build with nerdctl (--no-cache when NO_CACHE=true)
    local no_cache_arg=""
    [ "${NO_CACHE}" = "true" ] && no_cache_arg="--no-cache"
    if nerdctl build \
        -f "${dockerfile}" \
        -t "${image_name}" \
        -t "${image_latest}" \
        ${build_args} \
        ${no_cache_arg} \
        --namespace "${NAMESPACE}" \
        --progress=plain \
        .; then
        log_success "${component} built successfully"
        
        # Show image info
        local size=$(ctr -n "${NAMESPACE}" images ls | grep "${image_name}" | awk '{print $3}' || echo "unknown")
        echo "  Size: ${size}"
        echo ""
    else
        log_error "Failed to build ${component}"
        exit 1
    fi
}

# Verify image in containerd (ctr may list as docker.io/library/IMAGE:TAG)
verify_image() {
    local component=$1
    local image_name="${IMAGE_PREFIX}-${component}:${VERSION}"
    
    log_info "Verifying ${component} image in containerd..."
    
    if ctr -n "${NAMESPACE}" images ls | grep -q "${image_name}"; then
        log_success "Image ${image_name} found in containerd"
        ctr -n "${NAMESPACE}" images ls | grep "${image_name}" | head -1
        echo ""
        return 0
    fi
    if ctr -n "${NAMESPACE}" images ls | grep "${IMAGE_PREFIX}-${component}" | grep -q "${VERSION}"; then
        log_success "Image ${image_name} found in containerd (by prefix)"
        ctr -n "${NAMESPACE}" images ls | grep "${IMAGE_PREFIX}-${component}" | head -1
        echo ""
        return 0
    fi
    if nerdctl --namespace "${NAMESPACE}" images | grep -q "${IMAGE_PREFIX}-${component}"; then
        log_success "Image ${image_name} found (nerdctl)"
        return 0
    fi
    log_error "Image ${image_name} not found in containerd"
    return 1
}

# Export image for distribution
export_image() {
    local component=$1
    local image_name="${IMAGE_PREFIX}-${component}:${VERSION}"
    local export_file="/tmp/${IMAGE_PREFIX}-${component}-${VERSION}.tar"
    
    log_info "Exporting ${component} image..."
    
    if nerdctl --namespace "${NAMESPACE}" save -o "${export_file}" "${image_name}"; then
        local size=$(du -h "${export_file}" | cut -f1)
        log_success "Image exported to ${export_file} (${size})"
        echo ""
        echo "To import on another node:"
        echo "  ctr -n ${NAMESPACE} images import ${export_file}"
        echo "  # Or:"
        echo "  nerdctl --namespace ${NAMESPACE} load -i ${export_file}"
        echo ""
        return 0
    else
        log_error "Failed to export image"
        return 1
    fi
}

# Main
main() {
    echo "=========================================="
    echo "Fortuna Build and Load (Containerd)"
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
    
    # Build Core
    build_image "core" "core/Dockerfile"
    verify_image "core"
    
    # Build Agent
    build_image "agent" "agent/Dockerfile"
    verify_image "agent"
    
    # Build Dashboard (optional - can be skipped with SKIP_DASHBOARD=true)
    if [ "${SKIP_DASHBOARD:-false}" != "true" ]; then
        build_image "dashboard" "dashboard/Dockerfile"
        verify_image "dashboard"
    else
        log_info "Skipping dashboard build (SKIP_DASHBOARD=true)"
    fi
    
    # Export images if requested
    if [ "${EXPORT_IMAGES:-false}" = "true" ]; then
        log_info "Exporting images for distribution..."
        export_image "core"
        export_image "agent"
        if [ "${SKIP_DASHBOARD:-false}" != "true" ]; then
            export_image "dashboard"
        fi
    fi
    
    # Summary
    echo "=========================================="
    log_success "Build Complete!"
    echo "=========================================="
    echo ""
    echo "Images available in containerd:"
    ctr -n "${NAMESPACE}" images ls | grep "${IMAGE_PREFIX}" | head -6
    echo ""
    echo "To use in Kubernetes deployments:"
    echo "  image: ${IMAGE_PREFIX}-core:${VERSION}"
    echo "  image: ${IMAGE_PREFIX}-agent:${VERSION}"
    if [ "${SKIP_DASHBOARD:-false}" != "true" ]; then
        echo "  image: ${IMAGE_PREFIX}-dashboard:${VERSION}"
    fi
    echo "  imagePullPolicy: Never  # for local images"
    echo ""
    echo "To export images for other nodes:"
    echo "  EXPORT_IMAGES=true ${0}"
    echo ""
}

# Run main
main


